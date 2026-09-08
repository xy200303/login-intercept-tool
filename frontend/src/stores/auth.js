import { defineStore } from 'pinia'
import { login as apiLogin } from '../api'

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
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('fenx_token')
      localStorage.removeItem('fenx_user')
    },
  },
})
