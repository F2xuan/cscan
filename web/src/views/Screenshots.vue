<template>
  <div class="screenshots-tab">
    <!-- 搜索和过滤栏 -->
    <div class="toolbar">
      <el-input
        v-model="searchQuery"
        :placeholder="t('asset.screenshotsTab.searchPlaceholder')"
        clearable
        class="search-input"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button @click="showFilters = !showFilters">
        <el-icon><Filter /></el-icon>
        {{ t('asset.screenshotsTab.filters') }}
      </el-button>
      <el-button @click="refreshData">
        <el-icon><Refresh /></el-icon>
        {{ t('asset.screenshotsTab.refresh') }}
      </el-button>
    </div>

    <!-- 高级过滤器 -->
    <div v-if="showFilters" class="filters-panel">
      <el-form :inline="true">
        <el-form-item :label="t('asset.screenshotsTab.statusCodes')">
          <el-select v-model="filters.statusCodes" multiple :placeholder="t('asset.screenshotsTab.selectStatus')" clearable filterable>
            <el-option
              v-for="code in filterOptions.statusCodes"
              :key="code"
              :label="code"
              :value="code"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('asset.screenshotsTab.timeRange')">
          <el-select v-model="filters.timeRange" :placeholder="t('asset.screenshotsTab.selectTime')" clearable>
            <el-option :label="t('asset.screenshotsTab.allTime')" value="all" />
            <el-option :label="t('asset.screenshotsTab.last24h')" value="24h" />
            <el-option :label="t('asset.screenshotsTab.last7d')" value="7d" />
            <el-option :label="t('asset.screenshotsTab.last30d')" value="30d" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="applyFilters">{{ t('asset.screenshotsTab.apply') }}</el-button>
          <el-button @click="resetFilters">{{ t('asset.screenshotsTab.reset') }}</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 截图网格 -->
    <div v-loading="loading" class="screenshots-grid">
      <div
        v-for="item in screenshots"
        :key="item.id"
        class="screenshot-card"
        @click="viewDetails(item)"
      >
        <!-- 截图图片 -->
        <div
          class="screenshot-image-container"
          @mouseenter="showPreview(item, $event)"
          @mouseleave="hidePreview"
        >
          <img
            v-if="item.screenshot"
            :src="formatScreenshotUrl(item.screenshot)"
            :alt="item.name"
            class="screenshot-image"
            loading="lazy"
            @error="handleScreenshotError"
          />
          <div v-else class="no-screenshot">
            <el-icon><Picture /></el-icon>
            <span>{{ t('asset.screenshotsTab.noScreenshot') }}</span>
          </div>

          <!-- 状态标签 -->
          <div class="screenshot-status">
            <el-tag :type="getStatusType(item.status)" size="small">
              {{ item.status }}
            </el-tag>
          </div>
        </div>

        <!-- 截图信息 -->
        <div class="screenshot-info">
          <div class="screenshot-title">
            <el-icon class="icon"><Monitor /></el-icon>
            <span class="name">{{ item.name }}</span>
            <span class="port">:{{ item.port }}</span>
          </div>

          <div class="screenshot-meta">
            <span class="page-title">{{ item.title || t('asset.screenshotsTab.noTitle') }}</span>
          </div>

          <div class="screenshot-details">
            <span class="ip">{{ item.ip }}</span>
            <span class="time">{{ item.lastUpdated }}</span>
          </div>

          <!-- 技术标签 -->
          <div v-if="item.technologies && item.technologies.length" class="tech-tags">
            <el-tag
              v-for="tech in item.technologies.slice(0, 3)"
              :key="tech.name"
              size="small"
              class="tech-tag"
            >
              {{ tech.name }}
            </el-tag>
            <el-tag v-if="item.technologies.length > 3" size="small" type="info">
              +{{ item.technologies.length - 3 }}
            </el-tag>
          </div>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-if="!loading && screenshots.length === 0" class="empty-state">
      <el-empty description="暂无截图数据" />
    </div>

    <!-- 分页 -->
    <el-pagination
      v-if="screenshots.length > 0"
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      :total="total"
      :page-sizes="[5, 10, 20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      class="pagination"
      @size-change="loadData"
      @current-change="loadData"
    />

    <!-- 截图详情抽屉 - 使用共享组件 -->
    <AssetDetailDrawer
      v-model:visible="showDetailsDialog"
      :asset="selectedItem"
      @preview-show="showPreview"
      @preview-hide="hidePreview"
    />

    <!-- 图片预览浮层 -->
    <Teleport to="body">
      <Transition name="preview-fade">
        <div
          v-if="previewVisible"
          class="screenshot-preview-overlay"
          :style="{
            left: previewPosition.x + 'px',
            top: previewPosition.y + 'px',
            width: previewSize.width + 'px',
            maxHeight: previewSize.height + 'px'
          }"
        >
          <div class="preview-container">
            <img
              :src="previewImage"
              alt="Screenshot Preview"
              class="preview-image"
              @error="handleScreenshotError"
            />
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { debounce } from 'lodash-es'
import {
  Search,
  Filter,
  Refresh,
  Picture,
  Monitor
} from '@element-plus/icons-vue'
import { getScreenshots, getAssetFilterOptions } from '@/api/asset'
import { formatScreenshotUrl, handleScreenshotError } from '@/utils/screenshot'
import AssetDetailDrawer from '@/components/asset/AssetDetailDrawer.vue'

const { t } = useI18n()

const loading = ref(false)
const searchQuery = ref('')
const showFilters = ref(false)
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const screenshots = ref([])
const showDetailsDialog = ref(false)
const selectedItem = ref(null)
const filters = ref({
  statusCodes: [],
  timeRange: 'all'
})

// 过滤器选项（从后端动态加载）
const filterOptions = ref({
  statusCodes: []
})

// 图片预览
const previewVisible = ref(false)
const previewImage = ref('')
const previewPosition = ref({ x: 0, y: 0 })
const previewSize = ref({ width: 500, height: 400 })

const showPreview = (item, event) => {
  if (!item.screenshot) return

  previewImage.value = formatScreenshotUrl(item.screenshot)
  previewVisible.value = true

  // 计算预览位置
  const rect = event.currentTarget.getBoundingClientRect()

  // 检查是否在抽屉或对话框中
  const isInDrawer = event.currentTarget.closest('.el-drawer__body') !== null
  const isInDialog = event.currentTarget.closest('.el-dialog__body') !== null
  const isInDetailView = isInDrawer || isInDialog

  let previewWidth, previewHeight, padding

  if (isInDetailView) {
    // 在详情视图中，使用更大的预览尺寸
    previewWidth = Math.min(800, window.innerWidth * 0.5)
    previewHeight = Math.min(900, window.innerHeight * 0.8)
    padding = 30
  } else {
    // 在列表视图中，使用较小的预览尺寸
    previewWidth = 500
    previewHeight = 400
    padding = 20
  }

  previewSize.value = { width: previewWidth, height: previewHeight }

  // 默认显示在右侧
  let x = rect.right + padding
  let y = rect.top

  // 如果右侧空间不够，显示在左侧
  if (x + previewWidth > window.innerWidth) {
    x = rect.left - previewWidth - padding
  }

  // 如果下方空间不够，向上调整
  if (y + previewHeight > window.innerHeight) {
    y = window.innerHeight - previewHeight - padding
  }

  // 确保不超出顶部
  if (y < padding) {
    y = padding
  }

  // 确保不超出左侧
  if (x < padding) {
    x = padding
  }

  previewPosition.value = { x, y }
}

const hidePreview = () => {
  previewVisible.value = false
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await getScreenshots({
      page: currentPage.value,
      pageSize: pageSize.value,
      query: searchQuery.value,
      technologies: [],
      ports: [],
      statusCodes: filters.value.statusCodes,
      timeRange: filters.value.timeRange,
      hasScreenshot: true
    })
    if (res.code === 0) {
      screenshots.value = res.list || []
      total.value = res.total || 0
    } else {
      ElMessage.error(res.msg || t('asset.screenshotsTab.loadFailed'))
    }
  } catch (error) {
    console.error(t('asset.screenshotsTab.loadFailed'), error)
    ElMessage.error(t('asset.screenshotsTab.loadFailed'))
  } finally {
    loading.value = false
  }
}

const handleSearch = debounce(() => {
  currentPage.value = 1
  loadData()
}, 300)

const refreshData = () => {
  loadData()
  ElMessage.success(t('asset.screenshotsTab.refreshSuccess'))
}

const applyFilters = () => {
  currentPage.value = 1
  loadData()
}

const resetFilters = () => {
  filters.value = {
    statusCodes: [],
    timeRange: 'all'
  }
  currentPage.value = 1
  loadData()
}

const viewDetails = (item) => {
  selectedItem.value = item
  showDetailsDialog.value = true
}

const getStatusType = (status) => {
  const statusStr = String(status || '')
  if (statusStr.startsWith('2')) return 'success'
  if (statusStr.startsWith('3')) return 'warning'
  if (statusStr.startsWith('4') || statusStr.startsWith('5')) return 'danger'
  return 'info'
}

// 加载过滤器选项
const loadFilterOptions = async () => {
  try {
    const res = await getAssetFilterOptions({
      hasScreenshot: true
    })
    if (res.code === 0) {
      filterOptions.value = {
        statusCodes: res.statusCodes || []
      }
    }
  } catch (error) {
    console.error(t('asset.screenshotsTab.loadFailed'), error)
  }
}

onMounted(() => {
  loadFilterOptions()
  loadData()
})
</script>

<style lang="scss" scoped>
.screenshots-tab {
  .toolbar {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;

    .search-input {
      flex: 1;
      max-width: 400px;
    }
  }

  .filters-panel {
    background: hsl(var(--card));
    border: 1px solid hsl(var(--border));
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 16px;

    :deep(.el-select) {
      min-width: 200px;
    }
  }

  .screenshots-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 20px;
    margin-bottom: 24px;
  }

  .screenshot-card {
    background: hsl(var(--card));
    border: 1px solid hsl(var(--border));
    border-radius: 8px;
    overflow: hidden;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      border-color: hsl(var(--primary));
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
      transform: translateY(-2px);
    }
  }

  .screenshot-image-container {
    position: relative;
    height: 200px;
    background: hsl(var(--muted));

    .screenshot-image {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .no-screenshot {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      color: hsl(var(--muted-foreground));

      .el-icon {
        font-size: 48px;
        margin-bottom: 8px;
      }
    }

    .screenshot-status {
      position: absolute;
      top: 8px;
      right: 8px;
    }
  }

  .screenshot-info {
    padding: 16px;

    .screenshot-title {
      display: flex;
      align-items: center;
      gap: 6px;
      margin-bottom: 8px;

      .icon {
        color: hsl(var(--muted-foreground));
      }

      .name {
        font-weight: 500;
        color: hsl(var(--foreground));
      }

      .port {
        color: hsl(var(--primary));
        font-weight: 500;
      }
    }

    .screenshot-meta {
      margin-bottom: 8px;

      .page-title {
        font-size: 13px;
        color: hsl(var(--muted-foreground));
        display: block;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .screenshot-details {
      display: flex;
      justify-content: space-between;
      font-size: 12px;
      color: hsl(var(--muted-foreground));
      margin-bottom: 12px;
    }

    .tech-tags {
      display: flex;
      gap: 6px;
      flex-wrap: wrap;

      .tech-tag {
        font-size: 11px;
      }
    }
  }

  .empty-state {
    padding: 60px 20px;
    text-align: center;
  }

  .pagination {
    margin-top: 16px;
  }
}

// 图片预览样式
.screenshot-preview-overlay {
  position: fixed;
  z-index: 9999;
  pointer-events: none;
  max-width: 90vw;

  .preview-container {
    background: hsl(var(--card));
    border: 2px solid hsl(var(--primary));
    border-radius: 8px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
    overflow: hidden;
    width: 100%;
    height: 100%;

    .preview-image {
      width: 100%;
      height: 100%;
      object-fit: contain;
      display: block;
    }
  }
}

// 预览动画
.preview-fade-enter-active,
.preview-fade-leave-active {
  transition: opacity 0.2s ease;
}

.preview-fade-enter-from,
.preview-fade-leave-to {
  opacity: 0;
}
</style>
