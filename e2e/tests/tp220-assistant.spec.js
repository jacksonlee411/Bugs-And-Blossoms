import { expect, test } from "@playwright/test";

async function ensureKratosIdentity(ctx, kratosAdminURL, { traits, identifier, password }) {
  const resp = await ctx.request.post(`${kratosAdminURL}/admin/identities`, {
    data: {
      schema_id: "default",
      traits,
      credentials: {
        password: {
          identifiers: [identifier],
          config: { password }
        }
      }
    }
  });
  if (!resp.ok()) {
    expect(resp.status(), `unexpected status: ${resp.status()} (${await resp.text()})`).toBe(409);
  }
}

async function setupTenantAdminSession(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  const tenantHost = `t-tp220-${runID}.localhost`;
  const tenantName = `TP220 Tenant ${runID}`;
  const tenantAdminEmail = `tenant-admin+tp220-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const superadminBaseURL = process.env.E2E_SUPERADMIN_BASE_URL || "http://localhost:8081";
  const superadminUser = process.env.E2E_SUPERADMIN_USER || "admin";
  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || `admin+tp220-${runID}@example.invalid`;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  const superadminContext = await browser.newContext({
    baseURL: superadminBaseURL,
    httpCredentials: { username: superadminUser, password: superadminPass }
  });
  const superadminPage = await superadminContext.newPage();

  if (!process.env.E2E_SUPERADMIN_EMAIL) {
    await ensureKratosIdentity(superadminContext, kratosAdminURL, {
      traits: { email: superadminEmail },
      identifier: `sa:${superadminEmail.toLowerCase()}`,
      password: superadminLoginPass
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
  const tenantID = (await tenantRow.locator("code").first().innerText()).trim();
  expect(tenantID).not.toBe("");

  await ensureKratosIdentity(superadminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenantID, email: tenantAdminEmail, role_slug: "tenant-admin" },
    identifier: `${tenantID}:${tenantAdminEmail}`,
    password: tenantAdminPass
  });
  await superadminContext.close();

  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const appContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);

  const page = await appContext.newPage();
  return { appContext, page };
}

function assistantConversation({ state = "draft", riskTier = "low", candidates = [], resolvedCandidateID = "", withCommitResult = false }) {
  const turn =
    state === "draft"
      ? []
      : [
          {
            turn_id: "turn_tp220_1",
            user_input: "input",
            state,
            risk_tier: riskTier,
            request_id: "assistant_req_tp220_1",
            trace_id: "assistant_trace_tp220_1",
            policy_version: "v1",
            composition_version: "v1",
            mapping_version: "v1",
            intent: {
              action: "create_orgunit",
              parent_ref_text: "鲜花组织",
              entity_name: "运营部",
              effective_date: "2026-01-01"
            },
            ambiguity_count: candidates.length,
            confidence: 0.8,
            candidates,
            resolved_candidate_id: resolvedCandidateID,
            resolution_source: resolvedCandidateID ? "user_confirmed" : "",
            plan: {
              title: "创建组织计划",
              capability_key: "org.orgunit_create.field_policy",
              summary: "在指定父组织下创建部门，提交前需要确认候选主键"
            },
            dry_run: {
              explain: candidates.length > 1 ? "检测到多个同名父组织候选，需先确认候选主键" : "计划已生成，等待确认后可提交",
              diff: [
                { field: "name", after: "运营部" },
                { field: "effective_date", after: "2026-01-01" },
                { field: "parent_candidate_id", after: resolvedCandidateID || "pending_confirmation" }
              ]
            },
            commit_result: withCommitResult
              ? {
                  org_code: "AI2201",
                  parent_org_code: resolvedCandidateID || "FLOWER-A",
                  effective_date: "2026-01-01",
                  event_type: "orgunit_created",
                  event_uuid: "evt_tp220_1"
                }
              : null
          }
        ];
  return {
    conversation_id: "conv_tp220_1",
    tenant_id: "tenant_tp220",
    actor_id: "actor_tp220",
    actor_role: "tenant-admin",
    state,
    created_at: "2026-03-02T00:00:00Z",
    updated_at: "2026-03-02T00:00:00Z",
    turns: turn
  };
}

async function installAssistantMock(page, options) {
  const candidateA = {
    candidate_id: "FLOWER-A",
    candidate_code: "FLOWER-A",
    name: "鲜花组织",
    path: "/鲜花组织/华东",
    as_of: "2026-01-01",
    is_active: true,
    match_score: 0.9
  };
  const candidateB = {
    candidate_id: "FLOWER-B",
    candidate_code: "FLOWER-B",
    name: "鲜花组织",
    path: "/鲜花组织/华南",
    as_of: "2026-01-01",
    is_active: true,
    match_score: 0.8
  };
  const candidates = options.ambiguousCandidates ? [candidateA, candidateB] : [candidateA];
  let currentConversation = assistantConversation({ state: "draft" });

  const fulfillJSON = async (route, status, payload) => {
    await route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(payload)
    });
  };
  const fulfillError = async (route, status, code, message) => {
    await fulfillJSON(route, status, {
      code,
      message,
      trace_id: "trace-tp220"
    });
  };

  await page.route("**/internal/assistant/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "POST" && pathname === "/internal/assistant/conversations") {
      currentConversation = assistantConversation({ state: "draft" });
      await fulfillJSON(route, 200, currentConversation);
      return;
    }
    if (method === "GET" && pathname === "/internal/assistant/conversations") {
      const lastTurn = currentConversation.turns[currentConversation.turns.length - 1];
      await fulfillJSON(route, 200, {
        items: [
          {
            conversation_id: currentConversation.conversation_id,
            state: currentConversation.state,
            updated_at: currentConversation.updated_at,
            last_turn: lastTurn
              ? {
                  turn_id: lastTurn.turn_id,
                  user_input: lastTurn.user_input,
                  state: lastTurn.state,
                  risk_tier: lastTurn.risk_tier
                }
              : null
          }
        ],
        next_cursor: ""
      });
      return;
    }
    if (method === "GET" && pathname === "/internal/assistant/conversations/conv_tp220_1") {
      await fulfillJSON(route, 200, currentConversation);
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp220_1/turns") {
      currentConversation = assistantConversation({
        state: "validated",
        riskTier: options.riskTier,
        candidates,
        resolvedCandidateID: options.ambiguousCandidates ? "" : "FLOWER-A"
      });
      await fulfillJSON(route, 200, currentConversation);
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp220_1/turns/turn_tp220_1:confirm") {
      if (currentConversation.turns.length === 0) {
        await fulfillError(route, 409, "conversation_confirmation_required", "conversation confirmation required");
        return;
      }
      const body = request.postDataJSON ? request.postDataJSON() : {};
      const selectedCandidate = options.ambiguousCandidates
        ? String(body?.candidate_id || "")
        : "FLOWER-A";
      if (options.ambiguousCandidates && selectedCandidate.length === 0) {
        await fulfillError(route, 409, "conversation_confirmation_required", "conversation confirmation required");
        return;
      }
      currentConversation = assistantConversation({
        state: "confirmed",
        riskTier: options.riskTier,
        candidates,
        resolvedCandidateID: selectedCandidate
      });
      await fulfillJSON(route, 200, currentConversation);
      return;
    }
    if (method === "POST" && pathname === "/internal/assistant/conversations/conv_tp220_1/turns/turn_tp220_1:commit") {
      if (currentConversation.turns.length === 0 || currentConversation.turns[0].state !== "confirmed") {
        await fulfillError(route, 409, "conversation_confirmation_required", "conversation confirmation required");
        return;
      }
      if (options.commitError) {
        if (options.commitError.code === "conversation_confirmation_required") {
          currentConversation = assistantConversation({
            state: "validated",
            riskTier: options.riskTier,
            candidates,
            resolvedCandidateID: currentConversation.turns[0].resolved_candidate_id || ""
          });
        }
        await fulfillError(route, options.commitError.status, options.commitError.code, options.commitError.message);
        return;
      }
      currentConversation = assistantConversation({
        state: "committed",
        riskTier: options.riskTier,
        candidates,
        resolvedCandidateID: currentConversation.turns[0].resolved_candidate_id || "FLOWER-A",
        withCommitResult: true
      });
      await fulfillJSON(route, 200, currentConversation);
      return;
    }

    await route.continue();
  });
}

test("tp220-e2e-101: low-risk happy path (generate -> confirm -> commit)", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "101");
  await installAssistantMock(page, { riskTier: "low", ambiguousCandidates: false });

  await page.goto("/app/assistant?as_of=2026-01-01");
  await expect(page.getByRole("heading", { name: "AI 助手" })).toBeVisible();
  await page.getByTestId("assistant-generate-button").click();
  await expect(page.getByTestId("assistant-risk-tier")).toContainText("risk=low");
  await expect(page.getByTestId("assistant-dryrun")).toBeVisible();

  await expect(page.getByTestId("assistant-confirm-button")).toBeEnabled();
  await page.getByTestId("assistant-confirm-button").click();
  await expect(page.getByTestId("assistant-commit-button")).toBeEnabled();
  await page.getByTestId("assistant-commit-button").click();

  await expect(page.getByTestId("assistant-turn-state")).toContainText("committed");
  await expect(page.getByTestId("assistant-commit-result")).toContainText("org_code=AI2201");
  await appContext.close();
});

test("tp220-e2e-102: high-risk role drift fail-closed", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "102");
  await installAssistantMock(page, {
    riskTier: "high",
    ambiguousCandidates: false,
    commitError: {
      status: 403,
      code: "ai_actor_role_drift_detected",
      message: "ai actor role drift detected"
    }
  });

  await page.goto("/app/assistant?as_of=2026-01-01");
  await page.getByTestId("assistant-generate-button").click();
  await expect(page.getByTestId("assistant-risk-tier")).toContainText("risk=high");
  await expect(page.getByTestId("assistant-risk-blocker")).toBeVisible();

  await page.getByTestId("assistant-confirm-button").click();
  await page.getByTestId("assistant-commit-button").click();
  await expect(page.getByTestId("assistant-error-alert")).toContainText(/Role changed during this conversation|角色发生变化/);
  await expect(page.getByTestId("assistant-turn-state")).not.toContainText("committed");
  await appContext.close();
});

test("tp220-e2e-103: fixed prompt create department with trace fields", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "103");
  await installAssistantMock(page, { riskTier: "high", ambiguousCandidates: false });

  await page.goto("/app/assistant?as_of=2026-01-01");
  await page
    .getByLabel("输入需求")
    .fill("在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。");
  await page.getByTestId("assistant-generate-button").click();
  await expect(page.getByTestId("assistant-plan")).toContainText("创建组织计划");
  await expect(page.getByTestId("assistant-request-id")).toContainText("assistant_req_tp220_1");
  await expect(page.getByTestId("assistant-trace-id")).toContainText("assistant_trace_tp220_1");

  await page.getByTestId("assistant-confirm-button").click();
  await page.getByTestId("assistant-commit-button").click();
  await expect(page.getByTestId("assistant-commit-result")).toContainText("effective_date=2026-01-01");
  await appContext.close();
});

test("tp220-e2e-104: ambiguous candidate must confirm before commit", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "104");
  await installAssistantMock(page, { riskTier: "high", ambiguousCandidates: true });

  await page.goto("/app/assistant?as_of=2026-01-01");
  await page.getByTestId("assistant-generate-button").click();
  await expect(page.getByTestId("assistant-candidate-blocker")).toBeVisible();
  await expect(page.getByTestId("assistant-commit-button")).toBeDisabled();

  await page.getByLabel("鲜花组织 / FLOWER-B / /鲜花组织/华南 / 2026-01-01").click();
  await page.getByTestId("assistant-confirm-button").click();
  await page.getByTestId("assistant-commit-button").click();

  await expect(page.getByTestId("assistant-commit-result")).toContainText("parent=FLOWER-B");
  await appContext.close();
});

test("tp220-e2e-007: librechat formal entry cannot bypass business write routes", async ({ browser }) => {
  test.setTimeout(240_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "007");

  await page.goto("/app/assistant/librechat");
  await expect(page).toHaveTitle(/LibreChat/i);

  const bypassResp = await appContext.request.post("/assistant-ui/org/api/org-units", {
    data: {
      org_code: "BYPASS220",
      name: "Bypass220",
      effective_date: "2026-01-01",
      parent_org_code: ""
    }
  });
  expect([200, 201]).not.toContain(bypassResp.status());

  await appContext.close();
});
