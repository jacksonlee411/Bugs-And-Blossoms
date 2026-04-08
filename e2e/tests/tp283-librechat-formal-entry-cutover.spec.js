import { expect, test } from "@playwright/test";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

async function createTP283Session(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  return setupTenantAdminSession(browser, {
    tenantName: `TP283 Tenant ${runID}`,
    tenantHost: `t-tp283-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp283-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp283-${runID}@example.invalid`
  });
}

test("tp283-e2e-001: formal entry is the only accepted chat entry", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext } = await createTP283Session(browser, "001");

  const page = await appContext.newPage();
  await page.goto("/app/assistant/librechat");
  await expect(page).toHaveTitle(/(LibreChat|Bugs \& Blossoms Assistant)/i);

  const aliasResp = await appContext.request.get("/assistant-ui", { maxRedirects: 0 });
  expect(aliasResp.status()).toBe(302);
  expect(aliasResp.headers()["location"]).toBe("/app/assistant/librechat");

  const aliasPathResp = await appContext.request.get("/assistant-ui/some/path", { maxRedirects: 0 });
  expect(aliasPathResp.status()).toBe(302);
  expect(aliasPathResp.headers()["location"]).toBe("/app/assistant/librechat");

  const aliasWriteResp = await appContext.request.post("/assistant-ui", { data: {} });
  expect(aliasWriteResp.status()).toBe(405);

  await appContext.close();
});

test("tp283-e2e-002: formal static prefix is protected by session boundary", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appBaseURL, tenantHost, appContext } = await createTP283Session(browser, "002");

  const authedStaticResp = await appContext.request.get("/assets/librechat-web/registerSW.js");
  expect(authedStaticResp.status()).toBe(200);
  expect(await authedStaticResp.text()).toContain("serviceWorker.register");

  const anonContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const anonStaticResp = await anonContext.request.get("/assets/librechat-web/registerSW.js", { maxRedirects: 0 });
  expect(anonStaticResp.status()).toBe(302);
  expect(anonStaticResp.headers()["location"]).toBe("/app/login");

  await anonContext.close();
  await appContext.close();
});
