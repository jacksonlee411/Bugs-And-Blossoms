import { describe, expect, it } from 'vitest'
import { isDay, parseRequestedAsOf, resolveReadViewState } from './readViewState'

describe('readViewState', () => {
  it('treats null as current mode and uses fallback today', () => {
    expect(resolveReadViewState(null, '2026-04-08')).toEqual({
      mode: 'current',
      requestedAsOf: null,
      effectiveAsOf: '2026-04-08'
    })
  })

  it('treats invalid as_of as current mode', () => {
    expect(resolveReadViewState('bad-date', '2026-04-08')).toEqual({
      mode: 'current',
      requestedAsOf: null,
      effectiveAsOf: '2026-04-08'
    })
  })

  it('treats valid as_of as history mode', () => {
    expect(resolveReadViewState('2026-03-31', '2026-04-08')).toEqual({
      mode: 'history',
      requestedAsOf: '2026-03-31',
      effectiveAsOf: '2026-03-31'
    })
  })

  it('trims requested as_of before parsing', () => {
    expect(parseRequestedAsOf(' 2026-04-01 ')).toBe('2026-04-01')
  })

  it('returns null for empty or malformed requested as_of', () => {
    expect(parseRequestedAsOf('')).toBeNull()
    expect(parseRequestedAsOf('2026-4-1')).toBeNull()
  })

  it('accepts YYYY-MM-DD only', () => {
    expect(isDay('2026-04-08')).toBe(true)
    expect(isDay('2026-04-08T00:00:00Z')).toBe(false)
    expect(isDay('2026/04/08')).toBe(false)
  })
})
