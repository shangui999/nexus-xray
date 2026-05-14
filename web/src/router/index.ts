import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '../store/auth'

const Layout = () => import('../components/Layout.vue')
const Login = () => import('../views/Login.vue')
const Dashboard = () => import('../views/Dashboard.vue')
const Nodes = () => import('../views/Nodes.vue')
const Users = () => import('../views/Users.vue')
const Plans = () => import('../views/Plans.vue')
const Inbounds = () => import('../views/Inbounds.vue')
const Settings = () => import('../views/Settings.vue')

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    component: Login,
    meta: { public: true },
  },
  {
    path: '/',
    component: Layout,
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: Dashboard },
      { path: 'nodes', component: Nodes },
      { path: 'users', component: Users },
      { path: 'plans', component: Plans },
      { path: 'inbounds', component: Inbounds },
      { path: 'settings', component: Settings },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 路由守卫：未登录跳转 /login
router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore()
  if (to.meta.public || authStore.isLoggedIn) {
    next()
  } else {
    next('/login')
  }
})

export default router
