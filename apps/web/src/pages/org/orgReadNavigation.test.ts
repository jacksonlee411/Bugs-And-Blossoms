import { describe, expect, it } from 'vitest'
import { buildOrgFieldConfigsSearchParams, buildOrgUnitDetailSearchParams } from './orgReadNavigation'

describe('orgReadNavigation', () => {
  it('does not carry as_of in current mode', () => {
    const params = buildOrgFieldConfigsSearchParams('current', '2026-04-08')
    expect(params.get('as_of')).toBeNull()
  })

  it('carries as_of in history mode', () => {
    const params = buildOrgFieldConfigsSearchParams('history', '2026-04-08')
    expect(params.get('as_of')).toBe('2026-04-08')
  })

  it('builds detail params for current mode without history anchor', () => {
    const params = buildOrgUnitDetailSearchParams({
      readMode: 'current',
      asOf: '2026-04-08',
      includeDisabled: false
    })

    expect(params.get('as_of')).toBeNull()
    expect(params.get('include_disabled')).toBeNull()
  })

  it('builds detail params for history mode with include_disabled', () => {
    const params = buildOrgUnitDetailSearchParams({
      readMode: 'history',
      asOf: '2026-04-08',
      includeDisabled: true
    })

    expect(params.get('as_of')).toBe('2026-04-08')
    expect(params.get('include_disabled')).toBe('1')
  })

  it('keeps optional legacy params only when non-empty', () => {
    const params = buildOrgUnitDetailSearchParams({
      readMode: 'history',
      asOf: '2026-04-08',
      includeDisabled: false,
      effectiveDate: ' 2026-03-01 ',
      tab: ' records '
    })

    expect(params.get('effective_date')).toBe('2026-03-01')
    expect(params.get('tab')).toBe('records')
  })

  it('drops blank optional legacy params', () => {
    const params = buildOrgUnitDetailSearchParams({
      readMode: 'history',
      asOf: '2026-04-08',
      includeDisabled: false,
      effectiveDate: ' ',
      tab: ''
    })

    expect(params.get('effective_date')).toBeNull()
    expect(params.get('tab')).toBeNull()
  })
})
