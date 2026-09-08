<template>
  <main class="login-page">
    <section class="login-card">
      <div class="brand-mark">FX</div>
      <p class="eyebrow">FENX CONTROL CENTER</p>
      <h1>登录控制台</h1>
      <p class="muted">统一管理代理监控与 IP 合规策略</p>
      <form @submit.prevent="submit">
        <label>账号<input v-model="username" autocomplete="username" required /></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="primary" :disabled="loading">{{ loading ? '登录中…' : '进入平台' }}</button>
      </form>
    </section>
  </main>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()

async function submit() {
  loading.value = true
  error.value = ''
  try {
    await auth.login(username.value, password.value)
    router.push(auth.homePath)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>
