package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 弱口令 / 敏感信息持续复验分组（T3.3 / T3.4）：复验配置 + 立即触发。
func init() {
	tag := "Reverify"
	tagDesc := "弱口令与敏感信息持续复验（周期巡检 / 立即复验）"

	// 复验配置获取
	register(http.MethodPost, "/api/v1/vul/reverify/config/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取持续复验配置",
		Description: "返回当前工作空间的弱口令/敏感信息持续复验配置（无配置时返回默认值，weakPassEnabled=false）。",
		ReqType:     "ReverifyConfigGetReq",
		RespType:    "ReverifyConfigGetResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	// 复验配置保存
	register(http.MethodPost, "/api/v1/vul/reverify/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存持续复验配置",
		Description: "保存当前工作空间的复验配置（弱口令/敏感信息复验开关、周期、单批上限、并发度）。空值回退默认值。",
		ReqType:     "ReverifyConfigSaveReq",
		RespType:    "ReverifyConfigSaveResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	// 立即复验
	register(http.MethodPost, "/api/v1/vul/reverify/runNow", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "立即触发弱口令复验",
		Description: "对单个工作空间立即执行一次弱口令持续复验（验证已知凭据是否仍存在，仅复验不爆破）。",
		ReqType:     "ReverifyRunNowReq",
		RespType:    "ReverifyRunNowResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.ReverifyConfig{},
		types.ReverifyConfigGetReq{},
		types.ReverifyConfigGetResp{},
		types.ReverifyConfigSaveReq{},
		types.ReverifyConfigSaveResp{},
		types.ReverifyRunNowReq{},
		types.ReverifyRunNowResp{},
	)
}
