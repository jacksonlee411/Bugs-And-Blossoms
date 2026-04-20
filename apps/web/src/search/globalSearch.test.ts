import { describe, expect, it } from 'vitest'
import { createLocalSearchProvider, mergeSearchProviders } from './globalSearch'

describe('global search providers', () => {
  it('filters entries by keyword', async () => {
    const provider = createLocalSearchProvider([
      {
        key: 'nav-org',
        labelKey: 'nav_org_units',
        path: '/org/units',
        source: 'navigation',
        keywords: ['org', '组织']
      },
      {
        key: 'nav-approvals',
        labelKey: 'nav_approvals',
        path: '/approvals',
        source: 'navigation',
        keywords: ['approval', '审批']
      }
    ])

    const result = await provider.search('组织')
    expect(result).toHaveLength(1)
    expect(result[0]?.path).toBe('/org/units')
  })

  it('deduplicates entries when merging providers', async () => {
    const provider = mergeSearchProviders([
      createLocalSearchProvider([
        {
          key: 'entry-1',
          labelKey: 'nav_org_units',
          path: '/org/units',
          source: 'navigation',
          keywords: ['org']
        }
      ]),
      createLocalSearchProvider([
        {
          key: 'entry-1',
          labelKey: 'nav_org_units',
          path: '/org/units',
          source: 'navigation',
          keywords: ['organization']
        }
      ])
    ])

    const result = await provider.search('org')
    expect(result).toHaveLength(1)
  })
})
