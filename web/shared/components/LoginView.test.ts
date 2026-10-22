import { defineComponent } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-router', () => ({
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock('../session', () => ({
  useSessionStore: () => ({
    login: vi.fn(),
    redirectTo: '/',
  }),
}))

import LoginView from './LoginView.vue'

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

const InputStub = defineComponent({
  props: {
    modelValue: { type: String, default: '' },
    placeholder: { type: String, default: '' },
  },
  emits: ['update:modelValue'],
  template: '<input :value="modelValue" :placeholder="placeholder" />',
})

describe.each(['server', 'client'] as const)('%s login view', (role) => {
  function mountLogin() {
    return shallowMount(LoginView, {
      props: { role },
      global: {
        stubs: {
          'a-button': SlotStub,
          'a-form': SlotStub,
          'a-form-item': SlotStub,
          'a-input': InputStub,
          'a-input-password': SlotStub,
          'a-alert': SlotStub,
        },
      },
    })
  }

  it('does not prefill the administrator account', () => {
    const wrapper = mountLogin()

    const username = wrapper.get('input[autocomplete="username"], input').element
    expect((username as HTMLInputElement).value).toBe('')
    expect((username as HTMLInputElement).placeholder).toBe('请输入管理员账号')
  })

  it('uses the FRP brand and omits the removed login guidance', () => {
    const wrapper = mountLogin()

    expect(wrapper.text()).toContain('FRP 管理控制台')
    expect(wrapper.text()).not.toContain('登录状态将在当前浏览器中保持 7 天')
    expect(wrapper.text()).not.toContain('仅管理员可访问')
  })
})
