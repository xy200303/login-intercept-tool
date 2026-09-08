<template>
  <teleport to="body">
    <transition name="slide">
      <div v-if="open" class="drawer-overlay" @click.self="$emit('close')">
        <aside class="drawer" role="dialog" aria-modal="true">
          <header class="drawer-head">
            <div>
              <h3>{{ title }}</h3>
              <p v-if="subtitle" class="muted">{{ subtitle }}</p>
            </div>
            <button type="button" class="modal-close" aria-label="关闭" @click="$emit('close')">×</button>
          </header>
          <div class="drawer-body"><slot /></div>
          <footer v-if="$slots.footer" class="drawer-foot"><slot name="footer" /></footer>
        </aside>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  subtitle: { type: String, default: '' },
})
defineEmits(['close'])
</script>
