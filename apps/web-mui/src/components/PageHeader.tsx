import type { ReactNode } from 'react'
import { Box, Stack, Typography } from '@mui/material'

interface PageHeaderProps {
  title: string
  subtitle?: string
  actions?: ReactNode
}

export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <Stack
      alignItems={{ md: 'center', xs: 'flex-start' }}
      direction={{ md: 'row', xs: 'column' }}
      justifyContent='space-between'
      spacing={2}
      sx={{ mb: 2 }}
    >
      <Box>
        <Typography component='h2' variant='h5'>
          {title}
        </Typography>
        {subtitle ? (
          <Typography color='text.secondary' variant='body2'>
            {subtitle}
          </Typography>
        ) : null}
      </Box>
      {actions ? <Stack direction='row' spacing={1}>{actions}</Stack> : null}
    </Stack>
  )
}
