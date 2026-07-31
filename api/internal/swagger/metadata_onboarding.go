package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 引导式首次体验分组（T4.2）：查询/完成首次引导，标记持久化到用户档案。
func init() {
	tag := "UserOnboarding"
	tagDesc := "引导式首次体验（查询/完成首次引导，标记持久化到用户档案）"

	register(http.MethodPost, "/api/v1/user/onboarding/status", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "查询首次引导状态",
		Description: "返回当前登录用户首次引导是否已完成。已完成条件：用户已显式完成/跳过引导，或当前工作空间已存在扫描任务（老用户不重复弹窗）。",
		ReqType:     "",
		RespType:    "UserOnboardingStatusResp",
		Security:    TierAuth,
		Errors:      []int{401, 404, 500},
	})

	register(http.MethodPost, "/api/v1/user/onboarding/complete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "完成首次引导",
		Description: "标记当前登录用户首次引导已完成（完成扫描或点击跳过均调用此接口），后续登录不再弹出引导。",
		ReqType:     "",
		RespType:    "BaseResp",
		Security:    TierAuth,
		Errors:      []int{401, 500},
	})

	RegisterTypes(
		types.UserOnboardingStatusResp{},
	)
}
