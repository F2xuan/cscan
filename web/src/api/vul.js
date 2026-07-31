import request from './request'

// 获取弱口令/敏感信息持续复验配置（T3.3 / T3.4）
export function getReverifyConfig(data) {
  return request.post('/vul/reverify/config/get', data)
}

// 保存弱口令/敏感信息持续复验配置（T3.3 / T3.4）
export function saveReverifyConfig(data) {
  return request.post('/vul/reverify/config/save', data)
}

// 立即触发弱口令复验（T3.3）
export function runNowReverify(data) {
  return request.post('/vul/reverify/runNow', data)
}

// 单条漏洞复验（worker 执行复测）：下发复验任务到 worker
export function reverifyVul(data) {
  return request.post('/vul/reverify', data)
}
