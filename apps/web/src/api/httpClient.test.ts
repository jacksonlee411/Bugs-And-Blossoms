import { describe, expect, it } from 'vitest'
import { buildRequestHeaders } from './httpClient'

describe('buildRequestHeaders', () => {
  it('injects auth, tenant and request id headers', () => {
    const headers = buildRequestHeaders({
      token: 'token-1',
      tenantId: 'tenant-a',
      requestId: 'rid-1'
    })

    expect(headers.Authorization).toBe('Bearer token-1')
    expect(headers['X-Tenant-ID']).toBe('tenant-a')
    expect(headers['X-Request-ID']).toBe('rid-1')
  })

  it('returns empty headers when context is missing', () => {
    expect(buildRequestHeaders({})).toEqual({})
  })
})
