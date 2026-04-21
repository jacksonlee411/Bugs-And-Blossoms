import { describe, expect, it } from 'vitest'
import { navItems } from './config'

describe('navigation config', () => {
  it('exposes only current retained primary routes', () => {
    expect(navItems.map((item) => item.key)).toEqual([
      'foundation-demo',
      'org-units',
      'org-field-configs',
      'dict-configs',
      'approval-inbox'
    ])
    expect(navItems.find((item) => item.key === 'org-units')?.permissionKey).toBe('orgunit.read')
    expect(navItems.find((item) => item.key === 'org-field-configs')?.permissionKey).toBe('orgunit.admin')
    expect(navItems.find((item) => item.key === 'dict-configs')?.permissionKey).toBe('dict.admin')
    expect(navItems.find((item) => item.key === 'approval-inbox')?.permissionKey).toBe('approval.read')
  })
})
