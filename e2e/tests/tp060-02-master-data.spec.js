import { expect, test } from "@playwright/test";

test("tp060-02: master data (orgunit -> setid -> jobcatalog -> positions)", async ({ browser }) => {
  const asOf = "2026-01-01";
  const runID = `${Date.now()}`;
  const tenantHost = `t-tp060-02-${runID}.localhost`;
  const tenantName = `TP060-02 Tenant ${runID}`;

  const tenantAdminEmail = "tenant-admin@example.invalid";
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const defaultSuperadminEmail = `admin+tp060-02-${runID}@example.invalid`;
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || defaultSuperadminEmail;
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
  await expect(superadminPage.locator("h1")).toHaveText("SuperAdmin Login");
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);

  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(tenantName);
  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost);
  await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.getByText(tenantHost)).toBeVisible({ timeout: 15000 });

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost }).first();
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  await ensureIdentity(
    superadminContext,
    `${tenantID}:${tenantAdminEmail}`,
    tenantAdminEmail,
    tenantAdminPass,
    { tenant_id: tenantID }
  );
  await superadminContext.close();

  const appContext = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL || "http://localhost:8080",
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const page = await appContext.newPage();

  await page.goto("/login");
  await expect(page.locator("h1")).toHaveText("Login");
  await page.locator('input[name="email"]').fill(tenantAdminEmail);
  await page.locator('input[name="password"]').fill(tenantAdminPass);
  await page.getByRole("button", { name: "Login" }).click();
  await expect(page).toHaveURL(/\/app\?as_of=\d{4}-\d{2}-\d{2}$/);

  const findOrgUnitID = async (name) => {
    const li = page.locator("ul li", { hasText: name }).first();
    if ((await li.count()) === 0) {
      return "";
    }
    return ((await li.locator("code").first().innerText()) || "").trim();
  };

  const createOrgUnit = async (effectiveDate, parentID, name) => {
    const form = page.locator(`form[method="POST"][action="/org/nodes?as_of=${asOf}"]`).first();
    await form.locator('input[name="effective_date"]').fill(effectiveDate);
    await form.locator('input[name="parent_id"]').fill(parentID);
    await form.locator('input[name="name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/nodes\\?as_of=${asOf}$`));
  };

  await page.goto(`/org/nodes?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit");

  const rootName = "Bugs & Blossoms Co., Ltd.";
  let rootID = await findOrgUnitID(rootName);
  if (!rootID) {
    await createOrgUnit(asOf, "", rootName);
    rootID = await findOrgUnitID(rootName);
  }
  expect(rootID).not.toBe("");

  const level1 = ["HQ", "R&D", "Sales", "Ops", "Plant"];
  for (const name of level1) {
    const id = await findOrgUnitID(name);
    if (id) {
      continue;
    }
    await createOrgUnit(asOf, rootID, name);
    expect(await findOrgUnitID(name)).not.toBe("");
  }

  const emptyNameForm = page.locator(`form[method="POST"][action="/org/nodes?as_of=${asOf}"]`).first();
  await emptyNameForm.locator('input[name="effective_date"]').fill(asOf);
  await emptyNameForm.locator('input[name="parent_id"]').fill(rootID);
  await emptyNameForm.locator('input[name="name"]').fill("");
  await emptyNameForm.locator('button[type="submit"]').click();
  await expect(page.getByText("name is required")).toBeVisible();

  const createSetID = async (setid, name) => {
    const form = page.locator(`form[method="POST"][action="/org/setid?as_of=${asOf}"]`).filter({
      has: page.locator('input[name="setid"]')
    });
    await form.locator('input[name="setid"]').fill(setid);
    await form.locator('input[name="name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/setid\\?as_of=${asOf}$`));
  };

  const createBU = async (bu, name) => {
    const form = page.locator(`form[method="POST"][action="/org/setid?as_of=${asOf}"]`).filter({
      has: page.locator('input[name="business_unit_id"]')
    });
    await form.locator('input[name="business_unit_id"]').fill(bu);
    await form.locator('input[name="name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/setid\\?as_of=${asOf}$`));
  };

  await page.goto(`/org/setid?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("SetID Governance");
  await expect(page.getByRole("heading", { name: "SetIDs" })).toBeVisible();
  const setidsTable = page
    .locator('h2:has-text("SetIDs")')
    .locator("xpath=following-sibling::table[1]");
  await expect(setidsTable).toContainText("SHARE");

  if ((await page.locator("tr", { hasText: "S2601" }).count()) === 0) {
    await createSetID("S2601", "SetID S2601");
  }
  if ((await page.locator("tr", { hasText: "BU000" }).count()) === 0) {
    await createBU("BU000", "BU000");
  }
  if ((await page.locator("tr", { hasText: "BU901" }).count()) === 0) {
    await createBU("BU901", "BU901");
  }

  const mappingsForm = page.locator(`form[method="POST"][action="/org/setid?as_of=${asOf}"]`).filter({
    has: page.getByRole("button", { name: "Save Mappings" })
  });
  await mappingsForm.locator('select[name="map_BU000"]').selectOption("SHARE");
  await mappingsForm.locator('select[name="map_BU901"]').selectOption("S2601");
  await mappingsForm.getByRole("button", { name: "Save Mappings" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/setid\\?as_of=${asOf}$`));

  const asOfJobCatalogBase = asOf;
  const asOfJobCatalogBeforeReparent = "2026-01-15";
  const asOfJobCatalogAfterReparent = "2026-02-15";
  const reparentEffectiveDate = "2026-02-01";
  const beforeJobCatalogExists = "2025-12-31";

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&business_unit_id=BU901`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Resolved SetID:")).toBeVisible();
  await expect(page.getByText("Resolved SetID:")).toContainText("S2601");

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
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?business_unit_id=BU901&as_of=${asOfJobCatalogBase}$`));
  };
  await ensureJobFamilyGroup("JFG-ENG", "Engineering");
  await ensureJobFamilyGroup("JFG-SALES", "Sales");
  const groupsTable = page
    .locator('h2:has-text("Job Family Groups")')
    .locator("xpath=following-sibling::table[1]");
  await expect(groupsTable).toContainText("JFG-ENG");
  await expect(groupsTable).toContainText("JFG-SALES");

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
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?business_unit_id=BU901&as_of=${asOfJobCatalogBase}$`));
  };
  await ensureJobFamily("JF-BE", "Backend", "JFG-ENG");
  await ensureJobFamily("JF-FE", "Frontend", "JFG-ENG");
  const familiesTable = page
    .locator('h2:has-text("Job Families")')
    .locator("xpath=following-sibling::table[1]");
  await expect(familiesTable).toContainText("JF-BE");
  await expect(familiesTable).toContainText("JF-FE");
  await expect(familiesTable.locator("tr", { hasText: "JF-BE" })).toContainText("JFG-ENG");

  {
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="update_job_family_group"]')
    });
    await form.locator('input[name="effective_date"]').fill(reparentEffectiveDate);
    await form.locator('input[name="job_family_code"]').fill("JF-BE");
    await form.locator('input[name="job_family_group_code"]').fill("JFG-SALES");
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?business_unit_id=BU901&as_of=${reparentEffectiveDate}$`));
  }
  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBeforeReparent}&business_unit_id=BU901`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Resolved SetID:")).toContainText("S2601");
  await expect(
    page.locator('h2:has-text("Job Families")').locator("xpath=following-sibling::table[1]").locator("tr", { hasText: "JF-BE" })
  ).toContainText("JFG-ENG");

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogAfterReparent}&business_unit_id=BU901`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Resolved SetID:")).toContainText("S2601");
  await expect(
    page.locator('h2:has-text("Job Families")').locator("xpath=following-sibling::table[1]").locator("tr", { hasText: "JF-BE" })
  ).toContainText("JFG-SALES");

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&business_unit_id=BU901`);
  const ensureJobLevel = async (code, name) => {
    if ((await page.locator("tr", { hasText: code }).count()) > 0) {
      return;
    }
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="create_job_level"]')
    });
    await form.locator('input[name="job_level_code"]').fill(code);
    await form.locator('input[name="job_level_name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?business_unit_id=BU901&as_of=${asOfJobCatalogBase}$`));
  };
  await ensureJobLevel("JL-1", "Level 1");
  const levelsTable = page
    .locator('h2:has-text("Job Levels")')
    .locator("xpath=following-sibling::table[1]");
  await expect(levelsTable).toContainText("JL-1");

  await page.goto(`/org/job-catalog?as_of=${beforeJobCatalogExists}&business_unit_id=BU901`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  const levelsTableBefore = page
    .locator('h2:has-text("Job Levels")')
    .locator("xpath=following-sibling::table[1]");
  await expect(levelsTableBefore.locator("tr", { hasText: "JL-1" })).toHaveCount(0);

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&business_unit_id=BU901`);
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
    await expect(page).toHaveURL(new RegExp(`/org/job-catalog\\?business_unit_id=BU901&as_of=${asOfJobCatalogBase}$`));
  };
  await ensureJobProfile("JP-SWE", "Software Engineer", "JF-BE,JF-FE", "JF-BE");
  const profilesTable = page
    .locator('h2:has-text("Job Profiles")')
    .locator("xpath=following-sibling::table[1]");
  await expect(profilesTable).toContainText("JP-SWE");
  await expect(profilesTable).toContainText("JF-BE,JF-FE");
  await expect(profilesTable.locator("tr", { hasText: "JP-SWE" })).toContainText("JF-BE");

  {
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="create_job_profile"]')
    });
    await form.locator('input[name="job_profile_code"]').fill("JP-BAD");
    await form.locator('input[name="job_profile_name"]').fill("Bad Profile");
    await form.locator('input[name="job_profile_family_codes"]').fill("JF-BE");
    await form.locator('input[name="job_profile_primary_family_code"]').fill("JF-FE");
    await form.locator('button[type="submit"]').click();
    await expect(page.locator('div[style*="border:1px solid #c00"]')).toBeVisible();
  }

  await page.goto(`/org/setid?as_of=${asOf}`);
  if ((await page.locator("tr", { hasText: "BU902" }).count()) === 0) {
    await createBU("BU902", "BU902");
  }
  await expect(page.locator('select[name="map_BU902"]')).toBeVisible();

  await page.goto(`/org/job-catalog?as_of=${asOf}&business_unit_id=BU902`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Resolved SetID:")).toContainText("SHARE");

  await page.goto(`/org/job-catalog?as_of=${asOf}&business_unit_id=BU999`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Resolved SetID:")).toHaveCount(0);
  await expect(page.locator('div[style*="border:1px solid #c00"]')).toBeVisible();

  await page.goto(`/org/positions?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");
  await expect(page.locator('select[name="org_unit_id"] option', { hasText: "(no org units)" })).toHaveCount(0);

  const findOrgUnitOptionValue = async (name) => {
    const option = page.locator('select[name="org_unit_id"] option', { hasText: name }).first();
    const value = await option.getAttribute("value");
    expect(value).not.toBeNull();
    return value;
  };
  const orgIDs = {
    HQ: await findOrgUnitOptionValue("HQ"),
    "R&D": await findOrgUnitOptionValue("R&D"),
    Sales: await findOrgUnitOptionValue("Sales"),
    Ops: await findOrgUnitOptionValue("Ops"),
    Plant: await findOrgUnitOptionValue("Plant")
  };

  const positionSpecs = [
    { name: "P-ENG-01", org: orgIDs["R&D"] },
    { name: "P-ENG-02", org: orgIDs["R&D"] },
    { name: "P-SALES-01", org: orgIDs.Sales },
    { name: "P-HR-01", org: orgIDs.HQ },
    { name: "P-FIN-01", org: orgIDs.HQ },
    { name: "P-MGR-01", org: orgIDs.HQ },
    { name: "P-OPS-01", org: orgIDs.Ops },
    { name: "P-SUPPORT-01", org: orgIDs.Ops },
    { name: "P-PLANT-01", org: orgIDs.Plant },
    { name: "P-PLANT-02", org: orgIDs.Plant }
  ];

  for (const spec of positionSpecs) {
    if ((await page.locator("tr", { hasText: spec.name }).count()) > 0) {
      continue;
    }
    await page.locator('input[name="effective_date"]').fill(asOf);
    await page.locator('select[name="org_unit_id"]').selectOption(spec.org);
    await page.locator('input[name="name"]').fill(spec.name);
    await page.getByRole("button", { name: "Create" }).click();
    await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${asOf}$`));
  }
  for (const spec of positionSpecs) {
    await expect(page.locator("tr", { hasText: spec.name })).toBeVisible();
  }

  await appContext.close();
});
