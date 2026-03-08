import { expect, test } from "@playwright/test";

const liveRuntimeOnly = process.env.TP288_USE_EXISTING_RUNTIME === "1";

test.skip(!liveRuntimeOnly, "tp288 requires an existing interactive runtime on /app/assistant/librechat");

function buildReply(text, kind, stage, turnId) {
  return {
    text,
    kind,
    stage,
    conversation_id: "conv_tp288_1",
    turn_id: turnId,
  };
}

function buildTurn({
  turnId,
  requestId,
  traceId,
  userInput,
  state,
  phase,
  pendingDraftSummary,
  replyText,
  replyKind = "info",
  replyStage = "draft",
  commitResult = null,
  commitReply = null,
  errorCode = "",
}) {
  return {
    turn_id: turnId,
    user_input: userInput,
    state,
    phase,
    risk_tier: "low",
    request_id: requestId,
    trace_id: traceId,
    plan_hash: `plan-${turnId}`,
    policy_version: "v1",
    composition_version: "v1",
    mapping_version: "v1",
    pending_draft_summary: pendingDraftSummary,
    missing_fields: [],
    candidates: [],
    selected_candidate_id: "FLOWER-A",
    commit_result: commitResult,
    commit_reply: commitReply,
    error_code: errorCode,
    reply_nlg: buildReply(replyText, replyKind, replyStage, turnId),
  };
}

function buildConversation(turns) {
  return {
    conversation_id: "conv_tp288_1",
    tenant_id: "tenant_tp288",
    actor_id: "actor_tp288",
    actor_role: "tenant-admin",
    state: turns.length === 0 ? "draft" : turns[turns.length - 1].state,
    created_at: "2026-03-08T00:00:00Z",
    updated_at: "2026-03-08T00:00:00Z",
    turns,
  };
}

async function setupLocalAdminSession(browser) {
  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const appContext = await browser.newContext({ baseURL: appBaseURL });
  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: {
      email: process.env.E2E_LOCAL_ADMIN_EMAIL || "admin@localhost",
      password: process.env.E2E_LOCAL_ADMIN_PASS || "admin123",
    },
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);
  const page = await appContext.newPage();
  return { appContext, page };
}

async function installFormalEntryMock(page, options = {}) {
  const state = {
    turns: [],
    createTurnCount: 0,
    internalPostPaths: [],
    nativePostPaths: [],
    firstTurnCommitFailed: false,
  };

  page.on("request", (request) => {
    if (request.method() !== "POST") {
      return;
    }
    const pathname = new URL(request.url()).pathname;
    if (pathname.startsWith("/internal/assistant/")) {
      state.internalPostPaths.push(pathname);
      return;
    }
    if (pathname.includes("/api/agents/chat") || pathname.startsWith("/api/messages")) {
      state.nativePostPaths.push(pathname);
    }
  });

  const fulfillJSON = async (route, status, payload) => {
    await route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(payload),
    });
  };

  await page.route("**/internal/assistant/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "POST" && pathname === "/internal/assistant/conversations") {
      await fulfillJSON(route, 200, buildConversation(state.turns));
      return;
    }

    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp288_1/turns") {
      state.createTurnCount += 1;
      if (state.createTurnCount === 1) {
        state.turns.push(
          buildTurn({
            turnId: "turn_tp288_1",
            requestId: "req_tp288_1",
            traceId: "trace_tp288_1",
            userInput: "创建华东运营部",
            state: "validated",
            phase: "await_commit_confirm",
            pendingDraftSummary: "将在鲜花组织下创建华东运营部。",
            replyText: "草案已生成，请先确认。",
          }),
        );
      } else {
        state.turns.push(
          buildTurn({
            turnId: "turn_tp288_2",
            requestId: "req_tp288_2",
            traceId: "trace_tp288_2",
            userInput: "重新创建华北运营部",
            state: "validated",
            phase: "await_commit_confirm",
            pendingDraftSummary: "将在鲜花组织下创建华北运营部。",
            replyText: "第二轮草案已生成，请先确认。",
          }),
        );
      }
      await fulfillJSON(route, 200, buildConversation(state.turns));
      return;
    }

    if (method === "POST" && pathname.endsWith(":confirm")) {
      const activeTurn = state.turns[state.turns.length - 1];
      activeTurn.state = "confirmed";
      activeTurn.phase = "await_commit_confirm";
      activeTurn.reply_nlg = buildReply(
        activeTurn.turn_id === "turn_tp288_1" ? "已确认第一轮草案，现在可以提交。" : "已确认第二轮草案，现在可以提交。",
        "info",
        "confirmed",
        activeTurn.turn_id,
      );
      await fulfillJSON(route, 200, buildConversation(state.turns));
      return;
    }

    if (method === "POST" && pathname.endsWith(":commit")) {
      const activeTurn = state.turns[state.turns.length - 1];
      if (options.failFirstCommit && activeTurn.turn_id === "turn_tp288_1" && !state.firstTurnCommitFailed) {
        state.firstTurnCommitFailed = true;
        await fulfillJSON(route, 409, {
          code: "assistant_commit_failed",
          message: "提交失败，请重试。",
          trace_id: activeTurn.trace_id,
        });
        return;
      }
      activeTurn.state = "committed";
      activeTurn.phase = "completed";
      activeTurn.commit_result = {
        org_code: activeTurn.turn_id === "turn_tp288_1" ? "AI2881" : "AI2882",
        parent_org_code: "FLOWER-A",
        effective_date: "2026-03-08",
        event_type: "orgunit_created",
        event_uuid: `evt_${activeTurn.turn_id}`,
      };
      activeTurn.commit_reply = {
        outcome: "success",
        stage: "committed",
        kind: "success",
        text: activeTurn.turn_id === "turn_tp288_1" ? "已创建华东运营部。" : "已创建华北运营部。",
      };
      activeTurn.reply_nlg = buildReply(
        activeTurn.turn_id === "turn_tp288_1" ? "已创建华东运营部。" : "已创建华北运营部。",
        "success",
        "committed",
        activeTurn.turn_id,
      );
      await fulfillJSON(route, 200, buildConversation(state.turns));
      return;
    }

    await route.continue();
  });

  return state;
}

async function openFormalEntry(page) {
  await page.goto("/app/assistant/librechat");
  await expect(page.locator("main iframe").first()).toBeVisible({ timeout: 60_000 });
  const iframeHandle = await page.locator("main iframe").first().elementHandle();
  const iframe = await iframeHandle.contentFrame();
  await iframe.waitForLoadState("domcontentloaded");
  await iframe.evaluate(() => {
    window.history.replaceState({}, "", "/app/assistant/librechat/c/new");
  });
  const surface = page.frameLocator("main iframe").first();
  await expect(surface.getByRole("textbox").last()).toBeVisible({ timeout: 60_000 });
  return surface;
}

async function sendFromFormalEntry(surface, text) {
  const input = surface.getByRole("textbox").last();
  await input.fill(text);
  await surface.getByRole("button", { name: /发送消息|Send message/i }).click();
}

test("tp288-e2e-001: real entry success path stays in one official bubble", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupLocalAdminSession(browser);
  const network = await installFormalEntryMock(page);
  const surface = await openFormalEntry(page);
  const turn1BindingKey = "conv_tp288_1::turn_tp288_1::req_tp288_1";
  const turn1Bubble = surface.locator(`[data-assistant-binding-key="${turn1BindingKey}"]`);

  network.internalPostPaths.length = 0;
  network.nativePostPaths.length = 0;

  await sendFromFormalEntry(surface, "创建华东运营部");

  await expect(turn1Bubble).toHaveCount(1);
  await expect(turn1Bubble).toContainText("草案已生成，请先确认。");
  await expect(turn1Bubble).toContainText("将在鲜花组织下创建华东运营部。");
  await expect(turn1Bubble).toHaveAttribute("data-assistant-conversation-id", "conv_tp288_1");
  await expect(turn1Bubble).toHaveAttribute("data-assistant-turn-id", "turn_tp288_1");
  await expect(turn1Bubble).toHaveAttribute("data-assistant-request-id", "req_tp288_1");
  await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(0);
  await expect(surface.getByRole("button", { name: /确认|Confirm/i })).toHaveCount(1);

  await surface.getByRole("button", { name: /确认|Confirm/i }).click();
  await expect(turn1Bubble).toHaveCount(1);
  await expect(turn1Bubble).toContainText("已确认第一轮草案，现在可以提交。");
  await expect(surface.getByRole("button", { name: /确认|Confirm/i })).toHaveCount(0);
  await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(1);

  await surface.getByRole("button", { name: /提交|Submit/i }).click();
  await expect(turn1Bubble).toHaveCount(1);
  await expect(turn1Bubble).toContainText("已创建华东运营部。");
  await expect(turn1Bubble).toContainText("org_code: AI2881");
  await expect(surface.locator("[data-assistant-binding-key]")).toHaveCount(1);
  await expect(page.getByText("已创建华东运营部。")).toHaveCount(0);

  expect(network.nativePostPaths).toEqual([]);
  expect(network.internalPostPaths).toEqual([
    "/internal/assistant/conversations",
    "/internal/assistant/conversations/conv_tp288_1/turns",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:confirm",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:commit",
  ]);

  await appContext.close();
});

test("tp288-e2e-002: failure stays in-bubble and retry creates exactly one new bubble", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupLocalAdminSession(browser);
  const network = await installFormalEntryMock(page, { failFirstCommit: true });
  const surface = await openFormalEntry(page);
  const turn1BindingKey = "conv_tp288_1::turn_tp288_1::req_tp288_1";
  const turn2BindingKey = "conv_tp288_1::turn_tp288_2::req_tp288_2";
  const turn1Bubble = surface.locator(`[data-assistant-binding-key="${turn1BindingKey}"]`);
  const turn2Bubble = surface.locator(`[data-assistant-binding-key="${turn2BindingKey}"]`);

  network.internalPostPaths.length = 0;
  network.nativePostPaths.length = 0;

  await sendFromFormalEntry(surface, "创建华东运营部");
  await expect(turn1Bubble).toHaveCount(1);
  await surface.getByRole("button", { name: /确认|Confirm/i }).click();
  await surface.getByRole("button", { name: /提交|Submit/i }).click();

  await expect(turn1Bubble).toHaveCount(1);
  await expect(turn1Bubble).toContainText("提交失败，请重试。");
  await expect(surface.getByRole("button", { name: /确认|Confirm/i })).toHaveCount(0);
  await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(0);
  await expect(surface.locator("[data-assistant-binding-key]")).toHaveCount(1);
  await expect(page.getByText("提交失败，请重试。")).toHaveCount(0);

  await sendFromFormalEntry(surface, "重新创建华北运营部");
  await expect(turn2Bubble).toHaveCount(1);
  await expect(turn2Bubble).toContainText("第二轮草案已生成，请先确认。");
  await expect(surface.locator("[data-assistant-binding-key]")).toHaveCount(2);

  await surface.getByRole("button", { name: /确认|Confirm/i }).click();
  await surface.getByRole("button", { name: /提交|Submit/i }).click();

  await expect(turn2Bubble).toHaveCount(1);
  await expect(turn2Bubble).toContainText("已创建华北运营部。");
  await expect(turn2Bubble).toContainText("org_code: AI2882");
  await expect(turn1Bubble).toHaveCount(1);
  await expect(turn2Bubble).toHaveAttribute("data-assistant-conversation-id", "conv_tp288_1");
  await expect(turn2Bubble).toHaveAttribute("data-assistant-turn-id", "turn_tp288_2");
  await expect(turn2Bubble).toHaveAttribute("data-assistant-request-id", "req_tp288_2");
  await expect(surface.locator("[data-assistant-binding-key]")).toHaveCount(2);
  await expect(page.getByText("已创建华北运营部。")).toHaveCount(0);

  expect(network.nativePostPaths).toEqual([]);
  expect(network.internalPostPaths).toEqual([
    "/internal/assistant/conversations",
    "/internal/assistant/conversations/conv_tp288_1/turns",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:confirm",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:commit",
    "/internal/assistant/conversations/conv_tp288_1/turns",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_2:confirm",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_2:commit",
  ]);

  await appContext.close();
});
