import { act, renderHook, waitFor } from '@testing-library/react'
import { type PropsWithChildren } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CubeBoxProvider, useCubeBox } from './CubeBoxProvider'

const apiMocks = vi.hoisted(() => ({
  createConversation: vi.fn(),
  interruptTurn: vi.fn(),
  listConversations: vi.fn(),
  loadConversation: vi.fn(),
  streamTurn: vi.fn(),
  updateConversation: vi.fn()
}))

vi.mock('./api', () => apiMocks)

function wrapper({ children }: PropsWithChildren) {
  return (
    <MemoryRouter>
      <CubeBoxProvider>{children}</CubeBoxProvider>
    </MemoryRouter>
  )
}

describe('CubeBoxProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.listConversations.mockResolvedValue({ items: [] })
  })

  it('surfaces restore latest conversation failure as error message', async () => {
    apiMocks.listConversations.mockRejectedValue(new Error('list failed'))

    const { result } = renderHook(() => useCubeBox(), { wrapper })

    await waitFor(() => expect(result.current.state.errorMessage).toBe('list failed'))
    expect(result.current.conversations).toEqual([])
  })

  it('reuses the in-flight conversation list request during restore to avoid clearing successful results', async () => {
    const conversations = [{ id: 'conv_1', title: 'Latest', archived: false, updated_at: '2026-04-22T00:00:00Z' }]
    apiMocks.listConversations.mockImplementation(
      () =>
        new Promise((resolve) => {
          setTimeout(() => resolve({ items: conversations }), 0)
        })
    )
    apiMocks.loadConversation.mockResolvedValue({
      conversation: { id: 'conv_1', title: 'Latest', archived: false, status: 'active' },
      events: [],
      next_sequence: 1
    })

    const { result } = renderHook(() => useCubeBox(), { wrapper })

    await waitFor(() => expect(apiMocks.listConversations).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(result.current.conversations).toEqual(conversations))
    await waitFor(() => expect(apiMocks.loadConversation).toHaveBeenCalledWith('conv_1'))
    expect(result.current.state.errorMessage).toBeNull()
  })

  it('skips archived conversations and restores the most recent active conversation', async () => {
    apiMocks.listConversations.mockResolvedValue({
      items: [
        { id: 'conv_archived', title: 'Archived', status: 'archived', archived: true, updated_at: '2026-04-22T02:00:00Z' },
        { id: 'conv_active', title: 'Active', status: 'active', archived: false, updated_at: '2026-04-22T01:00:00Z' }
      ]
    })
    apiMocks.loadConversation.mockResolvedValue({
      conversation: { id: 'conv_active', title: 'Active', archived: false, status: 'active' },
      events: [],
      next_sequence: 1
    })

    renderHook(() => useCubeBox(), { wrapper })

    await waitFor(() => expect(apiMocks.loadConversation).toHaveBeenCalledWith('conv_active'))
    expect(apiMocks.loadConversation).not.toHaveBeenCalledWith('conv_archived')
  })

  it('surfaces rename failure as error message', async () => {
    apiMocks.updateConversation.mockRejectedValue(new Error('rename failed'))

    const { result } = renderHook(() => useCubeBox(), { wrapper })

    await act(async () => {
      await result.current.renameConversation('conv_1', 'new title')
    })

    await waitFor(() => expect(result.current.state.errorMessage).toBe('rename failed'))
  })

  it('surfaces archive failure as error message', async () => {
    apiMocks.updateConversation.mockRejectedValue(new Error('archive failed'))

    const { result } = renderHook(() => useCubeBox(), { wrapper })

    await act(async () => {
      await result.current.archiveConversation('conv_1', true)
    })

    await waitFor(() => expect(result.current.state.errorMessage).toBe('archive failed'))
  })

  it('preserves composer whitespace when sending message to stream api', async () => {
    apiMocks.createConversation.mockResolvedValue({
      conversation: { id: 'conv_1', title: '新对话', archived: false, status: 'active' },
      events: [],
      next_sequence: 2
    })
    apiMocks.streamTurn.mockResolvedValue(undefined)

    const { result } = renderHook(() => useCubeBox(), { wrapper })

    await act(async () => {
      result.current.setComposerText('\n  hello  \n')
    })

    await act(async () => {
      await result.current.sendMessage()
    })

    expect(apiMocks.streamTurn).toHaveBeenCalledWith(
      expect.objectContaining({
        conversationID: 'conv_1',
        prompt: '\n  hello  \n',
        nextSequence: 2,
        pageContext: {
          page: '/',
          business_object: 'conversation'
        }
      })
    )
  })

  it('derives controlled orgunit page context from current route', async () => {
    apiMocks.createConversation.mockResolvedValue({
      conversation: { id: 'conv_1', title: '新对话', archived: false, status: 'active' },
      events: [],
      next_sequence: 2
    })
    apiMocks.streamTurn.mockResolvedValue(undefined)

    const routeWrapper = ({ children }: PropsWithChildren) => (
      <MemoryRouter initialEntries={['/org/units/100000?as_of=2026-04-25']}>
        <CubeBoxProvider>{children}</CubeBoxProvider>
      </MemoryRouter>
    )
    const { result } = renderHook(() => useCubeBox(), { wrapper: routeWrapper })

    await act(async () => {
      result.current.setComposerText('查该组织详情')
    })

    await act(async () => {
      await result.current.sendMessage()
    })

    expect(apiMocks.streamTurn).toHaveBeenCalledWith(
      expect.objectContaining({
        pageContext: {
          page: '/org/units/100000',
          business_object: 'orgunit',
          current_object: {
            domain: 'orgunit',
            entity_key: '100000'
          },
          view: {
            as_of: '2026-04-25'
          }
        }
      })
    )
  })

  it('prefers effective_date when deriving orgunit detail page context', async () => {
    apiMocks.createConversation.mockResolvedValue({
      conversation: { id: 'conv_1', title: '新对话', archived: false, status: 'active' },
      events: [],
      next_sequence: 2
    })
    apiMocks.streamTurn.mockResolvedValue(undefined)

    const routeWrapper = ({ children }: PropsWithChildren) => (
      <MemoryRouter initialEntries={['/org/units/100000?effective_date=2026-03-01&as_of=2026-04-25']}>
        <CubeBoxProvider>{children}</CubeBoxProvider>
      </MemoryRouter>
    )
    const { result } = renderHook(() => useCubeBox(), { wrapper: routeWrapper })

    await act(async () => {
      result.current.setComposerText('查该组织历史详情')
    })

    await act(async () => {
      await result.current.sendMessage()
    })

    expect(apiMocks.streamTurn).toHaveBeenCalledWith(
      expect.objectContaining({
        pageContext: {
          page: '/org/units/100000',
          business_object: 'orgunit',
          current_object: {
            domain: 'orgunit',
            entity_key: '100000'
          },
          view: {
            as_of: '2026-03-01'
          }
        }
      })
    )
  })

  it('does not treat orgunit field config page as orgunit detail context', async () => {
    apiMocks.createConversation.mockResolvedValue({
      conversation: { id: 'conv_1', title: '新对话', archived: false, status: 'active' },
      events: [],
      next_sequence: 2
    })
    apiMocks.streamTurn.mockResolvedValue(undefined)

    const routeWrapper = ({ children }: PropsWithChildren) => (
      <MemoryRouter initialEntries={['/org/units/field-configs?as_of=2026-04-25']}>
        <CubeBoxProvider>{children}</CubeBoxProvider>
      </MemoryRouter>
    )
    const { result } = renderHook(() => useCubeBox(), { wrapper: routeWrapper })

    await act(async () => {
      result.current.setComposerText('看看当前页面')
    })

    await act(async () => {
      await result.current.sendMessage()
    })

    expect(apiMocks.streamTurn).toHaveBeenCalledWith(
      expect.objectContaining({
        pageContext: {
          page: '/org/units/field-configs',
          business_object: 'conversation',
          view: {
            as_of: '2026-04-25'
          }
        }
      })
    )
    expect(apiMocks.streamTurn).not.toHaveBeenCalledWith(
      expect.objectContaining({
        pageContext: expect.objectContaining({
          current_object: expect.anything()
        })
      })
    )
  })
})
