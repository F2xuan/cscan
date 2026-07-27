package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 主题配置分组：登录前公共主题 + 用户级主题保存。
func init() {
	tag := "主题配置"
	tagDesc := "前端主题模式（亮 / 暗 / 跟随系统）与品牌色配置"

	register(http.MethodPost, "/api/v1/theme/config/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取主题配置",
		Description: "返回全局默认主题配置；登录前可调用（TierPublic）。",
		RespType:    "ThemeConfigResp",
		Security:    TierPublic,
	})
	register(http.MethodPost, "/api/v1/theme/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存主题配置",
		Description: "保存当前用户的主题偏好（mode / brandColor / layout 等）。",
		ReqType:     "ThemeConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.ThemeConfig{},
		types.ThemeConfigResp{},
		types.ThemeConfigSaveReq{},
	)
}
