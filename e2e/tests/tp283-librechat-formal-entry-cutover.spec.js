import { expect, test } from "@playwright/test";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const formalEntryPath = "/app/cubebox";
const retiredEntryPath = "/app/assistant/librechat";
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

test("tp283-e2e-001: CubeBox is the only accepted chat entry", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext } = await createTP283Session(browser, "001");

  const page = await appContext.newPage();
  await page.goto(formalEntryPath);
  await expect(page).toHaveURL(/\/app\/cubebox$/);
  await expect(page.getByRole("heading", { name: "CubeBox" })).toBeVisible();
  await expect(page.getByTestId("cubebox-input")).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId("cubebox-send")).toBeVisible();
  await expect(page.getByRole("link", { name: "文件" })).toHaveAttribute("href", "/app/cubebox/files");
  await expect(page.getByRole("link", { name: "模型" })).toHaveAttribute("href", "/app/cubebox/models");

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

  const retiredEntryResp = await appContext.request.get(retiredEntryPath, { maxRedirects: 0 });
  expect(retiredEntryResp.status()).toBe(410);

  const aliasResp = await appContext.request.get("/assistant-ui", { maxRedirects: 0 });
  expect(aliasResp.status()).toBe(410);

  const aliasPathResp = await appContext.request.get("/assistant-ui/some/path", { maxRedirects: 0 });
  expect(aliasPathResp.status()).toBe(410);

  const aliasWriteResp = await appContext.request.post("/assistant-ui", { data: {} });
  expect(aliasWriteResp.status()).toBe(410);

  for (const compatPath of removedBootstrapCompatPaths) {
    const resp = await appContext.request.get(compatPath, { maxRedirects: 0 });
    expect(resp.status(), `${compatPath} should be retired during 380B`).toBe(410);
  }

  await appContext.close();
});

test("tp283-e2e-002: retired static prefix stays gone and still respects session boundary", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appBaseURL, tenantHost, appContext } = await createTP283Session(browser, "002");

  const authedStaticResp = await appContext.request.get("/assets/librechat-web/registerSW.js");
  expect(authedStaticResp.status()).toBe(410);

  const anonContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  const anonStaticResp = await anonContext.request.get("/assets/librechat-web/registerSW.js", { maxRedirects: 0 });
  expect([302, 410]).toContain(anonStaticResp.status());
  if (anonStaticResp.status() === 302) {
    expect(anonStaticResp.headers()["location"]).toBe("/app/login");
  }

  await anonContext.close();
  await appContext.close();
});
