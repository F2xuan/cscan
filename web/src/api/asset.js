import request from '@/api/request'

// 获取资产清单
export function getAssetInventory(data) {
  return request({
    url: '/asset/inventory',
    method: 'post',
    data
  })
}

// 获取资产详情（按需加载完整资产，含 body/header/banner 等大字段）
export function getAssetDetail(data) {
  return request({
    url: '/asset/detail',
    method: 'post',
    data
  })
}

// 获取截图清单
export function getScreenshots(data) {
  return request({
    url: '/asset/screenshots',
    method: 'post',
    data
  })
}

// 获取资产统计
export function getAssetStat() {
  return request({
    url: '/asset/stat',
    method: 'post'
  })
}

// 删除资产
export function deleteAsset(data) {
  return request({
    url: '/asset/delete',
    method: 'post',
    data
  })
}

// 清空资产
export function clearAssets() {
  return request({
    url: '/asset/clear',
    method: 'post'
  })
}

// 清空域名
export function clearDomains() {
  return request({
    url: '/asset/domain/clear',
    method: 'post'
  })
}

// 清空 IP
export function clearIPs() {
  return request({
    url: '/asset/ip/clear',
    method: 'post'
  })
}

// 清空站点
export function clearSites() {
  return request({
    url: '/asset/site/clear',
    method: 'post'
  })
}

// 清空端口
export function clearPorts() {
  return request({
    url: '/asset/port/clear',
    method: 'post'
  })
}

// 清空截图
export function clearScreenshots() {
  return request({
    url: '/asset/screenshots/clear',
    method: 'post'
  })
}

// 获取资产历史
export function getAssetHistory(data) {
  return request({
    url: '/assets/history',
    method: 'post',
    data
  })
}

// 资产变更时间线数据源（已合并到 V2 /assets/history，返回 versions + list）
export function getAssetChangeHistory(data) {
  return request({
    url: '/assets/history',
    method: 'post',
    data
  })
}

// 导入资产
export function importAssets(data) {
  return request({
    url: '/asset/import',
    method: 'post',
    data
  })
}

// 更新资产标签
export function updateAssetLabels(data) {
  return request({
    url: '/asset/updateLabels',
    method: 'post',
    data
  })
}

// 获取资产过滤器选项（技术栈、端口、状态码）
export function getAssetFilterOptions(data) {
  return request({
    url: '/asset/filterOptions',
    method: 'post',
    data
  })
}

// 获取资产暴露面（目录扫描和漏洞扫描结果）
export function getAssetExposures(data) {
  return request({
    url: '/asset/exposures',
    method: 'post',
    data
  })
}

// T1.3：批量更新漏洞生命周期状态（open / fixed / ignored）
export function updateVulStatus(data) {
  return request({
    url: '/vul/updateStatus',
    method: 'post',
    data
  })
}

/**
 * 顶层资产 (target) API — Phase 4
 * 资产 = 主机 IP 或主域名，以 "{type}:{value}" 编码为 targetId
 */

// 顶层资产分页列表（含 exposure/risk 气泡字段）
export function getAssetTargetList(data) {
  return request({
    url: '/asset/target/list',
    method: 'post',
    data
  })
}

// 删除顶层资产（可选级联删除底层 asset + vul）
export function deleteAssetTarget(data) {
  return request({
    url: '/asset/target/delete',
    method: 'post',
    data
  })
}
