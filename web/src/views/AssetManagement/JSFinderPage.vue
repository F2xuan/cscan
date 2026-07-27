<template>
  <div class="jsfinder-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.jsView.title') }}</h1>
        <p class="description">{{ $t('asset.jsView.description') }}</p>
      </div>
    </div>
    <JSFinderView />
  </div>
</template>

<script setup>
import JSFinderView from '@/components/asset/JSFinderView.vue'
</script>

<style scoped>
.jsfinder-page {
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
