import { defineComponent, nextTick, type Ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/' }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@vueuse/core', async () => {
  const { ref: createRef } = await import('vue')
  const viewport = createRef(false)
  const storedCollapsed = createRef(false)
  return {
    __testState: { viewport, storedCollapsed },
    useDark: () => createRef(false),
    useLocalStorage: () => storedCollapsed,
    useMediaQuery: () => viewport,
  }
})

vi.mock('../session', () => ({
  useSessionStore: () => ({
    displayName: 'admin',
    logout: vi.fn(),
  }),
}))

import * as vueuse from '@vueuse/core'
import AppShell from './AppShell.vue'

const testState = (
  vueuse as typeof vueuse & {
    __testState: {
      viewport: Ref<boolean>
      storedCollapsed: Ref<boolean>
    }
  }
).__testState

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

const SiderStub = defineComponent({
  props: {
    collapsed: Boolean,
  },
  template:
    '<aside class="sider-stub" :data-collapsed="collapsed ? \'true\' : \'false\'"><slot /></aside>',
})

const ButtonStub = defineComponent({
  emits: ['click'],
  template: '<button @click="$emit(\'click\')"><slot /></button>',
})

function mountShell() {
  return shallowMount(AppShell, {
    props: {
      role: 'server',
      items: [],
    },
    global: {
      stubs: {
        'a-layout': SlotStub,
        'a-layout-sider': SiderStub,
        'a-layout-header': SlotStub,
        'a-layout-content': SlotStub,
        'a-menu': SlotStub,
        'a-menu-item': SlotStub,
        'a-button': ButtonStub,
        'a-tooltip': SlotStub,
        'a-dropdown': SlotStub,
        'a-doption': SlotStub,
      },
    },
  })
}

function isCollapsed(wrapper: ReturnType<typeof mountShell>) {
  return wrapper.get('.sider-stub').attributes('data-collapsed') === 'true'
}

describe('AppShell', () => {
  beforeEach(() => {
    testState.viewport.value = false
    testState.storedCollapsed.value = false
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
  })

  it('uses FRP branding and marks healthy status indicators', async () => {
    const wrapper = mountShell()
    await flushPromises()

    expect(wrapper.text()).toContain('FRP 控制台')
    const indicators = wrapper.findAll('.health-dot')
    expect(indicators).toHaveLength(2)
    indicators.forEach((indicator) => {
      expect(indicator.classes()).toContain('ok')
    })
  })

  it('auto-collapses on narrow viewports and restores desktop expansion', async () => {
    const wrapper = mountShell()
    await flushPromises()
    expect(isCollapsed(wrapper)).toBe(false)

    testState.viewport.value = true
    await nextTick()
    expect(isCollapsed(wrapper)).toBe(true)

    testState.viewport.value = false
    await nextTick()
    expect(isCollapsed(wrapper)).toBe(false)
  })

  it('preserves an explicit collapse choice across viewport changes', async () => {
    const wrapper = mountShell()
    await flushPromises()

    await wrapper.get('button.icon-button').trigger('click')
    await nextTick()
    expect(isCollapsed(wrapper)).toBe(true)
    expect(testState.storedCollapsed.value).toBe(true)

    testState.viewport.value = true
    await nextTick()
    testState.viewport.value = false
    await nextTick()
    expect(isCollapsed(wrapper)).toBe(true)
  })
})
