<template>
  <div class="port-view">
    <ProTable
      ref="proTableRef"
      api="/asset/port/list"
      rowKey="port"
      :columns="portColumns"
      :searchItems="searchItems"
      selection
      :searchPlaceholder="searchPortPlaceholder"
      @data-changed="$emit('data-changed')"
      :searchKeys="['port']"
    >
      <template #toolbar-left>
        <el-dropdown @command="handleExport">
          <el-button type="success" size="default">
            {{ $t('common.export') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="selected-ports" :disabled="selectedRows.length === 0">{{ $t('common.exportSelected') || '导出选中' }}</el-dropdown-item>
              <el-dropdown-item divided command="all-ports">{{ $t('common.exportAll') || '导出所有' }}</el-dropdown-item>
              <el-dropdown-item command="csv">{{ $t('common.exportCsv') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>

      <template #toolbar-right>
        <el-button type="danger" plain @click="handleClear">{{ $t('asset.clearData') || '清空数据' }}</el-button>
      </template>

      <!-- 列: 端口 -->
      <template #port="{ row }">
        <div class="port-cell">
          <el-tag type="primary" size="large" effect="dark" class="port-tag">{{ row.port }}</el-tag>
        </div>
      </template>

      <!-- 列: 出现次数(数量) -->
      <template #assetCount="{ row }">
        <el-tag type="danger">{{ row.assetCount }}</el-tag>
      </template>

      <!-- 列: 关联服务 -->
      <template #services="{ row }">
        <div v-if="row.services && row.services.length > 0">
          <el-tag v-for="svc in row.services.slice(0, 3)" :key="svc" size="small" type="success" style="margin-right: 4px;">{{ svc }}</el-tag>
          <span v-if="row.services.length > 3" class="more-text">+{{ row.services.length - 3 }}</span>
        </div>
        <span v-else class="no-data">-</span>
      </template>

      <!-- 列: 关联资产 (Hosts) -->
      <template #hosts="{ row }">
        <div v-if="row.hosts && row.hosts.length > 0" class="host-list">
          <el-tag v-for="host in row.hosts.slice(0, 3)" :key="host" size="small" type="info" style="margin-right: 4px; margin-bottom: 4px;">{{ host }}</el-tag>
          <span v-if="row.hosts.length > 3" class="more-text">+{{ row.hosts.length - 3 }}</span>
        </div>
        <span v-else class="no-data">-</span>
      </template>

      <!-- 列: 所属组织 -->
      <template #org="{ row }">
        {{ row.orgName || $t('common.defaultOrganization') }}
      </template>

      <!-- 列: 操作 -->
      <template #operation="{ row }">
        <el-button type="primary" link size="small" @click="viewAssets(row)">{{ $t('asset.portView.viewAssets') }}</el-button>
      </template>
    </ProTable>
  </div>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import ProTable from '@/components/common/ProTable.vue'
import { useAssetView } from '@/composables/useAssetView'
import { clearPorts } from '@/api/asset'

const { t } = useI18n()
const emit = defineEmits(['data-changed'])

const {
  proTableRef, organizations, selectedRows,
  loadOrganizations, handleExport
} = useAssetView({
  apiPrefix: '/asset/port',
  viewType: 'port',
  exportHeaders: ['Port', 'AssetCount', 'Services', 'Hosts', 'Organization', 'CreateTime', 'UpdateTime'],
  exportRowFormatter: row => [
    row.port || '',
    row.assetCount || 0,
    (row.services || []).join(';'),
    (row.hosts || []).join(';'),
    row.orgName || '',
    row.createTime || '',
    row.updateTime || ''
  ]
})

async function handleClear() {
  try {
    await ElMessageBox.confirm(
      t('asset.confirmClearAll'),
      t('common.warning'),
      { type: 'error', confirmButtonText: t('asset.confirmClearBtn'), cancelButtonText: t('common.cancel') }
    )
    const res = await clearPorts()
    if (res.code === 0) {
      ElMessage.success(res.msg || t('asset.clearSuccess'))
      proTableRef.value?.loadData()
      emit('data-changed')
    } else {
      ElMessage.error(res.msg || t('asset.clearFailed'))
    }
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(t('asset.clearFailed'))
    }
  }
}

const searchPortPlaceholder = computed(() => t('asset.portNumber') || '搜索端口')

const portColumns = computed(() => [
  { label: t('asset.portView.columns.port'), prop: 'port', slot: 'port', width: 120 },
  { label: t('asset.portView.columns.assetCount'), prop: 'assetCount', slot: 'assetCount', width: 100 },
  { label: t('asset.portView.columns.services'), prop: 'services', slot: 'services', width: 180 },
  { label: t('asset.portView.columns.hosts'), prop: 'hosts', slot: 'hosts', minWidth: 250 },
  { label: t('asset.portView.columns.organization'), prop: 'orgName', slot: 'org', width: 120 },
  { label: t('asset.portView.columns.createTime'), prop: 'createTime', width: 160 },
  { label: t('asset.portView.columns.updateTime'), prop: 'updateTime', width: 160 },
  { label: t('asset.portView.columns.operation'), slot: 'operation', width: 100, fixed: 'right' }
])

const searchItems = computed(() => [
  { label: t('asset.portView.filters.port'), prop: 'port', type: 'input', inputType: 'number' },
  { label: t('asset.portView.filters.host'), prop: 'host', type: 'input' },
  {
    label: t('domain.organization'),
    prop: 'orgId',
    type: 'select',
    options: [
      { label: t('common.allOrganizations'), value: '' },
      ...organizations.value.map(org => ({ label: org.name, value: org.id }))
    ]
  }
])

function viewAssets(row) {
  window.location.href = `/asset-management?tab=inventory&port=${encodeURIComponent(row.port)}`
}

onMounted(() => {
  loadOrganizations()
})

defineExpose({ refresh: () => proTableRef.value?.loadData() })
</script>

<style scoped>
.port-view {
  height: 100%;
}
.port-cell {
  display: flex;
  align-items: center;
}
.port-tag {
  font-family: 'Consolas', 'Monaco', monospace;
  font-weight: bold;
}
.more-text {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: 4px;
}
.no-data {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
.host-list {
  display: flex;
  flex-wrap: wrap;
}
</style>