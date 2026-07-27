package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 黑名单分组：扫描目标黑/白名单配置。
func init() {
	tag := "黑名单"
	tagDesc := "扫描目标黑名单配置"

	register(http.MethodPost, "/api/v1/blacklist/config/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取黑名单配置",
		Description: "返回当前工作空间的黑名单规则。",
		RespType:    "BlacklistConfigResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/blacklist/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存黑名单配置",
		Description: "更新当前工作空间的黑名单规则。",
		ReqType:     "BlacklistConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.BlacklistConfig{},
		types.BlacklistConfigResp{},
		types.BlacklistConfigSaveReq{},
		types.BlacklistRulesResp{},
	)
}
