package logic

import (
	"context"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// AssetFingerprintsListLogic 资产指纹列表
type AssetFingerprintsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetFingerprintsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetFingerprintsListLogic {
	return &AssetFingerprintsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssetFingerprintsListLogic) AssetFingerprintsList(req *types.AssetFingerprintsListReq) (*types.AssetFingerprintsListResp, error) {
	// Limit 在缓存后再裁剪，不影响缓存命中率
	cacheKey := "asset_fingerprints"
	cached, err := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		fpSet := make(map[string]struct{})

		// 资产字段名是 app（数组）
		assetModel := l.svcCtx.GetAssetModel()
		values, err := assetModel.Distinct(l.ctx, "app", nil)
		if err != nil {
			l.Logger.Errorf("获取指纹列表失败: %v", err)
		} else {
			for _, v := range values {
				if s, ok := v.(string); ok && s != "" {
					fpSet[s] = struct{}{}
				}
			}
		}

		result := make([]string, 0, len(fpSet))
		for fp := range fpSet {
			result = append(result, fp)
		}
		return result, nil
	})

	if err != nil {
		l.Logger.Errorf("获取指纹列表失败: %v", err)
		return &types.AssetFingerprintsListResp{
			Code: 500,
			Msg:  "获取指纹列表失败",
			List: []string{},
		}, nil
	}

	result, _ := cached.([]string)
	// 限制返回数量
	if req.Limit > 0 && len(result) > req.Limit {
		result = result[:req.Limit]
	}

	return &types.AssetFingerprintsListResp{
		Code: 0,
		Msg:  "success",
		List: result,
	}, nil
}

// AssetPortsStatsLogic 资产端口统计
type AssetPortsStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetPortsStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetPortsStatsLogic {
	return &AssetPortsStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}

}

func (l *AssetPortsStatsLogic) AssetPortsStats() (*types.AssetPortsStatsResp, error) {
	cacheKey := "asset_ports_stats"
	cached, err := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		merged := make(map[int]*types.PortStatItem)

		// 端口字段是 port（int），AssetModel.AggregatePort 已封装聚合管道
		assetModel := l.svcCtx.GetAssetModel()
		stats, err := assetModel.AggregatePort(l.ctx, 200)
		if err != nil {
			l.Logger.Errorf("获取端口统计失败: %v", err)
		} else {
			for _, s := range stats {
				if s.Port <= 0 {
					continue
				}
				if existing, ok := merged[s.Port]; ok {
					existing.Count += int64(s.Count)
				} else {
					merged[s.Port] = &types.PortStatItem{
						Port:  s.Port,
						Count: int64(s.Count),
					}
				}
			}
		}

		list := make([]types.PortStatItem, 0, len(merged))
		for _, v := range merged {
			list = append(list, *v)
		}
		return list, nil
	})

	if err != nil {
		l.Logger.Errorf("获取端口统计失败: %v", err)
		return &types.AssetPortsStatsResp{
			Code: 500,
			Msg:  "获取端口统计失败",
			List: []types.PortStatItem{},
		}, nil
	}

	list, _ := cached.([]types.PortStatItem)
	if list == nil {
		list = []types.PortStatItem{}
	}

	return &types.AssetPortsStatsResp{
		Code: 0,
		Msg:  "success",
		List: list,
	}, nil
}
