import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Button,
  Box,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import type { GridColDef, GridPaginationModel, GridSortModel } from '@mui/x-data-grid'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'
import { DataGridPage } from '../components/DataGridPage'
import { DetailPanel } from '../components/DetailPanel'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { type TreePanelNode, TreePanel } from '../components/TreePanel'
import { trackUiEvent } from '../observability/tracker'
import {
  fromGridSortModel,
  parseGridQueryState,
  patchGridQueryState,
  toGridSortModel
} from '../utils/gridQueryState'

interface DepartmentNode {
  id: number
  name: string
  children?: DepartmentNode[]
}

interface EmployeeRow {
  id: number
  name: string
  department: string
  departmentId: number
  position: string
  status: 'active' | 'inactive'
}

const departmentTree: DepartmentNode[] = [
  {
    id: 1,
    name: '总部',
    children: [
      { id: 2, name: '人力资源部' },
      { id: 3, name: '财务部' },
      {
        id: 4,
        name: '研发中心',
        children: [
          { id: 5, name: '前端组' },
          { id: 6, name: '后端组' }
        ]
      }
    ]
  }
]

const sortableFields = ['name', 'department', 'position', 'status'] as const

function mapDepartmentNodes(nodes: DepartmentNode[]): TreePanelNode[] {
  return nodes.map((node) => ({
    id: String(node.id),
    label: node.name,
    children: node.children ? mapDepartmentNodes(node.children) : undefined
  }))
}

const rows: EmployeeRow[] = [
  { id: 1, name: '张三', department: '人力资源部', departmentId: 2, position: 'HRBP', status: 'active' },
  { id: 2, name: '李四', department: '财务部', departmentId: 3, position: '会计', status: 'active' },
  { id: 3, name: '王五', department: '前端组', departmentId: 5, position: '前端工程师', status: 'active' },
  { id: 4, name: '赵六', department: '后端组', departmentId: 6, position: '后端工程师', status: 'inactive' }
]

function parseDepartmentId(raw: string | null): number | null {
  if (!raw) {
    return null
  }

  const value = Number(raw)
  if (!Number.isInteger(value) || value <= 0) {
    return null
  }

  return value
}

export function FoundationDemoPage() {
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

  const selectedDepartmentId = parseDepartmentId(searchParams.get('node'))
  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [statusInput, setStatusInput] = useState<'all' | 'active' | 'inactive'>(query.status)
  const [selectedRowId, setSelectedRowId] = useState<number | null>(null)

  const columns = useMemo<GridColDef<EmployeeRow>[]>(
    () => [
      { field: 'name', headerName: t('text_name'), flex: 1, minWidth: 120 },
      { field: 'department', headerName: t('text_department'), flex: 1, minWidth: 120 },
      { field: 'position', headerName: t('text_position'), flex: 1, minWidth: 140 },
      {
        field: 'status',
        headerName: t('text_status'),
        flex: 1,
        minWidth: 120,
        renderCell: (params) =>
          params.row.status === 'active' ? t('status_active_short') : t('status_inactive_short')
      }
    ],
    [t]
  )

  const filteredRows = useMemo(() => {
    const normalizedKeyword = query.keyword.trim().toLowerCase()

    return rows.filter((row) => {
      const byDepartment = selectedDepartmentId ? row.departmentId === selectedDepartmentId : true
      const byStatus = query.status === 'all' ? true : row.status === query.status
      const byKeyword =
        normalizedKeyword.length === 0
          ? true
          : row.name.toLowerCase().includes(normalizedKeyword) ||
            row.department.toLowerCase().includes(normalizedKeyword) ||
            row.position.toLowerCase().includes(normalizedKeyword)

      return byDepartment && byStatus && byKeyword
    })
  }, [query.keyword, query.status, selectedDepartmentId])

  const selectedRow = rows.find((item) => item.id === selectedRowId) ?? null
  const sortModel = useMemo(
    () => toGridSortModel(query.sortField, query.sortOrder),
    [query.sortField, query.sortOrder]
  )

  const updateSearch = useCallback(
    (
      patch: Parameters<typeof patchGridQueryState>[1],
      options?: { departmentId?: number | null }
    ) => {
      const nextParams = patchGridQueryState(searchParams, patch)
      if (options && Object.hasOwn(options, 'departmentId')) {
        if (options.departmentId) {
          nextParams.set('node', String(options.departmentId))
        } else {
          nextParams.delete('node')
        }
      }
      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  function handleApplyFilters() {
    const startedAt = performance.now()
    updateSearch({
      keyword: keywordInput,
      status: statusInput,
      page: 0
    })
    trackUiEvent({
      eventName: 'filter_submit',
      tenant: tenantId,
      module: 'foundation',
      page: 'foundation-demo',
      action: 'apply_filters',
      result: 'success',
      latencyMs: Math.round(performance.now() - startedAt),
      metadata: {
        has_keyword: keywordInput.trim().length > 0,
        status: statusInput
      }
    })
  }

  function handleSortChange(nextSortModel: GridSortModel) {
    const nextSort = fromGridSortModel(nextSortModel, sortableFields)
    updateSearch({
      page: 0,
      sortField: nextSort.sortField,
      sortOrder: nextSort.sortOrder
    })
  }

  function handleTreeSelect(nodeId: string) {
    const nextDepartmentId = Number(nodeId)
    if (!Number.isInteger(nextDepartmentId) || nextDepartmentId <= 0) {
      return
    }

    updateSearch({ page: 0 }, { departmentId: nextDepartmentId })
  }

  return (
    <>
      <PageHeader subtitle={t('page_foundation_subtitle')} title={t('page_foundation_title')} />
      <FilterBar>
        <TextField
          fullWidth
          label={t('page_search_label')}
          onChange={(event) => setKeywordInput(event.target.value)}
          value={keywordInput}
        />
        <FormControl sx={{ minWidth: 180 }}>
          <InputLabel id='status-filter-label'>{t('page_status_label')}</InputLabel>
          <Select
            label={t('page_status_label')}
            labelId='status-filter-label'
            onChange={(event) => setStatusInput(event.target.value as 'all' | 'active' | 'inactive')}
            value={statusInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='active'>{t('status_active')}</MenuItem>
            <MenuItem value='inactive'>{t('status_inactive')}</MenuItem>
          </Select>
        </FormControl>
        <Button onClick={handleApplyFilters} variant='contained'>
          {t('action_apply_filters')}
        </Button>
      </FilterBar>

      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <TreePanel
          emptyLabel={t('text_no_data')}
          loadingLabel={t('text_loading')}
          nodes={mapDepartmentNodes(departmentTree)}
          onSelect={handleTreeSelect}
          selectedItemId={selectedDepartmentId ? String(selectedDepartmentId) : undefined}
          title={t('page_department_tree')}
        />

        <Box sx={{ flex: 1, minWidth: 0 }}>
          <DataGridPage
            columns={columns}
            gridProps={{
              onPaginationModelChange: (model: GridPaginationModel) => {
                updateSearch({ page: model.page, pageSize: model.pageSize })
              },
              onRowClick: (params) => {
                const nextRowId = typeof params.id === 'number' ? params.id : Number(params.id)
                setSelectedRowId(nextRowId)
                trackUiEvent({
                  eventName: 'detail_open',
                  tenant: tenantId,
                  module: 'foundation',
                  page: 'foundation-demo',
                  action: 'row_detail_open',
                  result: 'success',
                  metadata: { row_id: nextRowId }
                })
              },
              onSortModelChange: handleSortChange,
              pageSizeOptions: [10, 20, 50],
              pagination: true,
              paginationModel: { page: query.page, pageSize: query.pageSize },
              rowSelectionModel: {
                type: 'include',
                ids: selectedRowId === null ? new Set() : new Set([selectedRowId])
              },
              sortModel,
              sx: { minHeight: 480 }
            }}
            noRowsLabel={t('text_no_data')}
            rows={filteredRows}
          />
        </Box>
      </Stack>

      <DetailPanel
        onClose={() => setSelectedRowId(null)}
        open={selectedRow !== null}
        title={selectedRow ? `${selectedRow.name} · ${t('text_employment_info')}` : t('text_detail_title')}
      >
        {selectedRow ? (
          <Stack spacing={1.5}>
            <Typography>{t('text_department')}：{selectedRow.department}</Typography>
            <Typography>{t('text_position')}：{selectedRow.position}</Typography>
            <Typography>
              {t('text_status')}：{selectedRow.status === 'active' ? t('status_active_short') : t('status_inactive_short')}
            </Typography>
          </Stack>
        ) : null}
      </DetailPanel>
    </>
  )
}
