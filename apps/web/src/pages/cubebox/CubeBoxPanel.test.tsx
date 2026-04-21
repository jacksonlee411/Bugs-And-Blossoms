import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CubeBoxPanel } from './CubeBoxPanel'

const cubeBoxMocks = vi.hoisted(() => ({
  useCubeBox: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

vi.mock('./CubeBoxProvider', () => cubeBoxMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)

describe('CubeBoxPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    appPreferencesMocks.useAppPreferences.mockReturnValue({
      tenantId: 'tenant-a',
      locale: 'zh',
      setLocale: vi.fn(),
      themeMode: 'light',
      toggleThemeMode: vi.fn(),
      navDebugMode: false,
      hasPermission: vi.fn(),
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

    cubeBoxMocks.useCubeBox.mockReturnValue({
      archiveConversation: vi.fn(),
      conversations: [
        {
          id: 'conv_1',
          title: '需求澄清',
          status: 'active',
          archived: false,
          updated_at: '2026-04-21T10:00:00Z'
        }
      ],
      conversationsLoading: false,
      renameConversation: vi.fn(),
      selectConversation: vi.fn(),
      startNewConversation: vi.fn().mockResolvedValue(undefined),
      state: {
        conversation: {
          id: 'conv_1',
          title: '需求澄清',
          status: 'active',
          archived: false
        },
        items: [],
        turnStatus: 'idle',
        activeTurnID: null,
        nextSequence: 1,
        composerText: '',
        loading: false,
        errorMessage: null
      },
      interrupt: vi.fn(),
      sendMessage: vi.fn(),
      setComposerText: vi.fn()
    })
  })

  it('renders 431A header actions and conversation title', () => {
    render(<CubeBoxPanel />)

    expect(screen.getByRole('heading', { name: '需求澄清' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '历史记录' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新建对话' })).toBeInTheDocument()
  })

  it('opens history dialog from header history button', () => {
    render(<CubeBoxPanel />)

    fireEvent.click(screen.getByRole('button', { name: '历史记录' }))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('历史会话')).toBeInTheDocument()
    expect(screen.getAllByText('需求澄清').length).toBeGreaterThan(0)
  })

  it('starts a new conversation from header new chat button', () => {
    render(<CubeBoxPanel />)

    fireEvent.click(screen.getByRole('button', { name: '新建对话' }))

    expect(cubeBoxMocks.useCubeBox.mock.results[0]?.value.startNewConversation).toHaveBeenCalledTimes(1)
  })
})
