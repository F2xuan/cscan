import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'

// 动态导入重试包装器，解决 chunk 加载失败问题
// 返回纯异步函数（非 defineAsyncComponent），避免 Vue Router 警告
function lazyLoad(importFn) {
  return () => importFn().catch((err) => {
    if (err.message.includes('Failed to fetch dynamically imported module') ||
        err.message.includes('Loading chunk') ||
        err.message.includes('Loading CSS chunk')) {
      console.warn('[Router] Chunk load failed, reloading page...', err)
      window.location.reload()
      return new Promise(() => {})
    }
    throw err
  })
}

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: lazyLoad(() => import('@/views/Login.vue')),
    meta: { requiresAuth: false }
  },
  {
    path: '/',
    component: lazyLoad(() => import('@/layouts/MainLayout.vue')),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: lazyLoad(() => import('@/views/Dashboard.vue')),
        meta: { title: 'menu.Dashboard', icon: 'Odometer' }
      },
      // ===== 资产管理（顶层资产 + 暴露面 + 风险，全部挂 /asset-management/*）=====
      {
        path: 'asset-management',
        name: 'AssetManagement',
        component: lazyLoad(() => import('@/views/AssetManagement.vue')),
        meta: { title: 'menu.AssetManagement', icon: 'DataAnalysis' }
      },
      {
        path: 'asset-management/space-search',
        name: 'AssetSpaceSearch',
        component: lazyLoad(() => import('@/views/AssetSpaceSearch.vue')),
        meta: { title: 'menu.AssetSpaceSearch', icon: 'Search' }
      },
      // -------- 暴露面 --------
      {
        path: 'asset-management/exposure/screenshot',
        name: 'ExposureScreenshot',
        component: lazyLoad(() => import('@/views/Screenshots.vue')),
        meta: { title: 'menu.ExposureScreenshot', icon: 'Picture' }
      },
      {
        path: 'asset-management/exposure/subdomain',
        name: 'ExposureSubdomain',
        component: lazyLoad(() => import('@/views/Domain.vue')),
        meta: { title: 'menu.ExposureSubdomain', icon: 'Link' }
      },
      {
        path: 'asset-management/exposure/ip',
        name: 'ExposureIp',
        component: lazyLoad(() => import('@/views/IP.vue')),
        meta: { title: 'menu.ExposureIp', icon: 'Position' }
      },
      {
        path: 'asset-management/exposure/port',
        name: 'ExposurePort',
        component: lazyLoad(() => import('@/views/AssetManagement/PortPage.vue')),
        meta: { title: 'menu.ExposurePort', icon: 'Connection' }
      },
      {
        path: 'asset-management/exposure/site',
        name: 'ExposureSite',
        component: lazyLoad(() => import('@/views/Site.vue')),
        meta: { title: 'menu.ExposureSite', icon: 'Monitor' }
      },
      {
        path: 'asset-management/exposure/icon',
        name: 'ExposureIcon',
        component: lazyLoad(() => import('@/views/AssetManagement/IconPage.vue')),
        meta: { title: 'menu.ExposureIcon', icon: 'Picture' }
      },
      {
        path: 'asset-management/exposure/app',
        name: 'ExposureApp',
        component: lazyLoad(() => import('@/views/AssetManagement/AppPage.vue')),
        meta: { title: 'menu.ExposureApp', icon: 'Grid' }
      },
      {
        path: 'asset-management/exposure/dir',
        name: 'ExposureDir',
        component: lazyLoad(() => import('@/views/DirectoryManagement.vue')),
        meta: { title: 'menu.ExposureDir', icon: 'Folder' }
      },
      {
        path: 'asset-management/exposure/js',
        name: 'ExposureJs',
        component: lazyLoad(() => import('@/views/AssetManagement/JSFinderPage.vue')),
        meta: { title: 'menu.ExposureJs', icon: 'Document' }
      },
      // -------- 风险 --------
      {
        path: 'asset-management/risk/sensitive-info',
        name: 'RiskSensitiveInfo',
        component: lazyLoad(() => import('@/views/AssetManagement/SensitiveInfoPage.vue')),
        meta: { title: 'menu.RiskSensitiveInfo', icon: 'Warning' }
      },
      {
        path: 'asset-management/risk/vuln',
        name: 'RiskVuln',
        component: lazyLoad(() => import('@/views/VulnerabilityManagement.vue')),
        meta: { title: 'menu.RiskVuln', icon: 'Warning' }
      },
      {
        path: 'asset-management/fingerprint/cert',
        name: 'CertAsset',
        component: lazyLoad(() => import('@/views/CertAsset.vue')),
        meta: { title: 'menu.CertAsset', icon: 'Lock' }
      },
      {
        path: 'task/create',
        name: 'TaskCreate',
        component: lazyLoad(() => import('@/views/TaskCreate.vue')),
        meta: { title: 'menu.TaskCreate', icon: 'List', hidden: true }
      },
      // BUG-003 修复：添加旧路径重定向，兼容旧书签和外部链接
      {
        path: 'task-create',
        redirect: '/task/create'
      },
      {
        path: 'task/edit/:id',
        name: 'TaskEdit',
        component: lazyLoad(() => import('@/views/TaskCreate.vue')),
        meta: { title: 'menu.TaskEdit', icon: 'List', hidden: true }
      },
      {
        path: 'task/detail',
        name: 'TaskDetail',
        component: lazyLoad(() => import('@/views/TaskDetail.vue')),
        meta: { title: 'menu.TaskDetail', icon: 'List', hidden: true }
      },
      {
        path: 'task',
        name: 'Task',
        component: lazyLoad(() => import('@/views/Task.vue')),
        meta: { title: 'menu.Task', icon: 'List' }
      },
      {
        path: 'task/template',
        name: 'ScanTemplate',
        component: lazyLoad(() => import('@/views/ScanTemplate.vue')),
        meta: { title: 'menu.ScanTemplate', icon: 'Document', hidden: true }
      },
      {
        path: 'cron-task',
        name: 'CronTask',
        component: lazyLoad(() => import('@/views/CronTask.vue')),
        meta: { title: 'menu.CronTask', icon: 'Timer' }
      },
      {
        path: 'cron-task/create',
        name: 'CronTaskCreate',
        component: lazyLoad(() => import('@/views/CronTaskCreate.vue')),
        meta: { title: 'menu.CronTaskCreate', icon: 'Timer', hidden: true }
      },
      {
        path: 'cron-task/edit/:id',
        name: 'CronTaskEdit',
        component: lazyLoad(() => import('@/views/CronTaskCreate.vue')),
        meta: { title: 'menu.CronTaskEdit', icon: 'Timer', hidden: true }
      },

      // ===== 空间引擎（/space-engine/*）=====
      {
        path: 'space-engine/online-search',
        name: 'SpaceEngineOnlineSearch',
        component: lazyLoad(() => import('@/views/OnlineSearch.vue')),
        meta: { title: 'menu.SpaceEngineOnlineSearch', icon: 'Search' }
      },
      {
        path: 'space-engine/api-config',
        name: 'SpaceEngineApiConfig',
        component: lazyLoad(() => import('@/views/space-engine/ApiConfig.vue')),
        meta: { title: 'menu.SpaceEngineApiConfig', icon: 'Key' }
      },
      {
        path: 'space-engine/cron-task',
        name: 'SpaceEngineCronTask',
        component: lazyLoad(() => import('@/views/space-engine/CronTask.vue')),
        meta: { title: 'menu.SpaceEngineCronTask', icon: 'Timer' }
      },

      // 旧路径重定向
      {
        path: 'online-search',
        redirect: '/space-engine/online-search'
      },
      {
        path: 'worker',
        name: 'Worker',
        component: lazyLoad(() => import('@/views/Worker.vue')),
        meta: { title: 'menu.Worker', icon: 'Connection' }
      },
      {
        path: 'worker-logs',
        name: 'WorkerLogs',
        component: lazyLoad(() => import('@/views/WorkerLogs.vue')),
        meta: { title: 'menu.WorkerLogs', icon: 'Document' }
      },
      {
        path: 'blacklist',
        name: 'Blacklist',
        component: lazyLoad(() => import('@/views/Blacklist.vue')),
        meta: { title: 'menu.Blacklist', icon: 'CircleClose' }
      },
      {
        path: 'high-risk-filter',
        name: 'HighRiskFilter',
        component: lazyLoad(() => import('@/views/HighRiskFilter.vue')),
        meta: { title: 'menu.HighRiskFilter', icon: 'Warning' }
      },
      {
        path: 'poc',
        name: 'Poc',
        component: lazyLoad(() => import('@/views/Poc.vue')),
        meta: { title: 'menu.Poc', icon: 'Aim' }
      },
      {
        path: 'fingerprint',
        name: 'Fingerprint',
        component: lazyLoad(() => import('@/views/Fingerprint.vue')),
        meta: { title: 'menu.Fingerprint', icon: 'Stamp' }
      },
      {
        path: 'report',
        name: 'Report',
        component: lazyLoad(() => import('@/views/Report.vue')),
        meta: { title: 'menu.Report', icon: 'Document', hidden: true }
      },
      {
        path: 'user',
        name: 'User',
        component: lazyLoad(() => import('@/views/settings/UserManagement.vue')),
        meta: { title: 'menu.User', icon: 'User', roles: ['admin', 'superadmin'], hidden: true }
      },
      {
        path: 'organization',
        name: 'Organization',
        component: lazyLoad(() => import('@/views/settings/OrganizationManagement.vue')),
        meta: { title: 'menu.Organization', icon: 'OfficeBuilding', hidden: true }
      },
      // 旧 /settings?tab=* 入口按 tab 重定向至独立页面
      {
        path: 'settings',
        redirect: (to) => {
          const map = {
            onlineapi: '/space-engine/api-config',
            subfinder: '/settings-subfinder',
            notify: '/settings-notify',
            reverify: '/settings-reverify',
            user: '/user',
            organization: '/organization',
            branding: '/settings-branding'
          }
          return map[to.query.tab] || '/settings-branding'
        }
      },
      {
        path: 'settings-subfinder',
        name: 'SubfinderConfig',
        component: lazyLoad(() => import('@/views/settings/SubfinderConfig.vue')),
        meta: { title: 'menu.subdomainConfig', icon: 'Search' }
      },
      {
        path: 'settings-notify',
        name: 'NotifyConfig',
        component: lazyLoad(() => import('@/views/settings/NotifyConfig.vue')),
        meta: { title: 'menu.notifyConfig', icon: 'Bell' }
      },
      {
        path: 'settings-reverify',
        name: 'ReverifyConfig',
        component: lazyLoad(() => import('@/views/settings/ReverifyConfig.vue')),
        meta: { title: 'menu.reverifyConfig', icon: 'Timer' }
      },
      {
        path: 'settings-branding',
        name: 'BrandingConfig',
        component: lazyLoad(() => import('@/views/settings/BrandingConfig.vue')),
        meta: { title: 'menu.brandingConfig', icon: 'Picture', roles: ['admin', 'superadmin'] }
      },
      {
        path: 'profile',
        name: 'Profile',
        component: lazyLoad(() => import('@/views/Profile.vue')),
        meta: { title: 'menu.Profile', icon: 'User', hidden: true }
      },
      {
        path: 'ai-config',
        name: 'AIConfig',
        component: lazyLoad(() => import('@/views/AIConfig.vue')),
        meta: { title: 'menu.AIConfig', icon: 'MagicStick', roles: ['admin', 'superadmin'] }
      },
    ]
  },
  // 404 兜底路由：未匹配的路径重定向到 Dashboard
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    redirect: '/dashboard'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const userStore = useUserStore()

  // 路由切换时显示顶部加载进度条（修复 BUG #4：懒加载 chunk 首次加载空白）
  if (to.name !== from.name) {
    document.documentElement.classList.add('app-route-loading')
  }

  if (to.meta.requiresAuth !== false && !userStore.token) {
    next('/login')
  } else if (to.path === '/login' && userStore.token) {
    next('/dashboard')
  } else if (to.meta.roles && !to.meta.roles.includes(userStore.role)) {
    // 角色不匹配：拦截直接输入 URL 的越权访问（后端亦有对应校验）
    next('/dashboard')
  } else {
    next()
  }
})

// 路由加载完成后隐藏顶部进度条
router.afterEach(() => {
  setTimeout(() => {
    document.documentElement.classList.remove('app-route-loading')
  }, 300)
})

export default router
