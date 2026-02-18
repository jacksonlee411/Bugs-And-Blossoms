import { describe, expect, it } from 'vitest'
import { normalizePlainExtDraft } from './orgUnitPlainExtValidation'

describe('normalizePlainExtDraft', () => {
  it('omit_empty: empty -> undefined', () => {
    const result = normalizePlainExtDraft({ valueType: 'text', draft: '   ', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: undefined, errorCode: null })
  })

  it('null_empty: empty -> null', () => {
    const result = normalizePlainExtDraft({ valueType: 'text', draft: '   ', mode: 'null_empty' })
    expect(result).toEqual({ normalized: null, errorCode: null })
  })

  it('int: valid integer', () => {
    const result = normalizePlainExtDraft({ valueType: 'int', draft: '  -12 ', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: -12, errorCode: null })
  })

  it('int: invalid integer -> error', () => {
    const result = normalizePlainExtDraft({ valueType: 'int', draft: '12.3', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: null, errorCode: 'org_ext_plain_int_invalid' })
  })

  it('uuid: invalid -> error', () => {
    const result = normalizePlainExtDraft({ valueType: 'uuid', draft: 'not-a-uuid', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: null, errorCode: 'org_ext_plain_uuid_invalid' })
  })

  it('date: invalid -> error', () => {
    const result = normalizePlainExtDraft({ valueType: 'date', draft: '2026-2-1', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: null, errorCode: 'org_ext_plain_date_invalid' })
  })

  it('numeric: valid decimal', () => {
    const result = normalizePlainExtDraft({ valueType: 'numeric', draft: ' -12.50 ', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: -12.5, errorCode: null })
  })

  it('numeric: invalid -> error', () => {
    const result = normalizePlainExtDraft({ valueType: 'numeric', draft: '1.2.3', mode: 'omit_empty' })
    expect(result).toEqual({ normalized: null, errorCode: 'org_ext_plain_numeric_invalid' })
  })
})
