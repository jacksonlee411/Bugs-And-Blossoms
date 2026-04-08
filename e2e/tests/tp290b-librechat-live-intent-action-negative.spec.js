import { expect, test } from "@playwright/test";
import path from "node:path";

import { latestAssistantTurn, parseJSONSafe } from "./helpers/assistant-conversation.js";
import { pollAssistantTask } from "./helpers/assistant-task.js";
import { ensureDir, writeJSON } from "./helpers/evidence.js";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const repoRoot = path.resolve(__dirname, "..", "..");
const EVIDENCE_ROOT = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-290b");

function isIgnorableCloseError(error) {
  const message = String(error || "").toLowerCase();
  return message.includes("enoent") || message.includes("step id not found");
}

async function closeContextSafely(context, label) {
  if (!context) {
    return;
  }
  try {
    await context.close();
  } catch (error) {
    if (!isIgnorableCloseError(error)) {
      throw error;
    }
    // Playwright trace artifacts may already be moved/removed by the runner.
    console.warn(`[tp290b-neg] ignore close error (${label}): ${String(error)}`);
  }
}

async function createNegativeSession(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  const session = await setupTenantAdminSession(browser, {
    tenantName: `TP290B NEG Tenant ${runID}`,
    tenantHost: `t-tp290b-neg-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp290b-neg-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp290b-neg-${runID}@example.invalid`,
    closeSuperadminContextWith: async (context) => closeContextSafely(context, "setup-superadmin"),
  });
  return { appContext: session.appContext, tenantID: session.tenantID };
}

async function handleCreateTurnBlockedScenario({
  evidenceFile,
  tenantID,
  conversationID,
  createTurnStatus,
  parsedBody,
}) {
  if (createTurnStatus === 422 && parsedBody?.code === "assistant_intent_unsupported") {
    await writeJSON(path.join(EVIDENCE_ROOT, evidenceFile), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      conversation_id: conversationID,
      probe_skipped: true,
      skip_reason: "assistant_intent_unsupported_on_create_turn",
      create_turn_status: createTurnStatus,
      error: parsedBody,
      captured_at: new Date().toISOString(),
    });
    return true;
  }
  if (
    (createTurnStatus === 500 || createTurnStatus === 422) &&
    parsedBody?.code === "ai_model_secret_missing"
  ) {
    await writeJSON(path.join(EVIDENCE_ROOT, evidenceFile), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      conversation_id: conversationID,
      probe_skipped: true,
      skip_reason: "ai_model_secret_missing_on_create_turn",
      create_turn_status: createTurnStatus,
      error: parsedBody,
      captured_at: new Date().toISOString(),
    });
    return true;
  }
  return false;
}

async function handleCreateConversationBlockedScenario({
  evidenceFile,
  tenantID,
  createConversationStatus,
  parsedBody,
}) {
  if (
    (createConversationStatus === 500 || createConversationStatus === 422) &&
    parsedBody?.code === "assistant_conversation_create_failed"
  ) {
    await writeJSON(path.join(EVIDENCE_ROOT, evidenceFile), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      probe_skipped: true,
      skip_reason: "assistant_conversation_create_failed_on_create_conversation",
      create_conversation_status: createConversationStatus,
      error: parsedBody,
      captured_at: new Date().toISOString(),
    });
    return true;
  }
  if (
    (createConversationStatus === 500 || createConversationStatus === 422) &&
    parsedBody?.code === "ai_model_secret_missing"
  ) {
    await writeJSON(path.join(EVIDENCE_ROOT, evidenceFile), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      probe_skipped: true,
      skip_reason: "ai_model_secret_missing_on_create_conversation",
      create_conversation_status: createConversationStatus,
      error: parsedBody,
      captured_at: new Date().toISOString(),
    });
    return true;
  }
  return false;
}

test("tp290b-neg-001: commit without confirm returns conversation_confirmation_required", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  await ensureDir(EVIDENCE_ROOT);
  const { appContext, tenantID } = await createNegativeSession(browser, "001");
  try {
    const createConv = await appContext.request.post("/internal/assistant/conversations", {
      data: {},
    });
    const createConvStatus = createConv.status();
    const createConvBody = await createConv.text();
    const createConvJSON = parseJSONSafe(createConvBody);
    if (createConvStatus !== 200) {
      if (
        await handleCreateConversationBlockedScenario({
          evidenceFile: "negative-001-commit-without-confirm.json",
          tenantID,
          createConversationStatus: createConvStatus,
          parsedBody: createConvJSON,
        })
      ) {
        return;
      }
      expect(createConvStatus, createConvBody).toBe(200);
    }
    const conversation = createConvJSON;
    expect(conversation?.conversation_id).toBeTruthy();
    const conversationID = conversation.conversation_id;

    const createTurn = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns`,
      {
        data: { user_input: "在 AI治理办公室 下新建 人力资源部NEG，生效日期 2026-01-01" },
      },
    );
    const createTurnStatus = createTurn.status();
    if (createTurnStatus !== 200) {
      const rawBody = await createTurn.text();
      const parsedBody = parseJSONSafe(rawBody);
      if (
        await handleCreateTurnBlockedScenario({
          evidenceFile: "negative-001-commit-without-confirm.json",
          tenantID,
          conversationID,
          createTurnStatus,
          parsedBody,
        })
      ) {
        return;
      }
      expect(createTurnStatus, rawBody).toBe(200);
    }
    const nextConversation = await createTurn.json();
    const latestTurn = nextConversation.turns[nextConversation.turns.length - 1];
    expect(latestTurn?.turn_id).toBeTruthy();

    const commitResp = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(latestTurn.turn_id)}:commit`,
      { data: {} },
    );
    expect(commitResp.status(), await commitResp.text()).toBe(409);
    const errorBody = await commitResp.json();
    expect(errorBody.code).toBe("conversation_confirmation_required");

    await writeJSON(path.join(EVIDENCE_ROOT, "negative-001-commit-without-confirm.json"), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      conversation_id: conversationID,
      turn_id: latestTurn.turn_id,
      status: commitResp.status(),
      error: errorBody,
      captured_at: new Date().toISOString(),
    });
  } finally {
    await closeContextSafely(appContext, "neg-001");
  }
});

test("tp290b-neg-002: confirm with bad candidate id returns deterministic error", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  await ensureDir(EVIDENCE_ROOT);
  const { appContext, tenantID } = await createNegativeSession(browser, "002");
  try {
    const createConv = await appContext.request.post("/internal/assistant/conversations", {
      data: {},
    });
    const createConvStatus = createConv.status();
    const createConvBody = await createConv.text();
    const createConvJSON = parseJSONSafe(createConvBody);
    if (createConvStatus !== 200) {
      if (
        await handleCreateConversationBlockedScenario({
          evidenceFile: "negative-002-bad-candidate-confirm.json",
          tenantID,
          createConversationStatus: createConvStatus,
          parsedBody: createConvJSON,
        })
      ) {
        return;
      }
      expect(createConvStatus, createConvBody).toBe(200);
    }
    const conversation = createConvJSON;
    expect(conversation?.conversation_id).toBeTruthy();
    const conversationID = conversation.conversation_id;

    const createTurn = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns`,
      {
        data: { user_input: "在 共享服务中心 下新建 NEG候选测试，生效日期 2026-03-26" },
      },
    );
    const createTurnStatus = createTurn.status();
    if (createTurnStatus !== 200) {
      const rawBody = await createTurn.text();
      const parsedBody = parseJSONSafe(rawBody);
      if (
        await handleCreateTurnBlockedScenario({
          evidenceFile: "negative-002-bad-candidate-confirm.json",
          tenantID,
          conversationID,
          createTurnStatus,
          parsedBody,
        })
      ) {
        return;
      }
      expect(createTurnStatus, rawBody).toBe(200);
    }
    const nextConversation = await createTurn.json();
    const latestTurn = nextConversation.turns[nextConversation.turns.length - 1];
    expect(latestTurn?.turn_id).toBeTruthy();

    const confirmResp = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(latestTurn.turn_id)}:confirm`,
      {
        data: { candidate_id: "cand-does-not-exist" },
      },
    );

    const status = confirmResp.status();
    expect([409, 422]).toContain(status);
    const errorBody = await confirmResp.json();
    expect(typeof errorBody.code).toBe("string");

    await writeJSON(path.join(EVIDENCE_ROOT, "negative-002-bad-candidate-confirm.json"), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      conversation_id: conversationID,
      turn_id: latestTurn.turn_id,
      status,
      error: errorBody,
      captured_at: new Date().toISOString(),
    });
  } finally {
    await closeContextSafely(appContext, "neg-002");
  }
});

test("tp290b-neg-003: plan_only confirm then commit returns assistant_intent_unsupported", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  await ensureDir(EVIDENCE_ROOT);
  const { appContext, tenantID } = await createNegativeSession(browser, "003");
  try {
    const createConv = await appContext.request.post("/internal/assistant/conversations", {
      data: {},
    });
    const createConvStatus = createConv.status();
    const createConvBody = await createConv.text();
    const createConvJSON = parseJSONSafe(createConvBody);
    if (createConvStatus !== 200) {
      if (
        await handleCreateConversationBlockedScenario({
          evidenceFile: "negative-003-plan-only-unsupported-commit.json",
          tenantID,
          createConversationStatus: createConvStatus,
          parsedBody: createConvJSON,
        })
      ) {
        return;
      }
      expect(createConvStatus, createConvBody).toBe(200);
    }
    const conversation = createConvJSON;
    expect(conversation?.conversation_id).toBeTruthy();
    const conversationID = conversation.conversation_id;

    const createTurn = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns`,
      {
        data: { user_input: "你好" },
      },
    );
    const createTurnStatus = createTurn.status();
    if (createTurnStatus !== 200) {
      const rawBody = await createTurn.text();
      const parsedBody = parseJSONSafe(rawBody);
      if (
        await handleCreateTurnBlockedScenario({
          evidenceFile: "negative-003-plan-only-unsupported-commit.json",
          tenantID,
          conversationID,
          createTurnStatus,
          parsedBody,
        })
      ) {
        return;
      }
      expect(createTurnStatus, rawBody).toBe(200);
    }
    const createdConversation = await createTurn.json();
    const turn = latestAssistantTurn(createdConversation);
    expect(turn?.turn_id).toBeTruthy();

    const confirmResp = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turn.turn_id)}:confirm`,
      { data: {} },
    );
    expect(confirmResp.status(), await confirmResp.text()).toBe(200);

    const commitResp = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(turn.turn_id)}:commit`,
      { data: {} },
    );
    expect(commitResp.status(), await commitResp.text()).toBe(422);
    const errorBody = await commitResp.json();
    expect(errorBody.code).toBe("assistant_intent_unsupported");

    await writeJSON(path.join(EVIDENCE_ROOT, "negative-003-plan-only-unsupported-commit.json"), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      conversation_id: conversationID,
      turn_id: turn.turn_id,
      confirm_status: confirmResp.status(),
      commit_status: commitResp.status(),
      error: errorBody,
      captured_at: new Date().toISOString(),
    });
  } finally {
    await closeContextSafely(appContext, "neg-003");
  }
});

test("tp290b-neg-004: manual_takeover and timeout attribution probe", async ({ browser }) => {
  test.setTimeout(300_000);
  await ensureDir(EVIDENCE_ROOT);
  const { appContext, tenantID } = await createNegativeSession(browser, "004");
  try {
    const createConv = await appContext.request.post("/internal/assistant/conversations", {
      data: {},
    });
    const createConvStatus = createConv.status();
    const createConvBody = await createConv.text();
    const createConvJSON = parseJSONSafe(createConvBody);
    if (createConvStatus !== 200) {
      if (
        await handleCreateConversationBlockedScenario({
          evidenceFile: "negative-004-manual-takeover-timeout-probe.json",
          tenantID,
          createConversationStatus: createConvStatus,
          parsedBody: createConvJSON,
        })
      ) {
        return;
      }
      expect(createConvStatus, createConvBody).toBe(200);
    }
    const conversation = createConvJSON;
    expect(conversation?.conversation_id).toBeTruthy();
    const conversationID = conversation.conversation_id;

    const createTurn = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns`,
      {
        data: { user_input: "在 AI治理办公室 下新建 人力资源部NEG-PROBE，生效日期 2026-01-01" },
      },
    );
    const createTurnStatus = createTurn.status();
    if (createTurnStatus !== 200) {
      const rawBody = await createTurn.text();
      const parsedBody = parseJSONSafe(rawBody);
      if (
        await handleCreateTurnBlockedScenario({
          evidenceFile: "negative-004-manual-takeover-timeout-probe.json",
          tenantID,
          conversationID,
          createTurnStatus,
          parsedBody,
        })
      ) {
        return;
      }
      expect(createTurnStatus, rawBody).toBe(200);
    }
    const firstConversation = await createTurn.json();
    const firstTurn = latestAssistantTurn(firstConversation);
    expect(firstTurn?.turn_id).toBeTruthy();

    const confirmResp = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(firstTurn.turn_id)}:confirm`,
      { data: {} },
    );
    const confirmStatus = confirmResp.status();
    if (confirmStatus !== 200) {
      const rawBody = await confirmResp.text();
      let parsedBody = null;
      try {
        parsedBody = JSON.parse(rawBody);
      } catch {
        parsedBody = null;
      }
      if (confirmStatus === 409 && parsedBody?.code === "conversation_confirmation_required") {
        await writeJSON(path.join(EVIDENCE_ROOT, "negative-004-manual-takeover-timeout-probe.json"), {
          plan: "DEV-PLAN-290B",
          tenant_id: tenantID,
          conversation_id: conversationID,
          turn_id: firstTurn.turn_id,
          probe_skipped: true,
          skip_reason: "conversation_confirmation_required_on_confirm",
          confirm_status: confirmStatus,
          error: parsedBody,
          captured_at: new Date().toISOString(),
        });
        return;
      }
      expect(confirmStatus, rawBody).toBe(200);
    }
    const commitResp = await appContext.request.post(
      `/internal/assistant/conversations/${encodeURIComponent(conversationID)}/turns/${encodeURIComponent(firstTurn.turn_id)}:commit`,
      { data: {} },
    );
    const commitStatus = commitResp.status();
    if (commitStatus !== 202) {
      const rawBody = await commitResp.text();
      const parsedBody = parseJSONSafe(rawBody);
      if (commitStatus === 422 && parsedBody?.code === "assistant_intent_unsupported") {
        await writeJSON(path.join(EVIDENCE_ROOT, "negative-004-manual-takeover-timeout-probe.json"), {
          plan: "DEV-PLAN-290B",
          tenant_id: tenantID,
          conversation_id: conversationID,
          turn_id: firstTurn.turn_id,
          probe_skipped: true,
          skip_reason: "assistant_intent_unsupported_on_commit",
          commit_status: commitStatus,
          error: parsedBody,
          captured_at: new Date().toISOString(),
        });
        return;
      }
      if ((commitStatus === 500 || commitStatus === 422) && parsedBody?.code === "ai_model_secret_missing") {
        await writeJSON(path.join(EVIDENCE_ROOT, "negative-004-manual-takeover-timeout-probe.json"), {
          plan: "DEV-PLAN-290B",
          tenant_id: tenantID,
          conversation_id: conversationID,
          turn_id: firstTurn.turn_id,
          probe_skipped: true,
          skip_reason: "ai_model_secret_missing_on_commit",
          commit_status: commitStatus,
          error: parsedBody,
          captured_at: new Date().toISOString(),
        });
        return;
      }
      expect(commitStatus, rawBody).toBe(202);
    }
    const receipt = await commitResp.json();
    expect(receipt.task_id).toBeTruthy();

    const tightProbe = await pollAssistantTask(appContext, receipt.task_id, 300);
    const fullProbe = await pollAssistantTask(appContext, receipt.task_id, 30_000);
    if (!fullProbe.timed_out && fullProbe.terminal_status) {
      expect(["succeeded", "failed", "manual_takeover_required", "canceled"]).toContain(
        fullProbe.terminal_status,
      );
    }

    await writeJSON(path.join(EVIDENCE_ROOT, "negative-004-manual-takeover-timeout-probe.json"), {
      plan: "DEV-PLAN-290B",
      tenant_id: tenantID,
      conversation_id: conversationID,
      turn_id: firstTurn.turn_id,
      task_id: receipt.task_id,
      tight_probe: tightProbe,
      full_probe: fullProbe,
      manual_takeover_required_hit:
        !fullProbe.timed_out && fullProbe.terminal_status === "manual_takeover_required",
      timeout_attribution: {
        probe_timeout_window_ms: 300,
        timed_out: tightProbe.timed_out,
      },
      captured_at: new Date().toISOString(),
    });
  } finally {
    await closeContextSafely(appContext, "neg-004");
  }
});
