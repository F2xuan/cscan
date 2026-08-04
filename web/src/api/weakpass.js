import request from './request'

// 获取弱口令字典列表
export function getWeakpassDictList(params) {
  return request.post('/weakpass/dict/list', params)
}

// 保存弱口令字典
export function saveWeakpassDict(data) {
  return request.post('/weakpass/dict/save', data)
}

// 删除弱口令字典
export function deleteWeakpassDict(params) {
  return request.post('/weakpass/dict/delete', params)
}

// 清空所有非内置字典
export function clearWeakpassDict() {
  return request.post('/weakpass/dict/clear')
}