<template>
  <teleport to="body">
    <transition name="fade">
      <div v-if="open" class="modal-overlay" @click.self="$emit('close')">
        <div class="modal" :class="{ 'modal-danger': danger }" role="dialog" aria-modal="true">
          <header class="modal-head">
            <h3>{{ title }}</h3>
            <button type="button" class="modal-close" aria-label="关闭" @click="$emit('close')">×</button>
          </header>
          <div class="modal-body"><slot /></div>
          <footer v-if="$slots.footer" class="modal-foot"><slot name="footer" /></footer>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  danger: { type: Boolean, default: false },
})
defineEmits(['close'])
</script>
