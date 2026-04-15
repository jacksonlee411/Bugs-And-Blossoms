import { expect } from "@playwright/test"

import { createIAMSession, createIAMSessionWithRetry } from "./iam-session.js"
import { ensureKratosIdentity } from "./kratos-identity.js"

function e2eAuthConfig() {
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin"
  return {
    superadminBaseURL: process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081",
    superadminUser: process.env.E2E_SUPERADMIN_USER || "admin",
    superadminPass,
    superadminLoginPass: process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass,
    kratosAdminURL: process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434",
    appBaseURL: process.env.E2E_BASE_URL || "http://localhost:8080"
  }
}

function isIgnorableCloseError(error) {
  const message = String(error || "").toLowerCase()
  return message.includes("enoent") || message.includes("step id not found")
}

async function closeContext(context, closeWith) {
  if (!context) {
    return
  }
  if (typeof closeWith === "function") {
    await closeWith(context)
    return
  }
  try {
    await context.close()
  } catch (error) {
    if (!isIgnorableCloseError(error)) {
      throw error
    }
  }
}

export async function loginSuperadmin(page, { email, password, headingText } = {}) {
  await page.goto("/superadmin/login")
  if (typeof headingText === "string" && headingText.length > 0) {
    await expect(page.locator("h1")).toHaveText(headingText)
  }
  await page.locator('input[name="email"]').fill(email)
  await page.locator('input[name="password"]').fill(password)
  await page.getByRole("button", { name: "Login" }).click()
  await expect(page).toHaveURL(/\/superadmin\/tenants$/)
}

export async function createTenantAndGetID(page, { tenantName, tenantHost, timeoutMs = 60_000 }) {
  await page.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(tenantName)
  await page.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost)
  await page.locator('form[action="/superadmin/tenants"] button[type="submit"]').click()
  await expect(page).toHaveURL(/\/superadmin\/tenants$/)

  const tenantRow = page.locator("tr", { hasText: tenantHost }).first()
  await expect(tenantRow).toBeVisible({ timeout: timeoutMs })

  const tenantID = (await tenantRow.locator("code").first().innerText()).replace(/\s+/g, "").trim()
  expect(tenantID).not.toBe("")
  return tenantID
}

export async function setupTenantAdminSession(
  browser,
  {
    tenantName,
    tenantHost,
    tenantAdminEmail,
    tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw",
    superadminEmail,
    superadminLoginPass,
    superadminHeadingText,
    closeSuperadminContextWith,
    createPage = false,
    appContextOptions = {},
    sessionLoginRetryTimeoutMs = 0
  },
) {
  const auth = e2eAuthConfig()
  const resolvedSuperadminEmail =
    superadminEmail || process.env.E2E_SUPERADMIN_EMAIL || `admin+${Date.now()}@example.invalid`
  const resolvedSuperadminLoginPass = superadminLoginPass || auth.superadminLoginPass

  let tenantID = ""
  const superadminContext = await browser.newContext({
    baseURL: auth.superadminBaseURL,
    httpCredentials: { username: auth.superadminUser, password: auth.superadminPass }
  })

  try {
    const superadminPage = await superadminContext.newPage()

    if (!process.env.E2E_SUPERADMIN_EMAIL) {
      await ensureKratosIdentity(superadminContext, auth.kratosAdminURL, {
        traits: { email: resolvedSuperadminEmail },
        identifier: `sa:${resolvedSuperadminEmail.toLowerCase()}`,
        password: resolvedSuperadminLoginPass
      })
    }

    await loginSuperadmin(superadminPage, {
      email: resolvedSuperadminEmail,
      password: resolvedSuperadminLoginPass,
      headingText: superadminHeadingText
    })
    tenantID = await createTenantAndGetID(superadminPage, { tenantName, tenantHost })
    await ensureKratosIdentity(superadminContext, auth.kratosAdminURL, {
      traits: { tenant_uuid: tenantID, email: tenantAdminEmail, role_slug: "tenant-admin" },
      identifier: `${tenantID}:${tenantAdminEmail}`,
      password: tenantAdminPass
    })
  } finally {
    await closeContext(superadminContext, closeSuperadminContextWith)
  }

  const appContext = await browser.newContext({
    baseURL: auth.appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost },
    ...appContextOptions
  })
  if (sessionLoginRetryTimeoutMs > 0) {
    await createIAMSessionWithRetry(appContext, tenantAdminEmail, tenantAdminPass, sessionLoginRetryTimeoutMs)
  } else {
    await createIAMSession(appContext, tenantAdminEmail, tenantAdminPass)
  }

  const result = {
    appBaseURL: auth.appBaseURL,
    tenantHost,
    tenantID,
    tenantAdminEmail,
    tenantAdminPass,
    appContext
  }
  if (createPage) {
    result.page = await appContext.newPage()
  }
  return result
}
