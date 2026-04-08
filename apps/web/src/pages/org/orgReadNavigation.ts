import type { ReadMode } from './readViewState'

function trimToNull(value: string | null | undefined): string | null {
  const normalized = value?.trim() ?? ''
  return normalized.length > 0 ? normalized : null
}

export function buildOrgFieldConfigsSearchParams(readMode: ReadMode, asOf: string): URLSearchParams {
  const params = new URLSearchParams()
  if (readMode === 'history') {
    params.set('as_of', asOf)
  }
  return params
}

export function buildOrgUnitDetailSearchParams(options: {
  readMode: ReadMode
  asOf: string
  includeDisabled: boolean
  effectiveDate?: string | null
  tab?: string | null
}): URLSearchParams {
  const params = buildOrgFieldConfigsSearchParams(options.readMode, options.asOf)
  if (options.includeDisabled) {
    params.set('include_disabled', '1')
  }

  const effectiveDate = trimToNull(options.effectiveDate)
  if (effectiveDate) {
    params.set('effective_date', effectiveDate)
  }

  const tab = trimToNull(options.tab)
  if (tab) {
    params.set('tab', tab)
  }

  return params
}
