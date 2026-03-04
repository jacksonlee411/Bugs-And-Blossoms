import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const assistantAPIMocks = vi.hoisted(() => ({
  cancelAssistantTask: vi.fn(),
  commitAssistantTurn: vi.fn(),
  confirmAssistantTurn: vi.fn(),
  createAssistantConversation: vi.fn(),
  createAssistantTurn: vi.fn(),
  getAssistantRuntimeStatus: vi.fn(),
  getAssistantConversation: vi.fn(),
  listAssistantConversations: vi.fn(),
  getAssistantTask: vi.fn(),
  submitAssistantTask: vi.fn()
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
      effective_date: '2026-01-01',
      intent_schema_version: 'intent-v1',
      context_hash: 'ctx-hash',
      intent_hash: 'intent-hash'
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
      summary: '在指定父组织下创建部门',
      capability_map_version: 'cap-v1',
      compiler_contract_version: 'compiler-v1',
      skill_manifest_digest: 'skill-v1'
    },
    dry_run: {
      explain: '检测到多个同名父组织候选，需先确认候选主键',
      diff: [{ field: 'parent_candidate_id', after: 'pending_confirmation' }],
      plan_hash: 'plan-hash'
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
    state: 'validated',
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
    assistantAPIMocks.listAssistantConversations.mockResolvedValue({
      items: [
        {
          conversation_id: 'conv_1',
          state: 'validated',
          updated_at: '2026-03-02T00:00:00Z',
          last_turn: {
            turn_id: 'turn_1',
            user_input: 'input',
            state: 'validated',
            risk_tier: 'high'
          }
        }
      ],
      next_cursor: ''
    })
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(makeConversation())
    assistantAPIMocks.createAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.getAssistantRuntimeStatus.mockResolvedValue({
      status: 'healthy',
      checked_at: '2026-03-02T00:00:00Z',
      upstream: {
        url: 'http://127.0.0.1:3080',
        repo: 'danny-avila/LibreChat',
        ref: 'main'
      },
      services: [
        { name: 'api', required: true, healthy: 'healthy' },
        { name: 'mongodb', required: true, healthy: 'healthy' }
      ]
    })
    assistantAPIMocks.confirmAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.commitAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.getAssistantConversation.mockResolvedValue(makeConversation())
    assistantAPIMocks.submitAssistantTask.mockResolvedValue({
      task_id: 'task_1',
      task_type: 'assistant_async_plan',
      status: 'queued',
      workflow_id: 'wf_1',
      submitted_at: '2026-03-02T00:00:00Z',
      poll_uri: '/internal/assistant/tasks/task_1'
    })
    assistantAPIMocks.getAssistantTask.mockResolvedValue({
      task_id: 'task_1',
      task_type: 'assistant_async_plan',
      status: 'succeeded',
      dispatch_status: 'started',
      attempt: 1,
      max_attempts: 3,
      last_error_code: '',
      workflow_id: 'wf_1',
      request_id: 'request_1',
      trace_id: 'trace_1',
      conversation_id: 'conv_1',
      turn_id: 'turn_1',
      submitted_at: '2026-03-02T00:00:00Z',
      updated_at: '2026-03-02T00:00:01Z',
      contract_snapshot: {
        intent_schema_version: 'intent-v1',
        compiler_contract_version: 'compiler-v1',
        capability_map_version: 'cap-v1',
        skill_manifest_digest: 'skill-v1',
        context_hash: 'ctx-hash',
        intent_hash: 'intent-hash',
        plan_hash: 'plan-hash'
      }
    })
    assistantAPIMocks.cancelAssistantTask.mockResolvedValue({
      task_id: 'task_1',
      task_type: 'assistant_async_plan',
      status: 'canceled',
      dispatch_status: 'failed',
      attempt: 1,
      max_attempts: 3,
      last_error_code: '',
      workflow_id: 'wf_1',
      request_id: 'request_1',
      trace_id: 'trace_1',
      conversation_id: 'conv_1',
      turn_id: 'turn_1',
      submitted_at: '2026-03-02T00:00:00Z',
      updated_at: '2026-03-02T00:00:01Z',
      contract_snapshot: {
        intent_schema_version: 'intent-v1',
        compiler_contract_version: 'compiler-v1',
        capability_map_version: 'cap-v1',
        skill_manifest_digest: 'skill-v1',
        context_hash: 'ctx-hash',
        intent_hash: 'intent-hash',
        plan_hash: 'plan-hash'
      },
      cancel_accepted: true
    })
  })

  it('renders panel, tracking fields and candidate details', async () => {
    render(<AssistantPage />)

    expect(await screen.findByTestId('assistant-transaction-panel')).toBeInTheDocument()
    expect(await screen.findByTestId('assistant-runtime-alert')).toHaveTextContent('LibreChat Runtime：status=healthy')
    expect(screen.getByTestId('assistant-librechat-frame')).toBeInTheDocument()
    expect(screen.getByTestId('assistant-conversation-id')).toHaveTextContent('conv_1')
    expect(screen.getByTestId('assistant-turn-id')).toHaveTextContent('turn_1')
    expect(screen.getByTestId('assistant-request-id')).toHaveTextContent('request_1')
    expect(screen.getByTestId('assistant-trace-id')).toHaveTextContent('trace_1')
    expect(await screen.findByTestId('assistant-dryrun-explain')).toHaveTextContent('检测到多个同名父组织候选')
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
    assistantAPIMocks.getAssistantConversation
      .mockResolvedValueOnce(
        makeConversation({
          turns: [makeTurn({ state: 'confirmed', resolved_candidate_id: 'FLOWER-A', ambiguity_count: 1 })]
        })
      )
      .mockResolvedValueOnce(
        makeConversation({
          turns: [makeTurn({ state: 'validated', resolved_candidate_id: '' })]
        })
      )
    assistantAPIMocks.commitAssistantTurn.mockRejectedValue({
      message: '当前会话状态不允许执行该操作。',
      details: {
        code: 'conversation_state_invalid'
      }
    })

    render(<AssistantPage />)

    const commitButton = await screen.findByTestId('assistant-commit-button')
    await waitFor(() => expect(commitButton).toBeEnabled())
    fireEvent.click(commitButton)

    await waitFor(() => expect(assistantAPIMocks.getAssistantConversation).toHaveBeenCalledWith('conv_1'))
    expect(await screen.findByTestId('assistant-error-alert')).toHaveTextContent('当前会话状态不允许执行该操作。')
  })

  it('creates conversation when list is empty', async () => {
    assistantAPIMocks.listAssistantConversations.mockResolvedValue({
      items: [],
      next_cursor: ''
    })
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(
      makeConversation({
        turns: [makeTurn({ state: 'confirmed', resolved_candidate_id: 'FLOWER-A', ambiguity_count: 1 })]
      })
    )

    render(<AssistantPage />)

    await waitFor(() => expect(assistantAPIMocks.createAssistantConversation).toHaveBeenCalledTimes(1))
    expect(await screen.findByTestId('assistant-conversation-id')).toHaveTextContent('conv_1')
  })

  it('submits async task and renders task fields', async () => {
    render(<AssistantPage />)

    const submitButton = await screen.findByTestId('assistant-task-submit-button')
    await waitFor(() => expect(submitButton).toBeEnabled())
    fireEvent.click(submitButton)

    await waitFor(() =>
      expect(assistantAPIMocks.submitAssistantTask).toHaveBeenCalledWith({
        conversation_id: 'conv_1',
        turn_id: 'turn_1',
        task_type: 'assistant_async_plan',
        request_id: 'request_1',
        trace_id: 'trace_1',
        contract_snapshot: {
          intent_schema_version: 'intent-v1',
          compiler_contract_version: 'compiler-v1',
          capability_map_version: 'cap-v1',
          skill_manifest_digest: 'skill-v1',
          context_hash: 'ctx-hash',
          intent_hash: 'intent-hash',
          plan_hash: 'plan-hash'
        }
      })
    )
    await waitFor(() => expect(assistantAPIMocks.getAssistantTask).toHaveBeenCalledWith('task_1'))
    expect(screen.getByTestId('assistant-task-id')).toHaveTextContent('task_1')
    expect(screen.getByTestId('assistant-task-workflow-id')).toHaveTextContent('wf_1')
    expect(screen.getByTestId('assistant-task-status')).toHaveTextContent('succeeded')
  })

  it('renders safely when candidates and dry_run.diff are null', async () => {
    assistantAPIMocks.getAssistantConversation.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            candidates: null,
            dry_run: {
              explain: '计划已生成，等待确认后可提交',
              diff: null,
              plan_hash: 'plan-hash'
            }
          })
        ]
      })
    )

    render(<AssistantPage />)

    expect(await screen.findByTestId('assistant-transaction-panel')).toBeInTheDocument()
    expect(await screen.findByTestId('assistant-dryrun-explain')).toHaveTextContent('计划已生成，等待确认后可提交')
    expect(screen.queryByTestId('assistant-candidates')).not.toBeInTheDocument()
  })

  it('shows required-field guidance and blocks actions when intent fields are missing', async () => {
    assistantAPIMocks.getAssistantConversation.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            ambiguity_count: 0,
            candidates: [],
            dry_run: {
              explain: '信息不完整，请通过下一轮对话补充：上级组织；成立日期',
              diff: [],
              validation_errors: ['missing_parent_ref_text', 'missing_effective_date'],
              plan_hash: 'plan-hash'
            }
          })
        ]
      })
    )

    render(<AssistantPage />)

    expect(await screen.findByTestId('assistant-required-field-blocker')).toBeInTheDocument()
    expect(screen.getByText('请补充上级组织名称（例如：鲜花组织）')).toBeInTheDocument()
    expect(screen.getByText('请补充成立日期（YYYY-MM-DD）')).toBeInTheDocument()
    expect(screen.getByTestId('assistant-confirm-button')).toBeDisabled()
    expect(screen.getByTestId('assistant-commit-button')).toBeDisabled()
  })

  it('auto executes create flow from secure bridge message without right-side button clicks', async () => {
    const singleCandidate = [
      {
        candidate_id: 'FLOWER-A',
        candidate_code: 'FLOWER-A',
        name: '鲜花组织',
        path: '/鲜花组织/华东',
        as_of: '2026-01-01',
        is_active: true,
        match_score: 0.98
      }
    ]
    assistantAPIMocks.listAssistantConversations.mockResolvedValue({ items: [], next_cursor: '' })
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(makeConversation({ turns: [] }))
    assistantAPIMocks.createAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'validated',
            ambiguity_count: 1,
            candidates: singleCandidate,
            resolved_candidate_id: 'FLOWER-A',
            dry_run: {
              explain: '计划已生成，等待确认后可提交',
              diff: [],
              validation_errors: [],
              plan_hash: 'plan-hash'
            }
          })
        ]
      })
    )
    assistantAPIMocks.confirmAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'confirmed',
            ambiguity_count: 1,
            candidates: singleCandidate,
            resolved_candidate_id: 'FLOWER-A',
            resolution_source: 'auto',
            dry_run: {
              explain: '确认完成，准备提交',
              diff: [],
              validation_errors: [],
              plan_hash: 'plan-hash'
            }
          })
        ]
      })
    )
    assistantAPIMocks.commitAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'committed',
            ambiguity_count: 1,
            candidates: singleCandidate,
            resolved_candidate_id: 'FLOWER-A',
            commit_result: {
              org_code: 'HR2',
              parent_org_code: 'FLOWER-A',
              effective_date: '2026-01-01',
              event_type: 'CREATE',
              event_uuid: 'evt-1'
            }
          })
        ]
      })
    )

    render(<AssistantPage />)
    const iframe = await screen.findByTestId('assistant-librechat-frame')
    const src = iframe.getAttribute('src')
    const iframeURL = new URL(src ?? '', window.location.origin)
    const channel = iframeURL.searchParams.get('channel')
    const nonce = iframeURL.searchParams.get('nonce')

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '在鲜花组织之下，新建一个名为人力资源部2的部门，成立日期是2026-01-01。' }
    })

    await waitFor(() =>
      expect(assistantAPIMocks.createAssistantTurn).toHaveBeenCalledWith(
        'conv_1',
        '在鲜花组织之下，新建一个名为人力资源部2的部门，成立日期是2026-01-01。'
      )
    )
    await waitFor(() => expect(assistantAPIMocks.confirmAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1', 'FLOWER-A'))
    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1'))
    expect(await screen.findByTestId('assistant-commit-result')).toHaveTextContent('org_code=HR2')
  })

  it('handles candidate disambiguation directly from bridge dialogue message', async () => {
    assistantAPIMocks.confirmAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'confirmed',
            resolved_candidate_id: 'FLOWER-B',
            resolution_source: 'user_confirmed'
          })
        ]
      })
    )
    assistantAPIMocks.commitAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'committed',
            resolved_candidate_id: 'FLOWER-B',
            commit_result: {
              org_code: 'OPS-2',
              parent_org_code: 'FLOWER-B',
              effective_date: '2026-01-01',
              event_type: 'CREATE',
              event_uuid: 'evt-2'
            }
          })
        ]
      })
    )

    render(<AssistantPage />)
    const iframe = await screen.findByTestId('assistant-librechat-frame')
    const src = iframe.getAttribute('src')
    const iframeURL = new URL(src ?? '', window.location.origin)
    const channel = iframeURL.searchParams.get('channel')
    const nonce = iframeURL.searchParams.get('nonce')

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '选第2个，确认执行' }
    })

    await waitFor(() => expect(assistantAPIMocks.confirmAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1', 'FLOWER-B'))
    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1'))
    expect(assistantAPIMocks.createAssistantTurn).not.toHaveBeenCalled()
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
