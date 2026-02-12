import type { PropsWithChildren } from 'react'
import { Paper, Stack } from '@mui/material'

export function FilterBar({ children }: PropsWithChildren) {
  return (
    <Paper sx={{ mb: 2, p: 2 }} variant='outlined'>
      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        {children}
      </Stack>
    </Paper>
  )
}
