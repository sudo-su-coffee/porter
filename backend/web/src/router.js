import { createRouter, createWebHistory } from 'vue-router'
import { store } from './api'
import MainLayout from './layouts/MainLayout.vue'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('./views/Login.vue'),
    meta: { public: true }
  },
  {
    path: '/',
    component: MainLayout,
    children: [
      { path: '', name: 'dashboard', component: () => import('./views/Dashboard.vue') },
      { path: 'projects', name: 'projects', component: () => import('./views/Projects.vue') },
      { path: 'projects/:id', name: 'project', component: () => import('./views/ProjectDetail.vue') },
      { path: 'host', name: 'host', component: () => import('./views/Host.vue') },
      { path: 'images', name: 'images', component: () => import('./views/Images.vue') },
      { path: 'admin', name: 'admin', component: () => import('./views/Admin.vue') }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to) => {
  if (!to.meta.public && !store.token) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && store.token) {
    return { name: 'dashboard' }
  }
  return true
})

export default router