import type { ReadMode } from './readViewState'

export function trimToNull(value: string | null | undefined): string | null {
  const normalized = value?.trim() ?? ''
  return normalized.length > 0 ? normalized : null
}

export function buildReadSearchParams(
  readMode: ReadMode,
  asOf: string,
  extras: Record<string, string | null | undefined | boolean> = {}
): URLSearchParams {
  const params = new URLSearchParams()
  if (readMode === 'history') {
    params.set('as_of', asOf)
  }

  for (const [key, value] of Object.entries(extras)) {
    if (typeof value === 'boolean') {
      if (value) {
        params.set(key, '1')
      }
      continue
    }

    const normalized = trimToNull(value)
    if (normalized) {
      params.set(key, normalized)
    }
  }

  return params
}
