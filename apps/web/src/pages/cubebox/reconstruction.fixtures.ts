import type { ConversationReplayResponse, CubeBoxState } from './types'

interface ReconstructionFixture {
  name: string
  replay: ConversationReplayResponse
  golden: Pick<CubeBoxState, 'conversation' | 'items' | 'turnStatus' | 'activeTurnID' | 'nextSequence' | 'errorMessage'>
}

export const reconstructionFixtures: ReconstructionFixture[] = [
  {
    name: 'completed_turn_with_rename_and_archive_roundtrip',
    replay: {
      conversation: {
        id: 'conv_completed',
        title: '已归档会话',
        status: 'archived',
        archived: true
      },
      next_sequence: 10,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_completed',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-21T00:00:00Z',
          payload: { title: '新对话', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_completed',
          turn_id: null,
          sequence: 2,
          type: 'conversation.renamed',
          ts: '2026-04-21T00:00:01Z',
          payload: { title: '需求澄清', status: 'active', archived: false }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_completed',
          turn_id: 'turn_1',
          sequence: 3,
          type: 'turn.started',
          ts: '2026-04-21T00:00:02Z',
          payload: { user_message_id: 'msg_user_1' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_completed',
          turn_id: 'turn_1',
          sequence: 4,
          type: 'turn.user_message.accepted',
          ts: '2026-04-21T00:00:03Z',
          payload: { message_id: 'msg_user_1', text: '请总结当前进度' }
        },
        {
          event_id: 'evt_5',
          conversation_id: 'conv_completed',
          turn_id: 'turn_1',
          sequence: 5,
          type: 'turn.agent_message.delta',
          ts: '2026-04-21T00:00:04Z',
          payload: { message_id: 'msg_agent_1', delta: '当前已完成 Phase B，' }
        },
        {
          event_id: 'evt_6',
          conversation_id: 'conv_completed',
          turn_id: 'turn_1',
          sequence: 6,
          type: 'turn.agent_message.delta',
          ts: '2026-04-21T00:00:05Z',
          payload: { message_id: 'msg_agent_1', delta: '正在收口 Phase C。' }
        },
        {
          event_id: 'evt_7',
          conversation_id: 'conv_completed',
          turn_id: 'turn_1',
          sequence: 7,
          type: 'turn.agent_message.completed',
          ts: '2026-04-21T00:00:06Z',
          payload: { message_id: 'msg_agent_1' }
        },
        {
          event_id: 'evt_8',
          conversation_id: 'conv_completed',
          turn_id: 'turn_1',
          sequence: 8,
          type: 'turn.completed',
          ts: '2026-04-21T00:00:07Z',
          payload: { status: 'completed' }
        },
        {
          event_id: 'evt_9',
          conversation_id: 'conv_completed',
          turn_id: null,
          sequence: 9,
          type: 'conversation.archived',
          ts: '2026-04-21T00:00:08Z',
          payload: { title: '已归档会话', status: 'archived', archived: true }
        }
      ]
    },
    golden: {
      conversation: {
        id: 'conv_completed',
        title: '已归档会话',
        status: 'archived',
        archived: true
      },
      items: [
        {
          id: 'msg_user_1',
          kind: 'user_message',
          text: '请总结当前进度',
          status: 'completed'
        },
        {
          id: 'msg_agent_1',
          kind: 'agent_message',
          text: '当前已完成 Phase B，正在收口 Phase C。',
          status: 'completed'
        }
      ],
      turnStatus: 'completed',
      activeTurnID: null,
      nextSequence: 10,
      errorMessage: null
    }
  },
  {
    name: 'interrupted_turn_keeps_partial_agent_output',
    replay: {
      conversation: {
        id: 'conv_interrupted',
        title: '处理中断',
        status: 'active',
        archived: false
      },
      next_sequence: 6,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_interrupted',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-21T01:00:00Z',
          payload: { title: '处理中断', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_interrupted',
          turn_id: 'turn_2',
          sequence: 2,
          type: 'turn.started',
          ts: '2026-04-21T01:00:01Z',
          payload: { user_message_id: 'msg_user_2' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_interrupted',
          turn_id: 'turn_2',
          sequence: 3,
          type: 'turn.user_message.accepted',
          ts: '2026-04-21T01:00:02Z',
          payload: { message_id: 'msg_user_2', text: '继续输出剩余步骤' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_interrupted',
          turn_id: 'turn_2',
          sequence: 4,
          type: 'turn.agent_message.delta',
          ts: '2026-04-21T01:00:03Z',
          payload: { message_id: 'msg_agent_2', delta: '先补 reconstruction fixture' }
        },
        {
          event_id: 'evt_5',
          conversation_id: 'conv_interrupted',
          turn_id: 'turn_2',
          sequence: 5,
          type: 'turn.interrupted',
          ts: '2026-04-21T01:00:04Z',
          payload: { reason: 'user_requested' }
        }
      ]
    },
    golden: {
      conversation: {
        id: 'conv_interrupted',
        title: '处理中断',
        status: 'active',
        archived: false
      },
      items: [
        {
          id: 'msg_user_2',
          kind: 'user_message',
          text: '继续输出剩余步骤',
          status: 'completed'
        },
        {
          id: 'msg_agent_2',
          kind: 'agent_message',
          text: '先补 reconstruction fixture',
          status: 'interrupted'
        }
      ],
      turnStatus: 'interrupted',
      activeTurnID: 'turn_2',
      nextSequence: 6,
      errorMessage: null
    }
  },
  {
    name: 'failed_turn_restores_error_item_and_message',
    replay: {
      conversation: {
        id: 'conv_error',
        title: '失败案例',
        status: 'active',
        archived: false
      },
      next_sequence: 6,
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_error',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-21T02:00:00Z',
          payload: { title: '失败案例', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_error',
          turn_id: 'turn_3',
          sequence: 2,
          type: 'turn.started',
          ts: '2026-04-21T02:00:01Z',
          payload: { user_message_id: 'msg_user_3' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_error',
          turn_id: 'turn_3',
          sequence: 3,
          type: 'turn.user_message.accepted',
          ts: '2026-04-21T02:00:02Z',
          payload: { message_id: 'msg_user_3', text: '为什么这次失败了？' }
        },
        {
          event_id: 'evt_4',
          conversation_id: 'conv_error',
          turn_id: 'turn_3',
          sequence: 4,
          type: 'turn.error',
          ts: '2026-04-21T02:00:03Z',
          payload: { code: 'deterministic_provider_error', message: '当前回复暂时失败，请稍后重试。', retryable: false }
        },
        {
          event_id: 'evt_5',
          conversation_id: 'conv_error',
          turn_id: 'turn_3',
          sequence: 5,
          type: 'turn.completed',
          ts: '2026-04-21T02:00:04Z',
          payload: { status: 'failed' }
        }
      ]
    },
    golden: {
      conversation: {
        id: 'conv_error',
        title: '失败案例',
        status: 'active',
        archived: false
      },
      items: [
        {
          id: 'msg_user_3',
          kind: 'user_message',
          text: '为什么这次失败了？',
          status: 'completed'
        },
        {
          id: 'error-4',
          kind: 'error_item',
          text: '当前回复暂时失败，请稍后重试。',
          status: 'error'
        }
      ],
      turnStatus: 'error',
      activeTurnID: null,
      nextSequence: 6,
      errorMessage: '当前回复暂时失败，请稍后重试。'
    }
  }
]
