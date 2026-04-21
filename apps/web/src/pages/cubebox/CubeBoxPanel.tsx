import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import StopCircleOutlinedIcon from '@mui/icons-material/StopCircleOutlined'
import SendIcon from '@mui/icons-material/Send'
import {
  Alert,
  Button,
  CircularProgress,
  Divider,
  List,
  ListItem,
  Paper,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { useCubeBox } from './CubeBoxProvider'

export function CubeBoxPanel() {
  const { state, interrupt, sendMessage, setComposerText } = useCubeBox()
  const { t } = useAppPreferences()

  return (
    <Stack spacing={2} sx={{ height: '100%' }}>
      <Stack spacing={0.5}>
        <Typography component='h2' variant='h6'>
          {t('page_cubebox_title')}
        </Typography>
        <Typography color='text.secondary' variant='body2'>
          {t('page_cubebox_subtitle')}
        </Typography>
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
    </Stack>
  )
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
