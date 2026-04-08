export function nextDayISO(baseDate: string): string {
  const parsed = new Date(`${baseDate}T00:00:00Z`)
  if (Number.isNaN(parsed.getTime())) {
    return ''
  }
  parsed.setUTCDate(parsed.getUTCDate() + 1)
  return parsed.toISOString().slice(0, 10)
}

export function defaultRegistryEffectiveDate(currentDate: string): string {
  return currentDate
}

export function resolveForkEffectiveDate(currentEffectiveDate: string, currentDate: string): string {
  return nextDayISO(currentEffectiveDate) || currentDate
}

export function resolveDisableAsOf(rowEffectiveDate: string, currentDate: string): string {
  const fallbackDisableAsOf = nextDayISO(rowEffectiveDate) || currentDate
  return currentDate > rowEffectiveDate ? currentDate : fallbackDisableAsOf
}
