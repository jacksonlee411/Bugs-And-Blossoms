import type { OrgUnitListSortOrder } from '../../api/orgUnits'

export function parseSortOrder(raw: string | null): OrgUnitListSortOrder | null {
  if (!raw) {
    return null
  }
  const value = raw.trim().toLowerCase()
  if (value === 'asc' || value === 'desc') {
    return value
  }
  return null
}

export function parseExtSortField(raw: string | null): string | null {
  if (!raw) {
    return null
  }
  const value = raw.trim()
  if (!value.startsWith('ext:')) {
    return null
  }
  const fieldKey = value.slice(4).trim()
  return fieldKey.length > 0 ? fieldKey : null
}

export function clearExtQueryParams(params: URLSearchParams) {
  params.delete('ext_filter_field_key')
  params.delete('ext_filter_value')
  const sortValue = params.get('sort')?.trim() ?? ''
  if (sortValue.startsWith('ext:')) {
    params.delete('sort')
    params.delete('order')
  }
}
