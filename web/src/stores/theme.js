import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import request from '@/api/request'
import { useUserStore } from '@/stores/user'

// 可用的款式列表（设计语言风格）— 10 个设计系统
export const THEME_STYLES = [
  { value: 'vercel', label: 'theme.styleVercel', description: 'theme.styleVercelDesc', primaryColor: '#121212', darkPrimaryColor: '#e5e5e5' },
  { value: 'apple', label: 'theme.styleApple', description: 'theme.styleAppleDesc', primaryColor: '#007aff', darkPrimaryColor: '#2e8dff' },
  { value: '21th', label: 'theme.style21th', description: 'theme.style21thDesc', primaryColor: '#111111', darkPrimaryColor: '#ffffff' },
  { value: 'claude', label: 'theme.styleClaude', description: 'theme.styleClaudeDesc', primaryColor: '#c96442', darkPrimaryColor: '#d97757' },
  { value: 'google', label: 'theme.styleGoogle', description: 'theme.styleGoogleDesc', primaryColor: '#4285f4', darkPrimaryColor: '#fc2c50' },
  { value: 'minimal', label: 'theme.styleMinimal', description: 'theme.styleMinimalDesc', primaryColor: '#18181b', darkPrimaryColor: '#fafafa' },
  { value: 'motionfit', label: 'theme.styleMotionfit', description: 'theme.styleMotionfitDesc', primaryColor: '#ff4000', darkPrimaryColor: '#00ff85' },
  { value: 'nerv', label: 'theme.styleNerv', description: 'theme.styleNervDesc', primaryColor: '#ea343a', darkPrimaryColor: '#ff99cc' },
  { value: 'tiktok', label: 'theme.styleTiktok', description: 'theme.styleTiktokDesc', primaryColor: '#fe2c55', darkPrimaryColor: '#fe2c55' },
  { value: 'yuanli', label: 'theme.styleYuanli', description: 'theme.styleYuanliDesc', primaryColor: '#1664ff', darkPrimaryColor: '#387bff' },
]

export const useThemeStore = defineStore('theme', () => {
  // 主题模式（亮色/暗色/跟随系统）
  const theme = ref('system')
  // 款式（设计语言风格）
  const themeStyle = ref('vercel')
  // 是否为暗色模式
  const isDark = ref(false)
  // 是否已从服务端加载
  const loaded = ref(false)

  // 从服务端加载主题配置
  async function loadFromServer() {
    // BUG-001 修复：仅在已登录状态下才加载服务端主题配置
    const userStore = useUserStore()
    if (!userStore.token) {
      return
    }

    try {
      const res = await request.post('/theme/config/get')
      if (res.code === 0 && res.config) {
        theme.value = res.config.theme || 'system'
        // 仅在服务端返回了 themeStyle 时才覆盖，避免旧数据导致款式丢失
        if (res.config.themeStyle) {
          themeStyle.value = res.config.themeStyle
        }
        loaded.value = true
      }
    } catch (e) {
      console.error('Failed to load theme config:', e)
      // 加载失败时保留 localStorage 的值（initTheme 中已设置）
    }
  }

  // 保存到服务端
  async function saveToServer() {
    try {
      await request.post('/theme/config/save', {
        theme: theme.value,
        themeStyle: themeStyle.value
      })
    } catch (e) {
      console.error('Failed to save theme config:', e)
    }
  }

  // 初始化主题
  async function initTheme() {
    // 1. 先从 localStorage 同步加载（避免刷新时闪烁/丢失款式）
    const localTheme = localStorage.getItem('theme')
    const localStyle = localStorage.getItem('themeStyle')
    if (localTheme) theme.value = localTheme
    if (localStyle) themeStyle.value = localStyle
    // 立即应用一次，确保页面渲染时就有正确的款式
    updateTheme()

    // 2. 再从服务端加载（覆盖本地，确保多端同步）
    await loadFromServer()
    updateTheme()
  }

  // 更新主题
  function updateTheme() {
    const root = document.documentElement

    // 移除所有主题类
    root.classList.remove('light', 'dark')
    // 移除旧的颜色主题类（兼容性清理）
    const oldColorThemes = ['default', 'pure-white', 'forest-green', 'ocean-blue', 'sunset-orange',
      'royal-purple', 'cherry-blossom', 'midnight-teal', 'quantum-rose', 'vercel', 'clean-slate',
      'cosmic-night', 'vercel-dark']
    oldColorThemes.forEach(t => {
      root.classList.remove(`theme-${t}`)
    })
    // 移除所有款式类
    THEME_STYLES.forEach(s => {
      root.classList.remove(`style-${s.value}`)
    })

    // 确定是否使用暗色模式
    let shouldBeDark = false

    if (theme.value === 'dark') {
      shouldBeDark = true
    } else if (theme.value === 'system') {
      shouldBeDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    }

    isDark.value = shouldBeDark

    // 应用主题类
    root.classList.add(shouldBeDark ? 'dark' : 'light')

    // 应用款式类
    root.classList.add(`style-${themeStyle.value}`)

    // 同时保存到本地存储（作为备份）
    localStorage.setItem('theme', theme.value)
    localStorage.setItem('themeStyle', themeStyle.value)
  }

  // 切换主题模式
  function setTheme(newTheme) {
    theme.value = newTheme
    updateTheme()
    saveToServer()
  }

  // 切换款式
  function setThemeStyle(newStyle) {
    themeStyle.value = newStyle
    updateTheme()
    saveToServer()
  }

  // 切换暗色模式
  function toggleTheme() {
    if (theme.value === 'light') {
      setTheme('dark')
    } else if (theme.value === 'dark') {
      setTheme('light')
    } else {
      // 如果是 system，则切换到相反的模式
      const systemIsDark = window.matchMedia('(prefers-color-scheme: dark)').matches
      setTheme(systemIsDark ? 'light' : 'dark')
    }
  }

  // 监听系统主题变化
  function watchSystemTheme() {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

    const handleChange = () => {
      if (theme.value === 'system') {
        updateTheme()
      }
    }

    mediaQuery.addEventListener('change', handleChange)

    return () => {
      mediaQuery.removeEventListener('change', handleChange)
    }
  }

  // 监听主题变化
  watch([theme, themeStyle], () => {
    updateTheme()
  })

  return {
    theme,
    themeStyle,
    isDark,
    loaded,
    initTheme,
    loadFromServer,
    setTheme,
    setThemeStyle,
    toggleTheme,
    watchSystemTheme,
    THEME_STYLES,
  }
})
