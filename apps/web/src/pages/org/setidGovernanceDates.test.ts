import { describe, expect, it } from 'vitest'
import {
  defaultRegistryEffectiveDate,
  nextDayISO,
  resolveDisableAsOf,
  resolveForkEffectiveDate
} from './setidGovernanceDates'

describe('setidGovernanceDates', () => {
  it('uses current date as registry default effective date', () => {
    expect(defaultRegistryEffectiveDate('2026-04-08')).toBe('2026-04-08')
  })

  it('computes next day for valid ISO date', () => {
    expect(nextDayISO('2026-04-08')).toBe('2026-04-09')
  })

  it('returns empty string for invalid base date', () => {
    expect(nextDayISO('bad-date')).toBe('')
  })

  it('forks from next day of current effective date when possible', () => {
    expect(resolveForkEffectiveDate('2026-04-08', '2026-05-01')).toBe('2026-04-09')
  })

  it('falls back to current date when fork source date is invalid', () => {
    expect(resolveForkEffectiveDate('bad-date', '2026-05-01')).toBe('2026-05-01')
  })

  it('uses current date for disable_as_of when row is already in the past', () => {
    expect(resolveDisableAsOf('2026-04-01', '2026-04-08')).toBe('2026-04-08')
  })

  it('uses next day for disable_as_of when row effective date is today or future', () => {
    expect(resolveDisableAsOf('2026-04-08', '2026-04-08')).toBe('2026-04-09')
    expect(resolveDisableAsOf('2026-04-10', '2026-04-08')).toBe('2026-04-11')
  })
})
