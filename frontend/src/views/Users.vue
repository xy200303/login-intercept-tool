<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>平台账号</h2>
        <p class="muted">控制台登录账号管理（仅超级管理员）</p>
      </div>
      <button class="primary small" @click="showForm = !showForm">新建用户</button>
    </div>
    <form v-if="showForm" class="inline-form" @submit.prevent="submit">
      <label>用户名<input v-model.trim="form.username" required /></label>
      <label>密码<input v-model="form.password" type="password" minlength="8" placeholder="至少 8 位" required /></label>
      <label>
        角色
        <select v-model="form.role">
          <option value="operator">运营管理员</option>
          <option value="agent">代理账号</option>
        </select>
      </label>
      <label v-if="form.role === 'agent'">
        绑定代理
        <select v-model.number="form.agent_id" required>
          <option :value="0" disabled>请选择代理</option>
          <option v-for="agent in agents" :key="agent.id" :value="agent.id">{{ agent.display_name }}</option>
        </select>
      </label>
      <button class="primary small" :disabled="saving">{{ saving ? '创建中…' : '创建' }}</button>
    </form>
    <EmptyState v-if="!users.length" title="暂无平台账号" hint="点击「新建用户」创建运营或代理账号。" />
    <table v-else>
      <thead><tr><th>ID</th><th>用户名</th><th>角色</th><th>绑定代理</th><th>状态</th><th>最近登录 IP</th><th>创建时间</th></tr></thead>
      <tbody>
        <tr v-for="user in users" :key="user.id">
          <td>#{{ user.id }}</td>
          <td>{{ user.username }}</td>
          <td><StatusBadge :text="roleText(user.role)" :tone="user.role === 'agent' ? 'blue' : 'gray'" /></td>
          <td>{{ agentName(user.agent_id) }}</td>
          <td><StatusBadge :text="user.status === 'active' ? '正常' : user.status" :tone="user.status === 'active' ? 'green' : 'gray'" /></td>
          <td class="mono">{{ user.last_login_ip || '—' }}</td>
          <td>{{ formatDate(user.created_at) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { listUsers, createUser, listAgents } from '../api'
import { formatDate } from '../utils/format'
import { toast } from '../utils/toast'

const users = ref([])
const agents = ref([])
const showForm = ref(false)
const form = reactive({ username: '', password: '', role: 'agent', agent_id: 0 })
const saving = ref(false)

const roleText = (role) => ({ super_admin: '超级管理员', operator: '运营管理员', agent: '代理账号' }[role] || role)

async function load() {
  const results = await Promise.allSettled([listUsers(), listAgents()])
  if (results[0].status === 'fulfilled') users.value = results[0].value || []
  if (results[1].status === 'fulfilled') agents.value = results[1].value || []
  const failed = results.find((result) => result.status === 'rejected')
  if (failed) toast.error(failed.reason.message)
}

async function submit() {
  saving.value = true
  try {
    const payload = { username: form.username, password: form.password, role: form.role }
    if (form.role === 'agent') payload.agent_id = form.agent_id
    await createUser(payload)
    form.username = ''
    form.password = ''
    form.agent_id = 0
    showForm.value = false
    toast.success('用户已创建')
    await load()
  } catch (e) {
    toast.error(e.message)
  } finally {
    saving.value = false
  }
}

function agentName(id) {
  if (!id) return '—'
  const agent = agents.value.find((item) => item.id === id)
  return agent ? agent.display_name : `代理 #${id}`
}

onMounted(load)
</script>
