import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  CircularProgress,
  Divider,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select,
  Snackbar,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef, GridPaginationModel, GridSortModel } from '@mui/x-data-grid'
import {
  getOrgUnitWriteCapabilities,
  getOrgUnitFieldOptions,
  listOrgUnits,
  listOrgUnitsPage,
  listOrgUnitFieldConfigs,
  searchOrgUnit,
  writeOrgUnit,
  type OrgUnitAPIItem,
  type OrgUnitListSortField,
  type OrgUnitListSortOrder,
  type OrgUnitListStatusFilter
} from '../../api/orgUnits'
import { ApiClientError } from '../../api/errors'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { type TreePanelNode, TreePanel } from '../../components/TreePanel'
import { isMessageKey, type MessageKey } from '../../i18n/messages'
import { trackUiEvent } from '../../observability/tracker'
import {
  fromGridSortModel,
  parseGridQueryState,
  patchGridQueryState,
  toGridSortModel
} from '../../utils/gridQueryState'
import { normalizePlainExtDraft } from './orgUnitPlainExtValidation'
import { clearExtQueryParams, parseExtSortField, parseSortOrder } from './orgUnitListExtQuery'

type OrgStatus = 'active' | 'inactive'

interface OrgUnitRow {
  id: string
  code: string
  name: string
  status: OrgStatus
  isBusinessUnit: boolean
}

interface CreateOrgUnitForm {
  orgCode: string
  name: string
  parentOrgCode: string
  status: OrgStatus
  managerPernr: string
  effectiveDate: string
  isBusinessUnit: boolean
  requestID: string
  extValues: Record<string, unknown>
  extDisplayValues: Record<string, string>
}

const sortableFields = ['code', 'name', 'status'] as const

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

function parseOrgStatus(raw: string): OrgStatus {
  const value = raw.trim().toLowerCase()
  return value === 'disabled' || value === 'inactive' ? 'inactive' : 'active'
}

function trimToUndefined(value: string): string | undefined {
  const normalized = value.trim()
  return normalized.length > 0 ? normalized : undefined
}

function toOrgUnitRow(item: OrgUnitAPIItem): OrgUnitRow {
  return {
    id: item.org_code,
    code: item.org_code,
    name: item.name,
    status: parseOrgStatus(item.status),
    isBusinessUnit: Boolean(item.is_business_unit)
  }
}

function buildTreeNodes(
  roots: OrgUnitAPIItem[],
  childrenByParent: Record<string, OrgUnitAPIItem[]>
): TreePanelNode[] {
  function build(item: OrgUnitAPIItem, path: Set<string>): TreePanelNode {
    const status = parseOrgStatus(item.status)
    const labelSuffix = status === 'inactive' ? ' · Inactive' : ''

    if (path.has(item.org_code)) {
      return { id: item.org_code, label: `${item.name} (${item.org_code})${labelSuffix}`, hasChildren: false }
    }

    const nextPath = new Set(path)
    nextPath.add(item.org_code)
    const childrenLoaded = Object.hasOwn(childrenByParent, item.org_code)
    const children = childrenByParent[item.org_code] ?? []
    const childNodes = children.map((child) => build(child, nextPath))
    const hasChildren = childrenLoaded ? childNodes.length > 0 : item.has_children === true

    return {
      id: item.org_code,
      label: `${item.name} (${item.org_code})${labelSuffix}`,
      hasChildren,
      children: childNodes.length > 0 ? childNodes : undefined
    }
  }

  return roots.map((root) => build(root, new Set()))
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

type FieldOption = { value: string; label: string }

type OrgUnitExtQueryField = Pick<import('../../api/orgUnits').OrgUnitTenantFieldConfig, 'field_key' | 'value_type' | 'data_source_type'> & {
  label: string
  allowFilter: boolean
  allowSort: boolean
}

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const handle = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(handle)
  }, [delayMs, value])

  return debounced
}

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

function ExtFilterValueInput(props: {
  field: OrgUnitExtQueryField | null
  asOf: string
  label: string
  value: string
  disabled: boolean
  helperText?: string
  formatError?: (error: unknown) => string
  onChange: (nextValue: string) => void
}) {
  // 不把 Autocomplete 的 inputValue 作为受控值，否则选择 option 时（reason='reset'）
  // 若不同步更新会出现“已选中但输入框显示为空”的现象。
  // 这里仅用 keyword 驱动 options 查询，让 Autocomplete 自己管理输入框显示值。
  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebouncedValue(keyword, 250)
  const field = props.field
  const isDictField = Boolean(field && field.data_source_type === 'DICT')

  const optionsQuery = useQuery({
    enabled: isDictField && !props.disabled,
    queryKey: ['org-units', 'field-options', field?.field_key ?? '', props.asOf, debouncedKeyword],
    queryFn: () => {
      if (!field) {
        throw new Error('org ext filter field is required')
      }
      return getOrgUnitFieldOptions({
        fieldKey: field.field_key,
        asOf: props.asOf,
        keyword: debouncedKeyword,
        limit: 20
      })
    },
    staleTime: 30_000
  })

  const options = useMemo<FieldOption[]>(() => {
    if (!isDictField) {
      return []
    }
    const fetched = optionsQuery.data?.options ?? []
    const selectedValue = props.value.trim()
    if (selectedValue.length === 0) {
      return uniqueOptionsByValue(fetched)
    }

    const hasSelected = fetched.some((option) => option.value === selectedValue)
    if (hasSelected) {
      return uniqueOptionsByValue(fetched)
    }

    const fallbackOption = { value: selectedValue, label: selectedValue }
    return uniqueOptionsByValue([fallbackOption, ...fetched])
  }, [isDictField, optionsQuery.data?.options, props.value])

  const selected = useMemo<FieldOption | null>(() => {
    if (!isDictField) {
      return null
    }
    const currentValue = props.value.trim()
    if (currentValue.length === 0) {
      return null
    }
    return options.find((option) => option.value === currentValue) ?? { value: currentValue, label: currentValue }
  }, [isDictField, options, props.value])

  const queryErrorMessage =
    isDictField && optionsQuery.error
      ? props.formatError
        ? props.formatError(optionsQuery.error)
        : getErrorMessage(optionsQuery.error)
      : ''
  const effectiveDisabled = props.disabled || (isDictField && optionsQuery.isError)
  const helperText = queryErrorMessage.length > 0 ? queryErrorMessage : props.helperText

  if (!isDictField) {
    return (
      <TextField
        disabled={props.disabled || !field}
        label={props.label}
        onChange={(event) => props.onChange(event.target.value)}
        value={props.value}
        helperText={helperText}
        inputProps={{ 'data-testid': 'org-ext-filter-value' }}
      />
    )
  }

  return (
    <Autocomplete
      clearOnEscape
      disabled={effectiveDisabled}
      getOptionLabel={(option) => option.label}
      isOptionEqualToValue={(option, value) => option.value === value.value}
      loading={optionsQuery.isFetching}
      onChange={(_, option) => {
        props.onChange(option ? option.value : '')
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
          inputProps={{ ...params.inputProps, 'data-testid': 'org-ext-filter-value' }}
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

function emptyCreateForm(asOf: string, parentOrgCode: string | null): CreateOrgUnitForm {
  return {
    orgCode: '',
    name: '',
    parentOrgCode: parentOrgCode ?? '',
    status: 'active',
    managerPernr: '',
    effectiveDate: asOf,
    isBusinessUnit: false,
    requestID: `org-create:${Date.now()}`,
    extValues: {},
    extDisplayValues: {}
  }
}

export function OrgUnitsPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { t, tenantId, hasPermission } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => formatAsOfDate(new Date()), [])

  const query = useMemo(
    () =>
      parseGridQueryState(searchParams, {
        statusValues: ['active', 'inactive'] as const,
        sortFields: sortableFields
      }),
    [searchParams]
  )

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const includeDisabled = parseBool(searchParams.get('include_disabled'))
  const extFilterFieldKey = parseOptionalValue(searchParams.get('ext_filter_field_key'))
  const extFilterValue = parseOptionalValue(searchParams.get('ext_filter_value'))
  const extSortFieldKey = parseExtSortField(searchParams.get('sort'))
  const extSortOrder = parseSortOrder(searchParams.get('order')) ?? 'asc'

  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [statusInput, setStatusInput] = useState<'all' | OrgStatus>(query.status)
  const [asOfInput, setAsOfInput] = useState(asOf)
  const [includeDisabledInput, setIncludeDisabledInput] = useState(includeDisabled)
  const [treeSearchInput, setTreeSearchInput] = useState('')
  const [extFilterFieldInput, setExtFilterFieldInput] = useState(extFilterFieldKey ?? '')
  const [extFilterValueInput, setExtFilterValueInput] = useState(extFilterValue ?? '')
  const [sortFieldInput, setSortFieldInput] = useState<string>(
    extSortFieldKey ? `ext:${extSortFieldKey}` : query.sortField ?? ''
  )
  const [sortOrderInput, setSortOrderInput] = useState<OrgUnitListSortOrder>(
    extSortFieldKey ? extSortOrder : query.sortOrder ?? 'asc'
  )

  const [childrenByParent, setChildrenByParent] = useState<Record<string, OrgUnitAPIItem[]>>({})
  const [childrenLoading, setChildrenLoading] = useState(false)
  const [childrenErrorMessage, setChildrenErrorMessage] = useState('')
  const [treeSearchErrorMessage, setTreeSearchErrorMessage] = useState('')

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateOrgUnitForm>(() => emptyCreateForm(asOf, null))
  const [createErrorMessage, setCreateErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)

  const canWrite = hasPermission('orgunit.admin')
  const canUseExt = canWrite
  const createCapabilityOrgCode = createForm.orgCode.trim()
  const createCapabilityEffectiveDate = createForm.effectiveDate.trim() || asOf

  const formatApiErrorMessage = useCallback(
    (error: unknown): string => {
      if (error instanceof ApiClientError) {
        const details = error.details as { code?: string } | undefined
        const code = details?.code ?? ''
        switch (code) {
        case 'ORG_EXT_QUERY_FIELD_NOT_ALLOWED':
          return t('org_ext_query_not_allowed')
        case 'FIELD_NOT_MAINTAINABLE':
          return t('org_field_policy_error_FIELD_NOT_MAINTAINABLE')
        case 'DEFAULT_RULE_REQUIRED':
          return t('org_field_policy_error_DEFAULT_RULE_REQUIRED')
        case 'DEFAULT_RULE_EVAL_FAILED':
          return t('org_field_policy_error_DEFAULT_RULE_EVAL_FAILED')
        case 'FIELD_POLICY_EXPR_INVALID':
          return t('org_field_policy_error_FIELD_POLICY_EXPR_INVALID')
        case 'ORG_CODE_EXHAUSTED':
          return t('org_field_policy_error_ORG_CODE_EXHAUSTED')
        case 'ORG_CODE_CONFLICT':
          return t('org_field_policy_error_ORG_CODE_CONFLICT')
        case 'FIELD_POLICY_SCOPE_OVERLAP':
          return t('org_field_policy_error_FIELD_POLICY_SCOPE_OVERLAP')
        default:
          break
        }
      }
      return getErrorMessage(error)
    },
    [t]
  )

  useEffect(() => {
    setKeywordInput(query.keyword)
    setStatusInput(query.status)
  }, [query.keyword, query.status])

  useEffect(() => {
    setAsOfInput(asOf)
    setIncludeDisabledInput(includeDisabled)
  }, [asOf, includeDisabled])

  useEffect(() => {
    setExtFilterFieldInput(extFilterFieldKey ?? '')
    setExtFilterValueInput(extFilterValue ?? '')
  }, [extFilterFieldKey, extFilterValue])

  useEffect(() => {
    const nextSortField = extSortFieldKey ? `ext:${extSortFieldKey}` : query.sortField ?? ''
    const nextSortOrder = extSortFieldKey ? extSortOrder : query.sortOrder ?? 'asc'
    setSortFieldInput(nextSortField)
    setSortOrderInput(nextSortOrder)
  }, [extSortFieldKey, extSortOrder, query.sortField, query.sortOrder])

  useEffect(() => {
    setChildrenByParent({})
    setChildrenErrorMessage('')
  }, [asOf, includeDisabled])

  const rootOrgUnitsQuery = useQuery({
    queryKey: ['org-units', 'roots', asOf, includeDisabled],
    queryFn: () => listOrgUnits({ asOf, includeDisabled }),
    staleTime: 60_000
  })

  const rootOrgUnits = useMemo(() => rootOrgUnitsQuery.data?.org_units ?? [], [rootOrgUnitsQuery.data])
  const fieldConfigsQuery = useQuery({
    enabled: canUseExt,
    queryKey: ['org-units', 'field-configs', asOf],
    queryFn: () => listOrgUnitFieldConfigs({ asOf, status: 'enabled' }),
    staleTime: 30_000
  })

  const enabledExtFields = useMemo<OrgUnitExtQueryField[]>(() => {
    const cfgs = fieldConfigsQuery.data?.field_configs ?? []
    return cfgs
      .filter((cfg) => {
        const fieldClass = (cfg.field_class ?? 'EXT').trim().toUpperCase()
        return fieldClass === 'EXT'
      })
      .map((cfg) => {
      const key = cfg.label_i18n_key?.trim() ?? ''
      const literal = cfg.label?.trim() ?? ''
      const label = key && isMessageKey(key) ? t(key) : literal.length > 0 ? literal : cfg.field_key
      return {
        field_key: cfg.field_key,
        value_type: cfg.value_type,
        data_source_type: cfg.data_source_type,
        label,
        allowFilter: Boolean(cfg.allow_filter),
        allowSort: Boolean(cfg.allow_sort)
      }
      })
  }, [fieldConfigsQuery.data, t])
  const orgCodeFieldPolicy = useMemo(() => {
    const cfgs = fieldConfigsQuery.data?.field_configs ?? []
    return cfgs.find((cfg) => cfg.field_key === 'org_code') ?? null
  }, [fieldConfigsQuery.data])
  const createOrgCodeMaintainable = orgCodeFieldPolicy?.maintainable !== false
  const createOrgCodeAutoGenerated = !createOrgCodeMaintainable
  const createOrgCodeDefaultMode = (orgCodeFieldPolicy?.default_mode ?? 'NONE').trim().toUpperCase()
  const createOrgCodeDefaultRuleExpr = (orgCodeFieldPolicy?.default_rule_expr ?? '').trim()
  const createOrgCodePolicyMisconfigured =
    createOrgCodeAutoGenerated && (createOrgCodeDefaultMode !== 'CEL' || createOrgCodeDefaultRuleExpr.length === 0)
  useEffect(() => {
    if (!createOrgCodeAutoGenerated) {
      return
    }
    setCreateForm((previous) => {
      if (previous.orgCode.trim().length === 0) {
        return previous
      }
      return { ...previous, orgCode: '' }
    })
  }, [createOrgCodeAutoGenerated])
  const extFilterFields = useMemo(() => enabledExtFields.filter((field) => field.allowFilter), [enabledExtFields])
  const extSortFields = useMemo(() => enabledExtFields.filter((field) => field.allowSort), [enabledExtFields])
  const extMetadataError = fieldConfigsQuery.error
  const extMetadataReady = canUseExt && fieldConfigsQuery.isSuccess
  const extFilterFieldKeys = useMemo(() => new Set(extFilterFields.map((field) => field.field_key)), [extFilterFields])
  const extSortFieldKeys = useMemo(() => new Set(extSortFields.map((field) => field.field_key)), [extSortFields])
  const selectedExtFilterField = useMemo(() => extFilterFields.find((field) => field.field_key === extFilterFieldInput) ?? null, [extFilterFieldInput, extFilterFields])
  const sortFieldOptions = useMemo<FieldOption[]>(() => {
    const options: FieldOption[] = [
      { value: '', label: t('org_ext_sort_none') },
      { value: 'code', label: t('org_column_code') },
      { value: 'name', label: t('org_column_name') },
      { value: 'status', label: t('text_status') }
    ]
    const prefix = t('org_ext_field_prefix')
    extSortFields.forEach((field) => {
      options.push({ value: `ext:${field.field_key}`, label: `${prefix}${field.label}` })
    })
    return options
  }, [extSortFields, t])
  const hasExtFilterParams = Boolean(extFilterFieldKey || extFilterValue)
  const hasExtSortParams = Boolean(extSortFieldKey)
  const hasAnyExtParams = hasExtFilterParams || hasExtSortParams
  const selectedNodeCode = parseOptionalValue(searchParams.get('node')) ?? rootOrgUnits[0]?.org_code ?? null

  useEffect(() => {
    if (canUseExt || !hasAnyExtParams) {
      return
    }
    const nextParams = new URLSearchParams(searchParams)
    clearExtQueryParams(nextParams)
    if (nextParams.toString() === searchParams.toString()) {
      return
    }
    setSearchParams(nextParams, { replace: true })
    setToast({ message: t('org_ext_query_admin_only_cleared'), severity: 'warning' })
  }, [canUseExt, hasAnyExtParams, searchParams, setSearchParams, t])

  useEffect(() => {
    if (!canUseExt || !hasAnyExtParams) {
      return
    }
    if (extMetadataError) {
      const nextParams = new URLSearchParams(searchParams)
      clearExtQueryParams(nextParams)
      if (nextParams.toString() === searchParams.toString()) {
        return
      }
      setSearchParams(nextParams, { replace: true })
      setToast({ message: t('org_ext_query_metadata_failed_cleared'), severity: 'warning' })
      return
    }
    if (!extMetadataReady) {
      return
    }

    let messageKey = ''
    const nextParams = new URLSearchParams(searchParams)

    if (hasExtFilterParams) {
      if (!extFilterFieldKey || !extFilterValue) {
        nextParams.delete('ext_filter_field_key')
        nextParams.delete('ext_filter_value')
        messageKey = 'org_ext_query_pair_missing_cleared'
      } else if (!extFilterFieldKeys.has(extFilterFieldKey)) {
        nextParams.delete('ext_filter_field_key')
        nextParams.delete('ext_filter_value')
        messageKey = 'org_ext_query_field_unavailable_cleared'
      }
    }

    if (extSortFieldKey && !extSortFieldKeys.has(extSortFieldKey)) {
      const sortValue = nextParams.get('sort')?.trim() ?? ''
      if (sortValue.startsWith('ext:')) {
        nextParams.delete('sort')
        nextParams.delete('order')
        if (!messageKey) {
          messageKey = 'org_ext_query_sort_unavailable_cleared'
        }
      }
    }

    if (messageKey && nextParams.toString() !== searchParams.toString()) {
      setSearchParams(nextParams, { replace: true })
      setToast({ message: t(messageKey as MessageKey), severity: 'warning' })
    }
  }, [
    canUseExt,
    extFilterFieldKey,
    extFilterFieldKeys,
    extFilterValue,
    extMetadataError,
    extMetadataReady,
    extSortFieldKey,
    extSortFieldKeys,
    hasAnyExtParams,
    hasExtFilterParams,
    searchParams,
    setSearchParams,
    t
  ])

  const legacyDetailCode = parseOptionalValue(searchParams.get('detail'))
  useEffect(() => {
    if (!legacyDetailCode) {
      return
    }

    const nextParams = new URLSearchParams()
    nextParams.set('as_of', asOf)
    if (includeDisabled) {
      nextParams.set('include_disabled', '1')
    }

    const legacyEffectiveDate = parseOptionalValue(searchParams.get('effective_date'))
    if (legacyEffectiveDate) {
      nextParams.set('effective_date', legacyEffectiveDate)
    }

    const legacyTab = parseOptionalValue(searchParams.get('tab'))
    if (legacyTab) {
      nextParams.set('tab', legacyTab)
    }

    const nextSearch = nextParams.toString()
    navigate(
      { pathname: `/org/units/${legacyDetailCode}`, search: nextSearch.length > 0 ? `?${nextSearch}` : '' },
      { replace: true }
    )
  }, [asOf, includeDisabled, legacyDetailCode, navigate, searchParams])

  const ensureChildrenLoaded = useCallback(
    async (parentOrgCode: string) => {
      if (Object.hasOwn(childrenByParent, parentOrgCode)) {
        return
      }
      setChildrenLoading(true)
      setChildrenErrorMessage('')
      try {
        const response = await listOrgUnits({
          asOf,
          parentOrgCode,
          includeDisabled
        })
        setChildrenByParent((previous) => ({
          ...previous,
          [parentOrgCode]: response.org_units
        }))
      } catch (error) {
        setChildrenErrorMessage(getErrorMessage(error))
      } finally {
        setChildrenLoading(false)
      }
    },
    [asOf, childrenByParent, includeDisabled]
  )

  const ensurePathLoaded = useCallback(
    async (pathOrgCodes: string[] | undefined) => {
      if (!pathOrgCodes || pathOrgCodes.length <= 1) {
        return
      }
      for (const parentOrgCode of pathOrgCodes.slice(0, -1)) {
        await ensureChildrenLoaded(parentOrgCode)
      }
    },
    [ensureChildrenLoaded]
  )

  const treeNodes = useMemo(() => buildTreeNodes(rootOrgUnits, childrenByParent), [childrenByParent, rootOrgUnits])
  const sortModel = useMemo(() => toGridSortModel(query.sortField, query.sortOrder), [query.sortField, query.sortOrder])
  const extFilterForRequest = useMemo(() => {
    if (!canUseExt || !extMetadataReady) {
      return null
    }
    if (!extFilterFieldKey || !extFilterValue) {
      return null
    }
    if (!extFilterFieldKeys.has(extFilterFieldKey)) {
      return null
    }
    return { fieldKey: extFilterFieldKey, value: extFilterValue }
  }, [canUseExt, extMetadataReady, extFilterFieldKey, extFilterFieldKeys, extFilterValue])
  const extSortForRequest = useMemo(() => {
    if (!canUseExt || !extMetadataReady) {
      return null
    }
    if (!extSortFieldKey || !extSortFieldKeys.has(extSortFieldKey)) {
      return null
    }
    return { fieldKey: extSortFieldKey, order: extSortOrder }
  }, [canUseExt, extMetadataReady, extSortFieldKey, extSortFieldKeys, extSortOrder])
  const effectiveSortField = (extSortForRequest ? `ext:${extSortForRequest.fieldKey}` : query.sortField ?? null) as
    | OrgUnitListSortField
    | null
  const effectiveSortOrder = (extSortForRequest ? extSortForRequest.order : query.sortOrder ?? null) as
    | OrgUnitListSortOrder
    | null

  const orgUnitListQuery = useQuery({
    enabled: rootOrgUnitsQuery.isSuccess,
    queryKey: [
      'org-units',
      'list',
      asOf,
      includeDisabled,
      selectedNodeCode,
      query.keyword,
      query.status,
      query.page,
      query.pageSize,
      effectiveSortField,
      effectiveSortOrder,
      extFilterForRequest?.fieldKey ?? '',
      extFilterForRequest?.value ?? ''
    ],
    queryFn: () =>
      listOrgUnitsPage({
        asOf,
        includeDisabled,
        parentOrgCode: selectedNodeCode ?? undefined,
        keyword: query.keyword,
        status: query.status as OrgUnitListStatusFilter,
        page: query.page,
        pageSize: query.pageSize,
        sortField: effectiveSortField,
        sortOrder: effectiveSortOrder,
        extFilterFieldKey: extFilterForRequest?.fieldKey,
        extFilterValue: extFilterForRequest?.value
      })
  })

  const gridRows = useMemo(() => (orgUnitListQuery.data?.org_units ?? []).map((item) => toOrgUnitRow(item)), [
    orgUnitListQuery.data
  ])
  const gridRowCount = orgUnitListQuery.data?.total ?? gridRows.length

  const updateSearch = useCallback(
    (
      patch: Parameters<typeof patchGridQueryState>[1],
      options?: {
        asOf?: string | null
        includeDisabled?: boolean
        selectedNodeCode?: string | null
        extFilter?: {
          fieldKey: string | null
          value: string | null
        }
        extSort?: {
          fieldKey: string | null
          order: OrgUnitListSortOrder | null
        }
      }
    ) => {
      const nextParams = patchGridQueryState(searchParams, patch)

      if (options && Object.hasOwn(options, 'asOf')) {
        if (options.asOf && options.asOf.length > 0) {
          nextParams.set('as_of', options.asOf)
        } else {
          nextParams.delete('as_of')
        }
      }

      if (options && Object.hasOwn(options, 'includeDisabled')) {
        if (options.includeDisabled) {
          nextParams.set('include_disabled', '1')
        } else {
          nextParams.delete('include_disabled')
        }
      }

      if (options && Object.hasOwn(options, 'selectedNodeCode')) {
        if (options.selectedNodeCode) {
          nextParams.set('node', options.selectedNodeCode)
        } else {
          nextParams.delete('node')
        }
      }

      if (options?.extFilter) {
        const fieldKey = options.extFilter.fieldKey?.trim() ?? ''
        const value = options.extFilter.value?.trim() ?? ''
        if (fieldKey.length > 0 && value.length > 0) {
          nextParams.set('ext_filter_field_key', fieldKey)
          nextParams.set('ext_filter_value', value)
        } else {
          nextParams.delete('ext_filter_field_key')
          nextParams.delete('ext_filter_value')
        }
      }

      if (options?.extSort) {
        const fieldKey = options.extSort.fieldKey?.trim() ?? ''
        const order = options.extSort.order ?? null
        if (fieldKey.length > 0 && order) {
          nextParams.set('sort', `ext:${fieldKey}`)
          nextParams.set('order', order)
        } else {
          const sortValue = nextParams.get('sort')?.trim() ?? ''
          if (sortValue.startsWith('ext:')) {
            nextParams.delete('sort')
            nextParams.delete('order')
          }
        }
      }

      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  const columns = useMemo<GridColDef<OrgUnitRow>[]>(
    () => [
      { field: 'code', headerName: t('org_column_code'), minWidth: 140, flex: 1 },
      { field: 'name', headerName: t('org_column_name'), minWidth: 200, flex: 1.3 },
      {
        field: 'isBusinessUnit',
        headerName: t('org_column_is_business_unit'),
        minWidth: 140,
        flex: 0.9,
        sortable: false,
        renderCell: (params) => (params.row.isBusinessUnit ? t('common_yes') : t('common_no'))
      },
      {
        field: 'status',
        headerName: t('text_status'),
        minWidth: 120,
        flex: 0.8,
        renderCell: (params) => (
          <StatusChip
            color={params.row.status === 'active' ? 'success' : 'warning'}
            label={params.row.status === 'active' ? t('org_status_active_short') : t('org_status_inactive_short')}
          />
        )
      }
    ],
    [t]
  )

  const refreshAfterWrite = useCallback(async () => {
    setChildrenByParent({})
    await queryClient.invalidateQueries({ queryKey: ['org-units'] })
  }, [queryClient])

  const createCapabilityOrgCodeReady = !createOrgCodeMaintainable || createCapabilityOrgCode.length > 0

  const createCapabilitiesQuery = useQuery({
    enabled:
      canWrite &&
      createOpen &&
      createCapabilityOrgCodeReady &&
      createCapabilityEffectiveDate.length > 0 &&
      !createOrgCodePolicyMisconfigured,
    queryKey: ['org-units', 'write-capabilities', 'create_org', createCapabilityOrgCode, createCapabilityEffectiveDate],
    queryFn: () =>
      getOrgUnitWriteCapabilities({
        intent: 'create_org',
        orgCode: createCapabilityOrgCode,
        effectiveDate: createCapabilityEffectiveDate
      }),
    staleTime: 30_000
  })

  const createCapability = createCapabilitiesQuery.data
  const createTreeNotInitialized = createCapability?.tree_initialized === false
  const createAllowedFieldSet = useMemo(() => new Set(createCapability?.allowed_fields ?? []), [createCapability?.allowed_fields])
  const createDenyReasons = useMemo(() => createCapability?.deny_reasons ?? [], [createCapability?.deny_reasons])
  const createExtFields = useMemo(
    () => enabledExtFields.filter((field) => createAllowedFieldSet.has(field.field_key)),
    [createAllowedFieldSet, enabledExtFields]
  )
  const createPlainFieldDefinitions = useMemo(
    () => createExtFields.filter((field) => field.data_source_type === 'PLAIN'),
    [createExtFields]
  )
  const createDictFieldDefinitions = useMemo(
    () => createExtFields.filter((field) => field.data_source_type === 'DICT'),
    [createExtFields]
  )
  const hasCreateUnsupportedExtFieldDefinitions = useMemo(
    () => createExtFields.some((field) => field.data_source_type !== 'PLAIN' && field.data_source_type !== 'DICT'),
    [createExtFields]
  )
  const isCreateActionDisabled = useMemo(() => {
    if (!canWrite) {
      return true
    }
    if (createOrgCodePolicyMisconfigured) {
      return true
    }
    if (!createCapabilityOrgCodeReady || createCapabilityEffectiveDate.length === 0) {
      return true
    }
    if (createCapabilitiesQuery.isLoading || createCapabilitiesQuery.isError) {
      return true
    }
    if (!createCapability?.enabled) {
      return true
    }
    return false
  }, [
    canWrite,
    createCapability?.enabled,
    createCapabilityEffectiveDate.length,
    createCapabilityOrgCodeReady,
    createCapabilitiesQuery.isError,
    createCapabilitiesQuery.isLoading,
    createOrgCodePolicyMisconfigured
  ])

  const isCreateFieldEditable = useCallback(
    (fieldKey: string): boolean => {
      if (!canWrite) {
        return false
      }
      if (fieldKey === 'org_code' || fieldKey === 'effective_date') {
        if (fieldKey === 'org_code') {
          return createOrgCodeMaintainable
        }
        return true
      }
      if (!createCapabilityOrgCodeReady || createCapabilityEffectiveDate.length === 0) {
        return false
      }
      if (createCapabilitiesQuery.isLoading || createCapabilitiesQuery.isError) {
        return false
      }
      if (!createCapability?.enabled) {
        return false
      }
      return createAllowedFieldSet.has(fieldKey)
    },
    [
      canWrite,
      createAllowedFieldSet,
      createCapability?.enabled,
      createCapabilityEffectiveDate.length,
      createCapabilityOrgCodeReady,
      createCapabilitiesQuery.isError,
      createCapabilitiesQuery.isLoading,
      createOrgCodeMaintainable
    ]
  )

  const createPlainExtErrors = useMemo(() => {
    if (!createOpen) {
      return {}
    }
    const errors: Record<string, string> = {}
    for (const def of createPlainFieldDefinitions) {
      if (def.value_type === 'bool') {
        continue
      }
      const fieldKey = def.field_key
      if (!isCreateFieldEditable(fieldKey)) {
        continue
      }
      const draft = createForm.extDisplayValues[fieldKey] ?? ''
      const result = normalizePlainExtDraft({ valueType: def.value_type, draft, mode: 'omit_empty' })
      if (result.errorCode) {
        errors[fieldKey] = result.errorCode
      }
    }
    return errors
  }, [createForm.extDisplayValues, createOpen, createPlainFieldDefinitions, isCreateFieldEditable])

  useEffect(() => {
    if (!createOpen || !createTreeNotInitialized) {
      return
    }
    setCreateForm((previous) => {
      const nextParentOrgCode = previous.parentOrgCode.trim().length > 0 ? '' : previous.parentOrgCode
      const nextIsBusinessUnit = true
      if (nextParentOrgCode === previous.parentOrgCode && nextIsBusinessUnit === previous.isBusinessUnit) {
        return previous
      }
      return {
        ...previous,
        parentOrgCode: nextParentOrgCode,
        isBusinessUnit: nextIsBusinessUnit
      }
    })
  }, [createOpen, createTreeNotInitialized])
  const hasCreatePlainExtErrors = useMemo(
    () => Object.keys(createPlainExtErrors).length > 0,
    [createPlainExtErrors]
  )

  const createMutation = useMutation({
    mutationFn: async () => {
      const capability = createCapability
      if (!capability || !capability.enabled || createCapabilitiesQuery.isError) {
        throw new Error('write capabilities unavailable')
      }
      // Fail-closed: capabilities payload mapping must be a bijection.
      const allowedFields = capability.allowed_fields ?? []
      const payloadKeys = capability.field_payload_keys ?? {}
      for (const fieldKey of allowedFields) {
        if (!(fieldKey in payloadKeys)) {
          throw new Error('write capability payload invalid')
        }
      }
      for (const fieldKey of Object.keys(payloadKeys)) {
        if (!allowedFields.includes(fieldKey)) {
          throw new Error('write capability payload invalid')
        }
      }

      const normalizedPlainExtValues: Record<string, unknown> = {}
      for (const def of createPlainFieldDefinitions) {
        if (def.value_type === 'bool') {
          continue
        }
        const fieldKey = def.field_key
        if (!isCreateFieldEditable(fieldKey)) {
          continue
        }
        const draft = createForm.extDisplayValues[fieldKey] ?? ''
        const result = normalizePlainExtDraft({ valueType: def.value_type, draft, mode: 'omit_empty' })
        if (result.errorCode) {
          throw new Error(t(result.errorCode as MessageKey))
        }
        if (typeof result.normalized !== 'undefined') {
          normalizedPlainExtValues[fieldKey] = result.normalized
        }
      }

      const extPatch: Record<string, unknown> = {}
      for (const fieldKey of allowedFields) {
        if (!(fieldKey in createForm.extValues) && !(fieldKey in normalizedPlainExtValues)) {
          continue
        }
        const normalized = fieldKey in normalizedPlainExtValues ? normalizedPlainExtValues[fieldKey] : createForm.extValues[fieldKey]
        if (typeof normalized === 'undefined') {
          continue
        }
        extPatch[fieldKey] = normalized
      }

      const patch: Record<string, unknown> = {}
      if (allowedFields.includes('name')) {
        patch.name = createForm.name.trim()
      }
      if (allowedFields.includes('parent_org_code')) {
        patch.parent_org_code = trimToUndefined(createForm.parentOrgCode)
      }
      if (allowedFields.includes('status')) {
        patch.status = createForm.status === 'active' ? 'active' : 'disabled'
      }
      if (allowedFields.includes('is_business_unit')) {
        patch.is_business_unit = createForm.isBusinessUnit
      }
      if (allowedFields.includes('manager_pernr')) {
        patch.manager_pernr = trimToUndefined(createForm.managerPernr)
      }
      if (Object.keys(extPatch).length > 0) {
        patch.ext = extPatch
      }

      await writeOrgUnit({
        intent: 'create_org',
        org_code: createCapabilityOrgCode,
        effective_date: createCapabilityEffectiveDate,
        request_id: createForm.requestID.trim() || `org-create:${Date.now()}`,
        patch
      })
    },
    onSuccess: async () => {
      await refreshAfterWrite()
      setCreateOpen(false)
      setToast({ message: t('common_action_done'), severity: 'success' })
    },
    onError: (error) => {
      setCreateErrorMessage(formatApiErrorMessage(error))
    }
  })

  function handleApplyFilters() {
    const startedAt = performance.now()
    const nextSortField = sortFieldInput.trim()
    const isExtSort = nextSortField.startsWith('ext:')
    const extSortFieldKey = isExtSort ? nextSortField.slice(4).trim() : null
    const coreSortField = !isExtSort && nextSortField.length > 0 ? nextSortField : null
    updateSearch(
      {
        keyword: keywordInput,
        page: 0,
        status: statusInput,
        sortField: coreSortField,
        sortOrder: coreSortField ? sortOrderInput : null
      },
      {
        asOf: asOfInput,
        includeDisabled: includeDisabledInput,
        extFilter: {
          fieldKey: extFilterFieldInput,
          value: extFilterValueInput
        },
        extSort: {
          fieldKey: extSortFieldKey,
          order: isExtSort ? sortOrderInput : null
        }
      }
    )
    trackUiEvent({
      eventName: 'filter_submit',
      tenant: tenantId,
      module: 'orgunit',
      page: 'org-units',
      action: 'apply_filters',
      result: 'success',
      latencyMs: Math.round(performance.now() - startedAt),
      metadata: {
        has_keyword: keywordInput.trim().length > 0,
        status: statusInput,
        as_of: asOfInput,
        include_disabled: includeDisabledInput,
        ext_filter_field: extFilterFieldInput.trim(),
        ext_filter_value: extFilterValueInput.trim(),
        ext_sort_field: extSortFieldKey ?? '',
        ext_sort_order: isExtSort ? sortOrderInput : ''
      }
    })
  }

  function handleTreeSelect(nextNodeCode: string) {
    updateSearch(
      { page: 0 },
      {
        selectedNodeCode: nextNodeCode
      }
    )
  }

  function handleSortChange(nextSortModel: GridSortModel) {
    const nextSort = fromGridSortModel(nextSortModel, sortableFields)
    updateSearch({
      page: 0,
      sortField: nextSort.sortField,
      sortOrder: nextSort.sortOrder
    })
  }

  async function handleTreeSearch() {
    const queryValue = treeSearchInput.trim()
    if (queryValue.length === 0) {
      setTreeSearchErrorMessage(t('org_search_query_required'))
      return
    }

    setTreeSearchErrorMessage('')
    try {
      const result = await searchOrgUnit({
        query: queryValue,
        asOf,
        includeDisabled
      })
      await ensurePathLoaded(result.path_org_codes)
      updateSearch(
        { page: 0 },
        {
          selectedNodeCode: result.target_org_code
        }
      )
      trackUiEvent({
        eventName: 'filter_submit',
        tenant: tenantId,
        module: 'orgunit',
        page: 'org-units',
        action: 'tree_search',
        result: 'success',
        metadata: { query: queryValue, target: result.target_org_code }
      })
    } catch (error) {
      setTreeSearchErrorMessage(getErrorMessage(error))
    }
  }

  function openCreateDialog() {
    setCreateErrorMessage('')
    setCreateForm(() => emptyCreateForm(asOf, selectedNodeCode))
    setCreateOpen(true)
  }

  const requestErrorMessage = rootOrgUnitsQuery.error
    ? formatApiErrorMessage(rootOrgUnitsQuery.error)
    : orgUnitListQuery.error
    ? formatApiErrorMessage(orgUnitListQuery.error)
    : childrenErrorMessage
  const tableLoading = rootOrgUnitsQuery.isLoading || orgUnitListQuery.isFetching

  return (
    <>
      <PageHeader
        subtitle={t('page_org_subtitle')}
        title={t('page_org_title')}
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
            <Button disabled={!canWrite} onClick={openCreateDialog} size='small' variant='contained'>
              {t('org_action_create')}
            </Button>
          </>
        }
      />

      <FilterBar>
        <TextField
          fullWidth
          label={t('org_filter_keyword')}
          onChange={(event) => setKeywordInput(event.target.value)}
          value={keywordInput}
        />
        <FormControl sx={{ minWidth: 180 }}>
          <InputLabel id='org-status-filter'>{t('org_filter_status')}</InputLabel>
          <Select
            id='org-status-filter-select'
            label={t('org_filter_status')}
            labelId='org-status-filter'
            onChange={(event) => setStatusInput(String(event.target.value) as 'all' | OrgStatus)}
            value={statusInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='active'>{t('status_active')}</MenuItem>
            <MenuItem value='inactive'>{t('status_inactive')}</MenuItem>
          </Select>
        </FormControl>
        <TextField
          InputLabelProps={{ shrink: true }}
          label={t('org_filter_as_of')}
          onChange={(event) => setAsOfInput(event.target.value)}
          type='date'
          value={asOfInput}
        />
        <FormControlLabel
          control={
            <Switch
              checked={includeDisabledInput}
              onChange={(event) => setIncludeDisabledInput(event.target.checked)}
            />
          }
          label={t('org_filter_include_disabled')}
        />
        <Button onClick={handleApplyFilters} variant='contained'>
          {t('action_apply_filters')}
        </Button>
      </FilterBar>

      {canUseExt ? (
        <FilterBar>
          <FormControl sx={{ minWidth: 220 }} disabled={!extMetadataReady}>
            <InputLabel id='org-ext-filter-field'>{t('org_ext_filter_field')}</InputLabel>
            <Select
              id='org-ext-filter-field-select'
              label={t('org_ext_filter_field')}
              labelId='org-ext-filter-field'
              onChange={(event) => {
                const nextField = String(event.target.value)
                setExtFilterFieldInput(nextField)
                if (nextField !== extFilterFieldInput) {
                  setExtFilterValueInput('')
                }
              }}
              value={extFilterFieldInput}
            >
              <MenuItem value=''>{t('org_ext_filter_field_none')}</MenuItem>
              {extFilterFields.map((field) => (
                <MenuItem key={field.field_key} value={field.field_key}>
                  {field.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <ExtFilterValueInput
            asOf={asOfInput}
            disabled={!extMetadataReady || !selectedExtFilterField}
            field={selectedExtFilterField}
            formatError={formatApiErrorMessage}
            label={t('org_ext_filter_value')}
            onChange={(nextValue) => setExtFilterValueInput(nextValue)}
            value={extFilterValueInput}
          />
          <FormControl sx={{ minWidth: 200 }}>
            <InputLabel id='org-ext-sort-field'>{t('org_ext_sort_field')}</InputLabel>
            <Select
              id='org-ext-sort-field-select'
              label={t('org_ext_sort_field')}
              labelId='org-ext-sort-field'
              onChange={(event) => setSortFieldInput(String(event.target.value))}
              value={sortFieldInput}
            >
              {sortFieldOptions.map((option) => (
                <MenuItem key={option.value || 'none'} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControl sx={{ minWidth: 140 }} disabled={sortFieldInput.trim().length === 0}>
            <InputLabel id='org-ext-sort-order'>{t('org_ext_sort_order')}</InputLabel>
            <Select
              id='org-ext-sort-order-select'
              label={t('org_ext_sort_order')}
              labelId='org-ext-sort-order'
              onChange={(event) => setSortOrderInput(String(event.target.value) as OrgUnitListSortOrder)}
              value={sortOrderInput}
            >
              <MenuItem value='asc'>{t('org_ext_sort_order_asc')}</MenuItem>
              <MenuItem value='desc'>{t('org_ext_sort_order_desc')}</MenuItem>
            </Select>
          </FormControl>
          <Button onClick={handleApplyFilters} variant='contained'>
            {t('action_apply_filters')}
          </Button>
        </FilterBar>
      ) : null}

      <FilterBar>
        <TextField
          fullWidth
          label={t('org_search_label')}
          onChange={(event) => setTreeSearchInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              void handleTreeSearch()
            }
          }}
          value={treeSearchInput}
        />
        <Button onClick={() => void handleTreeSearch()} variant='outlined'>
          {t('org_search_action')}
        </Button>
      </FilterBar>

      {requestErrorMessage.length > 0 ? (
        <Alert severity='error' sx={{ mb: 2 }}>
          {requestErrorMessage}
        </Alert>
      ) : null}
      {treeSearchErrorMessage.length > 0 ? (
        <Alert severity='warning' sx={{ mb: 2 }}>
          {treeSearchErrorMessage}
        </Alert>
      ) : null}

      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <TreePanel
          emptyLabel={t('text_no_data')}
          loading={rootOrgUnitsQuery.isLoading || childrenLoading}
          loadingLabel={t('text_loading')}
          minWidth={300}
          nodes={treeNodes}
          onExpand={(nodeId) => {
            void ensureChildrenLoaded(nodeId)
          }}
          onSelect={handleTreeSelect}
          selectedItemId={selectedNodeCode ?? undefined}
          title={t('org_tree_title')}
        />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <DataGridPage
            columns={columns}
            gridProps={{
              onPaginationModelChange: (model: GridPaginationModel) => {
                updateSearch({ page: model.page, pageSize: model.pageSize })
              },
              onRowClick: (params) => {
                const orgCode = String(params.id)
                const nextParams = new URLSearchParams()
                nextParams.set('as_of', asOf)
                if (includeDisabled) {
                  nextParams.set('include_disabled', '1')
                }

                const nextSearch = nextParams.toString()
                navigate({ pathname: `/org/units/${orgCode}`, search: nextSearch.length > 0 ? `?${nextSearch}` : '' })
                trackUiEvent({
                  eventName: 'detail_open',
                  tenant: tenantId,
                  module: 'orgunit',
                  page: 'org-units',
                  action: 'row_detail_open',
                  result: 'success',
                  metadata: { row_id: orgCode }
                })
              },
              onSortModelChange: handleSortChange,
              pageSizeOptions: [10, 20, 50],
              pagination: true,
              paginationMode: 'server',
              paginationModel: { page: query.page, pageSize: query.pageSize },
              rowCount: gridRowCount,
              showToolbar: true,
              sortModel,
              sortingMode: 'server',
              sx: { minHeight: 560 }
            }}
            loading={tableLoading}
            loadingLabel={t('text_loading')}
            noRowsLabel={t('text_no_data')}
            rows={gridRows}
            storageKey={`org-units-grid/${tenantId}`}
          />
        </Box>
      </Stack>

      <Dialog
        onClose={() => setCreateOpen(false)}
        open={createOpen}
        fullWidth
        maxWidth='sm'
      >
        <DialogTitle>{t('org_action_create')}</DialogTitle>
        <DialogContent>
          {createErrorMessage.length > 0 ? (
            <Alert severity='error' sx={{ mb: 2 }}>
              {createErrorMessage}
            </Alert>
          ) : null}
          <Stack spacing={2} sx={{ mt: 0.5 }}>
            <TextField
              disabled={!isCreateFieldEditable('org_code')}
              helperText={
                createOrgCodeAutoGenerated
                  ? createOrgCodePolicyMisconfigured
                    ? t('org_field_policy_error_DEFAULT_RULE_REQUIRED')
                    : createOrgCodeDefaultRuleExpr.length > 0
                    ? `${t('org_append_create_org_code_auto_hint')} (${createOrgCodeDefaultRuleExpr})`
                    : t('org_append_create_org_code_auto_hint')
                  : undefined
              }
              label={t('org_column_code')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, orgCode: event.target.value }))}
              value={createForm.orgCode}
            />
            <TextField
              disabled={!isCreateFieldEditable('name')}
              helperText={
                !isCreateFieldEditable('name') && createCapabilityOrgCodeReady
                  ? t('org_append_field_not_allowed_helper')
                  : undefined
              }
              label={t('org_column_name')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, name: event.target.value }))}
              value={createForm.name}
            />
            <TextField
              disabled={!isCreateFieldEditable('parent_org_code') || createTreeNotInitialized}
              helperText={
                createTreeNotInitialized
                  ? t('org_tree_bootstrap_parent_locked')
                  : !isCreateFieldEditable('parent_org_code') && createCapabilityOrgCodeReady
                  ? t('org_append_field_not_allowed_helper')
                  : undefined
              }
              label={t('org_column_parent')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, parentOrgCode: event.target.value }))}
              value={createForm.parentOrgCode}
            />
            <TextField
              select
              disabled={!isCreateFieldEditable('status')}
              helperText={
                !isCreateFieldEditable('status') && createCapabilityOrgCodeReady
                  ? t('org_append_field_not_allowed_helper')
                  : undefined
              }
              label={t('org_column_status')}
              onChange={(event) =>
                setCreateForm((previous) => ({
                  ...previous,
                  status: event.target.value === 'inactive' ? 'inactive' : 'active'
                }))
              }
              value={createForm.status}
            >
              <MenuItem value='active'>{t('org_status_active_short')}</MenuItem>
              <MenuItem value='inactive'>{t('org_status_inactive_short')}</MenuItem>
            </TextField>
            <TextField
              disabled={!isCreateFieldEditable('manager_pernr')}
              helperText={
                !isCreateFieldEditable('manager_pernr') && createCapabilityOrgCodeReady
                  ? t('org_append_field_not_allowed_helper')
                  : undefined
              }
              label={t('org_column_manager')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
              value={createForm.managerPernr}
            />
            <TextField
              disabled={!isCreateFieldEditable('effective_date')}
              InputLabelProps={{ shrink: true }}
              label={t('org_column_effective_date')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, effectiveDate: event.target.value }))}
              type='date'
              value={createForm.effectiveDate}
            />
            <FormControlLabel
              control={
                <Switch
                  checked={createForm.isBusinessUnit}
                  disabled={!isCreateFieldEditable('is_business_unit') || createTreeNotInitialized}
                  onChange={(event) => setCreateForm((previous) => ({ ...previous, isBusinessUnit: event.target.checked }))}
                />
              }
              label={t('org_column_is_business_unit')}
            />

            {createPlainFieldDefinitions.length > 0 || createDictFieldDefinitions.length > 0 || hasCreateUnsupportedExtFieldDefinitions ? (
              <>
                <Divider sx={{ my: 0.5 }} />
                <Typography variant='subtitle2'>{t('org_section_ext_fields')}</Typography>
                {createPlainFieldDefinitions.map((def) => {
                  const fieldKey = def.field_key
                  const rawValue = createForm.extValues[fieldKey]
                  const valueType = def.value_type
                  const isEditable = isCreateFieldEditable(fieldKey)
                  const notAllowedHelper =
                    !isEditable && createCapabilityOrgCodeReady
                      ? t('org_append_field_not_allowed_helper')
                      : undefined
                  const draftValue = createForm.extDisplayValues[fieldKey] ?? ''
                  const validationErrorKey = createPlainExtErrors[fieldKey]
                  const validationErrorText = validationErrorKey ? t(validationErrorKey as MessageKey) : ''

                  if (valueType === 'bool') {
                    const value = rawValue === true ? 'true' : rawValue === false ? 'false' : ''
                    return (
                      <TextField
                        key={fieldKey}
                        select
                        disabled={!isEditable}
                        helperText={notAllowedHelper}
                        label={def.label}
                        onChange={(event) => {
                          const nextValue = event.target.value
                          const next =
                            nextValue === 'true'
                              ? true
                              : nextValue === 'false'
                              ? false
                              : undefined
                          setCreateForm((previous) => ({
                            ...previous,
                            extValues: {
                              ...previous.extValues,
                              [fieldKey]: next
                            }
                          }))
                        }}
                        value={value}
                      >
                        <MenuItem value=''>-</MenuItem>
                        <MenuItem value='true'>{t('common_yes')}</MenuItem>
                        <MenuItem value='false'>{t('common_no')}</MenuItem>
                      </TextField>
                    )
                  }

                  if (valueType === 'int') {
                    return (
                      <TextField
                        key={fieldKey}
                        disabled={!isEditable}
                        helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                        error={!notAllowedHelper && validationErrorText.length > 0}
                        label={def.label}
                        type='number'
                        onChange={(event) => {
                          const nextValue = event.target.value
                          setCreateForm((previous) => ({
                            ...previous,
                            extDisplayValues: {
                              ...previous.extDisplayValues,
                              [fieldKey]: nextValue
                            }
                          }))
                        }}
                        value={draftValue}
                      />
                    )
                  }

                  if (valueType === 'date') {
                    return (
                      <TextField
                        key={fieldKey}
                        disabled={!isEditable}
                        helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                        error={!notAllowedHelper && validationErrorText.length > 0}
                        InputLabelProps={{ shrink: true }}
                        label={def.label}
                        type='date'
                        onChange={(event) => {
                          const nextValue = event.target.value
                          setCreateForm((previous) => ({
                            ...previous,
                            extDisplayValues: {
                              ...previous.extDisplayValues,
                              [fieldKey]: nextValue
                            }
                          }))
                        }}
                        value={draftValue}
                      />
                    )
                  }

                  return (
                    <TextField
                      key={fieldKey}
                      disabled={!isEditable}
                      helperText={notAllowedHelper ?? (validationErrorText.length > 0 ? validationErrorText : undefined)}
                      error={!notAllowedHelper && validationErrorText.length > 0}
                      label={def.label}
                      onChange={(event) => {
                        const nextValue = event.target.value
                        setCreateForm((previous) => ({
                          ...previous,
                          extDisplayValues: {
                            ...previous.extDisplayValues,
                            [fieldKey]: nextValue
                          }
                        }))
                      }}
                      value={draftValue}
                    />
                  )
                })}
                {createDictFieldDefinitions.map((def) => {
                  const fieldKey = def.field_key
                  const isEditable = isCreateFieldEditable(fieldKey)
                  const notAllowedHelper =
                    !isEditable && createCapabilityOrgCodeReady
                      ? t('org_append_field_not_allowed_helper')
                      : undefined
                  const currentValue = createForm.extValues[fieldKey]
                  const value =
                    typeof currentValue === 'string'
                      ? currentValue
                      : currentValue === null || typeof currentValue === 'undefined'
                      ? ''
                      : String(currentValue)

                  return (
                    <ExtFilterValueInput
                      key={fieldKey}
                      asOf={createCapabilityEffectiveDate}
                      disabled={!isEditable}
                      field={def}
                      helperText={notAllowedHelper}
                      label={def.label}
                      onChange={(nextValue) => {
                        setCreateForm((previous) => ({
                          ...previous,
                          extValues: {
                            ...previous.extValues,
                            [fieldKey]: nextValue.trim().length > 0 ? nextValue.trim() : undefined
                          }
                        }))
                      }}
                      value={value}
                    />
                  )
                })}
                {hasCreateUnsupportedExtFieldDefinitions ? (
                  <Alert severity='warning'>{t('org_ext_field_unknown_type_warning')}</Alert>
                ) : null}
              </>
            ) : null}
            {!createCapabilityOrgCodeReady ? (
              <Alert severity='info'>{t('org_append_create_org_code_required')}</Alert>
            ) : null}
            {createOrgCodePolicyMisconfigured ? (
              <Alert severity='warning'>{t('org_field_policy_error_DEFAULT_RULE_REQUIRED')}</Alert>
            ) : createOrgCodeAutoGenerated ? (
              <Alert severity='info'>{t('org_append_create_org_code_auto_hint')}</Alert>
            ) : null}
            {createCapabilityOrgCodeReady && createCapabilitiesQuery.isLoading ? (
              <Alert severity='info'>{t('text_loading')}</Alert>
            ) : null}
            {createCapabilityOrgCodeReady && createCapabilitiesQuery.isError ? (
              <Alert severity='error'>
                {t('org_append_capabilities_load_failed')}：{getErrorMessage(createCapabilitiesQuery.error)}
              </Alert>
            ) : null}
            {createCapabilityOrgCodeReady && createCapability && createTreeNotInitialized ? (
              <Alert severity='info'>{t('org_tree_bootstrap_required_hint')}</Alert>
            ) : null}
            {createCapabilityOrgCodeReady && createCapability && !createCapability.enabled && createDenyReasons.length > 0 ? (
              <Alert severity='warning'>
                {t('org_append_denied')}：{createDenyReasons.join(', ')}
              </Alert>
            ) : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>{t('common_cancel')}</Button>
          <Button
            disabled={createMutation.isPending || isCreateActionDisabled || hasCreatePlainExtErrors}
            onClick={() => createMutation.mutate()}
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
