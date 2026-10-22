import { describe, expect, it, vi } from 'vitest'

import router from '../src/router'

describe('client router', () => {
  it('redirects the legacy diagnostics route to the overview', async () => {
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    await router.push('/diagnostics')
    await router.isReady()

    expect(router.currentRoute.value.path).toBe('/')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })
})
