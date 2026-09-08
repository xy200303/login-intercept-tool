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
  <p v-if="error" class="error">{{ error }}</p>

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
import { listAgents, listTasks, listSyncRuns, listConflicts, testConnections } from '../api'
import { formatDate, formatDuration, runStatusText, runStatusTone } from '../utils/format'

const agents = ref([])
const tasks = ref([])
const runs = ref([])
const openConflicts = ref(0)
const error = ref('')
const testing = ref(false)
const connResult = ref(null)

const enabledAgents = computed(() => agents.value.filter((agent) => agent.enabled).length)
const lastRun = computed(() => runs.value[0] || null)

async function load() {
  const results = await Promise.allSettled([
    listAgents(), listTasks(), listSyncRuns(), listConflicts({ status: 'open', limit: 1 }),
  ])
  const failed = results.find((result) => result.status === 'rejected')
  if (failed) error.value = failed.reason.message
  if (results[0].status === 'fulfilled') agents.value = results[0].value || []
  if (results[1].status === 'fulfilled') tasks.value = results[1].value || []
  if (results[2].status === 'fulfilled') runs.value = results[2].value || []
  if (results[3].status === 'fulfilled') openConflicts.value = results[3].value?.total || 0
}

async function runTest() {
  testing.value = true
  connResult.value = null
  try {
    connResult.value = await testConnections()
  } catch (e) {
    error.value = e.message
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>
