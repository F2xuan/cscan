<template>
  <div class="sensitive-info-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.sensitiveInfo.title') }}</h1>
        <p class="description">{{ $t('asset.sensitiveInfo.description') }}</p>
      </div>
    </div>
    <JSFinderView :extra-params="filterParams" mode="sensitive" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import JSFinderView from '@/components/asset/JSFinderView.vue'

const route = useRoute()

// 通过AI研判结果过滤：只显示AI判定为有风险且已完成研判的数据
const filterParams = computed(() => {
  const params = {
    aiResult: 'risk',
    aiStatus: 'completed'
  }
  // 从资产概览跳转时携带 rootDomain 或 ip 参数，用 query 做模糊匹配
  if (route.query.rootDomain) {
    params.query = route.query.rootDomain
  } else if (route.query.ip) {
    params.query = route.query.ip
  }
  return params
})
</script>

<style scoped>
.sensitive-info-page {
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
