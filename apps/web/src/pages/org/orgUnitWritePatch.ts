export interface WriteCapabilitiesShape {
  allowed_fields?: string[]
  field_payload_keys?: Record<string, string>
}

export type OrgUnitWritePatch = Record<string, unknown> & { ext?: Record<string, unknown> }

const coreFieldKeys = new Set(['name', 'parent_org_code', 'status', 'is_business_unit', 'manager_pernr'])

function isCapabilityBijectionValid(capability: WriteCapabilitiesShape): boolean {
  const allowedFields = capability.allowed_fields ?? []
  const payloadKeys = capability.field_payload_keys ?? {}

  for (const fieldKey of allowedFields) {
    if (!(fieldKey in payloadKeys)) {
      return false
    }
  }
  for (const fieldKey of Object.keys(payloadKeys)) {
    if (!allowedFields.includes(fieldKey)) {
      return false
    }
  }
  return true
}

export function buildOrgUnitWritePatch(input: {
  capability: WriteCapabilitiesShape
  original?: Record<string, unknown> & { ext?: Record<string, unknown> }
  next: Record<string, unknown> & { ext?: Record<string, unknown> }
}): OrgUnitWritePatch | null {
  if (!isCapabilityBijectionValid(input.capability)) {
    return null
  }

  const patch: OrgUnitWritePatch = {}
  const allowedFields = input.capability.allowed_fields ?? []
  const original = input.original ?? {}

  for (const fieldKey of allowedFields) {
    if (coreFieldKeys.has(fieldKey)) {
      const prevValue = original[fieldKey]
      const nextValue = input.next[fieldKey]
      if (typeof nextValue === 'undefined') {
        continue
      }
      if (Object.is(prevValue, nextValue)) {
        continue
      }
      patch[fieldKey] = nextValue
      continue
    }

    const prevExt = original.ext?.[fieldKey]
    const nextExt = input.next.ext?.[fieldKey]
    if (typeof nextExt === 'undefined') {
      continue
    }
    if (Object.is(prevExt, nextExt)) {
      continue
    }
    patch.ext ??= {}
    patch.ext[fieldKey] = nextExt
  }

  if (patch.ext && Object.keys(patch.ext).length === 0) {
    delete patch.ext
  }

  return patch
}

