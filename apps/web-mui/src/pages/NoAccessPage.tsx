import { Alert, Box, Typography } from '@mui/material'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'

export function NoAccessPage() {
  const { t } = useAppPreferences()

  return (
    <Box>
      <Typography component='h2' sx={{ mb: 1 }} variant='h5'>
        {t('page_no_access_title')}
      </Typography>
      <Alert severity='warning'>{t('page_no_access_subtitle')}</Alert>
    </Box>
  )
}
