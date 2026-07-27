<template>
  <div class="asset-space-search">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('navigation.assetSpaceSearch') }}</h1>
        <p class="description">{{ $t('asset.spaceSearchDescription') }}</p>
      </div>
    </div>
    <AssetInventoryCardView />
  </div>
</template>

<script setup>
import { defineAsyncComponent } from 'vue'

const AssetInventoryCardView = defineAsyncComponent(() =>
  import('@/components/asset/AssetInventoryCardView.vue')
)
</script>

<style lang="scss" scoped>
.asset-space-search {
  padding: 24px;
  background: hsl(var(--background));
  min-height: 100vh;
}

.page-header {
  margin-bottom: 24px;

  .header-content {
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
}
</style>
