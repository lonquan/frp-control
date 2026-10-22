import { describe, expect, it } from 'vitest'
import { clientNavItems } from '../src/navigation'

describe('client navigation', () => {
  it('places traffic capture before configuration', () => {
    const labels = clientNavItems.map((item) => item.label)

    expect(labels).toEqual([
      '总览',
      '代理',
      '访问者',
      '流量捕获',
      '配置',
      '证书',
      '设置',
    ])
  })
})
