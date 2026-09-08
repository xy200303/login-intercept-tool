import { defineStore } from 'pinia'
import { login as apiLogin, logoutSession } from '../api'
import { registerTokenUpdater } from '../api/client'

const readUser = () => {
  try {
    return JSON.parse(localStorage.getItem('fenx_user') || 'null')
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('fenx_token') || '',
    user: readUser(),
  }),
  getters: {
    isLoggedIn: (state) => Boolean(state.token),
    role: (state) => state.user?.role || '',
    isSuperAdmin() {
      return this.role === 'super_admin' || this.role === 'superadmin'
    },
    isOperator() {
      return this.role === 'operator'
    },
    isAgent() {
      return this.role === 'agent'
    },
    roleLabel() {
      if (this.isSuperAdmin) return '超级管理员'
      if (this.isOperator) return '运营管理员'
      if (this.isAgent) return '代理账号'
      return this.role || '未知角色'
    },
    homePath() {
      return this.isAgent ? '/agent/overview' : '/overview'
    },
  },
  actions: {
    async login(username, password) {
      const data = await apiLogin(username, password)
      this.token = data.access_token
      this.user = data.user
      localStorage.setItem('fenx_token', data.access_token)
      localStorage.setItem('fenx_user', JSON.stringify(data.user))
      // 降级情况（平台库不可用）可能没有 refresh_token，兼容处理
      if (data.refresh_token) localStorage.setItem('fenx_refresh', data.refresh_token)
    },
    logout() {
      const refreshToken = localStorage.getItem('fenx_refresh')
      // fire-and-forget：撤销失败不影响本地登出
      if (refreshToken) logoutSession(refreshToken).catch(() => {})
      this.token = ''
      this.user = null
      localStorage.removeItem('fenx_token')
      localStorage.removeItem('fenx_user')
      localStorage.removeItem('fenx_refresh')
    },
  },
})

// 静默刷新成功后把新 token 同步进 store；清空时同步登出
registerTokenUpdater((data) => {
  const store = useAuthStore()
  if (data) {
    store.token = data.access_token
    if (data.user) store.user = data.user
  } else {
    store.token = ''
    store.user = null
  }
})
