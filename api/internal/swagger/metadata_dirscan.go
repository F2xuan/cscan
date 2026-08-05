package swagger

import (
	"net/http"

	"cscan/api/internal/types"
	"cscan/internal/model"
)

// 目录扫描分组：字典 CRUD + 扫描结果列表/删除/清空/统计。
// 字典接口使用 types.DirScanDict*；扫描结果接口在 handler 内定义请求/响应，
// 此处为可编辑 Try it out，仅在 spec 上声明 requestBody，让用户能编辑 JSON。
func init() {
	tag := "目录扫描"
	tagDesc := "目录扫描字典管理 + 扫描结果查询与删除"

	// ===== DirScanDict 字典 CRUD（types 包内） =====
	register(http.MethodPost, "/api/v1/dirscan/dict/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "目录扫描字典列表",
		Description: "分页返回目录扫描字典，可按 `enabled` 过滤。",
		ReqType:     "DirScanDictListReq",
		RespType:    "DirScanDictListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/dirscan/dict/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存目录扫描字典",
		Description: "新增或更新目录扫描字典。`id` 为空时新增；非空时更新。",
		ReqType:     "DirScanDictSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/dirscan/dict/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除目录扫描字典",
		Description: "按 `id` 删除一个目录扫描字典。内置字典不可删除。",
		ReqType:     "DirScanDictDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/dirscan/dict/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空目录扫描字典",
		Description: "清空所有自定义目录扫描字典，返回删除数量。内置字典保留。",
		RespType:    "DirScanDictClearResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/dirscan/dict/enabled", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "启用的目录扫描字典列表",
		Description: "返回当前启用的目录扫描字典简化列表（`id / name / pathCount / isBuiltin`）。",
		RespType:    "DirScanDictEnabledListResp",
		Security:    TierAuth,
	})

	// ===== DirScanResult 结果（handler-local 类型，spec 仅注入空对象 body 供编辑） =====
	register(http.MethodPost, "/api/v1/dirscan/result/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "目录扫描结果列表",
		Description: "按 `taskId / authority / url / path / statusCode / sizeMin / sizeMax` 过滤，支持排序（`sortField` statusCode/contentLength/contentWords/contentLines/duration，`sortOrder` asc/desc）。\n\n**默认值**：page=1，pageSize=20。",
		ReqType:     "", // handler-local DirScanResultListReq
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/dirscan/result/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除扫描结果",
		Description: "按 `id` 删除单条目录扫描结果。",
		ReqType:     "",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/dirscan/result/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除扫描结果",
		Description: "按 `ids` 数组批量删除扫描结果。",
		ReqType:     "",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	register(http.MethodPost, "/api/v1/dirscan/result/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空扫描结果",
		Description: "清空当前工作空间全部目录扫描结果，返回删除数量。",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/dirscan/result/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "扫描结果统计",
		Description: "返回当前工作空间目录扫描结果按状态码分桶统计。",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.DirScanDict{},
		types.DirScanDictListReq{},
		types.DirScanDictListResp{},
		types.DirScanDictSaveReq{},
		types.DirScanDictDeleteReq{},
		types.DirScanDictClearResp{},
		types.DirScanDictEnabledListResp{},
		types.DirScanDictSimple{},
		model.DirScanResult{},
	)
}
