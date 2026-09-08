<template>
  <section class="stats">
    <article>
      <span>待处理冲突</span>
      <strong>{{ openConflicts }}</strong>
      <small>需要复核</small>
    </article>
    <article>
      <span>冲突总数</span>
      <strong>{{ totalConflicts }}</strong>
      <small>本代理范围内</small>
    </article>
    <article>
      <span>关联结果</span>
      <strong>{{ matchCount }}</strong>
      <small>最近 200 条匹配</small>
    </article>
    <article>
      <span>最近一次同步</span>
      <strong v-if="lastRun">
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
        <h2>待处理冲突</h2>
        <p class="muted">本代理范围内最近的重复 IP 冲突</p>
      </div>
      <router-link to="/agent/conflicts"><button class="ghost small">进入冲突处理</button></router-link>
    </div>
    <EmptyState v-if="!conflicts.length" title="暂无待处理冲突" hint="运行任务并触发检测后，命中的冲突会出现在这里。" />
    <table v-else>
      <thead><tr><th>IP</th><th>类型</th><th>风险</th><th>成员数</th><th>最近命中</th></tr></thead>
      <tbody>
        <tr v-for="row in conflicts" :key="row.id">
          <td class="mono">{{ row.ip }}</td>
          <td>{{ conflictTypeText(row.conflict_type) }}</td>
          <td><StatusBadge :text="riskText(row.risk)" :tone="riskTone(row.risk)" /></td>
          <td>{{ row.member_count }}</td>
          <td>{{ formatDate(row.last_seen_at) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import StatusBadge from '../../components/StatusBadge.vue'
import EmptyState from '../../components/EmptyState.vue'
import { listConflicts, listMatches, listSyncRuns } from '../../api'
import { formatDate, runStatusText, runStatusTone, riskText, riskTone, conflictTypeText } from '../../utils/format'

const openConflicts = ref(0)
const totalConflicts = ref(0)
const matchCount = ref(0)
const conflicts = ref([])
const lastRun = ref(null)
const error = ref('')

onMounted(async () => {
  const results = await Promise.allSettled([
    listConflicts({ status: 'open', limit: 5 }),
    listConflicts({ limit: 1 }),
    listMatches(),
    listSyncRuns(),
  ])
  if (results[0].status === 'fulfilled') {
    openConflicts.value = results[0].value?.total || 0
    conflicts.value = results[0].value?.items || []
  }
  if (results[1].status === 'fulfilled') totalConflicts.value = results[1].value?.total || 0
  if (results[2].status === 'fulfilled') matchCount.value = (results[2].value || []).length
  if (results[3].status === 'fulfilled') lastRun.value = (results[3].value || [])[0] || null
  const failed = results.find((result) => result.status === 'rejected')
  if (failed) error.value = failed.reason.message
})
</script>
