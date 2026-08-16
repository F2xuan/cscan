<template>
  <el-select
    :model-value="modelValue"
    :placeholder="$t('asset.targetView.allTargets')"
    class="target-filter-select"
    @update:model-value="handleChange"
  >
    <el-option
      v-for="opt in options"
      :key="opt.value"
      :label="opt.label"
      :value="opt.value"
    />
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

const options = computed(() => [
  { value: 'internal', label: t('asset.targetView.internal') },
  { value: 'external', label: t('asset.targetView.external') },
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
</style>
