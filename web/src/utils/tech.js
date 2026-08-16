/**
 * 技术栈展示工具：技术名归一化 + 图标 URL 构造
 *
 * 后端 asset.app 条目形如 "Kibana[httpx+wappalyzer]"、"Nginx:1.18.0[httpx]"，
 * 展示与图标匹配时需要去掉来源后缀与版本号。
 */

// 去掉检测来源后缀 [httpx+wappalyzer] 与 :版本号，保留原始大小写的技术名
export function getTechName(tech) {
  if (!tech) return ''
  let name = String(tech).replace(/\[[^\]]*\]\s*$/, '').trim()
  const colon = name.indexOf(':')
  if (colon > 0) name = name.slice(0, colon).trim()
  return name
}

// 技术栈图标 URL（后端按需从指纹库拉取并本地缓存；无图标时返回 404，由 TechTag 隐藏）
export function techIconUrl(tech) {
  const name = getTechName(tech)
  if (!name) return ''
  return `/api/v1/tech/icon?name=${encodeURIComponent(name)}`
}
