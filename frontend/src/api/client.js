import axios from 'axios'

const baseURL = `${import.meta.env.VITE_API_BASE || ''}/api/v1`

const client = axios.create({ baseURL, timeout: 30000 })

// auth store 注册的回调：刷新/清空 token 时同步内存状态（client 不反向依赖 store，避免循环引用）
let tokenUpdater = null
export const registerTokenUpdater = (fn) => { tokenUpdater = fn }

function applyTokens(data) {
  localStorage.setItem('fenx_token', data.access_token)
  if (data.refresh_token) localStorage.setItem('fenx_refresh', data.refresh_token)
  tokenUpdater?.(data)
}

export function clearSession() {
  localStorage.removeItem('fenx_token')
  localStorage.removeItem('fenx_user')
  localStorage.removeItem('fenx_refresh')
  tokenUpdater?.(null)
}

function redirectLogin() {
  if (window.location.pathname !== '/login') window.location.href = '/login'
}

// 全局单例：多个请求同时 401 只刷新一次，其余等待同一个 Promise
let refreshPromise = null
function refreshSession() {
  if (!refreshPromise) {
    const refreshToken = localStorage.getItem('fenx_refresh')
    refreshPromise = axios
      .post(`${baseURL}/auth/refresh`, { refresh_token: refreshToken }, { timeout: 15000 })
      .then((response) => {
        applyTokens(response.data)
        return response.data.access_token
      })
      .finally(() => { refreshPromise = null })
  }
  return refreshPromise
}

const isAuthCall = (config) =>
  ['/auth/login', '/auth/refresh', '/auth/logout'].some((path) => config?.url?.includes(path))

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('fenx_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

client.interceptors.response.use(
  (response) => response.data,
  async (error) => {
    const { response, config } = error
    if (response?.status === 401 && !isAuthCall(config) && !config._retried) {
      if (localStorage.getItem('fenx_refresh')) {
        config._retried = true
        try {
          const newToken = await refreshSession()
          config.headers = { ...config.headers, Authorization: `Bearer ${newToken}` }
          return client(config)
        } catch (refreshError) {
          // 仅 401（invalid_refresh_token）才强制重新登录；网络等错误保留会话，由用户重试
          if (refreshError.response?.status === 401) {
            clearSession()
            redirectLogin()
            throw new Error('登录已过期，请重新登录')
          }
          throw new Error(refreshError.response?.data?.message || '登录状态刷新失败，请稍后重试')
        }
      }
      clearSession()
      redirectLogin()
    }
    const message = response?.data?.message || error.message || '请求失败，请稍后重试'
    throw new Error(message)
  }
)

export default client
