import axios from 'axios'

const client = axios.create({
  baseURL: `${import.meta.env.VITE_API_BASE || ''}/api/v1`,
  timeout: 30000,
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('fenx_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

client.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('fenx_token')
      localStorage.removeItem('fenx_user')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    const message = error.response?.data?.message || error.message || '请求失败，请稍后重试'
    throw new Error(message)
  }
)

export default client
