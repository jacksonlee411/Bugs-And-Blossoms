import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  List,
  ListItemButton,
  ListItemText,
  MenuItem,
  Select,
  Snackbar,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef, GridPaginationModel, GridRowSelectionModel, GridSortModel } from '@mui/x-data-grid'
import {
  correctOrgUnit,
  createOrgUnit,
  disableOrgUnit,
  enableOrgUnit,
  getOrgUnitDetails,
  listOrgUnitAudit,
  listOrgUnitVersions,
  listOrgUnits,
  listOrgUnitsPage,
  moveOrgUnit,
  renameOrgUnit,
  rescindOrgUnit,
  rescindOrgUnitRecord,
  searchOrgUnit,
  setOrgUnitBusinessUnit,
  type OrgUnitAPIItem,
  type OrgUnitListSortField,
  type OrgUnitListSortOrder,
  type OrgUnitListStatusFilter
} from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { DetailPanel } from '../../components/DetailPanel'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { type TreePanelNode, TreePanel } from '../../components/TreePanel'
import { trackUiEvent } from '../../observability/tracker'
import {
  fromGridSortModel,
  parseGridQueryState,
  patchGridQueryState,
  toGridSortModel
} from '../../utils/gridQueryState'
import type { MessageKey, MessageVars } from '../../i18n/messages'

type OrgStatus = 'active' | 'inactive'
type DetailTab = 'profile' | 'records' | 'audit'
type OrgActionType =
  | 'create'
  | 'rename'
  | 'move'
  | 'set_business_unit'
  | 'enable'
  | 'disable'
  | 'correct'
  | 'rescind_record'
  | 'rescind_org'

interface OrgUnitRow {
  id: string
  code: string
  name: string
  status: OrgStatus
  isBusinessUnit: boolean
  effectiveDate: string
}

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
  requestId: string
  requestCode: string
  reason: string
}

const sortableFields = ['code', 'name', 'status'] as const
const detailTabs: readonly DetailTab[] = ['profile', 'records', 'audit']

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

function toOrgUnitRow(item: OrgUnitAPIItem, asOfDate: string): OrgUnitRow {
  return {
    id: item.org_code,
    code: item.org_code,
    name: item.name,
    status: parseOrgStatus(item.status),
    isBusinessUnit: Boolean(item.is_business_unit),
    effectiveDate: asOfDate
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
      return {
        id: item.org_code,
        label: `${item.name} (${item.org_code})${labelSuffix}`
      }
    }

    const nextPath = new Set(path)
    nextPath.add(item.org_code)
    const children = childrenByParent[item.org_code] ?? []

    return {
      id: item.org_code,
      label: `${item.name} (${item.org_code})${labelSuffix}`,
      children: children.length > 0 ? children.map((child) => build(child, nextPath)) : undefined
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

function newRequestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `org-${Date.now()}`
}

function emptyActionForm(asOf: string): OrgActionForm {
  return {
    orgCode: '',
    name: '',
    parentOrgCode: '',
    managerPernr: '',
    effectiveDate: asOf,
    correctedEffectiveDate: '',
    isBusinessUnit: false,
    requestId: newRequestID(),
    requestCode: `req-${Date.now()}`,
    reason: ''
  }
}

function actionLabel(
  type: OrgActionType,
  t: (key: MessageKey, vars?: MessageVars) => string
): string {
  switch (type) {
    case 'create':
      return t('org_action_create')
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

export function OrgUnitsPage() {
  const queryClient = useQueryClient()
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
  const detailTab = parseDetailTab(searchParams.get('tab'))
  const detailCodeParam = parseOptionalValue(searchParams.get('detail'))
  const detailEffectiveDateParam = parseOptionalValue(searchParams.get('effective_date'))

  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [statusInput, setStatusInput] = useState<'all' | OrgStatus>(query.status)
  const [asOfInput, setAsOfInput] = useState(asOf)
  const [includeDisabledInput, setIncludeDisabledInput] = useState(includeDisabled)
  const [treeSearchInput, setTreeSearchInput] = useState('')

  const [selectedRowId, setSelectedRowId] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<string[]>([])

  const [childrenByParent, setChildrenByParent] = useState<Record<string, OrgUnitAPIItem[]>>({})
  const [childrenLoading, setChildrenLoading] = useState(false)
  const [childrenErrorMessage, setChildrenErrorMessage] = useState('')
  const [treeSearchErrorMessage, setTreeSearchErrorMessage] = useState('')

  const [actionState, setActionState] = useState<OrgActionState | null>(null)
  const [actionForm, setActionForm] = useState<OrgActionForm>(() => emptyActionForm(asOf))
  const [actionErrorMessage, setActionErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)
  const [auditLimit, setAuditLimit] = useState(20)

  const canWrite = hasPermission('orgunit.admin')

  useEffect(() => {
    setKeywordInput(query.keyword)
    setStatusInput(query.status)
  }, [query.keyword, query.status])

  useEffect(() => {
    setAsOfInput(asOf)
    setIncludeDisabledInput(includeDisabled)
  }, [asOf, includeDisabled])

  useEffect(() => {
    setChildrenByParent({})
    setChildrenErrorMessage('')
    setSelectedIds([])
    setSelectedRowId(null)
  }, [asOf, includeDisabled])

  const rootOrgUnitsQuery = useQuery({
    queryKey: ['org-units', 'roots', asOf, includeDisabled],
    queryFn: () => listOrgUnits({ asOf, includeDisabled }),
    staleTime: 60_000
  })

  const rootOrgUnits = useMemo(() => rootOrgUnitsQuery.data?.org_units ?? [], [rootOrgUnitsQuery.data])
  const selectedNodeCode = parseOptionalValue(searchParams.get('node')) ?? rootOrgUnits[0]?.org_code ?? null
  const detailCode = detailCodeParam ?? selectedRowId ?? selectedNodeCode
  const detailAsOf = parseDateOrDefault(detailEffectiveDateParam, asOf)

  useEffect(() => {
    setAuditLimit(20)
  }, [detailCode])

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
      query.sortField,
      query.sortOrder
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
        sortField: (query.sortField as OrgUnitListSortField | null) ?? null,
        sortOrder: (query.sortOrder as OrgUnitListSortOrder | null) ?? null
      })
  })

  const gridRows = useMemo(
    () => (orgUnitListQuery.data?.org_units ?? []).map((item) => toOrgUnitRow(item, asOf)),
    [asOf, orgUnitListQuery.data]
  )
  const gridRowCount = orgUnitListQuery.data?.total ?? gridRows.length

  const selectedRow = useMemo(
    () => (selectedRowId ? gridRows.find((row) => row.id === selectedRowId) ?? null : null),
    [gridRows, selectedRowId]
  )

  const detailQuery = useQuery({
    enabled: detailCode !== null,
    queryKey: ['org-units', 'details', detailCode, detailAsOf, includeDisabled],
    queryFn: () => getOrgUnitDetails({ orgCode: detailCode ?? '', asOf: detailAsOf, includeDisabled })
  })

  const versionsQuery = useQuery({
    enabled: detailCode !== null,
    queryKey: ['org-units', 'versions', detailCode],
    queryFn: () => listOrgUnitVersions({ orgCode: detailCode ?? '' })
  })

  const auditQuery = useQuery({
    enabled: detailCode !== null,
    queryKey: ['org-units', 'audit', detailCode, auditLimit],
    queryFn: () => listOrgUnitAudit({ orgCode: detailCode ?? '', limit: auditLimit })
  })

  const updateSearch = useCallback(
    (
      patch: Parameters<typeof patchGridQueryState>[1],
      options?: {
        asOf?: string | null
        includeDisabled?: boolean
        selectedNodeCode?: string | null
        detailCode?: string | null
        detailEffectiveDate?: string | null
        tab?: DetailTab | null
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

      if (options && Object.hasOwn(options, 'detailCode')) {
        if (options.detailCode) {
          nextParams.set('detail', options.detailCode)
        } else {
          nextParams.delete('detail')
        }
      }

      if (options && Object.hasOwn(options, 'detailEffectiveDate')) {
        if (options.detailEffectiveDate) {
          nextParams.set('effective_date', options.detailEffectiveDate)
        } else {
          nextParams.delete('effective_date')
        }
      }

      if (options && Object.hasOwn(options, 'tab')) {
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
        field: 'effectiveDate',
        headerName: t('org_column_effective_date'),
        minWidth: 140,
        flex: 0.9,
        sortable: false
      },
      {
        field: 'status',
        headerName: t('text_status'),
        minWidth: 120,
        flex: 0.8,
        renderCell: (params) => (
          <StatusChip
            color={params.row.status === 'active' ? 'success' : 'warning'}
            label={params.row.status === 'active' ? t('status_active_short') : t('status_inactive_short')}
          />
        )
      }
    ],
    [t]
  )

  function handleApplyFilters() {
    const startedAt = performance.now()
    updateSearch(
      {
        keyword: keywordInput,
        page: 0,
        status: statusInput
      },
      {
        asOf: asOfInput,
        includeDisabled: includeDisabledInput
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
        include_disabled: includeDisabledInput
      }
    })
  }

  function handleTreeSelect(nextNodeCode: string) {
    updateSearch(
      { page: 0 },
      {
        selectedNodeCode: nextNodeCode,
        detailCode: nextNodeCode,
        detailEffectiveDate: null
      }
    )
    setSelectedRowId(null)
    setSelectedIds([])
    void ensureChildrenLoaded(nextNodeCode)
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
          selectedNodeCode: result.target_org_code,
          detailCode: result.target_org_code
        }
      )
      setSelectedRowId(result.target_org_code)
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

  function openAction(type: OrgActionType) {
    const details = detailQuery.data?.org_unit
    const code = detailCode ?? selectedRow?.code ?? ''
    const form = emptyActionForm(detailAsOf)
    form.orgCode = code
    form.name = details?.name ?? selectedRow?.name ?? ''
    form.parentOrgCode = details?.parent_org_code ?? ''
    form.managerPernr = details?.manager_pernr ?? ''
    form.isBusinessUnit = details?.is_business_unit ?? selectedRow?.isBusinessUnit ?? false
    if (type === 'rescind_record' && detailAsOf) {
      form.effectiveDate = detailAsOf
    }
    setActionErrorMessage('')
    setActionForm(form)
    setActionState({ type, targetCode: code.length > 0 ? code : null })
  }

  const refreshAfterWrite = useCallback(async () => {
    setChildrenByParent({})
    await queryClient.invalidateQueries({ queryKey: ['org-units'] })
  }, [queryClient])

  const actionMutation = useMutation({
    mutationFn: async () => {
      if (!actionState) {
        return
      }
      const type = actionState.type
      const targetCode = actionForm.orgCode.trim()
      const effectiveDate = actionForm.effectiveDate.trim() || asOf

      if (type === 'create') {
        await createOrgUnit({
          org_code: actionForm.orgCode.trim(),
          name: actionForm.name.trim(),
          effective_date: effectiveDate,
          parent_org_code: actionForm.parentOrgCode.trim() || undefined,
          is_business_unit: actionForm.isBusinessUnit,
          manager_pernr: actionForm.managerPernr.trim() || undefined
        })
        return
      }

      if (!targetCode) {
        throw new Error(t('org_action_target_required'))
      }

      switch (type) {
        case 'rename':
          await renameOrgUnit({
            org_code: targetCode,
            new_name: actionForm.name.trim(),
            effective_date: effectiveDate
          })
          return
        case 'move':
          await moveOrgUnit({
            org_code: targetCode,
            new_parent_org_code: actionForm.parentOrgCode.trim(),
            effective_date: effectiveDate
          })
          return
        case 'set_business_unit':
          await setOrgUnitBusinessUnit({
            org_code: targetCode,
            effective_date: effectiveDate,
            is_business_unit: actionForm.isBusinessUnit,
            request_code: actionForm.requestCode.trim()
          })
          return
        case 'enable':
          await enableOrgUnit({ org_code: targetCode, effective_date: effectiveDate })
          return
        case 'disable':
          await disableOrgUnit({ org_code: targetCode, effective_date: effectiveDate })
          return
        case 'correct': {
          const patch: {
            effective_date?: string
            name?: string
            parent_org_code?: string
            is_business_unit?: boolean
            manager_pernr?: string
          } = {}
          if (actionForm.correctedEffectiveDate.trim().length > 0) {
            patch.effective_date = actionForm.correctedEffectiveDate.trim()
          }
          if (actionForm.name.trim().length > 0) {
            patch.name = actionForm.name.trim()
          }
          if (actionForm.parentOrgCode.trim().length > 0) {
            patch.parent_org_code = actionForm.parentOrgCode.trim()
          }
          if (actionForm.managerPernr.trim().length > 0) {
            patch.manager_pernr = actionForm.managerPernr.trim()
          }
          patch.is_business_unit = actionForm.isBusinessUnit

          await correctOrgUnit({
            org_code: targetCode,
            effective_date: effectiveDate,
            request_id: actionForm.requestId.trim(),
            patch
          })
          return
        }
        case 'rescind_record':
          await rescindOrgUnitRecord({
            org_code: targetCode,
            effective_date: effectiveDate,
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
      setActionState(null)
      setToast({ message: t('common_action_done'), severity: 'success' })
    },
    onError: (error) => {
      setActionErrorMessage(getErrorMessage(error))
    }
  })

  async function runBulkStatusAction(target: OrgStatus) {
    if (selectedIds.length === 0) {
      setToast({ message: t('common_select_rows'), severity: 'warning' })
      return
    }
    if (!canWrite) {
      setToast({ message: t('org_no_write_permission'), severity: 'error' })
      return
    }

    let success = 0
    let failed = 0
    for (const orgCode of selectedIds) {
      try {
        if (target === 'active') {
          await enableOrgUnit({ org_code: orgCode, effective_date: asOf })
        } else {
          await disableOrgUnit({ org_code: orgCode, effective_date: asOf })
        }
        success += 1
      } catch {
        failed += 1
      }
    }

    await refreshAfterWrite()
    if (failed === 0) {
      setToast({ message: t('org_bulk_action_done', { count: success }), severity: 'success' })
    } else {
      setToast({ message: t('org_bulk_action_partial', { success, failed }), severity: 'warning' })
    }
  }

  const requestErrorMessage = rootOrgUnitsQuery.error
    ? getErrorMessage(rootOrgUnitsQuery.error)
    : orgUnitListQuery.error
    ? getErrorMessage(orgUnitListQuery.error)
    : childrenErrorMessage
  const tableLoading = rootOrgUnitsQuery.isLoading || orgUnitListQuery.isFetching

  return (
    <>
      <PageHeader
        subtitle={t('page_org_subtitle')}
        title={t('page_org_title')}
        actions={
          <>
            <Button disabled={!canWrite} onClick={() => openAction('create')} size='small' variant='contained'>
              {t('org_action_create')}
            </Button>
            <Button disabled={!canWrite || !detailCode} onClick={() => openAction('rename')} size='small' variant='outlined'>
              {t('org_action_rename')}
            </Button>
            <Button disabled={!canWrite || !detailCode} onClick={() => openAction('move')} size='small' variant='outlined'>
              {t('org_action_move')}
            </Button>
            <Button disabled={!canWrite || !detailCode} onClick={() => openAction('set_business_unit')} size='small' variant='outlined'>
              {t('org_action_set_business_unit')}
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
        <Button disabled={!canWrite || selectedIds.length === 0} onClick={() => void runBulkStatusAction('active')} variant='outlined'>
          {t('org_bulk_enable')}
        </Button>
        <Button disabled={!canWrite || selectedIds.length === 0} onClick={() => void runBulkStatusAction('inactive')} variant='outlined'>
          {t('org_bulk_disable')}
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
          onSelect={handleTreeSelect}
          selectedItemId={selectedNodeCode ?? undefined}
          title={t('org_tree_title')}
        />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <DataGridPage
            columns={columns}
            gridProps={{
              checkboxSelection: true,
              onPaginationModelChange: (model: GridPaginationModel) => {
                updateSearch({ page: model.page, pageSize: model.pageSize })
              },
              onRowClick: (params) => {
                const nextRowId = String(params.id)
                setSelectedRowId(nextRowId)
                updateSearch(
                  {},
                  {
                    detailCode: nextRowId,
                    detailEffectiveDate: null
                  }
                )
                trackUiEvent({
                  eventName: 'detail_open',
                  tenant: tenantId,
                  module: 'orgunit',
                  page: 'org-units',
                  action: 'row_detail_open',
                  result: 'success',
                  metadata: { row_id: nextRowId }
                })
              },
              onRowSelectionModelChange: (model: GridRowSelectionModel) => {
                const ids = [...model.ids].map((id) => String(id))
                setSelectedIds(ids)
              },
              onSortModelChange: handleSortChange,
              pageSizeOptions: [10, 20, 50],
              pagination: true,
              paginationMode: 'server',
              paginationModel: { page: query.page, pageSize: query.pageSize },
              rowCount: gridRowCount,
              rowSelectionModel: {
                type: 'include',
                ids: new Set(selectedIds)
              },
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

      <DetailPanel
        onClose={() => {
          setSelectedRowId(null)
          updateSearch({}, { detailCode: null, detailEffectiveDate: null })
        }}
        open={detailCode !== null}
        title={detailQuery.data?.org_unit ? `${detailQuery.data.org_unit.name} · ${t('org_detail_title_suffix')}` : t('common_detail')}
      >
        <Tabs
          onChange={(_, value: DetailTab) => updateSearch({}, { tab: value })}
          sx={{ mb: 1 }}
          value={detailTab}
        >
          <Tab label={t('org_tab_profile')} value='profile' />
          <Tab label={t('org_tab_records')} value='records' />
          <Tab label={t('org_tab_audit')} value='audit' />
        </Tabs>

        {detailQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
        {detailQuery.error ? (
          <Alert severity='error'>{getErrorMessage(detailQuery.error)}</Alert>
        ) : null}

        {detailQuery.data && detailTab === 'profile' ? (
          <Stack spacing={1.2}>
            <Typography>{t('org_column_code')}：{detailQuery.data.org_unit.org_code}</Typography>
            <Typography>{t('org_column_name')}：{detailQuery.data.org_unit.name}</Typography>
            <Typography>{t('org_column_parent')}：{detailQuery.data.org_unit.parent_org_code || '-'}</Typography>
            <Typography>{t('org_column_manager')}：{detailQuery.data.org_unit.manager_pernr || '-'}</Typography>
            <Typography>{t('org_column_is_business_unit')}：{detailQuery.data.org_unit.is_business_unit ? t('common_yes') : t('common_no')}</Typography>
            <Typography>
              {t('text_status')}：{parseOrgStatus(detailQuery.data.org_unit.status) === 'active' ? t('status_active_short') : t('status_inactive_short')}
            </Typography>
            <Stack direction='row' flexWrap='wrap' spacing={1}>
              <Button disabled={!canWrite} onClick={() => openAction('enable')} size='small' variant='outlined'>
                {t('org_action_enable')}
              </Button>
              <Button disabled={!canWrite} onClick={() => openAction('disable')} size='small' variant='outlined'>
                {t('org_action_disable')}
              </Button>
              <Button disabled={!canWrite} onClick={() => openAction('correct')} size='small' variant='outlined'>
                {t('org_action_correct')}
              </Button>
              <Button disabled={!canWrite} onClick={() => openAction('rescind_record')} size='small' variant='outlined'>
                {t('org_action_rescind_record')}
              </Button>
              <Button color='error' disabled={!canWrite} onClick={() => openAction('rescind_org')} size='small' variant='outlined'>
                {t('org_action_rescind_org')}
              </Button>
            </Stack>
          </Stack>
        ) : null}

        {detailTab === 'records' ? (
          <Stack spacing={1}>
            {versionsQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
            {versionsQuery.error ? <Alert severity='error'>{getErrorMessage(versionsQuery.error)}</Alert> : null}
            {versionsQuery.data ? (
              <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 320, overflow: 'auto' }}>
                {versionsQuery.data.versions.map((version) => (
                  <ListItemButton
                    key={`${version.event_id}-${version.effective_date}`}
                    onClick={() => updateSearch({}, { detailEffectiveDate: version.effective_date, tab: 'records' })}
                    selected={detailAsOf === version.effective_date}
                  >
                    <ListItemText
                      primary={version.effective_date}
                      secondary={`${t('org_version_event_type')}：${version.event_type}`}
                    />
                  </ListItemButton>
                ))}
              </List>
            ) : null}
          </Stack>
        ) : null}

        {detailTab === 'audit' ? (
          <Stack spacing={1}>
            {auditQuery.isLoading ? <Typography>{t('text_loading')}</Typography> : null}
            {auditQuery.error ? <Alert severity='error'>{getErrorMessage(auditQuery.error)}</Alert> : null}
            {auditQuery.data ? (
              <>
                <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 360, overflow: 'auto' }}>
                  {auditQuery.data.events.map((event) => (
                    <ListItemButton key={event.event_id}>
                      <ListItemText
                        primary={`${event.effective_date} · ${event.event_type}`}
                        secondary={`${event.initiator_name || '-'} / ${event.request_code || '-'}`}
                      />
                    </ListItemButton>
                  ))}
                </List>
                {auditQuery.data.has_more ? (
                  <Button onClick={() => setAuditLimit((previous) => previous + 20)} size='small' variant='outlined'>
                    {t('org_audit_load_more')}
                  </Button>
                ) : null}
              </>
            ) : null}
          </Stack>
        ) : null}
      </DetailPanel>

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
              {actionState.type === 'create' ? (
                <TextField
                  label={t('org_column_code')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, orgCode: event.target.value }))}
                  value={actionForm.orgCode}
                />
              ) : (
                <TextField
                  disabled
                  label={t('org_column_code')}
                  value={actionForm.orgCode}
                />
              )}

              {actionState.type === 'create' || actionState.type === 'rename' || actionState.type === 'correct' ? (
                <TextField
                  label={t('org_column_name')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, name: event.target.value }))}
                  value={actionForm.name}
                />
              ) : null}

              {actionState.type === 'create' || actionState.type === 'move' || actionState.type === 'correct' ? (
                <TextField
                  label={t('org_column_parent')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, parentOrgCode: event.target.value }))}
                  value={actionForm.parentOrgCode}
                />
              ) : null}

              {actionState.type === 'create' || actionState.type === 'correct' ? (
                <TextField
                  label={t('org_column_manager')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
                  value={actionForm.managerPernr}
                />
              ) : null}

              {actionState.type === 'create' || actionState.type === 'set_business_unit' || actionState.type === 'correct' ? (
                <FormControlLabel
                  control={
                    <Switch
                      checked={actionForm.isBusinessUnit}
                      onChange={(event) => setActionForm((previous) => ({ ...previous, isBusinessUnit: event.target.checked }))}
                    />
                  }
                  label={t('org_column_is_business_unit')}
                />
              ) : null}

              {actionState.type !== 'rescind_org' ? (
                <TextField
                  InputLabelProps={{ shrink: true }}
                  label={t('org_column_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, effectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.effectiveDate}
                />
              ) : null}

              {actionState.type === 'correct' ? (
                <TextField
                  InputLabelProps={{ shrink: true }}
                  label={t('org_corrected_effective_date')}
                  onChange={(event) => setActionForm((previous) => ({ ...previous, correctedEffectiveDate: event.target.value }))}
                  type='date'
                  value={actionForm.correctedEffectiveDate}
                />
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
          <Button onClick={() => setActionState(null)}>{t('common_cancel')}</Button>
          <Button onClick={() => actionMutation.mutate()} variant='contained'>
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
