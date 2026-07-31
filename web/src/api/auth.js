import request from './request'

export function login(data) {
  return request.post('/login', data)
}

export function getUserList(data) {
  return request.post('/user/list', data)
}

export function createUser(data) {
  return request.post('/user/create', data)
}

export function updateUser(data) {
  return request.post('/user/update', data)
}

export function deleteUser(data) {
  return request.post('/user/delete', data)
}

export function resetUserPassword(data) {
  return request.post('/user/resetPassword', data)
}

// 首次登录密码重置（不需要原密码验证）
export function firstLoginResetPassword(data) {
  return request.post('/user/firstLoginResetPassword', data)
}

// 用户头像上传（multipart）
export function uploadUserAvatar(file) {
  const formData = new FormData()
  formData.append('file', file)
  return request.post('/user/avatar/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

// ==================== 个人中心 ====================
export function getUserProfile() {
  return request.post('/user/profile/get')
}

export function updateUserProfile(data) {
  return request.post('/user/profile/update', data)
}

export function changeUserPassword(data) {
  return request.post('/user/password/change', data)
}

// ==================== 个人 API Token ====================
export function createUserToken(data) {
  return request.post('/user/token/create', data)
}

export function listUserTokens() {
  return request.post('/user/token/list')
}

export function setUserTokenStatus(data) {
  return request.post('/user/token/setStatus', data)
}

export function getUserTokenScopes() {
  return request.post('/user/token/scopes')
}

// ==================== 引导式首次体验 (T4.2) ====================
export function getOnboardingStatus() {
  return request.post('/user/onboarding/status')
}

export function completeOnboarding() {
  return request.post('/user/onboarding/complete')
}
