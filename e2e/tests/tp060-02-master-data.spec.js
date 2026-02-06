import { expect, test } from "@playwright/test";

test("tp060-02: master data (orgunit -> setid -> jobcatalog -> positions)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const m5EffectiveDate = "2026-01-15";
  const m5CrossEffectiveDate = "2026-01-16";
  const runID = `${Date.now()}`;
  const defltJobFamilyGroupCode = `JFG-DEF-${runID}`;
  const defltJobFamilyCode = `JF-DEF-${runID}`;
  const defltJobProfileCode = `JP-DEF-${runID}`;
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
  await expect(superadminPage.getByText(tenantHost)).toBeVisible({ timeout: 60000 });

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

  await page.goto("/login");
  await expect(page.locator("h1")).toHaveText("Login");
  await page.locator('input[name="email"]').fill(tenantAdminEmail);
  await page.locator('input[name="password"]').fill(tenantAdminPass);
  await page.getByRole("button", { name: "Login" }).click();
  await expect(page).toHaveURL(/\/app\?as_of=\d{4}-\d{2}-\d{2}$/);

  const orgCodeByName = (name) => {
    const map = {
      "Bugs & Blossoms Co., Ltd.": "ROOT",
      HQ: "HQ",
      "R&D": "RND",
      Sales: "SALES",
      Ops: "OPS",
      Plant: "PLANT"
    };
    if (map[name]) {
      return map[name];
    }
    const sanitized = name.toUpperCase().replace(/[^A-Z0-9_-]/g, "");
    return sanitized.slice(0, 16) || `ORG${runID.slice(-6)}`;
  };

  const findOrgUnitCode = async (name) => {
    const resp = await appContext.request.get(
      `/org/nodes/search?query=${encodeURIComponent(name)}&as_of=${encodeURIComponent(asOf)}`
    );
    if (resp.status() === 200) {
      const data = await resp.json();
      return (data && data.target_org_code) ? String(data.target_org_code) : "";
    }
    if (resp.status() !== 404) {
      throw new Error(`search org node failed: ${resp.status()}`);
    }
    return "";
  };

  const setBusinessUnitFlag = async (form, enabled) => {
    const input = form.locator('input[name="is_business_unit"]');
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

  const openCreateForm = async () => {
    await page.locator(".org-node-create-btn").click();
    const form = page.locator(`#org-node-details form[method="POST"][action="/org/nodes?tree_as_of=${asOf}"]`).first();
    await expect(form).toBeVisible();
    return form;
  };

  const createOrgUnit = async (effectiveDate, parentCode, name, isBusinessUnit = false) => {
    const form = await openCreateForm();
    await form.locator('input[name="effective_date"]').fill(effectiveDate);
    const orgCode = orgCodeByName(name);
    await form.locator('input[name="org_code"]').fill(orgCode);
    await form.locator('input[name="parent_code"]').fill(parentCode);
    await form.locator('input[name="name"]').fill(name);
    await setBusinessUnitFlag(form, isBusinessUnit);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/nodes\\?as_of=${asOf}$`));
  };

  await page.goto(`/org/nodes?tree_as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit Details");

  const rootName = "Bugs & Blossoms Co., Ltd.";
  let rootCode = await findOrgUnitCode(rootName);
  if (!rootCode) {
    await createOrgUnit(asOf, "", rootName, true);
    rootCode = await findOrgUnitCode(rootName);
  }
  expect(rootCode).not.toBe("");

  const level1 = ["HQ", "R&D", "Sales", "Ops", "Plant"];
  for (const name of level1) {
    const code = await findOrgUnitCode(name);
    if (code) {
      continue;
    }
    await createOrgUnit(asOf, rootCode, name, name === "R&D" || name === "Sales");
    expect(await findOrgUnitCode(name)).not.toBe("");
  }

  const orgCodesFromTree = {
    Root: rootCode,
    HQ: await findOrgUnitCode("HQ"),
    "R&D": await findOrgUnitCode("R&D"),
    Sales: await findOrgUnitCode("Sales"),
    Ops: await findOrgUnitCode("Ops"),
    Plant: await findOrgUnitCode("Plant")
  };
  for (const [name, code] of Object.entries(orgCodesFromTree)) {
    expect(code).not.toBe("");
  }

  const ensureBusinessUnit = async (orgCode, label) => {
    const resp = await appContext.request.post("/org/api/org-units/set-business-unit", {
      data: {
        org_code: orgCode,
        effective_date: asOf,
        is_business_unit: true,
        request_code: `tp060-02-bu-${label}-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(200);
  };

  await ensureBusinessUnit(orgCodesFromTree.Root, "root");
  await ensureBusinessUnit(orgCodesFromTree["R&D"], "rd");
  await ensureBusinessUnit(orgCodesFromTree.Sales, "sales");

  const emptyNameForm = await openCreateForm();
  await emptyNameForm.locator('input[name="effective_date"]').fill(asOf);
  await emptyNameForm.locator('input[name="org_code"]').fill("EMPTYNAME");
  await emptyNameForm.locator('input[name="parent_code"]').fill(rootCode);
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

  const bindSetID = async (orgCode, setid, effectiveDate) => {
    const form = page.locator(`form[method="POST"][action="/org/setid?as_of=${asOf}"]`).filter({
      has: page.locator('input[name="action"][value="bind_setid"]')
    });
    if ((await form.count()) === 0) {
      throw new Error("missing bind_setid form in /org/setid");
    }
    const fillField = async (name, value) => {
      const select = form.locator(`select[name="${name}"]`);
      if ((await select.count()) > 0) {
        await select.first().selectOption(value);
        return;
      }
      const input = form.locator(`input[name="${name}"]`);
      if ((await input.count()) > 0) {
        await input.first().fill(value);
        return;
      }
      throw new Error(`missing field ${name} in bind_setid form`);
    };
    await fillField("org_code", orgCode);
    await fillField("setid", setid);
    await fillField("effective_date", effectiveDate);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/setid\\?as_of=${asOf}$`));
  };

  const createScopePackage = async (scopeCode, packageCode, name, effectiveDate, ownerSetID) => {
    const resp = await appContext.request.post("/org/api/scope-packages", {
      data: {
        scope_code: scopeCode,
        package_code: packageCode,
        name,
        owner_setid: ownerSetID,
        effective_date: effectiveDate,
        request_code: `req:${runID}:scope-pkg:${packageCode}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
    const body = await resp.json();
    return body.package_id;
  };

  const subscribeScope = async (setid, scopeCode, packageID, effectiveDate) => {
    const resp = await appContext.request.post("/org/api/scope-subscriptions", {
      data: {
        setid,
        scope_code: scopeCode,
        package_id: packageID,
        package_owner: "tenant",
        effective_date: effectiveDate,
        request_code: `req:${runID}:scope-sub:${setid}:${scopeCode}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
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
  await expect(setidsTable).toContainText("DEFLT");

  const rdBindingExists =
    (await page.locator("tr", { hasText: orgCodesFromTree["R&D"] }).filter({ hasText: "S2601" }).count()) > 0 ||
    (await page.locator("tr", { hasText: "R&D" }).filter({ hasText: "S2601" }).count()) > 0;
  if (!rdBindingExists) {
    await bindSetID(orgCodesFromTree["R&D"], "S2601", asOf);
  }

  const s2601PkgSuffix = String(runID).slice(-4);
  const s2601PkgCode = `S2601_${s2601PkgSuffix}`.toUpperCase();
  await createScopePackage("jobcatalog", s2601PkgCode, `S2601 JobCatalog ${runID}`, asOf, "S2601");

  {
    const resp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: orgCodesFromTree.HQ,
        setid: "S2601",
        effective_date: asOf,
        request_code: `tp060-02-bind-hq-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(422);
    expect((await resp.json()).code).toBe("ORG_NOT_BUSINESS_UNIT_AS_OF");
  }

  const asOfJobCatalogBase = asOf;
  const asOfJobCatalogBeforeReparent = "2026-01-15";
  const asOfJobCatalogAfterReparent = "2026-02-15";
  const reparentEffectiveDate = "2026-02-01";
  const beforeJobCatalogExists = "2025-12-31";

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&package_code=${s2601PkgCode}`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Package:")).toContainText(s2601PkgCode);
  await expect(page.getByText("Owner SetID:")).toContainText("S2601");

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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2601PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2601PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2601PkgCode})(?=.*as_of=${reparentEffectiveDate}).*$`)
    );
  }
  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBeforeReparent}&package_code=${s2601PkgCode}`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Owner SetID:")).toContainText("S2601");
  await expect(
    page.locator('h2:has-text("Job Families")').locator("xpath=following-sibling::table[1]").locator("tr", { hasText: "JF-BE" })
  ).toContainText("JFG-ENG");

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogAfterReparent}&package_code=${s2601PkgCode}`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Owner SetID:")).toContainText("S2601");
  await expect(
    page.locator('h2:has-text("Job Families")').locator("xpath=following-sibling::table[1]").locator("tr", { hasText: "JF-BE" })
  ).toContainText("JFG-SALES");

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&package_code=${s2601PkgCode}`);
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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2601PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
  };
  await ensureJobLevel("JL-1", "Level 1");
  const levelsTable = page
    .locator('h2:has-text("Job Levels")')
    .locator("xpath=following-sibling::table[1]");
  await expect(levelsTable).toContainText("JL-1");

  await page.goto(`/org/job-catalog?as_of=${beforeJobCatalogExists}&package_code=${s2601PkgCode}`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  const levelsTableBefore = page
    .locator('h2:has-text("Job Levels")')
    .locator("xpath=following-sibling::table[1]");
  await expect(levelsTableBefore.locator("tr", { hasText: "JL-1" })).toHaveCount(0);

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&package_code=${s2601PkgCode}`);
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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2601PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
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

  await page.goto(`/org/job-catalog?as_of=${asOf}&package_code=DEFLT`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Owner SetID:")).toContainText("DEFLT");

  const ensureDefltJobFamilyGroup = async (code, name) => {
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

  const ensureDefltJobFamily = async (code, name, groupCode) => {
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

  const ensureDefltJobProfile = async (code, name, familyCodesCSV, primaryFamilyCode) => {
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

  await ensureDefltJobFamilyGroup(defltJobFamilyGroupCode, `Default Group ${runID}`);
  await ensureDefltJobFamily(defltJobFamilyCode, `Default Family ${runID}`, defltJobFamilyGroupCode);
  await ensureDefltJobProfile(defltJobProfileCode, `Default Profile ${runID}`, defltJobFamilyCode, defltJobFamilyCode);

  await page.goto(`/org/job-catalog?as_of=${asOf}&setid=S9999`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("SetID:")).toContainText("S9999");
  await expect(page.locator('div[style*="border:1px solid #c00"]')).toBeVisible();

  // Prepare another SetID + Job Profile for cross-setid reference tests (M5 fail-closed).
  await page.goto(`/org/setid?as_of=${asOf}`);
  if ((await page.locator("tr", { hasText: "S2602" }).count()) === 0) {
    await createSetID("S2602", "SetID S2602");
  }
  const salesBindingExists =
    (await page.locator("tr", { hasText: orgCodesFromTree.Sales }).filter({ hasText: "S2602" }).count()) > 0 ||
    (await page.locator("tr", { hasText: "Sales" }).filter({ hasText: "S2602" }).count()) > 0;
  if (!salesBindingExists) {
    await bindSetID(orgCodesFromTree.Sales, "S2602", asOf);
  }

  const s2602PkgSuffix = String(runID).slice(-4);
  const s2602PkgCode = `S2602_${s2602PkgSuffix}`.toUpperCase();
  await createScopePackage(
    "jobcatalog",
    s2602PkgCode,
    `S2602 JobCatalog ${runID}`,
    asOf,
    "S2602"
  );

  await page.goto(`/org/job-catalog?as_of=${asOfJobCatalogBase}&package_code=${s2602PkgCode}`);
  await expect(page.locator("h1")).toHaveText("Job Catalog");
  await expect(page.getByText("Package:")).toContainText(s2602PkgCode);
  await expect(page.getByText("Owner SetID:")).toContainText("S2602");

  const ensureJobFamilyGroupSales = async (code, name) => {
    if ((await page.locator("tr", { hasText: code }).count()) > 0) {
      return;
    }
    const form = page.locator(`form[method="POST"]`).filter({
      has: page.locator('input[name="action"][value="create_job_family_group"]')
    });
    await form.locator('input[name="job_family_group_code"]').fill(code);
    await form.locator('input[name="job_family_group_name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2602PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
  };
  await ensureJobFamilyGroupSales("JFG-OPS", "Operations");

  const ensureJobFamilySales = async (code, name, groupCode) => {
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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2602PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
  };
  await ensureJobFamilySales("JF-OPS", "Ops", "JFG-OPS");

  const ensureJobProfileSales = async (code, name, familyCodesCSV, primaryFamilyCode) => {
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
    await expect(page).toHaveURL(
      new RegExp(`/org/job-catalog\\?(?=.*package_code=${s2602PkgCode})(?=.*as_of=${asOfJobCatalogBase}).*$`)
    );
  };
  await ensureJobProfileSales("JP-OPS", "Ops Profile", "JF-OPS", "JF-OPS");

  const loadPositions = async (orgUnitID) => {
    await page.goto(`/org/positions?as_of=${asOf}&org_code=${orgUnitID}`);
    await expect(page.locator("h1")).toHaveText("Staffing / Positions");
    const form = page
      .locator(`form[method="POST"][action*="/org/positions"][action*="as_of=${asOf}"]`)
      .first();
    const hiddenValue = await form.locator('input[name="org_code"]').getAttribute("value");
    expect(hiddenValue).toBe(orgUnitID);
    return form;
  };

  let positionCreateForm = await loadPositions(orgCodesFromTree["R&D"]);
  const orgSelectIDs = {
    HQ: orgCodesFromTree.HQ,
    "R&D": orgCodesFromTree["R&D"],
    Sales: orgCodesFromTree.Sales,
    Ops: orgCodesFromTree.Ops,
    Plant: orgCodesFromTree.Plant
  };

  const jpSweOpt = positionCreateForm.locator('select[name="job_profile_uuid"] option', { hasText: "JP-SWE" }).first();
  const jpSweID = (await jpSweOpt.getAttribute("value")) || "";
  expect(jpSweID).not.toBe("");

  positionCreateForm = await loadPositions(orgCodesFromTree.Sales);
  const jpOpsOpt = positionCreateForm.locator('select[name="job_profile_uuid"] option', { hasText: "JP-OPS" }).first();
  const jpOpsID = (await jpOpsOpt.getAttribute("value")) || "";
  expect(jpOpsID).not.toBe("");

  positionCreateForm = await loadPositions(orgCodesFromTree.HQ);
  const jpDefOpt = positionCreateForm.locator('select[name="job_profile_uuid"] option', { hasText: defltJobProfileCode }).first();
  const jpDefID = (await jpDefOpt.getAttribute("value")) || "";
  expect(jpDefID).not.toBe("");

  const positionSpecsS2601 = [
    { name: "P-ENG-01", org: orgSelectIDs["R&D"] },
    { name: "P-ENG-02", org: orgSelectIDs["R&D"] }
  ];
  const positionSpecsS2602 = [{ name: "P-SALES-01", org: orgSelectIDs.Sales }];
  const positionSpecsDeflt = [
    { name: "P-HR-01", org: orgSelectIDs.HQ },
    { name: "P-FIN-01", org: orgSelectIDs.HQ },
    { name: "P-MGR-01", org: orgSelectIDs.HQ },
    { name: "P-OPS-01", org: orgSelectIDs.Ops },
    { name: "P-SUPPORT-01", org: orgSelectIDs.Ops },
    { name: "P-PLANT-01", org: orgSelectIDs.Plant },
    { name: "P-PLANT-02", org: orgSelectIDs.Plant }
  ];
  const positionSpecs = [...positionSpecsS2601, ...positionSpecsS2602, ...positionSpecsDeflt];

  const createPositions = async (jobProfileID, specs) => {
    let currentOrgUnitID = "";
    for (const spec of specs) {
      if ((await page.locator("tr", { hasText: spec.name }).count()) > 0) {
        continue;
      }
      if (currentOrgUnitID !== spec.org) {
        positionCreateForm = await loadPositions(spec.org);
        currentOrgUnitID = spec.org;
      }
      await positionCreateForm.locator('input[name="effective_date"]').fill(asOf);
      await positionCreateForm.locator('select[name="job_profile_uuid"]').selectOption(jobProfileID);
      await positionCreateForm.locator('input[name="name"]').fill(spec.name);
      await positionCreateForm.getByRole("button", { name: "Create" }).click();
      await expect(page).toHaveURL(
        new RegExp(`/org/positions\\?(?=.*as_of=${asOf})(?=.*org_code=${spec.org}).*$`)
      );
    }
  };

  await createPositions(jpSweID, positionSpecsS2601);
  await createPositions(jpOpsID, positionSpecsS2602);
  await createPositions(jpDefID, positionSpecsDeflt);

  for (const spec of positionSpecs) {
    await expect(page.locator("tr", { hasText: spec.name })).toBeVisible();
  }

  // Position M5: bind OrgUnit + Job Profile, and assert stable fail-closed errors via internal API.
  const pEng01Row = page.locator("tr", { hasText: "P-ENG-01" }).first();
  const pEng01ID = (await pEng01Row.locator("td").nth(1).innerText()).trim();
  expect(pEng01ID).not.toBe("");

  await page.goto(`/org/positions?as_of=${asOf}&org_code=${orgCodesFromTree["R&D"]}`);
  await expect(page.getByText("SetID:")).toBeVisible();
  await expect(page.getByText("SetID:")).toContainText("S2601");

  {
    const bindResp = await appContext.request.post(`/org/api/positions?as_of=${m5EffectiveDate}`, {
      data: {
        effective_date: m5EffectiveDate,
        position_uuid: pEng01ID,
        org_code: orgCodesFromTree["R&D"],
        job_profile_uuid: jpSweID
      }
    });
    expect(bindResp.status(), await bindResp.text()).toBe(200);
  }

  await page.goto(`/org/positions?as_of=${m5EffectiveDate}&org_code=${orgCodesFromTree["R&D"]}`);
  const pEng01BoundRow = page.locator("tr", { hasText: "P-ENG-01" }).first();
  await expect(pEng01BoundRow).toContainText(orgCodesFromTree["R&D"]);
  await expect(pEng01BoundRow).toContainText("S2601");
  await expect(pEng01BoundRow).toContainText("JP-SWE");

  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${asOf}`, {
      data: {
        effective_date: asOf,
        job_profile_uuid: jpSweID,
        name: `TP060-02 BAD NO ORG ${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(400);
    expect((await resp.json()).code).toBe("invalid_request");
  }
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${asOf}`, {
      data: {
        effective_date: asOf,
        org_code: "99999999",
        job_profile_uuid: jpSweID,
        name: `TP060-02 BAD ORG404 ${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(404);
    expect((await resp.json()).code).toBe("org_code_not_found");
  }
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${asOf}`, {
      data: {
        effective_date: asOf,
        org_code: orgCodesFromTree["R&D"],
        name: `TP060-02 BAD NO JOB PROFILE ${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(400);
    expect((await resp.json()).code).toBe("job_profile_uuid is required");
  }

  // Cross-setid Job Profile reference must fail-closed (org_unit resolves S2601, JP-OPS is in S2602).
  await page.goto(`/org/positions?as_of=${asOf}&org_code=${orgCodesFromTree.Sales}`);
  await expect(page.getByText("SetID:")).toBeVisible();
  await expect(page.getByText("SetID:")).toContainText("S2602");
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${m5CrossEffectiveDate}`, {
      data: {
        effective_date: m5CrossEffectiveDate,
        position_uuid: pEng01ID,
        org_code: orgCodesFromTree["R&D"],
        job_profile_uuid: jpOpsID
      }
    });
    expect(resp.status(), await resp.text()).toBe(422);
    expect((await resp.json()).code).toBe("JOBCATALOG_REFERENCE_NOT_FOUND");
  }

  await appContext.close();
});
