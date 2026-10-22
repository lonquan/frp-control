import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => ({
  supervisor: vi.fn(),
  changePassword: vi.fn(),
  clearSession: vi.fn(),
  replace: vi.fn(),
  validate: vi.fn(),
  clearValidate: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ replace: state.replace }),
}))

vi.mock('../admin', () => ({
  adminAPI: {
    supervisor: state.supervisor,
    changePassword: state.changePassword,
  },
}))

vi.mock('../session', () => ({
  useSessionStore: () => ({
    displayName: 'admin',
    role: 'server',
    expiresAt: '2026-10-29T12:00:00+08:00',
    clearSession: state.clearSession,
  }),
}))

vi.mock('@arco-design/web-vue', () => ({
  Message: {
    success: state.success,
    error: state.error,
  },
}))

import SettingsPage from './SettingsPage.vue'

const SlotStub = defineComponent({
  template: '<div><slot /></div>',
})

const ButtonStub = defineComponent({
  emits: ['click'],
  template: '<button @click="$emit(\'click\')"><slot /></button>',
})

const PasswordStub = defineComponent({
  props: {
    modelValue: { type: String, default: '' },
  },
  emits: ['update:modelValue'],
  template:
    '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
})

const FormStub = defineComponent({
  setup(_props, { expose, slots }) {
    expose({
      validate: state.validate,
      clearValidate: state.clearValidate,
    })
    return () => h('form', slots.default?.())
  },
})

const ModalStub = defineComponent({
  props: {
    visible: Boolean,
    onBeforeOk: Function,
  },
  emits: ['update:visible'],
  setup(props, { emit, slots }) {
    const cancel = () => emit('update:visible', false)
    const confirm = async () => {
      const accepted = await props.onBeforeOk?.()
      if (accepted) emit('update:visible', false)
    }
    return () =>
      props.visible
        ? h('section', { class: 'modal-stub' }, [
            ...(slots.default?.() || []),
            h('button', { class: 'modal-cancel', onClick: cancel }, '取消'),
            h('button', { class: 'modal-confirm', onClick: confirm }, '更新密码'),
          ])
        : null
  },
})

function mountPage() {
  return shallowMount(SettingsPage, {
    global: {
      stubs: {
        PageHeader: SlotStub,
        'a-tag': SlotStub,
        'a-button': ButtonStub,
        'a-alert': SlotStub,
        'a-modal': ModalStub,
        'a-form': FormStub,
        'a-form-item': SlotStub,
        'a-input-password': PasswordStub,
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mountPage>, label: string) {
  const button = wrapper.findAll('button').find((item) => item.text() === label)
  if (!button) throw new Error(`button not found: ${label}`)
  return button
}

describe('SettingsPage password controls', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    state.supervisor.mockResolvedValue({ state: 'running' })
    state.changePassword.mockResolvedValue({ message: 'ok' })
    state.replace.mockResolvedValue(undefined)
    state.validate.mockResolvedValue(undefined)
  })

  it('opens the password form in a modal and logs out after a successful update', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.find('.password-form').exists()).toBe(false)
    expect(wrapper.get('.password-summary').text()).toContain('管理员密码')

    await findButton(wrapper, '修改密码').trigger('click')
    await flushPromises()
    expect(wrapper.find('.password-form').exists()).toBe(true)

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('old-password')
    await inputs[1].setValue('new-password')
    await inputs[2].setValue('new-password')
    await wrapper.get('.modal-confirm').trigger('click')
    await flushPromises()

    expect(state.changePassword).toHaveBeenCalledWith('old-password', 'new-password')
    expect(state.clearSession).toHaveBeenCalledOnce()
    expect(state.replace).toHaveBeenCalledWith('/login')
    expect(wrapper.find('.modal-stub').exists()).toBe(false)
  })

  it('clears password fields after canceling and reopening the modal', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await findButton(wrapper, '修改密码').trigger('click')
    await flushPromises()
    const firstInputs = wrapper.findAll('input')
    await firstInputs[0].setValue('old-password')
    await firstInputs[1].setValue('new-password')
    await firstInputs[2].setValue('new-password')

    await wrapper.get('.modal-cancel').trigger('click')
    await flushPromises()
    await findButton(wrapper, '修改密码').trigger('click')
    await flushPromises()

    const reopenedInputs = wrapper.findAll('input')
    expect((reopenedInputs[0].element as HTMLInputElement).value).toBe('')
    expect((reopenedInputs[1].element as HTMLInputElement).value).toBe('')
    expect((reopenedInputs[2].element as HTMLInputElement).value).toBe('')
  })
})
