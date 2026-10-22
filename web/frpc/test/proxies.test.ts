import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const request = vi.hoisted(() => vi.fn())
const router = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
}))
const ui = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  confirm: vi.fn(),
}))
const certificates = vi.hoisted(() => vi.fn())

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => router,
}))

vi.mock('@shared/admin', () => ({
  legacyRequest: request,
  adminAPI: { certificates },
}))

vi.mock('@arco-design/web-vue', () => ({
  Message: {
    success: ui.success,
    error: ui.error,
  },
  Modal: {
    confirm: ui.confirm,
  },
}))

import ProxiesPage from '../src/views/ProxiesPage.vue'
import { directoryStubs, SwitchStub } from './directory-test-utils'

function mountPage() {
  return shallowMount(ProxiesPage, {
    global: {
      stubs: directoryStubs,
    },
  })
}

describe('client proxy directory', () => {
  let storeEnabled: boolean

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    storeEnabled = true
    certificates.mockResolvedValue([])
    request.mockImplementation(async (path: string, options?: RequestInit) => {
      if (path === '/api/status') {
        return {
          tcp: [{
            name: 'shared',
            type: 'tcp',
            status: 'running',
            err: '',
            local_addr: '127.0.0.1:22',
            remote_addr: 'frps.example.com:6000',
            plugin: '',
            source: 'store',
          }],
        }
      }
      if (path === '/api/config/proxies') {
        return {
          proxies: [
            {
              name: 'shared',
              type: 'tcp',
              source: 'config',
              enabled: false,
              local_addr: '127.0.0.1:22',
              remote_addr: 'frps.example.com:6000',
            },
            {
              name: 'shared',
              type: 'tcp',
              source: 'store',
              enabled: storeEnabled,
              local_addr: '127.0.0.1:22',
              remote_addr: 'frps.example.com:6000',
            },
          ],
        }
      }
      if (path === '/api/store/proxies' && (!options?.method || options.method === 'GET')) {
        return {
          proxies: [{
            name: 'shared',
            type: 'tcp',
            tcp: {
              enabled: storeEnabled,
              localPort: 22,
              remotePort: 6000,
            },
          }],
        }
      }
      if (
        path === '/api/store/proxies/shared' &&
        options?.method === 'PUT'
      ) {
        const payload = JSON.parse(String(options.body))
        storeEnabled = payload.tcp.enabled
        return payload
      }
      throw new Error(`unexpected request: ${path}`)
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders separate status and configuration lists with both sources', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('代理状态')
    expect(wrapper.text()).toContain('代理配置')
    expect(wrapper.text()).toContain('当前显示 1 个运行实例')
    expect(wrapper.text()).toContain('共 2 个配置')
    expect(wrapper.text()).toContain('配置文件')
    expect(wrapper.text()).toContain('动态配置')
    expect(wrapper.text()).toContain('打开配置中心')
  })

  it('opens the effective dynamic config for a same-name runtime status', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const detailButton = wrapper.findAll('button').find(
      (button) => button.text() === '详情',
    )
    await detailButton?.trigger('click')
    await flushPromises()

    const detail = wrapper.get('.drawer-stub').text()
    expect(detail).toContain('shared')
    expect(detail).toContain('动态配置')
    expect(detail).toContain('已启用')
  })

  it('persists switch state and renders enabled after reloading nested config', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const toggle = wrapper.findComponent(SwitchStub)
    expect(toggle.props('modelValue')).toBe(true)
    await toggle.trigger('click')
    await flushPromises()

    expect(storeEnabled).toBe(false)
    expect(request).toHaveBeenCalledWith(
      '/api/store/proxies/shared',
      expect.objectContaining({ method: 'PUT' }),
    )
    expect(ui.success).toHaveBeenCalledWith('代理已禁用')
    expect(wrapper.findComponent(SwitchStub).props('modelValue')).toBe(false)
    expect(wrapper.text()).toContain('已禁用')
  })

  it('refreshes only status on the visible-page interval', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const initialStatusCalls = request.mock.calls.filter(
      ([path]) => path === '/api/status',
    ).length

    await vi.advanceTimersByTimeAsync(10_000)
    await flushPromises()

    const nextStatusCalls = request.mock.calls.filter(
      ([path]) => path === '/api/status',
    ).length
    expect(nextStatusCalls).toBe(initialStatusCalls + 1)
    expect(request.mock.calls.filter(
      ([path]) => path === '/api/config/proxies',
    )).toHaveLength(1)
    wrapper.unmount()
  })
})
