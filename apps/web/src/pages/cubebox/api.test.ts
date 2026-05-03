import { describe, expect, it, vi, afterEach } from 'vitest'
import {
  deactivateModelCredential,
  interruptTurn,
  loadModelSettings,
  streamTurn,
  upsertModelProvider,
  verifyActiveModel
} from './api'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('cubebox api', () => {
  it('fails closed when stream ends without terminal event', async () => {
    const encoder = new TextEncoder()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        body: new ReadableStream({
          start(controller) {
            controller.enqueue(encoder.encode('data: {"event_id":"evt_1","conversation_id":"conv_1","turn_id":"turn_1","sequence":1,"type":"turn.started","ts":"2026-04-21T00:00:00Z","payload":{}}\n\n'))
            controller.close()
          }
        })
      })
    )

    await expect(
      streamTurn({
        conversationID: 'conv_1',
        prompt: 'hello',
        nextSequence: 1,
        signal: new AbortController().signal,
        onEvent: vi.fn()
      })
    ).rejects.toThrow('stream turn failed: missing terminal event')
  })

  it('posts only canonical turn fields to turn stream endpoint', async () => {
    const encoder = new TextEncoder()
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      body: new ReadableStream({
        start(controller) {
          controller.enqueue(encoder.encode('data: {"event_id":"evt_1","conversation_id":"conv_1","turn_id":"turn_1","sequence":1,"type":"turn.completed","ts":"2026-04-21T00:00:00Z","payload":{"status":"completed"}}\n\n'))
          controller.close()
        }
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await streamTurn({
      conversationID: 'conv_1',
      prompt: '查该组织详情',
      nextSequence: 3,
      signal: new AbortController().signal,
      onEvent: vi.fn()
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/turns:stream',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          conversation_id: 'conv_1',
          prompt: '查该组织详情',
          next_sequence: 3
        })
      })
    )
  })

  it('includes conversation_id when interrupting a turn', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true })
    vi.stubGlobal('fetch', fetchMock)

    await interruptTurn('turn_1', 'conv_1')

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/turns/turn_1:interrupt?conversation_id=conv_1',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include'
      })
    )
  })

  it('loads model settings snapshot', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        providers: [],
        credentials: [],
        selection: null,
        health: null
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await loadModelSettings()

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/settings',
      expect.objectContaining({
        method: 'GET',
        credentials: 'include'
      })
    )
  })

  it('posts provider config to settings endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        id: 'openai-compatible',
        provider_type: 'openai-compatible',
        display_name: 'Primary',
        base_url: 'https://example.invalid/v1',
        enabled: true,
        updated_at: '2026-04-22T00:00:00Z'
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await upsertModelProvider({
      providerID: 'openai-compatible',
      providerType: 'openai-compatible',
      displayName: 'Primary',
      baseURL: 'https://example.invalid/v1',
      enabled: true
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/settings/providers',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include'
      })
    )
  })

  it('posts to verify endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        id: 'health_1',
        provider_id: 'openai-compatible',
        model_slug: 'gpt-4.1',
        status: 'healthy',
        latency_ms: 120,
        validated_at: '2026-04-22T00:00:00Z'
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await verifyActiveModel()

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/settings/verify',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include'
      })
    )
  })

  it('posts to deactivate credential endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        id: 'cred_1',
        provider_id: 'openai-compatible',
        secret_ref: 'env://OPENAI_API_KEY',
        masked_secret: 'sk-****',
        version: 2,
        active: false,
        created_at: '2026-04-22T00:00:00Z',
        disabled_at: '2026-04-22T00:01:00Z'
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await deactivateModelCredential('cred_1')

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/settings/credentials/cred_1:deactivate',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include'
      })
    )
  })
})
