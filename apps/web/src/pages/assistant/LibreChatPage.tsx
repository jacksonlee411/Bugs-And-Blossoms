import ChatIcon from '@mui/icons-material/Chat'
import { Alert, Box, Button, Card, CardContent, Stack, Typography } from '@mui/material'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  commitAssistantTurn,
  confirmAssistantTurn,
  createAssistantConversation,
  createAssistantTurn,
  getAssistantConversation,
  type AssistantConversation,
  type AssistantTurn
} from '../../api/assistant'
import {
  createAssistantBridgeToken,
  parseAssistantAllowedOrigins,
  validateAssistantBridgeMessage
} from './assistantMessageBridge'
import {
  composeCreateOrgUnitPrompt,
  extractIntentDraftFromText,
  formatCandidatePrompt,
  isExecutionConfirmationText,
  looksLikeCreateOrgUnitRequest,
  mergeIntentDraft,
  resolveCandidateFromInput
} from './assistantAutoRun'

type BridgeNoticeSeverity = 'info' | 'success' | 'warning' | 'error'

function normalized(value: string | undefined): string {
  return (value ?? '').trim()
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

function errorCode(err: unknown): string {
  const details = (err as { details?: { code?: string } })?.details
  return normalized(details?.code)
}

function shouldRefreshConversation(code: string): boolean {
  return code === 'conversation_state_invalid' || code === 'conversation_confirmation_required'
}

function formatDryRunValidationError(code: string): string {
  const normalizedCode = normalized(code)
  switch (normalizedCode) {
    case 'missing_parent_ref_text':
      return '请补充上级组织名称（例如：AI治理办公室）'
    case 'missing_entity_name':
      return '请补充部门名称（例如：人力资源部2）'
    case 'missing_effective_date':
      return '请补充生效日期（YYYY-MM-DD）'
    case 'invalid_effective_date_format':
      return '生效日期格式不正确，请使用 YYYY-MM-DD'
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

export function LibreChatPage() {
  const iframeRef = useRef<HTMLIFrameElement | null>(null)
  const conversationRef = useRef<AssistantConversation | null>(null)
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

  const applyConversation = useCallback((next: AssistantConversation) => {
    conversationRef.current = next
  }, [])

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

  const postBridgeNotice = useCallback(
    (message: string, severity: BridgeNoticeSeverity = 'info') => {
      const text = normalized(message)
      if (text.length === 0 || typeof window === 'undefined') {
        return
      }
      const target = iframeRef.current?.contentWindow
      if (!target) {
        return
      }
      target.postMessage(
        {
          type: 'assistant.flow.notice',
          channel: bridgeChannel,
          nonce: bridgeNonce,
          payload: {
            text,
            severity
          }
        },
        window.location.origin
      )
    },
    [bridgeChannel, bridgeNonce]
  )

  const autoCommitTurnFromChat = useCallback(
    async (
      sourceConversation: AssistantConversation,
      sourceTurn: AssistantTurn,
      candidateChoice?: string
    ): Promise<AssistantConversation | null> => {
      let currentConversation: AssistantConversation = sourceConversation
      let currentTurn: AssistantTurn = sourceTurn
      const resolvedCandidate = normalized(candidateChoice) || normalized(currentTurn.resolved_candidate_id) || undefined

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
          const message = errorMessage(err, '确认失败')
          setBridgeError(message)
          postBridgeNotice(message, 'error')
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
        currentConversation = committed
        const committedTurn = latestTurn(committed)
        if (committedTurn?.commit_result) {
          postBridgeNotice(
            `已自动提交：org_code=${committedTurn.commit_result.org_code} / parent=${committedTurn.commit_result.parent_org_code} / effective_date=${committedTurn.commit_result.effective_date}`,
            'success'
          )
        } else {
          postBridgeNotice('已自动提交。', 'success')
        }
        return committed
      } catch (err) {
        const message = errorMessage(err, '提交失败')
        setBridgeError(message)
        postBridgeNotice(message, 'error')
        if (shouldRefreshConversation(errorCode(err))) {
          await refreshConversation().catch(() => undefined)
        }
        return null
      }
    },
    [applyConversation, postBridgeNotice, refreshConversation]
  )

  const autoAdvanceGeneratedTurn = useCallback(
    async (sourceConversation: AssistantConversation, sourceTurn: AssistantTurn, userInput: string) => {
      const validationCodes = Array.isArray(sourceTurn.dry_run?.validation_errors) ? sourceTurn.dry_run.validation_errors : []
      const missingFieldCodes = validationCodes.filter((code) =>
        ['missing_parent_ref_text', 'missing_entity_name', 'missing_effective_date', 'invalid_effective_date_format'].includes(code)
      )
      if (missingFieldCodes.length > 0) {
        const messages = dryRunValidationMessages(sourceTurn)
        if (messages.length > 0) {
          postBridgeNotice(`信息不完整，请补充后继续：${messages.join('；')}`, 'warning')
        }
        return
      }

      const candidatePending = sourceTurn.ambiguity_count > 1 && normalized(sourceTurn.resolved_candidate_id).length === 0
      if (candidatePending) {
        const candidateChoice = resolveCandidateFromInput(userInput, sourceTurn.candidates ?? [])
        if (candidateChoice.length === 0) {
          postBridgeNotice(formatCandidatePrompt(sourceTurn.candidates ?? []), 'warning')
          return
        }
        await autoCommitTurnFromChat(sourceConversation, sourceTurn, candidateChoice)
        return
      }

      await autoCommitTurnFromChat(sourceConversation, sourceTurn)
    },
    [autoCommitTurnFromChat, postBridgeNotice]
  )

  const tryHandlePendingTurnByDialogue = useCallback(
    async (sourceConversation: AssistantConversation, sourceTurn: AssistantTurn, userInput: string): Promise<boolean> => {
      const state = normalized(sourceTurn.state)
      const validationCodes = Array.isArray(sourceTurn.dry_run?.validation_errors) ? sourceTurn.dry_run.validation_errors : []
      const looksLikeCreateRequest = looksLikeCreateOrgUnitRequest(userInput)

      if (state === 'confirmed') {
        if (isExecutionConfirmationText(userInput)) {
          await autoCommitTurnFromChat(sourceConversation, sourceTurn)
          return true
        }
        if (!looksLikeCreateRequest) {
          postBridgeNotice('检测到待提交回合，请回复“确认执行”以继续；如需发起新任务，请明确输入创建需求。', 'info')
          return true
        }
      }

      if (state !== 'validated') {
        return false
      }

      const candidatePending = sourceTurn.ambiguity_count > 1 && normalized(sourceTurn.resolved_candidate_id).length === 0
      if (candidatePending && !looksLikeCreateRequest) {
        const candidateChoice = resolveCandidateFromInput(userInput, sourceTurn.candidates ?? [])
        if (candidateChoice.length === 0) {
          postBridgeNotice(formatCandidatePrompt(sourceTurn.candidates ?? []), 'warning')
          return true
        }
        await autoCommitTurnFromChat(sourceConversation, sourceTurn, candidateChoice)
        return true
      }

      if (validationCodes.length === 0 && isExecutionConfirmationText(userInput) && !looksLikeCreateRequest) {
        await autoCommitTurnFromChat(sourceConversation, sourceTurn)
        return true
      }

      return false
    },
    [autoCommitTurnFromChat, postBridgeNotice]
  )

  const runBridgeAutoFlow = useCallback(
    async (incomingText: string) => {
      const userInput = normalized(incomingText)
      if (userInput.length === 0) {
        return
      }
      setBridgeError('')

      let activeConversation = conversationRef.current
      if (!activeConversation) {
        try {
          const created = await createAssistantConversation()
          applyConversation(created)
          activeConversation = created
        } catch (err) {
          const message = errorMessage(err, '创建会话失败')
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

      let generationInput = userInput
      if (pendingTurn && normalized(pendingTurn.state) === 'validated') {
        const validationCodes = Array.isArray(pendingTurn.dry_run?.validation_errors) ? pendingTurn.dry_run.validation_errors : []
        const hasMissingFields = validationCodes.some((code) =>
          ['missing_parent_ref_text', 'missing_entity_name', 'missing_effective_date', 'invalid_effective_date_format'].includes(code)
        )
        if (hasMissingFields) {
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
      }

      let generatedConversation: AssistantConversation | null = null
      try {
        generatedConversation = await createAssistantTurn(activeConversation.conversation_id, generationInput)
        applyConversation(generatedConversation)
      } catch (err) {
        const message = errorMessage(err, '生成计划失败')
        setBridgeError(message)
        postBridgeNotice(message, 'error')
        return
      }

      const generatedTurn = latestTurn(generatedConversation)
      if (!generatedTurn || !generatedConversation) {
        return
      }
      await autoAdvanceGeneratedTurn(generatedConversation, generatedTurn, userInput)
    },
    [applyConversation, autoAdvanceGeneratedTurn, postBridgeNotice, tryHandlePendingTurnByDialogue]
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
        return
      }
      const message = validation.message
      if (!message) {
        return
      }
      if (message.type === 'assistant.bridge.ready') {
        setBridgeStatus('自动执行通道已连接：可直接在 LibreChat 对话中输入需求。')
        postBridgeNotice('自动执行通道已连接：可直接在 LibreChat 对话中输入需求。', 'info')
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
  }, [allowedOrigins, bridgeChannel, bridgeNonce, enqueueBridgePrompt, postBridgeNotice])

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
        独立聊天页：支持“对话式自动执行”，无需切回右侧事务按钮。
      </Typography>
      <Alert severity='info'>{bridgeStatus}</Alert>
      {bridgeError ? <Alert severity='error'>{bridgeError}</Alert> : null}
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
