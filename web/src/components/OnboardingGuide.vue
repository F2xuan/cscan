<template>
  <div class="og-overlay" role="dialog" aria-modal="true" :aria-label="t('onboarding.title')">
    <div class="og-card" tabindex="-1" ref="cardRef">
      <div class="og-header">
        <div class="og-title-row">
          <span class="og-title">{{ t('onboarding.title') }}</span>
          <button class="og-skip" type="button" @click="onSkip">{{ t('onboarding.skip') }}</button>
        </div>
        <span class="og-subtitle">{{ t('onboarding.subtitle') }}</span>
      </div>

      <el-steps :active="active" finish-status="success" class="og-steps" align-center>
        <el-step :title="t('onboarding.step1')" />
        <el-step :title="t('onboarding.step2')" />
        <el-step :title="t('onboarding.step3')" />
      </el-steps>

      <div class="og-body">
        <!-- 步骤1：输入目标 -->
        <div v-show="active === 0" class="og-pane">
          <label class="og-label">{{ t('onboarding.targetLabel') }}</label>
          <el-input
            v-model="targets"
            type="textarea"
            :rows="4"
            :placeholder="t('onboarding.targetPlaceholder')"
            @keyup.enter.ctrl="next"
          />
          <p class="og-hint">{{ t('onboarding.targetHint') }}</p>
        </div>

        <!-- 步骤2：选择扫描方式 -->
        <div v-show="active === 1" class="og-pane">
          <label class="og-label">{{ t('onboarding.modeLabel') }}</label>
          <el-radio-group v-model="mode">
            <el-radio-button value="quick">{{ t('onboarding.modeQuick') }}</el-radio-button>
            <el-radio-button value="full">{{ t('onboarding.modeFull') }}</el-radio-button>
          </el-radio-group>
          <p class="og-hint">
            {{ mode === 'full' ? t('onboarding.modeFullHint') : t('onboarding.modeQuickHint') }}
          </p>
        </div>

        <!-- 步骤3：确认并开始 -->
        <div v-show="active === 2" class="og-pane">
          <ul class="og-summary">
            <li>
              <span class="og-summary-key">{{ t('onboarding.summaryTarget') }}</span>
              <span class="og-summary-val">{{ targetCount }} {{ t('onboarding.summaryTargets') }}</span>
            </li>
            <li>
              <span class="og-summary-key">{{ t('onboarding.summaryMode') }}</span>
              <span class="og-summary-val">{{ mode === 'full' ? t('onboarding.modeFull') : t('onboarding.modeQuick') }}</span>
            </li>
          </ul>
          <p class="og-hint">{{ t('onboarding.startHint') }}</p>
        </div>
      </div>

      <div class="og-footer">
        <el-button v-if="active > 0" @click="prev">{{ t('onboarding.prev') }}</el-button>
        <el-button v-if="active < 2" type="primary" :disabled="!canNext" @click="next">
          {{ t('onboarding.next') }}
        </el-button>
        <el-button v-else type="primary" :loading="loading" @click="onStart">
          {{ t('onboarding.start') }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { quickCreateTask } from '@/api/task'
import { completeOnboarding } from '@/api/auth'
import { splitTargets, isValidTargets } from '@/utils/quickScan'

const emit = defineEmits(['finished'])
const router = useRouter()
const { t } = useI18n()

const active = ref(0)
const targets = ref('')
const mode = ref('quick')
const loading = ref(false)
const cardRef = ref(null)

const targetList = computed(() => splitTargets(targets.value))
const targetCount = computed(() => targetList.value.length)
const canNext = computed(() => (active.value === 0 ? isValidTargets(targets.value) : true))

function next() {
  if (active.value === 0 && !isValidTargets(targets.value)) return
  if (active.value < 2) active.value += 1
}
function prev() {
  if (active.value > 0) active.value -= 1
}

async function onStart() {
  if (!isValidTargets(targets.value)) {
    ElMessage.warning(t('onboarding.invalid'))
    return
  }
  loading.value = true
  try {
    const res = await quickCreateTask({ targets: targetList.value.join('\n'), mode: mode.value })
    if (res && res.code === 0 && res.taskId) {
      await completeOnboarding().catch(() => {})
      ElMessage.success(t('onboarding.success'))
      const taskId = res.taskId
      setTimeout(() => {
        router.push(`/task/detail?id=${taskId}`)
        emit('finished')
      }, 1200)
    } else {
      ElMessage.error((res && res.msg) || t('onboarding.failed'))
      loading.value = false
    }
  } catch (e) {
    ElMessage.error(t('onboarding.failed'))
    loading.value = false
  }
}

async function onSkip() {
  try {
    await completeOnboarding()
    ElMessage.info(t('onboarding.skipped'))
  } catch (e) {
    // 即使后端失败也允许关闭，避免阻塞用户
  }
  emit('finished')
}

onMounted(async () => {
  await nextTick()
  cardRef.value?.focus()
})
</script>

<style scoped lang="scss">
.og-overlay {
  position: fixed;
  inset: 0;
  z-index: 2000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: hsl(var(--overlay, 0 0% 0% / 0.55));
  padding: 16px;
}

.og-card {
  width: 560px;
  max-width: 100%;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: var(--radius, 12px);
  box-shadow: 0 20px 60px hsl(0 0% 0% / 0.3);
  padding: 24px 28px;
  outline: none;
  color: hsl(var(--foreground));
}

.og-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 8px;
}
.og-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.og-title {
  font-size: 18px;
  font-weight: 700;
  color: hsl(var(--foreground));
}
.og-skip {
  background: none;
  border: none;
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 6px;
  transition: background 0.15s, color 0.15s;

  &:hover {
    background: hsl(var(--muted));
    color: hsl(var(--foreground));
  }
}
.og-subtitle {
  font-size: 13px;
  color: hsl(var(--muted-foreground));
}

.og-steps {
  margin: 12px 0 20px;
}

.og-body {
  min-height: 140px;
}
.og-pane {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.og-label {
  font-size: 13px;
  font-weight: 600;
  color: hsl(var(--foreground));
}
.og-hint {
  font-size: 12px;
  color: hsl(var(--muted-foreground));
  margin: 0;
  line-height: 1.5;
}

.og-summary {
  list-style: none;
  margin: 0;
  padding: 0;
  border: 1px solid hsl(var(--border));
  border-radius: var(--radius, 8px);
  overflow: hidden;

  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    &:not(:last-child) {
      border-bottom: 1px solid hsl(var(--border));
    }
  }
}
.og-summary-key {
  font-size: 13px;
  color: hsl(var(--muted-foreground));
}
.og-summary-val {
  font-size: 13px;
  font-weight: 600;
  color: hsl(var(--foreground));
}

.og-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 20px;
}
</style>
