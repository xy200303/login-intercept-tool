import { createRouter, createWebHistory } from 'vue-router'
import AdminLayout from '../layouts/AdminLayout.vue'
import Login from '../views/Login.vue'
import Overview from '../views/Overview.vue'
import KkudUsers from '../views/KkudUsers.vue'
import FenxUsers from '../views/FenxUsers.vue'
import IpRecords from '../views/FenxIpLogs.vue'
import GuardRecords from '../views/GuardRecords.vue'
import Audit from '../views/Audit.vue'
import Settings from '../views/Settings.vue'

const ADMIN_ROLES = ['super_admin', 'superadmin', 'operator']

const readRole = () => {
  try {
    return JSON.parse(localStorage.getItem('fenx_user') || 'null')?.role || ''
  } catch {
    return ''
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true, title: '登录' } },
    {
      path: '/',
      component: AdminLayout,
      children: [
        { path: '', redirect: '/overview' },
        { path: 'overview', component: Overview, meta: { roles: ADMIN_ROLES, title: '总览' } },
        { path: 'kkud-users', component: KkudUsers, meta: { roles: ADMIN_ROLES, title: 'kkud 用户' } },
        { path: 'fenx-users', component: FenxUsers, meta: { roles: ADMIN_ROLES, title: 'fenx 用户' } },
        { path: 'ip-records', component: IpRecords, meta: { roles: ADMIN_ROLES, title: 'IP 记录管理' } },
        { path: 'guard-records', component: GuardRecords, meta: { roles: ADMIN_ROLES, title: '拦截记录' } },
        { path: 'audit', component: Audit, meta: { roles: ADMIN_ROLES, title: '审计日志' } },
        { path: 'settings', component: Settings, meta: { roles: ADMIN_ROLES, title: '系统设置' } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  // access token 过期但 refresh token 有效时仍算有会话，首个请求会触发静默刷新
  const hasSession = Boolean(localStorage.getItem('fenx_token') || localStorage.getItem('fenx_refresh'))
  const role = readRole()
  const isAdmin = ADMIN_ROLES.includes(role)
  if (to.meta.public) {
    if (hasSession && isAdmin && to.path === '/login') return '/overview'
    return true
  }
  if (!hasSession) return '/login'
  // 代理体系已下线，非管理角色没有可用页面，回到登录页换账号
  if (to.meta.roles && !to.meta.roles.includes(role)) return '/login'
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · Fenx 控制台` : 'Fenx 控制台'
})

export default router
