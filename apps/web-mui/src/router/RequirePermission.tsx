import type { PropsWithChildren } from 'react'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'
import { NoAccessPage } from '../pages/NoAccessPage'

interface RequirePermissionProps {
  permissionKey?: string
}

export function RequirePermission({ permissionKey, children }: PropsWithChildren<RequirePermissionProps>) {
  const { hasPermission } = useAppPreferences()
  if (!hasPermission(permissionKey)) {
    return <NoAccessPage />
  }

  return <>{children}</>
}
