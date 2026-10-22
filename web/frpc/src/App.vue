<template>
  <div v-if="!session.ready" class="app-loading" role="status">
    <div class="loading-mark"><LogoIcon /></div>
    <span>正在连接 FRP 客户端…</span>
  </div>

  <LoginView v-else-if="!session.authenticated" role="client" />

  <AppShell v-else role="client" :items="navItems" @password="router.push('/settings')">
    <router-view />
  </AppShell>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  IconApps,
  IconSafe,
  IconDashboard,
  IconFile,
  IconLink,
  IconSettings,
} from '@arco-design/web-vue/es/icon'
import AppShell from '@shared/components/AppShell.vue'
import LoginView from '@shared/components/LoginView.vue'
import { useSessionStore } from '@shared/session'
import LogoIcon from '@shared/assets/logo.svg?component'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()

const navItems = [
  { path: '/', label: '总览', icon: IconDashboard },
  { path: '/proxies', label: '代理', icon: IconLink },
  { path: '/visitors', label: '访问者', icon: IconDashboard },
  { path: '/config', label: '配置', icon: IconApps },
  { path: '/certificates', label: '证书', icon: IconSafe },
  { path: '/captures', label: '流量捕获', icon: IconFile },
  { path: '/settings', label: '设置', icon: IconSettings },
]

onMounted(async () => {
  await session.restore()
  if (session.authenticated && route.path === '/login') {
    await router.replace('/')
  }
})

watch(
  () => session.authenticated,
  (authenticated) => {
    if (!authenticated) {
      session.redirectTo = route.fullPath
    }
  },
)
</script>

<style>
.app-loading {
  display: flex;
  min-height: 100dvh;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 16px;
  background:
    radial-gradient(circle at 50% 30%, rgb(15 118 110 / 12%), transparent 28%),
    var(--frp-bg);
  color: var(--frp-muted-strong);
}

.loading-mark {
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  border-radius: 17px;
  background: linear-gradient(145deg, #0f766e, #0369a1);
  box-shadow: 0 18px 42px rgb(15 118 110 / 24%);
}

.loading-mark svg {
  width: 34px;
  height: 34px;
}
</style>
