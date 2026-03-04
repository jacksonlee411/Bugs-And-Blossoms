import { httpClient } from './httpClient'

export interface AssistantCandidate {
  candidate_id: string
  candidate_code: string
  name: string
  path: string
  as_of: string
  is_active: boolean
  match_score: number
}

export interface AssistantIntentSpec {
  action: string
  parent_ref_text?: string
  entity_name?: string
  effective_date?: string
  intent_schema_version?: string
  context_hash?: string
  intent_hash?: string
}

export interface AssistantSkillExecutionPlan {
  selected_skills: string[]
  execution_order: string[]
  risk_tier: string
  required_checks: string[]
}

export interface AssistantConfigChange {
  field: string
  after: unknown
}

export interface AssistantConfigDeltaPlan {
  capability_key: string
  changes: AssistantConfigChange[]
}

export interface AssistantTurn {
  turn_id: string
  user_input: string
  state: string
  risk_tier: 'low' | 'medium' | 'high'
  request_id: string
  trace_id: string
  policy_version: string
  composition_version: string
  mapping_version: string
  intent: AssistantIntentSpec
  ambiguity_count: number
  confidence: number
  resolved_candidate_id?: string
  resolution_source?: 'auto' | 'user_confirmed'
  candidates: AssistantCandidate[]
  plan: {
    title: string
    capability_key: string
    summary: string
    capability_map_version?: string
    compiler_contract_version?: string
    skill_manifest_digest?: string
    model_provider?: string
    model_name?: string
    model_revision?: string
    skill_execution_plan?: AssistantSkillExecutionPlan
    config_delta_plan?: AssistantConfigDeltaPlan
  }
  dry_run: {
    explain: string
    diff: Array<Record<string, unknown>>
    validation_errors?: string[]
    would_commit?: boolean
    plan_hash?: string
  }
  commit_result?: {
    org_code: string
    parent_org_code: string
    effective_date: string
    event_type: string
    event_uuid: string
  }
}

export interface AssistantConversation {
  conversation_id: string
  tenant_id: string
  actor_id: string
  actor_role: string
  state: string
  created_at: string
  updated_at: string
  turns: AssistantTurn[]
}

export interface AssistantConversationListItem {
  conversation_id: string
  state: string
  updated_at: string
  last_turn?: {
    turn_id: string
    user_input: string
    state: string
    risk_tier: string
  }
}

export interface AssistantConversationListResponse {
  items: AssistantConversationListItem[]
  next_cursor: string
}

export interface AssistantTaskContractSnapshot {
  intent_schema_version: string
  compiler_contract_version: string
  capability_map_version: string
  skill_manifest_digest: string
  context_hash: string
  intent_hash: string
  plan_hash: string
}

export interface AssistantTaskSubmitRequest {
  conversation_id: string
  turn_id: string
  task_type: 'assistant_async_plan'
  request_id: string
  trace_id?: string
  contract_snapshot: AssistantTaskContractSnapshot
}

export interface AssistantTaskAsyncReceipt {
  task_id: string
  task_type: string
  status: string
  workflow_id: string
  submitted_at: string
  poll_uri: string
}

export interface AssistantTaskDetail {
  task_id: string
  task_type: string
  status: string
  dispatch_status: string
  attempt: number
  max_attempts: number
  last_error_code?: string
  workflow_id: string
  request_id: string
  trace_id?: string
  conversation_id: string
  turn_id: string
  submitted_at: string
  cancel_requested_at?: string
  completed_at?: string
  updated_at: string
  contract_snapshot: AssistantTaskContractSnapshot
}

export interface AssistantTaskCancelResponse extends AssistantTaskDetail {
  cancel_accepted: boolean
}

export interface AssistantModelProvider {
  name: string
  enabled: boolean
  model: string
  endpoint: string
  timeout_ms: number
  retries: number
  priority: number
  key_ref: string
}

export interface AssistantModelProviderView extends AssistantModelProvider {
  healthy: 'healthy' | 'degraded' | 'unavailable' | 'disabled'
  health_reason?: string
}

export interface AssistantModelProvidersResponse {
  provider_routing: {
    strategy: string
    fallback_enabled: boolean
  }
  providers: AssistantModelProviderView[]
}

export interface AssistantModelConfigPayload {
  provider_routing: {
    strategy: string
    fallback_enabled: boolean
  }
  providers: AssistantModelProvider[]
}

export interface AssistantModelProvidersValidateResponse {
  valid: boolean
  errors?: string[]
  normalized: AssistantModelConfigPayload
}

export interface AssistantModelsResponse {
  models: Array<{
    provider: string
    model: string
  }>
}

export interface AssistantRuntimeUpstream {
  url?: string
  repo?: string
  ref?: string
  imported_at?: string
  rollback_ref?: string
}

export interface AssistantRuntimeService {
  name: string
  required: boolean
  healthy: 'healthy' | 'degraded' | 'unavailable'
  reason?: string
  image?: string
  tag?: string
  digest?: string
}

export interface AssistantRuntimeStatusResponse {
  status: 'healthy' | 'degraded' | 'unavailable'
  checked_at: string
  error_code?: string
  error_message?: string
  code?: string
  message?: string
  upstream: AssistantRuntimeUpstream
  services: AssistantRuntimeService[]
  capabilities?: {
    mcp_enabled: boolean
    actions_enabled: boolean
    agents_write_enabled: boolean
    domain_policy_version?: string
  }
}

export async function createAssistantConversation(): Promise<AssistantConversation> {
  return httpClient.post<AssistantConversation>('/internal/assistant/conversations', {})
}

export async function listAssistantConversations(params?: {
  page_size?: number
  cursor?: string
}): Promise<AssistantConversationListResponse> {
  const query = new URLSearchParams()
  if (typeof params?.page_size === 'number' && Number.isFinite(params.page_size)) {
    query.set('page_size', String(params.page_size))
  }
  if (typeof params?.cursor === 'string' && params.cursor.trim().length > 0) {
    query.set('cursor', params.cursor.trim())
  }
  const suffix = query.toString()
  const path = suffix.length > 0 ? `/internal/assistant/conversations?${suffix}` : '/internal/assistant/conversations'
  return httpClient.get<AssistantConversationListResponse>(path)
}

export async function createAssistantTurn(conversationID: string, userInput: string): Promise<AssistantConversation> {
  return httpClient.post<AssistantConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns`,
    { user_input: userInput }
  )
}

export async function getAssistantConversation(conversationID: string): Promise<AssistantConversation> {
  return httpClient.get<AssistantConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationID)}`
  )
}

export async function confirmAssistantTurn(
  conversationID: string,
  turnID: string,
  candidateID?: string
): Promise<AssistantConversation> {
  return httpClient.post<AssistantConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turnID)}:confirm`,
    candidateID ? { candidate_id: candidateID } : {}
  )
}

export async function commitAssistantTurn(conversationID: string, turnID: string): Promise<AssistantConversation> {
  return httpClient.post<AssistantConversation>(
    `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turnID)}:commit`,
    {}
  )
}

export async function submitAssistantTask(payload: AssistantTaskSubmitRequest): Promise<AssistantTaskAsyncReceipt> {
  return httpClient.post<AssistantTaskAsyncReceipt>('/internal/assistant/tasks', payload)
}

export async function getAssistantTask(taskID: string): Promise<AssistantTaskDetail> {
  return httpClient.get<AssistantTaskDetail>(`/internal/assistant/tasks/${encodeURIComponent(taskID)}`)
}

export async function cancelAssistantTask(taskID: string): Promise<AssistantTaskCancelResponse> {
  return httpClient.post<AssistantTaskCancelResponse>(`/internal/assistant/tasks/${encodeURIComponent(taskID)}:cancel`, {})
}

export async function getAssistantModelProviders(): Promise<AssistantModelProvidersResponse> {
  return httpClient.get<AssistantModelProvidersResponse>('/internal/assistant/model-providers')
}

export async function validateAssistantModelProviders(
  payload: AssistantModelConfigPayload
): Promise<AssistantModelProvidersValidateResponse> {
  return httpClient.post<AssistantModelProvidersValidateResponse>('/internal/assistant/model-providers:validate', payload)
}

export async function getAssistantModels(): Promise<AssistantModelsResponse> {
  return httpClient.get<AssistantModelsResponse>('/internal/assistant/models')
}

export async function getAssistantRuntimeStatus(): Promise<AssistantRuntimeStatusResponse> {
  try {
    return await httpClient.get<AssistantRuntimeStatusResponse>('/internal/assistant/runtime-status')
  } catch (error) {
    const details = (error as { details?: unknown })?.details
    if (details && typeof details === 'object') {
      const candidate = details as Partial<AssistantRuntimeStatusResponse>
      if (typeof candidate.status === 'string' && Array.isArray(candidate.services) && candidate.upstream) {
        return candidate as AssistantRuntimeStatusResponse
      }
    }
    throw error
  }
}
