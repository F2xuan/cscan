// validatePasswordStrength 校验密码强度：至少 8 位，且包含大写字母、小写字母与数字
// 返回 null 表示通过，否则返回错误消息字符串
export function validatePasswordStrength(pwd) {
  if (!pwd || pwd.length < 8) {
    return '密码长度不能少于8位'
  }
  let hasUpper = false
  let hasLower = false
  let hasDigit = false
  for (let i = 0; i < pwd.length; i++) {
    const c = pwd.charCodeAt(i)
    if (c >= 65 && c <= 90) hasUpper = true
    else if (c >= 97 && c <= 122) hasLower = true
    else if (c >= 48 && c <= 57) hasDigit = true
  }
  if (!hasUpper || !hasLower || !hasDigit) {
    return '密码必须包含大写字母、小写字母和数字'
  }
  return null
}
