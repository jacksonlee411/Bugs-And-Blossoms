import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Snackbar,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import type { GridColDef, GridPaginationModel, GridRowSelectionModel, GridSortModel } from '@mui/x-data-grid'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { DetailPanel } from '../../components/DetailPanel'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { trackUiEvent } from '../../observability/tracker'
import {
  fromGridSortModel,
  parseGridQueryState,
  patchGridQueryState,
  toGridSortModel
} from '../../utils/gridQueryState'

type EmploymentStatus = 'active' | 'inactive'
type AssignmentType = 'primary' | 'secondary'
type BulkAction = 'disable' | 'enable' | 'transfer'

interface PeopleRow {
  id: number
  employeeId: string
  name: string
  department: string
  position: string
  assignment: AssignmentType
  status: EmploymentStatus
}

const initialRows: PeopleRow[] = [
  {
    id: 101,
    employeeId: '00001001',
    name: '张晨',
    department: '人力资源部',
    position: 'HRBP',
    assignment: 'primary',
    status: 'active'
  },
  {
    id: 102,
    employeeId: '00001002',
    name: '林涛',
    department: '研发中心',
    position: '前端工程师',
    assignment: 'primary',
    status: 'active'
  },
  {
    id: 103,
    employeeId: '00001003',
    name: '高扬',
    department: '研发中心',
    position: '后端工程师',
    assignment: 'secondary',
    status: 'active'
  },
  {
    id: 104,
    employeeId: '00001004',
    name: '宋洁',
    department: '财务部',
    position: '会计',
    assignment: 'primary',
    status: 'inactive'
  }
]

const sortableFields = ['employeeId', 'name', 'department', 'position', 'assignment', 'status'] as const

function normalizeAssignmentType(raw: string | null): 'all' | AssignmentType {
  return raw === 'primary' || raw === 'secondary' ? raw : 'all'
}

function statusColor(status: EmploymentStatus): 'success' | 'warning' {
  return status === 'active' ? 'success' : 'warning'
}

export function PeopleAssignmentsPage() {
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
  const assignment = normalizeAssignmentType(searchParams.get('assignment'))

  const [rows, setRows] = useState<PeopleRow[]>(initialRows)
  const [keywordInput, setKeywordInput] = useState(query.keyword)
  const [statusInput, setStatusInput] = useState<'all' | EmploymentStatus>(query.status)
  const [assignmentInput, setAssignmentInput] = useState<'all' | AssignmentType>(assignment)
  const [selectedRowId, setSelectedRowId] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [pendingBulkAction, setPendingBulkAction] = useState<BulkAction | null>(null)
  const [toast, setToast] = useState<{ message: string; severity: 'success' | 'warning' } | null>(null)

  const columns = useMemo<GridColDef<PeopleRow>[]>(
    () => [
      { field: 'employeeId', headerName: t('people_column_employee_id'), minWidth: 130, flex: 1 },
      { field: 'name', headerName: t('people_column_name'), minWidth: 120, flex: 1 },
      { field: 'department', headerName: t('people_column_department'), minWidth: 140, flex: 1.2 },
      { field: 'position', headerName: t('people_column_position'), minWidth: 140, flex: 1.2 },
      {
        field: 'assignment',
        headerName: t('people_column_assignment'),
        minWidth: 120,
        flex: 0.9,
        renderCell: (params) =>
          params.row.assignment === 'primary' ? t('assignment_primary') : t('assignment_secondary')
      },
      {
        field: 'status',
        headerName: t('text_status'),
        minWidth: 120,
        flex: 0.9,
        renderCell: (params) => (
          <StatusChip
            color={statusColor(params.row.status)}
            label={params.row.status === 'active' ? t('status_active_short') : t('status_inactive_short')}
          />
        )
      }
    ],
    [t]
  )

  const filteredRows = useMemo(() => {
    const normalizedKeyword = query.keyword.trim().toLowerCase()
    return rows.filter((row) => {
      const byStatus = query.status === 'all' ? true : row.status === query.status
      const byAssignment = assignment === 'all' ? true : row.assignment === assignment
      const byKeyword =
        normalizedKeyword.length === 0
          ? true
          : row.employeeId.toLowerCase().includes(normalizedKeyword) ||
            row.name.toLowerCase().includes(normalizedKeyword) ||
            row.department.toLowerCase().includes(normalizedKeyword) ||
            row.position.toLowerCase().includes(normalizedKeyword)
      return byStatus && byAssignment && byKeyword
    })
  }, [assignment, query.keyword, query.status, rows])

  const selectedRow = rows.find((row) => row.id === selectedRowId) ?? null
  const sortModel = useMemo(
    () => toGridSortModel(query.sortField, query.sortOrder),
    [query.sortField, query.sortOrder]
  )

  const updateSearch = useCallback(
    (
      patch: Parameters<typeof patchGridQueryState>[1],
      options?: { assignment?: 'all' | AssignmentType }
    ) => {
      const nextParams = patchGridQueryState(searchParams, patch)
      if (options && Object.hasOwn(options, 'assignment')) {
        nextParams.set('assignment', options.assignment ?? 'all')
      }
      setSearchParams(nextParams)
    },
    [searchParams, setSearchParams]
  )

  function handleApplyFilters() {
    const startedAt = performance.now()
    updateSearch(
      {
        keyword: keywordInput,
        page: 0,
        status: statusInput
      },
      { assignment: assignmentInput }
    )
    trackUiEvent({
      eventName: 'filter_submit',
      tenant: tenantId,
      module: 'person',
      page: 'people-assignments',
      action: 'apply_filters',
      result: 'success',
      latencyMs: Math.round(performance.now() - startedAt),
      metadata: {
        status: statusInput,
        assignment: assignmentInput,
        has_keyword: keywordInput.trim().length > 0
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

  function handleBulkAction(action: BulkAction) {
    if (selectedIds.length === 0) {
      setToast({ message: t('common_select_rows'), severity: 'warning' })
      return
    }
    setPendingBulkAction(action)
  }

  function applyBulkAction() {
    if (!pendingBulkAction) {
      return
    }

    setRows((previous) =>
      previous.map((row) => {
        if (!selectedIds.includes(row.id)) {
          return row
        }

        if (pendingBulkAction === 'disable') {
          return { ...row, status: 'inactive' }
        }
        if (pendingBulkAction === 'enable') {
          return { ...row, status: 'active' }
        }
        return { ...row, assignment: row.assignment === 'primary' ? 'secondary' : 'primary' }
      })
    )

    trackUiEvent({
      eventName: 'bulk_action',
      tenant: tenantId,
      module: 'person',
      page: 'people-assignments',
      action: `bulk_${pendingBulkAction}`,
      result: 'success',
      metadata: { count: selectedIds.length }
    })

    setPendingBulkAction(null)
    setToast({ message: t('common_action_done'), severity: 'success' })
  }

  return (
    <>
      <PageHeader
        actions={
          <>
            <Button onClick={() => handleBulkAction('disable')} size='small' variant='outlined'>
              {t('people_bulk_disable')}
            </Button>
            <Button onClick={() => handleBulkAction('enable')} size='small' variant='outlined'>
              {t('people_bulk_enable')}
            </Button>
            <Button onClick={() => handleBulkAction('transfer')} size='small' variant='outlined'>
              {t('people_bulk_transfer')}
            </Button>
          </>
        }
        subtitle={t('page_people_subtitle')}
        title={t('page_people_title')}
      />
      <FilterBar>
        <TextField
          fullWidth
          label={t('people_filter_keyword')}
          onChange={(event) => setKeywordInput(event.target.value)}
          value={keywordInput}
        />
        <FormControl sx={{ minWidth: 160 }}>
          <InputLabel id='people-status-filter'>{t('people_filter_status')}</InputLabel>
          <Select
            label={t('people_filter_status')}
            labelId='people-status-filter'
            onChange={(event) => setStatusInput(event.target.value as 'all' | EmploymentStatus)}
            value={statusInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='active'>{t('status_active')}</MenuItem>
            <MenuItem value='inactive'>{t('status_inactive')}</MenuItem>
          </Select>
        </FormControl>
        <FormControl sx={{ minWidth: 160 }}>
          <InputLabel id='people-assignment-filter'>{t('people_filter_assignment')}</InputLabel>
          <Select
            label={t('people_filter_assignment')}
            labelId='people-assignment-filter'
            onChange={(event) => setAssignmentInput(event.target.value as 'all' | AssignmentType)}
            value={assignmentInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='primary'>{t('assignment_primary')}</MenuItem>
            <MenuItem value='secondary'>{t('assignment_secondary')}</MenuItem>
          </Select>
        </FormControl>
        <Button onClick={handleApplyFilters} variant='contained'>
          {t('action_apply_filters')}
        </Button>
      </FilterBar>

      <DataGridPage
        columns={columns}
        gridProps={{
          checkboxSelection: true,
          onPaginationModelChange: (model: GridPaginationModel) => {
            updateSearch({ page: model.page, pageSize: model.pageSize })
          },
          onRowClick: (params) => {
            const nextId = typeof params.id === 'number' ? params.id : Number(params.id)
            setSelectedRowId(nextId)
            trackUiEvent({
              eventName: 'detail_open',
              tenant: tenantId,
              module: 'person',
              page: 'people-assignments',
              action: 'row_detail_open',
              result: 'success',
              metadata: { row_id: nextId }
            })
          },
          onRowSelectionModelChange: (selection: GridRowSelectionModel) => {
            const ids = [...selection.ids].map((id) => Number(id))
            setSelectedIds(ids)
          },
          onSortModelChange: handleSortChange,
          pageSizeOptions: [10, 20, 50],
          pagination: true,
          paginationModel: { page: query.page, pageSize: query.pageSize },
          rowSelectionModel: {
            type: 'include',
            ids: new Set(selectedIds)
          },
          sortModel,
          sx: { minHeight: 560 }
        }}
        noRowsLabel={t('text_no_data')}
        rows={filteredRows}
      />

      <DetailPanel
        onClose={() => setSelectedRowId(null)}
        open={selectedRow !== null}
        title={selectedRow ? `${selectedRow.name} · ${t('text_employment_info')}` : t('common_detail')}
      >
        {selectedRow ? (
          <Stack spacing={1.2}>
            <Typography>{t('people_column_employee_id')}：{selectedRow.employeeId}</Typography>
            <Typography>{t('people_column_department')}：{selectedRow.department}</Typography>
            <Typography>{t('people_column_position')}：{selectedRow.position}</Typography>
            <Typography>
              {t('people_column_assignment')}：
              {selectedRow.assignment === 'primary' ? t('assignment_primary') : t('assignment_secondary')}
            </Typography>
            <Typography>
              {t('text_status')}：{selectedRow.status === 'active' ? t('status_active_short') : t('status_inactive_short')}
            </Typography>
          </Stack>
        ) : null}
      </DetailPanel>

      <Dialog onClose={() => setPendingBulkAction(null)} open={pendingBulkAction !== null}>
        <DialogTitle>{t('people_bulk_confirm_title')}</DialogTitle>
        <DialogContent>
          <Typography>{t('people_bulk_confirm_message')}</Typography>
          <Alert sx={{ mt: 2 }} severity='info'>
            {t('common_selected_count', { count: selectedIds.length })}
          </Alert>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingBulkAction(null)}>{t('common_cancel')}</Button>
          <Button onClick={applyBulkAction} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar autoHideDuration={2000} onClose={() => setToast(null)} open={toast !== null}>
        <Alert severity={toast?.severity ?? 'success'} variant='filled'>
          {toast?.message ?? ''}
        </Alert>
      </Snackbar>
    </>
  )
}
