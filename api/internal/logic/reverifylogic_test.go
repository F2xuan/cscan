package logic

import (
	"context"
	"testing"

	"cscan/api/internal/types"
)

// TestReverifyConfigGet_DefaultWorkspace 验证单租户化后复验配置在 workspaceId 为空时
// 仍能正确回退到默认工作空间并返回配置（前端不再下发 X-Workspace-Id）。
func TestReverifyConfigGet_DefaultWorkspace(t *testing.T) {
	svcCtx, cleanup := newTestSvcCtxDB(t)
	defer cleanup()
	ctx := context.Background()

	l := NewReverifyConfigGetLogic(ctx, svcCtx)
	// 空 workspaceId：应回退默认工作空间并返回默认值（无报错）
	resp, err := l.ReverifyConfigGet(&types.ReverifyConfigGetReq{})
	if err != nil {
		t.Fatalf("ReverifyConfigGet 失败: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("期望 Code=0，实际 %d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Config == nil {
		t.Fatal("期望返回默认配置，实际 nil")
	}
	if resp.Config.WorkspaceId == "" {
		t.Errorf("配置应绑定到回退后的工作空间 ID，实际为空")
	}

	// Save 后再次 Get 应读到保存值（验证以 workspaceId 为键的一致性）
	saveL := NewReverifyConfigSaveLogic(ctx, svcCtx)
	saveResp, err := saveL.ReverifyConfigSave(&types.ReverifyConfigSaveReq{
		WorkspaceId:      resp.Config.WorkspaceId,
		WeakPassEnabled:  true,
		ExposureEnabled:  true,
		CronSpec:         "0 0 4 * * *",
		MaxTargetsPerRun: 50,
		Concurrency:      2,
	})
	if err != nil {
		t.Fatalf("ReverifyConfigSave 失败: %v", err)
	}
	if saveResp.Code != 0 {
		t.Fatalf("保存期望 Code=0，实际 %d", saveResp.Code)
	}

	getL := NewReverifyConfigGetLogic(ctx, svcCtx)
	getResp, err := getL.ReverifyConfigGet(&types.ReverifyConfigGetReq{WorkspaceId: resp.Config.WorkspaceId})
	if err != nil {
		t.Fatalf("再次 Get 失败: %v", err)
	}
	if getResp.Config.WeakPassEnabled != true || getResp.Config.ExposureEnabled != true {
		t.Errorf("保存后读取的配置未生效: %+v", getResp.Config)
	}
}
