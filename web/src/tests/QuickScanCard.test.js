import { describe, test, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import zhCN from '@/i18n/locales/zh-CN.json'
import enUS from '@/i18n/locales/en-US.json'
import QuickScanCard from '@/components/QuickScanCard.vue'
import { splitTargets, isValidTargets } from '@/utils/quickScan'

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

const quickCreateTask = vi.fn()
vi.mock('@/api/task', () => ({
  quickCreateTask: (...args) => quickCreateTask(...args)
}))

const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  messages: { 'zh-CN': zhCN, 'en-US': enUS }
})

function mounted() {
  return mount(QuickScanCard, {
    global: {
      plugins: [i18n, ElementPlus],
      stubs: { 'router-link': { template: '<a class="qs-advanced"><slot /></a>' } }
    }
  })
}

const T4_KEYS = [
  'quickScanTitle', 'quickScanSubtitle', 'quickScanPlaceholder', 'quickScanQuick',
  'quickScanFull', 'quickScanStart', 'quickScanAdvanced', 'quickScanRecommended',
  'quickScanTypePort', 'quickScanTypeDomain', 'quickScanTypeWeb', 'quickScanEstimated',
  'quickScanMinutes', 'quickScanSuccess', 'quickScanInvalid', 'quickScanFailed'
]

describe('QuickScan util', () => {
  test('splitTargets handles comma / newline / semicolon and trims', () => {
    expect(splitTargets('example.com, 192.168.1.0/24\nhttps://foo.com; 8.8.8.8')).toEqual([
      'example.com', '192.168.1.0/24', 'https://foo.com', '8.8.8.8'
    ])
  })
  test('splitTargets drops blanks', () => {
    expect(splitTargets('  ,\n, foo.com,')).toEqual(['foo.com'])
  })
  test('isValidTargets false on empty', () => {
    expect(isValidTargets('   ')).toBe(false)
    expect(isValidTargets('')).toBe(false)
    expect(isValidTargets('example.com')).toBe(true)
  })
})

describe('QuickScanCard i18n parity', () => {
  test('all T4.1 keys present in both locales (task namespace)', () => {
    for (const k of T4_KEYS) {
      expect(zhCN.task).toHaveProperty(k)
      expect(enUS.task).toHaveProperty(k)
    }
  })
})

describe('QuickScanCard input parsing & API call', () => {
  beforeEach(() => quickCreateTask.mockClear())

  test('empty input does not call API', async () => {
    const wrapper = mounted()
    await wrapper.find('button.el-button').trigger('click')
    await flushPromises()
    expect(quickCreateTask).not.toHaveBeenCalled()
  })

  test('multi-target normalized to newline-joined list with quick mode', async () => {
    const wrapper = mounted()
    await wrapper.find('.el-textarea__inner').setValue('example.com, 192.168.1.0/24\nhttps://foo.com')
    await wrapper.find('button.el-button').trigger('click')
    await flushPromises()
    expect(quickCreateTask).toHaveBeenCalledTimes(1)
    const arg = quickCreateTask.mock.calls[0][0]
    expect(arg.targets.split('\n')).toEqual(['example.com', '192.168.1.0/24', 'https://foo.com'])
    expect(arg.mode).toBe('quick')
  })

  test('full mode is passed when deep radio selected', async () => {
    const wrapper = mounted()
    // 选择“深度” radio（驱动底层 input，规避 label 点击不冒泡）
    const radios = wrapper.findAll('input[type="radio"]')
    await radios[1].setChecked(true)
    await flushPromises()
    await wrapper.find('.el-textarea__inner').setValue('example.com')
    await wrapper.find('button.el-button').trigger('click')
    await flushPromises()
    expect(quickCreateTask.mock.calls[0][0].mode).toBe('full')
  })
})

describe('QuickScanCard jump on success', () => {
  beforeEach(() => {
    push.mockClear()
    quickCreateTask.mockReset()
  })

  test('redirects to task detail after successful creation', async () => {
    quickCreateTask.mockResolvedValue({
      code: 0,
      taskId: 'abc123',
      recommendedType: 'domain',
      estimatedMinutes: 5
    })
    const wrapper = mounted()
    await wrapper.find('.el-textarea__inner').setValue('example.com')
    await wrapper.find('button.el-button').trigger('click')
    await flushPromises()
    // 等待 onScan 内 setTimeout(1200) 跳转
    await new Promise((r) => setTimeout(r, 1400))
    expect(push).toHaveBeenCalledWith('/task/detail?id=abc123')
  })

  test('shows error and does not jump on failure', async () => {
    quickCreateTask.mockResolvedValue({ code: 500, msg: 'boom' })
    const wrapper = mounted()
    await wrapper.find('.el-textarea__inner').setValue('example.com')
    await wrapper.find('button.el-button').trigger('click')
    await flushPromises()
    await new Promise((r) => setTimeout(r, 1400))
    expect(push).not.toHaveBeenCalled()
  })
})
