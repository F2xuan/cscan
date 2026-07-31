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

afterEach(() => {
  vi.restoreAllMocks()
})
