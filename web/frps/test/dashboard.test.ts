import { defineComponent, h } from 'vue'
import { shallowMount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  metrics: vi.fn(),
  runtime: vi.fn(),
  diagnosticSettings: vi.fn(),
  supervisor: vi.fn(),
  restart: vi.fn(),
  saveDiagnosticSettings: vi.fn(),
  downloadGoroutine: vi.fn(),
  downloadHeap: vi.fn(),
  downloadCPU: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@shared/admin', () => ({
  adminAPI: api,
  downloadBlob: vi.fn(),
}))

import DashboardPage from '../src/views/DashboardPage.vue'

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

const ButtonStub = defineComponent({
  props: {
    loading: Boolean,
  },
  emits: ['click'],
  template:
    '<button :data-loading="loading ? \'true\' : \'false\'" @click="$emit(\'click\')"><slot /></button>',
})

const ModalStub = defineComponent({
  props: {
    visible: Boolean,
    onBeforeOk: Function,
  },
  emits: ['update:visible'],
  setup(props, { emit, slots }) {
    const confirm = async () => {
      const accepted = await props.onBeforeOk?.()
      if (accepted) emit('update:visible', false)
    }
    return () =>
      props.visible
        ? h('section', { class: 'modal-stub' }, [
            ...(slots.default?.() || []),
            h('button', { class: 'modal-confirm', onClick: confirm }, '确认重启'),
          ])
        : null
  },
})

function mountDashboard() {
  return shallowMount(DashboardPage, {
    global: {
      stubs: {
        PageHeader: SlotStub,
        'router-link': SlotStub,
        'a-button': ButtonStub,
        'a-switch': true,
        'a-alert': SlotStub,
        'a-tag': SlotStub,
        'a-empty': SlotStub,
        'a-modal': ModalStub,
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mountDashboard>, label: string) {
  const button = wrapper.findAll('button').find((item) => item.text() === label)
  if (!button) throw new Error(`button not found: ${label}`)
  return button
}

describe('server dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    api.metrics.mockResolvedValue({
      onlineClients: 2,
      knownClients: 3,
      proxies: 4,
      curConns: 5,
      totalTrafficIn: 1024,
      totalTrafficOut: 2048,
      proxyTypeCounts: { tcp: 4 },
      version: '0.68.0',
      distributionVersion: '0.0.1',
      upstreamVersion: '0.71.1',
    })
    api.runtime.mockResolvedValue({
      goVersion: 'go1.26',
      goroutines: 12,
      heapAlloc: 4096,
      sys: 8192,
      gcCount: 3,
      processUptime: 90,
      numCPU: 4,
    })
    api.diagnosticSettings.mockResolvedValue({ pprofEnabled: false })
    api.supervisor.mockResolvedValue({ state: 'running' })
    api.restart.mockResolvedValue({ message: 'accepted' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads observability data directly on the overview page', async () => {
    const wrapper = mountDashboard()

    await flushPromises()

    expect(api.metrics).toHaveBeenCalledOnce()
    expect(api.runtime).toHaveBeenCalledOnce()
    expect(api.diagnosticSettings).toHaveBeenCalledOnce()
    expect(api.supervisor).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('运行信息')
    expect(wrapper.text()).toContain('性能诊断')
    expect(wrapper.text()).toContain('已关闭')
    expect(wrapper.text()).toContain('代理类型分布')
    expect(wrapper.text()).toContain('服务监听')
    expect(wrapper.text()).toContain('重启服务')
    expect(wrapper.text()).toContain('分发版本')
    expect(wrapper.text()).toContain('0.0.1')
    expect(wrapper.text()).toContain('frp 版本')
    expect(wrapper.text()).toContain('0.71.1')
  })

  it('confirms a restart once and polls until recovery times out', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new Error('service offline'))
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mountDashboard()
    await flushPromises()

    await findButton(wrapper, '重启服务').trigger('click')
    await flushPromises()
    expect(wrapper.find('.modal-stub').exists()).toBe(true)

    await wrapper.get('.modal-confirm').trigger('click')
    await flushPromises()
    expect(api.restart).toHaveBeenCalledOnce()

    for (let attempt = 0; attempt < 30; attempt += 1) {
      await vi.advanceTimersByTimeAsync(1_000)
    }
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(30)
    expect(findButton(wrapper, '重启服务').attributes('data-loading')).toBe('false')
  })

  it('keeps the confirmation open when the restart request fails', async () => {
    api.restart.mockRejectedValue(new Error('Supervisor unavailable'))
    const wrapper = mountDashboard()
    await flushPromises()

    await findButton(wrapper, '重启服务').trigger('click')
    await flushPromises()
    await wrapper.get('.modal-confirm').trigger('click')
    await flushPromises()

    expect(api.restart).toHaveBeenCalledOnce()
    expect(wrapper.find('.modal-stub').exists()).toBe(true)
    expect(wrapper.text()).toContain('Supervisor unavailable')
  })
})
