import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Box,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { SimpleTreeView } from '@mui/x-tree-view/SimpleTreeView'
import { TreeItem } from '@mui/x-tree-view/TreeItem'
import type { GridColDef, GridPaginationModel, GridRowSelectionModel } from '@mui/x-data-grid'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { DetailPanel } from '../../components/DetailPanel'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { trackUiEvent } from '../../observability/tracker'

type OrgStatus = 'active' | 'inactive'

interface OrgUnitRow {
  id: number
  parentId: number | null
  code: string
  name: string
  manager: string
  headcount: number
  effectiveDate: string
  status: OrgStatus
}

const orgRows: OrgUnitRow[] = [
  {
    id: 1,
    parentId: null,
    code: '10000001',
    name: '总部',
    manager: '张涛',
    headcount: 120,
    effectiveDate: '2026-01-01',
    status: 'active'
  },
  {
    id: 2,
    parentId: 1,
    code: '10000002',
    name: '人力资源部',
    manager: '刘芳',
    headcount: 32,
    effectiveDate: '2026-01-01',
    status: 'active'
  },
  {
    id: 3,
    parentId: 1,
    code: '10000003',
    name: '财务部',
    manager: '王静',
    headcount: 24,
    effectiveDate: '2026-01-01',
    status: 'active'
  },
  {
    id: 4,
    parentId: 1,
    code: '10000004',
    name: '研发中心',
    manager: '李峰',
    headcount: 280,
    effectiveDate: '2026-01-01',
    status: 'active'
  },
  {
    id: 5,
    parentId: 4,
    code: '10000005',
    name: '前端组',
    manager: '周航',
    headcount: 45,
    effectiveDate: '2026-01-01',
    status: 'active'
  },
  {
    id: 6,
    parentId: 4,
    code: '10000006',
    name: '后端组',
    manager: '陈宁',
    headcount: 58,
    effectiveDate: '2026-01-01',
    status: 'inactive'
  }
]

function collectDescendantIds(allRows: OrgUnitRow[], nodeId: number): Set<number> {
  const childrenMap = new Map<number, number[]>()
  allRows.forEach((row) => {
    if (row.parentId === null) {
      return
    }

    const list = childrenMap.get(row.parentId) ?? []
    list.push(row.id)
    childrenMap.set(row.parentId, list)
  })

  const result = new Set<number>([nodeId])
  const queue: number[] = [nodeId]
  while (queue.length > 0) {
    const currentId = queue.shift()
    if (currentId === undefined) {
      continue
    }

    const children = childrenMap.get(currentId) ?? []
    children.forEach((id) => {
      if (!result.has(id)) {
        result.add(id)
        queue.push(id)
      }
    })
  }

  return result
}

function buildTree(
  parentId: number | null,
  rows: OrgUnitRow[],
  onSelect: (id: number) => void
) {
  return rows
    .filter((row) => row.parentId === parentId)
    .map((row) => (
      <TreeItem
        itemId={String(row.id)}
        key={row.id}
        label={
          <Box onClick={() => onSelect(row.id)} sx={{ cursor: 'pointer', py: 0.5 }}>
            {row.name}
          </Box>
        }
      >
        {buildTree(row.id, rows, onSelect)}
      </TreeItem>
    ))
}

export function OrgUnitsPage() {
  const { t, tenantId } = useAppPreferences()
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedRowId, setSelectedRowId] = useState<number | null>(null)
  const [loadingTree, setLoadingTree] = useState(false)

  const selectedNodeId = Number(searchParams.get('node') ?? '1')
  const keyword = searchParams.get('q') ?? ''
  const status = (searchParams.get('status') ?? 'all') as 'all' | OrgStatus
  const page = Number(searchParams.get('page') ?? '0')
  const pageSize = Number(searchParams.get('size') ?? '10')

  const visibleNodeIds = useMemo(
    () => collectDescendantIds(orgRows, selectedNodeId),
    [selectedNodeId]
  )

  const filteredRows = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase()
    return orgRows.filter((row) => {
      const byTree = visibleNodeIds.has(row.id)
      const byStatus = status === 'all' ? true : row.status === status
      const byKeyword =
        normalizedKeyword.length === 0
          ? true
          : row.name.toLowerCase().includes(normalizedKeyword) ||
            row.code.toLowerCase().includes(normalizedKeyword) ||
            row.manager.toLowerCase().includes(normalizedKeyword)

      return byTree && byStatus && byKeyword
    })
  }, [keyword, status, visibleNodeIds])

  const pagedRows = useMemo(() => {
    const start = page * pageSize
    return filteredRows.slice(start, start + pageSize)
  }, [filteredRows, page, pageSize])

  const selectedRow = orgRows.find((row) => row.id === selectedRowId) ?? null

  const columns = useMemo<GridColDef<OrgUnitRow>[]>(
    () => [
      { field: 'code', headerName: t('org_column_code'), minWidth: 130, flex: 1 },
      { field: 'name', headerName: t('org_column_name'), minWidth: 180, flex: 1.3 },
      { field: 'manager', headerName: t('org_column_manager'), minWidth: 140, flex: 1 },
      { field: 'headcount', headerName: t('org_column_headcount'), minWidth: 130, flex: 0.8 },
      { field: 'effectiveDate', headerName: t('org_column_effective_date'), minWidth: 140, flex: 1 },
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

  const updateParam = useCallback(
    (updater: (draft: URLSearchParams) => void) => {
      const draft = new URLSearchParams(searchParams)
      updater(draft)
      setSearchParams(draft)
    },
    [searchParams, setSearchParams]
  )

  function handleApplyFilters(nextKeyword: string, nextStatus: string) {
    const startedAt = performance.now()
    updateParam((draft) => {
      draft.set('q', nextKeyword)
      draft.set('status', nextStatus)
      draft.set('page', '0')
      draft.set('size', String(pageSize))
      draft.set('node', String(selectedNodeId))
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

  async function handleTreeSelect(nextNodeId: number) {
    setLoadingTree(true)
    await Promise.resolve()
    updateParam((draft) => {
      draft.set('node', String(nextNodeId))
      draft.set('page', '0')
      draft.set('size', String(pageSize))
    })
    setLoadingTree(false)
  }

  return (
    <>
      <PageHeader subtitle={t('page_org_subtitle')} title={t('page_org_title')} />
      <FilterBar>
        <TextField
          defaultValue={keyword}
          fullWidth
          label={t('org_filter_keyword')}
          onBlur={(event) => handleApplyFilters(event.target.value, status)}
        />
        <FormControl sx={{ minWidth: 180 }}>
          <InputLabel id='org-status-filter'>{t('org_filter_status')}</InputLabel>
          <Select
            defaultValue={status}
            id='org-status-filter-select'
            label={t('org_filter_status')}
            labelId='org-status-filter'
            onChange={(event) => handleApplyFilters(keyword, String(event.target.value))}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='active'>{t('status_active')}</MenuItem>
            <MenuItem value='inactive'>{t('status_inactive')}</MenuItem>
          </Select>
        </FormControl>
      </FilterBar>

      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <Paper sx={{ minWidth: 280, p: 2 }} variant='outlined'>
          <Typography sx={{ mb: 1 }} variant='subtitle2'>
            {t('org_tree_title')}
          </Typography>
          <SimpleTreeView>{buildTree(null, orgRows, handleTreeSelect)}</SimpleTreeView>
          {loadingTree ? (
            <Typography color='text.secondary' sx={{ mt: 1 }} variant='body2'>
              {t('text_loading')}
            </Typography>
          ) : null}
        </Paper>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <DataGridPage
            columns={columns}
            gridProps={{
              onPaginationModelChange: (model: GridPaginationModel) => {
                updateParam((draft) => {
                  draft.set('page', String(model.page))
                  draft.set('size', String(model.pageSize))
                })
              },
              onRowSelectionModelChange: (selection: GridRowSelectionModel) => {
                const first = selection.ids.values().next().value
                if (first === undefined) {
                  setSelectedRowId(null)
                  return
                }
                const nextRowId = Number(first)
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
              pageSizeOptions: [10, 20, 50],
              pagination: true,
              paginationMode: 'server',
              paginationModel: { page, pageSize },
              rowCount: filteredRows.length,
              rowSelectionModel: {
                type: 'include',
                ids: selectedRowId === null ? new Set() : new Set([selectedRowId])
              },
              sx: { minHeight: 520 }
            }}
            noRowsLabel={t('common_select_department')}
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
