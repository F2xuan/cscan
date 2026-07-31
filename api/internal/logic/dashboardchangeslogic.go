package logic

import (
	"context"
	"math"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// DashboardChangesLogic 工作台变化数据聚合
type DashboardChangesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDashboardChangesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DashboardChangesLogic {
	return &DashboardChangesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DashboardChanges 聚合工作台"资产变化"与"漏洞变化"，窗口默认为 7 天
// 数据源：资产 first_seen_time 窗口 + 漏洞 status/first_seen_time/fixed_at 窗口（T1.1/T1.3 口径）
func (l *DashboardChangesLogic) DashboardChanges(req *types.DashboardChangesReq, workspaceId string) (*types.DashboardChangesResp, error) {
	days := req.Days
	if days <= 0 {
		days = 7
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	asset := &types.AssetChanges{Total: 0, NewInWindow: 0, ByCategory: map[string]int64{}}
	risk := &types.RiskChanges{Open: 0, NewInWindow: 0, FixedInWindow: 0, BySeverity: map[string]int64{}}

	for _, wsId := range wsIds {
		assetModel := l.svcCtx.GetAssetModel(wsId)
		aStats, err := assetModel.AggregateChangesStats(l.ctx, cutoff)
		if err != nil {
			l.Logger.Errorf("[DASHBOARD] asset changes stat failed ws=%s: %v", wsId, err)
		} else {
			asset.Total += aStats.Total
			asset.NewInWindow += aStats.NewInWindow
			for k, v := range aStats.ByCategory {
				asset.ByCategory[k] += v
			}
		}

		vulModel := l.svcCtx.GetVulModel(wsId)
		vStats, err := vulModel.AggregateChangesStats(l.ctx, cutoff)
		if err != nil {
			l.Logger.Errorf("[DASHBOARD] vul changes stat failed ws=%s: %v", wsId, err)
		} else {
			risk.Open += vStats.Open
			risk.NewInWindow += vStats.NewInWindow
			risk.FixedInWindow += vStats.FixedInWindow
			for k, v := range vStats.BySeverity {
				risk.BySeverity[k] += v
			}
		}
	}

	// 增长率：窗口内新增 / 总数
	if asset.Total > 0 {
		asset.GrowthRate = round1(float64(asset.NewInWindow) * 100 / float64(asset.Total))
	}
	// 净变化：新增 - 已修复
	risk.NetChange = risk.NewInWindow - risk.FixedInWindow

	return &types.DashboardChangesResp{
		Code:  0,
		Msg:   "success",
		Asset: asset,
		Risk:  risk,
	}, nil
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
