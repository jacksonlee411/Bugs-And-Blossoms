import { createContext, useContext } from 'react'
import type { PaletteMode } from '@mui/material'
import type { Locale, MessageKey } from '../../i18n/messages'

export interface AppPreferencesContextValue {
  tenantId: string
  locale: Locale
  setLocale: (locale: Locale) => void
  themeMode: PaletteMode
  toggleThemeMode: () => void
  navDebugMode: boolean
  hasPermission: (permissionKey?: string) => boolean
  t: (key: MessageKey) => string
}

export const AppPreferencesContext = createContext<AppPreferencesContextValue | null>(null)

export function useAppPreferences(): AppPreferencesContextValue {
  const context = useContext(AppPreferencesContext)
  if (!context) {
    throw new Error('useAppPreferences must be used within AppPreferencesProvider')
  }

  return context
}
