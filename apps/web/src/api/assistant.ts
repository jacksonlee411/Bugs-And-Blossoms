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
  created_at: string
  updated_at: string
  turns: AssistantTurn[]
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

export interface AssistantModelProvidersApplyResponse {
  applied_at: string
  applied_by: string
  normalized: AssistantModelConfigPayload
}

export interface AssistantModelsResponse {
  models: Array<{
    provider: string
    model: string
  }>
}

export async function createAssistantConversation(): Promise<AssistantConversation> {
  return httpClient.post<AssistantConversation>('/internal/assistant/conversations', {})
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

export async function getAssistantModelProviders(): Promise<AssistantModelProvidersResponse> {
  return httpClient.get<AssistantModelProvidersResponse>('/internal/assistant/model-providers')
}

export async function validateAssistantModelProviders(
  payload: AssistantModelConfigPayload
): Promise<AssistantModelProvidersValidateResponse> {
  return httpClient.post<AssistantModelProvidersValidateResponse>('/internal/assistant/model-providers:validate', payload)
}

export async function applyAssistantModelProviders(
  payload: AssistantModelConfigPayload
): Promise<AssistantModelProvidersApplyResponse> {
  return httpClient.post<AssistantModelProvidersApplyResponse>('/internal/assistant/model-providers:apply', payload)
}

export async function getAssistantModels(): Promise<AssistantModelsResponse> {
  return httpClient.get<AssistantModelsResponse>('/internal/assistant/models')
}
