<template>
  <div class="api-docs-page">
    <el-card shadow="never" class="docs-card">
      <template #header>
        <div class="docs-header">
          <span class="title">{{ $t('navigation.apiDocs') }}</span>
          <span class="subtitle">{{ $t('apiDocs.subtitle', '基于 OpenAPI 3.0 自动生成，涵盖全部 /api/v1/* 端点') }}</span>
        </div>
      </template>
      <div class="docs-frame-wrapper">
        <iframe :src="swaggerUrl" class="docs-frame" :title="$t('navigation.apiDocs')"></iframe>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const swaggerUrl = computed(() => '/swagger-ui')
</script>

<style lang="scss" scoped>
.api-docs-page {
  padding: 0;
}
.docs-card {
  :deep(.el-card__body) {
    padding: 0;
  }
}
.docs-header {
  display: flex;
  align-items: baseline;
  gap: 12px;
  .title { font-size: 18px; font-weight: 600; }
  .subtitle { color: var(--el-text-color-secondary); font-size: 13px; }
}
.docs-frame-wrapper {
  width: 100%;
  height: calc(100vh - 64px - 80px - 40px);
  min-height: 480px;
}
.docs-frame {
  width: 100%;
  height: 100%;
  border: 0;
  display: block;
}
</style>
