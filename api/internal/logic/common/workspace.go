package common

import (
	"context"
	"time"

	"cscan/api/internal/svc"

	"go.mongodb.org/mongo-driver/bson"
)

// GetDefaultWorkspaceId 返回系统唯一工作空间标识。
// 系统已彻底移除多租户（workspace）概念，工作空间标识不再从请求头或数据库解析，
// 所有集合均为单一全局命名空间，因此这里直接返回固定值。
func GetDefaultWorkspaceId(ctx context.Context, svcCtx *svc.ServiceContext, workspaceId string) string {
	return "default"
}

// GetWorkspaceIds 获取工作空间ID列表。
// 单租户化改造：系统已移除"多工作空间"概念，所有数据归属于单一全局工作空间，
// 故恒返回固定标识。历史上此处会跨多个 {wsId}_* 集合做 $unionWith 合并，
// 现已无需此类跨空间聚合逻辑。
func GetWorkspaceIds(ctx context.Context, svcCtx *svc.ServiceContext, workspaceId string) []string {
	return []string{"default"}
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
