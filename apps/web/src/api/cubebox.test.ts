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
  cancelCubeBoxTask,
  commitCubeBoxTurn,
  confirmCubeBoxTurn,
  createCubeBoxConversation,
  createCubeBoxTurn,
  getCubeBoxConversation,
  getCubeBoxModels,
  getCubeBoxRuntimeStatus,
  getCubeBoxSession,
  getCubeBoxTask,
  getCubeBoxUIBootstrap,
  listCubeBoxConversations,
  logoutCubeBoxSession,
  refreshCubeBoxSession,
  renderCubeBoxTurnReply,
  submitCubeBoxTask
} from './cubebox'

describe('cubebox api', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('creates conversation via cubebox endpoint', async () => {
    postMock.mockResolvedValue({ conversation_id: 'conv_1', turns: [] })
    await createCubeBoxConversation()
    expect(postMock).toHaveBeenCalledWith('/internal/cubebox/conversations', {})
  })

  it('lists conversations with pagination query', async () => {
    getMock.mockResolvedValue({ items: [], next_cursor: '' })
    await listCubeBoxConversations({ page_size: 50, cursor: 'abc' })
    expect(getMock).toHaveBeenCalledWith('/internal/cubebox/conversations?page_size=50&cursor=abc')

    await listCubeBoxConversations()
    expect(getMock).toHaveBeenCalledWith('/internal/cubebox/conversations')
  })

  it('creates turn with encoded conversation id', async () => {
    postMock.mockResolvedValue({ conversation_id: 'conv_1', turns: [] })
    await createCubeBoxTurn('conv_1', 'hello')
    expect(postMock).toHaveBeenCalledWith(
      '/internal/cubebox/conversations/conv_1/turns',
      { user_input: 'hello' },
      { timeout: 60000, retry: 0 }
    )
  })

  it('gets conversation details', async () => {
    getMock.mockResolvedValue({ conversation_id: 'conv_1' })
    await getCubeBoxConversation('conv_1')
    expect(getMock).toHaveBeenCalledWith('/internal/cubebox/conversations/conv_1')
  })

  it('confirms, commits and renders turn actions with suffix routes', async () => {
    postMock.mockResolvedValue({ conversation_id: 'conv_1' })

    await confirmCubeBoxTurn('conv_1', 'turn_1', 'FLOWER-A')
    expect(postMock).toHaveBeenNthCalledWith(
      1,
      '/internal/cubebox/conversations/conv_1/turns/turn_1:confirm',
      { candidate_id: 'FLOWER-A' }
    )

    await commitCubeBoxTurn('conv_1', 'turn_1')
    expect(postMock).toHaveBeenNthCalledWith(
      2,
      '/internal/cubebox/conversations/conv_1/turns/turn_1:commit',
      {}
    )

    await renderCubeBoxTurnReply('conv_1', 'turn_1', { locale: 'zh', allow_missing_turn: false })
    expect(postMock).toHaveBeenNthCalledWith(
      3,
      '/internal/cubebox/conversations/conv_1/turns/turn_1:reply',
      { locale: 'zh', allow_missing_turn: false }
    )
  })

  it('loads models and runtime status from cubebox endpoints', async () => {
    getMock.mockResolvedValueOnce({ models: [] })
    getMock.mockResolvedValueOnce({
      status: 'healthy',
      checked_at: '2026-03-03T17:00:00Z',
      frontend: { healthy: 'healthy' },
      backend: { healthy: 'healthy' },
      knowledge_runtime: { healthy: 'healthy' },
      model_gateway: { healthy: 'healthy' },
      file_store: { healthy: 'healthy' },
      retired_capabilities: [],
      capabilities: {
        conversation_enabled: true,
        files_enabled: true,
        agents_ui_enabled: false,
        agents_write_enabled: false,
        memory_enabled: false,
        web_search_enabled: false,
        file_search_enabled: false,
        mcp_enabled: false
      }
    })

    await getCubeBoxModels()
    expect(getMock).toHaveBeenNthCalledWith(1, '/internal/cubebox/models')

    await getCubeBoxRuntimeStatus()
    expect(getMock).toHaveBeenNthCalledWith(2, '/internal/cubebox/runtime-status')
  })

  it('calls cubebox bootstrap and session endpoints', async () => {
    getMock.mockResolvedValue({ contract_version: 'v1' })
    postMock.mockResolvedValue({ contract_version: 'v1', authenticated: true })

    await getCubeBoxUIBootstrap()
    expect(getMock).toHaveBeenNthCalledWith(1, '/internal/cubebox/ui-bootstrap')

    await getCubeBoxSession()
    expect(getMock).toHaveBeenNthCalledWith(2, '/internal/cubebox/session')

    await refreshCubeBoxSession()
    expect(postMock).toHaveBeenNthCalledWith(1, '/internal/cubebox/session/refresh', {})

    await logoutCubeBoxSession()
    expect(postMock).toHaveBeenNthCalledWith(2, '/internal/cubebox/session/logout', {})
  })

  it('calls cubebox task lifecycle endpoints', async () => {
    postMock.mockResolvedValue({ task_id: 'task_1' })
    getMock.mockResolvedValue({ task_id: 'task_1', status: 'queued' })

    await submitCubeBoxTask({
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
    expect(postMock).toHaveBeenNthCalledWith(1, '/internal/cubebox/tasks', {
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

    await getCubeBoxTask('task_1')
    expect(getMock).toHaveBeenCalledWith('/internal/cubebox/tasks/task_1')

    await cancelCubeBoxTask('task_1')
    expect(postMock).toHaveBeenNthCalledWith(2, '/internal/cubebox/tasks/task_1:cancel', {})
  })
})
