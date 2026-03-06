import { expect, test } from "@playwright/test";
import {
  assistantDialogStream,
  expectAssistantDialogStoplines,
  gotoAIConversationPage,
  typePromptInAssistantChat
} from "./helpers/assistant-dialog";

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

test("tp265-e2e-001: AI对话 blocked reply still must go through gpt-5.2 without fallback", async ({ browser }) => {
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

  await gotoAIConversationPage(page);

  await typePromptInAssistantChat(page, "在鲜花组织之下，新建一个名为运营部的部门。");

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

  const streamLocator = assistantDialogStream(page);
  await expect(streamLocator.first()).toBeVisible({ timeout: 120_000 });
  await expect(streamLocator.first()).toContainText(/补充|日期|成立/i);
  await expectAssistantDialogStoplines(page);

  await appContext.close();
});
