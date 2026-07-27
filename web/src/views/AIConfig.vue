<template>
  <div class="ai-config-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('navigation.aiConfig') }}</span>
          <div>
            <el-button size="small" @click="handleFetchModels" :loading="fetchingModels">
              {{ $t('aiConfig.getModels') }}
            </el-button>
            <el-button size="small" @click="handleTest" :loading="testing">
              {{ $t('aiConfig.testConnection') }}
            </el-button>
            <el-button type="primary" size="small" @click="handleSave" :loading="saving">
              {{ $t('aiConfig.saveConfig') }}
            </el-button>
          </div>
        </div>
      </template>

      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        <template #title>{{ $t('aiConfig.alertDescription') }}</template>
      </el-alert>

      <el-form label-width="120px" v-loading="loading">
        <el-form-item :label="$t('aiConfig.protocolType')">
          <el-radio-group v-model="form.protocol">
            <el-radio-button v-for="p in AI_PROTOCOLS" :key="p.value" :label="p.value">
              {{ p.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item :label="$t('aiConfig.serverAddress')">
          <el-input v-model="form.baseUrl" placeholder="http://127.0.0.1:8045" />
          <div class="hint-secondary" style="margin-top: 4px">{{ protocolHint(form.protocol) }}</div>
        </el-form-item>

        <el-form-item :label="$t('aiConfig.apiKey')">
          <el-input v-model="form.apiKey" :placeholder="$t('aiConfig.apiKeyPlaceholder')" show-password />
        </el-form-item>

        <el-form-item :label="$t('aiConfig.model')">
          <el-select v-model="form.model" :placeholder="$t('aiConfig.selectModel')" style="width: 100%" allow-create filterable>
            <el-option v-for="m in availableModels" :key="m" :label="m" :value="m" />
          </el-select>
          <div class="hint-secondary" style="margin-top: 4px">{{ $t('aiConfig.modelHint') }}</div>
        </el-form-item>

        <el-form-item :label="$t('aiConfig.lastUpdate')">
          <span class="text-secondary">{{ form.updateTime || $t('aiConfig.notSaved') }}</span>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { saveAIConfig } from '@/api/ai'
import { AI_PROTOCOLS, DEFAULT_AI_CONFIG, loadAIConfig, testConnection, protocolHint, fetchModels } from '@/utils/aiClient'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const fetchingModels = ref(false)
const availableModels = ref([])

const form = reactive({ ...DEFAULT_AI_CONFIG, updateTime: '' })

onMounted(loadConfig)

async function loadConfig() {
  loading.value = true
  try {
    const config = await loadAIConfig()
    Object.assign(form, config)
    // Auto-load model into list if it exists but list is empty
    if (config.model && !availableModels.value.includes(config.model)) {
      availableModels.value = [config.model]
    }
  } finally {
    loading.value = false
  }
}

async function handleFetchModels() {
  if (!form.baseUrl) {
    ElMessage.warning(t('aiConfig.pleaseConfigBaseUrl'))
    return
  }

  fetchingModels.value = true
  try {
    const models = await fetchModels(form)
    availableModels.value = models
    if (form.model && !models.includes(form.model)) {
      models.unshift(form.model)
    }
    ElMessage.success(t('aiConfig.modelsLoaded', { count: models.length }))
  } catch (e) {
    ElMessage.error(
      e.code === 'NETWORK'
        ? t('aiConfig.cannotConnect')
        : `${t('aiConfig.fetchModelsFailed')}: ${e.message}`
    )
  } finally {
    fetchingModels.value = false
  }
}

async function handleSave() {
  if (!form.baseUrl) {
    ElMessage.warning(t('aiConfig.pleaseConfigBaseUrl'))
    return
  }

  saving.value = true
  try {
    const res = await saveAIConfig({
      protocol: form.protocol,
      baseUrl: form.baseUrl,
      apiKey: form.apiKey,
      model: form.model
    })
    if (res.code === 0) {
      ElMessage.success(t('aiConfig.saveSuccess'))
      await loadConfig()
    } else {
      ElMessage.error(res.msg || t('aiConfig.saveFailed'))
    }
  } catch (e) {
    ElMessage.error(t('aiConfig.saveFailed') + ': ' + e.message)
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  if (!form.baseUrl) {
    ElMessage.warning(t('aiConfig.pleaseConfigBaseUrl'))
    return
  }

  testing.value = true
  try {
    await testConnection(form)
    ElMessage.success(t('aiConfig.connectionSuccess'))
  } catch (e) {
    ElMessage.error(
      e.code === 'NETWORK'
        ? t('aiConfig.cannotConnect')
        : `${t('aiConfig.connectionFailed')}: ${e.message}`
    )
  } finally {
    testing.value = false
  }
}
</script>

<style scoped>
.ai-config-page {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
}
</style>
