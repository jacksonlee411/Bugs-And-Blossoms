import { expect, test } from "@playwright/test";

test("tp060-04: attendance 4A punches (manual + import + authz + fail-closed + cross-tenant)", async ({ browser }) => {
  test.setTimeout(240_000);

  const personAsOf = "2026-01-01";
  const asOf = "2026-01-02";
  const runID = `${Date.now()}`;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp060-04-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;

  const baseURL = process.env.E2E_BASE_URL || "http://localhost:8080";

  const t060Host = `t-tp060-04-a-${runID}.localhost`;
  const t060Name = `TP060-04 A ${runID}`;
  const t060AdminEmail = `tenant-admin+064-${runID}@example.invalid`;
  const t060AdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const t060ViewerEmail = `tenant-viewer+064-${runID}@example.invalid`;
  const t060ViewerPass = process.env.E2E_TENANT_VIEWER_PASS || "pw";

  const t060bHost = `t-tp060-04-b-${runID}.localhost`;
  const t060bName = `TP060-04 B ${runID}`;
  const t060bAdminEmail = `tenant-admin-b+064-${runID}@example.invalid`;
  const t060bAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

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

  const t060TenantID = await ensureTenant(t060Host, t060Name);
  const t060bTenantID = await ensureTenant(t060bHost, t060bName);

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
  await ensureIdentity(
    superadminContext,
    `${t060bTenantID}:${t060bAdminEmail}`,
    t060bAdminEmail,
    t060bAdminPass,
    { tenant_id: t060bTenantID, role_slug: "tenant-admin" }
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
    { key: "E03", pernr: "00000103" },
    { key: "E10", pernr: "110" }
  ];
  const personUUIDByKey = new Map();
  for (const p of persons) {
    const uuid = await ensurePerson(p.pernr, `TP060-04 ${p.key} ${runID}`);
    expect(uuid).not.toBe("");
    personUUIDByKey.set(p.key, uuid);
  }

  const e01UUID = personUUIDByKey.get("E01");
  const e03UUID = personUUIDByKey.get("E03");
  const e10UUID = personUUIDByKey.get("E10");
  expect(e01UUID).toBeTruthy();
  expect(e03UUID).toBeTruthy();
  expect(e10UUID).toBeTruthy();

  const gotoPunches = async (page, personUUID) => {
    await page.goto(
      `/org/attendance-punches?as_of=${asOf}&person_uuid=${encodeURIComponent(personUUID)}&from_date=${asOf}&to_date=${asOf}`
    );
    await expect(page.locator("h1")).toHaveText("Attendance / Punches");
  };

  const submitManual = async ({ page, personUUID, punchAt, punchType, note }) => {
    const manualForm = page
      .locator('form[method="POST"]')
      .filter({ has: page.getByRole("button", { name: "Submit" }) })
      .first();
    await manualForm.locator('select[name="person_uuid"]').selectOption(personUUID);
    await manualForm.locator('input[name="punch_at"]').fill(punchAt);
    await manualForm.locator('select[name="punch_type"]').selectOption(punchType);
    await manualForm.locator('input[name="note"]').fill(note);
    await manualForm.getByRole("button", { name: "Submit" }).click();
    await expect(page).toHaveURL(
      new RegExp(`/org/attendance-punches\\?as_of=${asOf}&person_uuid=${personUUID}&from_date=${asOf}&to_date=${asOf}$`)
    );
  };

  await gotoPunches(t060AdminPage, e01UUID);
  await submitManual({
    page: t060AdminPage,
    personUUID: e01UUID,
    punchAt: `${asOf}T09:00`,
    punchType: "IN",
    note: `E01 in ${runID}`
  });
  await submitManual({
    page: t060AdminPage,
    personUUID: e01UUID,
    punchAt: `${asOf}T18:00`,
    punchType: "OUT",
    note: `E01 out ${runID}`
  });
  await expect(t060AdminPage.locator("table")).toContainText(`${asOf} 09:00`);
  await expect(t060AdminPage.locator("table")).toContainText("IN");
  await expect(t060AdminPage.locator("table")).toContainText(`${asOf} 18:00`);
  await expect(t060AdminPage.locator("table")).toContainText("OUT");
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-04-e01-${runID}.png`, fullPage: true });

  await gotoPunches(t060AdminPage, e03UUID);
  await submitManual({
    page: t060AdminPage,
    personUUID: e03UUID,
    punchAt: `${asOf}T09:00`,
    punchType: "IN",
    note: `E03 in only ${runID}`
  });
  await expect(t060AdminPage.locator("table")).toContainText(`${asOf} 09:00`);
  await expect(t060AdminPage.locator("table")).toContainText("IN");
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-04-e03-${runID}.png`, fullPage: true });

  await gotoPunches(t060AdminPage, e10UUID);
  const importForm = t060AdminPage
    .locator('form[method="POST"]')
    .filter({ has: t060AdminPage.getByRole("button", { name: "Import" }) })
    .first();
  await importForm
    .locator('textarea[name="csv"]')
    .fill(`${e10UUID},${asOf}T09:00,IN\n${e10UUID},${asOf}T18:00,OUT\n`);
  await importForm.getByRole("button", { name: "Import" }).click();
  await expect(t060AdminPage).toHaveURL(new RegExp(`/org/attendance-punches\\?as_of=${asOf}&from_date=${asOf}&to_date=${asOf}$`));

  await gotoPunches(t060AdminPage, e10UUID);
  await expect(t060AdminPage.locator("table")).toContainText(`${asOf} 09:00`);
  await expect(t060AdminPage.locator("table")).toContainText(`${asOf} 18:00`);
  await expect(t060AdminPage.locator("table")).toContainText("IMPORT");
  await t060AdminPage.screenshot({ path: `_artifacts/tp060-04-e10-import-${runID}.png`, fullPage: true });

  // Import atomicity negative: invalid line must fail and nothing should be inserted.
  await t060AdminPage.goto(`/org/attendance-punches?as_of=${asOf}&person_uuid=${encodeURIComponent(e10UUID)}&from_date=${asOf}&to_date=${asOf}`);
  await expect(t060AdminPage.locator("h1")).toHaveText("Attendance / Punches");
  await importForm
    .locator('textarea[name="csv"]')
    .fill(`${e10UUID},${asOf}T12:01,IN\n${e10UUID},${asOf}T12:02,OUT\n${e10UUID},${asOf}T12:03,BAD\n`);
  await importForm.getByRole("button", { name: "Import" }).click();
  await expect(t060AdminPage.locator('p[style="color:#b00"]')).toContainText("line 3: punch_type must be IN|OUT");

  await gotoPunches(t060AdminPage, e10UUID);
  await expect(t060AdminPage.locator("table")).not.toContainText(`${asOf} 12:01`);
  await expect(t060AdminPage.locator("table")).not.toContainText(`${asOf} 12:02`);

  // Authz: viewer must be rejected for POST.
  const t060ViewerContext = await newTenantContext(t060Host);
  await login(t060ViewerContext, t060ViewerEmail, t060ViewerPass);
  const respViewerPost = await t060ViewerContext.request.post(`/org/attendance-punches?as_of=${asOf}`, {
    form: { op: "manual", person_uuid: e01UUID, punch_at: `${asOf}T10:00`, punch_type: "IN" }
  });
  expect(respViewerPost.status()).toBe(403);
  await t060ViewerContext.close();

  // Cross-tenant: ensure T060B has a punch, then assert it is not visible under T060 by person_uuid.
  const t060bAdminContext = await newTenantContext(t060bHost);
  const t060bAdminPage = await login(t060bAdminContext, t060bAdminEmail, t060bAdminPass);
  const ensurePerson060B = async () => {
    const pernr = "201";
    const resp = await t060bAdminContext.request.get(`/person/api/persons:by-pernr?pernr=${encodeURIComponent(pernr)}`);
    if (resp.status() === 200) {
      return (await resp.json()).person_uuid;
    }
    await t060bAdminPage.goto(`/person/persons?as_of=${personAsOf}`);
    await expect(t060bAdminPage.locator("h1")).toHaveText("Person");
    const form = t060bAdminPage.locator(`form[action="/person/persons?as_of=${personAsOf}"]`).first();
    await form.locator('input[name="pernr"]').fill(pernr);
    await form.locator('input[name="display_name"]').fill(`Tenant060B Person 201 ${runID}`);
    await form.locator('button[type="submit"]').click();
    await expect(t060bAdminPage).toHaveURL(new RegExp(`/person/persons\\?as_of=${personAsOf}$`));
    const resp2 = await t060bAdminContext.request.get(`/person/api/persons:by-pernr?pernr=${encodeURIComponent(pernr)}`);
    expect(resp2.status()).toBe(200);
    return (await resp2.json()).person_uuid;
  };
  const t060bPersonUUID = await ensurePerson060B();

  const punch060b = await t060bAdminContext.request.post("/org/api/attendance-punches", {
    data: {
      person_uuid: t060bPersonUUID,
      punch_time: `${asOf}T09:00:00+08:00`,
      punch_type: "IN",
      source_provider: "MANUAL",
      payload: { note: `tp060-04 cross-tenant ${runID}` },
      source_raw_payload: {},
      device_info: {}
    }
  });
  expect(punch060b.status(), await punch060b.text()).toBe(201);
  await t060bAdminContext.close();

  const respCrossTenant = await t060AdminContext.request.get(
    `/org/api/attendance-punches?person_uuid=${encodeURIComponent(t060bPersonUUID)}&from=${asOf}T00:00:00Z&to=2026-01-03T00:00:00Z`
  );
  expect(respCrossTenant.status(), await respCrossTenant.text()).toBe(200);
  const crossTenantJSON = await respCrossTenant.json();
  expect(Array.isArray(crossTenantJSON.punches)).toBe(true);
  expect(crossTenantJSON.punches.length).toBe(0);

  // Fail-closed: unknown host must not resolve tenant; any read/write must be denied.
  const adminStorage = await t060AdminContext.storageState();
  const noTenantContext = await browser.newContext({
    baseURL,
    storageState: adminStorage,
    extraHTTPHeaders: { "X-Forwarded-Host": `no-tenant-${runID}.localhost` }
  });
  const respNoTenantGet = await noTenantContext.request.get(`/org/attendance-punches?as_of=${asOf}`);
  expect(respNoTenantGet.status()).toBeGreaterThanOrEqual(400);

  const respNoTenantPost = await noTenantContext.request.post("/org/api/attendance-punches", {
    data: {
      person_uuid: e01UUID,
      punch_time: `${asOf}T12:34:00+08:00`,
      punch_type: "IN",
      source_provider: "MANUAL",
      payload: { note: `fail-closed probe ${runID}` },
      source_raw_payload: {},
      device_info: {}
    }
  });
  expect(respNoTenantPost.status()).toBeGreaterThanOrEqual(400);

  const respE01List = await t060AdminContext.request.get(
    `/org/api/attendance-punches?person_uuid=${encodeURIComponent(e01UUID)}&from=${asOf}T00:00:00Z&to=2026-01-03T00:00:00Z&limit=1000`
  );
  expect(respE01List.status(), await respE01List.text()).toBe(200);
  const e01JSON = await respE01List.json();
  const punches = Array.isArray(e01JSON.punches) ? e01JSON.punches : [];
  expect(punches.find((p) => p.punch_time === `${asOf}T04:34:00Z`)).toBeUndefined(); // 12:34+08
  expect(JSON.stringify(e01JSON)).not.toContain("fail-closed probe");

  await noTenantContext.close();
  await t060AdminContext.close();
});
