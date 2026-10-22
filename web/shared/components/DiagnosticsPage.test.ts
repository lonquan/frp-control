import { defineComponent } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  metrics: vi.fn(),
  runtime: vi.fn(),
  diagnosticSettings: vi.fn(),
  supervisor: vi.fn(),
  saveDiagnosticSettings: vi.fn(),
  downloadGoroutine: vi.fn(),
  downloadHeap: vi.fn(),
  downloadCPU: vi.fn(),
}))

vi.mock('../admin', () => ({
  adminAPI: api,
  downloadBlob: vi.fn(),
}))

import DiagnosticsPage from './DiagnosticsPage.vue'

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

describe('diagnostics page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.metrics.mockResolvedValue({
      distributionVersion: '0.0.1',
      upstreamVersion: '0.71.1',
      version: '0.71.1',
    })
    api.runtime.mockResolvedValue({})
    api.diagnosticSettings.mockResolvedValue({ pprofEnabled: false })
    api.supervisor.mockResolvedValue({ state: 'running' })
  })

  it('shows distribution and upstream versions separately', async () => {
    const wrapper = shallowMount(DiagnosticsPage, {
      props: { role: 'client' },
      global: {
        stubs: {
          PageHeader: SlotStub,
          MetricCard: SlotStub,
          'a-button': SlotStub,
          'a-switch': true,
          'a-alert': SlotStub,
          'a-tag': true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('分发版本')
    expect(wrapper.text()).toContain('0.0.1')
    expect(wrapper.text()).toContain('frp 版本')
    expect(wrapper.text()).toContain('0.71.1')
  })
})
