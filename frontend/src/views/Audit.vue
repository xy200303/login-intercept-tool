<template>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="notice" class="muted">{{ notice }}</p>

  <section class="stats">
    <article>
      <span>平台健康</span>
      <strong>
        <StatusBadge :text="health === 'ok' ? '正常' : health ? '降级' : '未知'" :tone="health === 'ok' ? 'green' : 'orange'" />
      </strong>
      <small>GET /health</small>
    </article>
    <article>
      <span>kkud 数据源</span>
      <strong>
        <StatusBadge v-if="conn" :text="conn.kkud?.ok ? '连接正常' : '连接失败'" :tone="conn.kkud?.ok ? 'green' : 'red'" />
        <span v-else class="muted" style="font-size:16px">未检测</span>
      </strong>
      <small>{{ conn?.kkud?.ok === false ? conn.kkud.error : 'MySQL 只读采集' }}</small>
    </article>
    <article>
      <span>fenx_site 数据源</span>
      <strong>
        <StatusBadge v-if="conn" :text="conn.fenx_site?.ok ? '连接正常' : '连接失败'" :tone="conn.fenx_site?.ok ? 'green' : 'red'" />
        <span v-else class="muted" style="font-size:16px">未检测</span>
      </strong>
      <small>{{ conn?.fenx_site?.ok === false ? conn.fenx_site.error : 'MySQL 用户库' }}</small>
    </article>
    <article>
      <span>操作</span>
      <strong style="font-size:16px">
        <button class="primary small" :disabled="testing" @click="runTest">{{ testing ? '检测中…' : '连接测试' }}</button>
      </strong>
      <small>检测两个 MySQL 数据源</small>
    </article>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>审计日志</h2>
        <p class="muted">平台关键操作的不可变审计事件</p>
      </div>
      <button class="ghost small" @click="loadAudit(0)">刷新</button>
    </div>
    <EmptyState v-if="!audit.items.length" title="暂无审计事件" hint="执行检测、处理或账号变更后自动生成。" />
    <template v-else>
      <table>
        <thead><tr><th>时间</th><th>操作者</th><th>动作</th><th>目标</th><th>详情</th></tr></thead>
        <tbody>
          <tr v-for="event in audit.items" :key="event.id">
            <td>{{ formatDate(event.created_at) }}</td>
            <td>{{ event.actor_id ? `#${event.actor_id}` : '系统' }}</td>
            <td class="mono">{{ event.action }}</td>
            <td class="mono">{{ event.target }}</td>
            <td class="muted">{{ event.detail || '—' }}</td>
          </tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost small" :disabled="audit.offset === 0" @click="loadAudit(audit.offset - 50)">上一页</button>
        <span class="muted">共 {{ audit.total }} 条 · 第 {{ Math.floor(audit.offset / 50) + 1 }} 页</span>
        <button class="ghost small" :disabled="audit.offset + 50 >= audit.total" @click="loadAudit(audit.offset + 50)">下一页</button>
      </div>
    </template>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>白名单</h2>
        <p class="muted">白名单账号 / IP 不参与冲突自动处理，必须设置过期时间</p>
      </div>
      <button class="primary small" @click="showForm = !showForm">新增白名单</button>
    </div>
    <form v-if="showForm" class="inline-form" @submit.prevent="createEntry">
      <label>
        代理
        <select v-model.number="form.agent_id" required>
          <option :value="0" disabled>请选择代理</option>
          <option v-for="agent in agents" :key="agent.id" :value="agent.id">{{ agent.display_name }}</option>
        </select>
      </label>
      <label>fenx UID<input v-model.trim="form.fenx_uid" placeholder="账号或 IP 至少填一个" /></label>
      <label>IP<input v-model.trim="form.ip" /></label>
      <label>原因<input v-model.trim="form.reason" placeholder="加入白名单的原因" /></label>
      <label>过期时间<input v-model="form.expires_at" type="datetime-local" required /></label>
      <button class="primary small" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
    </form>
    <EmptyState v-if="!allowlist.length" title="暂无白名单" hint="对需要豁免冲突处理的账号或 IP 新增白名单。" />
    <table v-else>
      <thead><tr><th>代理</th><th>fenx UID</th><th>IP</th><th>原因</th><th>过期时间</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="entry in allowlist" :key="entry.id">
          <td>{{ agentName(entry.agent_id) }}</td>
          <td class="mono">{{ entry.fenx_uid || '—' }}</td>
          <td class="mono">{{ entry.ip || '—' }}</td>
          <td>{{ entry.reason || '—' }}</td>
          <td>{{ formatDate(entry.expires_at) }}</td>
          <td>{{ formatDate(entry.created_at) }}</td>
          <td>
            <button class="ghost small" :disabled="removing === entry.id" @click="removeEntry(entry)">
              {{ removing === entry.id ? '删除中…' : '删除' }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import {
  listAuditEvents, listAllowlist, createAllowlist, deleteAllowlist, listAgents, testConnections, fetchHealth,
} from '../api'
import { formatDate } from '../utils/format'

const audit = reactive({ items: [], total: 0, offset: 0 })
const allowlist = ref([])
const agents = ref([])
const health = ref('')
const conn = ref(null)
const testing = ref(false)
const error = ref('')
const notice = ref('')

const showForm = ref(false)
const form = reactive({ agent_id: 0, fenx_uid: '', ip: '', reason: '', expires_at: '' })
const saving = ref(false)
const removing = ref(0)

async function loadAudit(nextOffset = 0) {
  error.value = ''
  try {
    const data = await listAuditEvents({ limit: 50, offset: Math.max(0, nextOffset) })
    audit.items = data.items || []
    audit.total = data.total || 0
    audit.offset = data.offset || 0
  } catch (e) {
    error.value = e.message
  }
}

async function loadAllowlist() {
  try {
    const data = await listAllowlist()
    allowlist.value = data.items || []
  } catch (e) {
    error.value = e.message
  }
}

async function createEntry() {
  saving.value = true
  error.value = ''
  try {
    await createAllowlist({
      agent_id: form.agent_id,
      fenx_uid: form.fenx_uid,
      ip: form.ip,
      reason: form.reason,
      expires_at: new Date(form.expires_at).toISOString(),
    })
    showForm.value = false
    form.fenx_uid = ''
    form.ip = ''
    form.reason = ''
    form.expires_at = ''
    notice.value = '白名单已创建'
    await loadAllowlist()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function removeEntry(entry) {
  removing.value = entry.id
  error.value = ''
  try {
    await deleteAllowlist(entry.id)
    notice.value = '白名单已删除'
    await loadAllowlist()
  } catch (e) {
    error.value = e.message
  } finally {
    removing.value = 0
  }
}

async function runTest() {
  testing.value = true
  try {
    conn.value = await testConnections()
  } catch (e) {
    error.value = e.message
  } finally {
    testing.value = false
  }
}

function agentName(id) {
  const agent = agents.value.find((item) => item.id === id)
  return agent ? agent.display_name : `代理 #${id}`
}

onMounted(async () => {
  loadAudit(0)
  loadAllowlist()
  try {
    agents.value = await listAgents() || []
  } catch { /* 代理列表不可用时白名单仍可按 ID 显示 */ }
  try {
    health.value = (await fetchHealth())?.status || ''
  } catch { health.value = '' }
})
</script>
