import {
  type PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useState
} from 'react'
import type { PaletteMode } from '@mui/material'
import { loadCurrentAuthzCapabilities } from '../../api/authz'
import type { AuthzCapabilityKey } from '../../authz/capabilities'
import { type Locale, type MessageKey, type MessageVars, getMessage } from '../../i18n/messages'
import { APP_ROUTER_BASENAME } from '../../router/paths'
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

function isLoginPath(pathname: string): boolean {
  return pathname === '/login' || pathname === `${APP_ROUTER_BASENAME}/login`
}

export function AppPreferencesProvider({ children }: PropsWithChildren) {
  const env = import.meta.env
  const [themeMode, setThemeMode] = useState<PaletteMode>(resolveThemeMode)
  const [locale, setLocaleState] = useState<Locale>(resolveLocale)
  const [authzCapabilityKeys, setAuthzCapabilityKeys] = useState<Set<AuthzCapabilityKey>>(() => new Set())
  const tenantId = env.VITE_TENANT_ID ?? 'demo-tenant'
  const navDebugMode = env.DEV && env.VITE_NAV_DEBUG === 'true'

  const resetAuthzCapabilities = useCallback(() => {
    setAuthzCapabilityKeys(new Set())
  }, [])

  const reloadAuthzCapabilities = useCallback(async () => {
    try {
      const capabilityKeys = await loadCurrentAuthzCapabilities()
      setAuthzCapabilityKeys(new Set(capabilityKeys))
    } catch {
      setAuthzCapabilityKeys(new Set())
    }
  }, [])

  useEffect(() => {
    if (typeof window !== 'undefined' && isLoginPath(window.location.pathname)) {
      return
    }

    let cancelled = false
    loadCurrentAuthzCapabilities()
      .then((capabilityKeys) => {
        if (!cancelled) {
          setAuthzCapabilityKeys(new Set(capabilityKeys))
        }
      })
      .catch(() => {
        if (!cancelled) {
          setAuthzCapabilityKeys(new Set())
        }
      })

    return () => {
      cancelled = true
    }
  }, [])

  const hasRequiredCapability = useCallback(
    (requiredCapabilityKey?: AuthzCapabilityKey) => {
      if (!requiredCapabilityKey || requiredCapabilityKey.length === 0) {
        return true
      }

      return authzCapabilityKeys.has(requiredCapabilityKey)
    },
    [authzCapabilityKeys]
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

  const t = useCallback((key: MessageKey, vars?: MessageVars) => getMessage(locale, key, vars), [locale])

  const value = useMemo<AppPreferencesContextValue>(
    () => ({
      tenantId,
      locale,
      setLocale,
      themeMode,
      toggleThemeMode,
      navDebugMode,
      hasRequiredCapability,
      reloadAuthzCapabilities,
      resetAuthzCapabilities,
      t
    }),
    [
      hasRequiredCapability,
      locale,
      navDebugMode,
      reloadAuthzCapabilities,
      resetAuthzCapabilities,
      setLocale,
      t,
      tenantId,
      themeMode,
      toggleThemeMode
    ]
  )

  return <AppPreferencesContext.Provider value={value}>{children}</AppPreferencesContext.Provider>
}
