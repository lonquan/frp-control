import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '../views/DashboardPage.vue'
import ClientsPage from '../views/ClientsPage.vue'
import ProxiesPage from '../views/ProxiesPage.vue'
import ConfigWorkspace from '@shared/components/ConfigWorkspace.vue'
import SettingsPage from '@shared/components/SettingsPage.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: Dashboard },
    { path: '/clients', name: 'clients', component: ClientsPage },
    { path: '/proxies', name: 'proxies', component: ProxiesPage },
    {
      path: '/config',
      name: 'config',
      component: ConfigWorkspace,
      props: { role: 'server' },
    },
    { path: '/settings', name: 'settings', component: SettingsPage },
    { path: '/login', redirect: '/' },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
