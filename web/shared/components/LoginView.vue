<template>
  <main class="login-page" :class="`role-${role}`">
    <a-button
      class="login-theme"
      shape="circle"
      :aria-label="isDark ? '切换到浅色主题' : '切换到深色主题'"
      @click="isDark = !isDark"
    >
      <IconSun v-if="isDark" />
      <IconMoon v-else />
    </a-button>
    <section class="login-panel" aria-labelledby="login-title">
      <div class="brand-row">
        <div class="brand-mark" aria-hidden="true"><LogoIcon /></div>
        <div>
          <span class="product">FRP CONTROL</span>
          <strong>FRP 管理控制台</strong>
        </div>
        <span class="role-chip">{{ roleLabel }}</span>
      </div>

      <div class="login-copy">
        <h1 id="login-title">登录{{ roleLabel }}</h1>
      </div>

      <a-form
        ref="formRef"
        :model="form"
        layout="vertical"
        class="login-form"
        @submit.prevent="submit"
      >
        <a-form-item
          field="username"
          label="管理员账号"
          :rules="[{ required: true, message: '请输入管理员账号' }]"
        >
          <a-input
            v-model="form.username"
            size="large"
            autocomplete="username"
            placeholder="请输入管理员账号"
            allow-clear
          >
            <template #prefix><IconUser /></template>
          </a-input>
        </a-form-item>
        <a-form-item
          field="password"
          label="密码"
          :rules="[{ required: true, message: '请输入密码' }]"
        >
          <a-input-password
            v-model="form.password"
            size="large"
            autocomplete="current-password"
            placeholder="请输入密码"
            @keydown.enter.prevent="submit"
          >
            <template #prefix><IconLock /></template>
          </a-input-password>
        </a-form-item>
        <a-alert
          v-if="errorMessage"
          type="error"
          :show-icon="true"
          class="login-error"
        >
          {{ errorMessage }}
        </a-alert>
        <a-button
          type="primary"
          html-type="button"
          size="large"
          long
          :loading="loading"
          @click="submit"
        >
          登录控制台
        </a-button>
      </a-form>

    </section>
    <aside class="visual-panel" aria-hidden="true">
      <div class="grid-lines" />
      <div class="signal-card signal-a">
        <span>隧道状态</span>
        <strong>安全接入</strong>
      </div>
      <div class="signal-card signal-b">
        <span>管理入口</span>
        <strong>会话认证</strong>
      </div>
      <div class="orbit orbit-a" />
      <div class="orbit orbit-b" />
      <div class="visual-copy">
        <span>PRIVATE NETWORK OPERATIONS</span>
        <h2>让每一次隧道连接<br />清晰、可控、可追踪。</h2>
        <p>代理状态、配置文件、证书与诊断能力统一在一个工作台中。</p>
      </div>
    </aside>
  </main>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useDark } from '@vueuse/core'
import { Message } from '@arco-design/web-vue'
import {
  IconLock,
  IconMoon,
  IconSun,
  IconUser,
} from '@arco-design/web-vue/es/icon'
import LogoIcon from '../assets/logo.svg?component'
import { useSessionStore } from '../session'

const props = defineProps<{
  role: 'server' | 'client'
}>()

const router = useRouter()
const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')
const formRef = ref()
const form = reactive({ username: '', password: '' })
const roleLabel = computed(() => (props.role === 'server' ? '服务端' : '客户端'))
const isDark = useDark({
  selector: 'html',
  attribute: 'class',
  valueDark: 'dark',
  valueLight: '',
})

watch(
  isDark,
  (dark) => {
    document.body.setAttribute('arco-theme', dark ? 'dark' : 'light')
  },
  { immediate: true },
)

async function submit() {
  const errors = await formRef.value?.validate()
  if (errors) return
  loading.value = true
  errorMessage.value = ''
  try {
    await session.login(form.username.trim(), form.password)
    Message.success('登录成功')
    await router.replace(session.redirectTo || '/')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  --login-primary: #1d4ed8;
  --login-secondary: #0e7490;
  display: grid;
  min-height: 100dvh;
  grid-template-columns: minmax(460px, 0.86fr) minmax(520px, 1.14fr);
  background: var(--frp-bg, #f3f6fb);
}

.login-theme {
  position: fixed;
  top: 24px;
  right: 24px;
  z-index: 10;
  width: 40px;
  height: 40px;
  border-color: var(--frp-border, #dce3ee);
  background: var(--frp-surface, #fff);
  color: var(--frp-muted-strong, #4b5b73);
}

.login-page.role-client {
  --login-primary: #0f766e;
  --login-secondary: #0e7490;
}

.login-panel {
  display: flex;
  width: min(520px, calc(100% - 64px));
  flex-direction: column;
  justify-content: center;
  margin: 0 auto;
  padding: 48px 0;
}

.brand-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-mark {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border-radius: 13px;
  background: linear-gradient(145deg, var(--login-primary), var(--login-secondary));
  box-shadow: 0 14px 30px rgb(29 78 216 / 20%);
}

.brand-mark :deep(svg) {
  width: 27px;
  height: 27px;
}

.brand-row > div:nth-child(2) {
  display: flex;
  flex-direction: column;
}

.product {
  color: var(--login-primary);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.brand-row strong {
  color: var(--frp-text, #162033);
  font-size: 16px;
}

.role-chip {
  margin-left: auto;
  border: 1px solid color-mix(in srgb, var(--login-primary) 25%, transparent);
  border-radius: 999px;
  padding: 6px 10px;
  background: color-mix(in srgb, var(--login-primary) 8%, transparent);
  color: var(--login-chip-text, var(--login-primary));
  font-size: 12px;
  font-weight: 600;
}

html.dark .login-page.role-server {
  --login-chip-text: #93c5fd;
}

html.dark .login-page.role-client {
  --login-chip-text: #5eead4;
}

.login-copy {
  margin: 58px 0 28px;
}

.login-copy h1 {
  margin: 0;
  color: var(--frp-text, #162033);
  font-size: 34px;
  letter-spacing: -0.03em;
}

.login-form :deep(.arco-form-item-label-col) {
  padding-bottom: 8px;
}

.login-form :deep(.arco-form-item-label) {
  color: var(--frp-text, #162033);
  font-size: 14px;
  font-weight: 600;
}

.login-form :deep(.arco-input-wrapper) {
  height: 48px;
  border-color: var(--frp-border, #dce3ee);
  border-radius: 12px;
  background: var(--frp-surface, #fff);
}

.login-error {
  margin-bottom: 16px;
}

.login-form :deep(.arco-btn-primary) {
  height: 48px;
  border-radius: 12px;
  background: var(--login-primary);
}

.visual-panel {
  position: relative;
  display: flex;
  overflow: hidden;
  align-items: flex-end;
  margin: 18px;
  border-radius: 24px;
  padding: 58px;
  background:
    radial-gradient(circle at 72% 22%, rgb(56 189 248 / 22%), transparent 22%),
    linear-gradient(145deg, #0b1930 0%, #0f2441 55%, #0b2730 100%);
  color: white;
}

.grid-lines {
  position: absolute;
  inset: 0;
  opacity: 0.16;
  background-image:
    linear-gradient(rgb(255 255 255 / 22%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(255 255 255 / 22%) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: linear-gradient(to bottom, black, transparent 86%);
}

.visual-copy {
  position: relative;
  z-index: 2;
  max-width: 620px;
}

.visual-copy > span {
  color: #67e8f9;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
}

.visual-copy h2 {
  margin: 18px 0;
  font-size: clamp(34px, 4vw, 58px);
  line-height: 1.16;
  letter-spacing: -0.04em;
}

.visual-copy p {
  max-width: 520px;
  margin: 0;
  color: rgb(226 232 240 / 72%);
  font-size: 15px;
  line-height: 1.8;
}

.signal-card {
  position: absolute;
  z-index: 2;
  display: flex;
  width: 170px;
  flex-direction: column;
  gap: 8px;
  border: 1px solid rgb(255 255 255 / 15%);
  border-radius: 15px;
  padding: 16px;
  background: rgb(15 23 42 / 60%);
  box-shadow: 0 18px 50px rgb(2 6 23 / 28%);
  backdrop-filter: blur(14px);
}

.signal-card span {
  color: rgb(226 232 240 / 58%);
  font-size: 11px;
}

.signal-card strong {
  font-size: 16px;
}

.signal-a {
  top: 88px;
  right: 100px;
}

.signal-b {
  top: 220px;
  right: 210px;
}

.orbit {
  position: absolute;
  border: 1px solid rgb(103 232 249 / 20%);
  border-radius: 50%;
}

.orbit-a {
  top: -180px;
  right: -90px;
  width: 520px;
  height: 520px;
}

.orbit-b {
  top: -110px;
  right: -20px;
  width: 390px;
  height: 390px;
}

@media (max-width: 1024px) {
  .login-page {
    grid-template-columns: 1fr;
  }

  .visual-panel {
    display: none;
  }

  .login-panel {
    width: min(500px, calc(100% - 40px));
  }
}
</style>
