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

test("tp070b: dict release ui flow and release authz", async ({ browser }) => {
  test.setTimeout(240_000);

  const runID = `${Date.now()}`;
  const asOf = "2026-01-07";
  const tenantHost = `t-070b1-${runID}.localhost`;

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+070b1-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const tenantAdminEmail = `tenant-admin+070b1-${runID}@example.invalid`;
  const tenantViewerEmail = `tenant-viewer+070b1-${runID}@example.invalid`;
  const tenantPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

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
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);

  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(`E2E Tenant 070B1 ${runID}`);
  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost);
  await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
  await expect(superadminPage.locator("tr", { hasText: tenantHost }).first()).toBeVisible({ timeout: 60_000 });

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost }).first();
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  await ensureKratosIdentity(superadminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenantID, email: tenantAdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenantID}:${tenantAdminEmail}`,
    password: tenantPass
  });
  await ensureKratosIdentity(superadminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenantID, email: tenantViewerEmail, role_slug: "tenant-viewer" },
    identifier: `${tenantID}:${tenantViewerEmail}`,
    password: tenantPass
  });

  await superadminContext.close();

  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const adminContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const adminLoginResp = await adminContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantPass }
  });
  expect(adminLoginResp.status(), await adminLoginResp.text()).toBe(204);

  const page = await adminContext.newPage();
  await page.goto(`/app/dicts?as_of=${asOf}`);
  await expect(page.getByRole("heading", { level: 2, name: "字典配置" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "字典基线发布" })).toBeVisible();

  await page.route("**/iam/api/dicts:release:preview", async (route) => {
    const payload = route.request().postDataJSON();
    expect(payload.release_id).toContain("rel-070b1");
    expect(payload.as_of).toBe(asOf);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        release_id: payload.release_id,
        source_tenant_id: payload.source_tenant_id,
        target_tenant_id: tenantID,
        as_of: payload.as_of,
        source_dict_count: 1,
        source_value_count: 1,
        target_dict_count: 1,
        target_value_count: 1,
        missing_dict_count: 0,
        dict_name_mismatch_count: 0,
        missing_value_count: 0,
        value_label_mismatch_count: 0,
        conflicts: []
      })
    });
  });
  await page.route("**/iam/api/dicts:release", async (route) => {
    const payload = route.request().postDataJSON();
    expect(payload.request_id).toContain("req-070b1");
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        task_id: `dict-release:${payload.release_id}:${tenantID.replaceAll("-", "")}:${payload.as_of}`,
        release_id: payload.release_id,
        request_id: payload.request_id,
        source_tenant_id: payload.source_tenant_id,
        target_tenant_id: tenantID,
        as_of: payload.as_of,
        status: "succeeded",
        dict_events_total: 1,
        dict_events_applied: 1,
        dict_events_retried: 0,
        value_events_total: 1,
        value_events_applied: 1,
        value_events_retried: 0,
        started_at: "2026-01-07T10:00:00Z",
        finished_at: "2026-01-07T10:00:01Z"
      })
    });
  });

  await page.getByLabel("as_of").nth(1).fill(asOf);
  await page.getByLabel("release_id").fill(`rel-070b1-${runID}`);
  await page.getByLabel("request_id").fill(`req-070b1-${runID}`);
  await page.getByRole("button", { name: "预检发布" }).click();
  await expect(page.getByText("预检通过，可执行发布。")).toBeVisible();
  await page.getByRole("button", { name: "执行发布" }).click();
  await expect(page.getByText("发布结果")).toBeVisible();
  await expect(page.getByText(`rel-070b1-${runID}`)).toBeVisible();
  await expect(page.getByText(`req-070b1-${runID}`)).toBeVisible();

  const viewerContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const viewerLoginResp = await viewerContext.request.post("/iam/api/sessions", {
    data: { email: tenantViewerEmail, password: tenantPass }
  });
  expect(viewerLoginResp.status(), await viewerLoginResp.text()).toBe(204);

  const viewerPreviewResp = await viewerContext.request.post("/iam/api/dicts:release:preview", {
    data: {
      source_tenant_id: "00000000-0000-0000-0000-000000000000",
      release_id: `rel-070b1-authz-${runID}`,
      as_of: asOf,
      max_conflicts: 10
    }
  });
  expect(viewerPreviewResp.status(), await viewerPreviewResp.text()).toBe(403);

  const viewerReleaseResp = await viewerContext.request.post("/iam/api/dicts:release", {
    data: {
      source_tenant_id: "00000000-0000-0000-0000-000000000000",
      release_id: `rel-070b1-authz-${runID}`,
      request_id: `req-070b1-authz-${runID}`,
      as_of: asOf,
      max_conflicts: 10
    }
  });
  expect(viewerReleaseResp.status(), await viewerReleaseResp.text()).toBe(403);

  await viewerContext.close();
  await adminContext.close();
});
