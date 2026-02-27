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
  setid?: string
  setid_source?: 'custom' | 'deflt' | 'share_preview'
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

export interface DictReleaseConflict {
  kind: string
  dict_code: string
  code?: string
  source_value?: string
  target_value?: string
}

export interface DictReleasePreviewResponse {
  release_id: string
  source_tenant_id: string
  target_tenant_id: string
  as_of: string
  source_dict_count: number
  source_value_count: number
  target_dict_count: number
  target_value_count: number
  missing_dict_count: number
  dict_name_mismatch_count: number
  missing_value_count: number
  value_label_mismatch_count: number
  conflicts: DictReleaseConflict[]
}

export interface DictReleaseResultResponse {
  task_id: string
  release_id: string
  request_id: string
  source_tenant_id: string
  target_tenant_id: string
  as_of: string
  status: string
  dict_events_total: number
  dict_events_applied: number
  dict_events_retried: number
  value_events_total: number
  value_events_applied: number
  value_events_retried: number
  started_at: string
  finished_at: string
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

export async function previewDictRelease(request: {
  source_tenant_id: string
  as_of: string
  release_id: string
  max_conflicts?: number
}): Promise<DictReleasePreviewResponse> {
  return httpClient.post<DictReleasePreviewResponse>('/iam/api/dicts:release:preview', request)
}

export async function executeDictRelease(request: {
  source_tenant_id: string
  as_of: string
  release_id: string
  request_id: string
  max_conflicts?: number
}): Promise<DictReleaseResultResponse> {
  return httpClient.post<DictReleaseResultResponse>('/iam/api/dicts:release', request)
}
