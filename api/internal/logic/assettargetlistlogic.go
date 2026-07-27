package logic

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

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
		return nil, fmt.Errorf("invalid targetType %q", targetType)
	}

	type metaWithWs struct {
		doc   model.AssetTargetMeta
		wsId  string
		stamp int64
	}
	merged := make([]metaWithWs, 0, 64)

	for _, wsId := range wsIds {
		metaModel := l.svcCtx.GetAssetTargetMetaModel(wsId)
		docs, _, err := metaModel.FindAll(l.ctx, targetType, query, req.Labels, 0, 0)
		if err != nil {
			l.Logger.Errorf("[AssetTargetList] FindAll ws=%s fail: %v", wsId, err)
			continue
		}
		for _, d := range docs {
			merged = append(merged, metaWithWs{
				doc:   d,
				wsId:  wsId,
				stamp: lastScanTs(d),
			})
		}
	}

	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].stamp != merged[j].stamp {
			return merged[i].stamp > merged[j].stamp
		}
		return merged[i].doc.Id < merged[j].doc.Id
	})

	total := int64(len(merged))
	start := (req.Page - 1) * req.PageSize
	if start >= len(merged) {
		return &types.AssetTargetListResp{
			Code:  0,
			Msg:   "success",
			Total: total,
			List:  []types.AssetTargetListItem{},
		}, nil
	}
	end := start + req.PageSize
	if end > len(merged) {
		end = len(merged)
	}
	pageDocs := merged[start:end]

	// 懒回填：仅对当前页需要刷新的 meta 触发实时计算 + 回写，避免全量 N+1
	detailLogic := NewAssetTargetDetailLogic(l.ctx, l.svcCtx)
	list := make([]types.AssetTargetListItem, 0, len(pageDocs))
	for _, mw := range pageDocs {
		d := mw.doc
		if model.NeedsRefresh(&d, assetTargetDenormMaxAge) {
			l.refreshDenormalized(detailLogic, mw.wsId, &d)
		}
		list = append(list, metaToItem(d))
	}

	return &types.AssetTargetListResp{
		Code:  0,
		Msg:   "success",
		Total: total,
		List:  list,
	}, nil
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

func lastScanTs(m model.AssetTargetMeta) int64 {
	if !m.LastScanTime.IsZero() {
		return m.LastScanTime.UnixMilli()
	}
	return m.UpdateTime.UnixMilli()
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
