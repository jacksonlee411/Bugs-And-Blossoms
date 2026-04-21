import { createContext, type PropsWithChildren, useCallback, useContext, useEffect, useMemo, useReducer, useRef } from 'react'
import { createConversation, interruptTurn, streamTurn } from './api'
import { cubeboxReducer, initialCubeBoxState } from './reducer'
import type { ConversationReplayResponse, CubeBoxState } from './types'

interface CubeBoxContextValue {
  state: CubeBoxState
  setComposerText: (value: string) => void
  ensureConversation: () => Promise<ConversationReplayResponse | null>
  sendMessage: () => Promise<void>
  interrupt: () => Promise<void>
}

const CubeBoxContext = createContext<CubeBoxContextValue | null>(null)

export function CubeBoxProvider({ children }: PropsWithChildren) {
  const [state, dispatch] = useReducer(cubeboxReducer, initialCubeBoxState)
  const abortRef = useRef<AbortController | null>(null)
  const ensurePromiseRef = useRef<Promise<ConversationReplayResponse | null> | null>(null)
  const conversationRef = useRef(state.conversation)
  const nextSequenceRef = useRef(state.nextSequence)
  const composerTextRef = useRef(state.composerText)
  const turnStatusRef = useRef(state.turnStatus)
  const activeTurnIDRef = useRef(state.activeTurnID)

  useEffect(() => () => abortRef.current?.abort(), [])
  useEffect(() => {
    conversationRef.current = state.conversation
    nextSequenceRef.current = state.nextSequence
    composerTextRef.current = state.composerText
    turnStatusRef.current = state.turnStatus
    activeTurnIDRef.current = state.activeTurnID
  }, [state])

  const ensureConversation = useCallback(async () => {
    if (conversationRef.current) {
      return {
        conversation: conversationRef.current,
        events: [],
        next_sequence: nextSequenceRef.current
      }
    }
    if (ensurePromiseRef.current) {
      return ensurePromiseRef.current
    }
    ensurePromiseRef.current = (async () => {
      dispatch({ type: 'loading_started' })
      try {
        const payload = await createConversation()
        dispatch({ type: 'conversation_loaded', payload })
        return payload
      } catch (error) {
        dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
        return null
      } finally {
        dispatch({ type: 'loading_finished' })
        ensurePromiseRef.current = null
      }
    })()
    return ensurePromiseRef.current
  }, [])

  const sendMessage = useCallback(async () => {
    if (turnStatusRef.current === 'streaming') {
      return
    }

    let conversationID = conversationRef.current?.id ?? ''
    let nextSequence = nextSequenceRef.current
    if (conversationID === '') {
      const payload = await ensureConversation()
      if (!payload) {
        return
      }
      conversationID = payload.conversation.id
      nextSequence = payload.next_sequence
    }

    const prompt = composerTextRef.current.trim()
    if (prompt.length === 0 || conversationID.length === 0) {
      return
    }

    dispatch({ type: 'error_message_set', message: null })
    const controller = new AbortController()
    abortRef.current = controller

    try {
      await streamTurn({
        conversationID,
        prompt,
        nextSequence,
        signal: controller.signal,
        onEvent: (event) => {
          dispatch({ type: 'event_received', payload: event })
        }
      })
    } catch (error) {
      if (controller.signal.aborted) {
        return
      }
      dispatch({
        type: 'stream_failed_locally',
        message: error instanceof Error ? error.message : 'unknown error'
      })
    } finally {
      abortRef.current = null
    }
  }, [ensureConversation])

  const interrupt = useCallback(async () => {
    const turnID = activeTurnIDRef.current
    if (!turnID) {
      return
    }
    await interruptTurn(turnID)
  }, [])

  const value = useMemo<CubeBoxContextValue>(
    () => ({
      state,
      setComposerText: (value) => dispatch({ type: 'composer_changed', value }),
      ensureConversation,
      sendMessage,
      interrupt
    }),
    [ensureConversation, interrupt, sendMessage, state]
  )

  return <CubeBoxContext.Provider value={value}>{children}</CubeBoxContext.Provider>
}

export function useCubeBox() {
  const context = useContext(CubeBoxContext)
  if (!context) {
    throw new Error('CubeBoxProvider missing')
  }
  return context
}
