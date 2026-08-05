<template>
  <el-container class="layout-container">
    <!-- 侧边栏 -->
    <el-aside :width="isCollapse ? '64px' : '250px'"
      :class="['aside', `style-${themeStore.themeStyle}`, { collapsed: isCollapse }]">
      <div class="logo">
        <img :src="brandingStore.logoSrc" alt="logo" />
        <span v-show="!isCollapse">{{ brandingStore.displayTitle }}</span>
      </div>

      <div class="menu-wrapper">
        <el-menu :default-active="$route.path" :default-openeds="defaultOpeneds" :collapse="isCollapse" router
          :unique-opened="false">
          <!-- 主控台分组 -->
          <el-menu-item index="/dashboard">
            <el-icon>
              <Odometer />
            </el-icon>
            <template #title>{{ $t('navigation.dashboard') }}</template>
          </el-menu-item>
          <el-sub-menu index="asset-menu">
            <template #title>
              <el-icon>
                <Monitor />
              </el-icon>
              <span>{{ $t('navigation.assetManagement') }}</span>
            </template>
            <el-menu-item index="/asset-management">
              <el-icon>
                <DataAnalysis />
              </el-icon>
              <template #title>{{ $t('navigation.assetOverview') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/space-search">
              <el-icon>
                <Search />
              </el-icon>
              <template #title>{{ $t('navigation.assetSpaceSearch') }}</template>
            </el-menu-item>
          </el-sub-menu>

          <!-- 暴露面管理 -->
          <el-sub-menu index="exposure-menu">
            <template #title>
              <el-icon>
                <View />
              </el-icon>
              <span>{{ $t('navigation.exposure') }}</span>
            </template>
            <el-menu-item index="/asset-management/exposure/subdomain">
              <template #title>{{ $t('navigation.exposureSubdomain') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/ip">
              <template #title>{{ $t('navigation.exposureIp') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/port">
              <template #title>{{ $t('navigation.exposurePort') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/site">
              <template #title>{{ $t('navigation.exposureSite') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/icon">
              <template #title>{{ $t('navigation.exposureIcon') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/app">
              <template #title>{{ $t('navigation.exposureApp') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/screenshot">
              <template #title>{{ $t('navigation.exposureScreenshot') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/dir">
              <template #title>{{ $t('navigation.exposureDir') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/exposure/js">
              <template #title>{{ $t('navigation.exposureJs') }}</template>
            </el-menu-item>
          </el-sub-menu>

          <!-- 风险（证书 / 敏感信息 / 漏洞） -->
          <el-sub-menu index="risk-menu">
            <template #title>
              <el-icon>
                <Warning />
              </el-icon>
              <span>{{ $t('navigation.risk') }}</span>
            </template>
            <el-menu-item index="/asset-management/fingerprint/cert">
              <template #title>{{ $t('navigation.certAsset') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/risk/sensitive-info">
              <template #title>{{ $t('navigation.riskSensitiveInfo') }}</template>
            </el-menu-item>
            <el-menu-item index="/asset-management/risk/vuln">
              <template #title>{{ $t('navigation.riskVuln') }}</template>
            </el-menu-item>
          </el-sub-menu>
          <!-- 分割线 -->
          <div class="menu-divider"></div>

          <!-- 任务管理 -->
          <el-menu-item index="/task">
            <el-icon>
              <List />
            </el-icon>
            <template #title>{{ $t('navigation.taskManagement') }}</template>
          </el-menu-item>

          <!-- 空间引擎分组 -->
          <el-sub-menu index="space-engine-menu">
            <template #title>
              <el-icon>
                <Connection />
              </el-icon>
              <span>{{ $t('navigation.spaceEngine') }}</span>
            </template>
            <el-menu-item index="/space-engine/online-search">
              <el-icon>
                <Search />
              </el-icon>
              <template #title>{{ $t('navigation.onlineSearch') }}</template>
            </el-menu-item>
            <el-menu-item index="/space-engine/api-config">
              <el-icon>
                <Key />
              </el-icon>
              <template #title>{{ $t('navigation.spaceEngineApiConfig') }}</template>
            </el-menu-item>
            <el-menu-item index="/space-engine/cron-task">
              <el-icon>
                <Timer />
              </el-icon>
              <template #title>{{ $t('navigation.spaceEngineCronTask') }}</template>
            </el-menu-item>
          </el-sub-menu>

          <!-- 扫描配置分组 -->
          <el-sub-menu index="scan-config-menu">
            <template #title>
              <el-icon>
                <Operation />
              </el-icon>
              <span>{{ $t('navigation.scanConfig') }}</span>
            </template>
            <el-menu-item index="/cron-task">
              <el-icon>
                <Timer />
              </el-icon>
              <template #title>{{ $t('navigation.cronTask') }}</template>
            </el-menu-item>
            <el-menu-item index="/settings-subfinder">
              <el-icon>
                <Search />
              </el-icon>
              <template #title>{{ $t('navigation.subdomainConfig') }}</template>
            </el-menu-item>
            <el-menu-item index="/poc">
              <el-icon>
                <Aim />
              </el-icon>
              <template #title>{{ $t('navigation.pocManagement') }}</template>
            </el-menu-item>
            <el-menu-item index="/fingerprint" :title="$t('navigation.fingerprintManagement')">
              <el-icon>
                <Stamp />
              </el-icon>
              <template #title>{{ $t('navigation.fingerprintManagement') }}</template>
            </el-menu-item>
            <el-menu-item index="/blacklist">
              <el-icon>
                <CircleClose />
              </el-icon>
              <template #title>{{ $t('navigation.blacklist') }}</template>
            </el-menu-item>
          </el-sub-menu>

          <!-- 分割线 -->
          <div class="menu-divider"></div>

          <!-- AI配置（管理员，置顶） -->
          <el-menu-item v-if="userStore.role === 'admin' || userStore.role === 'superadmin'" index="/ai-config">
            <el-icon>
              <MagicStick />
            </el-icon>
            <template #title>{{ $t('navigation.aiConfig') }}</template>
          </el-menu-item>
          <el-menu-item index="/worker">
            <el-icon>
              <Connection />
            </el-icon>
            <template #title>{{ $t('navigation.workerNodes') }}</template>
          </el-menu-item>
          <el-menu-item index="/worker-logs">
            <el-icon>
              <Document />
            </el-icon>
            <template #title>{{ $t('navigation.workerLogs') }}</template>
          </el-menu-item>

          <!-- 高级配置分组 -->
          <el-sub-menu index="advanced-config-menu">
            <template #title>
              <el-icon>
                <Operation />
              </el-icon>
              <span>{{ $t('navigation.advancedConfig') }}</span>
            </template>
            <el-menu-item index="/settings-notify">
              <el-icon>
                <Bell />
              </el-icon>
              <template #title>{{ $t('navigation.notifyConfig') }}</template>
            </el-menu-item>
            <el-menu-item index="/settings-reverify" :title="$t('navigation.reverifyConfig')">
              <el-icon>
                <Timer />
              </el-icon>
              <template #title>{{ $t('navigation.reverifyConfig') }}</template>
            </el-menu-item>
            <el-menu-item index="/high-risk-filter">
              <el-icon>
                <Warning />
              </el-icon>
              <template #title>{{ $t('navigation.highRiskFilter') }}</template>
            </el-menu-item>
          </el-sub-menu>

          <!-- 系统管理分组 -->
          <el-sub-menu index="system-management">
            <template #title>
              <el-icon>
                <Setting />
              </el-icon>
              <span>{{ $t('navigation.systemManagement') }}</span>
            </template>
            <el-menu-item v-if="userStore.role === 'admin' || userStore.role === 'superadmin'"
              index="/user">
              <el-icon>
                <User />
              </el-icon>
              <template #title>{{ $t('navigation.userManagement') }}</template>
            </el-menu-item>
            <el-menu-item index="/organization" :title="$t('navigation.organizationManagement')">
              <el-icon>
                <OfficeBuilding />
              </el-icon>
              <template #title>{{ $t('navigation.organizationManagement') }}</template>
            </el-menu-item>
            <el-menu-item v-if="userStore.role === 'admin' || userStore.role === 'superadmin'"
              index="/settings-branding">
              <el-icon>
                <Picture />
              </el-icon>
              <template #title>{{ $t('navigation.brandingConfig') }}</template>
            </el-menu-item>
          </el-sub-menu>

        </el-menu>
      </div>

    </el-aside>

    <el-container>
      <!-- 顶部导航 -->
      <el-header :class="['header', `style-${themeStore.themeStyle}`]">
        <div class="header-left">
          <el-icon class="collapse-btn" @click="isCollapse = !isCollapse">
            <Fold v-if="!isCollapse" />
            <Expand v-else />
          </el-icon>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">{{ $t('common.home') }}</el-breadcrumb-item>
            <el-breadcrumb-item>{{ $t($route.meta.title) }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <!-- 扫描引导 -->
          <el-tooltip :content="$t('onboarding.scanGuideBtn')" placement="bottom">
            <div class="scan-guide-btn" @click="showOnboarding = true">
              <el-icon>
                <Aim />
              </el-icon>
            </div>
          </el-tooltip>
          <!-- 语言切换 -->
          <LanguageSwitcher />
          <!-- 主题切换 -->
          <ThemeSwitcher />
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-avatar :size="32" :src="userStore.avatarSrc" />
              <span class="username">{{ userStore.username }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">{{ $t('auth.personalCenter') }}</el-dropdown-item>
                <el-dropdown-item divided command="logout">{{ $t('auth.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 主内容区 -->
      <el-main class="main">
        <router-view v-slot="{ Component }">
          <transition name="fade-transform" mode="out-in">
            <component :is="Component" :key="$route.path" />
          </transition>
        </router-view>
      </el-main>

      <!-- 扫描引导弹窗（首次登录自动弹出 + 顶栏按钮手动唤起）-->
      <OnboardingGuide v-if="showOnboarding" @finished="showOnboarding = false" />
      <!-- 首次登录进入系统后提示修改密码 -->
      <FirstLoginResetDialog />
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '@/stores/user'
import { useThemeStore } from '@/stores/theme'
import { useBrandingStore } from '@/stores/branding'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import ThemeSwitcher from '@/components/ThemeSwitcher.vue'
import OnboardingGuide from '@/components/OnboardingGuide.vue'
import FirstLoginResetDialog from '@/components/FirstLoginResetDialog.vue'
import { getOnboardingStatus } from '@/api/auth'
import { shouldShowOnboarding } from '@/utils/onboarding'
import { Setting, Monitor, List, Search, Aim, Odometer, Stamp, Connection, Fold, Expand, Key, OfficeBuilding, Bell, User, Document, CircleClose, Warning, Timer, DataAnalysis, View, Picture, MagicStick, Operation } from '@element-plus/icons-vue'

const router = useRouter()
const { t } = useI18n()
const userStore = useUserStore()
const themeStore = useThemeStore()
const brandingStore = useBrandingStore()
const isCollapse = ref(false)
const defaultOpeneds = ref(['scan-config-menu', 'system-management'])

// === 扫描引导：首次登录自动弹出，顶栏按钮可手动唤起 ===
const showOnboarding = ref(false)
async function checkOnboarding() {
  try {
    const res = await getOnboardingStatus()
    if (res && res.code === 0 && shouldShowOnboarding(res)) {
      showOnboarding.value = true
    }
  } catch (e) {
    // 引导检查失败不应阻塞主界面
  }
}

onMounted(() => {
  // 刷新当前登录用户信息（头像、邮箱等可能在其他会话中已变更）
  userStore.refreshProfile()
  // 首次登录自动弹出扫描引导
  checkOnboarding()
})

function handleCommand(command) {
  if (command === 'logout') {
    userStore.logout()
    router.push('/login')
  } else if (command === 'profile') {
    router.push('/profile')
  }
}
</script>

<style lang="scss" scoped>
.layout-container {
  height: 100vh;
  display: flex;
}

.aside {
  background: hsl(var(--sidebar));
  color: hsl(var(--sidebar-foreground));
  transition: width 0.3s ease; // 只有宽度过渡，使用简单的ease
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  border-right: 1px solid hsl(var(--sidebar-border));
  display: flex;
  flex-direction: column;
  flex-shrink: 0;

  .logo {
    min-height: 64px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    color: hsl(var(--sidebar-foreground));
    font-size: 16px;
    font-weight: 600;
    letter-spacing: 1px;
    border-bottom: 1px solid hsl(var(--sidebar-border));
    flex-shrink: 0;

    img {
      width: 36px;
      height: 36px;
      border-radius: 6px;
      background: transparent;
      flex-shrink: 0;
      object-fit: contain;
    }

    span {
      max-width: 100%;
      text-align: center;
      line-height: 1.25;
      word-break: break-word;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }
  }

  .menu-wrapper {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;

    &::-webkit-scrollbar {
      width: 4px;
    }

    &::-webkit-scrollbar-thumb {
      background: hsl(var(--sidebar-border));
      border-radius: 2px;
    }
  }

  .menu-divider {
    height: 1px;
    background: hsl(var(--sidebar-border));
    margin: 8px 16px;
  }

  .el-menu {
    border-right: none;
    background: transparent !important;

    .el-menu-item {
      margin: 2px 8px;
      border-radius: 8px;
      height: 40px;
      line-height: 40px;
      color: hsl(var(--sidebar-foreground));
      display: flex;
      align-items: center;
      padding: 0 12px !important; // 使用padding而不是复杂的定位
      overflow: hidden;
      white-space: nowrap;
      position: relative;

      .el-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        margin-right: 12px; // 图标和文字之间的间距
      }

      span {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        flex: 1;
      }

      &:hover {
        background: hsl(var(--sidebar-accent)) !important;
        color: hsl(var(--sidebar-accent-foreground)) !important;
      }

      &.is-active {
        background: hsl(var(--sidebar-primary) / 0.18) !important;
        color: hsl(var(--sidebar-primary)) !important;
        font-weight: 600;
        box-shadow: none;
      }
    }

    .el-sub-menu {
      .el-sub-menu__title {
        margin: 2px 8px;
        border-radius: 8px;
        height: 40px;
        line-height: 40px;
        color: hsl(var(--sidebar-foreground));
        display: flex;
        align-items: center;
        padding: 0 12px !important; // 使用padding而不是复杂的定位
        overflow: hidden;
        white-space: nowrap;
        position: relative;

        .el-icon {
          font-size: 18px;
          width: 18px;
          height: 18px;
          display: flex;
          align-items: center;
          justify-content: center;
          flex-shrink: 0;
          margin-right: 12px; // 图标和文字之间的间距
        }

        span {
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
          flex: 1;
        }

        &:hover {
          background: hsl(var(--sidebar-accent)) !important;
          color: hsl(var(--sidebar-accent-foreground)) !important;
        }
      }

      &.is-opened>.el-sub-menu__title {
        color: hsl(var(--sidebar-foreground));
      }

      .el-menu {
        background: transparent !important;

        .el-menu-item {
          padding-left: 44px !important;
          min-width: auto;
          height: 36px;
          line-height: 36px;
          font-size: 13px;

          .el-icon {
            margin-right: 8px;
          }
        }
      }
    }

    // 收起状态：让Element Plus处理，只调整必要的样式
    &.el-menu--collapse {
      .el-menu-item {
        margin: 2px 0;
        justify-content: center;
        padding: 0 !important;
      }

      .el-sub-menu {
        .el-sub-menu__title {
          margin: 2px 0;
          justify-content: center;
          padding: 0 !important;
        }
      }
    }
  }

}

// 简化的深度选择器，只处理必要的样式覆盖
:deep(.el-menu) {

  .el-menu-item,
  .el-sub-menu .el-sub-menu__title {

    // 重置所有可能的隐藏样式
    .el-icon {
      display: flex !important;
      visibility: visible !important;
      opacity: 1 !important;
    }

    span {
      display: block !important;
      visibility: visible !important;
      opacity: 1 !important;
    }
  }
}

.header {
  background: hsl(var(--background));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 64px;
  border-bottom: 1px solid hsl(var(--border));
  transition: background 0.3s;

  .header-left {
    display: flex;
    align-items: center;

    .collapse-btn {
      font-size: 20px;
      cursor: pointer;
      margin-right: 20px;
      color: hsl(var(--muted-foreground));
      transition: color 0.3s;

      &:hover {
        color: hsl(var(--primary));
      }
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 16px;

    .theme-switch {
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

    .scan-guide-btn {
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

    .user-info {
      display: flex;
      align-items: center;
      cursor: pointer;
      padding: 4px 8px;
      border-radius: 8px;
      transition: background 0.3s;

      &:hover {
        background: hsl(var(--accent));
      }

      .username {
        margin-left: 8px;
        color: hsl(var(--foreground));
      }
    }
  }
}

.main {
  background: hsl(var(--background));
  padding: 20px;
  overflow-y: auto;
  overflow-x: hidden;
  transition: background 0.3s;
  flex: 1;
  width: 100%;
  margin: 0 auto;

  /* 隐藏滚动条 */
  &::-webkit-scrollbar {
    display: none;
  }

  -ms-overflow-style: none;
  scrollbar-width: none;
}

/* fade-transform 动画 */
.fade-transform-leave-active,
.fade-transform-enter-active {
  transition: all 0.1s ease-out;
}

.fade-transform-enter-from {
  opacity: 0;
  transform: translateX(-10px);
}

.fade-transform-leave-to {
  opacity: 0;
  transform: translateX(10px);
}
</style>
