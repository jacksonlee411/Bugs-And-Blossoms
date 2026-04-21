import type { CanonicalEvent, ConversationReplayResponse, CubeBoxConversationListResponse } from './types'

export async function createConversation(): Promise<ConversationReplayResponse> {
  const response = await fetch('/internal/cubebox/conversations', {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: '{}'
  })
  if (!response.ok) {
    throw new Error(`create conversation failed: ${response.status}`)
  }
  return (await response.json()) as ConversationReplayResponse
}

export async function loadConversation(conversationID: string): Promise<ConversationReplayResponse> {
  const response = await fetch(`/internal/cubebox/conversations/${conversationID}`, {
    credentials: 'include',
    method: 'GET'
  })
  if (!response.ok) {
    throw new Error(`load conversation failed: ${response.status}`)
  }
  return (await response.json()) as ConversationReplayResponse
}

export async function listConversations(): Promise<CubeBoxConversationListResponse> {
  const response = await fetch('/internal/cubebox/conversations?limit=20', {
    credentials: 'include',
    method: 'GET'
  })
  if (!response.ok) {
    throw new Error(`list conversations failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxConversationListResponse
}

export async function updateConversation(input: {
  conversationID: string
  title?: string
  archived?: boolean
}): Promise<ConversationReplayResponse> {
  const response = await fetch(`/internal/cubebox/conversations/${input.conversationID}`, {
    credentials: 'include',
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      ...(typeof input.title === 'string' ? { title: input.title } : {}),
      ...(typeof input.archived === 'boolean' ? { archived: input.archived } : {})
    })
  })
  if (!response.ok) {
    throw new Error(`update conversation failed: ${response.status}`)
  }
  return (await response.json()) as ConversationReplayResponse
}

export async function streamTurn(input: {
  conversationID: string
  prompt: string
  nextSequence: number
  signal: AbortSignal
  onEvent: (event: CanonicalEvent) => void
}): Promise<void> {
  const response = await fetch('/internal/cubebox/turns:stream', {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      conversation_id: input.conversationID,
      prompt: input.prompt,
      next_sequence: input.nextSequence
    }),
    signal: input.signal
  })
  if (!response.ok || !response.body) {
    throw new Error(`stream turn failed: ${response.status}`)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let sawTerminalEvent = false

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split('\n\n')
    buffer = parts.pop() ?? ''

    for (const part of parts) {
      const line = part
        .split('\n')
        .find((entry) => entry.startsWith('data: '))
      if (!line) {
        continue
      }
      const event = JSON.parse(line.slice(6)) as CanonicalEvent
      if (event.type === 'turn.completed' || event.type === 'turn.error') {
        sawTerminalEvent = true
      }
      input.onEvent(event)
    }
  }

  if (!sawTerminalEvent) {
    throw new Error('stream turn failed: missing terminal event')
  }
}

export async function interruptTurn(turnID: string, conversationID: string): Promise<void> {
  const response = await fetch(`/internal/cubebox/turns/${turnID}:interrupt?conversation_id=${encodeURIComponent(conversationID)}`, {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ reason: 'user_requested' })
  })
  if (!response.ok) {
    throw new Error(`interrupt turn failed: ${response.status}`)
  }
}
