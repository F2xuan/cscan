package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 漏洞管理分组：漏洞列表、详情、删除、清空、统计。
// 全部接口为 JWT 鉴权（TierAuth），按 X-Workspace-Id 隔离。
func init() {
	tag := "漏洞管理"
	tagDesc := "扫描漏洞的列表、详情、删除与统计"

	register(http.MethodPost, "/api/v1/vul/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "漏洞列表",
		Description: "分页返回漏洞列表，可按 `query / authority / severity / source / host / port` 过滤。\n\n**默认值**：page=1，pageSize=20。\n\n**典型错误码**\n\n- 400 参数错误\n- 500 服务器错误",
		ReqType:     "VulListReq",
		RespType:    "VulListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/vul/detail", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "漏洞详情",
		Description: "按 `id` 返回漏洞完整数据，含证据、Payload、扫描时间线。",
		ReqType:     "VulDetailReq",
		RespType:    "VulDetailResp",
		Security:    TierAuth,
		Errors:      []int{400, 10401, 500},
	})

	register(http.MethodPost, "/api/v1/vul/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除漏洞",
		Description: "按 `id` 删除单个漏洞。",
		ReqType:     "VulDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 10401, 500},
	})

	register(http.MethodPost, "/api/v1/vul/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除漏洞",
		Description: "按 `ids` 数组批量删除漏洞，`ids` 至少包含一项。",
		ReqType:     "VulBatchDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 10401, 500},
	})

	register(http.MethodPost, "/api/v1/vul/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空当前工作空间漏洞",
		Description: "清空当前工作空间下的全部漏洞记录，不可恢复。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/vul/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "漏洞统计",
		Description: "返回当前工作空间漏洞按严重等级（Critical/High/Medium/Low/Info）的分布与近 7 / 30 天增长，以及生命周期状态计数（open/fixed/ignored）。",
		RespType:    "VulStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/vul/updateStatus", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量更新漏洞状态",
		Description: "批量将漏洞标记为 open / fixed / ignored（T1.3 漏洞生命周期状态机）。返回实际更新的条数。",
		ReqType:     "VulUpdateStatusReq",
		RespType:    "VulUpdateStatusResp",
		Security:    TierAuth,
		Errors:      []int{400, 10401, 500},
	})

	RegisterTypes(
		types.VulListReq{},
		types.VulListResp{},
		types.Vul{},
		types.VulDetailReq{},
		types.VulDetailResp{},
		types.VulDetail{},
		types.VulDeleteReq{},
		types.VulBatchDeleteReq{},
		types.VulStatResp{},
		types.VulUpdateStatusReq{},
		types.VulUpdateStatusResp{},
	)
}
