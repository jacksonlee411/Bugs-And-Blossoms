import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CubeBoxPanel } from './CubeBoxPanel'

const cubeBoxMocks = vi.hoisted(() => ({
  useCubeBox: vi.fn()
}))

const appPreferencesMocks = vi.hoisted(() => ({
  useAppPreferences: vi.fn()
}))

const apiMocks = vi.hoisted(() => ({
  deactivateModelCredential: vi.fn(),
  loadCubeBoxCapabilities: vi.fn(),
  loadModelSettings: vi.fn(),
  rotateModelCredential: vi.fn(),
  selectActiveModel: vi.fn(),
  upsertModelProvider: vi.fn(),
  verifyActiveModel: vi.fn()
}))

vi.mock('./CubeBoxProvider', () => cubeBoxMocks)
vi.mock('../../app/providers/AppPreferencesContext', () => appPreferencesMocks)
vi.mock('./api', () => apiMocks)

const SLOW_UI_TEST_TIMEOUT_MS = 12_000

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
      hasRequiredCapability: vi.fn().mockReturnValue(true),
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
            cubebox_settings_active_model: '当前模型',
            cubebox_settings_no_selection: '尚未选择当前模型。',
            cubebox_settings_health_unknown: '健康状态未知',
            cubebox_settings_health_healthy: '健康',
            cubebox_settings_health_degraded: '降级',
            cubebox_settings_health_failed: '失败',
            cubebox_settings_provider_id: 'Provider ID',
            cubebox_settings_provider_type: 'Provider 类型',
            cubebox_settings_provider_name: 'Provider 名称',
            cubebox_settings_base_url: 'Base URL',
            cubebox_settings_provider_enabled: '启用 Provider',
            cubebox_settings_save_provider: '保存 Provider',
            cubebox_settings_secret_ref: '密钥引用',
            cubebox_settings_masked_secret: '掩码密钥',
            cubebox_settings_rotate_credential: '轮换密钥',
            cubebox_settings_credential_active: '启用中',
            cubebox_settings_credential_inactive: '已停用',
            cubebox_settings_deactivate_credential: '停用',
            cubebox_settings_model_slug: '模型标识',
            cubebox_settings_capability_summary: '能力摘要',
            cubebox_settings_capability_summary_invalid: '能力摘要必须是 JSON 对象。',
            cubebox_settings_save_selection: '保存当前模型',
            cubebox_settings_verify: '验证',
            cubebox_empty_history: '还没有已保存的会话。',
            cubebox_rename: '重命名',
            cubebox_archive: '归档',
            cubebox_unarchive: '恢复',
            cubebox_history_context_item: '历史上下文',
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
    apiMocks.loadModelSettings.mockResolvedValue({
      providers: [
        {
          id: 'openai-compatible',
          provider_type: 'openai-compatible',
          display_name: 'Primary',
          base_url: 'https://example.invalid/v1',
          enabled: true,
          updated_at: '2026-04-22T00:00:00Z'
        }
      ],
      credentials: [
        {
          id: 'cred_1',
          provider_id: 'openai-compatible',
          secret_ref: 'env://OPENAI_API_KEY',
          masked_secret: 'sk-****',
          version: 1,
          active: true,
          created_at: '2026-04-22T00:00:00Z'
        }
      ],
      selection: {
        provider_id: 'openai-compatible',
        model_slug: 'gpt-4.1',
        capability_summary: { streaming: true },
        updated_at: '2026-04-22T00:00:00Z'
      },
      health: {
        id: 'health_1',
        provider_id: 'openai-compatible',
        model_slug: 'gpt-4.1',
        status: 'healthy',
        latency_ms: 120,
        validated_at: '2026-04-22T00:00:00Z'
      }
    })
    apiMocks.loadCubeBoxCapabilities.mockResolvedValue({
      conversation: {
        read: true,
        use: true
      },
      settings: {
        read: true,
        verify: true,
        select: true,
        update: true,
        rotate: true,
        deactivate: true
      }
    })
    apiMocks.deactivateModelCredential.mockResolvedValue(undefined)
    apiMocks.rotateModelCredential.mockResolvedValue(undefined)
    apiMocks.selectActiveModel.mockResolvedValue(undefined)
    apiMocks.upsertModelProvider.mockResolvedValue(undefined)
    apiMocks.verifyActiveModel.mockResolvedValue(undefined)
  })

  it('renders 431A header actions and conversation title', async () => {
    render(<CubeBoxPanel />)

    expect(screen.getByRole('heading', { name: '需求澄清' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '历史记录' })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: '设置' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新建对话' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '压缩上下文' })).not.toBeInTheDocument()
  }, SLOW_UI_TEST_TIMEOUT_MS)

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

  it('treats /compact as a normal message and uses the standard send flow', () => {
    const sendMessage = vi.fn()
    const setComposerText = vi.fn()
    const archiveConversation = vi.fn()
    const renameConversation = vi.fn()
    const selectConversation = vi.fn()
    const startNewConversation = vi.fn().mockResolvedValue(undefined)
    const interrupt = vi.fn()

    cubeBoxMocks.useCubeBox.mockReturnValueOnce({
      archiveConversation,
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
      renameConversation,
      selectConversation,
      startNewConversation,
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
        composerText: '/compact',
        loading: false,
        errorMessage: null
      },
      interrupt,
      sendMessage,
      setComposerText
    })

    render(<CubeBoxPanel />)

    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(sendMessage).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: '压缩上下文' })).not.toBeInTheDocument()
  })

  it('sends only on plain Enter', () => {
    const sendMessage = vi.fn()
    const setComposerText = vi.fn()

    cubeBoxMocks.useCubeBox.mockReturnValueOnce({
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
        composerText: '第一行',
        loading: false,
        errorMessage: null
      },
      interrupt: vi.fn(),
      sendMessage,
      setComposerText
    })

    render(<CubeBoxPanel />)

    const composer = screen.getByRole('textbox', { name: '输入消息' })

    fireEvent.keyDown(composer, { key: 'Enter' })
    fireEvent.keyDown(composer, { key: 'Enter', ctrlKey: true })
    fireEvent.keyDown(composer, { key: 'Enter', shiftKey: true })
    fireEvent.keyDown(composer, { key: 'Enter', altKey: true })
    fireEvent.keyDown(composer, { key: 'Enter', metaKey: true })

    expect(sendMessage).toHaveBeenCalledTimes(1)
    expect(setComposerText).not.toHaveBeenCalled()
  })

  it('renders multiline agent content with preserved numbered list boundaries', () => {
    cubeBoxMocks.useCubeBox.mockReturnValueOnce({
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
        items: [
          {
            id: 'msg_agent_1',
            kind: 'agent_message',
            text: '1) 关于我能帮你做什么\n\n2) 关于我“知道什么”',
            status: 'completed'
          }
        ],
        turnStatus: 'completed',
        activeTurnID: null,
        nextSequence: 2,
        composerText: '',
        loading: false,
        errorMessage: null
      },
      interrupt: vi.fn(),
      sendMessage: vi.fn(),
      setComposerText: vi.fn()
    })

    render(<CubeBoxPanel />)

    const message = screen.getByText((_, element) => element?.textContent === '1) 关于我能帮你做什么\n\n2) 关于我“知道什么”')
    expect(message).toBeInTheDocument()
    expect(message).toHaveStyle({ whiteSpace: 'pre-wrap' })
  })

  it('hides settings entry when model settings permission is missing', async () => {
    apiMocks.loadCubeBoxCapabilities.mockResolvedValueOnce({
      conversation: {
        read: true,
        use: true
      },
      settings: {
        read: false,
        verify: false,
        select: false,
        update: false,
        rotate: false,
        deactivate: false
      }
    })

    render(<CubeBoxPanel />)

    await waitFor(() => expect(apiMocks.loadCubeBoxCapabilities).toHaveBeenCalledTimes(1))
    expect(screen.queryByRole('button', { name: '设置' })).not.toBeInTheDocument()
  })

  it('shows local validation error when capability summary is not a json object', async () => {
    render(<CubeBoxPanel />)

    fireEvent.click(await screen.findByRole('button', { name: '设置' }))
    await screen.findByDisplayValue('{"streaming":true}')
    fireEvent.change(screen.getByLabelText('能力摘要'), { target: { value: '[]' } })
    fireEvent.click(screen.getByRole('button', { name: '保存当前模型' }))

    expect(await screen.findByText('能力摘要必须是 JSON 对象。')).toBeInTheDocument()
    expect(apiMocks.selectActiveModel).not.toHaveBeenCalled()
  })

  it('surfaces credential deactivate errors in settings dialog', async () => {
    apiMocks.deactivateModelCredential.mockRejectedValueOnce(new Error('停用失败'))

    render(<CubeBoxPanel />)

    fireEvent.click(await screen.findByRole('button', { name: '设置' }))
    await screen.findByText('sk-**** · v1')
    fireEvent.click(screen.getByRole('button', { name: '停用' }))

    await waitFor(() => expect(screen.getByText('停用失败')).toBeInTheDocument())
  })
})
