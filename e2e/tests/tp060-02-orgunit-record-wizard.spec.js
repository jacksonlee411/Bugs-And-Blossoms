import { expect, test } from "@playwright/test";

const addDays = (dateValue, days) => {
  const [year, month, day] = String(dateValue).split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  date.setUTCDate(date.getUTCDate() + days);
  const y = String(date.getUTCFullYear());
  const m = String(date.getUTCMonth() + 1).padStart(2, "0");
  const d = String(date.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
};

test("tp060-02: orgunit record wizard (4-step add/delete flow)", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-01";
  const runID = `${Date.now()}`;
  const tenantHost = `t-tp060-02-wizard-${runID}.localhost`;
  const tenantName = `TP060-02 Wizard Tenant ${runID}`;
  const orgName = `Wizard Root ${runID}`;
  const orgCode = `WZ${runID.slice(-6)}`;

  const tenantAdminEmail = "tenant-admin@example.invalid";
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp060-02-wizard-${runID}@example.invalid`;
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
  await page.locator('input[name="email"]').fill(tenantAdminEmail);
  await page.locator('input[name="password"]').fill(tenantAdminPass);
  await page.getByRole("button", { name: "Login" }).click();
  await expect(page).toHaveURL(/\/app\?as_of=\d{4}-\d{2}-\d{2}$/);

  await page.goto(`/org/nodes?tree_as_of=${asOf}`);
  await expect(page.locator("h1")).toHaveText("OrgUnit Details");

  await page.locator(".org-node-create-btn").click();
  const orgCreateForm = page
    .locator(`#org-node-details form[method="POST"][action="/org/nodes?tree_as_of=${asOf}"]`)
    .filter({ has: page.locator('input[name="name"]') })
    .first();
  await expect(orgCreateForm).toBeVisible();

  await orgCreateForm.locator('input[name="effective_date"]').fill(asOf);
  await orgCreateForm.locator('input[name="org_code"]').fill(orgCode);
  await orgCreateForm.locator('input[name="name"]').fill(orgName);
  await orgCreateForm.locator('input[name="parent_code"]').fill("");
  const createBUInput = orgCreateForm.locator('input[name="is_business_unit"]');
  if ((await createBUInput.count()) > 0) {
    const type = (await createBUInput.first().getAttribute("type")) || "";
    if (type === "checkbox") {
      await createBUInput.first().check();
    } else {
      await createBUInput.first().fill("true");
    }
  }

  await orgCreateForm.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(new RegExp(`/org/nodes\\?tree_as_of=${asOf}$`));

  const searchResp = await appContext.request.get(
    `/org/nodes/search?query=${encodeURIComponent(orgCode)}&tree_as_of=${encodeURIComponent(asOf)}`
  );
  expect(searchResp.ok(), await searchResp.text()).toBeTruthy();
  const searchData = await searchResp.json();
  const orgID = Number(searchData.target_org_id || 0);
  expect(orgID).toBeGreaterThan(0);

  const selectNode = async () => {
    const searchForm = page.locator(".org-node-search-form");
    await searchForm.locator('input[name="query"]').fill(orgCode);
    await searchForm.getByRole("button", { name: "查找" }).click();
    const panel = page.locator(`#org-node-details .org-node-details-panel[data-org-id="${orgID}"]`).first();
    await expect(panel).toBeVisible({ timeout: 20_000 });
    return panel;
  };

  const panel = await selectNode();
  const maxEffectiveDate = await panel.getAttribute("data-max-effective-date");
  expect(maxEffectiveDate).toBeTruthy();

  const addRecordDate = addDays(maxEffectiveDate, 1);
  const addRecordName = `${orgName} V2`;

  await panel.locator('.org-node-record-btn[data-action="add_record"]').click();
  const formWrap = panel.locator('.org-node-record-form[data-open="true"]');
  await expect(formWrap).toBeVisible();

  const recordForm = formWrap.locator("form.org-node-record-action-form");
  await expect(recordForm.locator('.org-node-record-step[data-step="1"][data-active="true"]')).toBeVisible();
  await expect(recordForm.locator(".org-node-record-submit")).toBeHidden();

  await recordForm.locator(".org-node-record-next").click();
  await expect(recordForm.locator('.org-node-record-step[data-step="2"][data-active="true"]')).toBeVisible();

  await recordForm.locator('input[name="effective_date"]').fill("");
  await recordForm.locator(".org-node-record-next").click();
  await expect(recordForm.locator(".org-node-record-form-hint")).toContainText("effective_date is required");

  await recordForm.locator('input[name="effective_date"]').fill(addRecordDate);
  await recordForm.locator(".org-node-record-next").click();
  await expect(recordForm.locator('.org-node-record-step[data-step="3"][data-active="true"]')).toBeVisible();

  await recordForm.locator('select[name="record_change_type"]').selectOption("rename");
  await recordForm.locator('input[name="name"]').fill(addRecordName);
  await recordForm.locator(".org-node-record-next").click();

  await expect(recordForm.locator('.org-node-record-step[data-step="4"][data-active="true"]')).toBeVisible();
  await expect(recordForm.locator(".org-node-record-summary")).toContainText("将执行：新增记录");
  await expect(recordForm.locator(".org-node-record-summary")).toContainText(addRecordName);
  await expect(recordForm.locator(".org-node-record-submit")).toBeVisible();

  await Promise.all([
    page.waitForURL(new RegExp(`/org/nodes\\?tree_as_of=${asOf}$`)),
    recordForm.locator(".org-node-record-submit").click()
  ]);

  const detailsResp = await appContext.request.get(
    `/org/nodes/details?org_id=${encodeURIComponent(String(orgID))}&effective_date=${encodeURIComponent(addRecordDate)}`
  );
  expect(detailsResp.ok(), await detailsResp.text()).toBeTruthy();
  expect(await detailsResp.text()).toContain(addRecordName);

  const panelAfterAdd = await selectNode();
  await panelAfterAdd.locator('.org-node-record-more-toggle').click();
  const deleteBtn = panelAfterAdd.locator('.org-node-record-actions-more .org-node-record-btn[data-action="delete_record"]');
  await expect(deleteBtn).toBeVisible();
  await expect(deleteBtn).toHaveClass(/is-danger/);

  await appContext.close();
});
