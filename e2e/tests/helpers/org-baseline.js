import { expect } from "@playwright/test"

function parseJSONSafe(raw) {
  const body = String(raw || "").trim()
  if (!body) {
    return null
  }
  try {
    return JSON.parse(body)
  } catch {
    return null
  }
}

async function parseResponseBody(response) {
  const text = await response.text()
  return { text, json: parseJSONSafe(text) }
}

export async function listOrgUnits(appContext, asOf) {
  const response = await appContext.request.get(`/org/api/org-units?as_of=${encodeURIComponent(asOf)}`)
  const { text, json } = await parseResponseBody(response)
  expect(response.status(), text).toBe(200)
  return Array.isArray(json?.org_units) ? json.org_units : []
}

export async function getOrgUnitDetails(appContext, orgCode, asOf) {
  const response = await appContext.request.get(
    `/org/api/org-units/details?as_of=${encodeURIComponent(asOf)}&org_code=${encodeURIComponent(orgCode)}`,
  )
  const { text, json } = await parseResponseBody(response)
  if (response.status() === 404) {
    return null
  }
  expect(response.status(), text).toBe(200)
  return json
}

export async function waitForOrgUnitDetails(appContext, orgCode, asOf, timeoutMs = 15_000) {
  const deadline = Date.now() + timeoutMs
  let details = null
  while (Date.now() < deadline) {
    details = await getOrgUnitDetails(appContext, orgCode, asOf)
    if (details?.org_unit) {
      return details
    }
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  return details
}

export async function createOrgUnit(appContext, payload) {
  const response = await appContext.request.post("/org/api/org-units", { data: payload })
  const { text, json } = await parseResponseBody(response)
  expect(response.status(), text).toBe(201)
  return json
}

export async function detectRootOrg(appContext, asOf, preferredRootCode = "ROOT") {
  const orgUnits = await listOrgUnits(appContext, asOf)
  const preferred = orgUnits.find((item) => String(item?.org_code || "").trim() === preferredRootCode)
  if (preferred) {
    const details = await getOrgUnitDetails(appContext, preferredRootCode, asOf)
    if (details?.org_unit && !String(details.org_unit.parent_org_code || "").trim()) {
      return details.org_unit
    }
  }
  for (const item of orgUnits) {
    const orgCode = String(item?.org_code || "").trim()
    if (!orgCode) {
      continue
    }
    const details = await getOrgUnitDetails(appContext, orgCode, asOf)
    if (details?.org_unit && !String(details.org_unit.parent_org_code || "").trim()) {
      return details.org_unit
    }
  }
  return null
}

export async function ensureOrgUnitByCode(
  appContext,
  spec,
  {
    effectiveDate,
    parentOrgCode,
    readAsOf = effectiveDate,
    lookupTimeoutMs = 500,
    createTimeoutMs = 15_000,
    isBusinessUnit = false,
    onCreated
  },
) {
  const existing = await waitForOrgUnitDetails(appContext, spec.code, readAsOf, lookupTimeoutMs)
  if (existing?.org_unit) {
    return existing.org_unit
  }

  await createOrgUnit(appContext, {
    org_code: spec.code,
    name: spec.name,
    effective_date: effectiveDate,
    parent_org_code: parentOrgCode,
    is_business_unit: isBusinessUnit
  })

  if (typeof onCreated === "function") {
    await onCreated({
      org_code: spec.code,
      name: spec.name,
      parent_org_code: parentOrgCode,
      effective_date: effectiveDate,
      is_business_unit: isBusinessUnit
    })
  }

  const created = await waitForOrgUnitDetails(appContext, spec.code, readAsOf, createTimeoutMs)
  expect(created?.org_unit, `org ${spec.code} should be readable after creation`).toBeTruthy()
  return created.org_unit
}

export function orgUnitDetailsSnapshot(details) {
  if (!details?.org_unit) {
    return null
  }
  return {
    org_code: String(details.org_unit.org_code || "").trim(),
    name: String(details.org_unit.name || "").trim(),
    parent_org_code: String(details.org_unit.parent_org_code || "").trim(),
    full_name_path: String(details.org_unit.full_name_path || "").trim(),
    status: String(details.org_unit.status || "").trim()
  }
}

export async function collectOrgDetailsBySpecs(appContext, specs, asOf, lookupTimeoutMs = 250) {
  const details = []
  for (const spec of specs) {
    const response = await waitForOrgUnitDetails(appContext, spec.code, asOf, lookupTimeoutMs)
    const snapshot = orgUnitDetailsSnapshot(response)
    if (snapshot) {
      details.push(snapshot)
    }
  }
  return details
}

export async function collectCandidateDetails(appContext, orgUnits, asOf) {
  const details = []
  for (const item of orgUnits) {
    const orgCode = String(item?.org_code || "").trim()
    if (!orgCode) {
      continue
    }
    const response = await getOrgUnitDetails(appContext, orgCode, asOf)
    details.push({
      org_code: orgCode,
      name: String(item?.name || "").trim(),
      full_name_path: String(response?.org_unit?.full_name_path || ""),
      parent_org_code: String(response?.org_unit?.parent_org_code || "")
    })
  }
  return details
}
