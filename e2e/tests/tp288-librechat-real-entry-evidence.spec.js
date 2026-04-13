import path from "node:path";
import { expect, test } from "@playwright/test";

import { ensureDir, writeJSON } from "./helpers/evidence.js";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const repoRoot = path.resolve(__dirname, "..", "..");
const tp288EvidenceRoot = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-266");
const tp288RetiredReason =
  "historical mock evidence retired; active live successor coverage is owned by tp288b/tp290b";

const tp288DefaultCommand =
  `pnpm --dir ${path.join(repoRoot, "e2e")} exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1`;

// Keep this file path stable for historical doc references, but retire it from active E2E gates.
test.beforeEach(async () => {
  test.skip(true, tp288RetiredReason);
});

const tp288StaleOn = [
  "290A pending placeholder bubble fix merged",
  "240C runtime gate semantics changed",
  "240D durable execution/compensation semantics changed",
  "240E MCP write admission semantics changed",
  "message binding/render path changed",
  "routing/authn chain changed",
  "error code semantics changed",
  "fail-closed behavior changed",
];

async function collectBubbleSnapshot(surface) {
  return surface.locator("[data-assistant-binding-key]").evaluateAll((nodes) =>
    nodes.map((node) => ({
      binding_key: node.getAttribute("data-assistant-binding-key") || "",
      conversation_id: node.getAttribute("data-assistant-conversation-id") || "",
      turn_id: node.getAttribute("data-assistant-turn-id") || "",
      request_id: node.getAttribute("data-assistant-request-id") || "",
      message_id: node.getAttribute("data-assistant-message-id") || "",
      text: (node.textContent || "").replace(/\s+/g, " ").trim(),
    })),
  );
}

function buildBindingAssertion(expectedBindings, bubbles) {
  const actualCounts = new Map();
  for (const bubble of bubbles) {
    actualCounts.set(bubble.binding_key, (actualCounts.get(bubble.binding_key) || 0) + 1);
  }

  const bindings = expectedBindings.map((binding) => {
    const bubble = bubbles.find((item) => item.binding_key === binding.binding_key);
    return {
      ...binding,
      actual_count: actualCounts.get(binding.binding_key) || 0,
      attrs_match:
        Boolean(bubble) &&
        bubble.conversation_id === binding.conversation_id &&
        bubble.turn_id === binding.turn_id &&
        bubble.request_id === binding.request_id,
      message_id: bubble?.message_id || "",
    };
  });

  return {
    all_expected_bindings_present_once: bindings.every(
      (binding) => binding.actual_count === 1 && binding.attrs_match,
    ),
    unique_binding_key_count: new Set(bubbles.map((bubble) => bubble.binding_key)).size,
    bindings,
  };
}

function assistantErrorCodeFromCall(call) {
  if (call?.json && typeof call.json === "object" && typeof call.json.code === "string") {
    return call.json.code.trim();
  }
  const body = String(call?.body || "").trim();
  if (!body) {
    return "";
  }
  try {
    const parsed = JSON.parse(body);
    return typeof parsed?.code === "string" ? parsed.code.trim() : "";
  } catch {
    return "";
  }
}

function buildReceiptTaskAssertion(network) {
  const commitReceipts = network.internalCalls.filter(
    (call) =>
      call.method === "POST" &&
      call.path.endsWith(":commit") &&
      call.status === 202 &&
      call.json &&
      typeof call.json.task_id === "string",
  );
  const taskPollCalls = network.internalCalls.filter(
    (call) =>
      call.method === "GET" &&
      call.path.startsWith("/internal/assistant/tasks/") &&
      call.json &&
      typeof call.json.status === "string",
  );
  const assistantErrorCodes = network.internalCalls
    .map((call) => assistantErrorCodeFromCall(call))
    .filter(Boolean);
  const polledTaskIDs = new Set(
    taskPollCalls
      .map((call) => String(call?.json?.task_id || "").trim())
      .filter(Boolean),
  );
  const missingReceiptPolls = commitReceipts
    .map((call) => String(call?.json?.task_id || "").trim())
    .filter((taskID) => taskID && !polledTaskIDs.has(taskID));

  return {
    commit_receipt_count: commitReceipts.length,
    commit_receipts: commitReceipts.map((call) => ({
      path: call.path,
      status: call.status,
      task_id: String(call?.json?.task_id || "").trim(),
      poll_uri: String(call?.json?.poll_uri || "").trim(),
      workflow_id: String(call?.json?.workflow_id || "").trim(),
      has_turns_payload: Boolean(call?.json && Object.prototype.hasOwnProperty.call(call.json, "turns")),
    })),
    task_poll_count: taskPollCalls.length,
    task_poll_paths: taskPollCalls.map((call) => call.path),
    task_poll_status_sequence: taskPollCalls.map((call) => String(call?.json?.status || "").trim()),
    invalid_task_poll_paths: [...network.invalidTaskPollPaths],
    missing_receipt_polls: missingReceiptPolls,
    assistant_error_codes: assistantErrorCodes,
    passed:
      network.invalidTaskPollPaths.length === 0 &&
      missingReceiptPolls.length === 0 &&
      !assistantErrorCodes.includes("assistant_task_dispatch_failed") &&
      commitReceipts.every((call) => !Object.prototype.hasOwnProperty.call(call.json, "turns")),
  };
}

function assertTp288ReceiptTaskContract(network, expectedReceiptCount) {
  const receiptTaskAssertion = buildReceiptTaskAssertion(network);
  expect(receiptTaskAssertion.commit_receipt_count).toBe(expectedReceiptCount);
  expect(receiptTaskAssertion.invalid_task_poll_paths).toEqual([]);
  expect(receiptTaskAssertion.missing_receipt_polls).toEqual([]);
  expect(receiptTaskAssertion.assistant_error_codes).not.toContain("assistant_task_dispatch_failed");
  expect(receiptTaskAssertion.commit_receipts.every((item) => item.status === 202)).toBe(true);
  expect(receiptTaskAssertion.commit_receipts.every((item) => Boolean(item.task_id) && Boolean(item.poll_uri))).toBe(true);
  expect(receiptTaskAssertion.commit_receipts.every((item) => item.poll_uri === `/internal/assistant/tasks/${item.task_id}`)).toBe(true);
  expect(receiptTaskAssertion.commit_receipts.every((item) => item.has_turns_payload === false)).toBe(true);
  expect(receiptTaskAssertion.task_poll_count).toBeGreaterThanOrEqual(expectedReceiptCount);
  expect(receiptTaskAssertion.passed).toBe(true);
  return receiptTaskAssertion;
}

async function persistTp288Evidence({
  appContext,
  page,
  surface,
  caseID,
  title,
  result,
  expectedBubbleCount,
  expectedBindings,
  expectedTexts,
  network,
}) {
  await ensureDir(tp288EvidenceRoot);

  const executedAt = new Date().toISOString();
  const command = process.env.TP288_EVIDENCE_COMMAND || tp288DefaultCommand;
  const casePrefix = `tp288-e2e-${caseID}`;
  const pagePath = path.join(tp288EvidenceRoot, `${casePrefix}-page.png`);
  const domPath = path.join(tp288EvidenceRoot, `${casePrefix}-dom.json`);
  const networkPath = path.join(tp288EvidenceRoot, `${casePrefix}-network.json`);
  const assertionsPath = path.join(tp288EvidenceRoot, `${casePrefix}-assertions.json`);

  await page.screenshot({ path: pagePath, fullPage: true });
  const bubbles = await collectBubbleSnapshot(surface);
  const bindingAssertion = buildBindingAssertion(expectedBindings, bubbles);
  const receiptTaskAssertion = buildReceiptTaskAssertion(network);
  const pageTextCounts = {};
  for (const text of expectedTexts) {
    pageTextCounts[text] = await page.getByText(text).count();
  }

  const domPayload = {
    plan: "DEV-PLAN-288",
    case_id: casePrefix,
    title,
    captured_at: executedAt,
    formal_entry: "/app/assistant/librechat",
    bubble_count: bubbles.length,
    bubbles,
  };

  const networkPayload = {
    plan: "DEV-PLAN-288",
    case_id: casePrefix,
    captured_at: executedAt,
    internal_post_paths: [...network.internalPostPaths],
    native_post_paths: [...network.nativePostPaths],
    native_send_emitted: network.nativePostPaths.length,
    task_poll_paths: receiptTaskAssertion.task_poll_paths,
    invalid_task_poll_paths: receiptTaskAssertion.invalid_task_poll_paths,
    commit_receipts: receiptTaskAssertion.commit_receipts,
    assistant_error_codes: receiptTaskAssertion.assistant_error_codes,
  };

  const assertionsPayload = {
    plan: "DEV-PLAN-288",
    case_id: casePrefix,
    title,
    formal_entry: "/app/assistant/librechat",
    command,
    executed_at: executedAt,
    result,
    stale_on: tp288StaleOn,
    stopline: {
      no_native_send_post: network.nativePostPaths.length === 0,
      official_message_tree_only: expectedTexts.every((text) => pageTextCounts[text] === 1),
      single_assistant_bubble: bindingAssertion.all_expected_bindings_present_once,
      conversation_turn_request_binding_unique:
        bindingAssertion.all_expected_bindings_present_once &&
        bindingAssertion.unique_binding_key_count === bubbles.length,
      expected_bubble_count_match: bubbles.length === expectedBubbleCount,
      no_invalid_task_poll_path: receiptTaskAssertion.invalid_task_poll_paths.length === 0,
      no_assistant_task_dispatch_failed:
        !receiptTaskAssertion.assistant_error_codes.includes("assistant_task_dispatch_failed"),
      receipt_task_chain_matched: receiptTaskAssertion.missing_receipt_polls.length === 0,
    },
    page_text_counts: pageTextCounts,
    expected_bubble_count: expectedBubbleCount,
    actual_bubble_count: bubbles.length,
    binding_assertion: bindingAssertion,
    receipt_task_contract: receiptTaskAssertion,
  };

  await writeJSON(domPath, domPayload);
  await writeJSON(networkPath, networkPayload);
  await writeJSON(assertionsPath, assertionsPayload);

  return {
    command,
    executedAt,
    artifacts: {
      page: path.relative(repoRoot, pagePath),
      dom: path.relative(repoRoot, domPath),
      network: path.relative(repoRoot, networkPath),
      assertions: path.relative(repoRoot, assertionsPath),
    },
  };
}

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

async function createTP288Session(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  return setupTenantAdminSession(browser, {
    tenantName: `TP288 Tenant ${runID}`,
    tenantHost: `t-tp288-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp288-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp288-${runID}@example.invalid`,
    createPage: true
  });
}

async function installFormalEntryMock(page, options = {}) {
  const state = {
    turns: [],
    createTurnCount: 0,
    internalPostPaths: [],
    nativePostPaths: [],
    firstTurnCommitFailed: false,
    taskByID: {},
    internalCalls: [],
    invalidTaskPollPaths: [],
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

  const recordInternalCall = (method, path, status, payload) => {
    const item = {
      method,
      path,
      status,
      body: "",
      json: null,
    };
    if (payload && typeof payload === "object") {
      item.json = JSON.parse(JSON.stringify(payload));
    } else if (typeof payload === "string") {
      item.body = payload;
    }
    state.internalCalls.push(item);
  };

  const fulfillInternalJSON = async (route, method, path, status, payload) => {
    recordInternalCall(method, path, status, payload);
    await fulfillJSON(route, status, payload);
  };

  const nowISO = () => new Date().toISOString();

  const buildTaskReceiptForTurn = (turn) => {
    const taskID = `task_${turn.turn_id}`;
    const submittedAt = nowISO();
    state.taskByID[taskID] = {
      task_id: taskID,
      task_type: "assistant_async_plan",
      status: "queued",
      dispatch_status: "pending",
      attempt: 0,
      max_attempts: 3,
      last_error_code: "",
      workflow_id: `wf_${turn.turn_id}`,
      request_id: turn.request_id,
      trace_id: turn.trace_id,
      conversation_id: "conv_tp288_1",
      turn_id: turn.turn_id,
      submitted_at: submittedAt,
      updated_at: submittedAt,
      poll_count: 0,
    };
    const receipt = {
      task_id: taskID,
      task_type: "assistant_async_plan",
      status: "queued",
      workflow_id: `wf_${turn.turn_id}`,
      submitted_at: submittedAt,
      poll_uri: `/internal/assistant/tasks/${taskID}`,
    };
    return receipt;
  };

  await page.route("**/internal/assistant/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "POST" && pathname === "/internal/assistant/conversations") {
      await fulfillInternalJSON(route, method, pathname, 200, buildConversation(state.turns));
      return;
    }

    if (method === "GET" && pathname === "/internal/assistant/conversations/conv_tp288_1") {
      await fulfillInternalJSON(route, method, pathname, 200, buildConversation(state.turns));
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
      await fulfillInternalJSON(route, method, pathname, 200, buildConversation(state.turns));
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
      await fulfillInternalJSON(route, method, pathname, 200, buildConversation(state.turns));
      return;
    }

    if (method === "POST" && pathname.endsWith(":commit")) {
      const activeTurn = state.turns[state.turns.length - 1];
      if (options.failFirstCommit && activeTurn.turn_id === "turn_tp288_1" && !state.firstTurnCommitFailed) {
        state.firstTurnCommitFailed = true;
        await fulfillInternalJSON(route, method, pathname, 409, {
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
      await fulfillInternalJSON(route, method, pathname, 202, buildTaskReceiptForTurn(activeTurn));
      return;
    }

    if (method === "GET" && pathname.startsWith("/internal/assistant/tasks/")) {
      const taskID = pathname.replace("/internal/assistant/tasks/", "").trim();
      if (!taskID || taskID === "undefined") {
        state.invalidTaskPollPaths.push(pathname);
        await fulfillInternalJSON(route, method, pathname, 404, {
          code: "assistant_task_not_found",
          message: "assistant task not found",
        });
        return;
      }
      const task = state.taskByID[taskID];
      if (!task) {
        state.invalidTaskPollPaths.push(pathname);
        await fulfillInternalJSON(route, method, pathname, 404, {
          code: "assistant_task_not_found",
          message: "assistant task not found",
        });
        return;
      }
      task.poll_count += 1;
      if (task.poll_count >= 1) {
        task.status = "succeeded";
        task.dispatch_status = "started";
        task.attempt = 1;
        task.updated_at = nowISO();
      }
      await fulfillInternalJSON(route, method, pathname, 200, {
        task_id: task.task_id,
        task_type: task.task_type,
        status: task.status,
        dispatch_status: task.dispatch_status,
        attempt: task.attempt,
        max_attempts: task.max_attempts,
        last_error_code: task.last_error_code,
        workflow_id: task.workflow_id,
        request_id: task.request_id,
        trace_id: task.trace_id,
        conversation_id: task.conversation_id,
        turn_id: task.turn_id,
        submitted_at: task.submitted_at,
        updated_at: task.updated_at,
      });
      return;
    }

    await route.continue();
  });

  return state;
}

async function openFormalEntry(page) {
  await page.goto("/app/assistant/librechat");

  const iframeLocator = page.locator("main iframe").first();
  if ((await iframeLocator.count()) > 0) {
    await expect(iframeLocator).toBeVisible({ timeout: 60_000 });
    const iframeHandle = await iframeLocator.elementHandle();
    const iframe = await iframeHandle.contentFrame();
    await iframe.waitForLoadState("domcontentloaded");
    await iframe.evaluate(() => {
      window.history.replaceState({}, "", "/app/assistant/librechat/c/new");
    });
    const surface = page.frameLocator("main iframe").first();
    await expect(surface.getByRole("textbox").last()).toBeVisible({ timeout: 60_000 });
    return surface;
  }

  await page.evaluate(() => {
    window.history.replaceState({}, "", "/app/assistant/librechat/c/new");
  });
  await expect(page.getByRole("textbox").last()).toBeVisible({ timeout: 60_000 });
  return page;
}

async function sendFromFormalEntry(surface, text) {
  const input = surface.getByRole("textbox").last();
  await input.fill(text);
  await surface.getByRole("button", { name: /发送消息|Send message/i }).click();
}

test("tp288-e2e-001: real entry success path stays in one official bubble", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await createTP288Session(browser, "001");
  await ensureDir(tp288EvidenceRoot);
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
  await expect(page.getByText("已创建华东运营部。")).toHaveCount(1);
  await expect(page.getByText("assistant_task_dispatch_failed", { exact: false })).toHaveCount(0);

  expect(network.nativePostPaths).toEqual([]);
  expect(network.internalPostPaths).toEqual([
    "/internal/assistant/conversations",
    "/internal/assistant/conversations/conv_tp288_1/turns",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:confirm",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:commit",
  ]);
  assertTp288ReceiptTaskContract(network, 1);

  await persistTp288Evidence({
    appContext,
    page,
    surface,
    caseID: "001",
    title: "real entry success path stays in one official bubble",
    result: "passed",
    expectedBubbleCount: 1,
    expectedBindings: [
      {
        binding_key: turn1BindingKey,
        conversation_id: "conv_tp288_1",
        turn_id: "turn_tp288_1",
        request_id: "req_tp288_1",
      },
    ],
    expectedTexts: ["已创建华东运营部。", "将在鲜花组织下创建华东运营部。"],
    network,
  });

  await appContext.close();
});

test("tp288-e2e-002: failure stays in-bubble and retry creates exactly one new bubble", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await createTP288Session(browser, "002");
  await ensureDir(tp288EvidenceRoot);
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
  await expect(page.getByText("提交失败，请重试。")).toHaveCount(1);

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
  await expect(page.getByText("已创建华北运营部。")).toHaveCount(1);
  await expect(page.getByText("assistant_task_dispatch_failed", { exact: false })).toHaveCount(0);

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
  assertTp288ReceiptTaskContract(network, 1);

  await persistTp288Evidence({
    appContext,
    page,
    surface,
    caseID: "002",
    title: "failure stays in-bubble and retry creates exactly one new bubble",
    result: "passed",
    expectedBubbleCount: 2,
    expectedBindings: [
      {
        binding_key: turn1BindingKey,
        conversation_id: "conv_tp288_1",
        turn_id: "turn_tp288_1",
        request_id: "req_tp288_1",
      },
      {
        binding_key: turn2BindingKey,
        conversation_id: "conv_tp288_1",
        turn_id: "turn_tp288_2",
        request_id: "req_tp288_2",
      },
    ],
    expectedTexts: ["提交失败，请重试。", "已创建华北运营部。"],
    network,
  });

  await appContext.close();
});
