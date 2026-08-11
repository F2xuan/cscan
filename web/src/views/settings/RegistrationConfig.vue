<template>
  <div class="registration-config-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('settings.registration.title') }}</span>
        </div>
      </template>
      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        <template #title>{{ $t('settings.registration.description') }}</template>
      </el-alert>

      <el-form label-width="160px" style="max-width: 560px;">
        <el-form-item :label="$t('settings.registration.enabled')">
          <el-switch
            v-model="config.enabled"
            active-text=""
            inactive-text=""
            @change="handleSave"
          />
          <span class="form-hint">{{ config.enabled ? $t('settings.registration.enabledHint') : $t('settings.registration.disabledHint') }}</span>
        </el-form-item>
        <el-form-item :label="$t('settings.registration.requireApproval')">
          <el-switch
            v-model="config.requireApproval"
            :disabled="!config.enabled"
            active-text=""
            inactive-text=""
            @change="handleSave"
          />
          <span class="form-hint">{{ config.requireApproval ? $t('settings.registration.requireApprovalHint') : $t('settings.registration.noApprovalHint') }}</span>
        </el-form-item>
        <el-form-item v-if="config.updateTime">
          <span class="update-time">{{ $t('common.updateTime') }}: {{ config.updateTime }}</span>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getRegistrationConfig, saveRegistrationConfig } from '@/api/auth'

const { t } = useI18n()

const config = reactive({
  enabled: false,
  requireApproval: true,
  updateTime: ''
})

onMounted(() => loadConfig())

async function loadConfig() {
  try {
    const res = await getRegistrationConfig()
    if (res && res.code === 0 && res.config) {
      config.enabled = res.config.enabled
      config.requireApproval = res.config.requireApproval
      config.updateTime = res.config.updateTime || ''
    }
  } catch (e) {
    ElMessage.error(e?.message || t('common.loadFailed'))
  }
}

async function handleSave() {
  try {
    const res = await saveRegistrationConfig({
      enabled: config.enabled,
      requireApproval: config.requireApproval
    })
    if (res && res.code === 0) {
      if (res.config && res.config.updateTime) {
        config.updateTime = res.config.updateTime
      }
      ElMessage.success(t('common.saveSuccess'))
    } else {
      ElMessage.error(res?.msg || t('common.saveFailed'))
    }
  } catch (e) {
    ElMessage.error(e?.message || t('common.saveFailed'))
  }
}
</script>

<style scoped>
.registration-config-page .card-header {
  font-size: 16px;
  font-weight: 500;
}

.form-hint {
  margin-left: 12px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.update-time {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
