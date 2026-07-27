import request from '@/api/request'

/**
 * 资产分组API
 */

// 获取资产分组列表
export function getAssetGroups(data) {
  return request({
    url: '/asset/groups',
    method: 'post',
    data
  })
}

// 删除资产分组
export function deleteAssetGroup(data) {
  return request({
    url: '/asset/groups/delete',
    method: 'post',
    data
  })
}

// 获取资产清单
export function getAssetInventory(data) {
  return request({
    url: '/asset/inventory',
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

// 获取资产列表（原有接口）
export function getAssetList(data) {
  return request({
    url: '/asset/list',
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

// 批量删除资产
export function batchDeleteAssets(data) {
  return request({
    url: '/asset/batchDelete',
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

// 获取资产历史
export function getAssetHistory(data) {
  return request({
    url: '/assets/history',
    method: 'post',
    data
  })
}

// 比较两个历史版本
export function compareVersions(data) {
  return request({
    url: '/assets/compareVersions',
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

// 手动添加资产
export function saveAsset(data) {
  return request({
    url: '/asset/save',
    method: 'post',
    data
  })
}

// 导出资产
export function exportAssets(data) {
  return request({
    url: '/asset/export',
    method: 'post',
    data,
    responseType: 'blob'
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

// 添加资产标签
export function addAssetLabel(data) {
  return request({
    url: '/asset/addLabel',
    method: 'post',
    data
  })
}

// 删除资产标签
export function removeAssetLabel(data) {
  return request({
    url: '/asset/removeLabel',
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

// 获取资产目录扫描结果（支持分页）
export function getAssetDirScans(data) {
  return request({
    url: '/assets/dirscans',
    method: 'post',
    data
  })
}

// 获取资产漏洞扫描结果（支持分页）
export function getAssetVulnScans(data) {
  return request({
    url: '/assets/vulnscans',
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

// 顶层资产详情（实时 exposure + risk + sensitive top-N）
export function getAssetTargetDetail(data) {
  return request({
    url: '/asset/target/detail',
    method: 'post',
    data
  })
}

// 更新顶层资产元信息（labels / memo / colorTag）
export function updateAssetTarget(data) {
  return request({
    url: '/asset/target/update',
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
