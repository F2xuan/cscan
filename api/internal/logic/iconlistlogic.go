package logic

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type IconListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIconListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IconListLogic {
	return &IconListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IconListLogic) IconList(req *types.IconListReq) (*types.IconListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	workspaceId := middleware.GetWorkspaceId(l.ctx)
	stats, err := l.aggregateIconStats(workspaceId)
	if err != nil {
		return nil, err
	}

	// 第一阶段：用 stat 数据（已含 count/iconData）做关键词过滤 + 分页
	// 不需要查 asset 即可得到 total 和当前页 icon hash 列表
	keyword := strings.TrimSpace(req.IconHash)
	filtered := make([]model.IconHashStatResult, 0, len(stats))
	for _, stat := range stats {
		if keyword != "" && !strings.Contains(strings.ToLower(stat.IconHash), strings.ToLower(keyword)) {
			continue
		}
		filtered = append(filtered, stat)
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
	pageStats := filtered[start:end]

	if len(pageStats) == 0 {
		return &types.IconListResp{Code: 0, Msg: "success", Total: total, List: []types.IconItem{}}, nil
	}

	// 第二阶段：仅对当前页的 icon 用 $in 批量查询资产（N+1 → 1 次批量查询）
	// 每个 icon 取最多 20 条资产，用于展示 host 列表/iconHashFile/screenshot/createTime/updateTime
	pageHashes := make([]string, 0, len(pageStats))
	for _, s := range pageStats {
		pageHashes = append(pageHashes, s.IconHash)
	}
	assetsByHash, err := l.findIconAssetsBatch(workspaceId, pageHashes)
	if err != nil {
		return nil, err
	}

	list := make([]types.IconItem, 0, len(pageStats))
	for _, stat := range pageStats {
		// stat.IconData 是 []byte，pickIconPresentation 需要 base64 字符串
		iconDataStr := ""
		if len(stat.IconData) > 0 {
			iconDataStr = base64.StdEncoding.EncodeToString(stat.IconData)
		}
		item, ok := l.pickIconPresentation(stat.IconHash, iconDataStr, assetsByHash[stat.IconHash])
		if !ok {
			continue
		}
		list = append(list, item)
	}

	return &types.IconListResp{Code: 0, Msg: "success", Total: total, List: list}, nil
}

func (l *IconListLogic) pickIconPresentation(iconHash, iconDataFromStat string, assets []model.Asset) (types.IconItem, bool) {
	assetNames := make([]string, 0, len(assets))
	seenHosts := make(map[string]struct{})
	iconHashFile := ""
	iconData := iconDataFromStat
	screenshot := ""
	var earliestCreate time.Time
	var latestUpdate time.Time

	for _, asset := range assets {
		if _, exists := seenHosts[asset.Host]; exists {
			continue
		}
		seenHosts[asset.Host] = struct{}{}
		assetNames = append(assetNames, asset.Host)
		if iconHashFile == "" && asset.IconHashFile != "" {
			iconHashFile = asset.IconHashFile
		}
		if iconData == "" && len(asset.IconHashBytes) > 0 {
			iconData = base64.StdEncoding.EncodeToString(asset.IconHashBytes)
		}
		if screenshot == "" && asset.Screenshot != "" {
			screenshot = asset.Screenshot
		}
		if !asset.CreateTime.IsZero() && (earliestCreate.IsZero() || asset.CreateTime.Before(earliestCreate)) {
			earliestCreate = asset.CreateTime
		}
		if !asset.UpdateTime.IsZero() && (latestUpdate.IsZero() || asset.UpdateTime.After(latestUpdate)) {
			latestUpdate = asset.UpdateTime
		}
	}

	if iconData == "" {
		return types.IconItem{}, false
	}

	item := types.IconItem{
		Id:           iconHash,
		IconHash:     iconHash,
		IconHashFile: iconHashFile,
		IconData:     iconData,
		Screenshot:   screenshot,
		Assets:       assetNames,
	}
	if !earliestCreate.IsZero() {
		item.CreateTime = earliestCreate.Format("2006-01-02 15:04:05")
	}
	if !latestUpdate.IsZero() {
		item.UpdateTime = latestUpdate.Format("2006-01-02 15:04:05")
	}
	return item, true
}

func (l *IconListLogic) IconStat() (*types.IconStatResp, error) {
	workspaceId := middleware.GetWorkspaceId(l.ctx)
	stats, err := l.aggregateIconStats(workspaceId)
	if err != nil {
		return nil, err
	}

	newCount, err := l.countNewIconAssets(workspaceId)
	if err != nil {
		return nil, err
	}

	return &types.IconStatResp{Code: 0, Msg: "success", Total: len(stats), NewCount: int(newCount)}, nil
}

func (l *IconListLogic) aggregateIconStats(workspaceId string) ([]model.IconHashStatResult, error) {
	// 聚合结果走 60s 缓存（带 singleflight 防击穿），扫描完成可主动失效
	cacheKey := "icon_stats:" + workspaceId
	cached, err := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
		merged := make(map[string]model.IconHashStatResult)
		for _, wsId := range wsIds {
			stats, err := l.svcCtx.GetAssetModel(wsId).AggregateIconHash(l.ctx, 1000)
			if err != nil {
				return nil, err
			}
			for _, stat := range stats {
				existing := merged[stat.IconHash]
				existing.IconHash = stat.IconHash
				existing.Count += stat.Count
				if len(existing.IconData) == 0 && len(stat.IconData) > 0 {
					existing.IconData = stat.IconData
				}
				merged[stat.IconHash] = existing
			}
		}

		results := make([]model.IconHashStatResult, 0, len(merged))
		for _, stat := range merged {
			results = append(results, stat)
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Count == results[j].Count {
				return results[i].IconHash < results[j].IconHash
			}
			return results[i].Count > results[j].Count
		})
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	if results, ok := cached.([]model.IconHashStatResult); ok {
		return results, nil
	}
	return nil, nil
}

// findIconAssetsBatch 批量查询多个 icon hash 对应的资产（替代 N+1 的 findIconAssets）
// 一次 $in 查询所有 hash，再按 hash 分组，每个 hash 最多保留 20 条最新资产
// 优化点：用 FindWithSort 走 AssetListProjection，排除 body/header/cert/banner 等大字段
// （只需要 host/iconHashFile/screenshot/createTime/updateTime/iconHashBytes）
func (l *IconListLogic) findIconAssetsBatch(workspaceId string, iconHashes []string) (map[string][]model.Asset, error) {
	if len(iconHashes) == 0 {
		return make(map[string][]model.Asset), nil
	}
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	result := make(map[string][]model.Asset, len(iconHashes))

	// 每个 ws 一次 $in 查询；用 sort + 限制总量避免拉过多
	// 单个 hash 最多需要 20 条，N 个 hash 最多 N*20 条
	limit := int64(len(iconHashes) * 20)
	filter := bson.M{"icon_hash": bson.M{"$in": iconHashes}}

	for _, wsId := range wsIds {
		// 用 FindWithOffset 走 AssetScreenshotProjection（保留 screenshot/icon_hash_bytes，排除 cert/banner）
		// 不用 FindWithSort（AssetListProjection 排除 screenshot/icon_hash_bytes，pickIconPresentation 需要这两个字段）
		assets, err := l.svcCtx.GetAssetModel(wsId).FindWithOffset(l.ctx, filter, 0, limit, "-update_time")
		if err != nil {
			return nil, err
		}
		for _, asset := range assets {
			if asset.IconHash == "" {
				continue
			}
			result[asset.IconHash] = append(result[asset.IconHash], asset)
		}
	}

	// 每个 hash 最多保留 20 条，并按 updateTime 降序
	for hash, assets := range result {
		sort.Slice(assets, func(i, j int) bool {
			return assets[i].UpdateTime.After(assets[j].UpdateTime)
		})
		if len(assets) > 20 {
			assets = assets[:20]
		}
		result[hash] = assets
	}
	return result, nil
}

func (l *IconListLogic) countNewIconAssets(workspaceId string) (int64, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var total int64
	for _, wsId := range wsIds {
		count, err := l.svcCtx.GetAssetModel(wsId).Count(l.ctx, bson.M{"icon_hash": bson.M{"$exists": true, "$ne": ""}, "new": true})
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}


func (l *IconListLogic) IconBatchDelete(req *types.IconBatchDeleteReq) (*types.BaseResp, error) {
	if len(req.Ids) == 0 {
		return &types.BaseResp{Code: 400, Msg: "请选择要删除的Icon"}, nil
	}

	deleted, err := l.deleteIconAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"icon_hash": bson.M{"$in": req.Ids}})
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *IconListLogic) IconClear() (*types.BaseResp, error) {
	deleted, err := l.deleteIconAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"icon_hash": bson.M{"$exists": true, "$ne": ""}})
	if err != nil {
		return nil, err
	}
	return &types.BaseResp{Code: 0, Msg: "成功清空 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *IconListLogic) deleteIconAssets(workspaceId string, filter bson.M) (int64, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var total int64
	for _, wsId := range wsIds {
		deleted, err := l.svcCtx.GetAssetModel(wsId).DeleteByFilter(l.ctx, filter)
		if err != nil {
			return 0, err
		}
		total += deleted
	}
	return total, nil
}
