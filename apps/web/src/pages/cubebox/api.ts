import type { CanonicalEvent, ConversationReplayResponse } from './types'

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
      input.onEvent(JSON.parse(line.slice(6)) as CanonicalEvent)
    }
  }
}

export async function interruptTurn(turnID: string): Promise<void> {
  const response = await fetch(`/internal/cubebox/turns/${turnID}:interrupt`, {
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
