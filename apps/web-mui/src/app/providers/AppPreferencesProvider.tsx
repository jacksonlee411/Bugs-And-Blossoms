import {
  type PropsWithChildren,
  useCallback,
  useMemo,
  useState
} from 'react'
import type { PaletteMode } from '@mui/material'
import { type Locale, type MessageKey, getMessage } from '../../i18n/messages'
import { AppPreferencesContext, type AppPreferencesContextValue } from './AppPreferencesContext'

const THEME_STORAGE_KEY = 'web-mui-theme-mode'
const LOCALE_STORAGE_KEY = 'web-mui-locale'

function resolveThemeMode(): PaletteMode {
  const value = window.localStorage.getItem(THEME_STORAGE_KEY)
  return value === 'dark' ? 'dark' : 'light'
}

function resolveLocale(): Locale {
  const value = window.localStorage.getItem(LOCALE_STORAGE_KEY)
  return value === 'en' ? 'en' : 'zh'
}

function resolvePermissions(rawPermissions: string | undefined): Set<string> {
  if (!rawPermissions || rawPermissions.trim().length === 0) {
    return new Set(['*'])
  }

  const values = rawPermissions
    .split(',')
    .map((value) => value.trim())
    .filter((value) => value.length > 0)

  if (values.length === 0) {
    return new Set(['*'])
  }

  return new Set(values)
}

export function AppPreferencesProvider({ children }: PropsWithChildren) {
  const env = import.meta.env
  const [themeMode, setThemeMode] = useState<PaletteMode>(resolveThemeMode)
  const [locale, setLocaleState] = useState<Locale>(resolveLocale)

  const permissions = useMemo(() => resolvePermissions(env.VITE_PERMISSIONS), [env.VITE_PERMISSIONS])
  const tenantId = env.VITE_TENANT_ID ?? 'demo-tenant'
  const navDebugMode = env.DEV && env.VITE_NAV_DEBUG === 'true'

  const hasPermission = useCallback(
    (permissionKey?: string) => {
      if (!permissionKey || permissionKey.length === 0) {
        return true
      }

      return permissions.has('*') || permissions.has(permissionKey)
    },
    [permissions]
  )

  const setLocale = useCallback((nextLocale: Locale) => {
    setLocaleState(nextLocale)
    window.localStorage.setItem(LOCALE_STORAGE_KEY, nextLocale)
  }, [])

  const toggleThemeMode = useCallback(() => {
    setThemeMode((previous) => {
      const nextMode: PaletteMode = previous === 'light' ? 'dark' : 'light'
      window.localStorage.setItem(THEME_STORAGE_KEY, nextMode)
      return nextMode
    })
  }, [])

  const t = useCallback((key: MessageKey) => getMessage(locale, key), [locale])

  const value = useMemo<AppPreferencesContextValue>(
    () => ({
      tenantId,
      locale,
      setLocale,
      themeMode,
      toggleThemeMode,
      navDebugMode,
      hasPermission,
      t
    }),
    [hasPermission, locale, navDebugMode, setLocale, t, tenantId, themeMode, toggleThemeMode]
  )

  return <AppPreferencesContext.Provider value={value}>{children}</AppPreferencesContext.Provider>
}
