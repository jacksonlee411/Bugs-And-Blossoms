import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn()
}))

vi.mock('./httpClient', () => ({
  httpClient: {
    get: getMock,
    post: postMock
  }
}))

import {
  cancelAssistantTask,
  applyAssistantModelProviders,
  commitAssistantTurn,
  confirmAssistantTurn,
  createAssistantConversation,
  createAssistantTurn,
  listAssistantConversations,
  getAssistantTask,
  getAssistantConversation,
  getAssistantModelProviders,
  getAssistantModels,
  submitAssistantTask,
  validateAssistantModelProviders
} from './assistant'

describe('assistant api', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('creates conversation via internal assistant endpoint', async () => {
    postMock.mockResolvedValue({ conversation_id: 'conv_1', turns: [] })
    await createAssistantConversation()
    expect(postMock).toHaveBeenCalledWith('/internal/assistant/conversations', {})
  })

  it('lists conversations with pagination query', async () => {
    getMock.mockResolvedValue({ items: [], next_cursor: '' })
    await listAssistantConversations({ page_size: 50, cursor: 'abc' })
    expect(getMock).toHaveBeenCalledWith('/internal/assistant/conversations?page_size=50&cursor=abc')

    await listAssistantConversations()
    expect(getMock).toHaveBeenCalledWith('/internal/assistant/conversations')
  })

  it('creates turn with encoded conversation id', async () => {
    postMock.mockResolvedValue({ conversation_id: 'conv_1', turns: [] })
    await createAssistantTurn('conv_1', 'hello')
    expect(postMock).toHaveBeenCalledWith('/internal/assistant/conversations/conv_1/turns', { user_input: 'hello' })
  })

  it('gets conversation details', async () => {
    getMock.mockResolvedValue({ conversation_id: 'conv_1' })
    await getAssistantConversation('conv_1')
    expect(getMock).toHaveBeenCalledWith('/internal/assistant/conversations/conv_1')
  })

  it('confirms and commits turn with suffix actions', async () => {
    postMock.mockResolvedValue({ conversation_id: 'conv_1' })
    await confirmAssistantTurn('conv_1', 'turn_1', 'FLOWER-A')
    expect(postMock).toHaveBeenNthCalledWith(
      1,
      '/internal/assistant/conversations/conv_1/turns/turn_1:confirm',
      { candidate_id: 'FLOWER-A' }
    )

    await commitAssistantTurn('conv_1', 'turn_1')
    expect(postMock).toHaveBeenNthCalledWith(
      2,
      '/internal/assistant/conversations/conv_1/turns/turn_1:commit',
      {}
    )
  })

  it('calls model provider governance endpoints', async () => {
    getMock.mockResolvedValue({ providers: [] })
    postMock.mockResolvedValue({ valid: true, normalized: { provider_routing: {}, providers: [] } })

    await getAssistantModelProviders()
    expect(getMock).toHaveBeenCalledWith('/internal/assistant/model-providers')

    await validateAssistantModelProviders({
      provider_routing: { strategy: 'priority_failover', fallback_enabled: true },
      providers: []
    })
    expect(postMock).toHaveBeenNthCalledWith(1, '/internal/assistant/model-providers:validate', {
      provider_routing: { strategy: 'priority_failover', fallback_enabled: true },
      providers: []
    })

    await applyAssistantModelProviders({
      provider_routing: { strategy: 'priority_failover', fallback_enabled: true },
      providers: []
    })
    expect(postMock).toHaveBeenNthCalledWith(2, '/internal/assistant/model-providers:apply', {
      provider_routing: { strategy: 'priority_failover', fallback_enabled: true },
      providers: []
    })

    await getAssistantModels()
    expect(getMock).toHaveBeenNthCalledWith(2, '/internal/assistant/models')
  })

  it('calls assistant task lifecycle endpoints', async () => {
    postMock.mockResolvedValue({ task_id: 'task_1' })
    getMock.mockResolvedValue({ task_id: 'task_1', status: 'queued' })

    await submitAssistantTask({
      conversation_id: 'conv_1',
      turn_id: 'turn_1',
      task_type: 'assistant_async_plan',
      request_id: 'request_1',
      trace_id: 'trace_1',
      contract_snapshot: {
        intent_schema_version: 'v1',
        compiler_contract_version: 'v1',
        capability_map_version: 'v1',
        skill_manifest_digest: 'digest',
        context_hash: 'ctx',
        intent_hash: 'intent',
        plan_hash: 'plan'
      }
    })
    expect(postMock).toHaveBeenNthCalledWith(1, '/internal/assistant/tasks', {
      conversation_id: 'conv_1',
      turn_id: 'turn_1',
      task_type: 'assistant_async_plan',
      request_id: 'request_1',
      trace_id: 'trace_1',
      contract_snapshot: {
        intent_schema_version: 'v1',
        compiler_contract_version: 'v1',
        capability_map_version: 'v1',
        skill_manifest_digest: 'digest',
        context_hash: 'ctx',
        intent_hash: 'intent',
        plan_hash: 'plan'
      }
    })

    await getAssistantTask('task_1')
    expect(getMock).toHaveBeenCalledWith('/internal/assistant/tasks/task_1')

    await cancelAssistantTask('task_1')
    expect(postMock).toHaveBeenNthCalledWith(2, '/internal/assistant/tasks/task_1:cancel', {})
  })
})
