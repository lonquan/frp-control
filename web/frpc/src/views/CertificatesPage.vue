<template>
  <div class="page-stack">
    <PageHeader
      title="ACME 证书"
      description="通过 DNS-01 签发托管证书，自动续期并触发现有 TLS 插件重载。"
      eyebrow="CERTIFICATES"
    >
      <a-button :loading="loading" @click="load">
        <template #icon><IconRefresh /></template>
        刷新
      </a-button>
      <a-button type="primary" @click="openCreate">
        <template #icon><IconPlus /></template>
        新建证书
      </a-button>
    </PageHeader>

    <a-form :model="filters" class="filter-panel" layout="vertical" @submit="load">
      <div class="filter-grid">
        <a-form-item label="搜索">
          <a-input v-model="filters.q" placeholder="证书名称或域名" allow-clear>
            <template #prefix><IconSearch /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model="filters.status" allow-clear placeholder="全部状态">
            <a-option value="valid">有效</a-option>
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
        <a-alert type="info" :show-icon="true" class="cert-alert">
          DNS 凭据通过容器环境变量或 secret 文件提供
        </a-alert>
      </div>
      <a-table
        :data="filtered"
        :loading="loading"
        :pagination="false"
        row-key="id"
        size="large"
        :scroll="{ x: 1050 }"
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
          <a-table-column title="DNS 提供商" :width="145">
            <template #cell="{ record }">
              {{ certificateProviderLabel(record.provider) }}
            </template>
          </a-table-column>
          <a-table-column title="到期时间" :width="190">
            <template #cell="{ record }">{{ formatDate(record.notAfter) }}</template>
          </a-table-column>
          <a-table-column title="最近签发" :width="190">
            <template #cell="{ record }">{{ formatDate(record.lastIssueAt) }}</template>
          </a-table-column>
          <a-table-column title="操作" :width="230" fixed="right">
            <template #cell="{ record }">
              <div class="table-actions">
                <a-button
                  type="text"
                  :loading="record.status === 'issuing'"
                  @click.stop="issue(record)"
                >
                  签发
                </a-button>
                <a-button type="text" @click.stop="openEdit(record)">编辑</a-button>
                <a-popconfirm
                  content="确认删除此证书配置？证书文件会同时删除。"
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
            <div class="empty-icon"><IconSafe /></div>
            <strong>暂无托管证书</strong>
            <p>创建证书后，将通过 DNS-01 自动签发。</p>
          </div>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:visible="editorVisible"
      :title="editing ? '编辑证书' : '新建证书'"
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
            <a-input v-model="form.domainsText" placeholder="example.com, *.example.com" />
            <template #extra>多个域名使用逗号或换行分隔；通配符只能使用 DNS-01。</template>
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
          <a-form-item
            field="provider"
            label="DNS 提供商"
            class="span-2"
            validate-trigger="blur"
            :rules="providerRules"
          >
            <div class="provider-field">
              <a-select
                :model-value="providerSelection"
                placeholder="选择常用 DNS 提供商"
                @change="selectProvider"
              >
                <a-option
                  v-for="provider in commonCertificateProviders"
                  :key="provider.value"
                  :value="provider.value"
                >
                  {{ provider.label }}
                </a-option>
                <a-option :value="customCertificateProviderValue">
                  自定义提供商…
                </a-option>
              </a-select>
              <a-input
                v-if="providerSelection === customCertificateProviderValue"
                ref="customProviderInput"
                v-model="form.provider"
                placeholder="输入 lego provider 名称，例如 cloudns"
                allow-clear
                @input="customProviderError = ''"
              />
              <p v-if="customProviderError" class="provider-field-error" role="alert">
                {{ customProviderError }}
              </p>
            </div>
            <template #extra>
              {{ providerHint }}
            </template>
          </a-form-item>
          <a-form-item label="ACME CA 地址" class="span-2">
            <a-input
              v-model="form.caURL"
              placeholder="留空使用 Let's Encrypt 正式环境"
              allow-clear
            />
            <template #extra>测试时建议使用 Let's Encrypt staging，避免触发生产速率限制。</template>
          </a-form-item>
        </div>
      </a-form>
      <template #footer>
        <a-space>
          <a-button @click="editorVisible = false">取消</a-button>
          <a-button :loading="saving" @click="save(false)">保存</a-button>
          <a-button type="primary" :loading="saving" @click="save(true)">
            保存并签发
          </a-button>
        </a-space>
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
              <span>DNS 提供商</span>
              <strong>{{ certificateProviderLabel(selected.provider) }}</strong>
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
        <a-alert v-if="selected.lastError" type="error" :show-icon="true">
          {{ selected.lastError }}
        </a-alert>
      </div>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconSafe,
  IconPlus,
  IconRefresh,
  IconSearch,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  type CertificateInput,
  type CertificateRecord,
} from '@shared/admin'
import PageHeader from '@shared/components/PageHeader.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'
import {
  certificateProviderLabel,
  certificateProviderSelection,
  commonCertificateProviders,
  customCertificateProviderValue,
  isCommonCertificateProvider,
} from '../types/certificate-provider'

const loading = ref(false)
const saving = ref(false)
const records = ref<CertificateRecord[]>([])
const filters = reactive({ q: '', status: '' })
const editorVisible = ref(false)
const detailVisible = ref(false)
const editing = ref(false)
const editingID = ref('')
const selected = ref<CertificateRecord>()
const formRef = ref()
const customProviderInput = ref<{ focus: () => void }>()
const customProviderError = ref('')
const form = reactive({
  name: '',
  domainsText: '',
  provider: 'cloudflare',
  email: '',
  caURL: '',
  keyType: 'rsa2048',
})

const providerSelection = computed(() =>
  certificateProviderSelection(form.provider),
)
const providerRules = computed(() =>
  providerSelection.value === customCertificateProviderValue
    ? []
    : [{ required: true, message: '请选择 DNS 提供商' }],
)
const selectedProvider = computed(() =>
  commonCertificateProviders.find(
    (option) => option.value === form.provider,
  ),
)
const providerHint = computed(() => {
  const provider = selectedProvider.value
  if (!provider) {
    return '输入固定版本 lego 支持的 provider 名称；未列出的提供商也可通过自定义项配置。'
  }
  if (provider.envHint) {
    return `当前 provider: ${provider.value}。请在 FRPC 容器环境变量中配置 ${provider.envHint}。`
  }
  return `当前 provider: ${provider.value}。请将 lego 对应凭据写入 FRPC 容器环境变量。`
})

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
    records.value = await adminAPI.certificates()
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
  editing.value = false
  editingID.value = ''
  Object.assign(form, {
    name: '',
    domainsText: '',
    provider: 'cloudflare',
    email: '',
    caURL: '',
    keyType: 'rsa2048',
  })
  editorVisible.value = true
}

function openEdit(record: CertificateRecord) {
  editing.value = true
  editingID.value = record.id
  Object.assign(form, {
    name: record.name,
    domainsText: record.domains.join(', '),
    provider: record.provider || '',
    email: record.email,
    caURL: record.caURL || '',
    keyType: record.keyType || 'rsa2048',
  })
  editorVisible.value = true
}

function openDetail(record: any) {
  selected.value = record as CertificateRecord
  detailVisible.value = true
}

async function selectProvider(value: unknown) {
  const provider = selectedValue(value)
  customProviderError.value = ''
  if (provider === customCertificateProviderValue) {
    if (isCommonCertificateProvider(form.provider)) {
      form.provider = ''
    }
    await nextTick()
    customProviderInput.value?.focus()
    return
  }
  form.provider = provider
}

function selectedValue(value: unknown): string {
  if (typeof value === 'string' || typeof value === 'number') {
    return String(value)
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false'
  }
  if (Array.isArray(value)) {
    return selectedValue(value[0])
  }
  if (value && typeof value === 'object' && 'value' in value) {
    return selectedValue((value as { value: unknown }).value)
  }
  return ''
}

async function save(issueNow: boolean) {
  const errors = await formRef.value?.validate()
  const provider = form.provider.trim()
  const providerMissing =
    providerSelection.value === customCertificateProviderValue && !provider
  if (providerMissing) {
    customProviderError.value = '请输入自定义 DNS 提供商名称'
    await nextTick()
    customProviderInput.value?.focus()
  }
  if (errors) return
  if (providerMissing) return
  saving.value = true
  const payload: CertificateInput = {
    name: form.name.trim(),
    domains: form.domainsText
      .split(/[,\n]/)
      .map((value) => value.trim())
      .filter(Boolean),
    provider,
    email: form.email.trim(),
    caURL: form.caURL.trim(),
    keyType: form.keyType,
    status: issueNow ? 'issue' : 'pending',
  }
  try {
    const record = editing.value
      ? await adminAPI.updateCertificate(editingID.value, payload)
      : await adminAPI.createCertificate(payload)
    if (issueNow && editing.value) {
      await adminAPI.issueCertificate(record.id)
    }
    Message.success(issueNow ? '证书签发任务已启动' : '证书配置已保存')
    editorVisible.value = false
    await load()
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '保存证书失败')
  } finally {
    saving.value = false
  }
}

function issue(record: CertificateRecord) {
  Modal.confirm({
    title: '签发证书',
    content: `确认开始签发 ${record.domains.join('、')}？DNS-01 需要提供商凭据已注入容器。`,
    onOk: async () => {
      await adminAPI.issueCertificate(record.id)
      Message.success('签发任务已启动')
      await load()
    },
  })
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

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() <= 1) return '-'
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}
</script>

<style scoped>
.cert-alert {
  width: fit-content;
}

.provider-field {
  display: grid;
  gap: 10px;
}

.provider-field-error {
  margin: 0;
  color: rgb(var(--danger-6));
  font-size: 12px;
}

.detail-item.span-2 {
  grid-column: span 2;
}
</style>
