<template>
  <el-select
    :model-value="modelValue"
    :placeholder="$t('asset.targetView.allTypes')"
    class="target-filter-select"
    @update:model-value="handleChange"
  >
    <el-option
      v-for="opt in options"
      :key="opt.value"
      :label="opt.label"
      :value="opt.value"
    >
      <div class="filter-option">
        <span class="color-dot" :style="{ background: opt.color }" />
        <span>{{ opt.label }}</span>
      </div>
    </el-option>
  </el-select>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  modelValue: { type: String, default: '' },
})

const emit = defineEmits(['update:modelValue'])

const { t } = useI18n()

// 对齐 open-asm：DOMAIN 蓝 / IP 橙（CIDR 无数据未列出）
const options = computed(() => [
  { value: 'domain', label: t('asset.targetView.typeDomain'), color: '#3b82f6' },
  { value: 'ip', label: t('asset.targetView.typeIp'), color: '#f97316' },
])

function handleChange(val) {
  emit('update:modelValue', val)
}
</script>

<style scoped lang="scss">
// 对齐 open-asm SelectTrigger：border-dashed text-xs 小号下拉
.target-filter-select {
  width: 120px;

  :deep(.el-select__wrapper) {
    min-height: 28px;
    font-size: 12px;
    border: 1px dashed var(--el-border-color);
    box-shadow: none !important;
    background: transparent;
  }

  :deep(.el-select__wrapper.is-hovering) {
    border-color: var(--el-color-primary);
  }
}

.filter-option {
  display: flex;
  align-items: center;
  gap: 8px;

  .color-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
}
</style>
