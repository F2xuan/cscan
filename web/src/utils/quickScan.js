// 前端目标输入解析（概念与后端 scanner.TargetParser 对齐：支持逗号/换行/分号分隔）
// 仅做拆分与基本校验，真实类型识别与扫描阶段选择由后端完成。

export function splitTargets(raw) {
  if (!raw) return []
  return String(raw)
    .split(/[\n,;]+/)
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
}

export function isValidTargets(raw) {
  return splitTargets(raw).length > 0
}
