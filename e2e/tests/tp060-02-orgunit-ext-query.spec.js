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

async function createOrgUnit(ctx, { asOf, orgCode, name, parentOrgCode, isBusinessUnit }) {
  const resp = await ctx.request.post("/org/api/org-units", {
    data: {
      org_code: orgCode,
      name,
      effective_date: asOf,
      parent_org_code: parentOrgCode,
      is_business_unit: isBusinessUnit
    }
  });
  expect(resp.status(), await resp.text()).toBe(201);
}

async function enableOrgTypeFieldConfig(page, asOf) {
  await page.goto(`/app/org/units/field-configs?as_of=${asOf}`);
  await expect(page.getByRole("heading", { name: /Field Configs/ })).toBeVisible({ timeout: 60_000 });

  await page.getByRole("button", { name: /Enable Field/ }).first().click();
  const dialog = page.getByRole("dialog", { name: /Enable Field/ });
  await expect(dialog).toBeVisible();

  await dialog.getByLabel(/Field Key/).click();
  await page.getByRole("option", { name: /Org Type/ }).click();

  const enabledOnInput = dialog.getByLabel(/Enabled On/);
  await enabledOnInput.fill(asOf);

  await dialog.getByLabel(/Data Source Config/).click();
  await page.getByRole("option", { name: /dict_code/ }).click();

  await dialog.getByRole("button", { name: /Confirm/ }).click();
  await expect(page.getByText(/Enabled successfully/)).toBeVisible({ timeout: 30_000 });
}

async function waitForEnabledFieldConfig(ctx, { asOf, fieldKey }) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    const resp = await ctx.request.get(
      `/org/api/org-units/field-configs?as_of=${encodeURIComponent(asOf)}&status=enabled`
    );
    if (!resp.ok()) {
      throw new Error(`field configs load failed: ${resp.status()} ${await resp.text()}`);
    }
    const payload = await resp.json();
    const configs = Array.isArray(payload.field_configs) ? payload.field_configs : [];
    if (configs.some((cfg) => cfg.field_key === fieldKey)) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`field config not enabled in time: ${fieldKey}`);
}

async function setOrgTypeViaAPI(ctx, { asOf, orgCode, value }) {
  const resp = await ctx.request.post("/org/api/org-units/corrections", {
    data: {
      org_code: orgCode,
      effective_date: asOf,
      request_id: `req-${Date.now()}-${orgCode}`,
      patch: {
        ext: {
          org_type: value
        }
      }
    }
  });
  expect(resp.status(), await resp.text()).toBe(200);
}

test("tp060-02: orgunit list ext filter/sort (admin)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const runID = `${Date.now()}`;
  const suffix = runID.slice(-4);

  const tenantHost = `t-tp060-02-ext-${runID}.localhost`;
  const tenantName = `TP060-02 EXT Tenant ${runID}`;

  const tenantAdminEmail = `tenant-admin+100g-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp060-02-ext-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass }
  });
  const superadminPage = await superadminContext.newPage();

  await ensureKratosIdentity(superadminContext, kratosAdminURL, {
    traits: { email: superadminEmail },
    identifier: `sa:${superadminEmail.toLowerCase()}`,
    password: superadminLoginPass
  });

  await superadminPage.goto("/superadmin/login");
  await expect(superadminPage.locator("h1")).toHaveText("SuperAdmin Login");
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
  await appContext.addInitScript(() => {
    window.localStorage.setItem("web-mui-locale", "en");
  });
  const page = await appContext.newPage();

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);

  await page.goto(`/app?as_of=${asOf}`);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");

  const org = {
    root: `ROOT${suffix}`.toUpperCase(),
    company: `COMP${suffix}`.toUpperCase(),
    dept: `DEPT${suffix}`.toUpperCase()
  };

  await createOrgUnit(appContext, {
    asOf,
    orgCode: org.root,
    name: `TP100G Root ${runID}`,
    parentOrgCode: "",
    isBusinessUnit: true
  });
  await createOrgUnit(appContext, {
    asOf,
    orgCode: org.company,
    name: `TP100G Company ${runID}`,
    parentOrgCode: org.root,
    isBusinessUnit: false
  });
  await createOrgUnit(appContext, {
    asOf,
    orgCode: org.dept,
    name: `TP100G Dept ${runID}`,
    parentOrgCode: org.root,
    isBusinessUnit: false
  });

  await enableOrgTypeFieldConfig(page, asOf);
  await waitForEnabledFieldConfig(appContext, { asOf, fieldKey: "org_type" });

  await setOrgTypeViaAPI(appContext, { asOf, orgCode: org.company, value: "COMPANY" });
  await setOrgTypeViaAPI(appContext, { asOf, orgCode: org.dept, value: "DEPARTMENT" });

  await page.goto(`/app/org/units?as_of=${asOf}&node=${org.root}`);

  await page.getByLabel(/Sort by/).click();
  await page.getByRole("option", { name: /Org Type/ }).click();
  await page.getByLabel(/Sort order/).click();
  await page.getByRole("option", { name: /Ascending/ }).click();

  const applyButtons = page.getByRole("button", { name: /Apply Filters/ });
  await applyButtons.last().click();

  const companyRow = page.getByRole("row", { name: new RegExp(org.company) });
  const deptRow = page.getByRole("row", { name: new RegExp(org.dept) });
  await expect(companyRow).toBeVisible({ timeout: 30_000 });
  await expect(deptRow).toBeVisible({ timeout: 30_000 });

  const companyBox = await companyRow.boundingBox();
  const deptBox = await deptRow.boundingBox();
  if (!companyBox || !deptBox) {
    throw new Error("row bounding box missing");
  }
  expect(companyBox.y).toBeLessThan(deptBox.y);

  const extFilterField = page.getByLabel(/Ext Filter Field|扩展筛选字段/);
  await expect(extFilterField).toBeEnabled({ timeout: 30_000 });
  await extFilterField.click();
  await page.getByRole("option", { name: /Org Type/ }).click();

  const extValueInput = page.getByLabel(/Ext Filter Value|扩展筛选值/);
  await expect(extValueInput).toBeEnabled({ timeout: 30_000 });
  await extValueInput.click();
  await extValueInput.fill("Company");
  await page.getByRole("option", { name: "Company" }).click();

  await applyButtons.last().click();

  await expect(page.getByText(org.dept)).toHaveCount(0);
  await expect(page.getByText(org.company)).toBeVisible({ timeout: 30_000 });

  await page.getByRole("row", { name: new RegExp(org.company) }).click();
  await expect(page).toHaveURL(new RegExp(`/app/org/units/${org.company}`));
  await expect(page.getByText(/Org Type/)).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText("Company")).toBeVisible({ timeout: 30_000 });

  await appContext.close();
});
