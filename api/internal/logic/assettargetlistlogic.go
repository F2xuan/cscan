package logic

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	assetTargetListCacheTTL = 30 * time.Second
	// assetTargetDenormMaxAge 决定 list 页触发懒回填的阈值：
	// 超过此值或字段缺失即重算 exposure+risk 快照。
	assetTargetDenormMaxAge = 30 * time.Minute
)

type AssetTargetListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetTargetListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetTargetListLogic {
	return &AssetTargetListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetTargetList 顶层资产（IP/主域名）分页列表，跨 workspace 合并。
func (l *AssetTargetListLogic) AssetTargetList(req *types.AssetTargetListReq, workspaceId string) (*types.AssetTargetListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	cacheKey := buildAssetTargetListCacheKey(wsIds, req)
	cached, cerr := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, assetTargetListCacheTTL, func() (interface{}, error) {
		return l.buildList(req, wsIds)
	})
	if cerr != nil {
		l.Logger.Errorf("[AssetTargetList] cache read fail: %v", cerr)
		return l.buildList(req, wsIds)
	}
	if r, ok := cached.(*types.AssetTargetListResp); ok && r != nil {
		return r, nil
	}
	return l.buildList(req, wsIds)
}

func (l *AssetTargetListLogic) buildList(req *types.AssetTargetListReq, wsIds []string) (*types.AssetTargetListResp, error) {
	query := strings.TrimSpace(req.Query)
	targetType := strings.TrimSpace(req.TargetType)
	if targetType != "" && targetType != string(model.AssetTargetTypeIP) && targetType != string(model.AssetTargetTypeDomain) {
		return nil, xerr.NewParamError(fmt.Sprintf("invalid targetType %q", targetType))
	}

	detailLogic := NewAssetTargetDetailLogic(l.ctx, l.svcCtx)

	// 单工作空间（常见路径）：分页和排序下推到 MongoDB
	if len(wsIds) == 1 {
		wsId := wsIds[0]
		metaModel := l.svcCtx.GetAssetTargetMetaModel(wsId)
		docs, total, err := metaModel.FindPage(l.ctx, targetType, query, req.Labels, req.Page, req.PageSize, "last_scan_time")
		if err != nil {
			return nil, err
		}
		list := make([]types.AssetTargetListItem, 0, len(docs))
		for i := range docs {
			d := &docs[i]
			if model.NeedsRefresh(d, assetTargetDenormMaxAge) {
				l.refreshDenormalized(detailLogic, wsId, d)
			}
			list = append(list, metaToItem(*d))
		}
		return &types.AssetTargetListResp{Code: 0, Msg: "success", Total: total, List: list}, nil
	}

	// 多工作空间：每 ws 拉取覆盖到当前页的数据量，内存合并后分页
	type metaWithWs struct {
		doc  model.AssetTargetMeta
		wsId string
	}
	needTotal := req.Page * req.PageSize
	if needTotal > 50000 {
		needTotal = 50000
	}
	merged := make([]metaWithWs, 0, 64)
	var total int64

	for _, wsId := range wsIds {
		metaModel := l.svcCtx.GetAssetTargetMetaModel(wsId)
		docs, wsTotal, err := metaModel.FindPage(l.ctx, targetType, query, req.Labels, 1, needTotal, "last_scan_time")
		if err != nil {
			l.Logger.Errorf("[AssetTargetList] FindPage ws=%s fail: %v", wsId, err)
			continue
		}
		total += wsTotal
		for _, d := range docs {
			merged = append(merged, metaWithWs{doc: d, wsId: wsId})
		}
	}

	// last_scan_time 已在 DB 层降序，跨 ws 合并后保持稳定排序
	start := (req.Page - 1) * req.PageSize
	if start >= len(merged) {
		return &types.AssetTargetListResp{Code: 0, Msg: "success", Total: total, List: []types.AssetTargetListItem{}}, nil
	}
	end := start + req.PageSize
	if end > len(merged) {
		end = len(merged)
	}
	pageDocs := merged[start:end]

	list := make([]types.AssetTargetListItem, 0, len(pageDocs))
	for _, mw := range pageDocs {
		d := mw.doc
		if model.NeedsRefresh(&d, assetTargetDenormMaxAge) {
			l.refreshDenormalized(detailLogic, mw.wsId, &d)
		}
		list = append(list, metaToItem(d))
	}
	return &types.AssetTargetListResp{Code: 0, Msg: "success", Total: total, List: list}, nil
}

// refreshDenormalized 复用 detail logic 的 computeExposure/computeRisk 重新算,
// 把 snapshot 写回 meta 集合并覆盖入参 doc 的内联字段，供本次响应返回最新值。
// UpdateDenormalized 失败仅记日志，不影响本次响应（保持内存副本已刷新）。
func (l *AssetTargetListLogic) refreshDenormalized(dl *AssetTargetDetailLogic, wsId string, d *model.AssetTargetMeta) {
	tType := model.AssetTargetType(d.TargetType)
	exp := dl.computeExposure(wsId, tType, d.TargetValue)
	risk := dl.computeRisk(wsId, tType, d.TargetValue)

	expSnap := model.ExposureSnapshot{
		Subdomains:  exp.Subdomains,
		Ips:         exp.Ips,
		Ports:       exp.Ports,
		Sites:       exp.Sites,
		Icons:       exp.Icons,
		Apps:        exp.Apps,
		Dirs:        exp.Dirs,
		Js:          exp.Js,
		Screenshots: exp.Screenshots,
	}
	riskSnap := model.RiskSnapshot{
		SensitiveInfo: risk.SensitiveInfo,
		SensitiveDir:  risk.SensitiveDir,
		VulnHigh:      risk.VulnHigh,
		VulnTotal:     risk.VulnTotal,
	}
	if err := l.svcCtx.GetAssetTargetMetaModel(wsId).UpdateDenormalized(l.ctx, d.Id, expSnap, riskSnap); err != nil {
		l.Logger.Errorf("[AssetTargetList] UpdateDenormalized ws=%s id=%s fail: %v", wsId, d.Id, err)
	}
	d.ExposureSubdomains = expSnap.Subdomains
	d.ExposureIps = expSnap.Ips
	d.ExposurePorts = expSnap.Ports
	d.ExposureSites = expSnap.Sites
	d.ExposureIcons = expSnap.Icons
	d.ExposureApps = expSnap.Apps
	d.ExposureDirs = expSnap.Dirs
	d.ExposureJs = expSnap.Js
	d.ExposureScreenshots = expSnap.Screenshots
	d.RiskSensitiveInfo = riskSnap.SensitiveInfo
	d.RiskSensitiveDir = riskSnap.SensitiveDir
	d.RiskVulnHigh = riskSnap.VulnHigh
	d.RiskVulnTotal = riskSnap.VulnTotal
	d.RiskUpdatedAt = time.Now()
}

func metaToItem(m model.AssetTargetMeta) types.AssetTargetListItem {
	labels := m.Labels
	if labels == nil {
		labels = []string{}
	}
	return types.AssetTargetListItem{
		Id:           m.Id,
		TargetType:   m.TargetType,
		TargetValue:  m.TargetValue,
		Labels:       labels,
		Memo:         m.Memo,
		ColorTag:     m.ColorTag,
		LastScanTime: tsMilli(m.LastScanTime),
		FirstSeen:    tsMilli(m.FirstSeenTime),
		TaskCount:    m.TaskCount,

		ExposureSubdomains: m.ExposureSubdomains,
		ExposureIps:        m.ExposureIps,
		ExposurePorts:      m.ExposurePorts,
		ExposureSites:      m.ExposureSites,
		ExposureIcons:      m.ExposureIcons,
		ExposureApps:       m.ExposureApps,
		ExposureDirs:       m.ExposureDirs,
		ExposureJs:         m.ExposureJs,
		ExposureScreenshots: m.ExposureScreenshots,
		RiskSensitiveInfo:  m.RiskSensitiveInfo,
		RiskSensitiveDir:   m.RiskSensitiveDir,
		RiskVulnHigh:       m.RiskVulnHigh,
		RiskVulnTotal:      m.RiskVulnTotal,
	}
}

func tsMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func buildAssetTargetListCacheKey(wsIds []string, req *types.AssetTargetListReq) string {
	wsHash := sha1.Sum([]byte(strings.Join(wsIds, ",")))
	labelsHash := sha1.Sum([]byte(strings.Join(req.Labels, ",")))
	queryHash := sha1.Sum([]byte(strings.TrimSpace(req.Query)))
	return fmt.Sprintf("asset_target_list:%s:%s:%s:%s:%d:%d",
		hex.EncodeToString(wsHash[:6]),
		strings.TrimSpace(req.TargetType),
		hex.EncodeToString(queryHash[:6]),
		hex.EncodeToString(labelsHash[:6]),
		req.Page, req.PageSize,
	)
}
