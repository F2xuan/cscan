package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 工作空间分组：工作空间 CRUD（多租户数据隔离的业务实体）。
func init() {
	tag := "工作空间"
	tagDesc := "工作空间 CRUD（多租户数据隔离单位）"

	register(http.MethodPost, "/api/v1/workspace/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "工作空间列表",
		Description: "返回当前用户可访问的工作空间列表。",
		RespType:    "WorkspaceListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/workspace/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存工作空间",
		Description: "新增或更新工作空间。id 为空时新增。",
		ReqType:     "WorkspaceSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/workspace/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除工作空间",
		Description: "按 id 删除一个工作空间。删除前会校验是否仍含资产。",
		ReqType:     "WorkspaceDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.Workspace{},
		types.WorkspaceListResp{},
		types.WorkspaceSaveReq{},
		types.WorkspaceDeleteReq{},
	)
}
