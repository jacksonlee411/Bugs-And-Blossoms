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
    | 'turn.error'
    | 'turn.interrupted'
    | 'turn.completed'
  ts: string
  payload: Record<string, unknown>
}

export interface ConversationReplayResponse {
  conversation: CubeBoxConversation
  events: CanonicalEvent[]
  next_sequence: number
}

export interface TimelineItem {
  id: string
  kind: 'user_message' | 'agent_message' | 'error_item'
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
