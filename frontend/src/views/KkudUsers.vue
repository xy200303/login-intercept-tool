<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>kkud 用户管理</h2>
        <p class="muted">直连 kkud 库的账号管理（仅超级管理员可写），密码不做展示</p>
      </div>
      <div class="row-actions">
        <button class="ghost small" @click="loadKu(0)">刷新</button>
        <button v-if="auth.isSuperAdmin" class="primary small" @click="openCreate">新增用户</button>
      </div>
    </div>
    <form class="inline-form" @submit.prevent="loadKu(0)">
      <label>用户名<input v-model.trim="kuFilters.user" placeholder="模糊匹配" /></label>
      <label>代理（daili）<input v-model.trim="kuFilters.daili" placeholder="精确匹配" /></label>
      <label class="check" style="margin:0"><input v-model="kuFilters.vipOnly" type="checkbox" />仅 VIP</label>
      <button class="primary small" type="submit">搜索</button>
    </form>

    <SkeletonTable v-if="ku.loading && !ku.items.length" :cols="9" />
    <EmptyState v-else-if="!ku.items.length" title="没有匹配的用户" hint="调整搜索条件，或点击「新增用户」直接创建。" />
    <template v-else>
      <table>
        <thead>
          <tr>
            <th>ID</th><th>用户名</th><th>代理</th><th>QQ</th><th style="text-align:right">金额</th><th style="text-align:right">推广积分</th><th>VIP</th><th>推广 IP</th><th>注册时间</th><th v-if="auth.isSuperAdmin">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in ku.items" :key="row.id">
            <td class="num">{{ row.id }}</td>
            <td>{{ row.user || '—' }}</td>
            <td>{{ row.daili || '—' }}</td>
            <td>{{ row.QQ || '—' }}</td>
            <td class="num">{{ row.jine ?? '—' }}</td>
            <td class="num">{{ row.tgjifen ?? '—' }}</td>
            <td><StatusBadge :text="isVip(row) ? 'VIP' : '普通'" :tone="isVip(row) ? 'green' : 'gray'" /></td>
            <td class="mono">{{ row.tgip || '—' }}</td>
            <td>{{ formatFlexibleDate(row.zcsj) }}</td>
            <td v-if="auth.isSuperAdmin">
              <div class="row-actions">
                <button class="ghost small" @click="openEdit(row)">编辑</button>
                <button v-if="!isVip(row)" class="ghost small" @click="openVip(row, true)">开通 VIP</button>
                <button v-else class="ghost small" @click="openVip(row, false)">取消 VIP</button>
                <button class="danger small" @click="openDelete(row)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost small" :disabled="ku.offset === 0" @click="loadKu(ku.offset - ku.limit)">上一页</button>
        <span class="muted">第 {{ Math.floor(ku.offset / ku.limit) + 1 }} 页</span>
        <button class="ghost small" :disabled="ku.items.length < ku.limit" @click="loadKu(ku.offset + ku.limit)">下一页</button>
      </div>
    </template>
  </section>

  <!-- 新增用户 -->
  <Modal :open="createOpen" title="新增 kkud 用户" @close="createOpen = false">
    <div class="form-grid">
      <label><span class="req">用户名</span><input v-model.trim="createForm.user" /></label>
      <label>
        <span class="req">密码</span>
        <input v-model="createForm.password" type="text" autocomplete="off" />
        <span class="helper">该系统明文存储密码，此处明文填写</span>
      </label>
      <label>代理（daili）<input v-model.trim="createForm.daili" /></label>
      <label>QQ<input v-model.trim="createForm.qq" /></label>
      <label>金额 jine<input v-model.trim="createForm.jine" type="number" /></label>
      <label class="check" style="margin:0"><input v-model="createForm.vip" type="checkbox" />开通 VIP</label>
    </div>
    <details class="more-fields">
      <summary>更多字段（txjl / tgip / mac / admin / 链接 / 积分）</summary>
      <div class="form-grid" style="margin-top:10px">
        <label>txjl<input v-model.trim="createForm.txjl" /></label>
        <label>推广 IP tgip<input v-model.trim="createForm.tgip" /></label>
        <label>mac<input v-model.trim="createForm.mac" /></label>
        <label>admin<input v-model.trim="createForm.admin" /></label>
        <label>adminurl<input v-model.trim="createForm.adminurl" /></label>
        <label>dailiurl<input v-model.trim="createForm.dailiurl" /></label>
        <label>superadmin<input v-model.trim="createForm.superadmin" /></label>
        <label>推广积分 tgjifen<input v-model.trim="createForm.tgjifen" type="number" /></label>
      </div>
    </details>
    <p v-if="createError" class="field-error">{{ createError }}</p>
    <template #footer>
      <button class="ghost" @click="createOpen = false">取消</button>
      <button class="primary" :disabled="createSaving" @click="submitCreate">{{ createSaving ? '创建中…' : '创建用户' }}</button>
    </template>
  </Modal>

  <!-- 编辑用户 -->
  <Modal :open="editOpen" title="编辑 kkud 用户" @close="editOpen = false">
    <p class="muted">ID <span class="mono">{{ editForm.id }}</span> · 用户名 {{ editOriginal.user }}（用户名与密码不可在此修改）· 仅提交被修改的字段</p>
    <div class="form-grid">
      <label v-for="field in KU_EDIT_FIELDS" :key="field.key">
        {{ field.label }}
        <input v-model.trim="editForm[field.key]" :type="field.number ? 'number' : 'text'" />
      </label>
      <label class="check" style="margin:0"><input v-model="editForm.vip" type="checkbox" />VIP</label>
    </div>
    <p v-if="editError" class="field-error">{{ editError }}</p>
    <template #footer>
      <button class="ghost" @click="editOpen = false">取消</button>
      <button class="primary" :disabled="editSaving" @click="submitEdit">{{ editSaving ? '保存中…' : '保存修改' }}</button>
    </template>
  </Modal>

  <!-- VIP 确认 -->
  <Modal :open="vipOpen" :title="vipForm.vip ? '开通 VIP' : '取消 VIP'" @close="vipOpen = false">
    <p>
      将对 kkud 用户 <strong>{{ vipForm.username }}</strong>（ID <span class="mono">{{ vipForm.sourceId }}</span>）
      {{ vipForm.vip ? '开通 VIP（写入 vip=1）' : '取消 VIP（清空 vip 字段）' }}。
    </p>
    <p class="muted">该操作会直接更新 kkud 外部库的 VIP 字段，并记录审计事件。</p>
    <p v-if="vipError" class="field-error">{{ vipError }}</p>
    <template #footer>
      <button class="ghost" @click="vipOpen = false">取消</button>
      <button class="primary" :disabled="vipSaving" @click="submitVip">
        {{ vipSaving ? '提交中…' : vipForm.vip ? '确认开通' : '确认取消' }}
      </button>
    </template>
  </Modal>

  <!-- 删除确认 -->
  <Modal :open="deleteOpen" danger title="删除 kkud 用户" @close="deleteOpen = false">
    <p>即将删除用户 <strong>{{ deleteTarget?.user }}</strong>（ID <span class="mono">{{ deleteTarget?.id }}</span>）。</p>
    <p class="muted">删除前系统会归档账号数据（生成归档记录），恢复需走人工流程；操作会记录审计事件。</p>
    <label class="field">
      请输入用户名（{{ deleteTarget?.user }}）以确认
      <input v-model.trim="deleteConfirm" :placeholder="deleteTarget?.user" />
    </label>
    <p v-if="deleteError" class="field-error">{{ deleteError }}</p>
    <template #footer>
      <button class="ghost" @click="deleteOpen = false">取消</button>
      <button class="danger" :disabled="deleteConfirm !== deleteTarget?.user || deleting" @click="submitDelete">
        {{ deleting ? '删除中…' : '确认删除' }}
      </button>
    </template>
  </Modal>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Modal from '../components/Modal.vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import SkeletonTable from '../components/SkeletonTable.vue'
import { listKkudUsers, createKkudUser, updateKkudUser, deleteKkudUser, updateVip } from '../api'
import { formatFlexibleDate } from '../utils/format'
import { toast } from '../utils/toast'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

/* ---------- 用户管理（直连） ---------- */
const ku = reactive({ items: [], limit: 50, offset: 0, loading: false })
const kuFilters = reactive({ user: '', daili: '', vipOnly: false })

const isVip = (row) => String(row.vip ?? '') === '1'

async function loadKu(nextOffset = 0) {
  ku.loading = true
  try {
    const params = { limit: ku.limit, offset: Math.max(0, nextOffset) }
    if (kuFilters.user) params.user = kuFilters.user
    if (kuFilters.daili) params.daili = kuFilters.daili
    if (kuFilters.vipOnly) params.vip = '1'
    const data = await listKkudUsers(params)
    ku.items = data.items || []
    ku.offset = data.offset || 0
  } catch (e) {
    toast.error(e.message)
  } finally {
    ku.loading = false
  }
}

/* ---------- 新增 ---------- */
const CREATE_BLANK = {
  user: '', password: '', daili: '', qq: '', jine: '', vip: false,
  txjl: '', tgip: '', mac: '', admin: '', adminurl: '', dailiurl: '', superadmin: '', tgjifen: '',
}
const createOpen = ref(false)
const createForm = reactive({ ...CREATE_BLANK })
const createSaving = ref(false)
const createError = ref('')

function openCreate() {
  Object.assign(createForm, CREATE_BLANK)
  createError.value = ''
  createOpen.value = true
}

async function submitCreate() {
  createError.value = ''
  if (!createForm.user || !createForm.password) {
    createError.value = '用户名和密码为必填项'
    return
  }
  createSaving.value = true
  try {
    const payload = { user: createForm.user, password: createForm.password, vip: createForm.vip }
    for (const key of ['daili', 'qq', 'jine', 'txjl', 'tgip', 'mac', 'admin', 'adminurl', 'dailiurl', 'superadmin', 'tgjifen']) {
      if (createForm[key] !== '') payload[key] = createForm[key]
    }
    const result = await createKkudUser(payload)
    createOpen.value = false
    toast.success(`用户 ${result.user || createForm.user} 已创建（ID ${result.id}）`)
    await loadKu(0)
  } catch (e) {
    createError.value = e.message
  } finally {
    createSaving.value = false
  }
}

/* ---------- 编辑 ---------- */
const KU_EDIT_FIELDS = [
  { key: 'daili', label: '代理（daili）' },
  { key: 'qq', label: 'QQ' },
  { key: 'jine', label: '金额 jine', number: true },
  { key: 'txjl', label: 'txjl' },
  { key: 'tgip', label: '推广 IP tgip' },
  { key: 'mac', label: 'mac' },
  { key: 'admin', label: 'admin' },
  { key: 'adminurl', label: 'adminurl' },
  { key: 'dailiurl', label: 'dailiurl' },
  { key: 'superadmin', label: 'superadmin' },
  { key: 'tgjifen', label: '推广积分 tgjifen', number: true },
]
// 列表行字段为 QQ 大写，编辑字段为小写 qq
const ROW_TO_EDIT = { qq: 'QQ' }

const editOpen = ref(false)
const editForm = reactive({ id: '', vip: false, ...Object.fromEntries(KU_EDIT_FIELDS.map((field) => [field.key, ''])) })
const editOriginal = ref({})
const editSaving = ref(false)
const editError = ref('')

function openEdit(row) {
  editForm.id = String(row.id)
  const snapshot = {}
  for (const field of KU_EDIT_FIELDS) {
    const rowKey = ROW_TO_EDIT[field.key] || field.key
    const value = row[rowKey] === null || row[rowKey] === undefined ? '' : String(row[rowKey])
    editForm[field.key] = value
    snapshot[field.key] = value
  }
  editForm.vip = isVip(row)
  snapshot.vip = isVip(row)
  snapshot.user = row.user || ''
  editOriginal.value = snapshot
  editError.value = ''
  editOpen.value = true
}

async function submitEdit() {
  const patch = {}
  for (const field of KU_EDIT_FIELDS) {
    if (editForm[field.key] !== editOriginal.value[field.key]) patch[field.key] = editForm[field.key]
  }
  if (editForm.vip !== editOriginal.value.vip) patch.vip = editForm.vip
  if (!Object.keys(patch).length) {
    toast.info('没有修改任何字段')
    return
  }
  editSaving.value = true
  editError.value = ''
  try {
    await updateKkudUser(editForm.id, patch)
    editOpen.value = false
    toast.success(`用户 ${editForm.id} 已更新 ${Object.keys(patch).length} 个字段`)
    await loadKu(ku.offset)
  } catch (e) {
    editError.value = e.message
  } finally {
    editSaving.value = false
  }
}

/* ---------- VIP ---------- */
const vipOpen = ref(false)
const vipForm = reactive({ sourceId: '', username: '', vip: true })
const vipSaving = ref(false)
const vipError = ref('')

function openVip(row, vip) {
  vipForm.sourceId = String(row.id)
  vipForm.username = row.user || ''
  vipForm.vip = vip
  vipError.value = ''
  vipOpen.value = true
}

async function submitVip() {
  vipSaving.value = true
  vipError.value = ''
  try {
    await updateVip(vipForm.sourceId, vipForm.vip)
    vipOpen.value = false
    toast.success(`用户 ${vipForm.username || vipForm.sourceId} 已${vipForm.vip ? '开通' : '取消'} VIP`)
    await loadKu(ku.offset)
  } catch (e) {
    vipError.value = e.message
  } finally {
    vipSaving.value = false
  }
}

/* ---------- 删除 ---------- */
const deleteOpen = ref(false)
const deleteTarget = ref(null)
const deleteConfirm = ref('')
const deleting = ref(false)
const deleteError = ref('')

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
    const result = await deleteKkudUser(deleteTarget.value.id)
    deleteOpen.value = false
    toast.success(`用户 ${deleteTarget.value.user} 已删除并归档（归档 #${result.action_job_id ?? '—'}）`)
    await loadKu(ku.offset)
  } catch (e) {
    deleteError.value = e.message
  } finally {
    deleting.value = false
  }
}

onMounted(() => loadKu(0))
</script>
