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
  page?: number
  size?: number
  total?: number
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

export type OrgUnitListStatusFilter = 'all' | 'active' | 'inactive'
export type OrgUnitListSortField = 'code' | 'name' | 'status' | `ext:${string}`
export type OrgUnitListSortOrder = 'asc' | 'desc'

export async function listOrgUnitsPage(options: {
  asOf: string
  parentOrgCode?: string
  includeDisabled?: boolean
  keyword?: string
  status?: OrgUnitListStatusFilter
  page: number
  pageSize: number
  sortField?: OrgUnitListSortField | null
  sortOrder?: OrgUnitListSortOrder | null
  extFilterFieldKey?: string
  extFilterValue?: string
}): Promise<OrgUnitListResponse> {
  const query = new URLSearchParams({
    as_of: options.asOf,
    mode: 'grid',
    page: String(options.page),
    size: String(options.pageSize)
  })
  if (options.parentOrgCode) {
    query.set('parent_org_code', options.parentOrgCode)
  }
  if (options.includeDisabled) {
    query.set('include_disabled', '1')
  }

  const keyword = options.keyword?.trim() ?? ''
  if (keyword.length > 0) {
    query.set('q', keyword)
  }

  if (options.status && options.status !== 'all') {
    query.set('status', options.status)
  }

  if (options.sortField && options.sortOrder) {
    query.set('sort', options.sortField)
    query.set('order', options.sortOrder)
  }
  if (options.extFilterFieldKey && options.extFilterValue) {
    query.set('ext_filter_field_key', options.extFilterFieldKey)
    query.set('ext_filter_value', options.extFilterValue)
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

export type OrgUnitExtValueType = 'text' | 'int' | 'uuid' | 'bool' | 'date'
export type OrgUnitExtDataSourceType = 'PLAIN' | 'DICT' | 'ENTITY'
export type OrgUnitExtDisplayValueSource =
  | 'plain'
  | 'versions_snapshot'
  | 'events_snapshot'
  | 'dict_fallback'
  | 'entity_join'
  | 'unresolved'
export type OrgUnitExtScalarValue = string | number | boolean | null

export interface OrgUnitExtField {
  field_key: string
  label_i18n_key: string | null
  label?: string | null
  value_type: OrgUnitExtValueType
  data_source_type: OrgUnitExtDataSourceType
  value: OrgUnitExtScalarValue
  display_value: string | null
  display_value_source: OrgUnitExtDisplayValueSource
}

export interface OrgUnitDetailsResponse {
  as_of: string
  org_unit: OrgUnitDetailsAPIItem
  ext_fields?: OrgUnitExtField[]
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
  ext?: Record<string, unknown>
}): Promise<OrgUnitWriteResult> {
  return httpClient.post<OrgUnitWriteResult>('/org/api/org-units', request)
}

export async function renameOrgUnit(request: {
  org_code: string
  new_name: string
  effective_date?: string
  ext?: Record<string, unknown>
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/rename', request)
}

export async function moveOrgUnit(request: {
  org_code: string
  new_parent_org_code: string
  effective_date?: string
  ext?: Record<string, unknown>
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/move', request)
}

export async function disableOrgUnit(request: {
  org_code: string
  effective_date?: string
  ext?: Record<string, unknown>
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/disable', request)
}

export async function enableOrgUnit(request: {
  org_code: string
  effective_date?: string
  ext?: Record<string, unknown>
}): Promise<{ org_code: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; effective_date: string }>('/org/api/org-units/enable', request)
}

export async function setOrgUnitBusinessUnit(request: {
  org_code: string
  effective_date: string
  is_business_unit: boolean
  request_code: string
  ext?: Record<string, unknown>
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
    ext?: Record<string, unknown>
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

export interface OrgUnitCorrectEventCapability {
  enabled: boolean
  allowed_fields: string[]
  field_payload_keys: Record<string, string>
  deny_reasons: string[]
}

export interface OrgUnitCorrectStatusCapability {
  enabled: boolean
  allowed_target_statuses: string[]
  deny_reasons: string[]
}

export interface OrgUnitBasicCapability {
  enabled: boolean
  deny_reasons: string[]
}

export interface OrgUnitMutationCapabilitiesEnvelope {
  correct_event: OrgUnitCorrectEventCapability
  correct_status: OrgUnitCorrectStatusCapability
  rescind_event: OrgUnitBasicCapability
  rescind_org: OrgUnitBasicCapability
}

export interface OrgUnitMutationCapabilitiesResponse {
  org_code: string
  effective_date: string
  effective_target_event_type: string
  raw_target_event_type: string
  capabilities: OrgUnitMutationCapabilitiesEnvelope
}

export interface OrgUnitAppendCapability {
  enabled: boolean
  allowed_fields: string[]
  field_payload_keys: Record<string, string>
  deny_reasons: string[]
}

export interface OrgUnitAppendCapabilitiesResponse {
  org_code: string
  effective_date: string
  capabilities: {
    create: OrgUnitAppendCapability
    event_update: Record<string, OrgUnitAppendCapability>
  }
}

export async function getOrgUnitMutationCapabilities(options: {
  orgCode: string
  effectiveDate: string
}): Promise<OrgUnitMutationCapabilitiesResponse> {
  const query = new URLSearchParams({
    org_code: options.orgCode,
    effective_date: options.effectiveDate
  })
  return httpClient.get<OrgUnitMutationCapabilitiesResponse>(`/org/api/org-units/mutation-capabilities?${query.toString()}`)
}

export async function getOrgUnitAppendCapabilities(options: {
  orgCode: string
  effectiveDate: string
}): Promise<OrgUnitAppendCapabilitiesResponse> {
  const query = new URLSearchParams({
    org_code: options.orgCode,
    effective_date: options.effectiveDate
  })
  return httpClient.get<OrgUnitAppendCapabilitiesResponse>(`/org/api/org-units/append-capabilities?${query.toString()}`)
}

export interface OrgUnitFieldOption {
  value: string
  label: string
}

export interface OrgUnitFieldOptionsResponse {
  field_key: string
  as_of: string
  options: OrgUnitFieldOption[]
}

export async function getOrgUnitFieldOptions(options: {
  fieldKey: string
  asOf: string
  keyword?: string
  limit?: number
}): Promise<OrgUnitFieldOptionsResponse> {
  const query = new URLSearchParams({
    field_key: options.fieldKey,
    as_of: options.asOf
  })

  const keyword = options.keyword?.trim() ?? ''
  if (keyword.length > 0) {
    query.set('q', keyword)
  }

  if (typeof options.limit === 'number' && Number.isFinite(options.limit) && options.limit > 0) {
    query.set('limit', String(options.limit))
  }

  return httpClient.get<OrgUnitFieldOptionsResponse>(`/org/api/org-units/fields:options?${query.toString()}`)
}

export type OrgUnitTenantFieldConfigStatus = 'all' | 'enabled' | 'disabled'

export interface OrgUnitFieldDefinition {
  field_key: string
  value_type: OrgUnitExtValueType
  data_source_type: OrgUnitExtDataSourceType
  data_source_config: Record<string, unknown>
  data_source_config_options?: Record<string, unknown>[]
  label_i18n_key: string
  allow_filter?: boolean
  allow_sort?: boolean
}

export interface OrgUnitFieldDefinitionsResponse {
  fields: OrgUnitFieldDefinition[]
}

export async function listOrgUnitFieldDefinitions(): Promise<OrgUnitFieldDefinitionsResponse> {
  return httpClient.get<OrgUnitFieldDefinitionsResponse>('/org/api/org-units/field-definitions')
}

export interface OrgUnitTenantFieldConfig {
  field_key: string
  value_type: OrgUnitExtValueType
  data_source_type: OrgUnitExtDataSourceType
  data_source_config: Record<string, unknown>
  physical_col: string
  enabled_on: string
  disabled_on: string | null
  updated_at: string
}

export interface OrgUnitFieldConfigsResponse {
  as_of: string
  field_configs: OrgUnitTenantFieldConfig[]
}

export async function listOrgUnitFieldConfigs(options: {
  asOf: string
  status?: OrgUnitTenantFieldConfigStatus
}): Promise<OrgUnitFieldConfigsResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  if (options.status && options.status !== 'all') {
    query.set('status', options.status)
  }
  return httpClient.get<OrgUnitFieldConfigsResponse>(`/org/api/org-units/field-configs?${query.toString()}`)
}

export async function enableOrgUnitFieldConfig(request: {
  field_key: string
  enabled_on: string
  request_code: string
  data_source_config?: Record<string, unknown>
}): Promise<OrgUnitTenantFieldConfig> {
  return httpClient.post<OrgUnitTenantFieldConfig>('/org/api/org-units/field-configs', request)
}

export async function disableOrgUnitFieldConfig(request: {
  field_key: string
  disabled_on: string
  request_code: string
}): Promise<OrgUnitTenantFieldConfig> {
  return httpClient.post<OrgUnitTenantFieldConfig>('/org/api/org-units/field-configs:disable', request)
}
