package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 指纹管理分组：指纹 CRUD、分类、同步、导入、批量验证、匹配资产、HTTP 服务映射、主动指纹。
func init() {
	tag := "指纹管理"
	tagDesc := "指纹规则管理、HTTP 服务映射、主动指纹探测"

	// ===== 指纹 CRUD 与同步 =====
	register(http.MethodPost, "/api/v1/fingerprint/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "指纹列表",
		Description: "分页返回指纹规则，可按 keyword / service / enabled 过滤。",
		ReqType:     "FingerprintListReq",
		RespType:    "FingerprintListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存指纹",
		Description: "新增或更新指纹规则。id 为空时新增。",
		ReqType:     "FingerprintSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除指纹",
		Description: "按 id 删除一条指纹规则。内置指纹不可删除。",
		ReqType:     "FingerprintDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/categories", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "指纹分类统计",
		Description: "返回指纹按 service / classification 的分类计数。",
		RespType:    "FingerprintCategoriesResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/fingerprint/sync", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "同步指纹库",
		Description: "从远程源同步内置指纹库。",
		ReqType:     "FingerprintSyncReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/updateEnabled", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新启用状态",
		Description: "更新单条指纹的 enabled 状态。Body：{id, enabled}。",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/batchUpdateEnabled", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量更新启用状态",
		Description: "批量更新多条指纹的 enabled 状态。Body：{ids, enabled, all}。",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入指纹",
		Description: "批量导入指纹 JSON 内容。",
		ReqType:     "FingerprintImportReq",
		RespType:    "FingerprintImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/importFromFile", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "从文件导入指纹",
		Description: "上传文件并导入指纹规则。",
		ReqType:     "FingerprintImportFromFileReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/clearCustom", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空自定义指纹",
		Description: "清空所有自定义指纹，返回删除数量。内置指纹保留。",
		ReqType:     "FingerprintClearCustomReq",
		RespType:    "FingerprintClearCustomResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/fingerprint/validate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "验证指纹",
		Description: "对单条指纹规则进行语法验证。",
		ReqType:     "FingerprintValidateReq",
		RespType:    "FingerprintValidateResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/batchValidate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量验证指纹",
		Description: "批量验证多条指纹规则的语法。",
		ReqType:     "FingerprintBatchValidateReq",
		RespType:    "FingerprintBatchValidateResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/matchAssets", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "指纹匹配资产",
		Description: "对当前工作空间的资产重跑指纹匹配。",
		ReqType:     "FingerprintMatchAssetsReq",
		RespType:    "FingerprintMatchAssetsResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// ===== HTTP 服务映射 =====
	register(http.MethodPost, "/api/v1/fingerprint/httpservice/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "HTTP 服务映射列表",
		Description: "返回 HTTP 服务名到指纹的映射列表。",
		ReqType:     "HttpServiceMappingListReq",
		RespType:    "HttpServiceMappingListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/fingerprint/httpservice/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存 HTTP 服务映射",
		Description: "新增或更新 HTTP 服务映射。id 为空时新增。",
		ReqType:     "HttpServiceMappingSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/httpservice/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除 HTTP 服务映射",
		Description: "按 id 删除一条 HTTP 服务映射。",
		ReqType:     "HttpServiceMappingDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// ===== 主动指纹 =====
	register(http.MethodPost, "/api/v1/fingerprint/active/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "主动指纹列表",
		Description: "分页返回主动指纹规则，可按 service / enabled 过滤。",
		ReqType:     "ActiveFingerprintListReq",
		RespType:    "ActiveFingerprintListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/active/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存主动指纹",
		Description: "新增或更新主动指纹规则。id 为空时新增。",
		ReqType:     "ActiveFingerprintSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/active/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除主动指纹",
		Description: "按 id 删除一条主动指纹规则。",
		ReqType:     "ActiveFingerprintDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/active/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入主动指纹",
		Description: "批量导入主动指纹 JSON。",
		ReqType:     "ActiveFingerprintImportReq",
		RespType:    "ActiveFingerprintImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/fingerprint/active/export", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导出主动指纹",
		Description: "按条件导出主动指纹。Body：{ids?, enabled?, service?}。",
		RespType:    "ActiveFingerprintExportResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/fingerprint/active/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空主动指纹",
		Description: "清空所有自定义主动指纹，返回删除数量。",
		RespType:    "ActiveFingerprintClearResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/fingerprint/active/validate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "验证主动指纹",
		Description: "对主动指纹规则进行语法验证。",
		ReqType:     "ActiveFingerprintValidateReq",
		RespType:    "ActiveFingerprintValidateResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.Fingerprint{},
		types.FingerprintListReq{},
		types.FingerprintListResp{},
		types.FingerprintSaveReq{},
		types.FingerprintDeleteReq{},
		types.FingerprintCategoriesResp{},
		types.FingerprintSyncReq{},
		types.FingerprintImportReq{},
		types.FingerprintImportResp{},
		types.FingerprintImportFromFileReq{},
		types.FingerprintClearCustomReq{},
		types.FingerprintClearCustomResp{},
		types.FingerprintValidateReq{},
		types.FingerprintValidateResp{},
		types.FingerprintBatchValidateReq{},
		types.FingerprintBatchValidateResp{},
		types.FingerprintMatchAssetsReq{},
		types.FingerprintMatchAssetsResp{},
		types.FingerprintMatchedAsset{},
		types.HttpServiceMapping{},
		types.HttpServiceMappingListReq{},
		types.HttpServiceMappingListResp{},
		types.HttpServiceMappingSaveReq{},
		types.HttpServiceMappingDeleteReq{},
		types.ActiveFingerprint{},
		types.ActiveFingerprintListReq{},
		types.ActiveFingerprintListResp{},
		types.ActiveFingerprintSaveReq{},
		types.ActiveFingerprintDeleteReq{},
		types.ActiveFingerprintImportReq{},
		types.ActiveFingerprintImportResp{},
		types.ActiveFingerprintExportResp{},
		types.ActiveFingerprintClearResp{},
		types.ActiveFingerprintValidateReq{},
		types.ActiveFingerprintValidateResp{},
		types.ActiveFingerprintValidateItem{},
	)
}
