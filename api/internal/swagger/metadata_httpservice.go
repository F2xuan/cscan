package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// HTTP 服务映射（V2）分组：独立端口到 HTTP 服务名的映射接口、配置与导入导出。
// V1 接口（/api/v1/fingerprint/httpservice/*）已包含在 metadata_fingerprint.go。
func init() {
	tag := "HTTP 服务映射"
	tagDesc := "HTTP 服务名映射、端口配置与导入导出"

	register(http.MethodPost, "/api/v1/httpservice/config", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存 HTTP 服务配置",
		Description: "更新 HTTP 服务相关全局配置（如默认端口、超时等）。",
		ReqType:     "HttpServiceConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/httpservice/mapping/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "HTTP 服务映射列表（V2）",
		Description: "返回 HTTP 服务名到指纹的映射列表（使用新模型）。",
		ReqType:     "HttpServiceMappingListReq",
		RespType:    "HttpServiceMappingListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/httpservice/mapping/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存 HTTP 服务映射（V2）",
		Description: "新增或更新 HTTP 服务映射（使用新模型）。id 为空时新增。",
		ReqType:     "HttpServiceMappingSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/httpservice/mapping/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除 HTTP 服务映射（V2）",
		Description: "按 id 删除一条 HTTP 服务映射（使用新模型）。",
		ReqType:     "HttpServiceMappingDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/httpservice/export", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导出 HTTP 服务映射",
		Description: "导出 HTTP 服务映射配置（含端口 / 服务名）。",
		ReqType:     "HttpServiceExportReq",
		RespType:    "HttpServiceExportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/httpservice/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入 HTTP 服务映射",
		Description: "批量导入 HTTP 服务映射文本。",
		ReqType:     "HttpServiceImportReq",
		RespType:    "HttpServiceImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// GET /api/v1/httpservice/config 走 http.MethodGet，spec.go 不会注入 POST body
	register(http.MethodGet, "/api/v1/httpservice/config", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取 HTTP 服务配置",
		Description: "返回 HTTP 服务相关全局配置。",
		RespType:    "HttpServiceConfigGetResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.HttpServiceConfig{},
		types.HttpServiceConfigGetResp{},
		types.HttpServiceConfigSaveReq{},
		types.HttpServiceMapping{},
		types.HttpServiceMappingListReq{},
		types.HttpServiceMappingListResp{},
		types.HttpServiceMappingSaveReq{},
		types.HttpServiceMappingDeleteReq{},
		types.HttpServiceExportReq{},
		types.HttpServiceExportResp{},
		types.HttpServiceImportReq{},
		types.HttpServiceImportResp{},
	)
}
