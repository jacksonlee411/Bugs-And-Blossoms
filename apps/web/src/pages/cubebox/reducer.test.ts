import { describe, expect, it } from 'vitest'
import { cubeboxReducer, initialCubeBoxState, replayConversation } from './reducer'
import { phaseCRoundtripGolden, phaseCRoundtripReplayFixture } from './lifecycle.fixture'
import { reconstructionFixtures } from './reconstruction.fixtures'

describe('cubebox reducer', () => {
  it.each(reconstructionFixtures)('replays reconstruction fixture %s into golden state', ({ replay, golden }) => {
    const state = replayConversation(replay)

    expect(state.conversation).toEqual(golden.conversation)
    expect(state.items).toEqual(golden.items)
    expect(state.turnStatus).toBe(golden.turnStatus)
    expect(state.activeTurnID).toBe(golden.activeTurnID)
    expect(state.nextSequence).toBe(golden.nextSequence)
    expect(state.errorMessage).toBe(golden.errorMessage)
  })

  it('keeps phase c lifecycle golden aligned with reducer replay output', () => {
    const state = replayConversation(phaseCRoundtripReplayFixture)

    expect(state.conversation).toEqual(phaseCRoundtripGolden.conversation)
    expect(state.items).toEqual(phaseCRoundtripGolden.items)
    expect(state.turnStatus).toBe(phaseCRoundtripGolden.turnStatus)
    expect(state.activeTurnID).toBe(phaseCRoundtripGolden.activeTurnID)
    expect(state.nextSequence).toBe(phaseCRoundtripGolden.nextSequence)
    expect(state.errorMessage).toBe(phaseCRoundtripGolden.errorMessage)
  })

  it('replays deterministic stream events into a shared timeline state', () => {
    const state = replayConversation({
      conversation: {
        id: 'conv_1',
        title: '新对话',
        status: 'active',
        archived: false
      },
      next_sequence: 7,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_1',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-21T00:00:00Z',
          payload: { title: '新对话', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 2,
          type: 'turn.started',
          ts: '2026-04-21T00:00:01Z',
          payload: { user_message_id: 'msg_user_1' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 3,
          type: 'turn.user_message.accepted',
          ts: '2026-04-21T00:00:02Z',
          payload: { message_id: 'msg_user_1', text: 'hello' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 4,
          type: 'turn.agent_message.delta',
          ts: '2026-04-21T00:00:03Z',
          payload: { message_id: 'msg_agent_1', delta: 'hi ' }
        },
        {
          event_id: 'evt_5',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 5,
          type: 'turn.agent_message.delta',
          ts: '2026-04-21T00:00:04Z',
          payload: { message_id: 'msg_agent_1', delta: 'there' }
        },
        {
          event_id: 'evt_6',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 6,
          type: 'turn.completed',
          ts: '2026-04-21T00:00:05Z',
          payload: { status: 'completed' }
        }
      ]
    })

    expect(state.conversation?.id).toBe('conv_1')
    expect(state.items).toHaveLength(2)
    expect(state.items[0]).toMatchObject({ kind: 'user_message', text: 'hello' })
    expect(state.items[1]).toMatchObject({ kind: 'agent_message', text: 'hi there', status: 'streaming' })
    expect(state.turnStatus).toBe('completed')
    expect(state.nextSequence).toBe(7)
  })

  it('preserves newline-only delta chunks so numbered items do not collapse during replay', () => {
    const state = replayConversation({
      conversation: {
        id: 'conv_1',
        title: '新对话',
        status: 'active',
        archived: false
      },
      next_sequence: 6,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 1,
          type: 'turn.started',
          ts: '2026-04-23T00:00:00Z',
          payload: { user_message_id: 'msg_user_1' }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 2,
          type: 'turn.user_message.accepted',
          ts: '2026-04-23T00:00:01Z',
          payload: { message_id: 'msg_user_1', text: '介绍一下你知道的' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 3,
          type: 'turn.agent_message.delta',
          ts: '2026-04-23T00:00:02Z',
          payload: { message_id: 'msg_agent_1', delta: '1) 关于我能帮你做什么' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 4,
          type: 'turn.agent_message.delta',
          ts: '2026-04-23T00:00:03Z',
          payload: { message_id: 'msg_agent_1', delta: '\n\n' }
        },
        {
          event_id: 'evt_5',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 5,
          type: 'turn.agent_message.delta',
          ts: '2026-04-23T00:00:04Z',
          payload: { message_id: 'msg_agent_1', delta: '2) 关于我“知道什么”' }
        },
        {
          event_id: 'evt_6',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 6,
          type: 'turn.agent_message.completed',
          ts: '2026-04-23T00:00:05Z',
          payload: { message_id: 'msg_agent_1' }
        },
        {
          event_id: 'evt_7',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 7,
          type: 'turn.completed',
          ts: '2026-04-23T00:00:06Z',
          payload: { status: 'completed' }
        }
      ]
    })

    expect(state.items).toHaveLength(2)
    expect(state.items[1]).toMatchObject({
      kind: 'agent_message',
      text: '1) 关于我能帮你做什么\n\n2) 关于我“知道什么”',
      status: 'completed'
    })
  })

  it('applies archived metadata events during replay so restored title and status match the event log', () => {
    const state = replayConversation({
      conversation: {
        id: 'conv_1',
        title: '旧标题',
        status: 'active',
        archived: false
      },
      next_sequence: 4,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_1',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-21T00:00:00Z',
          payload: { title: '旧标题', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_1',
          turn_id: null,
          sequence: 2,
          type: 'conversation.renamed',
          ts: '2026-04-21T00:00:01Z',
          payload: { title: '新标题', status: 'active', archived: false }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_1',
          turn_id: null,
          sequence: 3,
          type: 'conversation.archived',
          ts: '2026-04-21T00:00:02Z',
          payload: { title: '归档标题', status: 'archived', archived: true }
        }
      ]
    })

    expect(state.conversation).toEqual({
      id: 'conv_1',
      title: '归档标题',
      status: 'archived',
      archived: true
    })
    expect(state.nextSequence).toBe(4)
  })

  it('records turn error into the timeline', () => {
    const state = cubeboxReducer(initialCubeBoxState, {
      type: 'event_received',
      payload: {
        event_id: 'evt_err',
        conversation_id: 'conv_1',
        turn_id: 'turn_1',
        sequence: 1,
        type: 'turn.error',
        ts: '2026-04-21T00:00:00Z',
        payload: { code: 'deterministic_provider_error', message: 'boom', retryable: false }
      }
    })

    expect(state.turnStatus).toBe('error')
    expect(state.errorMessage).toBe('boom')
    expect(state.activeTurnID).toBeNull()
    expect(state.items[0]).toMatchObject({ kind: 'error_item', text: 'boom', status: 'error' })
  })

  it('records context compacted into compact timeline item', () => {
    const state = cubeboxReducer(initialCubeBoxState, {
      type: 'event_received',
      payload: {
        event_id: 'evt_compact',
        conversation_id: 'conv_1',
        turn_id: null,
        sequence: 3,
        type: 'turn.context_compacted',
        ts: '2026-04-22T00:00:00Z',
        payload: { summary_id: 'summary_1', summary_text: '已压缩旧历史。', source_range: [1, 2] }
      }
    })

    expect(state.items[0]).toMatchObject({
      id: 'summary_1',
      kind: 'compact_item',
      text: '已压缩旧历史。',
      status: 'completed'
    })
    expect(state.nextSequence).toBe(4)
  })

  it('keeps raw messages reconstructable after compaction event is replayed', () => {
    const state = replayConversation({
      conversation: {
        id: 'conv_1',
        title: '新对话',
        status: 'active',
        archived: false
      },
      next_sequence: 5,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 1,
          type: 'turn.user_message.accepted',
          ts: '2026-04-22T00:00:00Z',
          payload: { message_id: 'msg_user_1', text: '原始问题仍需保留' }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 2,
          type: 'turn.agent_message.delta',
          ts: '2026-04-22T00:00:01Z',
          payload: { message_id: 'msg_agent_1', delta: '原始回答仍需保留' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_1',
          turn_id: 'turn_1',
          sequence: 3,
          type: 'turn.agent_message.completed',
          ts: '2026-04-22T00:00:02Z',
          payload: { message_id: 'msg_agent_1' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_1',
          turn_id: null,
          sequence: 4,
          type: 'turn.context_compacted',
          ts: '2026-04-22T00:00:03Z',
          payload: { summary_id: 'summary_1', summary_text: '这里只是摘要，不替代原始消息。', source_range: [1, 3] }
        }
      ]
    })

    expect(state.items).toEqual([
      {
        id: 'msg_user_1',
        kind: 'user_message',
        text: '原始问题仍需保留',
        status: 'completed'
      },
      {
        id: 'msg_agent_1',
        kind: 'agent_message',
        text: '原始回答仍需保留',
        status: 'completed'
      },
      {
        id: 'summary_1',
        kind: 'compact_item',
        text: '这里只是摘要，不替代原始消息。',
        status: 'completed'
      }
    ])
  })

  it('keeps existing timeline when the same conversation metadata is reloaded', () => {
    const state = cubeboxReducer(
      {
        ...initialCubeBoxState,
        conversation: {
          id: 'conv_1',
          title: '新对话',
          status: 'active',
          archived: false
        },
        items: [
          {
            id: 'msg_user_1',
            kind: 'user_message',
            text: 'hello',
            status: 'completed'
          }
        ],
        nextSequence: 4
      },
      {
        type: 'conversation_loaded',
        payload: {
          conversation: {
            id: 'conv_1',
            title: '新对话',
            status: 'active',
            archived: false
          },
          events: [],
          next_sequence: 5
        }
      }
    )

    expect(state.items).toHaveLength(1)
    expect(state.items[0]).toMatchObject({ id: 'msg_user_1', text: 'hello' })
    expect(state.nextSequence).toBe(5)
  })

  it('fails closed when stream ends locally without terminal SSE event', () => {
    const state = cubeboxReducer(
      {
        ...initialCubeBoxState,
        activeTurnID: 'turn_1',
        turnStatus: 'streaming'
      },
      {
        type: 'stream_failed_locally',
        message: 'network down'
      }
    )

    expect(state.turnStatus).toBe('error')
    expect(state.activeTurnID).toBeNull()
    expect(state.errorMessage).toBe('network down')
  })

  it('fails closed when restored history contains a dangling streaming turn', () => {
    const state = replayConversation({
      conversation: {
        id: 'conv_dangling',
        title: '未完成会话',
        status: 'active',
        archived: false
      },
      next_sequence: 5,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_dangling',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-22T00:00:00Z',
          payload: { title: '未完成会话', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_dangling',
          turn_id: 'turn_1',
          sequence: 2,
          type: 'turn.started',
          ts: '2026-04-22T00:00:01Z',
          payload: { trace_id: 'trace_1', runtime: 'openai-chat-completions' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_dangling',
          turn_id: 'turn_1',
          sequence: 3,
          type: 'turn.user_message.accepted',
          ts: '2026-04-22T00:00:02Z',
          payload: { message_id: 'msg_user_1', text: 'hello' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_dangling',
          turn_id: 'turn_1',
          sequence: 4,
          type: 'turn.agent_message.delta',
          ts: '2026-04-22T00:00:03Z',
          payload: { message_id: 'msg_agent_1', delta: 'partial' }
        }
      ]
    })

    expect(state.turnStatus).toBe('error')
    expect(state.activeTurnID).toBeNull()
    expect(state.errorMessage).toBe('上次回复未正常结束，请重新发送。')
    expect(state.items[1]).toMatchObject({ id: 'msg_agent_1', status: 'error' })
  })
})
