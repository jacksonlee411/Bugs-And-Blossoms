import { Chip } from '@mui/material'

type StatusColor = 'default' | 'error' | 'info' | 'success' | 'warning'

interface StatusChipProps {
  label: string
  color?: StatusColor
}

export function StatusChip({ label, color = 'default' }: StatusChipProps) {
  return <Chip color={color} label={label} size='small' variant='outlined' />
}
