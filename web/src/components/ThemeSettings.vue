<template>
  <div class="theme-settings">
    <div class="setting-section">
      <h3>{{ $t('theme.mode') }}</h3>
      <div class="theme-modes">
        <div 
          v-for="mode in themeModes" 
          :key="mode.value"
          class="theme-mode"
          :class="{ active: themeStore.theme === mode.value }"
          @click="themeStore.setTheme(mode.value)"
        >
          <el-icon>
            <Sunny v-if="mode.value === 'light'" />
            <Moon v-else-if="mode.value === 'dark'" />
            <Monitor v-else />
          </el-icon>
          <span>{{ $t(mode.label) }}</span>
        </div>
      </div>
    </div>

    <div class="setting-section">
      <h3>{{ $t('theme.styleTitle') }}</h3>
      <div class="style-themes">
        <div 
          v-for="style in themeStore.THEME_STYLES" 
          :key="style.value"
          class="style-theme"
          :class="{ active: themeStore.themeStyle === style.value }"
          @click="themeStore.setThemeStyle(style.value)"
        >
          <div class="style-color-dot" :style="{ background: style.primaryColor }"></div>
          <div class="style-info">
            <span class="style-name">{{ $t(style.label) }}</span>
            <span class="style-desc">{{ $t(style.description) }}</span>
          </div>
          <el-icon v-if="themeStore.themeStyle === style.value" class="style-check">
            <Check />
          </el-icon>
        </div>
      </div>
    </div>

    <div class="setting-section">
      <h3>{{ $t('settings.language') }}</h3>
      <div class="language-options">
        <div 
          v-for="locale in localeStore.supportLocales" 
          :key="locale"
          class="language-option"
          :class="{ active: localeStore.currentLocale === locale }"
          @click="localeStore.changeLocale(locale)"
        >
          <span>{{ getLanguageName(locale) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useThemeStore } from '@/stores/theme'
import { useLocaleStore } from '@/stores/locale'
import { Sunny, Moon, Monitor, Check } from '@element-plus/icons-vue'

const themeStore = useThemeStore()
const localeStore = useLocaleStore()

const themeModes = [
  { value: 'light', label: 'theme.light', icon: 'Sunny' },
  { value: 'dark', label: 'theme.dark', icon: 'Moon' },
  { value: 'system', label: 'theme.system', icon: 'Monitor' }
]

function getLanguageName(locale) {
  const names = {
    'zh-CN': '简体中文',
    'en-US': 'English'
  }
  return names[locale] || locale
}
</script>

<style scoped>
.theme-settings {
  padding: 20px;
}

.setting-section {
  margin-bottom: 28px;

  h3 {
    color: hsl(var(--foreground));
    font-size: 15px;
    font-weight: 600;
    margin-bottom: 14px;
  }
}

.theme-modes {
  display: flex;
  gap: 12px;
}

.theme-mode {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px;
  border: 2px solid hsl(var(--border));
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  background: hsl(var(--card));

  &:hover {
    border-color: hsl(var(--primary));
  }

  &.active {
    border-color: hsl(var(--primary));
    background: hsl(var(--primary) / 0.1);
  }

  .el-icon {
    font-size: 24px;
    color: hsl(var(--foreground));
  }

  span {
    font-size: 12px;
    color: hsl(var(--muted-foreground));
  }
}

.style-themes {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
}

@media (max-width: 1200px) {
  .style-themes {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 768px) {
  .style-themes {
    grid-template-columns: repeat(2, 1fr);
  }
}

.style-theme {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 12px;
  border: 2px solid hsl(var(--border));
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  background: hsl(var(--card));

  &:hover {
    border-color: hsl(var(--primary));
  }

  &.active {
    border-color: hsl(var(--primary));
    background: hsl(var(--primary) / 0.1);
  }

  .style-color-dot {
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: 2px solid hsl(var(--border));
    box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
  }

  .style-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    width: 100%;
  }

  .style-name {
    font-size: 13px;
    font-weight: 600;
    color: hsl(var(--foreground));
  }

  .style-desc {
    font-size: 11px;
    color: hsl(var(--muted-foreground));
    line-height: 1.4;
  }

  .style-check {
    position: absolute;
    top: 8px;
    right: 8px;
    color: hsl(var(--primary));
    font-size: 16px;
  }
}

.language-options {
  display: flex;
  gap: 12px;
}

.language-option {
  padding: 12px 24px;
  border: 2px solid hsl(var(--border));
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  background: hsl(var(--card));

  &:hover {
    border-color: hsl(var(--primary));
  }

  &.active {
    border-color: hsl(var(--primary));
    background: hsl(var(--primary) / 0.1);
  }

  span {
    font-size: 14px;
    color: hsl(var(--foreground));
  }
}
</style>
