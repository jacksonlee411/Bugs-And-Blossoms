import { describe, expect, it } from 'vitest'
import { buildRequestHeaders } from './httpClient'

describe('buildRequestHeaders', () => {
  it('injects auth, tenant and traceparent headers', () => {
    const headers = buildRequestHeaders({
      token: 'token-1',
      tenantId: 'tenant-a',
      traceID: '0123456789abcdef0123456789abcdef'
    })

    expect(headers.Authorization).toBe('Bearer token-1')
    expect(headers['X-Tenant-ID']).toBe('tenant-a')
    expect(headers.traceparent).toMatch(/^00-0123456789abcdef0123456789abcdef-[0-9a-f]{16}-01$/)
  })

  it('returns empty headers when context is missing', () => {
    expect(buildRequestHeaders({}).traceparent).toMatch(/^00-[0-9a-f]{32}-[0-9a-f]{16}-01$/)
  })
})
