package logic

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type VulListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVulListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VulListLogic {
	return &VulListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VulListLogic) VulList(req *types.VulListReq, workspaceId string) (resp *types.VulListResp, err error) {
	// 构建查询条件
	filter := bson.M{}
	// 如果提供了通用 Query 且未显式指定 Authority/Host，则按多个字段模糊匹配
	if req.Query != "" && req.Authority == "" && req.Host == "" {
		q := regexp.QuoteMeta(req.Query)
		filter["$or"] = []bson.M{
			{"authority": bson.M{"$regex": q, "$options": "i"}},
			{"host": bson.M{"$regex": q, "$options": "i"}},
			{"url": bson.M{"$regex": q, "$options": "i"}},
			{"pocfile": bson.M{"$regex": q, "$options": "i"}},
			{"vul_name": bson.M{"$regex": q, "$options": "i"}},
		}
	}
	if req.Authority != "" {
		authQuery := req.Authority
		if strings.HasPrefix(authQuery, "http://") {
			authQuery = strings.TrimPrefix(authQuery, "http://")
		} else if strings.HasPrefix(authQuery, "https://") {
			authQuery = strings.TrimPrefix(authQuery, "https://")
		}
		authQuery = regexp.QuoteMeta(authQuery)
		filter["authority"] = bson.M{"$regex": authQuery, "$options": "i"}
	}
	if req.Severity != "" {
		filter["severity"] = req.Severity
	}
	if req.Source != "" {
		filter["source"] = req.Source
	}
	// 支持按host和port筛选（用于资产详情页查询漏洞）
	if req.Host != "" {
		filter["host"] = req.Host
	}
	if req.Port > 0 {
		filter["port"] = req.Port
	}
	// Phase 5: 敏感信息/敏感目录页面通过下列 3 个字段做服务端固定过滤。
	if req.IsRisk != nil {
		filter["is_risk"] = *req.IsRisk
	}
	if req.RiskSource != "" {
		filter["risk_source"] = req.RiskSource
	}
	if len(req.KeywordAny) > 0 {
		orClauses := make([]bson.M, 0, len(req.KeywordAny)*2)
		for _, kw := range req.KeywordAny {
			if kw == "" {
				continue
			}
			escaped := regexp.QuoteMeta(kw)
			orClauses = append(orClauses,
				bson.M{"vul_name": bson.M{"$regex": escaped, "$options": "i"}},
				bson.M{"tags": kw},
			)
		}
		if len(orClauses) > 0 {
			// 与已有 $or（通用 Query 模糊）冲突时后者优先，避免语义错乱：
			// 已有 Query 时不再叠加 keyword $or，让调用方二选一。
			if _, exists := filter["$or"]; !exists {
				filter["$or"] = orClauses
			}
		}
	}

	var total int64
	var vuls []model.Vul

	// 获取需要查询的工作空间列表
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	// 如果查询多个工作空间
	if len(wsIds) > 1 || workspaceId == "" || workspaceId == "all" {
		// 优化点：原实现 Find(filter, 0, 0) 把每个 ws 全部漏洞加载到内存，多 ws 时易 OOM
		// 现改为只拉取覆盖到当前页末尾的数据量（needTotal = page * pageSize），
		// 全局合并 + 排序 + 分页，既保证跨 ws 分页正确性又控制内存
		needTotal := req.Page * req.PageSize
		// 安全上限：避免用户翻到极深页时拉取过多数据
		if needTotal > 50000 {
			needTotal = 50000
		}

		var allVuls []model.Vul
		for _, wsId := range wsIds {
			vulModel := l.svcCtx.GetVulModel(wsId)
			wsTotal, _ := vulModel.Count(l.ctx, filter)
			total += wsTotal

			if wsTotal == 0 {
				continue
			}
			// 每个 ws 最多拉 needTotal 条（覆盖到当前页末尾），wsTotal 不足时按实际数拉
			limit := needTotal
			if wsTotal < int64(limit) {
				limit = int(wsTotal)
			}
			wsVuls, _ := vulModel.Find(l.ctx, filter, 1, limit)
			allVuls = append(allVuls, wsVuls...)
		}

		// 按创建时间排序
		sort.Slice(allVuls, func(i, j int) bool {
			return allVuls[i].CreateTime.After(allVuls[j].CreateTime)
		})

		// 分页
		start := (req.Page - 1) * req.PageSize
		end := start + req.PageSize
		if start > len(allVuls) {
			start = len(allVuls)
		}
		if end > len(allVuls) {
			end = len(allVuls)
		}
		vuls = allVuls[start:end]
	} else {
		vulModel := l.svcCtx.GetVulModel(workspaceId)

		// 查询总数
		total, err = vulModel.Count(l.ctx, filter)
		if err != nil {
			return &types.VulListResp{Code: 500, Msg: "查询失败"}, nil
		}

		// 查询列表
		vuls, err = vulModel.Find(l.ctx, filter, req.Page, req.PageSize)
		if err != nil {
			return &types.VulListResp{Code: 500, Msg: "查询失败"}, nil
		}
	}

	// 转换响应
	list := make([]types.Vul, 0, len(vuls))
	for _, v := range vuls {
		vul := types.Vul{
			Id:               v.Id.Hex(),
			Authority:        v.Authority,
			Url:              v.Url,
			PocFile:          v.PocFile,
			Source:           v.Source,
			Severity:         v.Severity,
			Result:           v.Result,
			VulName:          v.VulName,
			Tags:             v.Tags,
			CreateTime:       v.CreateTime.Local().Format("2006-01-02 15:04:05"),
			UpdateTime:       v.UpdateTime.Local().Format("2006-01-02 15:04:05"),
			ScanCount:        v.ScanCount,
			MatcherName:      v.MatcherName,
			ExtractedResults: v.ExtractedResults,
		}
		// 新增字段 - 时间追踪
		if !v.FirstSeenTime.IsZero() {
			vul.FirstSeenTime = v.FirstSeenTime.Local().Format("2006-01-02 15:04:05")
		}
		if !v.LastSeenTime.IsZero() {
			vul.LastSeenTime = v.LastSeenTime.Local().Format("2006-01-02 15:04:05")
		}
		list = append(list, vul)
	}

	return &types.VulListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  list,
	}, nil
}

// VulLogic 漏洞管理逻辑
type VulLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVulLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VulLogic {
	return &VulLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VulLogic) VulDelete(req *types.VulDeleteReq, workspaceId string) (resp *types.BaseResp, err error) {
	// 如果是全部空间模式，需要遍历查找并删除
	if workspaceId == "" || workspaceId == "all" {
		wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, "all")
		deleted := false
		for _, wsId := range wsIds {
			vulModel := l.svcCtx.GetVulModel(wsId)
			count, err := vulModel.Delete(l.ctx, req.Id)
			if err == nil && count > 0 {
				deleted = true
				break
			}
		}
		if !deleted {
			return &types.BaseResp{Code: 404, Msg: "漏洞不存在或删除失败"}, nil
		}
	} else {
		vulModel := l.svcCtx.GetVulModel(workspaceId)
		count, err := vulModel.Delete(l.ctx, req.Id)
		if err != nil {
			return &types.BaseResp{Code: 500, Msg: "删除失败: " + err.Error()}, nil
		}
		if count == 0 {
			return &types.BaseResp{Code: 404, Msg: "漏洞不存在"}, nil
		}
	}
	return &types.BaseResp{Code: 0, Msg: "删除成功"}, nil
}

func (l *VulLogic) VulBatchDelete(req *types.VulBatchDeleteReq, workspaceId string) (resp *types.BaseResp, err error) {
	var totalDeleted int64

	// 如果是全部空间模式，需要遍历所有工作空间删除
	if workspaceId == "" || workspaceId == "all" {
		wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, "all")
		for _, wsId := range wsIds {
			vulModel := l.svcCtx.GetVulModel(wsId)
			deleted, err := vulModel.BatchDelete(l.ctx, req.Ids)
			if err != nil {
				logx.Errorf("[VulBatchDelete] 删除工作空间 %s 漏洞失败: %v", wsId, err)
				continue
			}
			totalDeleted += deleted
		}
	} else {
		vulModel := l.svcCtx.GetVulModel(workspaceId)
		deleted, err := vulModel.BatchDelete(l.ctx, req.Ids)
		if err != nil {
			return &types.BaseResp{Code: 500, Msg: "删除失败: " + err.Error()}, nil
		}
		totalDeleted = deleted
	}

	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(totalDeleted, 10) + " 条记录"}, nil
}

func (l *VulLogic) VulClear(workspaceId string) (resp *types.BaseResp, err error) {
	var totalDeleted int64

	if workspaceId == "" || workspaceId == "all" {
		wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, "all")
		for _, wsId := range wsIds {
			vulModel := l.svcCtx.GetVulModel(wsId)
			deleted, err := vulModel.Clear(l.ctx)
			if err != nil {
				logx.Errorf("[VulClear] 清空工作空间 %s 漏洞失败: %v", wsId, err)
				continue
			}
			totalDeleted += deleted
		}
	} else {
		vulModel := l.svcCtx.GetVulModel(workspaceId)
		deleted, err := vulModel.Clear(l.ctx)
		if err != nil {
			return &types.BaseResp{Code: 500, Msg: "清空失败: " + err.Error()}, nil
		}
		totalDeleted = deleted
	}

	return &types.BaseResp{Code: 0, Msg: "成功清空 " + strconv.FormatInt(totalDeleted, 10) + " 条漏洞"}, nil
}

// VulStatLogic 漏洞统计逻辑
type VulStatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVulStatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VulStatLogic {
	return &VulStatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VulStatLogic) VulStat(workspaceId string) (resp *types.VulStatResp, err error) {
	// 聚合统计走 60s 缓存（带 singleflight 防击穿），扫描完成可主动失效
	cacheKey := "vul_stat:" + workspaceId
	cached, cacheErr := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		var total, critical, high, medium, low, info, week, month int64
		now := time.Now()

		// 获取需要查询的工作空间列表
		wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

		for _, wsId := range wsIds {
			vulModel := l.svcCtx.GetVulModel(wsId)
			stats, statErr := vulModel.AggregateStats(l.ctx, now)
			if statErr != nil {
				continue
			}
			total += stats.Total
			critical += stats.Critical
			high += stats.High
			medium += stats.Medium
			low += stats.Low
			info += stats.Info
			week += stats.Week
			month += stats.Month
		}

		return &types.VulStatResp{
			Code:     0,
			Msg:      "success",
			Total:    int(total),
			Critical: int(critical),
			High:     int(high),
			Medium:   int(medium),
			Low:      int(low),
			Info:     int(info),
			Week:     int(week),
			Month:    int(month),
		}, nil
	})
	if cacheErr != nil {
		return &types.VulStatResp{Code: 0, Msg: "success"}, nil
	}
	if r, ok := cached.(*types.VulStatResp); ok {
		return r, nil
	}
	return &types.VulStatResp{Code: 0, Msg: "success"}, nil
}

// VulDetailLogic 漏洞详情逻辑
type VulDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVulDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VulDetailLogic {
	return &VulDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VulDetailLogic) VulDetail(req *types.VulDetailReq, workspaceId string) (resp *types.VulDetailResp, err error) {
	if req.Id == "" {
		return &types.VulDetailResp{Code: 400, Msg: "漏洞ID不能为空"}, nil
	}

	var vul *model.Vul

	// 如果是全部空间模式，遍历所有工作空间查找
	if workspaceId == "" || workspaceId == "all" {
		wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, "all")
		for _, wsId := range wsIds {
			vulModel := l.svcCtx.GetVulModel(wsId)
			if v, err := vulModel.FindById(l.ctx, req.Id); err == nil && v != nil {
				vul = v
				break
			}
		}
	} else {
		vulModel := l.svcCtx.GetVulModel(workspaceId)
		vul, err = vulModel.FindById(l.ctx, req.Id)
	}

	if vul == nil {
		return &types.VulDetailResp{Code: 404, Msg: "漏洞不存在"}, nil
	}

	// 构建漏洞详情
	detail := &types.VulDetail{
		Id:         vul.Id.Hex(),
		Authority:  vul.Authority,
		Host:       vul.Host,
		Port:       vul.Port,
		Url:        vul.Url,
		PocFile:    vul.PocFile,
		Source:     vul.Source,
		Severity:   vul.Severity,
		Result:     vul.Result,
		VulName:    vul.VulName,
		Tags:       vul.Tags,
		CreateTime: vul.CreateTime.Local().Format("2006-01-02 15:04:05"),
		UpdateTime: vul.UpdateTime.Local().Format("2006-01-02 15:04:05"),
		// 知识库信息
		CvssScore:   vul.CvssScore,
		CveId:       vul.CveId,
		CweId:       vul.CweId,
		Remediation: vul.Remediation,
		References:  vul.References,
		// 时间追踪
		ScanCount: vul.ScanCount,
	}

	// 时间追踪字段
	if !vul.FirstSeenTime.IsZero() {
		detail.FirstSeenTime = vul.FirstSeenTime.Local().Format("2006-01-02 15:04:05")
	}
	if !vul.LastSeenTime.IsZero() {
		detail.LastSeenTime = vul.LastSeenTime.Local().Format("2006-01-02 15:04:05")
	}

	// 证据链
	if vul.MatcherName != "" || len(vul.ExtractedResults) > 0 || vul.CurlCommand != "" || vul.Request != "" || vul.Response != "" {
		detail.Evidence = &types.VulEvidence{
			MatcherName:       vul.MatcherName,
			ExtractedResults:  vul.ExtractedResults,
			CurlCommand:       vul.CurlCommand,
			Request:           vul.Request,
			Response:          vul.Response,
			ResponseTruncated: vul.ResponseTruncated,
		}
	}

	return &types.VulDetailResp{
		Code: 0,
		Msg:  "success",
		Data: detail,
	}, nil
}
