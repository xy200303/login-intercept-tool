<template>
  <span class="badge" :class="`badge-${tone}`">{{ text }}</span>
</template>

<script setup>
defineProps({
  text: { type: String, default: '—' },
  tone: { type: String, default: 'gray' },
})
</script>
