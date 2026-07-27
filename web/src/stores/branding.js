import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { getBrandingConfig, saveBrandingConfig } from '@/api/branding'

const DEFAULT_LOGO = '/logo.png'
const DEFAULT_TITLE = 'CSCAN'

export const useBrandingStore = defineStore('branding', () => {
  const logoData = ref('')
  const title = ref(DEFAULT_TITLE)
  const loaded = ref(false)

  const logoSrc = computed(() => logoData.value || DEFAULT_LOGO)
  const displayTitle = computed(() => title.value || DEFAULT_TITLE)

  async function load() {
    try {
      const res = await getBrandingConfig()
      if (res.code === 0 && res.config) {
        logoData.value = res.config.logoData || ''
        title.value = res.config.title || DEFAULT_TITLE
      }
    } catch (e) {
      // 静默失败，保留默认值即可
    } finally {
      loaded.value = true
    }
  }

  async function save(payload) {
    const res = await saveBrandingConfig(payload)
    if (res.code === 0 && res.config) {
      logoData.value = res.config.logoData || ''
      title.value = res.config.title || DEFAULT_TITLE
    }
    return res
  }

  watch(displayTitle, (val) => {
    if (val) document.title = val
  }, { immediate: false })

  return {
    logoData,
    title,
    loaded,
    logoSrc,
    displayTitle,
    load,
    save,
  }
})
