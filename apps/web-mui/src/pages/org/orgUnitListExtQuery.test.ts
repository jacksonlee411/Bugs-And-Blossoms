import { describe, expect, it } from 'vitest'
import { clearExtQueryParams, parseExtSortField, parseSortOrder } from './orgUnitListExtQuery'

describe('orgUnitListExtQuery', () => {
  it('parses sort order safely', () => {
    expect(parseSortOrder('asc')).toBe('asc')
    expect(parseSortOrder('DESC')).toBe('desc')
    expect(parseSortOrder('bad')).toBeNull()
    expect(parseSortOrder(null)).toBeNull()
  })

  it('parses ext sort field', () => {
    expect(parseExtSortField('ext:org_type')).toBe('org_type')
    expect(parseExtSortField('ext:')).toBeNull()
    expect(parseExtSortField('code')).toBeNull()
    expect(parseExtSortField(null)).toBeNull()
  })

  it('clears ext query params only', () => {
    const params = new URLSearchParams({
      sort: 'ext:org_type',
      order: 'desc',
      ext_filter_field_key: 'org_type',
      ext_filter_value: 'DEPARTMENT',
      q: 'keyword'
    })

    clearExtQueryParams(params)

    expect(params.get('ext_filter_field_key')).toBeNull()
    expect(params.get('ext_filter_value')).toBeNull()
    expect(params.get('sort')).toBeNull()
    expect(params.get('order')).toBeNull()
    expect(params.get('q')).toBe('keyword')
  })

  it('keeps core sort params when clearing ext query', () => {
    const params = new URLSearchParams({
      sort: 'name',
      order: 'asc',
      ext_filter_field_key: 'org_type',
      ext_filter_value: 'COMPANY'
    })

    clearExtQueryParams(params)

    expect(params.get('sort')).toBe('name')
    expect(params.get('order')).toBe('asc')
    expect(params.get('ext_filter_field_key')).toBeNull()
    expect(params.get('ext_filter_value')).toBeNull()
  })
})
