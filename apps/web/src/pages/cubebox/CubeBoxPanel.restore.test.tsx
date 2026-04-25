import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CubeBoxProvider } from './CubeBoxProvider'
import { CubeBoxPanel } from './CubeBoxPanel'

const apiMocks = vi.hoisted(() => ({
  compactConversation: vi.fn(),
  createConversation: vi.fn(),
  interruptTurn: vi.fn(),
  listConversations: vi.fn(),
  loadConversation: vi.fn(),
  streamTurn: vi.fn(),
  updateConversation: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('./api', () => apiMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)

describe('CubeBoxPanel restore flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appPreferencesMocks.useAppPreferences.mockReturnValue({
      hasPermission: vi.fn().mockReturnValue(true),
      t: (key: string) =>
        (
          {
            page_cubebox_title: 'CubeBox',
            page_cubebox_subtitle: '在右侧抽屉中发起并继续对话。',
            cubebox_conversation_id: '会话',
            cubebox_user_message: '你',
            cubebox_agent_message: 'CubeBox',
            cubebox_error_item: '错误',
            cubebox_empty_timeline: '先发送一句话开始对话。',
            cubebox_prompt_label: '输入消息',
            cubebox_stop: '停止',
            cubebox_send: '发送',
            cubebox_history: '历史记录',
            cubebox_settings: '设置',
            cubebox_new_chat: '新建对话',
            cubebox_history_title: '历史会话',
            cubebox_settings_title: 'CubeBox 设置',
            cubebox_settings_placeholder: '这里先保留设置壳层入口，后续再绑定模型与配置项。',
            cubebox_empty_history: '还没有已保存的会话。',
            cubebox_rename: '重命名',
            cubebox_archive: '归档',
            cubebox_unarchive: '恢复',
            cubebox_compact_item: '压缩摘要',
            cubebox_conversation_status_active: '进行中',
            cubebox_conversation_status_archived: '已归档',
            cubebox_status_idle: '空闲',
            cubebox_status_streaming: '流式处理中',
            cubebox_status_completed: '已完成',
            cubebox_status_error: '失败',
            cubebox_status_interrupted: '已中断',
            text_loading: '加载中'
          } as Record<string, string>
        )[key] ?? key
    })
  })

  it('restores the most recent active conversation into the panel after list then load', async () => {
    apiMocks.listConversations.mockResolvedValue({
      items: [
        {
          id: 'conv_archived',
          title: '旧归档会话',
          status: 'archived',
          archived: true,
          updated_at: '2026-04-22T09:00:00Z'
        },
        {
          id: 'conv_active',
          title: '当前活跃会话',
          status: 'active',
          archived: false,
          updated_at: '2026-04-22T08:00:00Z'
        }
      ]
    })
    apiMocks.loadConversation.mockResolvedValue({
      conversation: {
        id: 'conv_active',
        title: '当前活跃会话',
        status: 'active',
        archived: false
      },
      events: [
        {
          event_id: 'evt_1',
          conversation_id: 'conv_active',
          turn_id: null,
          sequence: 1,
          type: 'conversation.loaded',
          ts: '2026-04-22T08:00:00Z',
          payload: { title: '当前活跃会话', status: 'active', archived: false }
        },
        {
          event_id: 'evt_2',
          conversation_id: 'conv_active',
          turn_id: 'turn_1',
          sequence: 2,
          type: 'turn.user_message.accepted',
          ts: '2026-04-22T08:00:01Z',
          payload: { message_id: 'msg_user_1', text: '恢复我上次的上下文' }
        },
        {
          event_id: 'evt_3',
          conversation_id: 'conv_active',
          turn_id: null,
          sequence: 3,
          type: 'turn.context_compacted',
          ts: '2026-04-22T08:00:02Z',
          payload: { summary_id: 'summary_1', summary_text: '已压缩更早的历史。', source_range: [1, 2] }
        }
      ],
      next_sequence: 4
    })

    render(
      <MemoryRouter>
        <CubeBoxProvider>
          <CubeBoxPanel />
        </CubeBoxProvider>
      </MemoryRouter>
    )

    await waitFor(() => expect(apiMocks.listConversations).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(apiMocks.loadConversation).toHaveBeenCalledWith('conv_active'))
    await waitFor(() => expect(screen.getByRole('heading', { name: '当前活跃会话' })).toBeInTheDocument())
    expect(screen.getByText('恢复我上次的上下文')).toBeInTheDocument()
    expect(screen.getByText('已压缩更早的历史。')).toBeInTheDocument()
    expect(screen.getByText('压缩摘要')).toBeInTheDocument()
    expect(screen.getAllByText('会话: conv_active').length).toBeGreaterThan(0)
  })
})
