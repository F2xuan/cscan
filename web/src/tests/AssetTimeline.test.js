import { describe, test, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import zhCN from '@/i18n/locales/zh-CN.json'
import enUS from '@/i18n/locales/en-US.json'
import AssetTimeline from '@/components/asset/AssetTimeline.vue'

const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN, 'en-US': enUS } })

// 受控 API 响应（vi.hoisted 确保 mock 工厂可引用同一可变状态）
const h = vi.hoisted(() => ({
  state: {
    history: { code: 0, list: [] },
    vul: { code: 0, list: [] },
    rejectHistory: false
  }
}))

vi.mock('@/api/request', () => ({
  default: {
    post: vi.fn((url) => {
      if (url === '/vul/list') return Promise.resolve(h.state.vul)
      return Promise.resolve({ code: 0 })
    })
  }
}))

vi.mock('@/api/asset', () => ({
  getAssetChangeHistory: vi.fn(() => {
    if (h.state.rejectHistory) return Promise.reject(new Error('network'))
    return Promise.resolve(h.state.history)
  })
}))

function mounted(props = {}) {
  return mount(AssetTimeline, {
    props: { assetId: 'a1', host: 'example.com', port: 443, ...props },
    global: {
      plugins: [i18n, ElementPlus]
    }
  })
}

const TL_KEYS = [
  'tab', 'loading', 'loadError', 'retry', 'empty',
  'firstFound', 'propertyChange', 'vulnFound', 'vulnFixed'
]

describe('AssetTimeline 变化时间线 (T4.3)', () => {
  beforeEach(() => {
    h.state.history = { code: 0, list: [] }
    h.state.vul = { code: 0, list: [] }
    h.state.rejectHistory = false
  })

  test('i18n 双语 parity：timeline 命名空间 key 齐全', () => {
    for (const k of TL_KEYS) {
      expect(zhCN.asset.timeline[k], `zh asset.timeline.${k}`).toBeTruthy()
      expect(enUS.asset.timeline[k], `en asset.timeline.${k}`).toBeTruthy()
    }
    // field 别名至少含 propertyChange 涉及的字段
    for (const f of ['title', 'server', 'service', 'status', 'banner']) {
      expect(zhCN.asset.timeline.field[f], `zh asset.timeline.field.${f}`).toBeTruthy()
    }
  })

  test('四类事件分类：first_found / property_change / vuln_found / vuln_fixed', async () => {
    h.state.history = {
      code: 0,
      list: [
        // 无 changes → first_found
        { id: 'h1', createTime: '2026-07-29 10:00:00', taskId: 't1' },
        // 有 changes → property_change
        {
          id: 'h2',
          createTime: '2026-07-28 09:00:00',
          taskId: 't2',
          changes: [{ field: 'title', oldValue: 'Old', newValue: 'New' }]
        }
      ]
    }
    h.state.vul = {
      code: 0,
      list: [
        // open 漏洞 → vuln_found
        { id: 'v1', vulName: 'XSS', severity: 'high', firstSeenTime: '2026-07-27 08:00:00', authority: 'example.com', status: 'open' },
        // fixed 且有 fixedAt → vuln_found + vuln_fixed
        { id: 'v2', vulName: 'SQLi', severity: 'critical', firstSeenTime: '2026-07-26 07:00:00', authority: 'example.com', status: 'fixed', fixedAt: '2026-07-26 12:00:00' }
      ]
    }
    const wrapper = mounted()
    await flushPromises()
    await flushPromises() // 两次：onMounted 触发 + Promise.all resolve

    const events = wrapper.vm.events
    const types = events.map((e) => e.type)
    expect(types).toContain('first_found')
    expect(types).toContain('property_change')
    expect(types).toContain('vuln_found')
    expect(types).toContain('vuln_fixed')
    // 历史 2 条（first_found + property_change）+ 漏洞 3 条（v1 vuln_found + v2 vuln_found + v2 vuln_fixed）= 5
    expect(events.length).toBe(5)
    // 渲染节点数 == 事件数
    expect(wrapper.findAll('.tl-event').length).toBe(5)
  })

  test('属性变化渲染 old → new（标题：Old → New）', async () => {
    h.state.history = {
      code: 0,
      list: [
        {
          id: 'h2',
          createTime: '2026-07-28 09:00:00',
          taskId: 't2',
          changes: [{ field: 'title', oldValue: 'Old', newValue: 'New' }]
        }
      ]
    }
    const wrapper = mounted()
    await flushPromises()
    await flushPromises()

    const changeRows = wrapper.findAll('.tl-change-row')
    expect(changeRows.length).toBe(1)
    // field 经 i18n 映射为“标题”
    expect(wrapper.find('.tl-field').text()).toBe('标题')
    expect(wrapper.find('.tl-old').text()).toBe('Old')
    expect(wrapper.find('.tl-new').text()).toBe('New')
  })

  test('空状态：无历史且无误检漏洞时显示 el-empty（不报错）', async () => {
    h.state.history = { code: 0, list: [] }
    h.state.vul = { code: 0, list: [] }
    const wrapper = mounted()
    await flushPromises()
    await flushPromises()
    expect(wrapper.findAll('.tl-event').length).toBe(0)
    expect(wrapper.find('.el-empty').exists()).toBe(true)
    expect(wrapper.text()).toContain('暂无变化记录')
  })

  test('错误状态：历史接口失败展示重试按钮', async () => {
    h.state.rejectHistory = true
    const wrapper = mounted()
    await flushPromises()
    await flushPromises()
    expect(wrapper.vm.error).toBe(true)
    expect(wrapper.find('.tl-state.tl-error').exists()).toBe(true)
    expect(wrapper.text()).toContain('重试')
  })

  test('按时间倒序：较新事件排在前面', async () => {
    h.state.history = {
      code: 0,
      list: [
        // 较早的 first_found
        { id: 'h1', createTime: '2026-07-20 10:00:00' },
        // 较晚的 property_change
        {
          id: 'h2',
          createTime: '2026-07-29 10:00:00',
          changes: [{ field: 'server', oldValue: 'a', newValue: 'b' }]
        }
      ]
    }
    const wrapper = mounted()
    await flushPromises()
    await flushPromises()
    const times = wrapper.vm.events.map((e) => e.time)
    const parsed = times.map((s) => new Date(String(s).replace(' ', 'T')).getTime())
    const sortedDesc = [...parsed].sort((a, b) => b - a)
    expect(parsed).toEqual(sortedDesc)
  })
})
