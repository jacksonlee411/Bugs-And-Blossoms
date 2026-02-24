import { type FormEvent, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  MenuItem,
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
import { listOrgUnits } from '../../api/orgUnits'
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
import { FreeSoloDropdownField, mergeFreeSoloOptions, uniqueNonEmptyStrings } from '../../components/FreeSoloDropdownField'
import { PageHeader } from '../../components/PageHeader'
import { SetIDExplainPanel } from '../../components/SetIDExplainPanel'
import { resolveApiErrorMessage } from '../../errors/presentApiError'

type SetIDPageTab = 'governance' | 'security-context' | 'strategy-registry' | 'explainability'

interface RegistryFormState {
  capabilityKey: string
  ownerModule: string
  fieldKey: string
  personalizationMode: 'tenant_only' | 'setid'
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

interface DropdownOption {
  value: string
  label: string
}

const ownerModulePresets = ['iam', 'orgunit', 'jobcatalog', 'staffing', 'person']
const personalizationModeOptions: DropdownOption[] = [
  { value: 'setid', label: 'setid' },
  { value: 'tenant_only', label: 'tenant_only' }
]
const orgLevelOptions: DropdownOption[] = [
  { value: 'business_unit', label: 'business_unit' },
  { value: 'tenant', label: 'tenant' }
]
const changePolicyPresets = ['plan_required']
const priorityPresets = [50, 100, 200, 500]

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
  const orgUnitsQuery = useQuery({
    queryKey: ['setid-governance-org-units', asOf],
    queryFn: () => listOrgUnits({ asOf }),
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
  const strategyCatalogQuery = useQuery({
    queryKey: ['setid-strategy-registry-options', asOf],
    queryFn: () => listSetIDStrategyRegistry({ asOf }),
    staleTime: 10_000
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
      await queryClient.invalidateQueries({ queryKey: ['setid-strategy-registry-options', asOf] })
      setRegistryForm((previous) => ({
        ...defaultRegistryForm(asOf),
        capabilityKey: previous.capabilityKey,
        ownerModule: previous.ownerModule,
        fieldKey: previous.fieldKey,
        businessUnitID: previous.businessUnitID
      }))
    }
  })

  const setids = useMemo(() => setidsQuery.data?.setids ?? [], [setidsQuery.data])
  const bindings = useMemo(() => bindingsQuery.data?.bindings ?? [], [bindingsQuery.data])
  const orgUnits = useMemo(() => orgUnitsQuery.data?.org_units ?? [], [orgUnitsQuery.data])
  const strategyRows = useMemo(() => strategyQuery.data?.items ?? [], [strategyQuery.data])
  const strategyCatalogRows = useMemo(() => strategyCatalogQuery.data?.items ?? [], [strategyCatalogQuery.data])
  const setIDOptions = useMemo(() => mergeFreeSoloOptions([], setids.map((item) => item.setid)), [setids])

  const createSetIDOptions = useMemo(
    () => mergeFreeSoloOptions(setIDOptions, [], createSetIDValue),
    [createSetIDValue, setIDOptions]
  )
  const createNameOptions = useMemo(
    () => mergeFreeSoloOptions([], setids.map((item) => item.name), createName),
    [createName, setids]
  )
  const bindOrgCodeOptions = useMemo(
    () => mergeFreeSoloOptions([], orgUnits.map((item) => item.org_code), bindOrgCode),
    [bindOrgCode, orgUnits]
  )
  const bindSetIDOptions = useMemo(
    () => mergeFreeSoloOptions(setIDOptions, [], bindSetIDValue),
    [bindSetIDValue, setIDOptions]
  )

  const capabilityKeyOptions = useMemo(
    () => mergeFreeSoloOptions([], strategyCatalogRows.map((item) => item.capability_key), registryForm.capabilityKey),
    [registryForm.capabilityKey, strategyCatalogRows]
  )
  const fieldKeyOptions = useMemo(
    () => mergeFreeSoloOptions([], strategyCatalogRows.map((item) => item.field_key), registryForm.fieldKey),
    [registryForm.fieldKey, strategyCatalogRows]
  )
  const ownerModuleOptions = useMemo(
    () => mergeFreeSoloOptions(ownerModulePresets, strategyCatalogRows.map((item) => item.owner_module), registryForm.ownerModule),
    [registryForm.ownerModule, strategyCatalogRows]
  )
  const businessUnitOptions = useMemo(
    () =>
      mergeFreeSoloOptions(
        [],
        [...strategyCatalogRows.map((item) => item.business_unit_id ?? ''), ...bindings.map((item) => item.org_unit_id)],
        registryForm.businessUnitID
      ),
    [bindings, registryForm.businessUnitID, strategyCatalogRows]
  )
  const defaultRuleOptions = useMemo(
    () => mergeFreeSoloOptions([], strategyCatalogRows.map((item) => item.default_rule_ref ?? ''), registryForm.defaultRuleRef),
    [registryForm.defaultRuleRef, strategyCatalogRows]
  )
  const defaultValueOptions = useMemo(
    () => mergeFreeSoloOptions([], strategyCatalogRows.map((item) => item.default_value ?? ''), registryForm.defaultValue),
    [registryForm.defaultValue, strategyCatalogRows]
  )
  const priorityOptions = useMemo(() => {
    const values = uniqueNonEmptyStrings([
      ...priorityPresets.map((value) => String(value)),
      ...strategyCatalogRows.map((item) => String(item.priority)),
      String(registryForm.priority)
    ])
    return values.map((value) => Number(value)).filter((value) => Number.isFinite(value))
  }, [registryForm.priority, strategyCatalogRows])
  const changePolicyOptions = useMemo(
    () => mergeFreeSoloOptions(changePolicyPresets, strategyCatalogRows.map((item) => item.change_policy), registryForm.changePolicy),
    [registryForm.changePolicy, strategyCatalogRows]
  )

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
                <FreeSoloDropdownField
                  label='setid'
                  onChange={setCreateSetIDValue}
                  options={createSetIDOptions}
                  value={createSetIDValue}
                />
                <FreeSoloDropdownField
                  label='name'
                  onChange={setCreateName}
                  options={createNameOptions}
                  value={createName}
                />
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
                <FreeSoloDropdownField
                  label='org_code'
                  onChange={setBindOrgCode}
                  options={bindOrgCodeOptions}
                  value={bindOrgCode}
                />
                <FreeSoloDropdownField
                  label='setid'
                  onChange={setBindSetIDValue}
                  options={bindSetIDOptions}
                  value={bindSetIDValue}
                />
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
              title='Security Context Checker'
              subtitle='用于验证 OWNER_CONTEXT_REQUIRED / OWNER_CONTEXT_FORBIDDEN / ACTOR_SCOPE_FORBIDDEN 等上下文拒绝原因。'
            />
          </>
        ) : null}

        {tab === 'strategy-registry' ? (
          <>
            <Paper sx={{ p: 2 }} variant='outlined'>
              <Stack alignItems='center' direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
                <FreeSoloDropdownField
                  label='filter capability_key'
                  onChange={setRegistryCapabilityFilter}
                  options={capabilityKeyOptions}
                  value={registryCapabilityFilter}
                />
                <FreeSoloDropdownField
                  label='filter field_key'
                  onChange={setRegistryFieldFilter}
                  options={fieldKeyOptions}
                  value={registryFieldFilter}
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
                  <FreeSoloDropdownField
                    label='capability_key'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, capabilityKey: nextValue }))}
                    options={capabilityKeyOptions}
                    required
                    value={registryForm.capabilityKey}
                  />
                  <FreeSoloDropdownField
                    label='owner_module'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, ownerModule: nextValue }))}
                    options={ownerModuleOptions}
                    required
                    value={registryForm.ownerModule}
                  />
                  <FreeSoloDropdownField
                    label='field_key'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, fieldKey: nextValue }))}
                    options={fieldKeyOptions}
                    required
                    value={registryForm.fieldKey}
                  />
                  <TextField
                    label='personalization_mode'
                    required
                    select
                    size='small'
                    value={registryForm.personalizationMode}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        personalizationMode: event.target.value as RegistryFormState['personalizationMode'],
                        explainRequired: event.target.value === 'tenant_only' ? prev.explainRequired : true
                      }))
                    }
                  >
                    {personalizationModeOptions.map((option) => (
                      <MenuItem key={`form-personalization-${option.value}`} value={option.value}>
                        {option.label}
                      </MenuItem>
                    ))}
                  </TextField>
                  <TextField
                    label='org_level'
                    required
                    select
                    size='small'
                    value={registryForm.orgLevel}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        orgLevel: event.target.value as RegistryFormState['orgLevel'],
                        businessUnitID: event.target.value === 'tenant' ? '' : prev.businessUnitID
                      }))
                    }
                  >
                    {orgLevelOptions.map((option) => (
                      <MenuItem key={`form-org-level-${option.value}`} value={option.value}>
                        {option.label}
                      </MenuItem>
                    ))}
                  </TextField>
                  <FreeSoloDropdownField
                    label='business_unit_id'
                    disabled={registryForm.orgLevel === 'tenant'}
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, businessUnitID: nextValue }))}
                    options={businessUnitOptions}
                    value={registryForm.businessUnitID}
                  />
                  <FreeSoloDropdownField
                    label='default_rule_ref'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, defaultRuleRef: nextValue }))}
                    options={defaultRuleOptions}
                    value={registryForm.defaultRuleRef}
                  />
                  <FreeSoloDropdownField
                    label='default_value'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, defaultValue: nextValue }))}
                    options={defaultValueOptions}
                    value={registryForm.defaultValue}
                  />
                  <TextField
                    label='priority'
                    select
                    size='small'
                    value={String(registryForm.priority)}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        priority: Number(event.target.value) || 0
                      }))
                    }
                  >
                    {priorityOptions.map((option) => (
                      <MenuItem key={`form-priority-${option}`} value={String(option)}>
                        {option}
                      </MenuItem>
                    ))}
                  </TextField>
                  <FreeSoloDropdownField
                    label='change_policy'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, changePolicy: nextValue }))}
                    options={changePolicyOptions}
                    value={registryForm.changePolicy}
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
                  <TextField
                    label='required'
                    select
                    size='small'
                    value={registryForm.required ? 'true' : 'false'}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, required: event.target.value === 'true' }))}
                  >
                    <MenuItem value='true'>true</MenuItem>
                    <MenuItem value='false'>false</MenuItem>
                  </TextField>
                  <TextField
                    label='visible'
                    select
                    size='small'
                    value={registryForm.visible ? 'true' : 'false'}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, visible: event.target.value === 'true' }))}
                  >
                    <MenuItem value='true'>true</MenuItem>
                    <MenuItem value='false'>false</MenuItem>
                  </TextField>
                  <TextField
                    label='explain_required'
                    select
                    size='small'
                    value={registryForm.explainRequired ? 'true' : 'false'}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        explainRequired: event.target.value === 'true'
                      }))
                    }
                  >
                    <MenuItem value='true'>true</MenuItem>
                    <MenuItem disabled={registryForm.personalizationMode !== 'tenant_only'} value='false'>
                      false
                    </MenuItem>
                  </TextField>
                  <TextField
                    label='is_stable'
                    select
                    size='small'
                    value={registryForm.isStable ? 'true' : 'false'}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, isStable: event.target.value === 'true' }))}
                  >
                    <MenuItem value='true'>true</MenuItem>
                    <MenuItem value='false'>false</MenuItem>
                  </TextField>
                </Box>

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
            title='Explainability'
            subtitle='支持 brief/full 分级展示；错误统一展示 reason_code + trace_id + request_id。'
          />
        ) : null}
      </Stack>
    </Box>
  )
}
