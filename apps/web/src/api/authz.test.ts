import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock, putMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
  putMock: vi.fn()
}))

vi.mock('./httpClient', () => ({
  httpClient: {
    get: getMock,
    post: postMock,
    put: putMock
  }
}))

import {
  createAuthzRole,
  getPrincipalAuthzAssignment,
  listAuthzAPICatalog,
  listAuthzCapabilities,
  listAuthzRoles,
  listPrincipalAssignmentCandidates,
  loadCurrentAuthzCapabilities,
  replacePrincipalAuthzAssignment,
  updateAuthzRole
} from './authz'

describe('authz api', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
    putMock.mockReset()
  })

  it('loads canonical capability keys from the session endpoint', async () => {
    getMock.mockResolvedValue({
      authz_capability_keys: [
        'orgunit.orgunits:read',
        'orgunit.orgunits:read',
        'not-a-capability-key',
        'iam.dicts:admin',
        42
      ]
    })

    await expect(loadCurrentAuthzCapabilities()).resolves.toEqual([
      'orgunit.orgunits:read',
      'iam.dicts:admin'
    ])
    expect(getMock).toHaveBeenCalledWith('/iam/api/me/capabilities')
  })

  it('loads function authorization options without diagnostic parameters', async () => {
    getMock.mockResolvedValue({ capabilities: [], registry_rev: 'rev' })

    await expect(listAuthzCapabilities({ q: 'org', ownerModule: 'orgunit', scopeDimension: 'organization' })).resolves.toEqual({
      capabilities: [],
      registry_rev: 'rev'
    })

    expect(getMock).toHaveBeenCalledWith(
      '/iam/api/authz/capabilities?q=org&owner_module=orgunit&scope_dimension=organization'
    )
  })

  it('loads API catalog and supports server-side capability filtering', async () => {
    getMock.mockResolvedValue({ api_entries: [] })

    await expect(listAuthzAPICatalog({ authzCapabilityKey: 'orgunit.orgunits:read', method: 'GET' })).resolves.toEqual({
      api_entries: []
    })

    expect(getMock).toHaveBeenCalledWith(
      '/iam/api/authz/api-catalog?method=GET&authz_capability_key=orgunit.orgunits%3Aread'
    )
  })

  it('loads role definitions and drops malformed capability keys from client state', async () => {
    getMock.mockResolvedValue({
      roles: [
        {
          role_slug: 'flower-hr',
          name: '鲜花公司HR',
          description: '',
          system_managed: false,
          revision: 2,
          authz_capability_keys: ['orgunit.orgunits:read', 'bad-key'],
          requires_org_scope: true
        }
      ]
    })

    await expect(listAuthzRoles()).resolves.toEqual({
      roles: [
        {
          role_slug: 'flower-hr',
          name: '鲜花公司HR',
          description: '',
          system_managed: false,
          revision: 2,
          authz_capability_keys: ['orgunit.orgunits:read'],
          requires_org_scope: true
        }
      ]
    })
    expect(getMock).toHaveBeenCalledWith('/iam/api/authz/roles')
  })

  it('saves role definitions through create and update endpoints', async () => {
    postMock.mockResolvedValue({ role: { role_slug: 'flower-hr' } })
    putMock.mockResolvedValue({ role: { role_slug: 'flower-hr' } })

    await createAuthzRole({
      role_slug: 'flower-hr',
      name: '鲜花公司HR',
      description: '',
      authz_capability_keys: ['orgunit.orgunits:read']
    })
    expect(postMock).toHaveBeenCalledWith('/iam/api/authz/roles', {
      role_slug: 'flower-hr',
      name: '鲜花公司HR',
      description: '',
      revision: 0,
      authz_capability_keys: ['orgunit.orgunits:read']
    })

    await updateAuthzRole('flower-hr', {
      role_slug: 'flower-hr',
      name: '鲜花公司HR',
      description: '',
      revision: 2,
      authz_capability_keys: ['orgunit.orgunits:read']
    })
    expect(putMock).toHaveBeenCalledWith('/iam/api/authz/roles/flower-hr', {
      role_slug: 'flower-hr',
      name: '鲜花公司HR',
      description: '',
      revision: 2,
      authz_capability_keys: ['orgunit.orgunits:read']
    })
  })

  it('loads and replaces principal authorization assignments', async () => {
    getMock.mockResolvedValueOnce({ principals: [] }).mockResolvedValueOnce({ principal_id: 'p1', roles: [], org_scopes: [], revision: 1 })
    putMock.mockResolvedValue({ principal_id: 'p1', roles: [], org_scopes: [], revision: 2 })

    await listPrincipalAssignmentCandidates()
    expect(getMock).toHaveBeenCalledWith('/iam/api/authz/user-assignments')

    await getPrincipalAuthzAssignment('p1')
    expect(getMock).toHaveBeenCalledWith('/iam/api/authz/user-assignments?principal_id=p1')

    await replacePrincipalAuthzAssignment('p1', {
      roles: [{ role_slug: 'flower-hr' }],
      org_scopes: [{ org_code: 'FLOWERS', include_descendants: true }],
      revision: 1
    })
    expect(putMock).toHaveBeenCalledWith('/iam/api/authz/user-assignments/p1', {
      roles: [{ role_slug: 'flower-hr' }],
      org_scopes: [{ org_code: 'FLOWERS', include_descendants: true }],
      revision: 1
    })
  })
})
