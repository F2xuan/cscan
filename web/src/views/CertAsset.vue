<template>
  <div class="cert-asset-page">
    <div class="page-header">
      <div class="header-content">
        <h1>{{ $t('certAsset.title') }}</h1>
        <p class="description">{{ $t('certAsset.description') }}</p>
      </div>
    </div>
    <el-card>
      <el-form :inline="true" class="filter-form">
        <el-form-item :label="$t('certAsset.query')">
          <el-input v-model="query" :placeholder="$t('certAsset.queryPlaceholder')" clearable style="width: 220px" @keyup.enter="handleQuery" />
        </el-form-item>
        <el-form-item :label="$t('certAsset.issuer')">
          <el-input v-model="issuer" :placeholder="$t('certAsset.issuerPlaceholder')" clearable style="width: 200px" @keyup.enter="handleQuery" />
        </el-form-item>
        <el-form-item :label="$t('certAsset.validity')">
          <el-select v-model="validity" :placeholder="$t('certAsset.validityPlaceholder')" clearable style="width: 160px" @change="handleQuery">
            <el-option :label="$t('certAsset.status_valid')" value="valid" />
            <el-option :label="$t('certAsset.status_expired')" value="invalid" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleQuery"><el-icon><Search /></el-icon>{{ $t('common.search') }}</el-button>
          <el-button @click="handleReset"><el-icon><Refresh /></el-icon>{{ $t('common.reset') }}</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="tableData" v-loading="loading" border stripe max-height="600">
        <el-table-column prop="authority" :label="$t('certAsset.authority')" min-width="200" show-overflow-tooltip />
        <el-table-column prop="subjectDN" :label="$t('certAsset.subject')" min-width="280" show-overflow-tooltip />
        <el-table-column prop="issuerDN" :label="$t('certAsset.issuer')" min-width="280" show-overflow-tooltip />
        <el-table-column :label="$t('certAsset.notAfter')" width="160">
          <template #default="{ row }">
            <div class="not-after-cell">
              <span>{{ formatDate(row.notAfter) }}</span>
              <el-tag :type="statusTagType(row)" size="small" effect="light" class="status-tag">
                {{ statusLabel(row) }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('certAsset.selfSigned')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.isSelfSigned ? 'warning' : 'success'" size="small">
              {{ row.isSelfSigned ? $t('common.yes') : $t('common.no') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('certAsset.sha256')" width="160">
          <template #default="{ row }">
            <span class="fingerprint">{{ row.fingerprints?.sha256?.substring(0, 16) }}...</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="showDetail(row)">{{ $t('common.view') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @size-change="loadData"
        @current-change="loadData"
        style="margin-top: 16px; justify-content: flex-end;"
      />
    </el-card>

    <!-- 证书详情抽屉 -->
    <el-drawer v-model="detailVisible" :title="$t('certAsset.certDetail')" size="640px" direction="rtl">
      <el-descriptions v-if="currentCert" :column="2" border>
        <el-descriptions-item :label="$t('certAsset.host')">{{ currentCert.host }}</el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.port')">{{ currentCert.port }}</el-descriptions-item>
        <el-descriptions-item :span="2" :label="$t('certAsset.authority')">{{ currentCert.authority }}</el-descriptions-item>
        <el-descriptions-item :span="2" :label="$t('certAsset.subjectDN')">{{ currentCert.subjectDN }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.subject" :span="2" :label="$t('certAsset.subjectCN')">{{ currentCert.subject.commonName }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.subject?.organization" :span="2" :label="$t('certAsset.subjectOrg')">{{ currentCert.subject.organization }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.subject?.country" :label="$t('certAsset.subjectCountry')">{{ currentCert.subject.country }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.subject?.province" :label="$t('certAsset.subjectProvince')">{{ currentCert.subject.province }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.subject?.locality" :label="$t('certAsset.subjectLocality')">{{ currentCert.subject.locality }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.subject?.email" :label="$t('certAsset.subjectEmail')">{{ currentCert.subject.email }}</el-descriptions-item>
        <el-descriptions-item :span="2" :label="$t('certAsset.issuerDN')">{{ currentCert.issuerDN }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.issuer" :span="2" :label="$t('certAsset.issuerCN')">{{ currentCert.issuer.commonName }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.issuer?.organization" :span="2" :label="$t('certAsset.issuerOrg')">{{ currentCert.issuer.organization }}</el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.serialNumber')">{{ currentCert.serialNumber }}</el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.sigAlg')">{{ currentCert.sigAlg }}</el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.version')">{{ currentCert.version }}</el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.isSelfSigned')">
          <el-tag :type="currentCert.isSelfSigned ? 'warning' : 'success'" size="small">
            {{ currentCert.isSelfSigned ? $t('common.yes') : $t('common.no') }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.notBefore')">{{ formatDate(currentCert.notBefore) }}</el-descriptions-item>
        <el-descriptions-item :label="$t('certAsset.notAfter')">
          <div class="not-after-cell">
            <span>{{ formatDate(currentCert.notAfter) }}</span>
            <el-tag :type="statusTagType(currentCert)" size="small" effect="light" class="status-tag">{{ statusLabel(currentCert) }}</el-tag>
          </div>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentCert.fingerprints?.sha1" :span="2" :label="$t('certAsset.sha1')">
          <span class="fingerprint">{{ currentCert.fingerprints.sha1 }}</span>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentCert.fingerprints?.sha256" :span="2" :label="$t('certAsset.sha256')">
          <span class="fingerprint">{{ currentCert.fingerprints.sha256 }}</span>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentCert.fingerprints?.md5" :span="2" :label="$t('certAsset.md5')">
          <span class="fingerprint">{{ currentCert.fingerprints.md5 }}</span>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentCert.sans?.length" :span="2" :label="$t('certAsset.sans')">
          <div class="sans-list">
            <el-tag v-for="san in currentCert.sans" :key="san" size="small" style="margin: 2px;">{{ san }}</el-tag>
          </div>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentCert.taskId" :label="$t('certAsset.taskId')">{{ currentCert.taskId }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.createTime" :label="$t('common.createTime')">{{ formatDate(currentCert.createTime) }}</el-descriptions-item>
        <el-descriptions-item v-if="currentCert.updateTime" :label="$t('common.updateTime')">{{ formatDate(currentCert.updateTime) }}</el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { getCertList, getCertDetail } from '@/api/cert'

const { t } = useI18n()

// 到期状态阈值（天）
const EXPIRING_DAYS = 30

const query = ref('')
const issuer = ref('')
const validity = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const tableData = ref([])
const detailVisible = ref(false)
const currentCert = ref(null)

// 把证书时间字符串(后端 RFC3339 / 本地格式)解析为 Date，失败返回 null
function parseCertDate(value) {
  if (!value) return null
  const d = new Date(value)
  return isNaN(d.getTime()) ? null : d
}

// 统一日期格式化（YYYY-MM-DD HH:mm），无法解析时原样返回
function formatDate(value) {
  const d = parseCertDate(value)
  if (!d) return value || '-'
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// 到期状态：expired(已过期) / expiring(即将到期) / valid(有效)
function certStatus(row) {
  const d = parseCertDate(row?.notAfter)
  if (!d) return 'unknown'
  const now = Date.now()
  if (d.getTime() <= now) return 'expired'
  if (d.getTime() - now <= EXPIRING_DAYS * 24 * 3600 * 1000) return 'expiring'
  return 'valid'
}

const statusLabel = (row) => t(`certAsset.status_${certStatus(row)}`)
const statusTagType = (row) => {
  switch (certStatus(row)) {
    case 'expired': return 'danger'
    case 'expiring': return 'warning'
    case 'valid': return 'success'
    default: return 'info'
  }
}

async function loadData() {
  loading.value = true
  try {
    const res = await getCertList({
      query: query.value,
      issuer: issuer.value,
      validity: validity.value,
      page: page.value,
      pageSize: pageSize.value,
      sort: '-notAfter'
    })
    if (res.code === 0) {
      tableData.value = res.list || []
      total.value = res.total || 0
    }
  } catch (e) {
    ElMessage.error(t('certAsset.loadFailed'))
  } finally {
    loading.value = false
  }
}

function handleQuery() {
  page.value = 1
  loadData()
}

function handleReset() {
  query.value = ''
  issuer.value = ''
  validity.value = ''
  handleQuery()
}

async function showDetail(row) {
  try {
    const res = await getCertDetail({ id: row.id })
    if (res.code === 0 && res.data) {
      currentCert.value = res.data
      detailVisible.value = true
    } else {
      ElMessage.warning(res.msg || t('certAsset.detailFailed'))
    }
  } catch (e) {
    ElMessage.error(t('certAsset.detailFailed'))
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.cert-asset-page {
  padding: 24px;
  background: hsl(var(--background));
  min-height: 100vh;
}

.page-header {
  margin-bottom: 24px;

  h1 {
    font-size: 28px;
    font-weight: 600;
    color: hsl(var(--foreground));
    margin: 0 0 8px 0;
  }

  .description {
    color: hsl(var(--muted-foreground));
    font-size: 14px;
    margin: 0;
  }
}

.filter-form {
  margin-bottom: 16px;
}

.fingerprint {
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}

.sans-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.not-after-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-tag {
  align-self: flex-start;
}
</style>
