import ChatIcon from '@mui/icons-material/Chat'
import { Box, Button, Card, CardContent, Stack, Typography } from '@mui/material'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  commitAssistantTurn,
  confirmAssistantTurn,
  createAssistantConversation,
  createAssistantTurn,
  getAssistantConversation,
  renderAssistantTurnReply,
  type AssistantConversation,
  type AssistantTurn
} from '../../api/assistant'
import {
  createAssistantBridgeToken,
  parseAssistantAllowedOrigins,
  validateAssistantBridgeMessage
} from './assistantMessageBridge'
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
import { buildAssistantTurnCreationFailureReplyPayload } from './assistantReplyFailurePayload'
import {
  composeStructuredIntentRetryPrompt,
  composeCreateOrgUnitPrompt,
  extractIntentDraftFromText,
  formatCandidatePrompt,
  hasCompleteCreateIntent,
  isStructuredIntentRetryPrompt,
  isExecutionConfirmationText,
  looksLikeCreateOrgUnitRequest,
  mergeIntentDraft,
  resolveCandidateFromInput,
  shouldRetryStructuredPromptForError
} from './assistantAutoRun'

function normalized(value: string | undefined): string {
  return (value ?? '').trim()
}

function buildAssistantDialogMessageID(conversationID: string, turnID: string, stage: string): string {
  const stableConversationID = normalized(conversationID).replace(/[^a-zA-Z0-9_-]+/g, '_')
  const stableTurnID = normalized(turnID).replace(/[^a-zA-Z0-9_-]+/g, '_')
  if (stableConversationID.length > 0 && stableTurnID.length > 0) {
    return `dlg_${stableConversationID}_${stableTurnID}`
  }
  const parts = [stableConversationID, stableTurnID, normalized(stage).replace(/[^a-zA-Z0-9_-]+/g, '_')]
    .filter((value) => value.length > 0)
  if (parts.length === 0) {
    return `dlg_${Date.now()}_${Math.random().toString(16).slice(2, 10)}`
  }
  return `dlg_${parts.join('_')}`
}

function latestTurn(conversation: AssistantConversation | null): AssistantTurn | null {
  if (!conversation || !Array.isArray(conversation.turns) || conversation.turns.length === 0) {
    return null
  }
  return conversation.turns[conversation.turns.length - 1] ?? null
}

function errorMessage(err: unknown, fallback: string): string {
  const message = (err as { message?: string })?.message
  if (typeof message === 'string' && message.trim().length > 0) {
    return message
  }
  return fallback
}

function pushBridgeAudit(event: Record<string, unknown>) {
  if (typeof window === 'undefined') {
    return
  }
  const target = window as typeof window & { __assistantBridgeAudit?: Array<Record<string, unknown>> }
  target.__assistantBridgeAudit = Array.isArray(target.__assistantBridgeAudit) ? target.__assistantBridgeAudit : []
  target.__assistantBridgeAudit.push({ ts: Date.now(), ...event })
}

function errorCode(err: unknown): string {
  const details = (err as { details?: { code?: string } })?.details
  return normalized(details?.code)
}

const assistantMissingTurnContextID = 'missing-turn-context'

function shouldRefreshConversation(code: string): boolean {
  return code === 'conversation_state_invalid' || code === 'conversation_confirmation_required'
}

export function LibreChatPage() {
  const iframeRef = useRef<HTMLIFrameElement | null>(null)
  const conversationRef = useRef<AssistantConversation | null>(null)
  const dialogFlowRef = useRef<DialogFlowState>(createDialogFlowState())
  const bridgePromptQueueRef = useRef<string[]>([])
  const bridgeWorkerRunningRef = useRef(false)
  const [bridgeChannel] = useState(() => createAssistantBridgeToken('assistant_channel'))
  const [bridgeNonce] = useState(() => createAssistantBridgeToken('assistant_nonce'))
  const [bridgeError, setBridgeError] = useState('')
  const [bridgeStatus, setBridgeStatus] = useState('等待对话通道连接…')
  const allowedOrigins = useMemo(() => {
    const origin = typeof window !== 'undefined' ? window.location.origin : undefined
    return parseAssistantAllowedOrigins(import.meta.env.VITE_ASSISTANT_ALLOWED_ORIGINS, origin)
  }, [])
  const iframeSrc = useMemo(
    () => `/assistant-ui/?channel=${encodeURIComponent(bridgeChannel)}&nonce=${encodeURIComponent(bridgeNonce)}`,
    [bridgeChannel, bridgeNonce]
  )
  const bridgeReadyRef = useRef(false)
  const pendingBridgeDeliveryTimersRef = useRef<Map<string, number>>(new Map())
  const pendingBridgeDeliveriesRef = useRef<Map<string, { type: string; payload: Record<string, unknown> }>>(new Map())

  const cancelPendingBridgeDelivery = useCallback((deliveryKey: string) => {
    if (typeof window === 'undefined') {
      return
    }
    const timer = pendingBridgeDeliveryTimersRef.current.get(deliveryKey)
    if (typeof timer === 'number' && timer >= 0) {
      window.clearTimeout(timer)
    }
    pendingBridgeDeliveryTimersRef.current.delete(deliveryKey)
    pendingBridgeDeliveriesRef.current.delete(deliveryKey)
  }, [])

  const setDialogFlow = useCallback((next: DialogFlowState) => {
    dialogFlowRef.current = next
  }, [])

  const syncDialogFlowByTurn = useCallback(
    (conversationID: string, turn: AssistantTurn | null) => {
      const turnID = normalized(turn?.turn_id)
      const base = resetDialogFlowForConversation(conversationID, turnID)
      const analysis = analyzeTurnForDialog(turn)
      setDialogFlow(
        withDialogPhase(base, analysis.phase, {
          pending_draft_summary: analysis.draft_summary,
          missing_fields: analysis.missing_field_messages,
          candidates: analysis.candidates,
          selected_candidate_id: normalized(turn?.resolved_candidate_id)
        })
      )
    },
    [setDialogFlow]
  )

  const applyConversation = useCallback(
    (next: AssistantConversation) => {
      conversationRef.current = next
      syncDialogFlowByTurn(next.conversation_id, latestTurn(next))
    },
    [syncDialogFlowByTurn]
  )

  const refreshConversation = useCallback(async (): Promise<AssistantConversation | null> => {
    const current = conversationRef.current
    const conversationID = normalized(current?.conversation_id)
    if (conversationID.length === 0) {
      return null
    }
    const refreshed = await getAssistantConversation(conversationID)
    applyConversation(refreshed)
    return refreshed
  }, [applyConversation])

  const deliverBridgeMessage = useCallback(
    (type: string, payload: Record<string, unknown>, deliveryKey?: string, attempt?: number) => {
      if (typeof window === 'undefined') {
        return
      }
      const target = iframeRef.current?.contentWindow
      if (!target) {
        pushBridgeAudit({ kind: 'post_skipped', type, reason: 'missing_target', deliveryKey, attempt })
        return
      }
      pushBridgeAudit({ kind: 'post_sent', type, payload, deliveryKey, attempt, bridgeReady: bridgeReadyRef.current })
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

  const postBridgeMessage = useCallback(
    (
      type: string,
      payload: Record<string, unknown>,
      options?: {
        deliveryKey?: string
        retryDelays?: number[]
      }
    ) => {
      const deliveryKey = normalized(options?.deliveryKey)
      if (deliveryKey.length === 0 || typeof window === 'undefined') {
        deliverBridgeMessage(type, payload)
        return
      }
      cancelPendingBridgeDelivery(deliveryKey)
      pendingBridgeDeliveriesRef.current.set(deliveryKey, { type, payload })
      const retryDelays = options?.retryDelays ?? [300, 1000, 2500, 5000, 10000, 20000, 40000]
      const sendAttempt = (attempt: number) => {
        deliverBridgeMessage(type, payload, deliveryKey, attempt)
        if (attempt >= retryDelays.length) {
          pendingBridgeDeliveryTimersRef.current.delete(deliveryKey)
          pendingBridgeDeliveriesRef.current.delete(deliveryKey)
          return
        }
        const timer = window.setTimeout(() => {
          if (!pendingBridgeDeliveryTimersRef.current.has(deliveryKey)) {
            return
          }
          sendAttempt(attempt + 1)
        }, retryDelays[attempt])
        pendingBridgeDeliveryTimersRef.current.set(deliveryKey, timer)
      }
      pendingBridgeDeliveryTimersRef.current.set(deliveryKey, -1)
      sendAttempt(0)
    },
    [cancelPendingBridgeDelivery, deliverBridgeMessage]
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
      meta?: Record<string, string>,
      options?: {
        conversationID?: string
        turnID?: string
        outcome?: 'success' | 'failure'
        errorCode?: string
        errorMessage?: string
        nextAction?: string
        fallbackText?: string
        allowMissingTurn?: boolean
      }
    ) => {
      const sendBridgeDialog = (text: string, finalKind: DialogMessageKind, finalStage: DialogMessageStage, finalMeta?: Record<string, string>) => {
        const activeConversationID = normalized(options?.conversationID) || normalized(conversationRef.current?.conversation_id)
        const activeTurn = latestTurn(conversationRef.current)
        const activeTurnID = normalized(options?.turnID) || normalized(activeTurn?.turn_id)
        const messageID = buildAssistantDialogMessageID(activeConversationID, activeTurnID, finalStage)
        postBridgeMessage(
          'assistant.flow.dialog',
          {
            message_id: messageID,
            kind: finalKind,
            stage: finalStage,
            text,
            meta: {
              ...(finalMeta ?? {}),
              ...(activeConversationID.length > 0 ? { conversation_id: activeConversationID } : {}),
              ...(activeTurnID.length > 0 ? { turn_id: activeTurnID } : {}),
              ...(normalized(activeTurn?.request_id).length > 0 ? { request_id: normalized(activeTurn?.request_id) } : {}),
              ...(normalized(activeTurn?.trace_id).length > 0 ? { trace_id: normalized(activeTurn?.trace_id) } : {})
            }
          },
          { deliveryKey: `assistant.flow.dialog:${messageID}` }
        )
      }
      const text = normalized(message)
      if (text.length === 0) {
        return
      }
      const conversationID = normalized(options?.conversationID) || normalized(conversationRef.current?.conversation_id)
      const resolvedTurnID = normalized(options?.turnID) || normalized(latestTurn(conversationRef.current)?.turn_id)
      const allowMissingTurn = Boolean(options?.allowMissingTurn)
      pushBridgeAudit({ kind: 'dialog_requested', stage, conversationID, resolvedTurnID, allowMissingTurn })
      if (conversationID.length === 0 || (resolvedTurnID.length === 0 && !allowMissingTurn)) {
        const notice =
          conversationID.length === 0
            ? '回复生成链路不可用：缺少会话上下文，请先发起业务请求。'
            : '回复生成链路不可用：缺少轮次上下文，请重试。'
        setBridgeError(notice)
        pushBridgeAudit({ kind: 'dialog_blocked', stage, reason: notice, conversationID, resolvedTurnID, allowMissingTurn })
        postBridgeNotice(notice, 'warning')
        return
      }
      void (async () => {
        let fallbackPosted = false
        let fallbackTimer: number | null = null
        const emitLocalFallback = () => {
          if (fallbackPosted) {
            return
          }
          fallbackPosted = true
          pushBridgeAudit({ kind: 'dialog_fallback_emitted', stage, conversationID, resolvedTurnID })
          sendBridgeDialog(text, kind, stage, {
            ...(meta ?? {}),
            reply_source: 'local_fallback',
            used_fallback: 'true'
          })
        }
        try {
          if (typeof window !== 'undefined') {
            fallbackTimer = window.setTimeout(() => {
              emitLocalFallback()
            }, 8000)
          }
          const replyTurnID = resolvedTurnID || assistantMissingTurnContextID
          const replyPayload = {
            stage,
            kind,
            outcome: options?.outcome ?? (kind === 'error' ? 'failure' : 'success'),
            error_code: normalized(options?.errorCode),
            error_message: normalized(options?.errorMessage),
            next_action: normalized(options?.nextAction),
            locale: 'zh'
          } as const
          const fallbackText = normalized(options?.fallbackText)
          const rendered = await renderAssistantTurnReply(conversationID, replyTurnID, {
            ...replyPayload,
            ...(fallbackText.length > 0 ? { fallback_text: fallbackText } : {}),
            ...(allowMissingTurn && resolvedTurnID.length === 0 ? { allow_missing_turn: true } : {})
          })
          if (fallbackTimer !== null && typeof window !== 'undefined') {
            window.clearTimeout(fallbackTimer)
          }
          pushBridgeAudit({
            kind: 'dialog_rendered',
            stage,
            conversationID,
            replyTurnID,
            renderedStage: normalized(rendered.stage),
            renderedKind: normalized(rendered.kind),
            renderedText: normalized(rendered.text)
          })
          sendBridgeDialog(
            normalized(rendered.text) || text,
            (normalized(rendered.kind) as DialogMessageKind) || kind,
            (normalized(rendered.stage) as DialogMessageStage) || stage,
            {
              ...(meta ?? {}),
              reply_model_name: normalized(rendered.reply_model_name),
              reply_prompt_version: normalized(rendered.reply_prompt_version),
              reply_source: normalized(rendered.reply_source),
              used_fallback: rendered.used_fallback ? 'true' : 'false'
            }
          )
        } catch (error) {
          if (fallbackTimer !== null && typeof window !== 'undefined') {
            window.clearTimeout(fallbackTimer)
          }
          emitLocalFallback()
          const notice = '回复生成失败，请稍后重试。'
          setBridgeError(notice)
          pushBridgeAudit({ kind: 'dialog_failed', stage, conversationID, resolvedTurnID, error: errorMessage(error, notice) })
        }
      })()
    },
    [postBridgeMessage, postBridgeNotice]
  )

  const postBridgeTurnCreationFailure = useCallback(
    (conversationID: string, userInput: string, rawError: unknown) => {
      const code = errorCode(rawError)
      const message = errorMessage(rawError, '生成计划失败')
      const payload = buildAssistantTurnCreationFailureReplyPayload(userInput, code, message)
      setBridgeError(message)
      postBridgeDialog(
        normalized(payload.fallback_text) || message,
        (normalized(payload.kind) as DialogMessageKind) || 'error',
        (normalized(payload.stage) as DialogMessageStage) || 'commit_failed',
        undefined,
        {
          conversationID,
          outcome: payload.outcome,
          errorCode: payload.error_code,
          errorMessage: payload.error_message,
          fallbackText: payload.fallback_text,
          allowMissingTurn: payload.allow_missing_turn
        }
      )
    },
    [postBridgeDialog]
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
          setBridgeError(message)
          setDialogFlow(withDialogPhase(dialogFlowRef.current, 'failed'))
          postBridgeDialog(message, 'error', 'commit_failed', undefined, {
            errorCode: errorCode(err),
            errorMessage: message,
            outcome: 'failure'
          })
          if (shouldRefreshConversation(errorCode(err))) {
            await refreshConversation().catch(() => undefined)
          }
          return null
        }
      }

      if (normalized(currentTurn.state) !== 'confirmed') {
        return currentConversation
      }

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
        setBridgeError(message)
        setDialogFlow(withDialogPhase(dialogFlowRef.current, 'failed'))
        postBridgeDialog(message, 'error', 'commit_failed', undefined, {
          errorCode: errorCode(err),
          errorMessage: message,
          outcome: 'failure'
        })
        if (shouldRefreshConversation(errorCode(err))) {
          await refreshConversation().catch(() => undefined)
        }
        return null
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
      pushBridgeAudit({ kind: 'auto_flow_start', userInput })
      if (userInput.length === 0) {
        return
      }
      setBridgeError('')

      let activeConversation = conversationRef.current
      if (!activeConversation) {
        try {
          pushBridgeAudit({ kind: 'conversation_create_requested' })
          const created = await createAssistantConversation()
          pushBridgeAudit({ kind: 'conversation_created', conversationID: created.conversation_id })
          applyConversation(created)
          activeConversation = created
        } catch (err) {
          const message = errorMessage(err, '创建会话失败')
          pushBridgeAudit({ kind: 'conversation_create_failed', error: message })
          setBridgeError(message)
          postBridgeNotice(message, 'error')
          return
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

      const directDraft = extractIntentDraftFromText(userInput)
      let generationInput = composeCreateOrgUnitPrompt(directDraft) || userInput
      if (hasCompleteCreateIntent(directDraft)) {
        generationInput = composeStructuredIntentRetryPrompt(generationInput)
      }
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
          generationInput = hasCompleteCreateIntent(mergedDraft)
            ? composeStructuredIntentRetryPrompt(composedInput)
            : composedInput
        }
      }

      let generatedConversation: AssistantConversation | null = null
      try {
        pushBridgeAudit({ kind: 'turn_create_requested', conversationID: activeConversation.conversation_id, generationInput })
        generatedConversation = await createAssistantTurn(activeConversation.conversation_id, generationInput)
        applyConversation(generatedConversation)
        pushBridgeAudit({
          kind: 'turn_created',
          conversationID: generatedConversation.conversation_id,
          turnID: normalized(latestTurn(generatedConversation)?.turn_id),
          requestID: normalized(latestTurn(generatedConversation)?.request_id),
          traceID: normalized(latestTurn(generatedConversation)?.trace_id)
        })
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
            postBridgeTurnCreationFailure(activeConversation.conversation_id, userInput, retryErr)
            return
          }
        } else {
          postBridgeTurnCreationFailure(activeConversation.conversation_id, userInput, err)
          return
        }
      }

      const generatedTurn = latestTurn(generatedConversation)
      if (!generatedTurn || !generatedConversation) {
        return
      }

      const generatedAnalysis = analyzeTurnForDialog(generatedTurn)
      pushBridgeAudit({
        kind: 'turn_analyzed',
        phase: generatedAnalysis.phase,
        conversationID: generatedConversation.conversation_id,
        turnID: generatedTurn.turn_id,
        requestID: generatedTurn.request_id,
        traceID: generatedTurn.trace_id,
        missingFields: generatedAnalysis.missing_field_messages,
        candidateCount: generatedAnalysis.candidates.length,
        draftSummary: generatedAnalysis.draft_summary
      })
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
    [applyConversation, postBridgeDialog, postBridgeTurnCreationFailure, setDialogFlow, tryHandlePendingTurnByDialogue]
  )

  const enqueueBridgePrompt = useCallback(
    (incomingText: string) => {
      const text = normalized(incomingText)
      if (text.length === 0) {
        return
      }
      bridgePromptQueueRef.current.push(text)
      pushBridgeAudit({ kind: 'prompt_enqueued', text, queueSize: bridgePromptQueueRef.current.length })
      if (bridgeWorkerRunningRef.current) {
        return
      }
      bridgeWorkerRunningRef.current = true
      pushBridgeAudit({ kind: 'worker_started', queueSize: bridgePromptQueueRef.current.length })
      void (async () => {
        try {
          while (bridgePromptQueueRef.current.length > 0) {
            const nextInput = bridgePromptQueueRef.current.shift()
            if (!nextInput) {
              continue
            }
            pushBridgeAudit({ kind: 'worker_dequeued', text: nextInput, queueSize: bridgePromptQueueRef.current.length })
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
    const handleMessage = (event: MessageEvent<unknown>) => {
      const rawType =
        typeof event.data === 'object' && event.data !== null && 'type' in event.data
          ? String((event.data as { type?: unknown }).type ?? '')
          : ''
      pushBridgeAudit({ kind: 'message_seen', origin: event.origin, rawType })
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
        pushBridgeAudit({ kind: 'message_rejected', origin: event.origin, rawType, reason: validation.reason ?? 'unknown' })
        return
      }
      const message = validation.message
      if (!message) {
        pushBridgeAudit({ kind: 'message_rejected', origin: event.origin, rawType, reason: 'missing_message' })
        return
      }
      pushBridgeAudit({ kind: 'message_accepted', type: message.type, origin: event.origin })
      if (message.type === 'assistant.bridge.ready') {
        bridgeReadyRef.current = true
        setBridgeStatus('connected')
        pendingBridgeDeliveriesRef.current.forEach((delivery, deliveryKey) => {
          deliverBridgeMessage(delivery.type, delivery.payload, deliveryKey, -1)
        })
        pushBridgeAudit({ kind: 'bridge_ready', pendingDeliveryCount: pendingBridgeDeliveriesRef.current.size })
        return
      }
      if (message.type === 'assistant.bridge.delivery_ack') {
        const deliveryKey = normalized(String(message.payload?.delivery_key ?? ''))
        if (deliveryKey.length > 0) {
          cancelPendingBridgeDelivery(deliveryKey)
        }
        pushBridgeAudit({
          kind: 'delivery_ack',
          deliveryKey,
          messageID: normalized(String(message.payload?.message_id ?? '')),
          acceptedType: normalized(String(message.payload?.accepted_type ?? ''))
        })
        return
      }
      if (message.type === 'assistant.prompt.sync') {
        const nextInput = message.payload.input
        if (typeof nextInput === 'string' && nextInput.trim().length > 0) {
          enqueueBridgePrompt(nextInput)
        }
      }
    }

    window.addEventListener('message', handleMessage)
    return () => {
      window.removeEventListener('message', handleMessage)
    }
  }, [allowedOrigins, bridgeChannel, bridgeNonce, cancelPendingBridgeDelivery, deliverBridgeMessage, enqueueBridgePrompt])

  useEffect(() => {
    return () => {
      if (typeof window === 'undefined') {
        return
      }
      pendingBridgeDeliveryTimersRef.current.forEach((timer) => {
        if (typeof timer === 'number' && timer >= 0) {
          window.clearTimeout(timer)
        }
      })
      pendingBridgeDeliveryTimersRef.current.clear()
      pendingBridgeDeliveriesRef.current.clear()
    }
  }, [])

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <ChatIcon color='primary' />
        <Typography variant='h5'>LibreChat</Typography>
        <Box sx={{ flex: 1 }} />
        <Button component='a' href='/app/assistant' variant='text'>
          返回助手工作台
        </Button>
      </Stack>
      <Typography color='text.secondary' variant='body2'>
        独立聊天页：唯一交互入口；所有业务回复必须回写到官方聊天流内。
      </Typography>
      <Box data-testid='librechat-bridge-status' sx={{ display: 'none' }}>
        {bridgeStatus}
      </Box>
      <Box data-testid='librechat-bridge-error' sx={{ display: 'none' }}>
        {bridgeError}
      </Box>
      <Card>
        <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
          <Box
            component='iframe'
            data-testid='librechat-standalone-frame'
            ref={iframeRef}
            src={iframeSrc}
            title='LibreChat Standalone'
            sx={{ width: '100%', minHeight: 'calc(100vh - 240px)', border: 0, borderRadius: 1 }}
          />
        </CardContent>
      </Card>
    </Stack>
  )
}
