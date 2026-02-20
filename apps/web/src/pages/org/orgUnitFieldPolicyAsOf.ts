const dayPattern = /^\d{4}-\d{2}-\d{2}$/

function isDay(value: string): boolean {
  return dayPattern.test(value)
}

export function resolveAsOfAfterPolicySave(currentAsOf: string, enabledOn: string): string | null {
  if (!isDay(currentAsOf) || !isDay(enabledOn)) {
    return null
  }
  return enabledOn > currentAsOf ? enabledOn : null
}

export function shouldShowFutureEffectiveHint(currentAsOf: string, enabledOn: string): boolean {
  return resolveAsOfAfterPolicySave(currentAsOf, enabledOn) !== null
}
