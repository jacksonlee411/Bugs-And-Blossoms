import { expect, test } from "@playwright/test";

test("smoke: superadmin -> create tenant -> /login -> /app -> org/person/staffing vertical slice", async ({ browser }) => {
  const asOf = "2026-01-07";
  const runID = `${Date.now()}`;
  const tenantHost = `t-${runID}.localhost`;
  const tenantAdminEmail = "tenant-admin@example.invalid";
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const pernr = `${Math.floor(10000000 + Math.random() * 90000000)}`;
  const orgName = `E2E OrgUnit ${runID}`;
  const orgCode = `ORG${runID.slice(-6)}`;
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
  await expect(superadminPage.locator("tr", { hasText: tenantHost }).first()).toBeVisible({ timeout: 60000 });

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost });
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  const identifier = `${tenantID}:${tenantAdminEmail}`;
  const createIdentityResp = await superadminContext.request.post(`${kratosAdminURL}/admin/identities`, {
    data: {
      schema_id: "default",
      traits: { tenant_uuid: tenantID, email: tenantAdminEmail },
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
  await expect(page).toHaveURL(/\/app$/);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");

  await page.goto(`/org/nodes?tree_as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit Details");
  const nodeIDLocator = page.locator("sl-tree-item").first();
  const hasAnyNode = (await nodeIDLocator.count()) > 0;
  let parentCode = "";
  if (hasAnyNode) {
    const codeLocator = nodeIDLocator.locator(".org-node-code");
    if ((await codeLocator.count()) > 0) {
      parentCode = (await codeLocator.innerText()).trim();
    }
  }
  await page.locator(".org-node-create-btn").click();
  const orgCreateForm = page
    .locator(`#org-node-details form[method="POST"][action="/org/nodes?tree_as_of=${asOf}"]`)
    .filter({ has: page.locator('input[name="name"]') })
    .first();
  await expect(orgCreateForm).toBeVisible();
  const setBusinessUnitFlag = async (enabled) => {
    const input = orgCreateForm.locator('input[name="is_business_unit"]');
    if ((await input.count()) === 0) {
      if (enabled) {
        throw new Error("missing is_business_unit field in /org/nodes form");
      }
      return;
    }
    const inputType = (await input.first().getAttribute("type")) || "";
    if (inputType === "checkbox") {
      if (enabled) {
        await input.first().check();
      } else if (await input.first().isChecked()) {
        await input.first().uncheck();
      }
      return;
    }
    await input.first().fill(enabled ? "true" : "false");
  };
  await orgCreateForm.locator('input[name="org_code"]').fill(orgCode);
  if (parentCode) {
    await orgCreateForm.locator('input[name="parent_code"]').fill(parentCode);
  }
  await setBusinessUnitFlag(!parentCode);
  await orgCreateForm.locator('input[name="name"]').fill(orgName);
  await orgCreateForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/nodes\\?tree_as_of=${asOf}$`));
  await expect(page.locator("sl-tree-item", { hasText: orgName })).toBeVisible();
  const createdOrgCode = (await page.locator("sl-tree-item", { hasText: orgName }).first().locator(".org-node-code").innerText()).trim();
  expect(createdOrgCode).not.toBe("");
  if (!parentCode) {
    const bindResp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: createdOrgCode,
        setid: "DEFLT",
        effective_date: asOf,
        request_code: `smoke-bind-root-${runID}`
      }
    });
    expect(bindResp.status(), await bindResp.text()).toBe(201);
  }

  const jobFamilyGroupCode = `JFG-SM-${runID}`;
  const jobFamilyCode = `JF-SM-${runID}`;
  const jobProfileCode = `JP-SM-${runID}`;

  await page.goto(`/org/job-catalog?as_of=${asOf}&package_code=DEFLT`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");

  const ensureJobFamilyGroup = async (code, name) => {
    if ((await page.locator("tr", { hasText: code }).count()) > 0) {
      return;
    }
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="create_job_family_group"]')
    });
    await form.locator('input[name="job_family_group_code"]').fill(code);
    await form.locator('input[name="job_family_group_name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?(?=.*package_code=DEFLT)(?=.*as_of=${asOf}).*$`));
  };

  const ensureJobFamily = async (code, name, groupCode) => {
    if ((await page.locator("tr", { hasText: code }).count()) > 0) {
      return;
    }
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="create_job_family"]')
    });
    await form.locator('input[name="job_family_code"]').fill(code);
    await form.locator('input[name="job_family_name"]').fill(name);
    await form.locator('input[name="job_family_group_code"]').fill(groupCode);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?(?=.*package_code=DEFLT)(?=.*as_of=${asOf}).*$`));
  };

  const ensureJobProfile = async (code, name, familyCodesCSV, primaryFamilyCode) => {
    if ((await page.locator("tr", { hasText: code }).count()) > 0) {
      return;
    }
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="create_job_profile"]')
    });
    await form.locator('input[name="job_profile_code"]').fill(code);
    await form.locator('input[name="job_profile_name"]').fill(name);
    await form.locator('input[name="job_profile_family_codes"]').fill(familyCodesCSV);
    await form.locator('input[name="job_profile_primary_family_code"]').fill(primaryFamilyCode);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?(?=.*package_code=DEFLT)(?=.*as_of=${asOf}).*$`));
  };

  await ensureJobFamilyGroup(jobFamilyGroupCode, `Smoke Group ${runID}`);
  await ensureJobFamily(jobFamilyCode, `Smoke Family ${runID}`, jobFamilyGroupCode);
  await ensureJobProfile(jobProfileCode, `Smoke Profile ${runID}`, jobFamilyCode, jobFamilyCode);

  await page.goto(`/person/persons?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Person");
  await page.locator(`form[action="/person/persons?as_of=${asOf}"] input[name="pernr"]`).fill(pernr);
  await page.locator(`form[action="/person/persons?as_of=${asOf}"] input[name="display_name"]`).fill(`E2E Person ${runID}`);
  await page.locator(`form[action="/person/persons?as_of=${asOf}"] button[type="submit"]`).click();
  await expect(page).toHaveURL(new RegExp(`/person/persons\\?as_of=${asOf}$`));
  await expect(page.getByText(pernr)).toBeVisible();

  const personRow = page.locator("tr", { hasText: pernr }).first();
  const personUUID = (await personRow.locator("code").innerText()).trim();
  expect(personUUID).not.toBe("");

  await page.goto(`/org/positions?as_of=${asOf}&org_code=${createdOrgCode}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");
  const posCreateForm = page
    .locator(`form[method="POST"][action*="/org/positions"][action*="as_of=${asOf}"]`)
    .first();
  const orgUnitCode = createdOrgCode;
  const orgUnitHiddenValue = await posCreateForm.locator('input[name="org_code"]').getAttribute("value");
  expect(orgUnitHiddenValue).toBe(orgUnitCode);
  const jobProfileOption = posCreateForm.locator('select[name="job_profile_uuid"] option', { hasText: jobProfileCode }).first();
  const jobProfileID = await jobProfileOption.getAttribute("value");
  expect(jobProfileID).not.toBeNull();
  await posCreateForm.locator('select[name="job_profile_uuid"]').selectOption(jobProfileID);
  await posCreateForm.locator('input[name="name"]').fill(posName);
  await posCreateForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(
    new RegExp(`/org/positions\\?(?=.*as_of=${asOf})(?=.*org_code=${orgUnitCode}).*$`)
  );

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
    .locator('form[method="POST"] select[name="position_uuid"] option', { hasText: posName })
    .first()
    .getAttribute("value");
  expect(positionOption).not.toBeNull();
  await page.locator('form[method="POST"] select[name="position_uuid"]').selectOption(positionOption);
  await page.locator('form[method="POST"] button[type="submit"]').click();

  await expect(page).toHaveURL(new RegExp(`/org/assignments\\?as_of=${asOf}&pernr=${pernr}$`));
  await expect(page.locator("h2", { hasText: "Timeline" })).toBeVisible();
  await expect(page.locator("table")).toContainText(asOf);
  await expect(page.locator("table")).not.toContainText("end_date");

  await appContext.close();
});
