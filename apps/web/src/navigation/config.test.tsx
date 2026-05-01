import { describe, expect, it } from 'vitest'
import { AUTHZ_CAPABILITY_KEYS } from '../authz/capabilities'
import { commonSearchEntries, navItems } from './config'

describe('navigation config', () => {
  it('exposes only current retained primary routes', () => {
    expect(navItems.map((item) => item.key)).toEqual([
      'foundation-demo',
      'org-units',
      'org-field-configs',
      'dict-configs',
      'approval-inbox'
    ])
    expect(navItems.find((item) => item.key === 'foundation-demo')?.requiredCapabilityKey).toBeUndefined()
    expect(navItems.find((item) => item.key === 'org-units')?.requiredCapabilityKey).toBe(
      AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsRead
    )
    expect(navItems.find((item) => item.key === 'org-field-configs')?.requiredCapabilityKey).toBe(
      AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsAdmin
    )
    expect(navItems.find((item) => item.key === 'dict-configs')?.requiredCapabilityKey).toBe(
      AUTHZ_CAPABILITY_KEYS.iamDictsAdmin
    )
    expect(navItems.find((item) => item.key === 'approval-inbox')?.requiredCapabilityKey).toBeUndefined()
    expect(commonSearchEntries.find((entry) => entry.key === 'common-recent-changes')?.requiredCapabilityKey).toBe(
      AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsRead
    )
  })
})
