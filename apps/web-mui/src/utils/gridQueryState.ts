import type { GridSortDirection, GridSortModel } from '@mui/x-data-grid'

export const GRID_QUERY_KEY = {
  keyword: 'q',
  page: 'page',
  pageSize: 'size',
  sortField: 'sort',
  sortOrder: 'order',
  status: 'status'
} as const

export type GridQuerySortOrder = 'asc' | 'desc'

export interface GridQueryState<TStatus extends string> {
  keyword: string
  page: number
  pageSize: number
  sortField: string | null
  sortOrder: GridQuerySortOrder | null
  status: 'all' | TStatus
}

interface GridQueryParseOptions<TStatus extends string> {
  statusValues: readonly TStatus[]
  defaultPageSize?: number
  sortFields?: readonly string[]
}

export interface GridQueryPatch {
  keyword?: string | null
  page?: number | null
  pageSize?: number | null
  sortField?: string | null
  sortOrder?: GridQuerySortOrder | null
  status?: string | null
}

function parsePage(raw: string | null): number {
  if (!raw) {
    return 0
  }

  const value = Number(raw)
  if (!Number.isInteger(value) || value < 0) {
    return 0
  }

  return value
}

function parsePageSize(raw: string | null, fallback: number): number {
  if (!raw) {
    return fallback
  }

  const value = Number(raw)
  if (!Number.isInteger(value) || value <= 0) {
    return fallback
  }

  return value
}

function isSortOrder(value: GridSortDirection | string | null): value is GridQuerySortOrder {
  return value === 'asc' || value === 'desc'
}

function hasSortField(field: string | null, sortFields?: readonly string[]): boolean {
  if (!field) {
    return false
  }

  if (!sortFields || sortFields.length === 0) {
    return true
  }

  return sortFields.includes(field)
}

function setOrDeleteParam(params: URLSearchParams, key: string, value: string | null | undefined) {
  if (!value || value.length === 0) {
    params.delete(key)
    return
  }

  params.set(key, value)
}

export function parseGridQueryState<TStatus extends string>(
  searchParams: URLSearchParams,
  options: GridQueryParseOptions<TStatus>
): GridQueryState<TStatus> {
  const fallbackPageSize = options.defaultPageSize ?? 10
  const rawStatus = searchParams.get(GRID_QUERY_KEY.status)
  const status = rawStatus === 'all' || (rawStatus !== null && options.statusValues.includes(rawStatus as TStatus))
    ? (rawStatus as 'all' | TStatus)
    : 'all'

  const rawSortField = searchParams.get(GRID_QUERY_KEY.sortField)
  const rawSortOrder = searchParams.get(GRID_QUERY_KEY.sortOrder)
  const sortField = hasSortField(rawSortField, options.sortFields) ? rawSortField : null
  const sortOrder = isSortOrder(rawSortOrder) ? rawSortOrder : null

  return {
    keyword: searchParams.get(GRID_QUERY_KEY.keyword) ?? '',
    page: parsePage(searchParams.get(GRID_QUERY_KEY.page)),
    pageSize: parsePageSize(searchParams.get(GRID_QUERY_KEY.pageSize), fallbackPageSize),
    sortField: sortField && sortOrder ? sortField : null,
    sortOrder: sortField && sortOrder ? sortOrder : null,
    status
  }
}

export function patchGridQueryState(
  searchParams: URLSearchParams,
  patch: GridQueryPatch
): URLSearchParams {
  const nextParams = new URLSearchParams(searchParams)

  if (Object.hasOwn(patch, 'keyword')) {
    setOrDeleteParam(nextParams, GRID_QUERY_KEY.keyword, patch.keyword)
  }

  if (Object.hasOwn(patch, 'status')) {
    setOrDeleteParam(nextParams, GRID_QUERY_KEY.status, patch.status)
  }

  if (Object.hasOwn(patch, 'page')) {
    const page = patch.page
    if (page === null || page === undefined || !Number.isInteger(page) || page < 0) {
      nextParams.delete(GRID_QUERY_KEY.page)
    } else {
      nextParams.set(GRID_QUERY_KEY.page, String(page))
    }
  }

  if (Object.hasOwn(patch, 'pageSize')) {
    const pageSize = patch.pageSize
    if (pageSize === null || pageSize === undefined || !Number.isInteger(pageSize) || pageSize <= 0) {
      nextParams.delete(GRID_QUERY_KEY.pageSize)
    } else {
      nextParams.set(GRID_QUERY_KEY.pageSize, String(pageSize))
    }
  }

  if (Object.hasOwn(patch, 'sortField') || Object.hasOwn(patch, 'sortOrder')) {
    const sortField =
      patch.sortField === undefined ? nextParams.get(GRID_QUERY_KEY.sortField) : patch.sortField
    const sortOrder =
      patch.sortOrder === undefined ? nextParams.get(GRID_QUERY_KEY.sortOrder) : patch.sortOrder

    if (sortField && isSortOrder(sortOrder)) {
      nextParams.set(GRID_QUERY_KEY.sortField, sortField)
      nextParams.set(GRID_QUERY_KEY.sortOrder, sortOrder)
    } else {
      nextParams.delete(GRID_QUERY_KEY.sortField)
      nextParams.delete(GRID_QUERY_KEY.sortOrder)
    }
  }

  return nextParams
}

export function toGridSortModel(sortField: string | null, sortOrder: GridQuerySortOrder | null): GridSortModel {
  if (!sortField || !sortOrder) {
    return []
  }

  return [{ field: sortField, sort: sortOrder }]
}

export function fromGridSortModel(
  sortModel: GridSortModel,
  sortFields?: readonly string[]
): { sortField: string | null; sortOrder: GridQuerySortOrder | null } {
  const firstSort = sortModel[0]
  if (!firstSort || !isSortOrder(firstSort.sort)) {
    return { sortField: null, sortOrder: null }
  }

  if (sortFields && sortFields.length > 0 && !sortFields.includes(firstSort.field)) {
    return { sortField: null, sortOrder: null }
  }

  return { sortField: firstSort.field, sortOrder: firstSort.sort }
}
