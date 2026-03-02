import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const assistantAPIMocks = vi.hoisted(() => ({
  commitAssistantTurn: vi.fn(),
  confirmAssistantTurn: vi.fn(),
  createAssistantConversation: vi.fn(),
  createAssistantTurn: vi.fn(),
  getAssistantConversation: vi.fn()
}))

vi.mock('../../api/assistant', () => assistantAPIMocks)

import { AssistantPage } from './AssistantPage'

function makeTurn(overrides: Record<string, unknown> = {}) {
  return {
    turn_id: 'turn_1',
    user_input: 'input',
    state: 'validated',
    risk_tier: 'high',
    request_id: 'request_1',
    trace_id: 'trace_1',
    policy_version: 'v1',
    composition_version: 'v1',
    mapping_version: 'v1',
    intent: {
      action: 'create_orgunit',
      parent_ref_text: '鲜花组织',
      entity_name: '运营部',
      effective_date: '2026-01-01'
    },
    ambiguity_count: 2,
    confidence: 0.8,
    candidates: [
      {
        candidate_id: 'FLOWER-A',
        candidate_code: 'FLOWER-A',
        name: '鲜花组织',
        path: '/鲜花组织/华东',
        as_of: '2026-01-01',
        is_active: true,
        match_score: 0.9
      },
      {
        candidate_id: 'FLOWER-B',
        candidate_code: 'FLOWER-B',
        name: '鲜花组织',
        path: '/鲜花组织/华南',
        as_of: '2026-01-01',
        is_active: true,
        match_score: 0.8
      }
    ],
    plan: {
      title: '创建组织计划',
      capability_key: 'org.orgunit_create.field_policy',
      summary: '在指定父组织下创建部门'
    },
    dry_run: {
      explain: '检测到多个同名父组织候选，需先确认候选主键',
      diff: [{ field: 'parent_candidate_id', after: 'pending_confirmation' }]
    },
    ...overrides
  }
}

function makeConversation(overrides: Record<string, unknown> = {}) {
  return {
    conversation_id: 'conv_1',
    tenant_id: 'tenant_1',
    actor_id: 'actor_1',
    actor_role: 'tenant-admin',
    created_at: '2026-03-02T00:00:00Z',
    updated_at: '2026-03-02T00:00:00Z',
    turns: [makeTurn()],
    ...overrides
  }
}

describe('AssistantPage', () => {
  const dispatchBridgeMessage = async (origin: string, data: unknown) => {
    const event = new MessageEvent('message', { data })
    Object.defineProperty(event, 'origin', {
      configurable: true,
      value: origin
    })
    await act(async () => {
      window.dispatchEvent(event)
    })
  }

  beforeEach(() => {
    vi.clearAllMocks()
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(makeConversation())
    assistantAPIMocks.createAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.confirmAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.commitAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.getAssistantConversation.mockResolvedValue(makeConversation())
  })

  it('renders panel, tracking fields and candidate details', async () => {
    render(<AssistantPage />)

    expect(await screen.findByTestId('assistant-transaction-panel')).toBeInTheDocument()
    expect(screen.getByTestId('assistant-librechat-frame')).toBeInTheDocument()
    expect(screen.getByTestId('assistant-conversation-id')).toHaveTextContent('conv_1')
    expect(screen.getByTestId('assistant-turn-id')).toHaveTextContent('turn_1')
    expect(screen.getByTestId('assistant-request-id')).toHaveTextContent('request_1')
    expect(screen.getByTestId('assistant-trace-id')).toHaveTextContent('trace_1')
    expect(screen.getByTestId('assistant-dryrun-explain')).toHaveTextContent('检测到多个同名父组织候选')
    expect(screen.getByTestId('assistant-risk-blocker')).toBeInTheDocument()
    expect(screen.getByTestId('assistant-candidate-blocker')).toBeInTheDocument()
    expect(screen.getByText('鲜花组织 / FLOWER-A / /鲜花组织/华东 / 2026-01-01')).toBeInTheDocument()
    expect(screen.getByTestId('assistant-commit-button')).toBeDisabled()
  })

  it('enables confirm after selecting candidate and enables commit after confirm', async () => {
    assistantAPIMocks.confirmAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'confirmed',
            resolved_candidate_id: 'FLOWER-A',
            resolution_source: 'user_confirmed'
          })
        ]
      })
    )

    render(<AssistantPage />)

    await screen.findByTestId('assistant-candidates')
    const confirmButton = screen.getByTestId('assistant-confirm-button')
    const commitButton = screen.getByTestId('assistant-commit-button')
    expect(confirmButton).toBeDisabled()
    expect(commitButton).toBeDisabled()

    fireEvent.click(screen.getByDisplayValue('FLOWER-A'))
    await waitFor(() => expect(confirmButton).toBeEnabled())

    fireEvent.click(confirmButton)
    await waitFor(() =>
      expect(assistantAPIMocks.confirmAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1', 'FLOWER-A')
    )
    await waitFor(() => expect(commitButton).toBeEnabled())
  })

  it('refreshes conversation when commit returns conversation_state_invalid', async () => {
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(
      makeConversation({
        turns: [makeTurn({ state: 'confirmed', resolved_candidate_id: 'FLOWER-A', ambiguity_count: 1 })]
      })
    )
    assistantAPIMocks.commitAssistantTurn.mockRejectedValue({
      message: '当前会话状态不允许执行该操作。',
      details: {
        code: 'conversation_state_invalid'
      }
    })
    assistantAPIMocks.getAssistantConversation.mockResolvedValue(
      makeConversation({
        turns: [makeTurn({ state: 'validated', resolved_candidate_id: '' })]
      })
    )

    render(<AssistantPage />)

    const commitButton = await screen.findByTestId('assistant-commit-button')
    await waitFor(() => expect(commitButton).toBeEnabled())
    fireEvent.click(commitButton)

    await waitFor(() => expect(assistantAPIMocks.getAssistantConversation).toHaveBeenCalledWith('conv_1'))
    expect(await screen.findByTestId('assistant-error-alert')).toHaveTextContent('当前会话状态不允许执行该操作。')
  })

  it('accepts postMessage only with allowed origin, valid schema and matching nonce/channel', async () => {
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(makeConversation({ turns: [] }))
    render(<AssistantPage />)

    await screen.findByTestId('assistant-librechat-frame')
    const iframe = screen.getByTestId('assistant-librechat-frame')
    const src = iframe.getAttribute('src')
    expect(src).toBeTruthy()
    const iframeURL = new URL(src ?? '', window.location.origin)
    const channel = iframeURL.searchParams.get('channel')
    const nonce = iframeURL.searchParams.get('nonce')
    expect(channel).toBeTruthy()
    expect(nonce).toBeTruthy()

    const input = screen.getByLabelText('输入需求')
    expect(input).toHaveValue(
      '在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。'
    )

    await dispatchBridgeMessage('https://evil.example.com', {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '恶意输入' }
    })
    expect(input).not.toHaveValue('恶意输入')

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce: 'wrong',
      payload: { input: '错误 nonce 输入' }
    })
    expect(input).not.toHaveValue('错误 nonce 输入')

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '来自安全消息桥的输入' }
    })
    await waitFor(() => expect(input).toHaveValue('来自安全消息桥的输入'))
  })
})
