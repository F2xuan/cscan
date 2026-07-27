import request from './request'

// 容器日志 API - Docker 容器实时日志查看

// 列出所有 cscan 相关 Docker 容器
export function listContainers() {
  return request.post('/container/list')
}

// 一次性拉取最近 N 行容器日志(用于导出或 SSE 降级)
export function fetchContainerLogs(data) {
  return request.post('/container/logs/fetch', data)
}

// 构建单容器 SSE 订阅 URL(EventSource 无法设置 Authorization 头,token 通过查询参数传递)
export function buildStreamURL(params) {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, v)
  })
  return `/api/v1/container/logs/stream?${qs.toString()}`
}

// 构建多容器合并 SSE 订阅 URL
export function buildMergedStreamURL(params) {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, v)
  })
  return `/api/v1/container/logs/stream/merged?${qs.toString()}`
}
