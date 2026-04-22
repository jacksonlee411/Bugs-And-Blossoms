import { ApiClientError } from '../../api/errors'
import { resolveApiErrorMessage } from '../../errors/presentApiError'
import type { CanonicalEvent, CompactConversationResponse, ConversationReplayResponse, CubeBoxConversationListResponse } from './types'

async function readError(response: Response, fallbackCode: string, fallbackMessage: string): Promise<never> {
  let code = fallbackCode
  let message = fallbackMessage

  try {
    const payload = (await response.json()) as { code?: string; message?: string; trace_id?: string }
    code = payload.code?.trim() || fallbackCode
    message = resolveApiErrorMessage(payload.code, payload.message || fallbackMessage)
    throw new ApiClientError(message, response.status === 401 ? 'UNAUTHORIZED' : 'UNKNOWN_ERROR', response.status, payload.trace_id, payload)
  } catch (error) {
    if (error instanceof ApiClientError) {
      throw error
    }
  }

  throw new ApiClientError(resolveApiErrorMessage(code, fallbackMessage), response.status === 401 ? 'UNAUTHORIZED' : 'UNKNOWN_ERROR', response.status)
}

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
    await readError(response, 'cubebox_conversation_create_failed', `create conversation failed: ${response.status}`)
  }
  return (await response.json()) as ConversationReplayResponse
}

export async function loadConversation(conversationID: string): Promise<ConversationReplayResponse> {
  const response = await fetch(`/internal/cubebox/conversations/${conversationID}`, {
    credentials: 'include',
    method: 'GET'
  })
  if (!response.ok) {
    await readError(response, 'cubebox_conversation_read_failed', `load conversation failed: ${response.status}`)
  }
  return (await response.json()) as ConversationReplayResponse
}

export async function listConversations(): Promise<CubeBoxConversationListResponse> {
  const response = await fetch('/internal/cubebox/conversations?limit=20', {
    credentials: 'include',
    method: 'GET'
  })
  if (!response.ok) {
    await readError(response, 'cubebox_conversation_list_failed', `list conversations failed: ${response.status}`)
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
    await readError(response, 'cubebox_conversation_update_failed', `update conversation failed: ${response.status}`)
  }
  return (await response.json()) as ConversationReplayResponse
}

export async function compactConversation(conversationID: string, reason = 'manual'): Promise<CompactConversationResponse> {
  const response = await fetch(`/internal/cubebox/conversations/${conversationID}:compact`, {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ reason })
  })
  if (!response.ok) {
    await readError(response, 'cubebox_conversation_update_failed', `compact conversation failed: ${response.status}`)
  }
  return (await response.json()) as CompactConversationResponse
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
    await readError(response, 'cubebox_turn_stream_failed', `stream turn failed: ${response.status}`)
  }

  const responseBody = response.body
  if (!responseBody) {
    throw new Error('stream turn failed: missing response body')
  }

  const reader = responseBody.getReader()
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
    await readError(response, 'cubebox_turn_interrupt_failed', `interrupt turn failed: ${response.status}`)
  }
}
