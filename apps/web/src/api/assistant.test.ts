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
  commitAssistantTurn,
  confirmAssistantTurn,
  createAssistantConversation,
  createAssistantTurn,
  getAssistantConversation
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
})
