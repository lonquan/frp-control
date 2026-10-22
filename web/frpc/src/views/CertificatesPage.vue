<template>
  <div class="page-stack">
    <PageHeader
      title="ACME 证书"
      description="统一使用客户端配置的 DNS provider 签发证书，自动续期并触发现有 TLS 插件重载。"
      eyebrow="CERTIFICATES"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-button @click="openDNSSettings">
        <template #icon><IconSettings /></template>
        DNS 设置
      </a-button>
      <a-button
        :loading="renewingAll"
        :disabled="!dnsProviderConfig.configured || !records.length"
        @click="renewAll"
      >
        全部续期
      </a-button>
      <a-button
        type="primary"
        :disabled="!dnsProviderConfig.configured"
        @click="openCreate"
      >
        <template #icon><IconPlus /></template>
        新建证书
      </a-button>
    </PageHeader>

    <a-alert v-if="!dnsProviderConfig.configured" type="warning" show-icon>
      尚未配置 DNS provider。请先打开“DNS 设置”保存 provider 和凭据。
    </a-alert>
    <a-alert v-else type="info" show-icon>
      当前 DNS provider：{{
        dnsProviderConfig.providerName || dnsProviderConfig.provider
      }}
    </a-alert>

    <a-form
      :model="filters"
      class="filter-panel"
      layout="vertical"
      @submit="load"
    >
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input v-model="filters.q" placeholder="证书名称或域名" allow-clear>
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model="filters.status" allow-clear placeholder="全部状态">
            <a-option value="valid">有效</a-option>
            <a-option value="expired">已过期</a-option>
            <a-option value="issuing">签发中</a-option>
            <a-option value="pending">待签发</a-option>
            <a-option value="error">失败</a-option>
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
          <strong>托管证书</strong>
          <span>共 {{ filtered.length }} 个</span>
        </div>
      </div>
      <a-table
        :data="filtered"
        :loading="loading"
        :pagination="false"
        row-key="id"
        size="large"
        :scroll="{ x: 1200 }"
        @row-click="openDetail"
      >
        <template #columns>
          <a-table-column title="证书" :width="220">
            <template #cell="{ record }">
              <div class="row-primary">
                <strong>{{ record.name }}</strong>
                <span>{{ record.domains.join('、') }}</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="状态" :width="110">
            <template #cell="{ record }">
              <StatusBadge :status="record.status" />
            </template>
          </a-table-column>
          <a-table-column title="DNS 提供商" :width="160">
            <template #cell="{ record }">
              {{ providerLabel(record.provider) }}
            </template>
          </a-table-column>
          <a-table-column title="到期时间" :width="160">
            <template #cell="{ record }">{{
              formatDate(record.notAfter)
            }}</template>
          </a-table-column>
          <a-table-column title="最近签发" :width="160">
            <template #cell="{ record }">{{
              formatDate(record.lastIssueAt)
            }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="220" fixed="right">
            <template #cell="{ record }">
              <div class="table-actions">
                <a-button
                  type="text"
                  :loading="record.status === 'issuing'"
                  :disabled="
                    !dnsProviderConfig.configured || record.status === 'issuing'
                  "
                  @click.stop="renew(record)"
                >
                  续期
                </a-button>
                <a-button
                  type="text"
                  :disabled="!record.downloadable"
                  @click.stop="download(record)"
                >
                  下载
                </a-button>
                <a-popconfirm
                  content="确认删除此证书配置？证书文件会同时删除。"
                  @ok="remove(record)"
                >
                  <a-button type="text" status="danger" @click.stop
                    >删除</a-button
                  >
                </a-popconfirm>
              </div>
            </template>
          </a-table-column>
        </template>
        <template #empty>
          <div class="empty-state">
            <div class="empty-icon"><IconSafe /></div>
            <strong>暂无托管证书</strong>
            <p>创建证书后将立即通过 DNS-01 签发。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:visible="editorVisible"
      title="新建证书"
      :width="720"
      unmount-on-close
    >
      <a-form ref="formRef" :model="form" layout="vertical">
        <div class="form-grid">
          <a-form-item
            field="name"
            label="证书名称"
            :rules="[{ required: true, message: '请输入证书名称' }]"
          >
            <a-input v-model="form.name" placeholder="例如：主站 HTTPS" />
          </a-form-item>
          <a-form-item
            field="email"
            label="ACME 账号邮箱"
            :rules="[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]"
          >
            <a-input v-model="form.email" placeholder="admin@example.com" />
          </a-form-item>
          <a-form-item
            field="domainsText"
            label="域名"
            class="span-2"
            :rules="[{ required: true, message: '请输入至少一个域名' }]"
          >
            <a-input
              v-model="form.domainsText"
              placeholder="example.com, *.example.com"
            />
            <template #extra
              >多个域名使用逗号或换行分隔；证书创建后不可修改。</template
            >
          </a-form-item>
          <a-form-item label="验证方式">
            <a-tag color="purple">DNS-01</a-tag>
          </a-form-item>
          <a-form-item label="密钥类型">
            <a-select v-model="form.keyType">
              <a-option value="rsa2048">RSA 2048</a-option>
              <a-option value="rsa4096">RSA 4096</a-option>
              <a-option value="ec256">ECDSA P-256</a-option>
              <a-option value="ec384">ECDSA P-384</a-option>
            </a-select>
          </a-form-item>
          <a-form-item label="DNS 提供商" class="span-2">
            <div class="provider-readonly">
              <strong>{{
                dnsProviderConfig.providerName || dnsProviderConfig.provider
              }}</strong>
              <a-button type="text" @click="openDNSSettings">修改设置</a-button>
            </div>
          </a-form-item>
          <a-form-item label="ACME CA 地址" class="span-2">
            <a-input
              v-model="form.caURL"
              placeholder="留空使用 Let's Encrypt 正式环境"
              allow-clear
            />
            <template #extra
              >测试时建议使用 Let's Encrypt
              staging，避免触发生产速率限制。</template
            >
          </a-form-item>
        </div>
      </a-form>
      <template #footer>
        <a-space>
          <a-button @click="editorVisible = false">取消</a-button>
          <a-button
            type="primary"
            :loading="saving"
            :disabled="!dnsProviderConfig.configured"
            @click="createCertificate"
          >
            创建并签发
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <a-drawer
      v-model:visible="dnsDrawerVisible"
      title="DNS 设置"
      :width="720"
      unmount-on-close
    >
      <a-alert type="info" show-icon class="drawer-alert">
        凭据仅保存在客户端数据目录，不会通过部署环境变量读取。所有证书的后续签发和续期统一使用这里的
        provider。
      </a-alert>
      <a-form :model="dnsForm" layout="vertical">
        <a-form-item label="DNS provider" required>
          <a-select
            v-model="dnsForm.provider"
            filterable
            placeholder="搜索 lego DNS provider"
            @change="changeDNSProvider"
          >
            <a-option
              v-for="provider in dnsProviders"
              :key="provider.code"
              :value="provider.code"
              :label="`${provider.name} (${provider.code})`"
            />
          </a-select>
        </a-form-item>
        <a-alert
          v-if="
            dnsProviderConfig.configured &&
            dnsForm.provider &&
            dnsForm.provider !== dnsProviderConfig.provider
          "
          type="warning"
          show-icon
          class="drawer-alert"
        >
          切换 provider 后，现有证书的后续续期也会使用新
          provider。请确保域名由新 provider 管理。
        </a-alert>
        <div v-if="selectedDNSProvider" class="credential-section">
          <div class="credential-heading">
            <strong>凭据</strong>
            <span>
              按厂商要求填写必要字段；已配置字段留空会保留原值。同一
              provider 更换认证方式时，请先清除配置再重新录入。
            </span>
          </div>
          <a-form-item
            v-for="field in selectedDNSProvider.fields || []"
            :key="field.name"
            :label="field.label || field.name"
          >
            <a-input-password
              v-model="dnsForm.credentials[field.name]"
              :placeholder="
                configuredField(field.name)
                  ? '已配置，留空保持不变'
                  : field.name
              "
              allow-clear
            />
            <template #extra>{{ field.name }}</template>
          </a-form-item>
          <a-empty
            v-if="!(selectedDNSProvider.fields || []).length"
            description="该 provider 没有可录入的凭据字段，将使用预检时的默认身份配置。"
          />
        </div>
      </a-form>
      <template #footer>
        <div class="drawer-footer">
          <a-button
            status="danger"
            :disabled="!dnsProviderConfig.configured"
            @click="clearDNSConfig"
          >
            清除配置
          </a-button>
          <a-space>
            <a-button @click="dnsDrawerVisible = false">取消</a-button>
            <a-button
              type="primary"
              :loading="savingDNS"
              :disabled="!dnsForm.provider"
              @click="saveDNSConfig"
            >
              保存并预检
            </a-button>
          </a-space>
        </div>
      </template>
    </a-drawer>

    <a-drawer
      v-model:visible="detailVisible"
      title="证书详情"
      :width="680"
      unmount-on-close
    >
      <div v-if="selected" class="detail-sections">
        <div class="detail-section">
          <h3>证书信息</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>名称</span>
              <strong>{{ selected.name }}</strong>
            </div>
            <div class="detail-item">
              <span>状态</span>
              <StatusBadge :status="selected.status" />
            </div>
            <div class="detail-item span-2">
              <span>域名</span>
              <strong>{{ selected.domains.join('、') }}</strong>
            </div>
            <div class="detail-item">
              <span>最近签发提供商</span>
              <strong>{{ providerLabel(selected.provider) }}</strong>
            </div>
            <div class="detail-item">
              <span>到期时间</span>
              <strong>{{ formatDate(selected.notAfter) }}</strong>
            </div>
            <div class="detail-item">
              <span>最近签发</span>
              <strong>{{ formatDate(selected.lastIssueAt) }}</strong>
            </div>
          </div>
        </div>
        <div v-if="selected.certPath" class="detail-section">
          <h3>文件路径</h3>
          <div class="detail-grid">
            <div class="detail-item">
              <span>证书</span>
              <strong class="mono">{{ selected.certPath }}</strong>
            </div>
            <div class="detail-item">
              <span>私钥</span>
              <strong class="mono">{{ selected.keyPath }}</strong>
            </div>
          </div>
        </div>
        <a-alert v-if="selected.lastError" type="error" :show-icon="false">
          {{ selected.lastError }}
        </a-alert>
      </div>
      <template #footer>
        <a-space>
          <a-button
            :disabled="!selected?.downloadable"
            @click="selected && download(selected)"
          >
            下载
          </a-button>
          <a-button
            type="primary"
            :disabled="
              !selected ||
              !dnsProviderConfig.configured ||
              selected.status === 'issuing'
            "
            @click="selected && renew(selected)"
          >
            续期
          </a-button>
        </a-space>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconPlus,
  IconRefresh,
  IconSafe,
  IconSearch,
  IconSettings,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  downloadBlob,
  type CertificateInput,
  type CertificateRecord,
  type DNSProviderConfig,
  type DNSProviderDefinition,
} from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'

const loading = ref(false)
const saving = ref(false)
const savingDNS = ref(false)
const renewingAll = ref(false)
const records = ref<CertificateRecord[]>([])
const dnsProviders = ref<DNSProviderDefinition[]>([])
const dnsProviderConfig = ref<DNSProviderConfig>({
  configured: false,
  credentials: [],
})
const filters = reactive({ q: '', status: '' })
const editorVisible = ref(false)
const dnsDrawerVisible = ref(false)
const detailVisible = ref(false)
const selected = ref<CertificateRecord>()
const formRef = ref()
const form = reactive({
  name: '',
  domainsText: '',
  email: '',
  caURL: '',
  keyType: 'rsa2048',
})
const dnsForm = reactive({
  provider: '',
  credentials: {} as Record<string, string>,
})

const selectedDNSProvider = computed(() =>
  dnsProviders.value.find((provider) => provider.code === dnsForm.provider),
)
const providerNames = computed(
  () =>
    new Map(
      dnsProviders.value.map((provider) => [provider.code, provider.name]),
    ),
)
const filtered = computed(() => {
  const q = filters.q.toLowerCase().trim()
  return records.value.filter((record) => {
    if (filters.status && record.status !== filters.status) return false
    if (!q) return true
    return (
      record.name.toLowerCase().includes(q) ||
      record.domains.some((domain) => domain.toLowerCase().includes(q))
    )
  })
})

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [certificates, config, providers] = await Promise.all([
      adminAPI.certificates(),
      adminAPI.dnsProviderConfig(),
      dnsProviders.value.length
        ? Promise.resolve(dnsProviders.value)
        : adminAPI.dnsProviders(),
    ])
    records.value = certificates
    dnsProviderConfig.value = config
    dnsProviders.value = providers
    if (selected.value) {
      selected.value = certificates.find(
        (item) => item.id === selected.value?.id,
      )
    }
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '加载证书失败')
  } finally {
    loading.value = false
  }
}

function reset() {
  filters.q = ''
  filters.status = ''
}

function openCreate() {
  if (!dnsProviderConfig.value.configured) {
    Message.warning('请先配置 DNS provider')
    openDNSSettings()
    return
  }
  Object.assign(form, {
    name: '',
    domainsText: '',
    email: '',
    caURL: '',
    keyType: 'rsa2048',
  })
  editorVisible.value = true
}

function openDNSSettings() {
  dnsForm.provider =
    dnsProviderConfig.value.provider ||
    dnsProviders.value.find((provider) => provider.code === 'cloudflare')
      ?.code ||
    dnsProviders.value[0]?.code ||
    ''
  dnsForm.credentials = {}
  dnsDrawerVisible.value = true
}

function changeDNSProvider() {
  dnsForm.credentials = {}
}

function configuredField(name: string) {
  return (
    dnsProviderConfig.value.provider === dnsForm.provider &&
    dnsProviderConfig.value.credentials.some(
      (field) => field.name === name && field.configured,
    )
  )
}

async function saveDNSConfig() {
  savingDNS.value = true
  try {
    const credentials = Object.fromEntries(
      Object.entries(dnsForm.credentials).filter(([, value]) => value !== ''),
    )
    dnsProviderConfig.value = await adminAPI.saveDNSProviderConfig(
      dnsForm.provider,
      credentials,
    )
    Message.success('DNS provider 配置已保存')
    dnsDrawerVisible.value = false
    await load()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : 'DNS 配置预检失败')
  } finally {
    savingDNS.value = false
  }
}

function clearDNSConfig() {
  Modal.confirm({
    title: '清除 DNS 配置',
    content: '清除后现有证书将无法继续签发或续期，直到重新配置。',
    onOk: async () => {
      try {
        await adminAPI.clearDNSProviderConfig()
        dnsProviderConfig.value = { configured: false, credentials: [] }
        Message.success('DNS provider 配置已清除')
      } catch (error) {
        Message.error(error instanceof Error ? error.message : '清除配置失败')
      }
    },
  })
}

async function createCertificate() {
  const errors = await formRef.value?.validate()
  if (errors) return
  saving.value = true
  const payload: CertificateInput = {
    name: form.name.trim(),
    domains: form.domainsText
      .split(/[,\n]/)
      .map((value) => value.trim())
      .filter(Boolean),
    email: form.email.trim(),
    caURL: form.caURL.trim(),
    keyType: form.keyType,
  }
  try {
    await adminAPI.createCertificate(payload)
    Message.success('证书创建成功，签发任务已启动')
    editorVisible.value = false
    await load()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '创建证书失败')
  } finally {
    saving.value = false
  }
}

function openDetail(record: unknown) {
  selected.value = record as CertificateRecord
  detailVisible.value = true
}

function renew(record: CertificateRecord) {
  const content =
    record.status === 'valid'
      ? `确认续期 ${record.domains.join('、')}？提前续期可能触发 Let's Encrypt 重复签发限制。`
      : `确认续期 ${record.domains.join('、')}？证书文件将被新签发的证书替换。`
  Modal.confirm({
    title: '续期',
    content,
    onOk: async () => {
      try {
        await adminAPI.renewCertificate(record.id)
        Message.success('续期任务已启动')
        await load()
      } catch (error) {
        Message.error(error instanceof Error ? error.message : '续期失败')
      }
    },
  })
}

function renewAll() {
  Modal.confirm({
    title: '全部续期',
    content: '将依次处理所有非签发中证书，包括待签发、有效、已过期和失败记录。',
    onOk: async () => {
      renewingAll.value = true
      try {
        const result = await adminAPI.renewAllCertificates()
        Message.success(
          `已接受 ${result.accepted} 个续期任务，跳过 ${result.skipped} 个`,
        )
        await load()
      } catch (error) {
        Message.error(error instanceof Error ? error.message : '全部续期失败')
      } finally {
        renewingAll.value = false
      }
    },
  })
}

async function download(record: CertificateRecord) {
  try {
    const blob = await adminAPI.downloadCertificate(record.id)
    downloadBlob(blob, `frp-cert-${record.id}.zip`)
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '下载证书失败')
  }
}

async function remove(record: CertificateRecord) {
  try {
    await adminAPI.deleteCertificate(record.id)
    Message.success('证书已删除')
    await load()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '删除失败')
  }
}

function providerLabel(provider?: string) {
  if (!provider) return '-'
  return providerNames.value.get(provider) || provider
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() <= 1) return '-'
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}
</script>

<style scoped>
.provider-readonly {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 32px;
  border: 1px solid var(--frp-border);
  border-radius: 6px;
  padding: 0 4px 0 12px;
}

.drawer-alert {
  margin-bottom: 18px;
}

.credential-section {
  border-top: 1px solid var(--frp-border);
  padding-top: 18px;
}

.credential-heading {
  display: grid;
  gap: 4px;
  margin-bottom: 16px;
}

.credential-heading span {
  color: var(--frp-muted);
  font-size: 12px;
}

.drawer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.detail-item.span-2 {
  grid-column: span 2;
}
</style>
