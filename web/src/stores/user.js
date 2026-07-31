import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi, getUserList, getUserProfile } from '@/api/auth'

// 默认头像路径
export const DEFAULT_AVATAR = '/default-avatar.jpg'

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const userId = ref(localStorage.getItem('userId') || '')
  const username = ref(localStorage.getItem('username') || '')
  const role = ref(localStorage.getItem('role') || '')
  const workspaceId = ref(localStorage.getItem('workspaceId') || '')
  const avatar = ref(localStorage.getItem('avatar') || '')
  const profile = ref({
    email: '',
    phone: '',
    status: '',
    lastLoginTime: 0,
    createTime: 0
  })

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => role.value === 'admin' || role.value === 'superadmin')
  const avatarSrc = computed(() => avatar.value || DEFAULT_AVATAR)

  async function login(loginForm) {
    const res = await loginApi(loginForm)
    if (res.code === 0) {
      token.value = res.token
      userId.value = res.userId
      username.value = res.username
      role.value = res.role
      workspaceId.value = res.workspaceId || ''

      localStorage.setItem('token', res.token)
      localStorage.setItem('userId', res.userId)
      localStorage.setItem('username', res.username)
      localStorage.setItem('role', res.role)
      localStorage.setItem('workspaceId', res.workspaceId || '')
      localStorage.removeItem('currentWorkspaceId')

      await refreshProfile()
    }
    return res
  }

  // refreshProfile 拉取当前用户个人信息（含头像、邮箱、电话、登录时间）
  async function refreshProfile() {
    if (!token.value) return
    try {
      const res = await getUserProfile()
      if (res.code === 0) {
        setAvatar(res.avatar || '')
        setUsername(res.username || username.value)
        if (res.role) {
          role.value = res.role
          localStorage.setItem('role', res.role)
        }
        profile.value = {
          email: res.email || '',
          phone: res.phone || '',
          status: res.status || '',
          lastLoginTime: res.lastLoginTime || 0,
          createTime: res.createTime || 0
        }
      }
    } catch (e) {
      // ignore
    }
  }

  // 旧入口保留：仅刷新头像（向后兼容 Settings.vue 等调用点）
  async function refreshAvatar() {
    return refreshProfile()
  }

  function setAvatar(url) {
    avatar.value = url || ''
    if (url) {
      localStorage.setItem('avatar', url)
    } else {
      localStorage.removeItem('avatar')
    }
  }

  function setUsername(name) {
    if (!name) return
    username.value = name
    localStorage.setItem('username', name)
  }

  function setProfile(partial) {
    profile.value = { ...profile.value, ...partial }
  }

  function logout() {
    token.value = ''
    userId.value = ''
    username.value = ''
    role.value = ''
    workspaceId.value = ''
    avatar.value = ''
    profile.value = { email: '', phone: '', status: '', lastLoginTime: 0, createTime: 0 }

    localStorage.removeItem('token')
    localStorage.removeItem('userId')
    localStorage.removeItem('username')
    localStorage.removeItem('role')
    localStorage.removeItem('workspaceId')
    localStorage.removeItem('avatar')
  }

  function setWorkspace(id) {
    workspaceId.value = id
    localStorage.setItem('workspaceId', id)
  }

  return {
    token,
    userId,
    username,
    role,
    workspaceId,
    avatar,
    avatarSrc,
    profile,
    isLoggedIn,
    isAdmin,
    login,
    logout,
    setWorkspace,
    setAvatar,
    setUsername,
    setProfile,
    refreshProfile,
    refreshAvatar
  }
})
