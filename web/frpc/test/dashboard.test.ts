import { defineComponent } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  metrics: vi.fn(),
  runtime: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@shared/admin', () => ({
  adminAPI: api,
}))

import DashboardPage from '../src/views/DashboardPage.vue'

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

describe('client dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.metrics.mockResolvedValue({
      connected: true,
      serverAddr: 'frps.example.com',
      serverPort: 7000,
      protocol: 'tcp',
      version: '0.71.1',
      distributionVersion: '0.0.1',
      upstreamVersion: '0.71.1',
      proxyTypeCounts: { tcp: 1 },
    })
    api.runtime.mockResolvedValue({
      goroutines: 8,
      goVersion: 'go1.26',
      heapAlloc: 4096,
      sys: 8192,
    })
  })

  it('shows distribution and upstream versions separately', async () => {
    const wrapper = shallowMount(DashboardPage, {
      global: {
        stubs: {
          PageHeader: SlotStub,
          'router-link': SlotStub,
          'a-button': SlotStub,
          'a-tag': SlotStub,
          'a-empty': SlotStub,
        },
      },
    })

    await flushPromises()

    expect(api.metrics).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('分发版本')
    expect(wrapper.text()).toContain('0.0.1')
    expect(wrapper.text()).toContain('frp 版本')
    expect(wrapper.text()).toContain('0.71.1')
  })
})
