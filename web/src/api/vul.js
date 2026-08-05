import request from './request'

// 复验类接口必须携带 workspaceId，默认使用 "default"
function withWorkspace(data) {
  return { ...data, workspaceId: data?.workspaceId || 'default' }
}

// 获取弱口令/敏感信息持续复验配置（T3.3 / T3.4）
export function getReverifyConfig(data) {
  return request.post('/vul/reverify/config/get', withWorkspace(data))
}

// 保存弱口令/敏感信息持续复验配置（T3.3 / T3.4）
export function saveReverifyConfig(data) {
  return request.post('/vul/reverify/config/save', withWorkspace(data))
}

// 立即触发弱口令复验（T3.3）
export function runNowReverify(data) {
  return request.post('/vul/reverify/runNow', withWorkspace(data))
}

// 单条漏洞复验（worker 执行复测）：下发复验任务到 worker
export function reverifyVul(data) {
  return request.post('/vul/reverify', data)
}
