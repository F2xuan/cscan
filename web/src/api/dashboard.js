import request from '@/api/request'

// 工作台变化数据：资产变化 + 风险变化（T1.5）
export function getDashboardChanges(params) {
  return request({
    url: '/dashboard/changes',
    method: 'post',
    data: params || {}
  })
}
