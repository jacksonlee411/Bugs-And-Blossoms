import { describe, expect, it } from 'vitest'
import { resolveAsOfAfterPolicySave, shouldShowFutureEffectiveHint } from './orgUnitFieldPolicyAsOf'

describe('orgUnitFieldPolicyAsOf', () => {
  it('returns next as_of when enabled_on is after current as_of', () => {
    expect(resolveAsOfAfterPolicySave('2026-01-01', '2026-02-20')).toBe('2026-02-20')
  })

  it('does not switch as_of when enabled_on is not after current as_of', () => {
    expect(resolveAsOfAfterPolicySave('2026-02-20', '2026-02-20')).toBeNull()
    expect(resolveAsOfAfterPolicySave('2026-02-20', '2026-02-19')).toBeNull()
  })

  it('returns null for invalid date inputs', () => {
    expect(resolveAsOfAfterPolicySave('bad', '2026-02-20')).toBeNull()
    expect(resolveAsOfAfterPolicySave('2026-02-20', 'bad')).toBeNull()
  })

  it('reuses the same rule for visibility hint', () => {
    expect(shouldShowFutureEffectiveHint('2026-01-01', '2026-02-20')).toBe(true)
    expect(shouldShowFutureEffectiveHint('2026-02-20', '2026-02-20')).toBe(false)
  })
})
