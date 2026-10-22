import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatusBadge from './StatusBadge.vue'

describe('StatusBadge', () => {
  it.each([
    ['start error', 'danger'],
    ['check failed', 'danger'],
    ['running', 'success'],
    ['starting', 'warning'],
  ])('maps %s to the %s state', (status, state) => {
    const wrapper = mount(StatusBadge, { props: { status } })

    expect(wrapper.get('.status-badge').classes()).toContain(state)
  })
})
