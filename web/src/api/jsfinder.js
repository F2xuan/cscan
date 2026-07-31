import request from './request'

// 获取 JSFinder 全局配置
export function getJSFinderConfig() {
  return request.post('/jsfinder/config/get')
}

// 保存 JSFinder 全局配置
export function saveJSFinderConfig(data) {
  return request.post('/jsfinder/config/save', data)
}

// 重置为内置默认值
export function resetJSFinderConfig() {
  return request.post('/jsfinder/config/reset')
}

// 获取单条 JSFinder 结果详情（含 request/response/curl_command 大字段）
export function getJSFinderDetail(data) {
  return request.post('/jsfinder/detail', data)
}

// 单条AI研判
export function analyzeJSByAI(data) {
  return request.post('/jsfinder/ai/analyze', data)
}

// 批量研判所有未研判数据（异步）
export function batchAnalyzeJSByAI(data) {
  return request.post('/jsfinder/ai/batch-analyze', data)
}

// 查询批量研判进度
export function getBatchAnalyzeProgress(data) {
  return request.post('/jsfinder/ai/batch-progress', data)
}

// 停止批量AI研判
export function stopBatchAnalyze(data) {
  return request.post('/jsfinder/ai/batch-stop', data)
}
