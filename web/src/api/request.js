import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import router from '@/router'
import { i18n } from '@/i18n'

// 在拦截器中（非组件上下文）通过全局 i18n 实例进行翻译
const t = (key) => i18n.global.t(key)

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

// 请求拦截器
request.interceptors.request.use(
  config => {
    const userStore = useUserStore()

    // BUG-001 修复：未登录时，除登录/健康检查等公开接口外，拒绝所有需要认证的请求
    const publicEndpoints = ['/login', '/health']
    const isPublicEndpoint = publicEndpoints.some(endpoint => config.url?.includes(endpoint))

    if (!userStore.token && !isPublicEndpoint) {
      console.warn('[Request] Blocked unauthenticated request:', config.url)
      return Promise.reject(new Error('No authentication token'))
    }

    if (userStore.token) {
      config.headers['Authorization'] = `Bearer ${userStore.token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// 响应拦截器
// isRelogin 防重入标志：密码修改或 token 过期后，并发的 401 响应会集中涌入，
// 只处理第一个 401（logout + 跳转登录页），其余直接 reject 不重复弹窗。
// 标志在跳转完成后才重置，避免短时间窗口内重复触发。
let isRelogin = false

const isLoginRequest = (config) => {
  const url = config?.url || ''
  return url === '/login' || url.endsWith('/login')
}

request.interceptors.response.use(
  response => {
    const res = response.data
    if (res.code === 401) {
      if (isLoginRequest(response.config)) {
        return res
      }
      if (!isRelogin) {
        isRelogin = true
        const userStore = useUserStore()
        userStore.logout()
        router.push('/login').catch(() => {}).finally(() => {
          isRelogin = false
        })
        ElMessage({
          message: t('error.sessionExpired'),
          type: 'error',
          grouping: true
        })
      }
      return Promise.reject(new Error('Unauthorized'))
    }
    return res
  },
  error => {
    if (error.response && error.response.status === 401) {
      if (isLoginRequest(error.config)) {
        const data = error.response.data
        if (data && typeof data === 'object') {
          return Promise.resolve(data)
        }
        return Promise.reject(error)
      }
      if (!isRelogin) {
        isRelogin = true
        const userStore = useUserStore()
        userStore.logout()
        router.push('/login').catch(() => {}).finally(() => {
          isRelogin = false
        })
        ElMessage({
          message: t('error.sessionExpired'),
          type: 'error',
          grouping: true
        })
      }
    } else if (error.response && error.response.status === 503) {
      // 503 服务不可用：基础设施故障（如 MongoDB 宕机），返回结构化错误让调用方区分
      // 注意：登录接口返回 503 时，绝不能显示"密码错误"，否则会误导用户
      const data = error.response.data
      if (data && typeof data === 'object') {
        return Promise.resolve(data)
      }
      return Promise.reject(error)
    } else {
      // 优化网络错误和后端未启动时的弹窗提示
      const errorMsg = error.message || '请求失败'
      if (errorMsg === 'Network Error' || error.code === 'ECONNABORTED' || errorMsg.includes('timeout')) {
        ElMessage({
          message: t('error.networkError'),
          type: 'error',
          grouping: true
        })
      } else {
        ElMessage({
          message: errorMsg,
          type: 'error',
          grouping: true
        })
      }
    }
    return Promise.reject(error)
  }
)

export default request
