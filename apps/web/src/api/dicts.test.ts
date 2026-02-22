import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn()
}))

vi.mock('./httpClient', () => ({
  httpClient: {
    get: getMock,
    post: postMock
  }
}))

import { executeDictRelease, listDictValues, previewDictRelease } from './dicts'

describe('dicts api', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('builds list values query with q/status/limit', async () => {
    getMock.mockResolvedValue({ values: [] })
    await listDictValues({
      dictCode: 'org_type',
      asOf: '2026-01-01',
      q: 'dept',
      status: 'all',
      limit: 50
    })
    expect(getMock).toHaveBeenCalledWith('/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&q=dept&limit=50&status=all')
  })

  it('posts preview payload to release preview endpoint', async () => {
    postMock.mockResolvedValue({ release_id: 'r1' })
    await previewDictRelease({
      source_tenant_id: '00000000-0000-0000-0000-000000000000',
      as_of: '2026-01-01',
      release_id: 'r1',
      max_conflicts: 200
    })
    expect(postMock).toHaveBeenCalledWith('/iam/api/dicts:release:preview', {
      source_tenant_id: '00000000-0000-0000-0000-000000000000',
      as_of: '2026-01-01',
      release_id: 'r1',
      max_conflicts: 200
    })
  })

  it('posts execute payload to release endpoint', async () => {
    postMock.mockResolvedValue({ release_id: 'r1', request_id: 'req1' })
    await executeDictRelease({
      source_tenant_id: '00000000-0000-0000-0000-000000000000',
      as_of: '2026-01-01',
      release_id: 'r1',
      request_id: 'req1',
      max_conflicts: 200
    })
    expect(postMock).toHaveBeenCalledWith('/iam/api/dicts:release', {
      source_tenant_id: '00000000-0000-0000-0000-000000000000',
      as_of: '2026-01-01',
      release_id: 'r1',
      request_id: 'req1',
      max_conflicts: 200
    })
  })
})
