<template>
  <div class="theme-demo">
    <div class="dark-card">
      <div class="card-header">
        <span class="card-title">{{ $t('theme.mode') }}</span>
        <el-icon class="card-icon"><Palette /></el-icon>
      </div>
      
      <div class="demo-section">
        <h3>{{ $t('common.currentTheme') }}: {{ getCurrentThemeLabel() }}</h3>
        <p class="tip-text">{{ $t('theme.description') }}</p>
        
        <div class="theme-controls">
          <el-button-group>
            <el-button 
              :type="themeStore.theme === 'light' ? 'primary' : 'default'"
              @click="themeStore.setTheme('light')"
            >
              <el-icon><Sunny /></el-icon>
              {{ $t('theme.light') }}
            </el-button>
            <el-button 
              :type="themeStore.theme === 'dark' ? 'primary' : 'default'"
              @click="themeStore.setTheme('dark')"
            >
              <el-icon><Moon /></el-icon>
              {{ $t('theme.dark') }}
            </el-button>
            <el-button 
              :type="themeStore.theme === 'system' ? 'primary' : 'default'"
              @click="themeStore.setTheme('system')"
            >
              <el-icon><Monitor /></el-icon>
              {{ $t('theme.system') }}
            </el-button>
          </el-button-group>
        </div>
        
        <div class="style-theme-section">
          <h4>{{ $t('theme.styleTitle') }}</h4>
          <div class="style-themes">
            <div 
              v-for="style in themeStore.THEME_STYLES" 
              :key="style.value"
              class="style-theme-item"
              :class="{ active: themeStore.themeStyle === style.value }"
              @click="themeStore.setThemeStyle(style.value)"
            >
              <div class="style-color-dot" :style="{ background: style.primaryColor }"></div>
              <span>{{ $t(style.label) }}</span>
            </div>
          </div>
        </div>
        
        <div class="language-section">
          <h4>{{ $t('settings.language') }}</h4>
          <el-radio-group v-model="localeStore.currentLocale" @change="localeStore.changeLocale">
            <el-radio label="zh-CN">简体中文</el-radio>
            <el-radio label="en-US">English</el-radio>
          </el-radio-group>
        </div>
      </div>
    </div>
    
    <!-- 演示各种组件 -->
    <div class="dark-card">
      <div class="card-header">
        <span class="card-title">{{ $t('common.componentDemo') }}</span>
        <el-icon class="card-icon"><Menu /></el-icon>
      </div>
      
      <div class="demo-components">
        <el-row :gutter="20">
          <el-col :span="8">
            <el-card>
              <template #header>{{ $t('common.buttons') }}</template>
              <div class="demo-buttons">
                <el-button type="primary">{{ $t('common.primary') }}</el-button>
                <el-button type="success">{{ $t('common.success') }}</el-button>
                <el-button type="warning">{{ $t('common.warning') }}</el-button>
                <el-button type="danger">{{ $t('common.error') }}</el-button>
              </div>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card>
              <template #header>{{ $t('common.form') }}</template>
              <el-form label-width="80px">
                <el-form-item :label="$t('common.name')">
                  <el-input :placeholder="$t('common.pleaseEnter')" />
                </el-form-item>
                <el-form-item :label="$t('common.type')">
                  <el-select :placeholder="$t('common.pleaseSelect')">
                    <el-option label="Option 1" value="1" />
                    <el-option label="Option 2" value="2" />
                  </el-select>
                </el-form-item>
              </el-form>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card>
              <template #header>{{ $t('common.tags') }}</template>
              <div class="demo-tags">
                <el-tag>{{ $t('common.default') }}</el-tag>
                <el-tag type="success">{{ $t('common.success') }}</el-tag>
                <el-tag type="warning">{{ $t('common.warning') }}</el-tag>
                <el-tag type="danger">{{ $t('common.error') }}</el-tag>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { useLocaleStore } from '@/stores/locale'
import { Palette, Sunny, Moon, Monitor, Menu } from '@element-plus/icons-vue'

const { t } = useI18n()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()

const getCurrentThemeLabel = () => {
  const labels = {
    light: t('theme.light'),
    dark: t('theme.dark'),
    system: t('theme.system')
  }
  return labels[themeStore.theme] || themeStore.theme
}
</script>

<style scoped>
.theme-demo {
  .demo-section {
    h3 {
      color: hsl(var(--foreground));
      margin-bottom: 16px;
    }
    
    h4 {
      color: hsl(var(--foreground));
      margin: 24px 0 12px;
      font-size: 14px;
    }
  }
  
  .theme-controls {
    margin: 20px 0;
  }
  
  .style-theme-section {
    .style-themes {
      display: grid;
      grid-template-columns: repeat(5, 1fr);
      gap: 10px;
      margin-top: 12px;
    }

    @media (max-width: 768px) {
      .style-themes {
        grid-template-columns: repeat(2, 1fr);
      }
    }

    .style-theme-item {
      display: flex;
      align-items: center;
      gap: 10px;
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
        width: 18px;
        height: 18px;
        border-radius: 50%;
        border: 2px solid hsl(var(--border));
        box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
      }

      span {
        font-size: 12px;
        color: hsl(var(--muted-foreground));
      }
    }
  }
  
  .language-section {
    margin-top: 24px;
  }
  
  .demo-components {
    margin-top: 20px;
    
    .demo-buttons {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    
    .demo-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }
  }
}
</style>

