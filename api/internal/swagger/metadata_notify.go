package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 通知配置分组：通知渠道配置 CRUD + 测试 + 提供者列表 + 高危过滤器配置。
func init() {
	tag := "通知配置"
	tagDesc := "扫描结果通知渠道与高危过滤器配置"

	register(http.MethodPost, "/api/v1/notify/config/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "通知配置列表",
		Description: "返回所有通知渠道配置。",
		RespType:    "NotifyConfigListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/notify/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存通知配置",
		Description: "新增或更新通知渠道配置。id 为空时新增。",
		ReqType:     "NotifyConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/notify/config/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除通知配置",
		Description: "按 id 删除一条通知渠道配置。",
		ReqType:     "NotifyConfigDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/notify/config/test", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "测试通知配置",
		Description: "向目标渠道发送一条测试通知。返回 BaseResp。",
		ReqType:     "NotifyConfigTestReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/notify/providers", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "通知提供者列表",
		Description: "返回所有支持的通知提供者及其字段定义。",
		RespType:    "NotifyProviderListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/notify/highrisk/config/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取高危过滤器配置",
		Description: "返回当前工作空间的高危漏洞通知过滤配置。",
		RespType:    "HighRiskFilterConfigResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/notify/highrisk/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存高危过滤器配置",
		Description: "更新高危漏洞通知过滤配置。",
		ReqType:     "HighRiskFilterConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.NotifyConfig{},
		types.NotifyConfigListResp{},
		types.NotifyConfigSaveReq{},
		types.NotifyConfigDeleteReq{},
		types.NotifyConfigTestReq{},
		types.NotifyProvider{},
		types.NotifyConfigField{},
		types.NotifyProviderListResp{},
		types.HighRiskFilter{},
		types.HighRiskFilterConfig{},
		types.HighRiskFilterConfigResp{},
		types.HighRiskFilterConfigSaveReq{},
	)
}
