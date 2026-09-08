<template>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="notice" class="muted">{{ notice }}</p>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>手动操作</h2>
        <p class="muted">对绑定代理手动触发采集任务与冲突检测</p>
      </div>
      <button class="primary small" :disabled="!ownAgentId || detecting" @click="detect">
        {{ detecting ? '检测中…' : '手动检测冲突' }}
      </button>
    </div>
    <p v-if="!ownAgentId" class="muted">
      暂未识别到绑定的代理 ID（通常在产生冲突记录后可识别），手动检测入口暂不可用；任务仍可正常手动运行。
    </p>
    <EmptyState v-if="!tasks.length" title="暂无监控任务" hint="请联系管理员为本代理创建监控任务。" />
    <table v-else>
      <thead><tr><th>任务</th><th>代理</th><th>间隔</th><th>状态</th><th>下次运行</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="task in tasks" :key="task.id">
          <td>#{{ task.id }}</td>
          <td>代理 #{{ task.agent_id }}</td>
          <td>{{ task.interval_sec }} 秒</td>
          <td><StatusBadge :text="task.enabled ? '已启用' : '已停用'" :tone="task.enabled ? 'green' : 'gray'" /></td>
          <td>{{ formatDate(task.next_run_at) }}</td>
          <td>
            <button class="ghost small" :disabled="runningTask === task.id" @click="run(task.id)">
              {{ runningTask === task.id ? '运行中…' : '立即运行' }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>运行记录</h2>
        <p class="muted">最近 100 次采集与关联批次的状态、耗时与错误</p>
      </div>
      <button class="ghost small" @click="load">刷新</button>
    </div>
    <EmptyState v-if="!runs.length" title="暂无运行记录" hint="手动运行一次任务后可在这里查看结果。" />
    <table v-else>
      <thead><tr><th>批次</th><th>任务</th><th>状态</th><th>读取/保存</th><th>开始时间</th><th>耗时</th><th>错误</th></tr></thead>
      <tbody>
        <tr v-for="runItem in runs" :key="runItem.id">
          <td>#{{ runItem.id }}</td>
          <td>任务 #{{ runItem.task_id }}</td>
          <td><StatusBadge :text="runStatusText(runItem.status)" :tone="runStatusTone(runItem.status)" /></td>
          <td>{{ runItem.rows_read }} / {{ runItem.rows_saved }}</td>
          <td>{{ formatDate(runItem.started_at) }}</td>
          <td>{{ formatDuration(runItem.started_at, runItem.finished_at) }}</td>
          <td class="muted">{{ runItem.error || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import StatusBadge from '../../components/StatusBadge.vue'
import EmptyState from '../../components/EmptyState.vue'
import { listTasks, runTask, listSyncRuns, listConflicts, detectAgent } from '../../api'
import { formatDate, formatDuration, runStatusText, runStatusTone } from '../../utils/format'

const tasks = ref([])
const runs = ref([])
const ownAgentId = ref(0)
const runningTask = ref(0)
const detecting = ref(false)
const notice = ref('')
const error = ref('')

async function load() {
  error.value = ''
  const results = await Promise.allSettled([listTasks(), listSyncRuns()])
  if (results[0].status === 'fulfilled') tasks.value = results[0].value || []
  if (results[1].status === 'fulfilled') runs.value = results[1].value || []
  const failed = results.find((result) => result.status === 'rejected')
  if (failed) error.value = failed.reason.message
}

async function inferOwnAgent() {
  try {
    const data = await listConflicts({ limit: 1 })
    ownAgentId.value = data?.items?.[0]?.agent_id || 0
  } catch { /* 无法识别时保持检测入口不可用 */ }
}

async function run(id) {
  runningTask.value = id
  notice.value = ''
  error.value = ''
  try {
    const result = await runTask(id)
    notice.value = `任务 #${id} 完成：${runStatusText(result.status)}，读取 ${result.rows_read}，保存 ${result.rows_saved}${result.error ? `；${result.error}` : ''}`
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    runningTask.value = 0
  }
}

async function detect() {
  detecting.value = true
  notice.value = ''
  error.value = ''
  try {
    const summary = await detectAgent(ownAgentId.value)
    notice.value = `冲突检测完成：冲突 ${summary.conflicts} 个（新增 ${summary.created}，更新 ${summary.updated}，重开 ${summary.reopened}）`
  } catch (e) {
    error.value = e.message
  } finally {
    detecting.value = false
  }
}

onMounted(() => {
  load()
  inferOwnAgent()
})
</script>
