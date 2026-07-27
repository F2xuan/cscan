<template>
  <div class="app-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.appView.title') }}</h1>
        <p class="description">{{ $t('asset.appView.description') }}</p>
      </div>
    </div>
    <AppView />
  </div>
</template>

<script setup>
import AppView from '@/components/asset/AppView.vue'
</script>

<style scoped>
.app-page {
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
