import request from './request'

// Worker 控制台 API

// 获取 Worker 系统信息
export function getWorkerInfo(workerName) {
  return request.get('/worker/console/info', { params: { name: workerName } })
}

// 文件管理
export function listFiles(workerName, path) {
  return request.get('/worker/console/files', { params: { name: workerName, path: path || '.' } })
}

export function uploadFile(workerName, path, file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = async () => {
      try {
        // 将文件内容转为 Base64
        const base64 = reader.result.split(',')[1]
        const fullPath = path === '.' ? file.name : `${path}/${file.name}`
        const res = await request.post('/worker/console/files/upload', 
          { path: fullPath, data: base64 },
          { params: { name: workerName } }
        )
        resolve(res)
      } catch (e) {
        reject(e)
      }
    }
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

export function downloadFile(workerName, path) {
  return request.get('/worker/console/files/download', {
    params: { name: workerName, path }
  })
}

export function deleteFile(workerName, path) {
  return request.delete('/worker/console/files', { params: { name: workerName, path } })
}

export function createDir(workerName, path) {
  return request.post('/worker/console/files/mkdir', 
    { path },
    { params: { name: workerName } }
  )
}

// 终端操作
export function openTerminal(workerName) {
  return request.post('/worker/console/terminal/open', 
    { workerName }
  )
}

export function closeTerminal(workerName, sessionId) {
  return request.post('/worker/console/terminal/close', 
    { workerName, sessionId }
  )
}

export function execCommand(workerName, sessionId, command) {
  return request.post('/worker/console/terminal/exec', 
    { workerName, sessionId, command }
  )
}

// 审计日志
export function getAuditLogs(workerName, page = 1, pageSize = 20) {
  return request.get('/worker/console/audit', { params: { workerName, page, pageSize } })
}

// 清空审计日志
export function clearAuditLogs(workerName) {
  return request.delete('/worker/console/audit', { params: { workerName } })
}
