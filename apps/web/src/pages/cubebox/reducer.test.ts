import { describe, expect, it } from 'vitest'
import { cubeboxReducer, initialCubeBoxState, replayConversation } from './reducer'

describe('cubebox reducer', () => {
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
})
