import request from './request'
import { useUserStore } from '@/stores/user'

// 复验类接口必须携带 workspaceId（后端将该字段视为必填，缺失会返回
// "field workspaceId is not set"）。优先使用调用方显式传入的值，否则回退到
// 当前用户所在工作空间；为空时后端 common.GetDefaultWorkspaceId 会解析为默认空间。
function withWorkspace(data) {
  const ws = useUserStore().workspaceId || ''
  return { ...data, workspaceId: data?.workspaceId ?? ws }
}

// 获取弱口令/敏感信息持续复验配置（T3.3 / T3.4）
export function getReverifyConfig(data) {
  return request.post('/vul/reverify/config/get', withWorkspace(data))
}

// 保存弱口令/敏感信息持续复验配置（T3.3 / T3.4）
export function saveReverifyConfig(data) {
  return request.post('/vul/reverify/config/save', withWorkspace(data))
}

// 立即触发弱口令复验（T3.3）
export function runNowReverify(data) {
  return request.post('/vul/reverify/runNow', withWorkspace(data))
}

// 单条漏洞复验（worker 执行复测）：下发复验任务到 worker
export function reverifyVul(data) {
  return request.post('/vul/reverify', data)
}
