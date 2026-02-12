import { useMemo, useState } from 'react'
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
import type { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { DetailPanel } from '../../components/DetailPanel'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { trackUiEvent } from '../../observability/tracker'

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

function statusColor(status: EmploymentStatus): 'success' | 'warning' {
  return status === 'active' ? 'success' : 'warning'
}

export function PeopleAssignmentsPage() {
  const { t, tenantId } = useAppPreferences()
  const [rows, setRows] = useState<PeopleRow[]>(initialRows)
  const [keywordInput, setKeywordInput] = useState('')
  const [statusInput, setStatusInput] = useState<'all' | EmploymentStatus>('all')
  const [assignmentInput, setAssignmentInput] = useState<'all' | AssignmentType>('all')
  const [filters, setFilters] = useState({
    keyword: '',
    status: 'all' as 'all' | EmploymentStatus,
    assignment: 'all' as 'all' | AssignmentType
  })
  const [selectedRowId, setSelectedRowId] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [pendingBulkAction, setPendingBulkAction] = useState<BulkAction | null>(null)
  const [toastMessage, setToastMessage] = useState<string | null>(null)

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
    const normalizedKeyword = filters.keyword.trim().toLowerCase()
    return rows.filter((row) => {
      const byStatus = filters.status === 'all' ? true : row.status === filters.status
      const byAssignment = filters.assignment === 'all' ? true : row.assignment === filters.assignment
      const byKeyword =
        normalizedKeyword.length === 0
          ? true
          : row.employeeId.toLowerCase().includes(normalizedKeyword) ||
            row.name.toLowerCase().includes(normalizedKeyword) ||
            row.department.toLowerCase().includes(normalizedKeyword) ||
            row.position.toLowerCase().includes(normalizedKeyword)
      return byStatus && byAssignment && byKeyword
    })
  }, [filters.assignment, filters.keyword, filters.status, rows])

  const selectedRow = rows.find((row) => row.id === selectedRowId) ?? null

  function handleApplyFilters() {
    const startedAt = performance.now()
    setFilters({
      keyword: keywordInput,
      status: statusInput,
      assignment: assignmentInput
    })
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

  function handleBulkAction(action: BulkAction) {
    if (selectedIds.length === 0) {
      setToastMessage(t('common_select_rows'))
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
    setToastMessage(t('approvals_feedback_done'))
  }

  return (
    <>
      <PageHeader subtitle={t('page_people_subtitle')} title={t('page_people_title')} />
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
        title={t('page_people_title')}
      />

      <DataGridPage
        columns={columns}
        gridProps={{
          checkboxSelection: true,
          onRowSelectionModelChange: (selection: GridRowSelectionModel) => {
            const ids = [...selection.ids].map((id) => Number(id))
            setSelectedIds(ids)
            const first = selection.ids.values().next().value
            if (first === undefined) {
              setSelectedRowId(null)
              return
            }
            const nextId = Number(first)
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
          rowSelectionModel: {
            type: 'include',
            ids: new Set(selectedIds)
          },
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
            {selectedIds.length} records selected
          </Alert>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingBulkAction(null)}>{t('common_cancel')}</Button>
          <Button onClick={applyBulkAction} variant='contained'>
            {t('common_confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        autoHideDuration={2000}
        onClose={() => setToastMessage(null)}
        open={toastMessage !== null}
      >
        <Alert severity='success' variant='filled'>
          {toastMessage}
        </Alert>
      </Snackbar>
    </>
  )
}
