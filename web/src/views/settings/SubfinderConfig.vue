<template>
  <div class="subfinder-config-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('navigation.subdomainConfig') }}</span>
        </div>
      </template>
      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        <template #title>{{ $t('settings.subfinderTip') }}</template>
      </el-alert>

      <el-table :data="subfinderProviders" v-loading="subfinderLoading" max-height="500" stripe>
        <el-table-column prop="name" :label="$t('settings.dataSource')" width="130" />
        <el-table-column prop="description" :label="$t('common.description')" width="180" show-overflow-tooltip />
        <el-table-column prop="keyFormat" :label="$t('settings.keyFormat')" width="140" />
        <el-table-column :label="$t('settings.apiKeyColumn')" min-width="200">
          <template #default="{ row }">
            <el-input v-model="row.keyInput" :placeholder="row.maskedKey || row.keyFormat" size="small" clearable />
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.status')" width="70">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" size="small" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="140">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="saveSubfinderProvider(row)">{{ $t('common.save') }}</el-button>
            <el-button type="success" link size="small" @click="openApiUrl(row.url)">{{ $t('settings.applyApi') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getSubfinderProviderList, getSubfinderProviderInfo, saveSubfinderProvider as saveSubfinderProviderApi } from '@/api/subfinder'

const { t } = useI18n()
const subfinderLoading = ref(false)
const subfinderProviders = ref([])

onMounted(() => loadSubfinderProviders())

async function loadSubfinderProviders() {
  subfinderLoading.value = true
  try {
    const infoRes = await getSubfinderProviderInfo()
    if (infoRes.code !== 0) {
      ElMessage.error(infoRes.msg || t('common.loadFailed'))
      return
    }

    const listRes = await getSubfinderProviderList()
    const configuredMap = {}
    if (listRes.code === 0 && listRes.list) {
      listRes.list.forEach(item => {
        configuredMap[item.provider] = item
      })
    }

    subfinderProviders.value = infoRes.list.map(info => {
      const configured = configuredMap[info.provider]
      return {
        ...info,
        keyInput: '',
        enabled: configured ? configured.status === 'enable' : false,
        configured: !!configured,
        maskedKey: configured && configured.keys?.length > 0 ? configured.keys[0] : ''
      }
    })
  } finally {
    subfinderLoading.value = false
  }
}

async function saveSubfinderProvider(row) {
  if (!row.keyInput && !row.configured) {
    ElMessage.warning(t('settings.pleaseEnterInput') + t('settings.apiKey'))
    return
  }

  const data = {
    provider: row.provider,
    keys: row.keyInput ? [row.keyInput] : [],
    status: row.enabled ? 'enable' : 'disable',
    description: row.description
  }

  const res = await saveSubfinderProviderApi(data)
  if (res.code === 0) {
    ElMessage.success(t('common.operationSuccess'))
    row.configured = true
    row.keyInput = ''
    await loadSubfinderProviders()
  } else {
    ElMessage.error(res.msg || t('common.operationFailed'))
  }
}

function openApiUrl(url) {
  window.open(url, '_blank')
}
</script>

<style scoped>
.subfinder-config-page .card-header {
  font-size: 16px;
  font-weight: 500;
}
</style>
