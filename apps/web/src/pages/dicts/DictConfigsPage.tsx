import { type FormEvent, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createDict,
  createDictValue,
  disableDict,
  executeDictRelease,
  listDicts,
  listDictValues,
  previewDictRelease,
  type DictReleasePreviewResponse,
  type DictReleaseResultResponse
} from '../../api/dicts'
import { ApiClientError } from '../../api/errors'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { type MessageKey } from '../../i18n/messages'
import { PageHeader } from '../../components/PageHeader'
import {
  GLOBAL_TENANT_ID,
  nextStageAfterPreview,
  toMaxConflicts,
  validatePreviewForm,
  validateReleaseForm,
  type DictReleaseFormValues,
  type DictReleaseStage,
  type DictReleaseValidationIssue
} from './dictReleaseFlow'

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestID(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

function parseApiError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function statusColor(status: string): 'success' | 'default' {
  return status.trim().toLowerCase() === 'active' ? 'success' : 'default'
}

function releaseStageColor(stage: DictReleaseStage): 'default' | 'info' | 'warning' | 'success' | 'error' {
  switch (stage) {
    case 'previewing':
    case 'releasing':
      return 'info'
    case 'ready':
      return 'success'
    case 'conflict':
      return 'warning'
    case 'fail':
      return 'error'
    default:
      return 'default'
  }
}

function releaseStageLabelKey(stage: DictReleaseStage): MessageKey {
  switch (stage) {
    case 'previewing':
      return 'dict_release_stage_previewing'
    case 'conflict':
      return 'dict_release_stage_conflict'
    case 'ready':
      return 'dict_release_stage_ready'
    case 'releasing':
      return 'dict_release_stage_releasing'
    case 'success':
      return 'dict_release_stage_success'
    case 'fail':
      return 'dict_release_stage_fail'
    default:
      return 'dict_release_stage_idle'
  }
}

function releaseErrorMessageKey(code: string): MessageKey | null {
  switch (code) {
    case 'invalid_as_of':
      return 'dict_release_error_code_invalid_as_of'
    case 'forbidden':
      return 'dict_release_error_code_forbidden'
    case 'dict_baseline_not_ready':
      return 'dict_release_error_code_baseline_not_ready'
    case 'dict_release_id_required':
      return 'dict_release_error_code_release_id_required'
    case 'dict_release_source_invalid':
      return 'dict_release_error_code_source_invalid'
    case 'dict_release_target_required':
      return 'dict_release_error_code_target_required'
    case 'invalid_request':
      return 'dict_release_error_code_invalid_request'
    case 'dict_value_conflict':
      return 'dict_release_error_code_value_conflict'
    case 'dict_code_conflict':
      return 'dict_release_error_code_dict_conflict'
    case 'dict_release_payload_invalid':
      return 'dict_release_error_code_payload_invalid'
    default:
      return null
  }
}

function extractApiErrorCode(error: unknown): string | null {
  if (!(error instanceof ApiClientError)) {
    return null
  }
  const details = error.details
  if (!details || typeof details !== 'object') {
    return null
  }
  const code = Reflect.get(details, 'code')
  if (typeof code !== 'string' || code.trim().length === 0) {
    return null
  }
  return code.trim()
}

export function DictConfigsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { hasPermission, t } = useAppPreferences()

  const [asOf, setAsOf] = useState(todayISO())
  const [keyword, setKeyword] = useState('')
  const [selectedDictCode, setSelectedDictCode] = useState('')
  const [error, setError] = useState<string | null>(null)

  const [createDictOpen, setCreateDictOpen] = useState(false)
  const [createValueOpen, setCreateValueOpen] = useState(false)

  const [createDictCode, setCreateDictCode] = useState('')
  const [createDictName, setCreateDictName] = useState('')
  const [createDictEnabledOn, setCreateDictEnabledOn] = useState(todayISO())
  const [disableDictDay, setDisableDictDay] = useState(todayISO())

  const [createValueCode, setCreateValueCode] = useState('')
  const [createValueLabel, setCreateValueLabel] = useState('')
  const [createValueEnabledOn, setCreateValueEnabledOn] = useState(todayISO())

  const [releaseForm, setReleaseForm] = useState<DictReleaseFormValues>(() => ({
    sourceTenantID: GLOBAL_TENANT_ID,
    asOf: '',
    releaseID: '',
    requestID: newRequestID('mui-dict-release'),
    maxConflicts: '200'
  }))
  const [releaseStage, setReleaseStage] = useState<DictReleaseStage>('idle')
  const [releasePreview, setReleasePreview] = useState<DictReleasePreviewResponse | null>(null)
  const [releaseResult, setReleaseResult] = useState<DictReleaseResultResponse | null>(null)
  const [releaseError, setReleaseError] = useState<string | null>(null)
  const [releaseErrorCode, setReleaseErrorCode] = useState<string | null>(null)
  const [releaseNotice, setReleaseNotice] = useState<string | null>(null)

  const canDictRelease = hasPermission('dict.release.admin')
  const releaseBusy = releaseStage === 'previewing' || releaseStage === 'releasing'

  const dictsQuery = useQuery({
    queryKey: ['dicts', asOf],
    queryFn: () => listDicts(asOf),
    staleTime: 10_000
  })

  const dicts = useMemo(() => dictsQuery.data?.dicts ?? [], [dictsQuery.data])

  const effectiveSelectedDictCode = useMemo(() => {
    if (dicts.length === 0) {
      return ''
    }
    const current = selectedDictCode.trim()
    if (current.length > 0 && dicts.some((item) => item.dict_code === current)) {
      return current
    }
    return dicts[0]?.dict_code ?? ''
  }, [dicts, selectedDictCode])

  const valuesQuery = useQuery({
    enabled: effectiveSelectedDictCode.trim().length > 0,
    queryKey: ['dict-values', effectiveSelectedDictCode, asOf, keyword],
    queryFn: () =>
      listDictValues({
        dictCode: effectiveSelectedDictCode,
        asOf,
        q: keyword,
        status: 'all',
        limit: 50
      }),
    staleTime: 5_000
  })

  const values = useMemo(() => valuesQuery.data?.values ?? [], [valuesQuery.data])

  const createDictMutation = useMutation({
    mutationFn: (request: { dict_code: string; name: string; enabled_on: string; request_id: string }) => createDict(request),
    onSuccess: async (result) => {
      setSelectedDictCode(result.dict_code)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dicts', asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-values', result.dict_code, asOf] })
      ])
    }
  })

  const disableDictMutation = useMutation({
    mutationFn: (request: { dict_code: string; disabled_on: string; request_id: string }) => disableDict(request),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dicts', asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-values', effectiveSelectedDictCode, asOf] })
      ])
    }
  })

  const createValueMutation = useMutation({
    mutationFn: (request: { dict_code: string; code: string; label: string; enabled_on: string; request_id: string }) =>
      createDictValue(request),
    onSuccess: async (_, variables) => {
      await queryClient.invalidateQueries({ queryKey: ['dict-values', effectiveSelectedDictCode, asOf] })
      navigate({
        pathname: `/dicts/${variables.dict_code}/values/${encodeURIComponent(variables.code)}`,
        search: `?as_of=${asOf}`
      })
    }
  })

  const previewReleaseMutation = useMutation({
    mutationFn: (request: { source_tenant_id: string; as_of: string; release_id: string; max_conflicts: number }) =>
      previewDictRelease(request),
    onSuccess: (preview) => {
      setReleasePreview(preview)
      setReleaseResult(null)
      setReleaseError(null)
      setReleaseErrorCode(null)
      setReleaseNotice(null)
      setReleaseStage(nextStageAfterPreview(preview))
    },
    onError: (mutationError) => {
      const code = extractApiErrorCode(mutationError)
      const messageKey = code ? releaseErrorMessageKey(code) : null
      setReleaseError(messageKey ? t(messageKey) : parseApiError(mutationError))
      setReleaseErrorCode(code)
      setReleaseNotice(null)
      setReleaseStage('fail')
    }
  })

  const executeReleaseMutation = useMutation({
    mutationFn: (request: {
      source_tenant_id: string
      as_of: string
      release_id: string
      request_id: string
      max_conflicts: number
    }) => executeDictRelease(request),
    onSuccess: (result) => {
      setReleaseResult(result)
      setReleaseError(null)
      setReleaseErrorCode(null)
      setReleaseNotice(null)
      setReleaseStage('success')
    },
    onError: (mutationError) => {
      const code = extractApiErrorCode(mutationError)
      const messageKey = code ? releaseErrorMessageKey(code) : null
      setReleaseError(messageKey ? t(messageKey) : parseApiError(mutationError))
      setReleaseErrorCode(code)
      setReleaseNotice(null)
      setReleaseStage('fail')
    }
  })

  function onReleaseFieldChange<K extends keyof DictReleaseFormValues>(field: K, value: DictReleaseFormValues[K]) {
    setReleaseForm((previous) => ({ ...previous, [field]: value }))
    if (releaseStage !== 'previewing' && releaseStage !== 'releasing') {
      setReleaseStage('idle')
      setReleasePreview(null)
      setReleaseResult(null)
      setReleaseError(null)
      setReleaseErrorCode(null)
      setReleaseNotice(null)
    }
  }

  function formatValidationIssues(issues: DictReleaseValidationIssue[]): string {
    return issues.map((issue) => t(issue)).join('；')
  }

  async function onPreviewRelease(event: FormEvent) {
    event.preventDefault()
    const issues = validatePreviewForm(releaseForm)
    if (issues.length > 0) {
      setReleaseStage('fail')
      setReleaseError(formatValidationIssues(issues))
      setReleaseErrorCode(null)
      setReleaseNotice(null)
      return
    }

    setReleaseStage('previewing')
    setReleaseError(null)
    setReleaseErrorCode(null)
    setReleaseNotice(null)

    try {
      await previewReleaseMutation.mutateAsync({
        source_tenant_id: releaseForm.sourceTenantID.trim(),
        as_of: releaseForm.asOf.trim(),
        release_id: releaseForm.releaseID.trim(),
        max_conflicts: toMaxConflicts(releaseForm.maxConflicts)
      })
    } catch {
      // handled by mutation onError
    }
  }

  async function onExecuteRelease(event: FormEvent) {
    event.preventDefault()
    if (releaseStage !== 'ready') {
      setReleaseStage('fail')
      setReleaseError(t('dict_release_error_preview_required'))
      setReleaseErrorCode(null)
      setReleaseNotice(null)
      return
    }

    const issues = validateReleaseForm(releaseForm)
    if (issues.length > 0) {
      setReleaseStage('fail')
      setReleaseError(formatValidationIssues(issues))
      setReleaseErrorCode(null)
      setReleaseNotice(null)
      return
    }

    setReleaseStage('releasing')
    setReleaseError(null)
    setReleaseErrorCode(null)
    setReleaseNotice(null)

    try {
      await executeReleaseMutation.mutateAsync({
        source_tenant_id: releaseForm.sourceTenantID.trim(),
        as_of: releaseForm.asOf.trim(),
        release_id: releaseForm.releaseID.trim(),
        request_id: releaseForm.requestID.trim(),
        max_conflicts: toMaxConflicts(releaseForm.maxConflicts)
      })
    } catch {
      // handled by mutation onError
    }
  }

  function onResetRelease() {
    setReleaseForm({
      sourceTenantID: GLOBAL_TENANT_ID,
      asOf: '',
      releaseID: '',
      requestID: newRequestID('mui-dict-release'),
      maxConflicts: '200'
    })
    setReleaseStage('idle')
    setReleasePreview(null)
    setReleaseResult(null)
    setReleaseError(null)
    setReleaseErrorCode(null)
    setReleaseNotice(null)
  }

  async function onCopyField(value: string, fieldLabel: string) {
    if (!value.trim()) {
      return
    }
    try {
      await navigator.clipboard.writeText(value)
      setReleaseNotice(t('dict_release_copy_success', { field: fieldLabel }))
    } catch {
      setReleaseError(t('dict_release_copy_failed'))
      setReleaseErrorCode(null)
      setReleaseStage('fail')
    }
  }

  async function onCreateDict(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await createDictMutation.mutateAsync({
        dict_code: createDictCode.trim().toLowerCase(),
        name: createDictName.trim(),
        enabled_on: createDictEnabledOn,
        request_id: newRequestID('mui-dict-code-create')
      })
      setCreateDictOpen(false)
      setCreateDictCode('')
      setCreateDictName('')
      setCreateDictEnabledOn(todayISO())
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  async function onDisableDict(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (effectiveSelectedDictCode.trim().length === 0) {
      setError('请先选择字典字段')
      return
    }
    try {
      await disableDictMutation.mutateAsync({
        dict_code: effectiveSelectedDictCode,
        disabled_on: disableDictDay,
        request_id: newRequestID('mui-dict-code-disable')
      })
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  async function onCreateValue(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (effectiveSelectedDictCode.trim().length === 0) {
      setError('请先选择字典字段')
      return
    }
    try {
      await createValueMutation.mutateAsync({
        dict_code: effectiveSelectedDictCode,
        code: createValueCode.trim(),
        label: createValueLabel.trim(),
        enabled_on: createValueEnabledOn,
        request_id: newRequestID('mui-dict-value-create')
      })
      setCreateValueOpen(false)
      setCreateValueCode('')
      setCreateValueLabel('')
      setCreateValueEnabledOn(todayISO())
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  return (
    <Box>
      <PageHeader
        title='字典配置'
        subtitle='左侧字典字段列表，右侧值列表（点击值进入详情页）'
        actions={
          <>
            <Button onClick={() => setCreateDictOpen(true)} size='small' variant='outlined'>
              新增字典字段
            </Button>
            <Button
              disabled={effectiveSelectedDictCode.trim().length === 0}
              onClick={() => setCreateValueOpen(true)}
              size='small'
              variant='outlined'
            >
              新增字典值
            </Button>
          </>
        }
      />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}

        <Paper sx={{ p: 1.5 }} variant='outlined'>
          <Stack alignItems='center' direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
            <TextField label='as_of' type='date' value={asOf} onChange={(event) => setAsOf(event.target.value)} />
            <TextField label='q' value={keyword} onChange={(event) => setKeyword(event.target.value)} />
            <Typography color='text.secondary' variant='body2'>
              当前字典字段数：{dicts.length}
            </Typography>
          </Stack>
        </Paper>

        <Paper component='form' onSubmit={(event) => void onPreviewRelease(event)} sx={{ p: 1.5 }} variant='outlined'>
          <Stack spacing={1.5}>
            <Stack alignItems='center' direction='row' justifyContent='space-between'>
              <Box>
                <Typography variant='subtitle2'>{t('dict_release_title')}</Typography>
                <Typography color='text.secondary' variant='body2'>
                  {t('dict_release_subtitle')}
                </Typography>
              </Box>
              <Chip color={releaseStageColor(releaseStage)} label={t(releaseStageLabelKey(releaseStage))} size='small' variant='outlined' />
            </Stack>

            {!canDictRelease ? <Alert severity='warning'>{t('dict_release_no_permission')}</Alert> : null}
            {releaseNotice ? <Alert severity='success'>{releaseNotice}</Alert> : null}
            {releaseError ? (
              <Alert severity='error'>
                {releaseError}
                {releaseErrorCode ? ` (${releaseErrorCode})` : ''}
              </Alert>
            ) : null}

            <Box
              sx={{
                display: 'grid',
                gap: 1,
                gridTemplateColumns: {
                  xs: '1fr',
                  md: 'repeat(2, minmax(0, 1fr))',
                  lg: 'repeat(3, minmax(0, 1fr))'
                }
              }}
            >
              <TextField
                disabled={releaseBusy}
                label={t('dict_release_field_source_tenant_id')}
                required
                value={releaseForm.sourceTenantID}
                onChange={(event) => onReleaseFieldChange('sourceTenantID', event.target.value)}
              />
              <TextField
                disabled={releaseBusy}
                label={t('dict_release_field_as_of')}
                required
                type='date'
                value={releaseForm.asOf}
                onChange={(event) => onReleaseFieldChange('asOf', event.target.value)}
              />
              <TextField
                disabled={releaseBusy}
                label={t('dict_release_field_release_id')}
                required
                value={releaseForm.releaseID}
                onChange={(event) => onReleaseFieldChange('releaseID', event.target.value)}
              />
              <TextField
                disabled={releaseBusy}
                label={t('dict_release_field_request_id')}
                required
                value={releaseForm.requestID}
                onChange={(event) => onReleaseFieldChange('requestID', event.target.value)}
              />
              <TextField
                disabled={releaseBusy}
                label={t('dict_release_field_max_conflicts')}
                type='number'
                value={releaseForm.maxConflicts}
                onChange={(event) => onReleaseFieldChange('maxConflicts', event.target.value)}
              />
            </Box>

            <Stack direction='row' spacing={1}>
              <Button disabled={releaseBusy || !canDictRelease} type='submit' variant='outlined'>
                {t('dict_release_action_preview')}
              </Button>
              <Button
                disabled={releaseBusy || !canDictRelease || releaseStage !== 'ready'}
                onClick={(event) => void onExecuteRelease(event)}
                variant='contained'
              >
                {t('dict_release_action_release')}
              </Button>
              <Button disabled={releaseBusy} onClick={onResetRelease} variant='text'>
                {t('dict_release_action_reset')}
              </Button>
            </Stack>

            {releaseStage === 'ready' ? <Alert severity='success'>{t('dict_release_preview_ready')}</Alert> : null}
            {releaseStage === 'conflict' ? <Alert severity='warning'>{t('dict_release_preview_conflict')}</Alert> : null}

            {releasePreview ? (
              <Stack spacing={1}>
                <Typography variant='subtitle2'>{t('dict_release_preview_summary')}</Typography>
                <Box
                  sx={{
                    display: 'grid',
                    gap: 1,
                    gridTemplateColumns: {
                      xs: '1fr',
                      md: 'repeat(2, minmax(0, 1fr))'
                    }
                  }}
                >
                  <Typography variant='body2'>
                    {t('dict_release_preview_source_dict_count')}：{releasePreview.source_dict_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_target_dict_count')}：{releasePreview.target_dict_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_source_value_count')}：{releasePreview.source_value_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_target_value_count')}：{releasePreview.target_value_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_missing_dict_count')}：{releasePreview.missing_dict_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_dict_name_mismatch_count')}：{releasePreview.dict_name_mismatch_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_missing_value_count')}：{releasePreview.missing_value_count}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_preview_value_label_mismatch_count')}：{releasePreview.value_label_mismatch_count}
                  </Typography>
                </Box>
              </Stack>
            ) : null}

            {releaseStage === 'conflict' ? (
              <Stack spacing={1}>
                <Typography variant='subtitle2'>{t('dict_release_conflicts_title')}</Typography>
                <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'auto', maxHeight: 260 }}>
                  <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                    <thead>
                      <tr style={{ background: '#fff', position: 'sticky', top: 0 }}>
                        <th align='left'>{t('dict_release_conflict_kind')}</th>
                        <th align='left'>{t('dict_release_conflict_dict_code')}</th>
                        <th align='left'>{t('dict_release_conflict_code')}</th>
                        <th align='left'>{t('dict_release_conflict_source')}</th>
                        <th align='left'>{t('dict_release_conflict_target')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {releasePreview?.conflicts.map((conflict, index) => (
                        <tr key={`${conflict.kind}:${conflict.dict_code}:${conflict.code ?? '-'}:${index}`} style={{ borderTop: '1px solid #eee' }}>
                          <td>{conflict.kind}</td>
                          <td>{conflict.dict_code}</td>
                          <td>{conflict.code ?? '-'}</td>
                          <td>{conflict.source_value ?? '-'}</td>
                          <td>{conflict.target_value ?? '-'}</td>
                        </tr>
                      ))}
                      {(releasePreview?.conflicts.length ?? 0) === 0 ? (
                        <tr>
                          <td colSpan={5} style={{ padding: 16, textAlign: 'center' }}>
                            {t('dict_release_conflicts_empty')}
                          </td>
                        </tr>
                      ) : null}
                    </tbody>
                  </table>
                </Box>
              </Stack>
            ) : null}

            {releaseResult ? (
              <Stack spacing={1}>
                <Typography variant='subtitle2'>{t('dict_release_result_title')}</Typography>
                <Box
                  sx={{
                    display: 'grid',
                    gap: 1,
                    gridTemplateColumns: {
                      xs: '1fr',
                      md: 'repeat(2, minmax(0, 1fr))'
                    }
                  }}
                >
                  <Typography variant='body2'>
                    {t('dict_release_field_release_id')}：{releaseResult.release_id}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_field_request_id')}：{releaseResult.request_id}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_status')}：{releaseResult.status}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_field_as_of')}：{releaseResult.as_of}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_source_tenant')}：{releaseResult.source_tenant_id}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_target_tenant')}：{releaseResult.target_tenant_id}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_dict_events')}：{releaseResult.dict_events_applied}/{releaseResult.dict_events_total} (+
                    {releaseResult.dict_events_retried})
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_value_events')}：{releaseResult.value_events_applied}/{releaseResult.value_events_total} (+
                    {releaseResult.value_events_retried})
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_started_at')}：{releaseResult.started_at}
                  </Typography>
                  <Typography variant='body2'>
                    {t('dict_release_result_finished_at')}：{releaseResult.finished_at}
                  </Typography>
                </Box>
                <Stack direction='row' spacing={1}>
                  <Button
                    onClick={() => void onCopyField(releaseResult.release_id, t('dict_release_field_release_id'))}
                    size='small'
                    variant='text'
                  >
                    {t('dict_release_copy_release_id')}
                  </Button>
                  <Button
                    onClick={() => void onCopyField(releaseResult.request_id, t('dict_release_field_request_id'))}
                    size='small'
                    variant='text'
                  >
                    {t('dict_release_copy_request_id')}
                  </Button>
                </Stack>
              </Stack>
            ) : null}
          </Stack>
        </Paper>

        <Box
          sx={{
            display: 'grid',
            gap: 2,
            gridTemplateColumns: {
              xs: '1fr',
              md: '280px minmax(0, 1fr)'
            }
          }}
        >
          <Paper sx={{ p: 1.5 }} variant='outlined'>
            <Typography sx={{ mb: 1 }} variant='subtitle2'>
              字典字段列表
            </Typography>
            {dictsQuery.isLoading ? <Typography variant='body2'>加载中...</Typography> : null}
            {dictsQuery.error ? <Alert severity='error'>{parseApiError(dictsQuery.error)}</Alert> : null}

            {dicts.length > 0 ? (
              <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 420, overflow: 'auto', p: 0.5 }}>
                {dicts.map((dictItem) => (
                  <ListItemButton
                    key={dictItem.dict_code}
                    onClick={() => setSelectedDictCode(dictItem.dict_code)}
                    selected={dictItem.dict_code === effectiveSelectedDictCode}
                    sx={{ borderRadius: 1, mb: 0.5 }}
                  >
                    <Box sx={{ alignItems: 'center', display: 'flex', gap: 1, justifyContent: 'space-between', width: '100%' }}>
                      <ListItemText
                        primary={dictItem.name}
                        primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
                        secondary={dictItem.dict_code}
                        secondaryTypographyProps={{ variant: 'caption' }}
                      />
                      <Chip color={statusColor(dictItem.status)} label={dictItem.status} size='small' variant='outlined' />
                    </Box>
                  </ListItemButton>
                ))}
              </List>
            ) : (
              <Typography color='text.secondary' variant='body2'>
                暂无字典字段
              </Typography>
            )}

            <Divider sx={{ my: 1.5 }} />
            <Typography sx={{ mb: 1 }} variant='subtitle2'>
              停用字典字段
            </Typography>
            <Stack component='form' onSubmit={(event) => void onDisableDict(event)} spacing={1}>
              <TextField disabled label='dict_code' value={effectiveSelectedDictCode} />
              <TextField label='disabled_on' type='date' value={disableDictDay} onChange={(event) => setDisableDictDay(event.target.value)} />
              <Button
                disabled={disableDictMutation.isPending || effectiveSelectedDictCode.trim().length === 0}
                type='submit'
                variant='outlined'
              >
                停用字段
              </Button>
            </Stack>
          </Paper>

          <Paper sx={{ p: 1.5 }} variant='outlined'>
            <Typography sx={{ mb: 1 }} variant='subtitle2'>
              字典值列表（点击行进入详情）
            </Typography>
            {valuesQuery.isLoading ? <Typography variant='body2'>加载中...</Typography> : null}
            {valuesQuery.error ? <Alert severity='error'>{parseApiError(valuesQuery.error)}</Alert> : null}

            <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'auto', maxHeight: 420 }}>
              <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                <thead>
                  <tr style={{ position: 'sticky', top: 0, background: '#fff' }}>
                    <th align='left'>code</th>
                    <th align='left'>label</th>
                    <th align='left'>status</th>
                    <th align='left'>enabled_on</th>
                    <th align='left'>disabled_on</th>
                    <th align='left'>updated_at</th>
                  </tr>
                </thead>
                <tbody>
                  {values.map((value) => (
                    <tr
                      key={`${value.dict_code}:${value.code}:${value.enabled_on}`}
                      style={{ borderTop: '1px solid #eee', cursor: 'pointer' }}
                      onClick={() =>
                        navigate({
                          pathname: `/dicts/${value.dict_code}/values/${encodeURIComponent(value.code)}`,
                          search: `?as_of=${asOf}`
                        })
                      }
                    >
                      <td>{value.code}</td>
                      <td>{value.label}</td>
                      <td>{value.status}</td>
                      <td>{value.enabled_on}</td>
                      <td>{value.disabled_on ?? '-'}</td>
                      <td>{value.updated_at}</td>
                    </tr>
                  ))}
                  {values.length === 0 ? (
                    <tr>
                      <td colSpan={6} style={{ padding: 16, textAlign: 'center' }}>
                        暂无字典值
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </Box>
          </Paper>
        </Box>
      </Stack>

      <Dialog fullWidth maxWidth='sm' onClose={() => setCreateDictOpen(false)} open={createDictOpen}>
        <DialogTitle>新增字典字段</DialogTitle>
        <Box component='form' onSubmit={(event) => void onCreateDict(event)}>
          <DialogContent>
            <Stack spacing={1.5}>
              <TextField label='dict_code' required value={createDictCode} onChange={(event) => setCreateDictCode(event.target.value)} />
              <TextField label='name' required value={createDictName} onChange={(event) => setCreateDictName(event.target.value)} />
              <TextField
                label='enabled_on'
                required
                type='date'
                value={createDictEnabledOn}
                onChange={(event) => setCreateDictEnabledOn(event.target.value)}
              />
            </Stack>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateDictOpen(false)}>取消</Button>
            <Button disabled={createDictMutation.isPending} type='submit' variant='contained'>
              提交
            </Button>
          </DialogActions>
        </Box>
      </Dialog>

      <Dialog fullWidth maxWidth='sm' onClose={() => setCreateValueOpen(false)} open={createValueOpen}>
        <DialogTitle>新增字典值</DialogTitle>
        <Box component='form' onSubmit={(event) => void onCreateValue(event)}>
          <DialogContent>
            <Stack spacing={1.5}>
              <TextField disabled label='dict_code' value={effectiveSelectedDictCode} />
              <TextField label='code' required value={createValueCode} onChange={(event) => setCreateValueCode(event.target.value)} />
              <TextField label='label' required value={createValueLabel} onChange={(event) => setCreateValueLabel(event.target.value)} />
              <TextField
                label='enabled_on'
                required
                type='date'
                value={createValueEnabledOn}
                onChange={(event) => setCreateValueEnabledOn(event.target.value)}
              />
            </Stack>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateValueOpen(false)}>取消</Button>
            <Button disabled={createValueMutation.isPending} type='submit' variant='contained'>
              提交
            </Button>
          </DialogActions>
        </Box>
      </Dialog>
    </Box>
  )
}
