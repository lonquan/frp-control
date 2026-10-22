import { cloneVNode, defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  captureSettings: vi.fn(),
  saveCaptureSettings: vi.fn(),
  captures: vi.fn(),
  capture: vi.fn(),
  replayCaptures: vi.fn(),
  deleteCapture: vi.fn(),
  auditCaptureReveal: vi.fn(),
}))

const legacy = vi.hoisted(() => vi.fn())
const ui = vi.hoisted(() => ({
  confirm: vi.fn(),
}))

vi.mock('@shared/admin', () => ({
  adminAPI: api,
  legacyRequest: legacy,
}))

vi.mock('@arco-design/web-vue', () => ({
  Message: {
    success: vi.fn(),
    warning: vi.fn(),
    error: vi.fn(),
  },
  Modal: {
    confirm: ui.confirm,
  },
}))

import CapturesPage from '../src/views/CapturesPage.vue'
import StatusBadge from '@shared/components/StatusBadge.vue'

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

const ButtonStub = defineComponent({
  props: {
    disabled: Boolean,
    loading: Boolean,
  },
  emits: ['click'],
  template:
    '<button :disabled="disabled" :data-loading="loading ? \'true\' : \'false\'" @click="$emit(\'click\')"><slot /></button>',
})

const DrawerStub = defineComponent({
  props: {
    visible: Boolean,
    title: String,
  },
  template: `
    <section v-if="visible" class="drawer-stub">
      <h2>{{ title }}</h2>
      <slot />
      <footer><slot name="footer" /></footer>
    </section>
  `,
})

const TextareaStub = defineComponent({
  props: {
    modelValue: String,
  },
  emits: ['update:modelValue'],
  template:
    '<textarea :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
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
    return () => slots.cell?.({ record: props.record })
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
          h('div', { class: 'record-row', key: record.id },
            columns.map((column, index) =>
              cloneVNode(column, {
                record,
                key: `${record.id}-${index}`,
              }),
            ),
          ),
        ),
      )
  },
})

const DropdownStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', { class: 'dropdown-stub' }, [
      slots.default?.(),
      slots.content?.(),
    ])
  },
})

const DropdownOptionStub = defineComponent({
  props: {
    disabled: Boolean,
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          onClick: () => emit('click'),
        },
        slots.default?.(),
      )
  },
})

function mountPage() {
  return shallowMount(CapturesPage, {
    global: {
      stubs: {
        PageHeader: SlotStub,
        'router-link': SlotStub,
        'a-button': ButtonStub,
        'a-form': SlotStub,
        'a-form-item': SlotStub,
        'a-space': SlotStub,
        'a-input': true,
        'a-input-number': true,
        'a-select': SlotStub,
        'a-option': SlotStub,
        'a-switch': true,
        'a-textarea': TextareaStub,
        'a-alert': SlotStub,
        'a-tag': SlotStub,
        'a-dropdown': DropdownStub,
        'a-doption': DropdownOptionStub,
        'a-table': TableStub,
        'a-table-column': TableColumnStub,
        'a-empty': SlotStub,
        'a-drawer': DrawerStub,
        'a-tabs': SlotStub,
        'a-tab-pane': SlotStub,
      },
    },
  })
}

describe('capture page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ui.confirm.mockImplementation((options: { onOk?: () => unknown }) => options.onOk?.())
    legacy.mockResolvedValue({
      http: [{ name: 'web', type: 'http' }],
      tcp: [{ name: 'ssh', type: 'tcp' }],
    })
    api.captureSettings.mockResolvedValue({
      enabled: true,
      proxies: ['web'],
      excludePatterns: [],
      ttlSeconds: 86400,
      maxEntryBytes: 16 * 1024 * 1024,
      maxTotalBytes: 500 * 1024 * 1024,
    })
    api.captures.mockResolvedValue([
      {
        id: 'capture-1',
        proxy: 'web',
        protocol: 'http',
        method: 'GET',
        host: 'example.com',
        path: '/favicon.ico',
        sourceAddr: '203.0.113.10',
        sourcePort: 54321,
        responseStatus: 200,
        startedAt: '2026-10-22T09:00:00Z',
        durationMs: 12,
        truncated: false,
        webSocket: false,
        size: 42,
      },
    ])
    api.capture.mockResolvedValue({
      id: 'capture-1',
      proxy: 'web',
      protocol: 'http',
      method: 'GET',
      host: 'example.com',
      path: '/favicon.ico',
      scheme: 'http',
      query: '',
      sourceAddr: '203.0.113.10',
      sourcePort: 54321,
      responseStatus: 200,
      startedAt: '2026-10-22T09:00:00Z',
      durationMs: 12,
      truncated: false,
      webSocket: false,
      size: 42,
      target: '127.0.0.1:8080',
      responseBody: 'b2s=',
      replayResults: [],
    })
    api.replayCaptures.mockResolvedValue([
      {
        id: 'capture-1',
        result: {
          startedAt: '2026-10-22T09:01:00Z',
          status: 204,
          durationMs: 8,
        },
      },
    ])
    api.auditCaptureReveal.mockResolvedValue({ recorded: true })
    api.saveCaptureSettings.mockImplementation(async (value) => value)
  })

  it('keeps replay out of the header and uses a compact settings summary', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('刷新列表')
    expect(wrapper.text()).not.toContain('重放所选')
    expect(wrapper.findComponent(StatusBadge).props('label')).toBe('捕获已启用')
    expect(wrapper.text()).toContain('1 个代理')
    expect(wrapper.text()).toContain('0 条排除规则')
    expect(wrapper.text()).toContain('203.0.113.10:54321')
  })

  it('saves exclusion rules explicitly without reloading or restarting', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const configure = wrapper
      .findAll('button')
      .find((button) => button.text() === '配置')
    expect(configure).toBeDefined()
    await configure!.trigger('click')

    const textarea = wrapper.get('textarea')
    await textarea.setValue('favicon.ico\n/static/*\nfavicon.ico')
    expect(api.saveCaptureSettings).not.toHaveBeenCalled()

    const save = wrapper
      .findAll('button')
      .find((button) => button.text() === '保存设置')
    expect(save).toBeDefined()
    await save!.trigger('click')
    await flushPromises()

    expect(api.saveCaptureSettings).toHaveBeenCalledOnce()
    expect(api.saveCaptureSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enabled: true,
        proxies: ['web'],
        excludePatterns: ['favicon.ico', '/static/*'],
      }),
    )
    expect(wrapper.find('.drawer-stub').exists()).toBe(false)
  })

  it('refreshes only the list after replaying from a row action', async () => {
    const wrapper = mountPage()
    await flushPromises()
    api.captures.mockClear()

    const replay = wrapper
      .findAll('button')
      .find((button) => button.text() === '重放')
    expect(replay).toBeDefined()
    await replay!.trigger('click')
    await flushPromises()

    expect(api.replayCaptures).toHaveBeenCalledWith(['capture-1'], 1, 0)
    expect(api.captures).toHaveBeenCalledOnce()
    expect(api.capture).not.toHaveBeenCalled()
    expect(wrapper.find('.drawer-stub').exists()).toBe(false)
  })

  it('refreshes only the open detail and preserves sensitive reveal state', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const details = wrapper
      .findAll('button')
      .find((button) => button.text() === '详情')
    await details!.trigger('click')
    await flushPromises()
    expect(api.capture).toHaveBeenCalledTimes(1)
    api.captures.mockClear()
    api.capture.mockClear()

    const reveal = wrapper
      .findAll('button')
      .find((button) => button.text() === '显示敏感内容')
    await reveal!.trigger('click')
    await flushPromises()

    const replay = wrapper
      .findAll('button')
      .find((button) => button.text() === '重放请求')
    await replay!.trigger('click')
    await flushPromises()

    expect(api.replayCaptures).toHaveBeenCalledOnce()
    expect(api.capture).toHaveBeenCalledTimes(1)
    expect(api.captures).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('隐藏敏感内容')
    expect(wrapper.find('.drawer-stub').exists()).toBe(true)
  })
})
