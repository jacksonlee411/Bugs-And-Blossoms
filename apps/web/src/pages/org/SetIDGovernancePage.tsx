import { type FormEvent, useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
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
import { listOrgUnits } from '../../api/orgUnits'
import {
  activatePolicyVersion,
  bindSetID,
  createSetID,
  disableSetIDStrategyRegistry,
  getPolicyActivationState,
  listCapabilityCatalogByIntent,
  listSetIDBindings,
  listFunctionalAreaState,
  listSetIDStrategyRegistry,
  listSetIDs,
  rollbackPolicyVersion,
  setPolicyDraft,
  switchFunctionalAreaState,
  upsertSetIDStrategyRegistry,
  type CapabilityCatalogEntry,
  type CapabilityPolicyState,
  type FunctionalAreaStateItem,
  type SetIDStrategyRegistryItem
} from '../../api/setids'
import { DataGridPage } from '../../components/DataGridPage'
import { FreeSoloDropdownField, mergeFreeSoloOptions, uniqueNonEmptyStrings } from '../../components/FreeSoloDropdownField'
import { PageHeader } from '../../components/PageHeader'
import { SetIDExplainPanel } from '../../components/SetIDExplainPanel'
import { resolveApiErrorMessage } from '../../errors/presentApiError'

type SetIDPageTab = 'registry' | 'explain' | 'functional-area' | 'activation'

interface RegistryFormState {
  capabilityKey: string
  ownerModule: string
  fieldKey: string
  personalizationMode: 'tenant_only' | 'setid'
  orgApplicability: 'tenant' | 'business_unit'
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

interface DropdownOption {
  value: string
  label: string
}

const ownerModulePresets = ['iam', 'orgunit', 'jobcatalog', 'staffing', 'person']
const personalizationModeOptions: DropdownOption[] = [
  { value: 'setid', label: 'setid' },
  { value: 'tenant_only', label: 'tenant_only' }
]
const orgApplicabilityOptions: DropdownOption[] = [
  { value: 'business_unit', label: 'business_unit' },
  { value: 'tenant', label: 'tenant' }
]
const changePolicyPresets = ['plan_required']
const priorityPresets = [50, 100, 200, 500]

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

interface RegistryCatalogSelection {
  ownerModule: string
  targetObject: string
  surface: string
  intent: string
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
    orgApplicability: 'business_unit',
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

function defaultRegistryCatalogSelection(): RegistryCatalogSelection {
  return {
    ownerModule: '',
    targetObject: '',
    surface: '',
    intent: ''
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
    orgApplicability: row.org_applicability,
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
    item.org_applicability,
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
  const [registryMode, setRegistryMode] = useState<'object_intent' | 'advanced'>('object_intent')
  const [registryCatalog, setRegistryCatalog] = useState<RegistryCatalogSelection>(() => defaultRegistryCatalogSelection())
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
  const capabilityCatalogQuery = useQuery({
    queryKey: ['capability-catalog'],
    queryFn: () => listCapabilityCatalogByIntent(),
    staleTime: 10_000
  })
  const strategyCatalogQuery = useQuery({
    queryKey: ['setid-strategy-registry-options', asOf],
    queryFn: () => listSetIDStrategyRegistry({ asOf }),
    staleTime: 10_000
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
      await queryClient.invalidateQueries({ queryKey: ['setid-strategy-registry-options', asOf] })
      setRegistryForm(defaultRegistryForm(asOf))
      setRegistryMode('object_intent')
      setRegistryCatalog(defaultRegistryCatalogSelection())
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
  const orgUnits = useMemo(() => orgUnitsQuery.data?.org_units ?? [], [orgUnitsQuery.data])
  const strategyRows = useMemo(() => strategyQuery.data?.items ?? [], [strategyQuery.data])
  const strategyCatalogRows = useMemo(() => strategyCatalogQuery.data?.items ?? [], [strategyCatalogQuery.data])
  const capabilityCatalogRows = useMemo(() => capabilityCatalogQuery.data?.items ?? [], [capabilityCatalogQuery.data])
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
    () =>
      mergeFreeSoloOptions(
        [],
        [...strategyCatalogRows.map((item) => item.capability_key), ...capabilityCatalogRows.map((item) => item.capability_key)],
        registryForm.capabilityKey
      ),
    [capabilityCatalogRows, registryForm.capabilityKey, strategyCatalogRows]
  )
  const fieldKeyOptions = useMemo(
    () => mergeFreeSoloOptions([], strategyCatalogRows.map((item) => item.field_key), registryForm.fieldKey),
    [registryForm.fieldKey, strategyCatalogRows]
  )
  const preferredCatalogSelection = useMemo<RegistryCatalogSelection>(() => {
    const preferred =
      capabilityCatalogRows.find(
        (item) =>
          item.owner_module === 'orgunit' &&
          item.target_object === 'orgunit' &&
          item.surface === 'api_write' &&
          item.intent === 'write_all'
      ) ?? capabilityCatalogRows[0]
    if (!preferred) {
      return defaultRegistryCatalogSelection()
    }
    return {
      ownerModule: preferred.owner_module,
      targetObject: preferred.target_object,
      surface: preferred.surface,
      intent: preferred.intent
    }
  }, [capabilityCatalogRows])
  const effectiveRegistryCatalog = useMemo<RegistryCatalogSelection>(() => {
    if (
      registryCatalog.ownerModule.trim().length > 0 ||
      registryCatalog.targetObject.trim().length > 0 ||
      registryCatalog.surface.trim().length > 0 ||
      registryCatalog.intent.trim().length > 0
    ) {
      return registryCatalog
    }
    return preferredCatalogSelection
  }, [preferredCatalogSelection, registryCatalog])
  const ownerModuleOptions = useMemo(
    () =>
      mergeFreeSoloOptions(
        ownerModulePresets,
        [...strategyCatalogRows.map((item) => item.owner_module), ...capabilityCatalogRows.map((item) => item.owner_module)],
        effectiveRegistryCatalog.ownerModule || registryForm.ownerModule
      ),
    [capabilityCatalogRows, effectiveRegistryCatalog.ownerModule, registryForm.ownerModule, strategyCatalogRows]
  )
  const targetObjectOptions = useMemo(
    () =>
      mergeFreeSoloOptions(
        [],
        capabilityCatalogRows
          .filter((item) => effectiveRegistryCatalog.ownerModule.trim().length === 0 || item.owner_module === effectiveRegistryCatalog.ownerModule.trim())
          .map((item) => item.target_object),
        effectiveRegistryCatalog.targetObject
      ),
    [capabilityCatalogRows, effectiveRegistryCatalog.ownerModule, effectiveRegistryCatalog.targetObject]
  )
  const surfaceOptions = useMemo(
    () =>
      mergeFreeSoloOptions(
        [],
        capabilityCatalogRows
          .filter((item) => effectiveRegistryCatalog.ownerModule.trim().length === 0 || item.owner_module === effectiveRegistryCatalog.ownerModule.trim())
          .filter((item) => effectiveRegistryCatalog.targetObject.trim().length === 0 || item.target_object === effectiveRegistryCatalog.targetObject.trim())
          .map((item) => item.surface),
        effectiveRegistryCatalog.surface
      ),
    [capabilityCatalogRows, effectiveRegistryCatalog.ownerModule, effectiveRegistryCatalog.surface, effectiveRegistryCatalog.targetObject]
  )
  const intentOptions = useMemo(
    () =>
      mergeFreeSoloOptions(
        [],
        capabilityCatalogRows
          .filter((item) => effectiveRegistryCatalog.ownerModule.trim().length === 0 || item.owner_module === effectiveRegistryCatalog.ownerModule.trim())
          .filter((item) => effectiveRegistryCatalog.targetObject.trim().length === 0 || item.target_object === effectiveRegistryCatalog.targetObject.trim())
          .filter((item) => effectiveRegistryCatalog.surface.trim().length === 0 || item.surface === effectiveRegistryCatalog.surface.trim())
          .map((item) => item.intent),
        effectiveRegistryCatalog.intent
      ),
    [capabilityCatalogRows, effectiveRegistryCatalog.intent, effectiveRegistryCatalog.ownerModule, effectiveRegistryCatalog.surface, effectiveRegistryCatalog.targetObject]
  )
  const selectedCatalogEntry = useMemo<CapabilityCatalogEntry | null>(() => {
    const ownerModule = effectiveRegistryCatalog.ownerModule.trim()
    const targetObject = effectiveRegistryCatalog.targetObject.trim()
    const surface = effectiveRegistryCatalog.surface.trim()
    const intent = effectiveRegistryCatalog.intent.trim()
    if (ownerModule.length === 0 || targetObject.length === 0 || surface.length === 0 || intent.length === 0) {
      return null
    }
    return (
      capabilityCatalogRows.find(
        (item) =>
          item.owner_module === ownerModule &&
          item.target_object === targetObject &&
          item.surface === surface &&
          item.intent === intent
      ) ?? null
    )
  }, [capabilityCatalogRows, effectiveRegistryCatalog.intent, effectiveRegistryCatalog.ownerModule, effectiveRegistryCatalog.surface, effectiveRegistryCatalog.targetObject])
  const catalogByCapabilityKey = useMemo(
    () =>
      capabilityCatalogRows.reduce<Record<string, CapabilityCatalogEntry>>((acc, item) => {
        acc[item.capability_key] = item
        return acc
      }, {}),
    [capabilityCatalogRows]
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
  const functionalAreas = useMemo(() => functionalAreaQuery.data?.items ?? [], [functionalAreaQuery.data])
  const activationState = useMemo<CapabilityPolicyState | null>(() => activationStateQuery.data ?? null, [activationStateQuery.data])
  const isRegistryEditing = registryFormMode === 'edit'
  const isRegistryForking = registryFormMode === 'fork'
  const hasRegistryKeyLock = registryFormMode !== 'create'

  function resetRegistryFormState() {
    setRegistryForm(defaultRegistryForm(asOf))
    setRegistryMode('object_intent')
    setRegistryCatalog(defaultRegistryCatalogSelection())
    setRegistryFormMode('create')
  }

  const onEditStrategyRow = useCallback((row: SetIDStrategyRegistryItem) => {
    setRegistryForm(toRegistryFormFromRow(row))
    const entry = catalogByCapabilityKey[row.capability_key]
    if (entry) {
      setRegistryMode('object_intent')
      setRegistryCatalog({
        ownerModule: entry.owner_module,
        targetObject: entry.target_object,
        surface: entry.surface,
        intent: entry.intent
      })
    } else {
      setRegistryMode('advanced')
      setRegistryCatalog(defaultRegistryCatalogSelection())
    }
    setRegistryFormMode('edit')
    setRegistryNotice(null)
  }, [catalogByCapabilityKey])

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
      {
        field: 'source_type',
        headerName: 'source_type',
        minWidth: 130,
        valueGetter: (_, row: SetIDStrategyRegistryItem) => row.source_type ?? '-'
      },
      {
        field: 'target_object',
        headerName: 'target_object',
        minWidth: 130,
        valueGetter: (_, row: SetIDStrategyRegistryItem) => catalogByCapabilityKey[row.capability_key]?.target_object ?? '-'
      },
      {
        field: 'surface',
        headerName: 'surface',
        minWidth: 130,
        valueGetter: (_, row: SetIDStrategyRegistryItem) => catalogByCapabilityKey[row.capability_key]?.surface ?? '-'
      },
      {
        field: 'intent',
        headerName: 'intent',
        minWidth: 140,
        valueGetter: (_, row: SetIDStrategyRegistryItem) => catalogByCapabilityKey[row.capability_key]?.intent ?? '-'
      },
      { field: 'field_key', headerName: 'field_key', minWidth: 140 },
      { field: 'personalization_mode', headerName: 'mode', minWidth: 130 },
      { field: 'org_applicability', headerName: 'org_applicability', minWidth: 120 },
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
    [canManageGovernance, catalogByCapabilityKey, onEditStrategyRow, onOpenDisableDialog]
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
    let capabilityKey = registryForm.capabilityKey.trim()
    let ownerModule = registryForm.ownerModule.trim()
    if (registryMode === 'object_intent') {
      if (selectedCatalogEntry == null) {
        setError('请先选择对象/意图，系统会自动回填 capability_key。')
        return
      }
      capabilityKey = selectedCatalogEntry.capability_key
      ownerModule = selectedCatalogEntry.owner_module
    } else {
      const catalogEntry = catalogByCapabilityKey[capabilityKey]
      if (!catalogEntry) {
        setError('capability_key 未在能力目录注册，请改用对象/意图模式或修正 capability_key。')
        return
      }
      ownerModule = catalogEntry.owner_module
    }
    try {
      await strategyMutation.mutateAsync({
        capability_key: capabilityKey,
        owner_module: ownerModule,
        field_key: registryForm.fieldKey.trim(),
        personalization_mode: registryForm.personalizationMode,
        org_applicability: registryForm.orgApplicability,
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
        org_applicability: row.org_applicability,
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
              {capabilityCatalogQuery.isLoading ? (
                <Alert severity='info' sx={{ mb: 1.5 }}>
                  对象/意图目录加载中…
                </Alert>
              ) : null}
              {capabilityCatalogQuery.isError ? (
                <Alert severity='warning' sx={{ mb: 1.5 }}>
                  对象/意图目录暂不可用，请切换到高级模式并稍后重试。
                </Alert>
              ) : null}
              <Stack
                component='form'
                spacing={1.5}
                onSubmit={(event) => {
                  void onUpsertStrategy(event)
                }}
              >
                <Stack direction={{ xs: 'column', md: 'row' }} spacing={1}>
                  <TextField
                    label='配置模式'
                    select
                    size='small'
                    value={registryMode}
                    onChange={(event) => setRegistryMode(event.target.value as 'object_intent' | 'advanced')}
                  >
                    <MenuItem value='object_intent'>对象/意图模式（默认）</MenuItem>
                    <MenuItem value='advanced'>高级 capability_key</MenuItem>
                  </TextField>
                  <Button
                    type='button'
                    variant='outlined'
                    onClick={() => {
                      setRegistryMode('object_intent')
                      setRegistryCatalog({
                        ownerModule: 'orgunit',
                        targetObject: 'orgunit',
                        surface: 'api_write',
                        intent: 'write_all'
                      })
                    }}
                  >
                    应用到全部写场景
                  </Button>
                  <Button
                    type='button'
                    variant='outlined'
                    onClick={() => {
                      setRegistryMode('object_intent')
                      setRegistryCatalog((previous) => ({
                        ownerModule: previous.ownerModule || 'orgunit',
                        targetObject: previous.targetObject || 'orgunit',
                        surface: previous.surface === 'details_dialog' ? previous.surface : 'create_dialog',
                        intent: previous.intent === 'create_org' || previous.intent === 'add_version' || previous.intent === 'insert_version' || previous.intent === 'correct'
                          ? previous.intent
                          : 'create_org'
                      }))
                    }}
                  >
                    仅当前场景覆盖
                  </Button>
                </Stack>
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
                  {registryMode === 'object_intent' ? (
                    <>
                      <FreeSoloDropdownField
                        label='owner_module'
                        onChange={(nextValue) =>
                          setRegistryCatalog({
                            ownerModule: nextValue,
                            targetObject: '',
                            surface: '',
                            intent: ''
                          })
                        }
                        options={ownerModuleOptions}
                        required
                        value={effectiveRegistryCatalog.ownerModule}
                        disabled={hasRegistryKeyLock}
                      />
                      <FreeSoloDropdownField
                        label='target_object'
                        onChange={(nextValue) =>
                          setRegistryCatalog((previous) => ({
                            ...previous,
                            targetObject: nextValue,
                            surface: '',
                            intent: ''
                          }))
                        }
                        options={targetObjectOptions}
                        required
                        value={effectiveRegistryCatalog.targetObject}
                        disabled={hasRegistryKeyLock}
                      />
                      <FreeSoloDropdownField
                        label='surface'
                        onChange={(nextValue) =>
                          setRegistryCatalog((previous) => ({
                            ...previous,
                            surface: nextValue,
                            intent: ''
                          }))
                        }
                        options={surfaceOptions}
                        required
                        value={effectiveRegistryCatalog.surface}
                        disabled={hasRegistryKeyLock}
                      />
                      <FreeSoloDropdownField
                        label='intent'
                        onChange={(nextValue) =>
                          setRegistryCatalog((previous) => ({
                            ...previous,
                            intent: nextValue
                          }))
                        }
                        options={intentOptions}
                        required
                        value={effectiveRegistryCatalog.intent}
                        disabled={hasRegistryKeyLock}
                      />
                      <TextField
                        label='capability_key（自动回填）'
                        size='small'
                        value={selectedCatalogEntry?.capability_key ?? ''}
                        InputProps={{ readOnly: true }}
                      />
                    </>
                  ) : (
                    <>
                      <FreeSoloDropdownField
                        label='capability_key'
                        onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, capabilityKey: nextValue }))}
                        options={capabilityKeyOptions}
                        required
                        value={registryForm.capabilityKey}
                        disabled={hasRegistryKeyLock}
                      />
                      <FreeSoloDropdownField
                        label='owner_module'
                        onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, ownerModule: nextValue }))}
                        options={ownerModuleOptions}
                        required
                        value={registryForm.ownerModule}
                      />
                    </>
                  )}
                  <FreeSoloDropdownField
                    label='field_key'
                    onChange={(nextValue) => setRegistryForm((prev) => ({ ...prev, fieldKey: nextValue }))}
                    options={fieldKeyOptions}
                    required
                    value={registryForm.fieldKey}
                    disabled={hasRegistryKeyLock}
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
                    label='org_applicability'
                    required
                    select
                    size='small'
                    value={registryForm.orgApplicability}
                    disabled={hasRegistryKeyLock}
                    onChange={(event) =>
                      setRegistryForm((prev) => ({
                        ...prev,
                        orgApplicability: event.target.value as RegistryFormState['orgApplicability'],
                        businessUnitID: event.target.value === 'tenant' ? '' : prev.businessUnitID
                      }))
                    }
                  >
                    {orgApplicabilityOptions.map((option) => (
                      <MenuItem key={`form-org-level-${option.value}`} value={option.value}>
                        {option.label}
                      </MenuItem>
                    ))}
                  </TextField>
                  <FreeSoloDropdownField
                    label='business_unit_id'
                    disabled={hasRegistryKeyLock || registryForm.orgApplicability === 'tenant'}
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
                    label='allowed_value_codes (csv)'
                    size='small'
                    value={registryForm.allowedValueCodes}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, allowedValueCodes: event.target.value }))}
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
                    label='maintainable'
                    select
                    size='small'
                    value={registryForm.maintainable ? 'true' : 'false'}
                    onChange={(event) => setRegistryForm((prev) => ({ ...prev, maintainable: event.target.value === 'true' }))}
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
              <FreeSoloDropdownField
                label='operator'
                onChange={setFunctionalAreaOperator}
                options={mergeFreeSoloOptions([], [functionalAreaOperator], functionalAreaOperator)}
                value={functionalAreaOperator}
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
                <FreeSoloDropdownField
                  label='capability_key'
                  onChange={(nextValue) => setActivationForm((prev) => ({ ...prev, capabilityKey: nextValue }))}
                  options={capabilityKeyOptions}
                  value={activationForm.capabilityKey}
                />
                <FreeSoloDropdownField
                  label='operator'
                  onChange={(nextValue) => setActivationForm((prev) => ({ ...prev, operator: nextValue }))}
                  options={mergeFreeSoloOptions([], [activationForm.operator, functionalAreaOperator], activationForm.operator)}
                  value={activationForm.operator}
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
                <FreeSoloDropdownField
                  label='draft_policy_version'
                  onChange={(nextValue) => setActivationForm((prev) => ({ ...prev, draftPolicyVersion: nextValue }))}
                  options={mergeFreeSoloOptions([], [activationState?.draft_policy_version ?? '', activationForm.draftPolicyVersion], activationForm.draftPolicyVersion)}
                  value={activationForm.draftPolicyVersion}
                />
                <Button disabled={!canManageGovernance || policyDraftMutation.isPending} type='submit' variant='contained'>
                  Set Draft
                </Button>
              </Stack>

              <Stack component='form' direction={{ xs: 'column', md: 'row' }} spacing={1} onSubmit={(event) => void onActivatePolicy(event)}>
                <FreeSoloDropdownField
                  label='target_policy_version'
                  onChange={(nextValue) => setActivationForm((prev) => ({ ...prev, activatePolicyVersion: nextValue }))}
                  options={mergeFreeSoloOptions([], [activationState?.draft_policy_version ?? '', activationState?.active_policy_version ?? '', activationForm.activatePolicyVersion], activationForm.activatePolicyVersion)}
                  value={activationForm.activatePolicyVersion}
                />
                <Button disabled={!canManageGovernance || policyActivateMutation.isPending} type='submit' variant='contained'>
                  Activate
                </Button>
              </Stack>

              <Stack component='form' direction={{ xs: 'column', md: 'row' }} spacing={1} onSubmit={(event) => void onRollbackPolicy(event)}>
                <FreeSoloDropdownField
                  label='rollback_target_version（可空）'
                  onChange={(nextValue) => setActivationForm((prev) => ({ ...prev, rollbackPolicyVersion: nextValue }))}
                  options={mergeFreeSoloOptions([], [activationState?.active_policy_version ?? '', activationForm.rollbackPolicyVersion], activationForm.rollbackPolicyVersion)}
                  value={activationForm.rollbackPolicyVersion}
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
                field_key={registryDisableDialog.row?.field_key || '-'} · org_applicability={registryDisableDialog.row?.org_applicability || '-'} ·
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
