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
  request_code: string
}): Promise<{ setid: string; status: string }> {
  return httpClient.post<{ setid: string; status: string }>('/org/api/setids', request)
}

export async function bindSetID(request: {
  org_code: string
  setid: string
  effective_date: string
  request_code: string
}): Promise<{ org_code: string; setid: string; effective_date: string }> {
  return httpClient.post<{ org_code: string; setid: string; effective_date: string }>('/org/api/setid-bindings', request)
}

