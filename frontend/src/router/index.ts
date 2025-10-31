import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '@/views/LoginView.vue'
import RegisterView from '@/views/RegisterView.vue'
import DashboardView from '@/views/DashboardView.vue'
import AdminDashboard from '@/views/AdminDashboard.vue'
import ChatView from '@/views/ChatView.vue'
import { useAuth } from '@/composables/useAuth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/register',
      name: 'register',
      component: RegisterView,
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: DashboardView,
      meta: { requiresAuth: true },
    },
    {
      path: '/admin',
      name: 'admin',
      component: AdminDashboard,
      meta: { requiresAuth: true, requiresRoot: true },
    },
    {
      path: '/p/:id/chat',
      name: 'chat',
      component: ChatView,
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach(async (to, from, next) => {
  const { isAuthenticated, checkAuth, user } = useAuth()

  // Check authentication status
  await checkAuth()

  if (to.meta.requiresAuth && !isAuthenticated.value) {
    next('/login')
  } else if ((to.path === '/login' || to.path === '/register') && isAuthenticated.value) {
    // Redirect root user to admin dashboard
    if (user.value?.username === 'root') {
      next('/admin')
    } else {
      next('/dashboard')
    }
  } else if (to.path === '/dashboard' && user.value?.username === 'root') {
    // If root tries to access /dashboard, redirect to /admin
    next('/admin')
  } else if (to.meta.requiresRoot && user.value?.username !== 'root') {
    // If non-root tries to access admin, redirect to dashboard
    next('/dashboard')
  } else {
    next()
  }
})

export default router
