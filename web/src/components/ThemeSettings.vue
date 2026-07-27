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
          v-for="style in themeStyles" 
          :key="style.value"
          class="style-theme"
          :class="{ active: themeStore.themeStyle === style.value }"
          @click="themeStore.setThemeStyle(style.value)"
        >
          <div class="style-preview-mini" :class="`preview-style-${style.value}`">
            <div class="mini-card"></div>
            <div class="mini-btn"></div>
          </div>
          <span class="style-name">{{ $t(style.label) }}</span>
          <span class="style-desc">{{ $t(style.description) }}</span>
        </div>
      </div>
    </div>

    <div class="setting-section">
      <h3>{{ $t('theme.colorTheme') }}</h3>
      <div class="color-themes">
        <div 
          v-for="theme in colorThemes" 
          :key="theme.value"
          class="color-theme"
          :class="{ active: themeStore.colorTheme === theme.value }"
          @click="themeStore.setColorTheme(theme.value)"
        >
          <div class="color-preview" :class="`preview-${theme.value}`"></div>
          <span>{{ $t(theme.label) }}</span>
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
import { useThemeStore, THEME_STYLES } from '@/stores/theme'
import { useLocaleStore } from '@/stores/locale'
import { Sunny, Moon, Monitor } from '@element-plus/icons-vue'

const themeStore = useThemeStore()
const localeStore = useLocaleStore()

const themeModes = [
  { value: 'light', label: 'theme.light', icon: 'Sunny' },
  { value: 'dark', label: 'theme.dark', icon: 'Moon' },
  { value: 'system', label: 'theme.system', icon: 'Monitor' }
]

const themeStyles = THEME_STYLES

const colorThemes = [
  { value: 'default', label: 'theme.default' },
  { value: 'vercel', label: 'theme.vercel' },
  { value: 'vercel-dark', label: 'theme.vercelDark' },
  { value: 'cosmic-night', label: 'theme.cosmicNight' },
  { value: 'quantum-rose', label: 'theme.quantumRose' },
  { value: 'clean-slate', label: 'theme.cleanSlate' }
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
  margin-bottom: 32px;

  h3 {
    color: hsl(var(--foreground));
    font-size: 16px;
    font-weight: 600;
    margin-bottom: 16px;
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
  display: flex;
  gap: 12px;
}

.style-theme {
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
  min-width: 140px;

  &:hover {
    border-color: hsl(var(--primary));
  }

  &.active {
    border-color: hsl(var(--primary));
    background: hsl(var(--primary) / 0.1);
  }

  .style-preview-mini {
    width: 80px;
    height: 48px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 5px;
    background: hsl(var(--background));
    border: 1px solid hsl(var(--border));

    &.preview-style-vercel {
      border-radius: 4px;
      .mini-card { border-radius: 2px; }
      .mini-btn { border-radius: 2px; }
    }

    &.preview-style-apple {
      border-radius: 12px;
      .mini-card { border-radius: 6px; box-shadow: 0 1px 4px rgba(0,0,0,0.08); }
      .mini-btn { border-radius: 99px; }
    }

    .mini-card {
      width: 50px;
      height: 18px;
      border: 1px solid hsl(var(--border));
      background: hsl(var(--card));
    }

    .mini-btn {
      width: 28px;
      height: 8px;
      background: hsl(var(--primary));
    }
  }

  .style-name {
    font-size: 13px;
    font-weight: 600;
    color: hsl(var(--foreground));
  }

  .style-desc {
    font-size: 11px;
    color: hsl(var(--muted-foreground));
  }
}

.color-themes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 12px;
}

.color-theme {
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

  span {
    font-size: 12px;
    color: hsl(var(--muted-foreground));
  }
}

.color-preview {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: 2px solid hsl(var(--border));

  &.preview-default {
    background: linear-gradient(135deg, #409eff 0%, #66b1ff 100%);
  }

  &.preview-vercel {
    background: linear-gradient(135deg, #000 0%, #333 100%);
  }

  &.preview-vercel-dark {
    background: linear-gradient(135deg, #111 0%, #000 100%);
  }

  &.preview-cosmic-night {
    background: linear-gradient(135deg, #8b5cf6 0%, #3b82f6 100%);
  }

  &.preview-quantum-rose {
    background: linear-gradient(135deg, #ec4899 0%, #f97316 100%);
  }

  &.preview-clean-slate {
    background: linear-gradient(135deg, #64748b 0%, #94a3b8 100%);
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

