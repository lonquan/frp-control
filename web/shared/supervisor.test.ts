import { describe, expect, it } from 'vitest'
import { formatDateTime, supervisorStateLabel } from './supervisor'

describe('supervisor display helpers', () => {
  it('maps supervisor states to Chinese labels', () => {
    expect(supervisorStateLabel('running')).toBe('运行中')
    expect(supervisorStateLabel('unexpected-exit')).toBe('异常退出')
    expect(supervisorStateLabel('custom-state')).toBe('custom-state')
    expect(supervisorStateLabel()).toBe('-')
  })

  it('renders zero and invalid timestamps as an empty value', () => {
    expect(formatDateTime()).toBe('-')
    expect(formatDateTime('0001-01-01T00:00:00Z')).toBe('-')
    expect(formatDateTime('not-a-date')).toBe('-')
    expect(formatDateTime('2026-10-22T10:11:00+08:00')).not.toBe('-')
  })
})
