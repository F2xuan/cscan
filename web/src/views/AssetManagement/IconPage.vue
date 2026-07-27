<template>
  <div class="icon-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.iconView.title') }}</h1>
        <p class="description">{{ $t('asset.iconView.description') }}</p>
      </div>
    </div>
    <IconView />
  </div>
</template>

<script setup>
import IconView from '@/components/asset/IconView.vue'
</script>

<style scoped>
.icon-page {
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
