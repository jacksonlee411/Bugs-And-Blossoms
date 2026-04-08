const dayPattern = /^\d{4}-\d{2}-\d{2}$/

export type ReadMode = 'current' | 'history'

export interface ReadViewState {
  mode: ReadMode
  requestedAsOf: string | null
  effectiveAsOf: string
}

export function isDay(value: string): boolean {
  return dayPattern.test(value.trim())
}

export function todayISODate(): string {
  return new Date().toISOString().slice(0, 10)
}

export function parseRequestedAsOf(raw: string | null): string | null {
  if (!raw) {
    return null
  }
  const value = raw.trim()
  return isDay(value) ? value : null
}

export function resolveReadViewState(rawAsOf: string | null, fallbackToday = todayISODate()): ReadViewState {
  const requestedAsOf = parseRequestedAsOf(rawAsOf)
  if (requestedAsOf) {
    return {
      mode: 'history',
      requestedAsOf,
      effectiveAsOf: requestedAsOf
    }
  }
  return {
    mode: 'current',
    requestedAsOf: null,
    effectiveAsOf: fallbackToday
  }
}
