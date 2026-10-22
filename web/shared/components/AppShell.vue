<template>
  <a-layout class="app-shell" :class="`role-${role}`">
    <a href="#main-content" class="skip-link">跳转到主要内容</a>
    <a-layout-sider
      class="app-sider"
      :collapsed="collapsed"
      :width="236"
      :collapsed-width="76"
      collapse-mode="width"
    >
      <div class="brand-block">
        <div class="brand-mark" aria-hidden="true">
          <LogoIcon />
        </div>
        <div v-if="!collapsed" class="brand-copy">
          <strong>FRP 控制台</strong>
          <span>{{ roleLabel }}</span>
        </div>
      </div>

      <a-menu
        class="nav-menu"
        :selected-keys="[activePath]"
        :collapsed="collapsed"
        @menu-item-click="navigate"
      >
        <a-menu-item
          v-for="item in items"
          :key="item.path"
          :title="item.label"
          :aria-label="item.label"
        >
          <template #icon>
            <component :is="item.icon" />
          </template>
          {{ item.label }}
        </a-menu-item>
      </a-menu>

      <div v-if="!collapsed" class="sider-foot">
        <div class="health-row">
          <span :class="['health-dot', healthState]" aria-hidden="true"></span>
          <span>{{ healthLabel }}</span>
        </div>
        <span class="version">FRP 管理面板</span>
      </div>
    </a-layout-sider>

    <a-layout class="workspace">
      <a-layout-header class="topbar">
        <div class="topbar-left">
          <a-button
            class="icon-button"
            shape="circle"
            :aria-label="collapsed ? '展开导航' : '收起导航'"
            @click="toggleSidebar"
          >
            <IconMenuFold v-if="!collapsed" />
            <IconMenuUnfold v-else />
          </a-button>
          <div class="role-chip">
            <span class="role-dot" aria-hidden="true" />
            {{ roleLabel }}
          </div>
        </div>

        <div class="topbar-right">
          <div class="health-pill" :class="healthClass" role="status">
            <span :class="['health-dot', healthState]" aria-hidden="true"></span>
            {{ healthLabel }}
          </div>
          <a-tooltip :content="isDark ? '切换到浅色主题' : '切换到深色主题'">
            <a-button
              class="icon-button"
              shape="circle"
              :aria-label="isDark ? '切换到浅色主题' : '切换到深色主题'"
              @click="toggleTheme"
            >
              <IconSun v-if="isDark" />
              <IconMoon v-else />
            </a-button>
          </a-tooltip>
          <a-dropdown trigger="click">
            <a-button class="account-button">
              <template #icon><IconUser /></template>
              <span>{{ session.displayName }}</span>
            </a-button>
            <template #content>
              <a-doption @click="emit('password')">
                <template #icon><IconLock /></template>
                修改密码
              </a-doption>
              <a-doption class="danger-option" @click="logout">
                <template #icon><IconExport /></template>
                退出登录
              </a-doption>
            </template>
          </a-dropdown>
        </div>
      </a-layout-header>

      <a-layout-content id="main-content" class="main-content" tabindex="-1">
        <slot />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDark, useLocalStorage, useMediaQuery } from '@vueuse/core'
import {
  IconExport,
  IconLock,
  IconMenuFold,
  IconMenuUnfold,
  IconMoon,
  IconSun,
  IconUser,
} from '@arco-design/web-vue/es/icon'
import LogoIcon from '../assets/logo.svg?component'
import { useSessionStore } from '../session'

export interface NavItem {
  path: string
  label: string
  icon: unknown
}

const props = defineProps<{
  role: 'server' | 'client'
  items: NavItem[]
}>()

const emit = defineEmits<{
  password: []
}>()

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const sidebarCollapsed = useLocalStorage('frp-sidebar-collapsed', false)
const narrowViewport = useMediaQuery('(max-width: 1180px)')
const collapsed = ref(sidebarCollapsed.value || narrowViewport.value)
watch(
  narrowViewport,
  (narrow) => {
    collapsed.value = narrow || sidebarCollapsed.value
  },
  { immediate: true },
)
const isDark = useDark({
  selector: 'html',
  attribute: 'class',
  valueDark: 'dark',
  valueLight: '',
})
const healthState = ref<'ok' | 'down' | 'checking'>('checking')
let healthTimer: number | undefined

const roleLabel = computed(() => (props.role === 'server' ? '服务端' : '客户端'))
const activePath = computed(() => {
  const match = props.items
    .filter((item) => route.path === item.path || route.path.startsWith(`${item.path}/`))
    .sort((a, b) => b.path.length - a.path.length)[0]
  return match?.path || '/'
})
const healthLabel = computed(() => {
  if (healthState.value === 'ok') return '服务运行正常'
  if (healthState.value === 'down') return '服务连接异常'
  return '正在检查服务'
})
const healthClass = computed(() => healthState.value)

watch(
  isDark,
  (dark) => {
    document.body.setAttribute('arco-theme', dark ? 'dark' : 'light')
  },
  { immediate: true },
)

async function checkHealth() {
  try {
    const response = await fetch('/healthz', { cache: 'no-store' })
    healthState.value = response.ok ? 'ok' : 'down'
  } catch {
    healthState.value = 'down'
  }
}

function toggleTheme() {
  isDark.value = !isDark.value
}

function toggleSidebar() {
  const next = !collapsed.value
  collapsed.value = next
  sidebarCollapsed.value = next
}

function navigate(path: string) {
  if (router.currentRoute.value.path !== path) {
    router.push(path)
  }
}

async function logout() {
  await session.logout()
  await router.replace('/login')
}

onMounted(() => {
  checkHealth()
  healthTimer = window.setInterval(checkHealth, 15_000)
})

onBeforeUnmount(() => {
  if (healthTimer) window.clearInterval(healthTimer)
})
</script>

<style scoped>
.app-shell {
  height: 100dvh;
  min-height: 0;
  overflow: hidden;
  background: var(--frp-bg);
}

.skip-link {
  position: fixed;
  top: 8px;
  left: 8px;
  z-index: 1100;
  transform: translateY(-140%);
  border-radius: 8px;
  padding: 10px 14px;
  background: var(--color-bg-2);
  color: rgb(var(--primary-6));
  box-shadow: 0 8px 22px rgb(15 23 42 / 20%);
}

.skip-link:focus {
  transform: translateY(0);
}

.app-sider {
  position: relative;
  top: auto;
  height: 100dvh;
  flex: 0 0 auto;
  overflow: hidden;
  border-right: 1px solid var(--frp-border);
  background: var(--frp-sider);
  box-shadow: none;
}

.app-sider :deep(.arco-layout-sider-children) {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
}

.brand-block {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 64px;
  min-height: 64px;
  padding: 12px 18px;
  border-bottom: 1px solid var(--frp-border);
  overflow: hidden;
}

.brand-mark {
  display: grid;
  flex: 0 0 40px;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: 12px;
  background: linear-gradient(145deg, var(--frp-role-strong), var(--frp-role-soft));
  box-shadow: 0 10px 24px color-mix(in srgb, var(--frp-role-strong) 22%, transparent);
}

.brand-mark :deep(svg) {
  width: 25px;
  height: 25px;
}

.brand-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
  white-space: nowrap;
}

.brand-copy strong {
  color: var(--frp-text);
  font-size: 16px;
  letter-spacing: 0.02em;
}

.brand-copy span {
  color: var(--frp-muted);
  font-size: 12px;
}

.nav-menu {
  width: 100%;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  padding: 14px 10px;
  background: transparent;
}

.nav-menu :deep(.arco-menu-inner) {
  overflow-x: hidden;
  overflow-y: auto;
}

.nav-menu.arco-menu-collapsed :deep(.arco-menu-inner) {
  padding: 4px;
}

.nav-menu.arco-menu-collapsed :deep(.arco-menu-item) {
  width: 100%;
  justify-content: center;
  padding: 0;
}

.nav-menu.arco-menu-collapsed :deep(.arco-menu-item-inner) {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
  opacity: 1;
}

.nav-menu.arco-menu-collapsed :deep(.arco-menu-icon),
.nav-menu.arco-menu-collapsed :deep(.arco-menu-icon .arco-icon) {
  margin: 0;
}

.nav-menu :deep(.arco-menu-item) {
  min-height: 44px;
  margin-bottom: 4px;
  border-radius: 10px;
  background: transparent !important;
  color: var(--frp-muted-strong);
  font-size: 15px;
}

.nav-menu :deep(.arco-menu-selected) {
  background: var(--frp-role-faint) !important;
  color: var(--frp-role-strong);
  font-weight: 600;
}

.nav-menu :deep(.arco-menu-item:hover) {
  background: var(--frp-surface-muted) !important;
}

.sider-foot {
  margin: 12px;
  padding: 14px;
  border: 1px solid var(--frp-border);
  border-radius: 12px;
  background: var(--frp-surface-muted);
}

.health-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--frp-text);
  font-size: 13px;
}

.version {
  display: block;
  margin-top: 6px;
  color: var(--frp-muted);
  font-size: 11px;
}

.health-dot,
.role-dot {
  width: 8px;
  height: 8px;
  flex: 0 0 8px;
  border-radius: 50%;
  background: var(--color-text-4);
}

.health-dot.ok {
  background: var(--frp-success);
  box-shadow: 0 0 0 4px var(--frp-success-glow);
}

.health-dot.down {
  background: var(--frp-danger);
  box-shadow: 0 0 0 4px var(--frp-danger-glow);
}

.health-dot.checking {
  background: var(--frp-warning);
  box-shadow: 0 0 0 4px var(--frp-warning-glow);
}

.workspace {
  width: 0;
  height: 100dvh;
  min-width: 0;
  min-height: 0;
  flex: 1 1 0;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 100;
  display: flex;
  height: 64px;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  border-bottom: 1px solid var(--frp-border);
  background: color-mix(in srgb, var(--frp-bg) 88%, transparent);
  backdrop-filter: blur(16px);
}

.topbar-left,
.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.icon-button {
  width: 40px;
  height: 40px;
  border-color: var(--frp-border);
  background: var(--frp-surface);
  color: var(--frp-muted-strong);
}

.role-chip,
.health-pill {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--frp-border);
  border-radius: 999px;
  padding: 0 12px;
  background: var(--frp-surface);
  color: var(--frp-muted-strong);
  font-size: 13px;
}

.role-dot {
  background: var(--frp-role-strong);
}

.health-pill.ok {
  border-color: color-mix(in srgb, var(--frp-success) 34%, var(--frp-border));
  background: color-mix(in srgb, var(--frp-success) 7%, var(--frp-surface));
  color: var(--frp-success-text);
}

.health-pill.down {
  border-color: color-mix(in srgb, var(--frp-danger) 34%, var(--frp-border));
  background: color-mix(in srgb, var(--frp-danger) 7%, var(--frp-surface));
  color: var(--frp-danger-text);
}

.account-button {
  min-height: 40px;
  border-color: var(--frp-border);
  background: var(--frp-surface);
  color: var(--frp-text);
}

.danger-option {
  color: rgb(var(--danger-6));
}

.main-content {
  min-width: 0;
  min-height: calc(100dvh - 64px);
  padding: 28px 30px 40px;
  background:
    radial-gradient(circle at 100% 0, var(--frp-role-glow), transparent 28%),
    var(--frp-bg);
}

.main-content:focus {
  outline: none;
}

@media (max-width: 1180px) {
  .main-content {
    padding: 24px 22px 34px;
  }

  .health-pill {
    display: none;
  }
}

@media (max-width: 900px) {
  .topbar {
    padding: 0 16px;
  }

  .main-content {
    padding: 20px 16px 30px;
  }

  .account-button span {
    display: none;
  }
}
</style>
