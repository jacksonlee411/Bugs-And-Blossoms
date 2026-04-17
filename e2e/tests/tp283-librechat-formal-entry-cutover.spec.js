import { expect, test } from "@playwright/test";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const formalEntryPath = "/app/cubebox";
const retiredEntryPath = "/app/assistant/librechat";
const retiredAssistantFormalAPIPaths = ["/internal/assistant/ui-bootstrap", "/internal/assistant/session"];
const removedBootstrapCompatPaths = [
  "/assets/librechat-web/api/config",
  "/assets/librechat-web/api/endpoints",
  "/assets/librechat-web/api/models",
  "/app/assistant/librechat/api/config",
  "/app/assistant/librechat/api/endpoints",
  "/app/assistant/librechat/api/models",
];

async function expectRetiredJSONCode(response, code) {
  expect(response.status()).toBe(410);
  const payload = await response.json();
  expect(payload.code).toBe(code);
}

async function getRetiredJSON(appContext, path, code) {
  const response = await appContext.request.get(path, {
    maxRedirects: 0,
    headers: {
      Accept: "application/json"
    }
  });
  await expectRetiredJSONCode(response, code);
}

async function postRetiredJSON(appContext, path, data, code) {
  const response = await appContext.request.post(path, {
    data,
    headers: {
      Accept: "application/json"
    }
  });
  await expectRetiredJSONCode(response, code);
}

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
    retiredAssistantFormalAPIPaths.map((path) => appContext.request.get(path, { maxRedirects: 0 })),
  );
  expect(bootstrapResp.status()).toBe(410);
  expect(sessionResp.status()).toBe(410);

  const retiredEntryResp = await appContext.request.get(retiredEntryPath, { maxRedirects: 0 });
  expect(retiredEntryResp.status()).toBe(410);
  expect(retiredEntryResp.headers()["content-type"] || "").toContain("text/html");
  expect(await retiredEntryResp.text()).toMatch(/LibreChat 入口已退役|CubeBox 正式入口/);
  await getRetiredJSON(appContext, retiredEntryPath, "librechat_retired");

  await getRetiredJSON(appContext, "/assistant-ui", "assistant_ui_retired");

  await getRetiredJSON(appContext, "/assistant-ui/some/path", "assistant_ui_retired");

  await postRetiredJSON(appContext, "/assistant-ui", {}, "assistant_ui_retired");

  for (const compatPath of removedBootstrapCompatPaths) {
    await getRetiredJSON(appContext, compatPath, "librechat_retired");
  }

  await appContext.close();
});

test("tp283-e2e-002: retired static prefix stays gone and still respects session boundary", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appBaseURL, tenantHost, appContext } = await createTP283Session(browser, "002");

  await getRetiredJSON(appContext, "/assets/librechat-web/registerSW.js", "librechat_retired");

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
