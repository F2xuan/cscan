import request from './request'

// 获取定时任务列表
// 可选字段：data.taskType 用于按类型过滤 ('asset_scan' | 'space_engine')
export function getCronTaskList(data) {
  return request.post('/task/cron/list', data)
}

// 保存定时任务
// data 字段扩展：
//   - taskType: 'asset_scan' | 'space_engine' （默认 asset_scan）
//   - engineType: 空间引擎类型（如 fofa/hunter/quake/shodan/...）
//   - query: 空间引擎查询语句
//   - cronSpec: cron 表达式
//   - enabled: 是否启用
export function saveCronTask(data) {
  return request.post('/task/cron/save', data)
}

// 开关定时任务
export function toggleCronTask(data) {
  return request.post('/task/cron/toggle', data)
}

// 删除定时任务
export function deleteCronTask(data) {
  return request.post('/task/cron/delete', data)
}

// 批量删除定时任务
export function batchDeleteCronTask(data) {
  return request.post('/task/cron/batchDelete', data)
}

// 立即执行定时任务
export function runCronTaskNow(data) {
  return request.post('/task/cron/runNow', data)
}

// 验证Cron表达式
export function validateCronSpec(data) {
  return request.post('/task/cron/validate', data)
}

// 获取定时任务详情
export function getCronTaskDetail(data) {
  return request.post('/task/cron/detail', data)
}

// ===== 空间引擎定时任务便捷方法（内部自动带 taskType）=====

// 获取空间引擎定时任务列表
export function getSpaceEngineCronTaskList(data = {}) {
  return getCronTaskList({ ...data, taskType: 'space_engine' })
}

// 保存空间引擎定时任务
export function saveSpaceEngineCronTask(data) {
  return saveCronTask({ ...data, taskType: 'space_engine' })
}
