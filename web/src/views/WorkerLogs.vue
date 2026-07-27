<template>
  <div class="container-logs-page">
    <el-card>
      <template #header>
        <div class="log-header">
          <span>{{ $t('container.title') }}</span>
          <div class="log-filters">
            <el-input
              v-model="activeTab.searchKeyword"
              :placeholder="$t('container.searchLogs')"
              clearable
              size="small"
              style="width: 200px"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select v-model="activeTab.streamFilter" size="small" style="width: 100px">
              <el-option :label="$t('container.streamAll')" value="all" />
              <el-option :label="$t('container.streamStdout')" value="stdout" />
              <el-option :label="$t('container.streamStderr')" value="stderr" />
            </el-select>
            <el-select v-model="activeTab.levelFilter" size="small" style="width: 90px" :placeholder="$t('container.levelFilter')">
              <el-option :label="$t('container.allLevels')" value="all" />
              <el-option label="ERROR" value="ERROR" />
              <el-option label="WARN" value="WARN" />
              <el-option label="INFO" value="INFO" />
              <el-option label="DEBUG" value="DEBUG" />
            </el-select>
            <el-checkbox v-model="showFullTs" size="small">{{ $t('container.showFullTs') }}</el-checkbox>
            <el-button :type="activeTab.paused ? 'success' : 'warning'" size="small" @click="activeTab.paused = !activeTab.paused">
              {{ activeTab.paused ? $t('container.resume') : $t('container.pause') }}
            </el-button>
            <el-button size="small" @click="clearActiveTab">{{ $t('container.clear') }}</el-button>
            <el-dropdown size="small" @command="exportLogs">
              <el-button size="small">{{ $t('container.export') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="txt">{{ $t('container.exportTxt') }}</el-dropdown-item>
                  <el-dropdown-item command="json">{{ $t('container.exportJson') }}</el-dropdown-item>
                  <el-dropdown-item command="csv">{{ $t('container.exportCsv') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <span class="line-count">{{ $t('container.lineCount') }}: {{ activeTab.lines.length }}</span>
          </div>
        </div>
      </template>

      <div class="layout">
        <!-- 左侧容器列表 -->
        <aside class="sidebar">
          <div class="sidebar-header">
            <span>{{ $t('container.containers') }}</span>
            <el-button text size="small" @click="loadContainers">
              <el-icon><Refresh /></el-icon>
            </el-button>
          </div>
          <el-menu>
            <el-menu-item
              v-for="c in containers"
              :key="c.name"
              :index="c.name"
              @click="openTab(c.name)"
              :class="{ 'is-opened': tabs.some(t => t.name === c.name) }"
            >
              <span class="dot" :class="c.state === 'running' ? 'dot-on' : 'dot-off'"></span>
              <span class="ctn-name">{{ c.name }}</span>
              <span class="ctn-state">{{ c.status }}</span>
            </el-menu-item>
            <el-menu-item v-if="!containers.length && !loading" disabled>
              <span style="color: var(--el-text-color-secondary)">{{ $t('container.noContainers') }}</span>
            </el-menu-item>
          </el-menu>
        </aside>

        <!-- 右侧日志区 -->
        <section class="viewer">
          <!-- 标签栏 -->
          <div v-if="tabs.length" class="tab-bar">
            <div
              v-for="tab in tabs"
              :key="tab.name"
              class="tab-item"
              :class="{ active: tab.name === activeTabName }"
              @click="switchTab(tab.name)"
              @contextmenu.prevent="showTabMenu($event, tab.name)"
            >
              <span class="dot-sm" :class="tab.conn === 'connected' ? 'dot-on' : tab.conn === 'connecting' ? 'dot-warn' : 'dot-off'"></span>
              <span class="tab-name">{{ tab.name }}</span>
              <el-icon class="tab-close" @click.stop="closeTab(tab.name)"><Close /></el-icon>
            </div>
          </div>

          <!-- 空状态 -->
          <div v-if="!tabs.length" class="empty">
            <el-icon :size="48" style="color: var(--el-text-color-disabled)"><Document /></el-icon>
            <span>{{ $t('container.noTabs') }}</span>
          </div>
          <div v-else-if="dockerUnavailable" class="empty">
            <span>{{ $t('container.dockerUnavailable') }}</span>
          </div>

          <!-- 日志内容 -->
          <div v-else ref="logBox" class="log-box" @scroll="onScroll">
            <div
              v-for="(l, idx) in filteredLines"
              :key="idx"
              class="log-line"
              :class="lineClass(l)"
            >
              <span class="log-ln">{{ idx + 1 }}</span>
              <span v-if="showFullTs && l.ts" class="log-ts-full">{{ formatDockerTs(l.ts) }}</span>
              <span class="log-level" :class="levelClass(l.level)">{{ l.level || 'LOG' }}</span>
              <span v-if="l.container" class="log-container">{{ l.container }}</span>
              <span v-if="l.worker" class="log-worker">{{ l.worker }}</span>
              <span v-if="l.taskId" class="log-task">[{{ l.taskId }}]</span>
              <span class="log-time">{{ formatTime(l.time) }}</span>
              <span class="log-body">{{ l.body }}</span>
            </div>
          </div>

          <!-- 滚动到底部按钮 -->
          <transition name="el-fade-in">
            <button
              v-if="showScrollBtn"
              class="scroll-bottom-btn"
              @click="scrollToBottom"
            >
              <el-icon><Bottom /></el-icon>
              {{ $t('container.scrollToBottom') }}
            </button>
          </transition>
        </section>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { listContainers, fetchContainerLogs, buildStreamURL } from '@/api/container'

const userStore = useUserStore()

// ==================== 容器列表 ====================
const containers = ref([])
const loading = ref(false)
const dockerUnavailable = ref(false)

async function loadContainers() {
  loading.value = true
  try {
    const res = await listContainers()
    if (res.code === 503) {
      dockerUnavailable.value = true
      containers.value = []
      return
    }
    if (res.code !== 0) {
      ElMessage.error(res.msg || 'error')
      return
    }
    dockerUnavailable.value = false
    containers.value = (res.list || []).filter(c => !c.name.toLowerCase().includes('worker'))
  } catch (e) {
    ElMessage.error(e.message || 'error')
  } finally {
    loading.value = false
  }
}

// ==================== 标签页管理 ====================
const tabs = ref([])
const activeTabName = ref('')
const showFullTs = ref(false)
const logBox = ref(null)
const showScrollBtn = ref(false)

// 每个标签页的独立状态
function createTabState(name) {
  return reactive({
    name,
    lines: [],
    searchKeyword: '',
    streamFilter: 'all',
    levelFilter: 'all',
    paused: false,
    autoScroll: true,
    conn: 'disconnected',
    es: null,
    backoff: 1000
  })
}

const activeTab = computed(() => {
  return tabs.value.find(t => t.name === activeTabName.value) || {
    lines: [],
    searchKeyword: '',
    streamFilter: 'all',
    levelFilter: 'all',
    paused: false,
    conn: 'disconnected'
  }
})

function openTab(name) {
  const existing = tabs.value.find(t => t.name === name)
  if (existing) {
    activeTabName.value = name
    return
  }
  const tab = createTabState(name)
  tabs.value.push(tab)
  activeTabName.value = name
  openStream(tab)
}

function switchTab(name) {
  activeTabName.value = name
  nextTick(scrollToBottom)
}

function closeTab(name) {
  const idx = tabs.value.findIndex(t => t.name === name)
  if (idx < 0) return
  const tab = tabs.value[idx]
  closeStream(tab)
  tabs.value.splice(idx, 1)
  if (activeTabName.value === name) {
    activeTabName.value = tabs.value.length ? tabs.value[Math.min(idx, tabs.value.length - 1)].name : ''
  }
}

function closeOtherTabs(name) {
  tabs.value.forEach(tab => {
    if (tab.name !== name) closeStream(tab)
  })
  tabs.value = tabs.value.filter(t => t.name === name)
  activeTabName.value = name
}

function closeAllTabs() {
  tabs.value.forEach(tab => closeStream(tab))
  tabs.value = []
  activeTabName.value = ''
}

function showTabMenu(e, name) {
  // 简单实现：右键直接关闭
  // 可以后续升级为 ContextMenu 组件
}

// ==================== 日志流 ====================
function openStream(tab) {
  closeStream(tab)
  if (!tab.name || dockerUnavailable.value) return
  tab.conn = 'connecting'

  const url = buildStreamURL({
    name: tab.name,
    token: userStore.token,
    tail: '1000'
  })

  const es = new EventSource(url)
  tab.es = es

  es.onmessage = (ev) => {
    if (tab.paused) return
    try {
      const obj = JSON.parse(ev.data)
      const parsed = parseLogLine(obj)
      tab.lines.push(parsed)
      if (tab.lines.length > 5000) tab.lines.splice(0, tab.lines.length - 5000)
      if (tab.name === activeTabName.value && tab.autoScroll) {
        nextTick(scrollToBottom)
      }
    } catch (_) {}
  }

  es.addEventListener('end', () => {
    tab.conn = 'disconnected'
    closeStream(tab)
  })

  es.addEventListener('error', () => {
    tab.conn = 'disconnected'
    closeStream(tab)
  })

  es.onerror = () => {
    tab.conn = 'disconnected'
    closeStream(tab)
    if (!tab.name) return
    tab.backoff = Math.min(tab.backoff * 2, 30000)
    setTimeout(() => {
      if (tabs.value.some(t => t.name === tab.name)) openStream(tab)
    }, tab.backoff)
  }

  es.addEventListener('open', () => {
    tab.conn = 'connected'
    tab.backoff = 1000
  })
}

function closeStream(tab) {
  if (tab.es) {
    tab.es.close()
    tab.es = null
  }
}

// ==================== 日志解析（多格式） ====================
const ANSI_RE = /\x1b\[[0-9;]*m/g
const GOZERO_RE = /^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\t(info|error|debug|slow|stat|alert|fatal)\t([\s\S]*)$/i
const GOZERO_SHORT_RE = /^(\d{2}:\d{2}:\d{2})\t(info|error|debug|slow|stat|alert|fatal)\t([\s\S]*)$/i
const REDIS_RE = /^(\d+):([A-Z])\s+(\d{2}\s+\w{3}\s+\d{4}\s+\d{2}:\d{2}:\d{2}\.\d{3})\s+([*#-])\s+(.*)$/
const NGINX_ACCESS_RE = /^([\d.]+)\s+-\s+(\S+)\s+\[([^\]]+)\]\s+"(\S+)\s+(\S+)\s+(\S+)"\s+(\d{3})\s+(\d+|-)/
const NGINX_ERROR_RE = /^(\d{4}\/\d{2}\/\d{2}\s+\d{2}:\d{2}:\d{2})\s+\[(\w+)\]\s+(\d+)#(\d+):\s+(.*)$/
const WORKER_INNER_RE = /^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s+\[(ERROR|WARN|INFO|DEBUG|FATAL|PANIC|TRACE)\]\s+(?:\[([a-zA-Z0-9_-]+(?:-\d+)?)\]\s+)?(?:\[Task:([a-zA-Z0-9_-]+)\]\s+)?([\s\S]*)$/i
const LEVEL_RE = /\[(ERROR|WARN|INFO|DEBUG|FATAL|PANIC|TRACE)\]/i
const TASK_RE = /\[Task:([a-zA-Z0-9_-]+)\]/

const REDIS_LEVEL_MAP = { '*': 'INFO', '#': 'WARN', '-': 'DEBUG' }
const MONGO_SEVERITY_MAP = { F: 'FATAL', E: 'ERROR', W: 'WARN', I: 'INFO', D: 'DEBUG', D1: 'DEBUG', D2: 'DEBUG' }

function parseLogLine(obj) {
  const raw = (obj.line || '').replace(ANSI_RE, '')
  const containerName = obj.container || ''
  let level = ''
  let worker = ''
  let taskId = ''
  let time = ''
  let body = raw

  // 1) go-zero plain: "2026-07-27 10:11:12\tinfo\tmessage\tkey=val..."
  const gzMatch = raw.match(GOZERO_RE) || raw.match(GOZERO_SHORT_RE)
  if (gzMatch) {
    time = gzMatch[1]
    level = gzMatch[2].toUpperCase()
    const rest = gzMatch[3]
    // 分离 message 和 fields（以 \t 分割）
    const parts = rest.split('\t')
    body = parts[0] || rest
    // 检查是否是 worker 内嵌格式
    const innerMatch = body.match(WORKER_INNER_RE)
    if (innerMatch) {
      time = innerMatch[1]
      level = innerMatch[2].toUpperCase()
      worker = innerMatch[3] || ''
      taskId = innerMatch[4] || ''
      body = innerMatch[5] || body
    }
    // HTTP 访问日志标记
    if (body.startsWith('[HTTP]')) {
      const statusMatch = body.match(/\[HTTP\]\s+(\d{3})/)
      if (statusMatch) {
        const code = parseInt(statusMatch[1])
        if (code >= 500) level = 'ERROR'
        else if (level === 'SLOW') level = 'WARN'
      }
    }
    return { stream: obj.stream || 'stdout', ts: obj.ts || '', level, worker, taskId, time, body, container: containerName, raw }
  }

  // 2) Redis: "1:M 27 Jul 2026 02:24:36.132 * Background saving started"
  const redisMatch = raw.match(REDIS_RE)
  if (redisMatch) {
    time = redisMatch[3]
    level = REDIS_LEVEL_MAP[redisMatch[4]] || 'INFO'
    body = redisMatch[5].replace(/oO0OoO0OoO0Oo/g, '').trim()
    return { stream: obj.stream || 'stdout', ts: obj.ts || '', level, worker: '', taskId: '', time, body, container: containerName, raw }
  }

  // 3) MongoDB JSON: {"t":{"$date":...},"s":"I",...}
  if (raw.startsWith('{')) {
    try {
      const json = JSON.parse(raw)
      level = MONGO_SEVERITY_MAP[json.s] || 'INFO'
      const parts = []
      if (json.c) parts.push(`[${json.c}]`)
      if (json.ctx) parts.push(`(${json.ctx})`)
      if (json.msg) parts.push(json.msg)
      if (json.attr) {
        const attrStr = typeof json.attr === 'string' ? json.attr : JSON.stringify(json.attr)
        if (attrStr.length <= 200) parts.push(attrStr)
        else parts.push(attrStr.slice(0, 200) + '...')
      }
      body = parts.join(' ') || raw
      return { stream: obj.stream || 'stdout', ts: obj.ts || '', level, worker: '', taskId: '', time: '', body, container: containerName, raw }
    } catch (_) { /* not JSON, fall through */ }
  }

  // 4) nginx error: "2026/07/27 01:52:58 [error] 123#456: message"
  const nginxErrMatch = raw.match(NGINX_ERROR_RE)
  if (nginxErrMatch) {
    time = nginxErrMatch[1]
    const lvl = nginxErrMatch[2].toLowerCase()
    level = lvl === 'error' || lvl === 'crit' || lvl === 'alert' || lvl === 'emerg' ? 'ERROR' : lvl === 'warn' ? 'WARN' : 'INFO'
    body = nginxErrMatch[5]
    return { stream: obj.stream || 'stderr', ts: obj.ts || '', level, worker: '', taskId: '', time, body, container: containerName, raw }
  }

  // 5) nginx access: '172.18.0.1 - - [27/Jul/2026:01:52:58 +0000] "GET / HTTP/1.1" 200 612'
  const nginxAccMatch = raw.match(NGINX_ACCESS_RE)
  if (nginxAccMatch) {
    time = nginxAccMatch[3]
    const code = parseInt(nginxAccMatch[7])
    level = code >= 500 ? 'ERROR' : code >= 400 ? 'WARN' : 'INFO'
    body = `${nginxAccMatch[4]} ${nginxAccMatch[5]} → ${code}`
    return { stream: obj.stream || 'stdout', ts: obj.ts || '', level, worker: '', taskId: '', time, body, container: containerName, raw }
  }

  // 6) Worker 内嵌格式（无 go-zero 外层）
  const workerMatch = raw.match(WORKER_INNER_RE)
  if (workerMatch) {
    time = workerMatch[1]
    level = workerMatch[2].toUpperCase()
    worker = workerMatch[3] || ''
    taskId = workerMatch[4] || ''
    body = workerMatch[5] || raw
    return { stream: obj.stream || 'stdout', ts: obj.ts || '', level, worker, taskId, time, body, container: containerName, raw }
  }

  // 7) Fallback: 尝试提取 [LEVEL] 标记
  const lm = raw.match(LEVEL_RE)
  if (lm) level = lm[1].toUpperCase()
  const tm = raw.match(TASK_RE)
  if (tm) taskId = tm[1]

  return { stream: obj.stream || 'stdout', ts: obj.ts || '', level, worker, taskId, time, body: body || raw, container: containerName, raw }
}

// ==================== 过滤 ====================
const filteredLines = computed(() => {
  const tab = activeTab.value
  const kw = tab.searchKeyword?.trim() || ''
  const sf = tab.streamFilter || 'all'
  const lf = tab.levelFilter || 'all'

  return tab.lines.filter(l => {
    if (sf !== 'all' && l.stream !== sf) return false
    if (lf !== 'all' && l.level !== lf) return false
    if (kw) {
      const target = l.raw.toLowerCase()
      if (!target.includes(kw.toLowerCase())) return false
    }
    return true
  })
})

// ==================== 样式类 ====================
function lineClass(l) {
  return {
    'log-stderr': l.stream === 'stderr',
    'log-error': l.level === 'ERROR' || l.level === 'FATAL' || l.level === 'PANIC',
    'log-warn': l.level === 'WARN',
    'log-debug': l.level === 'DEBUG'
  }
}

function levelClass(level) {
  if (!level) return ''
  const map = {
    ERROR: 'level-error',
    FATAL: 'level-error',
    PANIC: 'level-error',
    WARN: 'level-warn',
    SLOW: 'level-warn',
    INFO: 'level-info',
    DEBUG: 'level-debug',
    TRACE: 'level-debug',
    STAT: 'level-debug'
  }
  return map[level] || ''
}

// ==================== 格式化 ====================
function formatDockerTs(ts) {
  if (!ts) return ''
  return ts.replace('T', ' ').replace(/\.\d+Z?$/, '')
}

function formatTime(time) {
  if (!time) return ''
  // "2026-07-27 10:11:12" → "10:11:12"
  const parts = time.split(' ')
  return parts.length > 1 ? parts[1] : time
}

// ==================== 滚动 ====================
function scrollToBottom() {
  const el = logBox.value
  if (el) el.scrollTop = el.scrollHeight
}

function onScroll() {
  const el = logBox.value
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60
  showScrollBtn.value = !atBottom
  const tab = activeTab.value
  if (tab) tab.autoScroll = atBottom
}

// ==================== 操作 ====================
function clearActiveTab() {
  const tab = activeTab.value
  if (tab && tab.lines) tab.lines = []
}

async function exportLogs(fmt) {
  const tab = activeTab.value
  if (!tab || !tab.name) return
  try {
    const res = await fetchContainerLogs({ name: tab.name, tail: '5000' })
    if (res.code !== 0) {
      ElMessage.error(res.msg || 'error')
      return
    }
    const rawList = res.list || []
    const list = rawList.map(l => parseLogLine(l))
    let blob
    let filename
    if (fmt === 'json') {
      blob = new Blob([JSON.stringify(list, null, 2)], { type: 'application/json' })
      filename = `${tab.name}.json`
    } else if (fmt === 'csv') {
      const rows = ['level,time,worker,task,message', ...list.map(l => {
        const esc = (v) => `"${String(v == null ? '' : v).replace(/"/g, '""')}"`
        return `${esc(l.level)},${esc(l.time)},${esc(l.worker)},${esc(l.taskId)},${esc(l.body)}`
      })]
      blob = new Blob([rows.join('\n')], { type: 'text/csv' })
      filename = `${tab.name}.csv`
    } else {
      blob = new Blob([list.map(l => {
        const parts = [l.time, l.level ? `[${l.level}]` : '', l.worker ? `[${l.worker}]` : '', l.taskId ? `[Task:${l.taskId}]` : '', l.body]
        return parts.filter(Boolean).join(' ')
      }).join('\n')], { type: 'text/plain' })
      filename = `${tab.name}.txt`
    }
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('container.exportSuccess')
  } catch (e) {
    ElMessage.error(e.message || 'container.exportFailed')
  }
}

// ==================== 生命周期 ====================
onBeforeUnmount(() => {
  tabs.value.forEach(tab => closeStream(tab))
})

loadContainers()
</script>

<style scoped lang="scss">
.container-logs-page {
  padding: 8px 12px;
}
.log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}
.log-filters {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.line-count {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
.layout {
  display: flex;
  height: calc(100vh - 220px);
  min-height: 400px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  overflow: hidden;
}

/* ========== 左侧容器列表 ========== */
.sidebar {
  width: 220px;
  min-width: 220px;
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
  overflow-y: auto;
}
.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  flex-shrink: 0;
}
.dot-on { background: #67c23a; }
.dot-off { background: var(--el-text-color-disabled); }
.dot-warn { background: #e6a23c; }
.dot-sm {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
  flex-shrink: 0;
}
.ctn-name {
  display: inline-block;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: middle;
  font-size: 13px;
}
.ctn-state {
  margin-left: auto;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
.is-opened {
  background: var(--el-fill-color-light) !important;
}

/* ========== 右侧查看器 ========== */
.viewer {
  flex: 1;
  background: var(--el-bg-color);
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}
.empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

/* ========== 标签栏 ========== */
.tab-bar {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  overflow-x: auto;
  min-height: 36px;
  &::-webkit-scrollbar { height: 2px; }
}
.tab-item {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 12px;
  height: 36px;
  font-size: 12px;
  color: var(--el-text-color-regular);
  cursor: pointer;
  white-space: nowrap;
  border-right: 1px solid var(--el-border-color-lighter);
  transition: background 0.15s, color 0.15s;
  user-select: none;
  &:hover {
    background: var(--el-fill-color-light);
  }
  &.active {
    background: var(--el-bg-color);
    color: var(--el-text-color-primary);
    font-weight: 500;
    border-bottom: 2px solid var(--el-color-primary);
    margin-bottom: -1px;
  }
}
.tab-name {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tab-close {
  font-size: 12px;
  margin-left: 4px;
  color: var(--el-text-color-placeholder);
  cursor: pointer;
  &:hover { color: var(--el-color-danger); }
}

/* ========== 日志内容区 ========== */
.log-box {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
  font-family: 'Cascadia Code', 'JetBrains Mono', 'Consolas', 'Menlo', monospace;
  font-size: 13px;
  line-height: 1.8;
  background: #1a1b26;
  counter-reset: log-line;
}
.log-line {
  display: flex;
  align-items: baseline;
  gap: 0;
  padding: 2px 12px 2px 0;
  transition: background 0.1s;
  &:hover {
    background: rgba(255, 255, 255, 0.05);
  }
}
.log-ln {
  display: inline-block;
  width: 48px;
  min-width: 48px;
  text-align: right;
  padding-right: 10px;
  color: #565f89;
  font-size: 11px;
  user-select: none;
  flex-shrink: 0;
}
.log-ts-full {
  color: #565f89;
  font-size: 11px;
  margin-right: 8px;
  white-space: nowrap;
  flex-shrink: 0;
}
.log-level {
  display: inline-block;
  min-width: 48px;
  padding: 0 6px;
  margin-right: 6px;
  text-align: center;
  font-size: 10px;
  font-weight: 600;
  border-radius: 3px;
  flex-shrink: 0;
  letter-spacing: 0.5px;
}
.level-error {
  color: #fff;
  background: rgba(247, 118, 142, 0.8);
}
.level-warn {
  color: #1a1b26;
  background: rgba(224, 175, 104, 0.85);
}
.level-info {
  color: #9ece6a;
  background: rgba(158, 206, 106, 0.12);
}
.level-debug {
  color: #565f89;
  background: rgba(86, 95, 137, 0.15);
}
.log-container {
  display: inline-block;
  padding: 0 5px;
  margin-right: 6px;
  font-size: 11px;
  color: #7aa2f7;
  background: rgba(122, 162, 247, 0.1);
  border-radius: 3px;
  flex-shrink: 0;
}
.log-worker {
  display: inline-block;
  padding: 0 5px;
  margin-right: 4px;
  font-size: 11px;
  color: #bb9af7;
  background: rgba(187, 154, 247, 0.1);
  border-radius: 3px;
  flex-shrink: 0;
}
.log-task {
  display: inline-block;
  margin-right: 6px;
  font-size: 11px;
  color: #c0caf5;
  flex-shrink: 0;
}
.log-time {
  color: #565f89;
  font-size: 12px;
  margin-right: 8px;
  white-space: nowrap;
  flex-shrink: 0;
}
.log-body {
  color: #c0caf5;
  word-break: break-all;
  white-space: pre-wrap;
  flex: 1;
  min-width: 0;
}

/* stderr 整行着色 */
.log-stderr .log-body {
  color: #f7768e;
}
.log-error .log-body {
  color: #f7768e;
}
.log-warn .log-body {
  color: #e0af68;
}
.log-debug .log-body {
  color: #565f89;
}

/* ========== 滚动按钮 ========== */
.scroll-bottom-btn {
  position: absolute;
  bottom: 16px;
  right: 24px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 14px;
  border: 1px solid var(--el-border-color);
  border-radius: 16px;
  background: var(--el-bg-color);
  color: var(--el-text-color-regular);
  font-size: 12px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  transition: all 0.2s;
  &:hover {
    color: var(--el-color-primary);
    border-color: var(--el-color-primary);
  }
}

/* ========== 亮色模式覆盖（:global 逃逸 scoped） ========== */
:global(html:not(.dark)) .log-box {
  background: #f8f9fc;
}
:global(html:not(.dark)) .log-line:hover {
  background: rgba(0, 0, 0, 0.03);
}
:global(html:not(.dark)) .log-body {
  color: #343b58;
}
:global(html:not(.dark)) .log-stderr .log-body,
:global(html:not(.dark)) .log-error .log-body {
  color: #c64343;
}
:global(html:not(.dark)) .log-warn .log-body {
  color: #8f5e15;
}
:global(html:not(.dark)) .log-debug .log-body {
  color: #9699a3;
}
:global(html:not(.dark)) .log-ln {
  color: #c0c8d8;
}
:global(html:not(.dark)) .log-time {
  color: #9699a3;
}
:global(html:not(.dark)) .log-ts-full {
  color: #b0b8c8;
}
:global(html:not(.dark)) .level-info {
  color: #3d7a3d;
  background: rgba(61, 122, 61, 0.1);
}
:global(html:not(.dark)) .level-debug {
  color: #9699a3;
  background: rgba(150, 153, 163, 0.1);
}
:global(html:not(.dark)) .log-container {
  color: #2a5db0;
  background: rgba(42, 93, 176, 0.08);
}
:global(html:not(.dark)) .log-worker {
  color: #7c4dcc;
  background: rgba(124, 77, 204, 0.08);
}
</style>
