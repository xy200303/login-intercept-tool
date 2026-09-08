<template>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="notice" class="muted">{{ notice }}</p>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>fenx 用户</h2>
        <p class="muted">搜索、编辑、禁用与删除 fenx_site.users 账号；手机号已脱敏</p>
      </div>
      <button class="ghost small" @click="load(0)">刷新</button>
    </div>
    <form class="inline-form" @submit.prevent="load(0)">
      <label>UID<input v-model.trim="filters.uid" placeholder="精确匹配" /></label>
      <label>用户名<input v-model.trim="filters.username" placeholder="模糊匹配" /></label>
      <label>手机号<input v-model.trim="filters.mobile" placeholder="完整手机号" /></label>
      <button class="primary small" type="submit">搜索</button>
    </form>

    <EmptyState v-if="!rows.length && !loading" title="没有匹配的账号" hint="调整搜索条件后重试。" />
    <template v-else>
      <table>
        <thead>
          <tr><th>UID</th><th>用户名</th><th>手机号</th><th>QQ</th><th>邮箱</th><th>状态</th><th>注册 IP</th><th>登录 IP</th><th>注册时间</th><th v-if="auth.isSuperAdmin">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="uidOf(row)">
            <td class="mono">{{ uidOf(row) }}</td>
            <td>{{ row.username || '—' }}</td>
            <td class="mono">{{ row.mobile || '—' }}</td>
            <td>{{ row.qq || '—' }}</td>
            <td>{{ row.email || '—' }}</td>
            <td>{{ userStatusText(row.status) }}</td>
            <td class="mono">{{ row.regip || '—' }}</td>
            <td class="mono">{{ row.loginip || '—' }}</td>
            <td>{{ formatDate(row.regtime) }}</td>
            <td v-if="auth.isSuperAdmin">
              <div class="row-actions">
                <button class="ghost small" @click="openEdit(row)">编辑</button>
                <button v-if="isNormal(row)" class="ghost small" @click="openStatus(row, 'disable')">禁用</button>
                <button v-else class="ghost small" @click="openStatus(row, 'enable')">恢复</button>
                <button class="danger small" @click="openDelete(row)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost small" :disabled="offset === 0" @click="load(offset - limit)">上一页</button>
        <span class="muted">第 {{ Math.floor(offset / limit) + 1 }} 页</span>
        <button class="ghost small" :disabled="rows.length < limit" @click="load(offset + limit)">下一页</button>
      </div>
    </template>
  </section>

  <Modal :open="editOpen" title="编辑 fenx 账号" @close="editOpen = false">
    <p>UID：<strong class="mono">{{ editForm.uid }}</strong>（留空的字段保持不变）</p>
    <label class="field">用户名<input v-model.trim="editForm.username" /></label>
    <label class="field">手机号<input v-model.trim="editForm.mobile" placeholder="输入完整新手机号" /></label>
    <label class="field">QQ<input v-model.trim="editForm.qq" /></label>
    <label class="field">邮箱<input v-model.trim="editForm.email" /></label>
    <p v-if="editError" class="error">{{ editError }}</p>
    <template #footer>
      <button class="ghost" @click="editOpen = false">取消</button>
      <button class="primary" :disabled="!editDirty || editSaving" @click="submitEdit">
        {{ editSaving ? '保存中…' : '保存修改' }}
      </button>
    </template>
  </Modal>

  <Modal
    :open="statusOpen"
    :danger="statusForm.mode === 'disable'"
    :title="statusForm.mode === 'disable' ? '禁用 fenx 账号' : '恢复 fenx 账号'"
    @close="statusOpen = false"
  >
    <p>
      账号 <strong>{{ statusForm.username }}</strong>（UID <span class="mono">{{ statusForm.uid }}</span>）：
      <template v-if="statusForm.mode === 'disable'">
        将被禁用（status 固定写为 4），禁用后无法登录。
      </template>
      <template v-else>
        将恢复正常（status 固定写为 2）。
      </template>
    </p>
    <p class="muted">恢复能力：禁用可随时通过「恢复」还原，操作会记录审计事件。</p>
    <p v-if="statusError" class="error">{{ statusError }}</p>
    <template #footer>
      <button class="ghost" @click="statusOpen = false">取消</button>
      <button
        :class="statusForm.mode === 'disable' ? 'danger' : 'primary'"
        :disabled="statusSaving"
        @click="submitStatus"
      >{{ statusSaving ? '处理中…' : statusForm.mode === 'disable' ? '确认禁用' : '确认恢复' }}</button>
    </template>
  </Modal>

  <Modal :open="deleteOpen" danger title="删除 fenx 账号" @close="deleteOpen = false">
    <p>
      即将删除账号 <strong>{{ deleteTarget?.username }}</strong>（UID <span class="mono">{{ deleteUid }}</span>）。
    </p>
    <p class="muted">
      删除前系统会归档账号关键字段，但恢复需要走人工流程，不可无条件恢复；该操作不触碰登录日志等审计记录。
    </p>
    <label class="field">
      请输入用户名（{{ deleteTarget?.username }}）以确认
      <input v-model.trim="deleteConfirm" :placeholder="deleteTarget?.username" />
    </label>
    <p v-if="deleteError" class="error">{{ deleteError }}</p>
    <template #footer>
      <button class="ghost" @click="deleteOpen = false">取消</button>
      <button class="danger" :disabled="deleteConfirm !== deleteTarget?.username || deleting" @click="submitDelete">
        {{ deleting ? '删除中…' : '确认删除' }}
      </button>
    </template>
  </Modal>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import Modal from '../components/Modal.vue'
import EmptyState from '../components/EmptyState.vue'
import { searchFenxUsers, updateFenxUser, disableFenxUser, enableFenxUser, deleteFenxUser } from '../api'
import { formatDate, userStatusText } from '../utils/format'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const rows = ref([])
const filters = reactive({ uid: '', username: '', mobile: '' })
const limit = 50
const offset = ref(0)
const loading = ref(false)
const error = ref('')
const notice = ref('')

const editOpen = ref(false)
const editForm = reactive({ uid: '', username: '', mobile: '', qq: '', email: '' })
const editSaving = ref(false)
const editError = ref('')

const statusOpen = ref(false)
const statusForm = reactive({ uid: '', username: '', mode: 'disable' })
const statusSaving = ref(false)
const statusError = ref('')

const deleteOpen = ref(false)
const deleteTarget = ref(null)
const deleteConfirm = ref('')
const deleting = ref(false)
const deleteError = ref('')

const deleteUid = computed(() => (deleteTarget.value ? uidOf(deleteTarget.value) : ''))
const editDirty = computed(
  () => Boolean(editForm.username || editForm.mobile || editForm.qq || editForm.email)
)

function uidOf(row) {
  return row.id ?? row.uid ?? row.userid ?? row.user_id ?? '—'
}

async function load(nextOffset = 0) {
  loading.value = true
  error.value = ''
  try {
    const params = { limit, offset: Math.max(0, nextOffset) }
    if (filters.uid) params.uid = filters.uid
    if (filters.username) params.username = filters.username
    if (filters.mobile) params.mobile = filters.mobile
    const data = await searchFenxUsers(params)
    rows.value = data.items || []
    offset.value = data.offset || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function openEdit(row) {
  editForm.uid = uidOf(row)
  editForm.username = ''
  editForm.mobile = ''
  editForm.qq = ''
  editForm.email = ''
  editError.value = ''
  editOpen.value = true
}

async function submitEdit() {
  editSaving.value = true
  editError.value = ''
  try {
    const patch = {}
    for (const field of ['username', 'mobile', 'qq', 'email']) {
      if (editForm[field]) patch[field] = editForm[field]
    }
    await updateFenxUser(editForm.uid, patch)
    editOpen.value = false
    notice.value = `账号 ${editForm.uid} 已更新`
    await load(offset.value)
  } catch (e) {
    editError.value = e.message
  } finally {
    editSaving.value = false
  }
}

const isNormal = (row) => String(row.status ?? '') === '2'

function openStatus(row, mode) {
  statusForm.uid = uidOf(row)
  statusForm.username = row.username || ''
  statusForm.mode = mode
  statusError.value = ''
  statusOpen.value = true
}

async function submitStatus() {
  statusSaving.value = true
  statusError.value = ''
  try {
    const call = statusForm.mode === 'disable' ? disableFenxUser : enableFenxUser
    const result = await call(statusForm.uid)
    statusOpen.value = false
    notice.value = `账号 ${statusForm.uid} 已${statusForm.mode === 'disable' ? '禁用' : '恢复'}（status ${result.before_status} → ${result.status}）`
    await load(offset.value)
  } catch (e) {
    statusError.value = e.message
  } finally {
    statusSaving.value = false
  }
}

function openDelete(row) {
  deleteTarget.value = row
  deleteConfirm.value = ''
  deleteError.value = ''
  deleteOpen.value = true
}

async function submitDelete() {
  deleting.value = true
  deleteError.value = ''
  try {
    await deleteFenxUser(deleteUid.value)
    deleteOpen.value = false
    notice.value = `账号 ${deleteUid.value} 已删除并归档`
    await load(offset.value)
  } catch (e) {
    deleteError.value = e.message
  } finally {
    deleting.value = false
  }
}

onMounted(() => load(0))
</script>
