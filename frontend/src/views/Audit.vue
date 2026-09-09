<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>审计日志</h2>
        <p class="muted">平台关键操作的不可变审计事件</p>
      </div>
      <button class="ghost small" @click="loadAudit(0)">刷新</button>
    </div>
    <SkeletonTable v-if="auditLoading && !audit.items.length" :cols="5" />
    <EmptyState v-else-if="!audit.items.length" title="暂无审计事件" hint="执行账号变更、设置修改等操作后自动生成。" />
    <template v-else>
      <table>
        <thead><tr><th>时间</th><th>操作者</th><th>动作</th><th>目标</th><th>详情</th></tr></thead>
        <tbody>
          <tr v-for="event in audit.items" :key="event.id">
            <td>{{ formatDate(event.created_at) }}</td>
            <td>{{ event.actor_id ? `#${event.actor_id}` : '系统' }}</td>
            <td class="mono">{{ event.action }}</td>
            <td class="mono">{{ event.target }}</td>
            <td class="muted">{{ event.detail || '—' }}</td>
          </tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost small" :disabled="audit.offset === 0" @click="loadAudit(audit.offset - 50)">上一页</button>
        <span class="muted">共 {{ audit.total }} 条 · 第 {{ Math.floor(audit.offset / 50) + 1 }} 页</span>
        <button class="ghost small" :disabled="audit.offset + 50 >= audit.total" @click="loadAudit(audit.offset + 50)">下一页</button>
      </div>
    </template>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import SkeletonTable from '../components/SkeletonTable.vue'
import { listAuditEvents } from '../api'
import { formatDate } from '../utils/format'
import { toast } from '../utils/toast'

const audit = reactive({ items: [], total: 0, offset: 0 })
const auditLoading = ref(false)

async function loadAudit(nextOffset = 0) {
  auditLoading.value = true
  try {
    const data = await listAuditEvents({ limit: 50, offset: Math.max(0, nextOffset) })
    audit.items = data.items || []
    audit.total = data.total || 0
    audit.offset = data.offset || 0
  } catch (e) {
    toast.error(e.message)
  } finally {
    auditLoading.value = false
  }
}

onMounted(() => loadAudit(0))
</script>
