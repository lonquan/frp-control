<template>
  <div v-if="!session.ready" class="app-loading" role="status">
    <div class="loading-mark"><LogoIcon /></div>
    <span>正在连接 FRP 服务端…</span>
  </div>

  <LoginView v-else-if="!session.authenticated" role="server" />

  <AppShell v-else role="server" :items="navItems" @password="router.push('/settings')">
    <router-view />
  </AppShell>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  IconApps,
  IconDashboard,
  IconDesktop,
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
  { path: '/clients', label: '客户端', icon: IconDesktop },
  { path: '/proxies', label: '代理', icon: IconLink },
  { path: '/config', label: '配置', icon: IconApps },
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
    radial-gradient(circle at 50% 30%, rgb(37 99 235 / 12%), transparent 28%),
    var(--frp-bg);
  color: var(--frp-muted-strong);
}

.loading-mark {
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  border-radius: 17px;
  background: linear-gradient(145deg, #1d4ed8, #0e7490);
  box-shadow: 0 18px 42px rgb(29 78 216 / 24%);
}

.loading-mark svg {
  width: 34px;
  height: 34px;
}
</style>
