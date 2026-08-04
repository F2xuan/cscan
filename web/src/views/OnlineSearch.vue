<template>
  <div class="online-search-page">
    <!-- 常驻导入任务进度条 -->
    <div v-if="persistentTask" class="persistent-task-bar">
      <el-card shadow="hover" class="persistent-task-card">
        <div class="persistent-task-content">
          <div class="persistent-task-info">
            <el-icon :size="18" :class="persistentTask.status === 'running' ? 'rotating' : ''">
              <Loading v-if="persistentTask.status === 'running'" />
              <CircleCheck v-else-if="persistentTask.status === 'completed'" color="#67c23a" />
              <CircleClose v-else-if="persistentTask.status === 'failed'" color="#f56c6c" />
            </el-icon>
            <span class="persistent-task-title">
              {{ persistentTask.status === 'running' ? '资产导入进行中' : persistentTask.status === 'completed' ? '资产导入已完成' : '资产导入失败' }}
            </span>
            <el-tag size="small" :type="getPlatformTagType(persistentTask.platform)">
              {{ getPlatformLabel(persistentTask.platform) }}
            </el-tag>
            <el-tag size="small" type="info">
              {{ persistentTask.importType === 'all' ? '导入全部' : '导入当前页' }}
            </el-tag>
          </div>
          <div v-if="persistentTask.status === 'running'" class="persistent-task-progress">
            <el-progress
              :percentage="persistentTask.total > 0 ? Math.min(99, Math.floor(persistentTask.completed / persistentTask.total * 100)) : 0"
              :stroke-width="8"
              style="width: 280px;"
            />
            <span class="persistent-task-stat">{{ persistentTask.completed }}/{{ persistentTask.total || '?' }}</span>
          </div>
          <div v-else-if="persistentTask.status === 'completed'" class="persistent-task-result">
            <el-tag type="success" size="small">成功导入 {{ persistentTask.imported }} 条</el-tag>
            <el-tag v-if="persistentTask.skipped > 0" type="warning" size="small" style="margin-left: 4px;">跳过 {{ persistentTask.skipped }} 条</el-tag>
          </div>
          <div class="persistent-task-actions">
            <el-button v-if="persistentTask.status === 'completed' || persistentTask.status === 'failed'" type="primary" size="small" @click="showImportResultDialog">
              查看结果
            </el-button>
            <el-button type="info" size="small" text @click="dismissPersistentTask">
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 搜索区域 -->
    <el-card class="search-card">
      <el-form :model="store.searchForm" inline>
        <el-form-item :label="$t('onlineSearch.dataSource')">
          <el-select v-model="store.searchForm.source" style="width: 120px" @change="handleSourceChange">
            <el-option label="Fofa" value="fofa" />
            <el-option label="Hunter" value="hunter" />
            <el-option label="Quake" value="quake" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('onlineSearch.queryStatement')" style="flex: 1">
          <el-input
            v-model="store.searchForm.query"
            :placeholder="$t('onlineSearch.queryPlaceholder')"
            style="width: 400px"
            @keyup.enter="handleSearch"
          />
        </el-form-item>
        <el-form-item :label="$t('onlineSearch.quantity')">
          <el-select v-model="store.searchForm.size" style="width: 100px">
            <el-option :value="10" label="10" />
            <el-option :value="50" label="50" />
            <el-option :value="100" label="100" />
            <el-option v-if="store.searchForm.source === 'fofa'" :value="500" label="500" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="handleSearch">{{ $t('onlineSearch.search') }}</el-button>
          <el-button @click="handleImport" :disabled="!tableData.length || importTaskRunning" :loading="importLoading">{{ $t('onlineSearch.importCurrent') }}</el-button>
          <el-button type="success" @click="handleImportAll" :disabled="!total || importTaskRunning" :loading="importAllLoading">{{ $t('onlineSearch.importAll') }}</el-button>
          <el-button type="warning" :icon="Timer" @click="openCronDialog" :disabled="!total">{{ $t('onlineSearch.cronDialog.createTaskBtn') }}</el-button>
          <el-button @click="showHelpDialog">{{ $t('onlineSearch.syntaxHelp') }}</el-button>
        </el-form-item>
      </el-form>

      <!-- 快捷查询 -->
      <div class="quick-search">
        <span class="label">{{ $t('onlineSearch.quickQuery') }}</span>
        <el-tag
          v-for="item in quickQueries"
          :key="item.query"
          class="quick-tag"
          @click="applyQuickQuery(item)"
        >
          {{ item.label }}
        </el-tag>
      </div>
    </el-card>

    <!-- 结果表格 -->
    <el-card class="result-card">
      <template #header>
        <div class="card-header">
          <span>{{ $t('onlineSearch.searchResult') }}</span>
          <span v-if="total > 0" class="total">{{ $t('onlineSearch.total') }} {{ total }} {{ $t('onlineSearch.items') }}</span>
        </div>
      </template>

      <el-table :data="tableData" v-loading="loading" stripe max-height="500">
        <el-table-column type="selection" width="50" />
        <el-table-column prop="host" :label="$t('onlineSearch.host')" min-width="250" show-overflow-tooltip />
        <el-table-column prop="ip" :label="$t('onlineSearch.ip')" width="140" />
        <el-table-column prop="port" :label="$t('onlineSearch.port')" width="80" />
        <el-table-column prop="protocol" :label="$t('onlineSearch.protocol')" width="80" />
        <el-table-column prop="domain" :label="$t('onlineSearch.domain')" min-width="150" show-overflow-tooltip />
        <el-table-column prop="title" :label="$t('onlineSearch.pageTitle')" min-width="200" show-overflow-tooltip />
        <el-table-column prop="server" :label="$t('onlineSearch.server')" width="120" show-overflow-tooltip />
        <el-table-column prop="product" :label="$t('onlineSearch.product')" width="120" show-overflow-tooltip />
        <el-table-column prop="country" :label="$t('onlineSearch.country')" width="80" />
        <el-table-column prop="city" :label="$t('onlineSearch.city')" width="100" />
        <el-table-column prop="icp" :label="$t('onlineSearch.icpRecord')" width="150" show-overflow-tooltip />
      </el-table>

      <el-pagination
        v-if="total > 0"
        v-model:current-page="store.searchForm.page"
        :page-size="store.searchForm.size"
        :total="total"
        layout="total, prev, pager, next"
        class="pagination"
        @current-change="handleSearch"
      />
    </el-card>

    <!-- 语法帮助对话框 -->
    <el-dialog v-model="helpDialogVisible" :title="$t('onlineSearch.syntaxHelp')" width="650px">
      <el-tabs v-model="helpTab">
        <el-tab-pane label="Fofa" name="fofa">
          <div class="syntax-help">
            <p><code>ip="1.1.1.1"</code> - Search by IP</p>
            <p><code>domain="example.com"</code> - Search by domain</p>
            <p><code>title="admin"</code> - Search by title</p>
            <p><code>body="content"</code> - Search by body content</p>
            <p><code>port="80"</code> - Search by port</p>
            <p><code>icp="ICP"</code> - Search by ICP record</p>
            <p><code>org="company"</code> - Search by organization</p>
            <p>Combined: <code>ip="1.1.1.1" && port="80"</code></p>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Hunter" name="hunter">
          <div class="syntax-help">
            <p><code>ip="1.1.1.1"</code> - Search by IP</p>
            <p><code>domain.suffix="example.com"</code> - Search by domain suffix</p>
            <p><code>web.title="admin"</code> - Search by web title</p>
            <p><code>icp.name="company"</code> - Search by ICP name</p>
            <p><code>icp.number="ICP"</code> - Search by ICP number</p>
            <p><code>port="443"</code> - Search by port</p>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Quake" name="quake">
          <div class="syntax-help">
            <p><code>ip:"1.1.1.1"</code> - Search by IP</p>
            <p><code>domain:"example.com"</code> - Search by domain</p>
            <p><code>title:"admin"</code> - Search by title</p>
            <p><code>service:"http"</code> - Search by service</p>
            <p><code>port:"80"</code> - Search by port</p>
            <p><code>country:"CN"</code> - Search by country</p>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- 导入结果对话框 -->
    <el-dialog v-model="importResultDialogVisible" title="导入结果" width="600px">
      <div v-if="importResult" class="import-result">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="任务状态">
            <el-tag :type="importResult.status === 'completed' ? 'success' : importResult.status === 'failed' ? 'danger' : 'info'">
              {{ importResult.status === 'completed' ? '已完成' : importResult.status === 'failed' ? '失败' : importResult.status }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="数据平台">
            {{ getPlatformLabel(importResult.platform) }}
          </el-descriptions-item>
          <el-descriptions-item label="导入类型">
            {{ importResult.importType === 'all' ? '导入全部' : '导入当前页' }}
          </el-descriptions-item>
          <el-descriptions-item label="开始时间">
            {{ importResult.startTime }}
          </el-descriptions-item>
          <el-descriptions-item label="结束时间">
            {{ importResult.endTime || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="耗时">
            {{ formatDuration(importResult.startTime, importResult.endTime) }}
          </el-descriptions-item>
          <el-descriptions-item label="成功导入">
            <span class="result-success">{{ importResult.imported }} 条</span>
          </el-descriptions-item>
          <el-descriptions-item label="跳过(空主机/重复)">
            <span class="result-warning">{{ importResult.skipped }} 条</span>
          </el-descriptions-item>
          <el-descriptions-item v-if="importResult.importType === 'all'" label="API获取总数">
            {{ importResult.totalFetched }} 条
          </el-descriptions-item>
          <el-descriptions-item v-if="importResult.importType === 'all'" label="遍历页数">
            {{ importResult.totalPages }} 页
          </el-descriptions-item>
        </el-descriptions>
        <el-alert v-if="importResult.errorMsg" type="error" :title="importResult.errorMsg" show-icon style="margin-top: 16px;" />
      </div>
      <template #footer>
        <el-button @click="importResultDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 创建定时任务对话框 -->
    <el-dialog v-model="cronDialogVisible" :title="$t('onlineSearch.cronDialog.title')" width="560px" @close="resetCronForm">
      <el-form ref="cronFormRef" :model="cronForm" :rules="cronRules" label-width="110px">
        <el-form-item :label="$t('onlineSearch.cronDialog.taskName')" prop="name">
          <el-input v-model="cronForm.name" :placeholder="$t('onlineSearch.cronDialog.enterTaskName')" />
        </el-form-item>
        <el-form-item :label="$t('onlineSearch.cronDialog.dataSource')">
          <el-input :value="platformLabel" disabled />
        </el-form-item>
        <el-form-item :label="$t('onlineSearch.cronDialog.queryStatement')">
          <el-input v-model="cronForm.query" type="textarea" :rows="2" disabled />
        </el-form-item>
        <el-form-item :label="$t('onlineSearch.cronDialog.scheduleType')" prop="scheduleType">
          <el-radio-group v-model="cronForm.scheduleType">
            <el-radio value="cron">{{ $t('onlineSearch.cronDialog.cycleExec') }}</el-radio>
            <el-radio value="once">{{ $t('onlineSearch.cronDialog.onceExec') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="cronForm.scheduleType === 'cron'" :label="$t('onlineSearch.cronDialog.cronExpression')" prop="cronSpec">
          <el-input v-model="cronForm.cronSpec" :placeholder="$t('onlineSearch.cronDialog.cronPlaceholder')" />
          <div class="cron-presets">
            <el-tag
              v-for="preset in cronPresets"
              :key="preset.value"
              class="preset-tag"
              @click="cronForm.cronSpec = preset.value"
            >
              {{ preset.label }}
            </el-tag>
          </div>
        </el-form-item>
        <el-form-item v-if="cronForm.scheduleType === 'once'" :label="$t('onlineSearch.cronDialog.execTime')" prop="scheduleTime">
          <el-date-picker
            v-model="cronForm.scheduleTime"
            type="datetime"
            :placeholder="$t('onlineSearch.cronDialog.selectExecTime')"
            format="YYYY-MM-DD HH:mm:ss"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item>
          <el-alert type="info" :closable="false" show-icon :title="$t('onlineSearch.cronDialog.infoTip')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cronDialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="cronSubmitting" @click="handleCronSubmit">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Timer, Loading, CircleCheck, CircleClose, Close } from '@element-plus/icons-vue'
import request from '@/api/request'
import { saveSpaceEngineCronTask } from '@/api/crontask'
import { useOnlineSearchStore } from '@/stores/onlineSearch'

const { t } = useI18n()
const store = useOnlineSearchStore()

const loading = ref(false)
const importLoading = ref(false)
const importAllLoading = ref(false)
const helpDialogVisible = ref(false)
const helpTab = ref('fofa')

// ===== 导入任务异步进度 =====
const importTaskRunning = computed(() => {
  return persistentTask.value && persistentTask.value.status === 'running'
})
const persistentTask = ref(null)
const importResultDialogVisible = ref(false)
const importResult = ref(null)
let importPollTimer = null
const ONLINE_IMPORT_STORAGE_KEY = 'cscan_online_import_task'

const platformMap = { fofa: 'Fofa', hunter: 'Hunter', quake: 'Quake' }
const platformTagTypeMap = { fofa: '', hunter: 'success', quake: 'warning' }

function getPlatformLabel(platform) {
  return platformMap[platform] || platform
}
function getPlatformTagType(platform) {
  return platformTagTypeMap[platform] || ''
}

function formatDuration(startStr, endStr) {
  if (!startStr || !endStr) return '-'
  // 支持带毫秒的时间格式 "2006-01-02 15:04:05.000" 和不带毫秒的格式
  const parseTime = (s) => {
    const normalized = s.replace(/-/g, '/')
    return new Date(normalized).getTime()
  }
  const start = parseTime(startStr)
  const end = parseTime(endStr)
  const diffMs = end - start
  if (diffMs < 0) return '-'
  if (diffMs < 1000) return diffMs + ' 毫秒'
  const totalSec = Math.floor(diffMs / 1000)
  const ms = diffMs % 1000
  if (totalSec < 60) {
    return ms > 0 ? totalSec + ' 秒 ' + ms + ' 毫秒' : totalSec + ' 秒'
  }
  const min = Math.floor(totalSec / 60)
  const sec = totalSec % 60
  if (min < 60) return min + ' 分 ' + sec + ' 秒'
  const hour = Math.floor(min / 60)
  const remMin = min % 60
  return hour + ' 时 ' + remMin + ' 分'
}

function saveImportTaskToStorage() {
  if (persistentTask.value) {
    localStorage.setItem(ONLINE_IMPORT_STORAGE_KEY, JSON.stringify(persistentTask.value))
  } else {
    localStorage.removeItem(ONLINE_IMPORT_STORAGE_KEY)
  }
}

function loadImportTaskFromStorage() {
  try {
    const saved = localStorage.getItem(ONLINE_IMPORT_STORAGE_KEY)
    if (saved) {
      const task = JSON.parse(saved)
      if (task && task.taskId) {
        persistentTask.value = task
        startImportPolling()
      }
    }
  } catch (e) {
    console.error('Load import task from storage failed:', e)
  }
}

function dismissPersistentTask() {
  persistentTask.value = null
  if (importPollTimer) {
    clearInterval(importPollTimer)
    importPollTimer = null
  }
  localStorage.removeItem(ONLINE_IMPORT_STORAGE_KEY)
}

function startImportPolling() {
  if (importPollTimer) clearInterval(importPollTimer)
  importPollTimer = setInterval(async () => {
    if (!persistentTask.value || !persistentTask.value.taskId) {
      clearInterval(importPollTimer)
      importPollTimer = null
      return
    }
    try {
      const res = await request.post('/onlineapi/import/progress', { taskId: persistentTask.value.taskId })
      if (res.code === 0) {
        persistentTask.value = {
          ...persistentTask.value,
          status: res.status,
          total: res.total,
          completed: res.completed,
          imported: res.imported,
          skipped: res.skipped,
          platform: res.platform,
          importType: res.importType,
        }

        if (res.status === 'running') {
          saveImportTaskToStorage()
        } else if (res.status === 'completed') {
          clearInterval(importPollTimer)
          importPollTimer = null
          saveImportTaskToStorage()
          ElMessage.success(`资产导入完成，成功导入 ${res.imported} 条`)
        } else if (res.status === 'failed') {
          clearInterval(importPollTimer)
          importPollTimer = null
          saveImportTaskToStorage()
          ElMessage.error('资产导入失败: ' + (res.errorMsg || '未知错误'))
        }
      } else if (res.code === 404) {
        clearInterval(importPollTimer)
        importPollTimer = null
        persistentTask.value = null
        localStorage.removeItem(ONLINE_IMPORT_STORAGE_KEY)
      }
    } catch (e) {
      console.error('Poll import progress error:', e)
    }
  }, 1500)
}

async function showImportResultDialog() {
  if (!persistentTask.value || !persistentTask.value.taskId) return
  try {
    const res = await request.post('/onlineapi/import/result', { taskId: persistentTask.value.taskId })
    if (res.code === 0) {
      importResult.value = res
      importResultDialogVisible.value = true
    } else {
      ElMessage.error(res.msg || '获取结果失败')
    }
  } catch (e) {
    ElMessage.error('获取结果失败: ' + e.message)
  }
}

// ===== 定时任务对话框 =====
const cronDialogVisible = ref(false)
const cronSubmitting = ref(false)
const cronFormRef = ref(null)

const platformLabel = computed(() => platformMap[store.searchForm.source] || store.searchForm.source)

const cronPresets = computed(() => [
  { label: t('onlineSearch.cronDialog.everyDay3am'), value: '0 0 3 * * ?' },
  { label: t('onlineSearch.cronDialog.everyMonday3am'), value: '0 0 3 ? * MON' },
  { label: t('onlineSearch.cronDialog.everyMonth1st3am'), value: '0 0 3 1 * ?' },
  { label: t('onlineSearch.cronDialog.everyHour'), value: '0 0 * * * ?' },
])

function buildDefaultTaskName() {
  const platform = platformLabel.value
  const q = (store.searchForm.query || '').replace(/\s+/g, ' ').trim()
  const truncated = q.length > 20 ? q.slice(0, 20) + '...' : q
  return `${t('onlineSearch.cronDialog.namePrefix')}-${platform}-${truncated || t('onlineSearch.cronDialog.defaultName')}`
}

const cronForm = reactive({
  name: '',
  query: '',
  scheduleType: 'cron',
  cronSpec: '0 0 3 * * ?',
  scheduleTime: null,
})

const cronRules = {
  name: [{ required: true, message: t('onlineSearch.cronDialog.enterTaskName'), trigger: 'blur' }],
  scheduleType: [{ required: true, message: t('onlineSearch.cronDialog.selectScheduleType'), trigger: 'change' }],
  cronSpec: [
    {
      required: true,
      validator: (_rule, value, callback) => {
        if (cronForm.scheduleType !== 'cron') return callback()
        if (!value) return callback(new Error(t('onlineSearch.cronDialog.enterCronSpec')))
        const parts = value.trim().split(/\s+/)
        if (parts.length < 6 || parts.length > 7) {
          return callback(new Error(t('onlineSearch.cronDialog.cronFormatError')))
        }
        callback()
      },
      trigger: 'blur',
    },
  ],
  scheduleTime: [
    {
      required: true,
      validator: (_rule, value, callback) => {
        if (cronForm.scheduleType !== 'once') return callback()
        if (!value) return callback(new Error(t('onlineSearch.cronDialog.selectExecTimeFirst')))
        callback()
      },
      trigger: 'change',
    },
  ],
}

function openCronDialog() {
  if (!total.value) {
    ElMessage.warning(t('onlineSearch.cronDialog.searchFirst'))
    return
  }
  cronForm.name = buildDefaultTaskName()
  cronForm.query = store.searchForm.query
  cronForm.scheduleType = 'cron'
  cronForm.cronSpec = '0 0 3 * * ?'
  cronForm.scheduleTime = null
  cronDialogVisible.value = true
}

function resetCronForm() {
  cronFormRef.value?.resetFields()
}

async function handleCronSubmit() {
  if (!cronFormRef.value) return
  await cronFormRef.value.validate(async (valid) => {
    if (!valid) return
    cronSubmitting.value = true
    try {
      const payload = {
        name: cronForm.name,
        platform: store.searchForm.source,
        query: store.searchForm.query,
        maxResults: store.searchForm.size,
        scheduleType: cronForm.scheduleType,
        cronSpec: cronForm.scheduleType === 'cron' ? cronForm.cronSpec : undefined,
        scheduleTime: cronForm.scheduleType === 'once' ? cronForm.scheduleTime : undefined,
      }
      const res = await saveSpaceEngineCronTask(payload)
      if (res.code === 0) {
        ElMessage.success(t('onlineSearch.cronDialog.createSuccess'))
        cronDialogVisible.value = false
      } else {
        ElMessage.error(res.msg || t('onlineSearch.cronDialog.createFailed'))
      }
    } finally {
      cronSubmitting.value = false
    }
  })
}

// 使用 store 中的数据
const tableData = computed(() => store.tableData)
const total = computed(() => store.total)

const quickQueries = computed(() => [
  { label: t('onlineSearch.ipSearch'), query: 'ip="1.1.1.1"' },
  { label: t('onlineSearch.domainSearch'), query: 'domain="example.com"' },
  { label: t('onlineSearch.titleSearch'), query: 'title="admin"' },
  { label: t('onlineSearch.icpSearch'), query: 'icp="ICP"' },
  { label: t('onlineSearch.portSearch'), query: 'port="3389"' },
])

async function handleSearch() {
  if (!store.searchForm.query) {
    ElMessage.warning(t('onlineSearch.enterQuery'))
    return
  }

  loading.value = true
  try {
    const res = await request.post('/onlineapi/search', {
      platform: store.searchForm.source,
      query: store.searchForm.query,
      page: store.searchForm.page,
      pageSize: store.searchForm.size
    })
    if (res.code === 0) {
      store.saveState(store.searchForm, res.list || [], res.total || 0)
    } else {
      ElMessage.error(res.msg || t('onlineSearch.searchFailed'))
    }
  } finally {
    loading.value = false
  }
}

function applyQuickQuery(item) {
  store.searchForm.query = item.query
}

// 数据源切换时，如果当前数量超过限制则自动调整
function handleSourceChange() {
  if (store.searchForm.source !== 'fofa' && store.searchForm.size > 100) {
    store.searchForm.size = 100
  }
}

function handleImport() {
  ElMessageBox.confirm(t('onlineSearch.confirmImportCurrent', { count: tableData.value.length }), t('common.tip'))
    .then(async () => {
      importLoading.value = true
      try {
        const res = await request.post('/onlineapi/import', {
          assets: tableData.value,
          platform: store.searchForm.source
        })
        if (res.code === 0 && res.taskId) {
          // 初始化常驻任务
          persistentTask.value = {
            taskId: res.taskId,
            status: 'running',
            total: tableData.value.length,
            completed: 0,
            imported: 0,
            skipped: 0,
            platform: store.searchForm.source,
            importType: 'current',
          }
          saveImportTaskToStorage()
          startImportPolling()
          ElMessage.success(res.msg || '导入任务已提交')
        } else {
          ElMessage.error(res.msg || t('onlineSearch.importFailed'))
        }
      } finally {
        importLoading.value = false
      }
    })
    .catch(() => {})
}

function handleImportAll() {
  if (!store.searchForm.query) {
    ElMessage.warning(t('onlineSearch.enterQueryFirst'))
    return
  }

  const estimatedCount = total.value

  ElMessageBox.confirm(
    t('onlineSearch.confirmImportAll', { count: estimatedCount }),
    t('onlineSearch.importAllTitle'),
    { type: 'warning' }
  )
    .then(async () => {
      importAllLoading.value = true
      try {
        const res = await request.post('/onlineapi/importAll', {
          platform: store.searchForm.source,
          query: store.searchForm.query,
          pageSize: store.searchForm.source === 'fofa' ? 500 : 100,
          maxPages: 0
        })
        if (res.code === 0 && res.taskId) {
          // 初始化常驻任务
          persistentTask.value = {
            taskId: res.taskId,
            status: 'running',
            total: estimatedCount,
            completed: 0,
            imported: 0,
            skipped: 0,
            platform: store.searchForm.source,
            importType: 'all',
          }
          saveImportTaskToStorage()
          startImportPolling()
          ElMessage.success(res.msg || '导入任务已提交')
        } else {
          ElMessage.error(res.msg || t('onlineSearch.importFailed'))
        }
      } finally {
        importAllLoading.value = false
      }
    })
    .catch(() => {})
}

function showHelpDialog() {
  helpDialogVisible.value = true
}

onMounted(() => {
  loadImportTaskFromStorage()
})

onBeforeUnmount(() => {
  if (importPollTimer) {
    clearInterval(importPollTimer)
    importPollTimer = null
  }
})
</script>

<style scoped>
.online-search-page {
  .persistent-task-bar {
    margin-bottom: 12px;
  }
  .persistent-task-card {
    border-radius: 8px;
    border-left: 4px solid var(--el-color-primary);
    :deep(.el-card__body) {
      padding: 12px 16px;
    }
  }
  .persistent-task-content {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
  }
  .persistent-task-info {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
  }
  .persistent-task-title {
    font-weight: 600;
    font-size: 14px;
    white-space: nowrap;
  }
  .persistent-task-progress {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .persistent-task-stat {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    white-space: nowrap;
    min-width: 60px;
  }
  .persistent-task-result {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .persistent-task-actions {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }
  .rotating {
    animation: rotating 1.5s linear infinite;
  }
  @keyframes rotating {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .search-card {
    margin-bottom: 20px;

    .quick-search {
      margin-top: 10px;
      padding-top: 10px;
      border-top: 1px solid var(--el-border-color);

      .label {
        color: var(--el-text-color-regular);
        margin-right: 10px;
      }

      .quick-tag {
        cursor: pointer;
        margin-right: 8px;

        &:hover {
          background: var(--el-color-primary);
          color: var(--el-color-white);
        }
      }
    }
  }

  .result-card {
    margin-bottom: 20px;

    .card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;

      .total {
        color: var(--el-text-color-secondary);
        font-size: 14px;
      }
    }

    .pagination {
      margin-top: 20px;
      justify-content: flex-end;
    }
  }

  .syntax-help {
    p {
      margin: 8px 0;
      line-height: 1.6;

      code {
        background: var(--el-fill-color-light);
        padding: 2px 6px;
        border-radius: 4px;
        color: var(--el-color-primary);
      }
    }
  }

  .cron-presets {
    margin-top: 8px;

    .preset-tag {
      cursor: pointer;
      margin-right: 8px;
      margin-bottom: 4px;

      &:hover {
        background: var(--el-color-primary);
        color: var(--el-color-white);
      }
    }
  }

  .import-result {
    .result-success {
      color: var(--el-color-success);
      font-weight: 600;
    }
    .result-warning {
      color: var(--el-color-warning);
      font-weight: 600;
    }
  }
}
</style>
