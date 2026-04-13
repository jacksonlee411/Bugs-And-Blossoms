import { expect, test } from "@playwright/test";
import path from "node:path";

import { ensureDir, writeJSON } from "./helpers/evidence.js";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const repoRoot = path.resolve(__dirname, "..", "..");
const EVIDENCE_ROOT = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-290");
const tp290RetiredReason =
  "historical mock matrix retired from active gates; live successor coverage is owned by tp290b";

// Keep this file path stable for historical doc references, but retire it from active E2E gates.
test.beforeEach(async () => {
  test.skip(true, tp290RetiredReason);
});

const CASE_INPUTS = {
  1: ["你好"],
  2: ["在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01", "确认"],
  3: ["在 AI治理办公室 下新建 人力资源部239A补全", "生效日期 2026-03-25", "确认"],
  4: ["在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26", "选第2个", "是的"],
};

const CASE_EXPECTED_PHASES = {
  1: ["idle", "idle"],
  2: ["idle", "await_commit_confirm", "committing", "committed"],
  3: ["idle", "await_missing_fields", "await_commit_confirm", "committing", "committed"],
  4: [
    "idle",
    "await_candidate_pick",
    "await_candidate_confirm",
    "await_commit_confirm",
    "committing",
    "committed",
  ],
};

function normalizeText(value) {
  return String(value || "").trim().replace(/\s+/g, " ");
}

function evidencePaths(caseId) {
  return {
    page: `${EVIDENCE_ROOT}/case-${caseId}-page.png`,
    dom: `${EVIDENCE_ROOT}/case-${caseId}-dom.json`,
    network: `${EVIDENCE_ROOT}/case-${caseId}-network.har`,
    trace: `${EVIDENCE_ROOT}/case-${caseId}-trace.zip`,
    phase: `${EVIDENCE_ROOT}/case-${caseId}-phase-assertions.json`,
  };
}

function buildReply(text, kind, stage, turnId) {
  return {
    text,
    kind,
    stage,
    conversation_id: "",
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
  missingFields = [],
  candidates = [],
  selectedCandidateID = "",
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
    missing_fields: missingFields,
    candidates,
    selected_candidate_id: selectedCandidateID,
    commit_result: commitResult,
    commit_reply: commitReply,
    error_code: errorCode,
    reply_nlg: buildReply(replyText, replyKind, replyStage, turnId),
  };
}

function buildConversation(conversationID, turns) {
  return {
    conversation_id: conversationID,
    tenant_id: "tenant_tp290",
    actor_id: "actor_tp290",
    actor_role: "tenant-admin",
    state: turns.length === 0 ? "draft" : turns[turns.length - 1].state,
    created_at: "2026-03-08T00:00:00Z",
    updated_at: "2026-03-08T00:00:00Z",
    turns,
  };
}

async function createTP290Session(browser, suffix, harPath) {
  const runID = `${Date.now()}-${suffix}`;
  return setupTenantAdminSession(browser, {
    tenantName: `TP290 Tenant ${runID}`,
    tenantHost: `t-tp290-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp290-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp290-${runID}@example.invalid`,
    createPage: true,
    appContextOptions: {
      recordHar: {
        path: harPath,
        content: "embed",
        mode: "full",
      },
    },
  });
}

function safeJSON(request) {
  try {
    return request.postDataJSON();
  } catch {
    return {};
  }
}

function caseCandidateList() {
  return [
    {
      candidate_id: "cand-1",
      candidate_code: "FLOWER-A",
      name: "共享服务中心（候选1）",
      path: "集团/共享服务中心",
      as_of: "2026-03-26",
      is_active: true,
      match_score: 0.91,
    },
    {
      candidate_id: "cand-2",
      candidate_code: "FLOWER-B",
      name: "共享服务中心（候选2）",
      path: "集团/共享服务中心/B",
      as_of: "2026-03-26",
      is_active: true,
      match_score: 0.96,
    },
  ];
}

function configureCaseProgression(caseId, state, userInput) {
  const normalized = normalizeText(userInput);
  const createCount = state.createTurnCount;
  const turnId = `turn_tp290_${caseId}`;
  const requestId = `req_tp290_${caseId}`;
  const traceId = `trace_tp290_${caseId}`;

  if (caseId === 1) {
    if (createCount !== 1 || normalized !== normalizeText(CASE_INPUTS[1][0])) {
      return { status: 422, error: "unexpected_case_1_input" };
    }
    state.turn = buildTurn({
      turnId,
      requestId,
      traceId,
      userInput: normalized,
      state: "validated",
      phase: "idle",
      pendingDraftSummary: "",
      replyText: "你好，我在。",
      replyKind: "info",
      replyStage: "draft",
    });
    state.phaseHistory.push("idle");
    return { status: 200 };
  }

  if (caseId === 2) {
    if (createCount === 1 && normalized === normalizeText(CASE_INPUTS[2][0])) {
      state.turn = buildTurn({
        turnId,
        requestId,
        traceId,
        userInput: normalized,
        state: "validated",
        phase: "await_commit_confirm",
        pendingDraftSummary: "将在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01。",
        replyText: "草案已生成，请确认后提交。",
        replyKind: "info",
        replyStage: "draft",
      });
      state.phaseHistory.push("await_commit_confirm");
      return { status: 200 };
    }
    if (createCount === 2 && normalized === normalizeText(CASE_INPUTS[2][1])) {
      state.turn.state = "confirmed";
      state.turn.phase = "await_commit_confirm";
      state.turn.reply_nlg = buildReply("已确认草案，点击提交继续。", "info", "candidate_confirm", turnId);
      state.phaseHistory.push("await_commit_confirm");
      return { status: 200 };
    }
    return { status: 422, error: "unexpected_case_2_input" };
  }

  if (caseId === 3) {
    if (createCount === 1 && normalized === normalizeText(CASE_INPUTS[3][0])) {
      state.turn = buildTurn({
        turnId,
        requestId,
        traceId,
        userInput: normalized,
        state: "validated",
        phase: "await_missing_fields",
        pendingDraftSummary: "",
        missingFields: ["effective_date"],
        replyText: "缺少生效日期，请补充后继续。",
        replyKind: "warning",
        replyStage: "missing_fields",
      });
      state.phaseHistory.push("await_missing_fields");
      return { status: 200 };
    }
    if (createCount === 2 && normalized === normalizeText(CASE_INPUTS[3][1])) {
      state.turn.state = "validated";
      state.turn.phase = "await_commit_confirm";
      state.turn.missing_fields = [];
      state.turn.pending_draft_summary = "补全完成：生效日期 2026-03-25。";
      state.turn.reply_nlg = buildReply("缺字段已补全，请确认后提交。", "info", "draft", turnId);
      state.phaseHistory.push("await_commit_confirm");
      return { status: 200 };
    }
    if (createCount === 3 && normalized === normalizeText(CASE_INPUTS[3][2])) {
      state.turn.state = "confirmed";
      state.turn.phase = "await_commit_confirm";
      state.turn.reply_nlg = buildReply("已确认草案，点击提交继续。", "info", "candidate_confirm", turnId);
      state.phaseHistory.push("await_commit_confirm");
      return { status: 200 };
    }
    return { status: 422, error: "unexpected_case_3_input" };
  }

  if (caseId === 4) {
    if (createCount === 1 && normalized === normalizeText(CASE_INPUTS[4][0])) {
      state.turn = buildTurn({
        turnId,
        requestId,
        traceId,
        userInput: normalized,
        state: "validated",
        phase: "await_candidate_pick",
        pendingDraftSummary: "",
        candidates: caseCandidateList(),
        replyText: "检测到多个候选父组织，请先选择。",
        replyKind: "warning",
        replyStage: "candidate_list",
      });
      state.phaseHistory.push("await_candidate_pick");
      return { status: 200 };
    }
    if (createCount === 2 && normalized === normalizeText(CASE_INPUTS[4][1])) {
      state.turn.state = "validated";
      state.turn.phase = "await_candidate_confirm";
      state.turn.selected_candidate_id = "cand-2";
      state.turn.reply_nlg = buildReply("已定位候选 2，请确认后继续。", "info", "candidate_confirm", turnId);
      state.phaseHistory.push("await_candidate_confirm");
      return { status: 200 };
    }
    if (createCount === 3 && normalized === normalizeText(CASE_INPUTS[4][2])) {
      state.turn.state = "confirmed";
      state.turn.phase = "await_commit_confirm";
      state.turn.reply_nlg = buildReply("候选与草案已确认，点击提交继续。", "info", "candidate_confirm", turnId);
      state.phaseHistory.push("await_commit_confirm");
      return { status: 200 };
    }
    return { status: 422, error: "unexpected_case_4_input" };
  }

  return { status: 500, error: "unknown_case" };
}

async function installCaseMock(page, caseId) {
  const state = {
    conversationId: `conv_tp290_${caseId}`,
    turn: null,
    createTurnCount: 0,
    phaseHistory: [],
    internalPostPaths: [],
    nativePostPaths: [],
    taskByID: {},
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
      conversation_id: state.conversationId,
      turn_id: turn.turn_id,
      submitted_at: submittedAt,
      updated_at: submittedAt,
      poll_count: 0,
    };
    return {
      task_id: taskID,
      task_type: "assistant_async_plan",
      status: "queued",
      workflow_id: `wf_${turn.turn_id}`,
      submitted_at: submittedAt,
      poll_uri: `/internal/assistant/tasks/${taskID}`,
    };
  };

  await page.route("**/internal/assistant/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "POST" && pathname === "/internal/assistant/conversations") {
      await fulfillJSON(route, 200, buildConversation(state.conversationId, state.turn ? [state.turn] : []));
      return;
    }

    if (method === "GET" && pathname === `/internal/assistant/conversations/${state.conversationId}`) {
      await fulfillJSON(route, 200, buildConversation(state.conversationId, state.turn ? [state.turn] : []));
      return;
    }

    if (method === "POST" && pathname === `/internal/assistant/conversations/${state.conversationId}/turns`) {
      state.createTurnCount += 1;
      const payload = safeJSON(request);
      const progress = configureCaseProgression(caseId, state, payload.user_input);
      if (progress.status !== 200) {
        await fulfillJSON(route, progress.status, {
          code: progress.error,
          message: `invalid input for case ${caseId}`,
        });
        return;
      }
      await fulfillJSON(route, 200, buildConversation(state.conversationId, state.turn ? [state.turn] : []));
      return;
    }

    if (
      method === "POST" &&
      state.turn &&
      pathname === `/internal/assistant/conversations/${state.conversationId}/turns/${state.turn.turn_id}:commit`
    ) {
      if (state.turn.state !== "confirmed") {
        await fulfillJSON(route, 409, {
          code: "conversation_confirmation_required",
          message: "Confirmation is required before commit.",
          trace_id: state.turn.trace_id,
        });
        return;
      }
      state.phaseHistory.push("committing");
      state.turn.state = "committed";
      state.turn.phase = "committed";
      state.turn.commit_result = {
        org_code: `AI290${caseId}`,
        parent_org_code: caseId === 4 ? "FLOWER-B" : "FLOWER-A",
        effective_date: caseId === 2 ? "2026-01-01" : caseId === 3 ? "2026-03-25" : caseId === 4 ? "2026-03-26" : "2026-03-08",
        event_type: "orgunit_created",
        event_uuid: `evt_tp290_${caseId}`,
      };
      state.turn.commit_reply = {
        outcome: "success",
        stage: "committed",
        kind: "success",
        text: `Case ${caseId} 提交成功。`,
      };
      state.turn.reply_nlg = buildReply(`Case ${caseId} 提交成功。`, "success", "commit_result", state.turn.turn_id);
      state.phaseHistory.push("committed");
      await fulfillJSON(route, 202, buildTaskReceiptForTurn(state.turn));
      return;
    }

    if (method === "GET" && pathname.startsWith("/internal/assistant/tasks/")) {
      const taskID = pathname.replace("/internal/assistant/tasks/", "").trim();
      const task = state.taskByID[taskID];
      if (!task) {
        await fulfillJSON(route, 404, {
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
      await fulfillJSON(route, 200, {
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
    return { surface, usedIframe: true };
  }

  await page.evaluate(() => {
    window.history.replaceState({}, "", "/app/assistant/librechat/c/new");
  });
  await expect(page.getByRole("textbox").last()).toBeVisible({ timeout: 60_000 });
  return { surface: page, usedIframe: false };
}

async function sendFromFormalEntry(surface, text) {
  const input = surface.getByRole("textbox").last();
  await input.fill(text);
  await surface.getByRole("button", { name: /发送消息|Send message/i }).click();
}

async function collectDOMEvidence(page, surface) {
  const bubbleLocator = surface.locator("[data-assistant-binding-key]");
  const bubbleCount = await bubbleLocator.count();
  const bubbles = [];
  for (let i = 0; i < bubbleCount; i += 1) {
    const item = bubbleLocator.nth(i);
    bubbles.push({
      binding_key: await item.getAttribute("data-assistant-binding-key"),
      conversation_id: await item.getAttribute("data-assistant-conversation-id"),
      turn_id: await item.getAttribute("data-assistant-turn-id"),
      request_id: await item.getAttribute("data-assistant-request-id"),
      message_id: await item.getAttribute("data-assistant-message-id"),
      text: normalizeText(await item.innerText()),
    });
  }

  const externalReplyContainerCount = await page.locator("[data-assistant-dialog-stream]").count();
  const connectionErrorCount =
    (await page.getByText("Connection error", { exact: false }).count()) +
    (await page.getByText("连接错误", { exact: false }).count());

  return {
    url: page.url(),
    bubble_count: bubbleCount,
    bubbles,
    external_reply_container_count: externalReplyContainerCount,
    connection_error_count: connectionErrorCount,
  };
}

function expectedInternalPaths(caseId, conversationId, turnId) {
  const base = `/internal/assistant/conversations/${conversationId}/turns`;
  if (caseId === 1) {
    return ["/internal/assistant/conversations", base];
  }
  if (caseId === 2) {
    return [
      "/internal/assistant/conversations",
      base,
      base,
      `/internal/assistant/conversations/${conversationId}/turns/${turnId}:commit`,
    ];
  }
  return [
    "/internal/assistant/conversations",
    base,
    base,
    base,
    `/internal/assistant/conversations/${conversationId}/turns/${turnId}:commit`,
  ];
}

function buildPhaseAssertionPayload({ caseId, state, domEvidence, usedIframe }) {
  const expectedPhasePath = CASE_EXPECTED_PHASES[caseId];
  const observedPhasePath = ["idle", ...state.phaseHistory];
  const rawBindingKeys = domEvidence.bubbles.map((bubble) => bubble.binding_key || "");
  const hasPendingPlaceholderBubble = rawBindingKeys.some((key) => key.trim() === "" || key === "::::");
  const uniqueBindingKeys = new Set(rawBindingKeys);
  const stopline266 = {
    single_channel: state.nativePostPaths.length === 0,
    single_assistant_bubble:
      domEvidence.bubble_count >= 1 &&
      uniqueBindingKeys.size === domEvidence.bubble_count &&
      !hasPendingPlaceholderBubble,
    no_official_connection_error: domEvidence.connection_error_count === 0,
    no_external_reply_container: domEvidence.external_reply_container_count === 0,
  };
  const stopline280 = {
    single_formal_entry: usedIframe === false,
    no_bridge_or_injected_stream: domEvidence.external_reply_container_count === 0,
    official_message_tree_only:
      domEvidence.bubble_count >= 1 &&
      domEvidence.external_reply_container_count === 0 &&
      !hasPendingPlaceholderBubble,
    dto_only_frontend: state.nativePostPaths.length === 0,
  };
  const passed = Object.values(stopline266).every(Boolean) && Object.values(stopline280).every(Boolean);

  return {
    case_id: caseId,
    status: passed ? "passed" : "failed",
    validated_at: new Date().toISOString(),
    observed_phase_path: observedPhasePath,
    expected_phase_path: expectedPhasePath,
    stopline_266: stopline266,
    stopline_280: stopline280,
    notes: hasPendingPlaceholderBubble
      ? "captured by tp290 real case e2e; pending placeholder bubble detected"
      : "captured by tp290 real case e2e",
  };
}

async function runCaseAndCollectEvidence(browser, caseId) {
  await ensureDir(EVIDENCE_ROOT);
  const paths = evidencePaths(caseId);

  const { appContext, page } = await createTP290Session(browser, `case-${caseId}`, paths.network);
  const state = await installCaseMock(page, caseId);
  let surface;
  let usedIframe = true;

  try {
    const entry = await openFormalEntry(page);
    surface = entry.surface;
    usedIframe = entry.usedIframe;
    await expect(usedIframe, "280 single formal entry requires direct page (no iframe)").toBe(false);

    const [input1, input2, input3] = CASE_INPUTS[caseId];
    await sendFromFormalEntry(surface, input1);

    const turnId = `turn_tp290_${caseId}`;
    const requestId = `req_tp290_${caseId}`;
    const conversationId = `conv_tp290_${caseId}`;
    const bindingKey = `${conversationId}::${turnId}::${requestId}`;
    const bubble = surface.locator(`[data-assistant-binding-key="${bindingKey}"]`);

    await expect(bubble).toHaveCount(1);

    if (caseId === 1) {
      await expect(surface.getByRole("button", { name: /确认|Confirm/i })).toHaveCount(0);
      await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(0);
      await expect(bubble).toContainText("你好，我在");
    }

    if (caseId === 2) {
      await expect(bubble).toContainText("草案已生成");
      await sendFromFormalEntry(surface, input2);
      await expect(bubble).toContainText("已确认草案");
      await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(1);
      await surface.getByRole("button", { name: /提交|Submit/i }).click();
      await expect(bubble).toContainText("Case 2 提交成功");
      await expect(bubble).toContainText("org_code: AI2902");
    }

    if (caseId === 3) {
      await expect(bubble).toContainText("缺少生效日期");
      await expect(bubble).toContainText("effective_date");
      await sendFromFormalEntry(surface, input2);
      await expect(bubble).toContainText("缺字段已补全");
      await sendFromFormalEntry(surface, input3);
      await expect(bubble).toContainText("已确认草案");
      await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(1);
      await surface.getByRole("button", { name: /提交|Submit/i }).click();
      await expect(bubble).toContainText("Case 3 提交成功");
      await expect(bubble).toContainText("org_code: AI2903");
    }

    if (caseId === 4) {
      await expect(bubble).toContainText("检测到多个候选父组织");
      await sendFromFormalEntry(surface, input2);
      await expect(bubble).toContainText("已定位候选 2");
      await expect(bubble).toContainText("Selected: 共享服务中心（候选2）");
      await sendFromFormalEntry(surface, input3);
      await expect(bubble).toContainText("候选与草案已确认");
      await expect(surface.getByRole("button", { name: /提交|Submit/i })).toHaveCount(1);
      await surface.getByRole("button", { name: /提交|Submit/i }).click();
      await expect(bubble).toContainText("Case 4 提交成功");
      await expect(bubble).toContainText("parent_org_code: FLOWER-B");
    }

    await expect(bubble).toHaveCount(1);
    const finalBubbleCount = await surface.locator("[data-assistant-binding-key]").count();
    expect(finalBubbleCount).toBeLessThanOrEqual(CASE_INPUTS[caseId].length);
    expect(state.nativePostPaths).toEqual([]);
    expect(state.internalPostPaths).toEqual(expectedInternalPaths(caseId, conversationId, turnId));

    const domEvidence = await collectDOMEvidence(page, surface);
    await page.screenshot({ path: paths.page, fullPage: true });
    await writeJSON(paths.dom, domEvidence);

    const phaseAssertionPayload = buildPhaseAssertionPayload({
      caseId,
      state,
      domEvidence,
      usedIframe,
    });
    await writeJSON(paths.phase, phaseAssertionPayload);
  } finally {
    await appContext.close();
  }
}

test("tp290-e2e-001: case 1 greeting stays idle and single-channel", async ({ browser }) => {
  test.setTimeout(300_000);
  await runCaseAndCollectEvidence(browser, 1);
});

test("tp290-e2e-002: case 2 draft confirm commit closes with one official bubble", async ({ browser }) => {
  test.setTimeout(300_000);
  await runCaseAndCollectEvidence(browser, 2);
});

test("tp290-e2e-003: case 3 missing-field supplement then commit succeeds", async ({ browser }) => {
  test.setTimeout(300_000);
  await runCaseAndCollectEvidence(browser, 3);
});

test("tp290-e2e-004: case 4 candidate pick confirm and commit succeeds", async ({ browser }) => {
  test.setTimeout(300_000);
  await runCaseAndCollectEvidence(browser, 4);
});
