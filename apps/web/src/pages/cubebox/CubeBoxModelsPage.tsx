import { Alert, Card, CardContent, Chip, Stack, Typography } from '@mui/material'
import { useEffect, useState } from 'react'
import { getCubeBoxModels, getCubeBoxRuntimeStatus, type CubeBoxRuntimeStatusResponse } from '../../api/cubebox'

function healthColor(value: string): 'default' | 'success' | 'warning' | 'error' {
  if (value === 'healthy') return 'success'
  if (value === 'degraded') return 'warning'
  if (value === 'unavailable') return 'error'
  return 'default'
}

function messageForError(error: unknown, fallback: string): string {
  const message = (error as { message?: string })?.message
  if (typeof message === 'string' && message.trim().length > 0) {
    return message
  }
  return fallback
}

export function CubeBoxModelsPage() {
  const [models, setModels] = useState<Array<{ provider: string; model: string }>>([])
  const [runtimeStatus, setRuntimeStatus] = useState<CubeBoxRuntimeStatusResponse | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const [modelsResponse, runtimeResponse] = await Promise.all([getCubeBoxModels(), getCubeBoxRuntimeStatus()])
        if (!active) {
          return
        }
        setModels(modelsResponse.models)
        setRuntimeStatus(runtimeResponse)
      } catch (error) {
        if (!active) {
          return
        }
        setErrorMessage(messageForError(error, '加载 CubeBox 模型失败'))
      }
    })()
    return () => {
      active = false
    }
  }, [])

  return (
    <Stack spacing={2}>
      <Typography variant='h5'>CubeBox 模型</Typography>
      <Typography color='text.secondary' variant='body2'>
        模型页只做只读展示，不再承接 LibreChat model provider 配置写入口。
      </Typography>
      {errorMessage ? <Alert severity='warning'>{errorMessage}</Alert> : null}

      <Card>
        <CardContent>
          <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap'>
            <Chip label={`frontend=${runtimeStatus?.frontend.healthy ?? '-'}`} size='small' variant='outlined' />
            <Chip
              color={healthColor(runtimeStatus?.model_gateway.healthy ?? '')}
              label={`model_gateway=${runtimeStatus?.model_gateway.healthy ?? '-'}`}
              size='small'
            />
            <Chip label={`knowledge=${runtimeStatus?.knowledge_runtime.healthy ?? '-'}`} size='small' variant='outlined' />
          </Stack>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Stack spacing={1}>
            {models.map((item) => (
              <Typography data-testid='cubebox-model-item' key={`${item.provider}-${item.model}`} variant='body2'>
                {item.provider}: {item.model}
              </Typography>
            ))}
            {models.length === 0 ? (
              <Typography color='text.secondary' variant='body2'>
                当前没有可用模型
              </Typography>
            ) : null}
          </Stack>
        </CardContent>
      </Card>
    </Stack>
  )
}
