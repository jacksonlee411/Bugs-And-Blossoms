import { Box, Typography } from '@mui/material'
import {
  DataGrid,
  type DataGridProps,
  type GridColDef,
  type GridRowsProp
} from '@mui/x-data-grid'

interface DataGridPageProps {
  columns: GridColDef[]
  rows: GridRowsProp
  noRowsLabel?: string
  loading?: boolean
  gridProps?: Partial<DataGridProps>
}

function NoRowsOverlay({ label }: { label: string }) {
  return (
    <Box sx={{ p: 3, textAlign: 'center' }}>
      <Typography color='text.secondary' variant='body2'>
        {label}
      </Typography>
    </Box>
  )
}

export function DataGridPage({
  columns,
  rows,
  noRowsLabel = 'No data',
  loading = false,
  gridProps
}: DataGridPageProps) {
  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 2,
        minHeight: 480,
        overflow: 'hidden'
      }}
    >
      <DataGrid
        columns={columns}
        disableRowSelectionOnClick
        loading={loading}
        pageSizeOptions={[10, 20, 50]}
        rows={rows}
        slots={{ noRowsOverlay: () => <NoRowsOverlay label={noRowsLabel} /> }}
        {...gridProps}
      />
    </Box>
  )
}
