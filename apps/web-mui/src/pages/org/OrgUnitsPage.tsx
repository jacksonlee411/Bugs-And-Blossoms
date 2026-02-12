import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import type { GridColDef, GridPaginationModel, GridSortModel } from '@mui/x-data-grid'
import { listOrgUnits, type OrgUnitAPIItem } from '../../api/orgUnits'
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

type OrgStatus = 'active' | 'inactive'

interface OrgUnitRow {
  id: string
  code: string
  name: string
  manager: string
  headcount: string
  effectiveDate: string
  status: OrgStatus
}

const sortableFields = ['code', 'name', 'status'] as const

function formatAsOfDate(date: Date): string {
  return date.toISOString().slice(0, 10)
}

function parseSelectedNode(raw: string | null): string | null {
  if (!raw) {
    return null
  }

  const value = raw.trim()
  if (value.length === 0) {
    return null
  }

  return value
}

function toOrgUnitRow(item: OrgUnitAPIItem, asOfDate: string): OrgUnitRow {
  return {
    id: item.org_code,
    code: item.org_code,
    name: item.name,
    manager: '-',
    headcount: '-',
    effectiveDate: asOfDate,
    status: 'active'
  }
}

function collectDescendantCodes(childrenByParent: Record<string, OrgUnitAPIItem[]>, nodeCode: string): Set<string> {
  const result = new Set<string>([nodeCode])
  const queue: string[] = [nodeCode]

  while (queue.length > 0) {
    const currentCode = queue.shift()
    if (!currentCode) {
      continue
    }

    const children = childrenByParent[currentCode] ?? []
    children.forEach((child) => {
      if (!result.has(child.org_code)) {
        result.add(child.org_code)
        queue.push(child.org_code)
      }
    })
  }

  return result
}

function buildTreeNodes(
  roots: OrgUnitAPIItem[],
  childrenByParent: Record<string, OrgUnitAPIItem[]>
): TreePanelNode[] {
  function build(item: OrgUnitAPIItem, path: Set<string>): TreePanelNode {
    if (path.has(item.org_code)) {
      return {
        id: item.org_code,
        label: `${item.name} (${item.org_code})`
      }
    }

    const nextPath = new Set(path)
    nextPath.add(item.org_code)
    const children = childrenByParent[item.org_code] ?? []

    return {
      id: item.org_code,
      label: `${item.name} (${item.org_code})`,
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

export function OrgUnitsPage() {
  const { t, tenantId } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = useMemo(
    () =>
      parseGridQueryState(searchParams, {
        statusValues: ['active', 'inactive'] as const,
        sortFields: sortableFields
      }),
    [searchParams]
  )
  const asOfDate = useMemo(() => formatAsOfDate(new Date()), [])
  const [selectedRowId, setSelectedRowId] = useState<string | null>(null)
  const [childrenByParent, setChildrenByParent] = useState<Record<string, OrgUnitAPIItem[]>>({})
  const [childrenLoading, setChildrenLoading] = useState(false)
  const [childrenErrorMessage, setChildrenErrorMessage] = useState('')

  const rootOrgUnitsQuery = useQuery({
    queryKey: ['org-units', 'roots', asOfDate],
    queryFn: () => listOrgUnits({ asOf: asOfDate }),
    staleTime: 60_000
  })

  const rootOrgUnits = useMemo(
    () => rootOrgUnitsQuery.data?.org_units ?? [],
    [rootOrgUnitsQuery.data]
  )
  const selectedNodeCode = parseSelectedNode(searchParams.get('node')) ?? rootOrgUnits[0]?.org_code ?? null
  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [statusInput, setStatusInput] = useState<'all' | OrgStatus>(query.status)

  const ensureChildrenLoaded = useCallback(
    async (parentOrgCode: string) => {
      if (Object.hasOwn(childrenByParent, parentOrgCode)) {
        return
      }

      setChildrenLoading(true)
      setChildrenErrorMessage('')
      try {
        const response = await listOrgUnits({ asOf: asOfDate, parentOrgCode })
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
    [asOfDate, childrenByParent]
  )

  const treeNodes = useMemo(
    () => buildTreeNodes(rootOrgUnits, childrenByParent),
    [childrenByParent, rootOrgUnits]
  )

  const sortModel = useMemo(
    () => toGridSortModel(query.sortField, query.sortOrder),
    [query.sortField, query.sortOrder]
  )

  const knownOrgUnits = useMemo(() => {
    const byCode = new Map<string, OrgUnitAPIItem>()

    rootOrgUnits.forEach((item) => {
      byCode.set(item.org_code, item)
    })

    Object.values(childrenByParent).forEach((children) => {
      children.forEach((child) => {
        byCode.set(child.org_code, child)
      })
    })

    if (selectedNodeCode && !byCode.has(selectedNodeCode)) {
      byCode.set(selectedNodeCode, {
        org_code: selectedNodeCode,
        name: selectedNodeCode
      })
    }

    return byCode
  }, [childrenByParent, rootOrgUnits, selectedNodeCode])

  const baseRows = useMemo(() => {
    if (!selectedNodeCode) {
      return rootOrgUnits.map((item) => toOrgUnitRow(item, asOfDate))
    }

    const visibleCodes = collectDescendantCodes(childrenByParent, selectedNodeCode)
    return Array.from(visibleCodes)
      .map((code) => knownOrgUnits.get(code))
      .filter((item): item is OrgUnitAPIItem => item !== undefined)
      .map((item) => toOrgUnitRow(item, asOfDate))
  }, [asOfDate, childrenByParent, knownOrgUnits, rootOrgUnits, selectedNodeCode])

  const filteredRows = useMemo(() => {
    const normalizedKeyword = query.keyword.trim().toLowerCase()

    return baseRows.filter((row) => {
      const byStatus = query.status === 'all' ? true : row.status === query.status
      const byKeyword =
        normalizedKeyword.length === 0
          ? true
          : row.name.toLowerCase().includes(normalizedKeyword) || row.code.toLowerCase().includes(normalizedKeyword)

      return byStatus && byKeyword
    })
  }, [baseRows, query.keyword, query.status])

  const sortedRows = useMemo(() => {
    if (!query.sortField || !query.sortOrder) {
      return filteredRows
    }

    const sorted = [...filteredRows]
    const direction = query.sortOrder === 'asc' ? 1 : -1
    sorted.sort((left, right) => {
      const field = query.sortField
      const leftValue = String(left[field as keyof OrgUnitRow] ?? '')
      const rightValue = String(right[field as keyof OrgUnitRow] ?? '')
      return leftValue.localeCompare(rightValue) * direction
    })

    return sorted
  }, [filteredRows, query.sortField, query.sortOrder])

  const pagedRows = useMemo(() => {
    const start = query.page * query.pageSize
    return sortedRows.slice(start, start + query.pageSize)
  }, [query.page, query.pageSize, sortedRows])

  const selectedRow = useMemo(
    () => (selectedRowId ? baseRows.find((row) => row.id === selectedRowId) ?? null : null),
    [baseRows, selectedRowId]
  )

  const columns = useMemo<GridColDef<OrgUnitRow>[]>(
    () => [
      { field: 'code', headerName: t('org_column_code'), minWidth: 130, flex: 1 },
      { field: 'name', headerName: t('org_column_name'), minWidth: 180, flex: 1.3 },
      { field: 'manager', headerName: t('org_column_manager'), minWidth: 140, flex: 1, sortable: false },
      { field: 'headcount', headerName: t('org_column_headcount'), minWidth: 130, flex: 0.8, sortable: false },
      {
        field: 'effectiveDate',
        headerName: t('org_column_effective_date'),
        minWidth: 140,
        flex: 1,
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

  const updateSearch = useCallback(
    (
      patch: Parameters<typeof patchGridQueryState>[1],
      options?: { selectedNodeId?: string | null }
    ) => {
      const nextParams = patchGridQueryState(searchParams, patch)
      if (options && Object.hasOwn(options, 'selectedNodeId')) {
        if (options.selectedNodeId) {
          nextParams.set('node', options.selectedNodeId)
        } else {
          nextParams.delete('node')
        }
      }
      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  function handleApplyFilters(nextKeyword: string, nextStatus: string) {
    const startedAt = performance.now()
    updateSearch({
      keyword: nextKeyword,
      page: 0,
      status: nextStatus
    })

    trackUiEvent({
      eventName: 'filter_submit',
      tenant: tenantId,
      module: 'orgunit',
      page: 'org-units',
      action: 'apply_filters',
      result: 'success',
      latencyMs: Math.round(performance.now() - startedAt),
      metadata: { has_keyword: nextKeyword.trim().length > 0, status: nextStatus }
    })
  }

  function handleTreeSelect(nextNodeId: string) {
    updateSearch(
      { page: 0 },
      { selectedNodeId: nextNodeId }
    )
    setSelectedRowId(null)
    void ensureChildrenLoaded(nextNodeId)
  }

  function handleSortChange(nextSortModel: GridSortModel) {
    const nextSort = fromGridSortModel(nextSortModel, sortableFields)
    updateSearch({
      page: 0,
      sortField: nextSort.sortField,
      sortOrder: nextSort.sortOrder
    })
  }

  const requestErrorMessage = rootOrgUnitsQuery.error
    ? getErrorMessage(rootOrgUnitsQuery.error)
    : childrenErrorMessage
  const treeLoading = rootOrgUnitsQuery.isLoading || childrenLoading
  const tableLoading = rootOrgUnitsQuery.isLoading || childrenLoading

  return (
    <>
      <PageHeader subtitle={t('page_org_subtitle')} title={t('page_org_title')} />
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
        <Button onClick={() => handleApplyFilters(keywordInput, statusInput)} variant='contained'>
          {t('action_apply_filters')}
        </Button>
      </FilterBar>

      {requestErrorMessage.length > 0 ? (
        <Alert severity='error' sx={{ mb: 2 }}>
          {requestErrorMessage}
        </Alert>
      ) : null}

      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <TreePanel
          emptyLabel={t('text_no_data')}
          loading={treeLoading}
          loadingLabel={t('text_loading')}
          minWidth={280}
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
                const nextRowId = String(params.id)
                setSelectedRowId(nextRowId)
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
              onSortModelChange: handleSortChange,
              pageSizeOptions: [10, 20, 50],
              pagination: true,
              paginationMode: 'server',
              paginationModel: { page: query.page, pageSize: query.pageSize },
              rowCount: sortedRows.length,
              rowSelectionModel: {
                type: 'include',
                ids: selectedRowId === null ? new Set() : new Set([selectedRowId])
              },
              sortModel,
              sortingMode: 'server',
              sx: { minHeight: 520 }
            }}
            loading={tableLoading}
            loadingLabel={t('text_loading')}
            noRowsLabel={t('text_no_data')}
            rows={pagedRows}
          />
        </Box>
      </Stack>

      <DetailPanel
        onClose={() => setSelectedRowId(null)}
        open={selectedRow !== null}
        title={selectedRow ? `${selectedRow.name} · ${t('org_detail_title_suffix')}` : t('common_detail')}
      >
        {selectedRow ? (
          <Stack spacing={1.2}>
            <Typography>{t('org_column_code')}：{selectedRow.code}</Typography>
            <Typography>{t('org_column_manager')}：{selectedRow.manager}</Typography>
            <Typography>{t('org_column_headcount')}：{selectedRow.headcount}</Typography>
            <Typography>{t('org_column_effective_date')}：{selectedRow.effectiveDate}</Typography>
            <Typography>
              {t('text_status')}：{selectedRow.status === 'active' ? t('status_active_short') : t('status_inactive_short')}
            </Typography>
          </Stack>
        ) : null}
      </DetailPanel>
    </>
  )
}
