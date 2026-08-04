import request from './request'

// 容器日志 API - Docker 容器日志查看

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
