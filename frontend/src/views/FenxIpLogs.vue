<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>登录 IP 记录</h2>
        <p class="muted">直接管理 fenx 站点 zyads_log_login 表；删除记录可释放 IP 占用</p>
      </div>
      <button class="ghost small" @click="loadLogs">刷新</button>
    </div>
    <form class="inline-form" @submit.prevent="searchLogs">
      <label>用户名<input v-model.trim="logFilters.username" placeholder="模糊匹配" /></label>
      <label>IP<input v-model.trim="logFilters.ip" placeholder="前缀匹配，如 183.211" /></label>
      <button class="primary small" type="submit">搜索</button>
    </form>
    <SkeletonTable v-if="logsLoading && !logs.length" :cols="7" />
    <EmptyState v-else-if="!logs.length" title="暂无登录记录" hint="用户登录成功后会在这里留下 IP 记录。" />
    <table v-else>
      <thead><tr><th>ID</th><th>用户名</th><th>方式</th><th>IP</th><th>状态</th><th>时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="row in logs" :key="row.id">
          <td class="mono">{{ row.id }}</td>
          <td>{{ row.username }}</td>
          <td>{{ String(row.type) === '1' ? 'QQ 登录' : '账号密码' }}</td>
          <td class="mono">{{ row.ip }}</td>
          <td><StatusBadge :text="String(row.status) === '1' ? '成功' : '失败'" :tone="String(row.status) === '1' ? 'green' : 'red'" /></td>
          <td>{{ formatDate(row.time) }}</td>
          <td>
            <button class="ghost small" @click="openDeleteLog(row)">删除该条</button>
            <button class="danger small" @click="openClearIp(row)">清空此 IP</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div class="pager" v-if="logs.length || logOffset > 0">
      <button class="ghost small" :disabled="logOffset === 0" @click="pageLogs(-1)">上一页</button>
      <span class="muted">第 {{ logOffset / LOG_LIMIT + 1 }} 页</span>
      <button class="ghost small" :disabled="logs.length < LOG_LIMIT" @click="pageLogs(1)">下一页</button>
    </div>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>注册 IP 查账号</h2>
        <p class="muted">按 regip 反查账号，可直接修正账号的注册 IP（修正后该账号以新 IP 为准）</p>
      </div>
    </div>
    <form class="inline-form" @submit.prevent="searchRegip">
      <label>注册 IP<input v-model.trim="regipQuery" placeholder="精确匹配，如 183.211.183.22" /></label>
      <button class="primary small" type="submit" :disabled="regipSearching">{{ regipSearching ? '查询中…' : '查询' }}</button>
    </form>
    <EmptyState v-if="regipSearched && !regipUsers.length" title="没有账号使用该注册 IP" hint="该 IP 当前不会被注册拦截规则命中。" />
    <table v-else-if="regipUsers.length">
      <thead><tr><th>UID</th><th>用户名</th><th>状态</th><th>注册 IP</th><th>注册时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="row in regipUsers" :key="row.uid">
          <td class="mono">{{ row.uid }}</td>
          <td>{{ row.username }}</td>
          <td><StatusBadge :text="userStatusText(row.status)" :tone="String(row.status) === '2' ? 'green' : 'orange'" /></td>
          <td class="mono">{{ row.regip || '—' }}</td>
          <td>{{ formatDate(row.regtime) }}</td>
          <td><button class="ghost small" @click="openEditRegip(row)">修改注册 IP</button></td>
        </tr>
      </tbody>
    </table>
  </section>

  <Modal :open="deleteLogOpen" danger title="删除登录记录" @close="deleteLogOpen = false">
    <p>即将删除记录 <strong>#{{ deleteLogTarget?.id }}</strong>（{{ deleteLogTarget?.username }} · {{ deleteLogTarget?.ip }}）。</p>
    <p class="muted">删除后该 IP 上此账号的登录占用随之消失，操作会记录审计事件。</p>
    <p v-if="deleteLogError" class="field-error">{{ deleteLogError }}</p>
    <template #footer>
      <button class="ghost" @click="deleteLogOpen = false">取消</button>
      <button class="danger" :disabled="deleteLogSaving" @click="submitDeleteLog">{{ deleteLogSaving ? '删除中…' : '确认删除' }}</button>
    </template>
  </Modal>

  <Modal :open="clearIpOpen" danger title="清空 IP 全部登录记录" @close="clearIpOpen = false">
    <p>即将删除 IP <strong class="mono">{{ clearIpTarget }}</strong> 的<strong>全部</strong>登录记录。</p>
    <p class="muted">该 IP 上所有账号的登录占用都会被释放，常用于放行被误拦的用户；操作会记录审计事件。</p>
    <p v-if="clearIpError" class="field-error">{{ clearIpError }}</p>
    <template #footer>
      <button class="ghost" @click="clearIpOpen = false">取消</button>
      <button class="danger" :disabled="clearIpSaving" @click="submitClearIp">{{ clearIpSaving ? '清空中…' : '确认清空' }}</button>
    </template>
  </Modal>

  <Modal :open="editRegipOpen" title="修改注册 IP" @close="editRegipOpen = false">
    <p>账号 <strong>{{ editRegipTarget?.username }}</strong>（UID <span class="mono">{{ editRegipTarget?.uid }}</span>）</p>
    <label class="modal-field">
      注册 IP
      <input v-model.trim="editRegipValue" placeholder="留空则清除注册 IP（该账号登录不再受初始 IP 限制）" />
    </label>
    <p class="muted">当前值：{{ editRegipTarget?.regip || '—' }}；留空保存 = 清除，登录拦截对该账号失效。</p>
    <p v-if="editRegipError" class="field-error">{{ editRegipError }}</p>
    <template #footer>
      <button class="ghost" @click="editRegipOpen = false">取消</button>
      <button class="primary" :disabled="editRegipSaving" @click="submitEditRegip">{{ editRegipSaving ? '保存中…' : '保存' }}</button>
    </template>
  </Modal>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import SkeletonTable from '../components/SkeletonTable.vue'
import Modal from '../components/Modal.vue'
import { listFenxLoginLogs, deleteFenxLoginLog, clearFenxLoginLogIp, searchFenxUsers, updateFenxUser } from '../api'
import { formatDate, userStatusText } from '../utils/format'
import { toast } from '../utils/toast'

const LOG_LIMIT = 50
const logs = ref([])
const logsLoading = ref(false)
const logOffset = ref(0)
const logFilters = reactive({ username: '', ip: '' })

async function loadLogs() {
  logsLoading.value = true
  try {
    const params = { limit: LOG_LIMIT, offset: logOffset.value }
    if (logFilters.username) params.username = logFilters.username
    if (logFilters.ip) params.ip = logFilters.ip
    const data = await listFenxLoginLogs(params)
    logs.value = data?.items || []
  } catch (e) {
    toast.error(e.message)
  } finally {
    logsLoading.value = false
  }
}

function searchLogs() {
  logOffset.value = 0
  loadLogs()
}

function pageLogs(direction) {
  logOffset.value = Math.max(0, logOffset.value + direction * LOG_LIMIT)
  loadLogs()
}

const deleteLogOpen = ref(false)
const deleteLogTarget = ref(null)
const deleteLogError = ref('')
const deleteLogSaving = ref(false)

function openDeleteLog(row) {
  deleteLogTarget.value = row
  deleteLogError.value = ''
  deleteLogOpen.value = true
}

async function submitDeleteLog() {
  if (!deleteLogTarget.value) return
  deleteLogSaving.value = true
  deleteLogError.value = ''
  try {
    await deleteFenxLoginLog(deleteLogTarget.value.id)
    toast.success(`记录 #${deleteLogTarget.value.id} 已删除`)
    deleteLogOpen.value = false
    await loadLogs()
  } catch (e) {
    deleteLogError.value = e.message
  } finally {
    deleteLogSaving.value = false
  }
}

const clearIpOpen = ref(false)
const clearIpTarget = ref('')
const clearIpError = ref('')
const clearIpSaving = ref(false)

function openClearIp(row) {
  clearIpTarget.value = row.ip
  clearIpError.value = ''
  clearIpOpen.value = true
}

async function submitClearIp() {
  clearIpSaving.value = true
  clearIpError.value = ''
  try {
    const result = await clearFenxLoginLogIp(clearIpTarget.value)
    toast.success(`已清空 ${clearIpTarget.value} 的 ${result.deleted} 条登录记录`)
    clearIpOpen.value = false
    await loadLogs()
  } catch (e) {
    clearIpError.value = e.message
  } finally {
    clearIpSaving.value = false
  }
}

const regipQuery = ref('')
const regipUsers = ref([])
const regipSearched = ref(false)
const regipSearching = ref(false)

async function searchRegip() {
  if (!regipQuery.value) {
    toast.error('请输入注册 IP')
    return
  }
  regipSearching.value = true
  try {
    const data = await searchFenxUsers({ regip: regipQuery.value, limit: 50 })
    regipUsers.value = data?.items || []
    regipSearched.value = true
  } catch (e) {
    toast.error(e.message)
  } finally {
    regipSearching.value = false
  }
}

const editRegipOpen = ref(false)
const editRegipTarget = ref(null)
const editRegipValue = ref('')
const editRegipError = ref('')
const editRegipSaving = ref(false)

function openEditRegip(row) {
  editRegipTarget.value = row
  editRegipValue.value = row.regip || ''
  editRegipError.value = ''
  editRegipOpen.value = true
}

async function submitEditRegip() {
  if (!editRegipTarget.value) return
  editRegipSaving.value = true
  editRegipError.value = ''
  try {
    await updateFenxUser(editRegipTarget.value.uid, { regip: editRegipValue.value })
    toast.success(`账号 ${editRegipTarget.value.uid} 的注册 IP 已更新`)
    editRegipOpen.value = false
    await searchRegip()
  } catch (e) {
    editRegipError.value = e.message
  } finally {
    editRegipSaving.value = false
  }
}

onMounted(loadLogs)
</script>

<style scoped>
.modal-field { display: grid; gap: 6px; margin-top: 12px; }
</style>
