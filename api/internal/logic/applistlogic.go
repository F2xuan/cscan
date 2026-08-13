package logic

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type AppListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppListLogic {
	return &AppListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AppListLogic) AppList(req *types.AppListReq) (*types.AppListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	stats, err := l.aggregateAppStats()
	if err != nil {
		return nil, err
	}

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)
	filtered := make([]model.StatResult, 0, len(stats))
	keyword := strings.TrimSpace(req.Query)
	for _, stat := range stats {
		if keyword == "" || strings.Contains(strings.ToLower(stat.Field), strings.ToLower(keyword)) {
			filtered = append(filtered, stat)
		}
	}

	total := int64(len(filtered))
	start := (req.Page - 1) * req.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + req.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	pageItems := filtered[start:end]
	if len(pageItems) == 0 {
		return &types.AppListResp{Code: 0, Msg: "success", Total: total, List: []types.AppItem{}}, nil
	}

	// 仅对当前页的 app 用 $in 批量查询资产（N+1 → 1 次批量查询）
	// 每个 app 取最多 20 条资产，用于展示 host 列表/createTime/updateTime/orgName
	pageApps := make([]string, 0, len(pageItems))
	for _, s := range pageItems {
		pageApps = append(pageApps, s.Field)
	}
	assetsByApp, err := l.findAppAssetsBatch(pageApps)
	if err != nil {
		return nil, err
	}

	list := make([]types.AppItem, 0, len(pageItems))
	for _, stat := range pageItems {
		assets := assetsByApp[stat.Field]
		assetNames := make([]string, 0, len(assets))
		var createTime, updateTime string
		orgName := ""
		for _, asset := range assets {
			assetNames = append(assetNames, asset.Host)
			if assetCreate := asset.CreateTime.Local().Format("2006-01-02 15:04:05"); createTime == "" || assetCreate < createTime {
				createTime = assetCreate
			}
			if assetUpdate := asset.UpdateTime.Local().Format("2006-01-02 15:04:05"); assetUpdate > updateTime {
				updateTime = assetUpdate
			}
			if orgName == "" {
				orgName = orgMap[asset.OrgId]
			}
		}

		list = append(list, types.AppItem{
			Id:         stat.Field,
			App:        stat.Field,
			Category:   "-",
			Assets:     assetNames,
			OrgName:    orgName,
			CreateTime: createTime,
			UpdateTime: updateTime,
		})
	}

	return &types.AppListResp{Code: 0, Msg: "success", Total: total, List: list}, nil
}

func (l *AppListLogic) AppStat() (*types.AppStatResp, error) {
	stats, err := l.aggregateAppStats()
	if err != nil {
		return nil, err
	}

	newCount, err := l.countNewAppAssets()
	if err != nil {
		return nil, err
	}

	return &types.AppStatResp{Code: 0, Msg: "success", Total: len(stats), NewCount: int(newCount)}, nil
}

func (l *AppListLogic) aggregateAppStats() ([]model.StatResult, error) {
	cacheKey := "app_stats"
	cached, err := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		stats, err := l.svcCtx.GetAssetModel().AggregateApp(l.ctx, 1000)
		if err != nil {
			return nil, err
		}
		sort.Slice(stats, func(i, j int) bool {
			if stats[i].Count == stats[j].Count {
				return stats[i].Field < stats[j].Field
			}
			return stats[i].Count > stats[j].Count
		})
		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	if results, ok := cached.([]model.StatResult); ok {
		return results, nil
	}
	return nil, nil
}

// findAppAssetsBatch 批量查询多个 app 对应的资产（替代 N+1 的 findAppAssets）
// 一次 $in 查询所有 app，再按 app 分组，每个 app 最多保留 20 条最新资产
// 优化点：用 FindWithSort 走 AssetListProjection，排除 body/header/cert/banner 等大字段
func (l *AppListLogic) findAppAssetsBatch(apps []string) (map[string][]model.Asset, error) {
	if len(apps) == 0 {
		return make(map[string][]model.Asset), nil
	}
	result := make(map[string][]model.Asset, len(apps))

	limit := int64(len(apps) * 20)
	filter := bson.M{"app": bson.M{"$in": apps}}

	assets, err := l.svcCtx.GetAssetModel().FindWithSort(l.ctx, filter, 1, int(limit), "update_time")
	if err != nil {
		return nil, err
	}
	for _, asset := range assets {
		for _, app := range asset.App {
			if containsString(apps, app) {
				result[app] = append(result[app], asset)
			}
		}
	}

	// 每个 app 最多保留 20 条，并按 updateTime 降序
	for app, assets := range result {
		sort.Slice(assets, func(i, j int) bool {
			return assets[i].UpdateTime.After(assets[j].UpdateTime)
		})
		if len(assets) > 20 {
			assets = assets[:20]
		}
		result[app] = assets
	}
	return result, nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (l *AppListLogic) countNewAppAssets() (int64, error) {
	return l.svcCtx.GetAssetModel().Count(l.ctx, bson.M{"app": bson.M{"$exists": true, "$ne": bson.A{}}, "new": true})
}

func (l *AppListLogic) AppBatchDelete(req *types.AppBatchDeleteReq) (*types.BaseResp, error) {
	if len(req.Ids) == 0 {
		return &types.BaseResp{Code: 400, Msg: "请选择要删除的应用"}, nil
	}

	deleted, err := l.deleteAppAssets(bson.M{"app": bson.M{"$in": req.Ids}})
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *AppListLogic) AppClear() (*types.BaseResp, error) {
	deleted, err := l.deleteAppAssets(bson.M{"app": bson.M{"$exists": true, "$ne": bson.A{}}})
	if err != nil {
		return nil, err
	}
	return &types.BaseResp{Code: 0, Msg: "成功清空 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *AppListLogic) deleteAppAssets(filter bson.M) (int64, error) {
	return l.svcCtx.GetAssetModel().DeleteByFilter(l.ctx, filter)
}
