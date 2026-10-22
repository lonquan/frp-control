import { defineComponent, markRaw } from 'vue'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHashHistory } from 'vue-router'
import { describe, expect, it } from 'vitest'
import MetricCard from './MetricCard.vue'

const PageStub = defineComponent({
  template: '<div />',
})

describe('MetricCard', () => {
  it.each([
    ['/clients', '/static/#/clients'],
    ['/proxies', '/static/#/proxies'],
  ])('generates a hash-router link for %s', async (to, expectedHref) => {
    const originalUrl = window.location.href
    window.history.replaceState({}, '', '/static/')
    const router = createRouter({
      history: createWebHashHistory(),
      routes: [
        { path: '/', component: PageStub },
        { path: '/clients', component: PageStub },
        { path: '/proxies', component: PageStub },
      ],
    })
    await router.push('/')
    await router.isReady()

    const wrapper = mount(MetricCard, {
      props: {
        label: '指标',
        value: 0,
        icon: markRaw(PageStub),
        to,
      },
      global: {
        plugins: [router],
      },
    })

    expect((wrapper.get('a').element as HTMLAnchorElement).href).toBe(
      new URL(expectedHref, window.location.origin).href,
    )
    window.history.replaceState({}, '', originalUrl)
  })
})
