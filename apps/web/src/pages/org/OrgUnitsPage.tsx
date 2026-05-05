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
  FormControlLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Snackbar,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef, GridPaginationModel, GridSortModel } from '@mui/x-data-grid'
import {
  getOrgUnitFieldOptions,
  listOrgUnits,
  listOrgUnitsPage,
  listOrgUnitFieldConfigs,
  searchOrgUnit,
  writeOrgUnit,
  type OrgUnitAPIItem,
  type OrgUnitListSortField,
  type OrgUnitListStatusFilter,
  type OrgUnitSearchAmbiguousDetails,
  type OrgUnitSearchCandidate
} from '../../api/orgUnits'
import type { OrgUnitSelectorNode } from '../../api/orgUnitSelector'
import { ApiClientError } from '../../api/errors'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { AUTHZ_CAPABILITY_KEYS } from '../../authz/capabilities'
import { DataGridPage } from '../../components/DataGridPage'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { OrgUnitTreeField } from '../../components/OrgUnitTreeSelector'
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
import { buildOrgFieldConfigsSearchParams, buildOrgUnitDetailSearchParams } from './orgReadNavigation'
import { clearExtQueryParams } from './orgUnitListExtQuery'
import { resolveReadViewState, todayISODate } from './readViewState'

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
  parentOrgNodeKey: string
  parentOrgCode: string
  parentOrgName: string
  status: OrgStatus
  managerPernr: string
  effectiveDate: string
  isBusinessUnit: boolean
  requestID: string
  extValues: Record<string, unknown>
  extDisplayValues: Record<string, string>
}

const sortableFields = ['code', 'name', 'status'] as const

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

function parseBoolDefault(raw: string | null, fallback: boolean): boolean {
  if (!raw) {
    return fallback
  }
  const value = raw.trim().toLowerCase()
  if (value === '1' || value === 'true' || value === 'yes' || value === 'on') {
    return true
  }
  if (value === '0' || value === 'false' || value === 'no' || value === 'off') {
    return false
  }
  return fallback
}

function isValidDay(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value)
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

function toOrgUnitSelectorNode(item: OrgUnitAPIItem | undefined): OrgUnitSelectorNode | null {
  const orgCode = item?.org_code.trim() ?? ''
  const orgNodeKey = item?.org_node_key?.trim() ?? ''
  if (!item || orgCode.length === 0 || orgNodeKey.length === 0) {
    return null
  }
  return {
    org_code: orgCode,
    org_node_key: orgNodeKey,
    name: item.name.trim(),
    status: parseOrgStatus(item.status),
    has_visible_children: item.has_visible_children ?? item.has_children
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
      return { id: item.org_code, label: `${item.name}${labelSuffix}`, hasChildren: false }
    }

    const nextPath = new Set(path)
    nextPath.add(item.org_code)
    const childrenLoaded = Object.hasOwn(childrenByParent, item.org_code)
    const children = childrenByParent[item.org_code] ?? []
    const childNodes = children.map((child) => build(child, nextPath))
    const hasChildren = childrenLoaded ? childNodes.length > 0 : item.has_children === true

    return {
      id: item.org_code,
      label: `${item.name}${labelSuffix}`,
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

function formatOrgUnitSearchAmbiguousMessage(error: unknown, prefix: string): string | null {
  if (!(error instanceof ApiClientError) || error.status !== 409) {
    return null
  }
  const details = error.details as OrgUnitSearchAmbiguousDetails | undefined
  if (!details || details.error_code !== 'org_unit_search_ambiguous') {
    return null
  }
  const candidates = Array.isArray(details.candidates) ? details.candidates : []
  if (candidates.length === 0) {
    return error.message
  }
  const labels = candidates
    .map((candidate) => {
      const code = candidate.org_code?.trim()
      const name = candidate.name?.trim()
      if (code && name) {
        return `${name} (${code})`
      }
      return code || name || ''
    })
    .filter((label) => label.length > 0)
  if (labels.length === 0) {
    return error.message
  }
  return `${prefix} ${labels.join(', ')}`
}

function getOrgUnitSearchAmbiguousCandidates(error: unknown): OrgUnitSearchCandidate[] {
  if (!(error instanceof ApiClientError) || error.status !== 409) {
    return []
  }
  const details = error.details as OrgUnitSearchAmbiguousDetails | undefined
  if (!details || details.error_code !== 'org_unit_search_ambiguous') {
    return []
  }
  return Array.isArray(details.candidates) ? details.candidates : []
}

type FieldOption = { value: string; label: string }

type OrgUnitExtFieldDefinition = Pick<import('../../api/orgUnits').OrgUnitTenantFieldConfig, 'field_key' | 'value_type' | 'data_source_type'> & {
  label: string
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

function formatFieldOptionLabel(option: FieldOption): string {
  return option.label
}

function OrgExtFieldValueInput(props: {
  field: OrgUnitExtFieldDefinition | null
  asOf: string
  label: string
  value: string
  disabled: boolean
  allowedValues?: string[]
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
        throw new Error('org ext field is required')
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
    const allowedValueSet = new Set((props.allowedValues ?? []).map((item) => item.trim()).filter((item) => item.length > 0))
    const fetchedRaw = optionsQuery.data?.options ?? []
    const fetched = allowedValueSet.size > 0
      ? fetchedRaw.filter((option) => allowedValueSet.has(option.value))
      : fetchedRaw
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
  }, [isDictField, optionsQuery.data?.options, props.allowedValues, props.value])

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
      />
    )
  }

  return (
    <Autocomplete
      clearOnEscape
      disabled={effectiveDisabled}
      getOptionLabel={(option) => formatFieldOptionLabel(option)}
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

function emptyCreateForm(defaultEffectiveDate: string, parent: OrgUnitSelectorNode | null): CreateOrgUnitForm {
  return {
    orgCode: '',
    name: '',
    parentOrgNodeKey: parent?.org_node_key ?? '',
    parentOrgCode: parent?.org_code ?? '',
    parentOrgName: parent?.name ?? '',
    status: 'active',
    managerPernr: '',
    effectiveDate: defaultEffectiveDate,
    isBusinessUnit: false,
    requestID: `org-create:${Date.now()}`,
    extValues: {},
    extDisplayValues: {}
  }
}

export function OrgUnitsPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { t, tenantId, hasRequiredCapability } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const currentDate = useMemo(() => todayISODate(), [])
  const readView = useMemo(() => resolveReadViewState(searchParams.get('as_of'), currentDate), [searchParams, currentDate])

  const query = useMemo(
    () =>
      parseGridQueryState(searchParams, {
        statusValues: ['active', 'inactive'] as const,
        sortFields: sortableFields
      }),
    [searchParams]
  )

  const asOf = readView.effectiveAsOf
  const requestedAsOf = readView.requestedAsOf
  const readMode = readView.mode
  const includeDisabled = parseBool(searchParams.get('include_disabled'))
  const includeDescendants = parseBoolDefault(searchParams.get('include_descendants'), false)
  const legacyStatusParam = parseOptionalValue(searchParams.get('status'))
  const legacyStatusForRequest =
    legacyStatusParam === 'active' || legacyStatusParam === 'inactive'
      ? legacyStatusParam
      : legacyStatusParam === 'disabled'
        ? 'inactive'
        : null
  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [asOfInput, setAsOfInput] = useState(requestedAsOf ?? asOf)
  const debouncedKeywordInput = useDebouncedValue(keywordInput, 350)

  const [childrenByParent, setChildrenByParent] = useState<Record<string, OrgUnitAPIItem[]>>({})
  const [expandedOrgCodes, setExpandedOrgCodes] = useState<string[]>([])
  const [childrenLoading, setChildrenLoading] = useState(false)
  const [childrenErrorMessage, setChildrenErrorMessage] = useState('')
  const [treeSearchErrorMessage, setTreeSearchErrorMessage] = useState('')
  const [searchCandidates, setSearchCandidates] = useState<OrgUnitSearchCandidate[]>([])

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateOrgUnitForm>(() => emptyCreateForm(currentDate, null))
  const [createErrorMessage, setCreateErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)

  const canWrite = hasRequiredCapability(AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsAdmin)
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
        case 'FIELD_OPTION_NOT_ALLOWED':
          return t('org_field_policy_error_FIELD_OPTION_NOT_ALLOWED')
        case 'FIELD_REQUIRED_VALUE_MISSING':
          return t('org_field_policy_error_FIELD_REQUIRED_VALUE_MISSING')
        case 'policy_missing':
        case 'FIELD_POLICY_MISSING':
          return t('org_field_policy_error_FIELD_POLICY_MISSING')
        case 'policy_conflict_ambiguous':
        case 'FIELD_POLICY_CONFLICT':
          return t('org_field_policy_error_FIELD_POLICY_CONFLICT')
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
  }, [query.keyword])

  useEffect(() => {
    setAsOfInput(requestedAsOf ?? asOf)
  }, [asOf, requestedAsOf])

  useEffect(() => {
    setChildrenByParent({})
    setExpandedOrgCodes([])
    setChildrenErrorMessage('')
  }, [asOf, includeDisabled])

  useEffect(() => {
    if (searchParams.has('include_descendants')) {
      return
    }
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set('include_descendants', 'false')
    setSearchParams(nextParams, { replace: true })
  }, [searchParams, setSearchParams])

  const rootOrgUnitsQuery = useQuery({
    queryKey: ['org-units', 'roots', asOf, includeDisabled],
    queryFn: () => listOrgUnits({ asOf, includeDisabled }),
    staleTime: 60_000
  })

  const rootOrgUnits = useMemo(() => rootOrgUnitsQuery.data?.org_units ?? [], [rootOrgUnitsQuery.data])
  const orgUnitNodeByCode = useMemo(() => {
    const nodes = new Map<string, OrgUnitAPIItem>()
    const visited = new Set<string>()
    const visit = (item: OrgUnitAPIItem) => {
      if (visited.has(item.org_code)) {
        return
      }
      visited.add(item.org_code)
      nodes.set(item.org_code, item)
      for (const child of childrenByParent[item.org_code] ?? []) {
        visit(child)
      }
    }
    rootOrgUnits.forEach(visit)
    return nodes
  }, [childrenByParent, rootOrgUnits])

  const fieldConfigsQuery = useQuery({
    enabled: canWrite,
    queryKey: ['org-units', 'field-configs', asOf],
    queryFn: () => listOrgUnitFieldConfigs({ asOf, status: 'enabled' }),
    staleTime: 30_000
  })

  const enabledExtFields = useMemo<OrgUnitExtFieldDefinition[]>(() => {
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
        label
      }
      })
  }, [fieldConfigsQuery.data, t])
  const selectedNodeCode = parseOptionalValue(searchParams.get('node')) ?? rootOrgUnits[0]?.org_code ?? null
  const createTreeNotInitialized = rootOrgUnits.length === 0
  const createAllowedFieldSet = useMemo(
    () =>
      new Set([
        'org_code',
        'effective_date',
        'name',
        'parent_org_code',
        'status',
        'is_business_unit',
        'manager_pernr',
        ...enabledExtFields.map((field) => field.field_key)
      ]),
    [enabledExtFields]
  )
  const createExtFields = enabledExtFields
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

  useEffect(() => {
    const nextParams = new URLSearchParams(searchParams)
    clearExtQueryParams(nextParams)
    if (nextParams.toString() !== searchParams.toString()) {
      setSearchParams(nextParams, { replace: true })
    }
  }, [searchParams, setSearchParams])

  const legacyDetailCode = parseOptionalValue(searchParams.get('detail'))
  useEffect(() => {
    if (!legacyDetailCode) {
      return
    }

    const nextParams = buildOrgUnitDetailSearchParams({
      readMode,
      asOf,
      includeDisabled,
      effectiveDate: parseOptionalValue(searchParams.get('effective_date')),
      tab: parseOptionalValue(searchParams.get('tab'))
    })
    const nextSearch = nextParams.toString()
    navigate(
      { pathname: `/org/units/${legacyDetailCode}`, search: nextSearch.length > 0 ? `?${nextSearch}` : '' },
      { replace: true }
    )
  }, [asOf, includeDisabled, legacyDetailCode, navigate, readMode, searchParams])

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
      const parentOrgCodes = pathOrgCodes.slice(0, -1)
      setExpandedOrgCodes((previous) => Array.from(new Set([...previous, ...parentOrgCodes])))
      for (const parentOrgCode of parentOrgCodes) {
        await ensureChildrenLoaded(parentOrgCode)
      }
    },
    [ensureChildrenLoaded]
  )

  const treeNodes = useMemo(() => buildTreeNodes(rootOrgUnits, childrenByParent), [childrenByParent, rootOrgUnits])
  const sortModel = useMemo(() => toGridSortModel(query.sortField, query.sortOrder), [query.sortField, query.sortOrder])

  const orgUnitListQuery = useQuery({
    enabled: rootOrgUnitsQuery.isSuccess,
    queryKey: [
      'org-units',
      'list',
      asOf,
      includeDisabled,
      includeDescendants,
      selectedNodeCode,
      query.keyword,
      legacyStatusForRequest ?? 'all',
      query.page,
      query.pageSize,
      query.sortField ?? null,
      query.sortOrder ?? null
    ],
    queryFn: () =>
      listOrgUnitsPage({
        asOf,
        includeDisabled,
        includeDescendants,
        parentOrgCode: selectedNodeCode ?? undefined,
        keyword: query.keyword,
        status: (legacyStatusForRequest ?? 'all') as OrgUnitListStatusFilter,
        page: query.page,
        pageSize: query.pageSize,
        sortField: query.sortField as OrgUnitListSortField | null,
        sortOrder: query.sortOrder
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
        includeDescendants?: boolean
        selectedNodeCode?: string | null
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

      if (options && Object.hasOwn(options, 'includeDescendants')) {
        nextParams.set('include_descendants', options.includeDescendants ? 'true' : 'false')
      }

      if (options && Object.hasOwn(options, 'selectedNodeCode')) {
        if (options.selectedNodeCode) {
          nextParams.set('node', options.selectedNodeCode)
        } else {
          nextParams.delete('node')
        }
      }

      if (
        Object.hasOwn(patch, 'keyword') ||
        (options &&
          (Object.hasOwn(options, 'asOf') ||
            Object.hasOwn(options, 'includeDisabled') ||
            Object.hasOwn(options, 'includeDescendants') ||
            Object.hasOwn(options, 'selectedNodeCode')))
      ) {
        nextParams.delete('status')
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

  const isCreateActionDisabled = useMemo(() => {
    if (!canWrite) {
      return true
    }
    if (createForm.orgCode.trim().length === 0) {
      return true
    }
    if (createForm.name.trim().length === 0) {
      return true
    }
    if ((createForm.effectiveDate.trim() || currentDate).length === 0) {
      return true
    }
    return false
  }, [canWrite, createForm.effectiveDate, createForm.name, createForm.orgCode, currentDate])

  const isCreateFieldEditable = useCallback(
    (fieldKey: string): boolean => {
      if (!canWrite) {
        return false
      }
      if (!createAllowedFieldSet.has(fieldKey)) {
        return false
      }
      if (fieldKey === 'parent_org_code' || fieldKey === 'is_business_unit') {
        return !createTreeNotInitialized
      }
      return true
    },
    [canWrite, createAllowedFieldSet, createTreeNotInitialized]
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
        parentOrgNodeKey: '',
        parentOrgCode: nextParentOrgCode,
        parentOrgName: '',
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
      for (const fieldKey of createExtFields.map((field) => field.field_key)) {
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
      patch.name = createForm.name.trim()
      if (!createTreeNotInitialized) {
        patch.parent_org_code = trimToUndefined(createForm.parentOrgCode)
        patch.is_business_unit = createForm.isBusinessUnit
      } else {
        patch.is_business_unit = true
      }
      patch.status = createForm.status === 'active' ? 'active' : 'disabled'
      patch.manager_pernr = trimToUndefined(createForm.managerPernr)
      if (Object.keys(extPatch).length > 0) {
        patch.ext = extPatch
      }

      await writeOrgUnit({
        intent: 'create_org',
        org_code: createForm.orgCode.trim(),
        effective_date: createForm.effectiveDate.trim() || currentDate,
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

  useEffect(() => {
    if (debouncedKeywordInput === query.keyword) {
      return
    }
    updateSearch({ keyword: debouncedKeywordInput, page: 0 })
  }, [debouncedKeywordInput, query.keyword, updateSearch])

  function handleIncludeDescendantsChange(nextValue: boolean) {
    updateSearch({ page: 0 }, { includeDescendants: nextValue })
  }

  function handleIncludeDisabledChange(nextValue: boolean) {
    updateSearch({ page: 0 }, { includeDisabled: nextValue })
  }

  function handleAsOfChange(nextValue: string) {
    setAsOfInput(nextValue)
    if (nextValue.trim().length === 0) {
      updateSearch({ page: 0 }, { asOf: null })
      return
    }
    if (isValidDay(nextValue)) {
      updateSearch({ page: 0 }, { asOf: nextValue })
    }
  }

  function clearLegacyStatusFilter() {
    updateSearch({ page: 0, status: null })
  }

  async function handleMainSearchLocate() {
    const queryValue = keywordInput.trim()
    if (queryValue.length === 0) {
      setTreeSearchErrorMessage(t('org_search_query_required'))
      return
    }

    setTreeSearchErrorMessage('')
    setSearchCandidates([])
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
      const candidates = getOrgUnitSearchAmbiguousCandidates(error)
      if (candidates.length > 0) {
        setSearchCandidates(candidates)
      }
      setTreeSearchErrorMessage(formatOrgUnitSearchAmbiguousMessage(error, t('org_search_ambiguous_prefix')) ?? getErrorMessage(error))
    }
  }

  async function resolveSearchCandidatePath(candidate: OrgUnitSearchCandidate): Promise<string[] | undefined> {
    if (candidate.path_org_codes && candidate.path_org_codes.length > 0) {
      return candidate.path_org_codes
    }
    const result = await searchOrgUnit({
      query: candidate.org_code,
      asOf: candidate.as_of && candidate.as_of.trim().length > 0 ? candidate.as_of : asOf,
      includeDisabled
    })
    return result.path_org_codes
  }

  async function selectSearchCandidate(candidate: OrgUnitSearchCandidate) {
    const orgCode = candidate.org_code.trim()
    if (!orgCode) {
      return
    }
    setTreeSearchErrorMessage('')
    setSearchCandidates([])
    try {
      await ensurePathLoaded(await resolveSearchCandidatePath(candidate))
    } catch {
      await ensurePathLoaded([orgCode])
    }
    updateSearch({ page: 0 }, { selectedNodeCode: orgCode })
  }

  function openCreateDialog() {
    setCreateErrorMessage('')
    const selectedParent =
      selectedNodeCode && selectedNodeCode.trim().length > 0
        ? toOrgUnitSelectorNode(orgUnitNodeByCode.get(selectedNodeCode))
        : null
    setCreateForm(() => emptyCreateForm(currentDate, selectedParent))
    setCreateOpen(true)
  }

  function closeCreateDialog() {
    setCreateOpen(false)
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
                  const params = buildOrgFieldConfigsSearchParams(readMode, asOf)
                  const nextSearch = params.toString()
                  navigate({ pathname: '/org/units/field-configs', search: nextSearch.length > 0 ? `?${nextSearch}` : '' })
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
	          onKeyDown={(event) => {
	            const nativeEvent = event.nativeEvent
	            if (nativeEvent.isComposing || nativeEvent.keyCode === 229) {
	              return
	            }
	            if (event.key === 'Enter') {
	              event.preventDefault()
	              void handleMainSearchLocate()
	            }
	          }}
	          InputProps={{
	            endAdornment: (
	              <InputAdornment position='end'>
	                <IconButton aria-label={t('org_search_action')} edge='end' onClick={() => void handleMainSearchLocate()} size='small'>
	                  <SearchIcon fontSize='small' />
	                </IconButton>
	              </InputAdornment>
	            )
          }}
          sx={{ flex: '1 1 520px', minWidth: { md: 360, xs: '100%' } }}
          value={keywordInput}
        />
        <FormControlLabel
          control={<Switch checked={includeDescendants} onChange={(event) => handleIncludeDescendantsChange(event.target.checked)} />}
          label={t('org_filter_include_descendants')}
          sx={{
            flex: '0 0 156px',
            m: 0,
            whiteSpace: 'nowrap',
            '& .MuiFormControlLabel-label': { whiteSpace: 'nowrap' }
          }}
        />
        <FormControlLabel
          control={<Switch checked={includeDisabled} onChange={(event) => handleIncludeDisabledChange(event.target.checked)} />}
          label={t('org_filter_include_disabled')}
          sx={{
            flex: '0 0 124px',
            m: 0,
            whiteSpace: 'nowrap',
            '& .MuiFormControlLabel-label': { whiteSpace: 'nowrap' }
          }}
        />
        <TextField
          InputLabelProps={{ shrink: true }}
          label={t('org_filter_as_of')}
          onChange={(event) => handleAsOfChange(event.target.value)}
          sx={{ flex: '0 0 176px', minWidth: 176 }}
          type='date'
          value={asOfInput}
        />
	      </FilterBar>

	      {legacyStatusForRequest ? (
	        <Alert
	          action={
	            <Button color='inherit' onClick={clearLegacyStatusFilter} size='small'>
	              {t('common_clear')}
	            </Button>
	          }
	          severity='info'
	          sx={{ mb: 2 }}
	        >
	          {t('org_legacy_status_filter_applied')}
	        </Alert>
	      ) : null}

      {requestErrorMessage.length > 0 ? (
        <Alert severity='error' sx={{ mb: 2 }}>
          {requestErrorMessage}
        </Alert>
      ) : null}
	      {treeSearchErrorMessage.length > 0 ? (
	        <Alert severity='warning' sx={{ mb: 2 }}>
	          <Stack spacing={1}>
	            <span>{treeSearchErrorMessage}</span>
	            {searchCandidates.length > 0 ? (
	              <Stack direction={{ md: 'row', xs: 'column' }} spacing={1}>
	                {searchCandidates.map((candidate) => (
	                  <Button
	                    key={candidate.org_code}
	                    onClick={() => void selectSearchCandidate(candidate)}
	                    size='small'
	                    variant='outlined'
	                  >
	                    {candidate.name} ({candidate.org_code})
	                  </Button>
	                ))}
	              </Stack>
	            ) : null}
	          </Stack>
	        </Alert>
	      ) : null}

      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <TreePanel
          emptyLabel={t('text_no_data')}
          loading={rootOrgUnitsQuery.isLoading || childrenLoading}
          loadingLabel={t('text_loading')}
          minWidth={300}
          nodes={treeNodes}
          expandedItemIds={expandedOrgCodes}
          onExpand={(nodeId) => {
            void ensureChildrenLoaded(nodeId)
          }}
          onExpandedItemIdsChange={setExpandedOrgCodes}
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
                const nextParams = buildOrgUnitDetailSearchParams({ readMode, asOf, includeDisabled })
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
              slotProps: { toolbar: { showQuickFilter: false } },
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
        onClose={closeCreateDialog}
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
              label={t('org_column_code')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, orgCode: event.target.value }))}
              value={createForm.orgCode}
            />
            <TextField
              disabled={!isCreateFieldEditable('name')}
              label={t('org_column_name')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, name: event.target.value }))}
              value={createForm.name}
            />
            <OrgUnitTreeField
              asOf={asOf}
              clearable
              disabled={!isCreateFieldEditable('parent_org_code') || createTreeNotInitialized}
              helperText={
                createTreeNotInitialized
                  ? t('org_tree_bootstrap_parent_locked')
                  : !isCreateFieldEditable('parent_org_code')
                  ? t('org_append_field_not_allowed_helper')
                  : undefined
              }
              label={t('org_column_parent')}
              onChange={(option) => {
                setCreateForm((previous) => ({
                  ...previous,
                  parentOrgNodeKey: option.org_node_key,
                  parentOrgCode: option.org_code,
                  parentOrgName: option.name
                }))
              }}
              onClear={() => {
                setCreateForm((previous) => ({
                  ...previous,
                  parentOrgNodeKey: '',
                  parentOrgCode: '',
                  parentOrgName: ''
                }))
              }}
              value={
                createForm.parentOrgCode.trim().length > 0
                  ? toOrgUnitSelectorNode({
                    org_code: createForm.parentOrgCode,
                    org_node_key: createForm.parentOrgNodeKey,
                    name: createForm.parentOrgName || createForm.parentOrgCode,
                    status: 'active',
                    has_children: false,
                    has_visible_children: false
                  })
                  : null
              }
            />
            <TextField
              select
              disabled={!isCreateFieldEditable('status')}
              helperText={!isCreateFieldEditable('status') ? t('org_append_field_not_allowed_helper') : undefined}
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
              helperText={!isCreateFieldEditable('manager_pernr') ? t('org_append_field_not_allowed_helper') : undefined}
              label={t('org_column_manager')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
              value={createForm.managerPernr}
            />
            <TextField
              disabled={!isCreateFieldEditable('effective_date')}
              InputLabelProps={{ shrink: true }}
              label={t('org_column_effective_date')}
              onChange={(event) => {
                setCreateForm((previous) => ({ ...previous, effectiveDate: event.target.value }))
              }}
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
                  const notAllowedHelper = !isEditable ? t('org_append_field_not_allowed_helper') : undefined
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
                  const notAllowedHelper = !isEditable ? t('org_append_field_not_allowed_helper') : undefined
                  const currentValue = createForm.extValues[fieldKey]
                  const value =
                    typeof currentValue === 'string'
                      ? currentValue
                      : currentValue === null || typeof currentValue === 'undefined'
                      ? ''
                      : String(currentValue)

                  return (
                    <OrgExtFieldValueInput
                      key={fieldKey}
                      asOf={createForm.effectiveDate.trim() || currentDate}
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
            {createTreeNotInitialized ? (
              <Alert severity='info'>{t('org_tree_bootstrap_required_hint')}</Alert>
            ) : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeCreateDialog}>{t('common_cancel')}</Button>
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
