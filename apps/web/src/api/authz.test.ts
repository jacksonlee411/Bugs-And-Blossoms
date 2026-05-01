import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn()
}))

vi.mock('./httpClient', () => ({
  httpClient: {
    get: getMock
  }
}))

import { listAuthzAPICatalog, listAuthzCapabilities, loadCurrentAuthzCapabilities } from './authz'

describe('authz api', () => {
  beforeEach(() => {
    getMock.mockReset()
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
})
