import request from './request'

// 获取报告详情
export function getReportDetail(data) {
  return request({
    url: '/report/detail',
    method: 'post',
    data
  })
}

// 导出报告
export function exportReport(data) {
  return request({
    url: '/report/export',
    method: 'post',
    data,
    responseType: 'blob'
  })
}

// T5.1 周期报告（日报/周报/月报）
export function getPeriodicReport(data) {
  return request({
    url: '/report/periodic/generate',
    method: 'post',
    data
  })
}

export function exportPeriodicReport(data) {
  return request({
    url: '/report/periodic/export',
    method: 'post',
    data,
    responseType: 'blob'
  })
}
