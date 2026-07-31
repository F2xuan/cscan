import { describe, test, expect } from 'vitest'
import { shouldShowOnboarding } from '@/utils/onboarding'

// 覆盖验收标准：完成/跳过引导后不再弹出（刷新、重登均不再出现）
describe('shouldShowOnboarding', () => {
  test('shows when status missing', () => {
    expect(shouldShowOnboarding(null)).toBe(true)
    expect(shouldShowOnboarding(undefined)).toBe(true)
  })
  test('shows when done is false (brand-new user)', () => {
    expect(shouldShowOnboarding({ code: 0, done: false })).toBe(true)
  })
  test('does not show when done is true (completed or skipped)', () => {
    expect(shouldShowOnboarding({ code: 0, done: true })).toBe(false)
  })
})
