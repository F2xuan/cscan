<template>
  <el-popover
    placement="bottom-end"
    :width="520"
    trigger="click"
    popper-class="theme-switcher-popover"
    :show-arrow="false"
    :offset="8"
  >
    <template #reference>
      <div class="theme-switch-btn" :title="$t('theme.settings')">
        <el-icon v-if="themeStore.isDark">
          <Moon />
        </el-icon>
        <el-icon v-else>
          <Sunny />
        </el-icon>
      </div>
    </template>
    
    <div class="theme-switcher">
      <!-- 主题模式选择 -->
      <div class="theme-section">
        <div class="section-title">{{ $t('theme.mode') }}</div>
        <div class="mode-options">
          <div
            v-for="mode in themeModes"
            :key="mode.value"
            class="mode-option"
            :class="{ active: themeStore.theme === mode.value }"
            @click="themeStore.setTheme(mode.value)"
          >
            <el-icon :size="18">
              <component :is="mode.icon" />
            </el-icon>
            <span>{{ $t(mode.label) }}</span>
          </div>
        </div>
      </div>
      
      <!-- 款式选择 -->
      <div class="theme-section">
        <div class="section-title">{{ $t('theme.styleTitle') }}</div>
        <div class="style-options">
          <div
            v-for="style in themeStore.THEME_STYLES"
            :key="style.value"
            class="style-option"
            :class="{ active: themeStore.themeStyle === style.value }"
            @click="themeStore.setThemeStyle(style.value)"
          >
            <div class="style-color-dot" :style="{ background: style.primaryColor }"></div>
            <div class="style-info">
              <span class="style-label">{{ $t(style.label) }}</span>
              <span class="style-desc">{{ $t(style.description) }}</span>
            </div>
            <el-icon v-if="themeStore.themeStyle === style.value" class="style-check">
              <Check />
            </el-icon>
          </div>
        </div>
      </div>
    </div>
  </el-popover>
</template>

<script setup>
import { useThemeStore } from '@/stores/theme'
import { Sunny, Moon, Monitor, Check } from '@element-plus/icons-vue'

const themeStore = useThemeStore()

const themeModes = [
  { value: 'light', label: 'theme.light', icon: Sunny },
  { value: 'dark', label: 'theme.dark', icon: Moon },
  { value: 'system', label: 'theme.system', icon: Monitor },
]
</script>

<style lang="scss" scoped>
.theme-switch-btn {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  cursor: pointer;
  color: hsl(var(--muted-foreground));
  transition: all 0.3s;

  &:hover {
    background: hsl(var(--accent));
    color: hsl(var(--primary));
  }

  .el-icon {
    font-size: 18px;
  }
}

.theme-switcher {
  .theme-section {
    margin-bottom: 14px;
    
    &:last-child {
      margin-bottom: 0;
    }
  }
  
  .section-title {
    font-size: 12px;
    font-weight: 600;
    color: hsl(var(--muted-foreground));
    margin-bottom: 8px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  
  .mode-options {
    display: flex;
    gap: 8px;
    
    .mode-option {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
      padding: 10px 8px;
      border-radius: 8px;
      cursor: pointer;
      border: 2px solid transparent;
      background: hsl(var(--muted));
      color: hsl(var(--muted-foreground));
      transition: all 0.2s;
      
      &:hover {
        background: hsl(var(--accent));
        color: hsl(var(--foreground));
      }
      
      &.active {
        border-color: hsl(var(--primary));
        background: hsl(var(--primary) / 0.1);
        color: hsl(var(--primary));
      }
      
      span {
        font-size: 12px;
        font-weight: 500;
      }
    }
  }
  
  .style-options {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 6px;

    .style-option {
      position: relative;
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 10px;
      border-radius: 8px;
      cursor: pointer;
      border: 2px solid transparent;
      background: hsl(var(--muted));
      transition: all 0.2s;

      &:hover {
        background: hsl(var(--accent));
      }

      &.active {
        border-color: hsl(var(--primary));
        background: hsl(var(--primary) / 0.06);
      }

      .style-color-dot {
        flex-shrink: 0;
        width: 16px;
        height: 16px;
        border-radius: 50%;
        border: 2px solid hsl(var(--border));
        box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
      }

      .style-info {
        display: flex;
        flex-direction: column;
        gap: 0;
        min-width: 0;
        flex: 1;
      }

      .style-label {
        font-size: 12px;
        font-weight: 600;
        color: hsl(var(--foreground));
        line-height: 1.3;
      }

      .style-desc {
        font-size: 10px;
        color: hsl(var(--muted-foreground));
        line-height: 1.3;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        min-width: 0;
      }

      .style-check {
        position: absolute;
        top: 4px;
        right: 4px;
        width: 14px;
        height: 14px;
        background: hsl(var(--primary));
        border-radius: 50%;
        color: hsl(var(--primary-foreground));
        font-size: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
      }
    }
  }
}
</style>

<style>
.theme-switcher-popover {
  padding: 14px !important;
}
</style>
