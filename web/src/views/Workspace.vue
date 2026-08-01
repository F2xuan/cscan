<template>
  <div class="workspace-page">
    <el-card class="action-card">
      <el-button type="primary" @click="showDialog()">
        <el-icon><Plus /></el-icon>{{ $t('workspace.newWorkspace') }}
      </el-button>
    </el-card>

    <el-card>
      <el-table :data="tableData" v-loading="loading" stripe max-height="500">
        <el-table-column prop="name" :label="$t('common.name')" min-width="150" />
        <el-table-column prop="description" :label="$t('common.description')" min-width="250" />
        <el-table-column prop="status" :label="$t('common.status')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'enable' ? 'success' : 'danger'">
              {{ row.status === 'enable' ? $t('common.enabled') : $t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createTime" :label="$t('common.createTime')" width="160" />
        <el-table-column :label="$t('common.operation')" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showDialog(row)">{{ $t('common.edit') }}</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">{{ $t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? $t('workspace.editWorkspace') : $t('workspace.newWorkspace')" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item :label="$t('common.name')" prop="name">
          <el-input v-model="form.name" :placeholder="$t('workspace.pleaseEnterName')" />
        </el-form-item>
        <el-form-item :label="$t('common.description')">
          <el-input v-model="form.description" type="textarea" :rows="3" :placeholder="$t('workspace.pleaseEnterDescription')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import request from '@/api/request'

const { t } = useI18n()
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const tableData = ref([])
const formRef = ref()

const form = reactive({ id: '', name: '', description: '' })
const rules = { name: [{ required: true, message: () => t('workspace.pleaseEnterName'), trigger: 'blur' }] }

onMounted(() => loadData())

async function loadData() {
  loading.value = true
  try {
    const res = await request.post('/workspace/list', { page: 1, pageSize: 100 })
    if (res.code === 0) tableData.value = res.list || []
  } finally {
    loading.value = false
  }
}

function showDialog(row = null) {
  if (row) {
    Object.assign(form, { id: row.id, name: row.name, description: row.description })
  } else {
    Object.assign(form, { id: '', name: '', description: '' })
  }
  dialogVisible.value = true
}

async function handleSubmit() {
  await formRef.value.validate()
  submitting.value = true
  try {
    const res = await request.post('/workspace/save', form)
    if (res.code === 0) {
      ElMessage.success(form.id ? t('common.updateSuccess') : t('common.createSuccess'))
      dialogVisible.value = false
      loadData()
    } else {
      ElMessage.error(res.msg)
    }
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('workspace.confirmDeleteWorkspace'), t('common.tip'), { type: 'warning' })
  const res = await request.post('/workspace/delete', { id: row.id })
  if (res.code === 0) {
    ElMessage.success(t('common.deleteSuccess'))
    loadData()
  }
}
</script>

<style scoped>
.workspace-page {
  .action-card { margin-bottom: 20px; }
}
</style>

