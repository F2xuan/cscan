package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// POC 管理分组：标签映射、自定义 POC、Nuclei 模板、POC 批量验证。
func init() {
	tag := "POC 管理"
	tagDesc := "POC 标签映射、自定义 POC、Nuclei 模板同步与批量验证"

	// ===== 标签映射 =====
	register(http.MethodPost, "/api/v1/poc/tagmapping/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "标签映射列表",
		Description: "返回 POC 标签到分类的映射列表。",
		RespType:    "TagMappingListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/poc/tagmapping/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存标签映射",
		Description: "新增或更新 POC 标签映射。",
		ReqType:     "TagMappingSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/tagmapping/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除标签映射",
		Description: "按 id 删除一个标签映射。",
		ReqType:     "TagMappingDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// ===== 自定义 POC =====
	register(http.MethodPost, "/api/v1/poc/custom/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "自定义 POC 列表",
		Description: "分页返回自定义 POC 列表，可按 name / tag / type 过滤。",
		ReqType:     "CustomPocListReq",
		RespType:    "CustomPocListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/custom/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存自定义 POC",
		Description: "新增或更新自定义 POC。id 为空时新增。",
		ReqType:     "CustomPocSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/custom/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除自定义 POC",
		Description: "按 id 删除一个自定义 POC。",
		ReqType:     "CustomPocDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/custom/batchImport", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量导入自定义 POC",
		Description: "批量导入自定义 POC 文件内容（多组）。",
		ReqType:     "CustomPocBatchImportReq",
		RespType:    "CustomPocBatchImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/custom/clearAll", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空自定义 POC",
		Description: "清空所有自定义 POC，返回删除数量。可按 name / severity / tag / templateId 过滤清空范围。",
		ReqType:     "CustomPocClearAllReq",
		RespType:    "CustomPocClearAllResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/poc/custom/scanAssets", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "使用自定义 POC 扫描资产",
		Description: "对选定资产批量执行自定义 POC 验证。",
		ReqType:     "CustomPocScanAssetsReq",
		RespType:    "CustomPocScanAssetsResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/custom/validate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "验证自定义 POC",
		Description: "对单个自定义 POC 进行语法与匹配验证。",
		ReqType:     "PocValidateReq",
		RespType:    "PocValidateResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/custom/validateSyntax", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "验证 POC 语法",
		Description: "对 POC 文本进行纯语法校验（不执行扫描）。",
		ReqType:     "ValidatePocSyntaxReq",
		RespType:    "ValidatePocSyntaxResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// ===== Nuclei 模板 =====
	register(http.MethodPost, "/api/v1/poc/nuclei/templates", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "Nuclei 模板列表",
		Description: "分页返回 Nuclei 模板，可按 tag / severity / enabled 过滤。",
		ReqType:     "NucleiTemplateListReq",
		RespType:    "NucleiTemplateListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/categories", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "Nuclei 模板分类",
		Description: "返回所有 Nuclei 模板的分类（tag）统计。",
		RespType:    "NucleiTemplateCategoriesResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/sync", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "同步 Nuclei 模板",
		Description: "从官方或自定义源同步 Nuclei 模板。",
		ReqType:     "NucleiTemplateSyncReq",
		RespType:    "NucleiTemplateSyncResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/download", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "下载 Nuclei 模板",
		Description: "触发 Nuclei 模板下载任务。",
		ReqType:     "NucleiTemplateDownloadReq",
		RespType:    "NucleiTemplateDownloadResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/download/status", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "Nuclei 模板下载状态",
		Description: "查询 Nuclei 模板下载任务状态。可按 taskId 过滤。",
		ReqType:     "NucleiTemplateDownloadStatusReq",
		RespType:    "NucleiTemplateDownloadStatusResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空 Nuclei 模板",
		Description: "清空所有已下载的 Nuclei 模板。",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/updateEnabled", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新启用状态",
		Description: "更新 Nuclei 模板的启用/禁用状态。",
		ReqType:     "NucleiTemplateUpdateEnabledReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/nuclei/detail", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "Nuclei 模板详情",
		Description: "按 id 返回 Nuclei 模板内容。",
		ReqType:     "NucleiTemplateDetailReq",
		RespType:    "NucleiTemplateDetailResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// ===== 批量验证 =====
	register(http.MethodPost, "/api/v1/poc/batchValidate", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量 POC 验证",
		Description: "批量对资产执行 POC 验证，返回任务 ID。",
		ReqType:     "PocBatchValidateReq",
		RespType:    "PocBatchValidateResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/poc/queryResult", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "查询 POC 验证结果",
		Description: "按任务 ID 查询 POC 批量验证的结果。",
		ReqType:     "PocValidationResultQueryReq",
		RespType:    "PocValidationResultQueryResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.TagMapping{},
		types.TagMappingListResp{},
		types.TagMappingSaveReq{},
		types.TagMappingDeleteReq{},
		types.CustomPoc{},
		types.CustomPocListReq{},
		types.CustomPocListResp{},
		types.CustomPocSaveReq{},
		types.CustomPocDeleteReq{},
		types.CustomPocBatchImportReq{},
		types.CustomPocBatchImportResp{},
		types.CustomPocClearAllReq{},
		types.CustomPocClearAllResp{},
		types.CustomPocScanAssetsReq{},
		types.CustomPocScanAssetsResp{},
		types.CustomPocScanVulnItem{},
		types.PocValidateReq{},
		types.PocValidateResp{},
		types.ValidatePocSyntaxReq{},
		types.ValidatePocSyntaxResp{},
		types.NucleiTemplate{},
		types.NucleiTemplateListReq{},
		types.NucleiTemplateListResp{},
		types.NucleiTemplateCategoriesResp{},
		types.NucleiTemplateUpdateEnabledReq{},
		types.NucleiTemplateDetailReq{},
		types.NucleiTemplateDetailResp{},
		types.NucleiTemplateSyncReq{},
		types.NucleiTemplateUploadItem{},
		types.NucleiTemplateSyncResp{},
		types.NucleiTemplateDownloadReq{},
		types.NucleiTemplateDownloadResp{},
		types.NucleiTemplateDownloadStatusReq{},
		types.NucleiTemplateDownloadStatusResp{},
		types.NucleiTemplateWithContent{},
		types.PocBatchValidateReq{},
		types.PocBatchValidateResp{},
		types.PocValidationResult{},
		types.PocValidationResultQueryReq{},
		types.PocValidationResultQueryResp{},
	)
}
