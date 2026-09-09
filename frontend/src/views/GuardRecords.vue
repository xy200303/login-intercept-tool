<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>拦截记录</h2>
        <p class="muted">来自联盟站点 fenx_guard 的实时拦截记录（最新 500 条）</p>
      </div>
      <button class="ghost small" :disabled="loading" @click="load">{{ loading ? '加载中…' : '刷新' }}</button>
    </div>

    <form v-if="records.length" class="inline-form" @submit.prevent>
      <label>过滤<input v-model.trim="keyword" placeholder="按账号或 IP 包含过滤（即时）" /></label>
      <span class="muted" style="align-self:center">共 {{ records.length }} 条 · 匹配 {{ filtered.length }} 条</span>
    </form>

    <SkeletonTable v-if="loading && !records.length" :cols="4" />
    <EmptyState v-else-if="needConfig" title="未配置拦截记录接口">
      <p class="muted">请先到「系统设置」配置拦截记录接口地址和密钥。</p>
      <router-link to="/settings"><button class="primary small">前往系统设置</button></router-link>
    </EmptyState>
    <EmptyState v-else-if="!records.length" title="暂无拦截记录" hint="联盟站点产生实时拦截后会出现在这里。" />
    <EmptyState v-else-if="!filtered.length" title="没有匹配的记录" hint="调整过滤关键词后重试。" />
    <table v-else>
      <thead><tr><th>时间</th><th>类型</th><th>账号</th><th>IP</th></tr></thead>
      <tbody>
        <tr v-for="(row, index) in filtered" :key="index">
          <td class="mono">{{ row.time }}</td>
          <td>
            <StatusBadge
              :text="row.action === 'register' ? '注册' : '登录'"
              :tone="row.action === 'register' ? 'orange' : 'blue'"
            />
          </td>
          <td class="mono">{{ row.account || '—' }}</td>
          <td class="mono">{{ row.ip || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import SkeletonTable from '../components/SkeletonTable.vue'
import { listGuardRecords } from '../api'
import { toast } from '../utils/toast'

const records = ref([])
const keyword = ref('')
const loading = ref(false)
const needConfig = ref(false)

const filtered = computed(() => {
  const key = keyword.value.toLowerCase()
  if (!key) return records.value
  return records.value.filter(
    (row) => (row.account || '').toLowerCase().includes(key) || (row.ip || '').toLowerCase().includes(key)
  )
})

async function load() {
  loading.value = true
  needConfig.value = false
  try {
    const data = await listGuardRecords()
    records.value = data?.records || []
  } catch (e) {
    if (e.message.includes('未配置')) {
      needConfig.value = true
      records.value = []
    } else {
      toast.error(e.message)
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
