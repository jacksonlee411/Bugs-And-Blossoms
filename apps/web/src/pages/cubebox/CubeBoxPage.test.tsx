import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const cubeboxAPIMocks = vi.hoisted(() => ({
  getCubeBoxRuntimeStatus: vi.fn(),
  listCubeBoxConversations: vi.fn(),
  getCubeBoxConversation: vi.fn(),
  listCubeBoxFiles: vi.fn(),
  createCubeBoxConversation: vi.fn(),
  createCubeBoxTurn: vi.fn(),
  renderCubeBoxTurnReply: vi.fn(),
  confirmCubeBoxTurn: vi.fn(),
  commitCubeBoxTurn: vi.fn(),
  getCubeBoxTask: vi.fn(),
  uploadCubeBoxFile: vi.fn()
}))

const navigateMock = vi.fn()
const routeState = vi.hoisted(() => ({
  params: { conversationId: 'conv_1' as string | undefined }
}))

vi.mock('../../api/cubebox', () => cubeboxAPIMocks)
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useParams: () => routeState.params
  }
})

import { CubeBoxPage } from './CubeBoxPage'

describe('CubeBoxPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeState.params = { conversationId: 'conv_1' }
    cubeboxAPIMocks.getCubeBoxRuntimeStatus.mockResolvedValue({
      status: 'healthy',
      checked_at: '2026-04-13T01:00:00Z',
      frontend: { healthy: 'healthy' },
      backend: { healthy: 'healthy' },
      knowledge_runtime: { healthy: 'healthy' },
      model_gateway: { healthy: 'healthy' },
      file_store: { healthy: 'healthy' },
      retired_capabilities: ['memory', 'mcp'],
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
    cubeboxAPIMocks.listCubeBoxConversations.mockResolvedValue({
      items: [
        {
          conversation_id: 'conv_1',
          state: 'draft',
          updated_at: '2026-04-13T01:10:00Z'
        }
      ],
      next_cursor: ''
    })
    cubeboxAPIMocks.getCubeBoxConversation.mockResolvedValue({
      conversation_id: 'conv_1',
      tenant_id: 'tenant_1',
      actor_id: 'actor_1',
      actor_role: 'tenant_admin',
      state: 'draft',
      created_at: '2026-04-13T01:00:00Z',
      updated_at: '2026-04-13T01:10:00Z',
      turns: [
        {
          turn_id: 'turn_1',
          user_input: '创建一个新的运营部门',
          state: 'draft',
          risk_tier: 'high',
          request_id: 'req_1',
          trace_id: 'trace_1',
          policy_version: 'v1',
          composition_version: 'v1',
          mapping_version: 'v1',
          intent: { action: 'create_orgunit' },
          ambiguity_count: 0,
          confidence: 0.9,
          candidates: [],
          plan: {
            title: '创建组织',
            capability_key: 'org.assistant_conversation.manage',
            summary: '将在鲜花组织下创建运营部'
          },
          dry_run: {
            explain: '将创建运营部',
            diff: []
          }
        }
      ]
    })
    cubeboxAPIMocks.listCubeBoxFiles.mockResolvedValue({
      items: [
        {
          file_id: 'file_1',
          file_name: 'brief.pdf',
          media_type: 'application/pdf',
          size_bytes: 12,
          sha256: 'abc',
          storage_key: 'tenant_1/file_1/brief.pdf',
          uploaded_by: 'actor_1',
          uploaded_at: '2026-04-13T01:12:00Z',
          conversation_id: 'conv_1'
        }
      ]
    })
  })

  it('renders CubeBox runtime, conversations and files', async () => {
    render(<CubeBoxPage />)

    await waitFor(() => expect(cubeboxAPIMocks.getCubeBoxRuntimeStatus).toHaveBeenCalled())
    await waitFor(() => expect(cubeboxAPIMocks.getCubeBoxConversation).toHaveBeenCalledWith('conv_1'))

    expect(screen.getByRole('heading', { name: 'CubeBox' })).toBeInTheDocument()
    expect(screen.getByTestId('cubebox-runtime-status')).toHaveTextContent('healthy')
    expect(screen.getByTestId('cubebox-conversation-item')).toHaveTextContent('conv_1')
    expect(screen.getByText('创建一个新的运营部门')).toBeInTheDocument()
    expect(screen.getByText('brief.pdf · application/pdf')).toBeInTheDocument()
    expect(screen.getByText('memory: retired')).toBeInTheDocument()
  })

})
