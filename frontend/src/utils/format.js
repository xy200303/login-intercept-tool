export function formatDate(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function formatDuration(start, end) {
  if (!start || !end) return '—'
  const ms = new Date(end) - new Date(start)
  if (Number.isNaN(ms) || ms < 0) return '—'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m${Math.round((ms % 60000) / 1000)}s`
}

export function maskMobile(value) {
  const raw = String(value || '').trim()
  if (!raw) return '—'
  if (raw.includes('*')) return raw
  if (raw.length >= 7) return raw.slice(0, 3) + '****' + raw.slice(-4)
  return '****'
}

export function parseSourceValues(raw) {
  if (!raw) return []
  const text = String(raw).trim()
  if (text.startsWith('[')) {
    try {
      return JSON.parse(text)
    } catch {
      return [text]
    }
  }
  return text.split(/[,;\n\r]+/).map((item) => item.trim()).filter(Boolean)
}

export function policyLabel(raw) {
  let action = 'alert'
  if (raw) {
    try {
      action = JSON.parse(raw)?.action || 'alert'
    } catch {
      action = 'alert'
    }
  }
  return { alert: '仅告警', disable: '禁用账号', delete: '删除账号' }[action] || action
}

export function policyTone(raw) {
  const label = policyLabel(raw)
  return { 仅告警: 'gray', 禁用账号: 'orange', 删除账号: 'red' }[label] || 'gray'
}

export const runStatusText = (s) =>
  ({ succeeded: '成功', failed: '失败', partial: '部分成功', running: '运行中', pending: '等待中' }[s] || s || '—')
export const runStatusTone = (s) =>
  ({ succeeded: 'green', failed: 'red', partial: 'orange', running: 'blue', pending: 'gray' }[s] || 'gray')

export const matchStateText = (s) =>
  ({ matched: '已匹配', review: '待复核', ambiguous: '歧义' }[s] || s || '—')
export const matchStateTone = (s) =>
  ({ matched: 'green', review: 'yellow', ambiguous: 'orange' }[s] || 'gray')

export const riskText = (r) => ({ high: '高风险', medium: '中风险', low: '低风险' }[r] || r || '—')
export const riskTone = (r) => ({ high: 'red', medium: 'orange', low: 'gray' }[r] || 'gray')

export const conflictStatusText = (s) => ({ open: '待处理', resolved: '已解决' }[s] || s || '—')
export const conflictStatusTone = (s) => ({ open: 'orange', resolved: 'green' }[s] || 'gray')

export const conflictTypeText = (t) =>
  ({ register: '注册 IP 冲突', login: '登录 IP 冲突', mixed: '注册+登录混合' }[t] || t || '—')

export const evidenceSourceText = (s) =>
  ({ register: '注册 IP', login_current: '当前登录 IP', login_log: '登录日志 IP' }[s] || s || '—')

export const jobStatusText = (s) =>
  ({ succeeded: '成功', failed: '失败', pending: '等待中', undone: '已撤销' }[s] || s || '—')
export const jobStatusTone = (s) =>
  ({ succeeded: 'green', failed: 'red', pending: 'gray', undone: 'blue' }[s] || 'gray')

export const actionText = (a) => ({ disable: '禁用', delete: '删除', alert: '仅告警' }[a] || a || '—')

export const skipReasonText = (r) =>
  ({ winner: '保留账号（winner）', low_confidence_review: '置信度低，待人工复核', low_confidence_ambiguous: '歧义匹配，不自动处理' }[r] || r || '—')

export const userStatusText = (s) => {
  const value = String(s ?? '')
  if (value === '2') return '正常'
  if (value === 'active') return '正常'
  return value ? `状态 ${value}` : '—'
}

export function newIdempotencyKey() {
  if (window.crypto?.randomUUID) return window.crypto.randomUUID()
  return `key-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
