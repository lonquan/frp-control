<template>
  <div class="page-stack">
    <PageHeader
      title="监控与诊断"
      description="当前实例实时指标与受会话保护的性能诊断。指标不持久化历史。"
      eyebrow="OBSERVABILITY"
    >
      <a-button :loading="loading" @click="loadAll">
        <template #icon><IconRefresh /></template>
        刷新指标
      </a-button>
    </PageHeader>

    <section class="metric-grid" aria-label="实时指标">
      <MetricCard
        v-for="item in metricCards"
        :key="item.label"
        :label="item.label"
        :value="item.value"
        :hint="item.hint"
        :tone="item.tone"
        :icon="item.icon"
      />
    </section>

    <div class="diagnostic-grid">
      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>运行信息</strong>
            <span>Go 运行时和实例元数据</span>
          </div>
          <a-tag color="arcoblue">实时</a-tag>
        </div>
        <div class="panel-body">
          <div class="detail-grid">
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

      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>性能诊断</strong>
            <span>默认关闭，产物仅通过当前会话下载</span>
          </div>
          <a-switch
            v-model="pprofEnabled"
            :loading="savingSettings"
            @change="saveSettings"
          />
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

    <section v-if="role === 'server'" class="panel">
      <div class="panel-head">
        <div class="panel-title">
          <strong>代理类型分布</strong>
          <span>当前在线代理数量</span>
        </div>
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
        <a-empty v-if="proxyDistribution.length === 0" description="暂无在线代理" />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  IconBranch,
  IconCloud,
  IconDashboard,
  IconDesktop,
  IconLink,
  IconRefresh,
  IconStorage,
  IconSwap,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  downloadBlob,
  type SupervisorStatus,
} from '../admin'
import PageHeader from './PageHeader.vue'
import MetricCard from './MetricCard.vue'

const props = defineProps<{
  role: 'server' | 'client'
}>()

const loading = ref(true)
const metrics = ref<Record<string, any>>({})
const runtime = ref<Record<string, any>>({})
const supervisor = ref<SupervisorStatus>()
const pprofEnabled = ref(false)
const savingSettings = ref(false)
const downloading = ref('')

const metricCards = computed(() => {
  if (props.role === 'server') {
    return [
      {
        label: '在线客户端',
        value: number(metrics.value.onlineClients),
        hint: `共记录 ${number(metrics.value.knownClients)} 个`,
        tone: 'blue' as const,
        icon: IconDesktop,
      },
      {
        label: '在线代理',
        value: number(metrics.value.proxies),
        hint: '当前注册的代理数量',
        tone: 'green' as const,
        icon: IconLink,
      },
      {
        label: '当前连接',
        value: number(metrics.value.curConns),
        hint: '活跃业务连接',
        tone: 'violet' as const,
        icon: IconSwap,
      },
      {
        label: '今日流量',
        value: formatBytes(
          number(metrics.value.totalTrafficIn) +
            number(metrics.value.totalTrafficOut),
        ),
        hint: '入站与出站合计',
        tone: 'amber' as const,
        icon: IconCloud,
      },
    ]
  }
  return [
    {
      label: '连接状态',
      value: metrics.value.connected ? '在线' : '离线',
      hint: `${metrics.value.serverAddr || '-'}:${metrics.value.serverPort || '-'}`,
      tone: 'blue' as const,
      icon: IconLink,
    },
    {
      label: '配置代理',
      value: number(metrics.value.proxies),
      hint: `运行中 ${number(metrics.value.runningProxies)}`,
      tone: 'green' as const,
      icon: IconDashboard,
    },
    {
      label: 'Goroutine',
      value: number(runtime.value.goroutines),
      hint: `Go ${runtime.value.goVersion || '-'}`,
      tone: 'violet' as const,
      icon: IconBranch,
    },
    {
      label: '堆内存',
      value: formatBytes(number(runtime.value.heapAlloc)),
      hint: `系统占用 ${formatBytes(number(runtime.value.sys))}`,
      tone: 'amber' as const,
      icon: IconStorage,
    },
  ]
})

const runtimeItems = computed(() => [
  {
    label: '分发版本',
    value: metrics.value.distributionVersion || '开发构建',
    mono: true,
  },
  {
    label: 'frp 版本',
    value: metrics.value.upstreamVersion || metrics.value.version || '-',
    mono: true,
  },
  {
    label: '进程运行时间',
    value: formatDuration(number(runtime.value.processUptime)),
    mono: false,
  },
  {
    label: 'Goroutine',
    value: number(runtime.value.goroutines).toLocaleString('zh-CN'),
    mono: true,
  },
  {
    label: 'CPU 数量',
    value: number(runtime.value.numCPU),
    mono: true,
  },
  {
    label: 'GC 次数',
    value: number(runtime.value.gcCount).toLocaleString('zh-CN'),
    mono: true,
  },
  {
    label: 'Supervisor',
    value: supervisor.value?.state || 'unmanaged',
    mono: true,
  },
])

const proxyDistribution = computed(() => {
  const counts = (metrics.value.proxyTypeCounts || {}) as Record<string, number>
  const values = Object.entries(counts).map(([type, count]) => ({
    type,
    count: Number(count),
  }))
  const maxValue = Math.max(1, ...values.map((item) => item.count))
  return values
    .sort((a, b) => b.count - a.count)
    .map((item) => ({
      ...item,
      percent: Math.max(5, (item.count / maxValue) * 100),
    }))
})

onMounted(loadAll)

async function loadAll() {
  loading.value = true
  try {
    const [metricData, runtimeData, diagnosticSettings, supervisorData] =
      await Promise.all([
        adminAPI.metrics(),
        adminAPI.runtime(),
        adminAPI.diagnosticSettings(),
        adminAPI.supervisor(),
      ])
    metrics.value = metricData
    runtime.value = runtimeData
    pprofEnabled.value = diagnosticSettings.pprofEnabled
    supervisor.value = supervisorData
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取指标失败')
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

async function download(type: 'goroutine' | 'heap' | 'cpu') {
  downloading.value = type
  try {
    const blob =
      type === 'goroutine'
        ? await adminAPI.downloadGoroutine()
        : type === 'heap'
          ? await adminAPI.downloadHeap()
          : await adminAPI.downloadCPU(15)
    downloadBlob(blob, `${type}-${Date.now()}.${type === 'goroutine' ? 'txt' : 'pprof'}`)
    Message.success('诊断文件已生成')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '下载诊断文件失败')
  } finally {
    downloading.value = ''
  }
}

function number(value: unknown) {
  return Number(value || 0)
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
  grid-template-columns: minmax(0, 1.2fr) minmax(360px, 0.8fr);
  gap: 16px;
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

@media (max-width: 1120px) {
  .diagnostic-grid {
    grid-template-columns: 1fr;
  }
}
</style>
