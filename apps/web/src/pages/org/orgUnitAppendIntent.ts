export interface AppendCapabilityShape {
  allowed_fields?: string[]
  field_payload_keys?: Record<string, string>
}

interface AppendPayload {
  ext?: Record<string, unknown>
  [key: string]: unknown
}

function setPayloadValue(payload: AppendPayload, payloadPath: string, value: unknown): boolean {
  const path = payloadPath.trim()
  if (path.length === 0) {
    return false
  }

  if (path.startsWith('ext.')) {
    const fieldKey = path.slice(4).trim()
    if (fieldKey.length === 0) {
      return false
    }
    payload.ext ??= {}
    payload.ext[fieldKey] = value
    return true
  }

  if (path.includes('.')) {
    return false
  }

  payload[path] = value
  return true
}

export function buildAppendPayload(input: {
  capability: AppendCapabilityShape
  values: Record<string, unknown>
}): Record<string, unknown> | null {
  const allowedFields = input.capability.allowed_fields ?? []
  const fieldPayloadKeys = input.capability.field_payload_keys ?? {}

  for (const fieldKey of allowedFields) {
    if (!(fieldKey in fieldPayloadKeys)) {
      return null
    }
  }

  for (const fieldKey of Object.keys(fieldPayloadKeys)) {
    if (!allowedFields.includes(fieldKey)) {
      return null
    }
  }

  const payload: AppendPayload = {}
  for (const fieldKey of allowedFields) {
    const payloadPath = fieldPayloadKeys[fieldKey]
    if (!payloadPath) {
      return null
    }

    const value = input.values[fieldKey]
    if (typeof value === 'undefined') {
      continue
    }

    if (!setPayloadValue(payload, payloadPath, value)) {
      return null
    }
  }

  if (payload.ext && Object.keys(payload.ext).length === 0) {
    delete payload.ext
  }

  return payload
}
