import { expect, test } from "@playwright/test";

async function ensureKratosIdentity(ctx, kratosAdminURL, { traits, identifier, password }) {
  const resp = await ctx.request.post(`${kratosAdminURL}/admin/identities`, {
    data: {
      schema_id: "default",
      traits,
      credentials: {
        password: {
          identifiers: [identifier],
          config: { password }
        }
      }
    }
  });
  if (!resp.ok()) {
    expect(resp.status(), `unexpected status: ${resp.status()} (${await resp.text()})`).toBe(409);
  }
}

async function setupTenantAdminSession(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  const tenantHost = `t-tp283-${runID}.localhost`;
  const tenantName = `TP283 Tenant ${runID}`;
  const tenantAdminEmail = `tenant-admin+tp283-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp283-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass }
  });
  const superadminPage = await superadminContext.newPage();

  if (!process.env.E2E_SUPERADMIN_EMAIL) {
    await ensureKratosIdentity(superadminContext, kratosAdminURL, {
      traits: { email: superadminEmail },
      identifier: `sa:${superadminEmail.toLowerCase()}`,
      password: superadminLoginPass
    });
  }

  await superadminPage.goto("/superadmin/login");
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);

  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(tenantName);
  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost);
  await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.locator("tr", { hasText: tenantHost }).first()).toBeVisible({ timeout: 60_000 });

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost }).first();
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  await ensureKratosIdentity(superadminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenantID, email: tenantAdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenantID}:${tenantAdminEmail}`,
    password: tenantAdminPass
  });
  await superadminContext.close();

  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const appContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);

  return { appBaseURL, tenantHost, appContext };
}

test("tp283-e2e-001: formal entry is the only accepted chat entry", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext } = await setupTenantAdminSession(browser, "001");

  const page = await appContext.newPage();
  await page.goto("/app/assistant/librechat");
  await expect(page).toHaveTitle(/LibreChat/i);

  const aliasResp = await appContext.request.get("/assistant-ui", { maxRedirects: 0 });
  expect(aliasResp.status()).toBe(302);
  expect(aliasResp.headers()["location"]).toBe("/app/assistant/librechat");

  const aliasPathResp = await appContext.request.get("/assistant-ui/some/path", { maxRedirects: 0 });
  expect(aliasPathResp.status()).toBe(302);
  expect(aliasPathResp.headers()["location"]).toBe("/app/assistant/librechat");

  const aliasWriteResp = await appContext.request.post("/assistant-ui", { data: {} });
  expect(aliasWriteResp.status()).toBe(405);

  await appContext.close();
});

test("tp283-e2e-002: formal static prefix is protected by session boundary", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appBaseURL, tenantHost, appContext } = await setupTenantAdminSession(browser, "002");

  const authedStaticResp = await appContext.request.get("/assets/librechat-web/registerSW.js");
  expect(authedStaticResp.status()).toBe(200);
  expect(await authedStaticResp.text()).toContain("serviceWorker.register");

  const anonContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const anonStaticResp = await anonContext.request.get("/assets/librechat-web/registerSW.js", { maxRedirects: 0 });
  expect(anonStaticResp.status()).toBe(302);
  expect(anonStaticResp.headers()["location"]).toBe("/app/login");

  await anonContext.close();
  await appContext.close();
});
