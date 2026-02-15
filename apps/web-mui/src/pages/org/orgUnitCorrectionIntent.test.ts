import { describe, expect, it } from 'vitest'
import { buildCorrectPatch } from './orgUnitCorrectionIntent'

describe('buildCorrectPatch', () => {
  it('生效日更正模式仅返回 effective_date', () => {
    const patch = buildCorrectPatch({
      capability: {
        allowed_fields: ['effective_date', 'name'],
        field_payload_keys: {
          effective_date: 'effective_date',
          name: 'name'
        }
      },
      effectiveDate: '2026-02-10',
      correctedEffectiveDate: '2026-02-11',
      original: { name: 'R&D', effective_date: '2026-02-10' },
      next: { name: 'R&D v2', effective_date: '2026-02-11' }
    })

    expect(patch).toEqual({ effective_date: '2026-02-11' })
  })

  it('生效日更正模式但无权限时返回 null', () => {
    const patch = buildCorrectPatch({
      capability: {
        allowed_fields: ['name'],
        field_payload_keys: { name: 'name' }
      },
      effectiveDate: '2026-02-10',
      correctedEffectiveDate: '2026-02-11',
      original: { name: 'R&D', effective_date: '2026-02-10' },
      next: { name: 'R&D', effective_date: '2026-02-11' }
    })

    expect(patch).toBeNull()
  })

  it('普通模式走最小变更裁剪', () => {
    const patch = buildCorrectPatch({
      capability: {
        allowed_fields: ['name', 'org_type'],
        field_payload_keys: {
          name: 'name',
          org_type: 'ext.org_type'
        }
      },
      effectiveDate: '2026-02-10',
      correctedEffectiveDate: '',
      original: { name: 'R&D', ext: { org_type: 'DEPARTMENT' } },
      next: { name: 'R&D', ext: { org_type: 'COMPANY' } }
    })

    expect(patch).toEqual({
      ext: {
        org_type: 'COMPANY'
      }
    })
  })
})

