import { type FormEvent, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useMutation } from '@tanstack/react-query'
import { ApiClientError } from '../api/errors'
import { getSetIDExplain, type SetIDExplainResponse } from '../api/setids'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'

type ExplainLevel = 'brief' | 'full'

interface SetIDExplainPanelProps {
  title: string
  subtitle?: string
  initialAsOf?: string
  initialCapabilityKey?: string
  initialFieldKey?: string
  initialBusinessUnitID?: string
  initialSetID?: string
  initialOrgUnitID?: string
  defaultLevel?: ExplainLevel
  fullPermissionKey?: string
}

interface ExplainErrorView {
  code: string
  message: string
  traceID: string
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestID(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

function parseExplainError(error: unknown): ExplainErrorView {
  if (error instanceof ApiClientError) {
    const details = error.details
    if (details && typeof details === 'object') {
      const code = String(Reflect.get(details, 'code') ?? '').trim()
      const message = String(Reflect.get(details, 'message') ?? error.message).trim()
      const traceID = String(Reflect.get(details, 'trace_id') ?? '').trim()
      return {
        code: code.length > 0 ? code : 'UNKNOWN_ERROR',
        message: message.length > 0 ? message : error.message,
        traceID
      }
    }
    return {
      code: error.code,
      message: error.message,
      traceID: error.traceId ?? ''
    }
  }
  if (error instanceof Error) {
    return {
      code: 'UNKNOWN_ERROR',
      message: error.message,
      traceID: ''
    }
  }
  return {
    code: 'UNKNOWN_ERROR',
    message: String(error),
    traceID: ''
  }
}

function reasonHint(code: string): string {
  switch (code.trim().toUpperCase()) {
    case 'OWNER_CONTEXT_REQUIRED':
      return '请补齐 owner_setid / business_unit_id 后重试。'
    case 'OWNER_CONTEXT_FORBIDDEN':
      return '当前 BU 上下文无权访问该 SetID，请切换业务单元或联系管理员。'
    case 'ACTOR_SCOPE_FORBIDDEN':
      return '当前角色无法查看 full explain，请使用 brief 或申请管理员权限。'
    case 'FIELD_POLICY_MISSING':
      return '字段策略尚未登记，请到 Strategy Registry 新增 capability_key + field_key。'
    case 'FIELD_DEFAULT_RULE_MISSING':
      return '字段默认规则缺失，请补齐 default_rule_ref 或 default_value。'
    case 'FIELD_POLICY_CONFLICT':
      return '字段策略冲突（例如 required=true 且 visible=false），请修正后重试。'
    default:
      return '请复制 trace_id/request_id 给管理员排查。'
  }
}

export function SetIDExplainPanel({
  title,
  subtitle,
  initialAsOf,
  initialCapabilityKey,
  initialFieldKey,
  initialBusinessUnitID,
  initialSetID,
  initialOrgUnitID,
  defaultLevel = 'brief',
  fullPermissionKey = 'setid.explain.full'
}: SetIDExplainPanelProps) {
  const { hasPermission } = useAppPreferences()
  const canViewFull = hasPermission(fullPermissionKey)

  const [capabilityKey, setCapabilityKey] = useState(initialCapabilityKey ?? '')
  const [fieldKey, setFieldKey] = useState(initialFieldKey ?? '')
  const [businessUnitID, setBusinessUnitID] = useState(initialBusinessUnitID ?? '')
  const [asOf, setAsOf] = useState(initialAsOf ?? todayISO())
  const [setID, setSetID] = useState(initialSetID ?? '')
  const [orgUnitID, setOrgUnitID] = useState(initialOrgUnitID ?? '')
  const [requestID, setRequestID] = useState(newRequestID('mui-setid-explain'))
  const [level, setLevel] = useState<ExplainLevel>(defaultLevel)
  const [result, setResult] = useState<SetIDExplainResponse | null>(null)
  const [errorView, setErrorView] = useState<ExplainErrorView | null>(null)
  const [copyNotice, setCopyNotice] = useState<string | null>(null)

  const effectiveLevel: ExplainLevel = useMemo(() => {
    if (level === 'full' && !canViewFull) {
      return 'brief'
    }
    return level
  }, [canViewFull, level])

  const mutation = useMutation({
    mutationFn: getSetIDExplain,
    onSuccess: (payload) => {
      setResult(payload)
      setErrorView(null)
    },
    onError: (error) => {
      setResult(null)
      setErrorView(parseExplainError(error))
    }
  })

  async function onCopy(value: string, label: string) {
    if (value.trim().length === 0) {
      return
    }
    try {
      await navigator.clipboard.writeText(value)
      setCopyNotice(`已复制 ${label}`)
    } catch {
      setCopyNotice('复制失败，请手动复制')
    }
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setCopyNotice(null)
    setResult(null)
    setErrorView(null)

    try {
      await mutation.mutateAsync({
        capabilityKey,
        fieldKey,
        businessUnitID,
        asOf,
        level: effectiveLevel,
        setID,
        orgUnitID,
        requestID
      })
    } catch {
      // handled in onError
    }
  }

  return (
    <Paper component='form' onSubmit={(event) => void onSubmit(event)} sx={{ p: 2 }} variant='outlined'>
      <Stack spacing={1.5}>
        <Box>
          <Typography variant='subtitle1'>{title}</Typography>
          {subtitle ? (
            <Typography color='text.secondary' variant='body2'>
              {subtitle}
            </Typography>
          ) : null}
        </Box>

        {!canViewFull ? (
          <Alert severity='info'>当前账号仅可查看 brief explain；full explain 需要管理员权限。</Alert>
        ) : null}
        {copyNotice ? <Alert severity='success'>{copyNotice}</Alert> : null}

        <Box
          sx={{
            display: 'grid',
            gap: 1,
            gridTemplateColumns: {
              xs: '1fr',
              md: 'repeat(3, minmax(0, 1fr))'
            }
          }}
        >
          <TextField label='capability_key' required size='small' value={capabilityKey} onChange={(event) => setCapabilityKey(event.target.value)} />
          <TextField label='field_key' required size='small' value={fieldKey} onChange={(event) => setFieldKey(event.target.value)} />
          <TextField
            label='business_unit_id'
            required
            size='small'
            value={businessUnitID}
            onChange={(event) => setBusinessUnitID(event.target.value)}
          />
          <TextField label='as_of' required size='small' type='date' value={asOf} onChange={(event) => setAsOf(event.target.value)} />
          <TextField label='request_id' required size='small' value={requestID} onChange={(event) => setRequestID(event.target.value)} />
          <TextField label='setid（可选）' size='small' value={setID} onChange={(event) => setSetID(event.target.value)} />
          <TextField label='org_unit_id（可选）' size='small' value={orgUnitID} onChange={(event) => setOrgUnitID(event.target.value)} />
          <Stack spacing={0.5}>
            <Typography color='text.secondary' variant='caption'>
              explain level
            </Typography>
            <Select
              disabled={!canViewFull}
              size='small'
              value={effectiveLevel}
              onChange={(event) => setLevel(event.target.value as ExplainLevel)}
            >
              <MenuItem value='brief'>brief</MenuItem>
              <MenuItem value='full'>full</MenuItem>
            </Select>
          </Stack>
        </Box>

        <Stack direction='row' spacing={1}>
          <Button disabled={mutation.isPending} type='submit' variant='contained'>
            获取命中解释
          </Button>
          <Button
            onClick={() => {
              setResult(null)
              setErrorView(null)
            }}
            variant='outlined'
          >
            清空结果
          </Button>
        </Stack>

        {errorView ? (
          <>
            <Alert severity='error'>
              {errorView.message}（{errorView.code}）
            </Alert>
            <Paper sx={{ p: 1.5 }} variant='outlined'>
              <Typography variant='subtitle2'>下一步建议</Typography>
              <Typography color='text.secondary' variant='body2'>
                {reasonHint(errorView.code)}
              </Typography>
              <Stack direction='row' spacing={1} sx={{ mt: 1 }}>
                <Button onClick={() => void onCopy(errorView.traceID, 'trace_id')} size='small' variant='text'>
                  复制 trace_id
                </Button>
                <Button onClick={() => void onCopy(requestID, 'request_id')} size='small' variant='text'>
                  复制 request_id
                </Button>
              </Stack>
            </Paper>
          </>
        ) : null}

        {result ? (
          <Paper sx={{ p: 1.5 }} variant='outlined'>
            <Stack direction='row' flexWrap='wrap' gap={1} sx={{ mb: 1 }}>
              <Chip
                color={result.decision === 'allow' ? 'success' : 'warning'}
                label={`decision: ${result.decision}`}
                size='small'
                variant='outlined'
              />
              <Chip label={`reason_code: ${result.reason_code || '-'}`} size='small' variant='outlined' />
              <Chip label={`resolved_setid: ${result.resolved_setid}`} size='small' variant='outlined' />
              <Chip label={`resolved_config_version: ${result.resolved_config_version || '-'}`} size='small' variant='outlined' />
              <Chip label={`level: ${result.level}`} size='small' variant='outlined' />
            </Stack>

            <Typography color='text.secondary' variant='body2'>
              trace_id: {result.trace_id || '-'} · request_id: {result.request_id || requestID}
            </Typography>
            <Stack direction='row' spacing={1} sx={{ mt: 0.5 }}>
              <Button onClick={() => void onCopy(result.trace_id || '', 'trace_id')} size='small' variant='text'>
                复制 trace_id
              </Button>
              <Button onClick={() => void onCopy(result.request_id || requestID, 'request_id')} size='small' variant='text'>
                复制 request_id
              </Button>
            </Stack>

            <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mt: 1.5, overflow: 'auto' }}>
              <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                <thead>
                  <tr style={{ background: '#fff' }}>
                    <th align='left'>field_key</th>
                    <th align='left'>required</th>
                    <th align='left'>visible</th>
                    <th align='left'>default_rule_ref</th>
                    <th align='left'>resolved_default_value</th>
                    <th align='left'>decision</th>
                    <th align='left'>reason_code</th>
                  </tr>
                </thead>
                <tbody>
                  {result.field_decisions.map((decision) => (
                    <tr key={`${decision.capability_key}:${decision.field_key}`} style={{ borderTop: '1px solid #eee' }}>
                      <td>{decision.field_key}</td>
                      <td>{decision.required ? 'true' : 'false'}</td>
                      <td>{decision.visible ? 'true' : 'false'}</td>
                      <td>{decision.default_rule_ref || '-'}</td>
                      <td>{decision.resolved_default_value || '-'}</td>
                      <td>{decision.decision}</td>
                      <td>{decision.reason_code || '-'}</td>
                    </tr>
                  ))}
                  {result.field_decisions.length === 0 ? (
                    <tr>
                      <td colSpan={7} style={{ padding: 16, textAlign: 'center' }}>
                        无字段判定结果
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </Box>
          </Paper>
        ) : null}
      </Stack>
    </Paper>
  )
}
