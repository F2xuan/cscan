import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const STORAGE_KEY = 'cscan_online_search_form'

function loadFormFromStorage() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved)
      return {
        source: parsed.source || 'fofa',
        query: parsed.query || '',
        page: 1,
        size: parsed.size || 50
      }
    }
  } catch (e) { /* ignore */ }
  return null
}

export const useOnlineSearchStore = defineStore('onlineSearch', () => {
  // 搜索表单（从 localStorage 恢复上次搜索条件）
  const searchForm = ref(loadFormFromStorage() || {
    source: 'fofa',
    query: '',
    page: 1,
    size: 50
  })

  // 搜索结果
  const tableData = ref([])
  const total = ref(0)

  // 持久化搜索条件到 localStorage（仅保存 source/query/size，不保存 page）
  function persistForm() {
    const { source, query, size } = searchForm.value
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ source, query, size }))
  }

  // 监听表单变化自动持久化
  watch(searchForm, () => persistForm(), { deep: true })

  // 保存搜索状态
  function saveState(form, data, totalCount) {
    searchForm.value = { ...form }
    tableData.value = data
    total.value = totalCount
  }

  // 清空状态
  function clearState() {
    searchForm.value = {
      source: 'fofa',
      query: '',
      page: 1,
      size: 50
    }
    tableData.value = []
    total.value = 0
  }

  return {
    searchForm,
    tableData,
    total,
    saveState,
    clearState
  }
})
