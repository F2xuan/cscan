<template>
  <div class="target-detail-view">
    <!-- Header -->
    <div class="detail-header">
      <div class="header-left">
        <el-button link class="back-button" @click="handleBack">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <TargetSwitcher :target-id="targetId" @select="handleSwitchTarget" />
        <span v-if="meta?.internalNetworkId" class="internal-badge">
          {{ $t('asset.targetView.internalBadge') }}
        </span>
        <TargetStatusBadge :status="meta?.scanStatus || 'pending'" />
      </div>
      <div class="header-right">
        <span v-if="lastScanTime" class="last-scan-time">
          {{ formatRelativeTime(lastScanTime) }}
        </span>
        <el-tooltip :content="$t('asset.targetView.settings')" placement="top">
          <el-button circle @click="settingsOpen = true">
            <el-icon><Setting /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </div>

    <!-- Exposure / Risk 气泡：页内下钻到 Inventory 子 Tab（目录/JS/敏感信息跳独立页） -->
    <div v-if="hasAnyStats" class="bubble-bar">
      <div class="bubble-row">
        <el-tag v-if="exposure.subdomains && isDomainTarget" size="small" type="info" class="bubble clickable" @click="goTab('subdomain')">{{ $t('asset.assetGroupsTab.expSubdomains') }} {{ exposure.subdomains }}</el-tag>
        <el-tag v-if="exposure.ips" size="small" type="info" class="bubble clickable" @click="goTab('ip')">{{ $t('asset.assetGroupsTab.expIps') }} {{ exposure.ips }}</el-tag>
        <el-tag v-if="exposure.ports" size="small" type="info" class="bubble clickable" @click="goTab('port')">{{ $t('asset.assetGroupsTab.expPorts') }} {{ exposure.ports }}</el-tag>
        <el-tag v-if="exposure.sites" size="small" type="info" class="bubble clickable" @click="goTab('services')">{{ $t('asset.assetGroupsTab.expSites') }} {{ exposure.sites }}</el-tag>
        <el-tag v-if="exposure.icons" size="small" type="info" class="bubble clickable" @click="goTab('services')">{{ $t('asset.assetGroupsTab.expIcons') }} {{ exposure.icons }}</el-tag>
        <el-tag v-if="exposure.apps" size="small" type="info" class="bubble clickable" @click="goTab('app')">{{ $t('asset.assetGroupsTab.expApps') }} {{ exposure.apps }}</el-tag>
        <el-tag v-if="exposure.screenshots" size="small" type="info" class="bubble clickable" @click="goTab('services')">{{ $t('asset.assetGroupsTab.expScreenshots') }} {{ exposure.screenshots }}</el-tag>
        <el-tag v-if="exposure.dirs" size="small" type="info" class="bubble clickable" @click="goExposurePage('dir')">{{ $t('asset.assetGroupsTab.expDirs') }} {{ exposure.dirs }}</el-tag>
        <el-tag v-if="exposure.js" size="small" type="info" class="bubble clickable" @click="goExposurePage('js')">{{ $t('asset.assetGroupsTab.expJs') }} {{ exposure.js }}</el-tag>
        <el-tag v-if="risk.vulnTotal" size="small" :type="risk.vulnHigh > 0 ? 'danger' : 'warning'" class="bubble clickable" @click="goVulnTab">{{ $t('asset.assetGroupsTab.riskVulnTotal') }} {{ risk.vulnTotal }}</el-tag>
        <el-tag v-if="risk.vulnHigh" size="small" type="danger" class="bubble clickable" @click="goVulnTab">{{ $t('asset.assetGroupsTab.riskVulnHigh') }} {{ risk.vulnHigh }}</el-tag>
        <el-tag v-if="risk.sensitiveInfo" size="small" type="warning" class="bubble clickable" @click="goSensitivePage">{{ $t('asset.assetGroupsTab.riskSensitiveInfo') }} {{ risk.sensitiveInfo }}</el-tag>
      </div>
    </div>

    <!-- Tabs -->
    <el-tabs v-model="activeTab" class="detail-tabs">
      <el-tab-pane :label="$t('asset.targetView.tabInventory')" name="inventory">
        <TargetInventory
          ref="inventoryRef"
          :key="targetId"
          :target-id="targetId"
          @view-asset="handleViewAsset"
        />
      </el-tab-pane>

      <el-tab-pane :label="$t('asset.targetView.tabVulnerabilities')" name="vulnerabilities">
        <div class="vuln-tab">
          <!-- 统计卡 -->
          <div class="vuln-stats">
            <div class="stat-card">
              <div class="stat-value">{{ risk.vulnTotal }}</div>
              <div class="stat-label">{{ $t('asset.targetView.vulTotal') }}</div>
            </div>
            <div class="stat-card danger">
              <div class="stat-value">{{ risk.vulnHigh }}</div>
              <div class="stat-label">{{ $t('asset.targetView.vulHigh') }}</div>
            </div>
            <div class="stat-card warning">
              <div class="stat-value">{{ risk.sensitiveInfo }}</div>
              <div class="stat-label">{{ $t('asset.targetView.sensitiveInfo') }}</div>
            </div>
            <div class="stat-card warning">
              <div class="stat-value">{{ risk.sensitiveDir }}</div>
              <div class="stat-label">{{ $t('asset.targetView.sensitiveDir') }}</div>
            </div>
          </div>

          <!-- 漏洞列表（目标下最新漏洞，含 POC 扫描结果） -->
          <el-table
            v-if="vulnTableData.length"
            :data="vulnTableData"
            class="vuln-table"
          >
            <el-table-column :label="$t('asset.targetView.colVulName')" min-width="200">
              <template #default="{ row }">
                <el-tag size="small" :type="severityTagType(row.severity)">{{ row.severity }}</el-tag>
                <span class="vuln-name">{{ row.vulName }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('asset.targetView.colHost')" min-width="180">
              <template #default="{ row }">
                <span class="mono">{{ row.host }}:{{ row.port }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('asset.targetView.colUrl')" min-width="220">
              <template #default="{ row }">
                <a :href="row.url" target="_blank" rel="noopener" class="vuln-url">{{ row.url }}</a>
              </template>
            </el-table-column>
            <el-table-column :label="$t('asset.targetView.colTime')" width="140">
              <template #default="{ row }">
                <span class="muted">{{ formatRelativeTime(row.createTime) }}</span>
              </template>
            </el-table-column>
          </el-table>

          <div v-else class="vuln-empty">{{ $t('asset.targetView.noVulns') }}</div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- Settings drawer -->
    <TargetSettingsDrawer
      v-model="settingsOpen"
      :meta="meta"
      @saved="fetchMeta"
      @deleted="handleBack"
    />

    <!-- Asset detail sheet（Inventory 服务行点击打开） -->
    <AssetDetailSheet
      v-model="assetSheetOpen"
      :asset-id="assetSheetId"
      :target-id="targetId"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Setting } from '@element-plus/icons-vue'
import TargetStatusBadge from './TargetStatusBadge.vue'
import TargetSwitcher from './TargetSwitcher.vue'
import TargetSettingsDrawer from './TargetSettingsDrawer.vue'
import TargetInventory from './TargetInventory.vue'
import AssetDetailSheet from './AssetDetailSheet.vue'
import { getAssetTargetDetail } from '@/api/asset'
import { formatRelativeTime } from './targetViewUtils'

const props = defineProps({
  targetId: { type: String, required: true },
})

const emit = defineEmits(['back', 'view-asset'])

const router = useRouter()

const activeTab = ref('inventory')
const meta = ref(null)
const risk = reactive({
  vulnTotal: 0,
  vulnHigh: 0,
  sensitiveInfo: 0,
  sensitiveDir: 0,
  vulnItems: [],
  sensitiveInfoItems: [],
})
const exposure = reactive({
  subdomains: 0,
  ips: 0,
  ports: 0,
  sites: 0,
  icons: 0,
  apps: 0,
  dirs: 0,
  js: 0,
  screenshots: 0,
})
const settingsOpen = ref(false)

// 资产详情抽屉（Inventory 服务行点击打开）
const assetSheetOpen = ref(false)
const assetSheetId = ref('')

let pollTimer = null

const lastScanTime = computed(() => meta.value?.lastScanTime || 0)

// 漏洞 Tab 列表数据：优先真实漏洞列表，兜底敏感信息条目
const vulnTableData = computed(() =>
  (risk.vulnItems && risk.vulnItems.length) ? risk.vulnItems : (risk.sensitiveInfoItems || [])
)

const hasAnyStats = computed(() =>
  Object.values(exposure).some(v => v > 0) ||
  risk.vulnTotal > 0 || risk.vulnHigh > 0 || risk.sensitiveInfo > 0
)

// 气泡下钻：页内切换到 Inventory 子 Tab（重复的暴露面独立页已移除）
const inventoryRef = ref(null)
const isDomainTarget = computed(() => props.targetId.startsWith('domain:'))

function goTab(name) {
  activeTab.value = 'inventory'
  nextTick(() => inventoryRef.value?.activateTab(name))
}

function goVulnTab() {
  activeTab.value = 'vulnerabilities'
}

// 目录/JS/敏感信息保留独立页面，跳转并携带 rootDomain/ip 预过滤参数
function exposureQuery() {
  return isDomainTarget.value
    ? { rootDomain: meta.value?.targetValue }
    : { ip: meta.value?.targetValue }
}

function goExposurePage(type) {
  router.push({ path: `/asset-management/exposure/${type}`, query: exposureQuery() })
}

function goSensitivePage() {
  router.push({ path: '/asset-management/risk/sensitive-info', query: exposureQuery() })
}

async function fetchMeta() {
  try {
    const res = await getAssetTargetDetail({ targetId: props.targetId })
    if (res?.data?.meta) {
      meta.value = res.data.meta
      Object.assign(risk, {
        vulnTotal: res.data.risk?.vulnTotal || 0,
        vulnHigh: res.data.risk?.vulnHigh || 0,
        sensitiveInfo: res.data.risk?.sensitiveInfo || 0,
        sensitiveDir: res.data.risk?.sensitiveDir || 0,
        vulnItems: res.data.risk?.vulnItems || [],
        sensitiveInfoItems: res.data.risk?.sensitiveInfoItems || [],
      })
      Object.assign(exposure, {
        subdomains: res.data.exposure?.subdomains || 0,
        ips: res.data.exposure?.ips || 0,
        ports: res.data.exposure?.ports || 0,
        sites: res.data.exposure?.sites || 0,
        icons: res.data.exposure?.icons || 0,
        apps: res.data.exposure?.apps || 0,
        dirs: res.data.exposure?.dirs || 0,
        js: res.data.exposure?.js || 0,
        screenshots: res.data.exposure?.screenshots || 0,
      })
    }
  } catch (err) {
    console.error('[TargetDetailView] fetchMeta error:', err)
  }
}

function handleBack() {
  emit('back')
}

function handleSwitchTarget(id) {
  emit('back', id)
}

function handleViewAsset(asset) {
  assetSheetId.value = asset.id
  assetSheetOpen.value = true
}

function severityTagType(severity) {
  if (severity === 'critical' || severity === 'high') return 'danger'
  if (severity === 'medium') return 'warning'
  return 'info'
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(fetchMeta, 5000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(() => props.targetId, () => {
  fetchMeta()
})

onMounted(() => {
  fetchMeta()
  startPolling()
})

onUnmounted(stopPolling)
</script>

<style scoped lang="scss">
.target-detail-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: var(--el-bg-color);
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;

    .back-button {
      font-size: 18px;
      color: var(--el-text-color-primary);
    }

    .internal-badge {
      display: inline-flex;
      align-items: center;
      padding: 2px 8px;
      border-radius: 4px;
      font-size: 12px;
      line-height: 18px;
      background: #f0f9ff;
      color: #409eff;
      border: 1px solid rgba(64, 158, 255, 0.2);
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 16px;

    .last-scan-time {
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }
  }
}

.detail-tabs {
  :deep(.el-tabs__content) {
    padding: 0;
  }
}

.bubble-bar {
  padding: 10px 20px;
  background: var(--el-bg-color);
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);

  .bubble-row {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;

    .bubble {
      cursor: pointer;

      &:hover {
        opacity: 0.8;
      }
    }
  }
}

.vuln-tab {
  display: flex;
  flex-direction: column;
  gap: 16px;

  .vuln-stats {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;

    .stat-card {
      flex: 1;
      min-width: 140px;
      padding: 14px 16px;
      border: 1px solid var(--el-border-color-light);
      border-radius: 8px;
      background: var(--el-bg-color);

      .stat-value {
        font-size: 24px;
        font-weight: 600;
        color: var(--el-text-color-primary);
      }

      .stat-label {
        font-size: 12px;
        color: var(--el-text-color-secondary);
        margin-top: 2px;
      }

      &.danger .stat-value { color: #f56c6c; }
      &.warning .stat-value { color: #e6a23c; }
    }
  }

  .vuln-name {
    margin-left: 8px;
    font-weight: 500;
  }

  .vuln-url {
    color: var(--el-color-primary);
    text-decoration: none;
    word-break: break-all;

    &:hover { text-decoration: underline; }
  }

  .vuln-empty {
    padding: 48px 24px;
    text-align: center;
    color: var(--el-text-color-secondary);
    background: var(--el-bg-color);
    border-radius: 8px;
    border: 1px solid var(--el-border-color-light);
  }
}

.muted {
  color: var(--el-text-color-secondary);
}

.mono {
  font-family: var(--el-font-family-mono, monospace);
}
</style>
