import { describe, expect, it } from 'vitest'
import {
  createAssistantBridgeToken,
  parseAssistantAllowedOrigins,
  parseAssistantBridgeMessage,
  validateAssistantBridgeMessage
} from './assistantMessageBridge'

describe('assistantMessageBridge', () => {
  it('parses allowlist origins and filters invalid entries', () => {
    const origins = parseAssistantAllowedOrigins(
      'https://librechat.example.com, *,not-a-url, https://librechat.example.com/path',
      'http://localhost:8080'
    )

    expect(origins).toEqual(['http://localhost:8080', 'https://librechat.example.com'])
  })

  it('validates message schema strictly', () => {
    expect(parseAssistantBridgeMessage({})).toBeNull()
    expect(
      parseAssistantBridgeMessage({
        type: 'assistant.prompt.sync',
        channel: 'c1',
        nonce: 'n1',
        payload: { input: 'hello' }
      })
    ).toEqual({
      type: 'assistant.prompt.sync',
      channel: 'c1',
      nonce: 'n1',
      payload: { input: 'hello' },
      request_id: undefined
    })
  })

  it('rejects disallowed origins', () => {
    const result = validateAssistantBridgeMessage({
      allowedOrigins: ['http://localhost:8080'],
      expectedChannel: 'c1',
      expectedNonce: 'n1',
      event: {
        origin: 'https://evil.example.com',
        data: {
          type: 'assistant.prompt.sync',
          channel: 'c1',
          nonce: 'n1',
          payload: { input: 'hello' }
        }
      }
    })

    expect(result).toEqual({ accepted: false, reason: 'origin_not_allowed' })
  })

  it('rejects nonce or channel mismatch', () => {
    const result = validateAssistantBridgeMessage({
      allowedOrigins: ['http://localhost:8080'],
      expectedChannel: 'c1',
      expectedNonce: 'n1',
      event: {
        origin: 'http://localhost:8080',
        data: {
          type: 'assistant.prompt.sync',
          channel: 'wrong',
          nonce: 'n1',
          payload: { input: 'hello' }
        }
      }
    })

    expect(result).toEqual({ accepted: false, reason: 'channel_mismatch' })
  })

  it('accepts message only when all checks pass', () => {
    const result = validateAssistantBridgeMessage({
      allowedOrigins: ['http://localhost:8080'],
      expectedChannel: 'c1',
      expectedNonce: 'n1',
      event: {
        origin: 'http://localhost:8080',
        data: {
          type: 'assistant.turn.refresh',
          channel: 'c1',
          nonce: 'n1',
          payload: {}
        }
      }
    })

    expect(result.accepted).toBe(true)
    expect(result.message?.type).toBe('assistant.turn.refresh')
  })

  it('generates prefixed bridge token', () => {
    const token = createAssistantBridgeToken('assistant_channel')
    expect(token.startsWith('assistant_channel_')).toBe(true)
    expect(token.length).toBeGreaterThan('assistant_channel_'.length)
  })
})
