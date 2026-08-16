// Vitest 全局钩子：当前仅做每个用例后的 mock 复原
import { afterEach, vi } from 'vitest'

// Element Plus 的 el-table 依赖 ResizeObserver 计算布局，happy-dom 未内置，提供 no-op 兜底
class ResizeObserverPolyfill {
  observe() {}
  unobserve() {}
  disconnect() {}
}
if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = ResizeObserverPolyfill
}

// el-table 带 row-key 时在 onMounted 中 new MutationObserver 观察 .hidden-columns 做列序重排。
// happy-dom 的 MutationObserver 用 #私有字段存状态，经 vitest 环境代理调用 observe 时
// 私有字段不可读（Cannot read private member #destroyed）导致挂载崩溃；测试无真实
// DOM 变更流诉求，提供 no-op 桩兜底（业务代码不直接使用 MutationObserver）。
class MutationObserverPolyfill {
  constructor(callback) {
    this.callback = callback
  }
  observe() {}
  disconnect() {}
  takeRecords() {
    return []
  }
}
globalThis.MutationObserver = MutationObserverPolyfill

afterEach(() => {
  vi.restoreAllMocks()
})
