import { expect, test } from "@playwright/test";

async function setupTenantAdminSession(browser) {
  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const appContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": "localhost" }
  });
  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: "admin@localhost", password: "admin123" }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);
  const page = await appContext.newPage();
  return { appContext, page };
}

async function typePromptInIframe(page, input) {
  const frame = page.frameLocator("[data-testid='librechat-standalone-frame']");
  const textarea = frame.locator("textarea").first();
  await expect(textarea).toBeVisible({ timeout: 120_000 });
  await textarea.fill(input);
  await frame.getByRole("button", { name: /发送|send/i }).click();
}

test("tp265-e2e-001: blocked reply still must go through gpt-5.2 without fallback", async ({ browser }) => {
  test.setTimeout(300_000);
  const { appContext, page } = await setupTenantAdminSession(browser);
  const observedReplyResponses = [];

  page.on("response", async (response) => {
    const pathname = new URL(response.url()).pathname;
    if (!/\/internal\/assistant\/conversations\/[^/]+\/turns\/[^/]+:reply$/.test(pathname)) {
      return;
    }
    let body = {};
    try {
      body = await response.json();
    } catch {
      body = {};
    }
    observedReplyResponses.push({ status: response.status(), body });
  });

  await page.goto("/app/assistant/librechat");
  await expect(page.getByRole("heading", { name: "LibreChat" })).toBeVisible();

  await typePromptInIframe(page, "在鲜花组织之下，新建一个名为运营部的部门。");

  await expect
    .poll(() => observedReplyResponses.length, { timeout: 120_000 })
    .toBeGreaterThan(0);

  const response = observedReplyResponses.at(-1);
  expect(response?.status).toBe(200);
  expect(String(response?.body.reply_model_name || "")).toBe("gpt-5.2");
  expect(String(response?.body.reply_source || "")).toBe("model");
  expect(response?.body.used_fallback).toBe(false);
  expect(["missing_fields", "draft"]).toContain(String(response?.body.stage || ""));
  expect(String(response?.body.text || "").trim().length).toBeGreaterThan(0);
  expect(String(response?.body.text || "")).not.toContain("ai_plan_schema_constrained_decode_failed");

  const streamLocator = page
    .frameLocator("[data-testid='librechat-standalone-frame']")
    .locator("[data-assistant-dialog-stream='1']");
  await expect(streamLocator.first()).toBeVisible({ timeout: 120_000 });
  await expect(streamLocator.first()).toContainText(/补充|日期|成立/i);

  await appContext.close();
});
