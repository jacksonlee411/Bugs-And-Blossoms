import { expect, test } from "@playwright/test";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";
import { legacyOrgFieldPattern } from "./helpers/org-contract-assert";

async function createTP220Session(browser, suffix) {
  const runID = `${Date.now()}-${suffix}`;
  return setupTenantAdminSession(browser, {
    tenantName: `TP220 Tenant ${runID}`,
    tenantHost: `t-tp220-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp220-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp220-${runID}@example.invalid`,
    createPage: true
  });
}

function defaultRuntimeStatus() {
  return {
    status: "healthy",
    checked_at: "2026-03-09T00:00:00Z",
    upstream: { url: "http://localhost:3080" },
    services: [
      { name: "api", required: true, healthy: "healthy" },
      { name: "mongodb", required: false, healthy: "retired", reason: "retired_by_design" }
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
  const { appContext, page } = await createTP220Session(browser, "101");

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
  const { appContext, page } = await createTP220Session(browser, "102");

  try {
    await installAssistantLogPageMock(page, {
      runtimeStatus: {
        status: "degraded",
        checked_at: "2026-03-09T00:02:00Z",
        upstream: { url: "http://localhost:3999" },
        services: [
          { name: "api", required: true, healthy: "unavailable" },
          { name: "mongodb", required: false, healthy: "retired", reason: "retired_by_design" }
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
              user_input: "请为 org_code=BU220 新建 AI治理办公室 下的人力资源部2",
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
    await expect(page.getByTestId("assistant-conversation-log-item")).toContainText("org_code=BU220");
    await expect(page.getByTestId("assistant-conversation-log-item")).toContainText("人力资源部2");
    await expect(page.getByTestId("assistant-conversation-log-item")).not.toContainText(legacyOrgFieldPattern);
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-103: /app/assistant points users to the formal LibreChat entry", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await createTP220Session(browser, "103");

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
  const { appContext, page } = await createTP220Session(browser, "104");

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
  const { appContext, page } = await createTP220Session(browser, "007");

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
    expect(bypassResp.status()).toBe(410);
  } finally {
    await appContext.close();
  }
});
