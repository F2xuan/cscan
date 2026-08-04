import request from './request'

// 获取支持的通知提供者列表
export function getNotifyProviders() {
  return request.post('/notify/providers', {})
}

// 获取通知配置列表
export function getNotifyConfigList() {
  return request.post('/notify/config/list', {})
}

// 保存通知配置
export function saveNotifyConfig(data) {
  return request.post('/notify/config/save', data)
}

// 删除通知配置
export function deleteNotifyConfig(id) {
  return request.post('/notify/config/delete', { id })
}

// 测试通知配置
export function testNotifyConfig(data) {
  return request.post('/notify/config/test', data)
}
