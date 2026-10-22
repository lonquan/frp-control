import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '../views/DashboardPage.vue'
import ProxiesPage from '../views/ProxiesPage.vue'
import VisitorsPage from '../views/VisitorsPage.vue'
import CertificatesPage from '../views/CertificatesPage.vue'
import CapturesPage from '../views/CapturesPage.vue'
import ConfigWorkspace from '@shared/components/ConfigWorkspace.vue'
import SettingsPage from '@shared/components/SettingsPage.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: Dashboard },
    { path: '/proxies', name: 'proxies', component: ProxiesPage },
    { path: '/visitors', name: 'visitors', component: VisitorsPage },
    {
      path: '/config',
      name: 'config',
      component: ConfigWorkspace,
      props: { role: 'client' },
    },
    { path: '/certificates', name: 'certificates', component: CertificatesPage },
    { path: '/captures', name: 'captures', component: CapturesPage },
    { path: '/diagnostics', redirect: '/' },
    { path: '/settings', name: 'settings', component: SettingsPage },
    { path: '/login', redirect: '/' },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
