import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import enUs from 'element-plus/dist/locale/en.mjs'
import 'element-plus/dist/index.css'
// 引入 Element Plus 官方暗黑模式样式
import 'element-plus/theme-chalk/dark/css-vars.css'

import App from './App.vue'
import router from './router'
import { setupI18n, i18n } from './i18n'

import './styles/index.css'

// 性能监控（仅开发环境）
import { enablePerformanceMonitoring, setupRouterPerformance } from './utils/performance'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

// 设置国际化
setupI18n(app)

// 根据当前语言设置 Element Plus 语言（初始值）
const currentLocale = i18n.global.locale.value
const elementLocale = currentLocale === 'zh-CN' ? zhCn : enUs
app.use(ElementPlus, { locale: elementLocale })

// Element Plus 语言包的运行时切换由 App.vue 中的 <el-config-provider> 负责，
// 它会根据 i18n.global.locale 动态切换，无需刷新页面。

// 初始化主题
import { useThemeStore } from './stores/theme'
const themeStore = useThemeStore()
themeStore.initTheme()
themeStore.watchSystemTheme()

// 初始化品牌配置（Logo / 标题）
// BUG-001 修复：仅在已登录状态下才加载品牌配置，避免未登录时发送 API 请求
import { useBrandingStore } from './stores/branding'
import { useUserStore } from './stores/user'
const brandingStore = useBrandingStore()
const userStore = useUserStore()
if (userStore.token) {
  brandingStore.load().then(() => {
    if (brandingStore.displayTitle) document.title = brandingStore.displayTitle
  })
} else {
  // 未登录时使用默认标题
  document.title = 'CSCAN'
}

// 启用性能监控（仅开发环境）
if (import.meta.env.DEV) {
  enablePerformanceMonitoring()
  setupRouterPerformance(router)
}

app.mount('#app')

