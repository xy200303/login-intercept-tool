import client from './client'

// 认证
export const login = (username, password) => client.post('/auth/login', { username, password })
export const logoutSession = (refreshToken) => client.post('/auth/logout', { refresh_token: refreshToken })
export const fetchMe = () => client.get('/me')
export const changePassword = (oldPassword, newPassword) =>
  client.post('/auth/change-password', { old_password: oldPassword, new_password: newPassword })

// 代理与监控任务
export const listAgents = () => client.get('/agents')
export const createAgent = (payload) => client.post('/agents', payload)
export const listTasks = () => client.get('/monitor-tasks')
export const createTask = (payload) => client.post('/monitor-tasks', payload)
export const runTask = (id) => client.post(`/monitor-tasks/${id}/run`)
export const listSyncRuns = () => client.get('/sync-runs')
export const detectAgent = (id) => client.post(`/agents/${id}/detect`)

// 数据与关联
export const listSnapshots = (params) => client.get('/kkud/snapshots', { params })
export const listMatches = () => client.get('/matches')
export const listConflicts = (params) => client.get('/conflicts', { params })
export const conflictDetail = (id) => client.get(`/conflicts/${id}`)
export const previewConflict = (id) => client.post(`/conflicts/${id}/preview`)
export const executeConflict = (id, idempotencyKey) =>
  client.post(`/conflicts/${id}/actions`, { confirm: true }, { headers: { 'Idempotency-Key': idempotencyKey } })
export const retryAction = (id) => client.post(`/actions/${id}/retry`)
export const undoDisable = (id) => client.post(`/actions/${id}/undo-disable`)

// 管理员数据管理
export const updateVip = (sourceId, vip) => client.patch(`/kkud/users/${encodeURIComponent(sourceId)}/vip`, { vip })
export const searchFenxUsers = (params) => client.get('/fenx/users', { params })
export const getFenxUserMeta = () => client.get('/fenx/users/meta')
export const updateFenxUser = (uid, patch) => client.patch(`/fenx/users/${uid}`, patch)
export const resetFenxUserPassword = (uid, password) => client.post(`/fenx/users/${uid}/password`, { password })
export const disableFenxUser = (uid) => client.post(`/fenx/users/${uid}/disable`)
export const enableFenxUser = (uid) => client.post(`/fenx/users/${uid}/enable`)
export const deleteFenxUser = (uid) => client.delete(`/fenx/users/${uid}`)

// kkud 用户直连管理（仅超管）
export const listKkudUsers = (params) => client.get('/kkud/users', { params })
export const createKkudUser = (payload) => client.post('/kkud/users', payload)
export const updateKkudUser = (id, patch) => client.patch(`/kkud/users/${id}`, patch)
export const deleteKkudUser = (id) => client.delete(`/kkud/users/${id}`)

// 外连数据库设置（仅超管）
export const getExternalDbSettings = () => client.get('/settings/external-db')
export const updateExternalDbSettings = (payload) => client.put('/settings/external-db', payload)

// 审计 / 白名单 / 平台账号
export const listAuditEvents = (params) => client.get('/audit-events', { params })
export const listAllowlist = () => client.get('/allowlist')
export const createAllowlist = (payload) => client.post('/allowlist', payload)
export const deleteAllowlist = (id) => client.delete(`/allowlist/${id}`)
export const listUsers = () => client.get('/users')
export const createUser = (payload) => client.post('/users', payload)

// 系统
// body 可选：传 {kkud:{...}} / {fenx:{...}} 测表单未保存值，不传测已保存配置
export const testConnections = (body) => client.post('/connections/test', body || undefined)
export const fetchHealth = () => client.get('/health')
