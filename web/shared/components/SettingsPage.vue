<template>
  <div class="page-stack">
    <PageHeader
      title="设置"
      description="管理员会话、安全设置和 Supervisor 运行状态。"
      eyebrow="SYSTEM"
    />

    <div class="settings-grid">
      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>管理员账号</strong>
            <span>单管理员会话，不使用 TOTP</span>
          </div>
          <a-tag color="green">已登录</a-tag>
        </div>
        <div class="panel-body">
          <div class="detail-grid">
            <div class="detail-item">
              <span>用户名</span>
              <strong>{{ session.displayName }}</strong>
            </div>
            <div class="detail-item">
              <span>角色</span>
              <strong>{{ session.role === 'server' ? '服务端管理员' : '客户端管理员' }}</strong>
            </div>
            <div class="detail-item">
              <span>会话到期</span>
              <strong>{{ formatDateTime(session.expiresAt) }}</strong>
            </div>
            <div class="detail-item">
              <span>认证方式</span>
              <strong>HttpOnly 服务端会话</strong>
            </div>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>Supervisor</strong>
            <span>容器进程生命周期管理</span>
          </div>
          <a-button size="small" :loading="loading" @click="loadSupervisor">
            <template #icon><IconRefresh /></template>
            刷新
          </a-button>
        </div>
        <div class="panel-body">
          <div class="detail-grid">
            <div class="detail-item">
              <span>状态</span>
              <strong>{{ supervisorStateLabel(supervisor?.state) }}</strong>
            </div>
            <div class="detail-item">
              <span>子进程 PID</span>
              <strong>{{ supervisor?.pid || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>启动时间</span>
              <strong>{{ formatDateTime(supervisor?.startedAt) }}</strong>
            </div>
            <div class="detail-item">
              <span>最近重启</span>
              <strong>{{ formatDateTime(supervisor?.restartedAt) }}</strong>
            </div>
          </div>
          <a-alert
            v-if="supervisor?.lastError"
            type="error"
            :show-icon="true"
            class="settings-alert"
          >
            {{ supervisor.lastError }}
          </a-alert>
        </div>
      </section>

      <section class="panel password-card">
        <div class="panel-head">
          <div class="panel-title">
            <strong>账号安全</strong>
            <span>修改后需要重新登录</span>
          </div>
        </div>
        <div class="panel-body password-summary">
          <div class="password-summary-copy">
            <span class="password-summary-icon"><IconLock /></span>
            <div>
              <strong>管理员密码</strong>
              <p>更新密码会注销所有现有会话，并要求你重新登录。</p>
            </div>
          </div>
          <a-button @click="openPasswordModal">修改密码</a-button>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-head">
        <div class="panel-title">
          <strong>管理入口说明</strong>
          <span>面板是唯一受支持的管理方式</span>
        </div>
      </div>
      <div class="panel-body">
        <a-alert type="info" :show-icon="true">
          Basic Auth、管理 CLI、Prometheus 和外部 pprof 入口已移除。管理与观察统一通过本面板完成。
        </a-alert>
        <div class="config-security">
          <IconLock />
          <div>
            <strong>WebServer 网络字段受保护</strong>
            <p>
              修改地址、端口、TLS 或 AssetsDir 需要显式设置
              <code>FRP_ALLOW_DASHBOARD_NETWORK_CHANGES=1</code>，避免保存后面板失联。
            </p>
          </div>
        </div>
      </div>
    </section>

    <a-modal
      v-model:visible="passwordVisible"
      title="修改密码"
      :width="480"
      :mask-closable="false"
      :ok-loading="saving"
      :on-before-ok="changePassword"
      ok-text="更新密码"
      cancel-text="取消"
    >
      <a-alert type="warning" :show-icon="true" class="password-modal-alert">
        更新后所有现有会话都会失效。
      </a-alert>
      <a-form
        ref="formRef"
        :model="form"
        layout="vertical"
        class="password-form"
        @submit.prevent="changePassword"
      >
        <a-form-item
          field="currentPassword"
          label="当前密码"
          :rules="[{ required: true, message: '请输入当前密码' }]"
        >
          <a-input-password
            v-model="form.currentPassword"
            size="large"
            autocomplete="current-password"
          />
        </a-form-item>
        <a-form-item
          field="newPassword"
          label="新密码"
          :rules="[
            { required: true, message: '请输入新密码' },
            { minLength: 8, message: '新密码至少需要 8 个字符' },
          ]"
        >
          <a-input-password
            v-model="form.newPassword"
            size="large"
            autocomplete="new-password"
          />
        </a-form-item>
        <a-form-item
          field="confirmPassword"
          label="确认新密码"
          :rules="[
            { required: true, message: '请再次输入新密码' },
            {
              validator: (value: string, callback: (message?: string) => void) =>
                value === form.newPassword
                  ? callback()
                  : callback('两次输入的新密码不一致'),
            },
          ]"
        >
          <a-input-password
            v-model="form.confirmPassword"
            size="large"
            autocomplete="new-password"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { IconLock, IconRefresh } from '@arco-design/web-vue/es/icon'
import { adminAPI, type SupervisorStatus } from '../admin'
import { useSessionStore } from '../session'
import { formatDateTime, supervisorStateLabel } from '../supervisor'
import PageHeader from './PageHeader.vue'

const router = useRouter()
const session = useSessionStore()
const formRef = ref()
const saving = ref(false)
const loading = ref(false)
const passwordVisible = ref(false)
const supervisor = ref<SupervisorStatus>()
const form = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})

onMounted(loadSupervisor)
watch(passwordVisible, (visible) => {
  if (!visible) resetPasswordForm()
})

async function loadSupervisor() {
  loading.value = true
  try {
    supervisor.value = await adminAPI.supervisor()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取 Supervisor 状态失败')
  } finally {
    loading.value = false
  }
}

async function openPasswordModal() {
  resetPasswordForm()
  passwordVisible.value = true
  await nextTick()
  formRef.value?.clearValidate()
}

async function changePassword() {
  const errors = await formRef.value?.validate()
  if (errors) return false
  saving.value = true
  try {
    await adminAPI.changePassword(form.currentPassword, form.newPassword)
    passwordVisible.value = false
    Message.success('密码已更新，请重新登录')
    session.clearSession()
    await router.replace('/login')
    return true
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '修改密码失败')
    return false
  } finally {
    saving.value = false
  }
}

function resetPasswordForm() {
  form.currentPassword = ''
  form.newPassword = ''
  form.confirmPassword = ''
  formRef.value?.clearValidate()
}
</script>

<style scoped>
.settings-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: stretch;
  gap: 16px;
}

.password-summary {
  display: flex;
  min-height: 132px;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.password-summary-copy {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 12px;
}

.password-summary-copy > div {
  min-width: 0;
}

.password-summary-copy strong {
  color: var(--frp-text);
  font-size: 14px;
}

.password-summary-copy p {
  margin: 4px 0 0;
  color: var(--frp-muted);
  font-size: 12px;
  line-height: 1.65;
}

.password-summary-icon {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border-radius: 10px;
  background: var(--frp-role-faint);
  color: var(--frp-role-strong);
  font-size: 18px;
}

.password-form {
  width: 100%;
}

.password-form :deep(.arco-form-item) {
  margin-bottom: 14px;
}

.password-form :deep(.arco-input-wrapper) {
  border-radius: 10px;
}

.password-modal-alert {
  margin-bottom: 16px;
}

.settings-alert {
  margin-top: 16px;
}

.config-security {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  margin-top: 16px;
  border: 1px solid var(--frp-border);
  border-radius: 12px;
  padding: 16px;
  background: var(--frp-surface-muted);
  color: var(--frp-muted-strong);
}

.config-security > svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--frp-role-strong);
  font-size: 20px;
}

.config-security strong {
  color: var(--frp-text);
}

.config-security p {
  margin: 5px 0 0;
  line-height: 1.7;
}

.config-security code {
  color: var(--frp-role-strong);
  overflow-wrap: anywhere;
}

@media (max-width: 1320px) {
  .settings-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .password-summary {
    align-items: stretch;
    flex-direction: column;
  }

  .password-summary :deep(.arco-btn) {
    width: 100%;
  }
}
</style>
