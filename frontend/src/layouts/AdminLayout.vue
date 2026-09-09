<template>
  <div class="shell">
    <div v-if="menuOpen" class="sidebar-mask" @click="menuOpen = false"></div>
    <aside class="sidebar" :class="{ open: menuOpen }">
      <div class="logo"><span>FX</span><strong>Fenx</strong></div>
      <p class="nav-label">管理工作台</p>
      <nav>
        <router-link
          v-for="item in menu"
          :key="item.to"
          :to="item.to"
          :class="{ active: isActive(item.to) }"
          @click="menuOpen = false"
        ><Icon :name="item.icon" /><span>{{ item.label }}</span></router-link>
      </nav>
      <div class="sidebar-foot">{{ auth.roleLabel }}<b class="dot"></b>已登录</div>
    </aside>

    <main class="content">
      <header class="topbar">
        <div class="topbar-left">
          <button class="menu-toggle" aria-label="打开菜单" @click="menuOpen = true">
            <span></span><span></span><span></span>
          </button>
          <div>
            <p class="eyebrow">{{ auth.roleLabel }}</p>
            <h1>{{ route.meta.title || '控制台' }}</h1>
          </div>
        </div>
        <div class="user-chip">
          <span class="user-name">{{ auth.user?.username }}</span>
          <button class="ghost small" @click="openPwd">修改密码</button>
          <button class="ghost small" @click="logout">退出登录</button>
        </div>
      </header>
      <router-view />
    </main>
  </div>

  <Modal :open="pwdOpen" title="修改密码" @close="pwdOpen = false">
    <label class="field">
      <span class="req">旧密码</span>
      <input v-model="pwdForm.oldPassword" type="password" autocomplete="current-password" />
    </label>
    <label class="field">
      <span class="req">新密码</span>
      <input v-model="pwdForm.newPassword" type="password" autocomplete="new-password" minlength="8" />
      <span class="helper">至少 8 位；修改成功后所有会话将被注销，需要重新登录</span>
    </label>
    <label class="field">
      <span class="req">确认新密码</span>
      <input v-model="pwdForm.confirm" type="password" autocomplete="new-password" minlength="8" />
    </label>
    <p v-if="pwdError" class="field-error">{{ pwdError }}</p>
    <template #footer>
      <button class="ghost" @click="pwdOpen = false">取消</button>
      <button class="primary" :disabled="pwdSaving" @click="submitPwd">{{ pwdSaving ? '提交中…' : '确认修改' }}</button>
    </template>
  </Modal>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { changePassword } from '../api'
import { toast } from '../utils/toast'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)

const ADMIN_MENU = [
  { to: '/overview', label: '总览', icon: 'dashboard' },
  { to: '/kkud-users', label: 'kkud 用户', icon: 'database' },
  { to: '/fenx-users', label: 'fenx 用户', icon: 'users' },
  { to: '/ip-records', label: 'IP 记录管理', icon: 'globe' },
  { to: '/guard-records', label: '拦截记录', icon: 'list' },
  { to: '/audit', label: '审计日志', icon: 'scroll-text' },
  { to: '/settings', label: '系统设置', icon: 'settings' },
]

const menu = computed(() => ADMIN_MENU)

const isActive = (path) => route.path === path || (path !== '/overview' && route.path.startsWith(path))

function logout() {
  auth.logout()
  router.push('/login')
}

const pwdOpen = ref(false)
const pwdForm = reactive({ oldPassword: '', newPassword: '', confirm: '' })
const pwdSaving = ref(false)
const pwdError = ref('')

function openPwd() {
  pwdForm.oldPassword = ''
  pwdForm.newPassword = ''
  pwdForm.confirm = ''
  pwdError.value = ''
  pwdOpen.value = true
}

async function submitPwd() {
  pwdError.value = ''
  if (pwdForm.newPassword.length < 8) {
    pwdError.value = '新密码至少 8 位'
    return
  }
  if (pwdForm.newPassword !== pwdForm.confirm) {
    pwdError.value = '两次输入的新密码不一致'
    return
  }
  pwdSaving.value = true
  try {
    await changePassword(pwdForm.oldPassword, pwdForm.newPassword)
    pwdOpen.value = false
    toast.success('密码已修改，所有会话已注销，请重新登录')
  } catch (e) {
    pwdError.value = e.message
  } finally {
    pwdSaving.value = false
  }
}
</script>
