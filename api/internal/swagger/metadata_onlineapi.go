package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 在线搜索分组：FOFA / Hunter / Quake 资产聚合搜索、导入、API 凭证管理。
func init() {
	tag := "在线搜索"
	tagDesc := "FOFA / Hunter / Quake 资产搜索与导入、API 凭证管理"

	register(http.MethodPost, "/api/v1/onlineapi/search", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "在线资产搜索",
		Description: "聚合调用 FOFA / Hunter / Quake 等在线 API 搜索资产。",
		ReqType:     "OnlineSearchReq",
		RespType:    "OnlineSearchResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/onlineapi/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入搜索结果",
		Description: "将选定的搜索结果导入到当前工作空间的资产库。",
		ReqType:     "OnlineImportReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/onlineapi/importAll", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入全部搜索结果",
		Description: "将整页搜索结果批量导入到当前工作空间。",
		ReqType:     "OnlineImportAllReq",
		RespType:    "OnlineImportAllResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/onlineapi/config/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "API 凭证列表",
		Description: "返回所有在线搜索 API（FOFA / Hunter / Quake 等）的凭证配置。",
		RespType:    "APIConfigListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/onlineapi/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存 API 凭证",
		Description: "新增或更新在线搜索 API 的凭证（按 provider 区分）。",
		ReqType:     "APIConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/onlineapi/pull/status", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "自动拉取状态",
		Description: "返回各平台在线 API 自动拉取的启用状态、上次拉取时间/条数、配额余量与下次执行时间（T3.1）。",
		RespType:    "OnlinePullStatusResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.OnlineSearchReq{},
		types.OnlineSearchResult{},
		types.OnlineSearchResp{},
		types.OnlineImportReq{},
		types.OnlineImportAllReq{},
		types.OnlineImportAllResp{},
		types.APIConfig{},
		types.APIConfigListResp{},
		types.APIConfigSaveReq{},
		types.OnlinePullStatusResp{},
		types.OnlinePullStatusItem{},
	)
}
