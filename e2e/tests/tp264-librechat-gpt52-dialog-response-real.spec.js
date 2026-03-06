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

test("tp264-e2e-001: AI对话 real typing must render gpt-5.2 reply via :reply", async ({ browser }) => {
  test.setTimeout(300_000);
  const { appContext, page } = await setupTenantAdminSession(browser);
  const observedReplyRequests = [];
  const observedReplyResponses = [];

  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    if (!/\/internal\/assistant\/conversations\/[^/]+\/turns\/[^/]+:reply$/.test(pathname)) {
      return;
    }
    let body = {};
    try {
      body = request.postDataJSON ? request.postDataJSON() : {};
    } catch {
      body = {};
    }
    observedReplyRequests.push({ pathname, body });
  });

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
    observedReplyResponses.push({ pathname, status: response.status(), body });
  });

  await gotoAIConversationPage(page);

  await typePromptInAssistantChat(
    page,
    "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。"
  );

  await expect
    .poll(() => observedReplyRequests.length, { timeout: 120_000 })
    .toBeGreaterThan(0);
  await expect
    .poll(() => observedReplyResponses.length, { timeout: 120_000 })
    .toBeGreaterThan(0);

  const request = observedReplyRequests.at(-1);
  const response = observedReplyResponses.at(-1);

  expect(response?.status).toBe(200);
  expect(String(response?.body.reply_model_name || "")).toBe("gpt-5.2");
  expect(String(response?.body.reply_prompt_version || "")).toBe("assistant.reply.v1");
  expect(String(response?.body.reply_source || "")).toBe("model");
  expect(response?.body.used_fallback).toBe(false);
  expect(String(response?.body.conversation_id || "")).not.toBe("");
  expect(String(response?.body.turn_id || "")).not.toBe("");
  expect(String(response?.body.text || "").trim().length).toBeGreaterThan(0);
  expect(String(response?.body.text || "")).not.toContain("ai_plan_schema_constrained_decode_failed");

  expect(String(request?.body.stage || "")).not.toBe("");
  expect(String(request?.body.kind || "")).not.toBe("");
  expect(String(request?.body.outcome || "")).not.toBe("");
  expect(Object.prototype.hasOwnProperty.call(request?.body || {}, "fallback_text")).toBe(false);
  expect(Object.prototype.hasOwnProperty.call(request?.body || {}, "allow_missing_turn")).toBe(false);

  const streamLocator = assistantDialogStream(page);
  await expect(streamLocator.first()).toBeVisible({ timeout: 120_000 });
  await expect(streamLocator.first()).not.toContainText("ai_plan_schema_constrained_decode_failed");
  await expectAssistantDialogStoplines(page);

  await expect(page.getByText("ai_plan_schema_constrained_decode_failed")).toHaveCount(0);

  await appContext.close();
});
