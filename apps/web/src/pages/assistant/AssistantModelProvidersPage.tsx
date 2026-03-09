import RefreshIcon from '@mui/icons-material/Refresh'
import VerifiedIcon from '@mui/icons-material/Verified'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  FormControlLabel,
  IconButton,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  getAssistantModelProviders,
  getAssistantModels,
  type AssistantModelConfigPayload,
  type AssistantModelProvider,
  type AssistantModelProvidersValidateResponse,
  validateAssistantModelProviders
} from '../../api/assistant'

function normalizePayload(payload: AssistantModelConfigPayload): AssistantModelConfigPayload {
  return {
    provider_routing: {
      strategy: payload.provider_routing.strategy,
      fallback_enabled: payload.provider_routing.fallback_enabled
    },
    providers: payload.providers.map((provider) => ({ ...provider }))
  }
}

function healthColor(status: string): 'default' | 'success' | 'warning' | 'error' {
  if (status === 'healthy') return 'success'
  if (status === 'degraded') return 'warning'
  if (status === 'unavailable') return 'error'
  return 'default'
}

export function AssistantModelProvidersPage() {
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [successMessage, setSuccessMessage] = useState('')
  const [payload, setPayload] = useState<AssistantModelConfigPayload | null>(null)
  const [healthByProvider, setHealthByProvider] = useState<Record<string, { healthy: string; reason?: string }>>({})
  const [availableModels, setAvailableModels] = useState<Array<{ provider: string; model: string }>>([])
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    setErrorMessage('')
    try {
      const [providersResp, modelsResp] = await Promise.all([getAssistantModelProviders(), getAssistantModels()])
      setPayload({
        provider_routing: providersResp.provider_routing,
        providers: providersResp.providers.map((provider) => ({
          name: provider.name,
          enabled: provider.enabled,
          model: provider.model,
          endpoint: provider.endpoint,
          timeout_ms: provider.timeout_ms,
          retries: provider.retries,
          priority: provider.priority,
          key_ref: provider.key_ref
        }))
      })
      const health: Record<string, { healthy: string; reason?: string }> = {}
      providersResp.providers.forEach((provider) => {
        health[provider.name] = { healthy: provider.healthy, reason: provider.health_reason }
      })
      setHealthByProvider(health)
      setAvailableModels(modelsResp.models)
    } catch (error) {
      const message = error instanceof Error ? error.message : '加载模型配置失败'
      setErrorMessage(message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const providers = payload?.providers ?? []

  const updateProvider = useCallback((name: string, next: Partial<AssistantModelProvider>) => {
    setPayload((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        providers: prev.providers.map((provider) =>
          provider.name === name
            ? {
                ...provider,
                ...next
              }
            : provider
        )
      }
    })
  }, [])

  const canOperate = useMemo(() => !loading && payload !== null, [loading, payload])

  const onValidate = useCallback(async () => {
    if (!payload) return
    setLoading(true)
    setErrorMessage('')
    setSuccessMessage('')
    try {
      const resp: AssistantModelProvidersValidateResponse = await validateAssistantModelProviders(payload)
      setValidationErrors(resp.errors ?? [])
      setPayload(normalizePayload(resp.normalized))
      if (resp.valid) {
        setSuccessMessage('模型配置校验通过。')
      } else {
        setErrorMessage('模型配置校验未通过，请修正后重试。')
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : '模型配置校验失败'
      setErrorMessage(message)
    } finally {
      setLoading(false)
    }
  }, [payload])

  return (
    <Stack spacing={2}>
      <Stack direction='row' spacing={1} alignItems='center'>
        <Typography variant='h5'>模型治理配置</Typography>
        <IconButton aria-label='refresh-model-providers' disabled={loading} onClick={() => void load()} size='small'>
          <RefreshIcon fontSize='small' />
        </IconButton>
        <Box sx={{ flex: 1 }} />
        <Button component='a' href='/app/assistant/librechat' variant='text'>
          返回 LibreChat
        </Button>
      </Stack>

      <Typography color='text.secondary' variant='body2'>
        本页面仅提供模型路由只读展示与校验，不提供配置写入入口（单主源：LibreChat）。
      </Typography>

      {errorMessage ? <Alert severity='error'>{errorMessage}</Alert> : null}
      {successMessage ? <Alert severity='success'>{successMessage}</Alert> : null}
      {validationErrors.length > 0 ? (
        <Alert severity='warning'>
          <Stack component='ul' sx={{ m: 0, pl: 2 }} spacing={0.5}>
            {validationErrors.map((item) => (
              <Typography component='li' key={item} variant='body2'>
                {item}
              </Typography>
            ))}
          </Stack>
        </Alert>
      ) : null}

      <Card>
        <CardContent>
          <Stack direction='row' spacing={2} alignItems='center'>
            <TextField
              label='Routing Strategy'
              size='small'
              value={payload?.provider_routing.strategy ?? 'priority_failover'}
              onChange={(event) =>
                setPayload((prev) =>
                  prev
                    ? {
                        ...prev,
                        provider_routing: {
                          ...prev.provider_routing,
                          strategy: event.target.value
                        }
                      }
                    : prev
                )
              }
              disabled={!canOperate}
            />
            <FormControlLabel
              control={
                <Switch
                  checked={payload?.provider_routing.fallback_enabled ?? true}
                  onChange={(event) =>
                    setPayload((prev) =>
                      prev
                        ? {
                            ...prev,
                            provider_routing: {
                              ...prev.provider_routing,
                              fallback_enabled: event.target.checked
                            }
                          }
                        : prev
                    )
                  }
                  disabled={!canOperate}
                />
              }
              label='Enable Fallback'
            />
            <Box sx={{ flex: 1 }} />
            <Button
              disabled={!canOperate}
              onClick={() => void onValidate()}
              startIcon={<VerifiedIcon />}
              variant='outlined'
            >
              Validate
            </Button>
          </Stack>
        </CardContent>
      </Card>

      <Stack spacing={2}>
        {providers.map((provider) => {
          const health = healthByProvider[provider.name]
          return (
            <Card key={provider.name} variant='outlined'>
              <CardContent>
                <Stack spacing={1.5}>
                  <Stack direction='row' spacing={1} alignItems='center'>
                    <Typography variant='subtitle1'>{provider.name}</Typography>
                    <Chip
                      size='small'
                      label={health?.healthy ?? 'unknown'}
                      color={healthColor(health?.healthy ?? 'unknown')}
                    />
                    {health?.reason ? <Typography color='text.secondary' variant='caption'>{health.reason}</Typography> : null}
                  </Stack>
                  <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
                    <FormControlLabel
                      control={
                        <Switch
                          checked={provider.enabled}
                          onChange={(event) => updateProvider(provider.name, { enabled: event.target.checked })}
                          disabled={!canOperate}
                        />
                      }
                      label='Enabled'
                    />
                    <TextField
                      label='Model'
                      size='small'
                      value={provider.model}
                      onChange={(event) => updateProvider(provider.name, { model: event.target.value })}
                      disabled={!canOperate}
                    />
                    <TextField
                      label='Endpoint'
                      size='small'
                      value={provider.endpoint}
                      onChange={(event) => updateProvider(provider.name, { endpoint: event.target.value })}
                      disabled={!canOperate}
                      sx={{ minWidth: 260 }}
                    />
                    <TextField
                      label='Timeout(ms)'
                      size='small'
                      type='number'
                      value={provider.timeout_ms}
                      onChange={(event) => updateProvider(provider.name, { timeout_ms: Number(event.target.value) })}
                      disabled={!canOperate}
                    />
                    <TextField
                      label='Retries'
                      size='small'
                      type='number'
                      value={provider.retries}
                      onChange={(event) => updateProvider(provider.name, { retries: Number(event.target.value) })}
                      disabled={!canOperate}
                    />
                    <TextField
                      label='Priority'
                      size='small'
                      type='number'
                      value={provider.priority}
                      onChange={(event) => updateProvider(provider.name, { priority: Number(event.target.value) })}
                      disabled={!canOperate}
                    />
                    <TextField
                      label='Key Ref'
                      size='small'
                      value={provider.key_ref}
                      onChange={(event) => updateProvider(provider.name, { key_ref: event.target.value })}
                      disabled={!canOperate}
                      sx={{ minWidth: 180 }}
                    />
                  </Stack>
                </Stack>
              </CardContent>
            </Card>
          )
        })}
      </Stack>

      <Card variant='outlined'>
        <CardContent>
          <Typography gutterBottom variant='subtitle2'>可用模型清单</Typography>
          <Stack spacing={0.5}>
            {availableModels.length === 0 ? (
              <Typography color='text.secondary' variant='body2'>当前无可用模型。</Typography>
            ) : (
              availableModels.map((item) => (
                <Typography key={`${item.provider}-${item.model}`} variant='body2'>
                  {item.provider}: {item.model}
                </Typography>
              ))
            )}
          </Stack>
        </CardContent>
      </Card>
    </Stack>
  )
}
