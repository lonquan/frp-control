<template>
  <div class="page-stack">
    <PageHeader
      title="访问者"
      description="查看访问者监听状态，并管理 STCP、XTCP 和 SUDP 配置。"
      eyebrow="VISITORS"
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
          新建访问者
        </a-button>
      </a-tooltip>
    </PageHeader>

    <a-alert
      v-if="!storeEnabled"
      type="warning"
      show-icon
      title="动态配置当前未启用"
    >
      文件配置中的访问者仍可查看。若要通过表单创建访问者，请在主配置中加入
      <code>[store] path = "./frpc_store.json"</code>。
    </a-alert>
    <a-alert v-if="statusError" type="error" show-icon title="访问者状态加载失败">
      {{ statusError }}
    </a-alert>
    <a-alert v-if="configError" type="error" show-icon title="访问者配置加载失败">
      {{ configError }}
    </a-alert>

    <section class="table-panel" aria-labelledby="visitor-status-title">
      <div class="table-toolbar directory-toolbar">
        <div class="table-toolbar-copy">
          <strong id="visitor-status-title">访问者状态</strong>
          <span>当前显示 {{ filteredStatus.length }} 个运行实例</span>
        </div>
        <div class="toolbar-filters">
          <a-input
            v-model="statusFilters.q"
            allow-clear
            placeholder="搜索名称、Server 代理或用户"
          >
            <template #prefix><IconSearch /></template>
          </a-input>
          <a-select
            v-model="statusFilters.type"
            allow-clear
            placeholder="全部类型"
          >
            <a-option v-for="type in VISITOR_TYPES" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </a-option>
          </a-select>
          <a-select
            v-model="statusFilters.status"
            allow-clear
            placeholder="全部状态"
          >
            <a-option value="starting">启动中</a-option>
            <a-option value="running">监听中/已就绪</a-option>
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
        :scroll="{ x: 1160 }"
        @row-click="openStatusDetail"
      >
        <template #columns>
          <a-table-column title="访问者" :width="210">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="105">
            <template #cell="{ record }">
              <a-tag color="green">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="状态" :width="130">
            <template #cell="{ record }">
              <StatusBadge
                :status="record.status"
                :label="visitorStatusLabel(record)"
              />
            </template>
          </a-table-column>
          <a-table-column title="Server 代理" :width="180">
            <template #cell="{ record }">{{ record.server_name || '-' }}</template>
          </a-table-column>
          <a-table-column title="Server 用户" :width="150">
            <template #cell="{ record }">
              {{ record.server_user || '当前用户' }}
            </template>
          </a-table-column>
          <a-table-column title="本地监听" :width="190">
            <template #cell="{ record }">
              <span class="mono">{{ listenerLabel(record.bind_addr, record.bind_port) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="插件" :width="120">
            <template #cell="{ record }">{{ record.plugin || '无' }}</template>
          </a-table-column>
          <a-table-column title="错误" :width="220">
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
            <div class="empty-icon"><IconUserGroup /></div>
            <strong>暂无访问者运行实例</strong>
            <p>启用的访问者会在本地 listener 初始化后显示。</p>
          </div>
        </template>
      </a-table>
    </section>

    <section class="table-panel" aria-labelledby="visitor-config-title">
      <div class="table-toolbar directory-toolbar">
        <div class="table-toolbar-copy">
          <strong id="visitor-config-title">访问者配置</strong>
          <span>共 {{ filteredConfig.length }} 个配置，动态配置优先于同名文件配置</span>
        </div>
        <div class="toolbar-filters config-filters">
          <a-input
            v-model="configFilters.q"
            allow-clear
            placeholder="搜索名称、Server 代理或用户"
          >
            <template #prefix><IconSearch /></template>
          </a-input>
          <a-select
            v-model="configFilters.type"
            allow-clear
            placeholder="全部类型"
          >
            <a-option v-for="type in VISITOR_TYPES" :key="type" :value="type">
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
        :scroll="{ x: 1160 }"
        @row-click="openConfigDetail"
      >
        <template #columns>
          <a-table-column title="访问者" :width="210">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ sourceLabel(record.source) }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="100">
            <template #cell="{ record }">
              <a-tag color="green">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="配置状态" :width="125">
            <template #cell="{ record }">
              <a-tag :color="record.enabled ? 'green' : 'gray'">
                {{ record.enabled ? '已启用' : '已禁用' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="Server 代理" :width="170">
            <template #cell="{ record }">{{ record.server_name || '-' }}</template>
          </a-table-column>
          <a-table-column title="Server 用户" :width="140">
            <template #cell="{ record }">
              {{ record.server_user || '当前用户' }}
            </template>
          </a-table-column>
          <a-table-column title="本地监听" :width="180">
            <template #cell="{ record }">
              <span class="mono">{{ listenerLabel(record.bind_addr, record.bind_port) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="插件" :width="110">
            <template #cell="{ record }">{{ record.plugin || '无' }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="255" fixed="right">
            <template #cell="{ record }">
              <div class="table-actions" @click.stop>
                <a-switch
                  v-if="record.source === 'store'"
                  :model-value="record.enabled"
                  :loading="togglingNames.has(record.key)"
                  size="small"
                  :aria-label="`${record.enabled ? '禁用' : '启用'}访问者 ${record.name}`"
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
                  content="确认删除此动态配置访问者？"
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
            <div class="empty-icon"><IconUserGroup /></div>
            <strong>暂无访问者配置</strong>
            <p>配置文件访问者和动态配置访问者都会显示在这里。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:visible="detailVisible"
      title="访问者详情"
      :width="700"
      unmount-on-close
    >
      <div class="detail-sections">
        <div v-if="selectedConfig" class="detail-section">
          <h3>配置信息</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>名称</span>
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
              <span>Server 用户</span>
              <strong>{{ selectedConfig.server_user || '当前用户' }}</strong>
            </div>
            <div class="detail-item">
              <span>Server 代理</span>
              <strong>{{ selectedConfig.server_name || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>本地监听</span>
              <strong class="mono">
                {{ listenerLabel(selectedConfig.bind_addr, selectedConfig.bind_port) }}
              </strong>
            </div>
            <div class="detail-item">
              <span>插件</span>
              <strong>{{ selectedConfig.plugin || '无' }}</strong>
            </div>
            <div class="detail-item">
              <span>Secret Key</span>
              <strong>{{ selectedConfig.secret_configured ? '已配置' : '未配置' }}</strong>
            </div>
            <div class="detail-item">
              <span>传输</span>
              <strong>
                {{ selectedConfig.use_encryption ? '加密' : '未加密' }}
                ·
                {{ selectedConfig.use_compression ? '压缩' : '未压缩' }}
              </strong>
            </div>
            <div v-if="selectedConfig.type === 'xtcp'" class="detail-item">
              <span>XTCP 协议</span>
              <strong>{{ selectedConfig.xtcp_protocol || 'QUIC' }}</strong>
            </div>
            <div v-if="selectedConfig.type === 'xtcp'" class="detail-item">
              <span>保持隧道</span>
              <strong>{{ selectedConfig.keep_tunnel_open ? '是' : '否' }}</strong>
            </div>
            <div
              v-if="selectedConfig.type === 'xtcp' && selectedConfig.fallback_to"
              class="detail-item span-2"
            >
              <span>回退访问者</span>
              <strong>{{ selectedConfig.fallback_to }}</strong>
            </div>
          </div>
        </div>

        <div class="detail-section">
          <h3>运行状态</h3>
          <div v-if="selectedStatus" class="detail-grid">
            <div class="detail-item">
              <span>状态</span>
              <StatusBadge
                :status="selectedStatus.status"
                :label="visitorStatusLabel(selectedStatus)"
              />
            </div>
            <div class="detail-item">
              <span>类型</span>
              <strong>{{ selectedStatus.type.toUpperCase() }}</strong>
            </div>
            <div class="detail-item">
              <span>本地监听</span>
              <strong class="mono">
                {{ listenerLabel(selectedStatus.bind_addr, selectedStatus.bind_port) }}
              </strong>
            </div>
            <div class="detail-item">
              <span>Server 代理</span>
              <strong>{{ selectedStatus.server_name || '-' }}</strong>
            </div>
            <div class="detail-item span-2">
              <span>错误</span>
              <strong>{{ selectedStatus.err || '无' }}</strong>
            </div>
          </div>
          <a-alert v-else type="info" show-icon>
            当前没有对应的本地访问者实例，配置可能已禁用或尚未加载。
          </a-alert>
          <p class="status-note">
            运行状态仅表示本地初始化和监听结果，不代表远端 Server 代理一定可达。
          </p>
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

    <VisitorEditor
      :visible="editorVisible"
      :initial="editingVisitor"
      @close="editorVisible = false"
      @saved="loadAll(false)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconPlus,
  IconRefresh,
  IconSearch,
  IconUserGroup,
} from '@arco-design/web-vue/es/icon'
import { legacyRequest } from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'
import VisitorEditor from '../components/VisitorEditor.vue'
import {
  VISITOR_TYPES,
  isStoreVisitorEnabled,
  type VisitorConfigSummary,
  type VisitorDefinition,
  type VisitorStatus,
} from '../types'

interface VisitorConfigRow extends VisitorConfigSummary {
  key: string
  definition?: VisitorDefinition
}

const router = useRouter()
const loading = ref(false)
const statusLoading = ref(false)
const configLoading = ref(false)
const statusError = ref('')
const configError = ref('')
const statuses = ref<VisitorStatus[]>([])
const configSummaries = ref<VisitorConfigSummary[]>([])
const storeVisitors = ref<VisitorDefinition[]>([])
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
const selectedStatus = ref<VisitorStatus>()
const selectedConfig = ref<VisitorConfigRow>()
const editorVisible = ref(false)
const editingVisitor = ref<VisitorDefinition | null>(null)

let statusTimer: number | undefined

const storeVisitorMap = computed(
  () => new Map(storeVisitors.value.map((visitor) => [visitor.name, visitor])),
)

const configRows = computed<VisitorConfigRow[]>(() =>
  configSummaries.value.map((summary) => ({
    ...summary,
    key: `${summary.source}:${summary.name}`,
    definition:
      summary.source === 'store'
        ? storeVisitorMap.value.get(summary.name)
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
    return [
      status.name,
      status.server_name,
      status.server_user,
      status.bind_addr,
    ]
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
    if (configFilters.enabled === 'enabled' && !config.enabled) return false
    if (configFilters.enabled === 'disabled' && config.enabled) return false
    if (!q) return true
    return [config.name, config.server_name, config.server_user]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(q))
  })
})

onMounted(async () => {
  await loadAll(true)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  statusTimer = window.setInterval(refreshStatusWhenVisible, 10_000)
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
    const response = await legacyRequest<{ visitors: VisitorStatus[] }>(
      '/api/visitors/status',
    )
    statuses.value = response.visitors.sort((a, b) =>
      a.name.localeCompare(b.name),
    )
    statusError.value = ''
  } catch (error) {
    statusError.value = errorMessage(error, '加载访问者状态失败')
  } finally {
    if (showLoading) statusLoading.value = false
  }
}

async function loadConfigs(showLoading: boolean) {
  if (showLoading) configLoading.value = true
  try {
    const [summaryResult, storeResult] = await Promise.allSettled([
      legacyRequest<{ visitors: VisitorConfigSummary[] }>(
        '/api/config/visitors',
      ),
      legacyRequest<{ visitors: VisitorDefinition[] }>('/api/store/visitors'),
    ])

    if (summaryResult.status === 'fulfilled') {
      configSummaries.value = summaryResult.value.visitors
      configError.value = ''
    } else {
      configError.value = errorMessage(
        summaryResult.reason,
        '加载访问者配置失败',
      )
    }

    if (storeResult.status === 'fulfilled') {
      storeVisitors.value = storeResult.value.visitors
      storeEnabled.value = true
    } else {
      storeVisitors.value = []
      storeEnabled.value = false
    }
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

function visitorStatusLabel(status: VisitorStatus) {
  switch (status.status) {
    case 'starting':
      return '启动中'
    case 'running':
      return status.bind_port > 0 ? '监听中' : '已就绪'
    case 'start error':
      return '启动失败'
    case 'closed':
      return '已关闭'
    default:
      return status.status
  }
}

function listenerLabel(bindAddr = '127.0.0.1', bindPort = 0) {
  if (bindPort === -1) return '仅接收重定向'
  if (bindPort <= 0) return '未绑定端口'
  return `${bindAddr || '127.0.0.1'}:${bindPort}`
}

function openCreate() {
  if (!storeEnabled.value) return
  editingVisitor.value = null
  editorVisible.value = true
}

function openEdit(visitor: VisitorDefinition) {
  editingVisitor.value = visitor
  editorVisible.value = true
}

function openConfigCenter() {
  void router.push('/config')
}

function openStatusDetail(record: any) {
  const status = record as VisitorStatus
  selectedStatus.value = status
  selectedConfig.value = findEffectiveConfig(status.name)
  detailVisible.value = true
}

function openConfigDetail(record: any) {
  const config = record as VisitorConfigRow
  selectedConfig.value = config
  selectedStatus.value = statuses.value.find(
    (status) => status.name === config.name,
  )
  detailVisible.value = true
}

function findEffectiveConfig(name: string) {
  return (
    configRows.value.find(
      (config) =>
        config.name === name &&
        config.source === 'store' &&
        config.enabled,
    ) ||
    configRows.value.find(
      (config) => config.name === name && config.enabled,
    ) ||
    configRows.value.find((config) => config.name === name)
  )
}

async function toggle(record: VisitorConfigRow, enabled: boolean) {
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
    } as VisitorDefinition
    await legacyRequest(
      `/api/store/visitors/${encodeURIComponent(record.definition.name)}`,
      {
        method: 'PUT',
        body: JSON.stringify(payload),
      },
    )
    await Promise.all([loadConfigs(false), loadStatus(false)])
    if (configError.value) {
      throw new Error(`配置已保存，但重新加载失败：${configError.value}`)
    }
    const updated = storeVisitors.value.find(
      (visitor) => visitor.name === record.name,
    )
    if (!updated || isStoreVisitorEnabled(updated) !== enabled) {
      throw new Error('配置已保存，但重新加载后状态不一致')
    }
    Message.success(enabled ? '访问者已启用' : '访问者已禁用')
  } catch (error) {
    Message.error(errorMessage(error, '更新访问者失败'))
  } finally {
    togglingNames.value.delete(record.key)
  }
}

function remove(visitor: VisitorDefinition) {
  Modal.confirm({
    title: '删除访问者',
    content: `确认删除动态配置访问者 ${visitor.name}？`,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await legacyRequest(
          `/api/store/visitors/${encodeURIComponent(visitor.name)}`,
          { method: 'DELETE' },
        )
        await loadAll(false)
        Message.success('访问者已删除')
      } catch (error) {
        Message.error(errorMessage(error, '删除访问者失败'))
        throw error
      }
    },
  })
}

function getBlock(visitor: VisitorDefinition) {
  return ((visitor as any)[visitor.type] || {}) as Record<string, any>
}
</script>

<style scoped>
.directory-toolbar {
  align-items: flex-start;
}

.toolbar-filters {
  display: grid;
  width: min(100%, 680px);
  grid-template-columns: minmax(210px, 1.4fr) repeat(2, minmax(130px, 0.75fr));
  gap: 8px;
}

.config-filters {
  width: min(100%, 780px);
  grid-template-columns: minmax(210px, 1.4fr) repeat(3, minmax(120px, 0.75fr));
}

.error-text {
  display: block;
  overflow: hidden;
  color: rgb(var(--danger-6));
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-note {
  margin: 12px 0 0;
  color: var(--frp-muted);
  font-size: 12px;
  line-height: 1.6;
}

code {
  color: var(--frp-role-strong);
}

@media (max-width: 1100px) {
  .directory-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-filters,
  .config-filters {
    width: 100%;
  }
}

@media (max-width: 720px) {
  .toolbar-filters,
  .config-filters {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
