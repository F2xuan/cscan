<template>
  <div class="asset-timeline">
    <!-- 加载状态 -->
    <div v-if="loading" class="tl-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>{{ t('asset.timeline.loading') }}</span>
    </div>

    <!-- 错误状态 -->
    <div v-else-if="error" class="tl-state tl-error">
      <el-icon><WarningFilled /></el-icon>
      <span>{{ t('asset.timeline.loadError') }}</span>
      <el-button type="primary" size="small" @click="loadTimeline">
        {{ t('asset.timeline.retry') }}
      </el-button>
    </div>

    <!-- 空状态 -->
    <el-empty v-else-if="events.length === 0" :description="t('asset.timeline.empty')" />

    <!-- 时间线 -->
    <el-timeline v-else class="tl-list">
      <el-timeline-item
        v-for="(ev, i) in events"
        :key="i"
        :timestamp="formatTs(ev.time)"
        placement="top"
        :color="eventMeta[ev.type].color"
        :type="ev.type === 'first_found' ? 'primary' : ''"
      >
        <div class="tl-event">
          <div class="tl-event-head">
            <el-icon class="tl-icon"><component :is="eventMeta[ev.type].icon" /></el-icon>
            <span class="tl-type">{{ typeLabel(ev) }}</span>
            <el-tag v-if="ev.severity" :type="sevType(ev.severity)" size="small">
              {{ sevLabel(ev.severity) }}
            </el-tag>
          </div>

          <!-- 属性变化：展示 old -> new -->
          <div v-if="ev.type === 'property_change' && ev.changes && ev.changes.length" class="tl-changes">
            <div v-for="(c, ci) in ev.changes" :key="ci" class="tl-change-row">
              <span class="tl-field">{{ fieldLabel(c.field) }}</span>
              <span class="tl-old">{{ c.oldValue || '-' }}</span>
              <el-icon class="tl-arrow"><Right /></el-icon>
              <span class="tl-new">{{ c.newValue || '-' }}</span>
            </div>
          </div>

          <!-- 漏洞名称 -->
          <div v-else-if="ev.name" class="tl-name">{{ ev.name }}</div>
        </div>
      </el-timeline-item>
    </el-timeline>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import {
  Loading,
  WarningFilled,
  Flag,
  Edit,
  Warning,
  CircleCheck,
  Right
} from '@element-plus/icons-vue'
import request from '@/api/request'
import { getAssetChangeHistory } from '@/api/asset'

const { t } = useI18n()

const props = defineProps({
  assetId: {
    type: String,
    default: ''
  },
  authority: {
    type: String,
    default: ''
  },
  host: {
    type: String,
    default: ''
  },
  port: {
    type: Number,
    default: 0
  }
})

const loading = ref(false)
const error = ref(false)
const events = ref([])

const eventMeta = {
  first_found: { color: '#22c55e', icon: Flag },
  property_change: { color: '#3b82f6', icon: Edit },
  vuln_found: { color: '#f97316', icon: Warning },
  vuln_fixed: { color: '#67c23a', icon: CircleCheck }
}

// 将 "YYYY-MM-DD HH:MM:SS" 解析为时间戳用于排序
function parseTime(s) {
  if (!s) return 0
  const d = new Date(String(s).replace(' ', 'T'))
  const ms = d.getTime()
  return isNaN(ms) ? 0 : ms
}

async function loadTimeline() {
  if (!props.assetId && !props.authority && !props.host) {
    events.value = []
    return
  }
  loading.value = true
  error.value = false
  try {
    // 漏洞查询优先用 host（资产列表对象通常含 host/port，不含 authority）
    const vulQuery = { pageSize: 200, page: 1 }
    if (props.host) {
      vulQuery.host = props.host
      if (props.port > 0) vulQuery.port = props.port
    } else if (props.authority) {
      vulQuery.authority = props.authority
    }
    const [histRes, vulRes] = await Promise.all([
      getAssetChangeHistory({ assetId: props.assetId, limit: 50 }),
      request.post('/vul/list', vulQuery)
    ])

    const evs = []
    if (histRes && histRes.code === 0 && Array.isArray(histRes.list)) {
      for (const item of histRes.list) {
        if (item.changes && item.changes.length) {
          evs.push({ type: 'property_change', time: item.createTime, changes: item.changes, taskId: item.taskId })
        } else {
          evs.push({ type: 'first_found', time: item.createTime, taskId: item.taskId })
        }
      }
    }
    if (vulRes && vulRes.code === 0 && Array.isArray(vulRes.list)) {
      for (const v of vulRes.list) {
        evs.push({
          type: 'vuln_found',
          time: v.firstSeenTime || v.createTime,
          severity: v.severity,
          name: v.vulName,
          authority: v.authority
        })
        if (v.status === 'fixed' && v.fixedAt) {
          evs.push({ type: 'vuln_fixed', time: v.fixedAt, name: v.vulName })
        }
      }
    }
    // 按时间倒序
    evs.sort((a, b) => parseTime(b.time) - parseTime(a.time))
    events.value = evs
  } catch (e) {
    error.value = true
    console.error('load asset timeline failed:', e)
  } finally {
    loading.value = false
  }
}

function typeLabel(ev) {
  switch (ev.type) {
    case 'first_found':
      return t('asset.timeline.firstFound')
    case 'property_change':
      return t('asset.timeline.propertyChange')
    case 'vuln_found':
      return t('asset.timeline.vulnFound')
    case 'vuln_fixed':
      return t('asset.timeline.vulnFixed')
    default:
      return ev.type
  }
}

function fieldLabel(field) {
  if (!field) return '-'
  // 优先用 i18n 字段别名，缺失时回退原始字段名
  const key = `asset.timeline.field.${field}`
  const translated = t(key)
  return translated === key ? field : translated
}

function formatTs(s) {
  return s || ''
}

function sevType(sev) {
  const map = { critical: 'danger', high: 'danger', medium: 'warning', low: 'info', info: 'info', unknown: 'info' }
  return map[sev] || 'info'
}

function sevLabel(sev) {
  const map = {
    critical: t('vul.critical'),
    high: t('vul.high'),
    medium: t('vul.medium'),
    low: t('vul.low'),
    info: t('vul.info'),
    unknown: t('vul.unknown')
  }
  return map[sev] || sev
}

onMounted(loadTimeline)
watch(() => [props.assetId, props.authority, props.host, props.port], loadTimeline)

defineExpose({ loadTimeline })
</script>

<style scoped lang="scss">
.asset-timeline {
  padding: 4px 0;

  .tl-state {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 48px 20px;
    color: hsl(var(--muted-foreground));
    font-size: 14px;

    &.tl-error {
      flex-direction: column;
      .el-icon {
        font-size: 40px;
      }
    }
  }

  .tl-list {
    padding-left: 4px;
    :deep(.el-timeline-item__timestamp) {
      font-size: 12px;
      color: hsl(var(--muted-foreground));
    }
  }

  .tl-event {
    background: hsl(var(--muted) / 0.25);
    border: 1px solid hsl(var(--border));
    border-radius: 8px;
    padding: 12px 14px;

    .tl-event-head {
      display: flex;
      align-items: center;
      gap: 8px;

      .tl-icon {
        font-size: 16px;
        color: hsl(var(--primary));
      }
      .tl-type {
        font-weight: 600;
        font-size: 14px;
        color: hsl(var(--foreground));
      }
    }

    .tl-changes {
      margin-top: 10px;
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    .tl-change-row {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;
      flex-wrap: wrap;

      .tl-field {
        color: hsl(var(--muted-foreground));
        min-width: 80px;
      }
      .tl-old {
        color: hsl(var(--muted-foreground));
        text-decoration: line-through;
        max-width: 280px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .tl-arrow {
        color: hsl(var(--primary));
        font-size: 14px;
      }
      .tl-new {
        color: hsl(var(--foreground));
        font-weight: 500;
        max-width: 280px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .tl-name {
      margin-top: 8px;
      font-size: 13px;
      color: hsl(var(--foreground));
    }
  }
}
</style>
