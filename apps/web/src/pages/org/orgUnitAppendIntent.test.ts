import { describe, expect, it } from 'vitest'
import { buildAppendPayload } from './orgUnitAppendIntent'

describe('buildAppendPayload', () => {
  it('按 field_payload_keys 构建 append 请求体', () => {
    const payload = buildAppendPayload({
      capability: {
        allowed_fields: ['effective_date', 'name', 'org_type'],
        field_payload_keys: {
          effective_date: 'effective_date',
          name: 'new_name',
          org_type: 'ext.org_type'
        }
      },
      values: {
        effective_date: '2026-02-16',
        name: 'R&D Center',
        org_type: 'COMPANY'
      }
    })

    expect(payload).toEqual({
      effective_date: '2026-02-16',
      new_name: 'R&D Center',
      ext: {
        org_type: 'COMPANY'
      }
    })
  })

  it('allowed_fields 与 field_payload_keys 不一致时 fail-closed', () => {
    const payload = buildAppendPayload({
      capability: {
        allowed_fields: ['name'],
        field_payload_keys: {
          name: 'new_name',
          effective_date: 'effective_date'
        }
      },
      values: {
        name: 'R&D Center'
      }
    })

    expect(payload).toBeNull()
  })

  it('丢失映射时 fail-closed', () => {
    const payload = buildAppendPayload({
      capability: {
        allowed_fields: ['name', 'effective_date'],
        field_payload_keys: {
          name: 'new_name'
        }
      },
      values: {
        name: 'R&D Center',
        effective_date: '2026-02-16'
      }
    })

    expect(payload).toBeNull()
  })

  it('忽略 undefined 字段，保留 false/null', () => {
    const payload = buildAppendPayload({
      capability: {
        allowed_fields: ['is_business_unit', 'manager_pernr', 'org_type'],
        field_payload_keys: {
          is_business_unit: 'is_business_unit',
          manager_pernr: 'manager_pernr',
          org_type: 'ext.org_type'
        }
      },
      values: {
        is_business_unit: false,
        manager_pernr: undefined,
        org_type: null
      }
    })

    expect(payload).toEqual({
      is_business_unit: false,
      ext: {
        org_type: null
      }
    })
  })
})
