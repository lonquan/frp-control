<template>
  <div class="page-stack">
    <PageHeader
      title="流量捕获"
      description="捕获 HTTP 与 HTTPS2HTTP 请求；WSS 记录消息帧，消息帧不支持重放。"
      eyebrow="TRAFFIC CAPTURE"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新列表
      </a-button>
    </PageHeader>

    <section class="panel capture-toolbar-panel">
      <div class="capture-toolbar">
        <div class="capture-toolbar-main">
          <StatusBadge
            :status="settings.enabled ? 'enabled' : 'closed'"
            :label="settings.enabled ? '捕获已启用' : '捕获已关闭'"
          />
          <div class="capture-summary-list" aria-label="捕获设置摘要">
            <span><strong>{{ settings.proxies.length }}</strong> 个代理</span>
            <span>保留 <strong>{{ formatHours(settings.ttlSeconds) }}</strong> 小时</span>
            <span>单条 <strong>{{ formatMiB(settings.maxEntryBytes) }}</strong> MiB</span>
            <span><strong>{{ settings.excludePatterns.length }}</strong> 条排除规则</span>
          </div>
        </div>
        <a-button @click="openSettings">
          <template #icon><IconSettings /></template>
          配置
        </a-button>
      </div>
      <a-alert type="warning" :show-icon="true" class="capture-warning">
        捕获保存完整 Authorization、Cookie 和正文；数据以明文保存在宿主机，总容量固定为 500 MiB。
      </a-alert>
    </section>

    <a-form
      :model="filters"
      class="filter-panel capture-filter-form"
      layout="vertical"
      @submit="load"
    >
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
        <div class="filter-actions capture-filter-actions">
          <a-button type="primary" html-type="submit">查询</a-button>
          <a-button @click="resetFilters">重置</a-button>
        </div>
      </div>
    </a-form>

    <section class="table-panel">
      <div class="table-toolbar compact-toolbar">
        <div class="table-toolbar-copy">
          <strong>捕获记录</strong>
          <span>{{ records.length }} 条 · 敏感请求头默认隐藏</span>
        </div>
        <a-tag color="orange">明文存储</a-tag>
      </div>
      <a-table
        :data="records"
        :loading="loading"
        :pagination="false"
        row-key="id"
        size="small"
        :scroll="{ x: compactTable ? 720 : 900 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="请求" :width="compactTable ? 230 : 260">
            <template #cell="{ record }">
              <div class="row-primary request-row">
                <div class="request-title">
                  <strong>{{ record.method }} {{ record.host }}</strong>
                  <a-tag v-if="record.webSocket" color="purple" size="small">WSS</a-tag>
                  <a-tag v-if="record.truncated" color="orange" size="small">截断</a-tag>
                </div>
                <span>{{ record.path || '/' }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="来源地址" :width="compactTable ? 140 : 155">
            <template #cell="{ record }">
              <span class="mono source-address">{{ sourceLabel(record) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="代理 / 协议" :width="compactTable ? 110 : 125">
            <template #cell="{ record }">
              <div class="row-primary compact-primary">
                <strong>{{ record.proxy }}</strong>
                <span>{{ record.protocol.toUpperCase() }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="结果" :width="compactTable ? 150 : 165">
            <template #cell="{ record }">
              <div class="result-cell">
                <a-tag :color="responseColor(record.responseStatus)" size="small">
                  {{ record.responseStatus || '-' }}
                </a-tag>
                <span>{{ record.durationMs }} ms · {{ formatBytes(record.size) }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column v-if="!compactTable" title="时间" :width="155">
            <template #cell="{ record }">{{ formatDate(record.startedAt) }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="64" fixed="right" align="center">
            <template #cell="{ record }">
              <a-dropdown
                trigger="click"
                position="br"
                @click.stop
              >
                <a-button
                  type="text"
                  class="row-more-button"
                  :aria-label="`${record.method} ${record.host} 的操作`"
                >
                  <IconMore />
                </a-button>
                <template #content>
                  <a-doption @click="openDetail(record)">详情</a-doption>
                  <a-doption
                    :disabled="!canReplay(record)"
                    @click="startReplay(record, 'list')"
                  >
                    重放
                  </a-doption>
                  <a-doption class="danger-option" @click="confirmRemove(record)">
                    删除
                  </a-doption>
                </template>
              </a-dropdown>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state compact-empty">
            <div class="empty-icon"><IconFile /></div>
            <strong>暂无捕获记录</strong>
            <p>启用捕获后，未被排除规则匹配的 HTTP 请求会出现在这里。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:visible="settingsVisible"
      title="捕获设置"
      :width="560"
      :on-before-cancel="confirmDiscardSettings"
      unmount-on-close
    >
      <a-form :model="settingsDraft" layout="vertical" class="settings-form">
        <a-form-item label="启用捕获">
          <div class="settings-switch-row">
            <a-switch v-model="settingsDraft.enabled" aria-label="启用流量捕获" />
            <span>{{ settingsDraft.enabled ? '捕获开启后立即生效' : '当前不会记录新请求' }}</span>
          </div>
        </a-form-item>
        <a-form-item
          label="启用代理"
          :validate-status="settingsDraft.enabled && settingsDraft.proxies.length === 0 ? 'error' : undefined"
          :help="settingsDraft.enabled && settingsDraft.proxies.length === 0 ? '启用捕获时至少选择一个代理' : undefined"
        >
          <a-select
            v-model="settingsDraft.proxies"
            multiple
            placeholder="请选择 HTTP 或 HTTPS2HTTP 代理"
            allow-clear
          >
            <a-option v-for="proxy in proxyNames" :key="proxy" :value="proxy">
              {{ proxy }}
            </a-option>
          </a-select>
          <template #extra>仅对选中的 HTTP/HTTPS2HTTP 代理生效。</template>
        </a-form-item>
        <div class="settings-grid">
          <a-form-item label="保留时间">
            <a-input-number
              v-model="settingsDraft.ttlHours"
              :min="1"
              :max="720"
              :step="1"
            >
              <template #suffix>小时</template>
            </a-input-number>
            <template #extra>超时记录在下次读写时清理。</template>
          </a-form-item>
          <a-form-item label="单条上限">
            <a-input-number
              v-model="settingsDraft.maxEntryMiB"
              :min="1"
              :max="512"
              :step="1"
            >
              <template #suffix>MiB</template>
            </a-input-number>
            <template #extra>超过上限时标记为截断，不能重放。</template>
          </a-form-item>
        </div>
        <a-form-item label="排除关键字">
          <a-textarea
            v-model="settingsDraft.excludeText"
            :auto-size="{ minRows: 6, maxRows: 12 }"
            :max-length="maxExcludeTextLength"
            show-word-limit
            placeholder="favicon.ico&#10;/static/*&#10;/healthz"
          />
          <template #extra>
            每行一条，匹配 Path 和 Query，不区分大小写。支持 * 和 ? 通配符；最多 {{ maxExcludePatterns }} 条。
          </template>
        </a-form-item>
        <a-alert type="warning" :show-icon="true">
          设置保存后立即生效，无需重启服务。排除规则只影响之后的新请求。
        </a-alert>
      </a-form>
      <template #footer>
        <a-space>
          <a-button @click="requestCloseSettings">取消</a-button>
          <a-button :disabled="!settingsDirty" @click="resetSettingsDraft">
            放弃更改
          </a-button>
          <a-button
            type="primary"
            :loading="settingsSaving"
            :disabled="!settingsDirty"
            @click="saveSettings"
          >
            保存设置
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <a-drawer
      v-model:visible="detailVisible"
      title="捕获详情"
      :width="920"
      :footer="false"
      unmount-on-close
    >
      <div v-if="detail" class="detail-sections">
        <div class="capture-detail-summary">
          <div class="capture-url">
            <a-tag :color="responseColor(detail.responseStatus)">
              {{ detail.responseStatus || '-' }}
            </a-tag>
            <strong>{{ detail.method }}</strong>
            <span class="mono">{{ displayURL(detail) }}</span>
          </div>
          <a-space wrap>
            <a-button :loading="detailLoading" @click="refreshDetail()">
              <template #icon><IconRefresh /></template>
              刷新
            </a-button>
            <a-button @click="toggleSensitive">
              {{ sensitiveVisible ? '隐藏敏感内容' : '显示敏感内容' }}
            </a-button>
            <a-button
              type="primary"
              :loading="replayingID === detail.id"
              :disabled="!canReplay(detail)"
              @click="startReplay(detail, 'detail')"
            >
              <template #icon><IconPlayArrow /></template>
              重放请求
            </a-button>
          </a-space>
        </div>

        <div class="capture-meta">
          <div>
            <span>来源地址</span>
            <strong class="mono">{{ detail.sourceAddr || '-' }}</strong>
          </div>
          <div>
            <span>来源端口</span>
            <strong class="mono">{{ detail.sourcePort || '-' }}</strong>
          </div>
          <div>
            <span>代理</span>
            <strong>{{ detail.proxy }}</strong>
          </div>
          <div>
            <span>耗时</span>
            <strong>{{ detail.durationMs }} ms</strong>
          </div>
          <div>
            <span>捕获时间</span>
            <strong>{{ formatDate(detail.startedAt) }}</strong>
          </div>
        </div>

        <a-tabs v-model:active-key="activeTab">
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
          <a-tab-pane
            key="replay"
            :title="`重放记录${detail.replayResults?.length ? ` (${detail.replayResults.length})` : ''}`"
          >
            <a-alert type="info" :show-icon="false" class="replay-help">
              每次重放都会发送到原代理后端。以下展示原始响应与最近 20 次重放结果的差异。
            </a-alert>
            <a-empty v-if="!detail.replayResults?.length" description="尚未重放此请求" />
            <div v-else class="replay-list">
              <article
                v-for="(result, index) in detail.replayResults"
                :key="`${result.startedAt}-${index}`"
                class="replay-attempt"
              >
                <div class="replay-attempt-head">
                  <strong>第 {{ index + 1 }} 次重放</strong>
                  <span>{{ formatDate(result.startedAt) }}</span>
                </div>
                <div class="replay-compare">
                  <section class="replay-column">
                    <h4>原始响应</h4>
                    <div class="result-cell">
                      <a-tag :color="responseColor(detail.responseStatus)" size="small">
                        {{ detail.responseStatus || '-' }}
                      </a-tag>
                      <span>{{ detail.durationMs }} ms · {{ formatBytes(captureBodySize(detail)) }}</span>
                    </div>
                    <details v-if="detail.responseBody" class="response-details">
                      <summary>查看原始响应正文</summary>
                      <pre class="code-block replay-body">{{ formatBody(detail.responseBody) }}</pre>
                    </details>
                  </section>
                  <section class="replay-column" :class="{ failed: !!result.error }">
                    <h4>本次重放</h4>
                    <div class="result-cell">
                      <a-tag
                        v-if="!result.error"
                        :color="responseColor(result.status)"
                        size="small"
                      >
                        {{ result.status || '-' }}
                      </a-tag>
                      <strong v-else class="replay-error">{{ result.error }}</strong>
                      <span>{{ result.durationMs }} ms</span>
                    </div>
                    <details v-if="result.responseBody" class="response-details">
                      <summary>查看重放响应正文</summary>
                      <pre class="code-block replay-body">{{ formatBody(result.responseBody) }}</pre>
                    </details>
                  </section>
                </div>
              </article>
            </div>
          </a-tab-pane>
        </a-tabs>
      </div>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconFile,
  IconMore,
  IconPlayArrow,
  IconRefresh,
  IconSearch,
  IconSettings,
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
import StatusBadge from '@shared/components/StatusBadge.vue'

const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const maxExcludePatterns = 100
const maxExcludePatternLength = 256
const maxExcludeTextLength = maxExcludePatterns * (maxExcludePatternLength + 1)

const loading = ref(false)
const compactTable = useMediaQuery('(max-width: 1100px)')
const records = ref<CaptureListItem[]>([])
const proxyNames = ref<string[]>([])
const detailVisible = ref(false)
const detail = ref<CaptureRecord>()
const detailLoading = ref(false)
const activeTab = ref('request')
const sensitiveVisible = ref(false)
const replayingID = ref('')
const settingsVisible = ref(false)
const settingsSaving = ref(false)
const settings = ref<CaptureSettings>(emptySettings())
const settingsDraft = reactive({
  enabled: false,
  proxies: [] as string[],
  ttlHours: 24,
  maxEntryMiB: 16,
  excludeText: '',
})
const filters = reactive({ q: '', proxy: '', method: '' })

const settingsDirty = computed(() => {
  const draft = buildDraftSettings(false)
  const saved = settings.value
  return (
    draft.enabled !== saved.enabled ||
    !sameStringArray(draft.proxies, saved.proxies) ||
    draft.ttlSeconds !== saved.ttlSeconds ||
    draft.maxEntryBytes !== saved.maxEntryBytes ||
    !sameStringArray(draft.excludePatterns, saved.excludePatterns)
  )
})

onMounted(async () => {
  await Promise.all([loadProxies(), loadSettings(), load()])
})

function emptySettings(): CaptureSettings {
  return {
    enabled: false,
    proxies: [],
    excludePatterns: [],
    ttlSeconds: 86400,
    maxEntryBytes: 16 * 1024 * 1024,
    maxTotalBytes: 500 * 1024 * 1024,
  }
}

async function loadProxies() {
  try {
    const status = await legacyRequest<Record<string, Array<{ name: string; type: string }>>>(
      '/api/status',
    )
    const names = Object.values(status)
      .flat()
      .filter((proxy) => ['http', 'https'].includes(proxy.type))
      .map((proxy) => proxy.name)
    proxyNames.value = [...new Set([...names, ...settings.value.proxies])].sort()
  } catch {
    proxyNames.value = [...settings.value.proxies]
  }
}

async function loadSettings() {
  try {
    const value = await adminAPI.captureSettings()
    settings.value = normalizeSettings(value)
    proxyNames.value = [...new Set([...proxyNames.value, ...settings.value.proxies])].sort()
    if (!settingsVisible.value) {
      resetSettingsDraft()
    }
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取捕获设置失败')
  }
}

function normalizeSettings(value: CaptureSettings): CaptureSettings {
  return {
    ...emptySettings(),
    ...value,
    proxies: Array.isArray(value.proxies) ? [...value.proxies] : [],
    excludePatterns: Array.isArray(value.excludePatterns)
      ? [...value.excludePatterns]
      : [],
  }
}

function openSettings() {
  resetSettingsDraft()
  settingsVisible.value = true
}

function resetSettingsDraft() {
  Object.assign(settingsDraft, {
    enabled: settings.value.enabled,
    proxies: [...settings.value.proxies],
    ttlHours: Math.max(1, Math.round(settings.value.ttlSeconds / 3600)),
    maxEntryMiB: Math.max(1, Math.round(settings.value.maxEntryBytes / 1024 / 1024)),
    excludeText: settings.value.excludePatterns.join('\n'),
  })
}

function parseExcludeText(text: string) {
  const seen = new Set<string>()
  const patterns: string[] = []
  for (const raw of text.split(/\r?\n/)) {
    const pattern = raw.trim()
    if (!pattern) continue
    const key = pattern.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    patterns.push(pattern)
  }
  return patterns
}

function buildDraftSettings(validate: boolean): CaptureSettings {
  const excludePatterns = parseExcludeText(settingsDraft.excludeText)
  if (validate) {
    if (settingsDraft.enabled && settingsDraft.proxies.length === 0) {
      throw new Error('启用捕获时至少选择一个代理')
    }
    if (excludePatterns.length > maxExcludePatterns) {
      throw new Error(`排除关键字最多支持 ${maxExcludePatterns} 条`)
    }
    if (excludePatterns.some((pattern) => [...pattern].length > maxExcludePatternLength)) {
      throw new Error(`排除关键字单条不能超过 ${maxExcludePatternLength} 个字符`)
    }
  }
  return {
    enabled: settingsDraft.enabled,
    proxies: [...settingsDraft.proxies],
    excludePatterns,
    ttlSeconds: Math.max(1, Number(settingsDraft.ttlHours || 24)) * 3600,
    maxEntryBytes:
      Math.max(1, Number(settingsDraft.maxEntryMiB || 16)) * 1024 * 1024,
    maxTotalBytes: 500 * 1024 * 1024,
  }
}

async function saveSettings() {
  let payload: CaptureSettings
  try {
    payload = buildDraftSettings(true)
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '检查捕获设置失败')
    return
  }
  settingsSaving.value = true
  try {
    settings.value = normalizeSettings(await adminAPI.saveCaptureSettings(payload))
    settingsVisible.value = false
    Message.success('捕获设置已保存并生效，无需重启服务')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '保存捕获设置失败')
  } finally {
    settingsSaving.value = false
  }
}

function confirmDiscardSettings() {
  if (!settingsDirty.value) {
    return true
  }
  Modal.confirm({
    title: '放弃未保存的设置？',
    content: '关闭后，本次修改的捕获代理、保留时间和排除规则不会生效。',
    okText: '放弃更改',
    okButtonProps: { status: 'danger' },
    onOk: () => {
      settingsVisible.value = false
    },
  })
  return false
}

function requestCloseSettings() {
  if (confirmDiscardSettings()) {
    settingsVisible.value = false
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
    const value = await adminAPI.capture(item.id)
    detail.value = value
    activeTab.value = 'request'
    sensitiveVisible.value = false
    detailVisible.value = true
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取捕获详情失败')
  }
}

async function refreshDetail(id = detail.value?.id) {
  if (!id) return false
  detailLoading.value = true
  try {
    const value = await adminAPI.capture(id)
    if (detail.value?.id === id) {
      detail.value = value
    }
    return true
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '刷新捕获详情失败')
    return false
  } finally {
    detailLoading.value = false
  }
}

function canReplay(record: Pick<CaptureListItem, 'webSocket' | 'truncated'>) {
  return !record.webSocket && !record.truncated
}

function startReplay(
  record: CaptureListItem,
  source: 'list' | 'detail',
) {
  if (!canReplay(record)) {
    Message.warning(record.webSocket ? 'WebSocket 消息帧不支持重放' : '截断记录不能重放')
    return
  }
  const unsafe = !['GET', 'HEAD', 'OPTIONS'].includes(record.method)
  Modal.confirm({
    title: unsafe ? '重放修改性请求' : '重放捕获请求',
    content: unsafe
      ? `将重放 ${record.method} ${record.host}${record.path || '/'}，可能修改后端数据。确认继续？`
      : `确认重放 ${record.method} ${record.host}${record.path || '/'} 到原代理后端？`,
    okText: '确认重放',
    okButtonProps: unsafe ? { status: 'danger' } : {},
    onOk: async () => {
      replayingID.value = record.id
      try {
        const [response] = await adminAPI.replayCaptures([record.id], 1, 0)
        if (source === 'list') {
          await load()
        } else {
          await refreshDetail(record.id)
        }
        if (response?.result.error) {
          Message.error(`重放失败：${response.result.error}`)
        } else {
          Message.success(`重放完成：HTTP ${response?.result.status || '-'}`)
        }
      } catch (error) {
        Message.error(error instanceof Error ? error.message : '重放失败')
      } finally {
        replayingID.value = ''
      }
    },
  })
}

function confirmRemove(record: CaptureListItem) {
  Modal.confirm({
    title: '删除捕获记录',
    content: `确认删除 ${record.method} ${record.host}${record.path || '/'}？`,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      await remove(record.id)
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
    if (decoded.length > 200_000) {
      return `${decoded.slice(0, 200_000)}\n\n[内容已截断显示]`
    }
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

function sourceLabel(record: Pick<CaptureListItem, 'sourceAddr' | 'sourcePort'>) {
  if (!record.sourceAddr) return '-'
  const address = record.sourceAddr.includes(':')
    ? `[${record.sourceAddr}]`
    : record.sourceAddr
  return record.sourcePort ? `${address}:${record.sourcePort}` : address
}

function responseColor(status: number) {
  if (!status) return 'gray'
  if (status >= 500) return 'red'
  if (status >= 400) return 'orange'
  return 'green'
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB']
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  )
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function captureBodySize(record: CaptureRecord) {
  return (
    base64Length(record.requestBody) +
    base64Length(record.responseBody)
  )
}

function base64Length(value?: string) {
  if (!value) return 0
  const padding = value.endsWith('==') ? 2 : value.endsWith('=') ? 1 : 0
  return Math.max(0, Math.floor((value.length * 3) / 4) - padding)
}

function formatHours(seconds: number) {
  return Math.max(1, Math.round(seconds / 3600))
}

function formatMiB(bytes: number) {
  return Math.max(1, Math.round(bytes / 1024 / 1024))
}

function sameStringArray(left: string[], right: string[]) {
  return left.length === right.length && left.every((value, index) => value === right[index])
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
.capture-toolbar-panel {
  overflow: hidden;
}

.capture-toolbar {
  display: flex;
  min-height: 64px;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 12px 18px;
}

.capture-toolbar-main {
  display: flex;
  min-width: 0;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px 18px;
}

.capture-summary-list {
  display: flex;
  min-width: 0;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 16px;
  color: var(--frp-muted);
  font-size: 12px;
}

.capture-summary-list strong {
  color: var(--frp-text);
  font-variant-numeric: tabular-nums;
}

.capture-warning {
  border-radius: 0;
}

.capture-filter-form .filter-grid {
  grid-template-columns: minmax(260px, 1.6fr) repeat(2, minmax(160px, 0.7fr));
}

.capture-filter-actions {
  grid-column: 1 / -1;
  justify-content: flex-start;
  padding-bottom: 16px;
}

.compact-toolbar {
  min-height: 52px;
}

.request-title {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
}

.request-title strong {
  min-width: 0;
}

.source-address {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-primary {
  gap: 1px;
}

.result-cell {
  display: flex;
  min-width: 0;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  color: var(--frp-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.row-more-button {
  min-width: 36px;
  min-height: 36px;
  padding: 0;
}

.compact-empty {
  min-height: 180px;
}

.settings-form :deep(.arco-form-item) {
  margin-bottom: 20px;
}

.settings-switch-row {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--frp-muted);
  font-size: 12px;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.capture-detail-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border: 1px solid var(--frp-border);
  border-radius: 12px;
  padding: 14px;
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

.capture-meta {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
}

.capture-meta > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
  border: 1px solid var(--frp-border);
  border-radius: 10px;
  padding: 10px 12px;
  background: var(--frp-surface-muted);
}

.capture-meta span {
  color: var(--frp-muted);
  font-size: 11px;
}

.capture-meta strong {
  overflow-wrap: anywhere;
  color: var(--frp-text);
  font-size: 12px;
}

.capture-section-title {
  margin: 20px 0 9px;
  color: var(--frp-text);
  font-size: 13px;
}

.ws-frame-list,
.replay-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ws-frame-head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 7px;
  color: var(--frp-muted);
  font-size: 11px;
}

.replay-help {
  margin-bottom: 14px;
}

.replay-attempt {
  overflow: hidden;
  border: 1px solid var(--frp-border);
  border-radius: 12px;
  background: var(--frp-surface);
}

.replay-attempt-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 13px;
  border-bottom: 1px solid var(--frp-border);
  background: var(--frp-surface-muted);
  color: var(--frp-muted);
  font-size: 11px;
}

.replay-attempt-head strong {
  color: var(--frp-text);
  font-size: 12px;
}

.replay-compare {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.replay-column {
  min-width: 0;
  padding: 13px;
}

.replay-column + .replay-column {
  border-left: 1px solid var(--frp-border);
}

.replay-column.failed {
  background: rgb(var(--danger-1));
}

.replay-column h4 {
  margin: 0 0 8px;
  color: var(--frp-text);
  font-size: 12px;
}

.replay-error {
  overflow-wrap: anywhere;
  color: rgb(var(--danger-7));
  font-size: 12px;
}

.response-details {
  margin-top: 10px;
}

.response-details summary {
  color: var(--frp-role-strong);
  cursor: pointer;
  font-size: 12px;
}

.replay-body {
  margin: 8px 0 0;
}

@media (max-width: 1024px) {
  .capture-filter-form .filter-grid {
    grid-template-columns: minmax(180px, 1.4fr) repeat(2, minmax(130px, 0.7fr));
  }

  .capture-filter-actions {
    grid-column: 1 / -1;
  }

  .capture-meta {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .capture-detail-summary {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 720px) {
  .capture-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .capture-toolbar > :deep(.arco-btn) {
    align-self: flex-start;
  }

  .capture-filter-form .filter-grid,
  .settings-grid,
  .capture-meta,
  .replay-compare {
    grid-template-columns: 1fr;
  }

  .replay-column + .replay-column {
    border-top: 1px solid var(--frp-border);
    border-left: 0;
  }
}
</style>
