<template>
  <div class="page-stack">
    <PageHeader
      title="流量捕获"
      description="捕获 HTTP 与 HTTPS2HTTP 请求；WSS 记录消息帧，消息帧不支持重放。"
      eyebrow="TRAFFIC CAPTURE"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-button
        type="primary"
        :disabled="selectedRowKeys.length === 0"
        @click="replaySelected"
      >
        <template #icon><IconPlayArrow /></template>
        重放所选
      </a-button>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <div class="panel-title">
          <strong>捕获设置</strong>
          <span>默认关闭，明文落盘，TTL 与容量自动清理</span>
        </div>
        <a-switch v-model="settings.enabled" @change="saveSettings" />
      </div>
      <div class="panel-body capture-settings">
        <div class="capture-setting-row">
          <div>
            <strong>启用代理</strong>
            <span>仅对选中的 HTTP/HTTPS2HTTP 代理生效</span>
          </div>
          <a-select
            v-model="settings.proxies"
            multiple
            placeholder="请选择代理"
            allow-clear
            class="proxy-select"
            @change="saveSettings"
          >
            <a-option v-for="proxy in proxyNames" :key="proxy" :value="proxy">
              {{ proxy }}
            </a-option>
          </a-select>
        </div>
        <div class="capture-setting-row">
          <div>
            <strong>保留时间</strong>
            <span>超时记录在下次读写时自动删除</span>
          </div>
          <a-input-number
            v-model="ttlHours"
            :min="1"
            :max="720"
            :step="1"
            @change="saveSettings"
          >
            <template #suffix>小时</template>
          </a-input-number>
        </div>
        <div class="capture-setting-row">
          <div>
            <strong>单条上限</strong>
            <span>请求或响应过大时标记为截断，不可重放</span>
          </div>
          <a-input-number
            v-model="maxEntryMiB"
            :min="1"
            :max="512"
            :step="1"
            @change="saveSettings"
          >
            <template #suffix>MiB</template>
          </a-input-number>
        </div>
        <a-alert type="warning" :show-icon="true">
          捕获包含完整 Authorization、Cookie 和正文，数据以明文保存在宿主机 data 目录。总容量固定为 500 MiB。
        </a-alert>
      </div>
    </section>

    <a-form :model="filters" class="filter-panel" layout="vertical" @submit="load">
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input v-model="filters.q" placeholder="Host、Path" allow-clear>
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="代理">
          <a-select v-model="filters.proxy" allow-clear placeholder="全部代理">
            <a-option v-for="proxy in proxyNames" :key="proxy" :value="proxy">
              {{ proxy }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="方法">
          <a-select v-model="filters.method" allow-clear placeholder="全部方法">
            <a-option v-for="method in methods" :key="method" :value="method">
              {{ method }}
            </a-option>
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
          <strong>捕获记录</strong>
          <span>敏感请求头不在列表中显示</span>
        </div>
        <a-tag color="orange">列表已隐藏敏感信息</a-tag>
      </div>
      <a-table
        v-model:selected-keys="selectedRowKeys"
        :data="records"
        :loading="loading"
        :pagination="false"
        row-key="id"
        size="large"
        :row-selection="{ type: 'checkbox', showCheckedAll: true }"
        :scroll="{ x: 1120 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="请求" :width="280">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.method }} {{ record.host }}</strong>
                <span>{{ record.path || '/' }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="协议" :width="95">
            <template #cell="{ record }">
              <a-tag>{{ record.protocol.toUpperCase() }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="代理" data-index="proxy" :width="140" />
          <a-table-column title="状态" :width="90">
            <template #cell="{ record }">
              <a-tag :color="record.responseStatus >= 400 ? 'red' : 'green'">
                {{ record.responseStatus || '-' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="耗时" :width="100">
            <template #cell="{ record }">{{ record.durationMs }} ms</template>
          </a-table-column>
          <a-table-column title="大小" :width="100">
            <template #cell="{ record }">{{ formatBytes(record.size) }}</template>
          </a-table-column>
          <a-table-column title="特征" :width="150">
            <template #cell="{ record }">
              <a-space>
                <a-tag v-if="record.webSocket" color="purple">WSS</a-tag>
                <a-tag v-if="record.truncated" color="orange">截断</a-tag>
                <span v-if="!record.webSocket && !record.truncated">完整</span>
              </a-space>
            </template>
          </a-table-column>
          <a-table-column title="时间" :width="190">
            <template #cell="{ record }">{{ formatDate(record.startedAt) }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="130" fixed="right">
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" @click.stop="openDetail(record)">详情</a-button>
                <a-popconfirm content="确认删除此捕获记录？" @ok="remove(record.id)">
                  <a-button type="text" status="danger" @click.stop>删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconFile /></div>
            <strong>暂无捕获记录</strong>
            <p>选择 HTTP 或 HTTPS2HTTP 代理并启用捕获后，请求会出现在这里。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:visible="detailVisible"
      title="捕获详情"
      :width="860"
      unmount-on-close
    >
      <div v-if="detail" class="detail-sections">
        <div class="capture-summary">
          <div class="capture-url">
            <a-tag :color="detail.responseStatus >= 400 ? 'red' : 'green'">
              {{ detail.responseStatus }}
            </a-tag>
            <strong>{{ detail.method }}</strong>
            <span class="mono">{{ displayURL(detail) }}</span>
          </div>
          <a-button size="small" @click="toggleSensitive">
            {{ sensitiveVisible ? '隐藏敏感内容' : '显示敏感内容' }}
          </a-button>
        </div>

        <a-tabs default-active-key="request">
          <a-tab-pane key="request" title="请求">
            <h3 class="capture-section-title">请求头</h3>
            <pre class="code-block">{{ formatHeaders(detail.requestHeaders, sensitiveVisible) }}</pre>
            <h3 class="capture-section-title">请求正文</h3>
            <pre class="code-block">{{ formatBody(detail.requestBody) }}</pre>
          </a-tab-pane>
          <a-tab-pane key="response" title="响应">
            <h3 class="capture-section-title">响应头</h3>
            <pre class="code-block">{{ formatHeaders(detail.responseHeaders, sensitiveVisible) }}</pre>
            <h3 class="capture-section-title">响应正文</h3>
            <pre class="code-block">{{ formatBody(detail.responseBody) }}</pre>
          </a-tab-pane>
          <a-tab-pane key="websocket" title="WebSocket 帧">
            <a-empty v-if="!detail.webSocketFrames?.length" description="此记录没有 WebSocket 消息帧" />
            <div v-else class="ws-frame-list">
              <div
                v-for="(frame, index) in detail.webSocketFrames"
                :key="`${frame.at}-${index}`"
              >
                <div class="ws-frame-head">
                  <a-tag :color="frame.direction === 'client' ? 'arcoblue' : 'green'">
                    {{ frame.direction === 'client' ? '客户端 → 服务端' : '服务端 → 客户端' }}
                  </a-tag>
                  <span>opcode {{ frame.opcode }}</span>
                  <span>{{ formatDate(frame.at) }}</span>
                </div>
                <pre class="code-block">{{ formatFrame(frame) }}</pre>
              </div>
            </div>
          </a-tab-pane>
          <a-tab-pane key="replay" title="重放结果">
            <a-empty v-if="!detail.replayResults?.length" description="尚未重放" />
            <div v-else class="ws-frame-list">
              <div
                v-for="(result, index) in detail.replayResults"
                :key="index"
                class="validation-item"
                :class="result.error ? 'error' : 'success'"
              >
                <strong>{{ result.error || `HTTP ${result.status}` }}</strong>
                <span> · {{ result.durationMs }} ms · {{ formatDate(result.startedAt) }}</span>
                <pre v-if="result.responseBody" class="code-block replay-body">{{
                  formatBody(result.responseBody)
                }}</pre>
              </div>
            </div>
          </a-tab-pane>
        </a-tabs>
      </div>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconFile,
  IconPlayArrow,
  IconRefresh,
  IconSearch,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  legacyRequest,
  type CaptureListItem,
  type CaptureRecord,
  type CaptureSettings,
  type WebSocketFrame,
} from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'

const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const loading = ref(false)
const records = ref<CaptureListItem[]>([])
const selectedRowKeys = ref<string[]>([])
const proxyNames = ref<string[]>([])
const detailVisible = ref(false)
const detail = ref<CaptureRecord>()
const sensitiveVisible = ref(false)
const settings = reactive<CaptureSettings>({
  enabled: false,
  proxies: [],
  ttlSeconds: 86400,
  maxEntryBytes: 16 * 1024 * 1024,
  maxTotalBytes: 500 * 1024 * 1024,
})
const filters = reactive({ q: '', proxy: '', method: '' })

const ttlHours = ref(24)
const maxEntryMiB = ref(16)

onMounted(async () => {
  await Promise.all([loadProxies(), loadSettings(), load()])
})

async function loadProxies() {
  try {
    const status = await legacyRequest<Record<string, Array<{ name: string; type: string }>>>(
      '/api/status',
    )
    proxyNames.value = Object.values(status)
      .flat()
      .filter((proxy) => ['http', 'https'].includes(proxy.type))
      .map((proxy) => proxy.name)
      .sort()
  } catch {
    proxyNames.value = []
  }
}

async function loadSettings() {
  try {
    const value = await adminAPI.captureSettings()
    Object.assign(settings, value)
    ttlHours.value = Math.max(1, Math.round(value.ttlSeconds / 3600))
    maxEntryMiB.value = Math.max(
      1,
      Math.round(value.maxEntryBytes / 1024 / 1024),
    )
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取捕获设置失败')
  }
}

async function saveSettings() {
  try {
    settings.ttlSeconds = Math.max(1, Number(ttlHours.value || 24)) * 3600
    settings.maxEntryBytes = Math.max(
      1,
      Number(maxEntryMiB.value || 16),
    ) * 1024 * 1024
    settings.maxTotalBytes = 500 * 1024 * 1024
    Object.assign(settings, await adminAPI.saveCaptureSettings(settings))
    Message.success('捕获设置已保存')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '保存捕获设置失败')
  }
}

async function load() {
  loading.value = true
  try {
    const query: Record<string, string> = {}
    if (filters.q.trim()) query.q = filters.q.trim()
    if (filters.proxy) query.proxy = filters.proxy
    if (filters.method) query.method = filters.method
    records.value = await adminAPI.captures(query)
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载捕获记录失败')
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.q = ''
  filters.proxy = ''
  filters.method = ''
  load()
}

async function toggleSensitive() {
  const next = !sensitiveVisible.value
  sensitiveVisible.value = next
  if (next && detail.value) {
    try {
      await adminAPI.auditCaptureReveal(detail.value.id)
    } catch {
      // The sensitive view remains available if audit storage is temporarily unavailable.
    }
  }
}

async function openDetail(record: any) {
  const item = record as CaptureListItem
  try {
    detail.value = await adminAPI.capture(item.id)
    sensitiveVisible.value = false
    detailVisible.value = true
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取捕获详情失败')
  }
}

function replaySelected() {
  const selected = records.value.filter((record) =>
    selectedRowKeys.value.includes(record.id),
  )
  const unsafe = selected.some(
    (record) => !['GET', 'HEAD', 'OPTIONS'].includes(record.method),
  )
  Modal.confirm({
    title: unsafe ? '重放修改性请求' : '重放捕获请求',
    content: unsafe
      ? '所选记录包含非幂等请求，重放可能修改后端数据。确认继续？'
      : `确认重放 ${selected.length} 条请求到原代理后端？`,
    okText: '确认重放',
    okButtonProps: unsafe ? { status: 'danger' } : {},
    onOk: async () => {
      try {
        const results = await adminAPI.replayCaptures(
          selectedRowKeys.value,
          1,
          200,
        )
        const failed = results.filter((item) => item.result.error)
        if (failed.length) {
          Message.warning(`${results.length - failed.length} 条成功，${failed.length} 条失败`)
        } else {
          Message.success(`${results.length} 条请求重放完成`)
        }
        selectedRowKeys.value = []
        await load()
      } catch (error) {
        Message.error(error instanceof Error ? error.message : '重放失败')
      }
    },
  })
}

async function remove(id: string) {
  try {
    await adminAPI.deleteCapture(id)
    Message.success('捕获记录已删除')
    await load()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '删除失败')
  }
}

function formatHeaders(
  headers: Record<string, string[]> | undefined,
  reveal: boolean,
) {
  if (!headers) return '无'
  return Object.entries(headers)
    .map(([key, values]) => {
      const sensitive = [
        'authorization',
        'proxy-authorization',
        'cookie',
        'set-cookie',
      ].includes(key.toLowerCase())
      return `${key}: ${sensitive && !reveal ? '••••••••' : values.join(', ')}`
    })
    .join('\n')
}

function formatBody(body?: string) {
  if (!body) return '无'
  try {
    const decoded = atob(body)
    if (decoded.length > 200_000) return `${decoded.slice(0, 200_000)}\n\n[内容已截断显示]`
    return decoded
  } catch {
    return body
  }
}

function formatFrame(frame: WebSocketFrame) {
  const payload = formatBody(frame.payload)
  if ([1, 8].includes(frame.opcode)) return payload
  return `[二进制帧，base64]\n${frame.payload || ''}`
}

function displayURL(record: CaptureRecord) {
  let query = record.query
  if (!sensitiveVisible.value && query) {
    const params = new URLSearchParams(query)
    for (const key of [...params.keys()]) {
      if (/token|key|secret|password|auth/i.test(key)) {
        params.set(key, '••••••')
      }
    }
    query = params.toString()
  }
  return `${record.protocol}://${record.host}${record.path || '/'}${query ? `?${query}` : ''}`
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB']
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatDate(value?: string) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'short',
    timeStyle: 'medium',
  }).format(new Date(value))
}
</script>

<style scoped>
.capture-settings {
  display: grid;
  gap: 18px;
}

.capture-setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.capture-setting-row > div:first-child {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.capture-setting-row strong {
  color: var(--frp-text);
  font-size: 13px;
}

.capture-setting-row span {
  color: var(--frp-muted);
  font-size: 11px;
}

.proxy-select {
  width: min(480px, 60%);
}

.capture-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  border: 1px solid var(--frp-border);
  border-radius: 11px;
  padding: 13px;
  background: var(--frp-surface-muted);
}

.capture-url {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
}

.capture-url span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.capture-section-title {
  margin: 20px 0 9px;
  color: var(--frp-text);
  font-size: 13px;
}

.ws-frame-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ws-frame-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 7px;
  color: var(--frp-muted);
  font-size: 11px;
}

.replay-body {
  margin: 8px 0 0;
}
</style>
