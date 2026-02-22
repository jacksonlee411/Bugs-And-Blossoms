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

async function createJobCatalogAction(ctx, { packageCode, effectiveDate, action, body }) {
  const resp = await ctx.request.post("/jobcatalog/api/catalog/actions", {
    data: {
      package_code: packageCode,
      effective_date: effectiveDate,
      action,
      ...body
    }
  });
  return resp;
}

async function getJobCatalog(ctx, { asOf, packageCode }) {
  const resp = await ctx.request.get(
    `/jobcatalog/api/catalog?as_of=${encodeURIComponent(asOf)}&package_code=${encodeURIComponent(packageCode)}`
  );
  expect(resp.status(), await resp.text()).toBe(200);
  return resp.json();
}

async function getPositionOptions(ctx, { asOf, orgCode }) {
  const resp = await ctx.request.get(
    `/org/api/positions:options?as_of=${encodeURIComponent(asOf)}&org_code=${encodeURIComponent(orgCode)}`
  );
  expect(resp.status(), await resp.text()).toBe(200);
  return resp.json();
}

test("tp060-02: master data (orgunit -> setid -> jobcatalog -> positions)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const m5EffectiveDate = "2026-01-15";
  const m5CrossEffectiveDate = "2026-01-16";
  const runID = `${Date.now()}`;

  const suffix = runID.slice(-4);
  const tenantHost = `t-tp060-02-${runID}.localhost`;
  const tenantName = `TP060-02 Tenant ${runID}`;

  const tenantAdminEmail = `tenant-admin+062-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const defaultSuperadminEmail = `admin+tp060-02-${runID}@example.invalid`;
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || defaultSuperadminEmail;
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
  const page = await appContext.newPage();

  const legacyLoginGet = await appContext.request.get("/login");
  expect(legacyLoginGet.status()).toBe(404);

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);

  await page.goto(`/app?as_of=${asOf}`);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");

  // Ensure SetID bootstrap (DEFLT/SHARE) is present before jobcatalog edit checks.
  const setidsResp = await appContext.request.get("/org/api/setids");
  expect(setidsResp.status(), await setidsResp.text()).toBe(200);
  const setidsJSON = await setidsResp.json();
  const existingSetIDs = new Set((setidsJSON.setids || []).map((s) => s.setid));
  expect(existingSetIDs.has("DEFLT")).toBeTruthy();
  expect(existingSetIDs.has("SHARE")).toBeTruthy();

  const ensureSetID = async (setid, name) => {
    if (existingSetIDs.has(setid)) {
      return;
    }
    const resp = await appContext.request.post("/org/api/setids", {
      data: {
        setid,
        name,
        effective_date: asOf,
        request_id: `tp060-02-setid-${setid}-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
    existingSetIDs.add(setid);
  };

  await ensureSetID("S2601", "SetID S2601");
  await ensureSetID("S2602", "SetID S2602");

  const org = {
    root: `ROOT${suffix}`.toUpperCase(),
    hq: `HQ${suffix}`.toUpperCase(),
    rnd: `RND${suffix}`.toUpperCase(),
    sales: `SALES${suffix}`.toUpperCase(),
    ops: `OPS${suffix}`.toUpperCase(),
    plant: `PLANT${suffix}`.toUpperCase()
  };

  const createOrgUnit = async ({ org_code, name, parent_org_code, is_business_unit }) => {
    const resp = await appContext.request.post("/org/api/org-units", {
      data: {
        org_code,
        name,
        effective_date: asOf,
        parent_org_code,
        is_business_unit
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
  };

  await createOrgUnit({
    org_code: org.root,
    name: `TP060-02 Root ${runID}`,
    parent_org_code: "",
    is_business_unit: true
  });
  await createOrgUnit({ org_code: org.hq, name: "HQ", parent_org_code: org.root, is_business_unit: false });
  // Mark R&D and Sales as BU at creation time; same-day "SET_BUSINESS_UNIT" would conflict with CREATE (one-event-per-day).
  await createOrgUnit({ org_code: org.rnd, name: "R&D", parent_org_code: org.root, is_business_unit: true });
  await createOrgUnit({ org_code: org.sales, name: "Sales", parent_org_code: org.root, is_business_unit: true });
  await createOrgUnit({ org_code: org.ops, name: "Ops", parent_org_code: org.root, is_business_unit: false });
  await createOrgUnit({ org_code: org.plant, name: "Plant", parent_org_code: org.root, is_business_unit: false });

  // Root BU: bind DEFLT so non-BU descendants can resolve DEFLT jobcatalog options.
  {
    const resp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: org.root,
        setid: "DEFLT",
        effective_date: asOf,
        request_id: `tp060-02-bind-root-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
  }

  // Bind non-BU must fail (HQ is not a BU as of).
  {
    const resp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: org.hq,
        setid: "S2601",
        effective_date: asOf,
        request_id: `tp060-02-bind-hq-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(422);
    expect((await resp.json()).code).toBe("ORG_NOT_BUSINESS_UNIT_AS_OF");
  }

  // Bind SetIDs for business units (R&D -> S2601, Sales -> S2602).
  {
    const resp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: org.rnd,
        setid: "S2601",
        effective_date: asOf,
        request_id: `tp060-02-bind-rnd-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
  }
  {
    const resp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: org.sales,
        setid: "S2602",
        effective_date: asOf,
        request_id: `tp060-02-bind-sales-${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
  }

  const createScopePackage = async ({ ownerSetID, packageCode, name }) => {
    const resp = await appContext.request.post("/org/api/scope-packages", {
      data: {
        scope_code: "jobcatalog",
        package_code: packageCode,
        name,
        owner_setid: ownerSetID,
        effective_date: asOf,
        request_id: `req:${runID}:scope-pkg:${packageCode}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
    return resp.json();
  };

  const s2601PkgCode = `S2601_${suffix}`.toUpperCase();
  await createScopePackage({ ownerSetID: "S2601", packageCode: s2601PkgCode, name: `S2601 JobCatalog ${runID}` });

  const s2602PkgCode = `S2602_${suffix}`.toUpperCase();
  await createScopePackage({ ownerSetID: "S2602", packageCode: s2602PkgCode, name: `S2602 JobCatalog ${runID}` });

  // JobCatalog (S2601): create groups/families/levels/profiles, then assert valid-time reparent.
  {
    const base = asOf;
    const beforeReparent = "2026-01-15";
    const afterReparent = "2026-02-15";
    const reparentEffectiveDate = "2026-02-01";
    const beforeJobCatalogExists = "2025-12-31";

    const mustAction = async (action, body, effectiveDate = base, expectedStatus = 201) => {
      const resp = await createJobCatalogAction(appContext, {
        packageCode: s2601PkgCode,
        effectiveDate,
        action,
        body
      });
      expect(resp.status(), await resp.text()).toBe(expectedStatus);
    };

    await mustAction("create_job_family_group", { code: "JFG-ENG", name: "Engineering" });
    await mustAction("create_job_family_group", { code: "JFG-SALES", name: "Sales" });

    await mustAction("create_job_family", { code: "JF-BE", name: "Backend", group_code: "JFG-ENG" });
    await mustAction("create_job_family", { code: "JF-FE", name: "Frontend", group_code: "JFG-ENG" });

    await mustAction("update_job_family_group", { code: "JF-BE", group_code: "JFG-SALES" }, reparentEffectiveDate, 200);

    const beforeCatalog = await getJobCatalog(appContext, { asOf: beforeReparent, packageCode: s2601PkgCode });
    const jfBeBefore = (beforeCatalog.job_families || []).find((f) => f.job_family_code === "JF-BE");
    expect(jfBeBefore && jfBeBefore.job_family_group_code).toBe("JFG-ENG");

    const afterCatalog = await getJobCatalog(appContext, { asOf: afterReparent, packageCode: s2601PkgCode });
    const jfBeAfter = (afterCatalog.job_families || []).find((f) => f.job_family_code === "JF-BE");
    expect(jfBeAfter && jfBeAfter.job_family_group_code).toBe("JFG-SALES");

    await mustAction("create_job_level", { code: "JL-1", name: "Level 1" });
    const catalogBeforeExists = await getJobCatalog(appContext, { asOf: beforeJobCatalogExists, packageCode: s2601PkgCode });
    expect((catalogBeforeExists.job_levels || []).some((l) => l.job_level_code === "JL-1")).toBeFalsy();

    await mustAction("create_job_profile", {
      code: "JP-SWE",
      name: "Software Engineer",
      family_codes_csv: "JF-BE,JF-FE",
      primary_family_code: "JF-BE"
    });
    const catalogWithProfile = await getJobCatalog(appContext, { asOf: base, packageCode: s2601PkgCode });
    const jpSwe = (catalogWithProfile.job_profiles || []).find((p) => p.job_profile_code === "JP-SWE");
    expect(jpSwe && jpSwe.primary_family_code).toBe("JF-BE");

    const badProfileResp = await createJobCatalogAction(appContext, {
      packageCode: s2601PkgCode,
      effectiveDate: base,
      action: "create_job_profile",
      body: {
        code: "JP-BAD",
        name: "Bad Profile",
        family_codes_csv: "JF-BE",
        primary_family_code: "JF-FE"
      }
    });
    expect(badProfileResp.status(), await badProfileResp.text()).toBe(422);
  }

  // JobCatalog (DEFLT): create a dedicated profile for default SetID branch.
  const defltJobFamilyGroupCode = `JFG_DEF_${suffix}`.toUpperCase();
  const defltJobFamilyCode = `JF_DEF_${suffix}`.toUpperCase();
  const defltJobProfileCode = `JP_DEF_${suffix}`.toUpperCase();
  {
    const mustCreate = async (action, body) => {
      const resp = await createJobCatalogAction(appContext, {
        packageCode: "DEFLT",
        effectiveDate: asOf,
        action,
        body
      });
      expect(resp.status(), await resp.text()).toBe(201);
    };
    await mustCreate("create_job_family_group", { code: defltJobFamilyGroupCode, name: `Default Group ${runID}` });
    await mustCreate("create_job_family", { code: defltJobFamilyCode, name: `Default Family ${runID}`, group_code: defltJobFamilyGroupCode });
    await mustCreate("create_job_profile", {
      code: defltJobProfileCode,
      name: `Default Profile ${runID}`,
      family_codes_csv: defltJobFamilyCode,
      primary_family_code: defltJobFamilyCode
    });
  }

  // JobCatalog invalid SetID selection must fail-closed (response is an error JSON).
  {
    const resp = await appContext.request.get(`/jobcatalog/api/catalog?as_of=${encodeURIComponent(asOf)}&setid=S9999`);
    expect(resp.status(), await resp.text()).toBe(422);
    const json = await resp.json();
    expect(json.code).toBe("JOBCATALOG_SETID_INVALID");
  }

  // JobCatalog (S2602): create a profile for cross-setid reference tests.
  {
    const mustCreate = async (action, body) => {
      const resp = await createJobCatalogAction(appContext, {
        packageCode: s2602PkgCode,
        effectiveDate: asOf,
        action,
        body
      });
      expect(resp.status(), await resp.text()).toBe(201);
    };
    await mustCreate("create_job_family_group", { code: "JFG-OPS", name: "Operations" });
    await mustCreate("create_job_family", { code: "JF-OPS", name: "Ops", group_code: "JFG-OPS" });
    await mustCreate("create_job_profile", {
      code: "JP-OPS",
      name: "Ops Profile",
      family_codes_csv: "JF-OPS",
      primary_family_code: "JF-OPS"
    });
  }

  // Resolve Job Profile UUIDs via options API.
  const rndOptions = await getPositionOptions(appContext, { asOf, orgCode: org.rnd });
  expect(rndOptions.jobcatalog_setid).toBe("S2601");
  const jpSweOpt = (rndOptions.job_profiles || []).find((p) => p.job_profile_code === "JP-SWE");
  expect(jpSweOpt && jpSweOpt.job_profile_uuid).toBeTruthy();

  const salesOptions = await getPositionOptions(appContext, { asOf, orgCode: org.sales });
  expect(salesOptions.jobcatalog_setid).toBe("S2602");
  const jpOpsOpt = (salesOptions.job_profiles || []).find((p) => p.job_profile_code === "JP-OPS");
  expect(jpOpsOpt && jpOpsOpt.job_profile_uuid).toBeTruthy();

  const hqOptions = await getPositionOptions(appContext, { asOf, orgCode: org.hq });
  expect(hqOptions.jobcatalog_setid).toBe("DEFLT");
  const jpDefOpt = (hqOptions.job_profiles || []).find((p) => p.job_profile_code === defltJobProfileCode);
  expect(jpDefOpt && jpDefOpt.job_profile_uuid).toBeTruthy();

  const createPosition = async ({ effectiveDate, orgCode, jobProfileUUID, name }) => {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(effectiveDate)}`, {
      data: {
        effective_date: effectiveDate,
        org_code: orgCode,
        job_profile_uuid: jobProfileUUID,
        name
      }
    });
    expect(resp.status(), await resp.text()).toBe(200);
    return resp.json();
  };

  const positionSpecsS2601 = [
    { name: "P-ENG-01", org: org.rnd },
    { name: "P-ENG-02", org: org.rnd }
  ];
  const positionSpecsS2602 = [{ name: "P-SALES-01", org: org.sales }];
  const positionSpecsDeflt = [
    { name: "P-HR-01", org: org.hq },
    { name: "P-FIN-01", org: org.hq }
  ];

  const createdPositions = [];
  for (const spec of positionSpecsS2601) {
    createdPositions.push(
      await createPosition({
        effectiveDate: asOf,
        orgCode: spec.org,
        jobProfileUUID: jpSweOpt.job_profile_uuid,
        name: spec.name
      })
    );
  }
  for (const spec of positionSpecsS2602) {
    createdPositions.push(
      await createPosition({
        effectiveDate: asOf,
        orgCode: spec.org,
        jobProfileUUID: jpOpsOpt.job_profile_uuid,
        name: spec.name
      })
    );
  }
  for (const spec of positionSpecsDeflt) {
    createdPositions.push(
      await createPosition({
        effectiveDate: asOf,
        orgCode: spec.org,
        jobProfileUUID: jpDefOpt.job_profile_uuid,
        name: spec.name
      })
    );
  }

  const listPositionsResp = await appContext.request.get(`/org/api/positions?as_of=${encodeURIComponent(asOf)}`);
  expect(listPositionsResp.status(), await listPositionsResp.text()).toBe(200);
  const listPositionsJSON = await listPositionsResp.json();
  const posNames = new Set((listPositionsJSON.positions || []).map((p) => p.name));
  for (const spec of [...positionSpecsS2601, ...positionSpecsS2602, ...positionSpecsDeflt]) {
    expect(posNames.has(spec.name)).toBeTruthy();
  }

  // Position M5: update an existing position at a later effective_date.
  const pEng01 = createdPositions.find((p) => p.name === "P-ENG-01");
  expect(pEng01 && pEng01.position_uuid).toBeTruthy();

  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(m5EffectiveDate)}`, {
      data: {
        effective_date: m5EffectiveDate,
        position_uuid: pEng01.position_uuid,
        org_code: org.rnd,
        job_profile_uuid: jpSweOpt.job_profile_uuid
      }
    });
    expect(resp.status(), await resp.text()).toBe(200);
  }

  // Positions API must fail-closed with stable codes.
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(asOf)}`, {
      data: {
        effective_date: asOf,
        job_profile_uuid: jpSweOpt.job_profile_uuid,
        name: `TP060-02 BAD NO ORG ${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(400);
    expect((await resp.json()).code).toBe("invalid_request");
  }
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(asOf)}`, {
      data: {
        effective_date: asOf,
        org_code: "99999999",
        job_profile_uuid: jpSweOpt.job_profile_uuid,
        name: `TP060-02 BAD ORG404 ${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(404);
    expect((await resp.json()).code).toBe("org_code_not_found");
  }
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(asOf)}`, {
      data: {
        effective_date: asOf,
        org_code: org.rnd,
        name: `TP060-02 BAD NO JOB PROFILE ${runID}`
      }
    });
    expect(resp.status(), await resp.text()).toBe(400);
    expect((await resp.json()).code).toBe("job_profile_uuid is required");
  }

  // Cross-setid Job Profile reference must fail-closed (R&D resolves S2601, JP-OPS is in S2602).
  {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(m5CrossEffectiveDate)}`, {
      data: {
        effective_date: m5CrossEffectiveDate,
        position_uuid: pEng01.position_uuid,
        org_code: org.rnd,
        job_profile_uuid: jpOpsOpt.job_profile_uuid
      }
    });
    expect(resp.status(), await resp.text()).toBe(422);
    expect((await resp.json()).code).toBe("JOBCATALOG_REFERENCE_NOT_FOUND");
  }

  // UI sanity checks (MUI-only pages)
  await page.goto(`/app/org/units?as_of=${asOf}`);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");
  await page.goto(`/app/org/setid`);
  await expect(page.getByRole("heading", { level: 2, name: "SetID Governance" })).toBeVisible();
  await page.goto(`/app/jobcatalog?as_of=${asOf}&package_code=${s2601PkgCode}`);
  await expect(page.getByRole("heading", { level: 2, name: "Job Catalog" })).toBeVisible();
  await page.goto(`/app/staffing/positions?as_of=${asOf}&org_code=${org.rnd}`);
  await expect(page.getByRole("heading", { level: 2, name: "Staffing / Positions" })).toBeVisible();

  await appContext.close();
});
