package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 组织管理分组：组织 CRUD + 状态切换。
func init() {
	tag := "组织管理"
	tagDesc := "组织（租户隔离单位）的 CRUD 与启停"

	register(http.MethodPost, "/api/v1/organization/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "组织列表",
		Description: "返回所有组织。",
		RespType:    "OrganizationListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/organization/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存组织",
		Description: "新增或更新组织。id 为空时新增。",
		ReqType:     "OrganizationSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/organization/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除组织",
		Description: "按 id 删除一个组织。",
		ReqType:     "OrganizationDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/organization/updateStatus", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新组织状态",
		Description: "切换组织启用/禁用状态。",
		ReqType:     "OrganizationUpdateStatusReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.Organization{},
		types.OrganizationListResp{},
		types.OrganizationSaveReq{},
		types.OrganizationDeleteReq{},
		types.OrganizationUpdateStatusReq{},
	)
}
