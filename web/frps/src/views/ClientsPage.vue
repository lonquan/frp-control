<template>
  <div class="page-stack">
    <PageHeader
      title="客户端"
      description="查看已连接 FRPC 的状态、版本和最近连接信息。"
      eyebrow="CLIENTS"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
    </PageHeader>

    <a-form :model="filters" class="filter-panel" layout="vertical" @submit="search">
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input
            v-model="filters.q"
            placeholder="用户名、Client ID、主机名"
            allow-clear
          >
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model="filters.status" allow-clear placeholder="全部状态">
            <a-option value="online">在线</a-option>
            <a-option value="offline">离线</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="用户">
          <a-input v-model="filters.user" placeholder="全部用户" allow-clear />
        </a-form-item>
        <div class="filter-actions">
          <a-button type="primary" html-type="submit">查询</a-button>
          <a-button @click="reset">重置</a-button>
        </div>
      </div>
    </a-form>

    <section class="table-panel">
      <div class="table-toolbar">
        <div class="table-toolbar-copy">
          <strong>客户端列表</strong>
          <span>共 {{ total }} 条记录</span>
        </div>
      </div>
      <a-table
        :data="clients"
        :loading="loading"
        :pagination="false"
        row-key="key"
        size="large"
        :scroll="{ x: compact ? 520 : 920 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="客户端" data-index="clientID" :width="200">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.clientID || record.key }}</strong>
                <span>{{ record.hostname || '未知主机' }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column v-if="!compact" title="用户" data-index="user" :width="100" />
          <a-table-column title="状态" :width="100">
            <template #cell="{ record }">
              <StatusBadge :status="record.status" />
            </template>
          </a-table-column>
          <a-table-column title="IP 地址" data-index="ip" :width="130">
            <template #cell="{ record }">
              <span class="mono">{{ record.ip || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column v-if="!compact" title="版本" data-index="version" :width="100">
            <template #cell="{ record }">
              <span class="mono">{{ record.version || '-' }}</span>
            </template>
          </a-table-column>
          <a-table-column v-if="!compact" title="Wire" data-index="wireProtocol" :width="80" />
          <a-table-column v-if="!compact" title="最近连接" :width="130">
            <template #cell="{ record }">
              <a-tooltip :content="formatFullDate(record.lastConnectedAt)">
                <span>{{ formatDate(record.lastConnectedAt) }}</span>
              </a-tooltip>
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
            <div class="empty-icon"><IconDesktop /></div>
            <strong>暂无客户端</strong>
            <p>FRPC 登录后会自动出现在这里。</p>
          </div>
        </template>
      </a-table>
      <div class="table-footer">
        <a-pagination
          :current="page"
          :page-size="pageSize"
          :total="total"
          show-total
          show-page-size
          :page-size-options="[20, 50, 100]"
          @change="changePage"
          @page-size-change="changePageSize"
        />
      </div>
    </section>

    <a-drawer
      v-model:visible="drawerVisible"
      title="客户端详情"
      :width="720"
      unmount-on-close
    >
      <div v-if="selected" class="detail-sections">
        <div class="detail-section">
          <h3>基本信息</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>Client ID</span>
              <strong class="mono">{{ selected.clientID || selected.key }}</strong>
            </div>
            <div class="detail-item">
              <span>用户名</span>
              <strong>{{ selected.user || '(空)' }}</strong>
            </div>
            <div class="detail-item">
              <span>主机名</span>
              <strong>{{ selected.hostname || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>IP 地址</span>
              <strong class="mono">{{ selected.ip || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>版本</span>
              <strong class="mono">{{ selected.version || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>状态</span>
              <StatusBadge :status="selected.status" />
            </div>
            <div class="detail-item">
              <span>Run ID</span>
              <strong class="mono">{{ selected.runID || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>首次连接</span>
              <strong>{{ formatFullDate(selected.firstConnectedAt) }}</strong>
            </div>
          </div>
        </div>
      </div>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { Message } from '@arco-design/web-vue'
import {
  IconDesktop,
  IconRefresh,
  IconSearch,
} from '@arco-design/web-vue/es/icon'
import { adminRequest } from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'

interface ClientRecord {
  key: string
  clientID: string
  user: string
  hostname: string
  ip: string
  version: string
  wireProtocol: string
  status: string
  runID: string
  firstConnectedAt: string | number
  lastConnectedAt: string | number
}

interface PageResult<T> {
  total: number
  page: number
  pageSize: number
  items: T[]
}

const loading = ref(false)
const compact = useMediaQuery('(max-width: 900px)')
const clients = ref<ClientRecord[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const drawerVisible = ref(false)
const selected = ref<ClientRecord>()
const filters = reactive({ q: '', status: '', user: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
    })
    if (filters.q.trim()) query.set('q', filters.q.trim())
    if (filters.status) query.set('status', filters.status)
    if (filters.user.trim()) query.set('user', filters.user.trim())
    const result = await adminRequest<PageResult<ClientRecord>>(
      `/api/v2/clients?${query}`,
    )
    clients.value = result.items.map(normalizeClient)
    total.value = result.total
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载客户端失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function reset() {
  filters.q = ''
  filters.status = ''
  filters.user = ''
  page.value = 1
  load()
}

function changePage(value: number) {
  page.value = value
  load()
}

function changePageSize(value: number) {
  pageSize.value = value
  page.value = 1
  load()
}

function openDetail(record: any) {
  selected.value = record as ClientRecord
  drawerVisible.value = true
}

function normalizeClient(item: any): ClientRecord {
  return {
    key: item.key,
    clientID: item.clientID || '',
    user: item.user || '',
    hostname: item.hostname || '',
    ip: item.clientIP || item.ip || '',
    version: item.version || '',
    wireProtocol: item.wireProtocol || '',
    status: item.online ? 'online' : 'offline',
    runID: item.runID || '',
    firstConnectedAt: item.firstConnectedAt,
    lastConnectedAt: item.lastConnectedAt,
  }
}

function formatDate(value: string | number) {
  if (!value) return '-'
  const date =
    typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function formatFullDate(value: string | number) {
  if (!value) return '-'
  const date =
    typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${formatDate(value)} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
</script>

<style scoped>
.table-footer {
  display: flex;
  justify-content: flex-end;
  padding: 14px 18px;
  border-top: 1px solid var(--frp-border);
}
</style>
