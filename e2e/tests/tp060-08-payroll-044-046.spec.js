import { randomUUID } from "crypto";
import { expect, test } from "@playwright/test";

test("tp060-08: payroll 044-046 (balances -> retro -> net-guaranteed IIT -> finalize -> balances)", async ({ browser }) => {
  test.setTimeout(360_000);

  const asOf = "2026-01-01";
  const lateEffectiveDate = "2026-01-15";
  const runID = `${Date.now()}`;

  const tenantHost = `t-tp060-08-${runID}.localhost`;
  const tenantName = `TP060-08 Tenant ${runID}`;

  const tenantAdminEmail = `tenant-admin+068-${runID}@example.invalid`;
  const tenantViewerEmail = `tenant-viewer+068-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const tenantViewerPass = process.env.E2E_TENANT_VIEWER_PASS || tenantAdminPass;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const defaultSuperadminEmail = `admin+tp060-08-${runID}@example.invalid`;
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
  await ensureIdentity(
    superadminContext,
    `${tenantID}:${tenantViewerEmail}`,
    tenantViewerEmail,
    tenantViewerPass,
    { tenant_id: tenantID, role_slug: "tenant-viewer" }
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

  // OrgUnit + Positions (required for assignments)
  await page.goto(`/org/nodes?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit");

  const rootName = `TP060-08 Root ${runID}`;
  const createNodeForm = page.locator(`form[method="POST"][action="/org/nodes?as_of=${asOf}"]`).first();
  await createNodeForm.locator('input[name="effective_date"]').fill(asOf);
  await createNodeForm.locator('input[name="parent_id"]').fill("");
  await createNodeForm.locator('input[name="name"]').fill(rootName);
  await createNodeForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/nodes\\?as_of=${asOf}$`));

  const rootRow = page.locator("ul li", { hasText: rootName }).first();
  await expect(rootRow).toBeVisible();

  await page.goto(`/org/positions?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");

  const positionCreateForm = page.locator(`form[method="POST"][action="/org/positions?as_of=${asOf}"]`).first();
  const orgOptionValue = await positionCreateForm
    .locator('select[name="org_unit_id"] option', { hasText: rootName })
    .first()
    .getAttribute("value");
  expect(orgOptionValue).not.toBeNull();

  const positionIDsByPernr = new Map();
  for (const pernr of ["105", "106", "107"]) {
    const positionName = `TP060-08 Position ${pernr} ${runID}`;
    await positionCreateForm.locator('input[name="effective_date"]').fill(asOf);
    await positionCreateForm.locator('select[name="org_unit_id"]').selectOption(orgOptionValue);
    await positionCreateForm.locator('input[name="name"]').fill(positionName);
    await positionCreateForm.locator('button[type="submit"]').click();
    await expect(page).toHaveURL(new RegExp(`/org/positions\\?as_of=${asOf}$`));

    const row = page.locator("tr", { hasText: positionName }).first();
    await expect(row).toBeVisible();
    const positionID = (await row.locator("td").nth(1).innerText()).trim();
    expect(positionID).not.toBe("");
    positionIDsByPernr.set(pernr, positionID);
  }

  // Persons (E05/E06/E07)
  await page.goto(`/person/persons?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Person");

  const personUUIDByPernr = new Map();
  for (const pernr of ["105", "106", "107"]) {
    const displayName = `TP060-08 Person ${pernr} ${runID}`;
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

  // Assignments with base_salary / allocated_fte (input for payroll)
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
    await expect(page.locator("table").first()).toContainText(effectiveDate);
  };

  await upsertAssignment({ pernr: "105", effectiveDate: asOf, baseSalary: "30000.00", allocatedFte: "1.0" });
  await upsertAssignment({ pernr: "106", effectiveDate: lateEffectiveDate, baseSalary: "30000.00", allocatedFte: "1.0" });
  await upsertAssignment({ pernr: "107", effectiveDate: asOf, baseSalary: "30000.00", allocatedFte: "1.0" });

  // Social insurance policies (minimal deterministic setup)
  const siBaseFloor = "5000.00";
  const siBaseCeiling = "30001.00";
  const cityCode = "CN-310000";
  const hukouType = "default";
  const siPolicies = [
    { insurance_type: "PENSION", employer_rate: "0.160000", employee_rate: "0.080000", rounding_rule: "HALF_UP", precision: 2 },
    { insurance_type: "MEDICAL", employer_rate: "0.095530", employee_rate: "0.020070", rounding_rule: "CEIL", precision: 2 },
    { insurance_type: "UNEMPLOYMENT", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 },
    { insurance_type: "INJURY", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 },
    { insurance_type: "MATERNITY", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 },
    { insurance_type: "HOUSING_FUND", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 }
  ];
  for (const p of siPolicies) {
    const resp = await appContext.request.post("/org/api/payroll-social-insurance-policies", {
      data: {
        event_id: randomUUID(),
        city_code: cityCode,
        hukou_type: hukouType,
        insurance_type: p.insurance_type,
        effective_date: asOf,
        employer_rate: p.employer_rate,
        employee_rate: p.employee_rate,
        base_floor: siBaseFloor,
        base_ceiling: siBaseCeiling,
        rounding_rule: p.rounding_rule,
        precision: p.precision,
        rules_config: {}
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
  }

  // Payroll period/run: 2026-01
  await page.goto(`/org/payroll-periods?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Payroll Periods");
  const periodCreateForm = page.locator('form[method="POST"]').first();
  await periodCreateForm.locator('input[name="pay_group"]').fill("monthly");
  await periodCreateForm.locator('input[name="start_date"]').fill("2026-01-01");
  await periodCreateForm.locator('input[name="end_date_exclusive"]').fill("2026-02-01");
  await periodCreateForm.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-periods\\?as_of=${asOf}$`));

  const periodRowJan = page.locator("tr", { hasText: "2026-01-01" }).filter({ hasText: "2026-02-01" }).first();
  await expect(periodRowJan).toBeVisible();
  const payPeriodIDJan = (await periodRowJan.locator("code").first().innerText()).trim();
  expect(payPeriodIDJan).not.toBe("");

  await page.goto(`/org/payroll-runs?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Payroll Runs");
  const runCreateForm = page.locator('form[method="POST"]').first();
  await runCreateForm.locator('select[name="pay_period_id"]').selectOption(payPeriodIDJan);
  await runCreateForm.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-runs/[^?]+\\?as_of=${asOf}$`));

  const runURLJan = new URL(page.url());
  const runIDJan = runURLJan.pathname.replace("/org/payroll-runs/", "");
  expect(runIDJan).not.toBe("");

  await page.getByRole("button", { name: "Calculate" }).click();
  await page.waitForURL(new RegExp(`/org/payroll-runs/${runIDJan}\\?as_of=${asOf}$`), { timeout: 60_000 });
  await expect(page.locator("li", { hasText: "state:" })).toContainText("calculated");

  await page.getByRole("button", { name: "Finalize" }).click();
  await page.waitForURL(new RegExp(`/org/payroll-runs/${runIDJan}\\?as_of=${asOf}$`), { timeout: 60_000 });
  await expect(page.locator("li", { hasText: "state:" })).toContainText("finalized");

  const e06UUID = personUUIDByPernr.get("106");
  expect(e06UUID).toBeTruthy();
  const balancesJanResp = await appContext.request.get(
    `/org/api/payroll-balances?person_uuid=${encodeURIComponent(e06UUID)}&tax_year=2026`
  );
  expect(balancesJanResp.status(), await balancesJanResp.text()).toBe(200);
  const balancesJan = await balancesJanResp.json();
  expect(balancesJan.first_tax_month).toBe(1);
  expect(balancesJan.last_tax_month).toBe(1);
  expect(balancesJan.ytd_standard_deduction).toBe("5000.00");

  // SAD (optional but valuable): idempotency + finalized-month read-only.
  {
    const sadEventID = randomUUID();
    const okResp = await appContext.request.post("/org/api/payroll-iit-special-additional-deductions", {
      data: { event_id: sadEventID, person_uuid: e06UUID, tax_year: 2026, tax_month: 2, amount: "0.00" }
    });
    expect(okResp.status(), await okResp.text()).toBe(200);

    const idemConflict = await appContext.request.post("/org/api/payroll-iit-special-additional-deductions", {
      data: { event_id: sadEventID, person_uuid: e06UUID, tax_year: 2026, tax_month: 2, amount: "1.00" }
    });
    expect(idemConflict.status(), await idemConflict.text()).toBe(409);
    expect((await idemConflict.json()).code).toBe("STAFFING_IDEMPOTENCY_REUSED");

    const finalizedProbe = await appContext.request.post("/org/api/payroll-iit-special-additional-deductions", {
      data: { event_id: randomUUID(), person_uuid: e06UUID, tax_year: 2026, tax_month: 1, amount: "0.00" }
    });
    expect(finalizedProbe.status(), await finalizedProbe.text()).toBe(409);
    expect((await finalizedProbe.json()).code).toBe("STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED");
  }

  // Payroll period/run: 2026-02 (target for retro adjustments + continued balances)
  await page.goto(`/org/payroll-periods?as_of=${asOf}`);
  const periodCreateForm2 = page.locator('form[method="POST"]').first();
  await periodCreateForm2.locator('input[name="pay_group"]').fill("monthly");
  await periodCreateForm2.locator('input[name="start_date"]').fill("2026-02-01");
  await periodCreateForm2.locator('input[name="end_date_exclusive"]').fill("2026-03-01");
  await periodCreateForm2.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-periods\\?as_of=${asOf}$`));

  const periodRowFeb = page.locator("tr", { hasText: "2026-02-01" }).filter({ hasText: "2026-03-01" }).first();
  await expect(periodRowFeb).toBeVisible();
  const payPeriodIDFeb = (await periodRowFeb.locator("code").first().innerText()).trim();
  expect(payPeriodIDFeb).not.toBe("");

  await page.goto(`/org/payroll-runs?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Payroll Runs");
  const runCreateForm2 = page.locator('form[method="POST"]').first();
  await runCreateForm2.locator('select[name="pay_period_id"]').selectOption(payPeriodIDFeb);
  await runCreateForm2.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-runs/[^?]+\\?as_of=${asOf}$`));

  const runURLFeb = new URL(page.url());
  const runIDFeb = runURLFeb.pathname.replace("/org/payroll-runs/", "");
  expect(runIDFeb).not.toBe("");

  // Trigger retro: after finalize, submit an earlier effective_date change.
  await upsertAssignment({ pernr: "105", effectiveDate: lateEffectiveDate, baseSalary: "31000.00", allocatedFte: "1.0" });

  const e05UUID = personUUIDByPernr.get("105");
  expect(e05UUID).toBeTruthy();
  const listRecalcResp = await appContext.request.get(`/org/api/payroll-recalc-requests?person_uuid=${encodeURIComponent(e05UUID)}`);
  expect(listRecalcResp.status(), await listRecalcResp.text()).toBe(200);
  const recalcList = await listRecalcResp.json();
  const rr = recalcList.find((r) => r.person_uuid === e05UUID && r.effective_date === lateEffectiveDate);
  expect(rr, `missing recalc request for person_uuid=${e05UUID} effective_date=${lateEffectiveDate}`).toBeTruthy();
  expect(rr.hit_pay_period_id).toBe(payPeriodIDJan);
  expect(rr.applied).toBe(false);
  const recalcRequestID = rr.recalc_request_id;

  await page.goto(`/org/payroll-recalc-requests?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Payroll Recalc Requests");
  await expect(page.locator("tr", { hasText: recalcRequestID }).first()).toBeVisible();

  const applyResp = await appContext.request.post(`/org/api/payroll-recalc-requests/${encodeURIComponent(recalcRequestID)}:apply`, {
    data: { target_run_id: runIDFeb }
  });
  expect(applyResp.status(), await applyResp.text()).toBe(200);
  const applyJSON = await applyResp.json();
  expect(applyJSON.recalc_request_id).toBe(recalcRequestID);
  expect(applyJSON.target_run_id).toBe(runIDFeb);
  expect(applyJSON.target_pay_period_id).toBe(payPeriodIDFeb);

  const recalcDetailResp = await appContext.request.get(`/org/api/payroll-recalc-requests/${encodeURIComponent(recalcRequestID)}`);
  expect(recalcDetailResp.status(), await recalcDetailResp.text()).toBe(200);
  const recalcDetail = await recalcDetailResp.json();
  expect(recalcDetail.application).toBeTruthy();
  expect(recalcDetail.application.target_run_id).toBe(runIDFeb);
  expect(recalcDetail.adjustments_summary.length).toBeGreaterThan(0);

  // Authz: tenant-viewer must be forbidden for retro apply.
  const viewerContext = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL || "http://localhost:8080",
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const viewerLoginResp = await viewerContext.request.post("/login", {
    form: { email: tenantViewerEmail, password: tenantViewerPass },
    maxRedirects: 0
  });
  expect(viewerLoginResp.status()).toBe(302);

  const viewerApplyResp = await viewerContext.request.post(`/org/api/payroll-recalc-requests/${encodeURIComponent(recalcRequestID)}:apply`, {
    data: { target_run_id: runIDFeb }
  });
  expect(viewerApplyResp.status()).toBe(403);
  expect((await viewerApplyResp.json()).code).toBe("forbidden");
  await viewerContext.close();

  // Calculate 2026-02 run (required to create payslips)
  await page.goto(`/org/payroll-runs/${encodeURIComponent(runIDFeb)}?as_of=${asOf}`);
  await page.getByRole("button", { name: "Calculate" }).click();
  await page.waitForURL(new RegExp(`/org/payroll-runs/${runIDFeb}\\?as_of=${asOf}$`), { timeout: 60_000 });
  await expect(page.locator("li", { hasText: "state:" })).toContainText("calculated");

  const listPayslipsResp = await appContext.request.get(`/org/api/payslips?run_id=${encodeURIComponent(runIDFeb)}`);
  expect(listPayslipsResp.status(), await listPayslipsResp.text()).toBe(200);
  const payslipsFeb = await listPayslipsResp.json();

  const findPayslipIDByPernr = (pernr) => {
    const personUUID = personUUIDByPernr.get(pernr);
    expect(personUUID).toBeTruthy();
    const slip = payslipsFeb.find((p) => p.person_uuid === personUUID);
    expect(slip, `missing payslip for pernr=${pernr}`).toBeTruthy();
    return slip.id;
  };

  const e07PayslipID = findPayslipIDByPernr("107");

  // Net-guaranteed IIT input (UI)
  const netGuaranteedRequestID = randomUUID();
  const ngPostURL = `/org/payroll-runs/${encodeURIComponent(runIDFeb)}/payslips/${encodeURIComponent(e07PayslipID)}/net-guaranteed-iit-items`;
  const ngPostResp = await appContext.request.post(ngPostURL, {
    form: {
      event_type: "UPSERT",
      item_code: "EARNING_LONG_SERVICE_AWARD",
      target_net: "20000.00",
      request_id: netGuaranteedRequestID
    },
    maxRedirects: 0
  });
  expect(ngPostResp.status(), await ngPostResp.text()).toBe(303);

  const ngIdemResp = await appContext.request.post(ngPostURL, {
    form: {
      event_type: "UPSERT",
      item_code: "EARNING_LONG_SERVICE_AWARD",
      target_net: "20000.00",
      request_id: netGuaranteedRequestID
    },
    maxRedirects: 0
  });
  expect(ngIdemResp.status(), await ngIdemResp.text()).toBe(303);

  const ngConflictResp = await appContext.request.post(
    `/org/api/payroll-runs/${encodeURIComponent(runIDFeb)}/payslips/${encodeURIComponent(e07PayslipID)}/net-guaranteed-iit-items`,
    {
      data: {
        event_type: "UPSERT",
        item_code: "EARNING_LONG_SERVICE_AWARD",
        target_net: "20001.00",
        request_id: netGuaranteedRequestID
      }
    }
  );
  expect(ngConflictResp.status(), await ngConflictResp.text()).toBe(409);
  expect((await ngConflictResp.json()).code).toBe("STAFFING_IDEMPOTENCY_REUSED");

  const runFebAfterNGResp = await appContext.request.get(`/org/api/payroll-runs?pay_period_id=${encodeURIComponent(payPeriodIDFeb)}`);
  expect(runFebAfterNGResp.status(), await runFebAfterNGResp.text()).toBe(200);
  const runFebAfterNG = (await runFebAfterNGResp.json()).find((r) => r.id === runIDFeb);
  expect(runFebAfterNG).toBeTruthy();
  expect(runFebAfterNG.needs_recalc).toBe(true);

  // Re-calculate to apply net guarantee
  await page.getByRole("button", { name: "Calculate" }).click();
  await page.waitForURL(new RegExp(`/org/payroll-runs/${runIDFeb}\\?as_of=${asOf}$`), { timeout: 60_000 });
  await expect(page.locator("li", { hasText: "state:" })).toContainText("calculated");

  const e07DetailResp = await appContext.request.get(`/org/api/payslips/${encodeURIComponent(e07PayslipID)}`);
  expect(e07DetailResp.status(), await e07DetailResp.text()).toBe(200);
  const e07Detail = await e07DetailResp.json();
  const ngItem = e07Detail.items.find((it) => it.calc_mode === "net_guaranteed_iit" && it.item_code === "EARNING_LONG_SERVICE_AWARD");
  expect(ngItem).toBeTruthy();
  expect(ngItem.target_net).toBe("20000.00");
  expect(ngItem.iit_delta).not.toBe("");
  expect(ngItem.amount).not.toBe("");
  expect(ngItem.meta).not.toBeNull();

  const toCents = (s) => {
    const [a, b = ""] = `${s}`.split(".");
    const cents = (b + "00").slice(0, 2);
    return BigInt(`${a}${cents}`);
  };
  expect(toCents(ngItem.amount) - toCents(ngItem.iit_delta)).toBe(toCents(ngItem.target_net));
  const metaObj = typeof ngItem.meta === "string" ? JSON.parse(ngItem.meta) : ngItem.meta;
  expect(`${metaObj.tax_year}`).toBe("2026");

  // fail-closed: missing tenant context must not leak data or write (keep session cookie, but drop tenant host)
  const adminState = await appContext.storageState();
  const noTenantContext = await browser.newContext({
    baseURL: "http://127.0.0.1:8080",
    storageState: adminState
  });
  const noTenantBalancesResp = await noTenantContext.request.get(
    `/org/api/payroll-balances?person_uuid=${encodeURIComponent(e06UUID)}&tax_year=2026`,
    { maxRedirects: 0 }
  );
  const noTenantBalancesStatus = noTenantBalancesResp.status();
  expect(noTenantBalancesStatus === 302 || noTenantBalancesStatus === 303 || noTenantBalancesStatus >= 400).toBeTruthy();

  const noTenantWriteResp = await noTenantContext.request.post(ngPostURL, {
    form: {
      event_type: "UPSERT",
      item_code: "EARNING_LONG_SERVICE_AWARD",
      target_net: "19999.00",
      request_id: randomUUID()
    },
    maxRedirects: 0
  });
  const s = noTenantWriteResp.status();
  expect(s === 302 || s >= 400).toBeTruthy();
  await noTenantContext.close();

  const e07DetailAfterResp = await appContext.request.get(`/org/api/payslips/${encodeURIComponent(e07PayslipID)}`);
  expect(e07DetailAfterResp.status(), await e07DetailAfterResp.text()).toBe(200);
  const e07DetailAfter = await e07DetailAfterResp.json();
  const ngItemAfter = e07DetailAfter.items.find((it) => it.calc_mode === "net_guaranteed_iit" && it.item_code === "EARNING_LONG_SERVICE_AWARD");
  expect(ngItemAfter).toBeTruthy();
  expect(ngItemAfter.target_net).toBe("20000.00");

  // Finalize 2026-02 and assert balances advance month
  await page.getByRole("button", { name: "Finalize" }).click();
  await page.waitForURL(new RegExp(`/org/payroll-runs/${runIDFeb}\\?as_of=${asOf}$`), { timeout: 60_000 });
  await expect(page.locator("li", { hasText: "state:" })).toContainText("finalized");

  const balancesFebResp = await appContext.request.get(
    `/org/api/payroll-balances?person_uuid=${encodeURIComponent(e06UUID)}&tax_year=2026`
  );
  expect(balancesFebResp.status(), await balancesFebResp.text()).toBe(200);
  const balancesFeb = await balancesFebResp.json();
  expect(balancesFeb.last_tax_month).toBe(2);
  expect(balancesFeb.ytd_standard_deduction).toBe("10000.00");

  // Assert retro adjustments are visible in payslip items and trace origin.
  const e05PayslipID = findPayslipIDByPernr("105");
  const e05DetailResp = await appContext.request.get(`/org/api/payslips/${encodeURIComponent(e05PayslipID)}`);
  expect(e05DetailResp.status(), await e05DetailResp.text()).toBe(200);
  const e05Detail = await e05DetailResp.json();
  const originItems = e05Detail.items.filter((it) => {
    try {
      const meta = typeof it.meta === "string" ? JSON.parse(it.meta || "{}") : it.meta || {};
      return `${meta.origin_pay_period_id || ""}` === payPeriodIDJan;
    } catch {
      return false;
    }
  });
  expect(originItems.length).toBeGreaterThan(0);

  await page.screenshot({ path: `_artifacts/tp060-08-payroll-${runID}.png`, fullPage: true });
  await appContext.close();
});
