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
  getOrgUnitFieldOptions,
  getOrgUnitDetails,
  getOrgUnitWriteCapabilities,
  listOrgUnitAudit,
  listOrgUnitVersions,
  rescindOrgUnit,
  rescindOrgUnitRecord,
  writeOrgUnit,
  type OrgUnitWriteIntent
} from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { PageHeader } from '../../components/PageHeader'
import { isMessageKey, type MessageKey } from '../../i18n/messages'
import { normalizePlainExtDraft } from './orgUnitPlainExtValidation'
import { resolveOrgUnitEffectiveDate } from './orgUnitVersionSelection'
import { buildOrgUnitWritePatch } from './orgUnitWritePatch'
import {
  planRecordEffectiveDate,
  validatePlannedEffectiveDate,
  type RecordDatePlan,
  type RecordWizardMode
} from './orgUnitRecordDateRules'

type DetailTab = 'profile' | 'audit'
type OrgStatus = 'active' | 'inactive'
type OrgUnitWriteActionIntent = Exclude<OrgUnitWriteIntent, 'create_org'>
type OrgActionType = OrgUnitWriteActionIntent | 'delete'

const detailTabs: readonly DetailTab[] = ['profile', 'audit']

interface OrgActionState {
  type: OrgActionType
  targetCode: string | null
}

interface OrgActionForm {
  orgCode: string
  name: string
  parentOrgCode: string
  status: OrgStatus
  managerPernr: string
  effectiveDate: string
  correctedEffectiveDate: string
  isBusinessUnit: boolean
  extValues: Record<string, unknown>
  originalExtValues: Record<string, unknown>
  originalName: string
  originalParentOrgCode: string
  originalStatus: string
  originalIsBusinessUnit: boolean
  originalManagerPernr: string
  extDisplayValues: Record<string, string>
  requestID: string
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

function isISODate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value.trim())
}

function parseOrgStatus(raw: string): OrgStatus {
  const value = raw.trim().toLowerCase()
  return value === 'disabled' || value === 'inactive' ? 'inactive' : 'active'
}

function trimToUndefined(value: string): string | undefined {
  const normalized = value.trim()
  return normalized.length > 0 ? normalized : undefined
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
    status: 'active',
    managerPernr: '',
    effectiveDate,
    correctedEffectiveDate: '',
    isBusinessUnit: false,
    extValues: {},
    originalExtValues: {},
    originalName: '',
    originalParentOrgCode: '',
    originalStatus: '',
    originalIsBusinessUnit: false,
    originalManagerPernr: '',
    extDisplayValues: {},
    requestID: newRequestID(),
    reason: ''
  }
}

function actionLabel(type: OrgActionType, t: (key: MessageKey) => string): string {
  switch (type) {
    case 'add_version':
      return t('org_action_add_version')
    case 'insert_version':
      return t('org_action_insert_version')
    case 'correct':
      return t('org_action_correct')
    case 'delete':
      return t('org_action_delete')
    default:
      return ''
  }
}

function isWriteActionType(type: OrgActionType): type is OrgUnitWriteActionIntent {
  switch (type) {
    case 'add_version':
    case 'insert_version':
    case 'correct':
      return true
    case 'delete':
    default:
      return false
  }
}

function buildDeleteActionLabel(isDeleteOrg: boolean, t: (key: MessageKey) => string): string {
  if (isDeleteOrg) {
    return t('org_action_rescind_org')
  }
  return t('org_action_rescind_record')
}

function toKernelStatus(status: OrgStatus): 'active' | 'disabled' {
  return status === 'active' ? 'active' : 'disabled'
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
  valueType: 'text' | 'int' | 'uuid' | 'bool' | 'date' | 'numeric',
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
    case 'numeric':
      if (typeof rawValue === 'number' && Number.isFinite(rawValue)) {
        return rawValue
      }
      if (typeof rawValue === 'string') {
        const trimmed = rawValue.trim()
        if (trimmed.length === 0) {
          return null
        }
        const parsed = Number.parseFloat(trimmed)
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
  // 注意：不要把 Autocomplete 的 inputValue 作为受控值传进去，否则在选择 option 时
  // MUI 会用 reason='reset' 更新输入框文本，但我们若只处理 reason='input' 会导致输入框显示为空。
  // 这里仅用 keyword 驱动 options 查询，让 Autocomplete 自己管理输入框的显示值。
  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebouncedValue(keyword, 250)

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
      isOptionEqualToValue={(option, value) => option.value === value.value}
      loading={optionsQuery.isFetching}
      onChange={(_, option) => {
        props.onChange(option ? option.value : null, option ? option.label : null)
        // 选择后清空 keyword，避免把选中 label 当作下一次 options 查询关键词。
        setKeyword('')
      }}
      onInputChange={(_, nextValue, reason) => {
        if (reason === 'input') {
          setKeyword(nextValue)
          return
        }
        if (reason === 'clear') {
          setKeyword('')
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
  const [recordDatePlan, setRecordDatePlan] = useState<RecordDatePlan | null>(null)
  const [actionErrorMessage, setActionErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)
  const [auditLimitByOrg, setAuditLimitByOrg] = useState<Record<string, number>>({})
  const auditLimit = auditLimitByOrg[orgCodeValue] ?? 20
  const actionType = actionState?.type ?? null

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

  const activeWriteIntent = useMemo<OrgUnitWriteActionIntent | null>(() => {
    if (!actionType) {
      return null
    }
    return isWriteActionType(actionType) ? actionType : null
  }, [actionType])

  const actionWriteEffectiveDate = useMemo(() => {
    if (!activeWriteIntent) {
      return effectiveDate
    }
    if (activeWriteIntent === 'correct') {
      const corrected = actionForm.correctedEffectiveDate.trim()
      if (isISODate(corrected)) {
        return corrected
      }
    }
    const input = actionForm.effectiveDate.trim()
    return input.length > 0 ? input : effectiveDate
  }, [actionForm.correctedEffectiveDate, actionForm.effectiveDate, activeWriteIntent, effectiveDate])

  const writeCapabilitiesQuery = useQuery({
    enabled:
      orgCodeValue.length > 0 &&
      canWrite &&
      activeWriteIntent !== null &&
      isISODate(actionWriteEffectiveDate) &&
      (activeWriteIntent !== 'correct' || isISODate(actionForm.effectiveDate)),
    queryKey: [
      'org-units',
      'write-capabilities',
      activeWriteIntent,
      orgCodeValue,
      actionWriteEffectiveDate,
      activeWriteIntent === 'correct' ? actionForm.effectiveDate : ''
    ],
    queryFn: () =>
      getOrgUnitWriteCapabilities({
        intent: activeWriteIntent ?? 'correct',
        orgCode: orgCodeValue,
        effectiveDate: actionWriteEffectiveDate,
        targetEffectiveDate: activeWriteIntent === 'correct' ? actionForm.effectiveDate : undefined
      }),
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
  const writeCapability = writeCapabilitiesQuery.data
  const writeAllowedFieldSet = useMemo(() => new Set(writeCapability?.allowed_fields ?? []), [writeCapability?.allowed_fields])
  const writeDenyReasons = useMemo(() => writeCapability?.deny_reasons ?? [], [writeCapability?.deny_reasons])
  const isWriteAction = activeWriteIntent !== null
  const isWriteActionDisabled = useMemo(() => {
    if (!isWriteAction) {
      return false
    }
    if (writeCapabilitiesQuery.isLoading || writeCapabilitiesQuery.isError) {
      return true
    }
    if (!writeCapability?.enabled) {
      return true
    }
    return false
  }, [isWriteAction, writeCapabilitiesQuery.isError, writeCapabilitiesQuery.isLoading, writeCapability?.enabled])

  const isWriteFieldEditable = useCallback(
    (fieldKey: string): boolean => {
      if (!isWriteAction) {
        return true
      }
      if (isWriteActionDisabled) {
        return false
      }
      return writeAllowedFieldSet.has(fieldKey)
    },
    [isWriteAction, isWriteActionDisabled, writeAllowedFieldSet]
  )

  const actionPlainExtErrors = useMemo(() => {
    if (!isWriteAction) {
      return {}
    }
    if (isWriteActionDisabled) {
      return {}
    }
    const errors: Record<string, string> = {}
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
      if (!writeAllowedFieldSet.has(fieldKey)) {
        return
      }
      const draft = actionForm.extDisplayValues[fieldKey] ?? ''
      const result = normalizePlainExtDraft({ valueType: field.value_type, draft, mode: 'null_empty' })
      if (result.errorCode) {
        errors[fieldKey] = result.errorCode
      }
    })
    return errors
  }, [
    actionExtFields,
    actionForm.extDisplayValues,
    isWriteAction,
    isWriteActionDisabled,
    writeAllowedFieldSet
  ])
  const hasActionPlainExtErrors = useMemo(() => Object.keys(actionPlainExtErrors).length > 0, [actionPlainExtErrors])

  const writePatchPreview = useMemo(() => {
    if (!isWriteAction) {
      return null
    }
    if (writeCapabilitiesQuery.isError || !writeCapability) {
      return null
    }

    const normalizedPlainExtValues: Record<string, unknown> = {}
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
      if (!writeAllowedFieldSet.has(fieldKey)) {
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

    const original = {
      name: actionForm.originalName,
      parent_org_code: actionForm.originalParentOrgCode,
      status: actionForm.originalStatus,
      manager_pernr: actionForm.originalManagerPernr,
      is_business_unit: actionForm.originalIsBusinessUnit,
      ext: actionForm.originalExtValues
    }
    const next = {
      name: actionForm.name.trim() || undefined,
      parent_org_code: trimToUndefined(actionForm.parentOrgCode),
      status: toKernelStatus(actionForm.status),
      manager_pernr: actionForm.managerPernr.trim(),
      is_business_unit: actionForm.isBusinessUnit,
      ext: { ...actionForm.extValues, ...normalizedPlainExtValues }
    }

    return buildOrgUnitWritePatch({
      capability: writeCapability,
      original,
      next
    })
  }, [
    actionForm.extValues,
    actionForm.isBusinessUnit,
    actionForm.managerPernr,
    actionForm.name,
    actionForm.originalExtValues,
    actionForm.originalIsBusinessUnit,
    actionForm.originalManagerPernr,
    actionForm.originalName,
    actionForm.originalParentOrgCode,
    actionForm.originalStatus,
    actionForm.parentOrgCode,
    actionForm.status,
    actionForm.extDisplayValues,
    actionExtFields,
    isWriteAction,
    writeAllowedFieldSet,
    writeCapability,
    writeCapabilitiesQuery.isError
  ])

  const isWritePatchEmpty = useMemo(() => {
    if (!isWriteAction) {
      return false
    }
    if (!writePatchPreview) {
      return true
    }
    return Object.keys(writePatchPreview).length === 0
  }, [isWriteAction, writePatchPreview])

  const recordWizardMode: RecordWizardMode | null = actionType === 'add_version' ? 'add' : actionType === 'insert_version' ? 'insert' : null
  const isRecordWizard = recordWizardMode === 'add' || recordWizardMode === 'insert'
  const isCorrectAction = actionType === 'correct'
  const isDeleteAction = actionType === 'delete'
  const shouldDeleteOrg = versionItems.length <= 1
  const recordWizardValidation = useMemo(() => {
    if (!isRecordWizard) {
      return { ok: true } as const
    }
    if (!recordDatePlan) {
      return { ok: false, reason: 'out_of_range' } as const
    }
    return validatePlannedEffectiveDate({ plan: recordDatePlan, effectiveDate: actionForm.effectiveDate })
  }, [actionForm.effectiveDate, isRecordWizard, recordDatePlan])
  const recordWizardValidationMessageKey = useMemo((): MessageKey | null => {
    if (!isRecordWizard) {
      return null
    }
    if (recordWizardValidation.ok) {
      return null
    }
    switch (recordWizardValidation.reason) {
      case 'required':
        return 'org_record_wizard_effective_date_required'
      case 'invalid_format':
        return 'org_record_wizard_effective_date_invalid'
      case 'no_slot':
        return 'org_record_wizard_effective_date_no_slot'
      case 'out_of_range':
      default:
        return 'org_record_wizard_effective_date_out_of_range'
    }
  }, [isRecordWizard, recordWizardValidation])

  const recordWizardPlanHint = useMemo(() => {
    if (!isRecordWizard || !recordDatePlan) {
      return null
    }
    const min = recordDatePlan.minDate ?? ''
    const max = recordDatePlan.maxDate ?? ''
    const defaultDate = recordDatePlan.defaultDate
    switch (recordDatePlan.kind) {
      case 'add':
        return { severity: 'info' as const, text: t('org_record_wizard_date_hint_add', { min, default: defaultDate }) }
      case 'insert':
        return { severity: 'info' as const, text: t('org_record_wizard_date_hint_insert', { min, max, default: defaultDate }) }
      case 'insert_as_add':
        return { severity: 'info' as const, text: t('org_record_wizard_date_hint_insert_as_add', { min, default: defaultDate }) }
      case 'insert_no_slot':
        return { severity: 'warning' as const, text: t('org_record_wizard_date_hint_no_slot', { min, max }) }
      default:
        return { severity: 'warning' as const, text: t('org_record_wizard_date_hint_unknown') }
    }
  }, [isRecordWizard, recordDatePlan, t])

  const refreshAfterWrite = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['org-units'] })
  }, [queryClient])

  function openRecordWizard(mode: RecordWizardMode) {
    const details = detailQuery.data?.org_unit
    if (!details) {
      return
    }

    const plan = planRecordEffectiveDate({ mode, versions: versionItems, selectedEffectiveDate: effectiveDate })
    if (plan.kind === 'invalid_input') {
      setToast({ message: t('org_record_wizard_open_failed'), severity: 'error' })
      return
    }

    const extFields = detailQuery.data?.ext_fields ?? []
    const form = emptyActionForm(plan.defaultDate)
    form.orgCode = orgCodeValue
    form.name = details?.name ?? ''
    form.parentOrgCode = details?.parent_org_code ?? ''
    form.status = parseOrgStatus(details?.status ?? '')
    form.managerPernr = details?.manager_pernr ?? ''
    form.isBusinessUnit = details?.is_business_unit ?? false
    form.originalName = details?.name?.trim() ?? ''
    form.originalParentOrgCode = details?.parent_org_code?.trim() ?? ''
    form.originalStatus = toKernelStatus(parseOrgStatus(details?.status ?? ''))
    form.originalIsBusinessUnit = details?.is_business_unit ?? false
    form.originalManagerPernr = details?.manager_pernr?.trim() ?? ''

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

    setActionErrorMessage('')
    setRecordDatePlan(plan)
    setActionForm(form)
    setActionState({
      type: mode === 'add' ? 'add_version' : 'insert_version',
      targetCode: orgCodeValue.length > 0 ? orgCodeValue : null
    })
  }

  function openCorrectAction() {
    const details = detailQuery.data?.org_unit
    const extFields = detailQuery.data?.ext_fields ?? []
    const form = emptyActionForm(effectiveDate)
    form.orgCode = orgCodeValue
    form.name = details?.name ?? ''
    form.parentOrgCode = details?.parent_org_code ?? ''
    form.status = parseOrgStatus(details?.status ?? '')
    form.managerPernr = details?.manager_pernr ?? ''
    form.isBusinessUnit = details?.is_business_unit ?? false
    form.originalName = details?.name?.trim() ?? ''
    form.originalParentOrgCode = details?.parent_org_code?.trim() ?? ''
    form.originalStatus = toKernelStatus(parseOrgStatus(details?.status ?? ''))
    form.originalIsBusinessUnit = details?.is_business_unit ?? false
    form.originalManagerPernr = details?.manager_pernr?.trim() ?? ''

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

    setActionErrorMessage('')
    setRecordDatePlan(null)
    setActionForm(form)
    setActionState({ type: 'correct', targetCode: orgCodeValue.length > 0 ? orgCodeValue : null })
  }

  function openDeleteAction() {
    const details = detailQuery.data?.org_unit
    const form = emptyActionForm(effectiveDate)
    form.orgCode = orgCodeValue
    form.name = details?.name ?? ''
    form.parentOrgCode = details?.parent_org_code ?? ''
    form.status = parseOrgStatus(details?.status ?? '')
    form.managerPernr = details?.manager_pernr ?? ''
    form.isBusinessUnit = details?.is_business_unit ?? false
    form.reason = ''

    setActionErrorMessage('')
    setRecordDatePlan(null)
    setActionForm(form)
    setActionState({ type: 'delete', targetCode: orgCodeValue.length > 0 ? orgCodeValue : null })
  }

  const actionMutation = useMutation({
    mutationFn: async () => {
      if (!actionState) {
        throw new Error('action state missing')
      }

      const type: OrgActionType = actionState.type
      const targetCode = actionForm.orgCode.trim()
      if (!targetCode) {
        throw new Error(t('org_action_target_required'))
      }

      if (type === 'add_version' || type === 'insert_version') {
        if (!recordDatePlan) {
          throw new Error(t('org_record_wizard_effective_date_out_of_range'))
        }
        const validation = validatePlannedEffectiveDate({ plan: recordDatePlan, effectiveDate: actionForm.effectiveDate })
        if (!validation.ok) {
          switch (validation.reason) {
            case 'required':
              throw new Error(t('org_record_wizard_effective_date_required'))
            case 'invalid_format':
              throw new Error(t('org_record_wizard_effective_date_invalid'))
            case 'no_slot':
              throw new Error(t('org_record_wizard_effective_date_no_slot'))
            case 'out_of_range':
            default:
              throw new Error(t('org_record_wizard_effective_date_out_of_range'))
          }
        }
      }

      if (type === 'delete') {
        const reason = actionForm.reason.trim()
        if (reason.length === 0) {
          throw new Error(t('org_delete_reason_required'))
        }
        const requestID = actionForm.requestID.trim() || newRequestID()

        if (shouldDeleteOrg) {
          await rescindOrgUnit({
            org_code: targetCode,
            request_id: requestID,
            reason
          })
          return { deletedOrg: true }
        }

        await rescindOrgUnitRecord({
          org_code: targetCode,
          effective_date: effectiveDate,
          request_id: requestID,
          reason
        })
        return { deletedOrg: false }
      }

      if (!activeWriteIntent || activeWriteIntent !== type) {
        throw new Error('write intent mismatch')
      }
      if (!writeCapability?.enabled || writeCapabilitiesQuery.isError) {
        throw new Error('write capabilities unavailable')
      }
      if (hasActionPlainExtErrors) {
        const firstErrorKey = Object.values(actionPlainExtErrors)[0]
        throw new Error(firstErrorKey ? t(firstErrorKey as MessageKey) : 'plain ext fields invalid')
      }
      const patch = writePatchPreview
      if (!patch) {
        throw new Error('write patch unavailable')
      }
      if (Object.keys(patch).length === 0) {
        throw new Error('ORG_UPDATE_PATCH_EMPTY')
      }

      const requestID = actionForm.requestID.trim() || newRequestID()
      const effectiveDateValue = activeWriteIntent === 'correct' ? actionWriteEffectiveDate : actionForm.effectiveDate.trim()

      await writeOrgUnit({
        intent: activeWriteIntent,
        org_code: targetCode,
        effective_date: effectiveDateValue,
        target_effective_date: activeWriteIntent === 'correct' ? actionForm.effectiveDate.trim() : undefined,
        request_id: requestID,
        patch: patch as Parameters<typeof writeOrgUnit>[0]['patch']
      })
      return { deletedOrg: false }
    },
    onSuccess: async (result) => {
      await refreshAfterWrite()
      if (actionState?.type === 'correct') {
        const nextEffectiveDate = actionWriteEffectiveDate
        if (nextEffectiveDate.length > 0 && nextEffectiveDate !== effectiveDate) {
          updateSearch({ effectiveDate: nextEffectiveDate, tab: 'profile' })
        }
      } else if (actionState?.type === 'add_version' || actionState?.type === 'insert_version') {
        const nextEffectiveDate = actionForm.effectiveDate.trim()
        if (nextEffectiveDate.length > 0 && nextEffectiveDate !== effectiveDate) {
          updateSearch({ effectiveDate: nextEffectiveDate, tab: 'profile' })
        }
      } else if (actionState?.type === 'delete') {
        if (result.deletedOrg) {
          navigate({ pathname: '/org/units', search: listLinkSearchValue })
        } else {
          updateSearch({ effectiveDate: null, tab: 'profile' })
        }
      }
      setRecordDatePlan(null)
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

  const listLinkParams = new URLSearchParams()
  listLinkParams.set('as_of', asOf)
  if (includeDisabled) {
    listLinkParams.set('include_disabled', '1')
  }
  const listLinkSearch = listLinkParams.toString()
  const listLinkSearchValue = listLinkSearch.length > 0 ? `?${listLinkSearch}` : ''

  const handleBack = useCallback(() => {
    if (window.history.length <= 1) {
      navigate({ pathname: '/org/units', search: listLinkSearchValue })
      return
    }
    navigate(-1)
  }, [listLinkSearchValue, navigate])

  const statusLabel = useMemo(() => {
    const raw = detailQuery.data?.org_unit?.status ?? ''
    return parseOrgStatus(raw) === 'active' ? t('org_status_active_short') : t('org_status_inactive_short')
  }, [detailQuery.data, t])
  const deleteActionLabel = useMemo(() => buildDeleteActionLabel(shouldDeleteOrg, t), [shouldDeleteOrg, t])

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
        <Link component={RouterLink} to={{ pathname: '/org/units', search: listLinkSearchValue }} underline='hover' color='inherit'>
          {t('nav_org_units')}
        </Link>
        <Typography color='text.primary'>{breadcrumbCurrentLabel}</Typography>
      </Breadcrumbs>

      <PageHeader
        title={titleLabel}
        actions={
          <Stack direction='row' flexWrap='wrap' spacing={1}>
            <Button
              component={RouterLink}
              disabled={!canWrite}
              size='small'
              to={{ pathname: '/org/units', search: listLinkSearchValue }}
              variant='outlined'
            >
              {t('org_action_create')}
            </Button>
            <Button
              disabled={!canWrite || versionsQuery.isLoading || versionItems.length === 0}
              onClick={() => openRecordWizard('add')}
              size='small'
              variant='outlined'
            >
              {t('org_action_add_version')}
            </Button>
            <Button
              disabled={!canWrite || versionsQuery.isLoading || versionItems.length === 0}
              onClick={() => openRecordWizard('insert')}
              size='small'
              variant='outlined'
            >
              {t('org_action_insert_version')}
            </Button>
            <Button disabled={!canWrite || !detailQuery.data} onClick={() => openCorrectAction()} size='small' variant='outlined'>
              {t('org_action_correct')}
            </Button>
            <Button
              color='error'
              disabled={!canWrite || versionsQuery.isLoading || versionItems.length === 0}
              onClick={() => openDeleteAction()}
              size='small'
              variant='outlined'
            >
              {t('org_action_delete')}
            </Button>
          </Stack>
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
              <Stack alignItems='center' direction='row' flexWrap='wrap' justifyContent='space-between' spacing={1} sx={{ mb: 1 }}>
                <Typography variant='subtitle2'>
                  {t('org_column_effective_date')}
                </Typography>
              </Stack>
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
                  {canWrite ? (
                    <Button
                      onClick={() => {
                        const params = new URLSearchParams()
                        params.set('as_of', asOf)
                        navigate({ pathname: '/org/units/field-configs', search: `?${params.toString()}` })
                      }}
                      size='small'
                      sx={{ mt: 0.5 }}
                      variant='text'
                    >
                      {t('nav_org_field_configs')}
                    </Button>
                  ) : null}
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
                        const literalLabel = field.label?.trim() ?? ''
                        const isCustom = field.field_key?.trim().toLowerCase().startsWith('x_')
                        if (literalLabel.length > 0 || isCustom) {
                          return false
                        }
                        return !labelKey || !isMessageKey(labelKey)
                      }) ? (
                        <Alert severity='warning' sx={{ mt: 1 }}>
                          {t('org_ext_field_i18n_missing_warning')}
                        </Alert>
                      ) : null}

                      <Stack spacing={1} sx={{ mt: 1 }}>
                        {detailQuery.data.ext_fields.map((field) => {
                          const labelKey = field.label_i18n_key?.trim()
                          const literalLabel = field.label?.trim() ?? ''
                          const label =
                            labelKey && isMessageKey(labelKey)
                              ? t(labelKey)
                              : literalLabel.length > 0
                              ? literalLabel
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
	                      {t('org_request_id')}：{toDisplayText(selectedAuditEvent.request_id)}
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
	                          {t('org_audit_rescinded_by_request_id')}：{toDisplayText(selectedAuditEvent.rescinded_by_request_id)}
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

      <Dialog
        onClose={() => {
          setRecordDatePlan(null)
          setActionState(null)
        }}
        open={actionState !== null}
        fullWidth
        maxWidth='sm'
      >
        <DialogTitle>{actionState ? actionLabel(actionState.type, t) : ''}</DialogTitle>
        <DialogContent>
          {actionErrorMessage.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {actionErrorMessage}
            </Alert>
          ) : null}
          {actionState ? (
            <Stack spacing={2} sx={{ mt: 0.5 }}>
              <TextField disabled label={t('org_column_code')} value={actionForm.orgCode} />

              {isDeleteAction ? (
                <Alert severity='warning'>
                  {t('org_action_delete')}：{deleteActionLabel}
                </Alert>
              ) : null}

              {isWriteAction ? (
                <>
                  {writeCapabilitiesQuery.isLoading ? <Alert severity='info'>{t('text_loading')}</Alert> : null}
                  {writeCapabilitiesQuery.isError ? (
                    <Alert severity='error'>{getErrorMessage(writeCapabilitiesQuery.error)}</Alert>
                  ) : null}
                  {!writeCapability?.enabled && writeDenyReasons.length > 0 ? (
                    <Alert severity='warning'>{writeDenyReasons.join(', ')}</Alert>
                  ) : null}
                  {writeCapability?.enabled && isWritePatchEmpty ? <Alert severity='info'>{t('org_write_no_changes')}</Alert> : null}
                </>
              ) : null}

              {!isDeleteAction ? (
                <TextField
                  disabled={!isWriteFieldEditable('name')}
                  label={t('org_column_name')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, name: event.target.value }))}
                  value={actionForm.name}
                />
              ) : null}

              {!isDeleteAction ? (
                <TextField
                  disabled={!isWriteFieldEditable('parent_org_code')}
                  label={t('org_column_parent')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, parentOrgCode: event.target.value }))}
                  value={actionForm.parentOrgCode}
                />
              ) : null}

              {!isDeleteAction ? (
                <TextField
                  disabled={!isWriteFieldEditable('manager_pernr')}
                  label={t('org_column_manager')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
                  value={actionForm.managerPernr}
                />
              ) : null}

              {!isDeleteAction ? (
                <TextField
                  select
                  disabled={!isWriteFieldEditable('status')}
                  label={t('org_column_status')}
                  onChange={(event) =>
                    setActionForm((previous) => ({
                      ...previous,
                      status: event.target.value === 'inactive' ? 'inactive' : 'active'
                    }))
                  }
                  value={actionForm.status}
                >
                  <MenuItem value='active'>{t('org_status_active_short')}</MenuItem>
                  <MenuItem value='inactive'>{t('org_status_inactive_short')}</MenuItem>
                </TextField>
              ) : null}

              {!isDeleteAction ? (
                <Box>
                  <FormControlLabel
                    control={
                      <Switch
                        checked={actionForm.isBusinessUnit}
                        disabled={!isWriteFieldEditable('is_business_unit')}
                        onChange={(event) => setActionForm((previous) => ({ ...previous, isBusinessUnit: event.target.checked }))}
                      />
                    }
                    label={t('org_column_is_business_unit')}
                  />
                </Box>
              ) : null}

              {isCorrectAction ? (
                <>
                  <TextField
                    disabled
                    InputLabelProps={{ shrink: true }}
                    label={t('org_column_effective_date')}
                    type='date'
                    value={actionForm.effectiveDate}
                  />
                  <TextField
                    InputLabelProps={{ shrink: true }}
                    label={t('org_corrected_effective_date')}
                    onChange={(event) => setActionForm((previous) => ({ ...previous, correctedEffectiveDate: event.target.value }))}
                    type='date'
                    value={actionForm.correctedEffectiveDate}
                  />
                </>
              ) : isWriteAction ? (
                <TextField
                  helperText={
                    isRecordWizard && recordWizardValidationMessageKey
                      ? t(recordWizardValidationMessageKey)
                      : isRecordWizard && recordWizardPlanHint
                      ? recordWizardPlanHint.text
                      : undefined
                  }
                  error={isRecordWizard && recordWizardValidationMessageKey !== null}
                  InputLabelProps={{ shrink: true }}
                  label={t('org_column_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, effectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.effectiveDate}
                />
              ) : null}

              {isWriteAction && actionExtFields.length > 0 ? (
                <>
                  <Divider sx={{ my: 0.5 }} />
                  <Typography variant='subtitle2'>{t('org_section_ext_fields')}</Typography>

                  {actionExtFields.map((field) => {
                    const fieldKey = field.field_key?.trim()
                    if (!fieldKey) {
                      return null
                    }
                    const labelKey = field.label_i18n_key?.trim()
                    const literalLabel = field.label?.trim() ?? ''
                    const label =
                      labelKey && isMessageKey(labelKey)
                        ? t(labelKey)
                        : literalLabel.length > 0
                        ? literalLabel
                        : fieldKey

                    const dataSourceType = field.data_source_type
                    const editableBase = isWriteFieldEditable(fieldKey)
                    const editable = editableBase && (dataSourceType === 'PLAIN' || dataSourceType === 'DICT' || dataSourceType === 'ENTITY')
                    const notAllowedHelper = undefined

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
                            helperText={validationErrorText.length > 0 ? validationErrorText : undefined}
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
                            helperText={validationErrorText.length > 0 ? validationErrorText : undefined}
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
                            helperText={validationErrorText.length > 0 ? validationErrorText : undefined}
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
                          helperText={validationErrorText.length > 0 ? validationErrorText : undefined}
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
                        asOf={actionWriteEffectiveDate}
                        disabled={!editable}
                        fieldKey={fieldKey}
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

              <TextField
                label={t('org_request_id')}
                onChange={(event) => setActionForm((previous) => ({ ...previous, requestID: event.target.value }))}
                value={actionForm.requestID}
              />

              {isDeleteAction ? (
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
          <Button
            disabled={actionMutation.isPending}
            onClick={() => {
              setRecordDatePlan(null)
              setActionState(null)
            }}
          >
            {t('common_cancel')}
          </Button>
          <Button
            disabled={
              actionMutation.isPending ||
              (isDeleteAction ? actionForm.reason.trim().length === 0 : false) ||
              (isWriteAction ? isWriteActionDisabled || isWritePatchEmpty || hasActionPlainExtErrors : false) ||
              (isWriteAction ? (isWriteFieldEditable('name') && actionForm.name.trim().length === 0) : false) ||
              (isRecordWizard ? !recordDatePlan || !recordWizardValidation.ok : false)
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
