import { type AuthzCapabilityKey, isAuthzCapabilityKey } from '../authz/capabilities'
import { httpClient } from './httpClient'

interface CurrentAuthzCapabilitiesResponse {
  authz_capability_keys?: unknown
}

export interface AuthzCapabilityOption {
  authz_capability_key: AuthzCapabilityKey
  object: string
  action: string
  owner_module: string
  resource_label: string
  action_label: string
  scope_dimension: string
  assignable: boolean
  status: string
  surface: string
  sort_order: number
  label: string
  covered: boolean
}

export interface AuthzCapabilitiesResponse {
  capabilities: AuthzCapabilityOption[]
  registry_rev: string
}

export interface AuthzRoleDefinition {
  role_slug: string
  name: string
  description: string
  system_managed: boolean
  revision: number
  authz_capability_keys: AuthzCapabilityKey[]
  requires_org_scope: boolean
}

export interface AuthzRolesResponse {
  roles: AuthzRoleDefinition[]
}

export interface AuthzRoleResponse {
  role: AuthzRoleDefinition
}

export interface SaveAuthzRoleDefinitionRequest {
  role_slug: string
  name: string
  description: string
  revision?: number
  authz_capability_keys: AuthzCapabilityKey[]
}

export interface PrincipalAssignmentCandidate {
  principal_id: string
  email: string
  display_name?: string
}

export interface PrincipalAssignmentCandidatesResponse {
  principals: PrincipalAssignmentCandidate[]
}

export interface PrincipalRoleAssignment {
  role_slug: string
  display_name: string
  description: string
  requires_org_scope: boolean
}

export interface PrincipalOrgScope {
  org_code?: string
  org_name?: string
  include_descendants: boolean
}

export interface PrincipalAuthzAssignmentResponse {
  principal_id: string
  roles: PrincipalRoleAssignment[]
  org_scopes: PrincipalOrgScope[]
  revision: number
}

export interface ReplacePrincipalAuthzAssignmentRequest {
  roles: Array<{ role_slug: string }>
  org_scopes: PrincipalOrgScope[]
  revision: number
}

export interface AuthzAPICatalogEntry {
  method: string
  path: string
  access_control: string
  owner_module: string
  resource_label?: string
  resource_object?: string
  action?: string
  authz_capability_key?: AuthzCapabilityKey
  capability_status?: string
  assignable: boolean
  cubebox_callable: boolean
}

export interface AuthzAPICatalogResponse {
  api_entries: AuthzAPICatalogEntry[]
}

export interface ListAuthzCapabilitiesOptions {
  q?: string
  ownerModule?: string
  scopeDimension?: string
}

export interface ListAuthzAPICatalogOptions {
  q?: string
  method?: string
  accessControl?: string
  ownerModule?: string
  resourceObject?: string
  authzCapabilityKey?: AuthzCapabilityKey
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

export async function listAuthzCapabilities(options: ListAuthzCapabilitiesOptions = {}): Promise<AuthzCapabilitiesResponse> {
  const query = new URLSearchParams()
  if (options.q && options.q.trim().length > 0) {
    query.set('q', options.q.trim())
  }
  if (options.ownerModule && options.ownerModule.trim().length > 0) {
    query.set('owner_module', options.ownerModule.trim())
  }
  if (options.scopeDimension && options.scopeDimension.trim().length > 0) {
    query.set('scope_dimension', options.scopeDimension.trim())
  }
  const suffix = query.toString()
  return httpClient.get<AuthzCapabilitiesResponse>(`/iam/api/authz/capabilities${suffix.length > 0 ? `?${suffix}` : ''}`)
}

export async function listAuthzAPICatalog(options: ListAuthzAPICatalogOptions = {}): Promise<AuthzAPICatalogResponse> {
  const query = new URLSearchParams()
  if (options.q && options.q.trim().length > 0) {
    query.set('q', options.q.trim())
  }
  if (options.method && options.method.trim().length > 0) {
    query.set('method', options.method.trim())
  }
  if (options.accessControl && options.accessControl.trim().length > 0) {
    query.set('access_control', options.accessControl.trim())
  }
  if (options.ownerModule && options.ownerModule.trim().length > 0) {
    query.set('owner_module', options.ownerModule.trim())
  }
  if (options.resourceObject && options.resourceObject.trim().length > 0) {
    query.set('resource_object', options.resourceObject.trim())
  }
  if (options.authzCapabilityKey && options.authzCapabilityKey.trim().length > 0) {
    query.set('authz_capability_key', options.authzCapabilityKey.trim())
  }
  const suffix = query.toString()
  return httpClient.get<AuthzAPICatalogResponse>(`/iam/api/authz/api-catalog${suffix.length > 0 ? `?${suffix}` : ''}`)
}

function normalizeCapabilityKeys(values: string[]): AuthzCapabilityKey[] {
  const keys: AuthzCapabilityKey[] = []
  values.forEach((value) => {
    if (isAuthzCapabilityKey(value)) {
      keys.push(value)
    }
  })
  return keys
}

export async function listAuthzRoles(): Promise<AuthzRolesResponse> {
  const response = await httpClient.get<AuthzRolesResponse>('/iam/api/authz/roles')
  return {
    roles: (response.roles ?? []).map((role) => ({
      ...role,
      authz_capability_keys: normalizeCapabilityKeys(role.authz_capability_keys ?? [])
    }))
  }
}

export async function createAuthzRole(request: SaveAuthzRoleDefinitionRequest): Promise<AuthzRoleResponse> {
  return httpClient.post<AuthzRoleResponse>('/iam/api/authz/roles', {
    role_slug: request.role_slug,
    name: request.name,
    description: request.description,
    revision: request.revision ?? 0,
    authz_capability_keys: request.authz_capability_keys
  })
}

export async function updateAuthzRole(roleSlug: string, request: SaveAuthzRoleDefinitionRequest): Promise<AuthzRoleResponse> {
  return httpClient.put<AuthzRoleResponse>(`/iam/api/authz/roles/${encodeURIComponent(roleSlug)}`, {
    role_slug: request.role_slug,
    name: request.name,
    description: request.description,
    revision: request.revision ?? 0,
    authz_capability_keys: request.authz_capability_keys
  })
}

export async function listPrincipalAssignmentCandidates(): Promise<PrincipalAssignmentCandidatesResponse> {
  return httpClient.get<PrincipalAssignmentCandidatesResponse>('/iam/api/authz/user-assignments')
}

export async function getPrincipalAuthzAssignment(principalId: string): Promise<PrincipalAuthzAssignmentResponse> {
  const query = new URLSearchParams({ principal_id: principalId })
  return httpClient.get<PrincipalAuthzAssignmentResponse>(`/iam/api/authz/user-assignments?${query.toString()}`)
}

export async function replacePrincipalAuthzAssignment(
  principalId: string,
  request: ReplacePrincipalAuthzAssignmentRequest
): Promise<PrincipalAuthzAssignmentResponse> {
  return httpClient.put<PrincipalAuthzAssignmentResponse>(`/iam/api/authz/user-assignments/${encodeURIComponent(principalId)}`, request)
}
