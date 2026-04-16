import AttachFileIcon from '@mui/icons-material/AttachFile'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import SendIcon from '@mui/icons-material/Send'
import TaskAltIcon from '@mui/icons-material/TaskAlt'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  List,
  ListItemButton,
  ListItemText,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { useNavigate, useParams } from 'react-router-dom'
import {
  commitCubeBoxTurn,
  confirmCubeBoxTurn,
  createCubeBoxConversation,
  createCubeBoxTurn,
  getCubeBoxConversation,
  getCubeBoxRuntimeStatus,
  getCubeBoxTask,
  listCubeBoxConversations,
  listCubeBoxFiles,
  renderCubeBoxTurnReply,
  uploadCubeBoxFile,
  type CubeBoxConversation,
  type CubeBoxFile,
  type CubeBoxRuntimeStatusResponse
} from '../../api/cubebox'
import { cubeBoxErrorMessage } from './errorMessage'

function cubeBoxFileLabel(file: CubeBoxFile): string {
  return file.filename ?? file.file_name
}

function cubeBoxFileContentType(file: CubeBoxFile): string {
  return file.content_type ?? file.media_type
}

function healthColor(value: string): 'default' | 'success' | 'warning' | 'error' {
  if (value === 'healthy') return 'success'
  if (value === 'degraded') return 'warning'
  if (value === 'unavailable') return 'error'
  return 'default'
}

function turnSecondaryText(conversation: CubeBoxConversation | null, turnID: string): string {
  if (!conversation) {
    return ''
  }
  const turns = Array.isArray(conversation.turns) ? conversation.turns : []
  const turn = turns.find((item) => item.turn_id === turnID)
  if (!turn) {
    return ''
  }
  if (turn.reply_nlg?.text) {
    return turn.reply_nlg.text
  }
  if (turn.plan?.summary) {
    return turn.plan.summary
  }
  return turn.user_input
}

function normalizeConversation(conversation: CubeBoxConversation): CubeBoxConversation {
  return {
    ...conversation,
    turns: Array.isArray(conversation.turns) ? conversation.turns : []
  }
}

export function CubeBoxPage() {
  const { locale, t } = useAppPreferences()
  const navigate = useNavigate()
  const { conversationId } = useParams<{ conversationId?: string }>()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [runtimeStatus, setRuntimeStatus] = useState<CubeBoxRuntimeStatusResponse | null>(null)
  const [conversations, setConversations] = useState<CubeBoxConversation['turns'][number][]>([])
  const [conversationItems, setConversationItems] = useState<Array<{ conversation_id: string; state: string; updated_at: string }>>([])
  const [selectedConversation, setSelectedConversation] = useState<CubeBoxConversation | null>(null)
  const [files, setFiles] = useState<CubeBoxFile[]>([])
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [taskMessage, setTaskMessage] = useState('')

  const currentTurn = useMemo(() => {
    const turns = selectedConversation && Array.isArray(selectedConversation.turns) ? selectedConversation.turns : []
    if (turns.length === 0) {
      return null
    }
    return turns[turns.length - 1]
  }, [selectedConversation])
  const currentCandidates = currentTurn?.candidates ?? []
  const candidateSelectionRequired = currentCandidates.length > 1 && !currentTurn?.resolved_candidate_id

  async function refreshConversationList() {
    const response = await listCubeBoxConversations({ page_size: 20 })
    setConversationItems(response.items.map((item) => ({
      conversation_id: item.conversation_id,
      state: item.state,
      updated_at: item.updated_at
    })))
  }

  async function refreshConversation(targetConversationID?: string) {
    if (!targetConversationID) {
      setSelectedConversation(null)
      setConversations([])
      setFiles([])
      return
    }
    const [conversation, fileResponse] = await Promise.all([
      getCubeBoxConversation(targetConversationID),
      listCubeBoxFiles({ conversation_id: targetConversationID })
    ])
    const normalizedConversation = normalizeConversation(conversation)
    setSelectedConversation(normalizedConversation)
    setConversations(normalizedConversation.turns)
    setFiles(fileResponse.items)
  }

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const [runtime] = await Promise.all([
          getCubeBoxRuntimeStatus(),
          refreshConversationList(),
          refreshConversation(conversationId)
        ])
        if (!active) {
          return
        }
        setRuntimeStatus(runtime)
      } catch (error) {
        if (!active) {
          return
        }
        setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_page_load'), locale))
      }
    })()
    return () => {
      active = false
    }
  }, [conversationId])

  async function ensureConversationID(): Promise<string> {
    if (conversationId && conversationId.trim().length > 0) {
      return conversationId
    }
    const created = await createCubeBoxConversation()
    const nextConversationID = created.conversation_id
    await refreshConversationList()
    navigate(`/cubebox/conversations/${nextConversationID}`)
    return nextConversationID
  }

  async function handleSend() {
    const userInput = draft.trim()
    if (userInput.length === 0) {
      return
    }
    setBusy(true)
    setErrorMessage('')
    try {
      const targetConversationID = await ensureConversationID()
      const conversation = normalizeConversation(await createCubeBoxTurn(targetConversationID, userInput))
      setDraft('')
      setSelectedConversation(conversation)
      setConversations(conversation.turns)
      await refreshConversationList()
      await refreshConversation(targetConversationID)
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_send'), locale))
    } finally {
      setBusy(false)
    }
  }

  async function handleGenerateReply() {
    if (!conversationId || !currentTurn) {
      return
    }
    setBusy(true)
    setErrorMessage('')
    try {
      const reply = await renderCubeBoxTurnReply(conversationId, currentTurn.turn_id, {
        locale,
        fallback_text: currentTurn.plan?.summary ?? currentTurn.user_input,
        allow_missing_turn: false
      })
      setSelectedConversation((prev) => {
        if (!prev) {
          return prev
        }
        return {
          ...prev,
          turns: prev.turns.map((turn) => turn.turn_id === reply.turn_id ? { ...turn, reply_nlg: reply } : turn)
        }
      })
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_generate_reply'), locale))
    } finally {
      setBusy(false)
    }
  }

  async function handleConfirm(candidateID?: string) {
    if (!conversationId || !currentTurn) {
      return
    }
    const candidateIDToConfirm =
      candidateID ??
      currentTurn.resolved_candidate_id ??
      (currentTurn.candidates.length === 1 ? currentTurn.candidates[0]?.candidate_id : undefined)
    setBusy(true)
    setErrorMessage('')
    try {
      const conversation = normalizeConversation(await confirmCubeBoxTurn(conversationId, currentTurn.turn_id, candidateIDToConfirm))
      setSelectedConversation(conversation)
      setConversations(conversation.turns)
      await refreshConversationList()
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_confirm'), locale))
    } finally {
      setBusy(false)
    }
  }

  async function handleCommit() {
    if (!conversationId || !currentTurn) {
      return
    }
    setBusy(true)
    setErrorMessage('')
    setTaskMessage('')
    try {
      const receipt = await commitCubeBoxTurn(conversationId, currentTurn.turn_id)
      setTaskMessage(t('cubebox_task_submitted', { taskId: receipt.task_id }))
      const detail = await getCubeBoxTask(receipt.task_id)
      setTaskMessage(t('cubebox_task_status', { status: detail.status }))
      await refreshConversation(conversationId)
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_commit'), locale))
    } finally {
      setBusy(false)
    }
  }

  async function handlePickConversation(targetConversationID: string) {
    navigate(`/cubebox/conversations/${targetConversationID}`)
    try {
      await refreshConversation(targetConversationID)
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_conversation_load'), locale))
    }
  }

  async function handleUploadFile(fileList: FileList | null) {
    const file = fileList?.item(0)
    if (!file) {
      return
    }
    setBusy(true)
    setErrorMessage('')
    try {
      const targetConversationID = conversationId ? conversationId : undefined
      await uploadCubeBoxFile(file, targetConversationID)
      if (targetConversationID) {
        const next = await listCubeBoxFiles({ conversation_id: targetConversationID })
        setFiles(next.items)
      } else {
        const next = await listCubeBoxFiles()
        setFiles(next.items)
      }
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_attachment_upload'), locale))
    } finally {
      setBusy(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <Typography variant='h5'>{t('cubebox_title')}</Typography>
        <Chip
          color={healthColor(runtimeStatus?.status ?? '')}
          data-testid='cubebox-runtime-status'
          label={runtimeStatus?.status ?? '-'}
          size='small'
        />
        <Box sx={{ flex: 1 }} />
        <Button onClick={() => navigate('/cubebox/files')} variant='text'>
          {t('cubebox_link_files')}
        </Button>
        <Button onClick={() => navigate('/cubebox/models')} variant='text'>
          {t('cubebox_link_models')}
        </Button>
      </Stack>

      <Typography color='text.secondary' variant='body2'>
        {t('cubebox_entry_subtitle')}
      </Typography>

      {errorMessage ? <Alert severity='warning'>{errorMessage}</Alert> : null}
      {taskMessage ? <Alert severity='info'>{taskMessage}</Alert> : null}

      <Stack direction={{ xs: 'column', lg: 'row' }} spacing={2}>
        <Card sx={{ minWidth: 280, width: { lg: 320 } }}>
          <CardContent>
            <Stack spacing={1}>
              <Typography variant='subtitle1'>{t('cubebox_conversation_list')}</Typography>
              <List disablePadding>
                {conversationItems.map((item) => (
                  <ListItemButton
                    data-testid='cubebox-conversation-item'
                    key={item.conversation_id}
                    onClick={() => void handlePickConversation(item.conversation_id)}
                    selected={item.conversation_id === conversationId}
                  >
                    <ListItemText primary={item.conversation_id} secondary={`${item.state} · ${item.updated_at}`} />
                  </ListItemButton>
                ))}
                {conversationItems.length === 0 ? (
                  <Typography color='text.secondary' variant='body2'>
                    {t('cubebox_conversation_empty')}
                  </Typography>
                ) : null}
              </List>
            </Stack>
          </CardContent>
        </Card>

        <Card sx={{ flex: 1 }}>
          <CardContent>
            <Stack spacing={2}>
              <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap'>
                <Chip
                  label={`knowledge=${runtimeStatus?.knowledge_runtime.healthy ?? '-'}`}
                  size='small'
                  variant='outlined'
                />
                <Chip
                  label={`model_gateway=${runtimeStatus?.model_gateway.healthy ?? '-'}`}
                  size='small'
                  variant='outlined'
                />
                <Chip
                  label={`file_store=${runtimeStatus?.file_store.healthy ?? '-'}`}
                  size='small'
                  variant='outlined'
                />
                {(runtimeStatus?.retired_capabilities ?? []).map((item) => (
                  <Chip key={item} label={`${item}: retired`} size='small' variant='outlined' />
                ))}
              </Stack>

              <Divider />

              <Stack spacing={1.5}>
                {conversations.map((turn) => (
                  <Card
                    data-conversation-id={selectedConversation?.conversation_id ?? ''}
                    data-request-id={turn.request_id}
                    data-testid='cubebox-turn-card'
                    data-turn-id={turn.turn_id}
                    key={turn.turn_id}
                    variant='outlined'
                  >
                    <CardContent>
                      <Stack spacing={1}>
                        <Stack alignItems='center' direction='row' spacing={1}>
                          <Chip label={turn.state} size='small' />
                          <Chip label={turn.risk_tier} size='small' variant='outlined' />
                          <Typography color='text.secondary' variant='caption'>
                            {turn.turn_id}
                          </Typography>
                        </Stack>
                        <Typography variant='body1'>{turn.user_input}</Typography>
                        <Typography color='text.secondary' variant='body2'>
                          {turnSecondaryText(selectedConversation, turn.turn_id)}
                        </Typography>
                      </Stack>
                    </CardContent>
                  </Card>
                ))}
                {conversations.length === 0 ? (
                  <Typography color='text.secondary' variant='body2'>
                    {t('cubebox_conversation_first_prompt')}
                  </Typography>
                ) : null}
              </Stack>

              {candidateSelectionRequired ? (
                <>
                  <Divider />
                  <Stack data-testid='cubebox-candidate-panel' spacing={1}>
                    <Typography variant='subtitle2'>{t('cubebox_candidate_title')}</Typography>
                    <Typography color='text.secondary' variant='body2'>
                      {t('cubebox_candidate_hint')}
                    </Typography>
                    <Stack spacing={1}>
                      {currentCandidates.map((candidate, index) => (
                        <Card key={candidate.candidate_id} variant='outlined'>
                          <CardContent>
                            <Stack
                              alignItems={{ xs: 'flex-start', md: 'center' }}
                              direction={{ xs: 'column', md: 'row' }}
                              spacing={1}
                            >
                              <Stack spacing={0.5} sx={{ flex: 1 }}>
                                <Typography variant='body2'>{candidate.name}</Typography>
                                <Typography color='text.secondary' variant='caption'>
                                  {candidate.path || candidate.candidate_code}
                                </Typography>
                              </Stack>
                              <Button
                                data-testid={`cubebox-candidate-select-${index + 1}`}
                                disabled={busy}
                                onClick={() => void handleConfirm(candidate.candidate_id)}
                                variant='outlined'
                              >
                                {t('cubebox_candidate_select', { index: index + 1 })}
                              </Button>
                            </Stack>
                          </CardContent>
                        </Card>
                      ))}
                    </Stack>
                  </Stack>
                </>
              ) : null}

              <Divider />

              <Stack spacing={1}>
                <Stack alignItems='center' direction='row' spacing={1}>
                  <Typography variant='subtitle2'>{t('cubebox_attachments_title')}</Typography>
                  <Box sx={{ flex: 1 }} />
                  <input
                    hidden
                    onChange={(event) => void handleUploadFile(event.target.files)}
                    ref={fileInputRef}
                    type='file'
                  />
                  <Button
                    onClick={() => fileInputRef.current?.click()}
                    size='small'
                    startIcon={<AttachFileIcon />}
                    variant='outlined'
                  >
                    {t('cubebox_attachments_upload')}
                  </Button>
                </Stack>
                <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap'>
                  {files.map((file) => (
                    <Chip key={file.file_id} label={`${cubeBoxFileLabel(file)} · ${cubeBoxFileContentType(file)}`} size='small' variant='outlined' />
                  ))}
                  {files.length === 0 ? (
                    <Typography color='text.secondary' variant='body2'>
                      {t('cubebox_attachments_empty')}
                    </Typography>
                  ) : null}
                </Stack>
              </Stack>

              <Divider />

              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1}>
                <TextField
                  data-testid='cubebox-input'
                  fullWidth
                  inputProps={{ 'data-testid': 'cubebox-input-field' }}
                  minRows={3}
                  multiline
                  onChange={(event) => setDraft(event.target.value)}
                  placeholder={t('cubebox_input_placeholder')}
                  value={draft}
                />
                <Stack direction={{ xs: 'row', md: 'column' }} spacing={1}>
                  <Button
                    data-testid='cubebox-send'
                    disabled={busy || draft.trim().length === 0}
                    onClick={() => void handleSend()}
                    startIcon={<SendIcon />}
                    variant='contained'
                  >
                    {t('cubebox_action_send')}
                  </Button>
                  <Button
                    data-testid='cubebox-generate-reply'
                    disabled={busy || !currentTurn}
                    onClick={() => void handleGenerateReply()}
                    startIcon={<AutoAwesomeIcon />}
                    variant='outlined'
                  >
                    {t('cubebox_action_generate_reply')}
                  </Button>
                  <Button
                    data-testid='cubebox-confirm'
                    disabled={busy || !currentTurn || candidateSelectionRequired}
                    onClick={() => void handleConfirm()}
                    startIcon={<TaskAltIcon />}
                    variant='outlined'
                  >
                    {t('cubebox_action_confirm')}
                  </Button>
                  <Button
                    data-testid='cubebox-commit'
                    disabled={busy || !currentTurn}
                    onClick={() => void handleCommit()}
                    variant='outlined'
                  >
                    {t('cubebox_action_commit')}
                  </Button>
                </Stack>
              </Stack>
            </Stack>
          </CardContent>
        </Card>
      </Stack>
    </Stack>
  )
}
