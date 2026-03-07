import SmartToyIcon from '@mui/icons-material/SmartToy'
import { Alert, Box, Button, Card, CardContent, Stack, Typography } from '@mui/material'

export function LibreChatPage() {
  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <SmartToyIcon color='primary' />
        <Typography variant='h5'>LibreChat 旧桥接入口已下线</Typography>
        <Box sx={{ flex: 1 }} />
        <Button component='a' href='/app/assistant' variant='text'>
          返回助手日志
        </Button>
      </Stack>

      <Alert severity='info'>
        `iframe`、`bridge.js`、HTML 注入与页面级业务编排职责已按 `DEV-PLAN-282` 退役；该页面不再承担正式对话交互职责。
      </Alert>

      <Card>
        <CardContent>
          <Stack spacing={1.5}>
            <Typography variant='subtitle1'>当前状态</Typography>
            <Typography color='text.secondary' variant='body2'>
              `/app/assistant/librechat` 仅保留历史入口占位，用于阻断旧桥接链路继续承担正式职责。
            </Typography>
            <Typography color='text.secondary' variant='body2'>
              正式入口切换与用户可见交互恢复由 `DEV-PLAN-283` 承接；在该计划完成前，不再通过本页承载旧 `iframe + postMessage` 方案。
            </Typography>
          </Stack>
        </CardContent>
      </Card>
    </Stack>
  )
}
