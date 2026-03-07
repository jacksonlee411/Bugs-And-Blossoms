import SmartToyIcon from '@mui/icons-material/SmartToy'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  List,
  ListItem,
  ListItemText,
  Stack,
  Typography
} from '@mui/material'
import { useEffect, useState } from 'react'
import {
  getAssistantRuntimeStatus,
  listAssistantConversations,
  type AssistantConversationListItem,
  type AssistantRuntimeStatusResponse
} from '../../api/assistant'

function errorMessage(err: unknown, fallback: string): string {
  const message = (err as { message?: string })?.message
  if (typeof message === 'string' && message.trim().length > 0) {
    return message
  }
  return fallback
}

function statusColor(status: string): 'success' | 'warning' | 'error' | 'default' {
  switch ((status || '').trim()) {
    case 'healthy':
      return 'success'
    case 'degraded':
      return 'warning'
    case 'unavailable':
      return 'error'
    default:
      return 'default'
  }
}

function conversationSummary(item: AssistantConversationListItem): string {
  const lastTurn = item.last_turn
  if (!lastTurn) {
    return '暂无轮次记录'
  }
  return `${lastTurn.state} / ${lastTurn.risk_tier} / ${lastTurn.user_input}`
}

export function AssistantPage() {
  const [runtimeStatus, setRuntimeStatus] = useState<AssistantRuntimeStatusResponse | null>(null)
  const [conversations, setConversations] = useState<AssistantConversationListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [pageError, setPageError] = useState('')

  useEffect(() => {
    let active = true
    void (async () => {
      setLoading(true)
      setPageError('')
      try {
        const [runtimeResponse, conversationResponse] = await Promise.all([
          getAssistantRuntimeStatus(),
          listAssistantConversations({ page_size: 10 })
        ])
        if (!active) {
          return
        }
        setRuntimeStatus(runtimeResponse)
        setConversations(Array.isArray(conversationResponse.items) ? conversationResponse.items : [])
      } catch (err) {
        if (!active) {
          return
        }
        setPageError(errorMessage(err, '助手日志加载失败'))
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    })()
    return () => {
      active = false
    }
  }, [])

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <SmartToyIcon color='primary' />
        <Typography variant='h5'>AI 助手日志</Typography>
        <Box sx={{ flex: 1 }} />
        <Button component='a' href='/app/assistant/librechat' variant='contained'>
          打开 LibreChat
        </Button>
        <Button component='a' href='/app/assistant/models' variant='text'>
          模型配置
        </Button>
      </Stack>

      <Typography color='text.secondary' variant='body2'>
        `/app/assistant` 仅保留运行态、会话与审计记录；旧 `iframe + bridge` 对话承载页已按 `DEV-PLAN-282` 退役。
      </Typography>

      <Alert severity='info'>正式交互入口已统一到 `/app/assistant/librechat`；本页不再承担正式聊天交互与验收职责。</Alert>

      {pageError ? <Alert severity='warning'>{pageError}</Alert> : null}

      <Card>
        <CardContent>
          <Stack spacing={1.5}>
            <Stack alignItems='center' direction='row' spacing={1}>
              <Typography variant='subtitle1'>运行态</Typography>
              <Chip
                color={statusColor(runtimeStatus?.status ?? '')}
                data-testid='assistant-runtime-status'
                label={runtimeStatus?.status ?? (loading ? 'loading' : '-')}
                size='small'
              />
            </Stack>
            <Typography data-testid='assistant-runtime-checked-at' variant='body2'>
              checked_at: {runtimeStatus?.checked_at ?? '-'}
            </Typography>
            <Typography data-testid='assistant-runtime-upstream-url' variant='body2'>
              upstream: {runtimeStatus?.upstream?.url ?? '-'}
            </Typography>
            <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap'>
              <Chip
                label={`actions=${runtimeStatus?.capabilities?.actions_enabled ? 'on' : 'off'}`}
                size='small'
                variant='outlined'
              />
              <Chip
                label={`agents_write=${runtimeStatus?.capabilities?.agents_write_enabled ? 'on' : 'off'}`}
                size='small'
                variant='outlined'
              />
              <Chip
                label={`mcp=${runtimeStatus?.capabilities?.mcp_enabled ? 'on' : 'off'}`}
                size='small'
                variant='outlined'
              />
            </Stack>
            <Divider />
            <Stack spacing={1}>
              <Typography variant='subtitle2'>依赖服务</Typography>
              <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap'>
                {(runtimeStatus?.services ?? []).map((service) => (
                  <Chip
                    color={statusColor(service.healthy)}
                    key={service.name}
                    label={`${service.name}:${service.healthy}`}
                    size='small'
                    variant='outlined'
                  />
                ))}
                {!runtimeStatus?.services?.length ? <Typography variant='body2'>暂无服务记录</Typography> : null}
              </Stack>
            </Stack>
          </Stack>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Stack spacing={1.5}>
            <Typography variant='subtitle1'>最近会话日志</Typography>
            <List disablePadding>
              {conversations.map((item) => (
                <ListItem data-testid='assistant-conversation-log-item' divider key={item.conversation_id} disableGutters>
                  <ListItemText
                    primary={`${item.conversation_id} · ${item.state}`}
                    secondary={`${item.updated_at} · ${conversationSummary(item)}`}
                  />
                </ListItem>
              ))}
              {!conversations.length ? <Typography variant='body2'>暂无会话日志</Typography> : null}
            </List>
          </Stack>
        </CardContent>
      </Card>
    </Stack>
  )
}
