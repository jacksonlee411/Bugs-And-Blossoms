import { describe, expect, it } from 'vitest'
import { buildReadSearchParams, trimToNull } from '../../utils/readNavigation'

describe('utils/readNavigation', () => {
  it('does not carry as_of in current mode', () => {
    const params = buildReadSearchParams('current', '2026-04-08')
    expect(params.get('as_of')).toBeNull()
  })

  it('carries as_of in history mode', () => {
    const params = buildReadSearchParams('history', '2026-04-08')
    expect(params.get('as_of')).toBe('2026-04-08')
  })

  it('builds detail params for current mode without history anchor', () => {
    const params = buildReadSearchParams('current', '2026-04-08', {
      include_disabled: false
    })

    expect(params.get('as_of')).toBeNull()
    expect(params.get('include_disabled')).toBeNull()
  })

  it('builds detail params for history mode with include_disabled', () => {
    const params = buildReadSearchParams('history', '2026-04-08', {
      include_disabled: true
    })

    expect(params.get('as_of')).toBe('2026-04-08')
    expect(params.get('include_disabled')).toBe('1')
  })

  it('keeps optional legacy params only when non-empty', () => {
    const params = buildReadSearchParams('history', '2026-04-08', {
      include_disabled: false,
      effective_date: ' 2026-03-01 ',
      tab: ' records '
    })

    expect(params.get('effective_date')).toBe('2026-03-01')
    expect(params.get('tab')).toBe('records')
  })

  it('drops blank optional legacy params', () => {
    const params = buildReadSearchParams('history', '2026-04-08', {
      include_disabled: false,
      effective_date: ' ',
      tab: ''
    })

    expect(params.get('effective_date')).toBeNull()
    expect(params.get('tab')).toBeNull()
  })

  it('trims optional values to null', () => {
    expect(trimToNull('  abc  ')).toBe('abc')
    expect(trimToNull('')).toBeNull()
    expect(trimToNull(undefined)).toBeNull()
  })
})
