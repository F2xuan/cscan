<template>
  <div class="domain-page">
    <!-- 搜索区域 -->
    <el-card class="search-card">
      <el-form :inline="true" class="search-form">
        <el-form-item :label="$t('domain.domain')">
          <el-input v-model="searchForm.domain" :placeholder="$t('domain.domain')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item :label="$t('domain.rootDomain')">
          <el-input v-model="searchForm.rootDomain" :placeholder="$t('domain.rootDomain')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item label="IP">
          <el-input v-model="searchForm.ip" :placeholder="$t('domain.resolveIP')" clearable @keyup.enter="handleSearch" />
        </el-form-item>
        <el-form-item :label="$t('domain.organization')">
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
          <div class="stat-label">{{ $t('domain.totalDomains') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.rootDomainCount }}</div>
          <div class="stat-label">{{ $t('domain.rootDomainCount') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.resolvedCount }}</div>
          <div class="stat-label">{{ $t('domain.resolvedCount') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-value">{{ stat.newCount }}</div>
          <div class="stat-label">{{ $t('domain.newDomains') }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 数据表格 -->
    <el-card class="table-card">
      <div class="table-header">
        <span class="total-info">{{ $t('common.total') }} {{ pagination.total }} {{ $t('domain.domain') }}</span>
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
        <el-table-column :label="$t('domain.domain')" min-width="250">
          <template #default="{ row }">
            <div class="domain-cell">
              <span class="domain-name">{{ row.domain }}</span>
              <el-tag v-if="row.isNew" type="success" size="small" effect="dark" class="new-tag">{{ $t('common.new') }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('domain.rootDomain')" width="160">
          <template #default="{ row }">
            {{ row.rootDomain || '-' }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('domain.resolveIP')" min-width="200">
          <template #default="{ row }">
            <div v-if="row.ips && row.ips.length > 0">
              <el-tag v-for="ip in row.ips.slice(0, 3)" :key="ip" size="small" type="info" style="margin-right: 4px">
                {{ ip }}
              </el-tag>
              <span v-if="row.ips.length > 3" class="more-ips">+{{ row.ips.length - 3 }}</span>
            </div>
            <span v-else class="no-resolve">{{ $t('domain.notResolved') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('domain.cname')" width="180">
          <template #default="{ row }">
            {{ row.cname || '-' }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('domain.source')" width="100">
          <template #default="{ row }">
            <el-tag size="small">{{ row.source || 'subfinder' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('domain.organization')" width="120">
          <template #default="{ row }">
            {{ row.orgName || $t('common.defaultOrganization') }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('domain.discoveryTime')" width="160">
          <template #default="{ row }">
            {{ row.createTime }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="80" fixed="right">
          <template #default="{ row }">
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

const searchForm = reactive({
  domain: '',
  rootDomain: '',
  ip: '',
  orgId: ''
})

const stat = reactive({
  total: 0,
  rootDomainCount: 0,
  resolvedCount: 0,
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
  if (route.query.rootDomain) {
    searchForm.rootDomain = String(route.query.rootDomain)
  } else if (route.query.domain) {
    searchForm.domain = String(route.query.domain)
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
    const res = await request.post('/asset/domain/list', {
      page: pagination.page,
      pageSize: pagination.pageSize,
      domain: searchForm.domain,
      rootDomain: searchForm.rootDomain,
      ip: searchForm.ip,
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
    const res = await request.post('/asset/domain/stat', {})
    if (res.code === 0) {
      stat.total = res.total || 0
      stat.rootDomainCount = res.rootDomainCount || 0
      stat.resolvedCount = res.resolvedCount || 0
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
  Object.assign(searchForm, { domain: '', rootDomain: '', ip: '', orgId: '' })
  handleSearch()
}

function handleSelectionChange(rows) {
  selectedRows.value = rows
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('domain.confirmDeleteDomain'), t('common.tip'), { type: 'warning' })
  const res = await request.post('/asset/domain/delete', { id: row.id })
  if (res.code === 0) {
    ElMessage.success(t('common.deleteSuccess'))
    loadData()
    loadStat()
  }
}

async function handleBatchDelete() {
  if (selectedRows.value.length === 0) return
  await ElMessageBox.confirm(t('domain.confirmBatchDelete', { count: selectedRows.value.length }), t('common.tip'), { type: 'warning' })
  const ids = selectedRows.value.map(row => row.id)
  const res = await request.post('/asset/domain/batchDelete', { ids })
  if (res.code === 0) {
    ElMessage.success(t('common.deleteSuccess'))
    selectedRows.value = []
    loadData()
    loadStat()
  }
}

function handleScan() {
  ElMessage.info(t('common.featureInDevelopment'))
}
</script>

<style scoped>
.domain-page {
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
  
  .domain-cell {
    display: flex;
    align-items: center;
    
    .domain-name {
      font-family: monospace;
    }
    
    .new-tag {
      margin-left: 8px;
    }
  }
  
  .more-ips {
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }
  
  .no-resolve {
    color: var(--el-text-color-placeholder);
  }
  
  .pagination {
    margin-top: 16px;
    justify-content: flex-end;
  }
}
</style>

