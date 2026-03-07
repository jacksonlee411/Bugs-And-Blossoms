import { act, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const assistantAPIMocks = vi.hoisted(() => ({
  commitAssistantTurn: vi.fn(),
  confirmAssistantTurn: vi.fn(),
  createAssistantConversation: vi.fn(),
  createAssistantTurn: vi.fn(),
  getAssistantConversation: vi.fn(),
  renderAssistantTurnReply: vi.fn()
}))

vi.mock('../../api/assistant', () => assistantAPIMocks)

import { LibreChatPage } from './LibreChatPage'
import {
  composeCreateOrgUnitPrompt,
  composeStructuredIntentRetryPrompt,
  extractIntentDraftFromText,
  mergeIntentDraft
} from './assistantAutoRun'

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
      parent_ref_text: 'AI治理办公室',
      entity_name: '人力资源部2',
      effective_date: '2026-01-01',
      intent_schema_version: 'intent-v1',
      context_hash: 'ctx-hash',
      intent_hash: 'intent-hash'
    },
    ambiguity_count: 1,
    confidence: 0.8,
    candidates: [
      {
        candidate_id: 'AI-GOV-A',
        candidate_code: 'AI-GOV-A',
        name: 'AI治理办公室',
        path: '/集团/AI治理办公室',
        as_of: '2026-01-01',
        is_active: true,
        match_score: 0.9
      }
    ],
    resolved_candidate_id: 'AI-GOV-A',
    plan: {
      title: '创建组织计划',
      capability_key: 'org.orgunit_create.field_policy',
      summary: '在指定父组织下创建部门',
      capability_map_version: 'cap-v1',
      compiler_contract_version: 'compiler-v1',
      skill_manifest_digest: 'skill-v1'
    },
    dry_run: {
      explain: '计划已生成，可提交',
      diff: [{ field: 'name', after: '人力资源部2' }],
      validation_errors: [],
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

async function dispatchBridgeMessage(origin: string, data: unknown) {
  const event = new MessageEvent('message', { data })
  Object.defineProperty(event, 'origin', {
    configurable: true,
    value: origin
  })
  await act(async () => {
    window.dispatchEvent(event)
  })
}

async function readBridgeTokens() {
  const iframe = await screen.findByTestId('librechat-standalone-frame')
  const src = iframe.getAttribute('src')
  const iframeURL = new URL(src ?? '', window.location.origin)
  return {
    channel: iframeURL.searchParams.get('channel'),
    nonce: iframeURL.searchParams.get('nonce')
  }
}

describe('LibreChatPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(makeConversation({ turns: [] }))
    assistantAPIMocks.createAssistantTurn.mockResolvedValue(makeConversation())
    assistantAPIMocks.confirmAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [makeTurn({ state: 'confirmed', ambiguity_count: 1, resolved_candidate_id: 'AI-GOV-A' })]
      })
    )
    assistantAPIMocks.commitAssistantTurn.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            state: 'committed',
            ambiguity_count: 1,
            resolved_candidate_id: 'AI-GOV-A',
            commit_result: {
              org_code: 'HR2',
              parent_org_code: 'AI-GOV-A',
              effective_date: '2026-01-01',
              event_type: 'CREATE',
              event_uuid: 'evt-1'
            }
          })
        ]
      })
    )
    assistantAPIMocks.getAssistantConversation.mockResolvedValue(makeConversation())
    assistantAPIMocks.renderAssistantTurnReply.mockImplementation(
      async (_conversationID: string, turnID: string, payload: { kind?: string; stage?: string }) => ({
        text: payload.stage === 'commit_failed' ? '模型已生成失败说明。' : '模型已生成业务回复。',
        kind: payload.kind ?? 'info',
        stage: payload.stage ?? 'draft',
        reply_model_name: 'gpt-5.2',
        reply_prompt_version: 'assistant.reply.v1',
        reply_source: 'model',
        used_fallback: false,
        conversation_id: 'conv_1',
        turn_id: turnID
      })
    )
  })

  it('tracks bridge readiness without rendering outer visible notice', async () => {
    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.bridge.ready',
      channel,
      nonce,
      payload: { source: 'assistant-ui-bridge' }
    })

    expect(screen.getByTestId('librechat-bridge-status')).toHaveTextContent('connected')
  })

	it('requires second-turn confirmation before commit for complete input', async () => {
	  render(<LibreChatPage />)
	  const { channel, nonce } = await readBridgeTokens()
	  const completeInput = '在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01'
	  const expectedStructuredPrompt = composeStructuredIntentRetryPrompt(
	    composeCreateOrgUnitPrompt(extractIntentDraftFromText(completeInput))
	  )

	  await dispatchBridgeMessage(window.location.origin, {
	    type: 'assistant.prompt.sync',
	    channel,
	    nonce,
	    payload: { input: completeInput }
	  })

	  await waitFor(() =>
	    expect(assistantAPIMocks.createAssistantTurn).toHaveBeenCalledWith(
	      'conv_1',
	      expectedStructuredPrompt
	    )
	  )
    expect(assistantAPIMocks.confirmAssistantTurn).not.toHaveBeenCalled()
    expect(assistantAPIMocks.commitAssistantTurn).not.toHaveBeenCalled()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '确认执行' }
    })

    await waitFor(() => expect(assistantAPIMocks.confirmAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1', 'AI-GOV-A'))
    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1'))
  })

  it('supports missing-field follow-up completion across two dialogue turns', async () => {
    assistantAPIMocks.createAssistantTurn
      .mockResolvedValueOnce(
        makeConversation({
          turns: [
            makeTurn({
              ambiguity_count: 0,
              candidates: [],
              resolved_candidate_id: '',
              intent: {
                action: 'create_orgunit',
                parent_ref_text: 'AI治理办公室',
                entity_name: '人力资源部239A补全',
                effective_date: ''
              },
              dry_run: {
                explain: '缺少生效日期',
                diff: [],
                validation_errors: ['missing_effective_date'],
                plan_hash: 'plan-hash-1'
              }
            })
          ]
        })
      )
      .mockResolvedValueOnce(
        makeConversation({
          turns: [
            makeTurn({
              intent: {
                action: 'create_orgunit',
                parent_ref_text: 'AI治理办公室',
                entity_name: '人力资源部239A补全',
                effective_date: '2026-03-25'
              },
              dry_run: {
                explain: '计划已生成，可提交',
                diff: [{ field: 'effective_date', after: '2026-03-25' }],
                validation_errors: [],
                plan_hash: 'plan-hash-2'
              }
            })
          ]
        })
      )

	  render(<LibreChatPage />)
	  const { channel, nonce } = await readBridgeTokens()
	  const missingFieldFollowUp = '生效日期 2026-03-25'
	  const expectedMergedStructuredPrompt = composeStructuredIntentRetryPrompt(
	    composeCreateOrgUnitPrompt(
	      mergeIntentDraft(
	        {
	          parent_ref_text: 'AI治理办公室',
	          entity_name: '人力资源部239A补全',
	          effective_date: ''
	        },
	        extractIntentDraftFromText(missingFieldFollowUp)
	      )
	    )
	  )

	  await dispatchBridgeMessage(window.location.origin, {
	    type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '在 AI治理办公室 下新建 人力资源部239A补全' }
    })
    await waitFor(() =>
      expect(assistantAPIMocks.createAssistantTurn).toHaveBeenNthCalledWith(1, 'conv_1', '在 AI治理办公室 下新建 人力资源部239A补全')
    )

	  await dispatchBridgeMessage(window.location.origin, {
	    type: 'assistant.prompt.sync',
	    channel,
	    nonce,
	    payload: { input: missingFieldFollowUp }
	  })
	  await waitFor(() =>
	    expect(assistantAPIMocks.createAssistantTurn).toHaveBeenNthCalledWith(
	      2,
	      'conv_1',
	      expectedMergedStructuredPrompt
	    )
	  )
    expect(assistantAPIMocks.confirmAssistantTurn).not.toHaveBeenCalled()
    expect(assistantAPIMocks.commitAssistantTurn).not.toHaveBeenCalled()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '确认提交' }
    })
    await waitFor(() => expect(assistantAPIMocks.confirmAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1', 'AI-GOV-A'))
    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1'))
  })

  it('requires second confirmation after candidate selection', async () => {
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(
      makeConversation({
        turns: [
          makeTurn({
            ambiguity_count: 2,
            resolved_candidate_id: '',
            candidates: [
              {
                candidate_id: 'SSC-1',
                candidate_code: 'SSC-1',
                name: '共享服务中心',
                path: '/集团/共享服务中心/一部',
                as_of: '2026-03-26',
                is_active: true,
                match_score: 0.91
              },
              {
                candidate_id: 'SSC-2',
                candidate_code: 'SSC-2',
                name: '共享服务中心',
                path: '/集团/共享服务中心/二部',
                as_of: '2026-03-26',
                is_active: true,
                match_score: 0.9
              }
            ]
          })
        ]
      })
    )
    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '选第2个' }
    })

    expect(assistantAPIMocks.confirmAssistantTurn).not.toHaveBeenCalled()
    expect(assistantAPIMocks.commitAssistantTurn).not.toHaveBeenCalled()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '确认执行' }
    })

    await waitFor(() => expect(assistantAPIMocks.confirmAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1', 'SSC-2'))
    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1'))
    expect(assistantAPIMocks.createAssistantTurn).not.toHaveBeenCalled()
  })

  it('does not treat normal sentence with 执行 as commit confirmation', async () => {
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(
      makeConversation({ turns: [makeTurn({ state: 'confirmed', resolved_candidate_id: 'AI-GOV-A' })] })
    )
    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '我们继续执行排查这个问题，不要提交。' }
    })

    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).not.toHaveBeenCalled())
    expect(assistantAPIMocks.createAssistantTurn).not.toHaveBeenCalled()
  })

  it('commits directly when confirmed turn receives strict confirmation command', async () => {
    assistantAPIMocks.createAssistantConversation.mockResolvedValue(
      makeConversation({ turns: [makeTurn({ state: 'confirmed', resolved_candidate_id: 'AI-GOV-A' })] })
    )
    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '确认执行' }
    })

    await waitFor(() => expect(assistantAPIMocks.commitAssistantTurn).toHaveBeenCalledWith('conv_1', 'turn_1'))
    expect(assistantAPIMocks.createAssistantTurn).not.toHaveBeenCalled()
  })
  it('does not render synthetic gpt reply when conversation creation fails', async () => {
    assistantAPIMocks.createAssistantConversation.mockRejectedValueOnce(new Error('创建会话失败'))
    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '在鲜花组织之下，新建一个名为运营部的部门' }
    })

    await waitFor(() => expect(screen.getByTestId('librechat-bridge-error')).toHaveTextContent('创建会话失败'))
    expect(assistantAPIMocks.renderAssistantTurnReply).not.toHaveBeenCalled()
  })

  it('renders commit failure through reply api without missing-turn fallback', async () => {
    assistantAPIMocks.commitAssistantTurn.mockRejectedValueOnce({
      message: '提交失败',
      details: { code: 'conversation_state_invalid' }
    })
    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01' }
    })
    await waitFor(() => expect(assistantAPIMocks.createAssistantTurn).toHaveBeenCalled())

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '确认执行' }
    })

    await waitFor(() => expect(assistantAPIMocks.renderAssistantTurnReply).toHaveBeenCalled())
    const lastCall = assistantAPIMocks.renderAssistantTurnReply.mock.calls.at(-1)
    expect(lastCall?.[0]).toBe('conv_1')
    expect(lastCall?.[1]).toBe('turn_1')
    expect(lastCall?.[2]).toMatchObject({
      stage: 'commit_failed',
      kind: 'error',
      outcome: 'failure',
      locale: 'zh'
    })
    expect(lastCall?.[2]).not.toHaveProperty('fallback_text')
    expect(lastCall?.[2]).not.toHaveProperty('allow_missing_turn')
  })

  it('renders first-turn structured failure through reply api with allow-missing-turn', async () => {
    assistantAPIMocks.createAssistantTurn
      .mockRejectedValueOnce({
        message: 'ai plan schema constrained decode failed',
        details: { code: 'ai_plan_schema_constrained_decode_failed' }
      })
      .mockRejectedValueOnce({
        message: 'ai plan schema constrained decode failed',
        details: { code: 'ai_plan_schema_constrained_decode_failed' }
      })

    render(<LibreChatPage />)
    const { channel, nonce } = await readBridgeTokens()

    await dispatchBridgeMessage(window.location.origin, {
      type: 'assistant.prompt.sync',
      channel,
      nonce,
      payload: { input: '在鲜花组织之下，新建一个名为运营部的部门。' }
    })

    await waitFor(() => expect(assistantAPIMocks.renderAssistantTurnReply).toHaveBeenCalled())
    const lastCall = assistantAPIMocks.renderAssistantTurnReply.mock.calls.at(-1)
    expect(lastCall?.[0]).toBe('conv_1')
    expect(lastCall?.[1]).toBe('missing-turn-context')
    expect(lastCall?.[2]).toMatchObject({
      stage: 'missing_fields',
      kind: 'warning',
      outcome: 'failure',
      locale: 'zh',
      allow_missing_turn: true
    })
    expect(String(lastCall?.[2]?.fallback_text || '')).toContain('请补充生效日期')
  })

})
