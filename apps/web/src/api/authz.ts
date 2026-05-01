import { type AuthzCapabilityKey, isAuthzCapabilityKey } from '../authz/capabilities'
import { httpClient } from './httpClient'

interface CurrentAuthzCapabilitiesResponse {
  authz_capability_keys?: unknown
}

export async function loadCurrentAuthzCapabilities(): Promise<AuthzCapabilityKey[]> {
  const response = await httpClient.get<CurrentAuthzCapabilitiesResponse>('/iam/api/me/capabilities')
  const rawKeys = Array.isArray(response.authz_capability_keys) ? response.authz_capability_keys : []
  const keys = new Set<AuthzCapabilityKey>()

  rawKeys.forEach((value) => {
    if (typeof value === 'string' && isAuthzCapabilityKey(value)) {
      keys.add(value)
    }
  })

  return [...keys]
}
