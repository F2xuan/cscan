package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 子域名字典分组：子域名爆破字典的 CRUD 与启用列表。
func init() {
	tag := "子域名字典"
	tagDesc := "子域名爆破字典管理（内置 + 自定义）"

	register(http.MethodPost, "/api/v1/subdomain/dict/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "子域名字典列表",
		Description: "分页返回子域名字典，可按 `enabled` 过滤。\n\n**默认值**：page=1，pageSize=20。",
		ReqType:     "SubdomainDictListReq",
		RespType:    "SubdomainDictListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/subdomain/dict/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存子域名字典",
		Description: "新增或更新子域名字典。`id` 为空时新增；非空时更新。\n\n**字段说明**\n\n- `name`：字典名称。\n- `content`：字典内容（每行一个子域）。\n- `enabled`：是否启用。",
		ReqType:     "SubdomainDictSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/subdomain/dict/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除子域名字典",
		Description: "按 `id` 删除一个子域名字典。内置字典不可删除。",
		ReqType:     "SubdomainDictDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/subdomain/dict/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空子域名字典",
		Description: "清空所有自定义子域名字典，返回删除数量。内置字典保留。",
		RespType:    "SubdomainDictClearResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/subdomain/dict/enabled", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "启用的子域名字典列表",
		Description: "返回当前启用的子域名字典简化列表（仅 `id / name / wordCount / isBuiltin`），供任务创建时选择。",
		RespType:    "SubdomainDictEnabledListResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.SubdomainDict{},
		types.SubdomainDictListReq{},
		types.SubdomainDictListResp{},
		types.SubdomainDictSaveReq{},
		types.SubdomainDictDeleteReq{},
		types.SubdomainDictClearResp{},
		types.SubdomainDictEnabledListResp{},
		types.SubdomainDictSimple{},
	)
}
