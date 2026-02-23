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

async function createJobCatalogAction(ctx, { setID, effectiveDate, action, body }) {
  const resp = await ctx.request.post("/jobcatalog/api/catalog/actions", {
    data: {
      setid: setID,
      effective_date: effectiveDate,
      action,
      ...body
    }
  });
  expect(resp.status(), await resp.text()).toBe(201);
}

test("tp060-03: person + assignments (with allocated_fte)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const midEffectiveDate = "2026-01-10";
  const asOfBeforeMid = "2026-01-09";
  const lateEffectiveDate = "2026-01-15";
  const runID = `${Date.now()}`;
  const suffix = runID.slice(-4);

  const tenantHost = `t-tp060-03-${runID}.localhost`;
  const tenantName = `TP060-03 Tenant ${runID}`;

  const tenantAdminEmail = `tenant-admin+063-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const defaultSuperadminEmail = `admin+tp060-03-${runID}@example.invalid`;
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || defaultSuperadminEmail;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;

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

  const ensureTenant = async (hostname, name) => {
    await superadminPage.goto("/superadmin/tenants");
    const tenantRow = superadminPage.locator("tr", { hasText: hostname }).first();
    if ((await tenantRow.count()) === 0) {
      await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(name);
      await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(hostname);
      await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
      await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
    }
    await expect(tenantRow).toBeVisible({ timeout: 60_000 });
    const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
    expect(tenantID).not.toBe("");
    return tenantID;
  };

  const tenantID = await ensureTenant(tenantHost, tenantName);

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

  // Ensure SetID bootstrap.
  const setidsResp = await appContext.request.get("/org/api/setids");
  expect(setidsResp.status(), await setidsResp.text()).toBe(200);

  const rootOrgCode = `TP06003R${suffix}`.toUpperCase();
  {
    const createOrgResp = await appContext.request.post("/org/api/org-units", {
      data: {
        org_code: rootOrgCode,
        name: `TP060-03 Root ${runID}`,
        effective_date: asOf,
        parent_org_code: "",
        is_business_unit: true
      }
    });
    expect(createOrgResp.status(), await createOrgResp.text()).toBe(201);

    const bindResp = await appContext.request.post("/org/api/setid-bindings", {
      data: {
        org_code: rootOrgCode,
        setid: "DEFLT",
        effective_date: asOf,
        request_id: `tp060-03-bind-root-${runID}`
      }
    });
    expect(bindResp.status(), await bindResp.text()).toBe(201);
  }

  // JobCatalog: create a dedicated job profile under DEFLT.
  const jobFamilyGroupCode = `JFG_063_${suffix}`.toUpperCase();
  const jobFamilyCode = `JF_063_${suffix}`.toUpperCase();
  const jobProfileCode = `JP_063_${suffix}`.toUpperCase();

  await createJobCatalogAction(appContext, {
    setID: "DEFLT",
    effectiveDate: asOf,
    action: "create_job_family_group",
    body: { code: jobFamilyGroupCode, name: `TP060-03 Group ${runID}` }
  });
  await createJobCatalogAction(appContext, {
    setID: "DEFLT",
    effectiveDate: asOf,
    action: "create_job_family",
    body: { code: jobFamilyCode, name: `TP060-03 Family ${runID}`, group_code: jobFamilyGroupCode }
  });
  await createJobCatalogAction(appContext, {
    setID: "DEFLT",
    effectiveDate: asOf,
    action: "create_job_profile",
    body: {
      code: jobProfileCode,
      name: `TP060-03 Profile ${runID}`,
      family_codes_csv: jobFamilyCode,
      primary_family_code: jobFamilyCode
    }
  });

  // Resolve Job Profile UUID via options API.
  const optionsResp = await appContext.request.get(
    `/org/api/positions:options?as_of=${encodeURIComponent(asOf)}&org_code=${encodeURIComponent(rootOrgCode)}`
  );
  expect(optionsResp.status(), await optionsResp.text()).toBe(200);
  const options = await optionsResp.json();
  const jobProfileOpt = (options.job_profiles || []).find((p) => p.job_profile_code === jobProfileCode);
  expect(jobProfileOpt && jobProfileOpt.job_profile_uuid).toBeTruthy();

  const upsertPosition = async (effectiveDate, payload) => {
    const resp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(effectiveDate)}`, {
      data: { effective_date: effectiveDate, ...payload }
    });
    expect(resp.status(), await resp.text()).toBe(200);
    return resp.json();
  };

  const pernrByIndex = Array.from({ length: 10 }, (_, i) => `${101 + i}`);
  const positionIDsByPernr = new Map();
  for (const pernr of pernrByIndex) {
    const positionName = `TP060-03 Position ${pernr} ${runID}`;
    const created = await upsertPosition(asOf, {
      org_code: rootOrgCode,
      job_profile_uuid: jobProfileOpt.job_profile_uuid,
      capacity_fte: pernr === "104" ? "0.50" : "1.0",
      name: positionName
    });
    expect(created.position_uuid).toBeTruthy();
    positionIDsByPernr.set(pernr, created.position_uuid);
  }

  const updateTargetPositionName = `TP060-03 UpdateTarget Position ${runID}`;
  const updateTarget = await upsertPosition(asOf, {
    org_code: rootOrgCode,
    job_profile_uuid: jobProfileOpt.job_profile_uuid,
    capacity_fte: "1.0",
    name: updateTargetPositionName
  });
  const updateTargetPositionID = updateTarget.position_uuid;
  expect(updateTargetPositionID).toBeTruthy();

  const disabledPositionName = `TP060-03 Disabled Position ${runID}`;
  const disabledCreated = await upsertPosition(asOf, {
    org_code: rootOrgCode,
    job_profile_uuid: jobProfileOpt.job_profile_uuid,
    name: disabledPositionName
  });
  const disabledPositionID = disabledCreated.position_uuid;
  expect(disabledPositionID).toBeTruthy();

  // Persons: create 10 persons via JSON API.
  const personUUIDByPernr = new Map();
  for (const pernr of pernrByIndex) {
    const displayName = `TP060-03 Person ${pernr} ${runID}`;
    const resp = await appContext.request.post("/person/api/persons", {
      data: { pernr, display_name: displayName }
    });
    expect(resp.status(), await resp.text()).toBe(201);
    const json = await resp.json();
    expect(json.person_uuid).toBeTruthy();
    personUUIDByPernr.set(pernr, json.person_uuid);
  }

  // Disable one position at lateEffectiveDate.
  await upsertPosition(lateEffectiveDate, {
    position_uuid: disabledPositionID,
    lifecycle_status: "disabled"
  });

  const listLateResp = await appContext.request.get(`/org/api/positions?as_of=${encodeURIComponent(lateEffectiveDate)}`);
  expect(listLateResp.status(), await listLateResp.text()).toBe(200);
  const listLate = await listLateResp.json();
  const disabledLate = (listLate.positions || []).find((p) => p.position_uuid === disabledPositionID);
  expect(disabledLate && disabledLate.lifecycle_status).toBe("disabled");

  // Reports-to relationships + fail-closed cycles/self/retro.
  const managerPernr = "101";
  const reporteePernr = "102";
  const managerPositionID = positionIDsByPernr.get(managerPernr);
  const reporteePositionID = positionIDsByPernr.get(reporteePernr);
  expect(managerPositionID).toBeTruthy();
  expect(reporteePositionID).toBeTruthy();

  await upsertPosition(lateEffectiveDate, {
    position_uuid: reporteePositionID,
    reports_to_position_uuid: managerPositionID
  });

  const reportsToCycleResp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(lateEffectiveDate)}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_uuid: managerPositionID,
      reports_to_position_uuid: reporteePositionID
    }
  });
  expect(reportsToCycleResp.status()).toBe(422);
  expect((await reportsToCycleResp.json()).code).toBe("STAFFING_POSITION_REPORTS_TO_CYCLE");

  const reportsToSelfResp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(lateEffectiveDate)}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_uuid: managerPositionID,
      reports_to_position_uuid: managerPositionID
    }
  });
  expect(reportsToSelfResp.status()).toBe(422);
  expect((await reportsToSelfResp.json()).code).toBe("STAFFING_POSITION_REPORTS_TO_SELF");

  const reportsToRetroResp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(midEffectiveDate)}`, {
    data: {
      effective_date: midEffectiveDate,
      position_uuid: reporteePositionID,
      reports_to_position_uuid: managerPositionID
    }
  });
  expect(reportsToRetroResp.status()).toBe(422);
  expect((await reportsToRetroResp.json()).code).toBe("STAFFING_INVALID_ARGUMENT");

  // Person normalization: leading zeros should resolve to canonical pernr.
  const byPernr = async (raw) => appContext.request.get(`/person/api/persons:by-pernr?pernr=${encodeURIComponent(raw)}`);

  const respLeadingZeros = await byPernr("00000103");
  expect(respLeadingZeros.status()).toBe(200);
  const leadingZerosJSON = await respLeadingZeros.json();
  expect(leadingZerosJSON.pernr).toBe("103");

  const respCanonical = await byPernr("103");
  expect(respCanonical.status()).toBe(200);
  const canonicalJSON = await respCanonical.json();
  expect(canonicalJSON.person_uuid).toBe(leadingZerosJSON.person_uuid);

  const respBad = await byPernr("BAD");
  expect(respBad.status()).toBe(400);
  expect((await respBad.json()).code).toBe("PERSON_PERNR_INVALID");

  const respNotFound = await byPernr("99999999");
  expect(respNotFound.status()).toBe(404);
  expect((await respNotFound.json()).code).toBe("PERSON_NOT_FOUND");

  // Assignments: disabled position cannot be assigned.
  {
    const resp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(lateEffectiveDate)}`, {
      data: {
        effective_date: lateEffectiveDate,
        person_uuid: personUUIDByPernr.get("101"),
        position_uuid: disabledPositionID,
        allocated_fte: "1.0"
      }
    });
    expect(resp.status(), await resp.text()).toBe(422);
    expect((await resp.json()).code).toBe("STAFFING_POSITION_DISABLED_AS_OF");
  }

  const upsertAssignment = async ({ pernr, effectiveDate, allocatedFte }) => {
    const personUUID = personUUIDByPernr.get(pernr);
    const positionUUID = positionIDsByPernr.get(pernr);
    expect(personUUID).toBeTruthy();
    expect(positionUUID).toBeTruthy();
    const resp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(effectiveDate)}`, {
      data: {
        effective_date: effectiveDate,
        person_uuid: personUUID,
        position_uuid: positionUUID,
        allocated_fte: allocatedFte
      }
    });
    expect(resp.status(), await resp.text()).toBe(200);
  };

  for (const pernr of pernrByIndex) {
    const isE04 = pernr === "104";
    const isE06 = pernr === "106";
    await upsertAssignment({
      pernr,
      effectiveDate: isE06 ? lateEffectiveDate : asOf,
      allocatedFte: isE04 ? "0.5" : "1.0"
    });
  }

  const p101 = personUUIDByPernr.get("101");
  const p102 = personUUIDByPernr.get("102");
  const pos101 = positionIDsByPernr.get("101");
  const pos104 = positionIDsByPernr.get("104");
  expect(p101).toBeTruthy();
  expect(p102).toBeTruthy();
  expect(pos101).toBeTruthy();
  expect(pos104).toBeTruthy();

  // Multi-slice Valid Time: update at midEffectiveDate, verify snapshot switches across as_of.
  {
    const moveResp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(midEffectiveDate)}`, {
      data: {
        effective_date: midEffectiveDate,
        person_uuid: p101,
        position_uuid: updateTargetPositionID,
        allocated_fte: "1.0"
      }
    });
    expect(moveResp.status(), await moveResp.text()).toBe(200);

    const beforeResp = await appContext.request.get(
      `/org/api/assignments?as_of=${encodeURIComponent(asOfBeforeMid)}&person_uuid=${encodeURIComponent(p101)}`
    );
    expect(beforeResp.status(), await beforeResp.text()).toBe(200);
    const beforeJSON = await beforeResp.json();
    expect(beforeJSON.assignments).toHaveLength(1);
    expect(beforeJSON.assignments[0].effective_date).toBe(asOf);
    expect(beforeJSON.assignments[0].position_uuid).toBe(pos101);

    const afterResp = await appContext.request.get(
      `/org/api/assignments?as_of=${encodeURIComponent(midEffectiveDate)}&person_uuid=${encodeURIComponent(p101)}`
    );
    expect(afterResp.status(), await afterResp.text()).toBe(200);
    const afterJSON = await afterResp.json();
    expect(afterJSON.assignments).toHaveLength(1);
    expect(afterJSON.assignments[0].effective_date).toBe(midEffectiveDate);
    expect(afterJSON.assignments[0].position_uuid).toBe(updateTargetPositionID);
  }

  // Rerunnable upsert: same effective_date same payload => OK; different payload => 409 STAFFING_IDEMPOTENCY_REUSED.
  {
    const okResp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(midEffectiveDate)}`, {
      data: {
        effective_date: midEffectiveDate,
        person_uuid: p101,
        position_uuid: updateTargetPositionID,
        allocated_fte: "1.0"
      }
    });
    expect(okResp.status(), await okResp.text()).toBe(200);

    const conflictResp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(midEffectiveDate)}`, {
      data: {
        effective_date: midEffectiveDate,
        person_uuid: p101,
        position_uuid: updateTargetPositionID,
        allocated_fte: "0.75"
      }
    });
    expect(conflictResp.status(), await conflictResp.text()).toBe(409);
    expect((await conflictResp.json()).code).toBe("STAFFING_IDEMPOTENCY_REUSED");
  }

  // Position exclusivity: occupied position cannot be assigned to another active assignment (fail-closed with stable code).
  {
    const occupiedResp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(midEffectiveDate)}`, {
      data: {
        effective_date: midEffectiveDate,
        person_uuid: p102,
        position_uuid: pos104,
        allocated_fte: "0.25"
      }
    });
    expect(occupiedResp.status(), await occupiedResp.text()).toBe(422);
    expect((await occupiedResp.json()).code).toBe("STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF");
  }

  const capacityPositionID = positionIDsByPernr.get("104");
  const capacityPersonUUID = personUUIDByPernr.get("104");
  expect(capacityPositionID).toBeTruthy();
  expect(capacityPersonUUID).toBeTruthy();

  const assignmentCapacityResp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(lateEffectiveDate)}`, {
    data: {
      effective_date: lateEffectiveDate,
      person_uuid: capacityPersonUUID,
      position_uuid: capacityPositionID,
      allocated_fte: "1.0"
    }
  });
  expect(assignmentCapacityResp.status(), await assignmentCapacityResp.text()).toBe(422);
  expect((await assignmentCapacityResp.json()).code).toBe("STAFFING_POSITION_CAPACITY_EXCEEDED");

  const reduceCapacityResp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(lateEffectiveDate)}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_uuid: capacityPositionID,
      capacity_fte: "0.25"
    }
  });
  expect(reduceCapacityResp.status(), await reduceCapacityResp.text()).toBe(422);
  expect((await reduceCapacityResp.json()).code).toBe("STAFFING_POSITION_CAPACITY_EXCEEDED");

  const disableConflictResp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(lateEffectiveDate)}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_uuid: capacityPositionID,
      lifecycle_status: "disabled"
    }
  });
  expect(disableConflictResp.status(), await disableConflictResp.text()).toBe(422);
  expect((await disableConflictResp.json()).code).toBe("STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF");

  // Valid-time empty timeline: pernr=106 has assignment only at lateEffectiveDate.
  {
    const p106 = personUUIDByPernr.get("106");
    expect(p106).toBeTruthy();

    const beforeResp = await appContext.request.get(
      `/org/api/assignments?as_of=${encodeURIComponent(asOf)}&person_uuid=${encodeURIComponent(p106)}`
    );
    expect(beforeResp.status(), await beforeResp.text()).toBe(200);
    expect((await beforeResp.json()).assignments).toHaveLength(0);

    const afterResp = await appContext.request.get(
      `/org/api/assignments?as_of=${encodeURIComponent(lateEffectiveDate)}&person_uuid=${encodeURIComponent(p106)}`
    );
    expect(afterResp.status(), await afterResp.text()).toBe(200);
    const afterJSON = await afterResp.json();
    expect(afterJSON.assignments.length).toBeGreaterThan(0);
    expect(afterJSON.assignments[0].effective_date).toBe(lateEffectiveDate);
  }

  // UI sanity checks (MUI-only pages)
  await page.goto(`/app/staffing/positions?as_of=${asOf}&org_code=${rootOrgCode}`);
  await expect(page.getByRole("heading", { level: 2, name: "Staffing / Positions" })).toBeVisible();
  await page.goto(`/app/staffing/assignments?as_of=${lateEffectiveDate}&pernr=106`);
  await expect(page.getByRole("heading", { level: 2, name: "Staffing / Assignments" })).toBeVisible();

  await appContext.close();
});
