import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link as RouterLink, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Autocomplete,
  Box,
  Breadcrumbs,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  Link,
  List,
  ListItemButton,
  ListItemText,
  MenuItem,
  Paper,
  Snackbar,
  Stack,
  Switch,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { format as formatDate } from 'date-fns'
import {
  correctOrgUnit,
  disableOrgUnit,
  enableOrgUnit,
  getOrgUnitAppendCapabilities,
  getOrgUnitFieldOptions,
  getOrgUnitDetails,
  getOrgUnitMutationCapabilities,
  listOrgUnitAudit,
  listOrgUnitVersions,
  moveOrgUnit,
  renameOrgUnit,
  rescindOrgUnit,
  rescindOrgUnitRecord,
  setOrgUnitBusinessUnit
} from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { PageHeader } from '../../components/PageHeader'
import { isMessageKey, type MessageKey } from '../../i18n/messages'
import { normalizePlainExtDraft } from './orgUnitPlainExtValidation'
import { buildAppendPayload } from './orgUnitAppendIntent'
import { resolveOrgUnitEffectiveDate } from './orgUnitVersionSelection'
import { buildCorrectPatch } from './orgUnitCorrectionIntent'

type DetailTab = 'profile' | 'audit'
type OrgStatus = 'active' | 'inactive'
type OrgActionType =
  | 'rename'
  | 'move'
  | 'set_business_unit'
  | 'enable'
  | 'disable'
  | 'correct'
  | 'rescind_record'
  | 'rescind_org'

type OrgUnitAppendEventType = 'RENAME' | 'MOVE' | 'SET_BUSINESS_UNIT' | 'ENABLE' | 'DISABLE'

const detailTabs: readonly DetailTab[] = ['profile', 'audit']

interface OrgActionState {
  type: OrgActionType
  targetCode: string | null
}

interface OrgActionForm {
  orgCode: string
  name: string
  parentOrgCode: string
  managerPernr: string
  effectiveDate: string
  correctedEffectiveDate: string
  isBusinessUnit: boolean
  extValues: Record<string, unknown>
  originalExtValues: Record<string, unknown>
  extDisplayValues: Record<string, string>
  requestId: string
  requestCode: string
  reason: string
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

function parseOptionalValue(raw: string | null): string | null {
  if (!raw) {
    return null
  }
  const value = raw.trim()
  if (value.length === 0) {
    return null
  }
  return value
}

function parseBool(raw: string | null): boolean {
  if (!raw) {
    return false
  }
  const value = raw.trim().toLowerCase()
  return value === '1' || value === 'true' || value === 'yes' || value === 'on'
}

function parseDetailTab(raw: string | null): DetailTab {
  if (raw && detailTabs.includes(raw as DetailTab)) {
    return raw as DetailTab
  }
  return 'profile'
}

function parseOrgStatus(raw: string): OrgStatus {
  const value = raw.trim().toLowerCase()
  return value === 'disabled' || value === 'inactive' ? 'inactive' : 'active'
}

function trimToUndefined(value: string): string | undefined {
  const normalized = value.trim()
  return normalized.length > 0 ? normalized : undefined
}

function appendEventTypeForAction(type: OrgActionType): OrgUnitAppendEventType | null {
  switch (type) {
    case 'rename':
      return 'RENAME'
    case 'move':
      return 'MOVE'
    case 'set_business_unit':
      return 'SET_BUSINESS_UNIT'
    case 'enable':
      return 'ENABLE'
    case 'disable':
      return 'DISABLE'
    default:
      return null
  }
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function newRequestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `org-${Date.now()}`
}

function emptyActionForm(effectiveDate: string): OrgActionForm {
  return {
    orgCode: '',
    name: '',
    parentOrgCode: '',
    managerPernr: '',
    effectiveDate,
    correctedEffectiveDate: '',
    isBusinessUnit: false,
    extValues: {},
    originalExtValues: {},
    extDisplayValues: {},
    requestId: newRequestID(),
    requestCode: `req-${Date.now()}`,
    reason: ''
  }
}

function actionLabel(type: OrgActionType, t: (key: MessageKey) => string): string {
  switch (type) {
    case 'rename':
      return t('org_action_rename')
    case 'move':
      return t('org_action_move')
    case 'set_business_unit':
      return t('org_action_set_business_unit')
    case 'enable':
      return t('org_action_enable')
    case 'disable':
      return t('org_action_disable')
    case 'correct':
      return t('org_action_correct')
    case 'rescind_record':
      return t('org_action_rescind_record')
    case 'rescind_org':
      return t('org_action_rescind_org')
    default:
      return ''
  }
}

interface DiffRow {
  field: string
  before: string
  after: string
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function toDisplayText(value: unknown): string {
  if (value === null || value === undefined) {
    return '-'
  }
  if (typeof value === 'string') {
    return value.trim().length > 0 ? value : '-'
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function formatTxTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return formatDate(date, 'yyyy-MM-dd HH:mm')
}

function normalizeExtValueByType(
  valueType: 'text' | 'int' | 'uuid' | 'bool' | 'date',
  rawValue: unknown
): unknown {
  if (rawValue === null || rawValue === undefined) {
    return null
  }

  switch (valueType) {
    case 'int':
      if (typeof rawValue === 'number' && Number.isFinite(rawValue)) {
        return rawValue
      }
      if (typeof rawValue === 'string') {
        const trimmed = rawValue.trim()
        if (trimmed.length === 0) {
          return null
        }
        const parsed = Number.parseInt(trimmed, 10)
        return Number.isFinite(parsed) ? parsed : rawValue
      }
      return rawValue
    case 'bool':
      if (typeof rawValue === 'boolean') {
        return rawValue
      }
      if (typeof rawValue === 'string') {
        const lowered = rawValue.trim().toLowerCase()
        if (lowered === 'true') {
          return true
        }
        if (lowered === 'false') {
          return false
        }
      }
      return rawValue
    case 'date':
    case 'text':
    case 'uuid':
      if (typeof rawValue === 'string') {
        return rawValue
      }
      return String(rawValue)
    default:
      return rawValue
  }
}

function formatAuditActor(name: string, employeeId: string): string {
  const trimmedName = name.trim()
  const trimmedEmployeeId = employeeId.trim()
  if (trimmedName.length > 0 && trimmedEmployeeId.length > 0) {
    return `${trimmedName}(${trimmedEmployeeId})`
  }
  if (trimmedName.length > 0) {
    return trimmedName
  }
  if (trimmedEmployeeId.length > 0) {
    return trimmedEmployeeId
  }
  return '-'
}

function buildSnapshotDiff(beforeSnapshot: unknown, afterSnapshot: unknown): DiffRow[] {
  if (!isObjectRecord(beforeSnapshot) || !isObjectRecord(afterSnapshot)) {
    return []
  }

  const keys = Array.from(new Set([...Object.keys(beforeSnapshot), ...Object.keys(afterSnapshot)])).sort((a, b) =>
    a.localeCompare(b)
  )

  return keys
    .map((key) => {
      const beforeValue = beforeSnapshot[key]
      const afterValue = afterSnapshot[key]
      const beforeText = toDisplayText(beforeValue)
      const afterText = toDisplayText(afterValue)
      if (beforeText === afterText) {
        return null
      }
      return {
        field: key,
        before: beforeText,
        after: afterText
      }
    })
    .filter((row): row is DiffRow => row !== null)
}

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const handle = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(handle)
  }, [delayMs, value])

  return debounced
}

type FieldOption = { value: string; label: string }

function uniqueOptionsByValue(options: FieldOption[]): FieldOption[] {
  const seen = new Set<string>()
  const out: FieldOption[] = []
  for (const option of options) {
    const key = option.value
    if (!key || seen.has(key)) {
      continue
    }
    seen.add(key)
    out.push(option)
  }
  return out
}

function OrgUnitExtFieldSelect(props: {
  fieldKey: string
  asOf: string
  label: string
  disabled: boolean
  value: string | null
  valueLabel: string | null
  helperText?: string
  onChange: (nextValue: string | null, nextLabel: string | null) => void
}) {
  const [inputValue, setInputValue] = useState('')
  const debouncedKeyword = useDebouncedValue(inputValue, 250)

  const optionsQuery = useQuery({
    enabled: !props.disabled,
    queryKey: ['org-units', 'field-options', props.fieldKey, props.asOf, debouncedKeyword],
    queryFn: () => getOrgUnitFieldOptions({ fieldKey: props.fieldKey, asOf: props.asOf, keyword: debouncedKeyword, limit: 20 }),
    staleTime: 30_000
  })

  const options = useMemo<FieldOption[]>(() => {
    const fetched = optionsQuery.data?.options ?? []
    const selectedValue = props.value?.trim() ?? ''
    const selectedLabel = props.valueLabel?.trim() ?? ''
    if (selectedValue.length === 0) {
      return uniqueOptionsByValue(fetched)
    }

    const hasSelected = fetched.some((option) => option.value === selectedValue)
    if (hasSelected) {
      return uniqueOptionsByValue(fetched)
    }

    const fallbackOption = { value: selectedValue, label: selectedLabel.length > 0 ? selectedLabel : selectedValue }
    return uniqueOptionsByValue([fallbackOption, ...fetched])
  }, [optionsQuery.data?.options, props.value, props.valueLabel])

  const selected = useMemo<FieldOption | null>(() => {
    const currentValue = props.value?.trim() ?? ''
    if (currentValue.length === 0) {
      return null
    }
    return options.find((option) => option.value === currentValue) ?? { value: currentValue, label: currentValue }
  }, [options, props.value])

  const queryErrorMessage = optionsQuery.error ? getErrorMessage(optionsQuery.error) : ''
  const effectiveDisabled = props.disabled || optionsQuery.isError
  const helperText = queryErrorMessage.length > 0 ? queryErrorMessage : props.helperText

  return (
    <Autocomplete
      clearOnEscape
      disabled={effectiveDisabled}
      getOptionLabel={(option) => option.label}
      inputValue={inputValue}
      isOptionEqualToValue={(option, value) => option.value === value.value}
      loading={optionsQuery.isFetching}
      onChange={(_, option) => props.onChange(option ? option.value : null, option ? option.label : null)}
      onInputChange={(_, nextValue, reason) => {
        if (reason === 'input') {
          setInputValue(nextValue)
        }
      }}
      options={options}
      value={selected}
      renderInput={(params) => (
        <TextField
          {...params}
          error={queryErrorMessage.length > 0}
          helperText={helperText}
          label={props.label}
          InputProps={{
            ...params.InputProps,
            endAdornment: (
              <>
                {optionsQuery.isFetching ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null}
                {params.InputProps.endAdornment}
              </>
            )
          }}
        />
      )}
    />
  )
}

export function OrgUnitDetailsPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { orgCode } = useParams()
  const { hasPermission, t } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => formatAsOfDate(new Date()), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const includeDisabled = parseBool(searchParams.get('include_disabled'))
  const detailTab = parseDetailTab(searchParams.get('tab'))
  const requestedEffectiveDate = parseOptionalValue(searchParams.get('effective_date'))
  const auditEventUUID = parseOptionalValue(searchParams.get('audit_event_uuid'))

  const canWrite = hasPermission('orgunit.admin')
  const orgCodeValue = (orgCode ?? '').trim()

  const [actionState, setActionState] = useState<OrgActionState | null>(null)
  const [actionForm, setActionForm] = useState<OrgActionForm>(() => emptyActionForm(asOf))
  const [actionErrorMessage, setActionErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)
  const [auditLimitByOrg, setAuditLimitByOrg] = useState<Record<string, number>>({})
  const auditLimit = auditLimitByOrg[orgCodeValue] ?? 20

  const updateSearch = useCallback(
    (options: {
      asOf?: string | null
      includeDisabled?: boolean
      effectiveDate?: string | null
      auditEventUUID?: string | null
      tab?: DetailTab | null
    }) => {
      const nextParams = new URLSearchParams(searchParams)

      if (Object.hasOwn(options, 'asOf')) {
        if (options.asOf && options.asOf.length > 0) {
          nextParams.set('as_of', options.asOf)
        } else {
          nextParams.delete('as_of')
        }
      }

      if (Object.hasOwn(options, 'includeDisabled')) {
        if (options.includeDisabled) {
          nextParams.set('include_disabled', '1')
        } else {
          nextParams.delete('include_disabled')
        }
      }

      if (Object.hasOwn(options, 'effectiveDate')) {
        if (options.effectiveDate && options.effectiveDate.length > 0) {
          nextParams.set('effective_date', options.effectiveDate)
        } else {
          nextParams.delete('effective_date')
        }
      }

      if (Object.hasOwn(options, 'auditEventUUID')) {
        if (options.auditEventUUID && options.auditEventUUID.length > 0) {
          nextParams.set('audit_event_uuid', options.auditEventUUID)
        } else {
          nextParams.delete('audit_event_uuid')
        }
      }

      if (Object.hasOwn(options, 'tab')) {
        if (options.tab) {
          nextParams.set('tab', options.tab)
        } else {
          nextParams.delete('tab')
        }
      }

      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  const versionsQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'versions', orgCodeValue],
    queryFn: () => listOrgUnitVersions({ orgCode: orgCodeValue })
  })

  const versionItems = useMemo(() => versionsQuery.data?.versions ?? [], [versionsQuery.data])

  const effectiveDate = useMemo(() => {
    return resolveOrgUnitEffectiveDate({
      asOf,
      requestedEffectiveDate,
      versions: versionItems
    })
  }, [asOf, requestedEffectiveDate, versionItems])

  const detailQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'details', orgCodeValue, effectiveDate, includeDisabled],
    queryFn: () => getOrgUnitDetails({ orgCode: orgCodeValue, asOf: effectiveDate, includeDisabled })
  })

  const mutationCapabilitiesQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'mutation-capabilities', orgCodeValue, effectiveDate],
    queryFn: () => getOrgUnitMutationCapabilities({ orgCode: orgCodeValue, effectiveDate }),
    staleTime: 30_000
  })

  const appendCapabilitiesQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'append-capabilities', orgCodeValue, effectiveDate],
    queryFn: () => getOrgUnitAppendCapabilities({ orgCode: orgCodeValue, effectiveDate }),
    staleTime: 30_000
  })

  const auditQuery = useQuery({
    enabled: orgCodeValue.length > 0,
    queryKey: ['org-units', 'audit', orgCodeValue, auditLimit],
    queryFn: () => listOrgUnitAudit({ orgCode: orgCodeValue, limit: auditLimit })
  })

  const selectedVersionEventType = useMemo(() => {
    return versionItems.find((version) => version.effective_date === effectiveDate)?.event_type?.trim() || '-'
  }, [effectiveDate, versionItems])

  const correctEventCapability = mutationCapabilitiesQuery.data?.capabilities.correct_event
  const appendActionEventType = useMemo(
    () => (actionState ? appendEventTypeForAction(actionState.type) : null),
    [actionState]
  )
  const appendEventCapability = useMemo(() => {
    if (!appendActionEventType) {
      return null
    }
    return appendCapabilitiesQuery.data?.capabilities.event_update?.[appendActionEventType] ?? null
  }, [appendActionEventType, appendCapabilitiesQuery.data?.capabilities.event_update])
  const appendAllowedFieldSet = useMemo(
    () => new Set(appendEventCapability?.allowed_fields ?? []),
    [appendEventCapability?.allowed_fields]
  )
  const appendDenyReasons = useMemo(() => appendEventCapability?.deny_reasons ?? [], [appendEventCapability?.deny_reasons])
  const isAppendActionDisabled = useMemo(() => {
    if (!appendActionEventType) {
      return false
    }
    if (appendCapabilitiesQuery.isLoading || appendCapabilitiesQuery.isError) {
      return true
    }
    if (!appendEventCapability?.enabled) {
      return true
    }
    return false
  }, [appendActionEventType, appendCapabilitiesQuery.isError, appendCapabilitiesQuery.isLoading, appendEventCapability?.enabled])

  const selectedAuditEvent = useMemo(() => {
    const events = auditQuery.data?.events ?? []
    if (events.length === 0) {
      return null
    }
    if (auditEventUUID) {
      const match = events.find((event) => event.event_uuid === auditEventUUID)
      if (match) {
        return match
      }
    }
    return events[0]
  }, [auditEventUUID, auditQuery.data])

  useEffect(() => {
    if (detailTab !== 'audit') {
      return
    }
    const events = auditQuery.data?.events ?? []
    if (events.length === 0) {
      return
    }
    if (auditEventUUID && events.some((event) => event.event_uuid === auditEventUUID)) {
      return
    }
    const firstEvent = events[0]
    if (!firstEvent) {
      return
    }
    updateSearch({ auditEventUUID: firstEvent.event_uuid })
  }, [auditEventUUID, auditQuery.data, detailTab, updateSearch])

  const auditDiffRows = useMemo(() => {
    if (!selectedAuditEvent) {
      return []
    }
    return buildSnapshotDiff(selectedAuditEvent.before_snapshot, selectedAuditEvent.after_snapshot)
  }, [selectedAuditEvent])

  const actionExtFields = useMemo(() => detailQuery.data?.ext_fields ?? [], [detailQuery.data?.ext_fields])
  const actionPlainExtErrors = useMemo(() => {
    if (!actionState) {
      return {}
    }
    const isCorrectAction = actionState.type === 'correct'
    if (!isCorrectAction && !appendActionEventType) {
      return {}
    }
    if (isCorrectAction) {
      if (mutationCapabilitiesQuery.isLoading || mutationCapabilitiesQuery.isError || !correctEventCapability?.enabled) {
        return {}
      }
      const correctedEffectiveDateInput = actionForm.correctedEffectiveDate.trim()
      const inEffectiveDateCorrectionMode =
        correctedEffectiveDateInput.length > 0 && correctedEffectiveDateInput !== actionForm.effectiveDate.trim()
      if (inEffectiveDateCorrectionMode) {
        return {}
      }
    } else if (isAppendActionDisabled) {
      return {}
    }
    const mode = actionState.type === 'correct' ? 'null_empty' : 'null_empty'
    const errors: Record<string, string> = {}
    const correctAllowedFieldSet = new Set(correctEventCapability?.allowed_fields ?? [])
    actionExtFields.forEach((field) => {
      if (field.data_source_type !== 'PLAIN') {
        return
      }
      if (field.value_type === 'bool') {
        return
      }
      const fieldKey = field.field_key?.trim() ?? ''
      if (fieldKey.length === 0) {
        return
      }
      const editable = isCorrectAction ? correctAllowedFieldSet.has(fieldKey) : appendAllowedFieldSet.has(fieldKey)
      if (!editable) {
        return
      }
      const draft = actionForm.extDisplayValues[fieldKey] ?? ''
      const result = normalizePlainExtDraft({ valueType: field.value_type, draft, mode })
      if (result.errorCode) {
        errors[fieldKey] = result.errorCode
      }
    })
    return errors
  }, [
    actionExtFields,
    actionForm.correctedEffectiveDate,
    actionForm.effectiveDate,
    actionForm.extDisplayValues,
    actionState,
    appendActionEventType,
    appendAllowedFieldSet,
    correctEventCapability?.allowed_fields,
    correctEventCapability?.enabled,
    isAppendActionDisabled,
    mutationCapabilitiesQuery.isError,
    mutationCapabilitiesQuery.isLoading
  ])
  const hasActionPlainExtErrors = useMemo(() => Object.keys(actionPlainExtErrors).length > 0, [actionPlainExtErrors])

  const correctPatchPreview = useMemo(() => {
    if (actionState?.type !== 'correct') {
      return null
    }
    if (mutationCapabilitiesQuery.isError || !correctEventCapability) {
      return null
    }

    const correctedEffectiveDateInput = actionForm.correctedEffectiveDate.trim()
    const inEffectiveDateCorrectionMode =
      correctedEffectiveDateInput.length > 0 && correctedEffectiveDateInput !== actionForm.effectiveDate.trim()
    const correctAllowedFieldSet = new Set(correctEventCapability.allowed_fields ?? [])
    const effectiveDateValue = actionForm.effectiveDate.trim() || effectiveDate
    const normalizedPlainExtValues: Record<string, unknown> = {}
    if (!inEffectiveDateCorrectionMode) {
      for (const field of actionExtFields) {
        if (field.data_source_type !== 'PLAIN') {
          continue
        }
        if (field.value_type === 'bool') {
          continue
        }
        const fieldKey = field.field_key?.trim() ?? ''
        if (fieldKey.length === 0) {
          continue
        }
        if (!correctAllowedFieldSet.has(fieldKey)) {
          continue
        }
        const draft = actionForm.extDisplayValues[fieldKey] ?? ''
        const result = normalizePlainExtDraft({ valueType: field.value_type, draft, mode: 'null_empty' })
        if (result.errorCode) {
          return null
        }
        if (typeof result.normalized !== 'undefined') {
          normalizedPlainExtValues[fieldKey] = result.normalized
        }
      }
    }

    const original = {
      name: detailQuery.data?.org_unit.name?.trim() ?? '',
      parent_org_code: detailQuery.data?.org_unit.parent_org_code?.trim() ?? '',
      manager_pernr: detailQuery.data?.org_unit.manager_pernr?.trim() ?? '',
      is_business_unit: detailQuery.data?.org_unit.is_business_unit ?? false,
      effective_date: effectiveDateValue,
      ext: actionForm.originalExtValues
    }
    const next = {
      name: actionForm.name.trim(),
      parent_org_code: actionForm.parentOrgCode.trim(),
      manager_pernr: actionForm.managerPernr.trim(),
      is_business_unit: actionForm.isBusinessUnit,
      effective_date: effectiveDateValue,
      ext: { ...actionForm.extValues, ...normalizedPlainExtValues }
    }

    return buildCorrectPatch({
      capability: correctEventCapability,
      effectiveDate: effectiveDateValue,
      correctedEffectiveDate: actionForm.correctedEffectiveDate,
      original,
      next
    })
  }, [
    actionForm.correctedEffectiveDate,
    actionForm.effectiveDate,
    actionForm.extValues,
    actionForm.isBusinessUnit,
    actionForm.managerPernr,
    actionForm.name,
    actionForm.originalExtValues,
    actionForm.parentOrgCode,
    actionForm.extDisplayValues,
    actionState?.type,
    correctEventCapability,
    detailQuery.data,
    effectiveDate,
    actionExtFields,
    mutationCapabilitiesQuery.isError
  ])

  const isCorrectPatchEmpty = useMemo(() => {
    if (actionState?.type !== 'correct') {
      return false
    }
    if (!correctPatchPreview) {
      return true
    }
    return Object.keys(correctPatchPreview).length === 0
  }, [actionState?.type, correctPatchPreview])

  const refreshAfterWrite = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['org-units'] })
  }, [queryClient])

  function openAction(type: OrgActionType) {
    const details = detailQuery.data?.org_unit
    const extFields = detailQuery.data?.ext_fields ?? []
    const form = emptyActionForm(effectiveDate)
    form.orgCode = orgCodeValue
    form.name = details?.name ?? ''
    form.parentOrgCode = details?.parent_org_code ?? ''
    form.managerPernr = details?.manager_pernr ?? ''
    form.isBusinessUnit = details?.is_business_unit ?? false

    if (type === 'rescind_record') {
      form.effectiveDate = effectiveDate
    }

    if (type === 'correct' || appendEventTypeForAction(type) !== null) {
      const extValues: Record<string, unknown> = {}
      const originalExtValues: Record<string, unknown> = {}
      const extDisplayValues: Record<string, string> = {}

      extFields.forEach((field) => {
        const key = field.field_key?.trim()
        if (!key) {
          return
        }
        const valueType = field.value_type
        const normalizedValue = normalizeExtValueByType(valueType, field.value)
        extValues[key] = normalizedValue
        originalExtValues[key] = normalizedValue

        const displayValue = field.display_value?.trim()
        if (field.data_source_type === 'PLAIN') {
          extDisplayValues[key] =
            normalizedValue === null || normalizedValue === undefined ? '' : String(normalizedValue)
        } else {
          extDisplayValues[key] = displayValue && displayValue.length > 0 ? displayValue : toDisplayText(field.value)
        }
      })

      form.extValues = extValues
      form.originalExtValues = originalExtValues
      form.extDisplayValues = extDisplayValues
    }

    setActionErrorMessage('')
    setActionForm(form)
    setActionState({ type, targetCode: orgCodeValue.length > 0 ? orgCodeValue : null })
  }

  const actionMutation = useMutation({
    mutationFn: async () => {
      if (!actionState) {
        return
      }

      const type = actionState.type
      const targetCode = actionForm.orgCode.trim()
      if (!targetCode) {
        throw new Error(t('org_action_target_required'))
      }

      const effectiveDateValue = actionForm.effectiveDate.trim() || effectiveDate
      const appendEventType = appendEventTypeForAction(type)
      const appendCapability = appendEventType
        ? appendCapabilitiesQuery.data?.capabilities.event_update?.[appendEventType]
        : null
      const normalizedPlainExtValues: Record<string, unknown> = {}
      if (type === 'correct') {
        if (!mutationCapabilitiesQuery.isLoading && !mutationCapabilitiesQuery.isError && correctEventCapability?.enabled) {
          const correctedEffectiveDateInput = actionForm.correctedEffectiveDate.trim()
          const inEffectiveDateCorrectionMode =
            correctedEffectiveDateInput.length > 0 && correctedEffectiveDateInput !== actionForm.effectiveDate.trim()
          if (!inEffectiveDateCorrectionMode) {
            const correctAllowedFieldSet = new Set(correctEventCapability.allowed_fields ?? [])
            for (const field of actionExtFields) {
              if (field.data_source_type !== 'PLAIN' || field.value_type === 'bool') {
                continue
              }
              const fieldKey = field.field_key?.trim() ?? ''
              if (fieldKey.length === 0) {
                continue
              }
              if (!correctAllowedFieldSet.has(fieldKey)) {
                continue
              }
              const draft = actionForm.extDisplayValues[fieldKey] ?? ''
              const result = normalizePlainExtDraft({ valueType: field.value_type, draft, mode: 'null_empty' })
              if (result.errorCode) {
                throw new Error(t(result.errorCode as MessageKey))
              }
              if (typeof result.normalized !== 'undefined') {
                normalizedPlainExtValues[fieldKey] = result.normalized
              }
            }
          }
        }
      } else if (appendEventType !== null) {
        if (!isAppendActionDisabled) {
          for (const field of actionExtFields) {
            if (field.data_source_type !== 'PLAIN' || field.value_type === 'bool') {
              continue
            }
            const fieldKey = field.field_key?.trim() ?? ''
            if (fieldKey.length === 0) {
              continue
            }
            if (!appendAllowedFieldSet.has(fieldKey)) {
              continue
            }
            const draft = actionForm.extDisplayValues[fieldKey] ?? ''
            const result = normalizePlainExtDraft({ valueType: field.value_type, draft, mode: 'null_empty' })
            if (result.errorCode) {
              throw new Error(t(result.errorCode as MessageKey))
            }
            if (typeof result.normalized !== 'undefined') {
              normalizedPlainExtValues[fieldKey] = result.normalized
            }
          }
        }
      }
      const nextExtValues = {
        ...actionForm.extValues,
        ...normalizedPlainExtValues
      }
      const appendExtValues: Record<string, unknown> = {}
      for (const [fieldKey, nextValue] of Object.entries(nextExtValues)) {
        if (Object.is(actionForm.originalExtValues[fieldKey], nextValue)) {
          continue
        }
        appendExtValues[fieldKey] = nextValue
      }
      const appendPayloadValues: Record<string, unknown> = {
        org_code: targetCode,
        effective_date: effectiveDateValue,
        name: actionForm.name.trim(),
        parent_org_code: trimToUndefined(actionForm.parentOrgCode),
        is_business_unit: actionForm.isBusinessUnit,
        manager_pernr: trimToUndefined(actionForm.managerPernr),
        ...appendExtValues
      }

      switch (type) {
        case 'rename': {
          if (!appendCapability?.enabled || appendCapabilitiesQuery.isError) {
            throw new Error('append capabilities unavailable')
          }
          const payload = buildAppendPayload({ capability: appendCapability, values: appendPayloadValues })
          if (!payload) {
            throw new Error('append capability payload invalid')
          }
          await renameOrgUnit(payload as Parameters<typeof renameOrgUnit>[0])
          return
        }
        case 'move': {
          if (!appendCapability?.enabled || appendCapabilitiesQuery.isError) {
            throw new Error('append capabilities unavailable')
          }
          const payload = buildAppendPayload({ capability: appendCapability, values: appendPayloadValues })
          if (!payload) {
            throw new Error('append capability payload invalid')
          }
          await moveOrgUnit(payload as Parameters<typeof moveOrgUnit>[0])
          return
        }
        case 'set_business_unit': {
          if (!appendCapability?.enabled || appendCapabilitiesQuery.isError) {
            throw new Error('append capabilities unavailable')
          }
          const payload = buildAppendPayload({ capability: appendCapability, values: appendPayloadValues })
          if (!payload) {
            throw new Error('append capability payload invalid')
          }
          await setOrgUnitBusinessUnit({
            ...(payload as Omit<Parameters<typeof setOrgUnitBusinessUnit>[0], 'request_code'>),
            request_code: actionForm.requestCode.trim()
          })
          return
        }
        case 'enable': {
          if (!appendCapability?.enabled || appendCapabilitiesQuery.isError) {
            throw new Error('append capabilities unavailable')
          }
          const payload = buildAppendPayload({ capability: appendCapability, values: appendPayloadValues })
          if (!payload) {
            throw new Error('append capability payload invalid')
          }
          await enableOrgUnit(payload as Parameters<typeof enableOrgUnit>[0])
          return
        }
        case 'disable': {
          if (!appendCapability?.enabled || appendCapabilitiesQuery.isError) {
            throw new Error('append capabilities unavailable')
          }
          const payload = buildAppendPayload({ capability: appendCapability, values: appendPayloadValues })
          if (!payload) {
            throw new Error('append capability payload invalid')
          }
          await disableOrgUnit(payload as Parameters<typeof disableOrgUnit>[0])
          return
        }
        case 'correct': {
          if (mutationCapabilitiesQuery.isError || !correctEventCapability) {
            throw new Error('mutation capabilities unavailable')
          }
          if (hasActionPlainExtErrors) {
            const firstErrorKey = Object.values(actionPlainExtErrors)[0]
            throw new Error(firstErrorKey ? t(firstErrorKey as MessageKey) : 'plain ext fields invalid')
          }
          const patch = correctPatchPreview
          if (!patch || Object.keys(patch).length === 0) {
            throw new Error('PATCH_REQUIRED')
          }

          await correctOrgUnit({
            org_code: targetCode,
            effective_date: effectiveDateValue,
            request_id: actionForm.requestId.trim(),
            patch
          })
          return
        }
        case 'rescind_record':
          await rescindOrgUnitRecord({
            org_code: targetCode,
            effective_date: effectiveDateValue,
            request_id: actionForm.requestId.trim(),
            reason: actionForm.reason.trim()
          })
          return
        case 'rescind_org':
          await rescindOrgUnit({
            org_code: targetCode,
            request_id: actionForm.requestId.trim(),
            reason: actionForm.reason.trim()
          })
          return
      }
    },
    onSuccess: async () => {
      await refreshAfterWrite()
      if (actionState?.type === 'correct') {
        const nextEffectiveDate = actionForm.correctedEffectiveDate.trim()
        if (nextEffectiveDate.length > 0 && nextEffectiveDate !== effectiveDate) {
          updateSearch({ effectiveDate: nextEffectiveDate, tab: 'profile' })
        }
      }
      setActionState(null)
      setToast({ message: t('common_action_done'), severity: 'success' })
    },
    onError: (error) => {
      setActionErrorMessage(getErrorMessage(error))
    }
  })

  const titleLabel = useMemo(() => {
    const details = detailQuery.data?.org_unit
    if (!details) {
      return orgCodeValue.length > 0 ? `${orgCodeValue} · ${t('common_detail')}` : t('common_detail')
    }
    return `${details.name} · ${t('org_detail_title_suffix')}`
  }, [detailQuery.data, orgCodeValue, t])

  const breadcrumbCurrentLabel = useMemo(() => {
    const details = detailQuery.data?.org_unit
    if (details) {
      return `${details.name} (${details.org_code})`
    }
    return orgCodeValue.length > 0 ? orgCodeValue : t('common_detail')
  }, [detailQuery.data, orgCodeValue, t])

  const listLinkSearch = useMemo(() => {
    const params = new URLSearchParams()
    params.set('as_of', asOf)
    if (includeDisabled) {
      params.set('include_disabled', '1')
    }
    const value = params.toString()
    return value.length > 0 ? `?${value}` : ''
  }, [asOf, includeDisabled])

  const handleBack = useCallback(() => {
    if (window.history.length <= 1) {
      navigate({ pathname: '/org/units', search: listLinkSearch })
      return
    }
    navigate(-1)
  }, [listLinkSearch, navigate])

  const statusLabel = useMemo(() => {
    const raw = detailQuery.data?.org_unit?.status ?? ''
    return parseOrgStatus(raw) === 'active' ? t('org_status_active_short') : t('org_status_inactive_short')
  }, [detailQuery.data, t])

  const correctedEffectiveDateInput = actionForm.correctedEffectiveDate.trim()
  const inEffectiveDateCorrectionMode =
    actionState?.type === 'correct' &&
    correctedEffectiveDateInput.length > 0 &&
    correctedEffectiveDateInput !== actionForm.effectiveDate.trim()

  const allowedFieldSet = useMemo(() => {
    return new Set(correctEventCapability?.allowed_fields ?? [])
  }, [correctEventCapability?.allowed_fields])

  const correctDenyReasons = useMemo(() => {
    return correctEventCapability?.deny_reasons ?? []
  }, [correctEventCapability?.deny_reasons])

  const isCorrectActionDisabled = useMemo(() => {
    if (actionState?.type !== 'correct') {
      return false
    }
    if (mutationCapabilitiesQuery.isLoading) {
      return true
    }
    if (mutationCapabilitiesQuery.isError) {
      return true
    }
    if (!correctEventCapability?.enabled) {
      return true
    }
    return false
  }, [actionState?.type, correctEventCapability?.enabled, mutationCapabilitiesQuery.isError, mutationCapabilitiesQuery.isLoading])

  const isCorrectFieldEditable = useCallback(
    (fieldKey: string): boolean => {
      if (actionState?.type !== 'correct') {
        return true
      }
      if (isCorrectActionDisabled) {
        return false
      }
      if (inEffectiveDateCorrectionMode) {
        return fieldKey === 'effective_date'
      }
      return allowedFieldSet.has(fieldKey)
    },
    [actionState?.type, allowedFieldSet, inEffectiveDateCorrectionMode, isCorrectActionDisabled]
  )

  const isAppendFieldEditable = useCallback(
    (fieldKey: string): boolean => {
      if (!appendActionEventType) {
        return true
      }
      if (isAppendActionDisabled) {
        return false
      }
      return appendAllowedFieldSet.has(fieldKey)
    },
    [appendActionEventType, appendAllowedFieldSet, isAppendActionDisabled]
  )

  const isAppendActionButtonDisabled = useCallback(
    (type: OrgActionType): boolean => {
      if (!canWrite) {
        return true
      }
      const eventType = appendEventTypeForAction(type)
      if (!eventType) {
        return false
      }
      if (appendCapabilitiesQuery.isLoading || appendCapabilitiesQuery.isError) {
        return true
      }
      const capability = appendCapabilitiesQuery.data?.capabilities.event_update?.[eventType]
      if (!capability?.enabled) {
        return true
      }
      return false
    },
    [appendCapabilitiesQuery.data?.capabilities.event_update, appendCapabilitiesQuery.isError, appendCapabilitiesQuery.isLoading, canWrite]
  )

  if (orgCodeValue.length === 0) {
    return (
      <Alert severity='error'>
        {t('org_action_target_required')}
      </Alert>
    )
  }

  return (
    <>
      <Breadcrumbs sx={{ mb: 1 }}>
        <Link component={RouterLink} to={{ pathname: '/org/units', search: listLinkSearch }} underline='hover' color='inherit'>
          {t('nav_org_units')}
        </Link>
        <Typography color='text.primary'>{breadcrumbCurrentLabel}</Typography>
      </Breadcrumbs>

      <PageHeader
        title={titleLabel}
        actions={
          <>
            {canWrite ? (
              <Button
                onClick={() => {
                  const params = new URLSearchParams()
                  params.set('as_of', asOf)
                  navigate({ pathname: '/org/units/field-configs', search: `?${params.toString()}` })
                }}
                size='small'
                variant='outlined'
              >
                {t('nav_org_field_configs')}
              </Button>
            ) : null}
            <Button disabled={isAppendActionButtonDisabled('rename')} onClick={() => openAction('rename')} size='small' variant='outlined'>
              {t('org_action_rename')}
            </Button>
            <Button disabled={isAppendActionButtonDisabled('move')} onClick={() => openAction('move')} size='small' variant='outlined'>
              {t('org_action_move')}
            </Button>
            <Button
              disabled={isAppendActionButtonDisabled('set_business_unit')}
              onClick={() => openAction('set_business_unit')}
              size='small'
              variant='outlined'
            >
              {t('org_action_set_business_unit')}
            </Button>
          </>
        }
      />

      <Tabs
        onChange={(_, value: DetailTab) => {
          if (value === 'audit') {
            updateSearch({ tab: value })
            return
          }
          updateSearch({ tab: value, auditEventUUID: null })
        }}
        sx={{ mb: 1 }}
        value={detailTab}
      >
        <Tab label={t('org_tab_profile')} value='profile' />
        <Tab label={t('org_tab_audit')} value='audit' />
      </Tabs>

      {detailQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
      {detailQuery.error ? <Alert severity='error'>{getErrorMessage(detailQuery.error)}</Alert> : null}
      {canWrite && appendCapabilitiesQuery.isError ? (
        <Alert severity='warning' sx={{ mb: 1 }}>
          {t('org_append_capabilities_load_failed')}：{getErrorMessage(appendCapabilitiesQuery.error)}
        </Alert>
      ) : null}

      {detailTab === 'profile' ? (
        <Paper sx={{ p: 1.5 }} variant='outlined'>
          <Box
            sx={{
              display: 'grid',
              gap: 1.5,
              gridTemplateColumns: {
                xs: '1fr',
                md: '240px minmax(0, 1fr)'
              }
            }}
          >
            <Box sx={{ minWidth: 0 }}>
              <Typography sx={{ mb: 1 }} variant='subtitle2'>
                {t('org_column_effective_date')}
              </Typography>
              {versionsQuery.isLoading ? <Typography variant='body2'>{t('text_loading')}</Typography> : null}
              {versionsQuery.error ? <Alert severity='error'>{getErrorMessage(versionsQuery.error)}</Alert> : null}
              {versionItems.length > 0 ? (
                <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 420, overflow: 'auto', p: 0.5 }}>
	                  {versionItems.map((version) => (
	                    <ListItemButton
	                      data-testid={`org-version-${version.effective_date}`}
	                      key={`${version.event_id}-${version.effective_date}`}
	                      onClick={() => updateSearch({ effectiveDate: version.effective_date, tab: 'profile' })}
	                      selected={effectiveDate === version.effective_date}
	                      sx={{ borderRadius: 1, mb: 0.5 }}
	                    >
                      <ListItemText
                        primary={version.effective_date}
                        primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
                        secondary={version.event_type || '-'}
                        secondaryTypographyProps={{ variant: 'caption' }}
                      />
                    </ListItemButton>
                  ))}
                </List>
              ) : (
                <Typography color='text.secondary' variant='body2'>
                  {t('text_no_data')}
                </Typography>
              )}
            </Box>

            <Box sx={{ minWidth: 0 }}>
              {detailQuery.data ? (
                <>
                  <Stack alignItems='center' direction='row' flexWrap='wrap' justifyContent='space-between' spacing={1}>
                    <Typography variant='subtitle1'>
                      {t('org_detail_selected_version')}
                      {' '}
                      {effectiveDate}
                    </Typography>
                    <Chip
                      color={parseOrgStatus(detailQuery.data.org_unit.status) === 'active' ? 'success' : 'default'}
                      label={`${t('text_status')}：${statusLabel}`}
                      size='small'
                      variant='outlined'
                    />
                  </Stack>
                  <Divider sx={{ my: 1.2 }} />
                  <Stack spacing={1}>
                    <Typography variant='body2'>{t('org_version_event_type')}：{selectedVersionEventType}</Typography>
                    <Typography variant='body2'>{t('org_column_code')}：{detailQuery.data.org_unit.org_code}</Typography>
                    <Typography variant='body2'>{t('org_column_name')}：{detailQuery.data.org_unit.name}</Typography>
                    <Typography variant='body2'>
                      {t('org_column_parent')}：
                      {detailQuery.data.org_unit.parent_name?.trim()
                        ? `${detailQuery.data.org_unit.parent_name} (${detailQuery.data.org_unit.parent_org_code})`
                        : detailQuery.data.org_unit.parent_org_code || '-'}
                    </Typography>
                    <Typography variant='body2'>
                      {t('org_column_manager')}：
                      {detailQuery.data.org_unit.manager_name?.trim()
                        ? `${detailQuery.data.org_unit.manager_name} (${detailQuery.data.org_unit.manager_pernr})`
                        : detailQuery.data.org_unit.manager_pernr || '-'}
                    </Typography>
                    <Typography variant='body2'>
                      {t('org_column_is_business_unit')}：{detailQuery.data.org_unit.is_business_unit ? t('common_yes') : t('common_no')}
                    </Typography>
                  </Stack>

                  {detailQuery.data.ext_fields && detailQuery.data.ext_fields.length > 0 ? (
                    <>
                      <Divider sx={{ my: 1.2 }} />
                      <Typography variant='subtitle2'>{t('org_section_ext_fields')}</Typography>

                      {detailQuery.data.ext_fields.some((field) => {
                        const labelKey = field.label_i18n_key?.trim()
                        return !labelKey || !isMessageKey(labelKey)
                      }) ? (
                        <Alert severity='warning' sx={{ mt: 1 }}>
                          {t('org_ext_field_i18n_missing_warning')}
                        </Alert>
                      ) : null}

                      <Stack spacing={1} sx={{ mt: 1 }}>
                        {detailQuery.data.ext_fields.map((field) => {
                          const labelKey = field.label_i18n_key?.trim()
                          const label =
                            labelKey && isMessageKey(labelKey)
                              ? t(labelKey)
                              : field.field_key
                          const displayValue = field.display_value?.trim()
                          const valueText = displayValue && displayValue.length > 0 ? displayValue : toDisplayText(field.value)
                          const sourceWarning =
                            field.display_value_source === 'dict_fallback'
                              ? t('org_ext_field_display_value_fallback_warning')
                              : field.display_value_source === 'unresolved'
                              ? t('org_ext_field_display_value_unresolved_warning')
                              : ''

                          return (
                            <Box
                              key={field.field_key}
                              sx={{
                                display: 'grid',
                                gap: 0.5,
                                gridTemplateColumns: { xs: '1fr', sm: '220px minmax(0, 1fr)' }
                              }}
                            >
                              <Typography color='text.secondary' variant='body2'>
                                {label}
                              </Typography>
                              <Stack spacing={0.25} sx={{ minWidth: 0 }}>
                                <Typography sx={{ wordBreak: 'break-word' }} variant='body2'>
                                  {valueText}
                                </Typography>
                                {sourceWarning.length > 0 ? (
                                  <Typography color='warning.main' variant='caption'>
                                    {sourceWarning}
                                  </Typography>
                                ) : null}
                              </Stack>
                            </Box>
                          )
                        })}
                      </Stack>
                    </>
                  ) : null}

                  <Stack direction='row' flexWrap='wrap' spacing={1} sx={{ mt: 1.5 }}>
                    <Button disabled={isAppendActionButtonDisabled('enable')} onClick={() => openAction('enable')} size='small' variant='outlined'>
                      {t('org_action_enable')}
                    </Button>
                    <Button disabled={isAppendActionButtonDisabled('disable')} onClick={() => openAction('disable')} size='small' variant='outlined'>
                      {t('org_action_disable')}
                    </Button>
                    <Button disabled={!detailQuery.data} onClick={() => openAction('correct')} size='small' variant='outlined'>
                      {t('org_action_correct')}
                    </Button>
                    <Button disabled={!canWrite} onClick={() => openAction('rescind_record')} size='small' variant='outlined'>
                      {t('org_action_rescind_record')}
                    </Button>
                    <Button color='error' disabled={!canWrite} onClick={() => openAction('rescind_org')} size='small' variant='outlined'>
                      {t('org_action_rescind_org')}
                    </Button>
                  </Stack>
                </>
              ) : (
                <Typography color='text.secondary' variant='body2'>
                  {detailQuery.isLoading ? t('text_loading') : t('text_no_data')}
                </Typography>
              )}
            </Box>
          </Box>
        </Paper>
      ) : null}

      {detailTab === 'audit' ? (
        <Paper sx={{ p: 1.5 }} variant='outlined'>
          <Box
            sx={{
              display: 'grid',
              gap: 1.5,
              gridTemplateColumns: {
                xs: '1fr',
                md: '240px minmax(0, 1fr)'
              }
            }}
          >
            <Box sx={{ minWidth: 0 }}>
              <Typography sx={{ mb: 1 }} variant='subtitle2'>
                {t('org_audit_timeline_time')}
              </Typography>
	              {auditQuery.data ? (
	                <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 420, overflow: 'auto', p: 0.5 }}>
	                  {auditQuery.data.events.map((event) => (
	                    <ListItemButton
	                      data-testid={`org-audit-${event.event_uuid}`}
	                      key={event.event_id}
	                      onClick={() => updateSearch({ tab: 'audit', auditEventUUID: event.event_uuid })}
	                      selected={selectedAuditEvent?.event_uuid === event.event_uuid}
	                      sx={{ borderRadius: 1, mb: 0.5 }}
	                    >
	                      <Box sx={{ alignItems: 'flex-start', display: 'flex', gap: 1, justifyContent: 'space-between', width: '100%' }}>
	                        <ListItemText
	                          primary={formatTxTime(event.tx_time)}
	                          primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
	                          secondary={formatAuditActor(event.initiator_name, event.initiator_employee_id)}
	                          secondaryTypographyProps={{ variant: 'caption' }}
	                        />
	                        {event.is_rescinded ? (
	                          <Chip color='warning' label={t('org_audit_rescinded')} size='small' sx={{ mt: 0.25 }} variant='outlined' />
	                        ) : null}
	                      </Box>
	                    </ListItemButton>
	                  ))}
	                </List>
	              ) : null}
              {auditQuery.data?.has_more ? (
                <Button
                  onClick={() =>
                    setAuditLimitByOrg((previous) => ({
                      ...previous,
                      [orgCodeValue]: (previous[orgCodeValue] ?? 20) + 20
                    }))
                  }
                  size='small'
                  sx={{ mt: 1 }}
                  variant='outlined'
                >
                  {t('org_audit_load_more')}
                </Button>
              ) : null}
            </Box>

            <Box sx={{ minWidth: 0 }}>
              {selectedAuditEvent ? (
                <>
                  <Typography variant='subtitle1'>{t('org_detail_selected_event')}</Typography>
                  <Divider sx={{ my: 1.2 }} />
	                  <Stack spacing={1}>
	                    <Typography variant='body2'>
	                      {t('org_column_effective_date')}：{selectedAuditEvent.effective_date}
	                      {' · '}
	                      {t('org_audit_timeline_time')}：{formatTxTime(selectedAuditEvent.tx_time)}
	                    </Typography>
	                    <Typography variant='body2'>
	                      {t('org_audit_operator')}：{formatAuditActor(selectedAuditEvent.initiator_name, selectedAuditEvent.initiator_employee_id)}
	                    </Typography>
	                    <Typography variant='body2'>
	                      event_uuid：{selectedAuditEvent.event_uuid}
	                    </Typography>
	                    <Typography variant='body2'>
	                      {t('org_version_event_type')}：{selectedAuditEvent.event_type}
	                    </Typography>
	                    <Typography variant='body2'>
	                      {t('org_request_code')}：{toDisplayText(selectedAuditEvent.request_code)}
	                    </Typography>
	                    <Typography variant='body2'>
	                      {t('org_reason')}：{toDisplayText(selectedAuditEvent.reason)}
	                    </Typography>
	                    <Typography variant='body2'>
	                      {t('org_audit_rescinded')}：{selectedAuditEvent.is_rescinded ? t('common_yes') : t('common_no')}
	                    </Typography>
	                    {selectedAuditEvent.is_rescinded ? (
	                      <>
	                        <Typography variant='body2'>
	                          {t('org_audit_rescinded_by_request_code')}：{toDisplayText(selectedAuditEvent.rescinded_by_request_code)}
	                        </Typography>
	                        <Typography variant='body2'>
	                          {t('org_audit_rescinded_by_tx_time')}：
	                          {toDisplayText(
	                            selectedAuditEvent.rescinded_by_tx_time ? formatTxTime(selectedAuditEvent.rescinded_by_tx_time) : ''
	                          )}
	                        </Typography>
	                        <Typography variant='body2'>
	                          {t('org_audit_rescinded_by_event_uuid')}：{toDisplayText(selectedAuditEvent.rescinded_by_event_uuid)}
	                        </Typography>
	                      </>
	                    ) : null}
	                  </Stack>
	                  <Box sx={{ mt: 1.5 }}>
	                    <Typography sx={{ mb: 0.8 }} variant='subtitle2'>
	                      {t('org_audit_diff_title')}
	                    </Typography>
                    {auditDiffRows.length > 0 ? (
                      <TableContainer component={Paper} sx={{ border: 1, borderColor: 'divider' }} variant='outlined'>
                        <Table size='small'>
                          <TableHead>
                            <TableRow>
                              <TableCell>{t('org_audit_diff_field')}</TableCell>
                              <TableCell>{t('org_audit_diff_before')}</TableCell>
                              <TableCell>{t('org_audit_diff_after')}</TableCell>
                            </TableRow>
                          </TableHead>
                          <TableBody>
                            {auditDiffRows.map((row) => (
                              <TableRow key={row.field}>
                                <TableCell sx={{ maxWidth: 180, verticalAlign: 'top', wordBreak: 'break-word' }}>{row.field}</TableCell>
                                <TableCell sx={{ maxWidth: 300, verticalAlign: 'top', wordBreak: 'break-word' }}>{row.before}</TableCell>
                                <TableCell sx={{ maxWidth: 300, verticalAlign: 'top', wordBreak: 'break-word' }}>{row.after}</TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </TableContainer>
                    ) : (
                      <Typography color='text.secondary' variant='body2'>
                        {t('org_audit_no_diff')}
                      </Typography>
                    )}
                  </Box>
                  <Accordion disableGutters sx={{ border: 1, borderColor: 'divider', borderRadius: 1, mt: 1.2 }}>
                    <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                      <Typography variant='body2'>{t('org_audit_raw_payload')}</Typography>
                    </AccordionSummary>
                    <AccordionDetails>
                      <Box
                        component='pre'
                        sx={{
                          bgcolor: 'background.default',
                          border: 1,
                          borderColor: 'divider',
                          borderRadius: 1,
                          fontSize: 12,
                          m: 0,
                          maxHeight: 260,
                          overflow: 'auto',
                          p: 1,
                          whiteSpace: 'pre-wrap',
                          wordBreak: 'break-word'
                        }}
                      >
                        {JSON.stringify(
                          {
                            payload: selectedAuditEvent.payload,
                            before_snapshot: selectedAuditEvent.before_snapshot,
                            after_snapshot: selectedAuditEvent.after_snapshot
                          },
                          null,
                          2
                        )}
                      </Box>
                    </AccordionDetails>
                  </Accordion>
                </>
              ) : (
                <Typography color='text.secondary' variant='body2'>
                  {t('text_no_data')}
                </Typography>
              )}
            </Box>
          </Box>

          {auditQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
          {auditQuery.error ? <Alert severity='error'>{getErrorMessage(auditQuery.error)}</Alert> : null}
        </Paper>
      ) : null}

      <Box sx={{ mt: 3 }}>
        <Button onClick={handleBack} variant='outlined'>
          {t('common_back')}
        </Button>
      </Box>

      <Dialog onClose={() => setActionState(null)} open={actionState !== null} fullWidth maxWidth='sm'>
        <DialogTitle>
          {actionState ? actionLabel(actionState.type, t) : ''}
        </DialogTitle>
        <DialogContent>
          {actionErrorMessage.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {actionErrorMessage}
            </Alert>
          ) : null}
          {actionState ? (
            <Stack spacing={2} sx={{ mt: 0.5 }}>
              <TextField disabled label={t('org_column_code')} value={actionForm.orgCode} />

              {actionState.type === 'correct' ? (
                <>
                  {mutationCapabilitiesQuery.isLoading ? (
                    <Alert severity='info'>{t('text_loading')}</Alert>
                  ) : null}
                  {mutationCapabilitiesQuery.isError ? (
                    <Alert severity='error'>
                      {t('org_correct_capabilities_load_failed')}：{getErrorMessage(mutationCapabilitiesQuery.error)}
                    </Alert>
                  ) : null}
                  {!correctEventCapability?.enabled && correctDenyReasons.length > 0 ? (
                    <Alert severity='warning'>
                      {t('org_correct_denied')}：{correctDenyReasons.join(', ')}
                    </Alert>
                  ) : null}
                  {inEffectiveDateCorrectionMode ? (
                    <Alert severity='warning'>
                      {t('org_correct_effective_date_only_mode')}
                    </Alert>
                  ) : null}
                  {correctEventCapability?.enabled && isCorrectPatchEmpty ? (
                    <Alert severity='info'>{t('org_correct_no_changes')}</Alert>
                  ) : null}
                </>
              ) : null}

              {appendActionEventType ? (
                <>
                  {appendCapabilitiesQuery.isLoading ? (
                    <Alert severity='info'>{t('text_loading')}</Alert>
                  ) : null}
                  {appendCapabilitiesQuery.isError ? (
                    <Alert severity='error'>
                      {t('org_append_capabilities_load_failed')}：{getErrorMessage(appendCapabilitiesQuery.error)}
                    </Alert>
                  ) : null}
                  {!appendEventCapability?.enabled && appendDenyReasons.length > 0 ? (
                    <Alert severity='warning'>
                      {t('org_append_denied')}：{appendDenyReasons.join(', ')}
                    </Alert>
                  ) : null}
                </>
              ) : null}

              {actionState.type === 'rename' || actionState.type === 'correct' ? (
                <TextField
                  disabled={
                    actionState.type === 'correct'
                      ? !isCorrectFieldEditable('name')
                      : actionState.type === 'rename'
                      ? !isAppendFieldEditable('name')
                      : false
                  }
                  helperText={
                    actionState.type === 'correct'
                      ? !allowedFieldSet.has('name')
                        ? t('org_correct_field_not_allowed_helper')
                        : undefined
                      : actionState.type === 'rename' && !appendAllowedFieldSet.has('name')
                      ? t('org_append_field_not_allowed_helper')
                      : undefined
                  }
                  label={t('org_column_name')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, name: event.target.value }))}
                  value={actionForm.name}
                />
              ) : null}

              {actionState.type === 'move' || actionState.type === 'correct' ? (
                <TextField
                  disabled={
                    actionState.type === 'correct'
                      ? !isCorrectFieldEditable('parent_org_code')
                      : actionState.type === 'move'
                      ? !isAppendFieldEditable('parent_org_code')
                      : false
                  }
                  helperText={
                    actionState.type === 'correct'
                      ? !allowedFieldSet.has('parent_org_code')
                        ? t('org_correct_field_not_allowed_helper')
                        : undefined
                      : actionState.type === 'move' && !appendAllowedFieldSet.has('parent_org_code')
                      ? t('org_append_field_not_allowed_helper')
                      : undefined
                  }
                  label={t('org_column_parent')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, parentOrgCode: event.target.value }))}
                  value={actionForm.parentOrgCode}
                />
              ) : null}

              {actionState.type === 'correct' ? (
                <TextField
                  disabled={!isCorrectFieldEditable('manager_pernr')}
                  helperText={!allowedFieldSet.has('manager_pernr') ? t('org_correct_field_not_allowed_helper') : undefined}
                  label={t('org_column_manager')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
                  value={actionForm.managerPernr}
                />
              ) : null}

              {actionState.type === 'set_business_unit' || actionState.type === 'correct' ? (
                <Box>
                  <FormControlLabel
                    control={
                      <Switch
                        checked={actionForm.isBusinessUnit}
                        disabled={
                          actionState.type === 'correct'
                            ? !isCorrectFieldEditable('is_business_unit')
                            : actionState.type === 'set_business_unit'
                            ? !isAppendFieldEditable('is_business_unit')
                            : false
                        }
                        onChange={(event) => setActionForm((previous) => ({ ...previous, isBusinessUnit: event.target.checked }))}
                      />
                    }
                    label={t('org_column_is_business_unit')}
                  />
                  {actionState.type === 'correct' && !allowedFieldSet.has('is_business_unit') ? (
                    <Typography color='text.secondary' variant='caption'>
                      {t('org_correct_field_not_allowed_helper')}
                    </Typography>
                  ) : actionState.type === 'set_business_unit' && !appendAllowedFieldSet.has('is_business_unit') ? (
                    <Typography color='text.secondary' variant='caption'>
                      {t('org_append_field_not_allowed_helper')}
                    </Typography>
                  ) : null}
                </Box>
              ) : null}

              {actionState.type === 'correct' ? (
                <TextField
                  disabled
                  InputLabelProps={{ shrink: true }}
                  label={t('org_column_effective_date')}
                  type='date'
                  value={actionForm.effectiveDate}
                />
              ) : actionState.type !== 'rescind_org' ? (
                <TextField
                  disabled={appendActionEventType ? !isAppendFieldEditable('effective_date') : false}
                  helperText={
                    appendActionEventType && !appendAllowedFieldSet.has('effective_date')
                      ? t('org_append_field_not_allowed_helper')
                      : undefined
                  }
                  InputLabelProps={{ shrink: true }}
                  label={t('org_column_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, effectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.effectiveDate}
                />
              ) : null}

              {actionState.type === 'correct' ? (
                <TextField
                  disabled={!isCorrectFieldEditable('effective_date')}
                  helperText={!allowedFieldSet.has('effective_date') ? t('org_correct_field_not_allowed_helper') : undefined}
                  InputLabelProps={{ shrink: true }}
                  label={t('org_corrected_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, correctedEffectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.correctedEffectiveDate}
                />
              ) : null}

              {(actionState.type === 'correct' || appendActionEventType) && actionExtFields.length > 0 ? (
                <>
                  <Divider sx={{ my: 0.5 }} />
                  <Typography variant='subtitle2'>{t('org_section_ext_fields')}</Typography>

                  {actionExtFields.map((field) => {
                    const fieldKey = field.field_key?.trim()
                    if (!fieldKey) {
                      return null
                    }
                    const labelKey = field.label_i18n_key?.trim()
                    const label =
                      labelKey && isMessageKey(labelKey)
                        ? t(labelKey)
                        : fieldKey

                    const dataSourceType = field.data_source_type
                    const editableBase =
                      actionState.type === 'correct' ? isCorrectFieldEditable(fieldKey) : isAppendFieldEditable(fieldKey)
                    const editable = editableBase && (dataSourceType === 'PLAIN' || dataSourceType === 'DICT' || dataSourceType === 'ENTITY')
                    const notAllowedHelper =
                      actionState.type === 'correct'
                        ? !allowedFieldSet.has(fieldKey)
                          ? t('org_correct_field_not_allowed_helper')
                          : undefined
                        : !appendAllowedFieldSet.has(fieldKey)
                        ? t('org_append_field_not_allowed_helper')
                        : undefined

                    const rawValue = actionForm.extValues[fieldKey]
                    const valueText = actionForm.extDisplayValues[fieldKey] ?? ''
                    const validationErrorKey = actionPlainExtErrors[fieldKey]
                    const validationErrorText = validationErrorKey ? t(validationErrorKey as MessageKey) : ''

                    if (dataSourceType === 'PLAIN') {
                      const valueType = field.value_type
                      if (valueType === 'bool') {
                        const current =
                          rawValue === true ? 'true' : rawValue === false ? 'false' : ''
                        return (
                          <TextField
                            key={fieldKey}
                            select
                            disabled={!editable}
                            helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                            error={!notAllowedHelper && validationErrorText.length > 0}
                            label={label}
                            value={current}
                            onChange={(event) => {
                              const nextValue = event.target.value
                              const next =
                                nextValue === 'true'
                                  ? true
                                  : nextValue === 'false'
                                  ? false
                                  : null
                              setActionForm((previous) => ({
                                ...previous,
                                extValues: {
                                  ...previous.extValues,
                                  [fieldKey]: next
                                },
                                extDisplayValues: {
                                  ...previous.extDisplayValues,
                                  [fieldKey]: nextValue
                                }
                              }))
                            }}
                          >
                            <MenuItem value=''>-</MenuItem>
                            <MenuItem value='true'>{t('common_yes')}</MenuItem>
                            <MenuItem value='false'>{t('common_no')}</MenuItem>
                          </TextField>
                        )
                      }

                      if (valueType === 'int') {
                        const currentValue =
                          rawValue === null || rawValue === undefined
                            ? ''
                            : typeof rawValue === 'number'
                            ? rawValue
                            : typeof rawValue === 'string'
                            ? rawValue
                            : String(rawValue)

                        return (
                          <TextField
                            key={fieldKey}
                            disabled={!editable}
                            helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                            error={!notAllowedHelper && validationErrorText.length > 0}
                            label={label}
                            type='number'
                            value={valueText.length > 0 ? valueText : String(currentValue)}
                            onChange={(event) => {
                              const nextValue = event.target.value
                              setActionForm((previous) => ({
                                ...previous,
                                extDisplayValues: {
                                  ...previous.extDisplayValues,
                                  [fieldKey]: nextValue
                                }
                              }))
                            }}
                          />
                        )
                      }

                      if (valueType === 'date') {
                        return (
                          <TextField
                            key={fieldKey}
                            disabled={!editable}
                            helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                            error={!notAllowedHelper && validationErrorText.length > 0}
                            InputLabelProps={{ shrink: true }}
                            label={label}
                            type='date'
                            value={valueText}
                            onChange={(event) => {
                              const nextValue = event.target.value
                              setActionForm((previous) => ({
                                ...previous,
                                extDisplayValues: {
                                  ...previous.extDisplayValues,
                                  [fieldKey]: nextValue
                                }
                              }))
                            }}
                          />
                        )
                      }

                      const currentValue =
                        rawValue === null || rawValue === undefined
                          ? ''
                          : typeof rawValue === 'string'
                          ? rawValue
                          : String(rawValue)

                      return (
                        <TextField
                          key={fieldKey}
                          disabled={!editable}
                          helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                          error={!notAllowedHelper && validationErrorText.length > 0}
                          label={label}
                          value={valueText.length > 0 ? valueText : currentValue}
                          onChange={(event) => {
                            const nextValue = event.target.value
                            setActionForm((previous) => ({
                              ...previous,
                              extDisplayValues: {
                                ...previous.extDisplayValues,
                                [fieldKey]: nextValue
                              }
                            }))
                          }}
                        />
                      )
                    }

                    if (dataSourceType !== 'DICT' && dataSourceType !== 'ENTITY') {
                      return (
                        <TextField
                          key={fieldKey}
                          disabled
                          helperText={notAllowedHelper}
                          label={label}
                          value={valueText.length > 0 ? valueText : toDisplayText(rawValue)}
                        />
                      )
                    }

                    const currentValue =
                      rawValue === null || rawValue === undefined
                        ? null
                        : typeof rawValue === 'string'
                        ? rawValue
                        : String(rawValue)

                    return (
                      <OrgUnitExtFieldSelect
                        key={fieldKey}
                        asOf={actionForm.effectiveDate}
                        disabled={!editable}
                        fieldKey={fieldKey}
                        helperText={notAllowedHelper}
                        label={label}
                        value={currentValue}
                        valueLabel={actionForm.extDisplayValues[fieldKey] ?? null}
                        onChange={(nextValue, nextLabel) => {
                          setActionForm((previous) => ({
                            ...previous,
                            extValues: {
                              ...previous.extValues,
                              [fieldKey]: nextValue
                            },
                            extDisplayValues: {
                              ...previous.extDisplayValues,
                              [fieldKey]: nextLabel ?? ''
                            }
                          }))
                        }}
                      />
                    )
                  })}

                  {actionExtFields.some((field) => field.data_source_type !== 'PLAIN' && !['DICT', 'ENTITY'].includes(field.data_source_type)) ? (
                    <Alert severity='warning'>
                      {t('org_ext_field_unknown_type_warning')}
                    </Alert>
                  ) : null}
                </>
              ) : null}

              {actionState.type === 'set_business_unit' ? (
                <TextField
                  label={t('org_request_code')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, requestCode: event.target.value }))}
                  value={actionForm.requestCode}
                />
              ) : null}

              {actionState.type === 'correct' || actionState.type === 'rescind_record' || actionState.type === 'rescind_org' ? (
                <TextField
                  label={t('org_request_id')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, requestId: event.target.value }))}
                  value={actionForm.requestId}
                />
              ) : null}

              {actionState.type === 'rescind_record' || actionState.type === 'rescind_org' ? (
                <TextField
                  label={t('org_reason')}
                  multiline
                  onChange={(event) => setActionForm((previous) => ({ ...previous, reason: event.target.value }))}
                  rows={3}
                  value={actionForm.reason}
                />
              ) : null}
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button disabled={actionMutation.isPending} onClick={() => setActionState(null)}>
            {t('common_cancel')}
          </Button>
          <Button
            disabled={
              actionMutation.isPending ||
              (actionState?.type === 'correct' ? isCorrectActionDisabled || isCorrectPatchEmpty || hasActionPlainExtErrors : false) ||
              (actionState && appendEventTypeForAction(actionState.type)
                ? isAppendActionDisabled || hasActionPlainExtErrors
                : false)
            }
            onClick={() => actionMutation.mutate()}
            variant='contained'
          >
            {t('common_confirm')}
          </Button>
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
