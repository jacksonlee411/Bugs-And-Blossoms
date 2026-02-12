import { httpClient } from './httpClient'

export interface OrgUnitAPIItem {
  org_code: string
  name: string
  status: string
  is_business_unit?: boolean
  has_children?: boolean
}

export interface OrgUnitListResponse {
  as_of: string
  include_disabled?: boolean
  org_units: OrgUnitAPIItem[]
}

export async function listOrgUnits(options: {
  asOf: string
  parentOrgCode?: string
  includeDisabled?: boolean
}): Promise<OrgUnitListResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  if (options.parentOrgCode) {
    query.set('parent_org_code', options.parentOrgCode)
  }
  if (options.includeDisabled) {
    query.set('include_disabled', '1')
  }

  return httpClient.get<OrgUnitListResponse>(`/org/api/org-units?${query.toString()}`)
}

export interface OrgUnitDetailsAPIItem {
  org_id: number
  org_code: string
  name: string
  status: string
  parent_org_code: string
  parent_name: string
  is_business_unit: boolean
  manager_pernr: string
  manager_name: string
  full_name_path: string
  created_at: string
  updated_at: string
  event_uuid: string
}

export interface OrgUnitDetailsResponse {
  as_of: string
  org_unit: OrgUnitDetailsAPIItem
}

export async function getOrgUnitDetails(options: {
  orgCode: string
  asOf: string
  includeDisabled?: boolean
}): Promise<OrgUnitDetailsResponse> {
  const query = new URLSearchParams({
    org_code: options.orgCode,
    as_of: options.asOf
  })
  if (options.includeDisabled) {
    query.set('include_disabled', '1')
  }

  return httpClient.get<OrgUnitDetailsResponse>(`/org/api/org-units/details?${query.toString()}`)
}

export interface OrgUnitVersionAPIItem {
  event_id: number
  event_uuid: string
  effective_date: string
  event_type: string
}

export interface OrgUnitVersionsResponse {
  org_code: string
  versions: OrgUnitVersionAPIItem[]
}

export async function listOrgUnitVersions(options: { orgCode: string }): Promise<OrgUnitVersionsResponse> {
  const query = new URLSearchParams({ org_code: options.orgCode })
  return httpClient.get<OrgUnitVersionsResponse>(`/org/api/org-units/versions?${query.toString()}`)
}

export interface OrgUnitAuditAPIItem {
  event_id: number
  event_uuid: string
  event_type: string
  effective_date: string
  tx_time: string
  initiator_name: string
  initiator_employee_id: string
  request_code: string
  reason: string
  is_rescinded: boolean
  rescinded_by_event_uuid: string
  rescinded_by_tx_time: string
  rescinded_by_request_code: string
  payload: unknown
  before_snapshot: unknown
  after_snapshot: unknown
}

export interface OrgUnitAuditResponse {
  org_code: string
  limit: number
  has_more: boolean
  events: OrgUnitAuditAPIItem[]
}

export async function listOrgUnitAudit(options: {
  orgCode: string
  limit?: number
}): Promise<OrgUnitAuditResponse> {
  const query = new URLSearchParams({ org_code: options.orgCode })
  if (options.limit) {
    query.set('limit', String(options.limit))
  }

  return httpClient.get<OrgUnitAuditResponse>(`/org/api/org-units/audit?${query.toString()}`)
}

export interface OrgUnitSearchResult {
  target_org_id: number
  target_org_code: string
  target_name: string
  path_org_ids: number[]
  path_org_codes?: string[]
  tree_as_of: string
}

export async function searchOrgUnit(options: {
  query: string
  asOf: string
  includeDisabled?: boolean
}): Promise<OrgUnitSearchResult> {
  const queryParams = new URLSearchParams({
    query: options.query,
    as_of: options.asOf
  })
  if (options.includeDisabled) {
    queryParams.set('include_disabled', '1')
  }

  return httpClient.get<OrgUnitSearchResult>(`/org/api/org-units/search?${queryParams.toString()}`)
}

export interface OrgUnitWriteResult {
  org_code: string
  effective_date: string
  fields?: Record<string, unknown>
}

export async function createOrgUnit(request: {
  org_code: string
  name: string
  effective_date?: string
  parent_org_code?: string
  is_business_unit?: boolean
  manager_pernr?: string
}): Promise<OrgUnitWriteResult> {
  return httpClient.post<OrgUnitWriteResult>('/org/api/org-units', request)
}

export async function renameOrgUnit(request: {
  org_code: string
  new_name: string
  effective_date?: string
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/rename', request)
}

export async function moveOrgUnit(request: {
  org_code: string
  new_parent_org_code: string
  effective_date?: string
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/move', request)
}

export async function disableOrgUnit(request: {
  org_code: string
  effective_date?: string
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/disable', request)
}

export async function enableOrgUnit(request: {
  org_code: string
  effective_date?: string
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/enable', request)
}

export async function setOrgUnitBusinessUnit(request: {
  org_code: string
  effective_date: string
  is_business_unit: boolean
  request_code: string
}): Promise<{ org_code: string; effective_date: string; is_business_unit: boolean }> {
  return httpClient.post<{ org_code: string; effective_date: string; is_business_unit: boolean }>(
    '/org/api/org-units/set-business-unit',
    request
  )
}

export async function correctOrgUnit(request: {
  org_code: string
  effective_date: string
  request_id: string
  patch: {
    effective_date?: string
    name?: string
    parent_org_code?: string
    is_business_unit?: boolean
    manager_pernr?: string
  }
}): Promise<OrgUnitWriteResult> {
  return httpClient.post<OrgUnitWriteResult>('/org/api/org-units/corrections', request)
}

export async function correctOrgUnitStatus(request: {
  org_code: string
  effective_date: string
  target_status: string
  request_id: string
}): Promise<OrgUnitWriteResult> {
  return httpClient.post<OrgUnitWriteResult>('/org/api/org-units/status-corrections', request)
}

export async function rescindOrgUnitRecord(request: {
  org_code: string
  effective_date: string
  request_id: string
  reason: string
}): Promise<{ org_code: string; effective_date: string; operation: string; request_id: string }> {
  return httpClient.post<{ org_code: string; effective_date: string; operation: string; request_id: string }>(
    '/org/api/org-units/rescinds',
    request
  )
}

export async function rescindOrgUnit(request: {
  org_code: string
  request_id: string
  reason: string
}): Promise<{ org_code: string; operation: string; request_id: string; rescinded_events: number }> {
  return httpClient.post<{ org_code: string; operation: string; request_id: string; rescinded_events: number }>(
    '/org/api/org-units/rescinds/org',
    request
  )
}

