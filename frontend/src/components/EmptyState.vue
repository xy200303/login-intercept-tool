<template>
  <div class="empty">
    <p class="empty-title">{{ title }}</p>
    <p v-if="hint" class="muted">{{ hint }}</p>
    <slot />
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '暂无数据' },
  hint: { type: String, default: '' },
})
</script>
