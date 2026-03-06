import { expect } from "@playwright/test";

export const assistantMainRoute = "/app/assistant";

export async function gotoAIConversationPage(page) {
  await page.goto(assistantMainRoute);
  await expect(page).toHaveURL(/\/app\/assistant$/);
  await expect(page.getByRole("heading", { name: "AI 助手" })).toBeVisible();
  await expect(page.getByTestId("assistant-librechat-frame")).toBeVisible();
  await expect(page.getByTestId("librechat-standalone-frame")).toHaveCount(0);
}

export async function readAssistantBridgeChannelNonce(page) {
  const frame = page.getByTestId("assistant-librechat-frame");
  await expect(frame).toBeVisible();
  const src = await frame.getAttribute("src");
  const iframeURL = new URL(src || "", "http://localhost:8080");
  return {
    channel: iframeURL.searchParams.get("channel"),
    nonce: iframeURL.searchParams.get("nonce")
  };
}

export async function dispatchAssistantBridgeMessage(page, payload) {
  await page.evaluate((data) => {
    window.dispatchEvent(new MessageEvent("message", { data, origin: window.location.origin }));
  }, payload);
}

export function assistantChatFrame(page) {
  return page.frameLocator("[data-testid='assistant-librechat-frame']");
}

export function assistantDialogStream(page) {
  return assistantChatFrame(page).locator("[role='log'] [data-assistant-dialog-stream='1']");
}

export async function typePromptInAssistantChat(page, input) {
  const frame = assistantChatFrame(page);
  const textarea = frame.locator("textarea").first();
  await expect(textarea).toBeVisible({ timeout: 120_000 });
  await textarea.fill(input);
  await frame.getByRole("button", { name: /发送|send/i }).click();
}

export async function expectAssistantDialogStoplines(page) {
  await expect(page.getByTestId("librechat-standalone-frame")).toHaveCount(0);
  await expect(page.locator("[data-assistant-dialog-stream='1']")).toHaveCount(0);
  await expect(page.getByText(/Connection error/i)).toHaveCount(0);
  await expect(assistantChatFrame(page).getByText(/Connection error/i)).toHaveCount(0);
}
