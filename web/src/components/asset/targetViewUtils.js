/**
 * 目标视图共享工具（列表/详情/Inventory 子页）
 */

// 相对时间（dayjs fromNow 风格），接受 unix ms
export function formatRelativeTime(timestamp) {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  if (isNaN(date.getTime())) return '-'
  const diff = Date.now() - date.getTime()
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'Just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  const weeks = Math.floor(days / 7)
  if (weeks < 4) return `${weeks}w ago`
  return date.toLocaleDateString()
}

// HTTP 状态码 → 徽章样式类（2xx/3xx/4xx/5xx）
export function getStatusCodeClass(statusCode) {
  const code = parseInt(statusCode)
  if (code >= 200 && code < 300) return 'status-2xx'
  if (code >= 300 && code < 400) return 'status-3xx'
  if (code >= 400 && code < 500) return 'status-4xx'
  if (code >= 500) return 'status-5xx'
  return 'status-other'
}

// 状态码归一化：从原始值（可能为 "502 Bad Gateway" / "0" / "" ）提取 3 位数字。
// 无有效状态码（0/空/非 3 位数字开头）返回 ''，调用方应隐藏徽章。
export function getStatusCodeText(statusCode) {
  if (!statusCode) return ''
  const m = String(statusCode).match(/\b([1-9]\d{2})\b/)
  if (!m) return ''
  const code = parseInt(m[1])
  if (code < 100 || code > 599) return ''
  return m[1]
}

// 证书状态：valid / expiring(30天内) / expired
export function getCertStatus(notAfterMs) {
  if (!notAfterMs) return 'valid'
  const remain = notAfterMs - Date.now()
  if (remain < 0) return 'expired'
  if (remain < 30 * 24 * 3600 * 1000) return 'expiring'
  return 'valid'
}
