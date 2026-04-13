import { expect, test } from "@playwright/test";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const successorBootstrapPaths = ["/internal/assistant/ui-bootstrap", "/internal/assistant/session"];
const removedBootstrapCompatPaths = [
  "/assets/librechat-web/api/config",
  "/assets/librechat-web/api/endpoints",
  "/assets/librechat-web/api/models",
  "/app/assistant/librechat/api/config",
  "/app/assistant/librechat/api/endpoints",
  "/app/assistant/librechat/api/models",
];

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
  await expect(page.getByRole("textbox").last()).toBeVisible({ timeout: 60_000 });
  await page.waitForTimeout(1500);

  await expect(page.locator('a[href*="/agents"], a[href*="/search"], a[href*="/mcp"]')).toHaveCount(0);
  await expect(page.getByText(/Agent Marketplace/i)).toHaveCount(0);
  await expect(page.getByText(/Web Search/i)).toHaveCount(0);
  await expect(page.getByText(/Code Interpreter/i)).toHaveCount(0);
  await expect(page.getByText(/^MCP$/i)).toHaveCount(0);

  const [bootstrapResp, sessionResp] = await Promise.all(
    successorBootstrapPaths.map((path) => appContext.request.get(path, { maxRedirects: 0 })),
  );
  expect(bootstrapResp.status()).toBe(200);
  expect(sessionResp.status()).toBe(200);

  const bootstrapPayload = await bootstrapResp.json();
  const sessionPayload = await sessionResp.json();
  expect(bootstrapPayload.contract_version).toBe("v1");
  expect(bootstrapPayload.runtime.runtime_cutover_mode).toBe("ui-shell-only");
  expect(bootstrapPayload.ui.agents_ui_enabled).toBe(false);
  expect(bootstrapPayload.ui.memory_enabled).toBe(false);
  expect(bootstrapPayload.ui.web_search_enabled).toBe(false);
  expect(bootstrapPayload.ui.file_search_enabled).toBe(false);
  expect(bootstrapPayload.ui.code_interpreter_enabled).toBe(false);
  expect(sessionPayload.contract_version).toBe("v1");
  expect(sessionPayload.authenticated).toBe(true);

  const aliasResp = await appContext.request.get("/assistant-ui", { maxRedirects: 0 });
  expect(aliasResp.status()).toBe(410);

  const aliasPathResp = await appContext.request.get("/assistant-ui/some/path", { maxRedirects: 0 });
  expect(aliasPathResp.status()).toBe(410);

  const aliasWriteResp = await appContext.request.post("/assistant-ui", { data: {} });
  expect(aliasWriteResp.status()).toBe(410);

  for (const compatPath of removedBootstrapCompatPaths) {
    const resp = await appContext.request.get(compatPath, { maxRedirects: 0 });
    expect(resp.status(), `${compatPath} should be removed during 375M1`).toBe(404);
  }

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
