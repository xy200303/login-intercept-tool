import client from './client'

// 认证
export const login = (username, password) => client.post('/auth/login', { username, password })
export const logoutSession = (refreshToken) => client.post('/auth/logout', { refresh_token: refreshToken })
export const fetchMe = () => client.get('/me')
export const changePassword = (oldPassword, newPassword) =>
  client.post('/auth/change-password', { old_password: oldPassword, new_password: newPassword })

// kkud 用户直连管理（仅超管）
export const listKkudUsers = (params) => client.get('/kkud/users', { params })
export const createKkudUser = (payload) => client.post('/kkud/users', payload)
export const updateKkudUser = (id, patch) => client.patch(`/kkud/users/${id}`, patch)
export const deleteKkudUser = (id) => client.delete(`/kkud/users/${id}`)
export const updateVip = (sourceId, vip) => client.patch(`/kkud/users/${encodeURIComponent(sourceId)}/vip`, { vip })

// fenx 用户管理
export const searchFenxUsers = (params) => client.get('/fenx/users', { params })
export const getFenxUserMeta = () => client.get('/fenx/users/meta')
export const updateFenxUser = (uid, patch) => client.patch(`/fenx/users/${uid}`, patch)
export const resetFenxUserPassword = (uid, password) => client.post(`/fenx/users/${uid}/password`, { password })
export const disableFenxUser = (uid) => client.post(`/fenx/users/${uid}/disable`)
export const enableFenxUser = (uid) => client.post(`/fenx/users/${uid}/enable`)
export const deleteFenxUser = (uid) => client.delete(`/fenx/users/${uid}`)

// fenx 登录 IP 记录
export const listFenxLoginLogs = (params) => client.get('/fenx/login-logs', { params })
export const deleteFenxLoginLog = (id) => client.delete(`/fenx/login-logs/${id}`)
export const clearFenxLoginLogIp = (ip) => client.post('/fenx/login-logs/clear-ip', { ip })

// 拦截记录（联盟站点 fenx_guard）
export const listGuardRecords = () => client.get('/guard/records')

// 审计
export const listAuditEvents = (params) => client.get('/audit-events', { params })

// 外连数据库 / 拦截记录接口设置（仅超管）
export const getExternalDbSettings = () => client.get('/settings/external-db')
export const updateExternalDbSettings = (payload) => client.put('/settings/external-db', payload)

// 系统
// body 可选：传 {kkud:{...}} / {fenx:{...}} 测表单未保存值，不传测已保存配置
export const testConnections = (body) => client.post('/connections/test', body || undefined)
export const fetchHealth = () => client.get('/health')
