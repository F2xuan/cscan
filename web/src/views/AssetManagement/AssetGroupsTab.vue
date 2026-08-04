<template>
  <div class="asset-groups-tab">
    <!-- 搜索和操作栏 -->
    <div class="toolbar">
      <el-input
        v-model="searchQuery"
        :placeholder="t('asset.assetGroupsTab.searchPlaceholder')"
        clearable
        class="search-input"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <!-- 类型过滤（域名 / IP） -->
      <el-dropdown @command="handleTypeCommand">
        <el-button>
          <el-icon><Filter /></el-icon>
          {{ currentTypeLabel }}
          <el-icon class="el-icon--right"><ArrowDown /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="all">{{ t('asset.assetGroupsTab.typeAll') }}</el-dropdown-item>
            <el-dropdown-item command="domain" divided>{{ t('asset.assetGroupsTab.typeDomain') }}</el-dropdown-item>
            <el-dropdown-item command="ip">{{ t('asset.assetGroupsTab.typeIp') }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <!-- 删除按钮 -->
      <el-button @click="handleBatchDelete" :disabled="selectedRows.length === 0">
        <el-icon><Delete /></el-icon>
        {{ t('common.delete') }}
      </el-button>

      <!-- 刷新按钮 -->
      <el-button @click="refreshData">
        <el-icon><Refresh /></el-icon>
      </el-button>
    </div>

    <!-- 顶层资产表格 -->
    <el-table
      v-loading="loading"
      :data="targets"
      style="width: 100%"
      class="groups-table"
      row-key="id"
      @selection-change="handleSelectionChange"
      @row-click="handleRowClick"
    >
      <el-table-column type="selection" width="55" reserve-selection />

      <el-table-column :label="t('asset.assetGroupsTab.assetTarget')" min-width="260">
        <template #default="{ row }">
          <div class="group-name-cell">
            <el-tag
              size="small"
              :type="row.targetType === 'ip' ? 'warning' : 'primary'"
              class="type-badge"
            >
              {{ row.targetType === 'ip' ? t('asset.assetGroupsTab.typeIp') : t('asset.assetGroupsTab.typeDomain') }}
            </el-tag>
            <span class="group-name">{{ row.targetValue }}</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('asset.assetGroupsTab.exposure')" min-width="280">
        <template #default="{ row }">
          <div class="bubble-row" @click.stop>
            <el-tag
              v-if="row.exposureSubdomains"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'subdomain')"
            >{{ t('asset.assetGroupsTab.expSubdomains') }} {{ row.exposureSubdomains }}</el-tag>
            <el-tag
              v-if="row.exposureIps"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'ip')"
            >{{ t('asset.assetGroupsTab.expIps') }} {{ row.exposureIps }}</el-tag>
            <el-tag
              v-if="row.exposurePorts"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'port')"
            >{{ t('asset.assetGroupsTab.expPorts') }} {{ row.exposurePorts }}</el-tag>
            <el-tag
              v-if="row.exposureSites"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'site')"
            >{{ t('asset.assetGroupsTab.expSites') }} {{ row.exposureSites }}</el-tag>
            <el-tag
              v-if="row.exposureIcons"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'icon')"
            >{{ t('asset.assetGroupsTab.expIcons') }} {{ row.exposureIcons }}</el-tag>
            <el-tag
              v-if="row.exposureApps"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'app')"
            >{{ t('asset.assetGroupsTab.expApps') }} {{ row.exposureApps }}</el-tag>
            <el-tag
              v-if="row.exposureDirs"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'dir')"
            >{{ t('asset.assetGroupsTab.expDirs') }} {{ row.exposureDirs }}</el-tag>
            <el-tag
              v-if="row.exposureJs"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'js')"
            >{{ t('asset.assetGroupsTab.expJs') }} {{ row.exposureJs }}</el-tag>
            <el-tag
              v-if="row.exposureScreenshots"
              size="small"
              type="info"
              class="bubble clickable"
              @click="goExposure(row, 'screenshot')"
            >{{ t('asset.assetGroupsTab.expScreenshots') }} {{ row.exposureScreenshots }}</el-tag>
            <span v-if="!hasAnyExposure(row)" class="bubble-empty">-</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('asset.assetGroupsTab.risk')" min-width="260">
        <template #default="{ row }">
          <div class="bubble-row" @click.stop>
            <el-tag
              v-if="row.riskVulnTotal"
              size="small"
              :type="row.riskVulnHigh > 0 ? 'danger' : 'warning'"
              class="bubble clickable"
              @click="goRisk(row, 'vuln')"
            >{{ t('asset.assetGroupsTab.riskVulnTotal') }} {{ row.riskVulnTotal }}</el-tag>
            <el-tag
              v-if="row.riskVulnHigh"
              size="small"
              type="danger"
              class="bubble clickable"
              @click="goRisk(row, 'vuln')"
            >{{ t('asset.assetGroupsTab.riskVulnHigh') }} {{ row.riskVulnHigh }}</el-tag>
            <el-tag
              v-if="row.riskSensitiveInfo"
              size="small"
              type="warning"
              class="bubble clickable"
              @click="goRisk(row, 'sensitive-info')"
            >{{ t('asset.assetGroupsTab.riskSensitiveInfo') }} {{ row.riskSensitiveInfo }}</el-tag>
            <span v-if="!hasAnyRisk(row)" class="bubble-empty">-</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('asset.assetGroupsTab.lastUpdated')" width="180" sortable>
        <template #default="{ row }">
          <div class="time-cell">
            {{ formatTimestamp(row.lastScanTime) }}
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('asset.assetGroupsTab.operations')" width="80" fixed="right" align="center">
        <template #default="{ row }">
          <div @click.stop>
            <el-dropdown @command="(cmd) => handleAction(cmd, row)">
              <el-button text>
                <el-icon><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="view">
                    <el-icon><View /></el-icon>
                    {{ t('asset.assetGroupsTab.viewAssets') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="delete" divided>
                    <el-icon><Delete /></el-icon>
                    {{ t('common.delete') }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <el-pagination
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      :total="total"
      :page-sizes="[5, 10, 20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      class="pagination"
      @size-change="loadData"
      @current-change="loadData"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { debounce } from 'lodash-es'
import {
  Search,
  Filter,
  Refresh,
  Delete,
  ArrowDown,
  MoreFilled,
  View
} from '@element-plus/icons-vue'
import { getAssetTargetList, deleteAssetTarget } from '@/api/asset'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const searchQuery = ref('')
const targetTypeFilter = ref('') // '' | 'domain' | 'ip'
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const targets = ref([])
const selectedRows = ref([])

const currentTypeLabel = computed(() => {
  if (targetTypeFilter.value === 'domain') return t('asset.assetGroupsTab.typeDomain')
  if (targetTypeFilter.value === 'ip') return t('asset.assetGroupsTab.typeIp')
  return t('asset.assetGroupsTab.typeAll')
})

const hasAnyExposure = (row) =>
  (row.exposureSubdomains || 0) + (row.exposureIps || 0) +
  (row.exposurePorts || 0) + (row.exposureSites || 0) +
  (row.exposureIcons || 0) + (row.exposureApps || 0) +
  (row.exposureDirs || 0) + (row.exposureJs || 0) +
  (row.exposureScreenshots || 0) > 0

const hasAnyRisk = (row) =>
  (row.riskVulnTotal || 0) + (row.riskVulnHigh || 0) +
  (row.riskSensitiveInfo || 0) > 0

const formatTimestamp = (ms) => {
  if (!ms) return '-'
  const d = new Date(ms)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const handleTypeCommand = (command) => {
  targetTypeFilter.value = command === 'all' ? '' : command
  currentPage.value = 1
  loadData()
}

const handleSelectionChange = (selection) => {
  selectedRows.value = selection
}

const handleRowClick = (row) => {
  viewAssets(row)
}

const handleAction = (command, row) => {
  if (command === 'view') {
    viewAssets(row)
  } else if (command === 'delete') {
    deleteTarget(row)
  }
}

const handleBatchDelete = async () => {
  if (selectedRows.value.length === 0) return

  try {
    await ElMessageBox.confirm(
      t('asset.assetGroupsTab.confirmBatchDelete', { count: selectedRows.value.length }),
      t('common.batchDelete'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )

    loading.value = true
    try {
      let totalDeleted = 0
      const results = await Promise.all(
        selectedRows.value.map(row =>
          deleteAssetTarget({ targetId: row.id, deleteAssets: true })
        )
      )
      results.forEach(res => {
        if (res.code === 0) {
          totalDeleted += res.deletedCount || 0
        }
      })
      ElMessage.success(`${t('asset.assetGroupsTab.deleteSuccess')} (${t('asset.assetGroupsTab.deletedCount')}: ${totalDeleted})`)
      selectedRows.value = []
      await loadData()
    } catch (error) {
      ElMessage.error(t('asset.assetGroupsTab.deleteFailed'))
    } finally {
      loading.value = false
    }
  } catch {
    // cancelled
  }
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await getAssetTargetList({
      page: currentPage.value,
      pageSize: pageSize.value,
      query: searchQuery.value || undefined,
      targetType: targetTypeFilter.value || undefined
    })
    if (res.code === 0) {
      targets.value = res.list || []
      total.value = res.total || 0
    } else {
      ElMessage.error(res.msg || t('common.loadFailed'))
    }
  } catch (error) {
    ElMessage.error(t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

const handleSearch = debounce(() => {
  currentPage.value = 1
  loadData()
}, 300)

const refreshData = () => {
  loadData()
  ElMessage.success(t('asset.assetGroupsTab.refreshSuccess'))
}

defineExpose({ refreshData, loadData })

const viewAssets = (row) => {
  // 按顶层目标类型跳到对应暴露面页，并把目标值作为预过滤参数
  if (row.targetType === 'ip') {
    router.push({
      path: '/asset-management/exposure/ip',
      query: { ip: row.targetValue }
    })
  } else {
    router.push({
      path: '/asset-management/exposure/subdomain',
      query: { rootDomain: row.targetValue }
    })
  }
}

const goExposure = (row, type) => {
  const query = {}
  if (type === 'subdomain' || type === 'site') {
    query.rootDomain = row.targetValue
  } else if (type === 'ip') {
    query.ip = row.targetValue
  }
  router.push({ path: `/asset-management/exposure/${type}`, query })
}

const goRisk = (row, type) => {
  const query = {}
  if (row.targetType === 'domain') {
    query.rootDomain = row.targetValue
  } else {
    query.ip = row.targetValue
  }
  router.push({ path: `/asset-management/risk/${type}`, query })
}

const deleteTarget = async (row) => {
  try {
    await ElMessageBox.confirm(
      t('asset.assetGroupsTab.confirmDelete', { name: row.targetValue }),
      t('common.warning'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )
    loading.value = true
    try {
      const res = await deleteAssetTarget({ targetId: row.id, deleteAssets: true })
      if (res.code === 0) {
        ElMessage.success(`${t('asset.assetGroupsTab.deleteSuccess')} (${t('asset.assetGroupsTab.deletedCount')}: ${res.deletedCount || 0})`)
        await loadData()
      } else {
        ElMessage.error(res.msg || t('asset.assetGroupsTab.deleteFailed'))
      }
    } catch (error) {
      ElMessage.error(t('asset.assetGroupsTab.deleteFailed'))
    } finally {
      loading.value = false
    }
  } catch {
    // cancelled
  }
}

onMounted(() => {
  loadData()
})
</script>

<style lang="scss" scoped>
.asset-groups-tab {
  .toolbar {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;

    .search-input {
      flex: 1;
      max-width: 400px;
    }
  }

  .groups-table {
    margin-bottom: 16px;

    :deep(.el-table__row) {
      cursor: pointer;

      &:hover {
        background-color: hsl(var(--muted) / 0.5);
      }
    }

    :deep(.el-table__cell) {
      .cell {
        > div {
          background: transparent !important;
        }
      }
    }

    .group-name-cell {
      display: flex;
      align-items: center;
      gap: 10px;

      .type-badge {
        flex-shrink: 0;
      }

      .group-name {
        font-weight: 500;
        color: hsl(var(--foreground));
        font-size: 14px;
        word-break: break-all;
      }
    }

    .bubble-row {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    .bubble {
      font-variant-numeric: tabular-nums;

      &.clickable {
        cursor: pointer;

        &:hover {
          opacity: 0.8;
          text-decoration: underline;
        }
      }
    }

    .bubble-empty {
      color: hsl(var(--muted-foreground));
      font-size: 13px;
    }

    .time-cell {
      color: hsl(var(--muted-foreground));
      font-size: 13px;
    }
  }

  .pagination {
    margin-top: 16px;
  }
}
</style>
