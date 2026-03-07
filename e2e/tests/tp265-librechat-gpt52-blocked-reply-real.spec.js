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

test("tp265-e2e-001: AI对话 blocked reply still must go through gpt-5.2 without fallback", async ({ browser }) => {
  test.setTimeout(300_000);
  const { appContext, page } = await setupTenantAdminSession(browser);

  await gotoAIConversationPage(page);

  const replyPromise = waitForAssistantReplyResponse(page);
  await typePromptInAssistantChat(page, "在鲜花组织之下，新建一个名为运营部的部门。");

  const { response, body } = await replyPromise;
  expect(response.status()).toBe(200);
  expect(String(body.reply_model_name || "")).toBe("gpt-5.2");
  expect(String(body.reply_source || "")).toBe("model");
  expect(body.used_fallback).toBe(false);
  expect(["missing_fields", "draft", "commit_failed"]).toContain(String(body.stage || ""));
  expect(String(body.text || "").trim().length).toBeGreaterThan(0);
  expect(String(body.text || "")).not.toContain("ai_plan_schema_constrained_decode_failed");

  const frameBody = assistantChatFrame(page).locator("body");
  await expect(frameBody).toContainText(/补充|日期|成立|Connection error|Something went wrong/i, { timeout: 120_000 });
  const streamLocator = assistantDialogStream(page);
  if (await streamLocator.count()) {
    await expect(streamLocator.first()).toContainText(/补充|日期|成立|Connection error|Something went wrong/i);
  }
  await expectAssistantDialogStoplines(page);

  await appContext.close();
});
