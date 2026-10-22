<template>
  <div class="page-stack">
    <PageHeader
      title="代理"
      description="查看当前代理状态，并在同一页面管理配置文件和动态配置。"
      eyebrow="PROXIES"
    >
      <a-button :loading="loading" @click="loadAll(true)">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-tooltip :disabled="storeEnabled" content="请先在 FRPC 配置中启用动态配置">
        <a-button
          type="primary"
          :disabled="!storeEnabled"
          @click="openCreate"
        >
          <template #icon><IconPlus /></template>
          新建代理
        </a-button>
      </a-tooltip>
    </PageHeader>

    <a-alert
      v-if="!storeEnabled"
      type="warning"
      show-icon
      title="动态配置当前未启用"
    >
      文件配置仍可查看。若要通过面板创建动态配置，请在主配置中加入
      <code>[store] path = "./frpc_store.json"</code>。
    </a-alert>
    <a-alert v-if="statusError" type="error" show-icon title="代理状态加载失败">
      {{ statusError }}
    </a-alert>
    <a-alert v-if="configError" type="error" show-icon title="代理配置加载失败">
      {{ configError }}
    </a-alert>

    <section class="table-panel" aria-labelledby="proxy-status-title">
      <div class="table-toolbar directory-toolbar">
        <div class="table-toolbar-copy">
          <strong id="proxy-status-title">代理状态</strong>
          <span>当前显示 {{ filteredStatus.length }} 个运行实例</span>
        </div>
        <div class="toolbar-filters">
          <a-input
            v-model="statusFilters.q"
            allow-clear
            placeholder="搜索名称或地址"
          >
            <template #prefix><IconSearch /></template>
          </a-input>
          <a-select
            v-model="statusFilters.type"
            allow-clear
            placeholder="全部类型"
          >
            <a-option v-for="type in PROXY_TYPES" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </a-option>
          </a-select>
          <a-select
            v-model="statusFilters.status"
            allow-clear
            placeholder="全部状态"
          >
            <a-option value="running">运行中</a-option>
            <a-option value="wait start">等待启动</a-option>
            <a-option value="start error">启动失败</a-option>
            <a-option value="closed">已关闭</a-option>
          </a-select>
        </div>
      </div>

      <a-table
        :data="filteredStatus"
        :loading="statusLoading"
        :pagination="false"
        row-key="name"
        size="large"
        :scroll="{ x: 1180 }"
        @row-click="openStatusDetail"
      >
        <template #columns>
          <a-table-column title="代理" :width="210">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ sourceLabel(record.source) }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="100">
            <template #cell="{ record }">
              <a-tag color="arcoblue">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="状态" :width="120">
            <template #cell="{ record }">
              <StatusBadge :status="record.status" />
            </template>
          </a-table-column>
          <a-table-column title="本地地址" :width="180">
            <template #cell="{ record }">
              <span class="mono">{{ record.local_addr || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column title="远程地址/域名" :width="210">
            <template #cell="{ record }">
              <span class="mono">{{ record.remote_addr || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column title="插件" data-index="plugin" :width="130" />
          <a-table-column title="错误" :width="250">
            <template #cell="{ record }">
              <a-tooltip v-if="record.err" :content="record.err">
                <span class="error-text">{{ record.err }}</span>
              </a-tooltip>
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="90" fixed="right">
            <template #cell="{ record }">
              <a-button type="text" @click.stop="openStatusDetail(record)">详情</a-button>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconLink /></div>
            <strong>暂无代理运行实例</strong>
            <p>启用的代理会在配置生效后显示；启动失败时会保留错误信息。</p>
          </div>
        </template>
      </a-table>
    </section>

    <section class="table-panel" aria-labelledby="proxy-config-title">
      <div class="table-toolbar directory-toolbar">
        <div class="table-toolbar-copy">
          <strong id="proxy-config-title">代理配置</strong>
          <span>共 {{ filteredConfig.length }} 个配置，动态配置优先于同名文件配置</span>
        </div>
        <div class="toolbar-filters">
          <a-input
            v-model="configFilters.q"
            allow-clear
            placeholder="搜索配置名称"
          >
            <template #prefix><IconSearch /></template>
          </a-input>
          <a-select
            v-model="configFilters.type"
            allow-clear
            placeholder="全部类型"
          >
            <a-option v-for="type in PROXY_TYPES" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </a-option>
          </a-select>
          <a-select
            v-model="configFilters.source"
            allow-clear
            placeholder="全部来源"
          >
            <a-option value="store">动态配置</a-option>
            <a-option value="config">配置文件</a-option>
          </a-select>
          <a-select
            v-model="configFilters.enabled"
            allow-clear
            placeholder="全部配置状态"
          >
            <a-option value="enabled">已启用</a-option>
            <a-option value="disabled">已禁用</a-option>
          </a-select>
        </div>
      </div>

      <a-table
        :data="filteredConfig"
        :loading="configLoading"
        :pagination="false"
        row-key="key"
        size="large"
        :scroll="{ x: 1180 }"
        @row-click="openConfigDetail"
      >
        <template #columns>
          <a-table-column title="代理" :width="220">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ sourceLabel(record.source) }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="100">
            <template #cell="{ record }">
              <a-tag color="arcoblue">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="配置状态" :width="125">
            <template #cell="{ record }">
              <a-tag :color="record.enabled ? 'green' : 'gray'">
                {{ record.enabled ? '已启用' : '已禁用' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="本地服务" :width="190">
            <template #cell="{ record }">
              <span class="mono">{{ record.local_addr || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column title="远程端口/域名" :width="210">
            <template #cell="{ record }">
              <span class="mono">{{ record.remote_addr || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column title="插件" :width="130">
            <template #cell="{ record }">{{ record.plugin || '无' }}</template>
          </a-table-column>
          <a-table-column title="证书" :width="170">
            <template #cell="{ record }">
              <span>{{ certificateLabel(record.certificate_id) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="260" fixed="right">
            <template #cell="{ record }">
              <div class="table-actions" @click.stop>
                <a-switch
                  v-if="record.source === 'store'"
                  :model-value="record.enabled"
                  :loading="togglingNames.has(record.key)"
                  size="small"
                  :aria-label="`${record.enabled ? '禁用' : '启用'}代理 ${record.name}`"
                  @change="(value: string | number | boolean) => toggle(record, Boolean(value))"
                />
                <a-button
                  v-if="record.definition"
                  type="text"
                  @click="openEdit(record.definition)"
                >
                  编辑
                </a-button>
                <a-button
                  v-if="record.source === 'config'"
                  type="text"
                  @click="openConfigCenter"
                >
                  打开配置中心
                </a-button>
                <a-popconfirm
                  v-if="record.definition"
                  content="确认删除此动态配置？"
                  @ok="remove(record.definition)"
                >
                  <a-button type="text" status="danger">删除</a-button>
                </a-popconfirm>
              </div>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconLink /></div>
            <strong>暂无代理配置</strong>
            <p>配置文件代理和动态配置代理都会显示在这里。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:visible="detailVisible"
      title="代理详情"
      :width="720"
      unmount-on-close
    >
      <div class="detail-sections">
        <div v-if="selectedConfig" class="detail-section">
          <h3>配置信息</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>代理名称</span>
              <strong>{{ selectedConfig.name }}</strong>
            </div>
            <div class="detail-item">
              <span>来源</span>
              <strong>{{ sourceLabel(selectedConfig.source) }}</strong>
            </div>
            <div class="detail-item">
              <span>类型</span>
              <strong>{{ selectedConfig.type.toUpperCase() }}</strong>
            </div>
            <div class="detail-item">
              <span>配置状态</span>
              <strong>{{ selectedConfig.enabled ? '已启用' : '已禁用' }}</strong>
            </div>
            <div class="detail-item">
              <span>本地服务</span>
              <strong class="mono">{{ selectedConfig.local_addr || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>远程端口/域名</span>
              <strong class="mono">{{ selectedConfig.remote_addr || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>插件</span>
              <strong>{{ selectedConfig.plugin || '无' }}</strong>
            </div>
            <div class="detail-item">
              <span>证书</span>
              <strong>{{ certificateLabel(selectedConfig.certificate_id) }}</strong>
            </div>
          </div>
        </div>

        <div class="detail-section">
          <h3>运行状态</h3>
          <div v-if="selectedStatus" class="detail-grid">
            <div class="detail-item">
              <span>状态</span>
              <StatusBadge :status="selectedStatus.status" />
            </div>
            <div class="detail-item">
              <span>运行类型</span>
              <strong>{{ selectedStatus.type.toUpperCase() }}</strong>
            </div>
            <div class="detail-item">
              <span>本地地址</span>
              <strong class="mono">{{ selectedStatus.local_addr || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>远程地址</span>
              <strong class="mono">{{ selectedStatus.remote_addr || '-' }}</strong>
            </div>
            <div class="detail-item span-2">
              <span>错误</span>
              <strong>{{ selectedStatus.err || '无' }}</strong>
            </div>
          </div>
          <a-alert v-else type="info" show-icon>
            当前没有对应的运行实例，配置可能已禁用、等待客户端连接或尚未加载。
          </a-alert>
        </div>
      </div>
      <template #footer>
        <a-space>
          <a-button @click="detailVisible = false">关闭</a-button>
          <a-button
            v-if="selectedConfig?.source === 'config'"
            type="primary"
            @click="openConfigCenter"
          >
            打开配置中心
          </a-button>
          <a-button
            v-else-if="selectedConfig?.definition"
            type="primary"
            @click="openEdit(selectedConfig.definition)"
          >
            编辑动态配置
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <ProxyEditor
      :visible="editorVisible"
      :initial="editingProxy"
      @close="editorVisible = false"
      @saved="loadAll(false)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconLink,
  IconPlus,
  IconRefresh,
  IconSearch,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  legacyRequest,
  type CertificateRecord,
} from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'
import ProxyEditor from '../components/ProxyEditor.vue'
import {
  PROXY_TYPES,
  isStoreProxyEnabled,
  type ProxyConfigSummary,
  type ProxyDefinition,
  type ProxyStatus,
  type StatusResponse,
} from '../types'

interface ProxyConfigRow extends ProxyConfigSummary {
  key: string
  definition?: ProxyDefinition
}

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const statusLoading = ref(false)
const configLoading = ref(false)
const statusError = ref('')
const configError = ref('')
const statuses = ref<ProxyStatus[]>([])
const configSummaries = ref<ProxyConfigSummary[]>([])
const storeProxies = ref<ProxyDefinition[]>([])
const certificates = ref<CertificateRecord[]>([])
const storeEnabled = ref(true)
const togglingNames = ref(new Set<string>())
const statusFilters = reactive({ q: '', type: '', status: '' })
const configFilters = reactive({
  q: '',
  type: '',
  source: '',
  enabled: '',
})
const detailVisible = ref(false)
const selectedStatus = ref<ProxyStatus>()
const selectedConfig = ref<ProxyConfigRow>()
const editorVisible = ref(false)
const editingProxy = ref<ProxyDefinition | null>(null)

let statusTimer: number | undefined

const storeProxyMap = computed(
  () => new Map(storeProxies.value.map((proxy) => [proxy.name, proxy])),
)

const configRows = computed<ProxyConfigRow[]>(() =>
  configSummaries.value.map((summary) => ({
    ...summary,
    key: `${summary.source}:${summary.name}`,
    definition:
      summary.source === 'store'
        ? storeProxyMap.value.get(summary.name)
        : undefined,
  })),
)

const filteredStatus = computed(() => {
  const q = statusFilters.q.toLowerCase().trim()
  return statuses.value.filter((status) => {
    if (statusFilters.type && status.type !== statusFilters.type) return false
    if (statusFilters.status && status.status !== statusFilters.status) {
      return false
    }
    if (!q) return true
    return [status.name, status.local_addr, status.remote_addr, status.plugin]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(q))
  })
})

const filteredConfig = computed(() => {
  const q = configFilters.q.toLowerCase().trim()
  return configRows.value.filter((config) => {
    if (configFilters.type && config.type !== configFilters.type) return false
    if (configFilters.source && config.source !== configFilters.source) {
      return false
    }
    if (
      configFilters.enabled === 'enabled' &&
      !config.enabled
    ) {
      return false
    }
    if (
      configFilters.enabled === 'disabled' &&
      config.enabled
    ) {
      return false
    }
    return !q || config.name.toLowerCase().includes(q)
  })
})

onMounted(async () => {
  await loadAll(true)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  statusTimer = window.setInterval(refreshStatusWhenVisible, 10_000)
  if (route.query.create === '1') {
    openCreate()
    await router.replace({ query: {} })
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (statusTimer !== undefined) {
    window.clearInterval(statusTimer)
  }
})

async function loadAll(showLoading: boolean) {
  if (showLoading) loading.value = true
  try {
    await Promise.all([loadStatus(showLoading), loadConfigs(showLoading)])
  } finally {
    if (showLoading) loading.value = false
  }
}

async function loadStatus(showLoading: boolean) {
  if (showLoading) statusLoading.value = true
  try {
    const response = await legacyRequest<StatusResponse>('/api/status')
    statuses.value = Object.values(response)
      .flat()
      .sort((a, b) => a.name.localeCompare(b.name))
    statusError.value = ''
  } catch (error) {
    statusError.value =
      error instanceof Error ? error.message : '加载代理状态失败'
  } finally {
    if (showLoading) statusLoading.value = false
  }
}

async function loadConfigs(showLoading: boolean) {
  if (showLoading) configLoading.value = true
  try {
    const [summaryResult, storeResult, certificateResult] =
      await Promise.allSettled([
        legacyRequest<{ proxies: ProxyConfigSummary[] }>('/api/config/proxies'),
        legacyRequest<{ proxies: ProxyDefinition[] }>('/api/store/proxies'),
        adminAPI.certificates(),
      ])

    if (summaryResult.status === 'fulfilled') {
      configSummaries.value = summaryResult.value.proxies
      configError.value = ''
    } else {
      configError.value = errorMessage(
        summaryResult.reason,
        '加载代理配置失败',
      )
    }

    if (storeResult.status === 'fulfilled') {
      storeProxies.value = storeResult.value.proxies
      storeEnabled.value = true
    } else {
      storeProxies.value = []
      storeEnabled.value = false
    }

    certificates.value =
      certificateResult.status === 'fulfilled' ? certificateResult.value : []
  } finally {
    if (showLoading) configLoading.value = false
  }
}

function refreshStatusWhenVisible() {
  if (document.visibilityState === 'visible') {
    void loadStatus(false)
  }
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') {
    void loadStatus(false)
  }
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

function sourceLabel(source?: string) {
  return source === 'store' ? '动态配置' : '配置文件'
}

function openCreate() {
  if (!storeEnabled.value) return
  editingProxy.value = null
  editorVisible.value = true
}

function openEdit(proxy: ProxyDefinition) {
  editingProxy.value = proxy
  editorVisible.value = true
}

function openConfigCenter() {
  void router.push('/config')
}

function openStatusDetail(record: any) {
  const status = record as ProxyStatus
  selectedStatus.value = status
  selectedConfig.value = findEffectiveConfig(status.name, status.source)
  detailVisible.value = true
}

function openConfigDetail(record: any) {
  const config = record as ProxyConfigRow
  selectedConfig.value = config
  selectedStatus.value = statuses.value.find(
    (status) => status.name === config.name,
  )
  detailVisible.value = true
}

function findEffectiveConfig(name: string, source?: string) {
  const preferredSource = source === 'store' ? 'store' : 'config'
  return (
    configRows.value.find(
      (config) =>
        config.name === name && config.source === preferredSource,
    ) ||
    configRows.value.find(
      (config) => config.name === name && config.source === 'store',
    ) ||
    configRows.value.find((config) => config.name === name)
  )
}

async function toggle(record: ProxyConfigRow, enabled: boolean) {
  if (!record.definition || togglingNames.value.has(record.key)) return
  togglingNames.value.add(record.key)
  try {
    const block = {
      ...getBlock(record.definition),
      enabled,
    }
    const payload = {
      name: record.definition.name,
      type: record.definition.type,
      [record.definition.type]: block,
    } as ProxyDefinition
    await legacyRequest(
      `/api/store/proxies/${encodeURIComponent(record.definition.name)}`,
      {
        method: 'PUT',
        body: JSON.stringify(payload),
      },
    )
    await Promise.all([loadConfigs(false), loadStatus(false)])
    if (configError.value) {
      throw new Error(`配置已保存，但重新加载失败：${configError.value}`)
    }
    const updated = storeProxies.value.find(
      (proxy) => proxy.name === record.name,
    )
    if (!updated || isStoreProxyEnabled(updated) !== enabled) {
      throw new Error('配置已保存，但重新加载后状态不一致')
    }
    Message.success(enabled ? '代理已启用' : '代理已禁用')
  } catch (error) {
    Message.error(errorMessage(error, '更新代理失败'))
  } finally {
    togglingNames.value.delete(record.key)
  }
}

function remove(proxy: ProxyDefinition) {
  Modal.confirm({
    title: '删除代理',
    content: `确认删除动态配置 ${proxy.name}？`,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await legacyRequest(
          `/api/store/proxies/${encodeURIComponent(proxy.name)}`,
          { method: 'DELETE' },
        )
        await loadAll(false)
        Message.success('代理已删除')
      } catch (error) {
        Message.error(errorMessage(error, '删除代理失败'))
        throw error
      }
    },
  })
}

function getBlock(proxy: ProxyDefinition) {
  return ((proxy as any)[proxy.type] || {}) as Record<string, any>
}

function certificateLabel(id?: string) {
  if (!id) return '手动/未托管'
  const certificate = certificates.value.find((item) => item.id === id)
  if (!certificate) return `${id.slice(0, 8)} · 缺失`
  if (certificate.status === 'error') {
    return `${certificate.name} · 续期异常`
  }
  if (
    certificate.notAfter &&
    new Date(certificate.notAfter).getTime() <= Date.now()
  ) {
    return `${certificate.name} · 已过期`
  }
  return certificate.name
}
</script>

<style scoped>
.directory-toolbar {
  align-items: flex-start;
}

.toolbar-filters {
  display: grid;
  width: min(100%, 680px);
  grid-template-columns: minmax(190px, 1.4fr) repeat(3, minmax(120px, 0.75fr));
  gap: 8px;
}

.error-text {
  display: block;
  overflow: hidden;
  color: rgb(var(--danger-6));
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

code {
  color: var(--frp-role-strong);
}

@media (max-width: 1100px) {
  .directory-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-filters {
    width: 100%;
  }
}

@media (max-width: 720px) {
  .toolbar-filters {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
