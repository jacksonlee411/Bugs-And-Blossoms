import { type FormEvent, useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Paper,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { ApiClientError } from '../../api/errors'
import {
  activatePolicyVersion,
  bindSetID,
  createSetID,
  disableSetIDStrategyRegistry,
  getPolicyActivationState,
  listSetIDBindings,
  listFunctionalAreaState,
  listSetIDStrategyRegistry,
  listSetIDs,
  rollbackPolicyVersion,
  setPolicyDraft,
  switchFunctionalAreaState,
  upsertSetIDStrategyRegistry,
  type CapabilityPolicyState,
  type FunctionalAreaStateItem,
  type SetIDStrategyRegistryItem
} from '../../api/setids'
import { DataGridPage } from '../../components/DataGridPage'
import { PageHeader } from '../../components/PageHeader'
import { SetIDExplainPanel } from '../../components/SetIDExplainPanel'
import { resolveApiErrorMessage } from '../../errors/presentApiError'

type SetIDPageTab = 'registry' | 'explain' | 'functional-area' | 'activation'

interface RegistryFormState {
  capabilityKey: string
  ownerModule: string
  fieldKey: string
  personalizationMode: 'tenant_only' | 'setid'
  orgLevel: 'tenant' | 'business_unit'
  businessUnitID: string
  required: boolean
  visible: boolean
  maintainable: boolean
  defaultRuleRef: string
  defaultValue: string
  allowedValueCodes: string
  priority: number
  explainRequired: boolean
  isStable: boolean
  changePolicy: string
  effectiveDate: string
  endDate: string
  requestID: string
}

interface RegistryDisableDialogState {
  open: boolean
  row: SetIDStrategyRegistryItem | null
  disableAsOf: string
}

interface ActivationFormState {
  capabilityKey: string
  draftPolicyVersion: string
  activatePolicyVersion: string
  rollbackPolicyVersion: string
  operator: string
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestID(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

function parseTab(raw: string | null): SetIDPageTab {
  switch ((raw ?? '').trim()) {
    case 'explain':
      return 'explain'
    case 'functional-area':
      return 'functional-area'
    case 'activation':
      return 'activation'
    default:
      return 'registry'
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
    maintainable: true,
    defaultRuleRef: '',
    defaultValue: '',
    allowedValueCodes: '',
    priority: 100,
    explainRequired: true,
    isStable: false,
    changePolicy: 'plan_required',
    effectiveDate: asOf,
    endDate: '',
    requestID: newRequestID('mui-setid-strategy')
  }
}

function parseAllowedValueCodes(raw: string): string[] {
  return raw
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

function toAllowedValueCodesText(values: string[] | undefined): string {
  return (values ?? []).map((item) => item.trim()).filter((item) => item.length > 0).join(', ')
}

function toRegistryFormFromRow(row: SetIDStrategyRegistryItem): RegistryFormState {
  return {
    capabilityKey: row.capability_key,
    ownerModule: row.owner_module,
    fieldKey: row.field_key,
    personalizationMode: row.personalization_mode,
    orgLevel: row.org_level,
    businessUnitID: row.business_unit_id ?? '',
    required: row.required,
    visible: row.visible,
    maintainable: row.maintainable,
    defaultRuleRef: row.default_rule_ref ?? '',
    defaultValue: row.default_value ?? '',
    allowedValueCodes: toAllowedValueCodesText(row.allowed_value_codes),
    priority: row.priority,
    explainRequired: row.explain_required,
    isStable: row.is_stable,
    changePolicy: row.change_policy,
    effectiveDate: row.effective_date,
    endDate: row.end_date ?? '',
    requestID: newRequestID('mui-setid-strategy')
  }
}

function nextDayISO(baseDate: string): string {
  const parsed = new Date(`${baseDate}T00:00:00Z`)
  if (Number.isNaN(parsed.getTime())) {
    return ''
  }
  parsed.setUTCDate(parsed.getUTCDate() + 1)
  return parsed.toISOString().slice(0, 10)
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

function defaultActivationForm(): ActivationFormState {
  return {
    capabilityKey: 'org.policy_activation.manage',
    draftPolicyVersion: '',
    activatePolicyVersion: '',
    rollbackPolicyVersion: '',
    operator: 'mui-operator'
  }
}

export function SetIDGovernancePage() {
  const { hasPermission } = useAppPreferences()
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
  const [registryFormMode, setRegistryFormMode] = useState<'create' | 'edit' | 'fork'>('create')
  const [registryDisableDialog, setRegistryDisableDialog] = useState<RegistryDisableDialogState>({
    open: false,
    row: null,
    disableAsOf: ''
  })
  const [activationForm, setActivationForm] = useState<ActivationFormState>(() => defaultActivationForm())
  const [functionalAreaOperator, setFunctionalAreaOperator] = useState('mui-operator')

  const [error, setError] = useState<string | null>(null)
  const [registryNotice, setRegistryNotice] = useState<string | null>(null)
  const [activationNotice, setActivationNotice] = useState<string | null>(null)

  const canManageGovernance = hasPermission('setid.governance.manage')
  const canViewFullExplain = hasPermission('setid.explain.full')

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

  const functionalAreaQuery = useQuery({
    queryKey: ['functional-area-state'],
    queryFn: () => listFunctionalAreaState(),
    staleTime: 5_000
  })

  const activationStateQuery = useQuery({
    queryKey: ['policy-activation-state', activationForm.capabilityKey],
    queryFn: () => getPolicyActivationState(activationForm.capabilityKey),
    staleTime: 3_000,
    enabled: activationForm.capabilityKey.trim().length > 0
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
      setRegistryForm(defaultRegistryForm(asOf))
      setRegistryFormMode('create')
    }
  })

  const strategyDisableMutation = useMutation({
    mutationFn: disableSetIDStrategyRegistry,
    onSuccess: async () => {
      setRegistryNotice('策略已停用')
      await queryClient.invalidateQueries({ queryKey: ['setid-strategy-registry', asOf] })
      setRegistryDisableDialog({ open: false, row: null, disableAsOf: '' })
    }
  })

  const functionalAreaSwitchMutation = useMutation({
    mutationFn: switchFunctionalAreaState,
    onSuccess: async () => {
      setRegistryNotice('Functional Area 开关已更新')
      await queryClient.invalidateQueries({ queryKey: ['functional-area-state'] })
    }
  })

  const policyDraftMutation = useMutation({
    mutationFn: setPolicyDraft,
    onSuccess: async (state) => {
      setActivationNotice(`Draft 已更新：${state.draft_policy_version || '-'}`)
      await queryClient.invalidateQueries({ queryKey: ['policy-activation-state', state.capability_key] })
    }
  })

  const policyActivateMutation = useMutation({
    mutationFn: activatePolicyVersion,
    onSuccess: async (state) => {
      setActivationNotice(`已激活：${state.active_policy_version}`)
      await queryClient.invalidateQueries({ queryKey: ['policy-activation-state', state.capability_key] })
    }
  })

  const policyRollbackMutation = useMutation({
    mutationFn: rollbackPolicyVersion,
    onSuccess: async (state) => {
      setActivationNotice(`已回滚到：${state.active_policy_version}`)
      await queryClient.invalidateQueries({ queryKey: ['policy-activation-state', state.capability_key] })
    }
  })

  const setids = useMemo(() => setidsQuery.data?.setids ?? [], [setidsQuery.data])
  const bindings = useMemo(() => bindingsQuery.data?.bindings ?? [], [bindingsQuery.data])
  const strategyRows = useMemo(() => strategyQuery.data?.items ?? [], [strategyQuery.data])
  const functionalAreas = useMemo(() => functionalAreaQuery.data?.items ?? [], [functionalAreaQuery.data])
  const activationState = useMemo<CapabilityPolicyState | null>(() => activationStateQuery.data ?? null, [activationStateQuery.data])
  const isRegistryEditing = registryFormMode === 'edit'
  const isRegistryForking = registryFormMode === 'fork'
  const hasRegistryKeyLock = registryFormMode !== 'create'

  function resetRegistryFormState() {
    setRegistryForm(defaultRegistryForm(asOf))
    setRegistryFormMode('create')
  }

  const onEditStrategyRow = useCallback((row: SetIDStrategyRegistryItem) => {
    setRegistryForm(toRegistryFormFromRow(row))
    setRegistryFormMode('edit')
    setRegistryNotice(null)
  }, [])

  function onForkStrategyFromCurrent() {
    const nextEffectiveDate = registryForm.effectiveDate.trim().length > 0 ? nextDayISO(registryForm.effectiveDate) : asOf
    setRegistryForm((previous) => ({
      ...previous,
      effectiveDate: nextEffectiveDate || asOf,
      endDate: '',
      requestID: newRequestID('mui-setid-strategy-fork')
    }))
    setRegistryFormMode('fork')
    setRegistryNotice('已切换为“另存为新版本”模式，请确认新的 effective_date。')
  }

  const onOpenDisableDialog = useCallback((row: SetIDStrategyRegistryItem) => {
    const fallbackDisableAsOf = nextDayISO(row.effective_date) || asOf
    const disableAsOf = asOf > row.effective_date ? asOf : fallbackDisableAsOf
    setRegistryDisableDialog({
      open: true,
      row,
      disableAsOf
    })
    setError(null)
    setRegistryNotice(null)
  }, [asOf])

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
        minWidth: 320,
        valueGetter: (_, row: SetIDStrategyRegistryItem) =>
          `${row.required ? 'required' : 'optional'} · ${row.visible ? 'visible' : 'hidden'} · ${
            row.maintainable ? 'maintainable' : 'system-managed'
          } · ${
            row.default_rule_ref || row.default_value || '-'
          }`
      },
      { field: 'priority', headerName: 'priority', minWidth: 100 },
      { field: 'effective_date', headerName: 'effective_date', minWidth: 130 },
      { field: 'end_date', headerName: 'end_date', minWidth: 120 },
      { field: 'updated_at', headerName: 'updated_at', minWidth: 180 },
      {
        field: 'actions',
        headerName: 'actions',
        minWidth: 180,
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<SetIDStrategyRegistryItem>) => (
          <Stack direction='row' spacing={0.5}>
            <Button
              disabled={!canManageGovernance}
              onClick={(event) => {
                event.stopPropagation()
                onEditStrategyRow(params.row)
              }}
              size='small'
              variant='text'
            >
              编辑
            </Button>
            <Button
              color='error'
              disabled={!canManageGovernance}
              onClick={(event) => {
                event.stopPropagation()
                onOpenDisableDialog(params.row)
              }}
              size='small'
              variant='text'
            >
              删除
            </Button>
          </Stack>
        )
      }
    ],
    [canManageGovernance, onEditStrategyRow, onOpenDisableDialog]
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
        maintainable: registryForm.maintainable,
        default_rule_ref: registryForm.defaultRuleRef.trim(),
        default_value: registryForm.defaultValue.trim(),
        allowed_value_codes: parseAllowedValueCodes(registryForm.allowedValueCodes),
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

  async function onDisableStrategy() {
    const row = registryDisableDialog.row
    if (!row) {
      return
    }
    const disableAsOf = registryDisableDialog.disableAsOf.trim()
    setError(null)
    setRegistryNotice(null)
    if (disableAsOf.length === 0) {
      setError('disable_as_of 不能为空')
      return
    }
    if (disableAsOf <= row.effective_date) {
      setError('disable_as_of 必须晚于 effective_date')
      return
    }
    try {
      await strategyDisableMutation.mutateAsync({
        capability_key: row.capability_key,
        field_key: row.field_key,
        org_level: row.org_level,
        business_unit_id: row.business_unit_id ?? '',
        effective_date: row.effective_date,
        disable_as_of: disableAsOf,
        request_id: newRequestID('mui-setid-strategy-disable')
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  async function onSwitchFunctionalArea(functionalAreaKey: string, enabled: boolean) {
    setError(null)
    setRegistryNotice(null)
    try {
      await functionalAreaSwitchMutation.mutateAsync({
        functional_area_key: functionalAreaKey,
        enabled,
        operator: functionalAreaOperator.trim() || 'mui-operator'
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  async function onSetPolicyDraft(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setActivationNotice(null)
    try {
      await policyDraftMutation.mutateAsync({
        capability_key: activationForm.capabilityKey.trim(),
        draft_policy_version: activationForm.draftPolicyVersion.trim(),
        operator: activationForm.operator.trim() || 'mui-operator'
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  async function onActivatePolicy(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setActivationNotice(null)
    try {
      await policyActivateMutation.mutateAsync({
        capability_key: activationForm.capabilityKey.trim(),
        target_policy_version: activationForm.activatePolicyVersion.trim(),
        operator: activationForm.operator.trim() || 'mui-operator'
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  async function onRollbackPolicy(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setActivationNotice(null)
    try {
      await policyRollbackMutation.mutateAsync({
        capability_key: activationForm.capabilityKey.trim(),
        target_policy_version: activationForm.rollbackPolicyVersion.trim() || undefined,
        operator: activationForm.operator.trim() || 'mui-operator'
      })
    } catch (err) {
      setError(parseApiError(err))
    }
  }

  return (
    <Box>
      <PageHeader title='SetID Governance' subtitle='Registry / Explain / Functional Area / Activation' />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}
        {registryNotice ? <Alert severity='success'>{registryNotice}</Alert> : null}
        {activationNotice ? <Alert severity='success'>{activationNotice}</Alert> : null}
        {!canManageGovernance ? (
          <Alert severity='warning'>当前为只读骨架模式：可查看页面信息，但关键操作已禁用。请申请 setid.governance.manage 权限。</Alert>
        ) : null}

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
          <Tab label='Registry' value='registry' />
          <Tab label='Explain' value='explain' />
          <Tab label='Functional Area' value='functional-area' />
          <Tab label='Activation' value='activation' />
        </Tabs>

        {tab === 'registry' ? (
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
                <Button disabled={!canManageGovernance || createMutation.isPending} type='submit' variant='contained'>
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
                <Button disabled={!canManageGovernance || bindMutation.isPending} type='submit' variant='contained'>
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

        {tab === 'explain' ? (
          <>
            <Alert severity='info'>
              Explain 默认展示 brief；若无 full 权限将自动降级为只读 brief，并提供申请提示。
            </Alert>
            <SetIDExplainPanel
              initialAsOf={asOf}
              title='Explain'
              subtitle='用于验证 capability 命中/拒绝原因，并展示 trace_id/request_id/policy_version。'
              defaultLevel={canViewFullExplain ? 'full' : 'brief'}
            />
          </>
        ) : null}

        {tab === 'registry' ? (
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
              {isRegistryEditing ? (
                <Alert severity='info' sx={{ mb: 1.5 }}>
                  当前为“编辑当前版本”模式：主键字段已锁定。你可以直接保存，或点击“另存为新版本”创建新生效日记录。
                </Alert>
              ) : null}
              {isRegistryForking ? (
                <Alert severity='info' sx={{ mb: 1.5 }}>
                  当前为“另存为新版本”模式：仅 effective_date 可修改，请确认后保存。
                </Alert>
              ) : null}
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
                    disabled={hasRegistryKeyLock}
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
                    disabled={hasRegistryKeyLock}
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
                    disabled={hasRegistryKeyLock}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        orgLevel: event.target.value as RegistryFormState['orgLevel']
                      }))
                    }
                  />
                  <TextField
                    label='business_unit_id'
                    size='small'
                    value={registryForm.businessUnitID}
                    disabled={hasRegistryKeyLock}
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
                    label='allowed_value_codes (csv)'
                    size='small'
                    value={registryForm.allowedValueCodes}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, allowedValueCodes: event.target.value }))}
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
                    disabled={isRegistryEditing}
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
                        checked={registryForm.maintainable}
                        onChange={(event) => setRegistryForm((prev) => ({ ...prev, maintainable: event.target.checked }))}
                      />
                    }
                    label='maintainable'
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
                  <Button disabled={!canManageGovernance || strategyMutation.isPending} type='submit' variant='contained'>
                    {isRegistryEditing ? '保存当前版本' : isRegistryForking ? '保存为新版本' : '保存策略'}
                  </Button>
                  {isRegistryEditing ? (
                    <Button
                      type='button'
                      disabled={!canManageGovernance || strategyMutation.isPending}
                      onClick={onForkStrategyFromCurrent}
                      variant='outlined'
                    >
                      另存为新版本
                    </Button>
                  ) : null}
                  {hasRegistryKeyLock ? (
                    <Button
                      type='button'
                      disabled={strategyMutation.isPending}
                      onClick={resetRegistryFormState}
                      variant='outlined'
                    >
                      取消编辑
                    </Button>
                  ) : null}
                  <Button
                    type='button'
                    onClick={resetRegistryFormState}
                    variant='outlined'
                  >
                    {hasRegistryKeyLock ? '新建空白' : '重置表单'}
                  </Button>
                </Stack>
              </Stack>
            </Paper>
          </>
        ) : null}

        {tab === 'functional-area' ? (
          <Paper sx={{ p: 2 }} variant='outlined'>
            <Stack spacing={1.5}>
              <Typography variant='subtitle1'>Functional Area</Typography>
              <Typography color='text.secondary' variant='body2'>
                展示租户功能域生命周期与开关状态；reserved/deprecated 会自动 fail-closed。
              </Typography>
              <TextField
                label='operator'
                size='small'
                value={functionalAreaOperator}
                onChange={(event) => setFunctionalAreaOperator(event.target.value)}
                sx={{ maxWidth: 320 }}
              />
              {functionalAreaQuery.isError ? <Alert severity='error'>Functional Area 加载失败</Alert> : null}
              <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'auto' }}>
                <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                  <thead>
                    <tr style={{ background: '#fff' }}>
                      <th align='left'>functional_area_key</th>
                      <th align='left'>lifecycle_status</th>
                      <th align='left'>enabled</th>
                      <th align='left'>operation</th>
                    </tr>
                  </thead>
                  <tbody>
                    {functionalAreas.map((item: FunctionalAreaStateItem) => (
                      <tr key={item.functional_area_key} style={{ borderTop: '1px solid #eee' }}>
                        <td>{item.functional_area_key}</td>
                        <td>{item.lifecycle_status}</td>
                        <td>{item.enabled ? 'true' : 'false'}</td>
                        <td>
                          <Stack direction='row' spacing={1}>
                            <Button
                              disabled={!canManageGovernance || item.lifecycle_status !== 'active' || !item.enabled}
                              onClick={() => void onSwitchFunctionalArea(item.functional_area_key, false)}
                              size='small'
                              variant='outlined'
                            >
                              Disable
                            </Button>
                            <Button
                              disabled={!canManageGovernance || item.lifecycle_status !== 'active' || item.enabled}
                              onClick={() => void onSwitchFunctionalArea(item.functional_area_key, true)}
                              size='small'
                              variant='outlined'
                            >
                              Enable
                            </Button>
                          </Stack>
                        </td>
                      </tr>
                    ))}
                    {functionalAreas.length === 0 ? (
                      <tr>
                        <td colSpan={4} style={{ padding: 16, textAlign: 'center' }}>
                          暂无功能域数据
                        </td>
                      </tr>
                    ) : null}
                  </tbody>
                </table>
              </Box>
            </Stack>
          </Paper>
        ) : null}

        {tab === 'activation' ? (
          <Paper sx={{ p: 2 }} variant='outlined'>
            <Stack spacing={1.5}>
              <Typography variant='subtitle1'>Activation</Typography>
              <Typography color='text.secondary' variant='body2'>
                支持 draft / activate / rollback，并展示当前 active 版本。
              </Typography>
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
                <TextField
                  label='capability_key'
                  size='small'
                  value={activationForm.capabilityKey}
                  onChange={(event) => setActivationForm((prev) => ({ ...prev, capabilityKey: event.target.value }))}
                />
                <TextField
                  label='operator'
                  size='small'
                  value={activationForm.operator}
                  onChange={(event) => setActivationForm((prev) => ({ ...prev, operator: event.target.value }))}
                />
              </Box>
              {activationState ? (
                <Alert severity='info'>
                  active={activationState.active_policy_version} · draft={activationState.draft_policy_version || '-'} · state=
                  {activationState.activation_state}
                </Alert>
              ) : null}
              {activationStateQuery.isError ? <Alert severity='error'>Activation 状态加载失败</Alert> : null}

              <Stack component='form' direction={{ xs: 'column', md: 'row' }} spacing={1} onSubmit={(event) => void onSetPolicyDraft(event)}>
                <TextField
                  label='draft_policy_version'
                  size='small'
                  value={activationForm.draftPolicyVersion}
                  onChange={(event) => setActivationForm((prev) => ({ ...prev, draftPolicyVersion: event.target.value }))}
                />
                <Button disabled={!canManageGovernance || policyDraftMutation.isPending} type='submit' variant='contained'>
                  Set Draft
                </Button>
              </Stack>

              <Stack component='form' direction={{ xs: 'column', md: 'row' }} spacing={1} onSubmit={(event) => void onActivatePolicy(event)}>
                <TextField
                  label='target_policy_version'
                  size='small'
                  value={activationForm.activatePolicyVersion}
                  onChange={(event) => setActivationForm((prev) => ({ ...prev, activatePolicyVersion: event.target.value }))}
                />
                <Button disabled={!canManageGovernance || policyActivateMutation.isPending} type='submit' variant='contained'>
                  Activate
                </Button>
              </Stack>

              <Stack component='form' direction={{ xs: 'column', md: 'row' }} spacing={1} onSubmit={(event) => void onRollbackPolicy(event)}>
                <TextField
                  label='rollback_target_version（可空）'
                  size='small'
                  value={activationForm.rollbackPolicyVersion}
                  onChange={(event) => setActivationForm((prev) => ({ ...prev, rollbackPolicyVersion: event.target.value }))}
                />
                <Button disabled={!canManageGovernance || policyRollbackMutation.isPending} type='submit' variant='outlined'>
                  Rollback
                </Button>
              </Stack>
            </Stack>
          </Paper>
        ) : null}

        <Dialog
          open={registryDisableDialog.open}
          onClose={() => {
            if (strategyDisableMutation.isPending) {
              return
            }
            setRegistryDisableDialog({ open: false, row: null, disableAsOf: '' })
          }}
          fullWidth
          maxWidth='sm'
        >
          <DialogTitle>停用策略</DialogTitle>
          <DialogContent>
            <Stack spacing={1.5} sx={{ mt: 0.5 }}>
              <Typography color='text.secondary' variant='body2'>
                capability_key={registryDisableDialog.row?.capability_key || '-'}
              </Typography>
              <Typography color='text.secondary' variant='body2'>
                field_key={registryDisableDialog.row?.field_key || '-'} · org_level={registryDisableDialog.row?.org_level || '-'} ·
                business_unit_id={registryDisableDialog.row?.business_unit_id || '-'}
              </Typography>
              <Typography color='text.secondary' variant='body2'>
                effective_date={registryDisableDialog.row?.effective_date || '-'}
              </Typography>
              <TextField
                label='disable_as_of'
                type='date'
                value={registryDisableDialog.disableAsOf}
                onChange={(event) =>
                  setRegistryDisableDialog((previous) => ({
                    ...previous,
                    disableAsOf: event.target.value
                  }))
                }
              />
            </Stack>
          </DialogContent>
          <DialogActions>
            <Button
              type='button'
              onClick={() => setRegistryDisableDialog({ open: false, row: null, disableAsOf: '' })}
              disabled={strategyDisableMutation.isPending}
            >
              取消
            </Button>
            <Button
              type='button'
              color='error'
              variant='contained'
              onClick={() => void onDisableStrategy()}
              disabled={!canManageGovernance || strategyDisableMutation.isPending}
            >
              确认停用
            </Button>
          </DialogActions>
        </Dialog>
      </Stack>
    </Box>
  )
}
