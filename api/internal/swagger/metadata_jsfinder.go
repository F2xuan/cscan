package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// JSFinder 分组：JS 资产发现配置 + 结果查询/清空。
func init() {
	tag := "JSFinder"
	tagDesc := "JavaScript 资产发现配置与结果管理"

	register(http.MethodPost, "/api/v1/jsfinder/config/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取 JSFinder 配置",
		Description: "返回当前 JSFinder 配置（含内置默认值）。",
		RespType:    "JSFinderConfigResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/jsfinder/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存 JSFinder 配置",
		Description: "更新 JSFinder 配置（覆盖式）。",
		ReqType:     "JSFinderConfigSaveReq",
		RespType:    "JSFinderConfigResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/jsfinder/config/reset", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "重置 JSFinder 配置",
		Description: "将 JSFinder 配置重置为内置默认值。",
		RespType:    "JSFinderConfigResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/jsfinder/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "JSFinder 结果列表",
		Description: "分页返回 JSFinder 提取到的 JS / 接口 / 敏感信息结果。",
		ReqType:     "JSFinderListReq",
		RespType:    "JSFinderListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/jsfinder/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空 JSFinder 结果",
		Description: "清空当前工作空间全部 JSFinder 结果。",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.JSFinderConfig{},
		types.JSFinderConfigResp{},
		types.JSFinderConfigSaveReq{},
		types.JSFinderResult{},
		types.JSFinderListReq{},
		types.JSFinderListResp{},
	)
}
