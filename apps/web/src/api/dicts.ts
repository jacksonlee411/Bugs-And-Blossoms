import { httpClient } from './httpClient'

export interface DictItem {
  dict_code: string
  name: string
  status: string
  enabled_on: string
  disabled_on?: string | null
}

export interface DictListResponse {
  as_of: string
  dicts: DictItem[]
}

export interface DictValueItem {
  dict_code: string
  code: string
  label: string
  status: string
  enabled_on: string
  disabled_on?: string | null
  updated_at: string
}

export interface DictValuesResponse {
  dict_code: string
  as_of: string
  values: DictValueItem[]
}

export interface DictMutationResponse extends DictValueItem {
  was_retry: boolean
}

export interface DictCodeMutationResponse extends DictItem {
  was_retry: boolean
}

export interface DictAuditItem {
  event_id: number
  event_uuid: string
  dict_code: string
  code: string
  event_type: string
  effective_day: string
  request_id: string
  initiator_uuid: string
  tx_time: string
  payload: unknown
  before_snapshot: unknown
  after_snapshot: unknown
}

export interface DictAuditResponse {
  dict_code: string
  code: string
  limit: number
  events: DictAuditItem[]
}

export async function listDicts(asOf: string): Promise<DictListResponse> {
  const query = new URLSearchParams({ as_of: asOf })
  return httpClient.get<DictListResponse>(`/iam/api/dicts?${query.toString()}`)
}

export async function listDictValues(options: {
  dictCode: string
  asOf: string
  q?: string
  limit?: number
  status?: 'active' | 'inactive' | 'all'
}): Promise<DictValuesResponse> {
  const query = new URLSearchParams({
    dict_code: options.dictCode,
    as_of: options.asOf
  })
  if (options.q && options.q.trim().length > 0) {
    query.set('q', options.q.trim())
  }
  if (options.limit) {
    query.set('limit', String(options.limit))
  }
  if (options.status) {
    query.set('status', options.status)
  }
  return httpClient.get<DictValuesResponse>(`/iam/api/dicts/values?${query.toString()}`)
}

export async function createDict(request: {
  dict_code: string
  name: string
  enabled_on: string
  request_id: string
}): Promise<DictCodeMutationResponse> {
  return httpClient.post<DictCodeMutationResponse>('/iam/api/dicts', request)
}

export async function disableDict(request: {
  dict_code: string
  disabled_on: string
  request_id: string
}): Promise<DictCodeMutationResponse> {
  return httpClient.post<DictCodeMutationResponse>('/iam/api/dicts:disable', request)
}

export async function createDictValue(request: {
  dict_code: string
  code: string
  label: string
  enabled_on: string
  request_id: string
}): Promise<DictMutationResponse> {
  return httpClient.post<DictMutationResponse>('/iam/api/dicts/values', request)
}

export async function disableDictValue(request: {
  dict_code: string
  code: string
  disabled_on: string
  request_id: string
}): Promise<DictMutationResponse> {
  return httpClient.post<DictMutationResponse>('/iam/api/dicts/values:disable', request)
}

export async function correctDictValue(request: {
  dict_code: string
  code: string
  label: string
  correction_day: string
  request_id: string
}): Promise<DictMutationResponse> {
  return httpClient.post<DictMutationResponse>('/iam/api/dicts/values:correct', request)
}

export async function listDictAudit(options: {
  dictCode: string
  code: string
  limit?: number
}): Promise<DictAuditResponse> {
  const query = new URLSearchParams({
    dict_code: options.dictCode,
    code: options.code
  })
  if (options.limit) {
    query.set('limit', String(options.limit))
  }
  return httpClient.get<DictAuditResponse>(`/iam/api/dicts/values/audit?${query.toString()}`)
}
