<template>
  <!-- 首次登录进入系统后提示修改密码弹窗 -->
  <el-dialog
    v-model="visible"
    :title="$t('auth.forceResetTitle', '首次登录请修改密码')"
    width="400px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    destroy-on-close
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="auto" label-position="top">
      <el-form-item :label="$t('auth.newPassword', '新密码')" prop="newPassword">
        <el-input v-model="form.newPassword" type="password" show-password />
      </el-form-item>
      <el-form-item :label="$t('auth.confirmPassword', '确认密码')" prop="confirmPassword">
        <el-input v-model="form.confirmPassword" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button type="primary" :loading="loading" @click="handleSubmit" style="width: 100%;">
        {{ $t('common.confirm', '确认修改') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { firstLoginResetPassword } from '@/api/auth'
import { useUserStore } from '@/stores/user'

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()

const visible = ref(false)
const loading = ref(false)
const formRef = ref()
const form = reactive({
  newPassword: '',
  confirmPassword: ''
})

const rules = computed(() => {
  const validatePass2 = (rule, value, callback) => {
    if (value === '') {
      callback(new Error(t('auth.pleaseConfirmPassword', '请再次输入密码')))
    } else if (value !== form.newPassword) {
      callback(new Error(t('auth.passwordMismatch', '两次输入密码不一致')))
    } else {
      callback()
    }
  }
  return {
    newPassword: [
      { required: true, message: t('auth.pleaseEnterNewPassword', '请输入新密码'), trigger: 'blur' },
      { min: 6, message: t('auth.passwordMinLengths', '密码长度不能小于6位'), trigger: 'blur' }
    ],
    confirmPassword: [
      { required: true, validator: validatePass2, trigger: 'blur' }
    ]
  }
})

onMounted(() => {
  // 进入系统后检查是否需要修改密码（首次登录场景）
  if (sessionStorage.getItem('pendingResetPassword') === '1') {
    sessionStorage.removeItem('pendingResetPassword')
    visible.value = true
  }
})

async function handleSubmit() {
  await formRef.value.validate()
  loading.value = true
  try {
    const res = await firstLoginResetPassword({
      id: userStore.userId,
      newPassword: form.newPassword
    })
    if (res.code === 0) {
      ElMessage.success(t('auth.passwordResetSuccess', '密码修改成功'))
      visible.value = false
      // 立即登出，防止旧 token 继续发请求触发大量 401
      userStore.logout()
      router.push('/login')
    } else {
      ElMessage.error(res.msg || t('auth.passwordResetFailed', '密码修改失败'))
    }
  } catch (error) {
    // 请求失败保留弹窗以便重试
    ElMessage.error(t('auth.passwordResetFailed', '请求失败'))
  } finally {
    loading.value = false
  }
}
</script>
