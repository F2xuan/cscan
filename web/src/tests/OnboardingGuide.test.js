import { describe, test, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import zhCN from '@/i18n/locales/zh-CN.json'
import enUS from '@/i18n/locales/en-US.json'
import OnboardingGuide from '@/components/OnboardingGuide.vue'

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

const quickCreateTask = vi.fn()
vi.mock('@/api/task', () => ({
  quickCreateTask: (...args) => quickCreateTask(...args)
}))

const completeOnboarding = vi.fn()
vi.mock('@/api/auth', () => ({
  completeOnboarding: (...args) => completeOnboarding(...args)
}))

const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  messages: { 'zh-CN': zhCN, 'en-US': enUS }
})

function mounted() {
  return mount(OnboardingGuide, {
    global: { plugins: [i18n, ElementPlus] },
    attachTo: document.body
  })
}

const OB_KEYS = [
  'title', 'subtitle', 'skip', 'step1', 'step2', 'step3', 'targetLabel',
  'targetPlaceholder', 'targetHint', 'modeLabel', 'modeQuick', 'modeFull',
  'modeQuickHint', 'modeFullHint', 'summaryTarget', 'summaryTargets', 'summaryMode',
  'startHint', 'next', 'prev', 'start', 'invalid', 'success', 'failed', 'skipped',
  'scanGuideBtn'
]

describe('OnboardingGuide i18n parity', () => {
  test('all onboarding keys present in both locales', () => {
    for (const k of OB_KEYS) {
      expect(zhCN.onboarding).toHaveProperty(k)
      expect(enUS.onboarding).toHaveProperty(k)
    }
  })
})

describe('OnboardingGuide render', () => {
  test('renders 3 steps, target input and skip button', () => {
    const wrapper = mounted()
    expect(wrapper.findAll('.el-step').length).toBe(3)
    expect(wrapper.find('.el-textarea__inner').exists()).toBe(true)
    expect(wrapper.find('.og-skip').exists()).toBe(true)
  })
})

describe('OnboardingGuide navigation & start', () => {
  beforeEach(() => {
    push.mockClear()
    quickCreateTask.mockReset()
    completeOnboarding.mockReset()
  })

  test('next disabled when target empty, enabled after input', async () => {
    const wrapper = mounted()
    expect(wrapper.find('.el-button--primary').classes()).toContain('is-disabled')
    await wrapper.find('.el-textarea__inner').setValue('example.com')
    await flushPromises()
    expect(wrapper.find('.el-button--primary').classes()).not.toContain('is-disabled')
  })

  test('full flow creates task, completes onboarding and redirects', async () => {
    quickCreateTask.mockResolvedValue({ code: 0, taskId: 'xyz789' })
    completeOnboarding.mockResolvedValue({ code: 0 })
    const wrapper = mounted()

    // 步骤1：输入目标
    await wrapper.find('.el-textarea__inner').setValue('example.com, 8.8.8.8')
    await wrapper.find('.el-button--primary').trigger('click')
    await flushPromises()
    expect(wrapper.find('.el-radio-group').exists()).toBe(true)

    // 步骤2：选深度模式
    const radios = wrapper.findAll('input[type="radio"]')
    await radios[1].setChecked(true)
    await flushPromises()
    await wrapper.find('.el-button--primary').trigger('click')
    await flushPromises()
    expect(wrapper.find('.og-summary').exists()).toBe(true)

    // 步骤3：开始
    await wrapper.find('.el-button--primary').trigger('click')
    await flushPromises()
    expect(quickCreateTask).toHaveBeenCalledTimes(1)
    const arg = quickCreateTask.mock.calls[0][0]
    expect(arg.targets.split('\n')).toEqual(['example.com', '8.8.8.8'])
    expect(arg.mode).toBe('full')
    expect(completeOnboarding).toHaveBeenCalledTimes(1)

    await new Promise((r) => setTimeout(r, 1400))
    expect(push).toHaveBeenCalledWith('/task/detail?id=xyz789')
    expect(wrapper.emitted('finished')).toBeTruthy()
  })
})

describe('OnboardingGuide skip', () => {
  beforeEach(() => {
    completeOnboarding.mockReset()
    push.mockClear()
  })
  test('skip calls completeOnboarding and emits finished (no redirect)', async () => {
    completeOnboarding.mockResolvedValue({ code: 0 })
    const wrapper = mounted()
    await wrapper.find('.og-skip').trigger('click')
    await flushPromises()
    expect(completeOnboarding).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('finished')).toBeTruthy()
    expect(push).not.toHaveBeenCalled()
  })
})
