<template>
  <div class="cron-task-page">
    <!-- 操作栏 -->
    <el-card class="action-card">
      <el-button type="primary" @click="goCreate">
        <el-icon><Plus /></el-icon>{{ $t('cronTask.newCronTask') }}
      </el-button>
      <el-button @click="loadData">
        <el-icon><Refresh /></el-icon>{{ $t('common.refresh') }}
      </el-button>
      <el-button 
        type="danger" 
        :disabled="selectedRows.length === 0"
        @click="handleBatchDelete"
      >
        <el-icon><Delete /></el-icon>{{ $t('common.batchDelete') }} {{ selectedRows.length > 0 ? `(${selectedRows.length})` : '' }}
      </el-button>
    </el-card>

    <!-- 数据表格 -->
    <el-card class="table-card">
      <el-table 
        :data="tableData" 
        v-loading="loading" 
        stripe
        @selection-change="handleCronSelectionChange"
      >
        <el-table-column type="selection" width="50" />
        <el-table-column prop="name" :label="$t('cronTask.cronTaskName')" min-width="140" />
        <!-- 目标来源标签：手动输入/资产选择/空间引擎 -->
        <el-table-column :label="$t('cronTask.targetSource')" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.targetMode === 'asset' || (row.assetIds && row.assetIds.length)" type="success" size="small">{{ $t('cronTask.targetSourceAsset') }}</el-tag>
            <el-tag v-else-if="row.taskType === 'space_engine' || row.targetMode === 'space_engine'" type="warning" size="small">{{ $t('cronTask.targetSourceSpaceEngine') }}</el-tag>
            <el-tag v-else type="info" size="small">{{ $t('cronTask.targetSourceManual') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="targetShort" :label="$t('cronTask.scanTarget')" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.targetMode === 'asset' || (row.assetIds && row.assetIds.length)" class="text-muted">
              {{ $t('cronTask.selectedAssets', { count: (row.assetIds && row.assetIds.length) || 0 }) }}
            </span>
            <span v-else>{{ truncateTarget(row.target) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('cronTask.scheduleType')" width="180">
          <template #default="{ row }">
            <div v-if="row.scheduleType === 'cron'">
              <el-tag type="primary" size="small">{{ $t('cronTask.cronExec').split(' ')[0] }}</el-tag>
              <el-tooltip :content="getCronDescription(row.cronSpec)" placement="top">
                <code class="cron-code">{{ row.cronSpec }}</code>
              </el-tooltip>
            </div>
            <div v-else>
              <el-tag type="warning" size="small">{{ $t('cronTask.onceExec').split(' ')[0] }}</el-tag>
              <span class="schedule-time">{{ row.scheduleTime }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('cronTask.status')" width="80">
          <template #default="{ row }">
            <el-switch
              v-model="row.status"
              active-value="enable"
              inactive-value="disable"
              @change="handleToggle(row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="nextRunTime" :label="$t('cronTask.nextRunTime')" width="160">
          <template #default="{ row }">
            <span v-if="row.status === 'enable' && row.nextRunTime">{{ row.nextRunTime }}</span>
            <span v-else class="text-muted">{{ row.status === 'disable' ? $t('cronTask.disabled') : '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="lastRunTime" :label="$t('cronTask.lastRunTime')" width="160">
          <template #default="{ row }">
            {{ row.lastRunTime || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="runCount" :label="$t('cronTask.runCount')" width="90">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.runCount || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="200" fixed="right">
          <template #default="{ row }">
            <el-button type="success" link size="small" @click="handleRunNow(row)">
              <el-icon><VideoPlay /></el-icon>{{ $t('cronTask.runNow') }}
            </el-button>
            <el-button type="primary" link size="small" @click="goEdit(row)">
              <el-icon><Edit /></el-icon>{{ $t('common.edit') }}
            </el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon>{{ $t('common.delete') }}
            </el-button>
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
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Edit, Delete, VideoPlay } from '@element-plus/icons-vue'
import { 
  getCronTaskList, 
  toggleCronTask, 
  deleteCronTask,
  batchDeleteCronTask,
  runCronTaskNow
} from '@/api/crontask'

const router = useRouter()
const { t } = useI18n()
const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 加载数据
async function loadData() {
  loading.value = true
  try {
    const res = await getCronTaskList({
      page: pagination.page,
      pageSize: pagination.pageSize,
      keyword: '',
      taskType: 'scan'
    })
    if (res.code === 0) {
      tableData.value = res.data?.list || res.list || []
      pagination.total = res.data?.total || res.total || 0
    } else {
      console.error('loadCronTaskFailed:', res.msg)
    }
  } catch (error) {
    console.error('loadCronTaskFailed:', error)
  } finally {
    loading.value = false
  }
}

function handleCronSelectionChange(val) {
  selectedRows.value = val
}

function handleBatchDelete() {
  if (!selectedRows.value.length) return
  ElMessageBox.confirm(t('cronTask.confirmBatchDelete', { count: selectedRows.value.length }), t('common.tip'), { type: 'warning' }).then(async () => {
    const res = await batchDeleteCronTask({ ids: selectedRows.value.map(item => item.id) })
    if (res.code === 0) {
      ElMessage.success(t('cronTask.deleteSuccess'))
      loadData()
    } else {
      ElMessage.error(res.msg || t('common.deleteFailed'))
    }
  }).catch(() => {})
}

// 状态切换
async function handleToggle(row) {
  try {
    const res = await toggleCronTask({ id: row.id, status: row.status })
    if (res.code === 0) {
      ElMessage.success(t('cronTask.statusUpdateSuccess'))
      loadData()
    } else {
      row.status = row.status === 'enable' ? 'disable' : 'enable'
      ElMessage.error(res.msg || t('common.updateFailed'))
    }
  } catch (err) {
    row.status = row.status === 'enable' ? 'disable' : 'enable'
  }
}

// 立即执行
async function handleRunNow(row) {
  try {
    await ElMessageBox.confirm(t('cronTask.confirmRunNow'), t('common.tip'), { type: 'warning' })
    const res = await runCronTaskNow({ id: row.id })
    if (res.code === 0) {
      ElMessage.success(t('cronTask.runSuccess'))
      loadData()
    } else {
      ElMessage.error(res.msg || t('cronTask.runFailed'))
    }
  } catch {}
}

function handleDelete(row) {
  ElMessageBox.confirm(t('cronTask.confirmDelete'), t('common.tip'), { type: 'warning' }).then(async () => {
    const res = await deleteCronTask({ id: row.id })
    if (res.code === 0) {
      ElMessage.success(t('cronTask.deleteSuccess'))
      loadData()
    } else {
      ElMessage.error(res.msg || t('common.deleteFailed'))
    }
  }).catch(() => {})
}

// 跳转到新建页
function goCreate() {
  router.push('/cron-task/create')
}

// 跳转到编辑页
function goEdit(row) {
  router.push(`/cron-task/edit/${row.id}`)
}

// 获取Cron表达式的中文描述（简单映射）
function getCronDescription(cronSpec) {
  if (!cronSpec) return ''
  const presets = [
    { label: t('cronTask.everyHour'), value: '0 0 * * * *' },
    { label: t('cronTask.everyDay2am'), value: '0 0 2 * * *' },
    { label: t('cronTask.everyMonday'), value: '0 0 3 * * 1' },
    { label: t('cronTask.every6hours'), value: '0 0 */6 * * *' }
  ]
  const preset = presets.find(p => p.value === cronSpec)
  if (preset) return preset.label
  return t('cronTask.customCron')
}

// 截取目标显示
function truncateTarget(target, maxLen = 40) {
  if (!target) return ''
  const firstLine = target.split('\n')[0]
  if (firstLine.length > maxLen) {
    return firstLine.substring(0, maxLen) + '...'
  }
  return firstLine
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.cron-task-page {
  padding: 20px;
}

.action-card {
  margin-bottom: 20px;
}

.table-card {
  margin-bottom: 20px;
}

.pagination {
  margin-top: 20px;
  justify-content: flex-end;
}

.cron-code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  margin-left: 6px;
}

.schedule-time {
  font-size: 12px;
  color: var(--el-text-color-regular);
  margin-left: 6px;
}

.text-muted {
  color: var(--el-text-color-placeholder);
}
</style>
