<template>
  <div class="branding-config-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ $t('navigation.brandingConfig') }}</span>
        </div>
      </template>
      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        <template #title>{{ $t('settings.brandingTip') }}</template>
      </el-alert>

      <el-form label-width="120px" style="max-width: 560px;">
        <el-form-item :label="$t('settings.brandingLogo')">
          <div class="branding-logo-editor">
            <div class="branding-logo-preview">
              <img :src="brandingPreviewSrc" alt="logo" />
            </div>
            <div class="branding-logo-actions">
              <el-upload
                :show-file-list="false"
                :before-upload="handleBrandingLogoBeforeUpload"
                :http-request="handleBrandingLogoSelect"
                :accept="'image/png,image/jpeg,image/gif,image/webp,image/svg+xml'"
              >
                <el-button type="primary" plain size="small">{{ $t('settings.uploadLogo') }}</el-button>
              </el-upload>
              <el-button size="small" @click="resetBrandingLogo" :disabled="!brandingForm.logoData">
                {{ $t('settings.resetLogo') }}
              </el-button>
              <div class="branding-logo-hint">{{ $t('settings.brandingLogoHint') }}</div>
            </div>
          </div>
        </el-form-item>
        <el-form-item :label="$t('settings.brandingTitle')">
          <el-input
            v-model="brandingForm.title"
            :placeholder="$t('settings.brandingTitlePlaceholder')"
            maxlength="32"
            show-word-limit
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="brandingSubmitting" @click="handleBrandingSave">
            {{ $t('common.save') }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useBrandingStore } from '@/stores/branding'

const { t } = useI18n()
const brandingStore = useBrandingStore()

const brandingSubmitting = ref(false)
const brandingForm = reactive({ logoData: '', title: '' })
const brandingPreviewSrc = computed(() => brandingForm.logoData || '/logo.png')

onMounted(() => loadBrandingConfig())

async function loadBrandingConfig() {
  await brandingStore.load()
  brandingForm.logoData = brandingStore.logoData || ''
  brandingForm.title = brandingStore.title || ''
}

function handleBrandingLogoBeforeUpload(file) {
  const maxBytes = 512 * 1024
  if (file.size > maxBytes) {
    ElMessage.error(t('settings.brandingLogoTooLarge'))
    return false
  }
  return true
}

function handleBrandingLogoSelect(options) {
  const file = options.file
  const reader = new FileReader()
  reader.onload = () => {
    brandingForm.logoData = reader.result || ''
  }
  reader.onerror = () => {
    ElMessage.error(t('settings.brandingLogoReadError'))
  }
  reader.readAsDataURL(file)
}

function resetBrandingLogo() {
  brandingForm.logoData = ''
}

async function handleBrandingSave() {
  brandingSubmitting.value = true
  try {
    const res = await brandingStore.save({
      logoData: brandingForm.logoData || '',
      title: (brandingForm.title || '').trim()
    })
    if (res && res.code === 0) {
      ElMessage.success(t('common.saveSuccess'))
    } else {
      ElMessage.error(res?.msg || t('common.saveFailed'))
    }
  } catch (e) {
    ElMessage.error(e?.message || t('common.saveFailed'))
  } finally {
    brandingSubmitting.value = false
  }
}
</script>

<style scoped>
.branding-config-page .card-header {
  font-size: 16px;
  font-weight: 500;
}

/* 品牌 Logo 编辑区 */
.branding-logo-editor {
  display: flex;
  align-items: flex-start;
  gap: 20px;
}

.branding-logo-preview {
  width: 88px;
  height: 88px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-lighter);
  overflow: hidden;
}

.branding-logo-preview img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.branding-logo-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.branding-logo-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
