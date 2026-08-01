<template>
  <el-card class="quick-scan-card" shadow="never">
    <div class="qs-inner">
      <div class="qs-left">
        <h2 class="qs-title">{{ t('task.quickScanTitle') }}</h2>
        <p class="qs-sub">{{ t('task.quickScanSubtitle') }}</p>
      </div>
      <div class="qs-main">
        <el-input
          v-model="targets"
          type="textarea"
          :rows="2"
          :placeholder="t('task.quickScanPlaceholder')"
          :disabled="loading"
          @keyup.ctrl.enter="onScan"
        />
        <p v-if="targetErrorMsg" class="qs-error">{{ targetErrorMsg }}</p>
        <div class="qs-actions">
          <el-radio-group v-model="mode">
            <el-radio-button value="quick">{{ t('task.quickScanQuick') }}</el-radio-button>
            <el-radio-button value="full">{{ t('task.quickScanFull') }}</el-radio-button>
          </el-radio-group>
          <el-button type="primary" :loading="loading" @click="onScan">
            {{ t('task.quickScanStart') }}
          </el-button>
          <router-link to="/task/create" class="qs-advanced">{{ t('task.quickScanAdvanced') }}</router-link>
        </div>
      </div>
      <div v-if="result" class="qs-result">
        <el-tag type="success" effect="dark">
          {{ t('task.quickScanRecommended') }}: {{ typeName(result.recommendedType) }}
        </el-tag>
        <span class="qs-est">{{ t('task.quickScanEstimated') }} {{ result.estimatedMinutes }} {{ t('task.quickScanMinutes') }}</span>
      </div>
    </div>
  </el-card>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { quickCreateTask } from '@/api/task'
import { splitTargets } from '@/utils/quickScan'
import { validateTargets, formatValidationErrors } from '@/utils/target'

const router = useRouter()
const { t } = useI18n()

const targets = ref('')
const mode = ref('quick')
const loading = ref(false)
const result = ref(null)

const targetErrors = computed(() => validateTargets(targets.value))
const targetErrorMsg = computed(() => targetErrors.value.length ? formatValidationErrors(targetErrors.value) : '')

async function onScan() {
  if (!splitTargets(targets.value).length) {
    ElMessage.warning(t('task.quickScanInvalid'))
    return
  }
  if (targetErrors.value.length > 0) {
    ElMessage.warning(targetErrorMsg.value)
    return
  }
  loading.value = true
  result.value = null
  try {
    const res = await quickCreateTask({ targets: splitTargets(targets.value).join('\n'), mode: mode.value })
    if (res.code === 0) {
      result.value = res
      ElMessage.success(t('task.quickScanSuccess'))
      const taskId = res.taskId
      setTimeout(() => {
        if (taskId) router.push(`/task/detail?id=${taskId}`)
      }, 1200)
    } else {
      ElMessage.error(res.msg || t('task.quickScanFailed'))
    }
  } catch (e) {
    ElMessage.error((e && e.message) || t('task.quickScanFailed'))
  } finally {
    loading.value = false
  }
}

function typeName(tn) {
  if (tn === 'port') return t('task.quickScanTypePort')
  if (tn === 'web') return t('task.quickScanTypeWeb')
  return t('task.quickScanTypeDomain')
}
</script>

<style scoped lang="scss">
.quick-scan-card {
  margin-bottom: 16px;

  .qs-inner {
    display: flex;
    align-items: center;
    gap: 24px;
    flex-wrap: wrap;
  }

  .qs-left {
    min-width: 150px;

    .qs-title {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
    }

    .qs-sub {
      margin: 4px 0 0;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .qs-main {
    flex: 1;
    min-width: 320px;
  }

  .qs-error {
    font-size: 12px;
    color: var(--el-color-error, #f56c6c);
    margin: 6px 0 0;
    line-height: 1.5;
    white-space: pre-line;
  }

  .qs-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 10px;
    flex-wrap: wrap;
  }

  .qs-advanced {
    font-size: 13px;
    color: var(--el-color-primary);
    text-decoration: none;
  }

  .qs-result {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 10px;
  }

  .qs-est {
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }
}
</style>
