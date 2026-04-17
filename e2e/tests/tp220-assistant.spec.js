import { expect, test } from "@playwright/test";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

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

async function getRetiredJSON(request, path, options = {}) {
  const response = await request.get(path, {
    maxRedirects: 0,
    ...options,
    headers: {
      Accept: "application/json",
      ...(options.headers || {})
    }
  });
  expect(response.status()).toBe(410);
  return response.json();
}

async function postRetiredJSON(request, path, data) {
  const response = await request.post(path, {
    data,
    headers: {
      Accept: "application/json"
    }
  });
  expect(response.status()).toBe(410);
  return response.json();
}

test("tp220-e2e-101: /app/assistant redirects to CubeBox after old bridge retirement", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await createTP220Session(browser, "101");

  try {
    await page.goto("/app/assistant?as_of=2026-01-01");
    await expect(page).toHaveURL(/\/app\/cubebox$/);
    await expect(page.getByRole("heading", { name: "CubeBox" })).toBeVisible();
    await expect(page.getByText(/正式入口已切换到 `\/app\/cubebox`/)).toBeVisible();
    await expect(page.getByTestId("cubebox-input")).toBeVisible();
    await expect(page.getByTestId("cubebox-send")).toBeVisible();
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-102: /app/assistant resolves to CubeBox runtime and conversation list", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await createTP220Session(browser, "102");

  try {
    await page.goto("/app/assistant");

    await expect(page).toHaveURL(/\/app\/cubebox$/);
    await expect(page.getByTestId("cubebox-runtime-status")).toBeVisible();
    await expect(page.getByText(/正式入口已切换到 `\/app\/cubebox`/)).toBeVisible();
    await expect(page.getByRole("link", { name: "文件" })).toHaveAttribute("href", "/app/cubebox/files");
    await expect(page.getByRole("link", { name: "模型" })).toHaveAttribute("href", "/app/cubebox/models");
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-103: /app/assistant now resolves to the CubeBox formal entry", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await createTP220Session(browser, "103");

  try {
    await page.goto("/app/assistant");
    await expect(page).toHaveURL(/\/app\/cubebox$/);
    await expect(page.getByRole("heading", { name: "CubeBox" })).toBeVisible();
    await expect(page.getByRole("link", { name: "文件" })).toHaveAttribute("href", "/app/cubebox/files");
    await expect(page.getByRole("link", { name: "模型" })).toHaveAttribute("href", "/app/cubebox/models");
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-104: CubeBox root entry stays safe before any conversation is selected", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await createTP220Session(browser, "104");

  try {
    await page.goto("/app/assistant?as_of=2026-01-01");

    await expect(page).toHaveURL(/\/app\/cubebox$/);
    await expect(page.getByText("输入第一条消息即可创建会话。")).toBeVisible();
    await expect(page.getByTestId("cubebox-generate-reply")).toBeDisabled();
    await expect(page.getByTestId("cubebox-confirm")).toBeDisabled();
    await expect(page.getByTestId("cubebox-commit")).toBeDisabled();
    await expect(page.getByTestId("cubebox-turn-card")).toHaveCount(0);
  } finally {
    await appContext.close();
  }
});

test("tp220-e2e-007: retired librechat entry cannot bypass business write routes", async ({ browser }) => {
  test.setTimeout(120_000);
  const { appContext, page } = await createTP220Session(browser, "007");

  try {
    const retiredResponse = await page.goto("/app/assistant/librechat");
    expect(retiredResponse?.status()).toBe(410);
    await expect(page.getByText(/LibreChat 入口已退役|CubeBox 正式入口/)).toBeVisible();

    const retiredPayload = await getRetiredJSON(appContext.request, "/app/assistant/librechat");
    expect(retiredPayload.code).toBe("librechat_retired");

    const bypassPayload = await postRetiredJSON(appContext.request, "/assistant-ui/org/api/org-units", {
      org_code: "BYPASS220",
      name: "Bypass220",
      effective_date: "2026-01-01",
      parent_org_code: ""
    });
    expect(bypassPayload.code).toBe("assistant_ui_retired");
  } finally {
    await appContext.close();
  }
});
