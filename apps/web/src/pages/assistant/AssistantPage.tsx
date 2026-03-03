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
import { useCallback, useEffect, useMemo, useState } from 'react'
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
import { deriveAssistantActionState } from './assistantUiState'

const samplePrompt =
  '在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。'

const assistantStatePlaceholder = '-'

function latestTurn(conversation: AssistantConversation | null): AssistantTurn | null {
  if (!conversation || conversation.turns.length === 0) {
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
  if (turn.candidates.some((candidate) => candidate.candidate_id === currentCandidateID)) {
    return currentCandidateID
  }
  return ''
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

export function AssistantPage() {
  const [conversation, setConversation] = useState<AssistantConversation | null>(null)
  const [input, setInput] = useState(samplePrompt)
  const [candidateID, setCandidateID] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [bridgeChannel] = useState(() => createAssistantBridgeToken('assistant_channel'))
  const [bridgeNonce] = useState(() => createAssistantBridgeToken('assistant_nonce'))

  const turn = useMemo(() => latestTurn(conversation), [conversation])
  const conversationID = normalized(conversation?.conversation_id)
  const allowedOrigins = useMemo(() => {
    const origin = typeof window !== 'undefined' ? window.location.origin : undefined
    return parseAssistantAllowedOrigins(import.meta.env.VITE_ASSISTANT_ALLOWED_ORIGINS, origin)
  }, [])
  const iframeSrc = useMemo(
    () =>
      `/assistant-ui/?channel=${encodeURIComponent(bridgeChannel)}&nonce=${encodeURIComponent(bridgeNonce)}`,
    [bridgeChannel, bridgeNonce]
  )

  const applyConversation = useCallback((nextConversation: AssistantConversation) => {
    setConversation(nextConversation)
    const nextTurn = latestTurn(nextConversation)
    setCandidateID((current) => resolveCandidateSelection(nextTurn, current))
  }, [])

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

  const refreshConversation = useCallback(async () => {
    if (conversationID.length === 0) {
      return
    }
    const nextConversation = await getAssistantConversation(conversationID)
    applyConversation(nextConversation)
  }, [applyConversation, conversationID])

  useEffect(() => {
    let active = true
    setLoading(true)
    createAssistantConversation()
      .then((result) => {
        if (!active) {
          return
        }
        applyConversation(result)
      })
      .catch((err: unknown) => {
        if (!active) {
          return
        }
        setError(errorMessage(err, '创建会话失败'))
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [applyConversation])

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
      const next = await createAssistantTurn(conversation.conversation_id, text)
      applyConversation(next)
    } catch (err) {
      setError(errorMessage(err, '生成计划失败'))
    } finally {
      setLoading(false)
    }
  }, [applyConversation, conversation, input])

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
      if (message.type === 'assistant.prompt.sync') {
        const nextInput = message.payload.input
        if (typeof nextInput === 'string' && nextInput.trim().length > 0) {
          setInput(nextInput)
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
  }, [allowedOrigins, bridgeChannel, bridgeNonce, refreshConversation])

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <SmartToyIcon color='primary' />
        <Typography variant='h5'>AI 助手</Typography>
        <Box sx={{ flex: 1 }} />
        <Button component='a' href='/app/assistant/models' variant='text'>
          模型配置
        </Button>
      </Stack>
      <Typography color='text.secondary' variant='body2'>
        左侧为 LibreChat 聊天展示层，右侧为本系统事务控制面板（Confirm / Commit 仍走后端 One Door）。
      </Typography>
      {error ? (
        <Alert data-testid='assistant-error-alert' severity='error'>
          {error}
        </Alert>
      ) : null}
      <Box sx={{ display: 'flex', gap: 2, flexDirection: { xs: 'column', md: 'row' } }}>
        <Box sx={{ flex: 7 }}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography gutterBottom variant='subtitle1'>
                聊天与计划展示层（LibreChat）
              </Typography>
              <Box
                component='iframe'
                data-testid='assistant-librechat-frame'
                src={iframeSrc}
                title='LibreChat'
                sx={{ width: '100%', height: 580, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}
              />
            </CardContent>
          </Card>
        </Box>
        <Box sx={{ flex: 5 }}>
          <Card data-testid='assistant-transaction-panel'>
            <CardContent>
              <Stack spacing={2}>
                <Typography variant='subtitle1'>事务控制面板</Typography>
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
                <Typography variant='caption'>
                  compiler_contract_version: {turn?.plan.compiler_contract_version ?? '-'}
                </Typography>
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
                    {turn.dry_run.diff.length > 0 ? (
                      <Stack component='ul' data-testid='assistant-dryrun-diff' spacing={0.5} sx={{ m: 0, pl: 2 }}>
                        {turn.dry_run.diff.map((item, index) => (
                          <Typography component='li' key={`${item.field ?? 'field'}-${index}`} variant='caption'>
                            {String(item.field ?? '-')} {'->'} {stringifyDiffValue(item.after)}
                          </Typography>
                        ))}
                      </Stack>
                    ) : null}
                    {turn.dry_run.validation_errors?.length ? (
                      <Alert severity='warning'>
                        {turn.dry_run.validation_errors.join(', ')}
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
              </Stack>
            </CardContent>
          </Card>
        </Box>
      </Box>
    </Stack>
  )
}
