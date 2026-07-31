// 引导式首次体验（T4.2）：根据后端状态判断是否需要弹出引导
// 后端 UserOnboardingStatusResp: { code, msg, done }
export function shouldShowOnboarding(status) {
  if (!status) return true
  return status.done !== true
}
