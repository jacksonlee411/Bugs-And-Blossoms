import { httpClient } from './httpClient'

export interface SetIDAPIItem {
  setid: string
  name: string
  status: string
  is_shared: boolean
}

export interface SetIDListResponse {
  tenant_id: string
  setids: SetIDAPIItem[]
}

export async function listSetIDs(): Promise<SetIDListResponse> {
  return httpClient.get<SetIDListResponse>('/org/api/setids')
}

export interface SetIDBindingAPIItem {
  org_unit_id: string
  setid: string
  valid_from: string
  valid_to: string
}

export interface SetIDBindingListResponse {
  tenant_id: string
  as_of: string
  bindings: SetIDBindingAPIItem[]
}

export async function listSetIDBindings(options: { asOf: string }): Promise<SetIDBindingListResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  return httpClient.get<SetIDBindingListResponse>(`/org/api/setid-bindings?${query.toString()}`)
}

export async function createSetID(request: {
  setid: string
  name: string
  effective_date: string
  request_id: string
}): Promise<{ setid: string; status: string }> {
  return httpClient.post<{ setid: string; status: string }>('/org/api/setids', request)
}

export async function bindSetID(request: {
  org_code: string
  setid: string
  effective_date: string
  request_id: string
}): Promise<{ org_code: string; setid: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; setid: string; effective_date: string }>('/org/api/setid-bindings', request)
}

export interface SetIDStrategyRegistryItem {
  capability_key: string
  owner_module: string
  field_key: string
  personalization_mode: 'tenant_only' | 'setid' | 'scope_package'
  scope_code?: string
  org_level: 'tenant' | 'business_unit'
  business_unit_id?: string
  required: boolean
  visible: boolean
  default_rule_ref?: string
  default_value?: string
  priority: number
  explain_required: boolean
  is_stable: boolean
  change_policy: string
  effective_date: string
  end_date?: string
  updated_at: string
}

export interface SetIDStrategyRegistryListResponse {
  tenant_id: string
  as_of: string
  items: SetIDStrategyRegistryItem[]
}

export interface SetIDStrategyRegistryUpsertRequest {
  capability_key: string
  owner_module: string
  field_key: string
  personalization_mode: 'tenant_only' | 'setid' | 'scope_package'
  scope_code: string
  org_level: 'tenant' | 'business_unit'
  business_unit_id: string
  required: boolean
  visible: boolean
  default_rule_ref: string
  default_value: string
  priority: number
  explain_required: boolean
  is_stable: boolean
  change_policy: string
  effective_date: string
  end_date: string
  request_id: string
}

export async function listSetIDStrategyRegistry(options: {
  asOf: string
  capabilityKey?: string
  fieldKey?: string
}): Promise<SetIDStrategyRegistryListResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  if (options.capabilityKey && options.capabilityKey.trim().length > 0) {
    query.set('capability_key', options.capabilityKey.trim())
  }
  if (options.fieldKey && options.fieldKey.trim().length > 0) {
    query.set('field_key', options.fieldKey.trim())
  }
  return httpClient.get<SetIDStrategyRegistryListResponse>(`/org/api/setid-strategy-registry?${query.toString()}`)
}

export async function upsertSetIDStrategyRegistry(
  request: SetIDStrategyRegistryUpsertRequest
): Promise<SetIDStrategyRegistryItem> {
  return httpClient.post<SetIDStrategyRegistryItem>('/org/api/setid-strategy-registry', request)
}

export interface SetIDExplainFieldDecision {
  capability_key: string
  field_key: string
  required: boolean
  visible: boolean
  default_rule_ref?: string
  resolved_default_value?: string
  decision: string
  reason_code?: string
}

export interface SetIDExplainResponse {
  trace_id: string
  request_id: string
  capability_key: string
  tenant_id?: string
  business_unit_id: string
  as_of: string
  org_unit_id?: string
  scope_code: string
  resolved_setid: string
  resolved_package_id: string
  package_owner: string
  decision: string
  reason_code?: string
  level: 'brief' | 'full'
  field_decisions: SetIDExplainFieldDecision[]
}

export async function getSetIDExplain(request: {
  capabilityKey: string
  fieldKey: string
  businessUnitID: string
  scopeCode: string
  asOf: string
  level?: 'brief' | 'full'
  setID?: string
  orgUnitID?: string
  requestID?: string
}): Promise<SetIDExplainResponse> {
  const query = new URLSearchParams({
    capability_key: request.capabilityKey.trim(),
    field_key: request.fieldKey.trim(),
    business_unit_id: request.businessUnitID.trim(),
    scope_code: request.scopeCode.trim(),
    as_of: request.asOf.trim()
  })
  if (request.level) {
    query.set('level', request.level)
  }
  if (request.setID && request.setID.trim().length > 0) {
    query.set('setid', request.setID.trim())
  }
  if (request.orgUnitID && request.orgUnitID.trim().length > 0) {
    query.set('org_unit_id', request.orgUnitID.trim())
  }
  if (request.requestID && request.requestID.trim().length > 0) {
    query.set('request_id', request.requestID.trim())
  }
  return httpClient.get<SetIDExplainResponse>(`/org/api/setid-explain?${query.toString()}`)
}
