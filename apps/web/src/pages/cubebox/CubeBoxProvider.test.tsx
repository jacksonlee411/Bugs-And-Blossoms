import { act, renderHook, waitFor } from '@testing-library/react'
import { type PropsWithChildren } from 'react'
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
  return <CubeBoxProvider>{children}</CubeBoxProvider>
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
})
