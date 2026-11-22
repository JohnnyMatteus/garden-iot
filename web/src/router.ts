import { createRouter, createWebHistory } from 'vue-router'
import { isAuthenticated } from '@/features/auth/session'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/entrar', name: 'login', component: () => import('@/features/auth/LoginView.vue'), meta: { public: true } },
    { path: '/', name: 'home', component: () => import('@/features/map/HomeRedirect.vue') },
    { path: '/h/:gardenId', name: 'map', component: () => import('@/features/map/MapView.vue'), props: true },
    { path: '/h/:gardenId/canteiros/:bedId', name: 'bed', component: () => import('@/features/bed/BedDetailView.vue'), props: true },
    { path: '/h/:gardenId/regas', name: 'commands', component: () => import('@/features/commands/CommandsView.vue'), props: true },
    { path: '/h/:gardenId/dispositivos', name: 'devices', component: () => import('@/features/devices/DevicesView.vue'), props: true },
  ],
})

router.beforeEach((to) => {
  if (!to.meta.public && !isAuthenticated.value) return { name: 'login' }
  if (to.name === 'login' && isAuthenticated.value) return { name: 'home' }
})

export default router
