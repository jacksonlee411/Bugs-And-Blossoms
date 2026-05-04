import { expect, test } from "@playwright/test"

import { createIAMSession } from "./helpers/iam-session.js"
import { ensureKratosIdentity } from "./helpers/kratos-identity.js"
import { createOrgUnit, waitForOrgUnitDetails } from "./helpers/org-baseline.js"
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js"

const AS_OF = "2026-01-01"
const TENANT_ADMIN_PASS = process.env.E2E_TENANT_ADMIN_PASS || "pw"
const TENANT_VIEWER_PASS = process.env.E2E_TENANT_VIEWER_PASS || TENANT_ADMIN_PASS

async function responseJSON(response) {
  const text = await response.text()
  return { text, json: text.trim() ? JSON.parse(text) : null }
}

async function getPrincipalByEmail(adminContext, email) {
  const response = await adminContext.request.get("/iam/api/authz/user-assignments", {
    headers: { Accept: "application/json" }
  })
  const { text, json } = await responseJSON(response)
  expect(response.status(), text).toBe(200)
  const principal = (json?.principals || []).find((item) => item.email === email)
  expect(principal, `principal not found for ${email}; candidates=${JSON.stringify(json?.principals || [])}`).toBeTruthy()
  return principal
}

async function getAssignment(context, principalID) {
  const query = new URLSearchParams({ principal_id: principalID })
  const response = await context.request.get(`/iam/api/authz/user-assignments?${query.toString()}`, {
    headers: { Accept: "application/json" }
  })
  const { text, json } = await responseJSON(response)
  expect(response.status(), text).toBe(200)
  return json
}

async function getOrgUnitListItem(context, orgCode, parentOrgCode) {
  const query = new URLSearchParams({
    as_of: AS_OF,
    parent_org_code: parentOrgCode
  })
  const response = await context.request.get(`/org/api/org-units?${query.toString()}`, {
    headers: { Accept: "application/json" }
  })
  const { text, json } = await responseJSON(response)
  expect(response.status(), text).toBe(200)
  const item = (json?.org_units || []).find((candidate) => candidate.org_code === orgCode)
  expect(item, `org unit ${orgCode} not found under ${parentOrgCode}; candidates=${JSON.stringify(json?.org_units || [])}`).toBeTruthy()
  return item
}

async function replaceAssignment(context, principalID, roleSlug, orgCode) {
  const assignment = await getAssignment(context, principalID)
  const response = await context.request.put(`/iam/api/authz/user-assignments/${principalID}`, {
    headers: { Accept: "application/json" },
    data: {
      roles: [{ role_slug: roleSlug }],
      org_scopes: [{ org_code: orgCode, include_descendants: true }],
      revision: assignment.revision
    }
  })
  const { text, json } = await responseJSON(response)
  expect(response.status(), text).toBe(200)
  expect(json.org_scopes?.[0]?.org_code).toBe(orgCode)
  return json
}

async function expectVisibleSelectorOption(page, query, label) {
  await page.locator('[data-field="orgCode"] input[readonly]').last().click()
  const dialog = page.getByRole("dialog")
  await expect(dialog.getByRole("heading", { name: /Select Organization|选择组织/ })).toBeVisible()
  await dialog.getByRole("textbox").fill(query)
  await dialog.getByRole("button", { name: /Locate|定位|搜索/ }).click()
  await expect(page.getByText(label)).toBeVisible()
}

test("dev491: restricted authz admin selector is scoped and assignment save is fail-closed", async ({ browser }) => {
  test.setTimeout(180_000)

  const runID = String(Date.now())
  const runSuffix = runID.slice(-6).toUpperCase()
  const runMarker = `DEV491 ${runSuffix}`
  const tenantHost = `t-dev491-authz-${runID}.localhost`
  const tenantAdminEmail = `tenant-admin+dev491-${runID}@example.invalid`
  const restrictedAdminEmail = `restricted-admin+dev491-${runID}@example.invalid`
  const viewerEmail = `viewer+dev491-${runID}@example.invalid`

  const tenant = await setupTenantAdminSession(browser, {
    tenantName: `DEV491 Authz Selector ${runID}`,
    tenantHost,
    tenantAdminEmail,
    tenantAdminPass: TENANT_ADMIN_PASS,
    sessionLoginRetryTimeoutMs: 15_000
  })

  const adminContext = tenant.appContext
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434"
  const rootCode = `D491R${runSuffix}`
  const flowersCode = `D491F${runSuffix}`
  const flowersChildCode = `D491FC${runSuffix}`
  const bugsCode = `D491B${runSuffix}`

  await createOrgUnit(adminContext, {
    org_code: rootCode,
    name: `${runMarker} Root`,
    effective_date: AS_OF,
    is_business_unit: true
  })
  await waitForOrgUnitDetails(adminContext, rootCode, AS_OF, 15_000)

  const tenantAdminPrincipal = await getPrincipalByEmail(adminContext, tenantAdminEmail)
  await replaceAssignment(adminContext, tenantAdminPrincipal.principal_id, "tenant-admin", rootCode)

  await createOrgUnit(adminContext, {
    org_code: flowersCode,
    name: `${runMarker} Flowers`,
    effective_date: AS_OF,
    parent_org_code: rootCode,
    is_business_unit: true
  })
  await createOrgUnit(adminContext, {
    org_code: flowersChildCode,
    name: `${runMarker} Flowers Child`,
    effective_date: AS_OF,
    parent_org_code: flowersCode,
    is_business_unit: false
  })
  await createOrgUnit(adminContext, {
    org_code: bugsCode,
    name: `${runMarker} Bugs`,
    effective_date: AS_OF,
    parent_org_code: rootCode,
    is_business_unit: true
  })
  await waitForOrgUnitDetails(adminContext, flowersChildCode, AS_OF, 15_000)
  const flowersChildItem = await getOrgUnitListItem(adminContext, flowersChildCode, flowersCode)
  expect(flowersChildItem.org_node_key).toBeTruthy()

  await ensureKratosIdentity(adminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenant.tenantID, email: restrictedAdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenant.tenantID}:${restrictedAdminEmail}`,
    password: TENANT_ADMIN_PASS
  })
  await ensureKratosIdentity(adminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenant.tenantID, email: viewerEmail, role_slug: "tenant-viewer" },
    identifier: `${tenant.tenantID}:${viewerEmail}`,
    password: TENANT_VIEWER_PASS
  })

  const restrictedContext = await browser.newContext({
    baseURL: tenant.appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  })
  await createIAMSession(restrictedContext, restrictedAdminEmail, TENANT_ADMIN_PASS)
  const viewerContext = await browser.newContext({
    baseURL: tenant.appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  })
  await createIAMSession(viewerContext, viewerEmail, TENANT_VIEWER_PASS)
  await viewerContext.close()

  const restrictedAdminPrincipal = await getPrincipalByEmail(adminContext, restrictedAdminEmail)
  const viewerPrincipal = await getPrincipalByEmail(adminContext, viewerEmail)
  await replaceAssignment(adminContext, restrictedAdminPrincipal.principal_id, "tenant-admin", flowersCode)
  await replaceAssignment(adminContext, viewerPrincipal.principal_id, "tenant-viewer", flowersCode)

  const page = await restrictedContext.newPage()
  await page.goto("/app/authz/user-assignments")
  await expect(page).toHaveURL(/\/app\/authz\/user-assignments/)
  await expect(page.getByRole("heading", { name: /User Authorization|用户授权/ })).toBeVisible()
  await page.getByLabel(/User|用户/).click()
  await page.getByRole("option", { name: new RegExp(viewerEmail.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")) }).click()
  await page.getByRole("tab", { name: /Organization Scopes|组织范围/ }).click()
  await page.getByRole("button", { name: /Add Row|添加行/ }).click()

  await expectVisibleSelectorOption(page, flowersChildCode, `${runMarker} Flowers Child (${flowersChildCode})`)
  await expect(page.getByText(`${runMarker} Root (${rootCode})`)).toHaveCount(0)
  await page.getByRole("button", { name: /Confirm|确认/ }).click()
  await expect(page.locator('[data-field="orgCode"] input[readonly]').last()).toHaveValue(`${runMarker} Flowers Child (${flowersChildCode})`)
  await page.getByRole("button", { name: /Save|保存/ }).click()
  await expect(page.getByText(/Action completed|操作已完成/)).toBeVisible()

  const saved = await getAssignment(adminContext, viewerPrincipal.principal_id)
  expect(saved.org_scopes).toEqual(expect.arrayContaining([
    expect.objectContaining({
      org_code: flowersChildCode,
      org_node_key: flowersChildItem.org_node_key,
      include_descendants: true
    })
  ]))

  await page.locator('[data-field="orgCode"] input[readonly]').last().click()
  const dialog = page.getByRole("dialog")
  await dialog.getByRole("textbox").fill(bugsCode)
  await dialog.getByRole("button", { name: /Locate|定位|搜索/ }).click()
  await expect(page.getByText(`${runMarker} Bugs (${bugsCode})`)).toHaveCount(0)
  await dialog.getByRole("button", { name: /Cancel|取消/ }).click()

  const beforeDirectSave = await getAssignment(restrictedContext, viewerPrincipal.principal_id)
  const directSave = await restrictedContext.request.put(`/iam/api/authz/user-assignments/${viewerPrincipal.principal_id}`, {
    headers: { Accept: "application/json" },
    data: {
      roles: [{ role_slug: "tenant-viewer" }],
      org_scopes: [{ org_code: bugsCode, include_descendants: true }],
      revision: beforeDirectSave.revision
    }
  })
  const { text: directBody, json: directJSON } = await responseJSON(directSave)
  expect(directSave.status(), directBody).toBe(403)
  expect(directJSON.code).toBe("authz_scope_forbidden")

  await page.close()
  await restrictedContext.close()
  await adminContext.close()
})
