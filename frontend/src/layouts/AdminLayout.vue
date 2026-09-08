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
import Icon from '../components/Icon.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)

const ADMIN_MENU = [
  { to: '/overview', label: '总览', icon: 'dashboard' },
  { to: '/agents', label: '代理与任务', icon: 'network' },
  { to: '/kkud-users', label: 'kkud 用户', icon: 'database' },
  { to: '/fenx-users', label: 'fenx 用户', icon: 'users' },
  { to: '/conflicts', label: 'IP 冲突中心', icon: 'shield-alert' },
  { to: '/audit', label: '审计日志', icon: 'scroll-text' },
  { to: '/settings', label: '系统设置', icon: 'settings' },
  { to: '/users', label: '平台账号', icon: 'user-cog' },
]

const AGENT_MENU = [
  { to: '/agent/overview', label: '我的概览', icon: 'home' },
  { to: '/agent/users', label: '我的用户', icon: 'id-card' },
  { to: '/agent/conflicts', label: '冲突处理', icon: 'shield-alert' },
  { to: '/agent/runs', label: '任务记录', icon: 'activity' },
  { to: '/agent/account', label: '账号设置', icon: 'user' },
]

const menu = computed(() => (auth.isAgent ? AGENT_MENU : ADMIN_MENU))

const isActive = (path) => route.path === path || (path !== '/overview' && route.path.startsWith(path))

function logout() {
  auth.logout()
  router.push('/login')
}
</script>
