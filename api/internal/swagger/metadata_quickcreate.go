package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 一键扫描 + 智能模板推荐分组（T4.1）：输入目标即扫，后端智能识别类型并选扫描阶段。
func init() {
	tag := "TaskQuickCreate"
	tagDesc := "一键扫描 + 智能模板推荐（输入目标即扫，后端智能识别类型并选扫描阶段）"

	register(http.MethodPost, "/api/v1/task/quickCreate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "一键扫描创建任务",
		Description: "仅传目标与可选模式(quick/full)，后端自动识别目标类型（IP/CIDR→端口扫描，域名→全面扫描，URL→Web扫描）并智能选扫描阶段创建任务，返回任务ID、推荐类型与预估耗时。",
		ReqType:     "TaskQuickCreateReq",
		RespType:    "TaskQuickCreateResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.TaskQuickCreateReq{},
		types.TaskQuickCreateResp{},
	)
}
