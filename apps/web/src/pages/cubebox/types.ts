export interface CubeBoxConversation {
  id: string
  title: string
  status: 'active' | 'archived'
  archived: boolean
}

export interface CubeBoxConversationSummary {
  id: string
  title: string
  status: 'active' | 'archived'
  archived: boolean
  updated_at: string
}

export interface CubeBoxConversationListResponse {
  items: CubeBoxConversationSummary[]
}

export interface CanonicalEvent {
  event_id: string
  conversation_id: string
  turn_id: string | null
  sequence: number
  type:
    | 'conversation.loaded'
    | 'conversation.renamed'
    | 'conversation.archived'
    | 'conversation.unarchived'
    | 'turn.started'
    | 'turn.user_message.accepted'
    | 'turn.agent_message.delta'
    | 'turn.agent_message.completed'
    | 'turn.context_compacted'
    | 'turn.query_entity.confirmed'
    | 'turn.error'
    | 'turn.interrupted'
    | 'turn.completed'
  ts: string
  payload: Record<string, unknown>
}

export interface CubeBoxPageContext {
  page?: string
  business_object?: string
  current_object?: {
    domain?: string
    entity_key?: string
    label?: string
  }
  view?: {
    as_of?: string
  }
}

export interface ConversationReplayResponse {
  conversation: CubeBoxConversation
  events: CanonicalEvent[]
  next_sequence: number
}

export interface TimelineItem {
  id: string
  kind: 'user_message' | 'agent_message' | 'error_item' | 'compact_item'
  text: string
  status?: 'streaming' | 'completed' | 'error' | 'interrupted'
}

export interface CubeBoxState {
  conversation: CubeBoxConversation | null
  items: TimelineItem[]
  turnStatus: 'idle' | 'streaming' | 'completed' | 'error' | 'interrupted'
  activeTurnID: string | null
  nextSequence: number
  composerText: string
  loading: boolean
  errorMessage: string | null
}

export interface CubeBoxModelProvider {
  id: string
  provider_type: string
  display_name: string
  base_url: string
  enabled: boolean
  updated_at: string
  disabled_at?: string
}

export interface CubeBoxModelCredential {
  id: string
  provider_id: string
  secret_ref: string
  masked_secret: string
  version: number
  active: boolean
  created_at: string
  disabled_at?: string
}

export interface CubeBoxActiveModelSelection {
  provider_id: string
  model_slug: string
  capability_summary: Record<string, unknown>
  updated_at: string
}

export interface CubeBoxModelHealth {
  id: string
  provider_id: string
  model_slug: string
  status: 'healthy' | 'degraded' | 'failed' | 'unknown'
  latency_ms?: number
  error_summary?: string
  validated_at: string
}

export interface CubeBoxModelSettingsSnapshot {
  providers: CubeBoxModelProvider[]
  credentials: CubeBoxModelCredential[]
  selection?: CubeBoxActiveModelSelection | null
  health?: CubeBoxModelHealth | null
}

export interface CubeBoxCapabilities {
  conversation: {
    read: boolean
    use: boolean
  }
  settings: {
    read: boolean
    verify: boolean
    select: boolean
    update: boolean
    rotate: boolean
    deactivate: boolean
  }
}
