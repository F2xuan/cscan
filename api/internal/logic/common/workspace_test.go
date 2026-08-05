package common

import (
	"context"
	"testing"

	"cscan/api/internal/svc"
)

// TestGetWorkspaceIds_SingleTenant 验证单租户化塌缩后 GetWorkspaceIds 恒返回固定的 "default"，
// 不再跨多个 {wsId}_* 集合做 fan-out（这也消除了原资产清单 $unionWith 全量内存排序瓶颈）。
func TestGetWorkspaceIds_SingleTenant(t *testing.T) {
	svcCtx := &svc.ServiceContext{}

	ids := GetWorkspaceIds(context.Background(), svcCtx, "explicit-ws")
	if len(ids) != 1 {
		t.Fatalf("单租户化后期望恰好 1 个工作空间 ID（无 fan-out），实际得到 %d: %v", len(ids), ids)
	}
	if ids[0] != "default" {
		t.Errorf("单租户化后期望固定返回 \"default\"，实际 %q", ids[0])
	}

	// 不同的输入参数也恒返回 "default"（单一全局命名空间）
	ids2 := GetWorkspaceIds(context.Background(), svcCtx, "another-ws")
	if len(ids2) != 1 || ids2[0] != "default" {
		t.Errorf("期望 [default]，实际 %v", ids2)
	}
}

// TestGetDefaultWorkspaceId_Explicit 验证单租户化后恒返回固定的 "default"，不再解析传入参数。
func TestGetDefaultWorkspaceId_Explicit(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	if got := GetDefaultWorkspaceId(context.Background(), svcCtx, "ws-123"); got != "default" {
		t.Errorf("单租户化后期望固定返回 \"default\"，实际 %q", got)
	}
}
