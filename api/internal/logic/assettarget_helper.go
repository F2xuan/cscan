package logic

import (
	"context"

	"cscan/api/internal/svc"
)

// invalidateAssetTargetCaches 失效 list + detail 缓存。
// LocalCache 不支持前缀扫删，Phase 2 简化为全清（list/detail 都是 30s TTL，影响可接受）。
// Phase 4 若引入反向索引再改为精确删除。
func invalidateAssetTargetCaches(svcCtx *svc.ServiceContext, targetId string) {
	svcCtx.QueryCache.Clear()
	_ = targetId
}

// locateOwningWsMeta 遍历 wsIds 找到 targetId 所在 workspace，返回 owning wsId；未找到返回空串。
// 供 update / delete logic 复用；context 走调用方传入以避免绑定特定 logic 结构。
func locateOwningWsMeta(ctx context.Context, svcCtx *svc.ServiceContext, wsIds []string, targetId string) string {
	for _, wsId := range wsIds {
		m, err := svcCtx.GetAssetTargetMetaModel(wsId).FindByID(ctx, targetId)
		if err != nil {
			continue
		}
		if m != nil {
			return wsId
		}
	}
	return ""
}
