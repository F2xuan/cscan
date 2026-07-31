import request from './request'

// 容器日志 API - Docker 容器日志查看

// 列出所有 cscan 相关 Docker 容器
export function listContainers() {
  return request.post('/container/list')
}

// 一次性拉取最近 N 行容器日志
export function fetchContainerLogs(data) {
  return request.post('/container/logs/fetch', data)
}

// ==================== 日志历史(本地文件) ====================

// 获取有日志的日期列表(降序)
export function getLogDates() {
  return request.get('/container/logs/dates')
}

// 获取某天有日志的容器文件列表
export function getLogFiles(date) {
  return request.get('/container/logs/files', { params: { date } })
}

// 读取指定日期+容器的历史日志
export function getLogHistory(params) {
  return request.get('/container/logs/history', { params })
}
