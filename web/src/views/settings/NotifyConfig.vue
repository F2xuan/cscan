<template>
  <div class="notify-config-page">
    <div class="notify-container">
      <!-- 左侧渠道列表 -->
      <div class="notify-list">
        <div class="notify-list-header">
          <span class="notify-list-title">{{ $t('settings.channelList') }}</span>
          <el-button type="primary" size="small" @click="showNotifyDrawer()">
            <el-icon><Plus /></el-icon>
          </el-button>
        </div>
        <div class="notify-list-content" v-loading="notifyLoading">
          <div
            v-for="item in notifyConfigList"
            :key="item.id"
            class="notify-item"
            :class="{ active: selectedNotifyId === item.id }"
            @click="selectNotifyConfig(item)"
          >
            <div class="notify-item-icon">
              <el-icon :size="20"><Bell /></el-icon>
            </div>
            <div class="notify-item-info">
              <div class="notify-item-name">{{ item.name }}</div>
              <div class="notify-item-provider">{{ getProviderName(item.provider) }}</div>
            </div>
            <el-switch
              v-model="item.status"
              active-value="enable"
              inactive-value="disable"
              size="small"
              @click.stop
              @change="handleNotifyStatusChange(item)"
            />
          </div>
          <el-empty v-if="notifyConfigList.length === 0" :description="$t('settings.noNotifyConfig')" />
        </div>
      </div>

      <!-- 右侧详情 -->
      <div class="notify-detail">
        <template v-if="selectedNotify">
          <div class="notify-detail-header">
            <div class="notify-detail-title">
              <el-icon :size="24"><Bell /></el-icon>
              <span>{{ selectedNotify.name }}</span>
            </div>
            <div class="notify-detail-meta">
              {{ $t('common.updateTime') }}: {{ selectedNotify.updateTime }}
            </div>
          </div>

          <div class="notify-detail-section">
            <div class="section-title">{{ $t('settings.basicInfo') }}</div>
            <div class="info-grid">
              <div class="info-item">
                <span class="info-label">{{ $t('settings.channelType') }}</span>
                <span class="info-value">
                  <el-tag>{{ getProviderName(selectedNotify.provider) }}</el-tag>
                </span>
              </div>
              <div class="info-item">
                <span class="info-label">{{ $t('common.status') }}</span>
                <span class="info-value">
                  <el-tag :type="selectedNotify.status === 'enable' ? 'success' : 'info'">
                    {{ selectedNotify.status === 'enable' ? $t('common.enabled') : $t('common.disabled') }}
                  </el-tag>
                </span>
              </div>
              <div class="info-item" v-if="selectedNotify.webUrl">
                <span class="info-label">{{ $t('settings.webUrl') }}</span>
                <span class="info-value">{{ selectedNotify.webUrl }}</span>
              </div>
            </div>
          </div>

          <div class="notify-detail-section">
            <div class="section-title">{{ $t('settings.notifyContent') }}</div>
            <div class="notify-preview">
              <div class="preview-item" v-for="field in notifyFields" :key="field.key">
                <el-checkbox v-model="field.enabled" disabled>{{ field.label }}</el-checkbox>
              </div>
            </div>
          </div>

          <div class="notify-detail-actions">
            <el-button type="primary" @click="showNotifyDrawer(selectedNotify)">
              <el-icon><Edit /></el-icon>{{ $t('common.edit') }}
            </el-button>
            <el-button type="success" @click="handleTestNotify(selectedNotify)" :loading="selectedNotify.testing">
              <el-icon><Position /></el-icon>{{ $t('settings.test') }}
            </el-button>
            <el-button type="danger" @click="handleDeleteNotify(selectedNotify)">
              <el-icon><Delete /></el-icon>{{ $t('common.delete') }}
            </el-button>
          </div>
        </template>
        <el-empty v-else :description="$t('settings.selectNotifyTip')" />
      </div>
    </div>

    <!-- 通知配置抽屉 -->
    <el-drawer
      v-model="notifyDrawerVisible"
      :title="notifyForm.id ? $t('settings.editNotifyConfig') : $t('settings.addNotifyChannelTitle')"
      size="480px"
      :close-on-click-modal="false"
    >
      <el-form ref="notifyFormRef" :model="notifyForm" :rules="notifyRules" label-position="top">
        <el-form-item :label="$t('settings.channelType')" prop="provider">
          <el-select v-model="notifyForm.provider" :placeholder="$t('settings.selectNotifyChannel')" @change="handleProviderChange" :disabled="!!notifyForm.id" style="width: 100%">
            <el-option v-for="p in notifyProviders" :key="p.id" :label="p.name" :value="p.id">
              <div class="provider-option">
                <span class="provider-name">{{ p.name }}</span>
                <span class="provider-desc">{{ p.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('settings.configName')" prop="name">
          <el-input v-model="notifyForm.name" :placeholder="$t('settings.enterConfigName')" />
        </el-form-item>

        <!-- 动态配置字段 -->
        <template v-if="currentProviderFields.length > 0">
          <el-divider>{{ $t('settings.channelConfig') }}</el-divider>
          <el-form-item
            v-for="field in currentProviderFields"
            :key="field.name"
            :label="field.label"
            :prop="'configData.' + field.name"
            :rules="field.required ? [{ required: true, message: t('settings.pleaseEnterInput') + field.label, trigger: 'blur' }] : []"
          >
            <el-input
              v-if="field.type === 'text'"
              v-model="notifyForm.configData[field.name]"
              :placeholder="field.placeholder"
            />
            <el-input
              v-else-if="field.type === 'password'"
              v-model="notifyForm.configData[field.name]"
              :placeholder="field.placeholder"
              show-password
            />
            <el-input
              v-else-if="field.type === 'textarea'"
              v-model="notifyForm.configData[field.name]"
              type="textarea"
              :rows="2"
              :placeholder="field.placeholder"
            />
            <el-input-number
              v-else-if="field.type === 'number'"
              v-model="notifyForm.configData[field.name]"
              :placeholder="field.placeholder"
              controls-position="right"
              style="width: 100%"
            />
            <el-switch
              v-else-if="field.type === 'switch'"
              v-model="notifyForm.configData[field.name]"
            />
            <el-select
              v-else-if="field.type === 'select'"
              v-model="notifyForm.configData[field.name]"
              :placeholder="field.placeholder || $t('common.pleaseSelect')"
              clearable
              style="width: 100%"
            >
              <el-option v-for="opt in field.options" :key="opt" :label="opt || $t('common.default')" :value="opt" />
            </el-select>
          </el-form-item>
        </template>

        <el-divider>{{ $t('settings.notifySettings') }}</el-divider>

        <el-form-item :label="$t('settings.webUrl')">
          <el-input v-model="notifyForm.webUrl" :placeholder="$t('settings.webUrlPlaceholder')" />
          <div class="form-tip">{{ $t('settings.webUrlTip') }}</div>
        </el-form-item>

        <el-form-item :label="$t('settings.notifyContent')">
          <div class="notify-fields-config">
            <el-checkbox v-model="notifyForm.fields.taskName">{{ $t('settings.fieldTaskName') }}</el-checkbox>
            <el-checkbox v-model="notifyForm.fields.status">{{ $t('settings.fieldStatus') }}</el-checkbox>
            <el-checkbox v-model="notifyForm.fields.assetCount">{{ $t('settings.fieldAssetCount') }}</el-checkbox>
            <el-checkbox v-model="notifyForm.fields.vulCount">{{ $t('settings.fieldVulCount') }}</el-checkbox>
            <el-checkbox v-model="notifyForm.fields.duration">{{ $t('settings.fieldDuration') }}</el-checkbox>
            <el-checkbox v-model="notifyForm.fields.time">{{ $t('settings.fieldTime') }}</el-checkbox>
            <el-checkbox v-model="notifyForm.fields.reportUrl">{{ $t('settings.fieldReportUrl') }}</el-checkbox>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="drawer-footer">
          <el-button @click="notifyDrawerVisible = false">{{ $t('common.cancel') }}</el-button>
          <el-button type="success" @click="handleTestNotifyForm" :loading="notifyTesting">{{ $t('settings.test') }}</el-button>
          <el-button type="primary" :loading="notifySubmitting" @click="handleNotifySubmit">{{ $t('common.save') }}</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Bell, Edit, Delete, Position } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { getNotifyProviders, getNotifyConfigList, saveNotifyConfig, deleteNotifyConfig, testNotifyConfig } from '@/api/notify'

const { t } = useI18n()

const notifyLoading = ref(false)
const notifyConfigList = ref([])
const notifyProviders = ref([])
const notifyDrawerVisible = ref(false)
const notifySubmitting = ref(false)
const notifyTesting = ref(false)
const notifyFormRef = ref()
const selectedNotifyId = ref('')
const selectedNotify = computed(() => notifyConfigList.value.find(item => item.id === selectedNotifyId.value))
const notifyForm = ref(defaultNotifyForm())
const currentProviderFields = ref([])

function defaultNotifyForm() {
  return {
    id: '',
    name: '',
    provider: '',
    configData: {},
    messageTemplate: '',
    webUrl: '',
    status: 'enable',
    fields: {
      taskName: true,
      status: true,
      assetCount: true,
      vulCount: true,
      duration: true,
      time: true,
      reportUrl: true
    }
  }
}

const notifyRules = computed(() => ({
  provider: [{ required: true, message: t('settings.selectNotifyChannel'), trigger: 'change' }],
  name: [{ required: true, message: t('settings.enterConfigName'), trigger: 'blur' }]
}))

const notifyFields = computed(() => [
  { key: 'taskName', label: t('settings.fieldTaskName'), enabled: true },
  { key: 'status', label: t('settings.fieldStatus'), enabled: true },
  { key: 'assetCount', label: t('settings.fieldAssetCount'), enabled: true },
  { key: 'vulCount', label: t('settings.fieldVulCount'), enabled: true },
  { key: 'duration', label: t('settings.fieldDuration'), enabled: true },
  { key: 'time', label: t('settings.fieldTime'), enabled: true },
  { key: 'reportUrl', label: t('settings.fieldReportUrl'), enabled: true }
])

onMounted(() => {
  loadNotifyProviders()
  loadNotifyConfigList()
})

async function loadNotifyProviders() {
  try {
    const res = await getNotifyProviders()
    if (res.code === 0) {
      notifyProviders.value = res.data?.list || res.list || []
    }
  } catch (error) {
    console.error('加载通知提供者失败', error)
  }
}

async function loadNotifyConfigList() {
  notifyLoading.value = true
  try {
    const res = await getNotifyConfigList()
    if (res.code === 0) {
      const list = res.data?.list || res.list || []
      notifyConfigList.value = list.map(item => ({ ...item, testing: false }))
    }
  } finally {
    notifyLoading.value = false
  }
}

function getProviderName(providerId) {
  const provider = notifyProviders.value.find(p => p.id === providerId)
  return provider ? provider.name : providerId
}

function handleProviderChange(providerId) {
  const provider = notifyProviders.value.find(p => p.id === providerId)
  currentProviderFields.value = provider ? provider.configFields || [] : []
  notifyForm.value.configData = {}
}

function selectNotifyConfig(item) {
  selectedNotifyId.value = item.id
}

function showNotifyDrawer(row = null) {
  if (row) {
    let configData = {}
    try {
      configData = JSON.parse(row.config || '{}')
    } catch {
      configData = {}
    }
    notifyForm.value = {
      id: row.id,
      name: row.name,
      provider: row.provider,
      configData,
      messageTemplate: row.messageTemplate || '',
      webUrl: row.webUrl || '',
      status: row.status,
      fields: defaultNotifyForm().fields
    }
    const provider = notifyProviders.value.find(p => p.id === row.provider)
    currentProviderFields.value = provider ? provider.configFields || [] : []
  } else {
    notifyForm.value = defaultNotifyForm()
    currentProviderFields.value = []
  }
  notifyDrawerVisible.value = true
}

// 根据字段配置生成消息模板
function generateMessageTemplate() {
  const fields = notifyForm.value.fields
  let template = '{{statusEmoji}} 扫描任务完成\n\n'
  if (fields.taskName) template += '任务名称: {{taskName}}\n'
  if (fields.status) template += '任务状态: {{status}}\n'
  if (fields.assetCount) template += '发现资产: {{assetCount}}\n'
  if (fields.vulCount) template += '发现漏洞: {{vulCount}}\n'
  if (fields.duration) template += '执行时长: {{duration}}\n'
  if (fields.time) template += '开始时间: {{startTime}}\n结束时间: {{endTime}}\n'
  if (fields.reportUrl) template += '报告地址: {{reportUrl}}'
  return template.trim()
}

async function handleNotifySubmit() {
  if (!notifyFormRef.value) return
  try {
    await notifyFormRef.value.validate()
    notifySubmitting.value = true

    const messageTemplate = generateMessageTemplate()
    const data = {
      id: notifyForm.value.id,
      name: notifyForm.value.name,
      provider: notifyForm.value.provider,
      config: JSON.stringify(notifyForm.value.configData),
      messageTemplate,
      webUrl: notifyForm.value.webUrl,
      status: notifyForm.value.status
    }

    const res = await saveNotifyConfig(data)
    if (res.code === 0) {
      ElMessage.success(res.msg || t('common.operationSuccess'))
      notifyDrawerVisible.value = false
      await loadNotifyConfigList()
      if (!notifyForm.value.id && notifyConfigList.value.length > 0) {
        selectedNotifyId.value = notifyConfigList.value[0].id
      }
    } else {
      ElMessage.error(res.msg || t('common.operationFailed'))
    }
  } catch (error) {
    console.error('表单验证失败:', error)
  } finally {
    notifySubmitting.value = false
  }
}

async function handleNotifyStatusChange(row) {
  const data = {
    id: row.id,
    name: row.name,
    provider: row.provider,
    config: row.config,
    messageTemplate: row.messageTemplate,
    webUrl: row.webUrl,
    status: row.status
  }
  const res = await saveNotifyConfig(data)
  if (res.code === 0) {
    ElMessage.success(t('common.statusUpdateSuccess'))
  } else {
    row.status = row.status === 'enable' ? 'disable' : 'enable'
    ElMessage.error(res.msg || t('common.statusUpdateFailed'))
  }
}

async function handleTestNotify(row) {
  row.testing = true
  try {
    const res = await testNotifyConfig({
      provider: row.provider,
      config: row.config,
      messageTemplate: row.messageTemplate
    })
    if (res.code === 0) {
      ElMessage.success(res.msg || t('settings.testSuccess'))
    } else {
      ElMessage.error(res.msg || t('settings.testFailed'))
    }
  } finally {
    row.testing = false
  }
}

async function handleTestNotifyForm() {
  if (!notifyForm.value.provider) {
    ElMessage.warning(t('settings.selectChannelFirst'))
    return
  }
  notifyTesting.value = true
  try {
    const res = await testNotifyConfig({
      provider: notifyForm.value.provider,
      config: JSON.stringify(notifyForm.value.configData),
      messageTemplate: notifyForm.value.messageTemplate
    })
    if (res.code === 0) {
      ElMessage.success(res.msg || t('settings.testSuccess'))
    } else {
      ElMessage.error(res.msg || t('settings.testFailed'))
    }
  } finally {
    notifyTesting.value = false
  }
}

async function handleDeleteNotify(row) {
  try {
    await ElMessageBox.confirm(t('settings.confirmDeleteNotify'), t('common.tip'), { type: 'warning' })
    const res = await deleteNotifyConfig(row.id)
    if (res.code === 0) {
      ElMessage.success(res.msg || t('common.deleteSuccess'))
      if (selectedNotifyId.value === row.id) {
        selectedNotifyId.value = ''
      }
      loadNotifyConfigList()
    } else {
      ElMessage.error(res.msg || t('common.operationFailed'))
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除通知配置失败', error)
    }
  }
}
</script>

<style scoped>
/* 通知配置左右分栏布局 */
.notify-container {
  display: flex;
  gap: 20px;
  height: calc(100vh - 180px);
  min-height: 500px;
}

.notify-list {
  width: 280px;
  flex-shrink: 0;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
}

.notify-list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.notify-list-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.notify-list-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.notify-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  margin-bottom: 4px;
}

.notify-item:hover {
  background: var(--el-fill-color-light);
}

.notify-item.active {
  background: var(--el-color-primary-light-9);
  border: 1px solid var(--el-color-primary-light-5);
}

.notify-item-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: var(--el-color-primary-light-8);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-color-primary);
}

.notify-item-info {
  flex: 1;
  min-width: 0;
}

.notify-item-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.notify-item-provider {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.notify-detail {
  flex: 1;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 24px;
  overflow-y: auto;
}

.notify-detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.notify-detail-title {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.notify-detail-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.notify-detail-section {
  margin-bottom: 24px;
}

.section-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
  margin-bottom: 12px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.info-value {
  font-size: 14px;
  color: var(--el-text-color-primary);
}

.notify-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.preview-item {
  min-width: 120px;
}

.notify-detail-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--el-border-color-light);
}

/* 抽屉样式 */
.provider-option {
  display: flex;
  flex-direction: column;
}

.provider-name {
  font-size: 14px;
}

.provider-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.notify-fields-config {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
