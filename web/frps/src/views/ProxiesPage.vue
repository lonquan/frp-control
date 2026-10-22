<template>
  <div class="page-stack">
    <PageHeader
      title="代理"
      description="查询所有代理的配置、状态和流量使用情况。"
      eyebrow="PROXIES"
    >
      <a-button @click="clearOffline">
        <template #icon><IconDelete /></template>
        清理离线记录
      </a-button>
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
    </PageHeader>

    <a-form :model="filters" class="filter-panel" layout="vertical" @submit="search">
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input v-model="filters.q" placeholder="代理名称、用户、Client ID" allow-clear>
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="代理类型">
          <a-select v-model="filters.type" allow-clear placeholder="全部类型">
            <a-option v-for="type in proxyTypes" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model="filters.status" allow-clear placeholder="全部状态">
            <a-option value="online">在线</a-option>
            <a-option value="offline">离线</a-option>
          </a-select>
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
          <strong>代理列表</strong>
          <span>共 {{ total }} 条记录</span>
        </div>
      </div>
      <a-table
        :data="proxies"
        :loading="loading"
        :pagination="false"
        row-key="name"
        size="large"
        :scroll="{ x: compact ? 500 : 930 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="代理" :width="210">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ record.clientID || record.user || '-' }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="90">
            <template #cell="{ record }">
              <a-tag color="arcoblue">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="状态" :width="110">
            <template #cell="{ record }">
              <StatusBadge :status="record.status" />
            </template>
          </a-table-column>
          <a-table-column v-if="!compact" title="今日入站" :width="110">
            <template #cell="{ record }">
              <span class="mono">{{ formatBytes(record.todayTrafficIn) }}</span>
            </template>
          </a-table-column>
          <a-table-column v-if="!compact" title="今日出站" :width="110">
            <template #cell="{ record }">
              <span class="mono">{{ formatBytes(record.todayTrafficOut) }}</span>
            </template>
          </a-table-column>
          <a-table-column v-if="!compact" title="连接数" data-index="curConns" :width="90" />
          <a-table-column v-if="!compact" title="最近启动" :width="130">
            <template #cell="{ record }">{{ record.lastStartTime || '-' }}</template>
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
            <strong>暂无代理</strong>
            <p>代理会在 FRPC 登录并启动后显示。</p>
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
      title="代理详情"
      :width="760"
      unmount-on-close
    >
      <div v-if="selected" class="detail-sections">
        <div class="detail-section">
          <h3>运行状态</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>代理名称</span>
              <strong>{{ selected.name }}</strong>
            </div>
            <div class="detail-item">
              <span>类型</span>
              <strong>{{ selected.type.toUpperCase() }}</strong>
            </div>
            <div class="detail-item">
              <span>状态</span>
              <StatusBadge :status="selected.status" />
            </div>
            <div class="detail-item">
              <span>连接数</span>
              <strong>{{ selected.curConns || 0 }}</strong>
            </div>
            <div class="detail-item">
              <span>今日入站</span>
              <strong>{{ formatBytes(selected.todayTrafficIn) }}</strong>
            </div>
            <div class="detail-item">
              <span>今日出站</span>
              <strong>{{ formatBytes(selected.todayTrafficOut) }}</strong>
            </div>
          </div>
        </div>
        <div v-if="selected.conf" class="detail-section">
          <h3>配置摘要</h3>
          <pre class="code-block">{{ JSON.stringify(selected.conf, null, 2) }}</pre>
        </div>
      </div>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconDelete,
  IconLink,
  IconRefresh,
  IconSearch,
} from '@arco-design/web-vue/es/icon'
import { adminRequest } from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'

interface ProxyRecord {
  name: string
  type: string
  status: string
  user: string
  clientID: string
  todayTrafficIn: number
  todayTrafficOut: number
  curConns: number
  lastStartTime: string
  conf?: Record<string, unknown>
}

interface PageResult<T> {
  total: number
  page: number
  pageSize: number
  items: T[]
}

const proxyTypes = ['tcp', 'udp', 'http', 'https', 'tcpmux', 'stcp', 'xtcp', 'sudp']
const loading = ref(false)
const compact = useMediaQuery('(max-width: 900px)')
const proxies = ref<ProxyRecord[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const drawerVisible = ref(false)
const selected = ref<ProxyRecord>()
const filters = reactive({ q: '', type: '', status: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
    })
    if (filters.q.trim()) query.set('q', filters.q.trim())
    if (filters.type) query.set('type', filters.type)
    if (filters.status) query.set('status', filters.status)
    const result = await adminRequest<PageResult<any>>(
      `/api/v2/proxies?${query}`,
    )
    proxies.value = result.items.map(normalizeProxy)
    total.value = result.total
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载代理失败')
  } finally {
    loading.value = false
  }
}

function normalizeProxy(item: any): ProxyRecord {
  const active =
    item.spec?.[item.spec.type] ||
    item.spec?.tcp ||
    item.spec?.udp ||
    item.spec?.http ||
    item.spec?.https ||
    item.spec?.tcpmux ||
    item.spec?.stcp ||
    item.spec?.xtcp ||
    item.spec?.sudp
  return {
    name: item.name,
    type: item.spec?.type || '-',
    status: item.status?.phase || '-',
    user: item.user || '',
    clientID: item.clientID || '',
    todayTrafficIn: item.status?.todayTrafficIn || 0,
    todayTrafficOut: item.status?.todayTrafficOut || 0,
    curConns: item.status?.curConns || 0,
    lastStartTime: formatDate(item.status?.lastStartAt),
    conf: active,
  }
}

function search() {
  page.value = 1
  load()
}

function reset() {
  filters.q = ''
  filters.type = ''
  filters.status = ''
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
  selected.value = record as ProxyRecord
  drawerVisible.value = true
}

function clearOffline() {
  Modal.confirm({
    title: '清理离线代理',
    content: '确认清理所有离线代理记录？该操作不会影响在线代理。',
    onOk: async () => {
      await adminRequest('/api/v2/system/prune?type=offline_proxies', {
        method: 'POST',
        body: JSON.stringify({}),
      })
      Message.success('离线代理记录已清理')
      load()
    },
  })
}

function formatBytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(value) / Math.log(1024)),
  )
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatDate(unix?: number) {
  if (!unix) return '-'
  const date = new Date(unix * 1000)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
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
