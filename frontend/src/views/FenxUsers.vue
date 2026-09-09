<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>fenx 用户</h2>
        <p class="muted">搜索、编辑、禁用与删除 fenx_site.users 账号；手机号 / 邮箱已脱敏</p>
      </div>
      <button class="ghost small" @click="load(0)">刷新</button>
    </div>
    <form class="inline-form" @submit.prevent="load(0)">
      <label>UID<input v-model.trim="filters.uid" placeholder="精确匹配" /></label>
      <label>用户名<input v-model.trim="filters.username" placeholder="模糊匹配" /></label>
      <label>手机号<input v-model.trim="filters.mobile" placeholder="完整手机号" /></label>
      <button class="primary small" type="submit">搜索</button>
    </form>

    <SkeletonTable v-if="loading && !rows.length" :cols="9" />
    <EmptyState v-else-if="!rows.length" title="没有匹配的账号" hint="调整搜索条件后重试。" />
    <template v-else>
      <table>
        <thead>
          <tr><th>UID</th><th>用户名</th><th>手机号</th><th>QQ</th><th>状态</th><th>注册 IP</th><th>登录 IP</th><th>注册时间</th><th v-if="auth.isSuperAdmin">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="uidOf(row)">
            <td class="num">{{ uidOf(row) }}</td>
            <td>{{ row.username || '—' }}</td>
            <td class="mono">{{ row.mobile || '—' }}</td>
            <td>{{ row.qq || '—' }}</td>
            <td><StatusBadge :text="statusLabel(row.status)" :tone="isNormal(row) ? 'green' : isDisabled(row) ? 'red' : 'gray'" /></td>
            <td class="mono">{{ row.regip || '—' }}</td>
            <td class="mono">{{ row.loginip || '—' }}</td>
            <td>{{ formatDate(row.regtime) }}</td>
            <td v-if="auth.isSuperAdmin">
              <div class="row-actions">
                <button class="ghost small" @click="openEdit(row)">编辑</button>
                <button class="ghost small" @click="openReset(row)">重置密码</button>
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

  <Drawer :open="editOpen" title="编辑 fenx 账号" :subtitle="`UID ${editForm.uid} · 仅提交被修改的字段`" @close="editOpen = false">
    <p v-if="metaFailed" class="helper">枚举元数据加载失败，状态 / 类型 / 等级 / 用户组已降级为数值输入。</p>
    <fieldset v-for="group in FIELD_GROUPS" :key="group.title">
      <legend>{{ group.title }}</legend>
      <div class="form-grid">
        <label v-for="field in group.fields" :key="field.key" :class="{ 'span-2': field.type === 'textarea' }">
          {{ field.label }}
          <textarea v-if="field.type === 'textarea'" v-model="editForm[field.key]"></textarea>
          <template v-else-if="field.type === 'select' && !metaFailed">
            <select v-model="editForm[field.key]">
              <option value="">（空）</option>
              <option v-for="opt in field.options()" :key="opt.value" :value="String(opt.value)">{{ opt.label }}</option>
            </select>
            <p v-if="field.key === 'levelid' && !meta.levels.length" class="helper">等级表为空，当前无可选等级。</p>
          </template>
          <input
            v-else
            v-model.trim="editForm[field.key]"
            :type="field.type === 'number' || (field.type === 'select' && metaFailed) ? 'number' : 'text'"
          />
        </label>
      </div>
    </fieldset>
    <p v-if="editError" class="field-error">{{ editError }}</p>
    <template #footer>
      <button class="ghost" @click="editOpen = false">取消</button>
      <button class="primary" :disabled="editSaving" @click="submitEdit">{{ editSaving ? '保存中…' : '保存修改' }}</button>
    </template>
  </Drawer>

  <Modal :open="resetOpen" danger title="重置登录密码" @close="resetOpen = false">
    <p>将为账号 <strong>{{ resetForm.username }}</strong>（UID <span class="mono">{{ resetForm.uid }}</span>）重置密码。</p>
    <label class="field">
      <span class="req">新密码</span>
      <span style="display:flex;gap:8px">
        <input v-model="resetForm.password" :type="resetForm.show ? 'text' : 'password'" minlength="6" style="flex:1" autocomplete="new-password" />
        <button type="button" class="ghost small" @click="resetForm.show = !resetForm.show">{{ resetForm.show ? '隐藏' : '显示' }}</button>
      </span>
      <span class="helper">至少 6 位</span>
    </label>
    <label class="field">
      <span class="req">确认新密码</span>
      <input v-model="resetForm.confirm" :type="resetForm.show ? 'text' : 'password'" minlength="6" autocomplete="new-password" />
    </label>
    <p v-if="resetError" class="field-error">{{ resetError }}</p>
    <template #footer>
      <button class="ghost" @click="resetOpen = false">取消</button>
      <button class="danger" :disabled="resetSaving" @click="submitReset">{{ resetSaving ? '提交中…' : '确认重置' }}</button>
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
      <template v-if="statusForm.mode === 'disable'">将被禁用（status 固定写为 4），禁用后无法登录。</template>
      <template v-else>将恢复正常（status 固定写为 2）。</template>
    </p>
    <p class="muted">恢复能力：禁用可随时通过「恢复」还原，操作会记录审计事件。</p>
    <p v-if="statusError" class="field-error">{{ statusError }}</p>
    <template #footer>
      <button class="ghost" @click="statusOpen = false">取消</button>
      <button :class="statusForm.mode === 'disable' ? 'danger' : 'primary'" :disabled="statusSaving" @click="submitStatus">
        {{ statusSaving ? '处理中…' : statusForm.mode === 'disable' ? '确认禁用' : '确认恢复' }}
      </button>
    </template>
  </Modal>

  <Modal :open="deleteOpen" danger title="删除 fenx 账号" @close="deleteOpen = false">
    <p>即将删除账号 <strong>{{ deleteTarget?.username }}</strong>（UID <span class="mono">{{ deleteUid }}</span>）。</p>
    <p class="muted">删除前系统会归档账号关键字段，但恢复需要走人工流程，不可无条件恢复；该操作不触碰登录日志等审计记录。</p>
    <label class="field">
      请输入用户名（{{ deleteTarget?.username }}）以确认
      <input v-model.trim="deleteConfirm" :placeholder="deleteTarget?.username" />
    </label>
    <p v-if="deleteError" class="field-error">{{ deleteError }}</p>
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
import Drawer from '../components/Drawer.vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import SkeletonTable from '../components/SkeletonTable.vue'
import {
  searchFenxUsers, getFenxUserMeta, updateFenxUser, resetFenxUserPassword,
  disableFenxUser, enableFenxUser, deleteFenxUser,
} from '../api'
import { formatDate } from '../utils/format'
import { toast } from '../utils/toast'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const rows = ref([])
const filters = reactive({ uid: '', username: '', mobile: '' })
const limit = 50
const offset = ref(0)
const loading = ref(false)

const meta = reactive({ statuses: [], types: [], groups: [], levels: [] })
const metaFailed = ref(false)

const INSITE_OPTIONS = [{ value: '1', label: '是' }, { value: '0', label: '否' }]
const FIELD_GROUPS = [
  {
    title: '基本信息',
    fields: [
      { key: 'username', label: '用户名' },
      { key: 'status', label: '状态', type: 'select', options: () => meta.statuses },
      { key: 'type', label: '类型', type: 'select', options: () => meta.types },
      {
        key: 'levelid', label: '等级', type: 'select',
        options: () => meta.levels.map((level) => ({ value: String(level.levelid), label: level.levelname })),
      },
      { key: 'groupid', label: '用户组', type: 'select', options: () => meta.groups },
      { key: 'insite', label: '站内账号', type: 'select', options: () => INSITE_OPTIONS },
    ],
  },
  {
    title: '联系方式',
    fields: [
      { key: 'email', label: '邮箱' },
      { key: 'qq', label: 'QQ' },
      { key: 'mobile', label: '手机号' },
      { key: 'tel', label: '电话' },
      { key: 'contact', label: '联系人' },
      { key: 'idcard', label: '身份证' },
    ],
  },
  {
    title: '结算信息',
    fields: [
      { key: 'accountname', label: '开户名' },
      { key: 'bankname', label: '银行' },
      { key: 'bankbranch', label: '支行' },
      { key: 'bankaccount', label: '银行账号' },
    ],
  },
  {
    title: 'IP 信息',
    fields: [
      { key: 'regip', label: '注册 IP' },
      { key: 'loginip', label: '最近登录 IP' },
    ],
  },
  {
    title: '其他',
    fields: [
      { key: 'recommend', label: '推荐人' },
      { key: 'money', label: '余额 money', type: 'number' },
      { key: 'integral', label: '积分 integral', type: 'number' },
      { key: 'serviceid', label: '客服 serviceid' },
      { key: 'zlink', label: 'zlink' },
      { key: 'memo', label: '备注 memo', type: 'textarea' },
    ],
  },
]
const EDIT_KEYS = FIELD_GROUPS.flatMap((group) => group.fields.map((field) => field.key))

const editOpen = ref(false)
const editForm = reactive({ uid: '', ...Object.fromEntries(EDIT_KEYS.map((key) => [key, ''])) })
const editOriginal = ref({})
const editSaving = ref(false)
const editError = ref('')

const resetOpen = ref(false)
const resetForm = reactive({ uid: '', username: '', password: '', confirm: '', show: false })
const resetSaving = ref(false)
const resetError = ref('')

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

function uidOf(row) {
  return row.id ?? row.uid ?? row.userid ?? row.user_id ?? '—'
}

const isNormal = (row) => String(row.status ?? '') === '2'
const isDisabled = (row) => String(row.status ?? '') === '4'

function statusLabel(value) {
  const text = String(value ?? '')
  const found = meta.statuses.find((item) => String(item.value) === text)
  return found ? found.label : text ? `状态 ${text}` : '—'
}

async function loadMeta() {
  try {
    Object.assign(meta, await getFenxUserMeta())
  } catch {
    metaFailed.value = true
  }
}

async function load(nextOffset = 0) {
  loading.value = true
  try {
    const params = { limit, offset: Math.max(0, nextOffset) }
    if (filters.uid) params.uid = filters.uid
    if (filters.username) params.username = filters.username
    if (filters.mobile) params.mobile = filters.mobile
    const data = await searchFenxUsers(params)
    rows.value = data.items || []
    offset.value = data.offset || 0
  } catch (e) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}

function openEdit(row) {
  editForm.uid = uidOf(row)
  const snapshot = {}
  for (const key of EDIT_KEYS) {
    const value = row[key] === null || row[key] === undefined ? '' : String(row[key])
    editForm[key] = value
    snapshot[key] = value
  }
  editOriginal.value = snapshot
  editError.value = ''
  editOpen.value = true
}

async function submitEdit() {
  const patch = {}
  for (const key of EDIT_KEYS) {
    if (editForm[key] !== editOriginal.value[key]) patch[key] = editForm[key]
  }
  if (!Object.keys(patch).length) {
    toast.info('没有修改任何字段')
    return
  }
  editSaving.value = true
  editError.value = ''
  try {
    const result = await updateFenxUser(editForm.uid, patch)
    editOpen.value = false
    toast.success(`账号 ${editForm.uid} 已更新 ${result.fields?.length || Object.keys(patch).length} 个字段`)
    await load(offset.value)
  } catch (e) {
    editError.value = e.message
  } finally {
    editSaving.value = false
  }
}

function openReset(row) {
  resetForm.uid = uidOf(row)
  resetForm.username = row.username || ''
  resetForm.password = ''
  resetForm.confirm = ''
  resetForm.show = false
  resetError.value = ''
  resetOpen.value = true
}

async function submitReset() {
  resetError.value = ''
  if (resetForm.password.length < 6) {
    resetError.value = '新密码至少 6 位'
    return
  }
  if (resetForm.password !== resetForm.confirm) {
    resetError.value = '两次输入的密码不一致'
    return
  }
  resetSaving.value = true
  try {
    await resetFenxUserPassword(resetForm.uid, resetForm.password)
    resetOpen.value = false
    toast.success(`账号 ${resetForm.uid} 密码已重置`)
  } catch (e) {
    resetError.value = e.message
  } finally {
    resetSaving.value = false
  }
}

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
    toast.success(`账号 ${statusForm.uid} 已${statusForm.mode === 'disable' ? '禁用' : '恢复'}（status ${result.before_status} → ${result.status}）`)
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
    toast.success(`账号 ${deleteUid.value} 已删除并归档`)
    await load(offset.value)
  } catch (e) {
    deleteError.value = e.message
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  loadMeta()
  load(0)
})
</script>
