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
  }
  dry_run: {
    explain: string
    diff: Array<Record<string, unknown>>
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
