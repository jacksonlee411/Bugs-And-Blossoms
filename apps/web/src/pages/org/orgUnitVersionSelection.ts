function isISODate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value)
}

export function resolveOrgUnitEffectiveDate(options: {
  asOf: string
  requestedEffectiveDate: string | null
  versions: readonly { effective_date: string }[]
}): string {
  const asOf = options.asOf
  const requestedRaw = (options.requestedEffectiveDate ?? '').trim()
  const requested = isISODate(requestedRaw) ? requestedRaw : null

  const versions = options.versions ?? []
  if (versions.length === 0) {
    return requested ?? asOf
  }

  if (requested && versions.some((version) => version.effective_date === requested)) {
    return requested
  }

  let earliest: string | null = null
  let latestNotAfterAsOf: string | null = null

  for (const version of versions) {
    const effectiveDate = version.effective_date
    if (!isISODate(effectiveDate)) {
      continue
    }

    if (earliest === null || effectiveDate < earliest) {
      earliest = effectiveDate
    }

    if (effectiveDate <= asOf && (latestNotAfterAsOf === null || effectiveDate > latestNotAfterAsOf)) {
      latestNotAfterAsOf = effectiveDate
    }
  }

  return latestNotAfterAsOf ?? earliest ?? (requested ?? asOf)
}

