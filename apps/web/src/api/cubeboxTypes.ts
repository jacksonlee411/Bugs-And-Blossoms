export interface CubeBoxCandidate {
  candidate_id: string
  candidate_code: string
  name: string
  path: string
  as_of: string
  is_active: boolean
  match_score: number
}

export interface CubeBoxIntentSpec {
  action: string
  parent_ref_text?: string
  entity_name?: string
  effective_date?: string
  intent_schema_version?: string
  context_hash?: string
  intent_hash?: string
}

export interface CubeBoxSkillExecutionPlan {
  selected_skills: string[]
  execution_order: string[]
  risk_tier: string
  required_checks: string[]
}

export interface CubeBoxConfigChange {
  field: string
  after: unknown
}

export interface CubeBoxConfigDeltaPlan {
  capability_key: string
  changes: CubeBoxConfigChange[]
}

export interface CubeBoxReply {
  text: string
  kind: 'info' | 'warning' | 'success' | 'error'
  stage: 'draft' | 'missing_fields' | 'candidate_list' | 'candidate_confirm' | 'commit_result' | 'commit_failed'
  reply_model_name: string
  reply_prompt_version: string
  reply_source: 'model'
  used_fallback: boolean
  conversation_id: string
  turn_id: string
}

export interface CubeBoxTurn {
  turn_id: string
  user_input: string
  state: string
  risk_tier: 'low' | 'medium' | 'high'
  request_id: string
  trace_id: string
  policy_version: string
  composition_version: string
  mapping_version: string
  intent: CubeBoxIntentSpec
  ambiguity_count: number
  confidence: number
  resolved_candidate_id?: string
  resolution_source?: 'auto' | 'user_confirmed'
  candidates: CubeBoxCandidate[]
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
    skill_execution_plan?: CubeBoxSkillExecutionPlan
    config_delta_plan?: CubeBoxConfigDeltaPlan
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
  reply_nlg?: CubeBoxReply
}

export interface CubeBoxConversation {
  conversation_id: string
  tenant_id: string
  actor_id: string
  actor_role: string
  state: string
  created_at: string
  updated_at: string
  turns: CubeBoxTurn[]
}

export interface CubeBoxConversationListItem {
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

export interface CubeBoxConversationListResponse {
  items: CubeBoxConversationListItem[]
  next_cursor: string
}

export interface CubeBoxTaskContractSnapshot {
  intent_schema_version: string
  compiler_contract_version: string
  capability_map_version: string
  skill_manifest_digest: string
  context_hash: string
  intent_hash: string
  plan_hash: string
}

export interface CubeBoxTaskSubmitRequest {
  conversation_id: string
  turn_id: string
  task_type: 'assistant_async_plan'
  request_id: string
  trace_id?: string
  contract_snapshot: CubeBoxTaskContractSnapshot
}

export interface CubeBoxTaskReceipt {
  task_id: string
  task_type: string
  status: string
  workflow_id: string
  submitted_at: string
  poll_uri: string
}

export interface CubeBoxTask {
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
  contract_snapshot: CubeBoxTaskContractSnapshot
}

export interface CubeBoxTaskCancelResponse extends CubeBoxTask {
  cancel_accepted: boolean
}

export interface CubeBoxFormalViewer {
  id: string
  username: string
  email: string
  name: string
  role: 'USER' | 'ADMIN' | string
}

export interface CubeBoxUIBootstrapResponse {
  contract_version: 'v1' | string
  viewer: CubeBoxFormalViewer
  ui: {
    model_select: boolean
    artifacts_enabled: boolean
    agents_ui_enabled: boolean
    memory_enabled: boolean
    web_search_enabled: boolean
    file_search_enabled: boolean
    code_interpreter_enabled: boolean
  }
  models: Array<{
    endpoint_key: string
    endpoint_type: string
    provider: string
    model: string
    label: string
  }>
  runtime: {
    status: 'healthy' | 'degraded' | 'unavailable'
    runtime_cutover_mode: 'cutover-prep' | 'ui-shell-only' | string
    domain_policy_version: string
  }
}

export interface CubeBoxSessionResponse {
  contract_version: 'v1' | string
  authenticated: boolean
  viewer: CubeBoxFormalViewer
}

export interface CubeBoxSessionRefreshResponse extends CubeBoxSessionResponse {
  refreshed_at: string
}

export interface CubeBoxRuntimeComponent {
  healthy: 'healthy' | 'degraded' | 'unavailable' | string
  reason?: string
}

export interface CubeBoxRuntimeCapabilities {
  conversation_enabled: boolean
  files_enabled: boolean
  agents_ui_enabled: boolean
  agents_write_enabled: boolean
  memory_enabled: boolean
  web_search_enabled: boolean
  file_search_enabled: boolean
  mcp_enabled: boolean
}

export interface CubeBoxRuntimeStatusResponse {
  status: 'healthy' | 'degraded' | 'unavailable' | string
  checked_at: string
  frontend: CubeBoxRuntimeComponent
  backend: CubeBoxRuntimeComponent
  knowledge_runtime: CubeBoxRuntimeComponent
  model_gateway: CubeBoxRuntimeComponent
  file_store: CubeBoxRuntimeComponent
  retired_capabilities: string[]
  capabilities: CubeBoxRuntimeCapabilities
}

export interface CubeBoxRenderReplyRequest {
  stage?: string
  kind?: string
  outcome?: 'success' | 'failure'
  locale?: 'zh' | 'en'
  fallback_text?: string
  allow_missing_turn?: boolean
}
