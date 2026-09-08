import { createRouter, createWebHistory } from 'vue-router'
import AdminLayout from '../layouts/AdminLayout.vue'
import Login from '../views/Login.vue'
import Overview from '../views/Overview.vue'
import Agents from '../views/Agents.vue'
import KkudUsers from '../views/KkudUsers.vue'
import FenxUsers from '../views/FenxUsers.vue'
import Conflicts from '../views/Conflicts.vue'
import Audit from '../views/Audit.vue'
import Users from '../views/Users.vue'
import AgentOverview from '../views/agent/AgentOverview.vue'
import AgentUsers from '../views/agent/AgentUsers.vue'
import AgentConflicts from '../views/agent/AgentConflicts.vue'
import AgentRuns from '../views/agent/AgentRuns.vue'
import Account from '../views/agent/Account.vue'

const ADMIN_ROLES = ['super_admin', 'superadmin', 'operator']

const readRole = () => {
  try {
    return JSON.parse(localStorage.getItem('fenx_user') || 'null')?.role || ''
  } catch {
    return ''
  }
}

const homeFor = (role) => (role === 'agent' ? '/agent/overview' : '/overview')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true, title: '登录' } },
    {
      path: '/',
      component: AdminLayout,
      children: [
        { path: '', redirect: () => homeFor(readRole()) },
        { path: 'overview', component: Overview, meta: { roles: ADMIN_ROLES, title: '总览' } },
        { path: 'agents', component: Agents, meta: { roles: ADMIN_ROLES, title: '代理与任务' } },
        { path: 'kkud-users', component: KkudUsers, meta: { roles: ADMIN_ROLES, title: 'kkud 用户' } },
        { path: 'fenx-users', component: FenxUsers, meta: { roles: ADMIN_ROLES, title: 'fenx 用户' } },
        { path: 'conflicts', component: Conflicts, meta: { roles: ADMIN_ROLES, title: 'IP 冲突中心' } },
        { path: 'audit', component: Audit, meta: { roles: ADMIN_ROLES, title: '审计与系统' } },
        { path: 'users', component: Users, meta: { roles: ADMIN_ROLES, title: '平台账号' } },
        { path: 'agent/overview', component: AgentOverview, meta: { roles: ['agent'], title: '我的概览' } },
        { path: 'agent/users', component: AgentUsers, meta: { roles: ['agent'], title: '我的用户' } },
        { path: 'agent/conflicts', component: AgentConflicts, meta: { roles: ['agent'], title: '冲突处理' } },
        { path: 'agent/runs', component: AgentRuns, meta: { roles: ['agent'], title: '任务记录' } },
        { path: 'agent/account', component: Account, meta: { roles: ['agent'], title: '账号设置' } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  const token = localStorage.getItem('fenx_token')
  const role = readRole()
  if (to.meta.public) {
    if (token && to.path === '/login') return homeFor(role)
    return true
  }
  if (!token) return '/login'
  if (to.meta.roles && !to.meta.roles.includes(role)) return homeFor(role)
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · Fenx 控制台` : 'Fenx 控制台'
})

export default router
