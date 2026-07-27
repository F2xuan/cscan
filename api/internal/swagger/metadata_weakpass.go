package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

func init() {
	tag := "弱口令字典"
	tagDesc := "弱口令字典管理（用户名 + 密码组合）"

	register(http.MethodPost, "/api/v1/weakpass/dict/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "弱口令字典列表",
		Description: "分页返回弱口令字典，可按 enabled 过滤。",
		ReqType:     "WeakpassDictListReq",
		RespType:    "WeakpassDictListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存弱口令字典",
		Description: "新增或更新弱口令字典。id 为空时新增。",
		ReqType:     "WeakpassDictSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除弱口令字典",
		Description: "按 id 删除一个弱口令字典。内置字典不可删除。",
		ReqType:     "WeakpassDictDeleteReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空弱口令字典",
		Description: "清空所有自定义弱口令字典，返回删除数量。内置字典保留。",
		RespType:    "WeakpassDictClearResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/enabled", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "启用的弱口令字典列表",
		Description: "返回当前启用的弱口令字典简化列表，供任务创建时选择。",
		RespType:    "WeakpassDictEnabledListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入弱口令字典",
		Description: "批量导入弱口令字典（可多组）。",
		ReqType:     "WeakpassDictImportReq",
		RespType:    "WeakpassDictImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/export", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导出弱口令字典",
		Description: "按 id 导出指定弱口令字典内容。",
		ReqType:     "WeakpassDictExportReq",
		RespType:    "WeakpassDictExportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/parse", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "解析弱口令字典内容",
		Description: "解析字典文本（按服务类型分组），用于保存前预览。",
		ReqType:     "WeakpassDictParseReq",
		RespType:    "WeakpassDictParseResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/weakpass/dict/stats", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "服务类型统计",
		Description: "返回弱口令字典按服务类型（SSH / MySQL / RDP 等）的计数分布。",
		RespType:    "WeakpassDictServiceStatsResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.WeakpassDict{},
		types.WeakpassDictListReq{},
		types.WeakpassDictListResp{},
		types.WeakpassDictSaveReq{},
		types.WeakpassDictDeleteReq{},
		types.WeakpassDictClearResp{},
		types.WeakpassDictEnabledListResp{},
		types.WeakpassDictSimple{},
		types.WeakpassDictImportReq{},
		types.WeakpassDictImportResp{},
		types.WeakpassDictExportReq{},
		types.WeakpassDictExportResp{},
		types.WeakpassDictServiceStatsResp{},
		types.WeakpassDictServiceStat{},
		types.WeakpassDictParseReq{},
		types.WeakpassDictParseResp{},
		types.WeakpassDictGroup{},
	)
}
