package common

import (
	"context"
	"testing"

	"cscan/api/internal/svc"
)

// TestGetWorkspaceIds_SingleTenant 验证单租户化塌缩后 GetWorkspaceIds 恒返回唯一工作空间 ID，
// 不再跨多个 {wsId}_* 集合做 fan-out（这也消除了原资产清单 $unionWith 全量内存排序瓶颈）。
func TestGetWorkspaceIds_SingleTenant(t *testing.T) {
	svcCtx := &svc.ServiceContext{}

	ids := GetWorkspaceIds(context.Background(), svcCtx, "explicit-ws")
	if len(ids) != 1 {
		t.Fatalf("单租户化后期望恰好 1 个工作空间 ID（无 fan-out），实际得到 %d: %v", len(ids), ids)
	}
	if ids[0] != "explicit-ws" {
		t.Errorf("期望解析结果为显式传入的 workspaceId，实际 %q", ids[0])
	}

	// 不同的显式 ID 也应各自独立返回（调用方 for-range 仍只执行一次）
	ids2 := GetWorkspaceIds(context.Background(), svcCtx, "another-ws")
	if len(ids2) != 1 || ids2[0] != "another-ws" {
		t.Errorf("期望 [another-ws]，实际 %v", ids2)
	}
}

// TestGetDefaultWorkspaceId_Explicit 验证显式（非 all/非空）workspaceId 直接透传，不触发数据库查询。
// 注意："all" 与 "" 会回退到首个真实工作空间（需 DB），由 workspace_db_test.go 覆盖。
func TestGetDefaultWorkspaceId_Explicit(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	if got := GetDefaultWorkspaceId(context.Background(), svcCtx, "ws-123"); got != "ws-123" {
		t.Errorf("显式 workspaceId 应原样返回，实际 %q", got)
	}
}

// TestInvalidateWorkspaceIds_Noop 单租户化后已无多工作空间缓存，该函数应为兼容空函数，绝不 panic。
func TestInvalidateWorkspaceIds_Noop(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	InvalidateWorkspaceIds(svcCtx) // 必须不 panic
}
