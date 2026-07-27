package common

import (
	"context"
	"time"

	"cscan/api/internal/svc"

	"go.mongodb.org/mongo-driver/bson"
)

// GetDefaultWorkspaceId 当 workspaceId 为空或 "all" 时，解析为第一个真实工作空间 ID
// 用于写入操作（如导入资产），确保数据写入真实的工作空间集合而非 "all_xxx"
func GetDefaultWorkspaceId(ctx context.Context, svcCtx *svc.ServiceContext, workspaceId string) string {
	if workspaceId != "" && workspaceId != "all" {
		return workspaceId
	}

	// 查询第一个工作空间
	workspaces, err := svcCtx.WorkspaceModel.Find(ctx, bson.M{}, 1, 1)
	if err != nil || len(workspaces) == 0 {
		return "default"
	}
	return workspaces[0].Id.Hex()
}

// GetWorkspaceIds 获取工作空间ID列表
// 当 workspaceId 为空或 "all" 时，返回所有工作空间ID（包括默认空间）
// 对 "all"/空 场景做 60s 缓存（带 singleflight 防击穿），单 workspaceId 不缓存
func GetWorkspaceIds(ctx context.Context, svcCtx *svc.ServiceContext, workspaceId string) []string {
	// 处理 "all" 值 - 前端传递 "all" 表示查询所有工作空间
	if workspaceId != "" && workspaceId != "all" {
		return []string{workspaceId}
	}

	// "all"/空 场景走缓存
	cached, err := svcCtx.QueryCache.GetOrSetWithTTL("workspace_ids:all", 60*time.Second, func() (interface{}, error) {
		return loadAllWorkspaceIds(ctx, svcCtx), nil
	})
	if err == nil {
		if ids, ok := cached.([]string); ok {
			return ids
		}
	}
	return loadAllWorkspaceIds(ctx, svcCtx)
}

func loadAllWorkspaceIds(ctx context.Context, svcCtx *svc.ServiceContext) []string {
	var ids []string

	// 查询所有工作空间（不分页）
	workspaces, err := svcCtx.WorkspaceModel.Find(ctx, bson.M{}, 0, 0)
	if err != nil {
		// 如果查询失败，至少返回默认空间
		return []string{"default"}
	}

	// 添加所有存在的工作空间
	for _, ws := range workspaces {
		ids = append(ids, ws.Id.Hex())
	}

	// 如果没有找到任何工作空间，添加默认空间
	if len(ids) == 0 {
		ids = append(ids, "default")
	} else {
		// 确保默认空间在列表中（如果存在的话）
		hasDefault := false
		for _, id := range ids {
			if id == "default" {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			// 检查默认空间是否真的存在数据
			defaultAssetModel := svcCtx.GetAssetModel("default")
			if count, err := defaultAssetModel.Count(ctx, bson.M{}); err == nil && count > 0 {
				ids = append(ids, "default")
			}
		}
	}

	return ids
}

// LoadOrgMap 加载组织ID到名称的映射（带 60s 缓存）
// 写入组织（创建/更新/删除）后应调用 InvalidateOrgMap 主动失效
func LoadOrgMap(ctx context.Context, svcCtx *svc.ServiceContext) map[string]string {
	cached, err := svcCtx.QueryCache.GetOrSetWithTTL("org_map", 60*time.Second, func() (interface{}, error) {
		orgMap := make(map[string]string)
		orgs, err := svcCtx.OrganizationModel.Find(ctx, bson.M{}, 0, 0)
		if err != nil {
			return orgMap, nil
		}
		for _, org := range orgs {
			orgMap[org.Id.Hex()] = org.Name
		}
		return orgMap, nil
	})
	if err != nil {
		return make(map[string]string)
	}
	if m, ok := cached.(map[string]string); ok {
		return m
	}
	return make(map[string]string)
}

// InvalidateOrgMap 主动失效组织映射缓存（组织 CRUD 后调用）
func InvalidateOrgMap(svcCtx *svc.ServiceContext) {
	svcCtx.QueryCache.Delete("org_map")
}

// InvalidateWorkspaceIds 主动失效工作空间列表缓存（工作空间 CRUD 后调用）
func InvalidateWorkspaceIds(svcCtx *svc.ServiceContext) {
	svcCtx.QueryCache.Delete("workspace_ids:all")
}
