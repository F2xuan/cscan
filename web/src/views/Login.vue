<template>
  <div class="login-container">
    <!-- 主题和语言切换按钮 -->
    <div class="controls">
      <!-- 语言切换 -->
      <div class="control-btn" @click="localeStore.toggleLocale">
        <el-icon><Position /></el-icon>
        <span>{{ localeStore.currentLocale === 'zh-CN' ? 'EN' : '中' }}</span>
      </div>
      <!-- 主题切换 -->
      <div class="control-btn" @click="themeStore.toggleTheme">
        <el-icon v-if="themeStore.isDark"><Sunny /></el-icon>
        <el-icon v-else><Moon /></el-icon>
      </div>
    </div>
    
    <div :class="['login-box', `style-${themeStore.themeStyle}`]">
      <div class="login-header">
        <h1>{{ brandingStore.displayTitle }}</h1>
        <p>{{ $t('auth.loginTitle') }}</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" class="login-form">
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            :placeholder="$t('auth.username')"
            prefix-icon="User"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            :placeholder="$t('auth.password')"
            prefix-icon="Lock"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="login-btn"
            @click="handleLogin"
          >
            {{ $t('auth.login') }}
          </el-button>
        </el-form-item>
      </el-form>
    </div>

  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '@/stores/user'
import { useThemeStore } from '@/stores/theme'
import { useLocaleStore } from '@/stores/locale'
import { useBrandingStore } from '@/stores/branding'
import { Sunny, Moon, Position } from '@element-plus/icons-vue'

const router = useRouter()
const { t } = useI18n()
const userStore = useUserStore()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const brandingStore = useBrandingStore()
const formRef = ref()
const loading = ref(false)

const form = reactive({
  username: '',
  password: ''
})

const rules = computed(() => ({
  username: [{ required: true, message: t('auth.pleaseEnterUsername'), trigger: 'blur' }],
  password: [{ required: true, message: t('auth.pleaseEnterPassword'), trigger: 'blur' }]
}))

async function handleLogin() {
  await formRef.value.validate()
  loading.value = true
  try {
    const res = await userStore.login(form)
    if (res.code === 0) {
      // 首次登录需要修改密码时，先正常进入系统（让浏览器正确保存账号密码），
      // 再由主框架在进入系统后弹出修改密码提示，避免在登录页中断导致浏览器保存格式异常。
      if (res.needChangePwd) {
        sessionStorage.setItem('pendingResetPassword', '1')
      }
      ElMessage.success(t('auth.loginSuccess'))
      router.push('/dashboard')
    } else {
      ElMessage.error(res.msg || t('auth.loginFailed'))
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: hsl(var(--background));
  position: relative;
  transition: background 0.3s;
}

.controls {
  position: absolute;
  top: 20px;
  right: 20px;
  display: flex;
  gap: 12px;
}

.control-btn {
  cursor: pointer;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  color: hsl(var(--muted-foreground));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: all 0.3s;
  
  &:hover {
    transform: scale(1.1);
    border-color: hsl(var(--primary));
    color: hsl(var(--primary));
  }
  
  .el-icon {
    font-size: 18px;
  }
  
  span {
    font-size: 12px;
    font-weight: 600;
  }
}

.login-box {
  width: 400px;
  padding: 40px;
  background: hsl(var(--card));
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  border: 1px solid hsl(var(--border));
  transition: all 0.3s;
}

.login-header {
  text-align: center;
  margin-bottom: 30px;

  h1 {
    font-size: 32px;
    color: hsl(var(--foreground));
    margin: 0 0 10px;
    letter-spacing: 4px;
    font-weight: 600;
  }

  p {
    color: hsl(var(--muted-foreground));
    margin: 0;
    font-size: 14px;
  }
}

.login-form {
  :deep(.el-input__wrapper) {
    background: hsl(var(--background));
    border: 1px solid hsl(var(--border));
    box-shadow: none;
    border-radius: 8px;
    
    &:hover {
      border-color: hsl(var(--border));
    }
    
    &.is-focus {
      border-color: hsl(var(--primary));
    }
  }
  
  :deep(.el-input__inner) {
    color: hsl(var(--foreground));
    
    &::placeholder {
      color: hsl(var(--muted-foreground));
    }
  }
  
  :deep(.el-input__prefix) {
    color: hsl(var(--muted-foreground));
  }

  .login-btn {
    width: 100%;
    height: 44px;
    background: hsl(var(--primary));
    color: hsl(var(--primary-foreground));
    border: none;
    border-radius: 8px;
    font-size: 16px;
    font-weight: 500;
    letter-spacing: 2px;
    
    &:hover {
      background: hsl(var(--primary) / 0.9);
    }
  }
}
</style>

