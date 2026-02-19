export type PlainExtValueType = 'text' | 'int' | 'uuid' | 'bool' | 'date' | 'numeric'

export type PlainExtNormalizeMode = 'omit_empty' | 'null_empty'

export type PlainExtValidationErrorCode =
  | 'org_ext_plain_int_invalid'
  | 'org_ext_plain_uuid_invalid'
  | 'org_ext_plain_date_invalid'
  | 'org_ext_plain_numeric_invalid'

const uuidLikeRe =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

const dateLikeRe = /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/

export function normalizePlainExtDraft(input: {
  valueType: PlainExtValueType
  draft: string
  mode: PlainExtNormalizeMode
}): { normalized: unknown; errorCode: null } | { normalized: null; errorCode: PlainExtValidationErrorCode } {
  const trimmed = input.draft.trim()
  const emptyNormalized = input.mode === 'omit_empty' ? undefined : null
  if (trimmed.length === 0) {
    return { normalized: emptyNormalized, errorCode: null }
  }

  switch (input.valueType) {
    case 'int': {
      if (!/^-?\d+$/.test(trimmed)) {
        return { normalized: null, errorCode: 'org_ext_plain_int_invalid' }
      }
      const parsed = Number.parseInt(trimmed, 10)
      if (!Number.isFinite(parsed)) {
        return { normalized: null, errorCode: 'org_ext_plain_int_invalid' }
      }
      return { normalized: parsed, errorCode: null }
    }
    case 'uuid': {
      if (!uuidLikeRe.test(trimmed)) {
        return { normalized: null, errorCode: 'org_ext_plain_uuid_invalid' }
      }
      return { normalized: trimmed, errorCode: null }
    }
    case 'date': {
      if (!dateLikeRe.test(trimmed)) {
        return { normalized: null, errorCode: 'org_ext_plain_date_invalid' }
      }
      return { normalized: trimmed, errorCode: null }
    }
    case 'numeric': {
      // Keep client-side validation lightweight; DB kernel remains the final source of truth.
      if (!/^-?(?:\d+|\d+\.\d+|\.\d+)$/.test(trimmed)) {
        return { normalized: null, errorCode: 'org_ext_plain_numeric_invalid' }
      }
      const parsed = Number(trimmed)
      if (!Number.isFinite(parsed)) {
        return { normalized: null, errorCode: 'org_ext_plain_numeric_invalid' }
      }
      return { normalized: parsed, errorCode: null }
    }
    case 'text':
    case 'bool':
    default:
      return { normalized: trimmed, errorCode: null }
  }
}
