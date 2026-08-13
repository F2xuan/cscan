package common

import (
	"context"
	"time"

	"cscan/api/internal/svc"

	"go.mongodb.org/mongo-driver/bson"
)

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
