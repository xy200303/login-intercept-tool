<template>
  <section class="stats">
    <article>
      <span>kkud 连接状态</span>
      <strong>
        <StatusBadge v-if="conn" :text="conn.kkud?.ok ? '连接正常' : '连接失败'" :tone="conn.kkud?.ok ? 'green' : 'red'" />
        <span v-else class="muted" style="font-size:16px">{{ testing ? '检测中…' : '未检测' }}</span>
      </strong>
      <small>{{ conn?.kkud?.ok === false ? conn.kkud.error : 'MySQL 用户来源库' }}</small>
    </article>
    <article>
      <span>fenx_site 连接状态</span>
      <strong>
        <StatusBadge v-if="conn" :text="conn.fenx_site?.ok ? '连接正常' : '连接失败'" :tone="conn.fenx_site?.ok ? 'green' : 'red'" />
        <span v-else class="muted" style="font-size:16px">{{ testing ? '检测中…' : '未检测' }}</span>
      </strong>
      <small>{{ conn?.fenx_site?.ok === false ? conn.fenx_site.error : 'MySQL 业务用户库' }}</small>
    </article>
    <article>
      <span>今日拦截</span>
      <strong>{{ guardToday }}</strong>
      <small>按拦截记录时间统计</small>
    </article>
    <article>
      <span>拦截记录总数</span>
      <strong>{{ guardTotal }}</strong>
      <small>来自 fenx_guard 实时接口</small>
    </article>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>外部库连接状态</h2>
        <p class="muted">检测 kkud 与 fenx_site 两个 MySQL 数据源的连通性</p>
      </div>
      <button class="primary small" :disabled="testing" @click="runTest">{{ testing ? '检测中…' : '重新检测' }}</button>
    </div>
    <div v-if="conn" class="chip-row" style="margin-top:18px">
      <span v-for="name in ['kkud', 'fenx_site']" :key="name" class="chip">
        {{ name }}：
        <StatusBadge
          :text="conn[name]?.ok ? '连接正常' : '连接失败'"
          :tone="conn[name]?.ok ? 'green' : 'red'"
        />
      </span>
    </div>
    <p v-for="name in ['kkud', 'fenx_site']" :key="name + '-err'" class="error">
      {{ conn?.[name]?.ok === false ? `${name}：${conn[name].error}` : '' }}
    </p>
    <p class="muted" style="margin-top:14px">
      连接异常或需要修改数据源时，请到
      <router-link to="/settings" style="color:var(--primary)">系统设置</router-link>
      中配置；拦截记录接口也在同一页面维护。
    </p>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import { testConnections, listGuardRecords } from '../api'
import { toast } from '../utils/toast'

const testing = ref(false)
const conn = ref(null)
const guardToday = ref('—')
const guardTotal = ref('—')

function todayPrefix() {
  const now = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

async function runTest() {
  testing.value = true
  try {
    conn.value = await testConnections()
  } catch (e) {
    toast.error(e.message)
  } finally {
    testing.value = false
  }
}

async function loadGuard() {
  try {
    const data = await listGuardRecords()
    const records = data?.records || []
    guardTotal.value = data?.count ?? records.length
    const today = todayPrefix()
    guardToday.value = records.filter((row) => String(row.time || '').startsWith(today)).length
  } catch (e) {
    // 503（未配置拦截记录接口）属预期情况，统计卡显示 —，不打扰用户
    if (!e.message.includes('未配置')) toast.error(e.message)
  }
}

onMounted(() => {
  runTest()
  loadGuard()
})
</script>
