import { describe, expect, it } from 'vitest'
import { nextStageAfterPreview, toMaxConflicts, validatePreviewForm, validateReleaseForm, type DictReleaseFormValues } from './dictReleaseFlow'

function makeValues(overrides?: Partial<DictReleaseFormValues>): DictReleaseFormValues {
  return {
    sourceTenantID: '00000000-0000-0000-0000-000000000000',
    asOf: '2026-01-01',
    releaseID: 'rel-1',
    requestID: 'req-1',
    maxConflicts: '200',
    ...overrides
  }
}

describe('dictReleaseFlow', () => {
  it('returns conflict stage when preview has conflicts', () => {
    expect(
      nextStageAfterPreview({
        release_id: 'r1',
        source_tenant_id: 's1',
        target_tenant_id: 't1',
        as_of: '2026-01-01',
        source_dict_count: 1,
        source_value_count: 1,
        target_dict_count: 0,
        target_value_count: 0,
        missing_dict_count: 1,
        dict_name_mismatch_count: 0,
        missing_value_count: 0,
        value_label_mismatch_count: 0,
        conflicts: []
      })
    ).toBe('conflict')
  })

  it('returns ready stage when preview has no conflicts', () => {
    expect(
      nextStageAfterPreview({
        release_id: 'r1',
        source_tenant_id: 's1',
        target_tenant_id: 't1',
        as_of: '2026-01-01',
        source_dict_count: 1,
        source_value_count: 1,
        target_dict_count: 1,
        target_value_count: 1,
        missing_dict_count: 0,
        dict_name_mismatch_count: 0,
        missing_value_count: 0,
        value_label_mismatch_count: 0,
        conflicts: []
      })
    ).toBe('ready')
  })

  it('validates preview fields before API call', () => {
    const issues = validatePreviewForm(
      makeValues({
        sourceTenantID: 'bad',
        asOf: '',
        releaseID: '',
        maxConflicts: '0'
      })
    )
    expect(issues).toEqual([
      'dict_release_error_source_tenant_invalid',
      'dict_release_error_as_of_required',
      'dict_release_error_release_id_required',
      'dict_release_error_max_conflicts_invalid'
    ])
  })

  it('requires request_id for release action', () => {
    expect(validateReleaseForm(makeValues({ requestID: '' }))).toContain('dict_release_error_request_id_required')
  })

  it('normalizes max conflicts', () => {
    expect(toMaxConflicts('')).toBe(200)
    expect(toMaxConflicts(' 15 ')).toBe(15)
    expect(toMaxConflicts('bad')).toBe(200)
  })
})
