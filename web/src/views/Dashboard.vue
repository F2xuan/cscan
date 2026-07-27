<template>
  <div class="dashboard-modern" :class="[{ 'is-dark': themeStore.isDark }, `style-${themeStore.themeStyle}`]">
    <!-- 第一排：5个资产数据卡片 -->
    <div class="top-cards-row">
      <!-- 域名 -->
      <div class="stat-card domain-card" @click="goAsset('domain')">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardDomain') }}
            <el-icon class="card-icon"><Postcard /></el-icon>
          </div>
          <div class="card-value">{{ animatedData.domains }}</div>
          <div class="card-sub" :class="{ positive: stats.domainNew > 0 }">
            {{ t('dashboard.compareYesterday') }} {{ stats.domainNew > 0 ? '↑' : '—' }} {{ stats.domainNew }}
          </div>
        </div>
      </div>

      <!-- IP -->
      <div class="stat-card ip-card" @click="goAsset('ip')">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardIp') }}
            <el-icon class="card-icon"><MapLocation /></el-icon>
          </div>
          <div class="card-value">{{ animatedData.ips }}</div>
          <div class="card-sub" :class="{ positive: stats.ipNew > 0 }">
            {{ t('dashboard.compareYesterday') }} {{ stats.ipNew > 0 ? '↑' : '—' }} {{ stats.ipNew }}
          </div>
        </div>
      </div>

      <!-- 端口 -->
      <div class="stat-card port-card" @click="goInventory()">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardPort') }}
            <el-icon class="card-icon"><Monitor /></el-icon>
          </div>
          <div class="card-value">{{ animatedData.ports }}</div>
          <div class="card-sub" :class="{ positive: stats.assetNew > 0 }">
            {{ t('dashboard.compareYesterday') }} {{ stats.assetNew > 0 ? '↑' : '—' }} {{ stats.assetNew }}
          </div>
        </div>
      </div>

      <!-- 站点 -->
      <div class="stat-card site-card" @click="goAsset('site')">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardSite') }}
            <el-icon class="card-icon"><Box /></el-icon>
          </div>
          <div class="card-value">{{ animatedData.sites }}</div>
          <div class="card-sub" :class="{ positive: stats.siteNew > 0 }">
            {{ t('dashboard.compareYesterday') }} {{ stats.siteNew > 0 ? '↑' : '—' }} {{ stats.siteNew }}
          </div>
        </div>
      </div>

      <!-- 分组 -->
      <div class="stat-card group-card" @click="$router.push('/asset-management?tab=groups')">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardGroup') }}
            <el-icon class="card-icon"><FolderOpened /></el-icon>
          </div>
          <div class="card-value">{{ animatedData.groups }}</div>
          <div class="card-sub neutral">
            {{ t('dashboard.workspaceIsolation') }}
          </div>
        </div>
      </div>
    </div>

    <!-- 第二排：3个风险与状态卡片 -->
    <div class="middle-cards-row">
      <!-- 任务执行与 Worker -->
      <div class="stat-card task-card" @click="$router.push('/task')">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardTask') }}
            <el-icon class="card-icon"><Cpu /></el-icon>
          </div>
          <div class="card-value">{{ taskStats.running }} / {{ workerStats.online }}</div>
          <div class="card-sub neutral">{{ t('dashboard.queuedTasks') }}: {{ taskStats.pending }}</div>
        </div>
      </div>

      <!-- 目录扫描 / 敏感文件 -->
      <div class="stat-card dir-card" @click="goAsset('dirscan')">
        <div class="card-glow"></div>
        <div class="card-content">
          <div class="card-header">
            <div class="dot"></div>{{ t('dashboard.cardDir') }}
            <el-icon class="card-icon"><Lightning /></el-icon>
          </div>
          <div class="card-value">{{ animatedData.dirScans }}</div>
          <div class="card-sub neutral">{{ t('dashboard.exposureMonitor') }}</div>
        </div>
      </div>

      <!-- 漏洞全景 (占据更大宽度) -->
      <div class="stat-card vuln-card" @click="$router.push('/asset-management?tab=inventory&subTab=vul')">
        <div class="card-content vuln-content">
          <div class="vuln-left">
            <div class="card-header">
              <div class="dot danger-dot"></div>{{ t('dashboard.cardVuln') }}
              <el-icon class="card-icon danger-icon"><Aim /></el-icon>
            </div>
            <div class="card-value danger-text">{{ animatedData.vulns }}</div>
            <div class="card-sub neutral">{{ t('dashboard.lifecycleThreat') }}</div>
          </div>
          <div class="vuln-right">
            <div class="vuln-bubbles">
              <div class="bubble">
                <el-progress type="circle" :percentage="calcPercent(stats.vulnCritical, stats.vulns)" :width="48" :stroke-width="4" color="#f53f3f" :show-text="false" />
                <div class="bubble-inner"><span class="pct">{{ calcPercent(stats.vulnCritical, stats.vulns) }}%</span></div>
                <div class="bubble-label"><span class="sv-num critical">{{ stats.vulnCritical }}</span>{{ t('dashboard.critical') }}</div>
              </div>
              <div class="bubble">
                <el-progress type="circle" :percentage="calcPercent(stats.vulnHigh, stats.vulns)" :width="48" :stroke-width="4" color="#ff7d00" :show-text="false" />
                <div class="bubble-inner"><span class="pct">{{ calcPercent(stats.vulnHigh, stats.vulns) }}%</span></div>
                <div class="bubble-label"><span class="sv-num high">{{ stats.vulnHigh }}</span>{{ t('dashboard.high') }}</div>
              </div>
              <div class="bubble">
                <el-progress type="circle" :percentage="calcPercent(stats.vulnMedium, stats.vulns)" :width="48" :stroke-width="4" color="#f7ba1e" :show-text="false" />
                <div class="bubble-inner"><span class="pct">{{ calcPercent(stats.vulnMedium, stats.vulns) }}%</span></div>
                <div class="bubble-label"><span class="sv-num medium">{{ stats.vulnMedium }}</span>{{ t('dashboard.medium') }}</div>
              </div>
              <div class="bubble">
                <el-progress type="circle" :percentage="calcPercent(stats.vulnLow, stats.vulns)" :width="48" :stroke-width="4" color="#165dff" :show-text="false" />
                <div class="bubble-inner"><span class="pct">{{ calcPercent(stats.vulnLow, stats.vulns) }}%</span></div>
                <div class="bubble-label"><span class="sv-num low">{{ stats.vulnLow }}</span>{{ t('dashboard.low') }}</div>
              </div>
               <div class="bubble">
                <el-progress type="circle" :percentage="calcPercent(stats.vulnInfo, stats.vulns)" :width="48" :stroke-width="4" color="#86909c" :show-text="false" />
                <div class="bubble-inner"><span class="pct">{{ calcPercent(stats.vulnInfo, stats.vulns) }}%</span></div>
                <div class="bubble-label"><span class="sv-num info">{{ stats.vulnInfo }}</span>{{ t('dashboard.info') }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 第三排：宽版趋势图 -->
    <div class="chart-row-large">
      <div class="chart-card">
        <div class="chart-header">
           <div class="title"><div class="vertical-bar"></div> {{ t('dashboard.trendChartTitle') }}</div>
        </div>
        <div ref="trendChartRef" class="chart-container trend-container"></div>
      </div>
    </div>

    <!-- 第四排：3x2 图表网格 -->
    <div class="bottom-charts-grid">
      <!-- 漏洞等级占比 -->
      <div class="chart-card">
         <div class="chart-header">
           <div class="title"><div class="vertical-bar warning"></div> {{ t('dashboard.vulnLevelPie') }}</div>
           <el-icon class="info-icon"><Warning /></el-icon>
         </div>
         <div ref="vulnPieChartRef" class="chart-container"></div>
      </div>

      <!-- 各资产类型占比 -->
      <div class="chart-card">
         <div class="chart-header">
           <div class="title"><div class="vertical-bar success"></div> {{ t('dashboard.assetTypeStruct') }}</div>
         </div>
         <div ref="assetTypePieChartRef" class="chart-container"></div>
      </div>

      <!-- 端口占比 TOP 10 -->
      <div class="chart-card">
         <div class="chart-header">
           <div class="title"><div class="vertical-bar primary"></div> {{ t('dashboard.portTop10') }}</div>
         </div>
         <div ref="portBarChartRef" class="chart-container"></div>
      </div>

       <!-- 指纹分类 TOP 10 -->
      <div class="chart-card">
         <div class="chart-header">
           <div class="title"><div class="vertical-bar primary"></div> {{ t('dashboard.fingerprintCategory') }}</div>
         </div>
         <div class="chart-container">
           <div v-if="stats.topApp.length === 0" class="empty-data">
             <el-icon><Box /></el-icon>{{ t('dashboard.noFingerprintData') }}
           </div>
           <div v-else ref="appRoseChartRef" style="width:100%;height:100%;"></div>
         </div>
      </div>

      <!-- 服务占比 TOP 10 -->
      <div class="chart-card">
         <div class="chart-header">
           <div class="title"><div class="vertical-bar success"></div> {{ t('dashboard.coreService') }}</div>
         </div>
         <div class="chart-container">
           <div v-if="stats.topService.length === 0" class="empty-data">
             <el-icon><Box /></el-icon>{{ t('dashboard.noServiceData') }}
           </div>
           <div v-else ref="servicePieChartRef" style="width:100%;height:100%;"></div>
         </div>
      </div>

      <!-- 指纹占比 TOP 10 -->
      <div class="chart-card">
         <div class="chart-header">
           <div class="title"><div class="vertical-bar"></div> {{ t('dashboard.fingerprintPerspective') }}</div>
         </div>
         <div class="chart-container">
           <div v-if="stats.topApp.length === 0" class="empty-data">
             <el-icon><Box /></el-icon>{{ t('dashboard.noFingerprintData') }}
           </div>
           <div v-else ref="appPieChartRef" style="width:100%;height:100%;"></div>
         </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import request from '@/api/request'
import { useThemeStore } from '@/stores/theme'
import {
  Postcard, MapLocation, Monitor, Box, FolderOpened, Cpu, Lightning, Aim, Warning
} from '@element-plus/icons-vue'

const router = useRouter()
const themeStore = useThemeStore()
const { t } = useI18n()

// === 数据整合区 ===
const stats = reactive({
  ports: 0, assetNew: 0,
  groups: 0,
  ips: 0, ipNew: 0,
  domains: 0, domainNew: 0,
  sites: 0, siteNew: 0,
  dirScans: 0,
  vulns: 0, vulnCritical: 0, vulnHigh: 0, vulnMedium: 0, vulnLow: 0, vulnInfo: 0,
  topPorts: [], topService: [], topApp: []
})

const animatedData = reactive({
  ports: 0, groups: 0, ips: 0, domains: 0, sites: 0, dirScans: 0, vulns: 0
})

const taskStats = reactive({
  total: 0, completed: 0, running: 0, failed: 0, pending: 0,
  trendDays: [], trendCompleted: [], trendFailed: []
})

const workerStats = reactive({ online: 0, offline: 0 })

// === 图表 Ref ===
const trendChartRef = ref()
const vulnPieChartRef = ref()
const assetTypePieChartRef = ref()
const portBarChartRef = ref()
const servicePieChartRef = ref()
const appPieChartRef = ref()
const appRoseChartRef = ref()
let charts = {}
let refreshInterval = null

// === 路由跳转 ===
function goAsset(subTab) {
  router.push(`/asset-management?tab=inventory&subTab=${subTab}`)
}

function goInventory() {
  router.push(`/asset-management?tab=inventory`)
}

// === 辅助方法 ===
function calcPercent(part, total) {
  if (total === 0) return 0
  return Math.round((part / total) * 100)
}

function animateValue(key, target, duration = 1200) {
  const start = animatedData[key]
  const change = target - start
  if (change === 0) return
  const startTime = performance.now()

  function update(currentTime) {
    const elapsed = currentTime - startTime
    const progress = Math.min(elapsed / duration, 1)
    const easeProgress = 1 - Math.pow(1 - progress, 3)
    animatedData[key] = Math.round(start + change * easeProgress)
    if (progress < 1) requestAnimationFrame(update)
  }
  requestAnimationFrame(update)
}

// === API 拉取 ===
async function silentFetch(apiRoute, params = {}) {
  try {
    const res = await request.post(apiRoute, params)
    return res.code === 0 ? res : null
  } catch (e) {
    return null
  }
}

async function loadAllData() {
  await Promise.all([
    fetchAssetStat(),
    fetchGroupsStat(),
    fetchIpStat(),
    fetchDomainStat(),
    fetchSiteStat(),
    fetchDirScanStat(),
    fetchVulnStat(),
    fetchTaskStat(),
    fetchWorkerStat()
  ])
  await nextTick()
  initAllCharts()
}

async function fetchAssetStat() {
  const res = await silentFetch('/asset/stat')
  if (res) {
    stats.ports = res.totalAsset || 0
    stats.assetNew = res.newCount || 0
    stats.topPorts = res.topPorts || []
    stats.topService = res.topService || []
    stats.topApp = res.topApp || []
    animateValue('ports', stats.ports)
  }
}

async function fetchGroupsStat() {
  const res = await silentFetch('/asset/groups', { page: 1, pageSize: 1 })
  if (res) {
    stats.groups = res.total || 0
    animateValue('groups', stats.groups)
  }
}

async function fetchIpStat() {
  const res = await silentFetch('/asset/ip/stat')
  if (res) {
    stats.ips = res.total || 0
    stats.ipNew = res.newCount || 0
    animateValue('ips', stats.ips)
  }
}

async function fetchDomainStat() {
  const res = await silentFetch('/asset/domain/stat')
  if (res) {
    stats.domains = res.total || 0
    stats.domainNew = res.newCount || 0
    animateValue('domains', stats.domains)
  }
}

async function fetchSiteStat() {
  const res = await silentFetch('/asset/site/stat')
  if (res) {
    stats.sites = res.total || 0
    stats.siteNew = res.newCount || 0
    animateValue('sites', stats.sites)
  }
}

async function fetchDirScanStat() {
  const statRes = await silentFetch('/dirscan/result/stat')
  if (statRes && statRes.stat) {
    stats.dirScans = statRes.stat.total || 0
  } else {
    const listRes = await silentFetch('/dirscan/result/list', { page: 1, pageSize: 1 })
    if (listRes) {
      stats.dirScans = listRes.total || 0
    }
  }
  animateValue('dirScans', stats.dirScans)
}

async function fetchVulnStat() {
  const statRes = await silentFetch('/vul/stat')
  if (statRes) {
    stats.vulns = statRes.total || 0
    stats.vulnCritical = statRes.critical || 0
    stats.vulnHigh = statRes.high || 0
    stats.vulnMedium = statRes.medium || 0
    stats.vulnLow = statRes.low || 0
    stats.vulnInfo = statRes.info || 0
    animateValue('vulns', stats.vulns)
  }
}

async function fetchTaskStat() {
  const res = await silentFetch('/task/stat')
  if (res) {
    taskStats.total = res.total || 0
    taskStats.completed = res.completed || 0
    taskStats.running = res.running || 0
    taskStats.failed = res.failed || 0
    taskStats.pending = res.pending || 0
    taskStats.trendDays = res.trendDays || []
    taskStats.trendCompleted = res.trendCompleted || []
    taskStats.trendFailed = res.trendFailed || []
  }
}

async function fetchWorkerStat() {
  const res = await silentFetch('/worker/list')
  if (res && res.list) {
    workerStats.online = res.list.filter(w => w.status === 'running').length
    workerStats.offline = res.list.filter(w => w.status !== 'running').length
  }
}

// === 图表渲染 ===
function getThemeColors() {
  const isDark = themeStore.isDark
  return {
    text: isDark ? '#86909c' : '#4e5969',
    title: isDark ? '#e5e6eb' : '#1d2129',
    line: isDark ? '#3f434a' : '#e5e6eb',
    tooltipBg: isDark ? '#232324' : '#ffffff',
    tooltipBorder: isDark ? '#3f434a' : '#e5e6eb',
    tooltipText: isDark ? '#e5e6eb' : '#1d2129',
    palette: ['#165dff', '#14c9c9', '#f7ba1e', '#ff7d00', '#f53f3f', '#722ed1', '#d91ad9', '#eb0aa4']
  }
}

const SV_COLORS = {
  critical: '#f53f3f', high: '#ff7d00', medium: '#f7ba1e', low: '#165dff', info: '#86909c'
}

function initAllCharts() {
  initTrendChart()
  initVulnPieChart()
  initAssetTypePieChart()
  initPortBarChart()
  initServicePieChart()
  initAppPieChart()
  initAppRoseChart()
}

function initTrendChart() {
  if (!trendChartRef.value) return
  if (!charts.trend) charts.trend = echarts.init(trendChartRef.value)
  const c = getThemeColors()

  let days = taskStats.trendDays.length ? taskStats.trendDays : ['03-27', '03-28', '03-29', '03-30', '03-31', '04-01', '04-02']
  let completed = taskStats.trendCompleted.length ? taskStats.trendCompleted : [0, 0, 0, 0, 0, 0, 0]
  let failed = taskStats.trendFailed.length ? taskStats.trendFailed : [0, 0, 0, 0, 0, 0, 0]

  charts.trend.setOption({
    backgroundColor: 'transparent',
    grid: { left: '2%', right: '2%', bottom: '5%', top: '15%', containLabel: true },
    tooltip: { trigger: 'axis', backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    legend: { icon: 'circle', right: '0%', top: 0, textStyle: { color: c.text }, itemWidth: 10, itemHeight: 10 },
    xAxis: {
      type: 'category', boundaryGap: false, data: days,
      axisLabel: { color: c.text, fontSize: 12, margin: 12 },
      axisLine: { lineStyle: { color: c.line } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: c.text, fontSize: 12 },
      splitLine: { lineStyle: { color: c.line, type: 'dashed' } }
    },
    series: [
      {
        name: t('dashboard.successTask'), type: 'line', smooth: true, symbolSize: 8,
        itemStyle: { color: '#165dff' },
        lineStyle: { width: 3, shadowColor: 'rgba(22,93,255,0.3)', shadowBlur: 10, shadowOffsetY: 5 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(22,93,255,0.2)' },
            { offset: 1, color: 'rgba(22,93,255,0.0)' }
          ])
        },
        data: completed
      },
      {
        name: t('dashboard.failedTask'), type: 'line', smooth: true, symbolSize: 8,
        itemStyle: { color: '#f53f3f' },
        lineStyle: { width: 3, shadowColor: 'rgba(245,63,63,0.3)', shadowBlur: 10, shadowOffsetY: 5 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245,63,63,0.2)' },
            { offset: 1, color: 'rgba(245,63,63,0.0)' }
          ])
        },
        data: failed
      }
    ]
  })
}

function initVulnPieChart() {
  if (!vulnPieChartRef.value) return
  if (!charts.vulnPie) charts.vulnPie = echarts.init(vulnPieChartRef.value)
  const c = getThemeColors()

  const data = [
    { value: stats.vulnCritical, name: t('dashboard.critical'), itemStyle: { color: SV_COLORS.critical } },
    { value: stats.vulnHigh, name: t('dashboard.high'), itemStyle: { color: SV_COLORS.high } },
    { value: stats.vulnMedium, name: t('dashboard.medium'), itemStyle: { color: SV_COLORS.medium } },
    { value: stats.vulnLow, name: t('dashboard.low'), itemStyle: { color: SV_COLORS.low } },
    { value: stats.vulnInfo, name: t('dashboard.info'), itemStyle: { color: SV_COLORS.info } }
  ].filter(d => d.value > 0)

  if (data.length === 0) data.push({ value: 1, name: t('dashboard.noRisk'), itemStyle: { color: c.line } })

  charts.vulnPie.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    legend: { icon: 'circle', right: '5%', top: '15%', orient: 'vertical', textStyle: { color: c.text, fontSize: 12 }, itemGap: 14 },
    series: [{
      type: 'pie',
      radius: ['55%', '80%'],
      center: ['40%', '50%'],
      avoidLabelOverlap: false,
      itemStyle: { borderColor: themeStore.isDark ? '#232324' : '#ffffff', borderWidth: 2 },
      label: { show: true, position: 'center', formatter: () => `${t('dashboard.totalVulnerabilities')}\n\n{num|${stats.vulns}}`, color: c.title, fontSize: 14, rich: { num: { fontSize: 28, fontWeight: 'bold' } } },
      labelLine: { show: false },
      data: data
    }]
  })
}

function initAssetTypePieChart() {
  if (!assetTypePieChartRef.value) return
  if (!charts.assetType) charts.assetType = echarts.init(assetTypePieChartRef.value)
  const c = getThemeColors()

  const data = [
    { value: stats.domains, name: t('dashboard.domainLabel'), itemStyle: { color: '#165dff' } },
    { value: stats.ips, name: t('dashboard.ipLabel'), itemStyle: { color: '#14c9c9' } },
    { value: stats.ports, name: t('dashboard.portLabel'), itemStyle: { color: '#f7ba1e' } },
    { value: stats.sites, name: t('dashboard.siteLabel'), itemStyle: { color: '#722ed1' } }
  ].filter(d => d.value > 0)

  if (data.length === 0) data.push({ value: 1, name: t('dashboard.noChartData'), itemStyle: { color: c.line } })

  charts.assetType.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    legend: { icon: 'circle', bottom: '0%', left: 'center', orient: 'horizontal', textStyle: { color: c.text, fontSize: 12 }, itemGap: 20 },
    series: [{
      type: 'pie',
      radius: '65%',
      center: ['50%', '45%'],
      itemStyle: { borderColor: themeStore.isDark ? '#232324' : '#ffffff', borderWidth: 2 },
      label: { show: false },
      data: data
    }]
  })
}

function initPortBarChart() {
  if (!portBarChartRef.value) return
  if (!charts.portBar) charts.portBar = echarts.init(portBarChartRef.value)
  const c = getThemeColors()

  let sourceData = [...stats.topPorts].sort((a, b) => a.count - b.count).slice(-10)
  const names = sourceData.map(d => String(d.name))
  const values = sourceData.map(d => d.count)

  if(names.length === 0) return

  charts.portBar.setOption({
    backgroundColor: 'transparent',
    grid: { left: '2%', right: '8%', bottom: '5%', top: '5%', containLabel: true },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    xAxis: { type: 'value', show: false },
    yAxis: { type: 'category', data: names, axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: c.text, fontSize: 12, margin: 15 } },
    series: [{
      type: 'bar',
      data: values,
      barWidth: 10,
      showBackground: true,
      backgroundStyle: { color: c.line, borderRadius: 5 },
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
          { offset: 0, color: '#14c9c9' },
          { offset: 1, color: '#165dff' }
        ]),
        borderRadius: [0, 5, 5, 0]
      },
      label: { show: true, position: 'right', formatter: '{c}', color: c.title, fontSize: 12, padding: [0, 0, 0, 8] }
    }]
  })
}

function initServicePieChart() {
  if (!servicePieChartRef.value || stats.topService.length === 0) return
  if (!charts.servicePie) charts.servicePie = echarts.init(servicePieChartRef.value)
  const c = getThemeColors()

  const data = stats.topService.slice(0, 10).map((d, i) => ({
    name: d.name, value: d.count, itemStyle: { color: c.palette[i % c.palette.length] }
  }))

  charts.servicePie.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    legend: { show: false },
    series: [{
      type: 'pie',
      radius: ['45%', '75%'],
      center: ['50%', '50%'],
      itemStyle: { borderColor: themeStore.isDark ? '#232324' : '#ffffff', borderWidth: 2 },
      label: { color: c.text, fontSize: 12, formatter: '{b} ({d}%)' },
      labelLine: { lineStyle: { color: c.line }, smooth: true },
      data: data
    }]
  })
}

function initAppPieChart() {
  if (!appPieChartRef.value || stats.topApp.length === 0) return
  if (!charts.appPie) charts.appPie = echarts.init(appPieChartRef.value)
  const c = getThemeColors()

  const data = stats.topApp.slice(0, 10).map((d, i) => ({
    name: d.name, value: d.count, itemStyle: { color: c.palette[i % c.palette.length] }
  }))

  charts.appPie.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    legend: { show: false },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['50%', '50%'],
      itemStyle: { borderColor: themeStore.isDark ? '#232324' : '#ffffff', borderWidth: 2 },
      label: { color: c.text, fontSize: 12, formatter: '{b}' },
      labelLine: { show: true, length: 15, length2: 10, lineStyle: { color: c.line }, smooth: true },
      data: data
    }]
  })
}

function initAppRoseChart() {
  if (!appRoseChartRef.value || stats.topApp.length === 0) return
  if (!charts.appRose) charts.appRose = echarts.init(appRoseChartRef.value)
  const c = getThemeColors()

  const reversedData = [...stats.topApp].slice(0, 8).reverse().map((d, i) => ({
    name: d.name, value: d.count, itemStyle: { color: c.palette[i % c.palette.length] }
  }))

  charts.appRose.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: c.tooltipBg, borderColor: c.tooltipBorder, textStyle: { color: c.tooltipText } },
    legend: { show: false },
    series: [{
      type: 'pie',
      roseType: 'area',
      radius: ['20%', '80%'],
      center: ['50%', '50%'],
      itemStyle: { borderRadius: 4, borderColor: themeStore.isDark ? '#232324' : '#ffffff', borderWidth: 2 },
      label: { color: c.text, fontSize: 12 },
      labelLine: { lineStyle: { color: c.line }, smooth: true },
      data: reversedData
    }]
  })
}

// === 生命周期与 Watchers ===
function handleResize() {
  Object.values(charts).forEach(chart => chart?.resize())
}

function handleWorkspaceChanged() {
  loadAllData()
}

// 侦听主题变化！重绘画布的色彩主题
watch(() => themeStore.isDark, () => {
  nextTick(() => {
    initAllCharts()
  })
})

onMounted(async () => {
  await loadAllData()
  refreshInterval = setInterval(loadAllData, 60000)
  window.addEventListener('resize', handleResize)
  window.addEventListener('workspace-changed', handleWorkspaceChanged)
})

onUnmounted(() => {
  clearInterval(refreshInterval)
  Object.values(charts).forEach(chart => chart?.dispose())
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('workspace-changed', handleWorkspaceChanged)
})
</script>

<style scoped lang="scss">
/**
 * 现代精致美学（Refined Cyber/Glassy Aesthetic）
 * 完全适配主题模式，使用优雅的渐变衬底与模糊过渡。
 */
.dashboard-modern {
  padding: 24px;
  background: hsl(var(--background));
  min-height: calc(100vh - 60px);
  display: flex;
  flex-direction: column;
  gap: 24px;
  color: hsl(var(--foreground));
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  transition: background-color 0.3s ease;
}

// 布局
.top-cards-row {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 20px;

  @media (max-width: 1400px) { grid-template-columns: repeat(3, 1fr); }
  @media (max-width: 900px) { grid-template-columns: repeat(2, 1fr); }
}

.middle-cards-row {
  display: grid;
  grid-template-columns: 2fr 2fr 3.5fr;
  gap: 20px;

  @media (max-width: 1400px) { grid-template-columns: 1fr 1fr; }
  @media (max-width: 900px) { grid-template-columns: 1fr; }
}

.chart-row-large {
  display: flex;
  .chart-card {
    flex: 1;
    .trend-container { height: 320px; }
  }
}

.bottom-charts-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
  @media (max-width: 1200px) { grid-template-columns: repeat(2, 1fr); }
  @media (max-width: 800px) { grid-template-columns: 1fr; }
}

// ==========================================
// 核心卡片组件 (Glassmorphism & Globs)
// ==========================================
.stat-card {
  position: relative;
  height: 124px;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 20px;
  cursor: pointer;
  overflow: hidden;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.02);
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

  &.vuln-card {
    height: auto;
    min-height: 124px;
  }

  // 悬停交互：浮起并加深弥散光
  &:hover {
    transform: translateY(-4px);
    box-shadow: 0 12px 24px rgba(0, 0, 0, 0.05);
    border-color: hsl(var(--primary) / 0.4);

    .card-glow {
      transform: scale(1.2);
      opacity: 0.8;
    }

    .card-icon {
      transform: scale(1.1) rotate(5deg);
    }
  }

  // 光晕背景元素
  .card-glow {
    position: absolute;
    top: -50px;
    right: -40px;
    width: 160px;
    height: 160px;
    border-radius: 50%;
    filter: blur(40px);
    opacity: 0.5;
    transition: all 0.5s ease;
    z-index: 0;
    pointer-events: none;
  }

  // 为各个卡片配置专属色调的光晕
  &.domain-card .card-glow { background: rgba(22, 93, 255, 0.3); }
  &.ip-card .card-glow { background: rgba(20, 201, 201, 0.3); }
  &.port-card .card-glow { background: rgba(247, 186, 30, 0.3); }
  &.site-card .card-glow { background: rgba(114, 46, 209, 0.3); }
  &.group-card .card-glow { background: rgba(235, 10, 164, 0.3); }
  &.task-card .card-glow { background: rgba(114, 46, 209, 0.3); }
  &.dir-card .card-glow { background: rgba(245, 63, 63, 0.2); }

  // 内部内容栈
  .card-content {
    position: relative;
    z-index: 1;
    padding: 20px 24px;
    height: 100%;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .card-header {
    display: flex;
    align-items: center;
    font-size: 14px;
    font-weight: 500;
    color: hsl(var(--muted-foreground));

    .dot {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: hsl(var(--foreground));
      margin-right: 8px;
      opacity: 0.6;

      &.danger-dot { background: #f53f3f; opacity: 1; box-shadow: 0 0 8px rgba(245,63,63,0.6); }
    }

    .card-icon {
      margin-left: auto;
      font-size: 20px;
      color: hsl(var(--foreground));
      opacity: 0.8;
      transition: all 0.3s ease;

      &.danger-icon { color: #f53f3f; opacity: 1; }
    }
  }

  .card-value {
    font-size: 34px;
    font-weight: 700;
    color: hsl(var(--foreground));
    line-height: 1;
    letter-spacing: -0.5px;
    margin-top: auto;
    margin-bottom: 6px;

    &.danger-text { color: #f53f3f; }
  }

  .card-sub {
    font-size: 12px;
    color: hsl(var(--muted-foreground));
    font-weight: 500;
    display: flex;
    align-items: center;

    &.positive { color: #00b42a; }
    &.negative { color: #f53f3f; }
    &.neutral { color: hsl(var(--muted-foreground)); }
  }
}

// 漏洞全景特殊排版
.vuln-content {
  flex-direction: row !important;
  align-items: center;
  justify-content: space-between !important;
  gap: 20px;

  .vuln-left {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    height: 100%;
    min-width: 140px;
  }

  .vuln-right {
    flex: 1;
    display: flex;
    justify-content: flex-end;
  }

  .vuln-bubbles {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
    justify-content: flex-end;

    .bubble {
      position: relative;
      display: flex;
      flex-direction: column;
      align-items: center;
      background: hsl(var(--muted) / 0.3);
      padding: 10px 14px;
      border-radius: 16px;
      border: 1px solid hsl(var(--border) / 0.5);

      .bubble-inner {
        position: absolute;
        top: 25px; /* 进度条圆心位置 */
        left: 0;
        width: 100%;
        text-align: center;
        .pct { font-size: 11px; font-weight: 600; color: hsl(var(--foreground)); }
      }

      .bubble-label {
        margin-top: 8px;
        font-size: 12px;
        color: hsl(var(--muted-foreground));
        display: flex;
        align-items: baseline;
        gap: 4px;

        .sv-num {
          font-weight: 700;
          font-size: 16px;
          &.critical { color: #f53f3f; }
          &.high { color: #ff7d00; }
          &.medium { color: #f7ba1e; }
          &.low { color: #165dff; }
          &.info { color: #86909c; }
        }
      }
    }
  }
}

// ==========================================
// 图表卡片排版
// ==========================================
.chart-card {
  background: hsl(var(--card));
  border-radius: 20px;
  border: 1px solid hsl(var(--border));
  box-shadow: 0 4px 16px rgba(0,0,0,0.02);
  padding: 24px;
  display: flex;
  flex-direction: column;
  transition: border-color 0.3s ease;

  &:hover {
    border-color: hsl(var(--border-hover, var(--border)));
  }

  .chart-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;

    .title {
      font-size: 15px;
      font-weight: 600;
      color: hsl(var(--foreground));
      display: flex;
      align-items: center;

      .vertical-bar {
        width: 4px;
        height: 16px;
        border-radius: 4px;
        background: #165dff;
        margin-right: 10px;

        &.primary { background: #165dff; }
        &.success { background: #00b42a; }
        &.warning { background: #ff7d00; }
        &.danger { background: #f53f3f; }
      }
    }

    .info-icon {
      color: hsl(var(--muted-foreground));
      font-size: 16px;
      cursor: help;
    }
  }

  .chart-container {
    flex: 1;
    min-height: 280px;
    position: relative;
    z-index: 1;
  }

  .empty-data {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    color: hsl(var(--muted-foreground));
    font-size: 13px;
    gap: 12px;
    background: hsl(var(--card) / 0.5);

    .el-icon {
      font-size: 36px;
      opacity: 0.5;
    }
  }
}
</style>