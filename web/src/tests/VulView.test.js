import { describe, test, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import zhCN from '@/i18n/locales/zh-CN.json'
import enUS from '@/i18n/locales/en-US.json'
import VulView from '@/components/asset/VulView.vue'

const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN, 'en-US': enUS } })

const NEW_KEYS = ['filterAll', 'filterNew', 'filterFixed', 'filterPending', 'firstSeen']

function mounted(props = {}) {
  return mount(VulView, {
    props,
    global: {
      plugins: [i18n, ElementPlus],
      stubs: {
        // 避免 ProTable 发起真实请求
        ProTable: { template: '<div class="pro-table-stub"></div>' },
        'router-link': { template: '<a class="ql"><slot /></a>' }
      }
    }
  })
}

describe('VulView 快速筛选与列 (T4.3)', () => {
  test('i18n 双语 parity：新增筛选/列 key 齐全', () => {
    for (const k of NEW_KEYS) {
      expect(zhCN.vul[k], `zh vul.${k}`).toBeTruthy()
      expect(enUS.vul[k], `en vul.${k}`).toBeTruthy()
    }
  })

  test('默认筛选仅下发 sort=severity（不破坏敏感信息页固定过滤）', () => {
    const wrapper = mounted({ extraParams: { isRisk: true, riskSource: 'auto:info-leak' } })
    expect(wrapper.vm.activeFilter).toBe('all')
    expect(wrapper.vm.mergedExtraParams).toMatchObject({
      isRisk: true,
      riskSource: 'auto:info-leak',
      sort: 'severity'
    })
    expect(wrapper.vm.mergedExtraParams.isNew).toBeUndefined()
  })

  test('🆕 新发现 tab → isNew=true（与 dashboard riskNewInWindow 口径一致）', async () => {
    const wrapper = mounted()
    wrapper.vm.activeFilter = 'new'
    await flushPromises()
    expect(wrapper.vm.mergedExtraParams.isNew).toBe(true)
    expect(wrapper.vm.mergedExtraParams.sort).toBe('severity')
  })

  test('Critical / High tab → severity 过滤', async () => {
    const wrapper = mounted()
    wrapper.vm.activeFilter = 'critical'
    await flushPromises()
    expect(wrapper.vm.mergedExtraParams.severity).toBe('critical')
    wrapper.vm.activeFilter = 'high'
    await flushPromises()
    expect(wrapper.vm.mergedExtraParams.severity).toBe('high')
  })

  test('已修复 / 待确认 tab → status / verifyPending', async () => {
    const wrapper = mounted()
    wrapper.vm.activeFilter = 'fixed'
    await flushPromises()
    expect(wrapper.vm.mergedExtraParams.status).toBe('fixed')
    wrapper.vm.activeFilter = 'pending'
    await flushPromises()
    expect(wrapper.vm.mergedExtraParams.verifyPending).toBe(true)
  })

  test('isNewlyFound：首见时间在 7 天窗口内判定为新发现', () => {
    const wrapper = mounted()
    const now = new Date()
    const within = new Date(now.getTime() - 3 * 24 * 3600 * 1000)
    const old = new Date(now.getTime() - 30 * 24 * 3600 * 1000)
    const fmt = (d) => d.toISOString().slice(0, 19).replace('T', ' ')
    expect(wrapper.vm.isNewlyFound({ firstSeenTime: fmt(within) })).toBe(true)
    expect(wrapper.vm.isNewlyFound({ firstSeenTime: fmt(old) })).toBe(false)
    expect(wrapper.vm.isNewlyFound({})).toBe(false)
    expect(wrapper.vm.isNewlyFound({ firstSeenTime: '' })).toBe(false)
  })

  test('首次发现列存在于列定义中', () => {
    const wrapper = mounted()
    const labels = wrapper.vm.vulColumns.map((c) => c.label)
    expect(labels).toContain('首次发现')
  })
})
