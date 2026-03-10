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

function defaultRuntimeStatus() {
  return {
    status: "healthy",
    checked_at: "2026-03-09T00:00:00Z",
    upstream: { url: "http://localhost:3080" },
    services: [
      { name: "api", required: true, healthy: "healthy" },
      { name: "mongodb", required: true, healthy: "healthy" }
    ],
    capabilities: {
      actions_enabled: true,
      agents_write_enabled: false,
      mcp_enabled: true
    }
  };
}

function defaultConversationList() {
  return {
    items: [
      {
        conversation_id: "conv_tp220_1",
        state: "confirmed",
        updated_at: "2026-03-09T00:00:01Z",
        last_turn: {
          turn_id: "turn_tp220_1",
          user_input: "在鲜花组织下新建运营部",
          state: "confirmed",
          risk_tier: "high"
        }
      }
    ],
    next_cursor: ""
  };
}

async function fulfillJSON(route, status, payload) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(payload)
  });
}

async function installAssistantLogPageMock(page, overrides = {}) {
  const runtimeStatus = overrides.runtimeStatus ?? defaultRuntimeStatus();
  const conversationList = overrides.conversationList ?? defaultConversationList();

  await page.route("**/internal/assistant/runtime-status", async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await fulfillJSON(route, 200, runtimeStatus);
  });

  await page.route("**/internal/assistant/conversations*", async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await fulfillJSON(route, 200, conversationList);
  });
}

test("tp220-e2e-101: /app/assistant stays read-only after old bridge retirement", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "101");

  try {
    await installAssistantLogPageMock(page);
    await page.goto("/app/assistant?as_of=2026-01-01");

    await expect(page.getByRole("heading", { name: "AI 助手日志" })).toBeVisible();
    await expect(page.getByText(/旧 `iframe \+ bridge` 对话承载页已按 `DEV-PLAN-282` 退役/)).toBeVisible();
    await expect(page.getByText(/正式交互入口已统一到 `\/app\/assistant\/librechat`/)).toBeVisible();
    await expect(page.getByRole("link", { name: "打开 LibreChat" })).toHaveAttribute("href", "/app/assistant/librechat");

    await expect(page.locator('[data-testid="assistant-generate-button"]')).toHaveCount(0);
    await expect(page.locator('[data-testid="assistant-confirm-button"]')).toHaveCount(0);
    await expect(page.locator('[data-testid="assistant-commit-button"]')).toHaveCount(0);
    await expect(page.locator('[data-testid="assistant-librechat-frame"]')).toHaveCount(0);
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-102: /app/assistant renders runtime summary and recent conversation logs", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "102");

  try {
    await installAssistantLogPageMock(page, {
      runtimeStatus: {
        status: "degraded",
        checked_at: "2026-03-09T00:02:00Z",
        upstream: { url: "http://localhost:3999" },
        services: [
          { name: "api", required: true, healthy: "unavailable" },
          { name: "mongodb", required: true, healthy: "healthy" }
        ],
        capabilities: {
          actions_enabled: true,
          agents_write_enabled: false,
          mcp_enabled: true
        }
      },
      conversationList: {
        items: [
          {
            conversation_id: "conv_tp220_runtime",
            state: "confirmed",
            updated_at: "2026-03-09T00:03:00Z",
            last_turn: {
              turn_id: "turn_tp220_runtime",
              user_input: "在 AI治理办公室 下新建 人力资源部2",
              state: "confirmed",
              risk_tier: "high"
            }
          }
        ],
        next_cursor: ""
      }
    });
    await page.goto("/app/assistant");

    await expect(page.getByTestId("assistant-runtime-status")).toContainText("degraded");
    await expect(page.getByTestId("assistant-runtime-upstream-url")).toContainText("http://localhost:3999");
    await expect(page.getByText("api:unavailable")).toBeVisible();
    await expect(page.getByTestId("assistant-conversation-log-item")).toContainText("conv_tp220_runtime");
    await expect(page.getByTestId("assistant-conversation-log-item")).toContainText("在 AI治理办公室 下新建 人力资源部2");
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-103: /app/assistant points users to the formal LibreChat entry", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "103");

  try {
    await installAssistantLogPageMock(page);
    await page.goto("/app/assistant");

    await expect(page.getByRole("link", { name: "打开 LibreChat" })).toHaveAttribute("href", "/app/assistant/librechat");
    await expect(page.getByRole("link", { name: "模型配置" })).toHaveAttribute("href", "/app/assistant/models");

    await page.getByRole("link", { name: "打开 LibreChat" }).click();
    await expect(page).toHaveURL(/\/app\/assistant\/librechat$/);
    await expect(page).toHaveTitle(/(LibreChat|Bugs \& Blossoms Assistant)/i);
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-104: draft conversation logs stay read-only and never re-enable old actions", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "104");

  try {
    await installAssistantLogPageMock(page, {
      conversationList: {
        items: [
          {
            conversation_id: "conv_tp220_draft",
            state: "draft",
            updated_at: "2026-03-09T00:04:00Z"
          }
        ],
        next_cursor: ""
      }
    });
    await page.goto("/app/assistant?as_of=2026-01-01");

    await expect(page.getByTestId("assistant-conversation-log-item")).toContainText("conv_tp220_draft · draft");
    await expect(page.getByTestId("assistant-conversation-log-item")).toContainText("暂无轮次记录");
    await expect(page.locator('[data-testid="assistant-generate-button"]')).toHaveCount(0);
    await expect(page.locator('[data-testid="assistant-confirm-button"]')).toHaveCount(0);
    await expect(page.locator('[data-testid="assistant-commit-button"]')).toHaveCount(0);
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-007: librechat formal entry cannot bypass business write routes", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await setupTenantAdminSession(browser, "007");

  try {
    await page.goto("/app/assistant/librechat");
    await expect(page).toHaveTitle(/(LibreChat|Bugs \& Blossoms Assistant)/i);

    const bypassResp = await appContext.request.post("/assistant-ui/org/api/org-units", {
      data: {
        org_code: "BYPASS220",
        name: "Bypass220",
        effective_date: "2026-01-01",
        parent_org_code: ""
      }
    });
    expect([200, 201]).not.toContain(bypassResp.status());
  } finally {
    await appContext.close();
  }
});
