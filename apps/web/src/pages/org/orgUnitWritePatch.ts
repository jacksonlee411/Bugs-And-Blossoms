export type OrgUnitWritePatch = Record<string, unknown> & { ext?: Record<string, unknown> }

const coreFieldKeys = new Set(['name', 'parent_org_code', 'status', 'is_business_unit', 'manager_pernr'])

export function buildOrgUnitWritePatch(input: {
  allowedFields: Iterable<string>
  original?: Record<string, unknown> & { ext?: Record<string, unknown> }
  next: Record<string, unknown> & { ext?: Record<string, unknown> }
}): OrgUnitWritePatch | null {
  const patch: OrgUnitWritePatch = {}
  const allowedFields = new Set(input.allowedFields)
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
