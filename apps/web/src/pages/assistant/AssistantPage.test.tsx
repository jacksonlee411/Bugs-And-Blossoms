import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const assistantAPIMocks = vi.hoisted(() => ({
  getAssistantRuntimeStatus: vi.fn(),
  listAssistantConversations: vi.fn()
}))

vi.mock('../../api/assistant', () => assistantAPIMocks)

import { AssistantPage } from './AssistantPage'

describe('AssistantPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    assistantAPIMocks.getAssistantRuntimeStatus.mockResolvedValue({
      status: 'healthy',
      checked_at: '2026-03-07T01:00:00Z',
      upstream: { url: 'http://localhost:3080' },
      services: [
        { name: 'assistant-ui', required: true, healthy: 'healthy' },
        { name: 'gateway', required: true, healthy: 'healthy' }
      ],
      capabilities: {
        mcp_enabled: false,
        actions_enabled: true,
        agents_write_enabled: true
      }
    })
    assistantAPIMocks.listAssistantConversations.mockResolvedValue({
      items: [
        {
          conversation_id: 'conv_1',
          state: 'confirmed',
          updated_at: '2026-03-07T01:01:00Z',
          last_turn: {
            turn_id: 'turn_1',
            user_input: '在 AI治理办公室 下新建 人力资源部2',
            state: 'confirmed',
            risk_tier: 'high'
          }
        }
      ],
      next_cursor: ''
    })
  })

  it('renders runtime summary and recent conversation logs', async () => {
    render(<AssistantPage />)

    await waitFor(() => expect(assistantAPIMocks.getAssistantRuntimeStatus).toHaveBeenCalled())
    await waitFor(() => expect(assistantAPIMocks.listAssistantConversations).toHaveBeenCalledWith({ page_size: 10 }))

    expect(screen.getByRole('heading', { name: 'AI 助手日志' })).toBeInTheDocument()
    expect(screen.getByTestId('assistant-runtime-status')).toHaveTextContent('healthy')
    expect(screen.getByTestId('assistant-runtime-upstream-url')).toHaveTextContent('http://localhost:3080')
    expect(screen.getByTestId('assistant-conversation-log-item')).toHaveTextContent('conv_1')
    expect(screen.getByTestId('assistant-conversation-log-item')).toHaveTextContent('在 AI治理办公室 下新建 人力资源部2')
  })

  it('keeps /app/assistant as read-only log page after old bridge retirement', async () => {
    render(<AssistantPage />)

    await screen.findByRole('heading', { name: 'AI 助手日志' })

    expect(screen.getByRole('link', { name: '打开 LibreChat' })).toHaveAttribute('href', '/app/assistant/librechat')
    expect(screen.getByText(/旧 `iframe \+ bridge` 对话承载页已按 `DEV-PLAN-282` 退役/)).toBeInTheDocument()
    expect(screen.getByText(/正式交互入口已统一到 `\/app\/assistant\/librechat`/)).toBeInTheDocument()
    expect(screen.queryByTestId('assistant-librechat-frame')).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Confirm' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Commit' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Submit Task' })).not.toBeInTheDocument()
  })

  it('shows warning when log loading fails', async () => {
    assistantAPIMocks.getAssistantRuntimeStatus.mockRejectedValueOnce(new Error('助手日志加载失败'))

    render(<AssistantPage />)

    await waitFor(() => expect(screen.getByText('助手日志加载失败')).toBeInTheDocument())
  })
})
