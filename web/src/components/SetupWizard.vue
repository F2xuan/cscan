<template>
  <div class="setup-container">
    <!-- Theme & language controls -->
    <div class="controls">
      <div class="control-btn" @click="localeStore.toggleLocale">
        <el-icon><Position /></el-icon>
        <span>{{ localeStore.currentLocale === 'zh-CN' ? 'EN' : '中' }}</span>
      </div>
      <div class="control-btn" @click="themeStore.toggleTheme">
        <el-icon v-if="themeStore.isDark"><Sunny /></el-icon>
        <el-icon v-else><Moon /></el-icon>
      </div>
    </div>

    <div class="setup-wrapper">
      <!-- Logo & title -->
      <div class="setup-brand">
        <img
          v-if="brandingStore.logoSrc"
          :src="brandingStore.logoSrc"
          :alt="brandingStore.displayTitle"
          class="brand-logo"
        />
        <h1 class="brand-title">
          {{ $t('setup.welcomeTitle') }} {{ brandingStore.displayTitle }}
        </h1>
        <p class="brand-desc">{{ $t('setup.welcomeText') }}</p>
      </div>

      <!-- Wizard card -->
      <div class="wizard-card">
        <!-- Card header -->
        <div class="wizard-header">
          <h2 class="wizard-title">{{ $t('setup.wizardTitle') }}</h2>
          <p class="wizard-desc">{{ $t('setup.wizardDesc') }}</p>
        </div>

        <!-- Step indicator -->
        <div class="step-indicator">
          <div
            v-for="(step, index) in steps"
            :key="step.titleKey"
            class="step-item"
            :class="{
              active: currentStep === index,
              completed: currentStep > index,
            }"
          >
            <div class="step-top">
              <div
                class="step-connector"
                :class="{ filled: currentStep >= index, hidden: index === 0 }"
              />
              <div class="step-circle">
                <el-icon v-if="currentStep > index" :size="16"><Check /></el-icon>
                <span v-else>{{ index + 1 }}</span>
              </div>
              <div
                class="step-connector"
                :class="{ filled: currentStep > index, hidden: index === steps.length - 1 }"
              />
            </div>
            <div class="step-text">
              <span class="step-title">{{ $t(step.titleKey) }}</span>
              <span class="step-desc">{{ $t(step.descKey) }}</span>
            </div>
          </div>
        </div>

        <!-- Step content -->
        <div class="wizard-body">
          <!-- Step 0: Welcome -->
          <div v-if="currentStep === 0" class="step-content welcome-step">
            <div class="feature-list">
              <div class="feature-item">
                <el-icon :size="28" class="feature-icon"><Monitor /></el-icon>
                <div class="feature-text">
                  <span class="feature-title">{{ $t('setup.featureScan') }}</span>
                </div>
              </div>
              <div class="feature-item">
                <el-icon :size="28" class="feature-icon"><Search /></el-icon>
                <div class="feature-text">
                  <span class="feature-title">{{ $t('setup.featureAsset') }}</span>
                </div>
              </div>
              <div class="feature-item">
                <el-icon :size="28" class="feature-icon"><Bell /></el-icon>
                <div class="feature-text">
                  <span class="feature-title">{{ $t('setup.featureMonitor') }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Step 1: Admin account -->
          <div v-else-if="currentStep === 1" class="step-content admin-step">
            <div class="step-section-title">{{ $t('setup.adminSetupTitle') }}</div>
            <p class="step-section-desc">{{ $t('setup.adminSetupDesc') }}</p>
            <el-form
              ref="formRef"
              :model="form"
              :rules="rules"
              label-position="top"
              class="admin-form"
            >
              <el-form-item prop="username" :label="$t('auth.username')">
                <el-input
                  v-model="form.username"
                  :placeholder="$t('auth.pleaseEnterUsername')"
                  prefix-icon="User"
                  size="large"
                />
              </el-form-item>
              <el-form-item prop="password" :label="$t('auth.password')">
                <el-input
                  v-model="form.password"
                  type="password"
                  :placeholder="$t('auth.pleaseEnterPassword')"
                  prefix-icon="Lock"
                  size="large"
                  show-password
                />
                <div v-if="form.password" class="password-strength">
                  <div class="strength-bars">
                    <div
                      v-for="i in 4"
                      :key="i"
                      class="strength-bar"
                      :class="{ filled: i <= passwordStrengthLevel }"
                      :style="{ background: i <= passwordStrengthLevel ? strengthColor : '' }"
                    />
                  </div>
                  <span class="strength-label" :style="{ color: strengthColor }">
                    {{ strengthLabel }}
                  </span>
                </div>
              </el-form-item>
              <el-form-item prop="confirmPassword" :label="$t('auth.confirmPassword')">
                <el-input
                  v-model="form.confirmPassword"
                  type="password"
                  :placeholder="$t('auth.pleaseConfirmPassword')"
                  prefix-icon="Lock"
                  size="large"
                  show-password
                  @keyup.enter="handleSubmit"
                />
              </el-form-item>
            </el-form>
          </div>

          <!-- Step 2: Complete -->
          <div v-else class="step-content complete-step">
            <div class="complete-icon">
              <el-icon :size="64"><CircleCheckFilled /></el-icon>
            </div>
            <h3 class="complete-title">{{ $t('setup.completeSuccess') }}</h3>
            <p class="complete-desc">{{ $t('setup.completeDesc') }}</p>
            <div v-if="redirecting" class="redirect-hint">
              <el-icon class="rotating"><Loading /></el-icon>
              <span>{{ $t('setup.redirecting') }}</span>
            </div>
          </div>
        </div>

        <!-- Footer navigation -->
        <div class="wizard-footer">
          <el-button
            v-if="currentStep > 0 && currentStep < steps.length - 1"
            size="large"
            plain
            @click="handlePrev"
          >
            {{ $t('setup.btnPrev') }}
          </el-button>
          <el-button
            v-if="currentStep < steps.length - 2"
            type="primary"
            size="large"
            @click="handleNext"
          >
            {{ $t('setup.btnNext') }}
          </el-button>
          <el-button
            v-if="currentStep === steps.length - 2"
            type="primary"
            size="large"
            :loading="submitting"
            @click="handleSubmit"
          >
            {{ $t('setup.btnSubmit') }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { useLocaleStore } from '@/stores/locale'
import { useBrandingStore } from '@/stores/branding'
import { Sunny, Moon, Position, Check, Monitor, Search, Bell, CircleCheckFilled, Loading } from '@element-plus/icons-vue'
import { register as apiRegister } from '@/api/auth'

const router = useRouter()
const { t } = useI18n()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const brandingStore = useBrandingStore()

const emit = defineEmits(['completed'])

const currentStep = ref(0)
const submitting = ref(false)
const redirecting = ref(false)
const formRef = ref()

const steps = [
  { titleKey: 'setup.stepWelcome', descKey: 'setup.stepWelcomeDesc' },
  { titleKey: 'setup.stepAdmin', descKey: 'setup.stepAdminDesc' },
  { titleKey: 'setup.stepComplete', descKey: 'setup.stepCompleteDesc' },
]

const form = reactive({
  username: '',
  password: '',
  confirmPassword: '',
})

const validateConfirmPassword = (rule, value, callback) => {
  if (!value) {
    callback(new Error(t('auth.pleaseConfirmPassword')))
  } else if (value !== form.password) {
    callback(new Error(t('auth.passwordMismatch')))
  } else {
    callback()
  }
}

const rules = computed(() => ({
  username: [{ required: true, message: t('auth.pleaseEnterUsername'), trigger: 'blur' }],
  password: [
    { required: true, message: t('auth.pleaseEnterPassword'), trigger: 'blur' },
    { min: 8, message: t('auth.passwordMinLength'), trigger: 'blur' },
    { pattern: /[A-Z]/, message: t('auth.passwordNeedUpper'), trigger: 'blur' },
    { pattern: /[a-z]/, message: t('auth.passwordNeedLower'), trigger: 'blur' },
    { pattern: /[0-9]/, message: t('auth.passwordNeedDigit'), trigger: 'blur' },
  ],
  confirmPassword: [{ validator: validateConfirmPassword, trigger: 'blur' }],
}))

// Password strength evaluation (0-4)
const passwordStrengthLevel = computed(() => {
  const pwd = form.password
  if (!pwd) return 0
  let level = 0
  if (pwd.length >= 8) level++
  if (/[A-Z]/.test(pwd) && /[a-z]/.test(pwd)) level++
  if (/[0-9]/.test(pwd)) level++
  if (/[^A-Za-z0-9]/.test(pwd) || pwd.length >= 16) level++
  return level
})

const strengthColor = computed(() => {
  const level = passwordStrengthLevel.value
  if (level <= 1) return 'var(--el-color-danger)'
  if (level <= 2) return 'var(--el-color-warning)'
  return 'var(--el-color-success)'
})

const strengthLabel = computed(() => {
  const level = passwordStrengthLevel.value
  if (level <= 1) return t('setup.passwordWeak')
  if (level <= 2) return t('setup.passwordMedium')
  return t('setup.passwordStrong')
})

function handleNext() {
  currentStep.value = Math.min(currentStep.value + 1, steps.length - 1)
}

function handlePrev() {
  currentStep.value = Math.max(currentStep.value - 1, 0)
}

async function handleSubmit() {
  try {
    await formRef.value.validate()
  } catch (e) {
    return
  }
  submitting.value = true
  try {
    const res = await apiRegister({
      username: form.username,
      password: form.password,
    })
    if (res.code === 0) {
      ElMessage.success(t('auth.registerSuccess'))
      currentStep.value = steps.length - 1
      redirecting.value = true
      emit('completed')
      setTimeout(() => {
        router.push('/login')
      }, 2000)
    } else {
      ElMessage.error(res.msg || t('auth.registerFailed'))
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: hsl(var(--background));
  position: relative;
  padding: 40px 16px;
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
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  transition: all 0.3s;
}

.control-btn:hover {
  transform: scale(1.1);
  border-color: hsl(var(--primary));
  color: hsl(var(--primary));
}

.control-btn .el-icon {
  font-size: 18px;
}

.control-btn span {
  font-size: 12px;
  font-weight: 600;
}

.setup-wrapper {
  width: 100%;
  max-width: 560px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 28px;
}

/* Brand section */
.setup-brand {
  text-align: center;
}

.brand-logo {
  width: 64px;
  height: 64px;
  border-radius: 14px;
  object-fit: cover;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
  margin: 0 auto 16px;
  display: block;
}

.brand-title {
  font-size: 26px;
  font-weight: 600;
  color: hsl(var(--foreground));
  margin: 0 0 8px;
  letter-spacing: 1px;
}

.brand-desc {
  font-size: 14px;
  color: hsl(var(--muted-foreground));
  margin: 0;
  line-height: 1.6;
  max-width: 440px;
}

/* Wizard card */
.wizard-card {
  width: 100%;
  background: hsl(var(--card));
  border-radius: 16px;
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.08);
  border: 1px solid hsl(var(--border));
  overflow: hidden;
  transition: all 0.3s;
}

.wizard-header {
  padding: 24px 32px 0;
  text-align: center;
}

.wizard-title {
  font-size: 20px;
  font-weight: 600;
  color: hsl(var(--foreground));
  margin: 0 0 6px;
}

.wizard-desc {
  font-size: 13px;
  color: hsl(var(--muted-foreground));
  margin: 0;
}

/* Step indicator */
.step-indicator {
  display: flex;
  align-items: flex-start;
  padding: 24px 32px 20px;
}

.step-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
}

.step-circle {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 600;
  border: 2px solid hsl(var(--border));
  background: hsl(var(--card));
  color: hsl(var(--muted-foreground));
  transition: all 0.3s;
  flex-shrink: 0;
  z-index: 1;
}

.step-item.active .step-circle {
  border-color: hsl(var(--primary));
  background: hsl(var(--primary));
  color: hsl(var(--primary-foreground));
  box-shadow: 0 0 0 4px hsl(var(--primary) / 0.1);
}

.step-item.completed .step-circle {
  border-color: hsl(var(--primary));
  background: hsl(var(--primary));
  color: hsl(var(--primary-foreground));
}

.step-text {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-top: 8px;
  text-align: center;
}

.step-title {
  font-size: 13px;
  font-weight: 600;
  color: hsl(var(--muted-foreground));
  transition: color 0.3s;
  white-space: nowrap;
}

.step-desc {
  font-size: 11px;
  color: hsl(var(--muted-foreground) / 0.7);
  margin-top: 2px;
  white-space: nowrap;
}

.step-item.active .step-title {
  color: hsl(var(--foreground));
}

.step-item.completed .step-title {
  color: hsl(var(--foreground));
}

.step-top {
  display: flex;
  align-items: center;
  width: 100%;
}

.step-connector {
  flex: 1;
  height: 2px;
  background: hsl(var(--border));
  border-radius: 1px;
  transition: background 0.3s;
  min-width: 12px;
}

.step-connector.hidden {
  visibility: hidden;
}

.step-connector.filled {
  background: hsl(var(--primary));
}

/* Wizard body */
.wizard-body {
  min-height: 240px;
  padding: 8px 32px 24px;
}

.step-content {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Welcome step */
.welcome-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 12px 0;
}

.feature-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}

.feature-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 18px;
  border-radius: 12px;
  background: hsl(var(--background) / 0.5);
  border: 1px solid hsl(var(--border) / 0.5);
  transition: all 0.2s;
}

.feature-item:hover {
  border-color: hsl(var(--primary) / 0.4);
  background: hsl(var(--primary) / 0.04);
}

.feature-icon {
  color: hsl(var(--primary));
  flex-shrink: 0;
}

.feature-title {
  font-size: 14px;
  font-weight: 500;
  color: hsl(var(--foreground));
}

/* Admin step */
.admin-step {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.step-section-title {
  font-size: 16px;
  font-weight: 600;
  color: hsl(var(--foreground));
  margin-bottom: 4px;
}

.step-section-desc {
  font-size: 13px;
  color: hsl(var(--muted-foreground));
  margin: 0 0 16px;
}

.admin-form {
  width: 100%;
}

.password-strength {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  width: 100%;
}

.strength-bars {
  display: flex;
  gap: 4px;
  flex: 1;
}

.strength-bar {
  height: 4px;
  flex: 1;
  border-radius: 2px;
  background: hsl(var(--border));
  transition: background 0.3s;
}

.strength-label {
  font-size: 12px;
  font-weight: 500;
  flex-shrink: 0;
}

/* Complete step */
.complete-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 32px 0;
  text-align: center;
}

.complete-icon {
  color: var(--el-color-success);
  animation: popIn 0.4s ease;
}

@keyframes popIn {
  0% { transform: scale(0); opacity: 0; }
  60% { transform: scale(1.15); }
  100% { transform: scale(1); opacity: 1; }
}

.complete-title {
  font-size: 18px;
  font-weight: 600;
  color: hsl(var(--foreground));
  margin: 4px 0 0;
}

.complete-desc {
  font-size: 14px;
  color: hsl(var(--muted-foreground));
  margin: 0;
}

.redirect-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: hsl(var(--muted-foreground));
  margin-top: 8px;
}

.rotating {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Footer */
.wizard-footer {
  padding: 16px 32px 24px;
  display: flex;
  justify-content: center;
  gap: 12px;
  border-top: 1px solid hsl(var(--border));
}

.wizard-footer .el-button {
  min-width: 120px;
  height: 42px;
  border-radius: 10px;
  font-weight: 500;
}

/* Form input styling */
:deep(.el-input__wrapper) {
  background: hsl(var(--background));
  border: 1px solid hsl(var(--border));
  box-shadow: none;
  border-radius: 10px;
  transition: border-color 0.2s;
}

:deep(.el-input__wrapper:hover) {
  border-color: hsl(var(--border));
}

:deep(.el-input__wrapper.is-focus) {
  border-color: hsl(var(--primary));
  box-shadow: 0 0 0 3px hsl(var(--primary) / 0.08);
}

:deep(.el-input__inner) {
  color: hsl(var(--foreground));
}

:deep(.el-input__inner::placeholder) {
  color: hsl(var(--muted-foreground));
}

:deep(.el-input__prefix) {
  color: hsl(var(--muted-foreground));
}

:deep(.el-form-item__label) {
  font-size: 13px;
  font-weight: 500;
  color: hsl(var(--foreground));
  padding-bottom: 4px;
}

/* Responsive */
@media (max-width: 640px) {
  .setup-wrapper {
    max-width: 100%;
  }

  .wizard-card {
    border-radius: 12px;
  }

  .wizard-header,
  .step-indicator,
  .wizard-body,
  .wizard-footer {
    padding-left: 20px;
    padding-right: 20px;
  }

  .step-desc {
    display: none;
  }

  .brand-desc {
    font-size: 13px;
  }
}
</style>
