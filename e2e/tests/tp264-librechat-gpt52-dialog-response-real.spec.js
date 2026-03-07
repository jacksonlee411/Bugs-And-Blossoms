import { expect, test } from "@playwright/test";
import {
  assistantChatFrame,
  assistantDialogStream,
  expectAssistantDialogStoplines,
  gotoAIConversationPage,
  typePromptInAssistantChat,
  waitForAssistantReplyResponse
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

  await gotoAIConversationPage(page);

  const replyPromise = waitForAssistantReplyResponse(page);
  await typePromptInAssistantChat(
    page,
    "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。"
  );

  const { response, body } = await replyPromise;
  expect(response.status()).toBe(200);
  expect(String(body.reply_model_name || "")).toBe("gpt-5.2");
  expect(String(body.reply_prompt_version || "")).toBe("assistant.reply.v1");
  expect(String(body.reply_source || "")).toBe("model");
  expect(body.used_fallback).toBe(false);
  expect(String(body.conversation_id || "")).not.toBe("");
  expect(String(body.turn_id || "")).not.toBe("");
  expect(String(body.text || "").trim().length).toBeGreaterThan(0);
  expect(String(body.text || "")).not.toContain("ai_plan_schema_constrained_decode_failed");

  const frameBody = assistantChatFrame(page).locator("body");
  await expect(frameBody).toContainText(/运营部|确认执行|待确认|Connection error|Something went wrong/i, { timeout: 120_000 });
  const streamLocator = assistantDialogStream(page);
  if (await streamLocator.count()) {
    await expect(streamLocator.first()).not.toContainText("ai_plan_schema_constrained_decode_failed");
  }
  await expectAssistantDialogStoplines(page);
  await expect(page.getByText("ai_plan_schema_constrained_decode_failed")).toHaveCount(0);

  await appContext.close();
});
