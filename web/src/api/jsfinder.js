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
