<template>
  <el-config-provider :locale="currentElLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { computed } from 'vue'
import { i18n } from './i18n'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import enUs from 'element-plus/dist/locale/en.mjs'

// 根据当前 i18n 语言动态切换 Element Plus 语言包，避免刷新页面
const currentElLocale = computed(() =>
  i18n.global.locale.value === 'zh-CN' ? zhCn : enUs
)
</script>

<style>
html, body, #app {
  height: 100%;
  margin: 0;
  padding: 0;
}

/* 路由懒加载顶部进度条 */
.app-route-loading::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 3px;
  background: linear-gradient(90deg, var(--el-color-primary, #409eff), var(--el-color-primary-light-3, #79bbff), var(--el-color-primary, #409eff));
  background-size: 200% 100%;
  animation: route-loading-slide 1.2s ease-in-out infinite;
  z-index: 99999;
}

@keyframes route-loading-slide {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
</style>

