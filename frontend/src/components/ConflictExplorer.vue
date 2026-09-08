<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>冲突列表</h2>
        <p class="muted">同一代理范围内重复注册 / 登录 IP 的治理入口</p>
      </div>
      <button class="ghost small" :disabled="loading" @click="load(0)">{{ loading ? '加载中…' : '刷新' }}</button>
    </div>

    <form class="inline-form" @submit.prevent="load(0)">
      <label v-if="showAgentFilter">
        代理
        <select v-model="filters.agent_id">
          <option value="">全部代理</option>
          <option v-for="agent in agents" :key="agent.id" :value="String(agent.id)">{{ agent.display_name }}</option>
        </select>
      </label>
      <label>
        状态
        <select v-model="filters.status">
          <option value="">全部状态</option>
          <option value="open">待处理</option>
          <option value="resolved">已解决</option>
        </select>
      </label>
      <label>
        风险
        <select v-model="filters.risk">
          <option value="">全部风险</option>
          <option value="high">高风险</option>
          <option value="medium">中风险</option>
          <option value="low">低风险</option>
        </select>
      </label>
      <label>
        IP
        <input v-model.trim="filters.ip" placeholder="精确匹配 IP" />
      </label>
      <button class="primary small" type="submit">查询</button>
    </form>

    <p v-if="error" class="error">{{ error }}</p>
    <EmptyState
      v-else-if="!rows.length && !loading"
      title="暂无冲突记录"
      hint="可以先运行监控任务并触发冲突检测；命中重复 IP 后会出现在这里。"
    />
    <template v-else>
      <table>
        <thead>
          <tr>
            <th>IP</th><th>类型</th><th>风险</th><th>成员数</th><th>状态</th><th>保留账号</th><th>最近命中</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id" class="clickable" @click="openDetail(row)">
            <td class="mono">{{ row.ip }}</td>
            <td>{{ conflictTypeText(row.conflict_type) }}</td>
            <td><StatusBadge :text="riskText(row.risk)" :tone="riskTone(row.risk)" /></td>
            <td>{{ row.member_count }}</td>
            <td><StatusBadge :text="conflictStatusText(row.status)" :tone="conflictStatusTone(row.status)" /></td>
            <td class="mono">{{ row.winner_uid || '—' }}</td>
            <td>{{ formatDate(row.last_seen_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost small" :disabled="offset === 0" @click="load(offset - limit)">上一页</button>
        <span class="muted">共 {{ total }} 条 · 第 {{ Math.floor(offset / limit) + 1 }} 页</span>
        <button class="ghost small" :disabled="offset + limit >= total" @click="load(offset + limit)">下一页</button>
      </div>
    </template>
  </section>

  <teleport to="body">
    <transition name="slide">
      <div v-if="drawerOpen" class="drawer-overlay" @click.self="drawerOpen = false">
        <aside class="drawer" role="dialog" aria-modal="true">
          <header class="drawer-head">
            <div>
              <h3>冲突详情 <span class="mono">{{ selected?.ip }}</span></h3>
              <p class="muted">#{{ selected?.id }} · {{ conflictTypeText(selected?.conflict_type) }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="drawerOpen = false">×</button>
          </header>

          <div v-if="detailLoading" class="empty">加载中…</div>
          <div v-else-if="selected" class="drawer-body">
            <div class="chip-row">
              <StatusBadge :text="riskText(selected.risk)" :tone="riskTone(selected.risk)" />
              <StatusBadge :text="conflictStatusText(selected.status)" :tone="conflictStatusTone(selected.status)" />
              <span class="chip">成员 {{ members.length }} 个账号</span>
              <span class="chip">winner：{{ selected.winner_uid || '—' }}</span>
            </div>
            <p class="muted">首次命中 {{ formatDate(selected.first_seen_at) }} · 最近命中 {{ formatDate(selected.last_seen_at) }}</p>

            <h4>冲突成员</h4>
            <article v-for="member in members" :key="member.id" class="member-card" :class="{ winner: member.is_winner }">
              <header>
                <strong class="mono">{{ member.fenx_uid }}</strong>
                <span>{{ member.username || '—' }}</span>
                <StatusBadge v-if="member.is_winner" text="保留账号" tone="blue" />
                <StatusBadge :text="matchStateText(member.match_state)" :tone="matchStateTone(member.match_state)" />
              </header>
              <p class="muted">注册 IP：{{ member.reg_ip || '—' }} · 当前登录 IP：{{ member.login_ip || '—' }}</p>
              <ul v-if="evidenceOf(member).length" class="timeline">
                <li v-for="(ev, index) in evidenceOf(member)" :key="index">
                  <span class="timeline-dot"></span>
                  <span>{{ evidenceSourceText(ev.source) }}</span>
                  <span class="mono">{{ ev.ip }}</span>
                  <span class="muted">{{ ev.time || '时间未知' }}</span>
                </li>
              </ul>
            </article>

            <div class="drawer-actions">
              <button class="ghost small" :disabled="previewLoading" @click="runPreview">
                {{ previewLoading ? '预览中…' : '预览处理影响' }}
              </button>
              <button
                v-if="canExecute"
                class="danger small"
                :disabled="!preview"
                :title="preview ? '' : '请先预览'"
                @click="openExec"
              >
                执行处理
              </button>
            </div>
            <p v-if="previewError" class="error">{{ previewError }}</p>

            <div v-if="preview" class="preview-box">
              <h4>预览结果（未写入任何数据）</h4>
              <p>
                当前策略：<StatusBadge :text="actionText(preview.policy)" :tone="preview.policy === 'alert' ? 'gray' : preview.policy === 'disable' ? 'orange' : 'red'" />
                · 将影响 <strong>{{ preview.affected?.length || 0 }}</strong> 个账号 · 跳过 {{ preview.skipped?.length || 0 }} 个
              </p>
              <p class="muted">恢复能力：禁用可撤销；删除仅能通过归档走人工恢复流程，不可无条件恢复。</p>
              <ul v-if="preview.affected?.length" class="uid-list">
                <li v-for="member in preview.affected" :key="member.fenx_uid">
                  <span class="mono">{{ member.fenx_uid }}</span> {{ member.username }}
                </li>
              </ul>
              <ul v-if="preview.skipped?.length" class="uid-list skipped">
                <li v-for="(item, index) in preview.skipped" :key="index">
                  <span class="mono">{{ item.fenx_uid }}</span> {{ item.username }} — {{ skipReasonText(item.reason) }}
                </li>
              </ul>
            </div>
          </div>
        </aside>
      </div>
    </transition>
  </teleport>

  <Modal :open="execOpen" danger title="确认执行冲突处理" @close="execOpen = false">
    <template v-if="!execResult">
      <p>
        即将对冲突 IP <strong class="mono">{{ selected?.ip }}</strong> 按策略「{{ actionText(preview?.policy) }}」处理
        <strong>{{ preview?.affected?.length || 0 }}</strong> 个账号（保留 winner：{{ preview?.winner || '—' }}）。
      </p>
      <p class="muted">禁用动作可撤销；删除动作不可直接撤销。执行会记录审计事件，幂等键已自动生成。</p>
      <label class="field">
        请输入冲突 IP（{{ selected?.ip }}）以确认
        <input v-model.trim="ipConfirm" :placeholder="selected?.ip" />
      </label>
      <label class="check">
        <input v-model="confirmChecked" type="checkbox" />
        我已了解影响范围与恢复能力，确认执行
      </label>
      <p v-if="execError" class="error">{{ execError }}</p>
    </template>
    <template v-else>
      <p v-if="execResult.message" class="muted">{{ execResult.message }}</p>
      <p v-else>
        策略「{{ actionText(execResult.policy) }}」：成功 {{ succeededJobs }} 个，失败 {{ execResult.failed || 0 }} 个，跳过 {{ execResult.skipped?.length || 0 }} 个。
      </p>
      <table v-if="execResult.jobs?.length">
        <thead><tr><th>账号</th><th>动作</th><th>状态</th><th>说明</th><th v-if="canExecute">操作</th></tr></thead>
        <tbody>
          <tr v-for="job in execResult.jobs" :key="job.id">
            <td class="mono">{{ job.target_uid }}</td>
            <td>{{ actionText(job.action) }}</td>
            <td><StatusBadge :text="jobStatusText(job.status)" :tone="jobStatusTone(job.status)" /></td>
            <td class="muted">{{ job.error || '—' }}</td>
            <td v-if="canExecute">
              <button
                v-if="job.status === 'failed' && job.action === 'disable'"
                class="ghost small"
                :disabled="jobBusy === job.id"
                @click="retryJob(job)"
              >{{ jobBusy === job.id ? '重试中…' : '重试' }}</button>
              <button
                v-if="job.status === 'succeeded' && job.action === 'disable'"
                class="ghost small"
                :disabled="jobBusy === job.id"
                @click="undoJob(job)"
              >{{ jobBusy === job.id ? (undoConfirm === job.id ? '确认？' : '处理中…') : (undoConfirm === job.id ? '确认撤销' : '撤销禁用') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>
    <template #footer>
      <button class="ghost" @click="execOpen = false">{{ execResult ? '关闭' : '取消' }}</button>
      <button
        v-if="!execResult"
        class="danger"
        :disabled="!canSubmitExec"
        @click="doExec"
      >{{ submitting ? '执行中…' : '确认执行' }}</button>
    </template>
  </Modal>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import Modal from './Modal.vue'
import StatusBadge from './StatusBadge.vue'
import EmptyState from './EmptyState.vue'
import {
  listAgents, listConflicts, conflictDetail, previewConflict, executeConflict, retryAction, undoDisable,
} from '../api'
import {
  formatDate, riskText, riskTone, conflictStatusText, conflictStatusTone, conflictTypeText,
  matchStateText, matchStateTone, evidenceSourceText, actionText, jobStatusText, jobStatusTone,
  skipReasonText, newIdempotencyKey,
} from '../utils/format'

const props = defineProps({
  canExecute: { type: Boolean, default: false },
  showAgentFilter: { type: Boolean, default: false },
})

const agents = ref([])
const filters = reactive({ agent_id: '', status: '', risk: '', ip: '' })
const rows = ref([])
const total = ref(0)
const limit = 50
const offset = ref(0)
const loading = ref(false)
const error = ref('')

const drawerOpen = ref(false)
const detailLoading = ref(false)
const selected = ref(null)
const members = ref([])
const preview = ref(null)
const previewLoading = ref(false)
const previewError = ref('')

const execOpen = ref(false)
const ipConfirm = ref('')
const confirmChecked = ref(false)
const submitting = ref(false)
const execError = ref('')
const execResult = ref(null)
const idemKey = ref('')
const jobBusy = ref(0)
const undoConfirm = ref(0)

const succeededJobs = computed(() => (execResult.value?.jobs || []).filter((job) => job.status === 'succeeded').length)
const canSubmitExec = computed(
  () => !submitting.value && confirmChecked.value && ipConfirm.value === selected.value?.ip
)

async function loadAgents() {
  if (!props.showAgentFilter) return
  try {
    agents.value = await listAgents()
  } catch {
    agents.value = []
  }
}

async function load(nextOffset = 0) {
  loading.value = true
  error.value = ''
  try {
    const params = { limit, offset: Math.max(0, nextOffset) }
    for (const key of ['agent_id', 'status', 'risk', 'ip']) {
      if (filters[key]) params[key] = filters[key]
    }
    const data = await listConflicts(params)
    rows.value = data.items || []
    total.value = data.total || 0
    offset.value = data.offset || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function openDetail(row) {
  selected.value = row
  members.value = []
  preview.value = null
  previewError.value = ''
  drawerOpen.value = true
  detailLoading.value = true
  try {
    const data = await conflictDetail(row.id)
    selected.value = data.conflict
    members.value = data.members || []
  } catch (e) {
    previewError.value = e.message
  } finally {
    detailLoading.value = false
  }
}

function evidenceOf(member) {
  if (!member.evidence_json) return []
  try {
    const parsed = JSON.parse(member.evidence_json)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

async function runPreview() {
  previewLoading.value = true
  previewError.value = ''
  try {
    preview.value = await previewConflict(selected.value.id)
  } catch (e) {
    previewError.value = e.message
  } finally {
    previewLoading.value = false
  }
}

function openExec() {
  idemKey.value = newIdempotencyKey()
  ipConfirm.value = ''
  confirmChecked.value = false
  execError.value = ''
  execResult.value = null
  execOpen.value = true
}

async function doExec() {
  submitting.value = true
  execError.value = ''
  try {
    execResult.value = await executeConflict(selected.value.id, idemKey.value)
    load(offset.value)
  } catch (e) {
    execError.value = e.message
  } finally {
    submitting.value = false
  }
}

async function retryJob(job) {
  jobBusy.value = job.id
  try {
    const updated = await retryAction(job.id)
    Object.assign(job, updated)
  } catch (e) {
    job.error = e.message
  } finally {
    jobBusy.value = 0
  }
}

async function undoJob(job) {
  if (undoConfirm.value !== job.id) {
    undoConfirm.value = job.id
    setTimeout(() => { if (undoConfirm.value === job.id) undoConfirm.value = 0 }, 3000)
    return
  }
  undoConfirm.value = 0
  jobBusy.value = job.id
  try {
    const updated = await undoDisable(job.id)
    Object.assign(job, updated)
  } catch (e) {
    job.error = e.message
  } finally {
    jobBusy.value = 0
  }
}

onMounted(() => {
  loadAgents()
  load(0)
})
</script>
