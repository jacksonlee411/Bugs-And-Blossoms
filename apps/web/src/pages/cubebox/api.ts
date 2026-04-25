import { ApiClientError } from '../../api/errors'
import { resolveApiErrorMessage } from '../../errors/presentApiError'
import type {
  CanonicalEvent,
  ConversationReplayResponse,
  CubeBoxPageContext,
  CubeBoxConversationListResponse,
  CubeBoxCapabilities,
  CubeBoxModelCredential,
  CubeBoxModelHealth,
  CubeBoxModelProvider,
  CubeBoxModelSettingsSnapshot,
  CubeBoxActiveModelSelection
} from './types'

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

export async function streamTurn(input: {
  conversationID: string
  prompt: string
  nextSequence: number
  pageContext?: CubeBoxPageContext
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
      next_sequence: input.nextSequence,
      ...(input.pageContext ? { page_context: input.pageContext } : {})
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

export async function loadModelSettings(): Promise<CubeBoxModelSettingsSnapshot> {
  const response = await fetch('/internal/cubebox/settings', {
    credentials: 'include',
    method: 'GET'
  })
  if (!response.ok) {
    await readError(response, 'ai_model_config_invalid', `load model settings failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxModelSettingsSnapshot
}

export async function loadCubeBoxCapabilities(): Promise<CubeBoxCapabilities> {
  const response = await fetch('/internal/cubebox/capabilities', {
    credentials: 'include',
    method: 'GET'
  })
  if (!response.ok) {
    await readError(response, 'forbidden', `load cubebox capabilities failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxCapabilities
}

export async function upsertModelProvider(input: {
  providerID: string
  providerType: string
  displayName: string
  baseURL: string
  enabled: boolean
}): Promise<CubeBoxModelProvider> {
  const response = await fetch('/internal/cubebox/settings/providers', {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      provider_id: input.providerID,
      provider_type: input.providerType,
      display_name: input.displayName,
      base_url: input.baseURL,
      enabled: input.enabled
    })
  })
  if (!response.ok) {
    await readError(response, 'ai_model_config_invalid', `save provider failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxModelProvider
}

export async function rotateModelCredential(input: {
  providerID: string
  secretRef: string
  maskedSecret: string
}): Promise<CubeBoxModelCredential> {
  const response = await fetch('/internal/cubebox/settings/credentials', {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      provider_id: input.providerID,
      secret_ref: input.secretRef,
      masked_secret: input.maskedSecret
    })
  })
  if (!response.ok) {
    await readError(response, 'ai_model_secret_missing', `rotate credential failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxModelCredential
}

export async function deactivateModelCredential(credentialID: string): Promise<CubeBoxModelCredential> {
  const response = await fetch(`/internal/cubebox/settings/credentials/${credentialID}:deactivate`, {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: '{}'
  })
  if (!response.ok) {
    await readError(response, 'ai_model_config_invalid', `deactivate credential failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxModelCredential
}

export async function selectActiveModel(input: {
  providerID: string
  modelSlug: string
  capabilitySummary: Record<string, unknown>
}): Promise<CubeBoxActiveModelSelection> {
  const response = await fetch('/internal/cubebox/settings/selection', {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      provider_id: input.providerID,
      model_slug: input.modelSlug,
      capability_summary: input.capabilitySummary
    })
  })
  if (!response.ok) {
    await readError(response, 'ai_model_config_invalid', `select active model failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxActiveModelSelection
}

export async function verifyActiveModel(): Promise<CubeBoxModelHealth> {
  const response = await fetch('/internal/cubebox/settings/verify', {
    credentials: 'include',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: '{}'
  })
  if (!response.ok) {
    await readError(response, 'ai_model_provider_unavailable', `verify active model failed: ${response.status}`)
  }
  return (await response.json()) as CubeBoxModelHealth
}
