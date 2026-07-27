// Vitest 全局钩子：当前仅做每个用例后的 mock 复原
import { afterEach, vi } from 'vitest'

afterEach(() => {
  vi.restoreAllMocks()
})
