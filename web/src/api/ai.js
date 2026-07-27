import request from './request'

// AI生成POC
export function generatePoc(data) {
  return request.post('/ai/generatePoc', data)
}

// AI配置（按工作空间隔离）
export function getAIConfig() {
  return request.post('/ai/config/get')
}

export function saveAIConfig(data) {
  return request.post('/ai/config/save', data)
}
