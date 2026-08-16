<script setup>
import { computed, ref, watch } from 'vue'
import { getTechName, techIconUrl } from '@/utils/tech'

const props = defineProps({
  // 原始技术条目（可含 [来源] / :版本 后缀）
  tech: { type: String, required: true },
  // 显示名覆盖：缺省用归一化后的技术名
  label: { type: String, default: '' },
  // el-tag 类型透传（primary/success/warning/danger/info）
  type: { type: String, default: '' },
  size: { type: String, default: 'small' }
})

const displayName = computed(() => props.label || getTechName(props.tech) || props.tech)
const iconUrl = computed(() => techIconUrl(props.tech))
const iconFailed = ref(false)

watch(() => props.tech, () => {
  iconFailed.value = false
})
</script>

<template>
  <el-tag :type="type || undefined" :size="size" class="tech-tag">
    <img
      v-if="iconUrl && !iconFailed"
      :src="iconUrl"
      :alt="displayName"
      class="tech-tag-icon"
      loading="lazy"
      @error="iconFailed = true"
    >
    <span class="tech-tag-label">{{ displayName }}</span>
  </el-tag>
</template>

<style scoped>
.tech-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.tech-tag-icon {
  width: 14px;
  height: 14px;
  object-fit: contain;
  flex-shrink: 0;
  /* 深色主题下浅色 SVG 仍可辨认 */
  border-radius: 2px;
}
</style>
