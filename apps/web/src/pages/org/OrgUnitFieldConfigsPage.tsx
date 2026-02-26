import { useCallback, useMemo, useState } from 'react'
import { Link as RouterLink, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Breadcrumbs,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  Link,
  MenuItem,
  Paper,
  Select,
  Snackbar,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef } from '@mui/x-data-grid'
import {
  disableOrgUnitFieldConfig,
  enableOrgUnitFieldConfig,
  listOrgUnitFieldConfigEnableCandidates,
  listOrgUnitFieldConfigs,
  listOrgUnitFieldDefinitions,
  upsertOrgUnitFieldPolicy,
  type OrgUnitExtValueType,
  type OrgUnitFieldDefinition,
  type OrgUnitFieldEnableCandidateField,
  type OrgUnitFieldPolicyDefaultMode,
  type OrgUnitFieldPolicyScopeType,
  type OrgUnitTenantFieldConfig
} from '../../api/orgUnits'
import { ApiClientError } from '../../api/errors'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { isMessageKey, type MessageKey } from '../../i18n/messages'
import { resolveAsOfAfterPolicySave, shouldShowFutureEffectiveHint } from './orgUnitFieldPolicyAsOf'

type FieldConfigListStatus = 'all' | 'enabled' | 'disabled'
type DisabledState = 'all' | 'pending' | 'disabled'
type RowState = 'enabled' | 'pending' | 'disabled'

interface FieldConfigRow {
  id: string
  fieldKey: string
  fieldClass: 'CORE' | 'EXT'
  fieldLabel: string
  valueType: string
  dataSourceType: string
  dataSourceConfig: Record<string, unknown>
  physicalCol: string
  enabledOn: string
  disabledOn: string | null
  updatedAt: string
  maintainable: boolean
  defaultMode: 'NONE' | 'CEL'
  defaultRuleExpr: string
  policyScopeType: string
  policyScopeKey: string
  state: RowState
}

interface EnableFormState {
  mode: 'builtin' | 'dict' | 'custom'
  fieldKey: string
  customValueType: OrgUnitExtValueType
  customDisplayLabel: string
  enabledOn: string
  dataSourceConfigOption: string
  dictDisplayLabel: string
}

interface DisableFormState {
  disabledOn: string
}

interface SelectedConfigState {
  mode: 'disable' | 'postpone'
  row: FieldConfigRow
}

interface PolicyFormState {
  fieldKey: string
  scopeType: OrgUnitFieldPolicyScopeType
  scopeKey: string
  maintainable: boolean
  defaultMode: OrgUnitFieldPolicyDefaultMode
  defaultRuleExpr: string
  enabledOn: string
}

function FieldConfigsFilterBar(props: {
  asOf: string
  status: FieldConfigListStatus
  disabledState: DisabledState
  keyword: string
  t: ReturnType<typeof useAppPreferences>['t']
  onApply: (value: { asOf: string; status: FieldConfigListStatus; disabledState: DisabledState; keyword: string }) => void
}) {
  const { t } = props
  const [asOfInput, setAsOfInput] = useState(props.asOf)
  const [statusInput, setStatusInput] = useState<FieldConfigListStatus>(props.status)
  const [disabledStateInput, setDisabledStateInput] = useState<DisabledState>(props.disabledState)
  const [keywordInput, setKeywordInput] = useState(props.keyword)

  return (
    <FilterBar>
      <TextField
        InputLabelProps={{ shrink: true }}
        label={t('org_field_configs_filter_as_of')}
        onChange={(event) => setAsOfInput(event.target.value)}
        type='date'
        value={asOfInput}
      />
      <FormControl sx={{ minWidth: 160 }}>
        <InputLabel id='org-field-configs-status-filter'>{t('org_field_configs_filter_status')}</InputLabel>
        <Select
          label={t('org_field_configs_filter_status')}
          labelId='org-field-configs-status-filter'
          onChange={(event) => {
            const next = String(event.target.value) as FieldConfigListStatus
            setStatusInput(next)
            if (next !== 'disabled') {
              setDisabledStateInput('all')
            }
          }}
          value={statusInput}
        >
          <MenuItem value='all'>{t('status_all')}</MenuItem>
          <MenuItem value='enabled'>{t('org_field_configs_state_enabled')}</MenuItem>
          <MenuItem value='disabled'>{t('org_field_configs_state_disabled_bucket')}</MenuItem>
        </Select>
      </FormControl>
      {statusInput === 'disabled' ? (
        <FormControl sx={{ minWidth: 160 }}>
          <InputLabel id='org-field-configs-disabled-state-filter'>{t('org_field_configs_filter_disabled_state')}</InputLabel>
          <Select
            label={t('org_field_configs_filter_disabled_state')}
            labelId='org-field-configs-disabled-state-filter'
            onChange={(event) => setDisabledStateInput(String(event.target.value) as DisabledState)}
            value={disabledStateInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='pending'>{t('org_field_configs_state_pending')}</MenuItem>
            <MenuItem value='disabled'>{t('org_field_configs_state_disabled')}</MenuItem>
          </Select>
        </FormControl>
      ) : null}
      <TextField
        fullWidth
        label={t('org_field_configs_filter_keyword')}
        onChange={(event) => setKeywordInput(event.target.value)}
        value={keywordInput}
      />
      <Button
        onClick={() =>
          props.onApply({
            asOf: asOfInput,
            status: statusInput,
            disabledState: statusInput === 'disabled' ? disabledStateInput : 'all',
            keyword: keywordInput
          })
        }
        variant='contained'
      >
        {t('action_apply_filters')}
      </Button>
    </FilterBar>
  )
}

function formatAsOfDate(date: Date): string {
  return date.toISOString().slice(0, 10)
}

function parseDateOrDefault(raw: string | null, fallback: string): string {
  if (!raw) {
    return fallback
  }
  const value = raw.trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return fallback
  }
  return value
}

function parseListStatus(raw: string | null): FieldConfigListStatus {
  const value = (raw ?? '').trim().toLowerCase()
  if (value === 'enabled' || value === 'disabled') {
    return value
  }
  return 'all'
}

function parseDisabledState(raw: string | null): DisabledState {
  const value = (raw ?? '').trim().toLowerCase()
  if (value === 'pending' || value === 'disabled') {
    return value
  }
  return 'all'
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function mapFieldPolicyErrorKey(code: string): MessageKey | null {
  switch (code) {
  case 'FIELD_NOT_MAINTAINABLE':
    return 'org_field_policy_error_FIELD_NOT_MAINTAINABLE'
  case 'DEFAULT_RULE_REQUIRED':
    return 'org_field_policy_error_DEFAULT_RULE_REQUIRED'
  case 'DEFAULT_RULE_EVAL_FAILED':
    return 'org_field_policy_error_DEFAULT_RULE_EVAL_FAILED'
  case 'FIELD_POLICY_EXPR_INVALID':
    return 'org_field_policy_error_FIELD_POLICY_EXPR_INVALID'
  case 'ORG_CODE_EXHAUSTED':
    return 'org_field_policy_error_ORG_CODE_EXHAUSTED'
  case 'ORG_CODE_CONFLICT':
    return 'org_field_policy_error_ORG_CODE_CONFLICT'
  case 'FIELD_POLICY_SCOPE_OVERLAP':
    return 'org_field_policy_error_FIELD_POLICY_SCOPE_OVERLAP'
  default:
    return null
  }
}

function maxDay(a: string, b: string): string {
  return a > b ? a : b
}

function addUtcDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

function newRequestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `req-${Date.now()}`
}

function toRowState(config: OrgUnitTenantFieldConfig, asOf: string): RowState {
  if (asOf < config.enabled_on) {
    return 'pending'
  }
  if (config.disabled_on && config.disabled_on <= asOf) {
    return 'disabled'
  }
  return 'enabled'
}

function formatDataSourceConfigSummary(config: FieldConfigRow): string {
  const cfg = config.dataSourceConfig ?? {}
  if (config.dataSourceType === 'DICT') {
    const code = typeof cfg.dict_code === 'string' ? cfg.dict_code : ''
    return code.length > 0 ? `dict:${code}` : '-'
  }
  if (config.dataSourceType === 'ENTITY') {
    const entity = typeof cfg.entity === 'string' ? cfg.entity : ''
    const idKind = typeof cfg.id_kind === 'string' ? cfg.id_kind : ''
    const left = entity.length > 0 ? entity : '-'
    const right = idKind.length > 0 ? idKind : '-'
    return `entity:${left}/${right}`
  }
  if (config.dataSourceType === 'PLAIN') {
    return '{}'
  }
  return '-'
}

function resolveFieldLabel(t: ReturnType<typeof useAppPreferences>['t'], defByKey: Map<string, OrgUnitFieldDefinition>, config: OrgUnitTenantFieldConfig): string {
  const explicitLabelKey = config.label_i18n_key?.trim() ?? ''
  if (explicitLabelKey.length > 0 && isMessageKey(explicitLabelKey)) {
    return t(explicitLabelKey)
  }
  const explicitLabel = config.label?.trim() ?? ''
  if (explicitLabel.length > 0) {
    return explicitLabel
  }
  const def = defByKey.get(config.field_key)
  const key = def?.label_i18n_key?.trim() ?? ''
  if (key.length === 0 || !isMessageKey(key)) {
    return config.field_key
  }
  return t(key)
}

function resolveDefinitionLabel(t: ReturnType<typeof useAppPreferences>['t'], def: OrgUnitFieldDefinition): string {
  const key = def.label_i18n_key?.trim() ?? ''
  if (key.length > 0 && isMessageKey(key)) {
    return t(key)
  }
  return def.field_key
}

const fieldPolicyFormScopes = [
  'orgunit.create_dialog',
  'orgunit.details.add_version_dialog',
  'orgunit.details.insert_version_dialog',
  'orgunit.details.correct_dialog'
] as const

function normalizeFieldClass(value: string | undefined): 'CORE' | 'EXT' {
  const normalized = (value ?? '').trim().toUpperCase()
  return normalized === 'CORE' ? 'CORE' : 'EXT'
}

function normalizeFieldPolicyScopeType(value: string | undefined): string {
  const normalized = (value ?? '').trim().toUpperCase()
  if (normalized === 'FORM' || normalized === 'GLOBAL') {
    return normalized
  }
  return 'SYSTEM_DEFAULT'
}

function formatDefaultPolicySummary(row: FieldConfigRow): string {
  if (row.defaultMode === 'CEL') {
    if (row.defaultRuleExpr.length > 0) {
      return `CEL: ${row.defaultRuleExpr}`
    }
    return 'CEL'
  }
  return '-'
}

const customPlainValueTypeFallback: OrgUnitExtValueType[] = ['text', 'int', 'uuid', 'bool', 'date', 'numeric']

export function OrgUnitFieldConfigsPage() {
  const queryClient = useQueryClient()
  const { t, tenantId } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => formatAsOfDate(new Date()), [])
  const todayUtc = useMemo(() => formatAsOfDate(new Date()), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const status = parseListStatus(searchParams.get('status'))
  const disabledState = parseDisabledState(searchParams.get('disabled_state'))
  const keyword = (searchParams.get('keyword') ?? '').trim()

  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)

  const [enableOpen, setEnableOpen] = useState(false)
  const [enableForm, setEnableForm] = useState<EnableFormState>(() => ({
    mode: 'builtin',
    fieldKey: '',
    customValueType: 'text',
    customDisplayLabel: '',
    enabledOn: todayUtc,
    dataSourceConfigOption: '',
    dictDisplayLabel: ''
  }))
  const [enableError, setEnableError] = useState('')
  const [enableRequestID, setEnableRequestID] = useState(() => newRequestID())

  const [selectedConfig, setSelectedConfig] = useState<SelectedConfigState | null>(null)
  const [disableForm, setDisableForm] = useState<DisableFormState>({ disabledOn: todayUtc })
  const [disableError, setDisableError] = useState('')
  const [disableRequestID, setDisableRequestID] = useState(() => newRequestID())

  const [viewRow, setViewRow] = useState<FieldConfigRow | null>(null)
  const [policyRow, setPolicyRow] = useState<FieldConfigRow | null>(null)
  const [policyForm, setPolicyForm] = useState<PolicyFormState>({
    fieldKey: '',
    scopeType: 'FORM',
    scopeKey: fieldPolicyFormScopes[0],
    maintainable: true,
    defaultMode: 'NONE',
    defaultRuleExpr: '',
    enabledOn: todayUtc
  })
  const policyWriteDisabled = true
  const [policyError, setPolicyError] = useState('')
  const [policyRequestID, setPolicyRequestID] = useState(() => newRequestID())

  const formatApiErrorMessage = useCallback(
    (error: unknown): string => {
      if (error instanceof ApiClientError) {
        const details = error.details as { code?: string } | undefined
        const code = details?.code ?? ''
        const key = mapFieldPolicyErrorKey(code)
        if (key) {
          return t(key)
        }
      }
      return getErrorMessage(error)
    },
    [t]
  )

  const updateSearch = useCallback(
    (options: {
      asOf?: string | null
      status?: FieldConfigListStatus | null
      disabledState?: DisabledState | null
      keyword?: string | null
    }) => {
      const nextParams = new URLSearchParams(searchParams)

      if (Object.hasOwn(options, 'asOf')) {
        const value = (options.asOf ?? '').trim()
        if (value.length > 0) {
          nextParams.set('as_of', value)
        } else {
          nextParams.delete('as_of')
        }
      }

      if (Object.hasOwn(options, 'status')) {
        const value = options.status ?? 'all'
        if (value === 'all') {
          nextParams.delete('status')
        } else {
          nextParams.set('status', value)
        }
      }

      if (Object.hasOwn(options, 'disabledState')) {
        const value = options.disabledState ?? 'all'
        if (value === 'all') {
          nextParams.delete('disabled_state')
        } else {
          nextParams.set('disabled_state', value)
        }
      }

      if (Object.hasOwn(options, 'keyword')) {
        const value = (options.keyword ?? '').trim()
        if (value.length > 0) {
          nextParams.set('keyword', value)
        } else {
          nextParams.delete('keyword')
        }
      }

      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  const fieldDefinitionsQuery = useQuery({
    queryKey: ['org-field-definitions'],
    queryFn: () => listOrgUnitFieldDefinitions(),
    staleTime: 60_000
  })

  const fieldConfigsQuery = useQuery({
    queryKey: ['org-field-configs', asOf],
    queryFn: () => listOrgUnitFieldConfigs({ asOf, status: 'all' }),
    staleTime: 10_000
  })

  const fieldDefinitions = useMemo(() => fieldDefinitionsQuery.data?.fields ?? [], [fieldDefinitionsQuery.data])
  const fieldConfigs = useMemo(() => fieldConfigsQuery.data?.field_configs ?? [], [fieldConfigsQuery.data])

  const fieldDefinitionByKey = useMemo(() => {
    const map = new Map<string, OrgUnitFieldDefinition>()
    fieldDefinitions.forEach((def) => {
      map.set(def.field_key, def)
    })
    return map
  }, [fieldDefinitions])

  const rowsAll = useMemo<FieldConfigRow[]>(() => {
    return fieldConfigs.map((cfg) => {
      const label = resolveFieldLabel(t, fieldDefinitionByKey, cfg)
      return {
        id: cfg.field_key,
        fieldKey: cfg.field_key,
        fieldClass: normalizeFieldClass(cfg.field_class),
        fieldLabel: label,
        valueType: cfg.value_type,
        dataSourceType: cfg.data_source_type,
        dataSourceConfig: cfg.data_source_config ?? {},
        physicalCol: cfg.physical_col,
        enabledOn: cfg.enabled_on,
        disabledOn: cfg.disabled_on,
        updatedAt: cfg.updated_at,
        maintainable: cfg.maintainable !== false,
        defaultMode: String(cfg.default_mode ?? 'NONE').toUpperCase() === 'CEL' ? 'CEL' : 'NONE',
        defaultRuleExpr: (cfg.default_rule_expr ?? '').trim(),
        policyScopeType: normalizeFieldPolicyScopeType(cfg.policy_scope_type),
        policyScopeKey: (cfg.policy_scope_key ?? '').trim(),
        state: toRowState(cfg, asOf)
      }
    })
  }, [asOf, fieldConfigs, fieldDefinitionByKey, t])

  const rowsFiltered = useMemo(() => {
    let next = rowsAll
    if (status === 'enabled') {
      next = next.filter((row) => row.state === 'enabled')
    } else if (status === 'disabled') {
      next = next.filter((row) => row.state === 'pending' || row.state === 'disabled')
      if (disabledState === 'pending') {
        next = next.filter((row) => row.state === 'pending')
      } else if (disabledState === 'disabled') {
        next = next.filter((row) => row.state === 'disabled')
      }
    }

    const q = keyword.trim().toLowerCase()
    if (q.length > 0) {
      next = next.filter((row) => {
        if (row.fieldKey.toLowerCase().includes(q)) {
          return true
        }
        if (row.fieldLabel.toLowerCase().includes(q)) {
          return true
        }
        return false
      })
    }
    return next
  }, [disabledState, keyword, rowsAll, status])

  const existingFieldKeys = useMemo(() => new Set(rowsAll.map((row) => row.fieldKey)), [rowsAll])

  const availableDefinitions = useMemo(() => {
    // Contract (DEV-PLAN-106A): built-in DICT field_keys are not enable targets.
    return fieldDefinitions.filter((def) => !existingFieldKeys.has(def.field_key) && String(def.data_source_type ?? '').toUpperCase() !== 'DICT')
  }, [existingFieldKeys, fieldDefinitions])

  const enableMutation = useMutation({
    mutationFn: (req: {
      field_key: string
      enabled_on: string
      request_id: string
      value_type?: OrgUnitExtValueType
      data_source_config?: Record<string, unknown>
      label?: string
    }) => enableOrgUnitFieldConfig(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['org-field-configs'] })
    }
  })

  const disableMutation = useMutation({
    mutationFn: (req: { field_key: string; disabled_on: string; request_id: string }) => disableOrgUnitFieldConfig(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['org-field-configs'] })
    }
  })

  const policyMutation = useMutation({
    mutationFn: (req: {
      field_key: string
      scope_type: OrgUnitFieldPolicyScopeType
      scope_key: string
      maintainable: boolean
      default_mode: OrgUnitFieldPolicyDefaultMode
      default_rule_expr?: string
      enabled_on: string
      request_id: string
    }) => upsertOrgUnitFieldPolicy(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['org-field-configs'] })
      await queryClient.invalidateQueries({ queryKey: ['org-units', 'field-configs'] })
    }
  })

  const requestErrorMessage = fieldDefinitionsQuery.error
    ? formatApiErrorMessage(fieldDefinitionsQuery.error)
    : fieldConfigsQuery.error
    ? formatApiErrorMessage(fieldConfigsQuery.error)
    : ''

  const tableLoading = fieldDefinitionsQuery.isLoading || fieldConfigsQuery.isFetching

  const openDisableDialog = useCallback(
    (mode: 'disable' | 'postpone', row: FieldConfigRow) => {
      setDisableError('')
      setSelectedConfig({ mode, row })
      const base = maxDay(todayUtc, asOf)
      if (mode === 'disable') {
        setDisableForm({ disabledOn: maxDay(base, row.enabledOn) })
      } else {
        const old = row.disabledOn ?? base
        const next = maxDay(maxDay(addUtcDays(old, 1), base), row.enabledOn)
        setDisableForm({ disabledOn: next })
      }
      setDisableRequestID(newRequestID())
    },
    [asOf, todayUtc]
  )

  const openPolicyDialog = useCallback(
    (row: FieldConfigRow) => {
      if (policyWriteDisabled) {
        setPolicyError(t('org_field_configs_policy_write_disabled'))
        return
      }
      const scopeType = row.policyScopeType === 'GLOBAL' ? 'GLOBAL' : 'FORM'
      const scopeKey =
        scopeType === 'GLOBAL'
          ? 'global'
          : fieldPolicyFormScopes.includes(row.policyScopeKey as (typeof fieldPolicyFormScopes)[number])
          ? row.policyScopeKey
          : fieldPolicyFormScopes[0]
      setPolicyRow(row)
      setPolicyError('')
      setPolicyForm({
        fieldKey: row.fieldKey,
        scopeType,
        scopeKey,
        maintainable: row.maintainable,
        defaultMode: row.defaultMode,
        defaultRuleExpr: row.defaultRuleExpr,
        enabledOn: maxDay(todayUtc, asOf)
      })
      setPolicyRequestID(newRequestID())
    },
    [asOf, policyWriteDisabled, t, todayUtc]
  )

  function closePolicyDialog() {
    setPolicyRow(null)
    setPolicyError('')
  }

  async function submitPolicy() {
    if (!policyRow) {
      return
    }
    if (policyWriteDisabled) {
      setPolicyError(t('org_field_configs_policy_write_disabled'))
      return
    }
    setPolicyError('')
    const enabledOn = policyForm.enabledOn.trim()
    if (!/^\d{4}-\d{2}-\d{2}$/.test(enabledOn)) {
      setPolicyError(t('org_field_configs_error_invalid_date'))
      return
    }

    const scopeType: OrgUnitFieldPolicyScopeType = policyForm.scopeType === 'GLOBAL' ? 'GLOBAL' : 'FORM'
    const scopeKey = scopeType === 'GLOBAL' ? 'global' : policyForm.scopeKey.trim()
    if (scopeType === 'FORM' && !fieldPolicyFormScopes.includes(scopeKey as (typeof fieldPolicyFormScopes)[number])) {
      setPolicyError(t('org_field_configs_policy_error_scope_key_invalid'))
      return
    }

    const defaultMode: OrgUnitFieldPolicyDefaultMode = policyForm.defaultMode === 'CEL' ? 'CEL' : 'NONE'
    const defaultRuleExpr = policyForm.defaultRuleExpr.trim()
    if (defaultMode === 'CEL' && defaultRuleExpr.length === 0) {
      setPolicyError(t('org_field_configs_policy_error_expr_required'))
      return
    }
    if (!policyForm.maintainable && defaultMode !== 'CEL') {
      setPolicyError(t('org_field_policy_error_DEFAULT_RULE_REQUIRED'))
      return
    }

    try {
      const savedPolicy = await policyMutation.mutateAsync({
        field_key: policyRow.fieldKey,
        scope_type: scopeType,
        scope_key: scopeKey,
        maintainable: policyForm.maintainable,
        default_mode: defaultMode,
        default_rule_expr: defaultMode === 'CEL' ? defaultRuleExpr : undefined,
        enabled_on: enabledOn,
        request_id: policyRequestID
      })
      const nextAsOf = resolveAsOfAfterPolicySave(asOf, savedPolicy.enabled_on)
      if (nextAsOf) {
        updateSearch({ asOf: nextAsOf })
        setToast({
          message: t('org_field_configs_toast_policy_saved_as_of_switched', { date: nextAsOf }),
          severity: 'success'
        })
      } else {
        setToast({ message: t('org_field_configs_toast_policy_saved'), severity: 'success' })
      }
      closePolicyDialog()
    } catch (error) {
      setPolicyError(formatApiErrorMessage(error))
    }
  }

  const columns = useMemo<GridColDef<FieldConfigRow>[]>(() => {
    return [
      {
        field: 'fieldLabel',
        headerName: t('org_field_configs_column_label'),
        minWidth: 200,
        flex: 1.2
      },
      {
        field: 'fieldKey',
        headerName: t('org_field_configs_column_key'),
        minWidth: 160,
        flex: 1
      },
      {
        field: 'fieldClass',
        headerName: t('org_field_configs_column_field_class'),
        minWidth: 110,
        flex: 0.6
      },
      {
        field: 'valueType',
        headerName: t('org_field_configs_column_value_type'),
        minWidth: 120,
        flex: 0.7
      },
      {
        field: 'dataSourceType',
        headerName: t('org_field_configs_column_data_source_type'),
        minWidth: 120,
        flex: 0.8
      },
      {
        field: 'dataSourceConfig',
        headerName: t('org_field_configs_column_data_source_config'),
        minWidth: 200,
        flex: 1.1,
        sortable: false,
        renderCell: (params) => {
          return (
            <Typography component='span' sx={{ fontFamily: 'monospace', fontSize: 12 }}>
              {formatDataSourceConfigSummary(params.row)}
            </Typography>
          )
        }
      },
      {
        field: 'maintainable',
        headerName: t('org_field_configs_column_maintainable'),
        minWidth: 120,
        flex: 0.7,
        renderCell: (params) => (params.row.maintainable ? t('common_yes') : t('common_no'))
      },
      {
        field: 'defaultMode',
        headerName: t('org_field_configs_column_default_value'),
        minWidth: 260,
        flex: 1.2,
        sortable: false,
        renderCell: (params) => {
          return (
            <Typography component='span' sx={{ fontFamily: 'monospace', fontSize: 12 }}>
              {formatDefaultPolicySummary(params.row)}
            </Typography>
          )
        }
      },
      {
        field: 'policyScope',
        headerName: t('org_field_configs_column_policy_scope'),
        minWidth: 210,
        flex: 1,
        sortable: false,
        renderCell: (params) => `${params.row.policyScopeType}:${params.row.policyScopeKey || '-'}`
      },
      {
        field: 'state',
        headerName: t('text_status'),
        minWidth: 140,
        flex: 0.8,
        sortable: false,
        renderCell: (params) => {
          const state = params.row.state
          if (state === 'enabled') {
            return <StatusChip color='success' label={t('org_field_configs_state_enabled')} />
          }
          if (state === 'pending') {
            return <StatusChip color='info' label={t('org_field_configs_state_pending')} />
          }
          return <StatusChip color='warning' label={t('org_field_configs_state_disabled')} />
        }
      },
      {
        field: 'enabledOn',
        headerName: t('org_field_configs_column_enabled_on'),
        minWidth: 120,
        flex: 0.8
      },
      {
        field: 'disabledOn',
        headerName: t('org_field_configs_column_disabled_on'),
        minWidth: 120,
        flex: 0.8,
        renderCell: (params) => params.row.disabledOn ?? '-'
      },
      {
        field: 'physicalCol',
        headerName: t('org_field_configs_column_physical_col'),
        minWidth: 140,
        flex: 0.9,
        renderCell: (params) => params.row.physicalCol || '-'
      },
      {
        field: 'updatedAt',
        headerName: t('org_field_configs_column_updated_at'),
        minWidth: 180,
        flex: 1,
        renderCell: (params) => params.row.updatedAt ?? '-'
      },
      {
        field: 'actions',
        headerName: t('org_field_configs_column_actions'),
        minWidth: 220,
        flex: 1,
        sortable: false,
        filterable: false,
        disableColumnMenu: true,
        renderCell: (params) => {
          const row = params.row
          const canDisable = row.fieldClass === 'EXT' && row.disabledOn == null
          const canPostpone = row.fieldClass === 'EXT' && row.disabledOn != null && todayUtc < row.disabledOn
          return (
            <Stack direction='row' spacing={1}>
              <Button onClick={() => setViewRow(row)} size='small' variant='text'>
                {t('common_detail')}
              </Button>
              <Button disabled={policyWriteDisabled} onClick={() => openPolicyDialog(row)} size='small' variant='text'>
                {t('org_field_configs_action_edit_policy')}
              </Button>
              <Button
                disabled={!canDisable}
                onClick={() => openDisableDialog('disable', row)}
                size='small'
                variant='text'
              >
                {t('org_field_configs_action_disable')}
              </Button>
              <Button
                disabled={!canPostpone}
                onClick={() => openDisableDialog('postpone', row)}
                size='small'
                variant='text'
              >
                {t('org_field_configs_action_postpone')}
              </Button>
            </Stack>
          )
        }
      }
    ]
  }, [openDisableDialog, openPolicyDialog, t, todayUtc])

  function openEnableDialog() {
    setEnableError('')
    setEnableOpen(true)
    setEnableForm({
      mode: 'builtin',
      fieldKey: '',
      customValueType: 'text',
      customDisplayLabel: '',
      enabledOn: maxDay(todayUtc, asOf),
      dataSourceConfigOption: '',
      dictDisplayLabel: ''
    })
    setEnableRequestID(newRequestID())
  }

  function closeEnableDialog() {
    setEnableOpen(false)
    setEnableError('')
  }

  const selectedDefinition = useMemo(() => {
    const key = enableForm.fieldKey.trim()
    if (key.length === 0) {
      return null
    }
    return fieldDefinitionByKey.get(key) ?? null
  }, [enableForm.fieldKey, fieldDefinitionByKey])

  const enableCandidatesQuery = useQuery({
    enabled: enableOpen && (enableForm.mode === 'dict' || enableForm.mode === 'custom'),
    queryKey: ['org-field-configs', 'enable-candidates', enableForm.enabledOn],
    queryFn: () => listOrgUnitFieldConfigEnableCandidates({ enabledOn: enableForm.enabledOn }),
    staleTime: 10_000
  })
  const dictFieldCandidates = useMemo<OrgUnitFieldEnableCandidateField[]>(() => enableCandidatesQuery.data?.dict_fields ?? [], [enableCandidatesQuery.data])
  const dictFieldCandidatesFiltered = useMemo(() => {
    return dictFieldCandidates.filter((f) => !existingFieldKeys.has(f.field_key))
  }, [dictFieldCandidates, existingFieldKeys])
  const customValueTypeOptions = useMemo<OrgUnitExtValueType[]>(() => {
    const hinted = enableCandidatesQuery.data?.plain_custom_hint?.value_types ?? []
    const normalized = hinted.map((item) => String(item).trim().toLowerCase()).filter((item): item is OrgUnitExtValueType => customPlainValueTypeFallback.includes(item as OrgUnitExtValueType))
    if (normalized.length === 0) {
      return customPlainValueTypeFallback
    }
    return Array.from(new Set(normalized))
  }, [enableCandidatesQuery.data])
  const customValueTypeDefault = useMemo<OrgUnitExtValueType>(() => {
    const hintedDefault = String(enableCandidatesQuery.data?.plain_custom_hint?.default_value_type ?? '').trim().toLowerCase()
    if (customValueTypeOptions.includes(hintedDefault as OrgUnitExtValueType)) {
      return hintedDefault as OrgUnitExtValueType
    }
    return customValueTypeOptions[0] ?? 'text'
  }, [customValueTypeOptions, enableCandidatesQuery.data])

  async function submitEnable() {
    setEnableError('')
    const fieldKey = enableForm.fieldKey.trim()
    const enabledOn = enableForm.enabledOn.trim()
    if (fieldKey.length === 0 || enabledOn.length === 0) {
      setEnableError(t('org_field_configs_error_required'))
      return
    }
    if (!/^\d{4}-\d{2}-\d{2}$/.test(enabledOn)) {
      setEnableError(t('org_field_configs_error_invalid_date'))
      return
    }
    if (existingFieldKeys.has(fieldKey)) {
      setEnableError(t('org_field_configs_error_already_exists'))
      return
    }

    if (enableForm.mode === 'custom') {
      if (!/^x_[a-z0-9_]{1,60}$/.test(fieldKey)) {
        setEnableError(t('org_field_configs_error_custom_field_key_invalid'))
        return
      }
      if (!customValueTypeOptions.includes(enableForm.customValueType)) {
        setEnableError(t('org_field_configs_error_required'))
        return
      }
    }

    if (enableForm.mode === 'dict') {
      // Contract (DEV-PLAN-106A): dict fields use d_<dict_code>.
      if (!/^d_[a-z][a-z0-9_]{0,61}$/.test(fieldKey)) {
        setEnableError(t('org_field_configs_error_definition_missing'))
        return
      }
    }

    const def = enableForm.mode === 'builtin' ? fieldDefinitionByKey.get(fieldKey) ?? null : null
    if (enableForm.mode === 'builtin' && !def) {
      setEnableError(t('org_field_configs_error_definition_missing'))
      return
    }

    const dataSourceType = String(def?.data_source_type ?? 'PLAIN').toUpperCase()
    let dataSourceConfig: Record<string, unknown> | undefined
    if (enableForm.mode === 'builtin' && dataSourceType === 'ENTITY') {
      const raw = enableForm.dataSourceConfigOption.trim()
      if (raw.length === 0) {
        setEnableError(t('org_field_configs_error_data_source_config_required'))
        return
      }
      try {
        dataSourceConfig = JSON.parse(raw) as Record<string, unknown>
      } catch {
        setEnableError(t('org_field_configs_error_data_source_config_required'))
        return
      }
    }

    try {
      const label =
        enableForm.mode === 'dict'
          ? enableForm.dictDisplayLabel.trim()
          : enableForm.mode === 'custom'
          ? enableForm.customDisplayLabel.trim()
          : ''
      await enableMutation.mutateAsync({
        field_key: fieldKey,
        enabled_on: enabledOn,
        request_id: enableRequestID,
        value_type: enableForm.mode === 'custom' ? enableForm.customValueType : undefined,
        data_source_config: dataSourceConfig,
        label: label.length > 0 ? label : undefined
      })
      setToast({ message: t('org_field_configs_toast_enable_success'), severity: 'success' })
      closeEnableDialog()
    } catch (error) {
      setEnableError(formatApiErrorMessage(error))
    }
  }

  function closeDisableDialog() {
    setSelectedConfig(null)
    setDisableError('')
  }

  async function submitDisable() {
    if (!selectedConfig) {
      return
    }
    setDisableError('')
    const disabledOn = disableForm.disabledOn.trim()
    if (!/^\d{4}-\d{2}-\d{2}$/.test(disabledOn)) {
      setDisableError(t('org_field_configs_error_invalid_date'))
      return
    }

    const row = selectedConfig.row
    if (disabledOn < todayUtc) {
      setDisableError(t('org_field_configs_error_disabled_on_backdated'))
      return
    }
    if (disabledOn < row.enabledOn) {
      setDisableError(t('org_field_configs_error_disabled_on_before_enabled_on'))
      return
    }

    if (selectedConfig.mode === 'postpone') {
      const old = row.disabledOn
      if (!old) {
        setDisableError(t('org_field_configs_error_postpone_requires_old'))
        return
      }
      if (!(todayUtc < old)) {
        setDisableError(t('org_field_configs_error_postpone_already_effective'))
        return
      }
      if (disabledOn <= old) {
        setDisableError(t('org_field_configs_error_postpone_not_later'))
        return
      }
    }

    try {
      await disableMutation.mutateAsync({
        field_key: row.fieldKey,
        disabled_on: disabledOn,
        request_id: disableRequestID
      })
      setToast({ message: t('org_field_configs_toast_disable_success'), severity: 'success' })
      closeDisableDialog()
    } catch (error) {
      setDisableError(formatApiErrorMessage(error))
    }
  }

  const listLinkSearch = useMemo(() => {
    const params = new URLSearchParams()
    params.set('as_of', asOf)
    return params.toString().length > 0 ? `?${params.toString()}` : ''
  }, [asOf])

  const pageSubtitle = t('org_field_configs_subtitle')

  return (
    <>
      <Breadcrumbs sx={{ mb: 1 }}>
        <Link component={RouterLink} to={{ pathname: '/org/units', search: listLinkSearch }} underline='hover' color='inherit'>
          {t('nav_org_units')}
        </Link>
        <Typography color='text.primary'>{t('org_field_configs_title')}</Typography>
      </Breadcrumbs>

      <PageHeader
        title={t('org_field_configs_title')}
        subtitle={pageSubtitle}
        actions={
          <Button onClick={openEnableDialog} size='small' variant='contained'>
            {t('org_field_configs_action_enable')}
          </Button>
        }
      />

      <FieldConfigsFilterBar
        asOf={asOf}
        disabledState={disabledState}
        keyword={keyword}
        key={`${asOf}|${status}|${disabledState}|${keyword}`}
        onApply={(next) => updateSearch(next)}
        status={status}
        t={t}
      />

      {requestErrorMessage.length > 0 ? (
        <Alert severity='error' sx={{ mb: 2 }}>
          {requestErrorMessage}
        </Alert>
      ) : null}

      {!tableLoading && requestErrorMessage.length === 0 && rowsFiltered.length === 0 ? (
        <Paper sx={{ mb: 2, p: 2 }} variant='outlined'>
          <Stack direction={{ md: 'row', xs: 'column' }} spacing={2} alignItems={{ md: 'center', xs: 'flex-start' }}>
            <Typography color='text.secondary' variant='body2'>
              {t('org_field_configs_empty')}
            </Typography>
            <Button onClick={openEnableDialog} size='small' variant='contained'>
              {t('org_field_configs_action_enable')}
            </Button>
          </Stack>
        </Paper>
      ) : null}

      <DataGridPage
        columns={columns}
        gridProps={{
          onRowDoubleClick: (params) => {
            setViewRow(params.row as FieldConfigRow)
          },
          pageSizeOptions: [10, 20, 50],
          pagination: true,
          initialState: { pagination: { paginationModel: { page: 0, pageSize: 20 } } },
          sx: { minHeight: 560 }
        }}
        loading={tableLoading}
        loadingLabel={t('text_loading')}
        noRowsLabel={t('text_no_data')}
        rows={rowsFiltered}
        storageKey={`org-field-configs-grid/${tenantId}`}
      />

      <Dialog onClose={closeEnableDialog} open={enableOpen} fullWidth maxWidth='sm'>
        <DialogTitle>{t('org_field_configs_action_enable')}</DialogTitle>
        <DialogContent>
          {enableError.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {enableError}
            </Alert>
          ) : null}
          <Stack spacing={2} sx={{ mt: 0.5 }}>
            <FormControl>
              <InputLabel id='org-field-key-mode-select-label'>{t('org_field_configs_form_field_key_mode')}</InputLabel>
              <Select
                label={t('org_field_configs_form_field_key_mode')}
                labelId='org-field-key-mode-select-label'
                onChange={(event) => {
                  const nextMode = String(event.target.value) as EnableFormState['mode']
                  setEnableRequestID(newRequestID())
                  setEnableForm((previous) => ({
                    ...previous,
                    mode: nextMode,
                    fieldKey: '',
                    customValueType: customValueTypeDefault,
                    customDisplayLabel: '',
                    dataSourceConfigOption: '',
                    dictDisplayLabel: ''
                  }))
                }}
                value={enableForm.mode}
              >
                <MenuItem value='builtin'>{t('org_field_configs_form_field_key_mode_builtin')}</MenuItem>
                <MenuItem value='dict'>{t('org_field_configs_form_field_key_mode_dict')}</MenuItem>
                <MenuItem value='custom'>{t('org_field_configs_form_field_key_mode_custom')}</MenuItem>
              </Select>
            </FormControl>
            {enableForm.mode === 'builtin' ? (
              <FormControl>
                <InputLabel id='org-field-key-select-label'>{t('org_field_configs_form_field_key')}</InputLabel>
                <Select
                  label={t('org_field_configs_form_field_key')}
                  labelId='org-field-key-select-label'
                  onChange={(event) => {
                    const nextFieldKey = String(event.target.value)
                    setEnableRequestID(newRequestID())
                    const def = fieldDefinitionByKey.get(nextFieldKey)
                    const dataSourceType = String(def?.data_source_type ?? '').toUpperCase()
                    let nextOption = ''
                    const options = def?.data_source_config_options ?? []
                    if (dataSourceType === 'ENTITY' && options.length === 1) {
                      nextOption = JSON.stringify(options[0] ?? {})
                    }
                    setEnableForm((previous) => ({
                      ...previous,
                      fieldKey: nextFieldKey,
                      customDisplayLabel: '',
                      dataSourceConfigOption: nextOption,
                      dictDisplayLabel: ''
                    }))
                  }}
                  value={enableForm.fieldKey}
                >
                  {availableDefinitions.map((def) => (
                    <MenuItem key={def.field_key} value={def.field_key}>
                      {resolveDefinitionLabel(t, def)} ({def.field_key})
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            ) : enableForm.mode === 'dict' ? (
              <FormControl>
                <InputLabel id='org-dict-field-key-select-label'>{t('org_field_configs_form_field_key')}</InputLabel>
                <Select
                  label={t('org_field_configs_form_field_key')}
                  labelId='org-dict-field-key-select-label'
                  onChange={(event) => {
                    const nextFieldKey = String(event.target.value)
                    setEnableRequestID(newRequestID())
                    setEnableForm((previous) => ({
                      ...previous,
                      fieldKey: nextFieldKey,
                      customDisplayLabel: '',
                      dataSourceConfigOption: '',
                      dictDisplayLabel: ''
                    }))
                  }}
                  value={enableForm.fieldKey}
                >
                  {dictFieldCandidatesFiltered.map((item) => (
                    <MenuItem key={item.field_key} value={item.field_key}>
                      {item.name} ({item.dict_code})
                    </MenuItem>
                  ))}
                </Select>
                {enableCandidatesQuery.isError ? (
                  <Typography color='error' variant='caption' sx={{ mt: 0.5 }}>
                    {getErrorMessage(enableCandidatesQuery.error)}
                  </Typography>
                ) : null}
              </FormControl>
            ) : (
              <TextField
                label={t('org_field_configs_form_field_key')}
                onChange={(event) => {
                  setEnableRequestID(newRequestID())
                  setEnableForm((previous) => ({
                    ...previous,
                    fieldKey: event.target.value,
                    dataSourceConfigOption: '',
                    dictDisplayLabel: ''
                  }))
                }}
                value={enableForm.fieldKey}
                helperText={t('org_field_configs_form_custom_field_key_helper')}
              />
            )}
            {enableForm.mode === 'custom' ? (
              <FormControl>
                <InputLabel id='org-custom-value-type-select-label'>{t('org_field_configs_form_custom_value_type')}</InputLabel>
                <Select
                  label={t('org_field_configs_form_custom_value_type')}
                  labelId='org-custom-value-type-select-label'
                  onChange={(event) => {
                    setEnableRequestID(newRequestID())
                    setEnableForm((previous) => ({ ...previous, customValueType: String(event.target.value) as OrgUnitExtValueType }))
                  }}
                  value={enableForm.customValueType}
                >
                  {customValueTypeOptions.map((valueType) => (
                    <MenuItem key={valueType} value={valueType}>
                      {valueType}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            ) : null}
            {enableForm.mode === 'custom' ? (
              <TextField
                label={t('org_field_configs_form_custom_field_label')}
                onChange={(event) => {
                  setEnableRequestID(newRequestID())
                  setEnableForm((previous) => ({ ...previous, customDisplayLabel: event.target.value }))
                }}
                value={enableForm.customDisplayLabel}
                helperText={t('org_field_configs_form_custom_field_label_helper')}
              />
            ) : null}
            <TextField
              InputLabelProps={{ shrink: true }}
              label={t('org_field_configs_form_enabled_on')}
              onChange={(event) => {
                setEnableRequestID(newRequestID())
                setEnableForm((previous) => ({ ...previous, enabledOn: event.target.value }))
              }}
              type='date'
              value={enableForm.enabledOn}
            />
            {enableForm.mode === 'dict' ? (
              <TextField
                label={t('org_field_configs_form_dict_field_label')}
                onChange={(event) => {
                  setEnableRequestID(newRequestID())
                  setEnableForm((previous) => ({ ...previous, dictDisplayLabel: event.target.value }))
                }}
                value={enableForm.dictDisplayLabel}
                helperText={t('org_field_configs_form_dict_field_label_helper')}
              />
            ) : null}
            {enableForm.mode === 'builtin' && selectedDefinition && String(selectedDefinition.data_source_type ?? '').toUpperCase() === 'ENTITY' ? (
              <FormControl>
                <InputLabel id='org-field-configs-data-source-config-label'>{t('org_field_configs_form_data_source_config')}</InputLabel>
                <Select
                  label={t('org_field_configs_form_data_source_config')}
                  labelId='org-field-configs-data-source-config-label'
                  onChange={(event) => {
                    setEnableRequestID(newRequestID())
                    setEnableForm((previous) => ({ ...previous, dataSourceConfigOption: String(event.target.value) }))
                  }}
                  value={enableForm.dataSourceConfigOption}
                >
                  {(selectedDefinition.data_source_config_options ?? []).map((opt, idx) => {
                    const raw = JSON.stringify(opt ?? {})
                    return (
                      <MenuItem key={`${raw}-${idx}`} value={raw}>
                        <Typography component='span' sx={{ fontFamily: 'monospace', fontSize: 13 }}>
                          {raw}
                        </Typography>
                      </MenuItem>
                    )
                  })}
                </Select>
              </FormControl>
            ) : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeEnableDialog}>{t('common_cancel')}</Button>
          <Button disabled={enableMutation.isPending} onClick={() => void submitEnable()} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog onClose={closeDisableDialog} open={selectedConfig != null} fullWidth maxWidth='sm'>
        <DialogTitle>
          {selectedConfig?.mode === 'postpone' ? t('org_field_configs_action_postpone') : t('org_field_configs_action_disable')}
        </DialogTitle>
        <DialogContent>
          {disableError.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {disableError}
            </Alert>
          ) : null}
          {selectedConfig ? (
            <Stack spacing={2} sx={{ mt: 0.5 }}>
              <Alert severity='warning'>
                {t('org_field_configs_disable_warning')}
              </Alert>
              <Typography variant='body2'>
                <strong>{selectedConfig.row.fieldLabel}</strong> ({selectedConfig.row.fieldKey}) · {selectedConfig.row.physicalCol}
              </Typography>
              {selectedConfig.mode === 'postpone' && selectedConfig.row.disabledOn ? (
                <Typography color='text.secondary' variant='body2'>
                  {t('org_field_configs_current_disabled_on', { date: selectedConfig.row.disabledOn })}
                </Typography>
              ) : null}
              <TextField
                InputLabelProps={{ shrink: true }}
                label={t('org_field_configs_form_disabled_on')}
                onChange={(event) => {
                  setDisableRequestID(newRequestID())
                  setDisableForm({ disabledOn: event.target.value })
                }}
                type='date'
                value={disableForm.disabledOn}
              />
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDisableDialog}>{t('common_cancel')}</Button>
          <Button disabled={disableMutation.isPending} onClick={() => void submitDisable()} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog onClose={closePolicyDialog} open={policyRow != null} fullWidth maxWidth='sm'>
        <DialogTitle>{t('org_field_configs_action_edit_policy')}</DialogTitle>
        <DialogContent>
          {policyError.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {policyError}
            </Alert>
          ) : null}
          {policyRow ? (
            <Stack spacing={2} sx={{ mt: 0.5 }}>
              <Typography variant='body2'>
                <strong>{policyRow.fieldLabel}</strong> ({policyRow.fieldKey})
              </Typography>
              <FormControl>
                <InputLabel id='org-field-policy-scope-type-label'>{t('org_field_configs_policy_scope_type')}</InputLabel>
                <Select
                  label={t('org_field_configs_policy_scope_type')}
                  labelId='org-field-policy-scope-type-label'
                  onChange={(event) => {
                    const nextScopeType: OrgUnitFieldPolicyScopeType = event.target.value === 'GLOBAL' ? 'GLOBAL' : 'FORM'
                    setPolicyRequestID(newRequestID())
                    setPolicyForm((previous) => ({
                      ...previous,
                      scopeType: nextScopeType,
                      scopeKey: nextScopeType === 'GLOBAL' ? 'global' : fieldPolicyFormScopes[0]
                    }))
                  }}
                  value={policyForm.scopeType}
                >
                  <MenuItem value='FORM'>FORM</MenuItem>
                  <MenuItem value='GLOBAL'>GLOBAL</MenuItem>
                </Select>
              </FormControl>
              {policyForm.scopeType === 'FORM' ? (
                <FormControl>
                  <InputLabel id='org-field-policy-scope-key-label'>{t('org_field_configs_policy_scope_key')}</InputLabel>
                  <Select
                    label={t('org_field_configs_policy_scope_key')}
                    labelId='org-field-policy-scope-key-label'
                    onChange={(event) => {
                      setPolicyRequestID(newRequestID())
                      setPolicyForm((previous) => ({ ...previous, scopeKey: String(event.target.value) }))
                    }}
                    value={policyForm.scopeKey}
                  >
                    {fieldPolicyFormScopes.map((scopeKey) => (
                      <MenuItem key={scopeKey} value={scopeKey}>
                        {scopeKey}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              ) : null}
              <FormControlLabel
                control={
                  <Switch
                    checked={policyForm.maintainable}
                    onChange={(event) => {
                      setPolicyRequestID(newRequestID())
                      setPolicyForm((previous) => ({ ...previous, maintainable: event.target.checked }))
                    }}
                  />
                }
                label={t('org_field_configs_column_maintainable')}
              />
              <FormControl>
                <InputLabel id='org-field-policy-default-mode-label'>{t('org_field_configs_policy_default_mode')}</InputLabel>
                <Select
                  label={t('org_field_configs_policy_default_mode')}
                  labelId='org-field-policy-default-mode-label'
                  onChange={(event) => {
                    const nextMode: OrgUnitFieldPolicyDefaultMode = event.target.value === 'CEL' ? 'CEL' : 'NONE'
                    setPolicyRequestID(newRequestID())
                    setPolicyForm((previous) => ({
                      ...previous,
                      defaultMode: nextMode,
                      defaultRuleExpr: nextMode === 'CEL' ? previous.defaultRuleExpr : ''
                    }))
                  }}
                  value={policyForm.defaultMode}
                >
                  <MenuItem value='NONE'>NONE</MenuItem>
                  <MenuItem value='CEL'>CEL</MenuItem>
                </Select>
              </FormControl>
              {policyForm.defaultMode === 'CEL' ? (
                <TextField
                  label={t('org_field_configs_policy_default_rule_expr')}
                  helperText={t('org_field_configs_policy_default_rule_expr_helper')}
                  onChange={(event) => {
                    setPolicyRequestID(newRequestID())
                    setPolicyForm((previous) => ({ ...previous, defaultRuleExpr: event.target.value }))
                  }}
                  placeholder='next_org_code("ORG", 6)'
                  value={policyForm.defaultRuleExpr}
                />
              ) : null}
              <TextField
                InputLabelProps={{ shrink: true }}
                label={t('org_field_configs_form_enabled_on')}
                onChange={(event) => {
                  setPolicyRequestID(newRequestID())
                  setPolicyForm((previous) => ({ ...previous, enabledOn: event.target.value }))
                }}
                type='date'
                value={policyForm.enabledOn}
              />
              {shouldShowFutureEffectiveHint(asOf, policyForm.enabledOn) ? (
                <Alert severity='info'>
                  {t('org_field_configs_policy_future_effective_hint', { asOf, enabledOn: policyForm.enabledOn })}
                </Alert>
              ) : null}
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={closePolicyDialog}>{t('common_cancel')}</Button>
          <Button
            disabled={policyWriteDisabled || policyMutation.isPending}
            onClick={() => void submitPolicy()}
            variant='contained'
          >
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog onClose={() => setViewRow(null)} open={viewRow != null} fullWidth maxWidth='md'>
        <DialogTitle>{t('common_detail')}</DialogTitle>
        <DialogContent>
          {viewRow ? (
            <Stack spacing={1.5} sx={{ mt: 0.5 }}>
              <Typography variant='body2'>
                <strong>{viewRow.fieldLabel}</strong> ({viewRow.fieldKey})
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_field_class')}: <code>{viewRow.fieldClass}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_value_type')}: <code>{viewRow.valueType}</code> · {t('org_field_configs_column_data_source_type')}: <code>{viewRow.dataSourceType}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_physical_col')}: <code>{viewRow.physicalCol || '-'}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_enabled_on')}: <code>{viewRow.enabledOn}</code> · {t('org_field_configs_column_disabled_on')}: <code>{viewRow.disabledOn ?? '-'}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_maintainable')}: <code>{viewRow.maintainable ? 'true' : 'false'}</code> · {t('org_field_configs_column_default_value')}:{' '}
                <code>{formatDefaultPolicySummary(viewRow)}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_policy_scope')}: <code>{viewRow.policyScopeType}:{viewRow.policyScopeKey || '-'}</code>
              </Typography>
              <Typography variant='body2'>
                {t('org_field_configs_column_data_source_config')}:
              </Typography>
              <Paper sx={{ p: 1.5 }} variant='outlined'>
                <Typography component='pre' sx={{ fontFamily: 'monospace', fontSize: 12, m: 0, whiteSpace: 'pre-wrap' }}>
                  {JSON.stringify(viewRow.dataSourceConfig ?? {}, null, 2)}
                </Typography>
              </Paper>
              <Typography variant='body2'>
                {t('org_field_configs_column_updated_at')}: <code>{viewRow.updatedAt}</code>
              </Typography>
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setViewRow(null)}>{t('common_confirm')}</Button>
        </DialogActions>
      </Dialog>

      <Snackbar autoHideDuration={2800} onClose={() => setToast(null)} open={toast !== null}>
        <Alert severity={toast?.severity ?? 'success'} variant='filled'>
          {toast?.message ?? ''}
        </Alert>
      </Snackbar>
    </>
  )
}
