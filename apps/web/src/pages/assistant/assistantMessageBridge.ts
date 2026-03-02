export interface AssistantBridgeMessage {
  type: string
  channel: string
  nonce: string
  request_id?: string
  payload: Record<string, unknown>
}

type BridgeValidationReason =
  | 'origin_not_allowed'
  | 'schema_invalid'
  | 'channel_mismatch'
  | 'nonce_mismatch'

export interface AssistantBridgeValidationResult {
  accepted: boolean
  message?: AssistantBridgeMessage
  reason?: BridgeValidationReason
}

interface ValidateBridgeMessageInput {
  allowedOrigins: readonly string[]
  expectedChannel: string
  expectedNonce: string
  event: {
    origin: string
    data: unknown
  }
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function toNonEmptyString(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null
  }
  const trimmed = value.trim()
  if (trimmed.length === 0) {
    return null
  }
  return trimmed
}

export function parseAssistantBridgeMessage(data: unknown): AssistantBridgeMessage | null {
  if (!isObjectRecord(data)) {
    return null
  }
  const type = toNonEmptyString(data.type)
  const channel = toNonEmptyString(data.channel)
  const nonce = toNonEmptyString(data.nonce)
  if (!type || !channel || !nonce) {
    return null
  }
  const payload = data.payload
  if (!isObjectRecord(payload)) {
    return null
  }
  const requestID = data.request_id
  if (requestID !== undefined && typeof requestID !== 'string') {
    return null
  }
  return {
    type,
    channel,
    nonce,
    request_id: requestID,
    payload
  }
}

export function parseAssistantAllowedOrigins(raw: string | undefined, currentOrigin: string | undefined): string[] {
  const values: string[] = []
  if (typeof currentOrigin === 'string' && currentOrigin.trim().length > 0) {
    values.push(currentOrigin.trim())
  }
  for (const item of (raw ?? '').split(',')) {
    const candidate = item.trim()
    if (candidate.length === 0 || candidate === '*') {
      continue
    }
    try {
      values.push(new URL(candidate).origin)
    } catch {
      continue
    }
  }
  return Array.from(new Set(values))
}

function randomHex(length: number): string {
  const bytes = new Uint8Array(Math.ceil(length / 2))
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256)
    }
  }
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('').slice(0, length)
}

function randomUUIDWithoutDashes(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID().replaceAll('-', '')
  }
  return randomHex(32)
}

export function createAssistantBridgeToken(prefix: string): string {
  return `${prefix}_${randomUUIDWithoutDashes()}`
}

export function validateAssistantBridgeMessage(input: ValidateBridgeMessageInput): AssistantBridgeValidationResult {
  if (!input.allowedOrigins.includes(input.event.origin)) {
    return { accepted: false, reason: 'origin_not_allowed' }
  }
  const message = parseAssistantBridgeMessage(input.event.data)
  if (!message) {
    return { accepted: false, reason: 'schema_invalid' }
  }
  if (message.channel !== input.expectedChannel) {
    return { accepted: false, reason: 'channel_mismatch' }
  }
  if (message.nonce !== input.expectedNonce) {
    return { accepted: false, reason: 'nonce_mismatch' }
  }
  return { accepted: true, message }
}
