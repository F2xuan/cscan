<template>
  <div class="ip-page">
    <!-- 搜索区域 -->
    <el-card class="search-card">
      <el-form :inline="true" class="search-form">
        <el-form-item label="IP">
          <el-input v-model="searchForm.ip" :placeholder="$t('ip.ipAddress')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item :label="$t('ip.port')">
          <el-input v-model="searchForm.port" :placeholder="$t('ip.port')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item :label="$t('ip.service')">
          <el-input v-model="searchForm.service" :placeholder="$t('ip.serviceType')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item :label="$t('ip.location')">
          <el-input v-model="searchForm.location" :placeholder="$t('ip.location')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item :label="$t('ip.organization')">
          <el-select v-model="searchForm.orgId" :placeholder="$t('common.allOrganizations')" clearable style="width: 140px">
            <el-option :label="$t('common.allOrganizations')" value="" />
            <el-option v-for="org in organizations" :key="org.id" :label="org.name" :value="org.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">{{ $t('common.search') }}</el-button>
          <el-button @click="handleReset">{{ $t('common.reset') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 统计信息 -->
    <el-row :gutter="16" class="stat-row">
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.total }}</div>
          <div class="stat-label">{{ $t('ip.totalIPs') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.portCount }}</div>
          <div class="stat-label">{{ $t('ip.openPorts') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.serviceCount }}</div>
          <div class="stat-label">{{ $t('ip.serviceTypes') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.newCount }}</div>
          <div class="stat-label">{{ $t('ip.newIPs') }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 数据表格 -->
    <el-card class="table-card">
      <div class="table-header">
        <span class="total-info">{{ $t('common.total') }} {{ pagination.total }} IP</span>
        <div class="table-actions">
          <el-button type="primary" size="small" :disabled="selectedRows.length === 0" @click="handleScan">
            {{ $t('common.scanSelected') }} ({{ selectedRows.length }})
          </el-button>
          <el-button type="danger" size="small" :disabled="selectedRows.length === 0" @click="handleBatchDelete">
            {{ $t('common.batchDelete') }} ({{ selectedRows.length }})
          </el-button>
        </div>
      </div>
      
      <el-table :data="tableData" v-loading="loading" stripe @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="40" />
        <el-table-column :label="$t('ip.ipAddress')" width="180">
          <template #default="{ row }">
            <div class="ip-cell">
              <span class="ip-text">{{ row.ip }}</span>
              <el-tag v-if="row.isNew" type="success" size="small" effect="dark" class="new-tag">{{ $t('common.new') }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('ip.location')" width="200">
          <template #default="{ row }">
            {{ row.location || '-' }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('ip.openPorts')" min-width="300">
          <template #default="{ row }">
            <div class="port-list">
              <el-tag 
                v-for="port in (row.ports)" 
                :key="port.port" 
                size="small" 
                :type="getPortType(port.service)"
                class="port-tag"
              >
                {{ port.port }}<span v-if="port.service">/{{ port.service }}</span>
              </el-tag>
              <span v-if="(row.ports || []).length > 10" class="more-ports">
                +{{ (row.ports || []).length - 10 }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('ip.relatedDomains')" min-width="200">
          <template #default="{ row }">
            <div v-if="row.domains && row.domains.length > 0" class="domain-list">
              <div v-for="domain in row.domains.slice(0, 5)" :key="domain" class="domain-item">
                {{ domain }}
              </div>
              <div v-if="row.domains.length > 5" class="more-domains">
                +{{ row.domains.length - 5 }} {{ $t('common.more') }}...
              </div>
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('ip.organization')" width="120">
          <template #default="{ row }">
            {{ row.orgName || $t('common.defaultOrganization') }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.updateTime')" width="160">
          <template #default="{ row }">
            {{ row.updateTime }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="100" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showDetail(row)">{{ $t('common.detail') }}</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">{{ $t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        class="pagination"
        @size-change="loadData"
        @current-change="loadData"
      />
    </el-card>

    <!-- 详情对话框 -->
    <el-dialog v-model="detailVisible" :title="$t('ip.ipDetail')" width="700px">
      <el-descriptions v-if="currentIP" :column="2" border>
        <el-descriptions-item :label="$t('ip.ipAddress')">{{ currentIP.ip }}</el-descriptions-item>
        <el-descriptions-item :label="$t('ip.location')">{{ currentIP.location || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('ip.asn')">{{ currentIP.asn || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('ip.isp')">{{ currentIP.isp || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('ip.openPorts')" :span="2">
          <div class="detail-ports">
            <el-tag 
              v-for="port in (currentIP.ports || [])" 
              :key="port.port" 
              size="small"
              style="margin: 2px"
            >
              {{ port.port }}<span v-if="port.service">/{{ port.service }}</span>
            </el-tag>
          </div>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('ip.relatedDomains')" :span="2">
          <div v-if="currentIP.domains && currentIP.domains.length > 0">
            <el-tag v-for="domain in currentIP.domains" :key="domain" type="info" size="small" style="margin: 2px">
              {{ domain }}
            </el-tag>
          </div>
          <span v-else>-</span>
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/api/request'
import { useWorkspaceStore } from '@/stores/workspace'

const { t } = useI18n()
const route = useRoute()
const workspaceStore = useWorkspaceStore()
const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const organizations = ref([])
const detailVisible = ref(false)
const currentIP = ref(null)

const searchForm = reactive({
  ip: '',
  port: '',
  service: '',
  location: '',
  orgId: ''
})

const stat = reactive({
  total: 0,
  portCount: 0,
  serviceCount: 0,
  newCount: 0
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

function handleWorkspaceChanged() {
  pagination.page = 1
  loadData()
  loadStat()
}

onMounted(() => {
  if (route.query.ip) {
    searchForm.ip = String(route.query.ip)
  }
  loadData()
  loadStat()
  loadOrganizations()
  window.addEventListener('workspace-changed', handleWorkspaceChanged)
})

onUnmounted(() => {
  window.removeEventListener('workspace-changed', handleWorkspaceChanged)
})

async function loadData() {
  loading.value = true
  try {
    const res = await request.post('/asset/ip/list', {
      page: pagination.page,
      pageSize: pagination.pageSize,
      ip: searchForm.ip,
      port: searchForm.port,
      service: searchForm.service,
      location: searchForm.location,
      orgId: searchForm.orgId
    })
    if (res.code === 0) {
      tableData.value = res.list || []
      pagination.total = res.total || 0
    }
  } finally {
    loading.value = false
  }
}

async function loadStat() {
  try {
    const res = await request.post('/asset/ip/stat', {})
    if (res.code === 0) {
      stat.total = res.total || 0
      stat.portCount = res.portCount || 0
      stat.serviceCount = res.serviceCount || 0
      stat.newCount = res.newCount || 0
    }
  } catch (e) {
    console.error('Failed to load stat:', e)
  }
}

async function loadOrganizations() {
  try {
    const res = await request.post('/organization/list', { page: 1, pageSize: 100 })
    if (res.code === 0) {
      organizations.value = res.list || []
    }
  } catch (e) {
    console.error('Failed to load organizations:', e)
  }
}

function handleSearch() {
  pagination.page = 1
  loadData()
}

function handleReset() {
  Object.assign(searchForm, { ip: '', port: '', service: '', location: '', orgId: '' })
  handleSearch()
}

function handleSelectionChange(rows) {
  selectedRows.value = rows
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('ip.confirmDeleteIP'), t('common.tip'), { type: 'warning' })
  const res = await request.post('/asset/ip/delete', { ip: row.ip })
  if (res.code === 0) {
    ElMessage.success(t('common.deleteSuccess'))
    loadData()
    loadStat()
  }
}

async function handleBatchDelete() {
  if (selectedRows.value.length === 0) return
  await ElMessageBox.confirm(t('ip.confirmBatchDeleteIP', { count: selectedRows.value.length }), t('common.tip'), { type: 'warning' })
  const ips = selectedRows.value.map(row => row.ip)
  const res = await request.post('/asset/ip/batchDelete', { ips })
  if (res.code === 0) {
    ElMessage.success(t('common.deleteSuccess'))
    selectedRows.value = []
    loadData()
    loadStat()
  }
}

function showDetail(row) {
  currentIP.value = row
  detailVisible.value = true
}

function handleScan() {
  ElMessage.info(t('common.featureInDevelopment'))
}

function getPortType(service) {
  if (!service) return 'info'
  if (['http', 'https'].includes(service)) return 'success'
  if (['ssh', 'ftp', 'telnet'].includes(service)) return 'warning'
  if (['mysql', 'redis', 'mongodb'].includes(service)) return 'danger'
  return 'info'
}
</script>

<style scoped>
.ip-page {
  .search-card {
    margin-bottom: 16px;
  }
  
  .stat-row {
    margin-bottom: 16px;
    
    .stat-card {
      text-align: center;
      
      .stat-value {
        font-size: 28px;
        font-weight: 600;
        color: var(--el-color-primary);
      }
      
      .stat-label {
        color: var(--el-text-color-secondary);
        margin-top: 8px;
      }
    }
  }
  
  .table-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    
    .total-info {
      color: var(--el-text-color-secondary);
    }
  }
  
  .ip-cell {
    display: flex;
    align-items: center;
    gap: 8px;
    
    .new-tag {
      flex-shrink: 0;
    }
  }
  
  .ip-text {
    font-family: monospace;
    font-weight: 500;
  }
  
  .port-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    
    .port-tag {
      font-family: monospace;
    }
    
    .more-ports {
      color: var(--el-text-color-secondary);
      font-size: 12px;
      line-height: 22px;
    }
  }
  
  .domain-count {
    color: var(--el-color-primary);
    cursor: pointer;
    
    &:hover {
      text-decoration: underline;
    }
  }
  
  .domain-list {
    .domain-item {
      font-family: monospace;
      font-size: 12px;
      line-height: 1.6;
      color: var(--el-text-color-regular);
      
      &:hover {
        color: var(--el-color-primary);
      }
    }
    
    .more-domains {
      font-size: 12px;
      color: var(--el-text-color-secondary);
      cursor: pointer;
      
      &:hover {
        color: var(--el-color-primary);
      }
    }
  }
  
  .pagination {
    margin-top: 16px;
    justify-content: flex-end;
  }
  
  .detail-ports {
    max-height: 150px;
    overflow-y: auto;
  }
}
</style>

