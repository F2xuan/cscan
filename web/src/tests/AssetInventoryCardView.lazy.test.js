import { describe, test, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import zhCN from '@/i18n/locales/zh-CN.json'
import enUS from '@/i18n/locales/en-US.json'
import AssetInventoryCardView from '@/components/asset/AssetInventoryCardView.vue'

// 受控 API 调用计数（验证懒加载行为）
const h = vi.hoisted(() => ({
  calls: { inventory: 0, stat: 0, filterOptions: 0, detail: 0 },
  inventory: { code: 0, total: 0, list: [] }
}))

vi.mock('@/api/request', () => ({
  default: { post: vi.fn(() => Promise.resolve({ code: 0 })) }
}))

vi.mock('@/api/asset', () => ({
  getAssetInventory: vi.fn(() => { h.calls.inventory++; return Promise.resolve(h.inventory) }),
  getAssetStat: vi.fn(() => { h.calls.stat++; return Promise.resolve({ code: 0, topPorts: [], topService: [], topApp: [], topIconHash: [] }) }),
  getAssetFilterOptions: vi.fn(() => { h.calls.filterOptions++; return Promise.resolve({ code: 0, technologies: [], ports: [], statusCodes: [], labels: [] }) }),
  getAssetDetail: vi.fn(() => { h.calls.detail++; return Promise.resolve({ code: 0, data: { id: 'a1' } }) }),
  updateAssetLabels: vi.fn(() => Promise.resolve({ code: 0 })),
  deleteAsset: vi.fn(() => Promise.resolve({ code: 0 })),
  getAssetHistory: vi.fn(() => Promise.resolve({ code: 0, list: [] })),
  getAssetExposures: vi.fn(() => Promise.resolve({ code: 0, list: [] })),
  clearAssets: vi.fn(() => Promise.resolve({ code: 0 }))
}))

// 抽屉子组件内部依赖较重，桩化为空渲染以避免引入其 import 链
vi.mock('@/components/asset/AssetDetailDrawer.vue', () => ({
  default: { template: '<div class="drawer-stub"></div>' }
}))

// 组件使用 useRoute/useRouter，提供桩实现（replace/push 需返回 Promise，否则 .catch 报错）
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {}, params: {}, path: '/' }),
  useRouter: () => ({ replace: vi.fn(() => Promise.resolve()), push: vi.fn(() => Promise.resolve()) })
}))

const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN, 'en-US': enUS } })

function mounted() {
  return mount(AssetInventoryCardView, {
    global: { plugins: [i18n, ElementPlus] }
  })
}

describe('AssetInventoryCardView 懒加载与按需详情', () => {
  beforeEach(() => {
    h.calls.inventory = 0
    h.calls.stat = 0
    h.calls.filterOptions = 0
    h.calls.detail = 0
  })

  test('onMounted 不立即加载筛选选项（避免进菜单即触发整表 distinct）', async () => {
    const wrapper = mounted()
    await flushPromises()
    // 让 setTimeout(loadStat, 250) 触发（统计延迟加载不影响筛选断言）
    await new Promise((r) => setTimeout(r, 350))
    await flushPromises()

    expect(h.calls.filterOptions).toBe(0)
    expect(h.calls.inventory).toBeGreaterThan(0) // 列表本身仍应加载
    wrapper.unmount()
  })

  test('首次展开筛选面板才加载筛选选项', async () => {
    const wrapper = mounted()
    await flushPromises()

    wrapper.vm.showFilters = true
    await flushPromises()

    expect(h.calls.filterOptions).toBe(1)
    wrapper.unmount()
  })

  test('再次展开不会重复加载筛选选项（filterOptionsLoaded 守卫）', async () => {
    const wrapper = mounted()
    await flushPromises()
    wrapper.vm.showFilters = true
    await flushPromises()
    wrapper.vm.showFilters = false
    await flushPromises()
    wrapper.vm.showFilters = true
    await flushPromises()

    expect(h.calls.filterOptions).toBe(1)
    wrapper.unmount()
  })

  test('点击卡片调用 getAssetDetail 按需补全详情（列表已瘦身）', async () => {
    const wrapper = mounted()
    await flushPromises()

    await wrapper.vm.handleCardClick({ id: 'a1', workspaceId: 'ws1' })
    await flushPromises()

    expect(h.calls.detail).toBe(1)
    const arg = (await import('@/api/asset')).getAssetDetail.mock.calls[0][0]
    expect(arg).toEqual({ id: 'a1', workspaceId: 'ws1' })
    wrapper.unmount()
  })
})
