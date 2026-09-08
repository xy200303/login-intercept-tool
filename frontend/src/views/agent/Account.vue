<template>
  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>个人信息</h2>
        <p class="muted">当前登录的平台账号</p>
      </div>
    </div>
    <div class="chip-row" style="margin-top:14px">
      <span class="chip">用户名：{{ auth.user?.username }}</span>
      <span class="chip">角色：{{ auth.roleLabel }}</span>
      <span class="chip">账号 ID：#{{ auth.user?.id }}</span>
    </div>
  </section>

  <section class="panel">
    <div class="panel-head">
      <div>
        <h2>修改密码</h2>
        <p class="muted">新密码至少 8 位，修改后需要重新登录</p>
      </div>
    </div>
    <form class="inline-form" @submit.prevent="submit">
      <label>旧密码<input v-model="form.oldPassword" type="password" autocomplete="current-password" required /></label>
      <label>新密码<input v-model="form.newPassword" type="password" autocomplete="new-password" minlength="8" required /></label>
      <label>确认新密码<input v-model="form.confirm" type="password" autocomplete="new-password" minlength="8" required /></label>
      <button class="primary small" :disabled="saving">{{ saving ? '提交中…' : '修改密码' }}</button>
    </form>
    <p v-if="formError" class="error">{{ formError }}</p>
    <p v-if="done" class="success">密码已修改，请使用新密码重新登录。</p>
  </section>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { changePassword } from '../../api'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })
const saving = ref(false)
const formError = ref('')
const done = ref(false)

async function submit() {
  formError.value = ''
  done.value = false
  if (form.newPassword !== form.confirm) {
    formError.value = '两次输入的新密码不一致'
    return
  }
  saving.value = true
  try {
    await changePassword(form.oldPassword, form.newPassword)
    done.value = true
    setTimeout(() => {
      auth.logout()
      router.push('/login')
    }, 1500)
  } catch (e) {
    formError.value = e.message
  } finally {
    saving.value = false
  }
}
</script>
