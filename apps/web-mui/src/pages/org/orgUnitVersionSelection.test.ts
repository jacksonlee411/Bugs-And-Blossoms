import { describe, expect, it } from 'vitest'
import { resolveOrgUnitEffectiveDate } from './orgUnitVersionSelection'

describe('resolveOrgUnitEffectiveDate', () => {
  it('when requested effective_date hits a real version, keeps requested date', () => {
    const effectiveDate = resolveOrgUnitEffectiveDate({
      asOf: '2026-02-13',
      requestedEffectiveDate: '2026-01-02',
      versions: [
        { effective_date: '2026-01-01' },
        { effective_date: '2026-01-02' },
        { effective_date: '2026-02-01' }
      ]
    })
    expect(effectiveDate).toBe('2026-01-02')
  })

  it('when requested effective_date is missing, picks nearest real version <= as_of', () => {
    const effectiveDate = resolveOrgUnitEffectiveDate({
      asOf: '2026-02-13',
      requestedEffectiveDate: null,
      versions: [
        { effective_date: '2026-01-01' },
        { effective_date: '2026-01-02' },
        { effective_date: '2026-02-01' }
      ]
    })
    expect(effectiveDate).toBe('2026-02-01')
  })

  it('when requested effective_date misses real versions, still falls back to nearest <= as_of', () => {
    const effectiveDate = resolveOrgUnitEffectiveDate({
      asOf: '2026-02-13',
      requestedEffectiveDate: '2026-02-13',
      versions: [
        { effective_date: '2026-01-01' },
        { effective_date: '2026-01-02' },
        { effective_date: '2026-02-01' }
      ]
    })
    expect(effectiveDate).toBe('2026-02-01')
  })

  it('when no version is <= as_of, falls back to earliest real version', () => {
    const effectiveDate = resolveOrgUnitEffectiveDate({
      asOf: '2026-01-01',
      requestedEffectiveDate: null,
      versions: [
        { effective_date: '2026-01-10' },
        { effective_date: '2026-02-01' }
      ]
    })
    expect(effectiveDate).toBe('2026-01-10')
  })
})

