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

vi.mock('vue-router', () => ({
  useRouter: () => router,
}))

vi.mock('@shared/admin', () => ({
  legacyRequest: request,
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

import VisitorsPage from '../src/views/VisitorsPage.vue'
import { directoryStubs, SwitchStub } from './directory-test-utils'

function mountPage() {
  return shallowMount(VisitorsPage, {
    global: {
      stubs: directoryStubs,
    },
  })
}

describe('client visitor directory', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    request.mockImplementation(async (path: string) => {
      if (path === '/api/visitors/status') {
        return {
          visitors: [{
            name: 'visitor',
            type: 'stcp',
            status: 'running',
            err: '',
            bind_addr: '127.0.0.1',
            bind_port: -1,
            server_name: 'server-proxy',
          }],
        }
      }
      if (path === '/api/config/visitors') {
        return {
          visitors: [{
            name: 'visitor',
            type: 'stcp',
            source: 'config',
            enabled: true,
            secret_configured: true,
            server_name: 'server-proxy',
            bind_addr: '127.0.0.1',
            bind_port: -1,
            use_encryption: true,
            use_compression: false,
          }],
        }
      }
      if (path === '/api/store/visitors') {
        return { visitors: [] }
      }
      throw new Error(`unexpected request: ${path}`)
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders listener readiness and optimized server defaults', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('访问者状态')
    expect(wrapper.text()).toContain('访问者配置')
    expect(wrapper.text()).toContain('已就绪')
    expect(wrapper.text()).toContain('当前用户')
    expect(wrapper.text()).toContain('仅接收重定向')
    expect(wrapper.text()).toContain('打开配置中心')
  })

  it('shows redacted secret state in configuration details', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const detailButton = wrapper.findAll('button').find(
      (button) => button.text() === '详情',
    )
    expect(detailButton).toBeDefined()
    await detailButton?.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Secret Key')
    expect(wrapper.text()).toContain('已配置')
    expect(wrapper.text()).not.toContain('top-secret')
    expect(wrapper.text()).toContain('不代表远端 Server 代理一定可达')
  })

  it('persists and renders a dynamic visitor switch after reload', async () => {
    let enabled = true
    request.mockImplementation(async (path: string, options?: RequestInit) => {
      if (path === '/api/visitors/status') {
        return { visitors: [] }
      }
      if (path === '/api/config/visitors') {
        return {
          visitors: [{
            name: 'dynamic',
            type: 'stcp',
            source: 'store',
            enabled,
            secret_configured: true,
            server_name: 'server-proxy',
            bind_port: 9000,
            use_encryption: false,
            use_compression: false,
          }],
        }
      }
      if (
        path === '/api/store/visitors' &&
        (!options?.method || options.method === 'GET')
      ) {
        return {
          visitors: [{
            name: 'dynamic',
            type: 'stcp',
            stcp: {
              enabled,
              serverName: 'server-proxy',
              bindPort: 9000,
            },
          }],
        }
      }
      if (
        path === '/api/store/visitors/dynamic' &&
        options?.method === 'PUT'
      ) {
        const payload = JSON.parse(String(options.body))
        enabled = payload.stcp.enabled
        return payload
      }
      throw new Error(`unexpected request: ${path}`)
    })

    const wrapper = mountPage()
    await flushPromises()
    const toggle = wrapper.findComponent(SwitchStub)
    expect(toggle.props('modelValue')).toBe(true)

    await toggle.trigger('click')
    await flushPromises()

    expect(enabled).toBe(false)
    expect(ui.success).toHaveBeenCalledWith('访问者已禁用')
    expect(wrapper.findComponent(SwitchStub).props('modelValue')).toBe(false)
  })
})
