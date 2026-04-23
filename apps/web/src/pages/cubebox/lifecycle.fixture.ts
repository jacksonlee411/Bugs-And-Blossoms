import type { ConversationReplayResponse, CubeBoxState } from './types'

export const phaseCRoundtripReplayFixture: ConversationReplayResponse = {
  conversation: {
    id: 'conv_roundtrip',
    title: '恢复后的活跃会话',
    status: 'active',
    archived: false
  },
  next_sequence: 9,
  events: [
    {
      event_id: 'evt_1',
      conversation_id: 'conv_roundtrip',
      turn_id: null,
      sequence: 1,
      type: 'conversation.loaded',
      ts: '2026-04-22T00:00:00Z',
      payload: { title: '新对话', status: 'active', archived: false }
    },
    {
      event_id: 'evt_2',
      conversation_id: 'conv_roundtrip',
      turn_id: null,
      sequence: 2,
      type: 'conversation.renamed',
      ts: '2026-04-22T00:00:01Z',
      payload: { title: '需求澄清', status: 'active', archived: false }
    },
    {
      event_id: 'evt_3',
      conversation_id: 'conv_roundtrip',
      turn_id: 'turn_1',
      sequence: 3,
      type: 'turn.user_message.accepted',
      ts: '2026-04-22T00:00:02Z',
      payload: { message_id: 'msg_user_1', text: '请总结当前状态' }
    },
    {
      event_id: 'evt_4',
      conversation_id: 'conv_roundtrip',
      turn_id: 'turn_1',
      sequence: 4,
      type: 'turn.agent_message.delta',
      ts: '2026-04-22T00:00:03Z',
      payload: { message_id: 'msg_agent_1', delta: '当前已完成持久化，' }
    },
    {
      event_id: 'evt_5',
      conversation_id: 'conv_roundtrip',
      turn_id: 'turn_1',
      sequence: 5,
      type: 'turn.agent_message.delta',
      ts: '2026-04-22T00:00:04Z',
      payload: { message_id: 'msg_agent_1', delta: '正在进入封板收口。' }
    },
    {
      event_id: 'evt_6',
      conversation_id: 'conv_roundtrip',
      turn_id: 'turn_1',
      sequence: 6,
      type: 'turn.agent_message.completed',
      ts: '2026-04-22T00:00:05Z',
      payload: { message_id: 'msg_agent_1' }
    },
    {
      event_id: 'evt_7',
      conversation_id: 'conv_roundtrip',
      turn_id: 'turn_1',
      sequence: 7,
      type: 'turn.completed',
      ts: '2026-04-22T00:00:06Z',
      payload: { status: 'completed' }
    },
    {
      event_id: 'evt_8',
      conversation_id: 'conv_roundtrip',
      turn_id: null,
      sequence: 8,
      type: 'conversation.unarchived',
      ts: '2026-04-22T00:00:07Z',
      payload: { title: '恢复后的活跃会话', status: 'active', archived: false }
    }
  ]
}

export const phaseCRoundtripGolden: Pick<CubeBoxState, 'conversation' | 'items' | 'turnStatus' | 'activeTurnID' | 'nextSequence' | 'errorMessage'> = {
  conversation: {
    id: 'conv_roundtrip',
    title: '恢复后的活跃会话',
    status: 'active',
    archived: false
  },
  items: [
    {
      id: 'msg_user_1',
      kind: 'user_message',
      text: '请总结当前状态',
      status: 'completed'
    },
    {
      id: 'msg_agent_1',
      kind: 'agent_message',
      text: '当前已完成持久化，正在进入封板收口。',
      status: 'completed'
    }
  ],
  turnStatus: 'completed',
  activeTurnID: null,
  nextSequence: 9,
  errorMessage: null
}
