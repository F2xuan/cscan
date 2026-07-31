<template>
  <div class="sensitive-dir-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.sensitiveDir.title') }}</h1>
        <p class="description">{{ $t('asset.sensitiveDir.description') }}</p>
      </div>
    </div>
    <DirScanView :extra-params="filterParams" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import DirScanView from '@/components/asset/DirScanView.vue'

const route = useRoute()

// 与后端 assettarget_risk_keywords.go sensitivePathKeywords 对齐
const sensitivePathRegex = '(\\.git|\\.svn|\\.env|backup|dump|config|admin|phpinfo|test|debug|\\.bak)'

const filterParams = computed(() => {
  const params = { path: sensitivePathRegex }
  // 从资产概览跳转时携带 rootDomain 或 ip 参数
  if (route.query.rootDomain) {
    params.authority = route.query.rootDomain
  } else if (route.query.ip) {
    params.authority = route.query.ip
  }
  return params
})
</script>

<style scoped>
.sensitive-dir-page {
  padding: 24px;
  background: hsl(var(--background));
  min-height: 100vh;
}
.page-header {
  margin-bottom: 24px;
  h1 {
    font-size: 28px;
    font-weight: 600;
    color: hsl(var(--foreground));
    margin: 0 0 8px 0;
  }
  .description {
    color: hsl(var(--muted-foreground));
    font-size: 14px;
    margin: 0;
  }
}
</style>
