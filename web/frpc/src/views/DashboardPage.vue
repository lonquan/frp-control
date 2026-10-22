<template>
  <div class="page-stack">
    <PageHeader
      title="客户端总览"
      description="查看隧道连接、代理运行状态与当前会话。"
      eyebrow="CLIENT OVERVIEW"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-button type="primary" @click="router.push('/proxies?create=1')">
        <template #icon><IconPlus /></template>
        新建代理
      </a-button>
    </PageHeader>

    <section class="metric-grid" aria-label="客户端关键指标">
      <MetricCard
        label="隧道连接"
        :value="metrics.connected ? '在线' : '离线'"
        :hint="`${metrics.serverAddr || '-'}:${metrics.serverPort || '-'}`"
        tone="blue"
        :icon="IconLink"
      />
      <MetricCard
        label="代理总数"
        :value="metrics.proxies || 0"
        :hint="`运行中 ${metrics.runningProxies || 0}`"
        tone="green"
        :icon="IconApps"
        to="/proxies"
        action="查看代理"
      />
      <MetricCard
        label="Goroutine"
        :value="runtime.goroutines || 0"
        :hint="runtime.goVersion || 'Go Runtime'"
        tone="violet"
        :icon="IconBranch"
        to="/diagnostics"
        action="查看诊断"
      />
      <MetricCard
        label="堆内存"
        :value="formatBytes(runtime.heapAlloc || 0)"
        :hint="`系统占用 ${formatBytes(runtime.sys || 0)}`"
        tone="amber"
        :icon="IconStorage"
      />
    </section>

    <div class="overview-grid">
      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>代理运行状态</strong>
            <span>当前运行中的代理类型分布</span>
          </div>
          <StatusBadge :status="metrics.connected ? 'online' : 'offline'" />
        </div>
        <div class="panel-body distribution-list">
          <div
            v-for="item in proxyDistribution"
            :key="item.type"
            class="distribution-row"
          >
            <span class="distribution-label">{{ item.type.toUpperCase() }}</span>
            <div class="distribution-track">
              <div class="distribution-fill" :style="{ width: `${item.percent}%` }" />
            </div>
            <span class="distribution-value">{{ item.count }}</span>
          </div>
          <a-empty v-if="proxyDistribution.length === 0" description="暂无可运行代理" />
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div class="panel-title">
            <strong>隧道信息</strong>
            <span>当前客户端连接参数</span>
          </div>
          <a-tag color="green">{{ metrics.protocol || 'tcp' }}</a-tag>
        </div>
        <div class="panel-body">
          <div class="detail-grid">
            <div class="detail-item">
              <span>服务器地址</span>
              <strong class="mono">{{ metrics.serverAddr || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>服务器端口</span>
              <strong class="mono">{{ metrics.serverPort || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>传输协议</span>
              <strong>{{ metrics.protocol || 'tcp' }}</strong>
            </div>
            <div class="detail-item">
              <span>分发版本</span>
              <strong class="mono">{{ distributionVersion }}</strong>
            </div>
            <div class="detail-item">
              <span>frp 版本</span>
              <strong class="mono">{{ upstreamVersion }}</strong>
            </div>
          </div>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-head">
        <div class="panel-title">
          <strong>快捷操作</strong>
          <span>常用客户端能力</span>
        </div>
      </div>
      <div class="panel-body quick-grid">
        <router-link to="/config" class="quick-card">
          <span class="quick-card-icon"><IconApps /></span>
          <span class="quick-card-copy">
            <strong>配置中心</strong>
            <span>编辑 TOML 和 include 文件</span>
          </span>
        </router-link>
        <router-link to="/certificates" class="quick-card">
          <span class="quick-card-icon"><IconSafe /></span>
          <span class="quick-card-copy">
            <strong>证书托管</strong>
            <span>ACME 签发、续期和绑定</span>
          </span>
        </router-link>
        <router-link to="/captures" class="quick-card">
          <span class="quick-card-icon"><IconFile /></span>
          <span class="quick-card-copy">
            <strong>流量捕获</strong>
            <span>查看 HTTP/WSS 请求与重放</span>
          </span>
        </router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import {
  IconApps,
  IconBranch,
  IconSafe,
  IconFile,
  IconLink,
  IconPlus,
  IconRefresh,
  IconStorage,
} from '@arco-design/web-vue/es/icon'
import { adminAPI } from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import MetricCard from '@shared/components/MetricCard.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'

const router = useRouter()
const loading = ref(true)
const metrics = ref<Record<string, any>>({})
const runtime = ref<Record<string, any>>({})
const distributionVersion = computed(() =>
  String(metrics.value.distributionVersion || '开发构建'),
)
const upstreamVersion = computed(() =>
  String(metrics.value.upstreamVersion || metrics.value.version || '-'),
)
const proxyDistribution = computed(() => {
  const counts = (metrics.value.proxyTypeCounts || {}) as Record<string, number>
  const values = Object.entries(counts)
    .map(([type, count]) => ({ type, count: Number(count) }))
    .sort((a, b) => b.count - a.count)
  const max = Math.max(1, ...values.map((item) => item.count))
  return values.map((item) => ({
    ...item,
    percent: Math.max(5, (item.count / max) * 100),
  }))
})

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [summary, runtimeData] = await Promise.all([
      adminAPI.metrics(),
      adminAPI.runtime(),
    ])
    metrics.value = summary
    runtime.value = runtimeData
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载客户端指标失败')
  } finally {
    loading.value = false
  }
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  )
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
</script>

<style scoped>
.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(380px, 0.85fr);
  gap: 16px;
}

@media (max-width: 1100px) {
  .overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
