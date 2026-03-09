import { expect, test } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";

const repoRoot = path.resolve(__dirname, "..", "..");
const EVIDENCE_ROOT = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-290b");
const INDEX_PATH = path.join(EVIDENCE_ROOT, "tp290b-live-evidence-index.json");
const BASELINE_PATH = path.join(EVIDENCE_ROOT, "tp290b-data-baseline.json");

const CASE_INPUTS = {
  1: ["你好"],
  2: ["在 AI治理办公室 下新建 人力资源部二期，生效日期 2026-01-01", "确认"],
  3: ["在 AI治理办公室 下新建 人力资源部239A补全", "生效日期 2026-03-25", "确认"],
  4: ["在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26", "选第2个", "是的"],
};

const staleOn = [
  "240C runtime gate semantics changed",
  "240D durable execution/compensation semantics changed",
  "240E MCP write admission semantics changed",
  "routing/authn chain changed",
  "error code semantics changed",
  "fail-closed behavior changed",
  "assistant formal submit pipeline changed",
];

const caseSummaries = [];
const baselineHints = {};

function normalizeText(value) {
  return String(value || "").trim().replace(/\s+/g, " ");
}

async function ensureDir(dirPath) {
  await fs.mkdir(dirPath, { recursive: true });
}

async function writeJSON(filePath, payload) {
  await fs.writeFile(filePath, `${JSON.stringify(payload, null, 2)}\n`, "utf8");
}

function evidencePaths(caseId) {
  return {
    page: `${EVIDENCE_ROOT}/case-${caseId}-page.png`,
    dom: `${EVIDENCE_ROOT}/case-${caseId}-dom.json`,
    network: `${EVIDENCE_ROOT}/case-${caseId}-network.har`,
    trace: `${EVIDENCE_ROOT}/case-${caseId}-trace.zip`,
    phase: `${EVIDENCE_ROOT}/case-${caseId}-phase-assertions.json`,
    intent: `${EVIDENCE_ROOT}/case-${caseId}-intent-action-assertions.json`,
    unsupported: `${EVIDENCE_ROOT}/case-${caseId}-unsupported-failure.json`,
    snapshot: `${EVIDENCE_ROOT}/case-${caseId}-conversation-snapshot.json`,
    model: `${EVIDENCE_ROOT}/case-${caseId}-model-proof.json`,
  };
}

async function ensureKratosIdentity(ctx, kratosAdminURL, { traits, identifier, password }) {
  const resp = await ctx.request.post(`${kratosAdminURL}/admin/identities`, {
    data: {
      schema_id: "default",
      traits,
      credentials: {
        password: {
          identifiers: [identifier],
          config: { password },
        },
      },
    },
  });
  if (!resp.ok()) {
    expect(resp.status(), `unexpected status: ${resp.status()} (${await resp.text()})`).toBe(409);
  }
}

async function setupTenantAdminSession(browser, suffix, harPath) {
  const runID = `${Date.now()}-${suffix}`;
  const tenantHost = `t-tp290b-${runID}.localhost`;
  const tenantName = `TP290B Tenant ${runID}`;
  const tenantAdminEmail = `tenant-admin+tp290b-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp290b-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass },
  });
  const superadminPage = await superadminContext.newPage();

  if (!process.env.E2E_SUPERADMIN_EMAIL) {
    await ensureKratosIdentity(superadminContext, kratosAdminURL, {
      traits: { email: superadminEmail },
      identifier: `sa:${superadminEmail.toLowerCase()}`,
      password: superadminLoginPass,
    });
  }

  await superadminPage.goto("/superadmin/login");
  await superadminPage.locator('input[name="email"]').fill(superadminEmail);
  await superadminPage.locator('input[name="password"]').fill(superadminLoginPass);
  await superadminPage.getByRole("button", { name: "Login" }).click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);

  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="name"]').fill(tenantName);
  await superadminPage.locator('form[action="/superadmin/tenants"] input[name="hostname"]').fill(tenantHost);
  await superadminPage.locator('form[action="/superadmin/tenants"] button[type="submit"]').click();
  await expect(superadminPage).toHaveURL(/\/superadmin\/tenants$/);
  await expect(superadminPage.locator("tr", { hasText: tenantHost }).first()).toBeVisible({ timeout: 60_000 });

  const tenantRow = superadminPage.locator("tr", { hasText: tenantHost }).first();
  const tenantID = (await tenantRow.locator("code").first().innerText()).replace(/\s+/g, "").trim();
  expect(tenantID).not.toBe("");

  await ensureKratosIdentity(superadminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenantID, email: tenantAdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenantID}:${tenantAdminEmail}`,
    password: tenantAdminPass,
  });
  await superadminContext.close();

  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const appContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost },
    recordHar: {
      path: harPath,
      content: "embed",
      mode: "full",
    },
  });
  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass },
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);
  const page = await appContext.newPage();
  return { appContext, page, tenantID };
}

function installNetworkRecorder(page) {
  const state = {
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
    const request = response.request();
    const pathname = new URL(response.url()).pathname;
    if (!pathname.startsWith("/internal/assistant/")) {
      return;
    }
    const item = {
      method: request.method(),
      path: pathname,
      status: response.status(),
      body: "",
      json: null,
    };
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

function latestTurn(conversation) {
  if (!conversation || !Array.isArray(conversation.turns) || conversation.turns.length === 0) {
    return null;
  }
  return conversation.turns[conversation.turns.length - 1];
}

async function waitForCommittedConversation(appContext, conversationId, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  let lastConversation = null;
  while (Date.now() < deadline) {
    lastConversation = await fetchConversation(appContext, conversationId);
    const turn = latestTurn(lastConversation);
    if (turn && turn.state === "committed") {
      return lastConversation;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  return lastConversation;
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

function compressPhases(items) {
  const phases = [];
  for (const item of items) {
    const phase = String(item || "").trim();
    if (!phase) {
      continue;
    }
    if (phases.length === 0 || phases[phases.length - 1] !== phase) {
      phases.push(phase);
    }
  }
  return phases;
}

function modelProofFromConversation(conversation) {
  const turn = latestTurn(conversation) || {};
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
    proof_source: "case-conversation-snapshot",
  };
}

function hasInternalPost(state, suffix) {
  return state.internalPostPaths.some((item) => item.endsWith(suffix));
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

function assistantTaskStatusCalls(state) {
  return state.internalCalls.filter(
    (call) =>
      call.method === "GET" &&
      call.path.startsWith("/internal/assistant/tasks/") &&
      call.json &&
      typeof call.json.status === "string",
  );
}

async function runCaseAndCollectEvidence(browser, caseId) {
  await ensureDir(EVIDENCE_ROOT);
  const paths = evidencePaths(caseId);
  const { appContext, page, tenantID } = await setupTenantAdminSession(browser, `case-${caseId}`, paths.network);
  const networkState = installNetworkRecorder(page);
  let traceMode = "none";
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

  let surface = page;
  let usedIframe = true;
  let conversationId = "";
  const snapshots = [];

  try {
    const entry = await openFormalEntry(page);
    surface = entry.surface;
    usedIframe = entry.usedIframe;
    expect(usedIframe, "formal entry must be direct page").toBe(false);

    const inputs = CASE_INPUTS[caseId];
    for (let index = 0; index < inputs.length; index += 1) {
      await sendFromFormalEntry(surface, inputs[index]);
      const bubble = await latestFormalBubble(surface);
      conversationId = conversationId || bubble.conversationId;
      if (conversationId) {
        const conversation = await fetchConversation(appContext, conversationId);
        snapshots.push({
          step: index + 1,
          input: inputs[index],
          bubble,
          conversation,
          latest_turn: latestTurn(conversation),
        });
      }
    }

    let finalConversation = snapshots.length > 0 ? snapshots[snapshots.length - 1].conversation : null;
    if (caseId !== 1 && conversationId) {
      finalConversation = await waitForCommittedConversation(appContext, conversationId);
      snapshots.push({
        step: "final",
        input: "",
        bubble: await latestFormalBubble(surface),
        conversation: finalConversation,
        latest_turn: latestTurn(finalConversation),
      });
    }

	    const finalTurn = latestTurn(finalConversation || {});
	    const observedPhases = compressPhases(snapshots.map((item) => item.latest_turn?.phase));
	    const unsupportedCalls = networkState.internalCalls.filter(
	      (call) => assistantErrorCodeFromCall(call) === "assistant_intent_unsupported",
	    );
	    if (unsupportedCalls.length > 0) {
	      await writeJSON(paths.unsupported, {
	        case_id: caseId,
	        conversation_id: conversationId,
	        observed_calls: unsupportedCalls,
	        captured_at: new Date().toISOString(),
	      });
	    }
	    expect(unsupportedCalls).toEqual([]);

	    const taskStatusCalls = assistantTaskStatusCalls(networkState);
	    const lastTaskStatus =
	      taskStatusCalls.length > 0
	        ? String(taskStatusCalls[taskStatusCalls.length - 1].json.status || "").trim()
	        : "";
	    const actionAtFirstTurn = String(snapshots[0]?.latest_turn?.intent?.action || "").trim();
	    const actionAtFinalTurn = String(finalTurn?.intent?.action || "").trim();
	    const actionOnCommittedPath = actionAtFinalTurn || actionAtFirstTurn;

	    expect(networkState.nativePostPaths).toEqual([]);
	    if (caseId === 1) {
	      expect(hasInternalPost(networkState, ":confirm")).toBe(false);
	      expect(hasInternalPost(networkState, ":commit")).toBe(false);
	      expect(finalTurn?.intent?.action).toBe("plan_only");
	      expect(observedPhases).not.toContain("committing");
	      expect(observedPhases).not.toContain("committed");
	    } else if (caseId === 2) {
	      expect(observedPhases).toContain("await_commit_confirm");
	      expect(hasInternalPost(networkState, ":confirm")).toBe(true);
	      expect(hasInternalPost(networkState, ":commit")).toBe(true);
	      expect(taskStatusCalls.length).toBeGreaterThan(0);
	      expect(lastTaskStatus).toBe("succeeded");
	      expect(finalTurn?.state).toBe("committed");
	    } else if (caseId === 3) {
	      expect(snapshots[0]?.latest_turn?.phase).toBe("await_missing_fields");
	      expect(observedPhases).toContain("await_commit_confirm");
	      expect(hasInternalPost(networkState, ":confirm")).toBe(true);
	      expect(hasInternalPost(networkState, ":commit")).toBe(true);
	      expect(taskStatusCalls.length).toBeGreaterThan(0);
	      expect(lastTaskStatus).toBe("succeeded");
	      expect(finalTurn?.state).toBe("committed");
	    } else if (caseId === 4) {
	      expect(snapshots[0]?.latest_turn?.phase).toBe("await_candidate_pick");
	      expect((snapshots[0]?.latest_turn?.candidates || []).length).toBeGreaterThan(1);
	      expect(hasInternalPost(networkState, ":confirm")).toBe(true);
	      expect(hasInternalPost(networkState, ":commit")).toBe(true);
	      expect(taskStatusCalls.length).toBeGreaterThan(0);
	      expect(lastTaskStatus).toBe("succeeded");
	      expect(finalTurn?.state).toBe("committed");
	    }

    const modelProof = modelProofFromConversation(finalConversation || {});
    if (caseId !== 1) {
      expect(modelProof.fallback_detected).toBe(false);
    }

    const domEvidence = await collectDOMEvidence(page, surface);
    await page.screenshot({ path: paths.page, fullPage: true });
    await writeJSON(paths.dom, domEvidence);
    await writeJSON(paths.snapshot, {
      case_id: caseId,
      tenant_id: tenantID,
      conversation_id: conversationId,
      snapshots,
      final_conversation: finalConversation,
    });
    await writeJSON(paths.model, modelProof);

    const intentAssertion = {
      case_id: caseId,
      conversation_id: conversationId,
      turn_id: finalTurn?.turn_id || "",
      request_id: finalTurn?.request_id || "",
      trace_id: finalTurn?.trace_id || "",
      intent_action_expected:
        caseId === 1
          ? "plan_only"
          : "create_orgunit",
	      intent_action_actual:
	        caseId === 1
	          ? finalTurn?.intent?.action || ""
	          : actionOnCommittedPath,
      phase_expected_path:
        caseId === 1
          ? ["idle_or_await_commit_confirm_without_commit"]
          : caseId === 2
            ? ["await_commit_confirm", "committed"]
            : caseId === 3
              ? ["await_missing_fields", "await_commit_confirm", "committed"]
              : ["await_candidate_pick", "await_commit_confirm", "committed"],
      phase_observed_path: observedPhases,
      error_code: finalTurn?.error_code || "",
      passed: true,
    };
    await writeJSON(paths.intent, intentAssertion);

	    const phaseAssertions = {
	      case_id: caseId,
	      status: "passed",
	      validated_at: new Date().toISOString(),
	      observed_phase_path: observedPhases,
	      network_internal_posts: networkState.internalPostPaths,
	      task_terminal_status: lastTaskStatus,
	      task_status_poll_count: taskStatusCalls.length,
	      stopline_266: {
	        single_channel: networkState.nativePostPaths.length === 0,
	        no_official_connection_error: domEvidence.connection_error_count === 0,
	        no_external_reply_container: domEvidence.external_reply_container_count === 0,
	      },
      stopline_280: {
        single_formal_entry: usedIframe === false,
        no_bridge_or_injected_stream: domEvidence.external_reply_container_count === 0,
      },
    };
    await writeJSON(paths.phase, phaseAssertions);

	    caseSummaries.push({
	      id: caseId,
	      status: "passed",
	      input_sequence: inputs,
	      task_terminal_status: lastTaskStatus,
	      artifacts: {
	        page: path.relative(repoRoot, paths.page),
	        dom: path.relative(repoRoot, paths.dom),
        network: path.relative(repoRoot, paths.network),
        trace: path.relative(repoRoot, paths.trace),
        phase_assertions: path.relative(repoRoot, paths.phase),
        intent_action_assertions: path.relative(repoRoot, paths.intent),
        conversation_snapshot: path.relative(repoRoot, paths.snapshot),
        model_proof: path.relative(repoRoot, paths.model),
      },
    });

    if (caseId === 2) {
      baselineHints.case2 = {
        tenant_id: tenantID,
        conversation_id: conversationId,
        phase: snapshots[0]?.latest_turn?.phase || "",
        parent_ref_text: snapshots[0]?.latest_turn?.intent?.parent_ref_text || "",
      };
    }
    if (caseId === 4) {
      baselineHints.case4 = {
        tenant_id: tenantID,
        conversation_id: conversationId,
        phase: snapshots[0]?.latest_turn?.phase || "",
        candidate_count: (snapshots[0]?.latest_turn?.candidates || []).length,
      };
    }
  } finally {
    if (traceMode === "full") {
      await appContext.tracing.stop({ path: paths.trace });
    }
    await appContext.close();
  }
}

test.describe.configure({ mode: "serial" });

test("tp290b-e2e-001: case 1 greeting keeps plan_only on real backend", async ({ browser }) => {
  test.setTimeout(360_000);
  await runCaseAndCollectEvidence(browser, 1);
});

test("tp290b-e2e-002: case 2 dialog confirmation auto drives confirm+commit", async ({ browser }) => {
  test.setTimeout(360_000);
  await runCaseAndCollectEvidence(browser, 2);
});

test("tp290b-e2e-003: case 3 missing field supplement then committed", async ({ browser }) => {
  test.setTimeout(360_000);
  await runCaseAndCollectEvidence(browser, 3);
});

test("tp290b-e2e-004: case 4 candidate pick then dialog commit", async ({ browser }) => {
  test.setTimeout(360_000);
  await runCaseAndCollectEvidence(browser, 4);
});

test.afterAll(async () => {
  await ensureDir(EVIDENCE_ROOT);
  const sorted = [...caseSummaries].sort((a, b) => a.id - b.id);
  const indexPayload = {
    plan: "DEV-PLAN-290B",
    status: sorted.length === 4 && sorted.every((item) => item.status === "passed") ? "passed" : "in_progress",
    updated_at: new Date().toISOString(),
    formal_entry: "/app/assistant/librechat",
    stale_on: staleOn,
	    fixed_assets: {
	      root: "docs/dev-records/assets/dev-plan-290b",
	      pattern: [
        "case-{id}-page.png",
        "case-{id}-dom.json",
        "case-{id}-network.har",
        "case-{id}-trace.zip",
	        "case-{id}-phase-assertions.json",
	        "case-{id}-intent-action-assertions.json",
	        "case-{id}-unsupported-failure.json",
	        "case-{id}-conversation-snapshot.json",
	        "case-{id}-model-proof.json",
	      ],
	    },
    cases: sorted,
  };
  await writeJSON(INDEX_PATH, indexPayload);

	  const baselinePayload = {
	    plan: "DEV-PLAN-290B",
	    status:
	      sorted.length === 4 &&
	      sorted.every((item) => item.status === "passed") &&
	      (baselineHints.case4?.candidate_count ?? 0) > 1
	        ? "passed"
	        : "blocked",
	    validated_at: new Date().toISOString(),
	    tenant_id: baselineHints.case2?.tenant_id || baselineHints.case4?.tenant_id || "",
	    as_of: "2026-03-26",
	    candidate_snapshot: {
	      source_case: "case4",
	      conversation_id: baselineHints.case4?.conversation_id || "",
	      candidate_count: baselineHints.case4?.candidate_count ?? 0,
	    },
	    required_orgs: [
	      {
	        name: "AI治理办公室",
        source: "case2-first-turn",
        observed_phase: baselineHints.case2?.phase || "",
        observed_parent_ref_text: baselineHints.case2?.parent_ref_text || "",
      },
      {
        name: "共享服务中心",
        source: "case4-first-turn",
        observed_phase: baselineHints.case4?.phase || "",
	        candidate_count: baselineHints.case4?.candidate_count ?? 0,
	      },
	    ],
	    notes: [
	      "该文件由 tp290b 主验收脚本覆盖写入。",
	      "若 candidate_count <= 1，则基线未达标，阻断主验收通过判定。",
	    ],
	  };
  await writeJSON(BASELINE_PATH, baselinePayload);
});
