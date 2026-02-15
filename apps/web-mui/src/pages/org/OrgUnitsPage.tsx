import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
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
  MenuItem,
  Select,
  Snackbar,
  Stack,
  Switch,
  TextField
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { GridColDef, GridPaginationModel, GridSortModel } from '@mui/x-data-grid'
import {
  createOrgUnit,
  listOrgUnits,
  listOrgUnitsPage,
  searchOrgUnit,
  type OrgUnitAPIItem,
  type OrgUnitListSortField,
  type OrgUnitListSortOrder,
  type OrgUnitListStatusFilter
} from '../../api/orgUnits'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
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
  managerPernr: string
  effectiveDate: string
  isBusinessUnit: boolean
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
      return { id: item.org_code, label: `${item.name} (${item.org_code})${labelSuffix}` }
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

function emptyCreateForm(asOf: string, parentOrgCode: string | null): CreateOrgUnitForm {
  return {
    orgCode: '',
    name: '',
    parentOrgCode: parentOrgCode ?? '',
    managerPernr: '',
    effectiveDate: asOf,
    isBusinessUnit: false
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

  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [statusInput, setStatusInput] = useState<'all' | OrgStatus>(query.status)
  const [asOfInput, setAsOfInput] = useState(asOf)
  const [includeDisabledInput, setIncludeDisabledInput] = useState(includeDisabled)
  const [treeSearchInput, setTreeSearchInput] = useState('')

  const [childrenByParent, setChildrenByParent] = useState<Record<string, OrgUnitAPIItem[]>>({})
  const [childrenLoading, setChildrenLoading] = useState(false)
  const [childrenErrorMessage, setChildrenErrorMessage] = useState('')
  const [treeSearchErrorMessage, setTreeSearchErrorMessage] = useState('')

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateOrgUnitForm>(() => emptyCreateForm(asOf, null))
  const [createErrorMessage, setCreateErrorMessage] = useState('')
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' | 'error' } | null>(null)

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
  }, [asOf, includeDisabled])

  const rootOrgUnitsQuery = useQuery({
    queryKey: ['org-units', 'roots', asOf, includeDisabled],
    queryFn: () => listOrgUnits({ asOf, includeDisabled }),
    staleTime: 60_000
  })

  const rootOrgUnits = useMemo(() => rootOrgUnitsQuery.data?.org_units ?? [], [rootOrgUnitsQuery.data])
  const selectedNodeCode = parseOptionalValue(searchParams.get('node')) ?? rootOrgUnits[0]?.org_code ?? null

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

  const createMutation = useMutation({
    mutationFn: async () => {
      const effectiveDate = createForm.effectiveDate.trim() || asOf
      await createOrgUnit({
        org_code: createForm.orgCode.trim(),
        name: createForm.name.trim(),
        effective_date: effectiveDate,
        parent_org_code: createForm.parentOrgCode.trim() || undefined,
        is_business_unit: createForm.isBusinessUnit,
        manager_pernr: createForm.managerPernr.trim() || undefined
      })
    },
    onSuccess: async () => {
      await refreshAfterWrite()
      setCreateOpen(false)
      setToast({ message: t('common_action_done'), severity: 'success' })
    },
    onError: (error) => {
      setCreateErrorMessage(getErrorMessage(error))
    }
  })

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
        selectedNodeCode: nextNodeCode
      }
    )
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
              label={t('org_column_code')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, orgCode: event.target.value }))}
              value={createForm.orgCode}
            />
            <TextField
              label={t('org_column_name')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, name: event.target.value }))}
              value={createForm.name}
            />
            <TextField
              label={t('org_column_parent')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, parentOrgCode: event.target.value }))}
              value={createForm.parentOrgCode}
            />
            <TextField
              label={t('org_column_manager')}
              onChange={(event) => setCreateForm((previous) => ({ ...previous, managerPernr: event.target.value }))}
              value={createForm.managerPernr}
            />
            <TextField
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
                  onChange={(event) => setCreateForm((previous) => ({ ...previous, isBusinessUnit: event.target.checked }))}
                />
              }
              label={t('org_column_is_business_unit')}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>{t('common_cancel')}</Button>
          <Button
            disabled={createMutation.isPending}
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
