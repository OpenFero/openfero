import { createRouter, createWebHistory } from 'vue-router'
import AlertsView from '@/views/AlertsView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'alerts',
      component: AlertsView,
    },
    {
      path: '/alerts',
      redirect: '/',
    },
    {
      path: '/jobs',
      name: 'jobs',
      component: () => import('@/views/JobsView.vue'),
    },
    {
      path: '/workflow',
      name: 'workflow',
      component: () => import('@/views/WorkflowView.vue'),
    },
  ],
})

export default router
