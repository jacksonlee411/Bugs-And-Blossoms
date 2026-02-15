import { type PropsWithChildren, useMemo } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CssBaseline, ThemeProvider } from '@mui/material'
import { LocalizationProvider } from '@mui/x-date-pickers'
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns'
import { buildAppTheme } from '../../theme/theme'
import { useAppPreferences } from './AppPreferencesContext'
import { AppPreferencesProvider } from './AppPreferencesProvider'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false
    }
  }
})

function AppProvidersInner({ children }: PropsWithChildren) {
  const { themeMode } = useAppPreferences()
  const theme = useMemo(() => buildAppTheme(themeMode), [themeMode])

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </LocalizationProvider>
    </ThemeProvider>
  )
}

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <AppPreferencesProvider>
      <AppProvidersInner>{children}</AppProvidersInner>
    </AppPreferencesProvider>
  )
}
