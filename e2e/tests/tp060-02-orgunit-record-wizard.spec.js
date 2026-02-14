import { expect, test } from "@playwright/test";

const addDays = (dateValue, days) => {
  const [year, month, day] = String(dateValue).split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  date.setUTCDate(date.getUTCDate() + days);
  const y = String(date.getUTCFullYear());
  const m = String(date.getUTCMonth() + 1).padStart(2, "0");
  const d = String(date.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
};

test("tp060-02: orgunit record wizard (4-step add/delete flow)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const secondEffectiveDate = addDays(asOf, 1);
  const runID = `${Date.now()}`;

  const tenantHost = `t-tp060-02-wizard-${runID}.localhost`;
  const tenantName = `TP060-02 Wizard Tenant ${runID}`;

  const orgCode = `WZ${runID.slice(-6)}`.toUpperCase();
  const orgName = `Wizard Root ${runID}`;
  const orgNameV2 = `${orgName} V2`;

  const tenantAdminEmail = "tenant-admin@example.invalid";
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp060-02-wizard-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const ensureIdentity = async (ctx, identifier, email, password, traits) => {
    const resp = await ctx.request.post(`${kratosAdminURL}/admin/identities`, {
      data: {
        schema_id: "default",
        traits: { email, ...traits },
        credentials: { password: { identifiers: [identifier], config: { password } } }
      }
    });
    if (!resp.ok()) {
      expect(resp.status(), `unexpected status: ${resp.status()} (${await resp.text()})`).toBe(409);
    }
  };

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass }
  });
  const superadminPage = await superadminContext.newPage();

  await ensureIdentity(superadminContext, `sa:${superadminEmail.toLowerCase()}`, superadminEmail, superadminLoginPass, {});

  await superadminPage.goto("/superadmin/login");
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);

  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(tenantName);
  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost);
  await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.getByText(tenantHost)).toBeVisible({ timeout: 60_000 });

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost }).first();
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  await ensureIdentity(
    superadminContext,
    `${tenantID}:${tenantAdminEmail}`,
    tenantAdminEmail,
    tenantAdminPass,
    { tenant_uuid: tenantID }
  );

  await superadminContext.close();

  const appContext = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL || "http://localhost:8080",
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const page = await appContext.newPage();

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);

  const createResp = await appContext.request.post("/org/api/org-units", {
    data: {
      org_code: orgCode,
      name: orgName,
      effective_date: asOf,
      is_business_unit: true
    }
  });
  expect(createResp.status(), await createResp.text()).toBe(201);

  const renameResp = await appContext.request.post("/org/api/org-units/rename", {
    data: {
      org_code: orgCode,
      new_name: orgNameV2,
      effective_date: secondEffectiveDate
    }
  });
  expect(renameResp.ok(), await renameResp.text()).toBeTruthy();

  await page.goto(`/app/org/units/${orgCode}?as_of=${asOf}&effective_date=${asOf}`);
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${orgCode}\\?(?=.*effective_date=${asOf}).*$`));
  await expect(page.getByRole("heading", { level: 2, name: new RegExp(orgName) })).toBeVisible({ timeout: 30_000 });

  const secondVersionItem = page.locator(`[data-testid="org-version-${secondEffectiveDate}"]`);
  await expect(secondVersionItem).toBeVisible({ timeout: 30_000 });
  await secondVersionItem.click();
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${orgCode}\\?(?=.*effective_date=${secondEffectiveDate}).*$`));
  await expect(page.getByRole("heading", { level: 2, name: new RegExp(orgNameV2) })).toBeVisible({ timeout: 30_000 });

  await page.reload();
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${orgCode}\\?(?=.*effective_date=${secondEffectiveDate}).*$`));
  await expect(secondVersionItem).toHaveClass(/Mui-selected/);

  await appContext.close();
});

