<template>
  <div class="blacklist-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('blacklist.globalBlacklist') }}</span>
          <span class="header-desc">{{ $t('blacklist.configDesc') }}</span>
        </div>
      </template>

      <div class="blacklist-content">
        <!-- 说明区域 -->
        <el-alert type="info" :closable="false" class="info-alert">
          <template #title>
            <div class="alert-content">
              <div class="alert-title">
                <el-icon><WarningFilled /></el-icon>
                {{ $t('blacklist.blacklistRules') }}
              </div>
              <div class="alert-desc">
                {{ $t('blacklist.rulesApplyDesc') }}
              </div>
            </div>
          </template>
        </el-alert>

        <!-- 支持的规则类型 -->
        <div class="rule-types">
          <span class="rule-label">{{ $t('blacklist.supportedRuleTypes') }}</span>
          <el-tag type="info" effect="plain">*.gov</el-tag>
          <span class="rule-example">{{ $t('blacklist.domain') }}</span>
          <el-tag type="info" effect="plain">*cdn*</el-tag>
          <span class="rule-example">{{ $t('blacklist.keyword') }}</span>
          <el-tag type="info" effect="plain">192.168.1.1</el-tag>
          <span class="rule-example">IP</span>
          <el-tag type="info" effect="plain">10.0.0.0/8</el-tag>
          <span class="rule-example">CIDR</span>
        </div>

        <!-- 提示信息 -->
        <div class="rule-hint">
          <el-icon><InfoFilled /></el-icon>
          {{ $t('blacklist.globalRulesHint') }}
        </div>

        <!-- 规则编辑区域 -->
        <div class="rules-editor">
          <el-input
            v-model="blacklistRules"
            type="textarea"
            :rows="18"
            :placeholder="$t('blacklist.rulesPlaceholder')"
            class="rules-textarea"
            :disabled="loading"
          />
        </div>

        <!-- 操作按钮 -->
        <div class="actions">
          <el-switch
            v-model="blacklistEnabled"
            :active-text="$t('common.enable')"
            :inactive-text="$t('common.disable')"
            :disabled="saving"
            style="margin-right: 20px"
          />
          <el-button type="primary" @click="saveBlacklist" :loading="saving">
            {{ $t('blacklist.saveRules') }}
          </el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { WarningFilled, InfoFilled } from '@element-plus/icons-vue'
import request from '@/api/request'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const blacklistRules = ref('')
const blacklistEnabled = ref(true)

// 加载黑名单配置
async function loadBlacklistConfig() {
  loading.value = true
  try {
    const res = await request.post('/blacklist/config/get', {})
    if (res.code === 0 && res.data) {
      blacklistRules.value = res.data.rules || ''
      blacklistEnabled.value = res.data.status !== 'disable'
    }
  } catch (err) {
    console.error('加载黑名单配置失败:', err)
  } finally {
    loading.value = false
  }
}

// 保存黑名单配置
async function saveBlacklist() {
  saving.value = true
  try {
    const res = await request.post('/blacklist/config/save', {
      rules: blacklistRules.value,
      status: blacklistEnabled.value ? 'enable' : 'disable'
    })
    if (res.code === 0) {
      ElMessage.success(t('common.operationSuccess'))
    } else {
      ElMessage.error(res.msg || t('common.operationFailed'))
    }
  } catch (err) {
    ElMessage.error(t('common.operationFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadBlacklistConfig()
})
</script>

<style scoped>
.blacklist-page {
  padding: 20px;
}

.card-header {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.header-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  font-weight: normal;
}

.blacklist-content {
  max-width: 100%;
}

.info-alert {
  margin-bottom: 20px;
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color-lighter);
}

.alert-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.alert-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.alert-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.rule-types {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.rule-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.rule-example {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  margin-right: 8px;
}

.rule-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 16px;
  padding: 10px 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 4px;
}

.rules-editor {
  margin-bottom: 20px;
}

.rules-textarea :deep(.el-textarea__inner) {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  background: var(--el-fill-color-darker);
  color: var(--el-text-color-primary);
  border: 1px solid var(--el-border-color);
  resize: vertical;
}

.rules-textarea :deep(.el-textarea__inner):focus {
  border-color: var(--el-color-primary);
}

.rules-textarea :deep(.el-textarea__inner)::placeholder {
  color: var(--el-text-color-placeholder);
  opacity: 0.6;
}

.actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

/* 暗色主题适配 */
:root[data-theme='dark'] .rules-textarea :deep(.el-textarea__inner) {
  background: var(--el-fill-color-darker, #1a1a2e);
}
</style>
