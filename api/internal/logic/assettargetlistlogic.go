package logic

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
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

// AssetTargetList 顶层资产（IP/主域名）分页列表。
func (l *AssetTargetListLogic) AssetTargetList(req *types.AssetTargetListReq) (*types.AssetTargetListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}

	cacheKey := buildAssetTargetListCacheKey(req)
	cached, cerr := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, assetTargetListCacheTTL, func() (interface{}, error) {
		return l.buildList(req)
	})
	if cerr != nil {
		l.Logger.Errorf("[AssetTargetList] cache read fail: %v", cerr)
		return l.buildList(req)
	}
	if r, ok := cached.(*types.AssetTargetListResp); ok && r != nil {
		return r, nil
	}
	return l.buildList(req)
}

func (l *AssetTargetListLogic) buildList(req *types.AssetTargetListReq) (*types.AssetTargetListResp, error) {
	query := strings.TrimSpace(req.Query)
	targetType := strings.TrimSpace(req.TargetType)
	if targetType != "" && targetType != string(model.AssetTargetTypeIP) && targetType != string(model.AssetTargetTypeDomain) {
		return nil, xerr.NewParamError(fmt.Sprintf("invalid targetType %q", targetType))
	}

	detailLogic := NewAssetTargetDetailLogic(l.ctx, l.svcCtx)

	metaModel := l.svcCtx.GetAssetTargetMetaModel()
	docs, total, err := metaModel.FindPage(l.ctx, targetType, query, req.Labels, req.Page, req.PageSize, "last_scan_time")
	if err != nil {
		return nil, err
	}
	list := make([]types.AssetTargetListItem, 0, len(docs))
	for i := range docs {
		d := &docs[i]
		if model.NeedsRefresh(d, assetTargetDenormMaxAge) {
			l.refreshDenormalized(detailLogic, d)
		}
		list = append(list, metaToItem(*d))
	}
	return &types.AssetTargetListResp{Code: 0, Msg: "success", Total: total, List: list}, nil
}

// refreshDenormalized 复用 detail logic 的 computeExposure/computeRisk 重新算,
// 把 snapshot 写回 meta 集合并覆盖入参 doc 的内联字段，供本次响应返回最新值。
// UpdateDenormalized 失败仅记日志，不影响本次响应（保持内存副本已刷新）。
func (l *AssetTargetListLogic) refreshDenormalized(dl *AssetTargetDetailLogic, d *model.AssetTargetMeta) {
	tType := model.AssetTargetType(d.TargetType)
	exp := dl.computeExposure(tType, d.TargetValue)
	risk := dl.computeRisk(tType, d.TargetValue)

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
	if err := l.svcCtx.GetAssetTargetMetaModel().UpdateDenormalized(l.ctx, d.Id, expSnap, riskSnap); err != nil {
		l.Logger.Errorf("[AssetTargetList] UpdateDenormalized id=%s fail: %v", d.Id, err)
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

		ExposureSubdomains:  m.ExposureSubdomains,
		ExposureIps:         m.ExposureIps,
		ExposurePorts:       m.ExposurePorts,
		ExposureSites:       m.ExposureSites,
		ExposureIcons:       m.ExposureIcons,
		ExposureApps:        m.ExposureApps,
		ExposureDirs:        m.ExposureDirs,
		ExposureJs:          m.ExposureJs,
		ExposureScreenshots: m.ExposureScreenshots,
		RiskSensitiveInfo:   m.RiskSensitiveInfo,
		RiskSensitiveDir:    m.RiskSensitiveDir,
		RiskVulnHigh:        m.RiskVulnHigh,
		RiskVulnTotal:       m.RiskVulnTotal,
	}
}

func tsMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func buildAssetTargetListCacheKey(req *types.AssetTargetListReq) string {
	labelsHash := sha1.Sum([]byte(strings.Join(req.Labels, ",")))
	queryHash := sha1.Sum([]byte(strings.TrimSpace(req.Query)))
	return fmt.Sprintf("asset_target_list:%s:%s:%s:%d:%d",
		strings.TrimSpace(req.TargetType),
		hex.EncodeToString(queryHash[:6]),
		hex.EncodeToString(labelsHash[:6]),
		req.Page, req.PageSize,
	)
}
