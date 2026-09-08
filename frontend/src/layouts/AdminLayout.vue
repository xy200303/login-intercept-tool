<template>
  <div class="shell">
    <div v-if="menuOpen" class="sidebar-mask" @click="menuOpen = false"></div>
    <aside class="sidebar" :class="{ open: menuOpen }">
      <div class="logo"><span>FX</span><strong>Fenx</strong></div>
      <p class="nav-label">{{ auth.isAgent ? '代理工作台' : '管理工作台' }}</p>
      <nav>
        <router-link
          v-for="item in menu"
          :key="item.to"
          :to="item.to"
          :class="{ active: isActive(item.to) }"
          @click="menuOpen = false"
        >{{ item.label }}</router-link>
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
          <button class="ghost small" @click="logout">退出登录</button>
        </div>
      </header>
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)

const ADMIN_MENU = [
  { to: '/overview', label: '总览' },
  { to: '/agents', label: '代理与任务' },
  { to: '/kkud-users', label: 'kkud 用户' },
  { to: '/fenx-users', label: 'fenx 用户' },
  { to: '/conflicts', label: 'IP 冲突中心' },
  { to: '/audit', label: '审计与系统' },
  { to: '/users', label: '平台账号' },
]

const AGENT_MENU = [
  { to: '/agent/overview', label: '我的概览' },
  { to: '/agent/users', label: '我的用户' },
  { to: '/agent/conflicts', label: '冲突处理' },
  { to: '/agent/runs', label: '任务记录' },
  { to: '/agent/account', label: '账号设置' },
]

const menu = computed(() => (auth.isAgent ? AGENT_MENU : ADMIN_MENU))

const isActive = (path) => route.path === path || (path !== '/overview' && route.path.startsWith(path))

function logout() {
  auth.logout()
  router.push('/login')
}
</script>
