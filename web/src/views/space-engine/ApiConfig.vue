<template>
  <div class="space-engine-api-config">
    <el-card shadow="never" v-loading="loading">
      <template #header>
        <div class="card-header">
          <el-icon><Key /></el-icon>
          <span>{{ $t('navigation.spaceEngineApiConfig') || '空间引擎API密钥配置' }}</span>
        </div>
      </template>

      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        <template #title>
          {{ $t('spaceEngine.apiConfigTip') || '配置 Fofa / Hunter / Quake 三大空间引擎的 API 凭证，启用后即可在空间搜索中使用对应平台。密钥以脱敏形式存储与展示。' }}
        </template>
      </el-alert>

      <div class="platform-cards">
        <el-card
          v-for="platform in platforms"
          :key="platform.key"
          shadow="hover"
          class="platform-card"
        >
          <template #header>
            <div class="platform-header">
              <div class="platform-title">
                <span class="platform-name">{{ platform.name }}</span>
                <el-tag v-if="isConfigured(platform.key)" type="success" size="small">
                  {{ $t('spaceEngine.configured') || '已配置' }}
                </el-tag>
                <el-tag v-else type="info" size="small">
                  {{ $t('spaceEngine.notConfigured') || '未配置' }}
                </el-tag>
              </div>
              <el-switch
                v-model="configs[platform.key].status"
                active-value="enable"
                inactive-value="disable"
                :active-text="$t('common.enabled') || '启用'"
                inactive-text=""
                inline-prompt
              />
            </div>
          </template>

          <el-form label-width="110px" size="default" class="platform-form" :aria-label="platform.name + ' API'">
            <!-- 关联 username 字段：供浏览器自动填充与辅助技术识别凭证输入组 -->
            <input
              type="text"
              v-model="accounts[platform.key]"
              :name="`username-${platform.key}`"
              autocomplete="username"
              class="sr-only-username"
              :aria-label="$t('spaceEngine.account') || '账号'"
            />
            <el-form-item :label="$t('spaceEngine.apiKey') || 'API Key'">
              <el-input
                v-model="configs[platform.key].key"
                :name="`apikey-${platform.key}`"
                :placeholder="platform.keyPlaceholder"
                type="password"
                show-password
                clearable
                autocomplete="new-password"
                data-lpignore="true"
                data-form-type="other"
              />
            </el-form-item>

            <el-form-item
              v-if="platform.key === 'fofa'"
              :label="$t('spaceEngine.apiSecret') || 'API 邮箱/Secret'"
            >
              <el-input
                v-model="configs[platform.key].secret"
                name="apisecret-fofa"
                :placeholder="$t('spaceEngine.fofaSecretPlaceholder') || '请输入 Fofa 账号邮箱'"
                type="password"
                show-password
                clearable
                autocomplete="new-password"
                data-lpignore="true"
                data-form-type="other"
              />
            </el-form-item>

            <el-form-item
              v-if="platform.key === 'fofa'"
              :label="$t('spaceEngine.apiVersion') || 'API 版本'"
            >
              <el-select v-model="configs[platform.key].version" style="width: 100%">
                <el-option
                  v-for="opt in fofaVersionOptions"
                  :key="opt.value"
                  :label="opt.label"
                  :value="opt.value"
                />
              </el-select>
            </el-form-item>

            <!-- 测试结果提示 -->
            <el-form-item v-if="testResults[platform.key].visible" label-width="0">
              <el-alert
                :title="testResults[platform.key].message"
                :type="testResults[platform.key].type"
                :closable="true"
                show-icon
                @close="testResults[platform.key].visible = false"
              />
            </el-form-item>

            <el-form-item label-width="0" class="action-row">
              <el-button
                type="primary"
                :loading="saving[platform.key]"
                @click="handleSave(platform.key)"
              >
                {{ $t('common.save') || '保存' }}
              </el-button>
              <el-button
                type="success"
                :loading="testing[platform.key]"
                @click="handleTest(platform.key)"
              >
                {{ $t('spaceEngine.testConnection') || '测试连接' }}
              </el-button>
              <el-button link type="primary" @click="openApiUrl(platform.applyUrl)">
                {{ $t('spaceEngine.applyApi') || '申请 API' }}
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Key } from '@element-plus/icons-vue'
import request from '@/api/request'

const { t } = useI18n()

// 平台定义
const platforms = [
  {
    key: 'fofa',
    name: 'Fofa',
    keyPlaceholder: t('spaceEngine.fofaKeyPlaceholder'),
    applyUrl: 'https://fofa.info/userInfo'
  },
  {
    key: 'hunter',
    name: 'Hunter',
    keyPlaceholder: t('spaceEngine.hunterKeyPlaceholder'),
    applyUrl: 'https://hunter.qianxin.com/home/myInfo'
  },
  {
    key: 'quake',
    name: 'Quake',
    keyPlaceholder: t('spaceEngine.quakeKeyPlaceholder'),
    applyUrl: 'https://quake.360.net/quake/#/personal?tab=message'
  }
]

// Fofa 版本选项（v4 对应 fofa.info，v5 对应 v5.fofa.info）
const fofaVersionOptions = [
  { value: 'v4', label: 'v4 (fofa.info)' },
  { value: 'v5', label: 'v5 (v5.fofa.info)' }
]

// 各平台配置
const configs = reactive({
  fofa: { id: '', key: '', secret: '', version: 'v4', status: 'disable' },
  hunter: { id: '', key: '', secret: '', version: '', status: 'disable' },
  quake: { id: '', key: '', secret: '', version: '', status: 'disable' }
})

// 各平台账号（仅前端持有，不提交后端）：为密码输入提供关联的 username 锚点，便于浏览器自动填充与辅助技术识别
const accounts = reactive({ fofa: '', hunter: '', quake: '' })

// 各平台 UI 状态
const loading = ref(false)
const saving = reactive({ fofa: false, hunter: false, quake: false })
const testing = reactive({ fofa: false, hunter: false, quake: false })
const testResults = reactive({
  fofa: { visible: false, type: 'success', message: '' },
  hunter: { visible: false, type: 'success', message: '' },
  quake: { visible: false, type: 'success', message: '' }
})

onMounted(loadConfigs)

// 判断某平台是否已配置（脱敏 key 非空即视为已配置）
function isConfigured(platform) {
  const cfg = configs[platform]
  return !!(cfg.key && cfg.key !== '****' && cfg.key.trim() !== '')
}

// 打开申请 API 的外链
function openApiUrl(url) {
  window.open(url, '_blank')
}

// 从后端加载配置
async function loadConfigs() {
  loading.value = true
  try {
    const res = await request.post('/onlineapi/config/list', {})
    if (res.code === 0 && Array.isArray(res.list)) {
      res.list.forEach(item => {
        if (configs[item.platform]) {
          configs[item.platform].id = item.id || ''
          // 后端返回的是脱敏后的 key/secret，直接展示
          configs[item.platform].key = item.key || ''
          configs[item.platform].secret = item.secret || ''
          configs[item.platform].status = item.status || 'disable'
          if (item.platform === 'fofa') {
            // 兼容后端返回 v1/v4 的场景，统一映射为 v4
            const v = item.version || 'v4'
            configs.fofa.version = v === 'v1' ? 'v4' : v
          }
        }
      })
    }
  } catch (e) {
    ElMessage.error(e?.message || t('spaceEngine.loadConfigFailed'))
  } finally {
    loading.value = false
  }
}

// 判断值是否为脱敏后的占位串（后端 maskSecret 规则：<=8 位为 "****"，否则前4+****+后4）
function isMaskedValue(v) {
  if (!v) return true
  if (v === '****') return true
  return /^.{4}\*\*\*\*.{4}$/.test(v)
}

// 保存单个平台配置
async function handleSave(platform) {
  const cfg = configs[platform]
  saving[platform] = true
  try {
    const data = {
      platform,
      status: cfg.status || 'disable'
    }
    // 仅在用户输入了新凭证（非脱敏值）时才提交，避免把脱敏占位符回写覆盖真实凭证
    if (!isMaskedValue(cfg.key)) {
      data.key = cfg.key
    }
    if (platform === 'fofa' && !isMaskedValue(cfg.secret)) {
      data.secret = cfg.secret
    }
    if (platform === 'fofa') {
      // 前端传 v4，后端识别为默认版本（fofa.info）
      data.version = cfg.version === 'v4' ? 'v1' : cfg.version
    }
    if (cfg.id) {
      data.id = cfg.id
    }

    const res = await request.post('/onlineapi/config/save', data)
    if (res.code === 0) {
      ElMessage.success(t('spaceEngine.saveSuccess'))
      // 清空测试结果；保留用户输入的真实凭证，避免重新加载脱敏值后需要再次输入
      testResults[platform].visible = false
      // 新记录没有 id，保存后重新加载以获取 id，但保留当前输入的 key/secret
      if (!cfg.id) {
        const prevKey = cfg.key
        const prevSecret = cfg.secret
        await loadConfigs()
        // 还原用户输入的真实值，不被脱敏占位符覆盖
        configs[platform].key = prevKey
        if (platform === 'fofa') {
          configs[platform].secret = prevSecret
        }
      }
    } else {
      ElMessage.error(res.msg || t('spaceEngine.saveFailed'))
    }
  } catch (e) {
    ElMessage.error(e?.message || t('spaceEngine.saveFailed'))
  } finally {
    saving[platform] = false
  }
}

// 测试连接：先保存当前配置，再发起一次最小查询以验证凭证是否可用
async function handleTest(platform) {
  const cfg = configs[platform]
  // 测试前要求必须填写 Key
  if (isMaskedValue(cfg.key)) {
    ElMessage.warning(t('spaceEngine.fillKeyFirst'))
    return
  }
  if (cfg.status !== 'enable') {
    ElMessage.warning(t('spaceEngine.enableFirst'))
    return
  }

  testing[platform] = true
  testResults[platform].visible = false
  try {
    // 先保存（确保后端使用最新凭证进行测试）
    await handleSave(platform)

    // 通过搜索接口发起一次最小查询来验证凭证
    const testQuery = platform === 'fofa' ? 'domain="fofa.info"' : (platform === 'hunter' ? 'domain="qianxin.com"' : 'domain="360.net"')
    const res = await request.post('/onlineapi/search', {
      platform,
      query: testQuery,
      page: 1,
      pageSize: 1
    })
    if (res.code === 0) {
      testResults[platform] = {
        visible: true,
        type: 'success',
        message: t('spaceEngine.connectSuccess')
      }
    } else {
      testResults[platform] = {
        visible: true,
        type: 'error',
        message: t('spaceEngine.connectFailed', { msg: res.msg || t('spaceEngine.unknownError') })
      }
    }
  } catch (e) {
    testResults[platform] = {
      visible: true,
      type: 'error',
      message: t('spaceEngine.connectFailed', { msg: e?.message || t('spaceEngine.networkError') })
    }
  } finally {
    testing[platform] = false
  }
}
</script>

<style lang="scss" scoped>
.space-engine-api-config {
  // 视觉隐藏但保留在可访问性树中：作为浏览器自动填充与辅助技术的 username 锚点
  .sr-only-username {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 600;
  }

  .platform-cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
    gap: 20px;
  }

  .platform-card {
    :deep(.el-card__header) {
      padding: 14px 18px;
    }

    .platform-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    .platform-title {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    .platform-name {
      font-size: 16px;
      font-weight: 600;
    }

    .platform-form {
      padding-top: 4px;
    }

    .action-row {
      margin-bottom: 0;

      .el-button + .el-button {
        margin-left: 8px;
      }
    }
  }
}
</style>
