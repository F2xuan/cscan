<template>
  <div class="space-engine-cron-task">
    <!-- 操作栏 -->
    <el-card class="action-card" shadow="never">
      <el-button type="primary" @click="showCreateDialog">
        <el-icon><Plus /></el-icon>{{ $t('spaceEngineCronTask.newTask') || '新建定时任务' }}
      </el-button>
      <el-button @click="loadData">
        <el-icon><Refresh /></el-icon>{{ $t('common.refresh') || '刷新' }}
      </el-button>
      <el-button
        type="danger"
        :disabled="selectedRows.length === 0"
        @click="handleBatchDelete"
      >
        <el-icon><Delete /></el-icon>{{ $t('common.batchDelete') || '批量删除' }}
        <span v-if="selectedRows.length > 0">({{ selectedRows.length }})</span>
      </el-button>

      <!-- 关键字搜索 -->
      <el-input
        v-model="pagination.keyword"
        class="search-input"
        clearable
        :placeholder="$t('common.searchByName') || '按名称/查询语句搜索'"
        style="width: 260px; margin-left: 12px"
        @keyup.enter="handleSearch"
        @clear="handleSearch"
      >
        <template #append>
          <el-button :icon="Search" @click="handleSearch" />
        </template>
      </el-input>
    </el-card>

    <!-- 数据表格 -->
    <el-card class="table-card" shadow="never">
      <el-table
        :data="tableData"
        v-loading="loading"
        stripe
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="50" />
        <el-table-column prop="name" :label="$t('spaceEngineCronTask.taskName') || '任务名称'" min-width="160" show-overflow-tooltip />
        <el-table-column :label="$t('spaceEngineCronTask.dataSource') || '数据源'" width="110">
          <template #default="{ row }">
            <el-tag :type="platformTagType(row.platform)" effect="light" size="small">
              {{ platformLabel(row.platform) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="query" :label="$t('spaceEngineCronTask.query') || '查询语句'" min-width="240">
          <template #default="{ row }">
            <el-tooltip :content="row.query" placement="top" :show-after="300">
              <code class="query-code">{{ truncate(row.query, 60) }}</code>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column :label="$t('spaceEngineCronTask.scheduleType') || '调度类型'" width="200">
          <template #default="{ row }">
            <div v-if="row.scheduleType === 'cron'" class="schedule-cell">
              <el-tag type="primary" size="small">{{ $t('spaceEngineCronTask.cronExec') || '周期执行' }}</el-tag>
              <el-tooltip :content="getCronDescription(row.cronSpec)" placement="top">
                <code class="cron-code">{{ row.cronSpec }}</code>
              </el-tooltip>
            </div>
            <div v-else class="schedule-cell">
              <el-tag type="warning" size="small">{{ $t('spaceEngineCronTask.onceExec') || '定时执行' }}</el-tag>
              <span class="schedule-time">{{ row.scheduleTime }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('spaceEngineCronTask.status') || '状态'" width="90">
          <template #default="{ row }">
            <el-switch
              v-model="row.status"
              active-value="enable"
              inactive-value="disable"
              @change="handleToggle(row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="nextRunTime" :label="$t('spaceEngineCronTask.nextRunTime') || '下次执行时间'" width="170">
          <template #default="{ row }">
            <span v-if="row.status === 'enable' && row.nextRunTime">{{ row.nextRunTime }}</span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="lastRunTime" :label="$t('spaceEngineCronTask.lastRunTime') || '上次执行时间'" width="170">
          <template #default="{ row }">
            {{ row.lastRunTime || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="runCount" :label="$t('spaceEngineCronTask.runCount') || '执行次数'" width="90" align="center">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.runCount || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation') || '操作'" width="200" fixed="right">
          <template #default="{ row }">
            <el-button type="success" link size="small" @click="handleRunNow(row)">
              <el-icon><VideoPlay /></el-icon>{{ $t('spaceEngineCronTask.runNow') || '立即执行' }}
            </el-button>
            <el-button type="primary" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon>{{ $t('common.edit') || '编辑' }}
            </el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon>{{ $t('common.delete') || '删除' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        class="pagination"
        @size-change="handlePageChange"
        @current-change="handlePageChange"
      />
    </el-card>

    <!-- 新建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? ($t('spaceEngineCronTask.editTask') || '编辑定时任务') : ($t('spaceEngineCronTask.newTask') || '新建定时任务')"
      width="640px"
      @close="handleDialogClose"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="120px">
        <el-form-item :label="$t('spaceEngineCronTask.taskName') || '任务名称'" prop="name">
          <el-input v-model="form.name" :placeholder="$t('spaceEngineCronTask.enterTaskName') || '请输入任务名称'" maxlength="64" show-word-limit />
        </el-form-item>

        <el-form-item :label="$t('spaceEngineCronTask.dataSource') || '数据源'" prop="platform">
          <el-select v-model="form.platform" :placeholder="$t('spaceEngineCronTask.selectDataSource') || '请选择数据源'" style="width: 100%">
            <el-option label="Fofa" value="fofa">
              <div class="platform-option">
                <el-tag type="danger" size="small">Fofa</el-tag>
                <span class="platform-desc">{{ $t('spaceEngineCronTask.fofaDesc') }}</span>
              </div>
            </el-option>
            <el-option label="Hunter" value="hunter">
              <div class="platform-option">
                <el-tag type="success" size="small">Hunter</el-tag>
                <span class="platform-desc">{{ $t('spaceEngineCronTask.hunterDesc') }}</span>
              </div>
            </el-option>
            <el-option label="Quake" value="quake">
              <div class="platform-option">
                <el-tag type="warning" size="small">Quake</el-tag>
                <span class="platform-desc">{{ $t('spaceEngineCronTask.quakeDesc') }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('spaceEngineCronTask.query') || '查询语句'" prop="query">
          <el-input
            v-model="form.query"
            type="textarea"
            :rows="3"
            :placeholder="$t('spaceEngineCronTask.queryPlaceholder') || '请输入空间引擎查询语句，如：ip=&quot;1.1.1.1&quot;'"
          />
          <div class="form-hint">
            <el-button link type="primary" size="small" @click="showHelpDialog">
              <el-icon><QuestionFilled /></el-icon>{{ $t('spaceEngineCronTask.syntaxHelp') || '语法帮助' }}
            </el-button>
            <span class="hint-text">{{ $t('spaceEngineCronTask.queryHint') || '查询语句的语法与所选数据源保持一致' }}</span>
          </div>
        </el-form-item>

        <el-form-item :label="$t('spaceEngineCronTask.scheduleType') || '调度类型'" prop="scheduleType">
          <el-radio-group v-model="form.scheduleType">
            <el-radio label="cron">{{ $t('spaceEngineCronTask.cronExec') || '周期执行' }}</el-radio>
            <el-radio label="once">{{ $t('spaceEngineCronTask.onceExec') || '定时执行' }}</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- Cron 表达式 -->
        <el-form-item v-if="form.scheduleType === 'cron'" :label="$t('spaceEngineCronTask.cronExpression') || 'Cron 表达式'" prop="cronSpec">
          <el-input v-model="form.cronSpec" :placeholder="$t('spaceEngineCronTask.cronPlaceholder') || '例如：0 0 2 * * *'">
            <template #append>
              <el-button :loading="validatingCron" @click="validateCron">{{ $t('spaceEngineCronTask.validate') || '验证' }}</el-button>
            </template>
          </el-input>
          <div class="cron-help">
            <div class="cron-presets">
              <span class="preset-label">{{ $t('spaceEngineCronTask.quickSelect') || '快捷选择：' }}</span>
              <el-tag
                v-for="preset in cronPresets"
                :key="preset.value"
                size="small"
                class="preset-tag"
                effect="plain"
                @click="applyPreset(preset)"
              >
                {{ preset.label }}
              </el-tag>
            </div>
            <div v-if="cronValidation.valid" class="cron-next-times">
              <div class="next-label">
                <el-icon><CircleCheckFilled /></el-icon>
                {{ $t('spaceEngineCronTask.next5Times') || '未来 5 次执行时间：' }}
              </div>
              <div v-for="(time, index) in cronValidation.nextTimes" :key="index" class="next-time">
                {{ index + 1 }}. {{ time }}
              </div>
            </div>
            <div v-else-if="cronValidation.error" class="cron-error">
              <el-icon><CircleCloseFilled /></el-icon>{{ cronValidation.error }}
            </div>
          </div>
        </el-form-item>

        <!-- 定时执行时间 -->
        <el-form-item v-if="form.scheduleType === 'once'" :label="$t('spaceEngineCronTask.execTime') || '执行时间'" prop="scheduleTime">
          <el-date-picker
            v-model="form.scheduleTimeDate"
            type="datetime"
            :placeholder="$t('common.pleaseSelect') || '请选择'"
            format="YYYY-MM-DD HH:mm:ss"
            value-format="YYYY-MM-DD HH:mm:ss"
            :disabled-date="disabledDate"
            style="width: 100%"
            @change="onScheduleTimeChange"
          />
          <div class="form-hint">{{ $t('spaceEngineCronTask.onceExecHint') || '到达指定时间后任务将只执行一次' }}</div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') || '取消' }}</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') || '确定' }}</el-button>
      </template>
    </el-dialog>

    <!-- 语法帮助对话框 -->
    <el-dialog v-model="helpDialogVisible" :title="$t('spaceEngineCronTask.syntaxHelp') || '查询语法帮助'" width="640px">
      <el-tabs v-model="helpTab">
        <el-tab-pane label="Fofa" name="fofa">
          <div class="syntax-help">
            <p><code>ip="1.1.1.1"</code> - {{ $t('spaceEngineCronTask.syntaxFofaIp') }}</p>
            <p><code>domain="example.com"</code> - {{ $t('spaceEngineCronTask.syntaxFofaDomain') }}</p>
            <p><code>title="admin"</code> - {{ $t('spaceEngineCronTask.syntaxFofaTitle') }}</p>
            <p><code>body="content"</code> - {{ $t('spaceEngineCronTask.syntaxFofaBody') }}</p>
            <p><code>port="80"</code> - {{ $t('spaceEngineCronTask.syntaxFofaPort') }}</p>
            <p><code>icp="ICP名"</code> - {{ $t('spaceEngineCronTask.syntaxFofaIcp') }}</p>
            <p><code>org="公司名"</code> - {{ $t('spaceEngineCronTask.syntaxFofaOrg') }}</p>
            <p>{{ $t('spaceEngineCronTask.syntaxFofaCombined') }}：<code>ip="1.1.1.1" && port="80"</code></p>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Hunter" name="hunter">
          <div class="syntax-help">
            <p><code>ip="1.1.1.1"</code> - {{ $t('spaceEngineCronTask.syntaxFofaIp') }}</p>
            <p><code>domain.suffix="example.com"</code> - {{ $t('spaceEngineCronTask.syntaxHunterDomainSuffix') }}</p>
            <p><code>web.title="admin"</code> - {{ $t('spaceEngineCronTask.syntaxHunterWebTitle') }}</p>
            <p><code>icp.name="公司名"</code> - {{ $t('spaceEngineCronTask.syntaxHunterIcpName') }}</p>
            <p><code>icp.number="京ICP..."</code> - {{ $t('spaceEngineCronTask.syntaxHunterIcpNumber') }}</p>
            <p><code>port="443"</code> - {{ $t('spaceEngineCronTask.syntaxFofaPort') }}</p>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Quake" name="quake">
          <div class="syntax-help">
            <p><code>ip:"1.1.1.1"</code> - {{ $t('spaceEngineCronTask.syntaxFofaIp') }}</p>
            <p><code>domain:"example.com"</code> - {{ $t('spaceEngineCronTask.syntaxFofaDomain') }}</p>
            <p><code>title:"admin"</code> - {{ $t('spaceEngineCronTask.syntaxFofaTitle') }}</p>
            <p><code>service:"http"</code> - {{ $t('spaceEngineCronTask.syntaxQuakeService') }}</p>
            <p><code>port:"80"</code> - {{ $t('spaceEngineCronTask.syntaxFofaPort') }}</p>
            <p><code>country:"CN"</code> - {{ $t('spaceEngineCronTask.syntaxQuakeCountry') }}</p>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus,
  Refresh,
  Edit,
  Delete,
  VideoPlay,
  Search,
  QuestionFilled,
  CircleCheckFilled,
  CircleCloseFilled
} from '@element-plus/icons-vue'
import {
  getSpaceEngineCronTaskList,
  saveSpaceEngineCronTask,
  toggleCronTask,
  deleteCronTask,
  batchDeleteCronTask,
  runCronTaskNow,
  validateCronSpec
} from '@/api/crontask'

const { t } = useI18n()

const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  keyword: ''
})

const dialogVisible = ref(false)
const helpDialogVisible = ref(false)
const helpTab = ref('fofa')
const isEdit = ref(false)
const submitting = ref(false)
const validatingCron = ref(false)
const formRef = ref(null)

// 表单默认值
function getDefaultForm() {
  return {
    id: '',
    name: '',
    platform: 'fofa',
    query: '',
    scheduleType: 'cron',
    cronSpec: '0 0 2 * * *',
    scheduleTime: '',
    scheduleTimeDate: null
  }
}

const form = reactive(getDefaultForm())

const rules = {
  name: [
    { required: true, message: t('spaceEngineCronTask.enterTaskName') || '请输入任务名称', trigger: 'blur' }
  ],
  platform: [
    { required: true, message: t('spaceEngineCronTask.selectDataSource') || '请选择数据源', trigger: 'change' }
  ],
  query: [
    { required: true, message: t('spaceEngineCronTask.enterQuery') || '请输入查询语句', trigger: 'blur' }
  ],
  scheduleType: [
    { required: true, message: t('common.pleaseSelect') || '请选择', trigger: 'change' }
  ],
  cronSpec: [
    {
      required: true,
      validator: (rule, value, callback) => {
        if (form.scheduleType === 'cron' && !value) {
          callback(new Error(t('spaceEngineCronTask.cronValidateError') || '请输入 Cron 表达式'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  scheduleTime: [
    {
      required: true,
      validator: (rule, value, callback) => {
        if (form.scheduleType === 'once' && !form.scheduleTimeDate) {
          callback(new Error(t('common.pleaseSelect') || '请选择'))
        } else {
          callback()
        }
      },
      trigger: 'change'
    }
  ]
}

// Cron 快捷预设
const cronPresets = computed(() => [
  { label: t('spaceEngineCronTask.everyHour') || '每小时', value: '0 0 * * * *' },
  { label: t('spaceEngineCronTask.everyDay2am') || '每日凌晨 2 点', value: '0 0 2 * * *' },
  { label: t('spaceEngineCronTask.everyMonday') || '每周一凌晨 3 点', value: '0 0 3 * * 1' },
  { label: t('spaceEngineCronTask.everyMonth1st') || '每月 1 号凌晨 3 点', value: '0 0 3 1 * *' },
  { label: t('spaceEngineCronTask.every6hours') || '每 6 小时', value: '0 0 */6 * * *' }
])

const cronValidation = reactive({
  valid: false,
  nextTimes: [],
  error: ''
})

// 数据源标签映射
const PLATFORM_MAP = {
  fofa: { label: 'Fofa', tagType: 'danger' },
  hunter: { label: 'Hunter', tagType: 'success' },
  quake: { label: 'Quake', tagType: 'warning' }
}

function platformLabel(platform) {
  return PLATFORM_MAP[platform]?.label || platform || '-'
}

function platformTagType(platform) {
  return PLATFORM_MAP[platform]?.tagType || 'info'
}

// 截断文本
function truncate(text, maxLen = 60) {
  if (!text) return ''
  const str = String(text)
  if (str.length <= maxLen) return str
  return str.substring(0, maxLen) + '...'
}

// 获取 Cron 描述
function getCronDescription(cronSpec) {
  if (!cronSpec) return ''
  const preset = cronPresets.value.find(p => p.value === cronSpec)
  if (preset) return preset.label
  return t('spaceEngineCronTask.customCron') || '自定义 Cron 表达式'
}

// 禁用过去日期
function disabledDate(time) {
  return time.getTime() < Date.now() - 86400000
}

function onScheduleTimeChange(val) {
  form.scheduleTime = val || ''
}

// 应用预设
function applyPreset(preset) {
  form.cronSpec = preset.value
  cronValidation.valid = false
  cronValidation.nextTimes = []
  cronValidation.error = ''
  validateCron()
}

// 验证 Cron 表达式
async function validateCron() {
  if (!form.cronSpec) {
    cronValidation.valid = false
    cronValidation.error = t('spaceEngineCronTask.cronValidateError') || '请输入 Cron 表达式'
    cronValidation.nextTimes = []
    return
  }

  validatingCron.value = true
  try {
    const res = await validateCronSpec({ cronSpec: form.cronSpec })
    if (res.code === 0 && res.data) {
      cronValidation.valid = res.data.valid
      if (res.data.valid) {
        cronValidation.error = ''
        cronValidation.nextTimes = res.data.nextTimes || []
      } else {
        cronValidation.error = res.data.message || t('spaceEngineCronTask.cronValidateError') || 'Cron 表达式无效'
        cronValidation.nextTimes = []
      }
    } else {
      cronValidation.valid = false
      cronValidation.error = res.msg || t('spaceEngineCronTask.cronValidateError') || 'Cron 表达式无效'
      cronValidation.nextTimes = []
    }
  } catch (error) {
    cronValidation.valid = false
    cronValidation.error = t('spaceEngineCronTask.validateRequestError') || '验证请求失败'
    cronValidation.nextTimes = []
  } finally {
    validatingCron.value = false
  }
}

// 加载数据
async function loadData() {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      pageSize: pagination.pageSize
    }
    if (pagination.keyword) {
      params.keyword = pagination.keyword
    }
    const res = await getSpaceEngineCronTaskList(params)
    if (res.code === 0) {
      tableData.value = res.data?.list || []
      pagination.total = res.data?.total || 0
    } else {
      ElMessage.error(res.msg || t('common.loadFailed') || '加载失败')
    }
  } catch (error) {
    console.error('loadSpaceEngineCronTaskFailed:', error)
    ElMessage.error(t('common.loadFailed') || '加载失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  loadData()
}

function handlePageChange() {
  loadData()
}

function handleSelectionChange(val) {
  selectedRows.value = val
}

// 状态切换
async function handleToggle(row) {
  try {
    const res = await toggleCronTask({ id: row.id, status: row.status })
    if (res.code === 0) {
      ElMessage.success(t('spaceEngineCronTask.statusUpdateSuccess') || '状态更新成功')
      loadData()
    } else {
      row.status = row.status === 'enable' ? 'disable' : 'enable'
      ElMessage.error(res.msg || t('common.updateFailed') || '更新失败')
    }
  } catch (err) {
    row.status = row.status === 'enable' ? 'disable' : 'enable'
    ElMessage.error(t('common.updateFailed') || '更新失败')
  }
}

// 立即执行
async function handleRunNow(row) {
  try {
    await ElMessageBox.confirm(
      t('spaceEngineCronTask.confirmRunNow') || '确定立即执行该任务吗？',
      t('common.tip') || '提示',
      { type: 'warning' }
    )
    const res = await runCronTaskNow({ id: row.id })
    if (res.code === 0) {
      ElMessage.success(t('spaceEngineCronTask.runSuccess') || '执行已触发')
      loadData()
    } else {
      ElMessage.error(res.msg || t('spaceEngineCronTask.runFailed') || '执行失败')
    }
  } catch (e) {
    // 用户取消
  }
}

// 单个删除
function handleDelete(row) {
  ElMessageBox.confirm(
    t('spaceEngineCronTask.confirmDelete') || '确定删除该定时任务吗？',
    t('common.tip') || '提示',
    { type: 'warning' }
  ).then(async () => {
    const res = await deleteCronTask({ id: row.id })
    if (res.code === 0) {
      ElMessage.success(t('spaceEngineCronTask.deleteSuccess') || '删除成功')
      loadData()
    } else {
      ElMessage.error(res.msg || t('common.deleteFailed') || '删除失败')
    }
  }).catch(() => {})
}

// 批量删除
function handleBatchDelete() {
  if (!selectedRows.value.length) return
  ElMessageBox.confirm(
    t('spaceEngineCronTask.confirmBatchDelete', { count: selectedRows.value.length }),
    t('common.tip') || '提示',
    { type: 'warning' }
  ).then(async () => {
    const res = await batchDeleteCronTask({ ids: selectedRows.value.map(item => item.id) })
    if (res.code === 0) {
      ElMessage.success(t('spaceEngineCronTask.deleteSuccess') || '删除成功')
      loadData()
    } else {
      ElMessage.error(res.msg || t('common.deleteFailed') || '删除失败')
    }
  }).catch(() => {})
}

// 新建
function showCreateDialog() {
  isEdit.value = false
  Object.assign(form, getDefaultForm())
  helpTab.value = 'fofa'
  cronValidation.valid = false
  cronValidation.nextTimes = []
  cronValidation.error = ''
  dialogVisible.value = true
}

// 打开语法帮助对话框
function showHelpDialog() {
  helpTab.value = form.platform || 'fofa'
  helpDialogVisible.value = true
}

// 编辑
function handleEdit(row) {
  isEdit.value = true
  Object.assign(form, getDefaultForm(), {
    id: row.id,
    name: row.name,
    platform: row.platform || 'fofa',
    query: row.query || '',
    scheduleType: row.scheduleType || 'cron',
    cronSpec: row.cronSpec || '0 0 2 * * *',
    scheduleTime: row.scheduleTime || '',
    scheduleTimeDate: row.scheduleTime || null
  })
  helpTab.value = form.platform
  cronValidation.valid = false
  cronValidation.nextTimes = []
  cronValidation.error = ''
  dialogVisible.value = true
}

// 关闭对话框重置
function handleDialogClose() {
  formRef.value?.resetFields()
  Object.assign(form, getDefaultForm())
  cronValidation.valid = false
  cronValidation.nextTimes = []
  cronValidation.error = ''
}

// 提交
async function handleSubmit() {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      const submitData = {
        id: form.id,
        name: form.name,
        platform: form.platform,
        query: form.query,
        maxResults: 100,
        scheduleType: form.scheduleType,
        cronSpec: form.scheduleType === 'cron' ? form.cronSpec : '',
        scheduleTime: form.scheduleType === 'once' ? form.scheduleTime : ''
      }

      const res = await saveSpaceEngineCronTask(submitData)
      if (res.code === 0) {
        ElMessage.success(
          isEdit.value
            ? (t('spaceEngineCronTask.updateSuccess') || '更新成功')
            : (t('spaceEngineCronTask.createSuccess') || '创建成功')
        )
        dialogVisible.value = false
        loadData()
      } else {
        ElMessage.error(
          res.msg ||
          (isEdit.value ? (t('common.updateFailed') || '更新失败') : (t('common.createFailed') || '创建失败'))
        )
      }
    } catch (error) {
      console.error('saveSpaceEngineCronTaskFailed:', error)
      ElMessage.error(
        isEdit.value ? (t('common.updateFailed') || '更新失败') : (t('common.createFailed') || '创建失败')
      )
    } finally {
      submitting.value = false
    }
  })
}

onMounted(() => {
  loadData()
})
</script>

<style lang="scss" scoped>
.space-engine-cron-task {
  .action-card {
    margin-bottom: 16px;

    :deep(.el-card__body) {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
    }

    .search-input {
      margin-left: auto;
    }
  }

  .table-card {
    margin-bottom: 16px;

    .pagination {
      margin-top: 16px;
      justify-content: flex-end;
    }
  }

  .query-code {
    background: var(--el-fill-color-light);
    padding: 2px 6px;
    border-radius: 4px;
    font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
    font-size: 12px;
    color: var(--el-text-color-primary);
    word-break: break-all;
  }

  .schedule-cell {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;

    .cron-code {
      background: var(--el-fill-color-light);
      padding: 2px 6px;
      border-radius: 4px;
      font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
      font-size: 12px;
    }

    .schedule-time {
      font-size: 12px;
      color: var(--el-text-color-regular);
    }
  }

  .text-muted {
    color: var(--el-text-color-placeholder);
  }

  .platform-option {
    display: flex;
    align-items: center;
    gap: 8px;

    .platform-desc {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .form-hint {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
    line-height: 1;

    .hint-text {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .form-hint-inline {
    margin-left: 12px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .cron-help {
    width: 100%;
    margin-top: 8px;

    .cron-presets {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 6px;
      margin-bottom: 8px;

      .preset-label {
        font-size: 12px;
        color: var(--el-text-color-secondary);
      }

      .preset-tag {
        cursor: pointer;
        transition: all 0.2s;

        &:hover {
          background: var(--el-color-primary);
          color: var(--el-color-white);
          border-color: var(--el-color-primary);
        }
      }
    }

    .cron-next-times {
      background: var(--el-color-success-light-9);
      border: 1px solid var(--el-color-success-light-5);
      border-radius: 4px;
      padding: 8px 12px;
      font-size: 12px;

      .next-label {
        display: flex;
        align-items: center;
        gap: 4px;
        color: var(--el-color-success);
        font-weight: 500;
        margin-bottom: 4px;
      }

      .next-time {
        color: var(--el-text-color-regular);
        line-height: 1.7;
        font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
      }
    }

    .cron-error {
      display: flex;
      align-items: center;
      gap: 4px;
      background: var(--el-color-danger-light-9);
      border: 1px solid var(--el-color-danger-light-5);
      border-radius: 4px;
      padding: 8px 12px;
      color: var(--el-color-danger);
      font-size: 12px;
    }
  }

  .syntax-help {
    p {
      margin: 8px 0;
      line-height: 1.7;
      font-size: 13px;

      code {
        background: var(--el-fill-color-light);
        padding: 2px 6px;
        border-radius: 4px;
        color: var(--el-color-primary);
        font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
        font-size: 12px;
      }
    }
  }
}
</style>
