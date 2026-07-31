import request from './request'

// 证书列表（只读，指纹识别附加产出的证书快照）
export function getCertList(data) {
  return request.post('/cert/list', data)
}

// 证书详情
export function getCertDetail(data) {
  return request.post('/cert/detail', data)
}
