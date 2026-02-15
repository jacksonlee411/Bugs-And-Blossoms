export type Patch = Record<string, unknown> & { ext?: Record<string, unknown> }

export function buildPatch(input: {
  allowedFields: string[]
  fieldPayloadKeys: Record<string, string>
  original: Record<string, unknown> & { ext?: Record<string, unknown> }
  next: Record<string, unknown> & { ext?: Record<string, unknown> }
}): Patch {
  const patch: Patch = {}

  for (const fieldKey of input.allowedFields) {
    const payloadKey = input.fieldPayloadKeys[fieldKey]
    if (!payloadKey) {
      continue
    }

    const isExt = payloadKey.startsWith('ext.')
    const prevValue = isExt ? input.original.ext?.[fieldKey] : input.original[payloadKey]
    const nextValue = isExt ? input.next.ext?.[fieldKey] : input.next[payloadKey]

    if (typeof nextValue === 'undefined') {
      continue
    }

    if (Object.is(prevValue, nextValue)) {
      continue
    }

    if (isExt) {
      patch.ext ??= {}
      patch.ext[fieldKey] = nextValue
      continue
    }

    patch[payloadKey] = nextValue
  }

  if (patch.ext && Object.keys(patch.ext).length === 0) {
    delete patch.ext
  }

  return patch
}
