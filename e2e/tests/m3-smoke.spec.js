import { expect, test } from "@playwright/test";

test("smoke: superadmin -> create tenant -> /login -> /app -> org/person/staffing vertical slice", async ({ browser }) => {
  const asOf = "2026-01-07";
  const runID = `${Date.now()}`;
  const tenantHost = `t-${runID}.localhost`;
  const tenantAdminEmail = "tenant-admin@example.invalid";
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const pernr = `${Math.floor(10000000 + Math.random() * 90000000)}`;
  const orgName = `E2E OrgUnit ${runID}`;
  const posName = `E2E Position ${runID}`;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const defaultSuperadminEmail = `admin+${runID}@example.invalid`;
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || defaultSuperadminEmail;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass }
  });
  const superadminPage = await superadminContext.newPage();

  const superadminIdentifier = `sa:${superadminEmail.toLowerCase()}`;
  const createSuperadminIdentityResp = await superadminContext.request.post(`${kratosAdminURL}/admin/identities`, {
    data: {
      schema_id: "default",
      traits: { email: superadminEmail },
      credentials: {
        password: {
          identifiers: [superadminIdentifier],
          config: { password: superadminLoginPass }
        }
      }
    }
  });
  expect(createSuperadminIdentityResp.ok()).toBeTruthy();

  await superadminPage.goto("/superadmin/login");
  await expect(superadminPage.locator("h1")).toHaveText("SuperAdmin Login");
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.locator("h1")).toHaveText("SuperAdmin / Tenants");

  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(`E2E Tenant ${runID}`);
  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost);
  await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.getByText(tenantHost)).toBeVisible();

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost });
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  const identifier = `${tenantID}:${tenantAdminEmail}`;
  const createIdentityResp = await superadminContext.request.post(`${kratosAdminURL}/admin/identities`, {
    data: {
      schema_id: "default",
      traits: { tenant_id: tenantID, email: tenantAdminEmail },
      credentials: {
        password: {
          identifiers: [identifier],
          config: { password: tenantAdminPass }
        }
      }
    }
  });
  expect(createIdentityResp.ok()).toBeTruthy();

  await superadminContext.close();

  const appContext = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL || "http://localhost:8080",
    extraHTTPHeaders: {
      "X-Forwarded-Host": tenantHost
    }
  });
  const page = await appContext.newPage();

  await page.goto("/login");
  await expect(page.locator("h1")).toHaveText("Login");

  await page.locator('input[name="email"]').fill(tenantAdminEmail);
  await page.locator('input[name="password"]').fill(tenantAdminPass);
  await page.getByRole("button", { name: "Login" }).click();
  await expect(page).toHaveURL(/\/app\?as_of=\d{4}-\d{2}-\d{2}$/);
  await expect(page.locator("h1")).toHaveText("Home");

  await page.goto(`/org/nodes?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit");
  const nodeIDLocator = page.locator("ul li code").first();
  const hasAnyNode = (await nodeIDLocator.count()) > 0;
  const parentID = hasAnyNode ? (await nodeIDLocator.innerText()).trim() : "";
  const orgCreateForm = page
    .locator(`form[method="POST"][action="/org/nodes?as_of=${asOf}"]`)
    .filter({ has: page.locator('input[name="name"]') })
    .first();
  if (parentID) {
    await orgCreateForm.locator('input[name="parent_id"]').fill(parentID);
  }
  await orgCreateForm.locator('input[name="name"]').fill(orgName);
  await orgCreateForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/nodes\\?as_of=${asOf}$`));
  await expect(page.locator("ul li", { hasText: orgName })).toBeVisible();

  await page.goto(`/person/persons?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Person");
  await page.locator(`form[action="/person/persons?as_of=${asOf}"] input[name="pernr"]`).fill(pernr);
  await page.locator(`form[action="/person/persons?as_of=${asOf}"] input[name="display_name"]`).fill(`E2E Person ${runID}`);
  await page.locator(`form[action="/person/persons?as_of=${asOf}"] button[type="submit"]`).click();
  await expect(page).toHaveURL(new RegExp(`/person/persons\\?as_of=${asOf}$`));
  await expect(page.getByText(pernr)).toBeVisible();

  await page.goto(`/org/positions?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");
  const orgUnitID = await page
    .locator('form[method="POST"] select[name="org_unit_id"] option', { hasText: orgName })
    .first()
    .getAttribute("value");
  expect(orgUnitID).not.toBeNull();
  await page.locator('form[method="POST"] select[name="org_unit_id"]').selectOption(orgUnitID);
  await page.locator('form[method="POST"] input[name="name"]').fill(posName);
  await page.locator('form[method="POST"] button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${asOf}$`));

  const posRow = page.locator("tr", { hasText: posName });
  await expect(posRow).toBeVisible();
  const positionID = (await posRow.locator("td").nth(1).innerText()).trim();
  await expect(positionID).not.toBe("");

  await page.goto(`/org/assignments?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Assignments");
  const pernrLoadForm = page.locator('form[method="GET"]', {
    has: page.locator('input[name="pernr"]')
  });
  await pernrLoadForm.locator('input[name="pernr"]').fill(pernr);
  await pernrLoadForm.getByRole("button", { name: "Load" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/assignments\\?as_of=${asOf}&pernr=${pernr}$`));

  const positionOption = await page
    .locator('form[method="POST"] select[name="position_id"] option', { hasText: posName })
    .first()
    .getAttribute("value");
  expect(positionOption).not.toBeNull();
  await page.locator('form[method="POST"] select[name="position_id"]').selectOption(positionOption);
  await page.locator('form[method="POST"] button[type="submit"]').click();

  await expect(page).toHaveURL(new RegExp(`/org/assignments\\?as_of=${asOf}&pernr=${pernr}$`));
  await expect(page.locator("h2", { hasText: "Timeline" })).toBeVisible();
  await expect(page.locator("table")).toContainText(asOf);
  await expect(page.locator("table")).not.toContainText("end_date");

  await appContext.close();
});
