import { useState } from 'react'
import { Alert, Box, Button, FormControl, InputLabel, MenuItem, Select, Snackbar, Stack, TextField, Typography } from '@mui/material'
import type { GridColDef, GridRenderCellParams, GridRowSelectionModel } from '@mui/x-data-grid'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { DataGridPage } from '../../components/DataGridPage'
import { DetailPanel } from '../../components/DetailPanel'
import { FilterBar } from '../../components/FilterBar'
import { PageHeader } from '../../components/PageHeader'
import { StatusChip } from '../../components/StatusChip'
import { trackUiEvent } from '../../observability/tracker'

type ApprovalStatus = 'approved' | 'forwarded' | 'pending' | 'rejected'
type ApprovalAction = 'approve' | 'forward' | 'reject'

interface ApprovalRow {
  id: number
  requester: string
  type: string
  submittedAt: string
  status: ApprovalStatus
  summary: string
}

const initialRows: ApprovalRow[] = [
  {
    id: 5001,
    requester: '周航',
    type: '组织调整',
    submittedAt: '2026-02-11 09:20',
    status: 'pending',
    summary: '研发中心前端组人员编制 +2'
  },
  {
    id: 5002,
    requester: '刘芳',
    type: '岗位调动',
    submittedAt: '2026-02-11 14:10',
    status: 'pending',
    summary: '张晨从 HRBP 调整为招聘经理'
  },
  {
    id: 5003,
    requester: '王静',
    type: '离职审批',
    submittedAt: '2026-02-12 10:05',
    status: 'forwarded',
    summary: '宋洁离职流程转交财务负责人'
  }
]

function mapStatusToChipColor(status: ApprovalStatus): 'error' | 'info' | 'success' | 'warning' {
  if (status === 'approved') {
    return 'success'
  }
  if (status === 'rejected') {
    return 'error'
  }
  if (status === 'forwarded') {
    return 'info'
  }
  return 'warning'
}

export function ApprovalsInboxPage() {
  const { t, tenantId } = useAppPreferences()
  const [rows, setRows] = useState(initialRows)
  const [statusInput, setStatusInput] = useState<'all' | ApprovalStatus>('all')
  const [keywordInput, setKeywordInput] = useState('')
  const [filters, setFilters] = useState({ status: 'all' as 'all' | ApprovalStatus, keyword: '' })
  const [selectedRowId, setSelectedRowId] = useState<number | null>(null)
  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const selectedRow = rows.find((row) => row.id === selectedRowId) ?? null

  const filteredRows = (() => {
    const normalizedKeyword = filters.keyword.trim().toLowerCase()
    return rows.filter((row) => {
      const byStatus = filters.status === 'all' ? true : row.status === filters.status
      const byKeyword =
        normalizedKeyword.length === 0
          ? true
          : row.requester.toLowerCase().includes(normalizedKeyword) ||
            row.type.toLowerCase().includes(normalizedKeyword) ||
            row.summary.toLowerCase().includes(normalizedKeyword)
      return byStatus && byKeyword
    })
  })()

  function statusLabel(status: ApprovalStatus): string {
    if (status === 'approved') {
      return t('status_approved')
    }
    if (status === 'rejected') {
      return t('status_rejected')
    }
    if (status === 'forwarded') {
      return t('status_forwarded')
    }
    return t('status_pending')
  }

  function updateStatus(targetRowId: number, action: ApprovalAction) {
    const nextStatus: ApprovalStatus =
      action === 'approve' ? 'approved' : action === 'reject' ? 'rejected' : 'forwarded'

    setRows((previous) =>
      previous.map((row) => {
        if (row.id !== targetRowId) {
          return row
        }
        return { ...row, status: nextStatus }
      })
    )

    trackUiEvent({
      eventName: 'bulk_action',
      tenant: tenantId,
      module: 'approval',
      page: 'approvals-inbox',
      action: `approval_${action}`,
      result: 'success',
      metadata: { row_id: targetRowId }
    })

    setToastMessage(t('approvals_feedback_done'))
  }

  function renderActionButtons(rowId: number) {
    return (
      <Stack direction='row' spacing={1}>
        <Button onClick={() => updateStatus(rowId, 'approve')} size='small' variant='text'>
          {t('approvals_action_approve')}
        </Button>
        <Button onClick={() => updateStatus(rowId, 'reject')} size='small' variant='text'>
          {t('approvals_action_reject')}
        </Button>
        <Button onClick={() => updateStatus(rowId, 'forward')} size='small' variant='text'>
          {t('approvals_action_forward')}
        </Button>
      </Stack>
    )
  }

  const columns: GridColDef<ApprovalRow>[] = [
    { field: 'requester', headerName: t('approvals_column_requester'), minWidth: 120, flex: 1 },
    { field: 'type', headerName: t('approvals_column_type'), minWidth: 130, flex: 1 },
    { field: 'submittedAt', headerName: t('approvals_column_submitted_at'), minWidth: 150, flex: 1 },
    {
      field: 'status',
      headerName: t('approvals_column_status'),
      minWidth: 130,
      flex: 0.9,
      renderCell: (params) => (
        <StatusChip color={mapStatusToChipColor(params.row.status)} label={statusLabel(params.row.status)} />
      )
    },
    {
      field: 'actions',
      headerName: t('approvals_column_actions'),
      minWidth: 280,
      flex: 1.6,
      sortable: false,
      filterable: false,
      renderCell: (params: GridRenderCellParams<ApprovalRow>) => renderActionButtons(params.row.id)
    }
  ]

  function handleApplyFilters() {
    const startedAt = performance.now()
    setFilters({ keyword: keywordInput, status: statusInput })
    trackUiEvent({
      eventName: 'filter_submit',
      tenant: tenantId,
      module: 'approval',
      page: 'approvals-inbox',
      action: 'apply_filters',
      result: 'success',
      latencyMs: Math.round(performance.now() - startedAt),
      metadata: {
        status: statusInput,
        has_keyword: keywordInput.trim().length > 0
      }
    })
  }

  return (
    <>
      <PageHeader subtitle={t('page_approvals_subtitle')} title={t('page_approvals_title')} />
      <FilterBar>
        <TextField
          fullWidth
          label={t('global_search')}
          onChange={(event) => setKeywordInput(event.target.value)}
          value={keywordInput}
        />
        <FormControl sx={{ minWidth: 180 }}>
          <InputLabel id='approval-status-filter'>{t('approvals_filter_status')}</InputLabel>
          <Select
            label={t('approvals_filter_status')}
            labelId='approval-status-filter'
            onChange={(event) => setStatusInput(event.target.value as 'all' | ApprovalStatus)}
            value={statusInput}
          >
            <MenuItem value='all'>{t('status_all')}</MenuItem>
            <MenuItem value='pending'>{t('status_pending')}</MenuItem>
            <MenuItem value='approved'>{t('status_approved')}</MenuItem>
            <MenuItem value='rejected'>{t('status_rejected')}</MenuItem>
            <MenuItem value='forwarded'>{t('status_forwarded')}</MenuItem>
          </Select>
        </FormControl>
        <Button onClick={handleApplyFilters} variant='contained'>
          {t('action_apply_filters')}
        </Button>
      </FilterBar>

      <DataGridPage
        columns={columns}
        gridProps={{
          disableRowSelectionOnClick: false,
          onRowSelectionModelChange: (selection: GridRowSelectionModel) => {
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
              module: 'approval',
              page: 'approvals-inbox',
              action: 'open_approval_detail',
              result: 'success',
              metadata: { row_id: nextId }
            })
          },
          rowSelectionModel: {
            type: 'include',
            ids: selectedRowId === null ? new Set() : new Set([selectedRowId])
          },
          sx: { minHeight: 560 }
        }}
        noRowsLabel={t('text_no_data')}
        rows={filteredRows}
      />

      <DetailPanel
        onClose={() => setSelectedRowId(null)}
        open={selectedRow !== null}
        title={selectedRow ? `${selectedRow.type} · #${selectedRow.id}` : t('common_detail')}
      >
        {selectedRow ? (
          <Stack spacing={1.5}>
            <Typography>{t('approvals_column_requester')}：{selectedRow.requester}</Typography>
            <Typography>{t('approvals_column_type')}：{selectedRow.type}</Typography>
            <Typography>{t('approvals_column_submitted_at')}：{selectedRow.submittedAt}</Typography>
            <Typography>{t('approvals_column_status')}：{statusLabel(selectedRow.status)}</Typography>
            <Alert severity='info'>{selectedRow.summary}</Alert>
            <Box>{renderActionButtons(selectedRow.id)}</Box>
          </Stack>
        ) : null}
      </DetailPanel>

      <Snackbar autoHideDuration={2000} onClose={() => setToastMessage(null)} open={toastMessage !== null}>
        <Alert severity='success' variant='filled'>
          {toastMessage}
        </Alert>
      </Snackbar>
    </>
  )
}
