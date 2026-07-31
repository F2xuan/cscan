package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 资产变化快照分组（T1.1）
func init() {
	tag := "资产变化"
	tagDesc := "扫描批次变化基线快照（新增/更新）"

	register(http.MethodPost, "/api/v1/asset/diff/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产变化列表",
		Description: "按任务/时间范围/变化类型分页查询变化明细。",
		ReqType:     "AssetDiffListReq",
		RespType:    "AssetDiffListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/asset/diff/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产变化统计",
		Description: "按 diff_type + change_type 聚合计数。",
		ReqType:     "AssetDiffStatReq",
		RespType:    "AssetDiffStatResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.AssetDiffListReq{},
		types.AssetDiffListResp{},
		types.AssetDiffItem{},
		types.FieldChange{},
		types.AssetDiffStatReq{},
		types.AssetDiffStatResp{},
		types.AssetDiffStatItem{},
	)
}
