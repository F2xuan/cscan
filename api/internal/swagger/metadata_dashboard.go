package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 工作台分组（T1.5）
func init() {
	tag := "工作台"
	tagDesc := "工作台变化数据：资产变化与风险变化（新增/已修复/净变化）"

	register(http.MethodPost, "/api/v1/dashboard/changes", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "工作台变化数据",
		Description: "返回窗口内（默认 7 天）的资产变化（总数/新增/增长率/分类分布）与风险变化（待处理/新发现/已修复/净变化/严重度分布）。数据源为资产 first_seen_time 窗口与漏洞 status/first_seen_time/fixed_at 窗口，单次聚合取数。",
		ReqType:     "DashboardChangesReq",
		RespType:    "DashboardChangesResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.DashboardChangesReq{},
		types.DashboardChangesResp{},
		types.AssetChanges{},
		types.RiskChanges{},
	)
}
