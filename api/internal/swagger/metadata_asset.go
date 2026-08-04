package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 资产管理分组：资产 CRUD、统计、标签、历史、应用 / 图标 / 站点 / 域名 / IP 视图、
// 扫描结果集成 API（/api/v1/assets/*）、资产指纹列表 / 端口统计、资产分组。
//
// 全部接口为 JWT 鉴权（TierAuth），并通过 `X-Workspace-Id` 请求头或中间件识别当前工作空间。
func init() {
	tag := "资产管理"
	tagDesc := "网络资产的发现、查询、更新与下钻视图（站点 / 域名 / IP / 应用 / 图标 / 截图 / 历史）"

	// ===== 资产核心 =====
	register(http.MethodPost, "/api/v1/asset/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产列表",
		Description: "按工作空间分页返回资产列表，支持按 host / port / service / title / app / httpStatus / iconHash / 组织 ID 等字段过滤。\n\n**默认值**：page=1，pageSize=20。\n\n**特殊过滤**\n\n- `onlyNew`：仅展示首次出现的资产（`isNew=true`）。\n- `onlyUpdated`：仅展示最近有更新的资产（`isUpdated=true`）。\n- `excludeCdn`：排除识别为 CDN 的资产。\n- `sortByUpdate` / `sortByRisk`：分别按更新时间 / 风险评分排序。\n- `updatedWithinDays`：仅展示近 N 天有更新的资产，0 表示不限制。",
		ReqType:     "AssetListReq",
		RespType:    "AssetListResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/asset/port/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产端口列表",
		Description: "返回当前工作空间下的端口列表，供端口维度下钻视图使用。",
		ReqType:     "PortListReq",
		RespType:    "PortListResp",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/asset/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产统计",
		Description: "返回资产大盘统计：总数、host 数、新增数、变更数；TOP 端口 / 服务 / 应用 / 标题 / 图标 hash；风险等级分布。\n\n前端仪表盘页面直接消费此响应。",
		RespType:    "AssetStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/groups", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产分组列表",
		Description: "返回当前工作空间下用户自定义资产分组（用于资产收藏与命名筛选）。",
		ReqType:     "AssetGroupsReq",
		RespType:    "AssetGroupsResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/groups/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除资产分组",
		Description: "按分组 ID 删除一个用户自定义资产分组。删除分组不会删除分组中的资产本身，仅解除聚合关系。",
		ReqType:     "DeleteAssetGroupReq",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/inventory", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产清单（聚合视图）",
		Description: "按资产清单视图（端口、站点、域名、IP、应用、图标、截图等）聚合返回资产摘要，供卡片视图逐类浏览。",
		ReqType:     "AssetInventoryReq",
		RespType:    "AssetInventoryResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/screenshots", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "截图列表",
		Description: "分页返回带截图的资产列表。可按 host / title / app / orgId 过滤，并可仅查询有截图的资产。",
		ReqType:     "ScreenshotsReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/filterOptions", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产过滤选项",
		Description: "返回当前工作空间下所有可用过滤选项：技术栈、端口、HTTP 状态码、标签。供过滤器下拉框渲染。",
		ReqType:     "AssetFilterOptionsReq",
		RespType:    "AssetFilterOptionsResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/exposures", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产暴露面",
		Description: "返回单个资产 `assetId` 关联的目录扫描结果与漏洞扫描结果，供暴露面下钻视图展示。",
		ReqType:     "AssetExposuresReq",
		RespType:    "AssetExposuresResp",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/updateLabels", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "更新资产标签（整体替换）",
		Description: "整体替换资产 `id` 的标签数组。空数组会清空标签。\n\n**业务规则**：更新不会触碰资产的 `memo`、`color_tag`、风险字段与任务追踪字段。",
		ReqType:     "AssetUpdateLabelsReq",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/addLabel", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "新增资产标签",
		Description: "给资产 `id` 增量添加一个标签。重复添加不会创建重复标签。",
		ReqType:     "AssetAddLabelReq",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/removeLabel", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "移除资产标签",
		Description: "从资产 `id` 上移除一个标签。若标签不存在，幂等返回成功。",
		ReqType:     "AssetRemoveLabelReq",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除单个资产",
		Description: "按 `id` 删除资产；可附带 `workspaceId` 进行跨工作空间删除（需要管理员权限的实际语义由调用方保证）。",
		ReqType:     "AssetDeleteReq",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除资产",
		Description: "按 `ids` 数组批量删除资产。空数组返回成功但不做任何删除。",
		ReqType:     "AssetBatchDeleteReq",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空当前工作空间资产",
		Description: "清空当前工作空间下所有资产及其关联历史。**此操作不可逆**，前端必须二次确认。",
		Security:    TierAuth,
		Errors:      []int{500},
	})

	register(http.MethodPost, "/api/v1/asset/history", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产历史变更记录",
		Description: "返回单个资产的历史扫描版本列表与字段变更记录（`changes` 数组含旧值 / 新值对照）。",
		ReqType:     "AssetScanHistoryReq",
		RespType:    "AssetScanHistoryResp",
		Security:    TierAuth,
		Errors:      []int{10301, 500},
	})

	register(http.MethodPost, "/api/v1/asset/import", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "导入资产",
		Description: "批量导入资产 `targets`，支持 `IP:端口` 或 `URL` 格式。响应返回新增、跳过（已存在）、错误计数。",
		ReqType:     "AssetImportReq",
		RespType:    "AssetImportResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/asset/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "手动添加资产",
		Description: "手动录入资产：host、port、protocol（http/https/自定义）、title、labels、memo。供用户在不一定能通过扫描发现的场景下补充资产。",
		ReqType:     "AssetSaveReq",
		RespType:    "AssetSaveResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	// ===== 应用管理 =====
	register(http.MethodPost, "/api/v1/asset/app/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "应用列表",
		Description: "按应用维度聚合资产：返回每个应用下的资产 ID 列表、所属组织等信息。",
		ReqType:     "AppListReq",
		RespType:    "AppListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/app/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "应用统计",
		Description: "返回当前工作空间下的应用总数与新增数。",
		RespType:    "AppStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/app/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除应用聚合",
		Description: "按 `ids` 批量删除应用聚合记录。仅删除聚合，原资产的 `app` 字段保留。",
		ReqType:     "AppBatchDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/app/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空当前工作空间应用聚合",
		Description: "清空当前工作空间下所有应用聚合记录。",
		Security:    TierAuth,
	})

	// ===== Icon 管理 =====
	register(http.MethodPost, "/api/v1/asset/icon/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "图标列表",
		Description: "按图标 hash 维度分页返回图标聚合，含 base64 图像数据、截图与关联资产列表。",
		ReqType:     "IconListReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/icon/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "图标统计",
		Description: "返回当前工作空间下不同图标 hash 的聚合统计。",
		RespType:    "IconStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/icon/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除图标聚合",
		Description: "按 `ids` 批量删除图标聚合记录。",
		ReqType:     "IconBatchDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/icon/clear", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "清空当前工作空间图标聚合",
		Description: "清空当前工作空间下所有图标聚合记录。",
		Security:    TierAuth,
	})

	// ===== 扫描结果集成 API =====
	register(http.MethodPost, "/api/v1/assets/withScans", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产 + 扫描摘要列表",
		Description: "返回资产列表并附带每个资产的目录扫描数、漏洞扫描数、高危漏洞数、最近一次扫描时间，供资产视图一体化展示。",
		ReqType:     "AssetsWithScansReq",
		RespType:    "AssetsWithScansResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/assets/dirscans", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产目录扫描结果",
		Description: "按资产 ID 列表查询目录扫描结果。",
		ReqType:     "AssetDirScansReq",
		RespType:    "AssetDirScansResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/assets/vulnscans", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产漏洞扫描结果",
		Description: "按资产 ID 列表查询漏洞扫描结果。",
		ReqType:     "AssetVulnScansReq",
		RespType:    "AssetVulnScansResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/assets/history", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产历史扫描版本",
		Description: "按资产 ID 与时间范围返回历史扫描版本列表（含目录扫描数 / 漏洞扫描数 / 变更摘要）。",
		ReqType:     "AssetScanHistoryReq",
		RespType:    "AssetScanHistoryResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/assets/compareVersions", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "比较两个历史版本",
		Description: "比较同一资产两个历史版本 `versionId1` 与 `versionId2` 之间的目录扫描与漏洞差异数（新增 / 消失）。",
		ReqType:     "CompareVersionsReq",
		RespType:    "CompareVersionsResp",
		Security:    TierAuth,
	})

	// ===== 站点 / 域名 / IP =====
	register(http.MethodPost, "/api/v1/asset/site/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "站点列表",
		Description: "按工作空间分页返回站点列表，支持按 site / title / app / httpStatus / org 过滤。",
		ReqType:     "SiteListReq",
		RespType:    "SiteListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/site/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "站点统计",
		Description: "返回站点总数、HTTP / HTTPS 站点数、新增站点数。",
		RespType:    "SiteStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/site/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除站点",
		Description: "按站点 `id` 删除单个站点记录。",
		ReqType:     "SiteDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/site/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除站点",
		Description: "按 `ids` 批量删除站点记录。",
		ReqType:     "SiteBatchDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/domain/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "域名列表",
		Description: "按工作空间分页返回域名列表，可按 rootDomain、IP、org、关键词过滤。",
		ReqType:     "DomainListReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/domain/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "域名统计",
		Description: "返回域名维度统计。",
		RespType:    "DomainStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/domain/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除域名",
		Description: "按 `id` 删除单个域名。",
		ReqType:     "DomainDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/domain/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除域名",
		Description: "按 `ids` 批量删除域名。",
		ReqType:     "DomainBatchDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/ip/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "IP 列表",
		Description: "按工作空间分页返回 IP 列表，含 IPv4 / IPv6 双栈信息与所属资产。",
		ReqType:     "IPListReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/ip/stat", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "IP 统计",
		Description: "返回 IP 维度统计：总数、新增数、CDN / 云识别数。",
		RespType:    "IPStatResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/ip/delete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "删除 IP",
		Description: "按 `id` 删除单个 IP 记录。",
		ReqType:    "IPDeleteReq",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/ip/batchDelete", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量删除 IP",
		Description: "按 `ids` 批量删除 IP 记录。",
		ReqType:     "IPBatchDeleteReq",
		Security:    TierAuth,
	})

	// ===== 资产指纹 / 端口统计 =====
	register(http.MethodPost, "/api/v1/asset/fingerprints/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产指纹列表",
		Description: "按 `limit` 返回当前工作空间识别到的指纹关键词列表，供指纹看板拖拽展示。",
		ReqType:     "AssetFingerprintsListReq",
		RespType:    "AssetFingerprintsListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/asset/ports/stats", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "资产端口统计",
		Description: "返回各端口的资产数和服务名聚合。",
		RespType:    "AssetPortsStatsResp",
		Security:    TierAuth,
	})

	// 反射器入口：把本分组用到的 types 结构体登记
	RegisterTypes(
		types.Asset{},
		types.IPInfo{},
		types.IPV4Info{},
		types.IPV6Info{},
		types.AssetListReq{},
		types.AssetListResp{},
		types.AssetStatResp{},
		types.StatItem{},
		types.IconHashStatItem{},
		types.AssetDeleteReq{},
		types.AssetBatchDeleteReq{},
		types.AssetImportReq{},
		types.AssetImportResp{},
		types.AssetSaveReq{},
		types.AssetSaveResp{},
		types.AssetHistoryItem{},
		types.FieldChange{},
		types.SiteListReq{},
		types.Site{},
		types.SiteListResp{},
		types.SiteStatResp{},
		types.SiteDeleteReq{},
		types.SiteBatchDeleteReq{},
		types.DomainListReq{},
		types.DomainStatResp{},
		types.DomainDeleteReq{},
		types.DomainBatchDeleteReq{},
		types.IPListReq{},
		types.IPStatResp{},
		types.IPDeleteReq{},
		types.IPBatchDeleteReq{},
		types.AssetGroupsReq{},
		types.AssetGroup{},
		types.AssetGroupsResp{},
		types.DeleteAssetGroupReq{},
		types.AssetInventoryReq{},
		types.AssetInventoryItem{},
		types.AssetInventoryResp{},
		types.ScreenshotsReq{},
		types.AssetUpdateLabelsReq{},
		types.AssetAddLabelReq{},
		types.AssetRemoveLabelReq{},
		types.AssetFilterOptionsReq{},
		types.AssetFilterOptionsResp{},
		types.AssetExposuresReq{},
		types.DirScanResultItem{},
		types.VulnResultItem{},
		types.AssetExposuresResp{},
		types.AssetsWithScansReq{},
		types.AssetWithScans{},
		types.AssetsWithScansResp{},
		types.AssetDirScansReq{},
		types.AssetDirScansResp{},
		types.AssetVulnScansReq{},
		types.AssetVulnScansResp{},
		types.AssetScanHistoryReq{},
		types.HistoricalVersion{},
		types.AssetScanHistoryResp{},
		types.CompareVersionsReq{},
		types.CompareVersionsResp{},
		types.AppListReq{},
		types.AppItem{},
		types.AppListResp{},
		types.AppStatResp{},
		types.AppDeleteReq{},
		types.AppBatchDeleteReq{},
		types.IconListReq{},
		types.IconItem{},
		types.IconStatResp{},
		types.IconBatchDeleteReq{},
		types.PortListReq{},
		types.PortListResp{},
		types.PortStatItem{},
		types.AssetFingerprintsListReq{},
		types.AssetFingerprintsListResp{},
		types.AssetPortsStatsResp{},
	)
}
