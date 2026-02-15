import { describe, expect, it } from 'vitest'
import {
  fromGridSortModel,
  parseGridQueryState,
  patchGridQueryState,
  toGridSortModel
} from './gridQueryState'

describe('gridQueryState', () => {
  it('解析查询参数并回退非法值', () => {
    const params = new URLSearchParams({
      q: 'keyword',
      status: 'active',
      page: '-1',
      size: '0',
      sort: 'name',
      order: 'asc'
    })

    const state = parseGridQueryState(params, {
      statusValues: ['active', 'inactive'] as const,
      sortFields: ['name', 'code'] as const
    })

    expect(state).toEqual({
      keyword: 'keyword',
      page: 0,
      pageSize: 10,
      sortField: 'name',
      sortOrder: 'asc',
      status: 'active'
    })
  })

  it('patch 会在空值时移除对应参数', () => {
    const params = new URLSearchParams({
      q: 'a',
      status: 'active',
      page: '2',
      size: '20',
      sort: 'code',
      order: 'desc'
    })

    const next = patchGridQueryState(params, {
      keyword: '',
      status: 'all',
      page: 0,
      pageSize: 10,
      sortField: null,
      sortOrder: null
    })

    expect(next.get('q')).toBeNull()
    expect(next.get('status')).toBe('all')
    expect(next.get('page')).toBe('0')
    expect(next.get('size')).toBe('10')
    expect(next.get('sort')).toBeNull()
    expect(next.get('order')).toBeNull()
  })

  it('sortModel 转换保持一致', () => {
    const sortModel = toGridSortModel('submittedAt', 'desc')
    expect(sortModel).toEqual([{ field: 'submittedAt', sort: 'desc' }])
    expect(fromGridSortModel(sortModel, ['submittedAt', 'status'])).toEqual({
      sortField: 'submittedAt',
      sortOrder: 'desc'
    })
    expect(fromGridSortModel([], ['submittedAt'])).toEqual({
      sortField: null,
      sortOrder: null
    })
  })
})
