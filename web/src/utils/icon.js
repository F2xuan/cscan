/**
 * Favicon 工具函数
 * 只允许已知图片格式的 base64 渲染为 data URI（白名单模式，防止 HTML/JSON/XML 等被误识别为图片）
 */

const IMAGE_BASE64_PREFIXES = [
  ['iVBOR', 'image/png'],
  ['/9j/', 'image/jpeg'],
  ['R0lG', 'image/gif'],
  ['AAABAA', 'image/x-icon'],
  ['PHN2Zy', 'image/svg+xml'],
  ['Qk0', 'image/bmp'],
  ['UklGR', 'image/webp']
]

/**
 * 将 favicon 的 base64 数据转为可渲染的 data URI
 * @param {string} iconData base64 字符串或完整 data URI
 * @returns {string} data URI，无法识别时返回空字符串
 */
export function getIconDataUrl(iconData) {
  if (typeof iconData !== 'string' || iconData.length === 0) return ''
  if (iconData.startsWith('data:')) return iconData

  const base64Str = iconData.replace(/\s/g, '')
  if (!base64Str) return ''

  const matched = IMAGE_BASE64_PREFIXES.find(([prefix]) => base64Str.startsWith(prefix))
  return matched ? `data:${matched[1]};base64,${base64Str}` : ''
}

/**
 * 图标加载失败时隐藏元素，避免出现浏览器默认破图
 * @param {Event} e
 */
export function handleIconError(e) {
  if (e && e.target) {
    e.target.style.opacity = '0'
    e.target.style.width = '16px'
  }
}
