import { httpClient } from './httpClient'

export interface OrgUnitAPIItem {
  org_code: string
  name: string
  is_business_unit?: boolean
  has_children?: boolean
}

export interface OrgUnitListResponse {
  as_of: string
  org_units: OrgUnitAPIItem[]
}

export async function listOrgUnits(options: {
  asOf: string
  parentOrgCode?: string
}): Promise<OrgUnitListResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  if (options.parentOrgCode) {
    query.set('parent_org_code', options.parentOrgCode)
  }

  return httpClient.get<OrgUnitListResponse>(`/org/api/org-units?${query.toString()}`)
}
