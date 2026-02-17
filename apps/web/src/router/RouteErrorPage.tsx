import { useRouteError } from 'react-router-dom'
import { Alert, Box, Button, Stack, Typography } from '@mui/material'

function toErrorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  try {
    return JSON.stringify(err)
  } catch {
    return 'Unknown error'
  }
}

export function RouteErrorPage() {
  const error = useRouteError()
  const message = toErrorMessage(error)

  return (
    <Box sx={{ p: 3 }}>
      <Stack spacing={2}>
        <Typography variant='h6' component='h1'>
          页面发生错误
        </Typography>
        <Alert severity='error'>{message}</Alert>
        <Box>
          <Button variant='contained' onClick={() => window.location.assign('/app/')}>
            返回首页
          </Button>
        </Box>
      </Stack>
    </Box>
  )
}

