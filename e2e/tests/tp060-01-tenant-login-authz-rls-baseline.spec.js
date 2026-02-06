import { expect, test } from "@playwright/test";

function headerValues(resp, headerName) {
  const wanted = headerName.toLowerCase();
  return resp
    .headersArray()
    .filter((h) => h.name.toLowerCase() === wanted)
    .map((h) => h.value);
}

async function createKratosIdentity(request, kratosAdminURL, { traits, identifier, password }) {
  const resp = await request.post(`${kratosAdminURL}/admin/identities`, {
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
  expect(resp.ok()).toBeTruthy();
}

test("tp060-01: tenant/login/authz/rls baseline", async ({ browser }) => {
  test.setTimeout(120_000);

  const asOf = "2026-01-01";
  const runID = `${Date.now()}`;

  const tenantAHost = `t-tp060-01-a-${runID}.localhost`;
  const tenantBHost = `t-tp060-01-b-${runID}.localhost`;

  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";
  const tenantViewerPass = process.env.E2E_TENANT_VIEWER_PASS || tenantAdminPass;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const defaultSuperadminEmail = `admin+061-${runID}@example.invalid`;
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || defaultSuperadminEmail;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;

  const t060AdminEmail = `tenant-admin+061-${runID}@example.invalid`;
  const t060ViewerEmail = `tenant-viewer+061-${runID}@example.invalid`;
  const t060bAdminEmail = `tenant-admin-b+061-${runID}@example.invalid`;
  const t060bViewerEmail = `tenant-viewer-b+061-${runID}@example.invalid`;

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass }
  });
  const superadminPage = await superadminContext.newPage();

  if (!process.env.E2E_SUPERADMIN_EMAIL) {
    const superadminIdentifier = `sa:${superadminEmail.toLowerCase()}`;
    await createKratosIdentity(superadminContext.request, kratosAdminURL, {
      traits: { email: superadminEmail },
      identifier: superadminIdentifier,
      password: superadminLoginPass
    });
  }

  await superadminPage.goto("/superadmin/login");
  await expect(superadminPage.locator("h1")).toHaveText("SuperAdmin Login");
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.locator("h1")).toHaveText("SuperAdmin / Tenants");

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

  const tenantAID = await ensureTenant(tenantAHost, `TP060-01 Tenant A ${runID}`);
  const tenantBID = await ensureTenant(tenantBHost, `TP060-01 Tenant B ${runID}`);

  await createKratosIdentity(superadminContext.request, kratosAdminURL, {
    traits: { tenant_uuid: tenantAID, email: t060AdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenantAID}:${t060AdminEmail}`,
    password: tenantAdminPass
  });
  await createKratosIdentity(superadminContext.request, kratosAdminURL, {
    traits: { tenant_uuid: tenantAID, email: t060ViewerEmail, role_slug: "tenant-viewer" },
    identifier: `${tenantAID}:${t060ViewerEmail}`,
    password: tenantViewerPass
  });
  await createKratosIdentity(superadminContext.request, kratosAdminURL, {
    traits: { tenant_uuid: tenantBID, email: t060bAdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenantBID}:${t060bAdminEmail}`,
    password: tenantAdminPass
  });
  await createKratosIdentity(superadminContext.request, kratosAdminURL, {
    traits: { tenant_uuid: tenantBID, email: t060bViewerEmail, role_slug: "tenant-viewer" },
    identifier: `${tenantBID}:${t060bViewerEmail}`,
    password: tenantViewerPass
  });

  await superadminContext.close();

  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";

  const badHost = `t-tp060-01-nope-${runID}.localhost`;
  const badHostContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": badHost }
  });
  const badHostResp = await badHostContext.request.get("/login", { headers: { Accept: "application/json" } });
  expect(badHostResp.status()).toBe(404);
  expect((await badHostResp.json()).code).toBe("tenant_not_found");
  await badHostContext.close();

  const tenantAContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantAHost }
  });

  const loginGetResp = await tenantAContext.request.get("/login");
  expect(loginGetResp.status()).toBe(200);

  const unauthAppResp = await tenantAContext.request.get(`/app?as_of=${asOf}`, { maxRedirects: 0 });
  expect(unauthAppResp.status()).toBe(302);
  expect(unauthAppResp.headers()["location"]).toBe("/login");

  const loginPostResp = await tenantAContext.request.post("/login", {
    form: { email: t060AdminEmail, password: tenantAdminPass },
    maxRedirects: 0
  });
  expect(loginPostResp.status()).toBe(302);
  expect(headerValues(loginPostResp, "set-cookie").join("\n")).toContain("sid=");

  const pageA = await tenantAContext.newPage();
  await pageA.goto(`/app?as_of=${asOf}`);
  await expect(pageA.getByRole("link", { name: "中文" })).toBeVisible();
  await expect(pageA.locator("#topbar").getByText("As-of", { exact: true })).toBeVisible();
  await expect(pageA.getByRole("link", { name: "OrgUnit", exact: true })).toBeVisible();

  await pageA.getByRole("link", { name: "中文" }).click();
  await expect(pageA.locator("#topbar").getByText("有效日期", { exact: true })).toBeVisible();
  await expect(pageA.getByRole("link", { name: "组织架构", exact: true })).toBeVisible();

  await pageA.goto(`/org/nodes?tree_as_of=${asOf}`);
  await expect(pageA.locator("#topbar").getByText("有效日期", { exact: true })).toBeVisible();

  await pageA.getByRole("link", { name: "EN" }).click();
  await expect(pageA.locator("#topbar").getByText("As-of", { exact: true })).toBeVisible();

  const sidCookie = (await tenantAContext.cookies()).find((c) => c.name === "sid");
  expect(sidCookie).toBeTruthy();

  const tenantBContextCross = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantBHost }
  });
  await tenantBContextCross.addCookies([sidCookie]);
  const crossTenantResp = await tenantBContextCross.request.get(`/app?as_of=${asOf}`, { maxRedirects: 0 });
  expect(crossTenantResp.status()).toBe(302);
  expect(crossTenantResp.headers()["location"]).toBe("/login");
  const crossTenantSetCookie = headerValues(crossTenantResp, "set-cookie").join("\n").toLowerCase();
  expect(crossTenantSetCookie).toContain("sid=");
  expect(crossTenantSetCookie).toContain("max-age=0");
  await tenantBContextCross.close();

  const tenantBAdminContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantBHost }
  });
  const tenantBLoginResp = await tenantBAdminContext.request.post("/login", {
    form: { email: t060bAdminEmail, password: tenantAdminPass },
    maxRedirects: 0
  });
  expect(tenantBLoginResp.status()).toBe(302);

  const personLookupB = await tenantBAdminContext.request.get("/person/api/persons:by-pernr?pernr=201");
  if (personLookupB.status() === 404) {
    await tenantBAdminContext.request.post(`/person/persons?as_of=${asOf}`, {
      form: { pernr: "201", display_name: `TP060-01 CrossTenant ${runID}` },
      maxRedirects: 0
    });
    const personLookupBAfter = await tenantBAdminContext.request.get("/person/api/persons:by-pernr?pernr=201");
    expect(personLookupBAfter.status()).toBe(200);
  } else {
    expect(personLookupB.status()).toBe(200);
  }
  await tenantBAdminContext.close();

  const crossTenantDataResp = await tenantAContext.request.get("/person/api/persons:by-pernr?pernr=201");
  expect(crossTenantDataResp.status()).toBe(404);
  expect((await crossTenantDataResp.json()).code).toBe("PERSON_NOT_FOUND");

  const tenantAViewerContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantAHost }
  });
  const viewerLoginResp = await tenantAViewerContext.request.post("/login", {
    form: { email: t060ViewerEmail, password: tenantViewerPass },
    maxRedirects: 0
  });
  expect(viewerLoginResp.status()).toBe(302);

  const viewerOrgGet = await tenantAViewerContext.request.get(`/org/nodes?tree_as_of=${asOf}`);
  expect(viewerOrgGet.status()).toBe(200);

  const viewerOrgPost = await tenantAViewerContext.request.post(`/org/nodes?tree_as_of=${asOf}`, {
    headers: { Accept: "application/json" },
    form: { name: `TP060-01 Forbidden ${runID}` },
    maxRedirects: 0
  });
  expect(viewerOrgPost.status()).toBe(403);
  expect((await viewerOrgPost.json()).code).toBe("forbidden");

  await tenantAViewerContext.close();
  await tenantAContext.close();
});
