package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 报告管理分组：扫描报告详情查看与导出。
// 全部接口为 JWT 鉴权（TierAuth），并按 X-Workspace-Id 隔离工作空间。
func init() {
	tag := "报告管理"
	tagDesc := "扫描报告详情查看与导出（Excel / PDF）"

	register(http.MethodPost, "/api/v1/report/detail", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "报告详情",
		Description: "按 `taskId` 返回该次扫描任务的完整报告数据：资产清单、漏洞列表、目录扫描结果、统计大盘（端口 / 服务 / 应用 / 漏洞等级分布）。\n\n**字段说明**\n\n- `taskId`：必填， MainTask ID。\n- 响应 `data` 包含 `assets / vuls / dirScans / dirScanStat / topPorts / topServices / topApps / vulStats` 等聚合字段。\n\n**典型错误码**\n\n- 400 参数错误（taskId 缺失）\n- 500 服务器错误",
		ReqType:     "ReportDetailReq",
		RespType:    "ReportDetailResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/report/export", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导出报告",
		Description: "按 `taskId` 导出扫描报告为 Excel 或 PDF 文件，响应为二进制流，前端需以 `Content-Disposition: attachment` 触发下载。\n\n**字段说明**\n\n- `taskId`：必填。\n- `format`：`excel` 或 `pdf`，缺省 `excel`。\n\n**典型错误码**\n\n- 400 参数错误\n- 500 服务器错误",
		ReqType:     "ReportExportReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.ReportDetailReq{},
		types.ReportDetailResp{},
		types.ReportData{},
		types.ReportAsset{},
		types.ReportVul{},
		types.ReportDirScan{},
		types.ReportDirScanStat{},
		types.ReportExportReq{},
	)
}
