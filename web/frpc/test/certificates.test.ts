import { cloneVNode, defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  certificates: vi.fn(),
  dnsProviderConfig: vi.fn(),
  dnsProviders: vi.fn(),
  createCertificate: vi.fn(),
  deleteCertificate: vi.fn(),
  renewCertificate: vi.fn(),
  renewAllCertificates: vi.fn(),
  downloadCertificate: vi.fn(),
  saveDNSProviderConfig: vi.fn(),
  clearDNSProviderConfig: vi.fn(),
}))
const ui = vi.hoisted(() => ({
  confirm: vi.fn(),
  warning: vi.fn(),
}))
const downloadBlob = vi.hoisted(() => vi.fn())

vi.mock('@shared/admin', () => ({
  adminAPI: api,
  downloadBlob,
}))

vi.mock('@arco-design/web-vue', () => ({
  Message: {
    success: vi.fn(),
    warning: vi.fn(),
    error: vi.fn(),
  },
  Modal: ui,
}))

import CertificatesPage from '../src/views/CertificatesPage.vue'

const SlotStub = defineComponent({
  template: '<div><slot /><slot name="extra" /></div>',
})

const DrawerStub = defineComponent({
  setup(_, { slots }) {
    return () =>
      h('section', [
        slots.default?.(),
        h('footer', slots.footer?.()),
      ])
  },
})

const ButtonStub = defineComponent({
  props: {
    disabled: Boolean,
    loading: Boolean,
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          'data-loading': props.loading ? 'true' : 'false',
          onClick: (event: Event) => emit('click', event),
        },
        slots.default?.(),
      )
  },
})

const PasswordInputStub = defineComponent({
  props: {
    modelValue: String,
    placeholder: String,
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.modelValue || '',
        placeholder: props.placeholder,
        onInput: (event: Event) =>
          emit('update:modelValue', (event.target as HTMLInputElement).value),
      })
  },
})

const TableColumnStub = defineComponent({
  props: {
    title: String,
    record: {
      type: Object,
      default: undefined,
    },
  },
  setup(props, { slots }) {
    return () => slots.cell?.({ record: props.record }) || slots.default?.()
  },
})

const TableStub = defineComponent({
  props: {
    data: {
      type: Array,
      default: () => [],
    },
  },
  setup(props, { slots }) {
    const columns = slots.columns?.() || []
    return () =>
      h(
        'div',
        { class: 'table-stub' },
        props.data.map((record: any) =>
          h(
            'div',
            { class: 'record-row', key: record.id },
            columns.map((column, index) =>
              cloneVNode(column, { record, key: `${record.id}-${index}` }),
            ),
          ),
        ),
      )
  },
})

function mountPage() {
  return shallowMount(CertificatesPage, {
    global: {
      stubs: {
        PageHeader: SlotStub,
        StatusBadge: true,
        'a-button': ButtonStub,
        'a-alert': SlotStub,
        'a-form': SlotStub,
        'a-form-item': SlotStub,
        'a-space': SlotStub,
        'a-input': true,
        'a-input-password': PasswordInputStub,
        'a-select': SlotStub,
        'a-option': true,
        'a-tag': SlotStub,
        'a-table': TableStub,
        'a-table-column': TableColumnStub,
        'a-popconfirm': SlotStub,
        'a-drawer': DrawerStub,
        'a-empty': SlotStub,
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mountPage>, label: string) {
  const button = wrapper.findAll('button').find((item) => item.text() === label)
  if (!button) throw new Error(`button not found: ${label}`)
  return button
}

describe('client certificates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ui.confirm.mockImplementation(
      async ({ onOk }: { onOk?: () => unknown }) => {
        await onOk?.()
      },
    )
    api.certificates.mockResolvedValue([
      {
        id: 'cert-1',
        name: 'main certificate',
        domains: ['example.com'],
        provider: 'cloudflare',
        email: 'admin@example.com',
        keyType: 'rsa2048',
        status: 'valid',
        certPath: '/var/lib/frp/acme/certificates/cert-1/cert.pem',
        keyPath: '/var/lib/frp/acme/certificates/cert-1/key.pem',
        notAfter: '2027-01-01T00:00:00Z',
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        lastIssueAt: '2026-01-01T00:00:00Z',
        downloadable: true,
      },
    ])
    api.dnsProviderConfig.mockResolvedValue({
      configured: true,
      provider: 'cloudflare',
      providerName: 'Cloudflare',
      credentials: [],
    })
    api.dnsProviders.mockResolvedValue([
      {
        code: 'cloudflare',
        name: 'Cloudflare',
        fields: [{ name: 'CF_DNS_API_TOKEN', label: 'API token' }],
      },
    ])
    api.renewCertificate.mockResolvedValue({ accepted: true })
    api.renewAllCertificates.mockResolvedValue({ accepted: 1, skipped: 0 })
    api.downloadCertificate.mockResolvedValue(new Blob(['zip']))
    api.saveDNSProviderConfig.mockResolvedValue({
      configured: true,
      provider: 'cloudflare',
      providerName: 'Cloudflare',
      credentials: [
        {
          name: 'CF_DNS_API_TOKEN',
          label: 'API token',
          configured: true,
        },
      ],
    })
  })

  it('renders separate single and bulk renew actions', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(findButton(wrapper, '续期').exists()).toBe(true)
    expect(findButton(wrapper, '全部续期').exists()).toBe(true)

    await findButton(wrapper, '续期').trigger('click')
    await flushPromises()
    expect(api.renewCertificate).toHaveBeenCalledWith('cert-1')
    expect(api.renewAllCertificates).not.toHaveBeenCalled()

    await findButton(wrapper, '全部续期').trigger('click')
    await flushPromises()
    expect(api.renewAllCertificates).toHaveBeenCalledOnce()
  })

  it('downloads the generated ZIP without an extra request step', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await findButton(wrapper, '下载').trigger('click')
    await flushPromises()

    expect(api.downloadCertificate).toHaveBeenCalledWith('cert-1')
    expect(downloadBlob).toHaveBeenCalledOnce()
    expect(downloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      'frp-cert-cert-1.zip',
    )
  })

  it('saves only newly entered DNS credential values', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await findButton(wrapper, 'DNS 设置').trigger('click')
    await flushPromises()
    const input = wrapper.find('input[placeholder="CF_DNS_API_TOKEN"]')
    expect(input.exists()).toBe(true)
    await input.setValue('new-token')
    await findButton(wrapper, '保存并预检').trigger('click')
    await flushPromises()

    expect(api.saveDNSProviderConfig).toHaveBeenCalledWith('cloudflare', {
      CF_DNS_API_TOKEN: 'new-token',
    })
  })
})
