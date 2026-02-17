function isISODate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value)
}

function toUTCDate(isoDate: string): Date | null {
  if (!isISODate(isoDate)) {
    return null
  }
  const parts = isoDate.split('-')
  if (parts.length !== 3) {
    return null
  }
  const year = Number(parts[0])
  const month = Number(parts[1])
  const day = Number(parts[2])
  if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) {
    return null
  }
  const date = new Date(Date.UTC(year, month - 1, day))
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return date
}

function formatUTCDate(date: Date): string {
  const y = String(date.getUTCFullYear())
  const m = String(date.getUTCMonth() + 1).padStart(2, '0')
  const d = String(date.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function addDays(isoDate: string, deltaDays: number): string | null {
  const date = toUTCDate(isoDate)
  if (!date) {
    return null
  }
  date.setUTCDate(date.getUTCDate() + deltaDays)
  return formatUTCDate(date)
}

function sortUniqueISODates(values: readonly string[]): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const raw of values) {
    const value = raw.trim()
    if (!isISODate(value)) {
      continue
    }
    if (seen.has(value)) {
      continue
    }
    seen.add(value)
    out.push(value)
  }
  out.sort()
  return out
}

export type RecordWizardMode = 'add' | 'insert'

export type RecordDatePlanKind =
  | 'add'
  | 'insert'
  | 'insert_as_add'
  | 'insert_no_slot'
  | 'invalid_input'

export interface RecordDatePlan {
  kind: RecordDatePlanKind
  selectedEffectiveDate: string
  lastEffectiveDate: string | null
  defaultDate: string
  minDate: string | null
  maxDate: string | null
}

export function planRecordEffectiveDate(options: {
  mode: RecordWizardMode
  versions: readonly { effective_date: string }[]
  selectedEffectiveDate: string
}): RecordDatePlan {
  const selected = options.selectedEffectiveDate.trim()
  const dates = sortUniqueISODates(options.versions.map((v) => v.effective_date))
  const last = dates.length > 0 ? dates[dates.length - 1] ?? null : null

  const fallbackDefault = last ? addDays(last, 1) : null
  if (!isISODate(selected) || !last || !fallbackDefault) {
    return {
      kind: 'invalid_input',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate: fallbackDefault ?? selected,
      minDate: null,
      maxDate: null
    }
  }

  if (options.mode === 'add') {
    return {
      kind: 'add',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate: fallbackDefault,
      minDate: addDays(last, 1),
      maxDate: null
    }
  }

  const selectedIndex = dates.indexOf(selected)
  if (selectedIndex === -1) {
    return {
      kind: 'invalid_input',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate: fallbackDefault,
      minDate: null,
      maxDate: null
    }
  }

  const isLatest = selectedIndex === dates.length - 1
  if (isLatest) {
    return {
      kind: 'insert_as_add',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate: fallbackDefault,
      minDate: addDays(last, 1),
      maxDate: null
    }
  }

  const next = dates[selectedIndex + 1] ?? null
  if (!next) {
    return {
      kind: 'invalid_input',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate: fallbackDefault,
      minDate: null,
      maxDate: null
    }
  }

  const max = addDays(next, -1)
  if (!max) {
    return {
      kind: 'invalid_input',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate: fallbackDefault,
      minDate: null,
      maxDate: null
    }
  }

  let min: string | null = null
  if (selectedIndex > 0) {
    const prev = dates[selectedIndex - 1] ?? null
    min = prev ? addDays(prev, 1) : null
  } else {
    // Frozen policy (DEV-PLAN-101I): don't allow insert earlier than earliest version.
    min = addDays(selected, 1)
  }

  const defaultDate = addDays(selected, 1) ?? fallbackDefault
  if (!min) {
    return {
      kind: 'invalid_input',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate,
      minDate: null,
      maxDate: max
    }
  }

  if (min > max) {
    return {
      kind: 'insert_no_slot',
      selectedEffectiveDate: selected,
      lastEffectiveDate: last,
      defaultDate,
      minDate: min,
      maxDate: max
    }
  }

  return {
    kind: 'insert',
    selectedEffectiveDate: selected,
    lastEffectiveDate: last,
    defaultDate,
    minDate: min,
    maxDate: max
  }
}

export function validatePlannedEffectiveDate(input: {
  plan: RecordDatePlan
  effectiveDate: string
}): { ok: true } | { ok: false; reason: 'required' | 'invalid_format' | 'out_of_range' | 'no_slot' } {
  const value = input.effectiveDate.trim()
  if (value.length === 0) {
    return { ok: false, reason: 'required' }
  }
  if (!isISODate(value)) {
    return { ok: false, reason: 'invalid_format' }
  }

  const plan = input.plan
  if (plan.kind === 'invalid_input') {
    return { ok: false, reason: 'out_of_range' }
  }
  if (plan.kind === 'insert_no_slot') {
    return { ok: false, reason: 'no_slot' }
  }

  const min = plan.minDate
  const max = plan.maxDate
  if (min && value < min) {
    return { ok: false, reason: 'out_of_range' }
  }
  if (max && value > max) {
    return { ok: false, reason: 'out_of_range' }
  }

  // Disallow no-op insert on same selected day.
  if ((plan.kind === 'insert' || plan.kind === 'insert_as_add') && value === plan.selectedEffectiveDate) {
    return { ok: false, reason: 'out_of_range' }
  }

  return { ok: true }
}
