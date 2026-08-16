import { describe, test, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ElementPlus, { ElTable } from 'element-plus'
import zhCN from '@/i18n/locales/zh-CN.json'
import enUS from '@/i18n/locales/en-US.json'
import AssetInventoryCardView from '@/components/asset/AssetInventoryCardView.vue'

const h = vi.hoisted(() => ({
  calls: { targetList: 0 },
  row: {
    id: 'domain:example.com',
    targetType: 'domain',
    targetValue: 'example.com',
    labels: ['重保', '客户A'],
    memo: '核心资产，变更需审批',
    colorTag: '#f56c6c',
    scanStatus: 'completed',
    totalAssetServices: 3,
    lastScanTime: Date.now(),
  },
  resp: { code: 0, msg: 'success', total: 1 },
}))

vi.mock('@/api/asset', () => ({
  getAssetTargetList: vi.fn(() => {
    h.calls.targetList++
    return Promise.resolve({ ...h.resp, list: [h.row] })
  }),
  deleteAssetTarget: vi.fn(() => Promise.resolve({ code: 0 })),
}))

const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN, 'en-US': enUS } })

function mounted() {
  return mount(AssetInventoryCardView, {
    global: { plugins: [i18n, ElementPlus] },
  })
}

describe('AssetInventoryCardView 目标列表', () => {
  beforeEach(() => {
    h.calls.targetList = 0
  })

  test('挂载即拉取目标列表并渲染目标行', async () => {
    const wrapper = mounted()
    await flushPromises()

    expect(h.calls.targetList).toBeGreaterThanOrEqual(1)
    expect(wrapper.text()).toContain('example.com')
    wrapper.unmount()
  })

  test('目标行渲染标签/备注/颜色标记', async () => {
    const wrapper = mounted()
    await flushPromises()

    const labels = wrapper.findAll('.row-label-tag').map(n => n.text())
    expect(labels).toEqual(expect.arrayContaining(['重保', '客户A']))
    expect(wrapper.find('.target-memo').text()).toContain('核心资产')
    expect(wrapper.find('.color-dot').attributes('style')).toContain('#f56c6c')
    wrapper.unmount()
  })

  test('行点击 emit view-target 携带目标 ID', async () => {
    const wrapper = mounted()
    await flushPromises()

    wrapper.findComponent(ElTable).vm.$emit('row-click', h.row)
    expect(wrapper.emitted('view-target')).toEqual([['domain:example.com']])
    wrapper.unmount()
  })

  test('画笔按钮 emit edit-target 且不触发行跳转', async () => {
    const wrapper = mounted()
    await flushPromises()

    await wrapper.find('.edit-icon').trigger('click')
    expect(wrapper.emitted('edit-target')).toEqual([['domain:example.com']])
    expect(wrapper.emitted('view-target')).toBeUndefined()
    wrapper.unmount()
  })
})
