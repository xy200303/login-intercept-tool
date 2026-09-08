<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>代理配置</h2>
        <p class="muted">代理名称、daili 匹配值与冲突处理策略</p>
      </div>
      <button class="primary small" @click="showAgentForm = !showAgentForm">新增代理</button>
    </div>
    <form v-if="showAgentForm" class="inline-form" @submit.prevent="createAgentSubmit">
      <label>代理名称<input v-model.trim="agentForm.name" placeholder="展示名称" required /></label>
      <label>
        daili 匹配值
        <input v-model.trim="agentForm.sources" placeholder="多个值用逗号分隔" required />
      </label>
      <label>
        冲突策略
        <select v-model="agentForm.action">
          <option value="alert">仅告警</option>
          <option value="disable">禁用账号</option>
          <option value="delete">删除账号</option>
        </select>
      </label>
      <button class="primary small" :disabled="savingAgent">{{ savingAgent ? '保存中…' : '保存' }}</button>
    </form>
    <SkeletonTable v-if="loading && !agents.length" :cols="6" />
    <EmptyState v-else-if="!agents.length" title="暂无代理配置" hint="先新增一个代理，系统才能按 daili 值采集 kkud 用户。" />
    <table v-else>
      <thead><tr><th>代理名称</th><th>匹配值</th><th>策略</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="agent in agents" :key="agent.id">
          <td>{{ agent.display_name }}</td>
          <td>{{ parseSourceValues(agent.source_values).join('、') || '—' }}</td>
          <td><StatusBadge :text="policyLabel(agent.policy_json)" :tone="policyTone(agent.policy_json)" /></td>
          <td><StatusBadge :text="agent.enabled ? '监控中' : '已停用'" :tone="agent.enabled ? 'green' : 'gray'" /></td>
          <td>{{ formatDate(agent.created_at) }}</td>
          <td>
            <button class="ghost small" :disabled="detecting === agent.id" @click="detect(agent)">
              {{ detecting === agent.id ? '检测中…' : '手动检测冲突' }}
            </button>
            <button class="danger small" @click="openDeleteAgent(agent)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>监控任务</h2>
        <p class="muted">按代理筛选 kkud.user568531942.daili，可手动立即运行</p>
      </div>
      <button class="primary small" @click="showTaskForm = !showTaskForm">新增任务</button>
    </div>
    <form v-if="showTaskForm" class="inline-form" @submit.prevent="createTaskSubmit">
      <label>
        代理
        <select v-model.number="taskForm.agentId" required>
          <option :value="0" disabled>请选择代理</option>
          <option v-for="agent in agents" :key="agent.id" :value="agent.id">{{ agent.display_name }}</option>
        </select>
      </label>
      <label>间隔（秒）<input v-model.number="taskForm.interval" type="number" min="300" step="60" required /></label>
      <button class="primary small" :disabled="savingTask">{{ savingTask ? '创建中…' : '创建' }}</button>
    </form>
    <SkeletonTable v-if="loading && !tasks.length" :cols="6" />
    <EmptyState v-else-if="!tasks.length" title="暂无监控任务" hint="为代理创建一个定时采集任务。" />
    <table v-else>
      <thead><tr><th>任务</th><th>代理</th><th>间隔</th><th>状态</th><th>下次运行</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="task in tasks" :key="task.id">
          <td>#{{ task.id }}</td>
          <td>{{ agentName(task.agent_id) }}</td>
          <td>{{ task.interval_sec }} 秒</td>
          <td><StatusBadge :text="task.enabled ? '已启用' : '已停用'" :tone="task.enabled ? 'green' : 'gray'" /></td>
          <td>{{ formatDate(task.next_run_at) }}</td>
          <td>
            <button class="ghost small" :disabled="runningTask === task.id" @click="run(task.id)">
              {{ runningTask === task.id ? '运行中…' : '立即运行' }}
            </button>
            <button class="danger small" @click="openDeleteTask(task)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>运行记录</h2>
        <p class="muted">最近 100 次采集与关联批次</p>
      </div>
      <button class="ghost small" @click="load">刷新</button>
    </div>
    <SkeletonTable v-if="loading && !runs.length" :cols="7" />
    <EmptyState v-else-if="!runs.length" title="暂无运行记录" hint="运行一次任务后可在这里查看状态与错误。" />
    <table v-else>
      <thead><tr><th>批次</th><th>任务</th><th>状态</th><th>读取/保存</th><th>开始时间</th><th>耗时</th><th>错误</th></tr></thead>
      <tbody>
        <tr v-for="run in runs" :key="run.id">
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

  <Modal :open="deleteAgentOpen" danger title="删除代理" @close="deleteAgentOpen = false">
    <p>即将删除代理 <strong>{{ deleteAgentTarget?.display_name }}</strong>。</p>
    <p class="muted">该代理下的监控任务、运行记录、快照、冲突与 IP 占用数据会一并删除，且不可恢复。</p>
    <p v-if="deleteAgentError" class="field-error">{{ deleteAgentError }}</p>
    <template #footer>
      <button class="ghost" @click="deleteAgentOpen = false">取消</button>
      <button class="danger" :disabled="deletingAgent" @click="submitDeleteAgent">
        {{ deletingAgent ? '删除中…' : '确认删除' }}
      </button>
    </template>
  </Modal>

  <Modal :open="deleteTaskOpen" danger title="删除监控任务" @close="deleteTaskOpen = false">
    <p>即将删除任务 <strong>#{{ deleteTaskTarget?.id }}</strong>（代理：{{ agentName(deleteTaskTarget?.agent_id) }}）。</p>
    <p class="muted">该任务的运行记录、快照与关联数据会一并删除，且不可恢复。</p>
    <p v-if="deleteTaskError" class="field-error">{{ deleteTaskError }}</p>
    <template #footer>
      <button class="ghost" @click="deleteTaskOpen = false">取消</button>
      <button class="danger" :disabled="deletingTask" @click="submitDeleteTask">
        {{ deletingTask ? '删除中…' : '确认删除' }}
      </button>
    </template>
  </Modal>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import SkeletonTable from '../components/SkeletonTable.vue'
import Modal from '../components/Modal.vue'
import { listAgents, createAgent, deleteAgentApi, listTasks, createTask, runTask, deleteTask, listSyncRuns, detectAgent } from '../api'
import {
  formatDate, formatDuration, parseSourceValues, policyLabel, policyTone, runStatusText, runStatusTone,
} from '../utils/format'
import { toast } from '../utils/toast'

const agents = ref([])
const tasks = ref([])
const runs = ref([])
const loading = ref(false)

const showAgentForm = ref(false)
const agentForm = reactive({ name: '', sources: '', action: 'alert' })
const savingAgent = ref(false)

const showTaskForm = ref(false)
const taskForm = reactive({ agentId: 0, interval: 1800 })
const savingTask = ref(false)

const runningTask = ref(0)
const detecting = ref(0)

async function load() {
  loading.value = true
  const results = await Promise.allSettled([listAgents(), listTasks(), listSyncRuns()])
  if (results[0].status === 'fulfilled') agents.value = results[0].value || []
  if (results[1].status === 'fulfilled') tasks.value = results[1].value || []
  if (results[2].status === 'fulfilled') runs.value = results[2].value || []
  const failed = results.find((result) => result.status === 'rejected')
  if (failed) toast.error(failed.reason.message)
  loading.value = false
}

async function createAgentSubmit() {
  savingAgent.value = true
  try {
    await createAgent({
      display_name: agentForm.name,
      source_values: agentForm.sources,
      enabled: true,
      policy_json: JSON.stringify({ action: agentForm.action }),
    })
    agentForm.name = ''
    agentForm.sources = ''
    agentForm.action = 'alert'
    showAgentForm.value = false
    toast.success('代理已创建')
    await load()
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingAgent.value = false
  }
}

async function createTaskSubmit() {
  savingTask.value = true
  try {
    await createTask({ agent_id: taskForm.agentId, interval_sec: taskForm.interval })
    showTaskForm.value = false
    toast.success('任务已创建')
    await load()
  } catch (e) {
    toast.error(e.message)
  } finally {
    savingTask.value = false
  }
}

async function run(id) {
  runningTask.value = id
  try {
    const runResult = await runTask(id)
    toast.success(`任务 #${id} 完成：${runStatusText(runResult.status)}，读取 ${runResult.rows_read}，保存 ${runResult.rows_saved}${runResult.error ? `；${runResult.error}` : ''}`)
    await load()
  } catch (e) {
    toast.error(e.message)
  } finally {
    runningTask.value = 0
  }
}

async function detect(agent) {
  detecting.value = agent.id
  try {
    const summary = await detectAgent(agent.id)
    toast.success(`代理「${agent.display_name}」检测完成：冲突 ${summary.conflicts} 个（新增 ${summary.created}，更新 ${summary.updated}，重开 ${summary.reopened}）`)
  } catch (e) {
    toast.error(e.message)
  } finally {
    detecting.value = 0
  }
}

const deleteAgentOpen = ref(false)
const deleteAgentTarget = ref(null)
const deleteAgentError = ref('')
const deletingAgent = ref(false)

function openDeleteAgent(agent) {
  deleteAgentTarget.value = agent
  deleteAgentError.value = ''
  deleteAgentOpen.value = true
}

async function submitDeleteAgent() {
  if (!deleteAgentTarget.value) return
  deletingAgent.value = true
  deleteAgentError.value = ''
  try {
    await deleteAgentApi(deleteAgentTarget.value.id)
    toast.success(`代理「${deleteAgentTarget.value.display_name}」已删除`)
    deleteAgentOpen.value = false
    await load()
  } catch (e) {
    deleteAgentError.value = e.message
  } finally {
    deletingAgent.value = false
  }
}

const deleteTaskOpen = ref(false)
const deleteTaskTarget = ref(null)
const deleteTaskError = ref('')
const deletingTask = ref(false)

function openDeleteTask(task) {
  deleteTaskTarget.value = task
  deleteTaskError.value = ''
  deleteTaskOpen.value = true
}

async function submitDeleteTask() {
  if (!deleteTaskTarget.value) return
  deletingTask.value = true
  deleteTaskError.value = ''
  try {
    await deleteTask(deleteTaskTarget.value.id)
    toast.success(`任务 #${deleteTaskTarget.value.id} 已删除`)
    deleteTaskOpen.value = false
    await load()
  } catch (e) {
    deleteTaskError.value = e.message
  } finally {
    deletingTask.value = false
  }
}

function agentName(id) {
  const agent = agents.value.find((item) => item.id === id)
  return agent ? agent.display_name : `代理 #${id}`
}

onMounted(load)
</script>
