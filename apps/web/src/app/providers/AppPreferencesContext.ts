import { createContext, useContext } from 'react'
import type { PaletteMode } from '@mui/material'
import type { AuthzCapabilityKey } from '../../authz/capabilities'
import type { Locale, MessageKey, MessageVars } from '../../i18n/messages'

export interface AppPreferencesContextValue {
  tenantId: string
  locale: Locale
  setLocale: (locale: Locale) => void
  themeMode: PaletteMode
  toggleThemeMode: () => void
  navDebugMode: boolean
  hasRequiredCapability: (requiredCapabilityKey?: AuthzCapabilityKey) => boolean
  reloadAuthzCapabilities?: () => Promise<void>
  resetAuthzCapabilities?: () => void
  t: (key: MessageKey, vars?: MessageVars) => string
}

export const AppPreferencesContext = createContext<AppPreferencesContextValue | null>(null)

export function useAppPreferences(): AppPreferencesContextValue {
  const context = useContext(AppPreferencesContext)
  if (!context) {
    throw new Error('useAppPreferences must be used within AppPreferencesProvider')
  }

  return context
}
