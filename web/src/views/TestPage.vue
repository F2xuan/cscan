<template>
  <div class="test-page">
    <div class="dark-card">
      <div class="card-header">
        <span class="card-title">功能测试页面</span>
        <el-icon class="card-icon"><Tools /></el-icon>
      </div>
      
      <div class="test-section">
        <h3>主题测试</h3>
        <p>当前主题: {{ themeStore.theme }}</p>
        <p>当前款式: {{ themeStore.themeStyle }}</p>
        <p>是否暗色模式: {{ themeStore.isDark }}</p>
        
        <div class="button-group">
          <el-button @click="themeStore.setTheme('light')">浅色主题</el-button>
          <el-button @click="themeStore.setTheme('dark')">深色主题</el-button>
          <el-button @click="themeStore.setTheme('system')">跟随系统</el-button>
        </div>
      </div>
      
      <div class="test-section">
        <h3>语言测试</h3>
        <p>当前语言: {{ localeStore.currentLocale }}</p>
        <p>翻译测试: {{ $t('common.success') }}</p>
        
        <div class="button-group">
          <el-button @click="localeStore.changeLocale('zh-CN')">中文</el-button>
          <el-button @click="localeStore.changeLocale('en-US')">English</el-button>
        </div>
      </div>
      
      <div class="test-section">
        <h3>组件测试</h3>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-card>
              <template #header>表单组件</template>
              <el-form label-width="80px">
                <el-form-item label="输入框">
                  <el-input placeholder="请输入内容" />
                </el-form-item>
                <el-form-item label="选择框">
                  <el-select placeholder="请选择">
                    <el-option label="选项1" value="1" />
                    <el-option label="选项2" value="2" />
                  </el-select>
                </el-form-item>
                <el-form-item label="开关">
                  <el-switch />
                </el-form-item>
              </el-form>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card>
              <template #header>按钮组件</template>
              <div class="button-demo">
                <el-button type="primary">主要按钮</el-button>
                <el-button type="success">成功按钮</el-button>
                <el-button type="warning">警告按钮</el-button>
                <el-button type="danger">危险按钮</el-button>
                <el-button type="info">信息按钮</el-button>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useThemeStore } from '@/stores/theme'
import { useLocaleStore } from '@/stores/locale'
import { Tools } from '@element-plus/icons-vue'

const themeStore = useThemeStore()
const localeStore = useLocaleStore()
</script>

<style scoped>
.test-page {
  .test-section {
    margin-bottom: 32px;
    
    h3 {
      color: hsl(var(--foreground));
      margin-bottom: 16px;
      font-size: 16px;
    }
    
    p {
      color: hsl(var(--muted-foreground));
      margin-bottom: 8px;
    }
  }
  
  .button-group {
    margin-top: 16px;
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }
  
  .button-demo {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
}
</style>

