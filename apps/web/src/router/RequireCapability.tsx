import type { PropsWithChildren } from 'react'
import type { AuthzCapabilityKey } from '../authz/capabilities'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'
import { NoAccessPage } from '../pages/NoAccessPage'

interface RequireCapabilityProps {
  requiredCapabilityKey?: AuthzCapabilityKey
}

export function RequireCapability({ requiredCapabilityKey, children }: PropsWithChildren<RequireCapabilityProps>) {
  const { hasRequiredCapability } = useAppPreferences()
  if (!hasRequiredCapability(requiredCapabilityKey)) {
    return <NoAccessPage />
  }

  return <>{children}</>
}
