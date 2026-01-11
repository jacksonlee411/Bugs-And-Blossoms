import { expect, test } from "@playwright/test";

test("tp060-03: person + assignments (with base_salary/allocated_fte)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const lateEffectiveDate = "2026-01-15";
  const runID = `${Date.now()}`;

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

  const ensureTenant = async (hostname, name) => {
    await superadminPage.goto("/superadmin/tenants");
    const tenantRow = superadminPage.locator("tr", { hasText: hostname }).first();
    if ((await tenantRow.count()) === 0) {
      await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(name);
      await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(hostname);
      await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
      await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
    }
    await expect(tenantRow).toBeVisible({ timeout: 60000 });
    const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
    expect(tenantID).not.toBe("");
    return tenantID;
  };

  const tenantID = await ensureTenant(tenantHost, tenantName);

  await ensureIdentity(
    superadminContext,
    `${tenantID}:${tenantAdminEmail}`,
    tenantAdminEmail,
    tenantAdminPass,
    { tenant_id: tenantID, role_slug: "tenant-admin" }
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

  await page.goto(`/org/nodes?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit");

  const createOrgUnit = async (effectiveDate, parentID, name) => {
    const form = page.locator(`form[method="POST"][action="/org/nodes?as_of=${asOf}"]`).first();
    await form.locator('input[name="effective_date"]').fill(effectiveDate);
    await form.locator('input[name="parent_id"]').fill(parentID);
    await form.locator('input[name="name"]').fill(name);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/nodes\\?as_of=${asOf}$`));
  };

  const rootName = `TP060-03 Root ${runID}`;
  await createOrgUnit(asOf, "", rootName);

  const rootID = (await page.locator("ul li", { hasText: rootName }).first().locator("code").first().innerText()).trim();
  expect(rootID).not.toBe("");

  await page.goto(`/org/positions?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");

  const positionCreateForm = page.locator(`form[method="POST"][action="/org/positions?as_of=${asOf}"]`).first();
  const orgOptionValue = await positionCreateForm
    .locator('select[name="org_unit_id"] option', { hasText: rootName })
    .first()
    .getAttribute("value");
  expect(orgOptionValue).not.toBeNull();

  const pernrByIndex = Array.from({ length: 10 }, (_, i) => `${101 + i}`);
  const positionIDsByPernr = new Map();
  for (const pernr of pernrByIndex) {
    const positionName = `TP060-03 Position ${pernr} ${runID}`;
    await positionCreateForm.locator('input[name="effective_date"]').fill(asOf);
    await positionCreateForm.locator('select[name="org_unit_id"]').selectOption(orgOptionValue);
    await positionCreateForm.locator('input[name="capacity_fte"]').fill(pernr === "104" ? "0.50" : "1.0");
    await positionCreateForm.locator('input[name="name"]').fill(positionName);
    await positionCreateForm.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${asOf}$`));

    const row = page.locator("tr", { hasText: positionName }).first();
    await expect(row).toBeVisible();
    const positionID = (await row.locator("td").nth(1).innerText()).trim();
    expect(positionID).not.toBe("");
    positionIDsByPernr.set(pernr, positionID);
  }

  const disabledPositionName = `TP060-03 Disabled Position ${runID}`;
  await positionCreateForm.locator('input[name="effective_date"]').fill(asOf);
  await positionCreateForm.locator('select[name="org_unit_id"]').selectOption(orgOptionValue);
  await positionCreateForm.locator('input[name="name"]').fill(disabledPositionName);
  await positionCreateForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${asOf}$`));

  const disabledRow = page.locator("tr", { hasText: disabledPositionName }).first();
  await expect(disabledRow).toBeVisible();
  const disabledPositionID = (await disabledRow.locator("td").nth(1).innerText()).trim();
  expect(disabledPositionID).not.toBe("");

  await page.goto(`/person/persons?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Person");

  const personUUIDByPernr = new Map();
  for (const pernr of pernrByIndex) {
    const displayName = `TP060-03 Person ${pernr} ${runID}`;
    const form = page.locator(`form[action="/person/persons?as_of=${asOf}"]`).first();
    await form.locator('input[name="pernr"]').fill(pernr);
    await form.locator('input[name="display_name"]').fill(displayName);
    await form.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/person/persons\\?as_of=${asOf}$`));

    const row = page.locator("tr", { hasText: pernr }).first();
    await expect(row).toBeVisible();
    const personUUID = (await row.locator("code").first().innerText()).trim();
    expect(personUUID).not.toBe("");
    personUUIDByPernr.set(pernr, personUUID);
  }

  await page.goto(`/org/positions?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");

  const positionUpdateForm = page.locator(`form[method="POST"][action="/org/positions?as_of=${asOf}"]`).nth(1);
  await positionUpdateForm.locator('input[name="effective_date"]').fill(lateEffectiveDate);
  await positionUpdateForm.locator('select[name="position_id"]').selectOption(disabledPositionID);
  await positionUpdateForm.locator('select[name="lifecycle_status"]').selectOption("disabled");
  await positionUpdateForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${lateEffectiveDate}$`));

  await expect(page.locator("tr", { hasText: disabledPositionName }).first()).toContainText("disabled");

  const positionUpdateFormLate = page.locator(`form[method="POST"][action="/org/positions?as_of=${lateEffectiveDate}"]`).nth(1);

  const managerPernr = "101";
  const reporteePernr = "102";
  const managerPositionName = `TP060-03 Position ${managerPernr} ${runID}`;
  const reporteePositionName = `TP060-03 Position ${reporteePernr} ${runID}`;
  const managerPositionID = positionIDsByPernr.get(managerPernr);
  const reporteePositionID = positionIDsByPernr.get(reporteePernr);
  expect(managerPositionID).not.toBeUndefined();
  expect(reporteePositionID).not.toBeUndefined();

  await positionUpdateFormLate.locator('input[name="effective_date"]').fill(lateEffectiveDate);
  await positionUpdateFormLate.locator('select[name="position_id"]').selectOption(reporteePositionID);
  await positionUpdateFormLate.locator('select[name="reports_to_position_id"]').selectOption(managerPositionID);
  await positionUpdateFormLate.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${lateEffectiveDate}$`));

  const reporteeRow = page.locator("tr", { hasText: reporteePositionName }).first();
  await expect(reporteeRow).toBeVisible();
  await expect(reporteeRow).toContainText(managerPositionID);

  const reportsToCycleResp = await appContext.request.post(`/org/api/positions?as_of=${lateEffectiveDate}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_id: managerPositionID,
      reports_to_position_id: reporteePositionID
    }
  });
  expect(reportsToCycleResp.status()).toBe(422);
  expect((await reportsToCycleResp.json()).code).toBe("STAFFING_POSITION_REPORTS_TO_CYCLE");

  const reportsToSelfResp = await appContext.request.post(`/org/api/positions?as_of=${lateEffectiveDate}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_id: managerPositionID,
      reports_to_position_id: managerPositionID
    }
  });
  expect(reportsToSelfResp.status()).toBe(422);
  expect((await reportsToSelfResp.json()).code).toBe("STAFFING_POSITION_REPORTS_TO_SELF");

  const reportsToRetroEffectiveDate = "2026-01-10";
  const reportsToRetroResp = await appContext.request.post(`/org/api/positions?as_of=${reportsToRetroEffectiveDate}`, {
    data: {
      effective_date: reportsToRetroEffectiveDate,
      position_id: reporteePositionID,
      reports_to_position_id: managerPositionID
    }
  });
  expect(reportsToRetroResp.status()).toBe(422);
  expect((await reportsToRetroResp.json()).code).toBe("STAFFING_INVALID_ARGUMENT");

  const byPernr = async (pernr) => {
    const resp = await appContext.request.get(`/person/api/persons:by-pernr?pernr=${encodeURIComponent(pernr)}`);
    return resp;
  };

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

  await page.screenshot({ path: `_artifacts/tp060-03-persons-${runID}.png`, fullPage: true });

  const assignmentDisabledResp = await appContext.request.post(`/org/api/assignments?as_of=${lateEffectiveDate}`, {
    data: {
      effective_date: lateEffectiveDate,
      person_uuid: personUUIDByPernr.get("101"),
      position_id: disabledPositionID,
      base_salary: "0",
      allocated_fte: "1.0"
    }
  });
  expect(assignmentDisabledResp.status()).toBe(422);
  expect((await assignmentDisabledResp.json()).code).toBe("STAFFING_POSITION_DISABLED_AS_OF");

  await page.goto(`/org/assignments?as_of=${lateEffectiveDate}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Assignments");
  await expect(page.locator(`select[name="position_id"] option[value="${disabledPositionID}"]`)).toHaveCount(0);

  await page.goto(`/org/assignments?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Assignments");

  const upsertAssignment = async ({ pernr, effectiveDate, baseSalary, allocatedFte }) => {
    await page.goto(`/org/assignments?as_of=${asOf}`);
    await expect(page.locator("h1")).toHaveText("Staffing / Assignments");

    const loadForm = page
      .locator('form[method="GET"][action="/org/assignments"]')
      .filter({ has: page.getByRole("button", { name: "Load" }) })
      .first();
    const upsertForm = page
      .locator('form[method="POST"]')
      .filter({ has: page.getByRole("button", { name: "Submit" }) })
      .first();

    await loadForm.locator('input[name="pernr"][type="text"]').fill(pernr);
    await loadForm.getByRole("button", { name: "Load" }).click();
    await expect(page).toHaveURL(new RegExp(`/org/assignments\\?as_of=${asOf}&pernr=${pernr}$`));

    const positionID = positionIDsByPernr.get(pernr);
    expect(positionID).toBeTruthy();

    await upsertForm.locator('input[name="effective_date"]').fill(effectiveDate);
    await upsertForm.locator('select[name="position_id"]').selectOption(positionID);
    await upsertForm.locator('input[name="allocated_fte"]').fill(allocatedFte);
    await upsertForm.locator('input[name="base_salary"]').fill(baseSalary);
    await upsertForm.getByRole("button", { name: "Submit" }).click();
    await expect(page).toHaveURL(new RegExp(`/org/assignments\\?as_of=${effectiveDate}&pernr=${pernr}$`));

    await expect(page.locator("h2", { hasText: "Timeline" })).toBeVisible();
    const table = page.locator("table").first();
    await expect(table).toContainText(effectiveDate);
    await expect(table).not.toContainText("end_date");
  };

  for (const pernr of pernrByIndex) {
    const isE04 = pernr === "104";
    const isE06 = pernr === "106";
    await upsertAssignment({
      pernr,
      effectiveDate: isE06 ? lateEffectiveDate : asOf,
      baseSalary: isE04 ? "30000.00" : "20000.00",
      allocatedFte: isE04 ? "0.5" : "1.0"
    });
  }

  const capacityPositionID = positionIDsByPernr.get("104");
  expect(capacityPositionID).toBeTruthy();
  const capacityPersonUUID = personUUIDByPernr.get("104");
  expect(capacityPersonUUID).toBeTruthy();

  const assignmentCapacityResp = await appContext.request.post(`/org/api/assignments?as_of=${lateEffectiveDate}`, {
    data: {
      effective_date: lateEffectiveDate,
      person_uuid: capacityPersonUUID,
      position_id: capacityPositionID,
      base_salary: "0",
      allocated_fte: "1.0"
    }
  });
  expect(assignmentCapacityResp.status(), await assignmentCapacityResp.text()).toBe(422);
  expect((await assignmentCapacityResp.json()).code).toBe("STAFFING_POSITION_CAPACITY_EXCEEDED");

  const reduceCapacityResp = await appContext.request.post(`/org/api/positions?as_of=${lateEffectiveDate}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_id: capacityPositionID,
      capacity_fte: "0.25"
    }
  });
  expect(reduceCapacityResp.status(), await reduceCapacityResp.text()).toBe(422);
  expect((await reduceCapacityResp.json()).code).toBe("STAFFING_POSITION_CAPACITY_EXCEEDED");

  const disableConflictResp = await appContext.request.post(`/org/api/positions?as_of=${lateEffectiveDate}`, {
    data: {
      effective_date: lateEffectiveDate,
      position_id: capacityPositionID,
      lifecycle_status: "disabled"
    }
  });
  expect(disableConflictResp.status(), await disableConflictResp.text()).toBe(422);
  expect((await disableConflictResp.json()).code).toBe("STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF");

  await page.screenshot({ path: `_artifacts/tp060-03-assignments-${runID}.png`, fullPage: true });

  await page.goto(`/org/assignments?as_of=${asOf}&pernr=106`);
  await expect(page.locator("h1")).toHaveText("Staffing / Assignments");
  await expect(page.locator("h2", { hasText: "Timeline" })).toBeVisible();
  const timelineEmpty = page.locator("h2", { hasText: "Timeline" }).locator("xpath=following-sibling::p[1]");
  await expect(timelineEmpty).toHaveText("(empty)");

  await page.goto(`/org/assignments?as_of=${lateEffectiveDate}&pernr=106`);
  await expect(page.locator("h1")).toHaveText("Staffing / Assignments");
  await expect(page.locator("table").first()).toContainText(lateEffectiveDate);
  await page.screenshot({ path: `_artifacts/tp060-03-valid-time-${runID}.png`, fullPage: true });

  await appContext.close();
});
