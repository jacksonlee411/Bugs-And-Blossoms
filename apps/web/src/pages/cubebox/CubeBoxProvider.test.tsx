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
