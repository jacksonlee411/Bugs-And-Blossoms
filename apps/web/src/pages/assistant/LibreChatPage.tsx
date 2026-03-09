import SmartToyIcon from '@mui/icons-material/SmartToy'
import { Alert, Box, Button, Card, CardContent, Stack, Typography } from '@mui/material'
import { useEffect } from 'react'

const formalEntryURL = '/app/assistant/librechat'

export function LibreChatPage() {
  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }
    window.location.replace(formalEntryURL)
  }, [])

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <SmartToyIcon color='primary' />
        <Typography variant='h5'>正在进入 LibreChat 正式入口</Typography>
        <Box sx={{ flex: 1 }} />
        <Button component='a' href='/app/assistant' variant='text'>
          返回助手日志
        </Button>
      </Stack>

      <Alert severity='info'>
        导航已统一到正式入口 `/app/assistant/librechat`；当前页面仅作为 SPA 内部跳转桥接，防止保留第二套正式入口语义。
      </Alert>

      <Card>
        <CardContent>
          <Stack spacing={1.5}>
            <Typography variant='subtitle1'>切换状态</Typography>
            <Typography color='text.secondary' variant='body2'>
              正在执行整页跳转以进入服务端正式入口。
            </Typography>
            <Typography color='text.secondary' variant='body2'>
              若浏览器未自动跳转，请点击下方按钮手动进入正式入口。
            </Typography>
            <Button component='a' href={formalEntryURL} variant='contained'>
              打开正式入口
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Stack>
  )
}
