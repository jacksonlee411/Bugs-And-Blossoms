import { type FormEvent, useMemo, useState } from 'react'
import { Link as RouterLink, useParams, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Divider,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { correctDictValue, disableDictValue, listDictAudit, listDictValues, type DictAuditItem, type DictValueItem } from '../../api/dicts'
import { PageHeader } from '../../components/PageHeader'

type DetailTab = 'profile' | 'audit'

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

function selectCodeVersions(values: DictValueItem[], code: string): DictValueItem[] {
  const targetCode = code.trim()
  if (targetCode.length === 0) {
    return []
  }
  return values
    .filter((item) => item.code === targetCode)
    .sort((a, b) => {
      if (a.enabled_on === b.enabled_on) {
        return a.code.localeCompare(b.code)
      }
      return b.enabled_on.localeCompare(a.enabled_on)
    })
}

export function DictValueDetailsPage() {
  const queryClient = useQueryClient()
  const { dictCode: rawDictCode = '', code: rawCode = '' } = useParams()
  const [searchParams] = useSearchParams()

  const dictCode = rawDictCode.trim().toLowerCase()
  const code = decodeURIComponent(rawCode).trim()
  const asOf = (searchParams.get('as_of') ?? '').trim() || todayISO()

  const [tab, setTab] = useState<DetailTab>('profile')
  const [selectedVersionEnabledOn, setSelectedVersionEnabledOn] = useState('')
  const [selectedAuditEventUUID, setSelectedAuditEventUUID] = useState('')
  const [error, setError] = useState<string | null>(null)

  const [disableValueDay, setDisableValueDay] = useState(todayISO())
  const [correctLabelDraft, setCorrectLabelDraft] = useState<string | null>(null)
  const [correctDay, setCorrectDay] = useState(todayISO())

  const versionsQuery = useQuery({
    enabled: dictCode.length > 0 && code.length > 0,
    queryKey: ['dict-value-versions', dictCode, code, asOf],
    queryFn: () =>
      listDictValues({
        dictCode,
        asOf,
        q: code,
        status: 'all',
        limit: 50
      }),
    staleTime: 5_000
  })

  const versions = useMemo(() => selectCodeVersions(versionsQuery.data?.values ?? [], code), [code, versionsQuery.data])

  const effectiveSelectedVersionEnabledOn = useMemo(() => {
    if (versions.length === 0) {
      return ''
    }
    const selected = selectedVersionEnabledOn.trim()
    if (selected.length > 0 && versions.some((item) => item.enabled_on === selected)) {
      return selected
    }
    return versions[0]?.enabled_on ?? ''
  }, [selectedVersionEnabledOn, versions])

  const selectedVersion = useMemo(
    () => versions.find((item) => item.enabled_on === effectiveSelectedVersionEnabledOn) ?? versions[0] ?? null,
    [effectiveSelectedVersionEnabledOn, versions]
  )

  const correctLabel = useMemo(() => {
    if (typeof correctLabelDraft === 'string') {
      return correctLabelDraft
    }
    return selectedVersion?.label ?? ''
  }, [correctLabelDraft, selectedVersion?.label])

  const auditQuery = useQuery({
    enabled: dictCode.length > 0 && code.length > 0,
    queryKey: ['dict-audit', dictCode, code],
    queryFn: () => listDictAudit({ dictCode, code, limit: 50 }),
    staleTime: 5_000
  })

  const auditEvents = useMemo(() => auditQuery.data?.events ?? [], [auditQuery.data])

  const effectiveSelectedAuditEventUUID = useMemo(() => {
    if (auditEvents.length === 0) {
      return ''
    }
    const selected = selectedAuditEventUUID.trim()
    if (selected.length > 0 && auditEvents.some((event) => event.event_uuid === selected)) {
      return selected
    }
    return auditEvents[0]?.event_uuid ?? ''
  }, [auditEvents, selectedAuditEventUUID])

  const selectedAuditEvent = useMemo(
    () => auditEvents.find((event) => event.event_uuid === effectiveSelectedAuditEventUUID) ?? auditEvents[0] ?? null,
    [auditEvents, effectiveSelectedAuditEventUUID]
  )

  const disableValueMutation = useMutation({
    mutationFn: (request: { dict_code: string; code: string; disabled_on: string; request_id: string }) => disableDictValue(request),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dict-value-versions', dictCode, code, asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-values', dictCode, asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-audit', dictCode, code] })
      ])
    }
  })

  const correctValueMutation = useMutation({
    mutationFn: (request: { dict_code: string; code: string; label: string; correction_day: string; request_id: string }) =>
      correctDictValue(request),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dict-value-versions', dictCode, code, asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-values', dictCode, asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-audit', dictCode, code] })
      ])
    }
  })

  async function onDisableValue(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (dictCode.length === 0 || code.length === 0) {
      setError('参数缺失，无法停用')
      return
    }
    try {
      await disableValueMutation.mutateAsync({
        dict_code: dictCode,
        code,
        disabled_on: disableValueDay,
        request_id: newRequestID('mui-dict-value-disable')
      })
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  async function onCorrectValue(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (dictCode.length === 0 || code.length === 0) {
      setError('参数缺失，无法修正')
      return
    }
    try {
      await correctValueMutation.mutateAsync({
        dict_code: dictCode,
        code,
        label: correctLabel.trim(),
        correction_day: correctDay,
        request_id: newRequestID('mui-dict-value-correct')
      })
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  return (
    <Box>
      <PageHeader
        title={`字典值详情：${dictCode}/${code}`}
        subtitle='基本信息与变更日志（参考 Org 模块双栏布局）'
        actions={
          <Button component={RouterLink} to={{ pathname: '/dicts', search: `?as_of=${asOf}` }} size='small' variant='outlined'>
            返回列表
          </Button>
        }
      />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}
        {versionsQuery.error ? <Alert severity='error'>{parseApiError(versionsQuery.error)}</Alert> : null}
        {auditQuery.error ? <Alert severity='error'>{parseApiError(auditQuery.error)}</Alert> : null}

        <Paper sx={{ p: 1.5 }} variant='outlined'>
          <Tabs onChange={(_, value: DetailTab) => setTab(value)} value={tab}>
            <Tab label='基本信息' value='profile' />
            <Tab label='变更日志' value='audit' />
          </Tabs>

          {tab === 'profile' ? (
            <Box
              sx={{
                display: 'grid',
                gap: 1.5,
                gridTemplateColumns: { xs: '1fr', md: '240px minmax(0, 1fr)' },
                mt: 1.5
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography sx={{ mb: 1 }} variant='subtitle2'>
                  生效日期
                </Typography>
                {versions.length > 0 ? (
                  <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 360, overflow: 'auto', p: 0.5 }}>
                    {versions.map((version) => (
                      <ListItemButton
                        key={`${version.code}:${version.enabled_on}`}
                        onClick={() => {
                          setSelectedVersionEnabledOn(version.enabled_on)
                          setCorrectLabelDraft(null)
                        }}
                        selected={version.enabled_on === effectiveSelectedVersionEnabledOn}
                        sx={{ borderRadius: 1, mb: 0.5 }}
                      >
                        <ListItemText
                          primary={version.enabled_on}
                          primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
                          secondary={version.status}
                          secondaryTypographyProps={{ variant: 'caption' }}
                        />
                      </ListItemButton>
                    ))}
                  </List>
                ) : (
                  <Typography color='text.secondary' variant='body2'>
                    暂无生效记录
                  </Typography>
                )}
              </Box>

              <Box sx={{ minWidth: 0 }}>
                {selectedVersion ? (
                  <>
                    <Typography variant='subtitle1'>
                      {selectedVersion.dict_code} / {selectedVersion.code}
                    </Typography>
                    <Divider sx={{ my: 1.2 }} />
                    <Stack spacing={1}>
                      <Typography variant='body2'>label：{selectedVersion.label}</Typography>
                      <Typography variant='body2'>status：{selectedVersion.status}</Typography>
                      <Typography variant='body2'>enabled_on：{selectedVersion.enabled_on}</Typography>
                      <Typography variant='body2'>disabled_on：{selectedVersion.disabled_on ?? '-'}</Typography>
                      <Typography variant='body2'>updated_at：{selectedVersion.updated_at}</Typography>
                    </Stack>

                    <Divider sx={{ my: 1.2 }} />
                    <Stack component='form' onSubmit={(event) => void onDisableValue(event)} spacing={1.2}>
                      <Typography variant='subtitle2'>停用当前值</Typography>
                      <TextField label='disabled_on' type='date' value={disableValueDay} onChange={(event) => setDisableValueDay(event.target.value)} />
                      <Button disabled={disableValueMutation.isPending} type='submit' variant='outlined'>
                        停用值
                      </Button>
                    </Stack>

                    <Divider sx={{ my: 1.2 }} />
                    <Stack component='form' onSubmit={(event) => void onCorrectValue(event)} spacing={1.2}>
                      <Typography variant='subtitle2'>修正标签</Typography>
                      <TextField label='label' value={correctLabel} onChange={(event) => setCorrectLabelDraft(event.target.value)} />
                      <TextField label='correction_day' type='date' value={correctDay} onChange={(event) => setCorrectDay(event.target.value)} />
                      <Button disabled={correctValueMutation.isPending} type='submit' variant='outlined'>
                        修正值
                      </Button>
                    </Stack>
                  </>
                ) : (
                  <Typography color='text.secondary' variant='body2'>
                    暂无详情
                  </Typography>
                )}
              </Box>
            </Box>
          ) : null}

          {tab === 'audit' ? (
            <Box
              sx={{
                display: 'grid',
                gap: 1.5,
                gridTemplateColumns: { xs: '1fr', md: '240px minmax(0, 1fr)' },
                mt: 1.5
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography sx={{ mb: 1 }} variant='subtitle2'>
                  修改时间
                </Typography>
                {auditEvents.length > 0 ? (
                  <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 360, overflow: 'auto', p: 0.5 }}>
                    {auditEvents.map((event) => (
                      <ListItemButton
                        key={event.event_uuid}
                        onClick={() => setSelectedAuditEventUUID(event.event_uuid)}
                        selected={selectedAuditEvent?.event_uuid === event.event_uuid}
                        sx={{ borderRadius: 1, mb: 0.5 }}
                      >
                        <ListItemText
                          primary={event.tx_time}
                          primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
                          secondary={event.event_type}
                          secondaryTypographyProps={{ variant: 'caption' }}
                        />
                      </ListItemButton>
                    ))}
                  </List>
                ) : (
                  <Typography color='text.secondary' variant='body2'>
                    暂无变更记录
                  </Typography>
                )}
              </Box>

              <Box sx={{ minWidth: 0 }}>
                {selectedAuditEvent ? <AuditEventDetail event={selectedAuditEvent} /> : <Typography color='text.secondary' variant='body2'>暂无变更详情</Typography>}
              </Box>
            </Box>
          ) : null}
        </Paper>
      </Stack>
    </Box>
  )
}

function AuditEventDetail({ event }: { event: DictAuditItem }) {
  return (
    <>
      <Typography variant='subtitle1'>变更详情</Typography>
      <Divider sx={{ my: 1.2 }} />
      <Stack spacing={1}>
        <Typography variant='body2'>tx_time：{event.tx_time}</Typography>
        <Typography variant='body2'>event_type：{event.event_type}</Typography>
        <Typography variant='body2'>request_id：{event.request_id}</Typography>
        <Typography variant='body2'>initiator_uuid：{event.initiator_uuid || '-'}</Typography>
        <Typography variant='body2'>effective_day：{event.effective_day}</Typography>
        <Typography variant='body2'>event_uuid：{event.event_uuid}</Typography>
      </Stack>
    </>
  )
}
