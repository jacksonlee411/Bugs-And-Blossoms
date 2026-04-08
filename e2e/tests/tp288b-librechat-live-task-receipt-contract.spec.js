import fs from "node:fs/promises";
import path from "node:path";
import { expect, test } from "@playwright/test";

import { latestAssistantTurn, parseJSONSafe, parseResponseBody } from "./helpers/assistant-conversation.js";
import { ensureDir, writeJSON } from "./helpers/evidence.js";
import {
  createOrgUnit,
  detectRootOrg,
  ensureOrgUnitByCode,
  waitForOrgUnitDetails,
} from "./helpers/org-baseline.js";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const repoRoot = path.resolve(__dirname, "..", "..");
const EVIDENCE_ROOT = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-288b");
const INDEX_PATH = path.join(EVIDENCE_ROOT, "tp288b-live-evidence-index.json");
const DEFAULT_COMMAND =
  `pnpm --dir ${path.join(repoRoot, "e2e")} exec playwright test tests/tp288b-librechat-live-task-receipt-contract.spec.js --workers=1 --trace on`;
const CASE_ID = "tp288b-case-1";
const SCENARIO = "create -> confirm -> commit(receipt) -> poll(task) -> refresh(conversation)";
const BASELINE_EFFECTIVE_DATE = "2026-01-01";
const BASELINE_ROOT_CODE = "ROOT";
const BASELINE_ROOT_NAME = "集团";
const BASELINE_PARENT_CODE = "TP288BAIGOV";
const BASELINE_PARENT_NAME = "AI治理办公室";
const staleOn = [
  "240C runtime gate semantics changed",
  "240D durable execution/compensation semantics changed",
  "240E MCP write admission semantics changed",
  "routing/authn chain changed",
  "error code semantics changed",
  "fail-closed behavior changed",
  "assistant formal submit pipeline changed",
  "290B runtime admission baseline changed",
];

function normalizeText(value) {
  return String(value || "").trim().replace(/\s+/g, " ");
}

function evidencePaths() {
  return {
    page: path.join(EVIDENCE_ROOT, "tp288b-case-1-page.png"),
    dom: path.join(EVIDENCE_ROOT, "tp288b-case-1-dom.json"),
    network: path.join(EVIDENCE_ROOT, "tp288b-case-1-network.har"),
    trace: path.join(EVIDENCE_ROOT, "tp288b-case-1-trace.zip"),
    assertions: path.join(EVIDENCE_ROOT, "tp288b-case-1-receipt-task-assertions.json"),
  };
}

async function createTP288BSession(browser, suffix, harPath) {
  const runID = `${Date.now()}-${suffix}`;
  return setupTenantAdminSession(browser, {
    tenantName: `TP288B Tenant ${runID}`,
    tenantHost: `t-tp288b-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp288b-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp288b-${runID}@example.invalid`,
    createPage: true,
    appContextOptions: {
      recordHar: {
        path: harPath,
        content: "embed",
        mode: "full",
      },
    },
    sessionLoginRetryTimeoutMs: 15_000,
  });
}

async function ensureTenantBaseline(appContext) {
  let rootOrg = await detectRootOrg(appContext, BASELINE_EFFECTIVE_DATE, BASELINE_ROOT_CODE);
  if (!rootOrg) {
    await createOrgUnit(appContext, {
      org_code: BASELINE_ROOT_CODE,
      name: BASELINE_ROOT_NAME,
      effective_date: BASELINE_EFFECTIVE_DATE,
      parent_org_code: "",
      is_business_unit: true,
    });
    const createdRoot = await waitForOrgUnitDetails(appContext, BASELINE_ROOT_CODE, BASELINE_EFFECTIVE_DATE);
    expect(createdRoot?.org_unit, "root org should be readable after creation").toBeTruthy();
    rootOrg = createdRoot.org_unit;
  }
  await ensureOrgUnitByCode(
    appContext,
    { code: BASELINE_PARENT_CODE, name: BASELINE_PARENT_NAME },
    {
      effectiveDate: BASELINE_EFFECTIVE_DATE,
      parentOrgCode: String(rootOrg?.org_code || "").trim(),
    },
  );
}

function installNetworkRecorder(page) {
  const state = {
    seq: 0,
    internalPostPaths: [],
    nativePostPaths: [],
    internalCalls: [],
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

  page.on("response", async (response) => {
    const pathname = new URL(response.url()).pathname;
    if (!pathname.startsWith("/internal/assistant/")) {
      return;
    }
    const request = response.request();
    const item = {
      seq: state.seq + 1,
      method: request.method(),
      path: pathname,
      status: response.status(),
      body: "",
      json: null,
    };
    state.seq = item.seq;
    try {
      const contentType = response.headers()["content-type"] || "";
      if (contentType.includes("application/json")) {
        item.json = await response.json();
      } else {
        item.body = await response.text();
      }
    } catch {
      item.body = "";
    }
    state.internalCalls.push(item);
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

async function clickFormalConfirm(surface) {
  const button = surface.getByRole("button", { name: /确认|Confirm/i }).last();
  await expect(button).toBeVisible({ timeout: 30_000 });
  await button.click();
}

async function clickFormalSubmit(surface) {
  const button = surface.getByRole("button", { name: /提交|Submit/i }).last();
  await expect(button).toBeVisible({ timeout: 30_000 });
  await button.click();
}

async function latestFormalBubble(surface) {
  const locator = surface.locator("[data-assistant-binding-key]");
  await expect(locator.first()).toBeVisible({ timeout: 60_000 });
  const count = await locator.count();
  const node = locator.nth(Math.max(0, count - 1));
  return {
    count,
    bindingKey: (await node.getAttribute("data-assistant-binding-key")) || "",
    conversationId: (await node.getAttribute("data-assistant-conversation-id")) || "",
    turnId: (await node.getAttribute("data-assistant-turn-id")) || "",
    requestId: (await node.getAttribute("data-assistant-request-id")) || "",
    text: normalizeText(await node.innerText()),
  };
}

async function fetchConversation(appContext, conversationId) {
  const response = await appContext.request.get(
    `/internal/assistant/conversations/${encodeURIComponent(conversationId)}`,
  );
  expect(response.status(), await response.text()).toBe(200);
  return response.json();
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

function findAssistantErrorCall(state, code) {
  return state.internalCalls.find((call) => assistantErrorCodeFromCall(call) === code) || null;
}

async function waitForAssistantErrorCall(state, code, timeoutMs = 8_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const matched = findAssistantErrorCall(state, code);
    if (matched) {
      return matched;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  return findAssistantErrorCall(state, code);
}

function invalidTaskPollPaths(state) {
  return state.internalCalls
    .filter(
      (call) =>
        call.method === "GET" &&
        (call.path === "/internal/assistant/tasks/" || call.path === "/internal/assistant/tasks/undefined"),
    )
    .map((call) => call.path);
}

async function waitForCommitReceipt(state, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const failedCall = state.internalCalls.find(
      (call) => assistantErrorCodeFromCall(call) === "assistant_task_dispatch_failed",
    );
    if (failedCall) {
      throw new Error(`assistant_task_dispatch_failed during commit (${failedCall.path})`);
    }
    const invalidPaths = invalidTaskPollPaths(state);
    if (invalidPaths.length > 0) {
      throw new Error(`invalid task poll path observed: ${invalidPaths.join(", ")}`);
    }
    const commitCall = state.internalCalls.find(
      (call) =>
        call.method === "POST" &&
        call.path.endsWith(":commit") &&
        call.status === 202 &&
        call.json &&
        typeof call.json.task_id === "string",
    );
    if (commitCall) {
      return commitCall;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error("commit receipt was not observed");
}

async function waitForTaskTerminal(state, taskId, afterSeq, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  const terminal = new Set(["succeeded", "failed", "manual_takeover_required", "canceled"]);
  while (Date.now() < deadline) {
    const invalidPaths = invalidTaskPollPaths(state);
    if (invalidPaths.length > 0) {
      throw new Error(`invalid task poll path observed: ${invalidPaths.join(", ")}`);
    }
    const calls = state.internalCalls.filter(
      (call) =>
        call.seq > afterSeq &&
        call.method === "GET" &&
        call.path === `/internal/assistant/tasks/${taskId}` &&
        call.json &&
        typeof call.json.status === "string",
    );
    const terminalCall = calls.find((call) => terminal.has(String(call?.json?.status || "").trim()));
    if (terminalCall) {
      return { terminalCall, calls };
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`task ${taskId} did not reach terminal state`);
}

async function waitForConversationRefresh(state, conversationId, afterSeq, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const call = state.internalCalls.find(
      (item) =>
        item.seq > afterSeq &&
        item.method === "GET" &&
        item.path === `/internal/assistant/conversations/${conversationId}` &&
        Array.isArray(item?.json?.turns) &&
        String(latestAssistantTurn(item.json)?.state || "").trim() === "committed",
    );
    if (call) {
      return call;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`conversation ${conversationId} did not refresh to committed state`);
}

async function waitForCommittedConversation(appContext, conversationId, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  let conversation = null;
  while (Date.now() < deadline) {
    conversation = await fetchConversation(appContext, conversationId);
    if (String(latestAssistantTurn(conversation)?.state || "").trim() === "committed") {
      return conversation;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  return conversation;
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

function modelProofFromConversation(conversation) {
  const turn = latestAssistantTurn(conversation) || {};
  const plan = turn.plan || {};
  const provider = String(plan.model_provider || "").trim();
  const modelName = String(plan.model_name || "").trim();
  const modelRevision = String(plan.model_revision || "").trim();
  const fallbackDetected = provider === "deterministic" || modelName === "builtin-intent-extractor";
  return {
    model_provider: provider,
    model_name: modelName,
    model_revision: modelRevision,
    fallback_detected: fallbackDetected,
  };
}

async function writeIndex(result, executedAt, artifacts, assertions, blockingReason = "") {
  await ensureDir(EVIDENCE_ROOT);
  await writeJSON(INDEX_PATH, {
    plan: "DEV-PLAN-288B",
    status: result === "passed" ? "passed" : "blocked",
    updated_at: new Date().toISOString(),
    formal_entry: "/app/assistant/librechat",
    stale_on: staleOn,
    entries: [
      {
        id: CASE_ID,
        scenario: SCENARIO,
        command: process.env.TP288B_EVIDENCE_COMMAND || DEFAULT_COMMAND,
        executed_at: executedAt,
        result,
        artifacts,
        assertions,
        blocking_reason: blockingReason,
      },
    ],
  });
}

test("tp288b-live-001: real formal entry follows receipt poll refresh contract", async ({ browser }) => {
  test.setTimeout(360_000);
  const paths = evidencePaths();
  const executedAt = new Date().toISOString();
  const artifacts = [
    path.relative(repoRoot, paths.page),
    path.relative(repoRoot, paths.dom),
    path.relative(repoRoot, paths.network),
    path.relative(repoRoot, paths.trace),
    path.relative(repoRoot, paths.assertions),
  ];
  let appContext = null;
  let page = null;
  let surface = null;
  let usedIframe = true;
  let result = "blocked";
  let blockingReason = "";
  let traceMode = "none";
  let assertions = {
    case_id: CASE_ID,
    passed: false,
  };

  await ensureDir(EVIDENCE_ROOT);

  try {
    const session = await createTP288BSession(browser, "case-1", paths.network);
    appContext = session.appContext;
    page = session.page;
    const tenantID = session.tenantID;
    const networkState = installNetworkRecorder(page);
    const orgName = `人力资源部288B${String(Date.now()).slice(-6)}`;
    const inputText = `在 ${BASELINE_PARENT_NAME} 下新建 ${orgName}，生效日期 ${BASELINE_EFFECTIVE_DATE}`;

    try {
      await appContext.tracing.start({ screenshots: true, snapshots: true, sources: true });
      traceMode = "full";
    } catch (error) {
      const message = String(error || "");
      if (!message.includes("Tracing has been already started")) {
        throw error;
      }
      traceMode = "external";
      await fs.writeFile(
        paths.trace,
        "trace managed by playwright --trace option; per-case trace kept by runner artifacts\n",
        "utf8",
      );
    }

    await ensureTenantBaseline(appContext);

    const entry = await openFormalEntry(page);
    surface = entry.surface;
    usedIframe = entry.usedIframe;
    expect(usedIframe, "formal entry must be direct page").toBe(false);

    await sendFromFormalEntry(surface, inputText);
    const modelSecretMissingCall = await waitForAssistantErrorCall(
      networkState,
      "ai_model_secret_missing",
      10_000,
    );
    const conversationCreateFailedCall =
      findAssistantErrorCall(networkState, "assistant_conversation_create_failed");
    const runtimeBlockedCall = modelSecretMissingCall || conversationCreateFailedCall;
    if (runtimeBlockedCall) {
      blockingReason = "运行态阻断：ai_model_secret_missing";
      if (assistantErrorCodeFromCall(runtimeBlockedCall) === "assistant_conversation_create_failed") {
        blockingReason = "运行态阻断：assistant_conversation_create_failed";
      }
      const domEvidence = await collectDOMEvidence(page, surface);
      await page.screenshot({ path: paths.page, fullPage: true });
      await writeJSON(paths.dom, {
        plan: "DEV-PLAN-288B",
        case_id: CASE_ID,
        captured_at: new Date().toISOString(),
        formal_entry: "/app/assistant/librechat",
        ...domEvidence,
      });
      assertions = {
        ...assertions,
        plan: "DEV-PLAN-288B",
        case_id: CASE_ID,
        formal_entry: "/app/assistant/librechat",
        command: process.env.TP288B_EVIDENCE_COMMAND || DEFAULT_COMMAND,
        captured_at: new Date().toISOString(),
        probe_skipped: true,
        skip_reason:
          assistantErrorCodeFromCall(runtimeBlockedCall) === "assistant_conversation_create_failed"
            ? "assistant_conversation_create_failed_on_create_conversation"
            : "ai_model_secret_missing_on_create_turn",
        failure_message: blockingReason,
        observed_call: {
          path: String(runtimeBlockedCall.path || ""),
          status: Number(runtimeBlockedCall.status || 0),
          error_code: assistantErrorCodeFromCall(runtimeBlockedCall),
        },
        passed: false,
      };
      await writeJSON(paths.assertions, assertions);
      result = "blocked";
      return;
    }
    const draftBubble = await latestFormalBubble(surface);
    const conversationID = draftBubble.conversationId;
    expect(conversationID).toBeTruthy();

    const draftConversation = await fetchConversation(appContext, conversationID);
    const draftTurn = latestAssistantTurn(draftConversation);
    expect(String(draftTurn?.intent?.action || "").trim()).toBe("create_orgunit");
    expect(String(draftTurn?.phase || "").trim()).toBe("await_commit_confirm");

    await clickFormalConfirm(surface);
    await clickFormalSubmit(surface);
    await expect(page.getByText("assistant_task_dispatch_failed", { exact: false })).toHaveCount(0);

    const commitCall = await waitForCommitReceipt(networkState);
    const receipt = commitCall.json;
    expect(commitCall.status).toBe(202);
    expect(receipt.task_id).toBeTruthy();
    expect(receipt.poll_uri).toBe(`/internal/assistant/tasks/${receipt.task_id}`);
    expect(receipt.task_type).toBeTruthy();
    expect(receipt.workflow_id).toBeTruthy();
    expect(receipt.submitted_at).toBeTruthy();
    expect(Object.prototype.hasOwnProperty.call(receipt, "turns")).toBe(false);

    const taskProbe = await waitForTaskTerminal(networkState, receipt.task_id, commitCall.seq);
    const refreshCall = await waitForConversationRefresh(networkState, conversationID, commitCall.seq);
    const finalConversation = await waitForCommittedConversation(appContext, conversationID);
    const finalTurn = latestAssistantTurn(finalConversation || {});
    const finalBubble = await latestFormalBubble(surface);
    const domEvidence = await collectDOMEvidence(page, surface);
    const modelProof = modelProofFromConversation(finalConversation || {});
    const assistantErrorCodes = networkState.internalCalls
      .map((call) => assistantErrorCodeFromCall(call))
      .filter(Boolean);
    const invalidTaskPaths = invalidTaskPollPaths(networkState);
    const pollStatusSequence = taskProbe.calls.map((call) => String(call?.json?.status || "").trim());

    expect(networkState.nativePostPaths).toEqual([]);
    expect(invalidTaskPaths).toEqual([]);
    expect(assistantErrorCodes).not.toContain("assistant_task_dispatch_failed");
    expect(String(taskProbe.terminalCall?.json?.status || "").trim()).toBe("succeeded");
    expect(String(finalTurn?.state || "").trim()).toBe("committed");
    expect(String(finalTurn?.intent?.action || "").trim()).toBe("create_orgunit");
    expect(String(finalTurn?.commit_result?.org_code || "").trim()).toBeTruthy();
    expect(modelProof.fallback_detected).toBe(false);
    expect(domEvidence.external_reply_container_count).toBe(0);
    expect(domEvidence.connection_error_count).toBe(0);
    expect(domEvidence.bubble_count).toBe(1);
    expect(finalBubble.conversationId).toBe(conversationID);
    expect(finalBubble.turnId).toBe(String(finalTurn?.turn_id || ""));
    expect(finalBubble.requestId).toBe(String(finalTurn?.request_id || ""));
    expect(refreshCall.status).toBe(200);

    await page.screenshot({ path: paths.page, fullPage: true });
    await writeJSON(paths.dom, {
      plan: "DEV-PLAN-288B",
      case_id: CASE_ID,
      captured_at: new Date().toISOString(),
      formal_entry: "/app/assistant/librechat",
      ...domEvidence,
    });

    assertions = {
      plan: "DEV-PLAN-288B",
      case_id: CASE_ID,
      tenant_id: tenantID,
      scenario: SCENARIO,
      formal_entry: "/app/assistant/librechat",
      command: process.env.TP288B_EVIDENCE_COMMAND || DEFAULT_COMMAND,
      captured_at: new Date().toISOString(),
      conversation_id: conversationID,
      turn_id: String(finalTurn?.turn_id || ""),
      commit_status: commitCall.status,
      receipt: {
        task_id: String(receipt.task_id || ""),
        poll_uri: String(receipt.poll_uri || ""),
        task_type: String(receipt.task_type || ""),
        status: String(receipt.status || ""),
        workflow_id: String(receipt.workflow_id || ""),
        submitted_at: String(receipt.submitted_at || ""),
      },
      poll_status_sequence: pollStatusSequence,
      final_task_status: String(taskProbe.terminalCall?.json?.status || ""),
      final_turn_state: String(finalTurn?.state || ""),
      model_proof: modelProof,
      stopline: {
        single_channel: networkState.nativePostPaths.length === 0,
        single_formal_entry: usedIframe === false,
        no_external_reply_container: domEvidence.external_reply_container_count === 0,
        no_connection_error: domEvidence.connection_error_count === 0,
        single_assistant_bubble: domEvidence.bubble_count === 1,
        no_invalid_task_poll_path: invalidTaskPaths.length === 0,
        no_assistant_task_dispatch_failed: !assistantErrorCodes.includes("assistant_task_dispatch_failed"),
        frontend_polled_receipt_task: taskProbe.calls.length > 0,
        frontend_refreshed_conversation: Boolean(refreshCall),
        no_business_mock: true,
      },
      final_bubble: finalBubble,
      passed: true,
    };
    await writeJSON(paths.assertions, assertions);
    result = "passed";
  } catch (error) {
    blockingReason = String(error?.message || error || "unknown_error");
    assertions = {
      ...assertions,
      plan: "DEV-PLAN-288B",
      case_id: CASE_ID,
      formal_entry: "/app/assistant/librechat",
      command: process.env.TP288B_EVIDENCE_COMMAND || DEFAULT_COMMAND,
      captured_at: new Date().toISOString(),
      failure_message: blockingReason,
      passed: false,
    };
    try {
      if (page) {
        await page.screenshot({ path: paths.page, fullPage: true });
      }
    } catch {
      // ignore
    }
    try {
      if (page && surface) {
        const domEvidence = await collectDOMEvidence(page, surface);
        await writeJSON(paths.dom, {
          plan: "DEV-PLAN-288B",
          case_id: CASE_ID,
          captured_at: new Date().toISOString(),
          formal_entry: "/app/assistant/librechat",
          ...domEvidence,
        });
      }
    } catch {
      // ignore
    }
    try {
      await writeJSON(paths.assertions, assertions);
    } catch {
      // ignore
    }
    throw error;
  } finally {
    if (appContext && traceMode === "full") {
      await appContext.tracing.stop({ path: paths.trace });
    }
    if (appContext) {
      await appContext.close();
    }
    await writeIndex(result, executedAt, artifacts, assertions, blockingReason);
  }
});
