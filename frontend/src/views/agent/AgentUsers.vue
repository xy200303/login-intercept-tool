<template>
  <p v-if="error" class="error">{{ error }}</p>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>关联结果</h2>
        <p class="muted">kkud 用户与 fenx 账号的匹配结果（手机号优先，低置信度需人工复核）</p>
      </div>
      <button class="ghost small" @click="loadMatches">刷新</button>
    </div>
    <EmptyState v-if="!matches.length" title="暂无关联结果" hint="任务运行并命中手机号 / QQ / 邮箱后生成。" />
    <table v-else>
      <thead><tr><th>批次</th><th>快照</th><th>fenx 账号</th><th>置信度</th><th>命中字段</th><th>状态</th><th>时间</th></tr></thead>
      <tbody>
        <tr v-for="row in matches" :key="row.id">
          <td>#{{ row.run_id }}</td>
          <td class="mono">{{ row.kkud_snapshot_id }}</td>
          <td class="mono">{{ row.fenx_user_id }}</td>
          <td>{{ Math.round((row.confidence || 0) * 100) }}%</td>
          <td>{{ row.matched_fields || '—' }}</td>
          <td><StatusBadge :text="matchStateText(row.state)" :tone="matchStateTone(row.state)" /></td>
          <td>{{ formatDate(row.created_at) }}</td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>kkud 用户快照</h2>
        <p class="muted">本代理 daili 值采集到的 kkud 用户，手机号已脱敏</p>
      </div>
      <button class="ghost small" @click="loadSnapshots">刷新</button>
    </div>
    <EmptyState
      v-if="snapshotDenied"
      title="暂无快照查看权限"
      hint="kkud 快照目前仅对管理员开放，如需查看请联系超级管理员。"
    />
    <EmptyState v-else-if="!snapshots.length" title="暂无快照" hint="运行监控任务后，采集到的用户会出现在这里。" />
    <table v-else>
      <thead><tr><th>来源 ID</th><th>代理值</th><th>手机号</th><th>QQ</th><th>邮箱</th><th>采集时间</th></tr></thead>
      <tbody>
        <tr v-for="row in snapshots" :key="row.id">
          <td class="mono">{{ row.source_pk }}</td>
          <td>{{ row.agent_value || '—' }}</td>
          <td class="mono">{{ maskMobile(row.mobile) }}</td>
          <td>{{ row.qq || '—' }}</td>
          <td>{{ row.email || '—' }}</td>
          <td>{{ formatDate(row.captured_at) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import StatusBadge from '../../components/StatusBadge.vue'
import EmptyState from '../../components/EmptyState.vue'
import { listMatches, listSnapshots } from '../../api'
import { formatDate, maskMobile, matchStateText, matchStateTone } from '../../utils/format'

const matches = ref([])
const snapshots = ref([])
const snapshotDenied = ref(false)
const error = ref('')

async function loadMatches() {
  try {
    matches.value = await listMatches() || []
  } catch (e) {
    error.value = e.message
  }
}

async function loadSnapshots() {
  try {
    snapshots.value = await listSnapshots({ limit: 100 }) || []
    snapshotDenied.value = false
  } catch (e) {
    if (e.message.includes('无权')) {
      snapshotDenied.value = true
    } else {
      error.value = e.message
    }
  }
}

onMounted(() => {
  loadMatches()
  loadSnapshots()
})
</script>
