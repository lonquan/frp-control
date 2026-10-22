<template>
  <div class="page-stack">
    <PageHeader
      title="访问者"
      description="管理 STCP、XTCP 和 SUDP 访问者配置。"
      eyebrow="VISITORS"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-button type="primary" :disabled="!storeEnabled" @click="openCreate">
        <template #icon><IconPlus /></template>
        新建访问者
      </a-button>
    </PageHeader>

    <a-alert v-if="!storeEnabled" type="warning" :show-icon="true">
      Store 未启用，无法通过表单创建访问者。请先在配置中心启用 Store。
    </a-alert>

    <a-form :model="filters" class="filter-panel" layout="vertical" @submit="applyFilters">
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input v-model="filters.q" placeholder="访问者名称" allow-clear>
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="类型">
          <a-select v-model="filters.type" allow-clear placeholder="全部类型">
            <a-option v-for="type in VISITOR_TYPES" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </a-option>
          </a-select>
        </a-form-item>
        <div></div>
        <div class="filter-actions">
          <a-button type="primary" html-type="submit">查询</a-button>
          <a-button @click="resetFilters">重置</a-button>
        </div>
      </div>
    </a-form>

    <section class="table-panel">
      <div class="table-toolbar">
        <div class="table-toolbar-copy">
          <strong>访问者列表</strong>
          <span>共 {{ filtered.length }} 条记录</span>
        </div>
      </div>
      <a-table
        :data="filtered"
        :loading="loading"
        :pagination="false"
        row-key="name"
        size="large"
        :scroll="{ x: 980 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="访问者" :width="220">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ block(record).enabled === false ? '已禁用' : '已启用' }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="105">
            <template #cell="{ record }">
              <a-tag color="green">{{ record.type.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="Server 代理" :width="190">
            <template #cell="{ record }">{{ block(record).serverName || '-' }}</template>
          </a-table-column>
          <a-table-column title="Server 用户" :width="150">
            <template #cell="{ record }">{{ block(record).serverUser || '-' }}</template>
          </a-table-column>
          <a-table-column title="本地监听" :width="190">
            <template #cell="{ record }">
              <span class="mono">
                {{ block(record).bindAddr || '127.0.0.1' }}:{{ block(record).bindPort ?? '-' }}
              </span>
            </template>
          </a-table-column>
          <a-table-column title="插件" :width="140">
            <template #cell="{ record }">{{ block(record).plugin?.type || '无' }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="190" fixed="right">
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" @click.stop="openEdit(record)">编辑</a-button>
                <a-popconfirm content="确认删除此访问者？" @ok="remove(record)">
                  <a-button type="text" status="danger" @click.stop>删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconUserGroup /></div>
            <strong>暂无访问者</strong>
            <p>创建访问者以连接 STCP、XTCP 或 SUDP 代理。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer v-model:visible="detailVisible" title="访问者详情" :width="680">
      <div v-if="selected" class="detail-sections">
        <div class="detail-section">
          <h3>配置信息</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>名称</span>
              <strong>{{ selected.name }}</strong>
            </div>
            <div class="detail-item">
              <span>类型</span>
              <strong>{{ selected.type.toUpperCase() }}</strong>
            </div>
            <div class="detail-item">
              <span>Server 用户</span>
              <strong>{{ block(selected).serverUser || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>Server 代理</span>
              <strong>{{ block(selected).serverName || '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>本地监听</span>
              <strong class="mono">{{ block(selected).bindAddr || '127.0.0.1' }}:{{ block(selected).bindPort ?? '-' }}</strong>
            </div>
            <div class="detail-item">
              <span>插件</span>
              <strong>{{ block(selected).plugin?.type || '无' }}</strong>
            </div>
          </div>
        </div>
      </div>
    </a-drawer>

    <VisitorEditor
      :visible="editorVisible"
      :initial="editingVisitor"
      @close="editorVisible = false"
      @saved="load"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconPlus,
  IconRefresh,
  IconSearch,
  IconUserGroup,
} from '@arco-design/web-vue/es/icon'
import { legacyRequest } from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import VisitorEditor from '../components/VisitorEditor.vue'
import {
  VISITOR_TYPES,
  type VisitorDefinition,
} from '../types'

const loading = ref(false)
const visitors = ref<VisitorDefinition[]>([])
const storeEnabled = ref(true)
const filters = reactive({ q: '', type: '' })
const detailVisible = ref(false)
const selected = ref<VisitorDefinition>()
const editorVisible = ref(false)
const editingVisitor = ref<VisitorDefinition | null>(null)

const filtered = computed(() => {
  const q = filters.q.toLowerCase().trim()
  return visitors.value.filter((visitor) => {
    if (filters.type && visitor.type !== filters.type) return false
    return !q || visitor.name.toLowerCase().includes(q)
  })
})

onMounted(load)

async function load() {
  loading.value = true
  try {
    const result = await legacyRequest<{ visitors: VisitorDefinition[] }>(
      '/api/store/visitors',
    )
    visitors.value = result.visitors.sort((a, b) => a.name.localeCompare(b.name))
    storeEnabled.value = true
  } catch {
    visitors.value = []
    storeEnabled.value = false
  } finally {
    loading.value = false
  }
}

function block(visitor: VisitorDefinition) {
  return ((visitor as any)[visitor.type] || {}) as Record<string, any>
}

function applyFilters() {
  // Computed filters update immediately.
}

function resetFilters() {
  filters.q = ''
  filters.type = ''
}

function openCreate() {
  editingVisitor.value = null
  editorVisible.value = true
}

function openEdit(visitor: VisitorDefinition) {
  editingVisitor.value = visitor
  editorVisible.value = true
}

function openDetail(visitor: any) {
  selected.value = visitor as VisitorDefinition
  detailVisible.value = true
}

function remove(visitor: VisitorDefinition) {
  Modal.confirm({
    title: '删除访问者',
    content: `确认删除 ${visitor.name}？`,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      await legacyRequest(`/api/store/visitors/${encodeURIComponent(visitor.name)}`, {
        method: 'DELETE',
      })
      Message.success('访问者已删除')
      await load()
    },
  })
}
</script>
