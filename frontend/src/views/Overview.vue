<template>
  <section class="stats">
    <article>
      <span>启用代理</span>
      <strong>{{ enabledAgents }}</strong>
      <small>共 {{ agents.length }} 个配置</small>
    </article>
    <article>
      <span>监控任务</span>
      <strong>{{ tasks.length }}</strong>
      <small>定时采集与关联</small>
    </article>
    <article>
      <span>待处理冲突</span>
      <strong>{{ openConflicts }}</strong>
      <small>需要复核处理</small>
    </article>
    <article>
      <span>最近同步</span>
      <strong v-if="lastRun" class="sync-status">
        <StatusBadge :text="runStatusText(lastRun.status)" :tone="runStatusTone(lastRun.status)" />
      </strong>
      <strong v-else>—</strong>
      <small>{{ lastRun ? formatDate(lastRun.started_at) : '暂无运行记录' }}</small>
    </article>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>外部库连接状态</h2>
        <p class="muted">检测 kkud 与 fenx_site 两个 MySQL 数据源的连通性</p>
      </div>
      <button class="primary small" :disabled="testing" @click="runTest">{{ testing ? '检测中…' : '连接测试' }}</button>
    </div>
    <div v-if="connResult" class="chip-row" style="margin-top:18px">
      <span v-for="name in ['kkud', 'fenx_site']" :key="name" class="chip">
        {{ name }}：
        <StatusBadge
          :text="connResult[name]?.ok ? '连接正常' : '连接失败'"
          :tone="connResult[name]?.ok ? 'green' : 'red'"
        />
      </span>
    </div>
    <p v-for="name in ['kkud', 'fenx_site']" :key="name + '-err'" class="error">
      {{ connResult?.[name]?.ok === false ? `${name}：${connResult[name].error}` : '' }}
    </p>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>联盟接入状态</h2>
        <p class="muted">PHP 站点调用决策接口的实时情况（近 24 小时统计）</p>
      </div>
      <button class="ghost small" @click="load">刷新</button>
    </div>
    <div class="chip-row" style="margin-top:18px">
      <span class="chip">
        连接状态：
        <StatusBadge
          :text="decision.connected ? '在线' : (decision.last_event_at ? '超过 10 分钟无调用' : '从未接入')"
          :tone="decision.connected ? 'green' : (decision.last_event_at ? 'orange' : 'gray')"
        />
      </span>
      <span class="chip">最近调用：{{ decision.last_event_at ? formatDate(decision.last_event_at) : '—' }}</span>
      <span class="chip">近 24 小时调用：<strong>{{ decision.day_total }}</strong> 次</span>
      <span class="chip">其中拦截：<strong>{{ decision.day_denied }}</strong> 次</span>
    </div>
    <EmptyState
      v-if="!decision.recent.length"
      title="暂无决策调用记录"
      hint="PHP 站点接入 fenx_guard 后，用户登录/注册时会在这里留下记录。"
    />
    <table v-else style="margin-top:18px">
      <thead><tr><th>时间</th><th>动作</th><th>账号</th><th>IP</th><th>结果</th><th>原因</th></tr></thead>
      <tbody>
        <tr v-for="event in decision.recent" :key="event.id">
          <td>{{ formatDate(event.created_at) }}</td>
          <td>{{ { login: '登录', register: '注册' }[event.action] || event.action || '—' }}</td>
          <td class="mono">{{ event.account_id || '—' }}</td>
          <td class="mono">{{ event.ip || '—' }}</td>
          <td><StatusBadge :text="decisionText(event.decision)" :tone="decisionTone(event.decision)" /></td>
          <td class="muted">{{ decisionReasonText(event.reason) }}</td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>最近运行记录</h2>
        <p class="muted">采集与关联任务的最新批次</p>
      </div>
      <router-link to="/agents"><button class="ghost small">查看全部任务</button></router-link>
    </div>
    <EmptyState v-if="!runs.length" title="暂无运行记录" hint="到「代理与任务」创建监控任务并手动运行一次。" />
    <table v-else>
      <thead><tr><th>批次</th><th>任务</th><th>状态</th><th>读取/保存</th><th>开始时间</th><th>耗时</th><th>错误</th></tr></thead>
      <tbody>
        <tr v-for="run in runs.slice(0, 8)" :key="run.id">
          <td>#{{ run.id }}</td>
          <td>任务 #{{ run.task_id }}</td>
          <td><StatusBadge :text="runStatusText(run.status)" :tone="runStatusTone(run.status)" /></td>
          <td>{{ run.rows_read }} / {{ run.rows_saved }}</td>
          <td>{{ formatDate(run.started_at) }}</td>
          <td>{{ formatDuration(run.started_at, run.finished_at) }}</td>
          <td class="muted">{{ run.error || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { listAgents, listTasks, listSyncRuns, listConflicts, testConnections, decisionStatus } from '../api'
import { formatDate, formatDuration, runStatusText, runStatusTone, decisionText, decisionTone, decisionReasonText } from '../utils/format'
import { toast } from '../utils/toast'

const agents = ref([])
const tasks = ref([])
const runs = ref([])
const openConflicts = ref(0)
const testing = ref(false)
const connResult = ref(null)
const decision = ref({ connected: false, last_event_at: null, day_total: 0, day_denied: 0, recent: [] })

const enabledAgents = computed(() => agents.value.filter((agent) => agent.enabled).length)
const lastRun = computed(() => runs.value[0] || null)

async function load() {
  const results = await Promise.allSettled([
    listAgents(), listTasks(), listSyncRuns(), listConflicts({ status: 'open', limit: 1 }), decisionStatus(),
  ])
  const failed = results.find((result) => result.status === 'rejected')
  if (failed) toast.error(failed.reason.message)
  if (results[0].status === 'fulfilled') agents.value = results[0].value || []
  if (results[1].status === 'fulfilled') tasks.value = results[1].value || []
  if (results[2].status === 'fulfilled') runs.value = results[2].value || []
  if (results[3].status === 'fulfilled') openConflicts.value = results[3].value?.total || 0
  if (results[4].status === 'fulfilled') {
    const data = results[4].value || {}
    decision.value = { connected: false, last_event_at: null, day_total: 0, day_denied: 0, recent: [], ...data }
  }
}

async function runTest() {
  testing.value = true
  connResult.value = null
  try {
    connResult.value = await testConnections()
  } catch (e) {
    toast.error(e.message)
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>
