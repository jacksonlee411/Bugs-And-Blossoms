import { Alert, Card, CardContent, Chip, Stack, Typography } from '@mui/material'
import { useEffect, useState } from 'react'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { getCubeBoxModels, getCubeBoxRuntimeStatus, type CubeBoxRuntimeStatusResponse } from '../../api/cubebox'
import { cubeBoxErrorMessage } from './errorMessage'

function healthColor(value: string): 'default' | 'success' | 'warning' | 'error' {
  if (value === 'healthy') return 'success'
  if (value === 'degraded') return 'warning'
  if (value === 'unavailable') return 'error'
  return 'default'
}

export function CubeBoxModelsPage() {
  const { locale, t } = useAppPreferences()
  const [models, setModels] = useState<Array<{ provider: string; model: string }>>([])
  const [runtimeStatus, setRuntimeStatus] = useState<CubeBoxRuntimeStatusResponse | null>(null)
  const [modelsErrorMessage, setModelsErrorMessage] = useState('')
  const [runtimeErrorMessage, setRuntimeErrorMessage] = useState('')

  useEffect(() => {
    let active = true
    void (async () => {
      const [modelsResult, runtimeResult] = await Promise.allSettled([getCubeBoxModels(), getCubeBoxRuntimeStatus()])
      if (!active) {
        return
      }

      if (modelsResult.status === 'fulfilled') {
        setModels(modelsResult.value.models)
        setModelsErrorMessage('')
      } else {
        setModels([])
        setModelsErrorMessage(cubeBoxErrorMessage(modelsResult.reason, t('cubebox_error_models_load'), locale))
      }

      if (runtimeResult.status === 'fulfilled') {
        setRuntimeStatus(runtimeResult.value)
        setRuntimeErrorMessage('')
      } else {
        setRuntimeStatus(null)
        setRuntimeErrorMessage(cubeBoxErrorMessage(runtimeResult.reason, t('cubebox_error_runtime_load'), locale))
      }
    })()
    return () => {
      active = false
    }
  }, [locale, t])

  return (
    <Stack spacing={2}>
      <Typography variant='h5'>{t('cubebox_models_title')}</Typography>
      <Typography color='text.secondary' variant='body2'>
        {t('cubebox_models_subtitle')}
      </Typography>
      {runtimeErrorMessage ? <Alert severity='warning'>{runtimeErrorMessage}</Alert> : null}
      {modelsErrorMessage ? <Alert severity='warning'>{modelsErrorMessage}</Alert> : null}

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
                {t('cubebox_models_empty')}
              </Typography>
            ) : null}
          </Stack>
        </CardContent>
      </Card>
    </Stack>
  )
}
