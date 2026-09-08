<template>
  <p v-if="pageError" class="error">{{ pageError }}</p>

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
      <small>{{ conn?.kkud?.ok === false ? conn.kkud.error : 'MySQL 采集来源库' }}</small>
    </article>
    <article>
      <span>fenx_site 数据源</span>
      <strong>
        <StatusBadge v-if="conn" :text="conn.fenx_site?.ok ? '连接正常' : '连接失败'" :tone="conn.fenx_site?.ok ? 'green' : 'red'" />
        <span v-else class="muted" style="font-size:16px">未检测</span>
      </strong>
      <small>{{ conn?.fenx_site?.ok === false ? conn.fenx_site.error : 'MySQL 业务用户库' }}</small>
    </article>
    <article>
      <span>操作</span>
      <strong style="font-size:16px">
        <button class="primary small" :disabled="testingSaved" @click="runSavedTest">
          {{ testingSaved ? '检测中…' : '测试已保存配置' }}
        </button>
      </strong>
      <small>检测 sys_config 中保存的两个数据源</small>
    </article>
  </section>

  <template v-if="auth.isSuperAdmin">
    <section v-for="side in SIDES" :key="side.key" class="panel">
      <div class="panel-head">
        <div>
          <h2>{{ side.label }}</h2>
          <p class="muted">{{ side.desc }}</p>
        </div>
        <StatusBadge
          :text="saved[side.key]?.has_password ? '已配置密码' : '未配置密码'"
          :tone="saved[side.key]?.has_password ? 'green' : 'yellow'"
        />
      </div>
      <form class="inline-form" @submit.prevent="saveSide(side.key)">
        <label>主机<input v-model.trim="forms[side.key].host" placeholder="如 103.96.75.84" required /></label>
        <label>端口<input v-model.trim="forms[side.key].port" placeholder="3306" /></label>
        <label>库名<input v-model.trim="forms[side.key].name" :placeholder="side.key === 'kkud' ? 'kkud' : 'fenx_site'" required /></label>
        <label>账号<input v-model.trim="forms[side.key].user" placeholder="最小权限账号" required /></label>
        <label>
          密码
          <input
            v-model="forms[side.key].password"
            type="password"
            autocomplete="new-password"
            :placeholder="saved[side.key]?.has_password ? '已保存，留空则不修改' : '未配置，请输入密码'"
          />
        </label>
      </form>
      <div class="drawer-actions">
        <button class="ghost small" :disabled="state[side.key].testing" @click="testSide(side.key)">
          {{ state[side.key].testing ? '检测中…' : '测试连接（当前表单值）' }}
        </button>
        <button class="primary small" :disabled="state[side.key].saving" @click="saveSide(side.key)">
          {{ state[side.key].saving ? '保存中…' : '保存配置' }}
        </button>
      </div>
      <div v-if="state[side.key].result" class="chip-row">
        <span class="chip">
          测试结果：
          <StatusBadge
            :text="state[side.key].result.ok ? '连接正常' : '连接失败'"
            :tone="state[side.key].result.ok ? 'green' : 'red'"
          />
        </span>
      </div>
      <p v-if="state[side.key].result && !state[side.key].result.ok" class="error">{{ state[side.key].result.error }}</p>
      <p v-if="state[side.key].message" class="success">{{ state[side.key].message }}</p>
      <p v-if="state[side.key].error" class="error">{{ state[side.key].error }}</p>
    </section>
  </template>
  <section v-else class="panel">
    <EmptyState title="外连数据库配置仅超级管理员可见" hint="如需修改 kkud / fenx_site 连接，请使用超级管理员账号登录。" />
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { getExternalDbSettings, updateExternalDbSettings, testConnections, fetchHealth } from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

const SIDES = [
  { key: 'kkud', label: 'kkud 数据库', desc: '采集来源库（kkud.user568531942，建议只读账号）' },
  { key: 'fenx', label: 'fenx_site 数据库', desc: '业务用户库（users / log_login，按权限写操作）' },
]

const blankForm = () => ({ host: '', port: '', name: '', user: '', password: '' })
const blankState = () => ({ testing: false, saving: false, result: null, message: '', error: '' })

const forms = reactive({ kkud: blankForm(), fenx: blankForm() })
const saved = reactive({ kkud: null, fenx: null })
const state = reactive({ kkud: blankState(), fenx: blankState() })

const health = ref('')
const conn = ref(null)
const testingSaved = ref(false)
const pageError = ref('')

// 测试响应里 fenx 侧的键是 fenx_site
const resultKey = (key) => (key === 'fenx' ? 'fenx_site' : key)

async function loadSettings() {
  if (!auth.isSuperAdmin) return
  try {
    const data = await getExternalDbSettings()
    for (const side of SIDES) {
      const cfg = data?.[side.key] || {}
      saved[side.key] = cfg
      forms[side.key].host = cfg.host || ''
      forms[side.key].port = cfg.port || ''
      forms[side.key].name = cfg.name || ''
      forms[side.key].user = cfg.user || ''
      forms[side.key].password = ''
    }
  } catch (e) {
    pageError.value = e.message
  }
}

async function testSide(key) {
  const st = state[key]
  st.testing = true
  st.result = null
  st.message = ''
  st.error = ''
  try {
    const data = await testConnections({ [key]: { ...forms[key] } })
    st.result = data?.[resultKey(key)] || { ok: false, error: '未返回该数据源结果' }
  } catch (e) {
    st.error = e.message
  } finally {
    st.testing = false
  }
}

async function saveSide(key) {
  const st = state[key]
  st.saving = true
  st.message = ''
  st.error = ''
  try {
    await updateExternalDbSettings({ [key]: { ...forms[key] } })
    st.message = '配置已保存'
    forms[key].password = ''
    await loadSettings()
  } catch (e) {
    st.error = e.message
  } finally {
    st.saving = false
  }
}

async function runSavedTest() {
  testingSaved.value = true
  pageError.value = ''
  try {
    conn.value = await testConnections()
  } catch (e) {
    pageError.value = e.message
  } finally {
    testingSaved.value = false
  }
}

onMounted(async () => {
  loadSettings()
  try {
    health.value = (await fetchHealth())?.status || ''
  } catch {
    health.value = ''
  }
})
</script>
