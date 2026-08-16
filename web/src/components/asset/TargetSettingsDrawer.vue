<template>
  <el-drawer
    v-model="visible"
    :title="meta?.targetValue || ''"
    size="420px"
    class="target-settings-drawer"
  >
    <div class="settings-body">
      <!-- 标签/备注/颜色（复用已有 /asset/target/update 能力，替代 open-asm 的 Scan Schedule 区块） -->
      <div class="settings-section">
        <div class="section-title">{{ $t('asset.targetView.settings') }}</div>

        <div class="form-row">
          <label>{{ $t('asset.targetView.labels') }}</label>
          <el-select
            v-model="form.labels"
            multiple
            filterable
            allow-create
            default-first-option
            :reserve-keyword="false"
            class="form-input"
            :placeholder="$t('asset.targetView.labels')"
          />
        </div>

        <div class="form-row">
          <label>{{ $t('asset.targetView.memo') }}</label>
          <el-input
            v-model="form.memo"
            type="textarea"
            :rows="3"
            class="form-input"
          />
        </div>

        <div class="form-row">
          <label>{{ $t('asset.targetView.colorTag') }}</label>
          <el-color-picker v-model="form.colorTag" class="form-color" />
        </div>

        <el-button
          type="primary"
          :loading="saving"
          class="save-btn"
          @click="handleSave"
        >
          {{ $t('asset.targetView.save') }}
        </el-button>
      </div>

      <!-- Re-discover（布局对齐 open-asm；当前无目标级重扫接口，按钮禁用） -->
      <div class="settings-section">
        <div class="rediscover-row">
          <div class="rediscover-info">
            <div class="rediscover-title">{{ $t('asset.targetView.rediscover') }}</div>
            <div class="rediscover-sub">{{ $t('asset.targetView.rediscoverHint') }}</div>
          </div>
          <el-tooltip
            :content="$t('asset.targetView.rediscoverDisabled')"
            placement="top"
          >
            <span>
              <el-button disabled>
                <el-icon><Refresh /></el-icon>
              </el-button>
            </span>
          </el-tooltip>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="drawer-footer">
        <el-button
          type="danger"
          plain
          :disabled="!deleteConfirmed"
          :loading="deleting"
          class="delete-btn"
          @click="handleDelete"
        >
          {{ $t('asset.targetView.deleteTarget') }}
        </el-button>
        <div class="delete-confirm">
          <el-checkbox v-model="deleteWithAssets">
            {{ $t('asset.targetView.deleteWithAssets') }}
          </el-checkbox>
          <el-input
            v-model="deleteText"
            size="small"
            :placeholder="$t('asset.targetView.deleteTypeHint', { name: meta?.targetValue || '' })"
          />
        </div>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { updateAssetTarget, deleteAssetTarget } from '@/api/asset'

const { t } = useI18n()

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  meta: { type: Object, default: null },
})

const emit = defineEmits(['update:modelValue', 'saved', 'deleted'])

const visible = computed({
  get: () => props.modelValue,
  set: v => emit('update:modelValue', v),
})

const form = reactive({ labels: [], memo: '', colorTag: '' })
const saving = ref(false)
const deleting = ref(false)
const deleteText = ref('')
const deleteWithAssets = ref(false)

const deleteConfirmed = computed(() =>
  deleteText.value.trim() !== '' &&
  deleteText.value.trim() === (props.meta?.targetValue || '__none__')
)

watch(() => props.modelValue, (open) => {
  if (open) {
    form.labels = [...(props.meta?.labels || [])]
    form.memo = props.meta?.memo || ''
    form.colorTag = props.meta?.colorTag || ''
    deleteText.value = ''
    deleteWithAssets.value = false
  }
})

async function handleSave() {
  saving.value = true
  try {
    await updateAssetTarget({
      targetId: props.meta?.id,
      labels: form.labels,
      memo: form.memo,
      colorTag: form.colorTag || '',
    })
    ElMessage.success(t('asset.targetView.saved'))
    emit('saved')
  } catch (err) {
    console.error('[TargetSettingsDrawer] save error:', err)
    ElMessage.error(String(err?.message || err))
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  try {
    await ElMessageBox.confirm(
      t('asset.targetView.deleteConfirm', { name: props.meta?.targetValue }),
      t('asset.targetView.deleteTarget'),
      { type: 'warning', confirmButtonText: t('asset.targetView.deleteTarget'), cancelButtonText: t('common.cancel') }
    )
  } catch {
    return
  }
  deleting.value = true
  try {
    await deleteAssetTarget({
      targetId: props.meta?.id,
      deleteAssets: deleteWithAssets.value,
    })
    ElMessage.success(t('asset.targetView.deleteSuccess'))
    visible.value = false
    emit('deleted')
  } catch (err) {
    console.error('[TargetSettingsDrawer] delete error:', err)
    ElMessage.error(t('asset.targetView.deleteFailed'))
  } finally {
    deleting.value = false
  }
}
</script>

<style scoped lang="scss">
.settings-body {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.settings-section {
  .section-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--el-text-color-primary);
    margin-bottom: 12px;
  }
}

.form-row {
  margin-bottom: 12px;

  label {
    display: block;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    margin-bottom: 6px;
  }

  .form-input {
    width: 100%;
  }
}

.save-btn {
  margin-top: 4px;
}

.rediscover-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;

  .rediscover-info {
    .rediscover-title {
      font-size: 13px;
      font-weight: 500;
    }

    .rediscover-sub {
      font-size: 12px;
      color: var(--el-text-color-secondary);
      margin-top: 2px;
    }
  }
}

.drawer-footer {
  display: flex;
  flex-direction: column;
  gap: 10px;

  .delete-btn {
    width: 100%;
  }

  .delete-confirm {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
}
</style>
