import { httpClient } from './httpClient'

export interface StaffingPositionAPIItem {
  position_uuid: string
  org_code: string
  reports_to_position_uuid: string
  jobcatalog_setid: string
  jobcatalog_setid_as_of: string
  job_profile_uuid: string
  job_profile_code: string
  name: string
  lifecycle_status: string
  capacity_fte: string
  effective_date: string
}

export interface StaffingPositionsListResponse {
  as_of: string
  tenant: string
  positions: StaffingPositionAPIItem[]
}

export async function listPositions(options: { asOf: string }): Promise<StaffingPositionsListResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  return httpClient.get<StaffingPositionsListResponse>(`/org/api/positions?${query.toString()}`)
}

export async function upsertPosition(request: {
  effective_date: string
  position_uuid?: string
  org_code: string
  job_profile_uuid: string
  capacity_fte?: string
  name: string
  lifecycle_status?: string
}): Promise<StaffingPositionAPIItem> {
  return httpClient.post<StaffingPositionAPIItem>('/org/api/positions', request)
}

export interface StaffingPositionOptionsResponse {
  as_of: string
  org_code: string
  jobcatalog_setid: string
  job_profiles: Array<{ job_profile_uuid: string; job_profile_code: string; name: string }>
}

export async function getPositionOptions(options: {
  asOf: string
  orgCode: string
}): Promise<StaffingPositionOptionsResponse> {
  const query = new URLSearchParams({ as_of: options.asOf, org_code: options.orgCode })
  return httpClient.get<StaffingPositionOptionsResponse>(`/org/api/positions:options?${query.toString()}`)
}

