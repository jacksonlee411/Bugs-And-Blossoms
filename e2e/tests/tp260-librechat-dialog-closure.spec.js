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

function makeTurn(overrides = {}) {
  return {
    turn_id: "turn_tp260_1",
    user_input: "input",
    state: "validated",
    risk_tier: "high",
    request_id: "assistant_req_tp260_1",
    trace_id: "assistant_trace_tp260_1",
    policy_version: "v1",
    composition_version: "v1",
    mapping_version: "v1",
    intent: {
      action: "create_orgunit",
      parent_ref_text: "AI治理办公室",
      entity_name: "人力资源部2",
      effective_date: "2026-01-01"
    },
    ambiguity_count: 1,
    confidence: 0.88,
    candidates: [
      {
        candidate_id: "AI-GOV-A",
        candidate_code: "AI-GOV-A",
        name: "AI治理办公室",
        path: "/集团/AI治理办公室",
        as_of: "2026-01-01",
        is_active: true,
        match_score: 0.95
      }
    ],
    resolved_candidate_id: "AI-GOV-A",
    plan: {
      title: "创建组织计划",
      capability_key: "org.orgunit_create.field_policy",
      summary: "在指定父组织下创建部门"
    },
    dry_run: {
      explain: "计划已生成，可提交",
      diff: [{ field: "name", after: "人力资源部2" }],
      validation_errors: []
    },
    ...overrides
  };
}

function makeConversation(turns) {
  return {
    conversation_id: "conv_tp260_1",
    tenant_id: "tenant_tp260",
    actor_id: "actor_tp260",
    actor_role: "tenant-admin",
    state: "validated",
    created_at: "2026-03-05T00:00:00Z",
    updated_at: "2026-03-05T00:00:00Z",
    turns
  };
}

async function installTp260Mock(page) {
  const stats = {
    createTurnCalls: 0,
    confirmCalls: 0,
    commitCalls: 0,
    lastConfirmCandidateID: ""
  };
  let currentConversation = makeConversation([]);

  const ambiguousCandidates = [
    {
      candidate_id: "SSC-1",
      candidate_code: "SSC-1",
      name: "共享服务中心",
      path: "/集团/共享服务中心/一部",
      as_of: "2026-03-26",
      is_active: true,
      match_score: 0.91
    },
    {
      candidate_id: "SSC-2",
      candidate_code: "SSC-2",
      name: "共享服务中心",
      path: "/集团/共享服务中心/二部",
      as_of: "2026-03-26",
      is_active: true,
      match_score: 0.9
    }
  ];

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
    if (method === "GET" && pathname === "/internal/assistant/conversations/conv_tp260_1") {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp260_1/turns") {
      const body = request.postDataJSON ? request.postDataJSON() : {};
      const input = String(body?.user_input || "");
      stats.createTurnCalls += 1;

      if (input.includes("239A补全") && !input.includes("2026-03-25")) {
        currentConversation = makeConversation([
          makeTurn({
            ambiguity_count: 0,
            candidates: [],
            resolved_candidate_id: "",
            intent: {
              action: "create_orgunit",
              parent_ref_text: "AI治理办公室",
              entity_name: "人力资源部239A补全",
              effective_date: ""
            },
            dry_run: {
              explain: "缺少生效日期",
              diff: [],
              validation_errors: ["missing_effective_date"]
            }
          })
        ]);
      } else if (input.includes("239A补全")) {
        currentConversation = makeConversation([
          makeTurn({
            intent: {
              action: "create_orgunit",
              parent_ref_text: "AI治理办公室",
              entity_name: "人力资源部239A补全",
              effective_date: "2026-03-25"
            },
            dry_run: {
              explain: "计划已生成，可提交",
              diff: [{ field: "effective_date", after: "2026-03-25" }],
              validation_errors: []
            }
          })
        ]);
      } else if (input.includes("239A候选验证部")) {
        currentConversation = makeConversation([
          makeTurn({
            ambiguity_count: 2,
            candidates: ambiguousCandidates,
            resolved_candidate_id: "",
            intent: {
              action: "create_orgunit",
              parent_ref_text: "共享服务中心",
              entity_name: "239A候选验证部",
              effective_date: "2026-03-26"
            },
            dry_run: {
              explain: "检测到多个候选",
              diff: [],
              validation_errors: []
            }
          })
        ]);
      } else {
        currentConversation = makeConversation([makeTurn()]);
      }

      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp260_1/turns/turn_tp260_1:confirm") {
      const body = request.postDataJSON ? request.postDataJSON() : {};
      const selectedCandidate = String(body?.candidate_id || "") || currentConversation.turns?.[0]?.resolved_candidate_id || "AI-GOV-A";
      stats.confirmCalls += 1;
      stats.lastConfirmCandidateID = selectedCandidate;
      currentConversation = makeConversation([
        makeTurn({
          state: "confirmed",
          resolved_candidate_id: selectedCandidate,
          candidates: selectedCandidate.startsWith("SSC-") ? ambiguousCandidates : makeTurn().candidates
        })
      ]);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp260_1/turns/turn_tp260_1:commit") {
      stats.commitCalls += 1;
      const parent = currentConversation.turns?.[0]?.resolved_candidate_id || "AI-GOV-A";
      const effectiveDate = currentConversation.turns?.[0]?.intent?.effective_date || "2026-01-01";
      currentConversation = makeConversation([
        makeTurn({
          state: "committed",
          resolved_candidate_id: parent,
          commit_result: {
            org_code: "TP260-ORG-1",
            parent_org_code: parent,
            effective_date: effectiveDate,
            event_type: "CREATE",
            event_uuid: "evt-tp260-1"
          }
        })
      ]);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(currentConversation) });
      return;
    }

    await route.continue();
  });

  return stats;
}

async function readBridgeChannelNonce(page) {
  const frame = page.getByTestId("librechat-standalone-frame");
  await expect(frame).toBeVisible();
  const src = await frame.getAttribute("src");
  const iframeURL = new URL(src || "", "http://localhost:8080");
  return {
    channel: iframeURL.searchParams.get("channel"),
    nonce: iframeURL.searchParams.get("nonce")
  };
}

async function dispatchBridgeMessage(page, payload) {
  await page.evaluate((data) => {
    window.dispatchEvent(new MessageEvent("message", { data, origin: window.location.origin }));
  }, payload);
}

test("tp260-e2e-001: case1~4 dialogue-closure flow", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser);
  const stats = await installTp260Mock(page);
  await page.goto("/app/assistant/librechat");
  await expect(page.getByRole("heading", { name: "LibreChat" })).toBeVisible();

  const { channel, nonce } = await readBridgeChannelNonce(page);
  expect(channel).toBeTruthy();
  expect(nonce).toBeTruthy();

  await dispatchBridgeMessage(page, {
    type: "assistant.bridge.ready",
    channel,
    nonce,
    payload: { source: "assistant-ui-bridge" }
  });
  await expect(page.getByText("自动执行通道已连接：可直接在 LibreChat 对话中输入需求。")).toBeVisible();

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01" }
  });
  await expect.poll(() => stats.createTurnCalls).toBe(1);
  await expect.poll(() => stats.confirmCalls).toBe(0);
  await expect.poll(() => stats.commitCalls).toBe(0);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "确认执行" }
  });
  await expect.poll(() => stats.confirmCalls).toBe(1);
  await expect.poll(() => stats.commitCalls).toBe(1);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "在 AI治理办公室 下新建 人力资源部239A补全" }
  });
  await expect.poll(() => stats.createTurnCalls).toBe(2);
  await expect.poll(() => stats.confirmCalls).toBe(1);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "生效日期 2026-03-25" }
  });
  await expect.poll(() => stats.createTurnCalls).toBe(3);
  await expect.poll(() => stats.confirmCalls).toBe(1);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "确认提交" }
  });
  await expect.poll(() => stats.confirmCalls).toBe(2);
  await expect.poll(() => stats.commitCalls).toBe(2);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26" }
  });
  await expect.poll(() => stats.createTurnCalls).toBe(4);
  await expect.poll(() => stats.confirmCalls).toBe(2);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "选第2个" }
  });
  await expect.poll(() => stats.confirmCalls).toBe(2);
  await expect.poll(() => stats.commitCalls).toBe(2);

  await dispatchBridgeMessage(page, {
    type: "assistant.prompt.sync",
    channel,
    nonce,
    payload: { input: "是的，确认" }
  });
  await expect.poll(() => stats.confirmCalls).toBe(3);
  await expect.poll(() => stats.commitCalls).toBe(3);
  await expect.poll(() => stats.lastConfirmCandidateID).toBe("SSC-2");

  await appContext.close();
});
