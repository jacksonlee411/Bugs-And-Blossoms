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

test("tp060-04: orgunit details two-pane profile/audit URL restore", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-10";
  const secondEffectiveDate = addDays(asOf, 1);
  const runID = `${Date.now()}`;
  const tenantHost = `t-tp060-04-${runID}.localhost`;
  const tenantName = `TP060-04 Tenant ${runID}`;
  const orgCode = `TP04${runID.slice(-6)}`;
  const orgName = `TP060-04 Root ${runID}`;
  const orgNameV2 = `${orgName} V2`;

  const tenantAdminEmail = "tenant-admin@example.invalid";
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp060-04-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const ensureIdentity = async (ctx, identifier, email, password, traits) => {
    const resp = await ctx.request.post(`${kratosAdminURL}/admin/identities`, {
      data: {
        schema_id: "default",
        traits: { email, ...traits },
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

  // /app 现为 MUI SPA：只做最小可见性断言，避免依赖旧 HTML login UI。
  await page.goto(`/app?as_of=${asOf}`);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");

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
  expect(renameResp.status(), await renameResp.text()).toBe(200);

  const auditResp = await appContext.request.get(
    `/org/api/org-units/audit?org_code=${encodeURIComponent(orgCode)}&limit=20`
  );
  expect(auditResp.status(), await auditResp.text()).toBe(200);
  const auditData = await auditResp.json();
  const auditEvents = Array.isArray(auditData.events) ? auditData.events : [];
  expect(auditEvents.length).toBeGreaterThan(0);
  const targetAuditEvent = auditEvents.length > 1 ? auditEvents[1] : auditEvents[0];

  await page.goto(`/app/org/units/${orgCode}?as_of=${asOf}&effective_date=${asOf}`);
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${orgCode}\\?(?=.*effective_date=${asOf}).*$`));
  await expect(page.getByRole("heading", { level: 2, name: new RegExp(orgName) })).toBeVisible({ timeout: 30_000 });

  const secondVersionItem = page.locator(`[data-testid="org-version-${secondEffectiveDate}"]`);
  await expect(secondVersionItem).toBeVisible({ timeout: 30_000 });
  await secondVersionItem.click();
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${orgCode}\\?(?=.*effective_date=${secondEffectiveDate}).*$`));
  await expect(page.getByRole("heading", { level: 2, name: new RegExp(orgNameV2) })).toBeVisible({ timeout: 30_000 });

  await page.getByRole("tab", { name: /Change Log|变更日志/ }).click();
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${orgCode}\\?(?=.*tab=audit).*$`));

  const targetAuditItem = page.locator(`[data-testid="org-audit-${targetAuditEvent.event_uuid}"]`);
  await expect(targetAuditItem).toBeVisible({ timeout: 30_000 });
  await targetAuditItem.click();
  await expect(page).toHaveURL(
    new RegExp(`/app/org/units/${orgCode}\\?(?=.*tab=audit)(?=.*audit_event_uuid=${targetAuditEvent.event_uuid}).*$`)
  );
  await expect(page.locator('[data-testid="org-context-summary"]')).toHaveCount(0);
  await expect(targetAuditItem).toHaveClass(/Mui-selected/);
  await expect(page.locator('[data-testid="org-audit-selected-event"]')).toBeVisible({ timeout: 30_000 });
  await expect(page.locator('[data-testid="org-audit-selected-event-uuid"]')).toHaveText(targetAuditEvent.event_uuid);

  await page.reload();
  await expect(page).toHaveURL(
    new RegExp(`/app/org/units/${orgCode}\\?(?=.*tab=audit)(?=.*audit_event_uuid=${targetAuditEvent.event_uuid}).*$`)
  );
  await expect(targetAuditItem).toHaveClass(/Mui-selected/);
  await expect(page.locator('[data-testid="org-audit-selected-event"]')).toBeVisible({ timeout: 30_000 });
  await expect(page.locator('[data-testid="org-audit-selected-event-uuid"]')).toHaveText(targetAuditEvent.event_uuid);

  await appContext.close();
});
