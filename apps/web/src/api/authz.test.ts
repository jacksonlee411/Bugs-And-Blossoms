import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn()
}))

vi.mock('./httpClient', () => ({
  httpClient: {
    get: getMock
  }
}))

import { loadCurrentAuthzCapabilities } from './authz'

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
})
