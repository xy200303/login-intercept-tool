<template>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="notice" class="muted">{{ notice }}</p>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>kkud 用户快照</h2>
        <p class="muted">采集到的 kkud.user568531942 最近快照，手机号已脱敏</p>
      </div>
      <button class="ghost small" @click="loadSnapshots">刷新</button>
    </div>
    <form class="inline-form" @submit.prevent="loadSnapshots">
      <label>手机号<input v-model.trim="filters.mobile" placeholder="完整手机号精确匹配" /></label>
      <label>代理值（daili）<input v-model.trim="filters.agent_value" placeholder="如 agent_a" /></label>
      <label>条数<input v-model.number="filters.limit" type="number" min="1" max="500" /></label>
      <button class="primary small" type="submit">查询</button>
    </form>
    <EmptyState v-if="!snapshots.length" title="暂无快照" hint="运行监控任务后，采集到的 kkud 用户会出现在这里。" />
    <table v-else>
      <thead>
        <tr><th>来源 ID</th><th>代理值</th><th>手机号</th><th>QQ</th><th>邮箱</th><th>采集时间</th><th v-if="auth.isSuperAdmin">操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="row in snapshots" :key="row.id">
          <td class="mono">{{ row.source_pk }}</td>
          <td>{{ row.agent_value || '—' }}</td>
          <td class="mono">{{ maskMobile(row.mobile) }}</td>
          <td>{{ row.qq || '—' }}</td>
          <td>{{ row.email || '—' }}</td>
          <td>{{ formatDate(row.captured_at) }}</td>
          <td v-if="auth.isSuperAdmin">
            <button class="ghost small" @click="openVip(row)">开通/修改 VIP</button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>关联结果</h2>
        <p class="muted">kkud 用户与 fenx 账号的最近 200 条匹配，低置信度结果只做复核不自动处理</p>
      </div>
      <button class="ghost small" @click="loadMatches">刷新</button>
    </div>
    <EmptyState v-if="!matches.length" title="暂无关联结果" hint="任务运行并命中手机号 / QQ / 邮箱后生成。" />
    <table v-else>
      <thead><tr><th>批次</th><th>快照</th><th>fenx 账号</th><th>置信度</th><th>命中字段</th><th>状态</th><th>时间</th></tr></thead>
      <tbody>
        <tr v-for="row in matches" :key="row.id">
          <td>#{{ row.run_id }}</td>
          <td class="mono">{{ row.kkud_snapshot_id }}</td>
          <td class="mono">{{ row.fenx_user_id }}</td>
          <td>{{ Math.round((row.confidence || 0) * 100) }}%</td>
          <td>{{ row.matched_fields || '—' }}</td>
          <td><StatusBadge :text="matchStateText(row.state)" :tone="matchStateTone(row.state)" /></td>
          <td>{{ formatDate(row.created_at) }}</td>
        </tr>
      </tbody>
    </table>
  </section>

  <Modal :open="vipOpen" title="修改 kkud 用户 VIP" @close="vipOpen = false">
    <p>用户来源 ID：<strong class="mono">{{ vipForm.sourceId }}</strong></p>
    <label class="field">
      VIP 值
      <input v-model.trim="vipForm.value" placeholder="如 1 / vip 等级值" required />
    </label>
    <p class="muted">该操作会直接更新 kkud 外部库的 VIP 字段，并记录审计事件。</p>
    <p v-if="vipError" class="error">{{ vipError }}</p>
    <template #footer>
      <button class="ghost" @click="vipOpen = false">取消</button>
      <button class="primary" :disabled="!vipForm.value || vipSaving" @click="submitVip">
        {{ vipSaving ? '提交中…' : '确认修改' }}
      </button>
    </template>
  </Modal>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Modal from '../components/Modal.vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { listSnapshots, listMatches, updateVip } from '../api'
import { formatDate, maskMobile, matchStateText, matchStateTone } from '../utils/format'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const snapshots = ref([])
const matches = ref([])
const filters = reactive({ mobile: '', agent_value: '', limit: 200 })
const error = ref('')
const notice = ref('')

const vipOpen = ref(false)
const vipForm = reactive({ sourceId: '', value: '' })
const vipSaving = ref(false)
const vipError = ref('')

async function loadSnapshots() {
  error.value = ''
  try {
    const params = { limit: filters.limit || 200 }
    if (filters.mobile) params.mobile = filters.mobile
    if (filters.agent_value) params.agent_value = filters.agent_value
    snapshots.value = await listSnapshots(params) || []
  } catch (e) {
    error.value = e.message
  }
}

async function loadMatches() {
  error.value = ''
  try {
    matches.value = await listMatches() || []
  } catch (e) {
    error.value = e.message
  }
}

function openVip(row) {
  vipForm.sourceId = row.source_pk
  vipForm.value = ''
  vipError.value = ''
  vipOpen.value = true
}

async function submitVip() {
  vipSaving.value = true
  vipError.value = ''
  try {
    await updateVip(vipForm.sourceId, vipForm.value)
    vipOpen.value = false
    notice.value = `用户 ${vipForm.sourceId} 的 VIP 已更新`
  } catch (e) {
    vipError.value = e.message
  } finally {
    vipSaving.value = false
  }
}

onMounted(() => {
  loadSnapshots()
  loadMatches()
})
</script>
