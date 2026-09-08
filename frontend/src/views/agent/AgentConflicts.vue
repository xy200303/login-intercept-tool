<template>
  <p class="muted">以下为本代理范围内的冲突（由后端自动限定）。代理账号可查看详情与预览，执行处理需超级管理员操作。</p>
  <ConflictExplorer :can-execute="false" :show-agent-filter="false" />
</template>

<script setup>
import ConflictExplorer from '../../components/ConflictExplorer.vue'
</script>
