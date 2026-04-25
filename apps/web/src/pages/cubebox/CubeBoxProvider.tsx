import { createContext, type PropsWithChildren, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { matchPath, useLocation } from 'react-router-dom'
import { createConversation, interruptTurn, listConversations, loadConversation, streamTurn, updateConversation } from './api'
import { cubeboxReducer, initialCubeBoxState } from './reducer'
import type { ConversationReplayResponse, CubeBoxConversationSummary, CubeBoxPageContext, CubeBoxState } from './types'

interface CubeBoxContextValue {
  state: CubeBoxState
  conversations: CubeBoxConversationSummary[]
  conversationsLoading: boolean
  setComposerText: (value: string) => void
  startNewConversation: () => Promise<void>
  refreshConversations: () => Promise<CubeBoxConversationSummary[]>
  restoreLatestConversation: () => Promise<void>
  selectConversation: (conversationID: string) => Promise<void>
  renameConversation: (conversationID: string, title: string) => Promise<void>
  archiveConversation: (conversationID: string, archived: boolean) => Promise<void>
  ensureConversation: () => Promise<ConversationReplayResponse | null>
  sendMessage: () => Promise<void>
  interrupt: () => Promise<void>
}

const CubeBoxContext = createContext<CubeBoxContextValue | null>(null)

export function CubeBoxProvider({ children }: PropsWithChildren) {
  const location = useLocation()
  const [state, dispatch] = useReducer(cubeboxReducer, initialCubeBoxState)
  const [conversations, setConversations] = useState<CubeBoxConversationSummary[]>([])
  const [conversationsLoading, setConversationsLoading] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const ensurePromiseRef = useRef<Promise<ConversationReplayResponse | null> | null>(null)
  const conversationsPromiseRef = useRef<Promise<CubeBoxConversationSummary[]> | null>(null)
  const conversationRef = useRef(state.conversation)
  const nextSequenceRef = useRef(state.nextSequence)
  const composerTextRef = useRef(state.composerText)
  const turnStatusRef = useRef(state.turnStatus)
  const activeTurnIDRef = useRef(state.activeTurnID)
  const pageContextRef = useRef<CubeBoxPageContext | undefined>(undefined)

  useEffect(() => () => abortRef.current?.abort(), [])
  useEffect(() => {
    conversationRef.current = state.conversation
    nextSequenceRef.current = state.nextSequence
    composerTextRef.current = state.composerText
    turnStatusRef.current = state.turnStatus
    activeTurnIDRef.current = state.activeTurnID
  }, [state])
  useEffect(() => {
    pageContextRef.current = buildCubeBoxPageContext(location.pathname, location.search)
  }, [location.pathname, location.search])

  const refreshConversations = useCallback(async () => {
    if (conversationsPromiseRef.current) {
      return conversationsPromiseRef.current
    }

    setConversationsLoading(true)

    conversationsPromiseRef.current = (async () => {
      try {
        const payload = await listConversations()
        setConversations(payload.items)
        return payload.items
      } catch (error) {
        setConversations([])
        throw error
      } finally {
        setConversationsLoading(false)
        conversationsPromiseRef.current = null
      }
    })()

    return conversationsPromiseRef.current
  }, [])

  const selectConversation = useCallback(async (conversationID: string) => {
    dispatch({ type: 'loading_started' })
    try {
      const payload = await loadConversation(conversationID)
      dispatch({ type: 'conversation_loaded', payload })
    } catch (error) {
      dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
    } finally {
      dispatch({ type: 'loading_finished' })
    }
  }, [])

  const restoreLatestConversation = useCallback(async () => {
    try {
      const items = await refreshConversations()
      const latest = items.find((item) => !item.archived)
      if (!latest) {
        return
      }
      if (conversationRef.current?.id === latest.id) {
        return
      }
      await selectConversation(latest.id)
    } catch (error) {
      dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
    }
  }, [refreshConversations, selectConversation])

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
        await refreshConversations()
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
  }, [refreshConversations])

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

    const prompt = composerTextRef.current
    if (prompt.trim().length === 0 || conversationID.length === 0) {
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
        pageContext: pageContextRef.current,
        signal: controller.signal,
        onEvent: (event) => {
          dispatch({ type: 'event_received', payload: event })
        }
      })
      await refreshConversations()
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
  }, [ensureConversation, refreshConversations])

  const interrupt = useCallback(async () => {
    const turnID = activeTurnIDRef.current
    const conversationID = conversationRef.current?.id ?? ''
    if (!turnID || conversationID.length === 0) {
      return
    }
    try {
      await interruptTurn(turnID, conversationID)
    } catch (error) {
      dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
    }
  }, [])

  const startNewConversation = useCallback(async () => {
    if (turnStatusRef.current === 'streaming') {
      return
    }
    dispatch({ type: 'loading_started' })
    dispatch({ type: 'error_message_set', message: null })
    try {
      const payload = await createConversation()
      dispatch({ type: 'conversation_loaded', payload })
      await refreshConversations()
    } catch (error) {
      dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
    } finally {
      dispatch({ type: 'loading_finished' })
    }
  }, [refreshConversations])

  const renameConversation = useCallback(
    async (conversationID: string, title: string) => {
      try {
        const payload = await updateConversation({ conversationID, title })
        dispatch({ type: 'conversation_loaded', payload })
        await refreshConversations()
      } catch (error) {
        dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
      }
    },
    [refreshConversations]
  )

  const archiveConversation = useCallback(
    async (conversationID: string, archived: boolean) => {
      try {
        const payload = await updateConversation({ conversationID, archived })
        dispatch({ type: 'conversation_loaded', payload })
        await refreshConversations()
      } catch (error) {
        dispatch({ type: 'error_message_set', message: error instanceof Error ? error.message : 'unknown error' })
      }
    },
    [refreshConversations]
  )

  useEffect(() => {
    void refreshConversations().catch(() => {})
    void restoreLatestConversation()
  }, [refreshConversations, restoreLatestConversation])

  const value = useMemo<CubeBoxContextValue>(
    () => ({
      state,
      conversations,
      conversationsLoading,
      setComposerText: (value) => dispatch({ type: 'composer_changed', value }),
      startNewConversation,
      refreshConversations,
      restoreLatestConversation,
      selectConversation,
      renameConversation,
      archiveConversation,
      ensureConversation,
      sendMessage,
      interrupt
    }),
    [
      archiveConversation,
      conversations,
      conversationsLoading,
      ensureConversation,
      interrupt,
      refreshConversations,
      renameConversation,
      restoreLatestConversation,
      selectConversation,
      sendMessage,
      startNewConversation,
      state
    ]
  )

  return <CubeBoxContext.Provider value={value}>{children}</CubeBoxContext.Provider>
}

function buildCubeBoxPageContext(pathname: string, search: string): CubeBoxPageContext | undefined {
  const page = normalizePathname(pathname)
  if (!page) {
    return undefined
  }
  const params = new URLSearchParams(search)
  const asOf = resolvePageContextAsOf(params)

  if (page === '/org/units') {
    const selectedNode = normalizeEntityKey(params.get('node'))
    return compactPageContext({
      page,
      business_object: 'orgunit',
      current_object: selectedNode ? { domain: 'orgunit', entity_key: selectedNode } : undefined,
      view: asOf ? { as_of: asOf } : undefined
    })
  }

  if (page === '/org/units/field-configs') {
    return compactPageContext({
      page,
      business_object: 'conversation',
      view: asOf ? { as_of: asOf } : undefined
    })
  }

  const detailMatch = matchPath('/org/units/:orgCode', page)
  if (detailMatch) {
    const orgCode = normalizeEntityKey(detailMatch.params.orgCode ?? '')
    return compactPageContext({
      page,
      business_object: 'orgunit',
      current_object: orgCode ? { domain: 'orgunit', entity_key: orgCode } : undefined,
      view: asOf ? { as_of: asOf } : undefined
    })
  }

  return compactPageContext({
    page,
    business_object: 'conversation'
  })
}

function compactPageContext(value: CubeBoxPageContext): CubeBoxPageContext | undefined {
  const page = normalizePathname(value.page ?? '')
  const businessObject = normalizeSlug(value.business_object ?? '')
  const currentDomain = normalizeSlug(value.current_object?.domain ?? '')
  const currentEntityKey = normalizeEntityKey(value.current_object?.entity_key ?? '')
  const currentLabel = normalizeLabel(value.current_object?.label ?? '')
  const asOf = normalizeAsOf(value.view?.as_of ?? '')

  const currentObject =
    currentDomain || currentEntityKey || currentLabel
      ? {
          ...(currentDomain ? { domain: currentDomain } : {}),
          ...(currentEntityKey ? { entity_key: currentEntityKey } : {}),
          ...(currentLabel ? { label: currentLabel } : {})
        }
      : undefined
  const view = asOf ? { as_of: asOf } : undefined
  if (!page && !businessObject && !currentObject && !view) {
    return undefined
  }
  return {
    ...(page ? { page } : {}),
    ...(businessObject ? { business_object: businessObject } : {}),
    ...(currentObject ? { current_object: currentObject } : {}),
    ...(view ? { view } : {})
  }
}

function normalizePathname(value: string): string {
  const trimmed = value.trim()
  if (!trimmed.startsWith('/')) {
    return ''
  }
  return trimmed.length > 160 ? trimmed.slice(0, 160) : trimmed
}

function normalizeSlug(value: string): string {
  const trimmed = value.trim().toLowerCase()
  return /^[a-z0-9][a-z0-9._-]{0,63}$/.test(trimmed) ? trimmed : ''
}

function normalizeEntityKey(value: string): string {
  const trimmed = value.trim()
  return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(trimmed) ? trimmed : ''
}

function normalizeLabel(value: string): string {
  const trimmed = value.trim()
  if (!trimmed || /[\r\n\t]/.test(trimmed)) {
    return ''
  }
  return trimmed.length > 120 ? trimmed.slice(0, 120) : trimmed
}

function normalizeAsOf(value: string | null): string {
  const trimmed = (value ?? '').trim()
  return /^\d{4}-\d{2}-\d{2}$/.test(trimmed) ? trimmed : ''
}

function resolvePageContextAsOf(params: URLSearchParams): string {
  const effectiveDate = normalizeAsOf(params.get('effective_date'))
  if (effectiveDate) {
    return effectiveDate
  }
  return normalizeAsOf(params.get('as_of'))
}

export function useCubeBox() {
  const context = useContext(CubeBoxContext)
  if (!context) {
    throw new Error('CubeBoxProvider missing')
  }
  return context
}
