<template>
  <div class="page-stack">
    <PageHeader
      title="代理"
      description="查看运行状态或管理 Store 中的代理配置。"
      eyebrow="PROXIES"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-tooltip :disabled="storeEnabled" content="请先在 FRPC 配置中启用 Store">
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
      :show-icon="true"
      title="Store 当前未启用"
    >
      可视化创建和编辑依赖 Store。请在主配置中加入
      <code>[store] path = "./frpc_store.json"</code>，或通过配置中心开启。
    </a-alert>

    <a-form :model="filters" class="filter-panel" layout="vertical" @submit="applyFilters">
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input v-model="filters.q" placeholder="代理名称、本地或远程地址" allow-clear>
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="类型">
          <a-select v-model="filters.type" allow-clear placeholder="全部类型">
            <a-option v-for="type in PROXY_TYPES" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model="filters.status" allow-clear placeholder="全部状态">
            <a-option value="running">运行中</a-option>
            <a-option value="start error">启动失败</a-option>
            <a-option value="closed">已关闭</a-option>
            <a-option value="wait start">等待启动</a-option>
          </a-select>
        </a-form-item>
        <div class="filter-actions">
          <a-button type="primary" html-type="submit">查询</a-button>
          <a-button @click="resetFilters">重置</a-button>
        </div>
      </div>
    </a-form>

    <section class="table-panel">
      <div class="table-toolbar">
        <div class="table-toolbar-copy">
          <strong>代理列表</strong>
          <span>共 {{ filteredStatus.length }} 个运行代理，{{ filteredStore.length }} 个 Store 代理</span>
        </div>
        <a-radio-group v-model="activeView" type="button">
          <a-radio value="status">运行状态</a-radio>
          <a-radio value="store">Store 配置</a-radio>
        </a-radio-group>
      </div>

      <a-table
        v-if="activeView === 'status'"
        :data="filteredStatus"
        :loading="loading"
        :pagination="false"
        row-key="name"
        size="large"
        :scroll="{ x: 1280 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="代理" :width="220">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ record.source === 'store' ? 'Store' : '配置文件' }}</span>
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
          <a-table-column title="本地地址" :width="190">
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
          <a-table-column title="证书" :width="180">
            <template #cell="{ record }">
              <span>{{ certificateLabel(record.certificate_id) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="错误" :width="260">
            <template #cell="{ record }">
              <a-tooltip v-if="record.err" :content="record.err">
                <span class="error-text">{{ record.err }}</span>
              </a-tooltip>
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="90" fixed="right">
            <template #cell="{ record }">
              <a-button type="text" @click.stop="openDetail(record)">详情</a-button>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconLink /></div>
            <strong>暂无运行代理</strong>
            <p>代理会在配置生效或 Store 条目启用后显示。</p>
          </div>
        </template>
      </a-table>

      <a-table
        v-else
        :data="filteredStore"
        :loading="loading"
        :pagination="false"
        row-key="name"
        size="large"
        :scroll="{ x: 1220 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="代理" :width="230">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ record.enabled ? '已启用' : '已禁用' }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="105">
            <template #cell="{ record }">
              <a-tag color="arcoblue">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="本地服务" :width="220">
            <template #cell="{ record }">
              <span class="mono">{{ localLabel(record) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="远程端口/域名" :width="220">
            <template #cell="{ record }">
              <span class="mono">{{ remoteLabel(record) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="插件" :width="150">
            <template #cell="{ record }">{{ pluginLabel(record) }}</template>
          </a-table-column>
          <a-table-column title="证书" :width="180">
            <template #cell="{ record }">
              <span>{{ certificateLabel(record.certificateId) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="270" fixed="right">
            <template #cell="{ record }">
              <div class="table-actions">
                <a-switch
                  :model-value="record.enabled"
                  size="small"
                  @change="(value: string | number | boolean) => toggle(record, Boolean(value))"
                  @click.stop
                />
                <a-button type="text" @click.stop="openEdit(record)">编辑</a-button>
                <a-popconfirm
                  content="确认删除此 Store 代理？"
                  @ok="remove(record)"
                >
                  <a-button type="text" status="danger" @click.stop>删除</a-button>
                </a-popconfirm>
              </div>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconLink /></div>
            <strong>暂无 Store 代理</strong>
            <p>点击“新建代理”创建 Store 条目。</p>
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
      <div v-if="selectedStatus" class="detail-sections">
        <div class="detail-section">
          <h3>运行状态</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>代理名称</span>
              <strong>{{ selectedStatus.name }}</strong>
            </div>
            <div class="detail-item">
              <span>类型</span>
              <strong>{{ selectedStatus.type.toUpperCase() }}</strong>
            </div>
            <div class="detail-item">
              <span>状态</span>
              <StatusBadge :status="selectedStatus.status" />
            </div>
            <div class="detail-item">
              <span>来源</span>
              <strong>{{ selectedStatus.source === 'store' ? 'Store' : '配置文件' }}</strong>
            </div>
            <div class="detail-item">
              <span>本地地址</span>
              <strong class="mono">{{ selectedStatus.local_addr || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>远程地址</span>
              <strong class="mono">{{ selectedStatus.remote_addr || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>插件</span>
              <strong>{{ selectedStatus.plugin || '无' }}</strong>
            </div>
            <div class="detail-item">
              <span>错误</span>
              <strong>{{ selectedStatus.err || '无' }}</strong>
            </div>
            <div
              v-if="certificateRecord(selectedStatus.certificate_id)"
              class="detail-item span-2"
            >
              <span>托管证书</span>
              <strong>
                {{ certificateRecord(selectedStatus.certificate_id)?.name }}
                ·
                {{ certificateRecord(selectedStatus.certificate_id)?.status }}
                ·
                {{ certificateRecord(selectedStatus.certificate_id)?.notAfter || '未签发' }}
              </strong>
            </div>
          </div>
        </div>
      </div>
    </a-drawer>

    <ProxyEditor
      :visible="editorVisible"
      :initial="editingProxy"
      @close="editorVisible = false"
      @saved="load"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
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
  type ProxyDefinition,
  type ProxyStatus,
  type StatusResponse,
} from '../types'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const statuses = ref<ProxyStatus[]>([])
const storeProxies = ref<ProxyDefinition[]>([])
const certificates = ref<CertificateRecord[]>([])
const storeEnabled = ref(true)
const activeView = ref<'status' | 'store'>('status')
const filters = reactive({ q: '', type: '', status: '' })
const detailVisible = ref(false)
const selectedStatus = ref<ProxyStatus>()
const editorVisible = ref(false)
const editingProxy = ref<ProxyDefinition | null>(null)

const filteredStatus = computed(() => filterStatus(statuses.value))
const filteredStore = computed(() => filterStore(storeProxies.value))

onMounted(async () => {
  await load()
  if (route.query.create === '1') {
    openCreate()
    await router.replace({ query: {} })
  }
})

watch(activeView, () => {
  filters.q = ''
  filters.type = ''
  filters.status = ''
})

async function load() {
  loading.value = true
  try {
    const status = await legacyRequest<StatusResponse>('/api/status')
    statuses.value = Object.values(status)
      .flat()
      .sort((a, b) => a.name.localeCompare(b.name))
    try {
      const store = await legacyRequest<{ proxies: ProxyDefinition[] }>(
        '/api/store/proxies',
      )
      storeProxies.value = store.proxies
      storeEnabled.value = true
    } catch {
      storeProxies.value = []
      storeEnabled.value = false
    }
    try {
      certificates.value = await adminAPI.certificates()
    } catch {
      certificates.value = []
    }
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载代理失败')
  } finally {
    loading.value = false
  }
}

function filterStatus(items: ProxyStatus[]) {
  const q = filters.q.toLowerCase().trim()
  return items.filter((item) => {
    if (filters.type && item.type !== filters.type) return false
    if (filters.status && item.status !== filters.status) return false
    if (!q) return true
    return [item.name, item.local_addr, item.remote_addr, item.plugin]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(q))
  })
}

function filterStore(items: ProxyDefinition[]) {
  const q = filters.q.toLowerCase().trim()
  return items.filter((item) => {
    if (filters.type && item.type !== filters.type) return false
    if (!q) return true
    return item.name.toLowerCase().includes(q)
  })
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

function certificateRecord(id?: string) {
  if (!id) return undefined
  return certificates.value.find((item) => item.id === id)
}

function applyFilters() {
  // Computed filters update immediately; this method provides explicit submit
  // semantics and keyboard behavior.
}

function resetFilters() {
  filters.q = ''
  filters.type = ''
  filters.status = ''
}

function openCreate() {
  if (!storeEnabled.value) return
  editingProxy.value = null
  editorVisible.value = true
}

function openEdit(record: ProxyDefinition) {
  editingProxy.value = record
  editorVisible.value = true
}

function openDetail(record: any) {
  selectedStatus.value = record as ProxyStatus
  detailVisible.value = true
}

async function toggle(record: ProxyDefinition, enabled: boolean) {
  const block = { ...getBlock(record), enabled }
  const payload = withBlock({ name: record.name, type: record.type }, record.type, block)
  try {
    await legacyRequest(`/api/store/proxies/${encodeURIComponent(record.name)}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
    Message.success(enabled ? '代理已启用' : '代理已禁用')
    await load()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '更新代理失败')
  }
}

function remove(record: ProxyDefinition) {
  Modal.confirm({
    title: '删除代理',
    content: `确认删除 ${record.name}？`,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      await legacyRequest(`/api/store/proxies/${encodeURIComponent(record.name)}`, {
        method: 'DELETE',
      })
      Message.success('代理已删除')
      await load()
    },
  })
}

function getBlock(record: ProxyDefinition) {
  return ((record as any)[record.type] || {}) as Record<string, any>
}

function withBlock(
  payload: ProxyDefinition,
  type: string,
  block: Record<string, any>,
) {
  ;(payload as any)[type] = block
  return payload
}

function localLabel(record: ProxyDefinition) {
  const block = getBlock(record)
  if (block.plugin?.localAddr) return block.plugin.localAddr
  return `${block.localIP || '127.0.0.1'}:${block.localPort || '-'}`
}

function remoteLabel(record: ProxyDefinition) {
  const block = getBlock(record)
  return (
    block.remotePort ||
    block.customDomains?.join(',') ||
    block.subdomain ||
    '-'
  )
}

function pluginLabel(record: ProxyDefinition) {
  return getBlock(record).plugin?.type || '无'
}
</script>

<style scoped>
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
</style>
