<template>
  <div class="organization-management-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('navigation.organizationManagement') }}</span>
          <el-button type="primary" size="small" @click="showOrgDialog()">
            <el-icon><Plus /></el-icon>{{ $t('organization.newOrganization') }}
          </el-button>
        </div>
      </template>
      <el-table :data="orgList" v-loading="orgLoading" stripe max-height="500">
        <el-table-column prop="name" :label="$t('organization.organizationName')" min-width="150" />
        <el-table-column prop="description" :label="$t('common.description')" min-width="250" />
        <el-table-column prop="status" :label="$t('common.status')" width="100">
          <template #default="{ row }">
            <el-switch v-model="row.status" active-value="enable" inactive-value="disable" @change="handleOrgStatusChange(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="createTime" :label="$t('common.createTime')" width="160" />
        <el-table-column :label="$t('common.operation')" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showOrgDialog(row)">{{ $t('common.edit') }}</el-button>
            <el-button type="danger" link size="small" @click="handleDeleteOrg(row)">{{ $t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 组织对话框 -->
    <el-dialog v-model="orgDialogVisible" :title="orgForm.id ? $t('organization.editOrganization') : $t('organization.newOrganization')" width="500px">
      <el-form ref="orgFormRef" :model="orgForm" :rules="orgRules" label-width="80px">
        <el-form-item :label="$t('common.name')" prop="name">
          <el-input v-model="orgForm.name" :placeholder="$t('organization.pleaseEnterOrgName')" />
        </el-form-item>
        <el-form-item :label="$t('common.description')">
          <el-input v-model="orgForm.description" type="textarea" :rows="3" :placeholder="$t('organization.pleaseEnterDescription')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="orgDialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="orgSubmitting" @click="handleOrgSubmit">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import request from '@/api/request'

const { t } = useI18n()

const orgLoading = ref(false)
const orgList = ref([])
const orgDialogVisible = ref(false)
const orgSubmitting = ref(false)
const orgFormRef = ref()
const orgForm = reactive({ id: '', name: '', description: '' })
const orgRules = computed(() => ({
  name: [{ required: true, message: t('organization.pleaseEnterOrgName'), trigger: 'blur' }]
}))

onMounted(() => loadOrgList())

async function loadOrgList() {
  orgLoading.value = true
  try {
    const res = await request.post('/organization/list', { page: 1, pageSize: 100 })
    const data = res.data || res
    if (data.code === 0) orgList.value = data.list || []
  } finally {
    orgLoading.value = false
  }
}

function showOrgDialog(row = null) {
  if (row) {
    Object.assign(orgForm, { id: row.id, name: row.name, description: row.description })
  } else {
    Object.assign(orgForm, { id: '', name: '', description: '' })
  }
  orgDialogVisible.value = true
}

async function handleOrgSubmit() {
  await orgFormRef.value.validate()
  orgSubmitting.value = true
  try {
    const res = await request.post('/organization/save', orgForm)
    const data = res.data || res
    if (data.code === 0) {
      ElMessage.success(orgForm.id ? t('common.updateSuccess') : t('common.createSuccess'))
      orgDialogVisible.value = false
      loadOrgList()
    } else {
      ElMessage.error(data.msg)
    }
  } finally {
    orgSubmitting.value = false
  }
}

async function handleDeleteOrg(row) {
  await ElMessageBox.confirm(t('organization.confirmDeleteOrg'), t('common.tip'), { type: 'warning' })
  const res = await request.post('/organization/delete', { id: row.id })
  const data = res.data || res
  if (data.code === 0) {
    ElMessage.success(t('common.deleteSuccess'))
    loadOrgList()
  }
}

async function handleOrgStatusChange(row) {
  const res = await request.post('/organization/updateStatus', {
    id: row.id,
    status: row.status
  })
  const data = res.data || res
  if (data.code === 0) {
    ElMessage.success(t('common.statusUpdateSuccess'))
  } else {
    row.status = row.status === 'enable' ? 'disable' : 'enable'
    ElMessage.error(data.msg || t('common.statusUpdateFailed'))
  }
}
</script>

<style scoped>
.organization-management-page .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 16px;
  font-weight: 500;
}
</style>
