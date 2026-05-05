import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn()
}))

vi.mock('./httpClient', () => ({
  httpClient: {
    get: getMock
  }
}))

import {
  listOrgUnitSelectorChildren,
  listOrgUnitSelectorRoots,
  searchOrgUnitSelector
} from './orgUnitSelector'

describe('orgUnitSelector api', () => {
  beforeEach(() => {
    getMock.mockReset()
  })

  it('loads selector roots through the existing orgunit list endpoint', async () => {
    getMock.mockResolvedValue({
      org_units: [
        {
          org_code: 'ROOT',
          org_node_key: '10000000',
          name: 'Root',
          status: 'active',
          has_visible_children: true
        }
      ]
    })

    await expect(listOrgUnitSelectorRoots({ asOf: '2026-05-04' })).resolves.toEqual([
      {
        org_code: 'ROOT',
        org_node_key: '10000000',
        name: 'Root',
        status: 'active',
        has_visible_children: true,
        path_org_codes: undefined
      }
    ])
    expect(getMock).toHaveBeenCalledWith('/org/api/org-units?as_of=2026-05-04')
  })

  it('loads selector children and keeps include_disabled explicit', async () => {
    getMock.mockResolvedValue({ org_units: [] })

    await listOrgUnitSelectorChildren({ asOf: '2026-05-04', includeDisabled: true, parentOrgCode: 'ROOT' })

    expect(getMock).toHaveBeenCalledWith('/org/api/org-units?as_of=2026-05-04&include_disabled=1&parent_org_code=ROOT')
  })

  it('searches through the existing orgunit search endpoint', async () => {
    getMock.mockResolvedValue({
      target_org_code: 'SH',
      target_name: 'Shanghai',
      path_org_codes: ['ROOT', 'EAST', 'SH'],
      tree_as_of: '2026-05-04'
    })

    await expect(searchOrgUnitSelector({ asOf: '2026-05-04', query: '上海' })).resolves.toEqual({
      org_code: 'SH',
      name: 'Shanghai',
      path_org_codes: ['ROOT', 'EAST', 'SH'],
      tree_as_of: '2026-05-04'
    })
    expect(getMock).toHaveBeenCalledWith('/org/api/org-units/search?as_of=2026-05-04&query=%E4%B8%8A%E6%B5%B7')
  })
})
