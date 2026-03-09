import fs from "node:fs/promises";
import path from "node:path";
import { expect, test } from "@playwright/test";

const repoRoot = path.resolve(__dirname, "..", "..");
const tp288EvidenceRoot = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-266");

const tp288DefaultCommand =
  "pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1";

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

async function ensureEvidenceRoot() {
  await fs.mkdir(tp288EvidenceRoot, { recursive: true });
}

async function writeJSON(filePath, payload) {
  await fs.writeFile(filePath, `${JSON.stringify(payload, null, 2)}\n`, "utf8");
}

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
  await ensureEvidenceRoot();

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
    },
    page_text_counts: pageTextCounts,
    expected_bubble_count: expectedBubbleCount,
    actual_bubble_count: bubbles.length,
    binding_assertion: bindingAssertion,
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

async function setupTenantAdminSession(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  const tenantHost = `t-tp288-${runID}.localhost`;
  const tenantName = `TP288 Tenant ${runID}`;
  const tenantAdminEmail = `tenant-admin+tp288-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp288-${runID}@example.invalid`;
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
  });

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass },
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
  const { appContext, page } = await setupTenantAdminSession(browser, "001");
  await ensureEvidenceRoot();
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

  expect(network.nativePostPaths).toEqual([]);
  expect(network.internalPostPaths).toEqual([
    "/internal/assistant/conversations",
    "/internal/assistant/conversations/conv_tp288_1/turns",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:confirm",
    "/internal/assistant/conversations/conv_tp288_1/turns/turn_tp288_1:commit",
  ]);

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
  const { appContext, page } = await setupTenantAdminSession(browser, "002");
  await ensureEvidenceRoot();
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
