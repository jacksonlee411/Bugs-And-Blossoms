import AddCommentOutlinedIcon from '@mui/icons-material/AddCommentOutlined'
import ArchiveOutlinedIcon from '@mui/icons-material/ArchiveOutlined'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import DriveFileRenameOutlineIcon from '@mui/icons-material/DriveFileRenameOutline'
import HistoryOutlinedIcon from '@mui/icons-material/HistoryOutlined'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import SendIcon from '@mui/icons-material/Send'
import StopCircleOutlinedIcon from '@mui/icons-material/StopCircleOutlined'
import {
  Alert,
  Button,
  CircularProgress,
  Divider,
  Dialog,
  DialogContent,
  DialogTitle,
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
import { useState } from 'react'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { useCubeBox } from './CubeBoxProvider'

export function CubeBoxPanel() {
  const {
    archiveConversation,
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
  const { t } = useAppPreferences()
  const [historyOpen, setHistoryOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const currentTitle = state.conversation?.title?.trim() || t('page_cubebox_title')

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
              <IconButton aria-label={t('cubebox_history')} onClick={() => setHistoryOpen(true)}>
                <HistoryOutlinedIcon />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('cubebox_settings')}>
            <span>
              <IconButton aria-label={t('cubebox_settings')} onClick={() => setSettingsOpen(true)}>
                <SettingsOutlinedIcon />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('cubebox_new_chat')}>
            <span>
              <IconButton
                aria-label={t('cubebox_new_chat')}
                disabled={state.loading || state.turnStatus === 'streaming'}
                onClick={() => void startNewConversation()}
              >
                <AddCommentOutlinedIcon />
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
            disabled={state.loading}
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
              <Button
                color='warning'
                disabled={state.turnStatus !== 'streaming'}
                onClick={() => void interrupt()}
                startIcon={<StopCircleOutlinedIcon />}
                variant='outlined'
              >
                {t('cubebox_stop')}
              </Button>
              <Button
                disabled={state.loading || state.composerText.trim().length === 0 || state.turnStatus === 'streaming'}
                onClick={() => void sendMessage()}
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
                        <IconButton edge='end' size='small' onClick={() => void archiveConversation(conversation.id, !conversation.archived)}>
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
          <Typography color='text.secondary' variant='body2'>
            {t('cubebox_settings_placeholder')}
          </Typography>
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
