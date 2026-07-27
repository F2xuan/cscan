<template>
  <div class="asset-management">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('asset.overviewTitle') }}</h1>
        <p class="description">
          {{ $t('asset.pageDescription') }}
        </p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="handleStartScan">
          <el-icon><Search /></el-icon>
          {{ $t('asset.startScan') }}
        </el-button>
        <el-button @click="openAddDialog">
          <el-icon><Plus /></el-icon>
          {{ $t('asset.manualAddAsset') }}
        </el-button>
      </div>
    </div>

    <!-- 顶层资产列表 -->
    <AssetGroupsTab ref="assetGroupsRef" />

    <!-- 手动添加资产 dialog -->
    <el-dialog
      v-model="addDialogVisible"
      :title="$t('asset.manualAddAssetTitle')"
      width="640px"
      :close-on-click-modal="false"
    >
      <div class="add-batch-hint">{{ $t('asset.addBatchHint') }}</div>
      <el-input
        v-model="addForm.targets"
        type="textarea"
        :rows="10"
        :placeholder="$t('asset.addBatchPlaceholder')"
      />
      <div v-if="addErrors.length" class="add-errors">
        <div
          v-for="(e, i) in addErrors"
          :key="i"
          class="add-error-line"
        >
          {{ $t('asset.addBatchLine', { line: e.line, target: e.target, msg: e.message }) }}
        </div>
      </div>
      <template #footer>
        <el-button @click="addDialogVisible = false">
          {{ $t('asset.addAssetCancel') }}
        </el-button>
        <el-button
          type="primary"
          :loading="addSubmitting"
          @click="handleAddSubmit"
        >
          {{ $t('asset.addAssetConfirm') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, defineAsyncComponent } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Search, Plus } from '@element-plus/icons-vue'
import { importAssets } from '@/api/asset'
import { validateTargets } from '@/utils/target'

const AssetGroupsTab = defineAsyncComponent(() =>
  import('./AssetManagement/AssetGroupsTab.vue')
)

const assetGroupsRef = ref(null)

const { t } = useI18n()
const router = useRouter()

const handleStartScan = () => {
  router.push('/task/create')
}

// 手动添加资产（批量粘贴）
const addDialogVisible = ref(false)
const addSubmitting = ref(false)
const addForm = reactive({ targets: '' })
const addErrors = ref([])

const openAddDialog = () => {
  addForm.targets = ''
  addErrors.value = []
  addDialogVisible.value = true
}

const handleAddSubmit = async () => {
  const errors = validateTargets(addForm.targets)
  addErrors.value = errors
  if (errors.length) return

  const lines = addForm.targets
    .split('\n')
    .map(s => s.trim())
    .filter(s => s && !s.startsWith('#'))
  if (!lines.length) {
    ElMessage.warning(t('asset.addBatchEmptyTip'))
    return
  }

  addSubmitting.value = true
  try {
    const res = await importAssets({ targets: lines })
    if (res.code === 0) {
      ElMessage.success(res.msg || t('asset.addAssetSuccess'))
      addDialogVisible.value = false
      addForm.targets = ''
      addErrors.value = []
      assetGroupsRef.value?.refreshData?.()
    } else {
      ElMessage.error(res.msg || t('asset.addAssetFailed'))
    }
  } catch (e) {
    // axios 拦截器已统一提示
  } finally {
    addSubmitting.value = false
  }
}
</script>

<style lang="scss" scoped>
.asset-management {
  padding: 24px;
  background: hsl(var(--background));
  min-height: 100vh;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
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

  .header-actions {
    display: flex;
    gap: 12px;
  }
}

.add-batch-hint {
  margin-bottom: 12px;
  font-size: 13px;
  color: hsl(var(--muted-foreground));
  line-height: 1.6;
}

.add-errors {
  margin-top: 12px;
  max-height: 160px;
  overflow-y: auto;
  padding: 8px 12px;
  border-radius: 6px;
  background: hsl(var(--destructive) / 0.08);
  border: 1px solid hsl(var(--destructive) / 0.3);
  font-size: 12px;
  color: hsl(var(--destructive));
}

.add-error-line {
  line-height: 1.5;
  word-break: break-all;
}
</style>
