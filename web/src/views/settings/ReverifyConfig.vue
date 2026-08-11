<template>
  <div class="reverify-config-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('navigation.reverifyConfig') }}</h1>
        <p class="description">{{ $t('reverify.tip') }}</p>
      </div>
    </div>

    <el-card>
      <el-form label-width="140px" style="max-width: 680px;" v-loading="reverifyConfigLoading">
        <el-form-item :label="$t('reverify.weakPassEnabled')">
          <el-switch v-model="reverifyConfigForm.weakPassEnabled" />
          <span class="ap-hint">{{ $t('reverify.weakPassHint') }}</span>
        </el-form-item>
        <el-form-item :label="$t('reverify.exposureEnabled')">
          <el-switch v-model="reverifyConfigForm.exposureEnabled" />
          <span class="ap-hint">{{ $t('reverify.exposureHint') }}</span>
        </el-form-item>

        <!-- 复验周期 -->
        <el-form-item :label="$t('reverify.cronSpec')">
          <el-input v-model="reverifyConfigForm.cronSpec" :placeholder="$t('reverify.cronPlaceholder')" style="max-width: 320px">
            <template #append>
              <el-button @click="handleReverifyCronValidate">{{ $t('cronTask.validate') || '验证' }}</el-button>
            </template>
          </el-input>
          <div class="cron-help">
            <div class="cron-presets">
              <span class="preset-label">{{ $t('cronTask.quickSelect') || '快捷选择' }}</span>
              <el-tag
                v-for="preset in reverifyCronPresets"
                :key="preset.value"
                size="small"
                class="preset-tag"
                :class="{ 'is-active': reverifyConfigForm.cronSpec === preset.value }"
                @click="reverifyConfigForm.cronSpec = preset.value; handleReverifyCronValidate()"
              >
                {{ preset.label }}
              </el-tag>
            </div>
            <div v-if="reverifyCronValidation.valid" class="cron-next-times">
              <div class="next-label">{{ $t('cronTask.next5Times') || '接下来 5 次执行时间' }}</div>
              <div v-for="(time, index) in reverifyCronValidation.nextTimes" :key="index" class="next-time">
                {{ index + 1 }}. {{ time }}
              </div>
            </div>
            <div v-else-if="reverifyCronValidation.error" class="cron-error">
              {{ reverifyCronValidation.error }}
            </div>
          </div>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="reverifyConfigSubmitting" @click="handleReverifyConfigSubmit">{{ $t('common.save') }}</el-button>
          <el-button type="success" :loading="reverifyRunning" @click="handleReverifyRunNow">{{ $t('reverify.runNow') }}</el-button>
        </el-form-item>
      </el-form>

      <el-divider content-position="left">{{ $t('reverify.statusTitle') }}</el-divider>
      <el-descriptions :column="2" border v-loading="reverifyConfigLoading">
        <el-descriptions-item :label="$t('reverify.lastRunTime')">{{ reverifyStatus.lastRunTime || $t('common.none') }}</el-descriptions-item>
        <el-descriptions-item :label="$t('reverify.lastRunStatus')">
          <el-tag :type="reverifyStatusTagType(reverifyStatus.lastRunStatus)">{{ reverifyStatusText(reverifyStatus.lastRunStatus) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('reverify.lastRunCount')">{{ reverifyStatus.lastRunCount }}</el-descriptions-item>
        <el-descriptions-item :label="$t('reverify.nextRunTime')">{{ reverifyStatus.nextRunTime || $t('common.none') }}</el-descriptions-item>
        <el-descriptions-item :label="$t('reverify.lastRunError')" :span="2">{{ reverifyStatus.lastRunError || $t('common.none') }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getReverifyConfig, saveReverifyConfig, runNowReverify } from '@/api/vul'
import { validateCronSpec } from '@/api/crontask'

const { t } = useI18n()

const reverifyConfigLoading = ref(false)
const reverifyConfigSubmitting = ref(false)
const reverifyRunning = ref(false)
const reverifyConfigForm = reactive({
  weakPassEnabled: false,
  exposureEnabled: false,
  cronSpec: '0 0 3 * * *'
})
const reverifyStatus = reactive({
  lastRunTime: '',
  lastRunStatus: '',
  lastRunCount: 0,
  nextRunTime: '',
  lastRunError: ''
})

// 复验周期快捷选择（与定时任务保持一致的 6 段秒级 cron）
const reverifyCronPresets = computed(() => [
  { label: t('cronTask.everyHour') || '每小时', value: '0 0 * * * *' },
  { label: t('reverify.everyDay3am') || '每日 03:00', value: '0 0 3 * * *' },
  { label: t('reverify.everyDay6am') || '每日 06:00', value: '0 0 6 * * *' },
  { label: t('cronTask.every6hours') || '每 6 小时', value: '0 0 */6 * * *' },
  { label: t('cronTask.everyMonday') || '每周一 03:00', value: '0 0 3 * * 1' }
])

const reverifyCronValidation = reactive({
  valid: false,
  nextTimes: [],
  error: ''
})

onMounted(() => loadReverifyConfig())

async function loadReverifyConfig() {
  reverifyConfigLoading.value = true
  try {
    const res = await getReverifyConfig({})
    if (res.code === 0 && res.config) {
      reverifyConfigForm.weakPassEnabled = res.config.weakPassEnabled || false
      reverifyConfigForm.exposureEnabled = res.config.exposureEnabled || false
      reverifyConfigForm.cronSpec = res.config.cronSpec || '0 0 3 * * *'
      reverifyStatus.lastRunTime = res.config.lastRunTime || ''
      reverifyStatus.lastRunStatus = res.config.lastRunStatus || ''
      reverifyStatus.lastRunCount = res.config.lastRunCount || 0
      reverifyStatus.nextRunTime = res.config.nextRunTime || ''
      reverifyStatus.lastRunError = res.config.lastRunError || ''
      if (reverifyConfigForm.cronSpec) {
        handleReverifyCronValidate()
      }
    }
  } catch (e) {
    console.error('load reverify config failed', e)
  } finally {
    reverifyConfigLoading.value = false
  }
}

async function handleReverifyConfigSubmit() {
  reverifyConfigSubmitting.value = true
  try {
    const res = await saveReverifyConfig({
      weakPassEnabled: reverifyConfigForm.weakPassEnabled,
      exposureEnabled: reverifyConfigForm.exposureEnabled,
      cronSpec: reverifyConfigForm.cronSpec
    })
    if (res.code === 0) {
      ElMessage.success(t('common.saveSuccess'))
    } else {
      ElMessage.error(res.msg || t('common.saveFailed'))
    }
  } catch (e) {
    ElMessage.error(e.message || t('common.saveFailed'))
  } finally {
    reverifyConfigSubmitting.value = false
  }
}

async function handleReverifyRunNow() {
  reverifyRunning.value = true
  try {
    const res = await runNowReverify({})
    if (res.code === 0) {
      ElMessage.success(t('reverify.runNowSuccess'))
      await loadReverifyConfig()
    } else {
      ElMessage.error(res.msg || t('common.operationFailed'))
    }
  } catch (e) {
    ElMessage.error(e.message || t('common.operationFailed'))
  } finally {
    reverifyRunning.value = false
  }
}

// Cron 表达式验证（复用定时任务的验证接口）
async function handleReverifyCronValidate() {
  if (!reverifyConfigForm.cronSpec) {
    reverifyCronValidation.valid = false
    reverifyCronValidation.error = t('cronTask.cronValidateError') || '请输入 Cron 表达式'
    reverifyCronValidation.nextTimes = []
    return
  }
  try {
    const res = await validateCronSpec({ cronSpec: reverifyConfigForm.cronSpec })
    if (res.code === 0 && res.data) {
      reverifyCronValidation.valid = res.data.valid
      if (res.data.valid) {
        reverifyCronValidation.error = ''
        reverifyCronValidation.nextTimes = res.data.nextTimes || []
      } else {
        reverifyCronValidation.error = res.data.message || t('cronTask.cronValidateError') || 'Cron 表达式无效'
        reverifyCronValidation.nextTimes = []
      }
    } else {
      reverifyCronValidation.valid = false
      reverifyCronValidation.error = res.msg || t('cronTask.cronValidateError') || '验证失败'
      reverifyCronValidation.nextTimes = []
    }
  } catch {
    reverifyCronValidation.valid = false
    reverifyCronValidation.error = t('cronTask.validateRequestError') || '验证请求失败'
    reverifyCronValidation.nextTimes = []
  }
}

function reverifyStatusText(s) {
  switch (s) {
    case 'success': return t('reverify.statusSuccess')
    case 'failed': return t('reverify.statusFailed')
    case 'partial': return t('reverify.statusPartial')
    default: return t('reverify.statusEmpty')
  }
}

function reverifyStatusTagType(s) {
  switch (s) {
    case 'success': return 'success'
    case 'failed': return 'danger'
    case 'partial': return 'warning'
    default: return 'info'
  }
}
</script>

<style scoped>
.reverify-config-page {
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

.card-header {
  font-size: 16px;
  font-weight: 500;
}

.ap-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: 10px;
}

/* Cron 调度快捷选择样式 */
.cron-help {
  margin-top: 8px;
}

.cron-help .cron-presets {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.cron-help .cron-presets .preset-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.cron-help .cron-presets .preset-tag {
  cursor: pointer;
  transition: all 0.2s;
}

.cron-help .cron-presets .preset-tag:hover {
  color: var(--el-color-primary);
  border-color: var(--el-color-primary);
}

.cron-help .cron-presets .preset-tag.is-active {
  color: var(--el-color-primary);
  border-color: var(--el-color-primary);
  background-color: var(--el-color-primary-light-9);
}

.cron-help .cron-next-times {
  font-size: 12px;
  color: var(--el-text-color-regular);
  margin-top: 4px;
}

.cron-help .cron-next-times .next-label {
  color: var(--el-text-color-secondary);
  margin-bottom: 2px;
}

.cron-help .cron-next-times .next-time {
  line-height: 1.6;
}

.cron-help .cron-error {
  font-size: 12px;
  color: var(--el-color-danger);
  margin-top: 4px;
}
</style>
