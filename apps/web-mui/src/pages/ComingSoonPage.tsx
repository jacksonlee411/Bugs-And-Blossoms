import { Alert, Box, Typography } from '@mui/material'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'

interface ComingSoonPageProps {
  moduleNameKey: 'nav_approvals' | 'nav_org_units' | 'nav_people'
}

export function ComingSoonPage({ moduleNameKey }: ComingSoonPageProps) {
  const { t } = useAppPreferences()

  return (
    <Box>
      <Typography component='h2' sx={{ mb: 1 }} variant='h5'>
        {t(moduleNameKey)}
      </Typography>
      <Alert severity='info'>
        <Typography>{t('page_coming_soon_title')}</Typography>
        <Typography variant='body2'>{t('page_coming_soon_subtitle')}</Typography>
      </Alert>
    </Box>
  )
}
