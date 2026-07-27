package logic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// jsfinderListCacheTTL 列表结果缓存时长，平衡刷新延迟与重复扫描开销
	jsfinderListCacheTTL = 30 * time.Second
)

// jsfinderListProjection 列表查询投影：排除 request/response/curl_command 等大字段，
// 这些字段剥离后单行内存占用大幅下降，跨 ws 合并 + 内存分页可移除硬上限，
// 从而修复"深页取空"问题；大字段由 /jsfinder/detail 按需加载。
var jsfinderListProjection = bson.M{
	"request":      0,
	"response":     0,
	"curl_command": 0,
}

// JSFinderConfigLogic JSFinder 配置逻辑
type JSFinderConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJSFinderConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JSFinderConfigLogic {
	return &JSFinderConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Get 获取 JSFinder 配置（不存在则返回内置默认值）
func (l *JSFinderConfigLogic) Get() (*types.JSFinderConfigResp, error) {
	m := model.NewJSFinderConfigModel(l.svcCtx.MongoDB)
	doc, err := m.Get(l.ctx)
	if err != nil {
		l.Errorf("[JSFinderConfig] Get error: %v", err)
		return &types.JSFinderConfigResp{Code: 500, Msg: "获取JSFinder配置失败"}, nil
	}

	updateTime := ""
	if !doc.UpdateTime.IsZero() {
		updateTime = doc.UpdateTime.Format("2006-01-02 15:04:05")
	}

	return &types.JSFinderConfigResp{
		Code: 0,
		Msg:  "success",
		Data: &types.JSFinderConfig{
			HighRiskRoutes:       doc.HighRiskRoutes,
			AuthRequiredKeywords: doc.AuthRequiredKeywords,
			SensitiveKeywords:    doc.SensitiveKeywords,
			DomainBlacklist:      doc.DomainBlacklist,
			UpdateTime:           updateTime,
		},
	}, nil
}

// Save 保存 JSFinder 配置
func (l *JSFinderConfigLogic) Save(req *types.JSFinderConfigSaveReq) (*types.JSFinderConfigResp, error) {
	m := model.NewJSFinderConfigModel(l.svcCtx.MongoDB)

	doc := &model.JSFinderConfig{
		HighRiskRoutes:       sanitizeJSFinderList(req.HighRiskRoutes),
		AuthRequiredKeywords: sanitizeJSFinderList(req.AuthRequiredKeywords),
		SensitiveKeywords:    sanitizeJSFinderList(req.SensitiveKeywords),
		DomainBlacklist:      sanitizeJSFinderList(req.DomainBlacklist),
		UpdateTime:           time.Now(),
	}

	if err := m.Save(l.ctx, doc); err != nil {
		l.Errorf("[JSFinderConfig] Save error: %v", err)
		return &types.JSFinderConfigResp{Code: 500, Msg: "保存JSFinder配置失败"}, nil
	}

	return &types.JSFinderConfigResp{
		Code: 0,
		Msg:  "保存成功",
		Data: &types.JSFinderConfig{
			HighRiskRoutes:       doc.HighRiskRoutes,
			AuthRequiredKeywords: doc.AuthRequiredKeywords,
			SensitiveKeywords:    doc.SensitiveKeywords,
			DomainBlacklist:      doc.DomainBlacklist,
			UpdateTime:           doc.UpdateTime.Format("2006-01-02 15:04:05"),
		},
	}, nil
}

// Reset 重置为内置默认值
func (l *JSFinderConfigLogic) Reset() (*types.JSFinderConfigResp, error) {
	m := model.NewJSFinderConfigModel(l.svcCtx.MongoDB)

	def := model.NewDefaultJSFinderConfig()
	if err := m.Save(l.ctx, def); err != nil {
		l.Errorf("[JSFinderConfig] Reset error: %v", err)
		return &types.JSFinderConfigResp{Code: 500, Msg: "重置JSFinder配置失败"}, nil
	}

	return &types.JSFinderConfigResp{
		Code: 0,
		Msg:  "重置成功",
		Data: &types.JSFinderConfig{
			HighRiskRoutes:       def.HighRiskRoutes,
			AuthRequiredKeywords: def.AuthRequiredKeywords,
			SensitiveKeywords:    def.SensitiveKeywords,
			DomainBlacklist:      def.DomainBlacklist,
			UpdateTime:           def.UpdateTime.Format("2006-01-02 15:04:05"),
		},
	}, nil
}

// sanitizeJSFinderList 去除空字符串与首尾空格，保留顺序与重复（用户自管去重）
func sanitizeJSFinderList(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

type JSFinderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJSFinderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JSFinderLogic {
	return &JSFinderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SaveJSFinderResult 保存 JSFinder 扫描结果
func (l *JSFinderLogic) SaveJSFinderResult(req *types.SaveJSFinderResultReq) error {
	if req.WorkspaceId == "" {
		return xerr.NewParamError("workspaceId cannot be empty")
	}

	if len(req.Results) == 0 {
		return nil
	}

	modelResults := make([]*model.JSFinderResult, 0, len(req.Results))
	for _, r := range req.Results {
		modelResults = append(modelResults, &model.JSFinderResult{
			WorkspaceId:      req.WorkspaceId,
			MainTaskId:       req.MainTaskId,
			Authority:        r.Authority,
			Host:             r.Host,
			Port:             r.Port,
			URL:              r.URL,
			Severity:         r.Severity,
			VulName:          r.VulName,
			Result:           r.Result,
			Tags:             r.Tags,
			MatcherName:      r.MatcherName,
			ExtractedResults: r.ExtractedResults,
			CurlCommand:      r.CurlCommand,
			Request:          r.Request,
			Response:         r.Response,
		})
	}

	m := l.svcCtx.GetJSFinderResultModel(req.WorkspaceId)
	// 确保索引存在
	_ = m.EnsureIndexes(l.ctx)

	if err := m.InsertMany(l.ctx, modelResults); err != nil {
		l.Logger.Errorf("SaveJSFinderResult Error: %v", err)
		// InsertMany可能会因为唯一索引冲突而报错，在这里忽略 Duplicate Key Error，保证其余正常插入
		// 这里只是打出错误日志，由于 MongoDB 的 InsertMany Ordered: false，出错条目会被跳过
	}

	return nil
}

// GetJSFinderList 获取 JSFinder 结果列表（带 30s 缓存）
func (l *JSFinderLogic) GetJSFinderList(req *types.JSFinderListReq) (*types.JSFinderListResp, error) {
	wsKey := req.WorkspaceId
	if wsKey == "" {
		wsKey = "all"
	}
	cacheKey := fmt.Sprintf("jsfinder_list:%s:%d:%d:%s:%s:%s:%s",
		wsKey, req.Page, req.PageSize, req.Query, req.Severity, req.Tags, req.MatcherName)

	cached, cerr := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, jsfinderListCacheTTL, func() (interface{}, error) {
		return l.getJSFinderListUncached(req)
	})
	if cerr != nil {
		l.Logger.Errorf("[JSFinder] 缓存读取失败: %v", cerr)
		return l.getJSFinderListUncached(req)
	}
	if r, ok := cached.(*types.JSFinderListResp); ok && r != nil {
		return r, nil
	}
	return l.getJSFinderListUncached(req)
}

// getJSFinderListUncached 无缓存版本：实际查询逻辑
func (l *JSFinderLogic) getJSFinderListUncached(req *types.JSFinderListReq) (*types.JSFinderListResp, error) {
	workspaceId := req.WorkspaceId

	filter := bson.M{}

	if req.Query != "" {
		filter["$or"] = []bson.M{
			{"url": primitive.Regex{Pattern: req.Query, Options: "i"}},
			{"vul_name": primitive.Regex{Pattern: req.Query, Options: "i"}},
			{"host": primitive.Regex{Pattern: req.Query, Options: "i"}},
		}
	}

	if req.Severity != "" {
		filter["severity"] = req.Severity
	}

	if req.Tags != "" {
		filter["tags"] = req.Tags
	}

	if req.MatcherName != "" {
		filter["matcher_name"] = req.MatcherName
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	var total int64
	var allResults []*model.JSFinderResult

	// 支持多工作空间查询
	if len(wsIds) > 1 || workspaceId == "" || workspaceId == "all" {
		// 跨工作空间分页正确性：从每个 ws 按 create_time 降序拉取覆盖到当前页末尾的数据量
		// （needTotal = page * pageSize），合并排序后在内存中按页切片。
		// OOM 防护改由 jsfinderListProjection 投影剥离 request/response/curl_command 大字段实现，
		// 单行内存占用大幅下降，故不再设硬上限 —— 原上限会令 page*pageSize 超限后的所有页空数据。
		needTotal := req.Page * req.PageSize

		// 跨工作空间合并：filter 为空时使用 EstimatedCount（O(1)），避免逐 ws CountDocuments 扫描
		emptyFilter := len(filter) == 0
		for _, wsId := range wsIds {
			m := l.svcCtx.GetJSFinderResultModel(wsId)
			var wsTotal int64
			if emptyFilter {
				wsTotal, _ = m.EstimatedCount(l.ctx)
			} else {
				wsTotal, _ = m.Count(l.ctx, filter)
			}
			total += wsTotal

			if wsTotal == 0 {
				continue
			}
			// 每个 ws 最多拉 needTotal 条（覆盖到当前页末尾），按 create_time 降序
			limit := int64(needTotal)
			if wsTotal < limit {
				limit = wsTotal
			}
			opt := options.Find().
				SetLimit(limit).
				SetSort(bson.D{{Key: "create_time", Value: -1}}).
				SetProjection(jsfinderListProjection)
			wsResults, _ := m.Find(l.ctx, filter, opt)
			allResults = append(allResults, wsResults...)
		}

		// 按创建时间排序
		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].CreateTime.After(allResults[j].CreateTime)
		})

		// 内存分页
		start := (req.Page - 1) * req.PageSize
		end := start + req.PageSize
		if start > len(allResults) {
			start = len(allResults)
		}
		if end > len(allResults) {
			end = len(allResults)
		}
		allResults = allResults[start:end]
	} else {
		m := l.svcCtx.GetJSFinderResultModel(workspaceId)

		var err error
		if len(filter) == 0 {
			total, err = m.EstimatedCount(l.ctx)
		} else {
			total, err = m.Count(l.ctx, filter)
		}
		if err != nil {
			return nil, xerr.NewServerError("Count JSFinderResult Error: " + err.Error())
		}

		opt := options.Find().
			SetSkip(int64((req.Page - 1) * req.PageSize)).
			SetLimit(int64(req.PageSize)).
			SetSort(bson.D{{Key: "create_time", Value: -1}}).
			SetProjection(jsfinderListProjection)

		allResults, err = m.Find(l.ctx, filter, opt)
		if err != nil {
			return nil, xerr.NewServerError("Find JSFinderResult Error: " + err.Error())
		}
	}

	respList := make([]*types.JSFinderResult, 0, len(allResults))
	for _, r := range allResults {
		respList = append(respList, &types.JSFinderResult{
			Id:               r.Id.Hex(),
			WorkspaceId:      r.WorkspaceId,
			MainTaskId:       r.MainTaskId,
			TaskName:         r.TaskName,
			Authority:        r.Authority,
			Host:             r.Host,
			Port:             r.Port,
			URL:              r.URL,
			Severity:         r.Severity,
			VulName:          r.VulName,
			Result:           r.Result,
			Tags:             r.Tags,
			MatcherName:      r.MatcherName,
			ExtractedResults: r.ExtractedResults,
			CurlCommand:      r.CurlCommand,
			Request:          r.Request,
			Response:         r.Response,
			CreateTime:       r.CreateTime.Format("2006-01-02 15:04:05"),
			UpdateTime:       r.UpdateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &types.JSFinderListResp{
		Code:  0,
		Msg:   "success",
		Total: total,
		List:  respList,
	}, nil
}

// ClearJSFinderResults 清空 JSFinder 结果
func (l *JSFinderLogic) ClearJSFinderResults(workspaceId string) error {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	for _, wsId := range wsIds {
		m := l.svcCtx.GetJSFinderResultModel(wsId)
		_, err := m.DeleteMany(l.ctx, bson.M{})
		if err != nil {
			l.Logger.Errorf("ClearJSFinderResults Error for workspace %s: %v", wsId, err)
			return xerr.NewServerError("清空JSFinder结果失败: " + err.Error())
		}
	}

	return nil
}

// GetJSFinderDetail 按 id 取单条 JSFinder 结果（含 request/response/curl_command 大字段）。
// 列表查询已投影剥离这些大字段，详情按需回填；id 在哪个 workspace 由前端随 id 一并给出。
// workspaceId 为空/"all" 时遍历所有工作空间定位，命中后返回。
func (l *JSFinderLogic) GetJSFinderDetail(req *types.JSFinderDetailReq) (*types.JSFinderDetailResp, error) {
	id := strings.TrimSpace(req.Id)
	if id == "" {
		return &types.JSFinderDetailResp{Code: 400, Msg: "id 不能为空"}, nil
	}

	wsIds := []string{strings.TrimSpace(req.WorkspaceId)}
	if wsIds[0] == "" || wsIds[0] == "all" {
		wsIds = common.GetWorkspaceIds(l.ctx, l.svcCtx, req.WorkspaceId)
	}

	for _, wsId := range wsIds {
		doc, err := l.svcCtx.GetJSFinderResultModel(wsId).FindByID(l.ctx, id)
		if err != nil {
			continue // 该 ws 无此 id 或集合缺失，尝试下一个
		}
		if doc == nil {
			continue
		}
		return &types.JSFinderDetailResp{
			Code: 0,
			Msg:  "success",
			Data: &types.JSFinderResult{
				Id:               doc.Id.Hex(),
				WorkspaceId:      doc.WorkspaceId,
				MainTaskId:       doc.MainTaskId,
				TaskName:         doc.TaskName,
				Authority:        doc.Authority,
				Host:             doc.Host,
				Port:             doc.Port,
				URL:              doc.URL,
				Severity:         doc.Severity,
				VulName:          doc.VulName,
				Result:           doc.Result,
				Tags:             doc.Tags,
				MatcherName:      doc.MatcherName,
				ExtractedResults: doc.ExtractedResults,
				CurlCommand:      doc.CurlCommand,
				Request:          doc.Request,
				Response:         doc.Response,
				CreateTime:       doc.CreateTime.Format("2006-01-02 15:04:05"),
				UpdateTime:       doc.UpdateTime.Format("2006-01-02 15:04:05"),
			},
		}, nil
	}

	return &types.JSFinderDetailResp{Code: 404, Msg: "未找到该 JSFinder 结果"}, nil
}
