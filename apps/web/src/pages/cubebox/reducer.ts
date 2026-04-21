import type { CanonicalEvent, ConversationReplayResponse, CubeBoxState, TimelineItem } from './types'

export const initialCubeBoxState: CubeBoxState = {
  conversation: null,
  items: [],
  turnStatus: 'idle',
  activeTurnID: null,
  nextSequence: 1,
  composerText: '',
  loading: false,
  errorMessage: null
}

type CubeBoxAction =
  | { type: 'loading_started' }
  | { type: 'loading_finished' }
  | { type: 'composer_changed'; value: string }
  | { type: 'conversation_loaded'; payload: ConversationReplayResponse }
  | { type: 'event_received'; payload: CanonicalEvent }
  | { type: 'error_message_set'; message: string | null }
  | { type: 'stream_failed_locally'; message: string }
  | { type: 'reset' }

export function cubeboxReducer(state: CubeBoxState, action: CubeBoxAction): CubeBoxState {
  switch (action.type) {
    case 'loading_started':
      return { ...state, loading: true, errorMessage: null }
    case 'loading_finished':
      return { ...state, loading: false }
    case 'composer_changed':
      return { ...state, composerText: action.value }
    case 'conversation_loaded':
      if (state.conversation && state.conversation.id === action.payload.conversation.id && state.items.length > 0) {
        return {
          ...state,
          conversation: action.payload.conversation,
          nextSequence: action.payload.next_sequence
        }
      }
      return replayConversation(action.payload)
    case 'event_received':
      return applyEvent(state, action.payload)
    case 'error_message_set':
      return { ...state, errorMessage: action.message, loading: false }
    case 'stream_failed_locally':
      return {
        ...state,
        activeTurnID: null,
        errorMessage: action.message,
        loading: false,
        turnStatus: 'error'
      }
    case 'reset':
      return initialCubeBoxState
    default:
      return state
  }
}

export function replayConversation(payload: ConversationReplayResponse): CubeBoxState {
  return payload.events.reduce<CubeBoxState>(
    (state, event) => applyEvent(state, event),
    {
      ...initialCubeBoxState,
      conversation: payload.conversation,
      nextSequence: payload.next_sequence
    }
  )
}

function applyEvent(state: CubeBoxState, event: CanonicalEvent): CubeBoxState {
  const nextSequence = Math.max(state.nextSequence, event.sequence + 1)

  switch (event.type) {
    case 'conversation.loaded':
      return {
        ...state,
        conversation: {
          id: event.conversation_id,
          title: String(event.payload.title ?? state.conversation?.title ?? '新对话'),
          status: normalizeStatus(event.payload.status),
          archived: Boolean(event.payload.archived)
        },
        nextSequence
      }
    case 'turn.started':
      return {
        ...state,
        activeTurnID: event.turn_id,
        turnStatus: 'streaming',
        errorMessage: null,
        nextSequence
      }
    case 'turn.user_message.accepted':
      return {
        ...state,
        items: appendOrReplaceItem(state.items, {
          id: String(event.payload.message_id ?? `user-${event.sequence}`),
          kind: 'user_message',
          text: String(event.payload.text ?? ''),
          status: 'completed'
        }),
        composerText: '',
        nextSequence
      }
    case 'turn.agent_message.delta':
      return {
        ...state,
        items: mergeAgentDelta(state.items, String(event.payload.message_id ?? `agent-${event.sequence}`), String(event.payload.delta ?? '')),
        turnStatus: 'streaming',
        nextSequence
      }
    case 'turn.agent_message.completed':
      return {
        ...state,
        items: finalizeItem(state.items, String(event.payload.message_id ?? '')),
        nextSequence
      }
    case 'turn.error':
      return {
        ...state,
        activeTurnID: null,
        items: appendOrReplaceItem(state.items, {
          id: `error-${event.sequence}`,
          kind: 'error_item',
          text: String(event.payload.message ?? 'unknown error'),
          status: 'error'
        }),
        turnStatus: 'error',
        errorMessage: String(event.payload.message ?? 'unknown error'),
        nextSequence
      }
    case 'turn.interrupted':
      return {
        ...state,
        turnStatus: 'interrupted',
        items: markStreamingInterrupted(state.items),
        nextSequence
      }
    case 'turn.completed':
      return {
        ...state,
        activeTurnID: null,
        turnStatus: normalizeTurnStatus(event.payload.status),
        nextSequence
      }
    default:
      return { ...state, nextSequence }
  }
}

function appendOrReplaceItem(items: TimelineItem[], nextItem: TimelineItem): TimelineItem[] {
  const index = items.findIndex((item) => item.id === nextItem.id)
  if (index < 0) {
    return [...items, nextItem]
  }
  return items.map((item, currentIndex) => (currentIndex === index ? nextItem : item))
}

function mergeAgentDelta(items: TimelineItem[], messageID: string, delta: string): TimelineItem[] {
  const existing = items.find((item) => item.id === messageID)
  if (!existing) {
    return [
      ...items,
      {
        id: messageID,
        kind: 'agent_message',
        text: delta,
        status: 'streaming'
      }
    ]
  }

  return items.map((item) =>
    item.id === messageID
      ? {
          ...item,
          kind: 'agent_message',
          text: `${item.text}${delta}`,
          status: 'streaming'
        }
      : item
  )
}

function finalizeItem(items: TimelineItem[], messageID: string): TimelineItem[] {
  return items.map((item) => (item.id === messageID ? { ...item, status: 'completed' } : item))
}

function markStreamingInterrupted(items: TimelineItem[]): TimelineItem[] {
  return items.map((item) => (item.status === 'streaming' ? { ...item, status: 'interrupted' } : item))
}

function normalizeTurnStatus(input: unknown): CubeBoxState['turnStatus'] {
  const value = String(input ?? '').trim().toLowerCase()
  if (value === 'completed') {
    return 'completed'
  }
  if (value === 'failed') {
    return 'error'
  }
  if (value === 'interrupted') {
    return 'interrupted'
  }
  return 'idle'
}

function normalizeStatus(input: unknown): 'active' | 'archived' {
  return String(input ?? '').trim().toLowerCase() === 'archived' ? 'archived' : 'active'
}
