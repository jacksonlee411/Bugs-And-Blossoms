import { describe, expect, it } from 'vitest'
import { buildPatch } from './orgUnitCorrectionPatch'

describe('buildPatch', () => {
  it('只提交变更字段并按 payload key 写入', () => {
    const patch = buildPatch({
      allowedFields: ['name', 'org_type', 'effective_date'],
      fieldPayloadKeys: {
        name: 'name',
        org_type: 'ext.org_type',
        effective_date: 'effective_date'
      },
      original: {
        name: 'R&D',
        effective_date: '2026-02-10',
        ext: {
          org_type: 'DEPARTMENT'
        }
      },
      next: {
        name: 'R&D',
        effective_date: '2026-02-11',
        ext: {
          org_type: 'BUSINESS_UNIT'
        }
      }
    })

    expect(patch).toEqual({
      effective_date: '2026-02-11',
      ext: {
        org_type: 'BUSINESS_UNIT'
      }
    })
  })

  it('支持扩展字段显式清空（null）', () => {
    const patch = buildPatch({
      allowedFields: ['org_type'],
      fieldPayloadKeys: {
        org_type: 'ext.org_type'
      },
      original: {
        ext: {
          org_type: 'DEPARTMENT'
        }
      },
      next: {
        ext: {
          org_type: null
        }
      }
    })

    expect(patch).toEqual({
      ext: {
        org_type: null
      }
    })
  })

  it('支持 PLAIN 扩展字段最小变更写入', () => {
    const patch = buildPatch({
      allowedFields: ['description'],
      fieldPayloadKeys: {
        description: 'ext.description'
      },
      original: {
        ext: {
          description: 'Old Desc'
        }
      },
      next: {
        ext: {
          description: 'New Desc'
        }
      }
    })

    expect(patch).toEqual({
      ext: {
        description: 'New Desc'
      }
    })
  })

  it('非 allow 字段不会进入 patch', () => {
    const patch = buildPatch({
      allowedFields: ['name'],
      fieldPayloadKeys: {
        name: 'name'
      },
      original: {
        name: 'R&D',
        ext: {
          org_type: 'DEPARTMENT'
        }
      },
      next: {
        name: 'R&D',
        ext: {
          org_type: 'BUSINESS_UNIT'
        }
      }
    })

    expect(patch).toEqual({})
  })
})
