import SmartToyIcon from '@mui/icons-material/SmartToy'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  cancelAssistantTask,
  commitAssistantTurn,
  confirmAssistantTurn,
  createAssistantConversation,
  createAssistantTurn,
  getAssistantRuntimeStatus,
  getAssistantTask,
  getAssistantConversation,
  listAssistantConversations,
  submitAssistantTask,
  type AssistantConversation,
  type AssistantConversationListItem,
  type AssistantRuntimeService,
  type AssistantRuntimeStatusResponse,
  type AssistantTaskDetail,
  type AssistantTurn
} from '../../api/assistant'
import {
  createAssistantBridgeToken,
  parseAssistantAllowedOrigins,
  validateAssistantBridgeMessage
} from './assistantMessageBridge'
import { deriveAssistantActionState } from './assistantUiState'
import {
  analyzeTurnForDialog,
  createDialogFlowState,
  formatCandidateConfirmMessage,
  formatCommitSuccessMessage,
  formatMissingFieldMessageText,
  resetDialogFlowForConversation,
  type DialogFlowState,
  type DialogMessageKind,
  type DialogMessageStage,
  withDialogPhase
} from './assistantDialogFlow'
import {
  composeStructuredIntentRetryPrompt,
  composeCreateOrgUnitPrompt,
  extractIntentDraftFromText,
  formatCandidatePrompt,
  isStructuredIntentRetryPrompt,
  isExecutionConfirmationText,
  looksLikeCreateOrgUnitRequest,
  mergeIntentDraft,
  resolveCandidateFromInput,
  shouldRetryStructuredPromptForError
} from './assistantAutoRun'

const samplePrompt =
  '在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。'

const assistantStatePlaceholder = '-'
const assistantTaskPollIntervalMS = 1000
const conversationStorageKey = 'assistant.active_conversation_id'

function latestTurn(conversation: AssistantConversation | null): AssistantTurn | null {
  if (!conversation || !Array.isArray(conversation.turns) || conversation.turns.length === 0) {
    return null
  }
  const turn = conversation.turns[conversation.turns.length - 1]
  return turn ?? null
}

function normalized(value: string | undefined): string {
  return (value ?? '').trim()
}

function resolveCandidateSelection(turn: AssistantTurn | null, currentCandidateID: string): string {
  if (!turn) {
    return ''
  }
  const resolved = normalized(turn.resolved_candidate_id)
  if (resolved.length > 0) {
    return resolved
  }
  const candidates = Array.isArray(turn.candidates) ? turn.candidates : []
  if (candidates.some((candidate) => candidate.candidate_id === currentCandidateID)) {
    return currentCandidateID
  }
  return ''
}

function normalizedDryRunDiff(turn: AssistantTurn | null): Array<Record<string, unknown>> {
  const diff = turn?.dry_run?.diff
  return Array.isArray(diff) ? diff : []
}

function errorMessage(err: unknown, fallback: string): string {
  const message = (err as { message?: string })?.message
  if (typeof message === 'string' && message.trim().length > 0) {
    return message
  }
  return fallback
}

function errorCode(err: unknown): string {
  const details = (err as { details?: { code?: string } })?.details
  return normalized(details?.code)
}

function shouldRefreshConversation(code: string): boolean {
  return code === 'conversation_state_invalid' || code === 'conversation_confirmation_required'
}

function stringifyDiffValue(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function formatDryRunValidationError(code: string): string {
  const normalizedCode = normalized(code)
  switch (normalizedCode) {
    case 'missing_parent_ref_text':
      return '请补充上级组织名称（例如：鲜花组织）'
    case 'missing_entity_name':
      return '请补充部门名称（例如：运营部）'
    case 'missing_effective_date':
      return '请补充成立日期（YYYY-MM-DD）'
    case 'invalid_effective_date_format':
      return '成立日期格式不正确，请使用 YYYY-MM-DD'
    case 'candidate_confirmation_required':
      return '请先确认父组织候选'
    default:
      return normalizedCode
  }
}

function dryRunValidationMessages(turn: AssistantTurn | null): string[] {
  const codes = Array.isArray(turn?.dry_run?.validation_errors) ? turn.dry_run.validation_errors : []
  const messages: string[] = []
  for (const code of codes) {
    const message = formatDryRunValidationError(code)
    if (message.length === 0 || messages.includes(message)) {
      continue
    }
    messages.push(message)
  }
  return messages
}

function assistantTaskTerminal(status: string | undefined): boolean {
  const current = normalized(status)
  return (
    current === 'succeeded' ||
    current === 'failed' ||
    current === 'manual_takeover_required' ||
    current === 'canceled'
  )
}

function summarizeConversation(conversation: AssistantConversation): AssistantConversationListItem {
  const turn = latestTurn(conversation)
  return {
    conversation_id: conversation.conversation_id,
    state: conversation.state,
    updated_at: conversation.updated_at,
    last_turn: turn
      ? {
          turn_id: turn.turn_id,
          user_input: turn.user_input,
          state: turn.state,
          risk_tier: turn.risk_tier
        }
      : undefined
  }
}

function sortConversationItems(items: AssistantConversationListItem[]): AssistantConversationListItem[] {
  return [...items].sort((left, right) => {
    const leftTime = new Date(left.updated_at).getTime()
    const rightTime = new Date(right.updated_at).getTime()
    if (leftTime === rightTime) {
      return right.conversation_id.localeCompare(left.conversation_id)
    }
    return rightTime - leftTime
  })
}

function formatTimestamp(input: string | undefined): string {
  const value = normalized(input)
  if (value.length === 0) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

function formatRuntimeService(service: AssistantRuntimeService): string {
  const status = normalized(service.healthy) || assistantStatePlaceholder
  const reason = normalized(service.reason)
  if (reason.length > 0) {
    return `${service.name}=${status}(${reason})`
  }
  return `${service.name}=${status}`
}

function saveActiveConversationID(conversationID: string) {
  if (typeof window === 'undefined') {
    return
  }
  if (conversationID.trim().length === 0) {
    window.localStorage.removeItem(conversationStorageKey)
    return
  }
  window.localStorage.setItem(conversationStorageKey, conversationID)
}

function loadActiveConversationID(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  return normalized(window.localStorage.getItem(conversationStorageKey) ?? '')
}

export function AssistantPage() {
  const [conversation, setConversation] = useState<AssistantConversation | null>(null)
  const [conversations, setConversations] = useState<AssistantConversationListItem[]>([])
  const [conversationCursor, setConversationCursor] = useState('')
  const [selectedConversationID, setSelectedConversationID] = useState('')
  const [taskDetail, setTaskDetail] = useState<AssistantTaskDetail | null>(null)
  const [input, setInput] = useState(samplePrompt)
  const [candidateID, setCandidateID] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadingList, setLoadingList] = useState(false)
  const [error, setError] = useState('')
  const [runtimeStatus, setRuntimeStatus] = useState<AssistantRuntimeStatusResponse | null>(null)
  const [runtimeError, setRuntimeError] = useState('')
  const [bridgeChannel] = useState(() => createAssistantBridgeToken('assistant_channel'))
  const [bridgeNonce] = useState(() => createAssistantBridgeToken('assistant_nonce'))
  const librechatFrameRef = useRef<HTMLIFrameElement | null>(null)
  const conversationRef = useRef<AssistantConversation | null>(null)
  const dialogFlowRef = useRef<DialogFlowState>(createDialogFlowState())
  const bridgePromptQueueRef = useRef<string[]>([])
  const bridgeWorkerRunningRef = useRef(false)

  const turn = useMemo(() => latestTurn(conversation), [conversation])
  const turnDryRunDiff = useMemo(() => normalizedDryRunDiff(turn), [turn])
  const turnDryRunValidationMessages = useMemo(() => dryRunValidationMessages(turn), [turn])
  const taskID = normalized(taskDetail?.task_id)
  const taskStatus = normalized(taskDetail?.status)
  const taskTerminal = useMemo(() => assistantTaskTerminal(taskStatus), [taskStatus])
  const conversationID = normalized(conversation?.conversation_id)
  const selectedTurnID = normalized(turn?.turn_id)
  const allowedOrigins = useMemo(() => {
    const origin = typeof window !== 'undefined' ? window.location.origin : undefined
    return parseAssistantAllowedOrigins(import.meta.env.VITE_ASSISTANT_ALLOWED_ORIGINS, origin)
  }, [])
  const iframeSrc = useMemo(
    () =>
      `/assistant-ui/?channel=${encodeURIComponent(bridgeChannel)}&nonce=${encodeURIComponent(bridgeNonce)}`,
    [bridgeChannel, bridgeNonce]
  )
  const runtimeSummary = useMemo(() => {
    if (!runtimeStatus) {
      return ''
    }
    const services = Array.isArray(runtimeStatus.services) ? runtimeStatus.services : []
    const summary = services.map((service) => formatRuntimeService(service)).join(' | ')
    if (summary.length > 0) {
      return summary
    }
    return '无运行时服务状态快照'
  }, [runtimeStatus])

  const setDialogFlow = useCallback((next: DialogFlowState) => {
    dialogFlowRef.current = next
  }, [])

  const syncDialogFlowByTurn = useCallback(
    (conversationID: string, nextTurn: AssistantTurn | null) => {
      const base = resetDialogFlowForConversation(conversationID, normalized(nextTurn?.turn_id))
      const analysis = analyzeTurnForDialog(nextTurn)
      setDialogFlow(
        withDialogPhase(base, analysis.phase, {
          pending_draft_summary: analysis.draft_summary,
          missing_fields: analysis.missing_field_messages,
          candidates: analysis.candidates,
          selected_candidate_id: normalized(nextTurn?.resolved_candidate_id)
        })
      )
    },
    [setDialogFlow]
  )

  const applyConversation = useCallback(
    (nextConversation: AssistantConversation) => {
      setConversation(nextConversation)
      setSelectedConversationID(nextConversation.conversation_id)
      saveActiveConversationID(nextConversation.conversation_id)
      setConversations((current) => {
        const summary = summarizeConversation(nextConversation)
        const remaining = current.filter((item) => item.conversation_id !== summary.conversation_id)
        return sortConversationItems([summary, ...remaining])
      })
      const nextTurn = latestTurn(nextConversation)
      setCandidateID((current) => resolveCandidateSelection(nextTurn, current))
      syncDialogFlowByTurn(nextConversation.conversation_id, nextTurn)
    },
    [syncDialogFlowByTurn]
  )

  const actionState = useMemo(
    () =>
      deriveAssistantActionState({
        hasConversation: Boolean(conversation),
        loading,
        selectedCandidateID: candidateID,
        turn
      }),
    [candidateID, conversation, loading, turn]
  )

  useEffect(() => {
    conversationRef.current = conversation
  }, [conversation])

  const loadConversation = useCallback(
    async (targetConversationID: string, options?: { silent?: boolean }) => {
      const normalizedID = normalized(targetConversationID)
      if (normalizedID.length === 0) {
        return
      }
      if (!options?.silent) {
        setLoading(true)
      }
      try {
        const nextConversation = await getAssistantConversation(normalizedID)
        applyConversation(nextConversation)
      } catch (err) {
        setError(errorMessage(err, '加载会话失败'))
      } finally {
        if (!options?.silent) {
          setLoading(false)
        }
      }
    },
    [applyConversation]
  )

  const refreshConversation = useCallback(async () => {
    if (conversationID.length === 0) {
      return
    }
    const nextConversation = await getAssistantConversation(conversationID)
    applyConversation(nextConversation)
  }, [applyConversation, conversationID])

  const refreshConversationList = useCallback(
    async (cursor?: string, append?: boolean) => {
      setLoadingList(true)
      try {
        const response = await listAssistantConversations({
          page_size: 20,
          cursor
        })
        const nextItems = append ? sortConversationItems([...conversations, ...response.items]) : response.items
        const dedup = Array.from(new Map(nextItems.map((item) => [item.conversation_id, item])).values())
        const sorted = sortConversationItems(dedup)
        setConversations(sorted)
        setConversationCursor(response.next_cursor)
        if (!append) {
          const remembered = loadActiveConversationID()
          const preferred =
            sorted.find((item) => item.conversation_id === remembered)?.conversation_id ?? sorted[0]?.conversation_id ?? ''
          if (preferred.length > 0) {
            await loadConversation(preferred, { silent: true })
          }
        }
      } catch (err) {
        setError(errorMessage(err, '加载会话列表失败'))
      } finally {
        setLoadingList(false)
      }
    },
    [conversations, loadConversation]
  )

  const handleCreateConversation = useCallback(async () => {
    setError('')
    setLoading(true)
    try {
      const created = await createAssistantConversation()
      applyConversation(created)
    } catch (err) {
      setError(errorMessage(err, '创建会话失败'))
    } finally {
      setLoading(false)
    }
  }, [applyConversation])

  useEffect(() => {
    let active = true
    void getAssistantRuntimeStatus()
      .then((status) => {
        if (!active) {
          return
        }
        setRuntimeStatus(status)
        setRuntimeError('')
      })
      .catch((err: unknown) => {
        if (!active) {
          return
        }
        setRuntimeError(errorMessage(err, '加载 LibreChat 运行状态失败'))
      })
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    let active = true
    setLoadingList(true)
    listAssistantConversations({ page_size: 20 })
      .then(async (response) => {
        if (!active) {
          return
        }
        setConversationCursor(response.next_cursor)
        const sorted = sortConversationItems(response.items)
        setConversations(sorted)
        const remembered = loadActiveConversationID()
        const preferred =
          sorted.find((item) => item.conversation_id === remembered)?.conversation_id ?? sorted[0]?.conversation_id ?? ''
        if (preferred.length > 0) {
          await loadConversation(preferred, { silent: true })
          return
        }
        const created = await createAssistantConversation()
        if (!active) {
          return
        }
        applyConversation(created)
      })
      .catch((err: unknown) => {
        if (!active) {
          return
        }
        setError(errorMessage(err, '初始化助手会话失败'))
      })
      .finally(() => {
        if (active) {
          setLoadingList(false)
        }
      })

    return () => {
      active = false
    }
  }, [applyConversation, loadConversation])

  useEffect(() => {
    setTaskDetail(null)
  }, [selectedTurnID, selectedConversationID])

  const handleGenerate = useCallback(async () => {
    if (!conversation) {
      return
    }
    const text = input.trim()
    if (!text) {
      setError('请输入对话内容')
      return
    }
    setError('')
    setLoading(true)
    try {
      let next: AssistantConversation
      try {
        next = await createAssistantTurn(conversation.conversation_id, text)
      } catch (err) {
        const code = errorCode(err)
        if (shouldRetryStructuredPromptForError(code) && !isStructuredIntentRetryPrompt(text)) {
          const retryPrompt = composeStructuredIntentRetryPrompt(text)
          next = await createAssistantTurn(conversation.conversation_id, retryPrompt)
        } else {
          throw err
        }
      }
      applyConversation(next)
    } catch (err) {
      setError(errorMessage(err, '生成计划失败'))
    } finally {
      setLoading(false)
    }
  }, [applyConversation, conversation, input])

  const handleSelectConversation = useCallback(
    async (targetConversationID: string) => {
      if (normalized(targetConversationID) === selectedConversationID) {
        return
      }
      setError('')
      setSelectedConversationID(targetConversationID)
      saveActiveConversationID(targetConversationID)
      await loadConversation(targetConversationID)
    },
    [loadConversation, selectedConversationID]
  )

  const handleConfirm = useCallback(async () => {
    if (!conversation || !turn) {
      return
    }
    setError('')
    setLoading(true)
    try {
      const next = await confirmAssistantTurn(conversation.conversation_id, turn.turn_id, candidateID || undefined)
      applyConversation(next)
    } catch (err) {
      setError(errorMessage(err, '确认失败'))
      if (shouldRefreshConversation(errorCode(err))) {
        await refreshConversation().catch(() => undefined)
      }
    } finally {
      setLoading(false)
    }
  }, [applyConversation, candidateID, conversation, refreshConversation, turn])

  const handleCommit = useCallback(async () => {
    if (!conversation || !turn) {
      return
    }
    setError('')
    setLoading(true)
    try {
      const next = await commitAssistantTurn(conversation.conversation_id, turn.turn_id)
      applyConversation(next)
    } catch (err) {
      setError(errorMessage(err, '提交失败'))
      if (shouldRefreshConversation(errorCode(err))) {
        await refreshConversation().catch(() => undefined)
      }
    } finally {
      setLoading(false)
    }
  }, [applyConversation, conversation, refreshConversation, turn])

  const handleSubmitTask = useCallback(async () => {
    if (!conversation || !turn) {
      return
    }
    const snapshot = {
      intent_schema_version: normalized(turn.intent.intent_schema_version),
      compiler_contract_version: normalized(turn.plan.compiler_contract_version),
      capability_map_version: normalized(turn.plan.capability_map_version),
      skill_manifest_digest: normalized(turn.plan.skill_manifest_digest),
      context_hash: normalized(turn.intent.context_hash),
      intent_hash: normalized(turn.intent.intent_hash),
      plan_hash: normalized(turn.dry_run.plan_hash)
    }
    if (Object.values(snapshot).some((value) => value.length === 0)) {
      setError('任务提交失败：contract_snapshot 不完整，请先重新生成计划。')
      return
    }
    setError('')
    setLoading(true)
    try {
      const receipt = await submitAssistantTask({
        conversation_id: conversation.conversation_id,
        turn_id: turn.turn_id,
        task_type: 'assistant_async_plan',
        request_id: turn.request_id,
        trace_id: turn.trace_id,
        contract_snapshot: snapshot
      })
      const detail = await getAssistantTask(receipt.task_id).catch(() => null)
      if (detail) {
        setTaskDetail(detail)
        return
      }
      setTaskDetail({
        task_id: receipt.task_id,
        task_type: receipt.task_type,
        status: receipt.status,
        dispatch_status: 'pending',
        attempt: 0,
        max_attempts: 3,
        workflow_id: receipt.workflow_id,
        request_id: turn.request_id,
        trace_id: normalized(turn.trace_id) || undefined,
        conversation_id: conversation.conversation_id,
        turn_id: turn.turn_id,
        submitted_at: receipt.submitted_at,
        updated_at: receipt.submitted_at,
        contract_snapshot: snapshot
      })
    } catch (err) {
      setError(errorMessage(err, '任务提交失败'))
    } finally {
      setLoading(false)
    }
  }, [conversation, turn])

  const handleCancelTask = useCallback(async () => {
    if (!taskDetail) {
      return
    }
    setError('')
    setLoading(true)
    try {
      const next = await cancelAssistantTask(taskDetail.task_id)
      setTaskDetail(next)
    } catch (err) {
      setError(errorMessage(err, '任务取消失败'))
    } finally {
      setLoading(false)
    }
  }, [taskDetail])

  const postBridgeMessage = useCallback(
    (type: string, payload: Record<string, unknown>) => {
      if (typeof window === 'undefined') {
        return
      }
      const target = librechatFrameRef.current?.contentWindow
      if (!target) {
        return
      }
      target.postMessage(
        {
          type,
          channel: bridgeChannel,
          nonce: bridgeNonce,
          payload
        },
        window.location.origin
      )
    },
    [bridgeChannel, bridgeNonce]
  )

  const postBridgeNotice = useCallback(
    (message: string, severity: DialogMessageKind = 'info') => {
      const text = normalized(message)
      if (text.length === 0) {
        return
      }
      postBridgeMessage('assistant.flow.notice', {
        text,
        severity
      })
    },
    [postBridgeMessage]
  )

  const postBridgeDialog = useCallback(
    (
      message: string,
      kind: DialogMessageKind = 'info',
      stage: DialogMessageStage = 'draft',
      meta?: Record<string, string>
    ) => {
      const text = normalized(message)
      if (text.length === 0) {
        return
      }
      postBridgeMessage('assistant.flow.dialog', {
        message_id: `dlg_${Date.now()}_${Math.random().toString(16).slice(2, 10)}`,
        kind,
        stage,
        text,
        meta: meta ?? {}
      })
    },
    [postBridgeMessage]
  )

  const commitTurnByDialogue = useCallback(
    async (
      sourceConversation: AssistantConversation,
      sourceTurn: AssistantTurn,
      candidateChoice?: string
    ): Promise<AssistantConversation | null> => {
      let currentConversation = sourceConversation
      let currentTurn = sourceTurn
      setDialogFlow(withDialogPhase(dialogFlowRef.current, 'committing'))
      const resolvedCandidate =
        normalized(candidateChoice) ||
        normalized(dialogFlowRef.current.selected_candidate_id) ||
        normalized(currentTurn.resolved_candidate_id) ||
        undefined

      if (normalized(currentTurn.state) === 'validated') {
        setLoading(true)
        try {
          const confirmed = await confirmAssistantTurn(currentConversation.conversation_id, currentTurn.turn_id, resolvedCandidate)
          applyConversation(confirmed)
          currentConversation = confirmed
          const refreshedTurn = latestTurn(confirmed)
          if (!refreshedTurn) {
            return confirmed
          }
          currentTurn = refreshedTurn
        } catch (err) {
          const message = errorMessage(err, '确认失败，请稍后重试。')
          setError(message)
          setDialogFlow(withDialogPhase(dialogFlowRef.current, 'failed'))
          postBridgeDialog(message, 'error', 'commit_failed')
          if (shouldRefreshConversation(errorCode(err))) {
            await refreshConversation().catch(() => undefined)
          }
          return null
        } finally {
          setLoading(false)
        }
      }

      if (normalized(currentTurn.state) !== 'confirmed') {
        return currentConversation
      }

      setLoading(true)
      try {
        const committed = await commitAssistantTurn(currentConversation.conversation_id, currentTurn.turn_id)
        applyConversation(committed)
        const committedTurn = latestTurn(committed)
        setDialogFlow(withDialogPhase(dialogFlowRef.current, 'committed'))
        postBridgeDialog(formatCommitSuccessMessage(committedTurn ?? currentTurn), 'success', 'commit_result', {
          effective_date: normalized(committedTurn?.commit_result?.effective_date)
        })
        return committed
      } catch (err) {
        const message = errorMessage(err, '提交失败，请按最新提示继续。')
        setError(message)
        setDialogFlow(withDialogPhase(dialogFlowRef.current, 'failed'))
        postBridgeDialog(message, 'error', 'commit_failed')
        if (shouldRefreshConversation(errorCode(err))) {
          await refreshConversation().catch(() => undefined)
        }
        return null
      } finally {
        setLoading(false)
      }
    },
    [applyConversation, postBridgeDialog, refreshConversation, setDialogFlow]
  )

  const tryHandlePendingTurnByDialogue = useCallback(
    async (sourceConversation: AssistantConversation, sourceTurn: AssistantTurn, userInput: string): Promise<boolean> => {
      const looksLikeCreateRequest = looksLikeCreateOrgUnitRequest(userInput)
      const analysis = analyzeTurnForDialog(sourceTurn)

      if (dialogFlowRef.current.phase === 'await_candidate_confirm') {
        if (isExecutionConfirmationText(userInput)) {
          await commitTurnByDialogue(sourceConversation, sourceTurn, dialogFlowRef.current.selected_candidate_id)
          return true
        }
        if (!looksLikeCreateRequest) {
          postBridgeDialog('已选择候选，请回复“确认执行”后继续提交。', 'info', 'candidate_confirm')
          return true
        }
        return false
      }

      if (analysis.phase === 'await_candidate_pick') {
        if (looksLikeCreateRequest) {
          return false
        }
        const candidateChoice = resolveCandidateFromInput(userInput, analysis.candidates)
        if (candidateChoice.length === 0) {
          setDialogFlow(
            withDialogPhase(dialogFlowRef.current, 'await_candidate_pick', {
              candidates: analysis.candidates
            })
          )
          postBridgeDialog(formatCandidatePrompt(analysis.candidates), 'warning', 'candidate_list')
          return true
        }
        setDialogFlow(
          withDialogPhase(dialogFlowRef.current, 'await_candidate_confirm', {
            candidates: analysis.candidates,
            selected_candidate_id: candidateChoice
          })
        )
        postBridgeDialog(
          formatCandidateConfirmMessage(analysis.candidates, candidateChoice),
          'info',
          'candidate_confirm',
          { candidate_id: candidateChoice }
        )
        return true
      }

      if (analysis.phase === 'await_commit_confirm') {
        if (isExecutionConfirmationText(userInput)) {
          await commitTurnByDialogue(sourceConversation, sourceTurn)
          return true
        }
        if (!looksLikeCreateRequest) {
          postBridgeDialog('已生成草案，请回复“确认执行”后继续提交。', 'info', 'draft')
          return true
        }
      }

      if (analysis.phase === 'await_missing_fields' && isExecutionConfirmationText(userInput)) {
        postBridgeDialog('当前信息尚未补全，暂不能提交。请先补充缺失字段。', 'warning', 'missing_fields')
        return true
      }

      return false
    },
    [commitTurnByDialogue, postBridgeDialog, setDialogFlow]
  )

  const runBridgeAutoFlow = useCallback(
    async (incomingText: string) => {
      const userInput = normalized(incomingText)
      if (userInput.length === 0) {
        return
      }
      setInput(userInput)
      setError('')

      let activeConversation = conversationRef.current
      if (!activeConversation) {
        setLoading(true)
        try {
          const created = await createAssistantConversation()
          applyConversation(created)
          activeConversation = created
        } catch (err) {
          const message = errorMessage(err, '创建会话失败')
          setError(message)
          postBridgeDialog(message, 'error', 'commit_failed')
          return
        } finally {
          setLoading(false)
        }
      }
      if (!activeConversation) {
        return
      }

      const pendingTurn = latestTurn(activeConversation)
      if (pendingTurn) {
        const handled = await tryHandlePendingTurnByDialogue(activeConversation, pendingTurn, userInput)
        if (handled) {
          return
        }
      }

      let generationInput = userInput
      const pendingAnalysis = analyzeTurnForDialog(pendingTurn)
      if (pendingTurn && pendingAnalysis.phase === 'await_missing_fields') {
        const mergedDraft = mergeIntentDraft(
          {
            parent_ref_text: pendingTurn.intent.parent_ref_text,
            entity_name: pendingTurn.intent.entity_name,
            effective_date: pendingTurn.intent.effective_date
          },
          extractIntentDraftFromText(userInput)
        )
        const composedInput = composeCreateOrgUnitPrompt(mergedDraft)
        if (composedInput.length > 0) {
          generationInput = composedInput
        }
      }

      setLoading(true)
      let generatedConversation: AssistantConversation | null = null
      try {
        generatedConversation = await createAssistantTurn(activeConversation.conversation_id, generationInput)
        applyConversation(generatedConversation)
      } catch (err) {
        const code = errorCode(err)
        const recoverable = shouldRetryStructuredPromptForError(code)
        if (recoverable && !isStructuredIntentRetryPrompt(generationInput)) {
          try {
            const retryPrompt = composeStructuredIntentRetryPrompt(generationInput)
            generatedConversation = await createAssistantTurn(activeConversation.conversation_id, retryPrompt)
            applyConversation(generatedConversation)
            postBridgeDialog('模型返回非结构化内容，已自动重试并生成可确认草案。', 'warning', 'draft')
          } catch (retryErr) {
            const retryMessage = errorMessage(retryErr, '生成计划失败')
            setError(retryMessage)
            postBridgeDialog(retryMessage, 'error', 'commit_failed')
            return
          }
        } else {
          const message = errorMessage(err, '生成计划失败')
          setError(message)
          postBridgeDialog(message, 'error', 'commit_failed')
          return
        }
      } finally {
        setLoading(false)
      }

      const generatedTurn = latestTurn(generatedConversation)
      if (!generatedTurn || !generatedConversation) {
        return
      }

      const generatedAnalysis = analyzeTurnForDialog(generatedTurn)
      setDialogFlow(
        withDialogPhase(dialogFlowRef.current, generatedAnalysis.phase, {
          candidates: generatedAnalysis.candidates,
          missing_fields: generatedAnalysis.missing_field_messages,
          pending_draft_summary: generatedAnalysis.draft_summary,
          selected_candidate_id: normalized(generatedTurn.resolved_candidate_id),
          conversation_id: generatedConversation.conversation_id,
          turn_id: generatedTurn.turn_id
        })
      )

      if (generatedAnalysis.phase === 'await_missing_fields') {
        postBridgeDialog(
          formatMissingFieldMessageText(generatedAnalysis.missing_field_messages),
          'warning',
          'missing_fields'
        )
        return
      }
      if (generatedAnalysis.phase === 'await_candidate_pick') {
        postBridgeDialog(formatCandidatePrompt(generatedAnalysis.candidates), 'warning', 'candidate_list')
        return
      }
      if (generatedAnalysis.phase === 'await_commit_confirm') {
        postBridgeDialog(generatedAnalysis.draft_summary, 'info', 'draft')
      }
    },
    [applyConversation, postBridgeDialog, setDialogFlow, tryHandlePendingTurnByDialogue]
  )

  const enqueueBridgePrompt = useCallback(
    (incomingText: string) => {
      const text = normalized(incomingText)
      if (text.length === 0) {
        return
      }
      bridgePromptQueueRef.current.push(text)
      if (bridgeWorkerRunningRef.current) {
        return
      }
      bridgeWorkerRunningRef.current = true
      void (async () => {
        try {
          while (bridgePromptQueueRef.current.length > 0) {
            const nextInput = bridgePromptQueueRef.current.shift()
            if (!nextInput) {
              continue
            }
            await runBridgeAutoFlow(nextInput)
          }
        } finally {
          bridgeWorkerRunningRef.current = false
        }
      })()
    },
    [runBridgeAutoFlow]
  )

  useEffect(() => {
    if (taskID.length === 0 || assistantTaskTerminal(taskStatus)) {
      return
    }
    let active = true
    const timer = window.setInterval(() => {
      void getAssistantTask(taskID)
        .then((next) => {
          if (!active) {
            return
          }
          setTaskDetail(next)
        })
        .catch(() => undefined)
    }, assistantTaskPollIntervalMS)

    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [taskID, taskStatus])

  useEffect(() => {
    const handleMessage = (event: MessageEvent<unknown>) => {
      const validation = validateAssistantBridgeMessage({
        allowedOrigins,
        expectedChannel: bridgeChannel,
        expectedNonce: bridgeNonce,
        event: {
          origin: event.origin,
          data: event.data
        }
      })

      if (!validation.accepted) {
        if (import.meta.env.DEV && import.meta.env.MODE !== 'test') {
          console.info('[assistant-bridge] dropped message', {
            origin: event.origin,
            reason: validation.reason
          })
        }
        return
      }

      const message = validation.message
      if (!message) {
        return
      }
      if (message.type === 'assistant.bridge.ready') {
        postBridgeNotice('自动执行通道已连接：可直接在 LibreChat 对话中输入需求。', 'info')
        return
      }
      if (message.type === 'assistant.prompt.sync') {
        const nextInput = message.payload.input
        if (typeof nextInput === 'string' && nextInput.trim().length > 0) {
          enqueueBridgePrompt(nextInput)
        }
        return
      }
      if (message.type === 'assistant.turn.refresh') {
        void refreshConversation()
      }
    }

    window.addEventListener('message', handleMessage)
    return () => {
      window.removeEventListener('message', handleMessage)
    }
  }, [allowedOrigins, bridgeChannel, bridgeNonce, enqueueBridgePrompt, postBridgeNotice, refreshConversation])

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <SmartToyIcon color='primary' />
        <Typography variant='h5'>AI 助手</Typography>
        <Box sx={{ flex: 1 }} />
        <Chip
          color='info'
          label={`provider=${turn?.plan.model_provider ?? '-'} / model=${turn?.plan.model_name ?? '-'} / rev=${turn?.plan.model_revision ?? '-'}`}
          size='small'
        />
        <Button component='a' href='/app/assistant/librechat' variant='text'>
          LibreChat 独立页
        </Button>
        <Button component='a' href='/app/assistant/models' variant='text'>
          模型配置
        </Button>
      </Stack>
      <Typography color='text.secondary' variant='body2'>
        三栏工作台：左侧会话列表，中间时间线回放，右侧当前回合操作区。支持在 LibreChat 对话中一句话触发自动执行（写入仍走后端 One Door）。
      </Typography>
      {runtimeStatus ? (
        <Alert
          data-testid='assistant-runtime-alert'
          severity={runtimeStatus.status === 'healthy' ? 'success' : runtimeStatus.status === 'degraded' ? 'warning' : 'error'}
        >
          <Typography variant='body2'>
            LibreChat Runtime：status={runtimeStatus.status} / code={runtimeStatus.error_code ?? '-'} / upstream=
            {runtimeStatus.upstream?.repo ?? '-'}@{runtimeStatus.upstream?.ref ?? '-'} ({runtimeStatus.upstream?.url ?? '-'})
          </Typography>
          <Typography variant='caption'>checked_at={formatTimestamp(runtimeStatus.checked_at)}</Typography>
          <br />
          <Typography variant='caption'>{runtimeSummary}</Typography>
        </Alert>
      ) : null}
      {runtimeError ? (
        <Alert data-testid='assistant-runtime-error-alert' severity='warning'>
          {runtimeError}
        </Alert>
      ) : null}
      {error ? (
        <Alert data-testid='assistant-error-alert' severity='error'>
          {error}
        </Alert>
      ) : null}
      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: {
            xs: '1fr',
            lg: 'minmax(260px, 1fr) minmax(320px, 1.4fr) minmax(380px, 1.2fr)'
          }
        }}
      >
        <Card>
          <CardContent>
            <Stack spacing={1.5}>
              <Stack alignItems='center' direction='row' justifyContent='space-between'>
                <Typography variant='subtitle1'>会话列表</Typography>
                <Stack direction='row' spacing={1}>
                  <Button disabled={loadingList || loading} onClick={() => void refreshConversationList()} size='small' variant='outlined'>
                    刷新
                  </Button>
                  <Button disabled={loading} onClick={() => void handleCreateConversation()} size='small' variant='contained'>
                    新建
                  </Button>
                </Stack>
              </Stack>
              {conversations.length === 0 ? <Alert severity='info'>暂无会话，点击“新建”开始多轮对话。</Alert> : null}
              <Stack spacing={1}>
                {conversations.map((item) => {
                  const active = item.conversation_id === selectedConversationID
                  return (
                    <Button
                      data-testid={active ? 'assistant-conversation-active' : undefined}
                      key={item.conversation_id}
                      onClick={() => void handleSelectConversation(item.conversation_id)}
                      size='small'
                      sx={{ justifyContent: 'flex-start', textTransform: 'none' }}
                      variant={active ? 'contained' : 'outlined'}
                    >
                      <Stack alignItems='flex-start' spacing={0.4} sx={{ width: '100%' }}>
                        <Typography sx={{ fontSize: 12 }}>{item.conversation_id}</Typography>
                        <Typography color='text.secondary' sx={{ fontSize: 11 }}>
                          state={item.state} · {formatTimestamp(item.updated_at)}
                        </Typography>
                        <Typography color='text.secondary' noWrap sx={{ fontSize: 11, width: '100%' }}>
                          {item.last_turn?.user_input ?? '（暂无回合）'}
                        </Typography>
                      </Stack>
                    </Button>
                  )
                })}
              </Stack>
              {conversationCursor.length > 0 ? (
                <Button
                  disabled={loadingList}
                  onClick={() => void refreshConversationList(conversationCursor, true)}
                  size='small'
                  variant='text'
                >
                  加载更多
                </Button>
              ) : null}
            </Stack>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <Stack spacing={1.5}>
              <Typography variant='subtitle1'>会话时间线</Typography>
              {conversation?.turns?.length ? (
                <Stack spacing={1.2}>
                  {[...conversation.turns].reverse().map((timelineTurn) => {
                    const taskState = taskDetail?.turn_id === timelineTurn.turn_id ? taskDetail.status : ''
                    return (
                      <Card key={timelineTurn.turn_id} variant='outlined'>
                        <CardContent>
                          <Stack spacing={0.6}>
                            <Stack alignItems='center' direction='row' justifyContent='space-between'>
                              <Typography variant='caption'>turn={timelineTurn.turn_id}</Typography>
                              <Stack direction='row' spacing={0.5}>
                                <Chip label={timelineTurn.state} size='small' />
                                <Chip label={`risk=${timelineTurn.risk_tier}`} size='small' />
                                {taskState ? <Chip label={`task=${taskState}`} size='small' /> : null}
                              </Stack>
                            </Stack>
                            <Typography variant='body2'>{timelineTurn.user_input}</Typography>
                            <Typography color='text.secondary' variant='caption'>
                              request_id={timelineTurn.request_id} · trace_id={timelineTurn.trace_id}
                            </Typography>
                            <Typography color='text.secondary' variant='caption'>
                              provider={timelineTurn.plan.model_provider ?? '-'} / model={timelineTurn.plan.model_name ?? '-'}
                            </Typography>
                            <Typography color='text.secondary' variant='caption'>dry-run: {timelineTurn.dry_run.explain}</Typography>
                            {timelineTurn.commit_result ? (
                              <Alert severity='success'>
                                已提交：org_code={timelineTurn.commit_result.org_code} / parent={timelineTurn.commit_result.parent_org_code}
                              </Alert>
                            ) : null}
                          </Stack>
                        </CardContent>
                      </Card>
                    )
                  })}
                </Stack>
              ) : (
                <Alert severity='info'>当前会话暂无回合，请在右侧输入并生成。</Alert>
              )}
            </Stack>
          </CardContent>
        </Card>

        <Card data-testid='assistant-transaction-panel'>
          <CardContent>
            <Stack spacing={2}>
              <Typography variant='subtitle1'>当前回合操作</Typography>
              <TextField
                data-testid='assistant-input'
                label='输入需求'
                minRows={4}
                multiline
                onChange={(event) => setInput(event.target.value)}
                value={input}
              />
              <Button
                data-testid='assistant-generate-button'
                disabled={!actionState.canRegenerate}
                onClick={() => void handleGenerate()}
                variant='contained'
              >
                Regenerate
              </Button>
              <Divider />
              <Typography data-testid='assistant-conversation-id' variant='body2'>
                conversation_id: {conversation?.conversation_id ?? '-'}
              </Typography>
              <Typography data-testid='assistant-turn-id' variant='body2'>
                turn_id: {turn?.turn_id ?? '-'}
              </Typography>
              <Typography data-testid='assistant-request-id' variant='body2'>
                request_id: {turn?.request_id ?? '-'}
              </Typography>
              <Typography data-testid='assistant-trace-id' variant='body2'>
                trace_id: {turn?.trace_id ?? '-'}
              </Typography>
              <Typography variant='caption'>intent_schema_version: {turn?.intent.intent_schema_version ?? '-'}</Typography>
              <Typography variant='caption'>compiler_contract_version: {turn?.plan.compiler_contract_version ?? '-'}</Typography>
              <Typography variant='caption'>capability_map_version: {turn?.plan.capability_map_version ?? '-'}</Typography>
              <Typography variant='caption'>skill_manifest_digest: {turn?.plan.skill_manifest_digest ?? '-'}</Typography>
              <Typography variant='caption'>context_hash: {turn?.intent.context_hash ?? '-'}</Typography>
              <Typography variant='caption'>intent_hash: {turn?.intent.intent_hash ?? '-'}</Typography>
              <Typography variant='caption'>plan_hash: {turn?.dry_run.plan_hash ?? '-'}</Typography>
              <Stack alignItems='center' direction='row' spacing={1}>
                <Typography variant='body2'>状态：</Typography>
                <Chip data-testid='assistant-turn-state' label={turn?.state ?? assistantStatePlaceholder} size='small' />
                <Chip
                  color={turn?.risk_tier === 'high' ? 'warning' : 'default'}
                  data-testid='assistant-risk-tier'
                  label={`risk=${turn?.risk_tier ?? '-'}`}
                  size='small'
                />
              </Stack>
              {actionState.showRiskBlocker ? (
                <Alert data-testid='assistant-risk-blocker' severity='warning'>
                  高风险计划需要先 Confirm，再允许 Commit。
                </Alert>
              ) : null}
              {actionState.showCandidateBlocker ? (
                <Alert data-testid='assistant-candidate-blocker' severity='warning'>
                  存在多个同名候选组织，请先选择候选并 Confirm。
                </Alert>
              ) : null}
              {actionState.showRequiredFieldBlocker ? (
                <Alert data-testid='assistant-required-field-blocker' severity='warning'>
                  当前信息不完整，请先在对话中补全必填信息，再执行 Confirm / Commit。
                </Alert>
              ) : null}
              {turn?.plan ? (
                <Alert data-testid='assistant-plan' severity='info'>
                  <strong>{turn.plan.title}</strong>
                  <br />
                  {turn.plan.summary}
                  <br />
                  <Typography component='span' data-testid='assistant-plan-capability' variant='caption'>
                    capability_key={turn.plan.capability_key}
                  </Typography>
                  <br />
                  <Typography component='span' variant='caption'>
                    provider={turn.plan.model_provider ?? '-'} / model={turn.plan.model_name ?? '-'}
                  </Typography>
                </Alert>
              ) : null}
              {turn?.dry_run ? (
                <Stack data-testid='assistant-dryrun' spacing={1}>
                  <Typography variant='subtitle2'>Dry Run</Typography>
                  <Typography data-testid='assistant-dryrun-explain' variant='body2'>
                    {turn.dry_run.explain}
                  </Typography>
                  {turnDryRunDiff.length > 0 ? (
                    <Stack component='ul' data-testid='assistant-dryrun-diff' spacing={0.5} sx={{ m: 0, pl: 2 }}>
                      {turnDryRunDiff.map((item, index) => (
                        <Typography component='li' key={`${item.field ?? 'field'}-${index}`} variant='caption'>
                          {String(item.field ?? '-')} {'->'} {stringifyDiffValue(item.after)}
                        </Typography>
                      ))}
                    </Stack>
                  ) : null}
                  {turnDryRunValidationMessages.length ? (
                    <Alert severity='warning'>
                      {turnDryRunValidationMessages.map((message, index) => (
                        <Typography component='div' key={`${message}-${index}`} variant='body2'>
                          {message}
                        </Typography>
                      ))}
                    </Alert>
                  ) : null}
                </Stack>
              ) : null}
              {turn?.candidates?.length ? (
                <Stack data-testid='assistant-candidates' spacing={1}>
                  <Typography variant='subtitle2'>父组织候选</Typography>
                  <RadioGroup onChange={(_, value) => setCandidateID(value)} value={candidateID}>
                    {turn.candidates.map((candidate) => (
                      <FormControlLabel
                        control={<Radio />}
                        key={candidate.candidate_id}
                        label={`${candidate.name} / ${candidate.candidate_code} / ${candidate.path} / ${candidate.as_of}`}
                        value={candidate.candidate_id}
                      />
                    ))}
                  </RadioGroup>
                </Stack>
              ) : null}
              <Stack direction='row' spacing={1}>
                <Button
                  data-testid='assistant-confirm-button'
                  disabled={!actionState.canConfirm}
                  onClick={() => void handleConfirm()}
                  variant='outlined'
                >
                  Confirm
                </Button>
                <Button
                  data-testid='assistant-commit-button'
                  disabled={!actionState.canCommit}
                  onClick={() => void handleCommit()}
                  variant='contained'
                >
                  Commit
                </Button>
              </Stack>
              {turn?.commit_result ? (
                <Alert data-testid='assistant-commit-result' severity='success'>
                  已提交：org_code={turn.commit_result.org_code} / parent={turn.commit_result.parent_org_code} /
                  effective_date={turn.commit_result.effective_date}
                </Alert>
              ) : null}
              <Divider />
              <Typography variant='subtitle2'>异步任务（225）</Typography>
              <Stack direction='row' spacing={1}>
                <Button
                  data-testid='assistant-task-submit-button'
                  disabled={!turn || loading}
                  onClick={() => void handleSubmitTask()}
                  variant='contained'
                >
                  Submit Task
                </Button>
                <Button
                  data-testid='assistant-task-cancel-button'
                  disabled={!taskDetail || taskTerminal || loading}
                  onClick={() => void handleCancelTask()}
                  variant='outlined'
                >
                  Cancel Task
                </Button>
              </Stack>
              <Typography data-testid='assistant-task-id' variant='body2'>
                task_id: {taskDetail?.task_id ?? '-'}
              </Typography>
              <Typography data-testid='assistant-task-workflow-id' variant='body2'>
                workflow_id: {taskDetail?.workflow_id ?? '-'}
              </Typography>
              <Stack alignItems='center' direction='row' spacing={1}>
                <Typography variant='body2'>task_status：</Typography>
                <Chip data-testid='assistant-task-status' label={taskDetail?.status ?? assistantStatePlaceholder} size='small' />
                <Chip data-testid='assistant-task-dispatch-status' label={`dispatch=${taskDetail?.dispatch_status ?? '-'}`} size='small' />
              </Stack>
              <Typography data-testid='assistant-task-attempt' variant='body2'>
                attempt: {taskDetail?.attempt ?? 0} / {taskDetail?.max_attempts ?? 0}
              </Typography>
              <Typography data-testid='assistant-task-error-code' variant='body2'>
                last_error_code: {taskDetail?.last_error_code ?? '-'}
              </Typography>
              <Typography data-testid='assistant-task-request-id' variant='body2'>
                request_id: {taskDetail?.request_id ?? '-'}
              </Typography>
              <Typography data-testid='assistant-task-trace-id' variant='body2'>
                trace_id: {taskDetail?.trace_id ?? '-'}
              </Typography>
            </Stack>
          </CardContent>
        </Card>
      </Box>

      <Card>
        <CardContent>
          <Typography gutterBottom variant='subtitle1'>
            聊天壳层（LibreChat）
          </Typography>
          <Box
            component='iframe'
            data-testid='assistant-librechat-frame'
            ref={librechatFrameRef}
            src={iframeSrc}
            title='LibreChat'
            sx={{ width: '100%', height: 380, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}
          />
        </CardContent>
      </Card>
    </Stack>
  )
}
