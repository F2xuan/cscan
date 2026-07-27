<template>
  <div class="sensitive-dir-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.sensitiveDir.title') }}</h1>
        <p class="description">{{ $t('asset.sensitiveDir.description') }}</p>
      </div>
    </div>
    <VulView :extra-params="filterParams" />
  </div>
</template>

<script setup>
import VulView from '@/components/asset/VulView.vue'

// Phase 5：与 assettarget_risk_keywords.go sensitiveDirKeywords 对齐。
const filterParams = {
  isRisk: true,
  riskSource: 'auto:info-leak',
  keywordAny: [
    '敏感目录', '敏感文件', 'dir-listing', 'directory listing', 'dir listing',
    '_backup', 'backup', '.git', '.svn', '.env', 'dump', 'exposed'
  ]
}
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
