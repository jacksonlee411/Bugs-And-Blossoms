import { buildReadSearchParams } from '../../utils/readNavigation'
import type { ReadMode } from '../../utils/readViewState'

export function buildOrgFieldConfigsSearchParams(readMode: ReadMode, asOf: string): URLSearchParams {
  return buildReadSearchParams(readMode, asOf)
}

export function buildOrgUnitDetailSearchParams(options: {
  readMode: ReadMode
  asOf: string
  includeDisabled: boolean
  effectiveDate?: string | null
  tab?: string | null
}): URLSearchParams {
  return buildReadSearchParams(options.readMode, options.asOf, {
    include_disabled: options.includeDisabled,
    effective_date: options.effectiveDate,
    tab: options.tab
  })
}
