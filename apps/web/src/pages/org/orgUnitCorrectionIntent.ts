import { buildPatch } from './orgUnitCorrectionPatch'

export interface CorrectEventCapabilityShape {
  allowed_fields?: string[]
  field_payload_keys?: Record<string, string>
}

export interface BuildCorrectPatchInput {
  capability: CorrectEventCapabilityShape
  effectiveDate: string
  correctedEffectiveDate: string
  original: Record<string, unknown> & { ext?: Record<string, unknown> }
  next: Record<string, unknown> & { ext?: Record<string, unknown> }
}

export function buildCorrectPatch(input: BuildCorrectPatchInput): Record<string, unknown> | null {
  const allowedFields = input.capability.allowed_fields ?? []
  const fieldPayloadKeys = input.capability.field_payload_keys ?? {}
  const correctedEffectiveDate = input.correctedEffectiveDate.trim()
  const effectiveDate = input.effectiveDate.trim()

  const inEffectiveDateCorrectionMode =
    correctedEffectiveDate.length > 0 && correctedEffectiveDate !== effectiveDate

  if (inEffectiveDateCorrectionMode) {
    const canWriteEffectiveDate =
      allowedFields.includes('effective_date') && fieldPayloadKeys.effective_date === 'effective_date'
    if (!canWriteEffectiveDate) {
      return null
    }
    return { effective_date: correctedEffectiveDate }
  }

  return buildPatch({
    allowedFields,
    fieldPayloadKeys,
    original: input.original,
    next: input.next
  })
}

