<template>
  <a-drawer
    :visible="visible"
    :title="editing ? '编辑代理' : '新建代理'"
    :width="drawerWidth"
    unmount-on-close
    @cancel="requestClose"
  >
    <a-form ref="formRef" :model="form" layout="vertical">
      <section class="section-card">
        <div class="section-title section-title-row">
          <span>基础信息</span>
          <a-button size="mini" @click="exampleVisible = true">
            <template #icon><IconBook /></template>
            查看 {{ form.type.toUpperCase() }} 示例
          </a-button>
        </div>
        <div class="form-grid three">
          <a-form-item
            field="name"
            label="代理名称"
            :rules="[
              { required: true, message: '请输入代理名称' },
              { match: /^[a-zA-Z0-9_.-]+$/, message: '仅支持字母、数字、点、下划线和短横线' },
            ]"
          >
            <a-input v-model="form.name" :disabled="editing" placeholder="my-proxy" />
          </a-form-item>
          <a-form-item label="代理类型">
            <a-select v-model="form.type" :disabled="editing" @change="changeType">
              <a-option v-for="type in PROXY_TYPES" :key="type" :value="type">
                {{ type.toUpperCase() }}
              </a-option>
            </a-select>
          </a-form-item>
          <a-form-item label="启用">
            <div class="switch-field">
              <a-switch v-model="form.enabled" />
              <span>{{ form.enabled ? '启用代理' : '暂时禁用' }}</span>
            </div>
          </a-form-item>
        </div>
      </section>

      <section class="section-card">
        <div class="section-title">后端服务</div>
        <a-radio-group v-model="backendMode" type="button" class="backend-mode">
          <a-radio value="direct">直接连接</a-radio>
          <a-radio value="plugin">客户端插件</a-radio>
        </a-radio-group>

        <div v-if="backendMode === 'direct'" class="form-grid">
          <a-form-item
            field="localIP"
            label="本地 IP / 主机名"
            :rules="[{ required: true, message: '请输入本地 IP 或主机名' }]"
          >
            <a-input v-model="form.localIP" placeholder="host.docker.internal" />
            <template #extra>Docker Desktop 访问宿主机服务时请使用 host.docker.internal。</template>
          </a-form-item>
          <a-form-item
            field="localPort"
            label="本地端口"
            :rules="[{ required: true, message: '请输入本地端口' }]"
          >
            <a-input-number v-model="form.localPort" :min="1" :max="65535" class="full-width" />
          </a-form-item>
        </div>

        <template v-else>
          <div class="form-grid">
            <a-form-item label="插件类型">
              <a-select v-model="form.pluginType" @change="resetPluginConfig">
                <a-option v-for="plugin in PLUGIN_TYPES" :key="plugin" :value="plugin">
                  {{ plugin }}
                </a-option>
              </a-select>
            </a-form-item>
            <div v-if="needsLocalAddr" class="form-grid">
              <a-form-item label="本地服务地址">
                <a-input v-model="form.pluginConfig.localAddr" placeholder="127.0.0.1:8080" />
              </a-form-item>
              <a-form-item v-if="supportsHostRewrite" label="Host 重写">
                <a-input v-model="form.pluginConfig.hostHeaderRewrite" placeholder="留空保持原 Host" />
              </a-form-item>
            </div>
            <a-form-item v-if="supportsHostRewrite" label="请求头操作">
              <a-textarea
                v-model="pluginRequestHeadersText"
                :auto-size="{ minRows: 3, maxRows: 6 }"
                placeholder="X-Header: value"
              />
            </a-form-item>
          </div>

          <div v-if="usesCertificate" class="form-grid">
            <a-form-item label="证书来源" class="span-2">
              <a-switch
                :model-value="form.certificateMode === 'managed'"
                @change="setCertificateMode"
              >
                <template #checked>ACME 托管证书</template>
                <template #unchecked>手动证书配置</template>
              </a-switch>
            </a-form-item>
            <a-form-item
              v-if="form.certificateMode === 'managed'"
              label="可用证书"
              class="span-2"
              :rules="[{ required: true, message: '请选择可用证书' }]"
            >
              <a-select
                :model-value="selectedCertificateID"
                placeholder="选择当前可用证书"
                allow-clear
                @change="selectCertificate"
              >
                <a-option
                  v-for="certificate in certificates"
                  :key="certificate.id"
                  :value="certificate.id"
                >
                  {{ certificate.name }} · {{ certificate.domains.join('、') }}
                </a-option>
              </a-select>
              <template #extra>
                仅显示证书和私钥可读取、未过期且密钥匹配的 ACME 证书。
              </template>
            </a-form-item>
            <div
              v-if="form.certificateMode === 'managed' && selectedCertificate"
              class="certificate-summary span-2"
            >
              <span>到期时间：{{ formatCertificateDate(selectedCertificate.notAfter) }}</span>
              <span>剩余 {{ selectedCertificate.daysRemaining }} 天</span>
              <a-tag v-if="selectedCertificate.expiringSoon" color="orange">
                即将到期
              </a-tag>
              <a-tag v-if="selectedCertificate.renewalStatus === 'error'" color="red">
                续期失败，当前证书仍可用
              </a-tag>
            </div>
            <a-alert
              v-if="form.certificateMode === 'managed' && !certificates.length"
              class="span-2"
              type="warning"
              :show-icon="true"
            >
              暂无可用 ACME 证书，请先到证书页面完成 DNS-01 签发，或切换为手动证书配置。
            </a-alert>
            <a-alert
              v-if="customDomainMismatches.length"
              class="span-2"
              type="error"
              :show-icon="true"
            >
              证书未覆盖：{{ customDomainMismatches.join('、') }}。保存时后端将拒绝该配置。
            </a-alert>
            <a-alert
              v-if="form.certificateMode === 'managed' && form.subdomain"
              class="span-2"
              type="warning"
              :show-icon="true"
            >
              subdomain 的最终域名还取决于 frps.subDomainHost，请确认证书覆盖对应完整域名。
            </a-alert>
            <a-form-item v-if="form.certificateMode === 'manual'" label="证书路径">
              <a-input v-model="form.pluginConfig.crtPath" placeholder="/var/lib/frp/acme/.../cert.pem" />
            </a-form-item>
            <a-form-item v-if="form.certificateMode === 'manual'" label="私钥路径">
              <a-input v-model="form.pluginConfig.keyPath" placeholder="/var/lib/frp/acme/.../key.pem" />
            </a-form-item>
            <a-form-item
              v-if="['https2http', 'https2https'].includes(form.pluginType)"
              label="启用 HTTP/2"
            >
              <a-switch v-model="form.pluginConfig.enableHTTP2" />
            </a-form-item>
          </div>

          <div v-if="pluginFields.length" class="form-grid">
            <a-form-item
              v-for="field in pluginFields"
              :key="field.key"
              :label="field.label"
            >
              <a-switch
                v-if="field.type === 'bool'"
                v-model="form.pluginConfig[field.key]"
              />
              <a-input-password
                v-else-if="field.type === 'password'"
                v-model="form.pluginConfig[field.key]"
              />
              <a-input
                v-else
                v-model="form.pluginConfig[field.key]"
              />
            </a-form-item>
          </div>
        </template>
      </section>

      <section v-if="form.type === 'tcp' || form.type === 'udp'" class="section-card">
        <div class="section-title">远程端口</div>
        <div class="form-grid">
          <a-form-item label="远程端口">
            <a-input-number v-model="form.remotePort" :min="0" :max="65535" class="full-width" />
            <template #extra>0 表示由 FRPS 随机分配。</template>
          </a-form-item>
        </div>
      </section>

      <section
        v-if="['http', 'https', 'tcpmux'].includes(form.type)"
        class="section-card"
      >
        <div class="section-title">域名与路由</div>
        <div class="form-grid">
          <a-form-item label="自定义域名">
            <a-input v-model="domainsText" placeholder="example.com, www.example.com" />
          </a-form-item>
          <a-form-item v-if="form.type !== 'tcpmux'" label="子域名">
            <a-input v-model="form.subdomain" placeholder="test" />
          </a-form-item>
          <a-form-item v-else label="复用器">
            <a-select v-model="form.multiplexer">
              <a-option value="httpconnect">HTTP CONNECT</a-option>
            </a-select>
          </a-form-item>
        </div>
      </section>

      <section
        v-if="['http', 'tcpmux', 'stcp', 'sudp', 'xtcp'].includes(form.type)"
        class="section-card"
      >
        <div class="section-title">访问认证</div>
        <div v-if="['http', 'tcpmux'].includes(form.type)" class="form-grid three">
          <a-form-item label="HTTP 用户">
            <a-input v-model="form.httpUser" />
          </a-form-item>
          <a-form-item label="HTTP 密码">
            <a-input-password v-model="form.httpPassword" />
          </a-form-item>
          <a-form-item label="按用户路由">
            <a-input v-model="form.routeByHTTPUser" />
          </a-form-item>
        </div>
        <div v-else class="form-grid">
          <a-form-item label="Secret Key">
            <a-input-password v-model="form.secretKey" />
          </a-form-item>
          <a-form-item label="允许用户">
            <a-input v-model="allowUsersText" placeholder="user1, user2, *" />
          </a-form-item>
        </div>
      </section>
    </a-form>

    <template #footer>
      <div class="editor-footer">
        <a-button @click="advancedVisible = true">
          <template #icon><IconSettings /></template>
          高级设置
        </a-button>
        <div>
          <a-button @click="requestClose">取消</a-button>
          <a-button type="primary" :loading="saving" @click="save">
            保存代理
          </a-button>
        </div>
      </div>
    </template>
  </a-drawer>

  <a-drawer
    v-model:visible="advancedVisible"
    title="高级设置"
    :width="620"
    unmount-on-close
  >
    <a-form :model="form" layout="vertical">
      <section class="section-card">
        <div class="section-title">传输</div>
        <div class="form-grid three">
          <a-form-item label="加密">
            <a-switch v-model="form.useEncryption" />
          </a-form-item>
          <a-form-item label="压缩">
            <a-switch v-model="form.useCompression" />
          </a-form-item>
          <a-form-item label="Proxy Protocol">
            <a-select v-model="form.proxyProtocolVersion" allow-clear>
              <a-option value="v1">v1</a-option>
              <a-option value="v2">v2</a-option>
            </a-select>
          </a-form-item>
          <a-form-item label="带宽限制">
            <a-input v-model="form.bandwidthLimit" placeholder="1MB" />
          </a-form-item>
          <a-form-item label="限速位置">
            <a-select v-model="form.bandwidthLimitMode">
              <a-option value="client">客户端</a-option>
              <a-option value="server">服务端</a-option>
            </a-select>
          </a-form-item>
        </div>
      </section>

      <section class="section-card">
        <div class="section-title">健康检查</div>
        <div class="form-grid">
          <a-form-item label="检查类型">
            <a-select v-model="form.healthCheckType" allow-clear>
              <a-option value="tcp">TCP</a-option>
              <a-option value="http">HTTP</a-option>
            </a-select>
          </a-form-item>
          <a-form-item label="检查路径">
            <a-input v-model="form.healthCheckPath" placeholder="/health" />
          </a-form-item>
          <a-form-item label="超时秒数">
            <a-input-number v-model="form.healthCheckTimeoutSeconds" class="full-width" />
          </a-form-item>
          <a-form-item label="失败次数">
            <a-input-number v-model="form.healthCheckMaxFailed" class="full-width" />
          </a-form-item>
          <a-form-item label="检查间隔">
            <a-input-number v-model="form.healthCheckIntervalSeconds" class="full-width" />
          </a-form-item>
          <a-form-item
            v-if="form.healthCheckType === 'http'"
            label="健康检查请求头"
            class="span-2"
          >
            <a-textarea
              v-model="healthHeadersText"
              :auto-size="{ minRows: 2, maxRows: 5 }"
              placeholder="X-Header: value"
            />
          </a-form-item>
        </div>
      </section>

      <section class="section-card">
        <div class="section-title">负载均衡</div>
        <div class="form-grid">
          <a-form-item label="分组">
            <a-input v-model="form.loadBalancerGroup" />
          </a-form-item>
          <a-form-item label="分组密钥">
            <a-input-password v-model="form.loadBalancerGroupKey" />
          </a-form-item>
        </div>
      </section>

      <section v-if="form.type === 'http'" class="section-card">
        <div class="section-title">HTTP 路由</div>
        <div class="form-grid">
          <a-form-item label="Locations">
            <a-input v-model="locationsText" placeholder="/api, /static" />
          </a-form-item>
          <a-form-item label="Host Header 重写">
            <a-input v-model="form.hostHeaderRewrite" />
          </a-form-item>
          <a-form-item label="请求头" class="span-2">
            <a-textarea
              v-model="requestHeadersText"
              :auto-size="{ minRows: 3, maxRows: 6 }"
              placeholder="X-Header: value"
            />
          </a-form-item>
          <a-form-item label="响应头" class="span-2">
            <a-textarea
              v-model="responseHeadersText"
              :auto-size="{ minRows: 3, maxRows: 6 }"
              placeholder="X-Header: value"
            />
          </a-form-item>
        </div>
      </section>

      <section class="section-card">
        <div class="section-title">元数据</div>
        <div class="form-grid">
          <a-form-item label="Metadata">
            <a-textarea
              v-model="metadataText"
              :auto-size="{ minRows: 3, maxRows: 7 }"
              placeholder="key=value"
            />
          </a-form-item>
          <a-form-item label="Annotations">
            <a-textarea
              v-model="annotationsText"
              :auto-size="{ minRows: 3, maxRows: 7 }"
              placeholder="key=value"
            />
          </a-form-item>
        </div>
      </section>

      <section v-if="form.type === 'xtcp'" class="section-card">
        <div class="section-title">NAT 穿透</div>
        <a-form-item label="禁用辅助地址">
          <a-switch v-model="form.natTraversalDisableAssistedAddrs" />
        </a-form-item>
      </section>
    </a-form>
  </a-drawer>

  <a-modal
    v-model:visible="exampleVisible"
    :title="`${form.type.toUpperCase()} 代理配置示例`"
    :footer="false"
    :width="760"
  >
    <p class="example-hint">
      示例包含当前代理类型的常用字段。保存到 Store 后，完整配置会由服务端校验。
    </p>
    <pre class="code-block">{{ currentExample }}</pre>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { Message, Modal } from '@arco-design/web-vue'
import { IconBook, IconSettings } from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  legacyRequest,
  type AvailableCertificate,
} from '@shared/admin'
import {
  PLUGIN_TYPES,
  PROXY_TYPES,
  createDefaultProxyForm,
  formToStoreProxy,
  storeProxyToForm,
  type ProxyDefinition,
  type ProxyFormData,
} from '../types'

const props = defineProps<{
  visible: boolean
  initial?: ProxyDefinition | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const formRef = ref()
const saving = ref(false)
const advancedVisible = ref(false)
const exampleVisible = ref(false)
const dirty = ref(false)
const hydrating = ref(false)
const compactViewport = useMediaQuery('(max-width: 900px)')
const drawerWidth = computed(() => (compactViewport.value ? '100%' : 860))
const backendMode = ref<'direct' | 'plugin'>('direct')
const form = reactive<ProxyFormData>(createDefaultProxyForm())
const certificates = ref<AvailableCertificate[]>([])
const domainsText = ref('')
const allowUsersText = ref('')
const locationsText = ref('')
const requestHeadersText = ref('')
const responseHeadersText = ref('')
const pluginRequestHeadersText = ref('')
const metadataText = ref('')
const annotationsText = ref('')
const healthHeadersText = ref('')

const editing = computed(() => Boolean(props.initial?.name))
const needsLocalAddr = computed(() =>
  ['http2https', 'https2http', 'https2https', 'http2http', 'tls2raw'].includes(
    form.pluginType,
  ),
)
const supportsHostRewrite = computed(() =>
  ['http2https', 'https2http', 'https2https', 'http2http'].includes(
    form.pluginType,
  ),
)
const usesCertificate = computed(() =>
  ['https2http', 'https2https', 'tls2raw'].includes(form.pluginType),
)
const selectedCertificateID = computed({
  get: () => form.certificateId,
  set: (value: string) => {
    form.certificateId = value
  },
})
const selectedCertificate = computed(() =>
  certificates.value.find((item) => item.id === form.certificateId),
)
const customDomainMismatches = computed(() => {
  if (
    !usesCertificate.value ||
    form.certificateMode !== 'managed' ||
    !selectedCertificate.value
  ) {
    return []
  }
  return form.customDomains.filter(
    (domain) =>
      !selectedCertificate.value?.domains.some((pattern) =>
        certificateCoversHost(pattern, domain),
      ),
  )
})
const pluginFields = computed(() => {
  if (form.pluginType === 'socks5') {
    return [
      { key: 'username', label: 'SOCKS5 用户名', type: 'text' },
      { key: 'password', label: 'SOCKS5 密码', type: 'password' },
    ]
  }
  if (form.pluginType === 'http_proxy') {
    return [
      { key: 'httpUser', label: '代理用户名', type: 'text' },
      { key: 'httpPassword', label: '代理密码', type: 'password' },
    ]
  }
  if (form.pluginType === 'static_file') {
    return [
      { key: 'localPath', label: '静态文件目录', type: 'text' },
      { key: 'stripPrefix', label: '移除路径前缀', type: 'text' },
      { key: 'httpUser', label: 'HTTP 用户名', type: 'text' },
      { key: 'httpPassword', label: 'HTTP 密码', type: 'password' },
    ]
  }
  if (form.pluginType === 'unix_domain_socket') {
    return [{ key: 'unixPath', label: 'Socket 路径', type: 'text' }]
  }
  return []
})
const currentExample = computed(() => proxyExamples[form.type] || '')

watch(
  () => props.visible,
  async (visible) => {
    if (!visible) return
    hydrating.value = true
    dirty.value = false
    certificates.value = await adminAPI.availableCertificates()
    const data = props.initial
      ? storeProxyToForm(props.initial)
      : createDefaultProxyForm()
    Object.assign(form, JSON.parse(JSON.stringify(data)))
    backendMode.value = form.pluginType ? 'plugin' : 'direct'
    hydrateTextFields()
    await nextTick()
    dirty.value = false
    hydrating.value = false
  },
)

watch(
  () => form,
  () => {
    if (props.visible && !hydrating.value) dirty.value = true
  },
  { deep: true },
)

function hydrateTextFields() {
  domainsText.value = form.customDomains.join(', ')
  allowUsersText.value = form.allowUsers.join(', ')
  locationsText.value = form.locations.join(', ')
  requestHeadersText.value = headersToText(form.requestHeaders)
  responseHeadersText.value = headersToText(form.responseHeaders)
  pluginRequestHeadersText.value = headersToText(
    Object.entries(form.pluginConfig?.requestHeaders?.set || {}).map(
      ([key, value]) => ({ key, value: String(value) }),
    ),
  )
  metadataText.value = recordsToText(form.metadatas)
  annotationsText.value = recordsToText(form.annotations)
  healthHeadersText.value = headersToText(
    form.healthCheckHTTPHeaders.map((header) => ({
      key: header.name,
      value: header.value,
    })),
  )
}

async function save() {
  const errors = await formRef.value?.validate()
  if (errors) return
  if (backendMode.value === 'direct' && !form.localPort) {
    Message.error('请输入本地端口')
    return
  }
  if (backendMode.value === 'plugin' && !form.pluginType) {
    Message.error('请选择客户端插件')
    return
  }
  if (
    usesCertificate.value &&
    form.certificateMode === 'managed' &&
    !form.certificateId
  ) {
    Message.error('请选择可用的 ACME 证书')
    return
  }
  if (customDomainMismatches.value.length) {
    Message.error(`证书未覆盖：${customDomainMismatches.value.join('、')}`)
    return
  }
  syncCollections()
  saving.value = true
  try {
    const payload = formToStoreProxy(form)
    if (editing.value) {
      await legacyRequest(`/api/store/proxies/${encodeURIComponent(form.name)}`, {
        method: 'PUT',
        body: JSON.stringify(payload),
      })
    } else {
      await legacyRequest('/api/store/proxies', {
        method: 'POST',
        body: JSON.stringify(payload),
      })
    }
    dirty.value = false
    Message.success(editing.value ? '代理已更新' : '代理已创建')
    emit('saved')
    emit('close')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '保存代理失败')
  } finally {
    saving.value = false
  }
}

function syncCollections() {
  form.customDomains = splitList(domainsText.value)
  form.allowUsers = splitList(allowUsersText.value)
  form.locations = splitList(locationsText.value)
  form.requestHeaders = parseHeaderLines(requestHeadersText.value)
  form.responseHeaders = parseHeaderLines(responseHeadersText.value)
  if (supportsHostRewrite.value) {
    const headers = parseHeaderLines(pluginRequestHeadersText.value)
    if (headers.length > 0) {
      form.pluginConfig.requestHeaders = {
        set: Object.fromEntries(
          headers.map((header) => [header.key, header.value]),
        ),
      }
    } else {
      delete form.pluginConfig.requestHeaders
    }
  }
  form.metadatas = parseRecordLines(metadataText.value)
  form.annotations = parseRecordLines(annotationsText.value)
  form.healthCheckHTTPHeaders = parseHeaderLines(healthHeadersText.value).map(
    (header) => ({ name: header.key, value: header.value }),
  )
}

function changeType() {
  const defaults = createDefaultProxyForm()
  defaults.name = form.name
  defaults.type = form.type
  defaults.enabled = form.enabled
  Object.assign(form, defaults)
  backendMode.value = 'direct'
  hydrateTextFields()
}

function resetPluginConfig() {
  form.pluginConfig = {}
}

function selectCertificate(
  value:
    | string
    | number
    | boolean
    | Record<string, any>
    | Array<string | number | boolean | Record<string, any>>,
) {
  const certificate = certificates.value.find((item) => item.id === value)
  if (!certificate) {
    form.certificateId = ''
    return
  }
  form.certificateMode = 'managed'
  form.certificateId = certificate.id
  delete form.pluginConfig.crtPath
  delete form.pluginConfig.keyPath
}

function setCertificateMode(value: string | number | boolean) {
  form.certificateMode = value ? 'managed' : 'manual'
  if (form.certificateMode === 'manual') {
    form.certificateId = ''
  }
}

function certificateCoversHost(pattern: string, host: string) {
  const normalizedPattern = pattern.toLowerCase().replace(/\.$/, '')
  const normalizedHost = host.toLowerCase().replace(/\.$/, '')
  if (!normalizedPattern.startsWith('*.')) {
    return normalizedPattern === normalizedHost
  }
  const suffix = normalizedPattern.slice(1)
  return (
    normalizedHost.endsWith(suffix) &&
    normalizedHost.split('.').length === normalizedPattern.split('.').length
  )
}

function requestClose() {
  if (!dirty.value) {
    emit('close')
    return
  }
  Modal.confirm({
    title: '放弃未保存修改',
    content: '关闭后本次编辑内容会丢失。',
    okText: '放弃修改',
    okButtonProps: { status: 'danger' },
    onOk: () => emit('close'),
  })
}

function formatCertificateDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function splitList(value: string) {
  return value
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseHeaderLines(value: string) {
  return value
    .split('\n')
    .map((line) => line.split(/:(.*)/s))
    .filter((parts) => parts[0]?.trim() && parts[1]?.trim())
    .map((parts) => ({ key: parts[0].trim(), value: parts[1].trim() }))
}

function headersToText(headers: Array<{ key: string; value: string }>) {
  return headers.map((header) => `${header.key}: ${header.value}`).join('\n')
}

function parseRecordLines(value: string) {
  return value
    .split('\n')
    .map((line) => line.split(/=(.*)/s))
    .filter((parts) => parts[0]?.trim() && parts[1] !== undefined)
    .map((parts) => ({ key: parts[0].trim(), value: parts[1].trim() }))
}

function recordsToText(records: Array<{ key: string; value: string }>) {
  return records.map((record) => `${record.key}=${record.value}`).join('\n')
}

const proxyExamples: Record<string, string> = {
  tcp: `[[proxies]]
name = "ssh"
type = "tcp"
localIP = "host.docker.internal"
localPort = 22
remotePort = 6000`,
  udp: `[[proxies]]
name = "dns"
type = "udp"
localIP = "host.docker.internal"
localPort = 53
remotePort = 6000`,
  http: `[[proxies]]
name = "web"
type = "http"
localIP = "host.docker.internal"
localPort = 8080
customDomains = ["app.example.com"]
locations = ["/"]`,
  https: `[[proxies]]
name = "secure-web"
type = "https"
customDomains = ["app.example.com"]

[proxies.plugin]
type = "https2http"
localAddr = "host.docker.internal:8080"
crtPath = "/var/lib/frp/acme/certificates/example/cert.pem"
keyPath = "/var/lib/frp/acme/certificates/example/key.pem"
enableHTTP2 = true`,
  tcpmux: `[[proxies]]
name = "mux"
type = "tcpmux"
customDomains = ["mux.example.com"]
multiplexer = "httpconnect"
localIP = "host.docker.internal"
localPort = 22`,
  stcp: `[[proxies]]
name = "secret-ssh"
type = "stcp"
secretKey = "replace-with-a-secret"
localIP = "host.docker.internal"
localPort = 22
allowUsers = ["visitor-user"]`,
  xtcp: `[[proxies]]
name = "p2p-ssh"
type = "xtcp"
secretKey = "replace-with-a-secret"
localIP = "host.docker.internal"
localPort = 22`,
  sudp: `[[proxies]]
name = "secret-dns"
type = "sudp"
secretKey = "replace-with-a-secret"
localIP = "host.docker.internal"
localPort = 53`,
}
</script>

<style scoped>
.switch-field {
  display: flex;
  min-height: 32px;
  align-items: center;
  gap: 10px;
  color: var(--frp-muted-strong);
  font-size: 13px;
}

.backend-mode {
  margin-bottom: 18px;
}

.full-width {
  width: 100%;
}

.certificate-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 14px;
  border: 1px solid var(--frp-border);
  border-radius: 10px;
  padding: 10px 12px;
  background: var(--frp-surface-muted);
  color: var(--frp-muted-strong);
  font-size: 12px;
}

.editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.editor-footer > div {
  display: flex;
  gap: 10px;
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.example-hint {
  margin: 0 0 12px;
  color: var(--frp-muted);
}
</style>
