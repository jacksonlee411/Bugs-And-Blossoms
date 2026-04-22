import AddCommentOutlinedIcon from '@mui/icons-material/AddCommentOutlined'
import ArchiveOutlinedIcon from '@mui/icons-material/ArchiveOutlined'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import CompressOutlinedIcon from '@mui/icons-material/CompressOutlined'
import DriveFileRenameOutlineIcon from '@mui/icons-material/DriveFileRenameOutline'
import HistoryOutlinedIcon from '@mui/icons-material/HistoryOutlined'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import SendIcon from '@mui/icons-material/Send'
import StopCircleOutlinedIcon from '@mui/icons-material/StopCircleOutlined'
import {
  Alert,
  Chip,
  Button,
  CircularProgress,
  Divider,
  Dialog,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  ListItemSecondaryAction,
  Switch,
  IconButton,
  List,
  ListItemButton,
  ListItem,
  ListItemText,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography
} from '@mui/material'
import { useEffect, useMemo, useState } from 'react'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import {
  deactivateModelCredential,
  loadModelSettings,
  rotateModelCredential,
  selectActiveModel,
  upsertModelProvider,
  verifyActiveModel
} from './api'
import type { CubeBoxModelSettingsSnapshot } from './types'
import { useCubeBox } from './CubeBoxProvider'

export function CubeBoxPanel() {
  const {
    archiveConversation,
    compactCurrentConversation,
    conversations,
    conversationsLoading,
    renameConversation,
    selectConversation,
    startNewConversation,
    state,
    interrupt,
    sendMessage,
    setComposerText
  } = useCubeBox()
  const { hasPermission, t } = useAppPreferences()
  const [historyOpen, setHistoryOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [settingsLoading, setSettingsLoading] = useState(false)
  const [settingsSaving, setSettingsSaving] = useState(false)
  const [settingsError, setSettingsError] = useState<string | null>(null)
  const [settingsSnapshot, setSettingsSnapshot] = useState<CubeBoxModelSettingsSnapshot | null>(null)
  const [providerID, setProviderID] = useState('openai-compatible')
  const [providerType, setProviderType] = useState('openai-compatible')
  const [providerName, setProviderName] = useState('Primary Provider')
  const [providerBaseURL, setProviderBaseURL] = useState('')
  const [providerEnabled, setProviderEnabled] = useState(true)
  const [credentialSecretRef, setCredentialSecretRef] = useState('')
  const [credentialMaskedSecret, setCredentialMaskedSecret] = useState('sk-****')
  const [modelSlug, setModelSlug] = useState('gpt-4.1')
  const [capabilitySummaryText, setCapabilitySummaryText] = useState('{"streaming":true,"tool_calls":false}')
  const canReadConversations = hasPermission('cubebox.conversations.read') || hasPermission('cubebox.conversations.use')
  const canUseConversations = hasPermission('cubebox.conversations.use')
  const canOpenSettings = hasPermission('cubebox.model_credential.read')
  const canEditProvider = hasPermission('cubebox.model_provider.update')
  const canRotateCredential = hasPermission('cubebox.model_credential.rotate')
  const canDeactivateCredential = hasPermission('cubebox.model_credential.deactivate')
  const canSelectActiveModel = hasPermission('cubebox.model_selection.select')
  const canVerifyActiveModel = hasPermission('cubebox.model_selection.verify')
  const currentTitle = state.conversation?.title?.trim() || t('page_cubebox_title')
  const latestHealthLabel = settingsSnapshot?.health ? formatHealth(settingsSnapshot.health.status, t) : t('cubebox_settings_health_unknown')
  const activeSelectionLabel = settingsSnapshot?.selection
    ? `${settingsSnapshot.selection.provider_id} / ${settingsSnapshot.selection.model_slug}`
    : t('cubebox_settings_no_selection')

  async function refreshSettings() {
    setSettingsLoading(true)
    setSettingsError(null)
    try {
      const payload = await loadModelSettings()
      setSettingsSnapshot(payload)
      const provider = payload.providers[0]
      if (provider) {
        setProviderID(provider.id)
        setProviderType(provider.provider_type)
        setProviderName(provider.display_name)
        setProviderBaseURL(provider.base_url)
        setProviderEnabled(provider.enabled)
      }
      const credential = payload.credentials.find((item) => item.active) ?? payload.credentials[0]
      if (credential) {
        setCredentialSecretRef(credential.secret_ref)
        setCredentialMaskedSecret(credential.masked_secret)
      }
      if (payload.selection) {
        setModelSlug(payload.selection.model_slug)
        setCapabilitySummaryText(JSON.stringify(payload.selection.capability_summary))
      }
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : 'unknown error')
    } finally {
      setSettingsLoading(false)
    }
  }

  useEffect(() => {
    if (!settingsOpen) {
      return
    }
    void refreshSettings()
  }, [settingsOpen])

  const providerCredentials = useMemo(
    () => settingsSnapshot?.credentials.filter((item) => item.provider_id === providerID) ?? [],
    [providerID, settingsSnapshot]
  )

  async function handleProviderSave() {
    setSettingsSaving(true)
    setSettingsError(null)
    try {
      await upsertModelProvider({
        providerID,
        providerType,
        displayName: providerName,
        baseURL: providerBaseURL,
        enabled: providerEnabled
      })
      await refreshSettings()
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : 'unknown error')
    } finally {
      setSettingsSaving(false)
    }
  }

  async function handleCredentialRotate() {
    setSettingsSaving(true)
    setSettingsError(null)
    try {
      await rotateModelCredential({
        providerID,
        secretRef: credentialSecretRef,
        maskedSecret: credentialMaskedSecret
      })
      await refreshSettings()
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : 'unknown error')
    } finally {
      setSettingsSaving(false)
    }
  }

  async function handleSelectionSave() {
    setSettingsSaving(true)
    setSettingsError(null)
    try {
      const parsed = JSON.parse(capabilitySummaryText) as unknown
      if (!isRecord(parsed)) {
        throw new Error(t('cubebox_settings_capability_summary_invalid'))
      }
      await selectActiveModel({
        providerID,
        modelSlug,
        capabilitySummary: parsed
      })
      await refreshSettings()
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : 'unknown error')
    } finally {
      setSettingsSaving(false)
    }
  }

  async function handleVerify() {
    setSettingsSaving(true)
    setSettingsError(null)
    try {
      await verifyActiveModel()
      await refreshSettings()
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : 'unknown error')
    } finally {
      setSettingsSaving(false)
    }
  }

  async function handleCredentialDeactivate(credentialID: string) {
    setSettingsSaving(true)
    setSettingsError(null)
    try {
      await deactivateModelCredential(credentialID)
      await refreshSettings()
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : 'unknown error')
    } finally {
      setSettingsSaving(false)
    }
  }

  return (
    <Stack spacing={2} sx={{ height: '100%' }}>
      <Stack alignItems='center' direction='row' justifyContent='space-between' spacing={2}>
        <Stack spacing={0.5} sx={{ minWidth: 0 }}>
          <Typography component='h2' noWrap variant='h6'>
            {currentTitle}
          </Typography>
          <Typography color='text.secondary' noWrap variant='body2'>
            {state.conversation?.id ? `${t('cubebox_conversation_id')}: ${state.conversation.id}` : t('page_cubebox_subtitle')}
          </Typography>
        </Stack>
        <Stack direction='row' spacing={0.5}>
          <Tooltip title={t('cubebox_history')}>
            <span>
              <IconButton aria-label={t('cubebox_history')} disabled={!canReadConversations} onClick={() => setHistoryOpen(true)}>
                <HistoryOutlinedIcon />
              </IconButton>
            </span>
          </Tooltip>
          {canOpenSettings ? (
            <Tooltip title={t('cubebox_settings')}>
              <span>
                <IconButton aria-label={t('cubebox_settings')} onClick={() => setSettingsOpen(true)}>
                  <SettingsOutlinedIcon />
                </IconButton>
              </span>
            </Tooltip>
          ) : null}
          <Tooltip title={t('cubebox_new_chat')}>
            <span>
              <IconButton
                aria-label={t('cubebox_new_chat')}
                disabled={!canUseConversations || state.loading || state.turnStatus === 'streaming'}
                onClick={() => void startNewConversation()}
              >
                <AddCommentOutlinedIcon />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('cubebox_compact')}>
            <span>
              <IconButton
                aria-label={t('cubebox_compact')}
                disabled={!canUseConversations || !state.conversation?.id || state.loading || state.compacting || state.turnStatus === 'streaming'}
                onClick={() => void compactCurrentConversation()}
              >
                <CompressOutlinedIcon />
              </IconButton>
            </span>
          </Tooltip>
        </Stack>
      </Stack>

      <Paper sx={{ flex: 1, overflow: 'auto', p: 2 }} variant='outlined'>
        <Stack direction='row' spacing={1} sx={{ mb: 2 }}>
          <AutoAwesomeIcon color='primary' fontSize='small' />
          <Typography variant='body2'>
            {state.conversation?.id ? `${t('cubebox_conversation_id')}: ${state.conversation.id}` : t('text_loading')}
          </Typography>
        </Stack>
        <Divider sx={{ mb: 2 }} />
        {state.errorMessage ? <Alert severity='error'>{state.errorMessage}</Alert> : null}
        <List sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {state.items.map((item) => (
            <ListItem
              key={item.id}
              sx={{
                alignItems: 'flex-start',
                bgcolor: item.kind === 'user_message' ? 'primary.50' : item.kind === 'error_item' ? 'error.50' : 'grey.50',
                borderRadius: 2,
                border: '1px solid',
                borderColor: item.kind === 'error_item' ? 'error.200' : 'divider'
              }}
            >
              <Stack spacing={0.5}>
                <Typography variant='caption'>
                  {item.kind === 'user_message'
                    ? t('cubebox_user_message')
                    : item.kind === 'compact_item'
                      ? t('cubebox_compact_item')
                    : item.kind === 'error_item'
                      ? t('cubebox_error_item')
                      : t('cubebox_agent_message')}
                </Typography>
                <Typography sx={{ whiteSpace: 'pre-wrap' }} variant='body2'>
                  {item.text}
                </Typography>
                {item.status ? (
                  <Typography color='text.secondary' variant='caption'>
                    {item.status}
                  </Typography>
                ) : null}
              </Stack>
            </ListItem>
          ))}
          {state.items.length === 0 && !state.loading ? (
            <Typography color='text.secondary' variant='body2'>
              {t('cubebox_empty_timeline')}
            </Typography>
          ) : null}
        </List>
      </Paper>

      <Paper sx={{ p: 2 }} variant='outlined'>
        <Stack spacing={2}>
          <TextField
            fullWidth
            disabled={!canUseConversations || state.loading}
            label={t('cubebox_prompt_label')}
            multiline
            minRows={3}
            onChange={(event) => setComposerText(event.target.value)}
            value={state.composerText}
          />
          <Stack direction='row' justifyContent='space-between' spacing={2}>
            <Typography color='text.secondary' variant='body2'>
              {statusLabel(state.turnStatus, t)}
            </Typography>
            <Stack direction='row' spacing={1}>
              {state.loading ? <CircularProgress size={18} /> : null}
              {state.compacting ? <CircularProgress size={18} /> : null}
              <Button
                color='warning'
                disabled={!canUseConversations || state.turnStatus !== 'streaming'}
                onClick={() => void interrupt()}
                startIcon={<StopCircleOutlinedIcon />}
                variant='outlined'
              >
                {t('cubebox_stop')}
              </Button>
              <Button
                disabled={!canUseConversations || state.loading || state.composerText.trim().length === 0 || state.turnStatus === 'streaming'}
                onClick={() => {
                  if (state.composerText.trim() === '/compact') {
                    void compactCurrentConversation()
                    return
                  }
                  void sendMessage()
                }}
                startIcon={<SendIcon />}
                variant='contained'
              >
                {t('cubebox_send')}
              </Button>
            </Stack>
          </Stack>
        </Stack>
      </Paper>

      <Dialog fullWidth maxWidth='sm' onClose={() => setHistoryOpen(false)} open={historyOpen}>
        <DialogTitle>{t('cubebox_history_title')}</DialogTitle>
        <DialogContent dividers>
          <List sx={{ p: 0 }}>
            {conversations.map((conversation) => (
              <ListItem
                key={conversation.id}
                disablePadding
                secondaryAction={
                  <Stack direction='row' spacing={0.5}>
                    <Tooltip title={t('cubebox_rename')}>
                      <span>
                        <IconButton
                          disabled={!canUseConversations}
                          edge='end'
                          size='small'
                          onClick={() => {
                            const title = window.prompt(t('cubebox_prompt_label'), conversation.title)
                            if (typeof title === 'string' && title.trim().length > 0) {
                              void renameConversation(conversation.id, title.trim())
                            }
                          }}
                        >
                          <DriveFileRenameOutlineIcon fontSize='small' />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip title={conversation.archived ? t('cubebox_unarchive') : t('cubebox_archive')}>
                      <span>
                        <IconButton disabled={!canUseConversations} edge='end' size='small' onClick={() => void archiveConversation(conversation.id, !conversation.archived)}>
                          <ArchiveOutlinedIcon fontSize='small' />
                        </IconButton>
                      </span>
                    </Tooltip>
                  </Stack>
                }
              >
                <ListItemButton
                  selected={state.conversation?.id === conversation.id}
                  onClick={() => {
                    void selectConversation(conversation.id)
                    setHistoryOpen(false)
                  }}
                >
                  <ListItemText
                    primary={conversation.title}
                    primaryTypographyProps={{ noWrap: true, variant: 'body2' }}
                    secondary={`${conversation.id} · ${formatConversationStatus(conversation.archived, t)}`}
                    secondaryTypographyProps={{ noWrap: true, variant: 'caption' }}
                  />
                </ListItemButton>
              </ListItem>
            ))}
            {conversations.length === 0 && !conversationsLoading ? (
              <Typography color='text.secondary' sx={{ py: 1 }} variant='body2'>
                {t('cubebox_empty_history')}
              </Typography>
            ) : null}
            {conversationsLoading ? (
              <Stack alignItems='center' direction='row' spacing={1} sx={{ py: 1 }}>
                <CircularProgress size={16} />
                <Typography color='text.secondary' variant='body2'>
                  {t('text_loading')}
                </Typography>
              </Stack>
            ) : null}
          </List>
        </DialogContent>
      </Dialog>

      <Dialog fullWidth maxWidth='xs' onClose={() => setSettingsOpen(false)} open={settingsOpen}>
        <DialogTitle>{t('cubebox_settings_title')}</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={2}>
            <Stack spacing={0.5}>
              <Typography variant='subtitle2'>{t('cubebox_settings_active_model')}</Typography>
              <Typography color='text.secondary' variant='body2'>
                {activeSelectionLabel}
              </Typography>
              <Stack direction='row' spacing={1}>
                <Chip color={settingsSnapshot?.health?.status === 'healthy' ? 'success' : settingsSnapshot?.health?.status === 'failed' ? 'error' : 'warning'} label={latestHealthLabel} size='small' />
                {settingsSnapshot?.health?.latency_ms ? <Chip label={`${settingsSnapshot.health.latency_ms} ms`} size='small' variant='outlined' /> : null}
              </Stack>
            </Stack>
            {settingsError ? <Alert severity='error'>{settingsError}</Alert> : null}
            {settingsLoading ? (
              <Stack alignItems='center' direction='row' spacing={1}>
                <CircularProgress size={16} />
                <Typography color='text.secondary' variant='body2'>
                  {t('text_loading')}
                </Typography>
              </Stack>
            ) : null}
            <TextField fullWidth disabled={!canEditProvider || settingsSaving} label={t('cubebox_settings_provider_id')} onChange={(event) => setProviderID(event.target.value)} value={providerID} />
            <TextField fullWidth disabled={!canEditProvider || settingsSaving} label={t('cubebox_settings_provider_type')} onChange={(event) => setProviderType(event.target.value)} value={providerType} />
            <TextField fullWidth disabled={!canEditProvider || settingsSaving} label={t('cubebox_settings_provider_name')} onChange={(event) => setProviderName(event.target.value)} value={providerName} />
            <TextField fullWidth disabled={!canEditProvider || settingsSaving} label={t('cubebox_settings_base_url')} onChange={(event) => setProviderBaseURL(event.target.value)} value={providerBaseURL} />
            <FormControlLabel control={<Switch checked={providerEnabled} disabled={!canEditProvider || settingsSaving} onChange={(event) => setProviderEnabled(event.target.checked)} />} label={t('cubebox_settings_provider_enabled')} />
            <Button disabled={!canEditProvider || settingsSaving} onClick={() => void handleProviderSave()} variant='outlined'>
              {t('cubebox_settings_save_provider')}
            </Button>
            <Divider />
            <TextField fullWidth disabled={!canRotateCredential || settingsSaving} label={t('cubebox_settings_secret_ref')} onChange={(event) => setCredentialSecretRef(event.target.value)} value={credentialSecretRef} />
            <TextField fullWidth disabled={!canRotateCredential || settingsSaving} label={t('cubebox_settings_masked_secret')} onChange={(event) => setCredentialMaskedSecret(event.target.value)} value={credentialMaskedSecret} />
            <Button disabled={!canRotateCredential || settingsSaving} onClick={() => void handleCredentialRotate()} variant='outlined'>
              {t('cubebox_settings_rotate_credential')}
            </Button>
            <List dense sx={{ p: 0 }}>
              {providerCredentials.map((credential) => (
                <ListItem key={credential.id}>
                  <ListItemText primary={`${credential.masked_secret} · v${credential.version}`} secondary={`${credential.secret_ref} · ${credential.active ? t('cubebox_settings_credential_active') : t('cubebox_settings_credential_inactive')}`} />
                  {canDeactivateCredential && credential.active ? (
                    <ListItemSecondaryAction>
                      <Button color='warning' disabled={settingsSaving} onClick={() => void handleCredentialDeactivate(credential.id)} size='small'>
                        {t('cubebox_settings_deactivate_credential')}
                      </Button>
                    </ListItemSecondaryAction>
                  ) : null}
                </ListItem>
              ))}
            </List>
            <Divider />
            <TextField fullWidth disabled={!canSelectActiveModel || settingsSaving} label={t('cubebox_settings_model_slug')} onChange={(event) => setModelSlug(event.target.value)} value={modelSlug} />
            <TextField
              fullWidth
              disabled={!canSelectActiveModel || settingsSaving}
              label={t('cubebox_settings_capability_summary')}
              multiline
              minRows={3}
              onChange={(event) => setCapabilitySummaryText(event.target.value)}
              value={capabilitySummaryText}
            />
            <Stack direction='row' spacing={1}>
              <Button disabled={!canSelectActiveModel || settingsSaving} onClick={() => void handleSelectionSave()} variant='contained'>
                {t('cubebox_settings_save_selection')}
              </Button>
              <Button disabled={!canVerifyActiveModel || settingsSaving} onClick={() => void handleVerify()} variant='outlined'>
                {t('cubebox_settings_verify')}
              </Button>
            </Stack>
          </Stack>
        </DialogContent>
      </Dialog>
    </Stack>
  )
}

function formatConversationStatus(
  archived: boolean,
  t: (key: 'cubebox_conversation_status_active' | 'cubebox_conversation_status_archived') => string
) {
  return archived ? t('cubebox_conversation_status_archived') : t('cubebox_conversation_status_active')
}

function statusLabel(
  status: 'idle' | 'streaming' | 'completed' | 'error' | 'interrupted',
  t: (key: 'cubebox_status_streaming' | 'cubebox_status_completed' | 'cubebox_status_error' | 'cubebox_status_interrupted' | 'cubebox_status_idle') => string
) {
  switch (status) {
    case 'streaming':
      return t('cubebox_status_streaming' as never)
    case 'completed':
      return t('cubebox_status_completed' as never)
    case 'error':
      return t('cubebox_status_error' as never)
    case 'interrupted':
      return t('cubebox_status_interrupted' as never)
    default:
      return t('cubebox_status_idle' as never)
  }
}

function formatHealth(
  status: 'healthy' | 'degraded' | 'failed',
  t: (key: 'cubebox_settings_health_healthy' | 'cubebox_settings_health_degraded' | 'cubebox_settings_health_failed') => string
) {
  switch (status) {
    case 'healthy':
      return t('cubebox_settings_health_healthy')
    case 'degraded':
      return t('cubebox_settings_health_degraded')
    default:
      return t('cubebox_settings_health_failed')
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
