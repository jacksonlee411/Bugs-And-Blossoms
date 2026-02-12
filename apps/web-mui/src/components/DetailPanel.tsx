import type { PropsWithChildren } from 'react'
import { Box, Drawer, Stack, Typography } from '@mui/material'

interface DetailPanelProps {
  open: boolean
  onClose: () => void
  title: string
}

export function DetailPanel({ open, onClose, title, children }: PropsWithChildren<DetailPanelProps>) {
  return (
    <Drawer anchor='right' onClose={onClose} open={open}>
      <Box sx={{ p: 3, width: 420 }}>
        <Stack spacing={2}>
          <Typography component='h3' variant='h6'>
            {title}
          </Typography>
          {children}
        </Stack>
      </Box>
    </Drawer>
  )
}
