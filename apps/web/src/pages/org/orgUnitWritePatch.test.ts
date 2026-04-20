import { describe, expect, it } from 'vitest'
import { buildOrgUnitWritePatch } from './orgUnitWritePatch'

describe('buildOrgUnitWritePatch', () => {
  it('按 allowed_fields 构建 patch：core 直写，ext 聚合到 ext 对象', () => {
    const patch = buildOrgUnitWritePatch({
      allowedFields: ['name', 'parent_org_code', 'status', 'is_business_unit', 'manager_pernr', 'org_type', 'description'],
      original: {
        name: 'Old',
        parent_org_code: 'P001',
        status: 'active',
        is_business_unit: false,
        manager_pernr: '100',
        ext: { org_type: 'DEPT', description: 'A' }
      },
      next: {
        name: 'New',
        parent_org_code: 'P001',
        status: 'disabled',
        is_business_unit: true,
        manager_pernr: '',
        ext: { org_type: 'DEPT', description: null }
      }
    })

    expect(patch).toEqual({
      name: 'New',
      status: 'disabled',
      is_business_unit: true,
      manager_pernr: '',
      ext: {
        description: null
      }
    })
  })

  it('未允许的字段不会进入 patch', () => {
    const patch = buildOrgUnitWritePatch({
      allowedFields: ['name'],
      next: { name: 'X', status: 'disabled', ext: { description: 'hidden' } }
    })
    expect(patch).toEqual({ name: 'X' })
  })
})
