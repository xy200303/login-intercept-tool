<template>
  <div class="toast-wrap" aria-live="polite" aria-atomic="false">
    <transition-group name="toast">
      <div v-for="item in toasts" :key="item.id" class="toast" :class="`toast-${item.type}`">
        <span class="toast-dot"></span>
        <span class="toast-text">{{ item.message }}</span>
      </div>
    </transition-group>
  </div>
</template>

<script setup>
import { toasts } from '../utils/toast'
</script>
