import { randomUUID } from "crypto";
import { expect, test } from "@playwright/test";

test("tp060-07: payroll 041-043 (period/run -> payslip -> social insurance -> finalize)", async ({ browser }) => {
  test.setTimeout(300_000);

  const asOf = "2026-01-01";
  const runID = `${Date.now()}`;

  const tenantHost = `t-tp060-07-${runID}.localhost`;
  const tenantName = `TP060-07 Tenant ${runID}`;

  const tenantAdminEmail = `tenant-admin+067-${runID}@example.invalid`;
  const tenantViewerEmail = `tenant-viewer+067-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const tenantViewerPass = process.env.E2E_TENANT_VIEWER_PASS || tenantAdminPass;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const defaultSuperadminEmail = `admin+tp060-07-${runID}@example.invalid`;
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
    await expect(tenantRow).toBeVisible({ timeout: 15000 });
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

  const rootName = `TP060-07 Root ${runID}`;
  const createNodeForm = page.locator(`form[method="POST"][action="/org/nodes?as_of=${asOf}"]`).first();
  await createNodeForm.locator('input[name="effective_date"]').fill(asOf);
  await createNodeForm.locator('input[name="parent_id"]').fill("");
  await createNodeForm.locator('input[name="name"]').fill(rootName);
  await createNodeForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/nodes\\?as_of=${asOf}$`));

  const rootRow = page.locator("ul li", { hasText: rootName }).first();
  await expect(rootRow).toBeVisible();
  const rootOrgID = (await rootRow.locator("code").first().innerText()).trim();
  expect(rootOrgID).not.toBe("");

  await page.goto(`/org/positions?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Staffing / Positions");

  const positionCreateForm = page.locator(`form[method="POST"][action="/org/positions?as_of=${asOf}"]`).first();
  const orgOptionValue = await positionCreateForm
    .locator('select[name="org_unit_id"] option', { hasText: rootName })
    .first()
    .getAttribute("value");
  expect(orgOptionValue).not.toBeNull();

  const positionIDsByPernr = new Map();
  for (const pernr of ["102", "103", "104"]) {
    const positionName = `TP060-07 Position ${pernr} ${runID}`;
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

  // Persons
  await page.goto(`/person/persons?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Person");

  const personUUIDByPernr = new Map();
  for (const pernr of ["102", "103", "104"]) {
    const displayName = `TP060-07 Person ${pernr} ${runID}`;
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

  const upsertAssignment = async ({ pernr, baseSalary, allocatedFte }) => {
    await page.goto(`/org/assignments?as_of=${asOf}`);
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

    await upsertForm.locator('input[name="effective_date"]').fill(asOf);
    await upsertForm.locator('select[name="position_id"]').selectOption(positionID);
    await upsertForm.locator('input[name="allocated_fte"]').fill(allocatedFte);
    await upsertForm.locator('input[name="base_salary"]').fill(baseSalary);
    await upsertForm.getByRole("button", { name: "Submit" }).click();
    await expect(page).toHaveURL(new RegExp(`/org/assignments\\?as_of=${asOf}&pernr=${pernr}$`));

    const table = page.locator("table").first();
    await expect(table).toContainText(asOf);
  };

  await upsertAssignment({ pernr: "102", baseSalary: "80000.00", allocatedFte: "1.0" });
  await upsertAssignment({ pernr: "103", baseSalary: "3000.00", allocatedFte: "1.0" });
  await upsertAssignment({ pernr: "104", baseSalary: "20000.00", allocatedFte: "0.5" });

  // Social insurance policies (use internal API for deterministic setup)
  const siBaseFloor = "5000.00";
  const siBaseCeiling = "30001.00";
  const cityCode = "CN-310000";
  const hukouType = "default";

  const siPolicies = [
    {
      insurance_type: "PENSION",
      employer_rate: "0.160000",
      employee_rate: "0.080000",
      rounding_rule: "HALF_UP",
      precision: 2
    },
    {
      insurance_type: "MEDICAL",
      employer_rate: "0.095530",
      employee_rate: "0.020070",
      rounding_rule: "CEIL",
      precision: 2
    },
    { insurance_type: "UNEMPLOYMENT", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 },
    { insurance_type: "INJURY", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 },
    { insurance_type: "MATERNITY", employer_rate: "0.000000", employee_rate: "0.000000", rounding_rule: "HALF_UP", precision: 2 },
    {
      insurance_type: "HOUSING_FUND",
      employer_rate: "0.000000",
      employee_rate: "0.000000",
      rounding_rule: "HALF_UP",
      precision: 2
    }
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

  const siListResp = await appContext.request.get(`/org/api/payroll-social-insurance-policies?as_of=${asOf}`);
  expect(siListResp.status(), await siListResp.text()).toBe(200);
  const siVersions = await siListResp.json();
  expect(siVersions.length).toBe(6);

  // Payroll period (UI)
  await page.goto(`/org/payroll-periods?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Payroll Periods");
  const periodCreateForm = page.locator('form[method="POST"]').first();
  await periodCreateForm.locator('input[name="pay_group"]').fill("monthly");
  await periodCreateForm.locator('input[name="start_date"]').fill("2026-01-01");
  await periodCreateForm.locator('input[name="end_date_exclusive"]').fill("2026-02-01");
  await periodCreateForm.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-periods\\?as_of=${asOf}$`));

  const periodRow = page.locator("tr", { hasText: "2026-01-01" }).filter({ hasText: "2026-02-01" }).first();
  await expect(periodRow).toBeVisible();
  const payPeriodID = (await periodRow.locator("code").first().innerText()).trim();
  expect(payPeriodID).not.toBe("");

  // Payroll run (UI)
  await page.goto(`/org/payroll-runs?as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("Payroll Runs");
  const runCreateForm = page.locator('form[method="POST"]').first();
  await runCreateForm.locator('select[name="pay_period_id"]').selectOption(payPeriodID);
  await runCreateForm.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-runs/[^?]+\\?as_of=${asOf}$`));

  const runURL = new URL(page.url());
  const runIDCreated = runURL.pathname.replace("/org/payroll-runs/", "");
  expect(runIDCreated).not.toBe("");

  // Calculate (UI action)
  await page.getByRole("button", { name: "Calculate" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-runs/${runIDCreated}\\?as_of=${asOf}$`));
  await expect(page.locator("li", { hasText: "state:" })).toContainText("calculated");

  // Payslips + assertions (use internal APIs for stable checks)
  const listPayslipsResp = await appContext.request.get(`/org/api/payslips?run_id=${encodeURIComponent(runIDCreated)}`);
  expect(listPayslipsResp.status(), await listPayslipsResp.text()).toBe(200);
  const payslips = await listPayslipsResp.json();
  expect(payslips.length).toBe(3);

  const findPayslipIDByPernr = (pernr) => {
    const personUUID = personUUIDByPernr.get(pernr);
    expect(personUUID).toBeTruthy();
    const slip = payslips.find((p) => p.person_uuid === personUUID);
    expect(slip, `missing payslip for pernr=${pernr}`).toBeTruthy();
    return slip.id;
  };

  const getPayslipDetail = async (payslipID) => {
    const resp = await appContext.request.get(`/org/api/payslips/${encodeURIComponent(payslipID)}`);
    expect(resp.status(), await resp.text()).toBe(200);
    return await resp.json();
  };

  // E04: FTE 0.5 => base salary item is 10,000.00
  const e04PayslipID = findPayslipIDByPernr("104");
  const e04Detail = await getPayslipDetail(e04PayslipID);
  expect(e04Detail.gross_pay).toBe("10000.00");
  const e04BaseItem = e04Detail.items.find((it) => it.item_code === "EARNING_BASE_SALARY");
  expect(e04BaseItem).toBeTruthy();
  expect(e04BaseItem.amount).toBe("10000.00");

  // E03: clamp to base_floor=5,000.00
  const e03PayslipID = findPayslipIDByPernr("103");
  const e03Detail = await getPayslipDetail(e03PayslipID);
  const e03Pension = e03Detail.social_insurance_items.find((it) => it.insurance_type === "PENSION");
  const e03Medical = e03Detail.social_insurance_items.find((it) => it.insurance_type === "MEDICAL");
  expect(e03Pension).toBeTruthy();
  expect(e03Medical).toBeTruthy();
  expect(e03Pension.base_amount).toBe("5000.00");
  expect(e03Pension.employee_amount).toBe("400.00");
  expect(e03Pension.employer_amount).toBe("800.00");
  expect(e03Medical.base_amount).toBe("5000.00");
  expect(e03Medical.employee_amount).toBe("100.35");
  expect(e03Medical.employer_amount).toBe("477.65");

  // E02: clamp to base_ceiling=30,001.00
  const e02PayslipID = findPayslipIDByPernr("102");
  const e02Detail = await getPayslipDetail(e02PayslipID);
  const e02Pension = e02Detail.social_insurance_items.find((it) => it.insurance_type === "PENSION");
  const e02Medical = e02Detail.social_insurance_items.find((it) => it.insurance_type === "MEDICAL");
  expect(e02Pension).toBeTruthy();
  expect(e02Medical).toBeTruthy();
  expect(e02Pension.base_amount).toBe("30001.00");
  expect(e02Pension.employee_amount).toBe("2400.08");
  expect(e02Pension.employer_amount).toBe("4800.16");
  expect(e02Medical.base_amount).toBe("30001.00");
  expect(e02Medical.employee_amount).toBe("602.13");
  expect(e02Medical.employer_amount).toBe("2866.00");

  // Finalize (UI action)
  await page.getByRole("button", { name: "Finalize" }).click();
  await expect(page).toHaveURL(new RegExp(`/org/payroll-runs/${runIDCreated}\\?as_of=${asOf}$`));
  await expect(page.locator("li", { hasText: "state:" })).toContainText("finalized");

  // Pay period should be closed after finalize
  await page.goto(`/org/payroll-periods?as_of=${asOf}`);
  const closedRow = page.locator("tr", { hasText: payPeriodID }).first();
  await expect(closedRow).toBeVisible();
  await expect(closedRow).toContainText("closed");

  // finalized read-only: calculate/finalize again must not change state/timestamps
  const runListBeforeResp = await appContext.request.get(`/org/api/payroll-runs?pay_period_id=${encodeURIComponent(payPeriodID)}`);
  expect(runListBeforeResp.status(), await runListBeforeResp.text()).toBe(200);
  const runListBefore = await runListBeforeResp.json();
  const before = runListBefore.find((r) => r.id === runIDCreated);
  expect(before).toBeTruthy();

  await appContext.request.post(`/org/payroll-runs/${encodeURIComponent(runIDCreated)}/calculate?as_of=${asOf}`, {
    headers: { Accept: "application/json" }
  });
  await appContext.request.post(`/org/payroll-runs/${encodeURIComponent(runIDCreated)}/finalize?as_of=${asOf}`, {
    headers: { Accept: "application/json" }
  });

  const runListAfterResp = await appContext.request.get(`/org/api/payroll-runs?pay_period_id=${encodeURIComponent(payPeriodID)}`);
  expect(runListAfterResp.status(), await runListAfterResp.text()).toBe(200);
  const runListAfter = await runListAfterResp.json();
  const after = runListAfter.find((r) => r.id === runIDCreated);
  expect(after).toBeTruthy();
  expect(after.run_state).toBe("finalized");
  expect(after.calc_finished_at).toBe(before.calc_finished_at);
  expect(after.finalized_at).toBe(before.finalized_at);

  // Authz: tenant-viewer must be forbidden for writes
  const viewerContext = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL || "http://localhost:8080",
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const viewerLoginResp = await viewerContext.request.post("/login", {
    form: { email: tenantViewerEmail, password: tenantViewerPass },
    maxRedirects: 0
  });
  expect(viewerLoginResp.status()).toBe(302);

  const viewerCreatePeriodResp = await viewerContext.request.post(`/org/payroll-periods?as_of=${asOf}`, {
    headers: { Accept: "application/json" },
    form: { pay_group: "monthly", start_date: "2098-01-01", end_date_exclusive: "2098-02-01" },
    maxRedirects: 0
  });
  expect(viewerCreatePeriodResp.status()).toBe(403);
  expect((await viewerCreatePeriodResp.json()).code).toBe("forbidden");
  await viewerContext.close();

  // fail-closed: missing tenant context must not allow writes (and must not create data)
  const periodListBeforeResp = await appContext.request.get("/org/api/payroll-periods");
  expect(periodListBeforeResp.status(), await periodListBeforeResp.text()).toBe(200);
  const periodsBefore = await periodListBeforeResp.json();
  expect(periodsBefore.some((p) => p.start_date === "2099-01-01")).toBeFalsy();

  const noTenantContext = await browser.newContext({ baseURL: process.env.E2E_BASE_URL || "http://localhost:8080" });
  const noTenantPostResp = await noTenantContext.request.post("/org/payroll-periods", {
    headers: { Accept: "application/json" },
    form: { pay_group: "monthly", start_date: "2099-01-01", end_date_exclusive: "2099-02-01" },
    maxRedirects: 0
  });
  // fail-closed is satisfied by either:
  // - redirect to /login (unauthenticated), or
  // - an explicit 4xx/5xx error (tenant missing, forbidden, etc).
  const noTenantStatus = noTenantPostResp.status();
  expect(noTenantStatus === 302 || noTenantStatus === 303 || noTenantStatus >= 400).toBeTruthy();
  await noTenantContext.close();

  const periodListAfterResp = await appContext.request.get("/org/api/payroll-periods");
  expect(periodListAfterResp.status(), await periodListAfterResp.text()).toBe(200);
  const periodsAfter = await periodListAfterResp.json();
  expect(periodsAfter.some((p) => p.start_date === "2099-01-01")).toBeFalsy();

  await page.screenshot({ path: `_artifacts/tp060-07-payroll-run-${runID}.png`, fullPage: true });
  await appContext.close();
});
