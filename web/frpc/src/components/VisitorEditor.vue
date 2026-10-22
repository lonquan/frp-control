<template>
  <a-drawer
    :visible="visible"
    :title="editing ? '编辑访问者' : '新建访问者'"
    :width="760"
    unmount-on-close
    @cancel="requestClose"
  >
    <a-form ref="formRef" :model="form" layout="vertical">
      <section class="section-card">
        <div class="section-title">基础信息</div>
        <div class="form-grid three">
          <a-form-item
            field="name"
            label="访问者名称"
            :rules="[{ required: true, message: '请输入访问者名称' }]"
          >
            <a-input v-model="form.name" :disabled="editing" placeholder="my-visitor" />
          </a-form-item>
          <a-form-item label="类型">
            <a-select v-model="form.type" :disabled="editing">
              <a-option v-for="type in VISITOR_TYPES" :key="type" :value="type">
                {{ type.toUpperCase() }}
              </a-option>
            </a-select>
          </a-form-item>
          <a-form-item label="启用">
            <div class="switch-field">
              <a-switch v-model="form.enabled" />
              <span>{{ form.enabled ? '已启用' : '已禁用' }}</span>
            </div>
          </a-form-item>
        </div>
      </section>

      <section class="section-card">
        <div class="section-title">连接设置</div>
        <div class="form-grid">
          <a-form-item label="Server 用户">
            <a-input v-model="form.serverUser" />
          </a-form-item>
          <a-form-item label="Server 代理名">
            <a-input v-model="form.serverName" />
          </a-form-item>
          <a-form-item label="Secret Key">
            <a-input-password v-model="form.secretKey" />
          </a-form-item>
          <a-form-item label="本地监听地址">
            <a-input v-model="form.bindAddr" placeholder="127.0.0.1" />
          </a-form-item>
          <a-form-item label="本地监听端口">
            <a-input-number v-model="form.bindPort" :min="-1" :max="65535" class="full-width" />
            <template #extra>-1 表示仅接收其他访问者重定向。</template>
          </a-form-item>
        </div>
      </section>

      <section v-if="['stcp', 'xtcp'].includes(form.type)" class="section-card">
        <div class="section-title">访问者插件</div>
        <div class="form-grid">
          <a-form-item label="插件">
            <a-select v-model="form.pluginType" allow-clear>
              <a-option value="virtual_net">virtual_net</a-option>
            </a-select>
          </a-form-item>
          <a-form-item v-if="form.pluginType === 'virtual_net'" label="目标 IP">
            <a-input v-model="form.pluginDestinationIP" placeholder="10.10.10.10" />
          </a-form-item>
        </div>
      </section>

      <section class="section-card">
        <div class="section-title">传输</div>
        <div class="form-grid">
          <a-form-item label="加密">
            <a-switch v-model="form.useEncryption" />
          </a-form-item>
          <a-form-item label="压缩">
            <a-switch v-model="form.useCompression" />
          </a-form-item>
        </div>
      </section>

      <section v-if="form.type === 'xtcp'" class="section-card">
        <div class="section-title">XTCP</div>
        <div class="form-grid three">
          <a-form-item label="协议">
            <a-select v-model="form.protocol">
              <a-option value="quic">QUIC</a-option>
              <a-option value="kcp">KCP</a-option>
            </a-select>
          </a-form-item>
          <a-form-item label="保持隧道">
            <a-switch v-model="form.keepTunnelOpen" />
          </a-form-item>
          <a-form-item label="回退代理">
            <a-input v-model="form.fallbackTo" />
          </a-form-item>
          <a-form-item label="每小时最大重试">
            <a-input-number v-model="form.maxRetriesAnHour" class="full-width" />
          </a-form-item>
          <a-form-item label="最小重试间隔">
            <a-input-number v-model="form.minRetryInterval" class="full-width" />
          </a-form-item>
          <a-form-item label="回退超时">
            <a-input-number v-model="form.fallbackTimeoutMs" class="full-width" />
          </a-form-item>
          <a-form-item label="禁用辅助地址" class="span-3">
            <a-switch v-model="form.natTraversalDisableAssistedAddrs" />
          </a-form-item>
        </div>
      </section>
    </a-form>
    <template #footer>
      <a-space>
        <a-button @click="requestClose">取消</a-button>
        <a-button type="primary" :loading="saving" @click="save">保存访问者</a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { legacyRequest } from '@shared/admin'
import {
  VISITOR_TYPES,
  createDefaultVisitorForm,
  formToStoreVisitor,
  type VisitorDefinition,
  type VisitorFormData,
} from '../types'

const props = defineProps<{
  visible: boolean
  initial?: VisitorDefinition | null
}>()
const emit = defineEmits<{ close: []; saved: [] }>()
const formRef = ref()
const saving = ref(false)
const dirty = ref(false)
const hydrating = ref(false)
const form = reactive<VisitorFormData>(createDefaultVisitorForm())
const editing = computed(() => Boolean(props.initial?.name))

watch(
  () => props.visible,
  async (visible) => {
    if (!visible) return
    hydrating.value = true
    Object.assign(
      form,
      props.initial
        ? storeVisitorToForm(props.initial)
        : createDefaultVisitorForm(),
    )
    dirty.value = false
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

async function save() {
  const errors = await formRef.value?.validate()
  if (errors) return
  saving.value = true
  try {
    const payload = formToStoreVisitor(form)
    if (editing.value) {
      await legacyRequest(`/api/store/visitors/${encodeURIComponent(form.name)}`, {
        method: 'PUT',
        body: JSON.stringify(payload),
      })
    } else {
      await legacyRequest('/api/store/visitors', {
        method: 'POST',
        body: JSON.stringify(payload),
      })
    }
    dirty.value = false
    Message.success(editing.value ? '访问者已更新' : '访问者已创建')
    emit('saved')
    emit('close')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '保存访问者失败')
  } finally {
    saving.value = false
  }
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

function storeVisitorToForm(visitor: VisitorDefinition): VisitorFormData {
  const formData = createDefaultVisitorForm()
  const block = ((visitor as any)[visitor.type] || {}) as Record<string, any>
  formData.name = visitor.name
  formData.type = visitor.type
  formData.enabled = block.enabled !== false
  formData.useEncryption = Boolean(block.transport?.useEncryption)
  formData.useCompression = Boolean(block.transport?.useCompression)
  formData.secretKey = block.secretKey || ''
  formData.serverUser = block.serverUser || ''
  formData.serverName = block.serverName || ''
  formData.bindAddr = block.bindAddr || '127.0.0.1'
  formData.bindPort = block.bindPort
  formData.pluginType = block.plugin?.type || ''
  formData.pluginDestinationIP = block.plugin?.destinationIP || ''
  formData.protocol = block.protocol || 'quic'
  formData.keepTunnelOpen = Boolean(block.keepTunnelOpen)
  formData.maxRetriesAnHour = block.maxRetriesAnHour
  formData.minRetryInterval = block.minRetryInterval
  formData.fallbackTo = block.fallbackTo || ''
  formData.fallbackTimeoutMs = block.fallbackTimeoutMs
  formData.natTraversalDisableAssistedAddrs = Boolean(
    block.natTraversal?.disableAssistedAddrs,
  )
  return formData
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

.full-width {
  width: 100%;
}
</style>
