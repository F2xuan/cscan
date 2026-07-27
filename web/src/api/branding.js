import request from './request'

export function getBrandingConfig() {
  return request.post('/branding/config/get')
}

export function saveBrandingConfig(data) {
  return request.post('/branding/config/save', data)
}
