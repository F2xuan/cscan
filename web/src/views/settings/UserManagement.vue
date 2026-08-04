<template>
  <div class="user-management-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('navigation.userManagement') }}</span>
          <el-button type="primary" size="small" @click="showUserDialog()">
            <el-icon><Plus /></el-icon>{{ $t('user.newUser') }}
          </el-button>
        </div>
      </template>
      <div class="user-toolbar">
        <el-input
          v-model="searchKeyword"
          :placeholder="$t('user.searchPlaceholder', '搜索用户名')"
          clearable
          size="small"
          style="width: 200px"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="filterRole" :placeholder="$t('user.role')" clearable size="small" style="width: 120px">
          <el-option label="Admin" value="admin" />
          <el-option label="User" value="user" />
        </el-select>
      </div>
      <el-table :data="pagedUserList" v-loading="userLoading" stripe max-height="500">
        <el-table-column :label="$t('user.avatar')" width="80">
          <template #default="{ row }">
            <el-avatar :size="40" :src="row.avatar || DEFAULT_AVATAR" />
          </template>
        </el-table-column>
        <el-table-column prop="username" :label="$t('user.userName')" min-width="150" />
        <el-table-column prop="role" :label="$t('user.role')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' || row.role === 'superadmin' ? 'danger' : 'info'">
              {{ row.role === 'admin' || row.role === 'superadmin' ? $t('user.admin') : $t('user.user') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" :label="$t('common.status')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'enable' ? 'success' : 'danger'">
              {{ row.status === 'enable' ? $t('common.enabled') : $t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="200" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showUserDialog(row)">{{ $t('common.edit') }}</el-button>
            <el-button type="warning" link size="small" @click="showResetPasswordDialog(row)">{{ $t('user.resetPassword') }}</el-button>
            <el-button type="danger" link size="small" @click="handleDeleteUser(row)">{{ $t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="user-pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="filteredUserList.length"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          small
        />
      </div>
    </el-card>

    <!-- 用户对话框 -->
    <el-dialog v-model="userDialogVisible" :title="userForm.id ? $t('user.editUser') : $t('user.newUser')" width="500px">
      <el-form ref="userFormRef" :model="userForm" :rules="userRules" label-width="80px">
        <el-form-item :label="$t('user.avatar')">
          <div class="avatar-updater">
            <el-avatar :size="80" :src="avatarPreview" />
            <el-upload
              :show-file-list="false"
              :before-upload="handleAvatarBeforeUpload"
              :http-request="handleAvatarUpload"
              :accept="'image/png,image/jpeg,image/gif,image/webp'"
              class="avatar-upload-btn"
            >
              <el-button size="small">{{ $t('user.changeAvatar') }}</el-button>
              <template #tip>
                <div class="avatar-tip">{{ $t('user.avatarTip', 'JPG/PNG/GIF/WebP, 最大 2MB') }}</div>
              </template>
            </el-upload>
            <el-button v-if="userForm.avatar" text type="danger" size="small" @click="clearAvatar">
              {{ $t('common.delete') }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item :label="$t('user.userName')" prop="username">
          <el-input v-model="userForm.username" :placeholder="$t('user.pleaseEnterUsername')" />
        </el-form-item>
        <el-form-item v-if="!userForm.id" :label="$t('user.password')" prop="password">
          <el-input v-model="userForm.password" type="password" :placeholder="$t('user.pleaseEnterPassword')" />
        </el-form-item>
        <el-form-item :label="$t('user.role')" prop="role">
          <el-select v-model="userForm.role" :placeholder="$t('user.pleaseSelectRole')" :disabled="isAdminRow">
            <el-option :label="$t('user.admin')" value="admin" />
            <el-option :label="$t('user.user')" value="user" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('common.status')" prop="status">
          <el-select v-model="userForm.status" :placeholder="$t('user.pleaseSelectStatus')" :disabled="isAdminRow">
            <el-option :label="$t('common.enabled')" value="enable" />
            <el-option :label="$t('common.disabled')" value="disable" />
          </el-select>
          <div v-if="isAdminRow" class="form-tip">{{ $t('user.adminStatusLockTip', 'admin 账号状态与角色受保护，不允许修改') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="userDialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="userSubmitting" @click="handleUserSubmit">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码对话框 -->
    <el-dialog v-model="resetPasswordVisible" :title="$t('user.resetPassword')" width="400px">
      <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-width="80px">
        <el-form-item :label="$t('user.oldPassword')" prop="oldPassword">
          <el-input v-model="resetForm.oldPassword" type="password" :placeholder="$t('user.pleaseEnterOldPassword')" show-password />
        </el-form-item>
        <el-form-item :label="$t('user.newPassword')" prop="newPassword">
          <el-input v-model="resetForm.newPassword" type="password" :placeholder="$t('user.pleaseEnterNewPassword')" show-password />
        </el-form-item>
        <el-form-item :label="$t('user.confirmPassword')" prop="confirmPassword">
          <el-input v-model="resetForm.confirmPassword" type="password" :placeholder="$t('user.pleaseConfirmPassword')" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPasswordVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="resetting" @click="handleResetPassword">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { getUserList, createUser, updateUser, deleteUser, resetUserPassword, uploadUserAvatar } from '@/api/auth'
import { useUserStore, DEFAULT_AVATAR } from '@/stores/user'

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()

const userLoading = ref(false)
const userList = ref([])

// 搜索与分页
const searchKeyword = ref('')
const filterRole = ref('')
const currentPage = ref(1)
const pageSize = ref(10)

const filteredUserList = computed(() => {
  let list = userList.value
  if (searchKeyword.value) {
    const kw = searchKeyword.value.toLowerCase()
    list = list.filter(u => u.username?.toLowerCase().includes(kw))
  }
  if (filterRole.value) {
    if (filterRole.value === 'admin') {
      list = list.filter(u => u.role === 'admin' || u.role === 'superadmin')
    } else {
      list = list.filter(u => u.role !== 'admin' && u.role !== 'superadmin')
    }
  }
  return list
})

const pagedUserList = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredUserList.value.slice(start, start + pageSize.value)
})
const userDialogVisible = ref(false)
const userSubmitting = ref(false)
const userFormRef = ref()
const userForm = ref({ id: '', username: '', password: '', role: 'user', status: 'enable', avatar: '' })
const isAdminRow = computed(() => userForm.value.username === 'admin')
const avatarPreview = computed(() => userForm.value.avatar || DEFAULT_AVATAR)
const avatarUploading = ref(false)

// 密码强度校验器
function validatePasswordStrength(rule, value, callback) {
  if (!value) return callback()
  if (value.length < 8) return callback(new Error(t('user.passwordMinLength')))
  if (!/[A-Z]/.test(value)) return callback(new Error(t('user.passwordNeedUpper')))
  if (!/[a-z]/.test(value)) return callback(new Error(t('user.passwordNeedLower')))
  if (!/[0-9]/.test(value)) return callback(new Error(t('user.passwordNeedDigit')))
  callback()
}

const userRules = computed(() => ({
  username: [{ required: true, message: t('user.pleaseEnterUsername'), trigger: 'blur' }],
  password: [
    { required: true, message: t('user.pleaseEnterPassword'), trigger: 'blur' },
    { validator: validatePasswordStrength, trigger: 'blur' }
  ],
  role: [{ required: true, message: t('user.pleaseSelectRole'), trigger: 'change' }],
  status: [{ required: true, message: t('user.pleaseSelectStatus'), trigger: 'change' }]
}))

// 重置密码相关
const resetPasswordVisible = ref(false)
const resetting = ref(false)
const resetFormRef = ref()
const resetForm = ref({ id: '', oldPassword: '', newPassword: '', confirmPassword: '' })
const resetRules = computed(() => ({
  oldPassword: [{ required: true, message: t('user.pleaseEnterOldPassword'), trigger: 'blur' }],
  newPassword: [
    { required: true, message: t('user.pleaseEnterNewPassword'), trigger: 'blur' },
    { validator: validatePasswordStrength, trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: t('user.pleaseConfirmPassword'), trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== resetForm.value.newPassword) {
          callback(new Error(t('user.passwordMismatch')))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}))

onMounted(() => loadUserList())

async function loadUserList() {
  userLoading.value = true
  try {
    const res = await getUserList({ page: 1, pageSize: 100 })
    if (res.code === 0) userList.value = res.list || []
  } finally {
    userLoading.value = false
  }
}

function showUserDialog(row = null) {
  if (row) {
    userForm.value = { ...row, password: '' }
  } else {
    userForm.value = { id: '', username: '', password: '', role: 'user', status: 'enable', avatar: '' }
  }
  userDialogVisible.value = true
}

function handleAvatarBeforeUpload(file) {
  const allowed = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']
  if (!allowed.includes(file.type)) {
    ElMessage.error(t('user.avatarFormatError', '仅支持 JPG/PNG/GIF/WebP 格式'))
    return false
  }
  if (file.size > 2 * 1024 * 1024) {
    ElMessage.error(t('user.avatarTooLarge', '头像文件不能超过 2MB'))
    return false
  }
  return true
}

async function handleAvatarUpload({ file }) {
  if (!file) return
  avatarUploading.value = true
  try {
    const res = await uploadUserAvatar(file)
    if (res.code === 0 && res.avatar) {
      userForm.value.avatar = res.avatar
      ElMessage.success(t('user.avatarUploadSuccess', '头像上传成功'))
    } else {
      ElMessage.error(res.msg || t('user.avatarUploadFailed', '头像上传失败'))
    }
  } finally {
    avatarUploading.value = false
  }
}

function clearAvatar() {
  userForm.value.avatar = ''
}

async function handleUserSubmit() {
  if (!userFormRef.value) return
  try {
    await userFormRef.value.validate()
    userSubmitting.value = true
    const api = userForm.value.id ? updateUser : createUser
    const res = await api(userForm.value)
    if (res.code === 0) {
      ElMessage.success(res.msg || t('common.operationSuccess'))
      userDialogVisible.value = false
      loadUserList()
      // 若修改的是当前登录用户,实时同步头像到顶栏 store
      if (userForm.value.username === userStore.username) {
        userStore.setAvatar(userForm.value.avatar || '')
      }
    } else {
      ElMessage.error(res.msg || t('common.operationFailed'))
    }
  } catch (error) {
    console.error('表单验证失败:', error)
  } finally {
    userSubmitting.value = false
  }
}

async function handleDeleteUser(row) {
  try {
    await ElMessageBox.confirm(t('user.confirmDeleteUser'), t('common.tip'), { type: 'warning' })
    const res = await deleteUser({ id: row.id })
    if (res.code === 0) {
      ElMessage.success(res.msg || t('common.deleteSuccess'))
      loadUserList()
    } else {
      ElMessage.error(res.msg || t('common.operationFailed'))
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除用户失败:', error)
    }
  }
}

function showResetPasswordDialog(row) {
  resetForm.value = { id: row.id, oldPassword: '', newPassword: '', confirmPassword: '' }
  resetPasswordVisible.value = true
}

async function handleResetPassword() {
  if (!resetFormRef.value) return
  try {
    await resetFormRef.value.validate()
    resetting.value = true
    const res = await resetUserPassword({
      id: resetForm.value.id,
      oldPassword: resetForm.value.oldPassword,
      newPassword: resetForm.value.newPassword
    })
    if (res.code === 0) {
      ElMessage.success(res.msg || t('user.passwordResetSuccess'))
      resetPasswordVisible.value = false
      // 如果修改的是自己的密码，立即登出避免旧 token 触发大量 401
      if (resetForm.value.id === userStore.userId) {
        userStore.logout()
        router.push('/login')
      }
    } else {
      ElMessage.error(res.msg || t('user.passwordResetFailed'))
    }
  } catch (error) {
    console.error('表单验证失败:', error)
  } finally {
    resetting.value = false
  }
}
</script>

<style scoped>
.user-management-page .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 16px;
  font-weight: 500;
}

.user-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.user-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

/* 用户头像上传 */
.avatar-updater {
  display: flex;
  align-items: center;
  gap: 16px;
}

.avatar-updater .avatar-upload-btn {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.avatar-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>
