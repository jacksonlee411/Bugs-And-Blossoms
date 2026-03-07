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

function frameHTML() {
  return `<!doctype html>
<html>
  <head><meta charset="utf-8" /></head>
  <body>
    <div data-testid="conversation-container">
      <div role="log" aria-label="Assistant Transcript"></div>
    </div>
    <form id="chat-form">
      <textarea aria-label="Message input"></textarea>
      <button type="submit" aria-label="Send">Send</button>
    </form>
    <script>
      window.__tp266Native = { submitCount: 0, clickCount: 0 };
      document.getElementById('chat-form').addEventListener('submit', function () {
        window.__tp266Native.submitCount += 1;
      });
      document.querySelector('button').addEventListener('click', function () {
        window.__tp266Native.clickCount += 1;
      });
    </script>
    <script src="/assistant-ui/bridge.js"></script>
  </body>
</html>`;
}

function makeTurn() {
  return {
    turn_id: "turn_tp266_1",
    user_input: "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。",
    state: "validated",
    risk_tier: "high",
    request_id: "assistant_req_tp266_1",
    trace_id: "assistant_trace_tp266_1",
    policy_version: "v1",
    composition_version: "v1",
    mapping_version: "v1",
    intent: {
      action: "create_orgunit",
      parent_ref_text: "鲜花组织",
      entity_name: "运营部",
      effective_date: "2026-01-01"
    },
    ambiguity_count: 0,
    confidence: 0.95,
    candidates: [],
    resolved_candidate_id: "",
    plan: {
      title: "创建组织计划",
      capability_key: "org.orgunit_create.field_policy",
      summary: "在指定父组织下创建部门"
    },
    dry_run: {
      explain: "计划已生成，等待确认后可提交",
      diff: [{ field: "name", after: "运营部" }],
      validation_errors: []
    }
  };
}

function makeConversation(turns) {
  return {
    conversation_id: "conv_tp266_1",
    tenant_id: "tenant_tp266",
    actor_id: "actor_tp266",
    actor_role: "tenant-admin",
    state: "validated",
    created_at: "2026-03-06T00:00:00Z",
    updated_at: "2026-03-06T00:00:00Z",
    turns
  };
}

test("tp266-e2e-001: LibreChat standalone page must block native send and embed single reply in chat bubble", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser);
  const observedTurnRequests = [];
  let currentConversation = makeConversation([]);

  await page.route("**/assistant-ui/?**", async (route) => {
    await route.fulfill({ status: 200, contentType: "text/html", body: frameHTML() });
  });

  await page.route("**/internal/assistant/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "GET" && pathname === "/internal/assistant/runtime-status") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "healthy",
          checked_at: "2026-03-06T00:00:00Z",
          upstream: { reachable: true, code: "ok", message: "mock", url: "http://localhost:3080" },
          services: [{ name: "assistant", status: "ok", reason: "mock" }],
          capabilities: { mcp_enabled: false, actions_enabled: true, agents_write_enabled: true }
        })
      });
      return;
    }
    if (method === "GET" && pathname === "/internal/assistant/conversations") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ items: [], next_cursor: "" })
      });
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations") {
      currentConversation = makeConversation([]);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp266_1/turns") {
      let body = {};
      try {
        body = request.postDataJSON ? request.postDataJSON() : {};
      } catch {
        body = {};
      }
      observedTurnRequests.push(body);
      currentConversation = makeConversation([makeTurn()]);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp266_1/turns/turn_tp266_1:reply") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          text: "已生成待确认草案：将在鲜花组织下创建运营部，请回复确认执行。",
          kind: "info",
          stage: "draft",
          reply_model_name: "gpt-5.2",
          reply_prompt_version: "assistant.reply.v1",
          reply_source: "model",
          used_fallback: false,
          conversation_id: "conv_tp266_1",
          turn_id: "turn_tp266_1"
        })
      });
      return;
    }

    await route.continue();
  });

  await page.goto("/app/assistant/librechat");
  await expect(page.getByRole("heading", { name: "LibreChat" })).toBeVisible();
  const frame = page.frameLocator("[data-testid='librechat-standalone-frame']");
  const textarea = frame.getByLabel("Message input");
  await expect(textarea).toBeVisible();
  await textarea.fill("在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。");
  await frame.getByRole("button", { name: "Send" }).click();

  await expect.poll(() => observedTurnRequests.length).toBeGreaterThan(0);

  const bubble = frame.locator('[role="log"] [data-assistant-dialog-stream="1"] [data-assistant-turn-id="turn_tp266_1"]');
  await expect(bubble).toHaveCount(1);
  await expect(bubble).toBeVisible();

  await expect
    .poll(() => page.evaluate(() => {
      const iframe = document.querySelector('[data-testid="librechat-standalone-frame"]');
      return iframe?.contentWindow?.__assistantBridgeMetrics?.bridge_reply_embedded ?? 0;
    }), { timeout: 30_000 })
    .toBeGreaterThan(0);

  const { metrics, nativeCounts } = await page.evaluate(() => {
    const iframe = document.querySelector('[data-testid="librechat-standalone-frame"]');
    const win = iframe?.contentWindow;
    return {
      metrics: win?.__assistantBridgeMetrics || null,
      nativeCounts: win?.__tp266Native || null
    };
  });
  expect(metrics.native_send_attempted).toBeGreaterThan(0);
  expect(metrics.native_send_blocked).toBeGreaterThan(0);
  expect(metrics.native_send_emitted).toBe(0);
  expect(metrics.prompt_emitted).toBe(1);
  expect(metrics.bridge_reply_embedded).toBeGreaterThan(0);
  expect(nativeCounts.submitCount).toBe(0);
  expect(nativeCounts.clickCount).toBe(0);
  const messageID = await bubble.getAttribute("data-assistant-message-id");
  expect(String(messageID || "")).toContain("conv_tp266_1");
  expect(String(messageID || "")).toContain("turn_tp266_1");
  await expect(bubble).toContainText(/运营部/);
  await expect(bubble).toContainText(/确认执行|待确认/);
  await expect(bubble).toHaveAttribute("data-assistant-conversation-id", "conv_tp266_1");
  await expect(bubble).toHaveAttribute("data-assistant-turn-id", "turn_tp266_1");
  await expect(bubble).toHaveAttribute("data-assistant-request-id", "assistant_req_tp266_1");

  await page.evaluate((resolvedMessageID) => {
    const iframe = document.querySelector('[data-testid="librechat-standalone-frame"]');
    const iframeURL = new URL(iframe.getAttribute("src"), window.location.origin);
    iframe.contentWindow.postMessage({
      type: "assistant.flow.dialog",
      channel: iframeURL.searchParams.get("channel"),
      nonce: iframeURL.searchParams.get("nonce"),
      payload: {
        message_id: resolvedMessageID,
        kind: "success",
        stage: "draft",
        text: "已更新为同轮唯一气泡。",
        meta: {
          conversation_id: "conv_tp266_1",
          turn_id: "turn_tp266_1",
          request_id: "assistant_req_tp266_1"
        }
      }
    }, window.location.origin);
  }, messageID);
  await expect(frame.locator('[role="log"] [data-assistant-dialog-stream="1"] [data-assistant-turn-id="turn_tp266_1"]')).toHaveCount(1);
  await expect(bubble).toContainText("已更新为同轮唯一气泡。");

  await expect(page.locator('[data-assistant-dialog-stream="1"]')).toHaveCount(0);
  await expect(frame.locator('body > [data-assistant-dialog-stream="1"]')).toHaveCount(0);
  await expect(page.getByText(/Connection error/i)).toHaveCount(0);
  await expect(frame.getByText(/Connection error/i)).toHaveCount(0);

  await appContext.close();
});
