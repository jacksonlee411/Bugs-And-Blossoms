import { describe, expect, it, vi, afterEach } from 'vitest'
import { compactConversation, interruptTurn, streamTurn } from './api'

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

  it('posts to the compact endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        conversation: { id: 'conv_1', title: 'Latest', status: 'active', archived: false },
        event: {
          event_id: 'evt_compact',
          conversation_id: 'conv_1',
          turn_id: null,
          sequence: 5,
          type: 'turn.context_compacted',
          ts: '2026-04-22T00:00:00Z',
          payload: { summary_id: 'summary_1', source_range: [1, 3] }
        },
        prompt_view: [],
        next_sequence: 6
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await compactConversation('conv_1')

    expect(fetchMock).toHaveBeenCalledWith(
      '/internal/cubebox/conversations/conv_1:compact',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include'
      })
    )
  })
})
