import { expect, test } from "@playwright/test";

test("tp060-05: attendance 4B-4E (daily results + config + time bank + corrections)", async ({ browser }) => {
  test.setTimeout(300_000);

  const personAsOf = "2026-01-01";
  const month = "2026-01";
  const dHoliday = "2026-01-01";
  const dWorkday = "2026-01-02";
  const dRestday = "2026-01-03";
  const runID = `${Date.now()}`;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp060-05-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;

  const baseURL = process.env.E2E_BASE_URL || "http://localhost:8080";

  const t060Host = `t-tp060-05-${runID}.localhost`;
  const t060Name = `TP060-05 ${runID}`;
  const t060AdminEmail = `tenant-admin+065-${runID}@example.invalid`;
  const t060AdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const t060ViewerEmail = `tenant-viewer+065-${runID}@example.invalid`;
  const t060ViewerPass = process.env.E2E_TENANT_VIEWER_PASS || "pw";

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
    await expect(tenantRow).toBeVisible({ timeout: 15_000 });
    const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
    expect(tenantID).not.toBe("");
    return tenantID;
  };

  const t060TenantID = await ensureTenant(t060Host, t060Name);

  await ensureIdentity(
    superadminContext,
    `${t060TenantID}:${t060AdminEmail}`,
    t060AdminEmail,
    t060AdminPass,
    { tenant_id: t060TenantID, role_slug: "tenant-admin" }
  );
  await ensureIdentity(
    superadminContext,
    `${t060TenantID}:${t060ViewerEmail}`,
    t060ViewerEmail,
    t060ViewerPass,
    { tenant_id: t060TenantID, role_slug: "tenant-viewer" }
  );
  await superadminContext.close();

  const newTenantContext = async (tenantHost) => {
    return browser.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
    });
  };

  const login = async (ctx, email, pass) => {
    const page = await ctx.newPage();
    await page.goto("/login");
    await expect(page.locator("h1")).toHaveText("Login");
    await page.locator('input[name="email"]').fill(email);
    await page.locator('input[name="password"]').fill(pass);
    await page.getByRole("button", { name: "Login" }).click();
    await expect(page).toHaveURL(/\/app\?as_of=\d{4}-\d{2}-\d{2}$/);
    return page;
  };

  const t060AdminContext = await newTenantContext(t060Host);
  const t060AdminPage = await login(t060AdminContext, t060AdminEmail, t060AdminPass);

  const ensurePerson = async (pernrInput, displayName) => {
    const resp = await t060AdminContext.request.get(`/person/api/persons:by-pernr?pernr=${encodeURIComponent(pernrInput)}`);
    if (resp.status() === 200) {
      return (await resp.json()).person_uuid;
    }
    if (resp.status() !== 404) {
      expect(resp.status(), `unexpected status: ${resp.status()} (${await resp.text()})`).toBe(404);
    }
    const notFound = await resp.json();
    expect(notFound.code).toBe("PERSON_NOT_FOUND");

    await t060AdminPage.goto(`/person/persons?as_of=${personAsOf}`);
    await expect(t060AdminPage.locator("h1")).toHaveText("Person");
    const form = t060AdminPage.locator(`form[action="/person/persons?as_of=${personAsOf}"]`).first();
    await form.locator('input[name="pernr"]').fill(pernrInput);
    await form.locator('input[name="display_name"]').fill(displayName);
    await form.locator('button[type="submit"]').click();
    await expect(t060AdminPage).toHaveURL(new RegExp(`/person/persons\\?as_of=${personAsOf}$`));

    const resp2 = await t060AdminContext.request.get(`/person/api/persons:by-pernr?pernr=${encodeURIComponent(pernrInput)}`);
    expect(resp2.status()).toBe(200);
    return (await resp2.json()).person_uuid;
  };

  const persons = [
    { key: "E01", pernr: "101" },
    { key: "E02", pernr: "102" },
    { key: "E03", pernr: "00000103" },
    { key: "E10", pernr: "110" }
  ];
  const personUUIDByKey = new Map();
  const displayNameByKey = new Map();
  for (const p of persons) {
    const displayName = `TP060-05 ${p.key} ${runID}`;
    displayNameByKey.set(p.key, displayName);
    const uuid = await ensurePerson(p.pernr, displayName);
    expect(uuid).not.toBe("");
    personUUIDByKey.set(p.key, uuid);
  }

  const e01UUID = personUUIDByKey.get("E01");
  const e02UUID = personUUIDByKey.get("E02");
  const e03UUID = personUUIDByKey.get("E03");
  const e10UUID = personUUIDByKey.get("E10");
  expect(e01UUID).toBeTruthy();
  expect(e02UUID).toBeTruthy();
  expect(e03UUID).toBeTruthy();
  expect(e10UUID).toBeTruthy();

  // 4C: TimeProfile save (UI).
  await t060AdminPage.goto(`/org/attendance-time-profile?as_of=${dHoliday}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / TimeProfile");
  const saveTPForm = t060AdminPage
    .locator('form[method="POST"]')
    .filter({ has: t060AdminPage.getByRole("button", { name: "Save" }) })
    .first();
  await saveTPForm.locator('input[name="effective_date"]').fill(dHoliday);
  await saveTPForm.locator('input[name="shift_start_local"]').fill("09:00");
  await saveTPForm.locator('input[name="shift_end_local"]').fill("18:00");
  await saveTPForm.getByRole("button", { name: "Save" }).click();
  await expect(t060AdminPage).toHaveURL(new RegExp(`/org/attendance-time-profile\\?as_of=${dHoliday}$`));

  // 4C: HolidayCalendar override (UI).
  await t060AdminPage.goto(`/org/attendance-holiday-calendar?as_of=${dHoliday}&month=${month}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / HolidayCalendar");
  const holidayRow = t060AdminPage.locator("tr", { hasText: dHoliday }).first();
  await holidayRow.locator('select[name="day_type"]').selectOption("LEGAL_HOLIDAY");
  await holidayRow.getByRole("button", { name: "Set" }).click();
  await expect(t060AdminPage).toHaveURL(new RegExp(`/org/attendance-holiday-calendar\\?as_of=${dHoliday}&month=${month}$`));
  await expect(holidayRow).toContainText("LEGAL_HOLIDAY");
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-holiday-${runID}.png`, fullPage: true });

  const gotoPunches = async (page, asOf, personUUID, date) => {
    await page.goto(
      `/org/attendance-punches?as_of=${asOf}&person_uuid=${encodeURIComponent(personUUID)}&from_date=${date}&to_date=${date}`
    );
    await expect(page.locator("h1")).toHaveText("Attendance / Punches");
  };

  const submitManual = async ({ page, asOf, personUUID, punchAt, punchType, note }) => {
    const manualForm = page
      .locator('form[method="POST"]')
      .filter({ has: page.getByRole("button", { name: "Submit" }) })
      .first();
    await manualForm.locator('select[name="person_uuid"]').selectOption(personUUID);
    await manualForm.locator('input[name="punch_at"]').fill(punchAt);
    await manualForm.locator('select[name="punch_type"]').selectOption(punchType);
    await manualForm.locator('input[name="note"]').fill(note);
    await manualForm.getByRole("button", { name: "Submit" }).click();
    await expect(page).toHaveURL(new RegExp(`/org/attendance-punches\\?as_of=${asOf}&person_uuid=${personUUID}&from_date=`));
  };

  // punches: E02 (Holiday) / E10 (RESTDAY) / E01 (workday) / E03 (missing OUT).
  await gotoPunches(t060AdminPage, dHoliday, e02UUID, dHoliday);
  await submitManual({
    page: t060AdminPage,
    asOf: dHoliday,
    personUUID: e02UUID,
    punchAt: `${dHoliday}T08:00`,
    punchType: "IN",
    note: `E02 holiday in ${runID}`
  });
  await submitManual({
    page: t060AdminPage,
    asOf: dHoliday,
    personUUID: e02UUID,
    punchAt: `${dHoliday}T20:00`,
    punchType: "OUT",
    note: `E02 holiday out ${runID}`
  });

  await gotoPunches(t060AdminPage, dRestday, e10UUID, dRestday);
  await submitManual({
    page: t060AdminPage,
    asOf: dRestday,
    personUUID: e10UUID,
    punchAt: `${dRestday}T08:00`,
    punchType: "IN",
    note: `E10 restday in ${runID}`
  });
  await submitManual({
    page: t060AdminPage,
    asOf: dRestday,
    personUUID: e10UUID,
    punchAt: `${dRestday}T20:00`,
    punchType: "OUT",
    note: `E10 restday out ${runID}`
  });

  await gotoPunches(t060AdminPage, dWorkday, e01UUID, dWorkday);
  await submitManual({
    page: t060AdminPage,
    asOf: dWorkday,
    personUUID: e01UUID,
    punchAt: `${dWorkday}T09:00`,
    punchType: "IN",
    note: `E01 in ${runID}`
  });
  await submitManual({
    page: t060AdminPage,
    asOf: dWorkday,
    personUUID: e01UUID,
    punchAt: `${dWorkday}T18:00`,
    punchType: "OUT",
    note: `E01 out ${runID}`
  });

  await gotoPunches(t060AdminPage, dWorkday, e03UUID, dWorkday);
  await submitManual({
    page: t060AdminPage,
    asOf: dWorkday,
    personUUID: e03UUID,
    punchAt: `${dWorkday}T09:00`,
    punchType: "IN",
    note: `E03 in only ${runID}`
  });

  // Daily results (workday): E01 PRESENT; E03 EXCEPTION + MISSING_OUT.
  await t060AdminPage.goto(`/org/attendance-daily-results?as_of=${dWorkday}&work_date=${dWorkday}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Daily Results");
  const e01Row = t060AdminPage.locator("tr", { hasText: displayNameByKey.get("E01") }).first();
  await expect(e01Row).toContainText("PRESENT");
  const e03Row = t060AdminPage.locator("tr", { hasText: displayNameByKey.get("E03") }).first();
  await expect(e03Row).toContainText("EXCEPTION");
  await expect(e03Row).toContainText("MISSING_OUT");
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-daily-workday-${runID}.png`, fullPage: true });

  // Daily results (holiday): E02 LEGAL_HOLIDAY and OT300 > 0.
  await t060AdminPage.goto(`/org/attendance-daily-results?as_of=${dHoliday}&work_date=${dHoliday}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Daily Results");
  const e02Row = t060AdminPage.locator("tr", { hasText: displayNameByKey.get("E02") }).first();
  await expect(e02Row).toContainText("LEGAL_HOLIDAY");
  const e02OT300 = Number.parseInt((await e02Row.locator("td").nth(10).innerText()).trim(), 10);
  expect(e02OT300).toBeGreaterThan(0);

  // Daily results (restday): E10 RESTDAY and OT200 > 0.
  await t060AdminPage.goto(`/org/attendance-daily-results?as_of=${dRestday}&work_date=${dRestday}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Daily Results");
  const e10Row = t060AdminPage.locator("tr", { hasText: displayNameByKey.get("E10") }).first();
  await expect(e10Row).toContainText("RESTDAY");
  const e10OT200 = Number.parseInt((await e10Row.locator("td").nth(9).innerText()).trim(), 10);
  expect(e10OT200).toBeGreaterThan(0);
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-daily-restday-${runID}.png`, fullPage: true });

  // 4E: void -> EXCEPTION + MISSING_OUT, with audit VOIDED marker.
  await t060AdminPage.goto(`/org/attendance-daily-results/${encodeURIComponent(e01UUID)}/${dWorkday}?as_of=${dWorkday}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Daily Results / Detail");
  await expect(t060AdminPage.getByRole("link", { name: "Go to punches" })).toBeVisible();

  const voidSelect = t060AdminPage.locator('select[name="target_punch_event_id"]').first();
  const outOption = voidSelect.locator("option", { hasText: "18:00 OUT" }).first();
  const outEventID = (await outOption.getAttribute("value")) || "";
  expect(outEventID).not.toBe("");
  await voidSelect.selectOption(outEventID);
  await t060AdminPage.locator('form[method="POST"] input[name="reason"]').first().fill(`tp060-05 void ${runID}`);
  await t060AdminPage.getByRole("button", { name: "Void" }).click();
  await expect(t060AdminPage).toHaveURL(new RegExp(`/org/attendance-daily-results/${e01UUID}/${dWorkday}\\?as_of=${dWorkday}$`));
  await expect(t060AdminPage.locator("h2", { hasText: "Summary" })).toBeVisible();
  await expect(t060AdminPage.locator("li", { hasText: "Status:" }).locator("code")).toHaveText("EXCEPTION");
  await expect(t060AdminPage.locator("li", { hasText: "Flags:" }).locator("code")).toContainText("MISSING_OUT");
  await expect(t060AdminPage.locator("table", { hasText: "VOIDED" })).toBeVisible();
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-void-${runID}.png`, fullPage: true });

  // List sync: E01 becomes EXCEPTION.
  await t060AdminPage.goto(`/org/attendance-daily-results?as_of=${dWorkday}&work_date=${dWorkday}`);
  const e01RowAfterVoid = t060AdminPage.locator("tr", { hasText: displayNameByKey.get("E01") }).first();
  await expect(e01RowAfterVoid).toContainText("EXCEPTION");
  await expect(e01RowAfterVoid).toContainText("MISSING_OUT");

  // Replace correction: add a new OUT, then status returns to PRESENT (flags no longer contain MISSING_OUT).
  await gotoPunches(t060AdminPage, dWorkday, e01UUID, dWorkday);
  await submitManual({
    page: t060AdminPage,
    asOf: dWorkday,
    personUUID: e01UUID,
    punchAt: `${dWorkday}T18:01`,
    punchType: "OUT",
    note: `E01 corrected out ${runID}`
  });

  await t060AdminPage.goto(`/org/attendance-daily-results/${encodeURIComponent(e01UUID)}/${dWorkday}?as_of=${dWorkday}`);
  await expect(t060AdminPage.locator("li", { hasText: "Status:" }).locator("code")).toHaveText("PRESENT");
  await expect(t060AdminPage.locator("li", { hasText: "Flags:" }).locator("code")).not.toContainText("MISSING_OUT");
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-corrected-${runID}.png`, fullPage: true });

  // 4D: time bank (E10): OT200 > 0 and comp earned > 0; trace contains 2026-01-03.
  await t060AdminPage.goto(`/org/attendance-time-bank?as_of=${dRestday}&month=${month}&person_uuid=${encodeURIComponent(e10UUID)}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Time Bank");
  const ot200Text = (await t060AdminPage.locator("li", { hasText: "OT Minutes 200:" }).locator("code").innerText()).trim();
  const compEarnedText = (
    await t060AdminPage.locator("li", { hasText: "Comp Earned Minutes:" }).locator("code").innerText()
  ).trim();
  expect(Number.parseInt(ot200Text, 10)).toBeGreaterThan(0);
  expect(Number.parseInt(compEarnedText, 10)).toBeGreaterThan(0);
  await expect(t060AdminPage.locator("table", { hasText: dRestday })).toBeVisible();
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-time-bank-e10-${runID}.png`, fullPage: true });

  // 4D: time bank (E02): OT300 > 0; trace contains 2026-01-01.
  await t060AdminPage.goto(`/org/attendance-time-bank?as_of=${dHoliday}&month=${month}&person_uuid=${encodeURIComponent(e02UUID)}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Time Bank");
  const ot300Text = (await t060AdminPage.locator("li", { hasText: "OT Minutes 300:" }).locator("code").innerText()).trim();
  expect(Number.parseInt(ot300Text, 10)).toBeGreaterThan(0);
  await expect(t060AdminPage.locator("table", { hasText: dHoliday })).toBeVisible();
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-05-time-bank-e02-${runID}.png`, fullPage: true });

  // Authz: viewer must be rejected for POST/admin actions (time profile save).
  const t060ViewerContext = await newTenantContext(t060Host);
  await login(t060ViewerContext, t060ViewerEmail, t060ViewerPass);
  const respViewerPost = await t060ViewerContext.request.post(`/org/attendance-time-profile?as_of=${dHoliday}`, {
    form: {
      op: "save",
      effective_date: dHoliday,
      shift_start_local: "09:00",
      shift_end_local: "18:00"
    }
  });
  expect(respViewerPost.status()).toBe(403);
  await t060ViewerContext.close();

  // Fail-closed: unknown host must not resolve tenant; read must be denied.
  const adminStorage = await t060AdminContext.storageState();
  const noTenantContext = await browser.newContext({
    baseURL,
    storageState: adminStorage,
    extraHTTPHeaders: { "X-Forwarded-Host": `no-tenant-${runID}.localhost` }
  });
  const respNoTenantGet = await noTenantContext.request.get(`/org/attendance-daily-results?as_of=${dWorkday}&work_date=${dWorkday}`);
  expect(respNoTenantGet.status()).toBeGreaterThanOrEqual(400);
  await noTenantContext.close();

  // Parameter validation: invalid work_date must show a stable error message.
  await t060AdminPage.goto(`/org/attendance-daily-results?as_of=${dWorkday}&work_date=BAD`);
  await expect(t060AdminPage.locator('p[style="color:#b00"]')).toContainText("work_date 无效");

  await t060AdminContext.close();
});
