import { type FormEvent, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  Paper,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import type { GridColDef } from '@mui/x-data-grid'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiClientError } from '../../api/errors'
import {
  bindSetID,
  createSetID,
  listSetIDBindings,
  listSetIDStrategyRegistry,
  listSetIDs,
  upsertSetIDStrategyRegistry,
  type SetIDStrategyRegistryItem
} from '../../api/setids'
import { DataGridPage } from '../../components/DataGridPage'
import { PageHeader } from '../../components/PageHeader'
import { SetIDExplainPanel } from '../../components/SetIDExplainPanel'
import { resolveApiErrorMessage } from '../../errors/presentApiError'

type SetIDPageTab = 'governance' | 'security-context' | 'strategy-registry' | 'explainability'

interface RegistryFormState {
  capabilityKey: string
  ownerModule: string
  fieldKey: string
  personalizationMode: 'tenant_only' | 'setid' | 'scope_package'
  scopeCode: string
  orgLevel: 'tenant' | 'business_unit'
  businessUnitID: string
  required: boolean
  visible: boolean
  defaultRuleRef: string
  defaultValue: string
  priority: number
  explainRequired: boolean
  isStable: boolean
  changePolicy: string
  effectiveDate: string
  endDate: string
  requestID: string
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestID(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

function parseTab(raw: string | null): SetIDPageTab {
  switch ((raw ?? '').trim()) {
    case 'security-context':
      return 'security-context'
    case 'strategy-registry':
      return 'strategy-registry'
    case 'explainability':
      return 'explainability'
    default:
      return 'governance'
  }
}

function parseApiError(error: unknown): string {
  if (error instanceof ApiClientError) {
    const details = error.details
    if (details && typeof details === 'object') {
      const code = String(Reflect.get(details, 'code') ?? '').trim()
      const traceID = String(Reflect.get(details, 'trace_id') ?? '').trim()
      const message = resolveApiErrorMessage(code, error.message).trim()
      if (code.length > 0 && traceID.length > 0) {
        return `${message} (${code}, trace_id=${traceID})`
      }
      if (code.length > 0) {
        return `${message} (${code})`
      }
      return message
    }
  }
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function defaultRegistryForm(asOf: string): RegistryFormState {
  return {
    capabilityKey: '',
    ownerModule: '',
    fieldKey: '',
    personalizationMode: 'setid',
    scopeCode: '',
    orgLevel: 'business_unit',
    businessUnitID: '',
    required: true,
    visible: true,
    defaultRuleRef: '',
    defaultValue: '',
    priority: 100,
    explainRequired: true,
    isStable: false,
    changePolicy: 'plan_required',
    effectiveDate: asOf,
    endDate: '',
    requestID: newRequestID('mui-setid-strategy')
  }
}

function strategyRowID(item: SetIDStrategyRegistryItem): string {
  return [
    item.capability_key,
    item.field_key,
    item.org_level,
    item.business_unit_id ?? '-',
    item.effective_date
  ].join(':')
}

export function SetIDGovernancePage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  const [asOf, setAsOf] = useState(searchParams.get('as_of')?.trim() || todayISO())
  const [createSetIDValue, setCreateSetIDValue] = useState('')
  const [createName, setCreateName] = useState('')
  const [bindOrgCode, setBindOrgCode] = useState('')
  const [bindSetIDValue, setBindSetIDValue] = useState('')
  const [tab, setTab] = useState<SetIDPageTab>(parseTab(searchParams.get('tab')))

  const [registryCapabilityFilter, setRegistryCapabilityFilter] = useState('')
  const [registryFieldFilter, setRegistryFieldFilter] = useState('')
  const [registryForm, setRegistryForm] = useState<RegistryFormState>(() => defaultRegistryForm(asOf))

  const [error, setError] = useState<string | null>(null)
  const [registryNotice, setRegistryNotice] = useState<string | null>(null)

  const setidsQuery = useQuery({
    queryKey: ['setids'],
    queryFn: () => listSetIDs(),
    staleTime: 30_000
  })

  const bindingsQuery = useQuery({
    queryKey: ['setid-bindings', asOf],
    queryFn: () => listSetIDBindings({ asOf }),
    staleTime: 10_000
  })

  const strategyQuery = useQuery({
    queryKey: ['setid-strategy-registry', asOf, registryCapabilityFilter, registryFieldFilter],
    queryFn: () =>
      listSetIDStrategyRegistry({
        asOf,
        capabilityKey: registryCapabilityFilter,
        fieldKey: registryFieldFilter
      }),
    staleTime: 5_000
  })

  const createMutation = useMutation({
    mutationFn: (req: { setid: string; name: string; effective_date: string; request_id: string }) => createSetID(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['setids'] })
      setCreateSetIDValue('')
      setCreateName('')
    }
  })

  const bindMutation = useMutation({
    mutationFn: (req: { org_code: string; setid: string; effective_date: string; request_id: string }) => bindSetID(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['setid-bindings', asOf] })
      setBindOrgCode('')
      setBindSetIDValue('')
    }
  })

  const strategyMutation = useMutation({
    mutationFn: upsertSetIDStrategyRegistry,
    onSuccess: async () => {
      setRegistryNotice('策略已保存')
      await queryClient.invalidateQueries({ queryKey: ['setid-strategy-registry', asOf] })
      setRegistryForm((previous) => ({
        ...defaultRegistryForm(asOf),
        capabilityKey: previous.capabilityKey,
        ownerModule: previous.ownerModule,
        fieldKey: previous.fieldKey,
        scopeCode: previous.scopeCode,
        businessUnitID: previous.businessUnitID
      }))
    }
  })

  const setids = useMemo(() => setidsQuery.data?.setids ?? [], [setidsQuery.data])
  const bindings = useMemo(() => bindingsQuery.data?.bindings ?? [], [bindingsQuery.data])
  const strategyRows = useMemo(() => strategyQuery.data?.items ?? [], [strategyQuery.data])

  const strategyColumns = useMemo<GridColDef[]>(
    () => [
      { field: 'capability_key', headerName: 'capability_key', flex: 1.3, minWidth: 200 },
      { field: 'field_key', headerName: 'field_key', minWidth: 140 },
      { field: 'personalization_mode', headerName: 'mode', minWidth: 130 },
      { field: 'org_level', headerName: 'org_level', minWidth: 120 },
      { field: 'business_unit_id', headerName: 'business_unit_id', minWidth: 140 },
      {
        field: 'policy',
        headerName: 'required / visible / default',
        minWidth: 260,
        valueGetter: (_, row: SetIDStrategyRegistryItem) =>
          `${row.required ? 'required' : 'optional'} · ${row.visible ? 'visible' : 'hidden'} · ${
            row.default_rule_ref || row.default_value || '-'
          }`
      },
      { field: 'priority', headerName: 'priority', minWidth: 100 },
      { field: 'effective_date', headerName: 'effective_date', minWidth: 130 },
      { field: 'end_date', headerName: 'end_date', minWidth: 120 },
      { field: 'updated_at', headerName: 'updated_at', minWidth: 180 }
    ],
    []
  )

  function updateURL(nextTab: SetIDPageTab, nextAsOf: string) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set('tab', nextTab)
    nextParams.set('as_of', nextAsOf)
    setSearchParams(nextParams)
  }

  async function onCreate(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await createMutation.mutateAsync({
        setid: createSetIDValue.trim(),
        name: createName.trim(),
        effective_date: asOf,
        request_id: newRequestID('mui-setid-create')
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  async function onBind(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await bindMutation.mutateAsync({
        org_code: bindOrgCode.trim(),
        setid: bindSetIDValue.trim(),
        effective_date: asOf,
        request_id: newRequestID('mui-setid-bind')
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  async function onUpsertStrategy(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setRegistryNotice(null)
    try {
      await strategyMutation.mutateAsync({
        capability_key: registryForm.capabilityKey.trim(),
        owner_module: registryForm.ownerModule.trim(),
        field_key: registryForm.fieldKey.trim(),
        personalization_mode: registryForm.personalizationMode,
        scope_code: registryForm.scopeCode.trim(),
        org_level: registryForm.orgLevel,
        business_unit_id: registryForm.businessUnitID.trim(),
        required: registryForm.required,
        visible: registryForm.visible,
        default_rule_ref: registryForm.defaultRuleRef.trim(),
        default_value: registryForm.defaultValue.trim(),
        priority: registryForm.priority,
        explain_required: registryForm.explainRequired,
        is_stable: registryForm.isStable,
        change_policy: registryForm.changePolicy.trim(),
        effective_date: registryForm.effectiveDate.trim(),
        end_date: registryForm.endDate.trim(),
        request_id: registryForm.requestID.trim()
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  return (
    <Box>
      <PageHeader title='SetID Governance' subtitle='C1/C2/C3 UI：上下文安全、策略注册表、命中解释' />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}
        {registryNotice ? <Alert severity='success'>{registryNotice}</Alert> : null}

        <Paper sx={{ p: 2 }} variant='outlined'>
          <Stack alignItems='center' direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
            <TextField
              label='as_of'
              name='as_of'
              type='date'
              value={asOf}
              onChange={(event) => {
                const nextAsOf = event.target.value
                setAsOf(nextAsOf)
                setRegistryForm((previous) => ({ ...previous, effectiveDate: nextAsOf }))
                updateURL(tab, nextAsOf)
              }}
            />
            <Typography color='text.secondary' variant='body2'>
              当前上下文：{asOf}
            </Typography>
          </Stack>
        </Paper>

        <Tabs
          onChange={(_, value: SetIDPageTab) => {
            setTab(value)
            updateURL(value, asOf)
          }}
          value={tab}
        >
          <Tab label='Governance' value='governance' />
          <Tab label='Security Context' value='security-context' />
          <Tab label='Strategy Registry' value='strategy-registry' />
          <Tab label='Explainability' value='explainability' />
        </Tabs>

        {tab === 'governance' ? (
          <>
            <Paper sx={{ p: 2 }} variant='outlined'>
              <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
                Create SetID
              </Typography>
              <Stack
                component='form'
                spacing={1.5}
                onSubmit={(event) => {
                  void onCreate(event)
                }}
              >
                <TextField label='setid' name='setid' value={createSetIDValue} onChange={(event) => setCreateSetIDValue(event.target.value)} />
                <TextField label='name' name='name' value={createName} onChange={(event) => setCreateName(event.target.value)} />
                <Button disabled={createMutation.isPending} type='submit' variant='contained'>
                  Create
                </Button>
              </Stack>
            </Paper>

            <Paper sx={{ p: 2 }} variant='outlined'>
              <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
                Bind SetID
              </Typography>
              <Stack
                component='form'
                spacing={1.5}
                onSubmit={(event) => {
                  void onBind(event)
                }}
              >
                <TextField label='org_code' name='org_code' value={bindOrgCode} onChange={(event) => setBindOrgCode(event.target.value)} />
                <TextField label='setid' name='setid' value={bindSetIDValue} onChange={(event) => setBindSetIDValue(event.target.value)} />
                <Button disabled={bindMutation.isPending} type='submit' variant='contained'>
                  Bind
                </Button>
              </Stack>
            </Paper>

            <Paper sx={{ p: 2 }} variant='outlined'>
              <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
                SetIDs
              </Typography>
              {setidsQuery.isError ? <Alert severity='error'>SetID list failed</Alert> : null}
              <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'auto' }}>
                <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                  <thead>
                    <tr style={{ background: '#fff' }}>
                      <th align='left'>setid</th>
                      <th align='left'>name</th>
                      <th align='left'>status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {setids.map((item) => (
                      <tr key={item.setid} style={{ borderTop: '1px solid #eee' }}>
                        <td>{item.setid}</td>
                        <td>{item.name}</td>
                        <td>{item.status}</td>
                      </tr>
                    ))}
                    {setids.length === 0 ? (
                      <tr>
                        <td colSpan={3} style={{ padding: 16, textAlign: 'center' }}>
                          暂无 SetID
                        </td>
                      </tr>
                    ) : null}
                  </tbody>
                </table>
              </Box>
            </Paper>

            <Paper sx={{ p: 2 }} variant='outlined'>
              <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
                Bindings
              </Typography>
              {bindingsQuery.isError ? <Alert severity='error'>Binding list failed</Alert> : null}
              <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'auto' }}>
                <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                  <thead>
                    <tr style={{ background: '#fff' }}>
                      <th align='left'>org_unit_id</th>
                      <th align='left'>setid</th>
                      <th align='left'>valid_from</th>
                      <th align='left'>valid_to</th>
                    </tr>
                  </thead>
                  <tbody>
                    {bindings.map((item) => (
                      <tr key={`${item.org_unit_id}:${item.setid}:${item.valid_from}`} style={{ borderTop: '1px solid #eee' }}>
                        <td>{item.org_unit_id}</td>
                        <td>{item.setid}</td>
                        <td>{item.valid_from}</td>
                        <td>{item.valid_to}</td>
                      </tr>
                    ))}
                    {bindings.length === 0 ? (
                      <tr>
                        <td colSpan={4} style={{ padding: 16, textAlign: 'center' }}>
                          暂无绑定记录
                        </td>
                      </tr>
                    ) : null}
                  </tbody>
                </table>
              </Box>
            </Paper>
          </>
        ) : null}

        {tab === 'security-context' ? (
          <>
            <Alert severity='info'>
              输入 capability/field/BU 上下文，直接查看 deny reason 与下一步动作建议（reason_code + trace_id + request_id）。
            </Alert>
            <SetIDExplainPanel
              initialAsOf={asOf}
              initialScopeCode='jobcatalog'
              title='Security Context Checker'
              subtitle='用于验证 OWNER_CONTEXT_REQUIRED / OWNER_CONTEXT_FORBIDDEN / ACTOR_SCOPE_FORBIDDEN 等上下文拒绝原因。'
            />
          </>
        ) : null}

        {tab === 'strategy-registry' ? (
          <>
            <Paper sx={{ p: 2 }} variant='outlined'>
              <Stack alignItems='center' direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
                <TextField
                  label='filter capability_key'
                  size='small'
                  value={registryCapabilityFilter}
                  onChange={(event) => setRegistryCapabilityFilter(event.target.value)}
                />
                <TextField
                  label='filter field_key'
                  size='small'
                  value={registryFieldFilter}
                  onChange={(event) => setRegistryFieldFilter(event.target.value)}
                />
                <Button onClick={() => setRegistryNotice(null)} size='small' variant='outlined'>
                  清除提示
                </Button>
              </Stack>
            </Paper>

            <DataGridPage
              columns={strategyColumns}
              loading={strategyQuery.isLoading}
              noRowsLabel='暂无策略'
              rows={strategyRows.map((item) => ({ id: strategyRowID(item), ...item }))}
              storageKey='setid-strategy-registry-grid'
            />

            <Paper sx={{ p: 2 }} variant='outlined'>
              <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
                Upsert Strategy
              </Typography>
              <Stack
                component='form'
                spacing={1.5}
                onSubmit={(event) => {
                  void onUpsertStrategy(event)
                }}
              >
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
                  <TextField
                    label='capability_key'
                    required
                    size='small'
                    value={registryForm.capabilityKey}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, capabilityKey: event.target.value }))}
                  />
                  <TextField
                    label='owner_module'
                    required
                    size='small'
                    value={registryForm.ownerModule}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, ownerModule: event.target.value }))}
                  />
                  <TextField
                    label='field_key'
                    required
                    size='small'
                    value={registryForm.fieldKey}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, fieldKey: event.target.value }))}
                  />
                  <TextField
                    label='personalization_mode'
                    required
                    size='small'
                    value={registryForm.personalizationMode}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        personalizationMode: event.target.value as RegistryFormState['personalizationMode']
                      }))
                    }
                  />
                  <TextField
                    label='org_level'
                    required
                    size='small'
                    value={registryForm.orgLevel}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        orgLevel: event.target.value as RegistryFormState['orgLevel']
                      }))
                    }
                  />
                  <TextField
                    label='scope_code'
                    size='small'
                    value={registryForm.scopeCode}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, scopeCode: event.target.value }))}
                  />
                  <TextField
                    label='business_unit_id'
                    size='small'
                    value={registryForm.businessUnitID}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, businessUnitID: event.target.value }))}
                  />
                  <TextField
                    label='default_rule_ref'
                    size='small'
                    value={registryForm.defaultRuleRef}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, defaultRuleRef: event.target.value }))}
                  />
                  <TextField
                    label='default_value'
                    size='small'
                    value={registryForm.defaultValue}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, defaultValue: event.target.value }))}
                  />
                  <TextField
                    label='priority'
                    size='small'
                    type='number'
                    value={registryForm.priority}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        priority: Number(event.target.value) || 0
                      }))
                    }
                  />
                  <TextField
                    label='change_policy'
                    size='small'
                    value={registryForm.changePolicy}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, changePolicy: event.target.value }))}
                  />
                  <TextField
                    label='effective_date'
                    required
                    size='small'
                    type='date'
                    value={registryForm.effectiveDate}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, effectiveDate: event.target.value }))}
                  />
                  <TextField
                    label='end_date'
                    size='small'
                    type='date'
                    value={registryForm.endDate}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, endDate: event.target.value }))}
                  />
                  <TextField
                    label='request_id'
                    required
                    size='small'
                    value={registryForm.requestID}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, requestID: event.target.value }))}
                  />
                </Box>

                <Stack direction='row' flexWrap='wrap' gap={1}>
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={registryForm.required}
                        onChange={(event) => setRegistryForm((prev) => ({ ...prev, required: event.target.checked }))}
                      />
                    }
                    label='required'
                  />
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={registryForm.visible}
                        onChange={(event) => setRegistryForm((prev) => ({ ...prev, visible: event.target.checked }))}
                      />
                    }
                    label='visible'
                  />
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={registryForm.explainRequired}
                        onChange={(event) => setRegistryForm((prev) => ({ ...prev, explainRequired: event.target.checked }))}
                      />
                    }
                    label='explain_required'
                  />
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={registryForm.isStable}
                        onChange={(event) => setRegistryForm((prev) => ({ ...prev, isStable: event.target.checked }))}
                      />
                    }
                    label='is_stable'
                  />
                </Stack>

                <Stack direction='row' spacing={1}>
                  <Button disabled={strategyMutation.isPending} type='submit' variant='contained'>
                    保存策略
                  </Button>
                  <Button
                    onClick={() => setRegistryForm(defaultRegistryForm(asOf))}
                    variant='outlined'
                  >
                    重置表单
                  </Button>
                </Stack>
              </Stack>
            </Paper>
          </>
        ) : null}

        {tab === 'explainability' ? (
          <SetIDExplainPanel
            initialAsOf={asOf}
            initialScopeCode='jobcatalog'
            title='Explainability'
            subtitle='支持 brief/full 分级展示；错误统一展示 reason_code + trace_id + request_id。'
          />
        ) : null}
      </Stack>
    </Box>
  )
}
