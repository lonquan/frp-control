<template>
  <div v-if="!session.ready" class="app-loading" role="status">
    <div class="loading-mark"><LogoIcon /></div>
    <span>正在连接 FRP 客户端…</span>
  </div>

  <LoginView v-else-if="!session.authenticated" role="client" />

  <AppShell v-else role="client" :items="clientNavItems" @password="router.push('/settings')">
    <router-view />
  </AppShell>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '@shared/components/AppShell.vue'
import LoginView from '@shared/components/LoginView.vue'
import { useSessionStore } from '@shared/session'
import LogoIcon from '@shared/assets/logo.svg?component'
import { clientNavItems } from './navigation'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()

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
