import { expect } from "@playwright/test";

export const assistantMainRoute = "/app/assistant/librechat";

export async function gotoAIConversationPage(page) {
  await page.goto(assistantMainRoute);
  await expect(page).toHaveURL(/\/app\/assistant\/librechat$/);
  await expect(page.getByRole("heading", { name: "LibreChat" })).toBeVisible();
  await expect(page.getByTestId("librechat-standalone-frame")).toBeVisible();
  await expect(page.getByTestId("assistant-librechat-frame")).toHaveCount(0);
  await expect
    .poll(() =>
      page.evaluate(() =>
        Array.isArray(window.__assistantBridgeAudit)
          ? window.__assistantBridgeAudit.some((item) => item && item.kind === "bridge_ready")
          : false
      ),
    { timeout: 30000 })
    .toBe(true);
}

export async function readAssistantBridgeChannelNonce(page) {
  const frame = page.getByTestId("librechat-standalone-frame");
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
  return page.frameLocator("[data-testid='librechat-standalone-frame']");
}

export function assistantDialogStream(page) {
  return assistantChatFrame(page).locator("[data-assistant-dialog-stream='1']");
}

export async function typePromptInAssistantChat(page, input) {
  const frame = assistantChatFrame(page);
  const textareaCandidates = [
    frame.getByTestId("text-input").first(),
    frame.locator('textarea:not([aria-hidden="true"])').first(),
    frame.locator('[contenteditable="true"]:not([aria-hidden="true"])').first(),
    frame.locator("textarea").first()
  ];
  let textarea = textareaCandidates[textareaCandidates.length - 1];
  for (const candidate of textareaCandidates) {
    if ((await candidate.count()) === 0) {
      continue;
    }
    const visible = await candidate.isVisible().catch(() => false);
    if (visible) {
      textarea = candidate;
      break;
    }
  }
  await expect(textarea).toBeVisible({ timeout: 120_000 });
  await textarea.fill(input);
  const sendButtonCandidates = [
    frame.getByTestId("send-button").first(),
    frame.getByRole("button", { name: /发送|send/i }).first(),
    frame.locator('button:not([aria-hidden="true"])').first()
  ];
  let sendButton = sendButtonCandidates[sendButtonCandidates.length - 1];
  for (const candidate of sendButtonCandidates) {
    if ((await candidate.count()) === 0) {
      continue;
    }
    const visible = await candidate.isVisible().catch(() => false);
    if (visible) {
      sendButton = candidate;
      break;
    }
  }
  await expect(sendButton).toBeVisible({ timeout: 120_000 });
  await sendButton.click();
}

export async function expectAssistantDialogStoplines(page) {
  await expect(page.getByTestId("librechat-standalone-frame")).toBeVisible();
  await expect(page.locator("[data-assistant-dialog-stream='1']")).toHaveCount(0);
}

export async function waitForAssistantReplyResponse(page, timeout = 120_000) {
  try {
    const response = await page.waitForResponse((resp) => {
      try {
        const pathname = new URL(resp.url()).pathname;
        return /\/internal\/assistant\/conversations\/[^/]+\/turns\/[^/]+:reply$/.test(pathname);
      } catch {
        return false;
      }
    }, { timeout: Math.min(timeout, 30_000) });
    const body = await response.json().catch(() => ({}));
    if (String(body?.reply_model_name || '').trim().length > 0) {
      return { response, body };
    }
  } catch {}

  const bubble = assistantChatFrame(page)
    .locator('[data-assistant-dialog-stream="1"] [data-assistant-reply-model-name]')
    .last();
  await expect(bubble).toBeVisible({ timeout });
  const body = await bubble.evaluate((node) => ({
    text: String(node.textContent || '').trim(),
    kind: node.getAttribute('data-assistant-kind') || '',
    stage: node.getAttribute('data-assistant-stage') || '',
    reply_model_name: node.getAttribute('data-assistant-reply-model-name') || '',
    reply_prompt_version: node.getAttribute('data-assistant-reply-prompt-version') || '',
    reply_source: node.getAttribute('data-assistant-reply-source') || '',
    used_fallback: (node.getAttribute('data-assistant-used-fallback') || '').toLowerCase() === 'true',
    conversation_id: node.getAttribute('data-assistant-conversation-id') || '',
    turn_id: node.getAttribute('data-assistant-turn-id') || '',
    request_id: node.getAttribute('data-assistant-request-id') || '',
    trace_id: node.getAttribute('data-assistant-trace-id') || ''
  }));
  return {
    response: {
      status() {
        return 200;
      }
    },
    body
  };
}
