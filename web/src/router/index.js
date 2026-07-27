import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'

// 动态导入重试包装器，解决 chunk 加载失败问题
function lazyLoad(importFn) {
  return () => {
    return importFn().catch((err) => {
      // 如果是 chunk 加载失败，尝试刷新页面
      if (err.message.includes('Failed to fetch dynamically imported module') ||
          err.message.includes('Loading chunk') ||
          err.message.includes('Loading CSS chunk')) {
        console.warn('[Router] Chunk load failed, reloading page...', err)
        window.location.reload()
        return new Promise(() => {}) // 阻止后续执行
      }
      throw err
    })
  }
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
        meta: { title: '工作台', icon: 'Odometer' }
      },
      // ===== 资产管理（顶层资产 + 暴露面 + 风险，全部挂 /asset-management/*）=====
      {
        path: 'asset-management',
        name: 'AssetManagement',
        component: lazyLoad(() => import('@/views/AssetManagement.vue')),
        meta: { title: '资产概览', icon: 'DataAnalysis' }
      },
      {
        path: 'asset-management/space-search',
        name: 'AssetSpaceSearch',
        component: lazyLoad(() => import('@/views/AssetSpaceSearch.vue')),
        meta: { title: '资产空间搜索', icon: 'Search' }
      },
      // -------- 暴露面 --------
      {
        path: 'asset-management/exposure/screenshot',
        name: 'ExposureScreenshot',
        component: lazyLoad(() => import('@/views/Screenshots.vue')),
        meta: { title: '截图', icon: 'Picture' }
      },
      {
        path: 'asset-management/exposure/subdomain',
        name: 'ExposureSubdomain',
        component: lazyLoad(() => import('@/views/Domain.vue')),
        meta: { title: '子域名', icon: 'Link' }
      },
      {
        path: 'asset-management/exposure/ip',
        name: 'ExposureIp',
        component: lazyLoad(() => import('@/views/IP.vue')),
        meta: { title: 'IP', icon: 'Position' }
      },
      {
        path: 'asset-management/exposure/port',
        name: 'ExposurePort',
        component: lazyLoad(() => import('@/views/AssetManagement/PortPage.vue')),
        meta: { title: '端口', icon: 'Connection' }
      },
      {
        path: 'asset-management/exposure/site',
        name: 'ExposureSite',
        component: lazyLoad(() => import('@/views/Site.vue')),
        meta: { title: '站点', icon: 'Monitor' }
      },
      {
        path: 'asset-management/exposure/icon',
        name: 'ExposureIcon',
        component: lazyLoad(() => import('@/views/AssetManagement/IconPage.vue')),
        meta: { title: 'Icon', icon: 'Picture' }
      },
      {
        path: 'asset-management/exposure/app',
        name: 'ExposureApp',
        component: lazyLoad(() => import('@/views/AssetManagement/AppPage.vue')),
        meta: { title: '应用', icon: 'Grid' }
      },
      {
        path: 'asset-management/exposure/dir',
        name: 'ExposureDir',
        component: lazyLoad(() => import('@/views/DirectoryManagement.vue')),
        meta: { title: '目录', icon: 'Folder' }
      },
      {
        path: 'asset-management/exposure/js',
        name: 'ExposureJs',
        component: lazyLoad(() => import('@/views/AssetManagement/JSFinderPage.vue')),
        meta: { title: 'JS', icon: 'Document' }
      },
      // -------- 风险 --------
      {
        path: 'asset-management/risk/sensitive-dir',
        name: 'RiskSensitiveDir',
        component: lazyLoad(() => import('@/views/AssetManagement/SensitiveDirPage.vue')),
        meta: { title: '敏感目录/文件', icon: 'FolderOpened' }
      },
      {
        path: 'asset-management/risk/vuln',
        name: 'RiskVuln',
        component: lazyLoad(() => import('@/views/VulnerabilityManagement.vue')),
        meta: { title: '漏洞', icon: 'Warning' }
      },
      {
        path: 'task/create',
        name: 'TaskCreate',
        component: lazyLoad(() => import('@/views/TaskCreate.vue')),
        meta: { title: '新建任务', icon: 'List', hidden: true }
      },
      {
        path: 'task/edit/:id',
        name: 'TaskEdit',
        component: lazyLoad(() => import('@/views/TaskCreate.vue')),
        meta: { title: '编辑任务', icon: 'List', hidden: true }
      },
      {
        path: 'task',
        name: 'Task',
        component: lazyLoad(() => import('@/views/Task.vue')),
        meta: { title: '任务管理', icon: 'List' }
      },
      {
        path: 'task/template',
        name: 'ScanTemplate',
        component: lazyLoad(() => import('@/views/ScanTemplate.vue')),
        meta: { title: '扫描模板', icon: 'Document', hidden: true }
      },
      {
        path: 'cron-task',
        name: 'CronTask',
        component: lazyLoad(() => import('@/views/CronTask.vue')),
        meta: { title: '定时扫描', icon: 'Timer' }
      },

      {
        path: 'online-search',
        name: 'OnlineSearch',
        component: lazyLoad(() => import('@/views/OnlineSearch.vue')),
        meta: { title: '在线搜索', icon: 'Search' }
      },
      {
        path: 'workspace',
        name: 'Workspace',
        redirect: '/settings?tab=workspace',
        meta: { title: '工作空间', icon: 'Folder', hidden: true }
      },
      {
        path: 'worker',
        name: 'Worker',
        component: lazyLoad(() => import('@/views/Worker.vue')),
        meta: { title: 'Worker节点', icon: 'Connection' }
      },
      {
        path: 'worker-logs',
        name: 'WorkerLogs',
        component: lazyLoad(() => import('@/views/WorkerLogs.vue')),
        meta: { title: '容器日志', icon: 'Document' }
      },
      {
        path: 'blacklist',
        name: 'Blacklist',
        component: lazyLoad(() => import('@/views/Blacklist.vue')),
        meta: { title: '全局黑名单', icon: 'CircleClose' }
      },
      {
        path: 'high-risk-filter',
        name: 'HighRiskFilter',
        component: lazyLoad(() => import('@/views/HighRiskFilter.vue')),
        meta: { title: '高危过滤配置', icon: 'Warning' }
      },
      {
        path: 'worker/console/:name',
        name: 'WorkerConsole',
        component: lazyLoad(() => import('@/views/WorkerConsole.vue')),
        meta: { title: 'Worker控制台', icon: 'Monitor', hidden: true }
      },
      {
        path: 'poc',
        name: 'Poc',
        component: lazyLoad(() => import('@/views/Poc.vue')),
        meta: { title: 'POC管理', icon: 'Aim' }
      },
      {
        path: 'fingerprint',
        name: 'Fingerprint',
        component: lazyLoad(() => import('@/views/Fingerprint.vue')),
        meta: { title: '指纹管理', icon: 'Stamp' }
      },
      {
        path: 'report',
        name: 'Report',
        component: lazyLoad(() => import('@/views/Report.vue')),
        meta: { title: '扫描报告', icon: 'Document', hidden: true }
      },
      {
        path: 'user',
        name: 'User',
        redirect: '/settings?tab=user',
        meta: { title: '用户管理', icon: 'User', roles: ['superadmin'], hidden: true }
      },
      {
        path: 'organization',
        name: 'Organization',
        redirect: '/settings?tab=organization',
        meta: { title: '组织管理', icon: 'OfficeBuilding', hidden: true }
      },
      {
        path: 'settings',
        name: 'Settings',
        component: lazyLoad(() => import('@/views/Settings.vue')),
        meta: { title: '系统配置', icon: 'Setting' }
      },
      {
        path: 'profile',
        name: 'Profile',
        component: lazyLoad(() => import('@/views/Profile.vue')),
        meta: { title: '个人中心', icon: 'User', hidden: true }
      },
      {
        path: 'api-docs',
        name: 'ApiDocs',
        component: lazyLoad(() => import('@/views/ApiDocs.vue')),
        meta: { title: '接口管理', icon: 'Document', roles: ['admin', 'superadmin'] }
      },
      {
        path: 'ai-config',
        name: 'AIConfig',
        component: lazyLoad(() => import('@/views/AIConfig.vue')),
        meta: { title: 'AI配置', icon: 'MagicStick', roles: ['admin', 'superadmin'] }
      },
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const userStore = useUserStore()

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

export default router
