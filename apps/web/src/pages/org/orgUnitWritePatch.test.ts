import { describe, expect, it } from 'vitest'
import { buildOrgUnitWritePatch } from './orgUnitWritePatch'

describe('buildOrgUnitWritePatch', () => {
  it('按 allowed_fields 构建 patch：core 直写，ext 聚合到 ext 对象', () => {
    const patch = buildOrgUnitWritePatch({
      capability: {
        allowed_fields: ['name', 'parent_org_code', 'status', 'is_business_unit', 'manager_pernr', 'org_type', 'description'],
        field_payload_keys: {
          name: 'name',
          parent_org_code: 'parent_id',
          status: 'status',
          is_business_unit: 'is_business_unit',
          manager_pernr: 'manager_pernr',
          org_type: 'ext.org_type',
          description: 'ext.description'
        }
      },
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

  it('allowed_fields 与 field_payload_keys 不一致时 fail-closed', () => {
    const patch = buildOrgUnitWritePatch({
      capability: {
        allowed_fields: ['name'],
        field_payload_keys: { name: 'name', status: 'status' }
      },
      next: { name: 'X' }
    })
    expect(patch).toBeNull()
  })
})

