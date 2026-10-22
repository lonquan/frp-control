<template>
  <div class="page-stack">
    <PageHeader
      title="服务端总览"
      description="实时掌握客户端、代理、连接和流量状态。"
      eyebrow="SERVER OVERVIEW"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-button :loading="restarting" @click="openRestartDialog">
        <template #icon><IconPoweroff /></template>
        重启服务
      </a-button>
      <a-button type="primary" @click="router.push('/config')">
        <template #icon><IconSettings /></template>
        管理配置
      </a-button>
    </PageHeader>

    <section class="metric-grid" aria-label="服务端关键指标">
      <MetricCard
        label="在线客户端"
        :value="metrics.onlineClients || 0"
        :hint="`累计记录 ${metrics.knownClients || 0}`"
        tone="blue"
        :icon="IconDesktop"
        to="/clients"
        action="查看客户端"
      />
      <MetricCard
        label="在线代理"
        :value="metrics.proxies || 0"
        hint="当前已注册代理"
        tone="green"
        :icon="IconLink"
        to="/proxies"
        action="查看代理"
      />
      <MetricCard
        label="当前连接"
        :value="metrics.curConns || 0"
        hint="当前业务连接数"
        tone="violet"
        :icon="IconSwap"
      />
      <MetricCard
        label="今日流量"
        :value="formatBytes(totalTraffic)"
        :hint="`入站 ${formatBytes(metrics.totalTrafficIn || 0)}`"
        tone="amber"
        :icon="IconCloud"
      />
    </section>

    <div class="diagnostic-grid">
      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>运行信息</strong>
            <span>Go 运行时与实例元数据</span>
          </div>
          <StatusBadge :status="supervisor?.state || 'checking'" />
        </div>
        <div class="panel-body">
          <div class="detail-grid runtime-grid">
            <div
              v-for="item in runtimeItems"
              :key="item.label"
              class="detail-item"
            >
              <span>{{ item.label }}</span>
              <strong :class="{ mono: item.mono }">{{ item.value }}</strong>
            </div>
          </div>
        </div>
      </section>

      <section class="panel diagnostic-panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>性能诊断</strong>
            <span>产物仅通过当前会话下载</span>
          </div>
          <div class="diagnostic-switch">
            <span>{{ pprofEnabled ? '已开启' : '已关闭' }}</span>
            <a-switch
              v-model="pprofEnabled"
              :loading="savingSettings"
              aria-label="启用性能诊断"
              @change="saveSettings"
            />
          </div>
        </div>
        <div class="panel-body diagnostic-actions">
          <a-alert v-if="!pprofEnabled" type="info" :show-icon="true">
            开启后可下载 Goroutine、Heap 和 CPU profile。CPU 采样会阻塞约 15 秒。
          </a-alert>
          <div class="diagnostic-buttons">
            <a-button
              :disabled="!pprofEnabled"
              :loading="downloading === 'goroutine'"
              @click="download('goroutine')"
            >
              <template #icon><IconBranch /></template>
              Goroutine 快照
            </a-button>
            <a-button
              :disabled="!pprofEnabled"
              :loading="downloading === 'heap'"
              @click="download('heap')"
            >
              <template #icon><IconStorage /></template>
              Heap profile
            </a-button>
            <a-button
              :disabled="!pprofEnabled"
              :loading="downloading === 'cpu'"
              @click="download('cpu')"
            >
              <template #icon><IconDashboard /></template>
              CPU profile
            </a-button>
          </div>
        </div>
      </section>
    </div>

    <div class="overview-grid">
      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>代理类型分布</strong>
            <span>当前在线代理构成</span>
          </div>
          <a-tag color="arcoblue">{{ proxyTotal }} 个</a-tag>
        </div>
        <div class="panel-body distribution-list">
          <div
            v-for="item in proxyDistribution"
            :key="item.type"
            class="distribution-row"
          >
            <span class="distribution-label">{{ item.type.toUpperCase() }}</span>
            <div class="distribution-track">
              <div
                class="distribution-fill"
                :style="{ width: `${item.percent}%` }"
              />
            </div>
            <span class="distribution-value">{{ item.count }}</span>
          </div>
          <a-empty v-if="proxyDistribution.length === 0" description="暂无在线代理">
            <template #image>
              <div class="empty-icon"><IconLink /></div>
            </template>
          </a-empty>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>服务监听</strong>
            <span>FRPS 当前配置端口</span>
          </div>
          <StatusBadge :status="supervisor?.state || 'checking'" />
        </div>
        <div class="panel-body listener-grid">
          <div class="listener-item">
            <IconApps />
            <div>
              <span>FRP 主端口</span>
              <strong>TCP {{ metrics.bindPort || '-' }}</strong>
            </div>
          </div>
          <div class="listener-item">
            <IconPublic />
            <div>
              <span>HTTP Vhost</span>
              <strong>{{ metrics.vhostHTTPPort || '未启用' }}</strong>
            </div>
          </div>
          <div class="listener-item">
            <IconSafe />
            <div>
              <span>HTTPS Vhost</span>
              <strong>{{ metrics.vhostHTTPSPort || '未启用' }}</strong>
            </div>
          </div>
          <div class="listener-item">
            <IconSwap />
            <div>
              <span>KCP / QUIC</span>
              <strong>{{ metrics.kcpBindPort || 0 }} / {{ metrics.quicBindPort || 0 }}</strong>
            </div>
          </div>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-head">
        <div class="panel-title">
          <strong>快捷操作</strong>
          <span>常用运维入口</span>
        </div>
      </div>
      <div class="panel-body quick-grid">
        <router-link to="/clients" class="quick-card">
          <span class="quick-card-icon"><IconDesktop /></span>
          <span class="quick-card-copy">
            <strong>客户端管理</strong>
            <span>查看在线状态和连接信息</span>
          </span>
        </router-link>
        <router-link to="/proxies" class="quick-card">
          <span class="quick-card-icon"><IconLink /></span>
          <span class="quick-card-copy">
            <strong>代理管理</strong>
            <span>筛选代理并清理离线记录</span>
          </span>
        </router-link>
        <router-link to="/settings" class="quick-card">
          <span class="quick-card-icon"><IconSettings /></span>
          <span class="quick-card-copy">
            <strong>设置</strong>
            <span>管理会话与实例状态</span>
          </span>
        </router-link>
      </div>
    </section>

    <a-modal
      v-model:visible="restartVisible"
      title="重启服务"
      :mask-closable="false"
      :on-before-ok="restartService"
      ok-text="确认重启"
      cancel-text="取消"
    >
      <a-alert type="warning" :show-icon="true">
        服务端将完整重启，现有客户端连接会短暂中断。
      </a-alert>
      <a-alert
        v-if="restartError"
        type="error"
        :show-icon="true"
        class="restart-alert"
      >
        {{ restartError }}
      </a-alert>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import {
  IconApps,
  IconBranch,
  IconCloud,
  IconDashboard,
  IconDesktop,
  IconPublic,
  IconLink,
  IconPoweroff,
  IconRefresh,
  IconSafe,
  IconSettings,
  IconStorage,
  IconSwap,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  downloadBlob,
  type SupervisorStatus,
} from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import MetricCard from '@shared/components/MetricCard.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'

const router = useRouter()
const loading = ref(true)
const metrics = ref<Record<string, any>>({})
const runtime = ref<Record<string, any>>({})
const supervisor = ref<SupervisorStatus>()
const pprofEnabled = ref(false)
const savingSettings = ref(false)
const downloading = ref('')
const restartVisible = ref(false)
const restarting = ref(false)
const restartError = ref('')

const totalTraffic = computed(
  () => Number(metrics.value.totalTrafficIn || 0) + Number(metrics.value.totalTrafficOut || 0),
)
const proxyTotal = computed(() =>
  Object.values(metrics.value.proxyTypeCounts || {}).reduce(
    (sum: number, count) => sum + Number(count),
    0,
  ),
)
const proxyDistribution = computed(() => {
  const counts = (metrics.value.proxyTypeCounts || {}) as Record<string, number>
  const values = Object.entries(counts)
    .map(([type, count]) => ({ type, count: Number(count) }))
    .sort((a, b) => b.count - a.count)
  const maxValue = Math.max(1, ...values.map((item) => item.count))
  return values.map((item) => ({
    ...item,
    percent: Math.max(5, (item.count / maxValue) * 100),
  }))
})
const distributionVersion = computed(() =>
  String(metrics.value.distributionVersion || '开发构建'),
)
const upstreamVersion = computed(() =>
  String(metrics.value.upstreamVersion || metrics.value.version || '-'),
)
const runtimeItems = computed(() => [
  { label: '分发版本', value: distributionVersion.value, mono: true },
  { label: 'frp 版本', value: upstreamVersion.value, mono: true },
  {
    label: '进程运行时间',
    value: formatDuration(Number(runtime.value.processUptime || 0)),
    mono: false,
  },
  {
    label: 'Goroutine',
    value: Number(runtime.value.goroutines || 0).toLocaleString('zh-CN'),
    mono: true,
  },
  {
    label: '堆内存',
    value: formatBytes(Number(runtime.value.heapAlloc || 0)),
    mono: true,
  },
  {
    label: '系统内存',
    value: formatBytes(Number(runtime.value.sys || 0)),
    mono: true,
  },
  {
    label: 'CPU 数量',
    value: Number(runtime.value.numCPU || 0),
    mono: true,
  },
  {
    label: 'GC 次数',
    value: Number(runtime.value.gcCount || 0).toLocaleString('zh-CN'),
    mono: true,
  },
  { label: 'Go 版本', value: runtime.value.goVersion || '-', mono: true },
])

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [summary, runtimeData, diagnosticSettings, supervisorStatus] =
      await Promise.all([
        adminAPI.metrics(),
        adminAPI.runtime(),
        adminAPI.diagnosticSettings(),
        adminAPI.supervisor(),
      ])
    metrics.value = summary
    runtime.value = runtimeData
    pprofEnabled.value = diagnosticSettings.pprofEnabled
    supervisor.value = supervisorStatus
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载服务端指标失败')
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const settings = await adminAPI.saveDiagnosticSettings(pprofEnabled.value)
    pprofEnabled.value = settings.pprofEnabled
    Message.success(pprofEnabled.value ? '诊断已启用' : '诊断已关闭')
  } catch (error) {
    pprofEnabled.value = !pprofEnabled.value
    Message.error(error instanceof Error ? error.message : '保存诊断设置失败')
  } finally {
    savingSettings.value = false
  }
}

function openRestartDialog() {
  restartError.value = ''
  restartVisible.value = true
}

async function restartService() {
  restarting.value = true
  restartError.value = ''
  try {
    await adminAPI.restart()
    restartVisible.value = false
    Message.success('服务正在重启，请稍候刷新')
    void waitForService()
    return true
  } catch (error) {
    restartError.value = error instanceof Error ? error.message : '重启服务失败'
    Message.error(restartError.value)
    restarting.value = false
    return false
  }
}

async function waitForService() {
  try {
    for (let attempt = 0; attempt < 30; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 1_000))
      try {
        const response = await fetch('/healthz', { cache: 'no-store' })
        if (response.ok) {
          Message.success('服务已恢复')
          window.location.reload()
          return
        }
      } catch {
        // Keep polling while Supervisor restarts the process.
      }
    }
    Message.warning('服务尚未恢复，请检查 Supervisor 状态和宿主机日志')
  } finally {
    restarting.value = false
  }
}

async function download(type: 'goroutine' | 'heap' | 'cpu') {
  downloading.value = type
  try {
    const blob =
      type === 'goroutine'
        ? await adminAPI.downloadGoroutine()
        : type === 'heap'
          ? await adminAPI.downloadHeap()
          : await adminAPI.downloadCPU(15)
    downloadBlob(
      blob,
      `${type}-${Date.now()}.${type === 'goroutine' ? 'txt' : 'pprof'}`,
    )
    Message.success('诊断文件已生成')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '下载诊断文件失败')
  } finally {
    downloading.value = ''
  }
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  )
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatDuration(seconds: number) {
  if (!seconds) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days) return `${days} 天 ${hours} 小时`
  if (hours) return `${hours} 小时 ${minutes} 分钟`
  return `${Math.max(1, minutes)} 分钟`
}
</script>

<style scoped>
.diagnostic-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(360px, 0.85fr);
  gap: 16px;
}

.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(380px, 0.9fr);
  gap: 16px;
}

.listener-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.runtime-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.listener-item {
  display: flex;
  min-height: 82px;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--frp-border);
  border-radius: 12px;
  padding: 14px;
  background: var(--frp-surface-muted);
}

.listener-item > svg {
  color: var(--frp-role-strong);
  font-size: 21px;
}

.listener-item div {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.listener-item span {
  color: var(--frp-muted);
  font-size: 11px;
}

.listener-item strong {
  color: var(--frp-text);
  font-size: 13px;
}

.diagnostic-switch {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--frp-muted-strong);
  font-size: 12px;
}

.diagnostic-switch span {
  white-space: nowrap;
}

.diagnostic-actions {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.diagnostic-buttons {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.diagnostic-buttons :deep(.arco-btn) {
  height: 46px;
}

.restart-alert {
  margin-top: 12px;
}

@media (max-width: 1200px) {
  .diagnostic-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 1100px) {
  .overview-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .runtime-grid,
  .listener-grid,
  .diagnostic-buttons {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .diagnostic-panel .panel-head {
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .diagnostic-switch {
    justify-content: space-between;
  }
}
</style>
