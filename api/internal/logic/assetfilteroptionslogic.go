package logic

import (
	"context"
	"sort"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AssetFilterOptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetFilterOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetFilterOptionsLogic {
	return &AssetFilterOptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetFilterOptions 获取资产过滤器选项
// 优化点：
//  1. 用 LocalCache 缓存结果（60s TTL + singleflight 防击穿）
//  2. 用 DB 端 distinct 命令替代全表加载到内存 distinct
//  3. 仅取必要字段（app/port/http_status/labels），避免拉 body/header/screenshot 等大字段
func (l *AssetFilterOptionsLogic) AssetFilterOptions(req *types.AssetFilterOptionsReq, workspaceId string) (resp *types.AssetFilterOptionsResp, err error) {
	l.Logger.Infof("AssetFilterOptions查询: workspaceId=%s, domain=%s, hasScreenshot=%v", workspaceId, req.Domain, req.HasScreenshot)

	// 缓存键：按 workspaceId + domain + hasScreenshot 维度隔离
	cacheKey := "asset_filter_opts:" + workspaceId + ":" + req.Domain + ":" + boolToStr(req.HasScreenshot)

	cached, err := l.svcCtx.QueryCache.GetOrSetWithTTL(cacheKey, 60*time.Second, func() (interface{}, error) {
		return l.loadFilterOptions(workspaceId, req)
	})
	if err != nil {
		l.Logger.Errorf("AssetFilterOptions查询失败: %v", err)
		return &types.AssetFilterOptionsResp{Code: 500, Msg: "查询失败"}, nil
	}

	result, ok := cached.(*types.AssetFilterOptionsResp)
	if !ok {
		return &types.AssetFilterOptionsResp{Code: 500, Msg: "查询失败"}, nil
	}
	return result, nil
}

func (l *AssetFilterOptionsLogic) loadFilterOptions(workspaceId string, req *types.AssetFilterOptionsReq) (*types.AssetFilterOptionsResp, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	// 构建查询条件
	filter := bson.M{}
	if req.Domain != "" {
		filter["host"] = bson.M{"$regex": req.Domain, "$options": "i"}
	}
	if req.HasScreenshot {
		filter["screenshot"] = bson.M{"$ne": ""}
	}

	techSet := make(map[string]struct{})
	portSet := make(map[int]struct{})
	statusSet := make(map[string]struct{})
	labelSet := make(map[string]struct{})

	// 用 DB 端 distinct 替代全表加载，避免把 body/header/screenshot 等大字段拉到内存
	for _, wsId := range wsIds {
		assetModel := l.svcCtx.GetAssetModel(wsId)

		if values, err := assetModel.Distinct(l.ctx, "app", filter); err == nil {
			for _, v := range values {
				if s, ok := v.(string); ok && s != "" {
					techSet[s] = struct{}{}
				}
			}
		} else {
			l.Logger.Errorf("查询工作空间 %s app distinct 失败: %v", wsId, err)
		}

		if values, err := assetModel.Distinct(l.ctx, "port", filter); err == nil {
			for _, v := range values {
				if i, ok := v.(int32); ok && i > 0 {
					portSet[int(i)] = struct{}{}
				} else if i, ok := v.(int64); ok && i > 0 {
					portSet[int(i)] = struct{}{}
				} else if i, ok := v.(int); ok && i > 0 {
					portSet[i] = struct{}{}
				}
			}
		} else {
			l.Logger.Errorf("查询工作空间 %s port distinct 失败: %v", wsId, err)
		}

		if values, err := assetModel.Distinct(l.ctx, "status", filter); err == nil {
			for _, v := range values {
				if s, ok := v.(string); ok && s != "" {
					statusSet[s] = struct{}{}
				}
			}
		}

		if values, err := assetModel.Distinct(l.ctx, "labels", filter); err == nil {
			for _, v := range values {
				switch val := v.(type) {
				case string:
					if val != "" {
						labelSet[val] = struct{}{}
					}
				case primitive.A:
					for _, item := range val {
						if s, ok := item.(string); ok && s != "" {
							labelSet[s] = struct{}{}
						}
					}
				}
			}
		} else {
			l.Logger.Errorf("查询工作空间 %s labels distinct 失败: %v", wsId, err)
		}
	}

	technologies := make([]string, 0, len(techSet))
	for tech := range techSet {
		technologies = append(technologies, tech)
	}
	sort.Strings(technologies)

	ports := make([]int, 0, len(portSet))
	for port := range portSet {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	statusCodes := make([]string, 0, len(statusSet))
	for status := range statusSet {
		statusCodes = append(statusCodes, status)
	}
	sort.Strings(statusCodes)

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	return &types.AssetFilterOptionsResp{
		Code:         0,
		Msg:          "success",
		Technologies: technologies,
		Ports:        ports,
		StatusCodes:  statusCodes,
		Labels:       labels,
	}, nil
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
