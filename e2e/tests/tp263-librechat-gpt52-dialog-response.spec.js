import { expect, test } from "@playwright/test";
import {
  assistantDialogStream,
  dispatchAssistantBridgeMessage,
  expectAssistantDialogStoplines,
  gotoAIConversationPage,
  readAssistantBridgeChannelNonce
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

function makeTurn(overrides = {}) {
  return {
    turn_id: "turn_tp263_1",
    user_input: "input",
    state: "validated",
    risk_tier: "high",
    request_id: "assistant_req_tp263_1",
    trace_id: "assistant_trace_tp263_1",
    policy_version: "v1",
    composition_version: "v1",
    mapping_version: "v1",
    intent: {
      action: "create_orgunit",
      parent_ref_text: "鲜花组织",
      entity_name: "运营部",
      effective_date: "2026-01-01"
    },
    ambiguity_count: 1,
    confidence: 0.9,
    candidates: [
      {
        candidate_id: "FLOWERS-ROOT",
        candidate_code: "FLOWERS-ROOT",
        name: "鲜花组织",
        path: "/集团/鲜花组织",
        as_of: "2026-01-01",
        is_active: true,
        match_score: 0.95
      }
    ],
    resolved_candidate_id: "FLOWERS-ROOT",
    plan: {
      title: "创建组织计划",
      capability_key: "org.orgunit_create.field_policy",
      summary: "在指定父组织下创建部门",
      model_provider: "openai",
      model_name: "gpt-5.2"
    },
    dry_run: {
      explain: "计划已生成，等待确认后可提交",
      diff: [{ field: "name", after: "运营部" }],
      validation_errors: []
    },
    ...overrides
  };
}

function makeConversation(turns) {
  return {
    conversation_id: "conv_tp263_1",
    tenant_id: "tenant_tp263",
    actor_id: "actor_tp263",
    actor_role: "tenant-admin",
    state: "validated",
    created_at: "2026-03-06T00:00:00Z",
    updated_at: "2026-03-06T00:00:00Z",
    turns
  };
}

async function readBridgeChannelNonce(page) {
  return readAssistantBridgeChannelNonce(page);
}

async function dispatchBridgeMessage(page, payload) {
  await dispatchAssistantBridgeMessage(page, payload);
}

test("tp263-e2e-001: AI对话 reply must go through gpt-5.2 reply pipeline", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser);

  const observedReplyCalls = [];
  let createTurnCalls = 0;
  let currentConversation = makeConversation([]);

  await page.route("**/internal/assistant/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "POST" && pathname === "/internal/assistant/conversations") {
      currentConversation = makeConversation([]);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
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
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp263_1/turns") {
      createTurnCalls += 1;
      currentConversation = makeConversation([makeTurn()]);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }
    if (method === "POST" && /\/internal\/assistant\/conversations\/[^/]+\/turns\/[^/]+:reply$/.test(pathname)) {
      const body = request.postDataJSON ? request.postDataJSON() : {};
      observedReplyCalls.push(body);
      const pathSegments = pathname.split("/");
      const conversationID = pathSegments[4] || "conv_tp263_1";
      const turnAction = pathSegments[6] || "turn_tp263_1:reply";
      const turnID = String(turnAction).split(":")[0] || "turn_tp263_1";
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          text: "已收到需求，正在为你生成可确认草案。",
          kind: "info",
          stage: "draft",
          reply_model_name: "gpt-5.2",
          reply_prompt_version: "assistant.reply.v1",
          conversation_id: conversationID,
          turn_id: turnID
        })
      });
      return;
    }

    await route.continue();
  });

  await gotoAIConversationPage(page);

  const { channel, nonce } = await readBridgeChannelNonce(page);
  expect(channel).toBeTruthy();
  expect(nonce).toBeTruthy();

  await dispatchBridgeMessage(page, {
    type: "assistant.bridge.ready",
    channel,
    nonce,
    payload: { source: "assistant-ui-bridge" }
  });

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: {
      input: "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。"
    }
  });

  await expect.poll(() => createTurnCalls, { timeout: 15_000 }).toBeGreaterThan(0);
  await expect.poll(() => observedReplyCalls.length, { timeout: 15_000 }).toBeGreaterThan(0);
  expect(String(observedReplyCalls[0]?.stage || "")).toBe("draft");
  expect(String(observedReplyCalls[0]?.fallback_text || "")).not.toContain("ai_plan_schema_constrained_decode_failed");
  await expect(assistantDialogStream(page).first()).toContainText("已收到需求，正在为你生成可确认草案。", { timeout: 15_000 });
  await expectAssistantDialogStoplines(page);
  await expect(page.getByText("ai_plan_schema_constrained_decode_failed")).toHaveCount(0);

  await appContext.close();
});
