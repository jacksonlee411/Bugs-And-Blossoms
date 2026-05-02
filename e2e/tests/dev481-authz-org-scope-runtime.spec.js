import { expect, test } from "@playwright/test";

import { createIAMSession } from "./helpers/iam-session.js";
import { ensureKratosIdentity } from "./helpers/kratos-identity.js";
import { createOrgUnit, waitForOrgUnitDetails } from "./helpers/org-baseline.js";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const AS_OF = "2026-01-01";
const TENANT_ADMIN_PASS = process.env.E2E_TENANT_ADMIN_PASS || "pw";
const TENANT_VIEWER_PASS = process.env.E2E_TENANT_VIEWER_PASS || TENANT_ADMIN_PASS;
const CUBEBOX_PROVIDER_ID = process.env.CUBEBOX_PROVIDER_ID || "deepseek";
const CUBEBOX_MODEL_SLUG = process.env.CUBEBOX_MODEL_SLUG || "deepseek-v4-flash";

test.skip(process.env.E2E_ENABLE_LIVE_CUBEBOX !== "1", "requires E2E_ENABLE_LIVE_CUBEBOX=1");

function expectJSONStatus(response, expectedStatus, bodyText) {
  expect(response.status(), bodyText).toBe(expectedStatus);
}

async function responseJSON(response) {
  const text = await response.text();
  return { text, json: text.trim() ? JSON.parse(text) : null };
}

function codeSet(items) {
  return new Set((items || []).map((item) => String(item?.org_code || item?.entity_key || "").trim()).filter(Boolean));
}

function expectSetIncludesAll(values, expected) {
  for (const item of expected) {
    expect(values.has(item), `expected set to include ${item}; got ${[...values].join(", ")}`).toBeTruthy();
  }
}

function expectSetExcludesAll(values, forbidden) {
  for (const item of forbidden) {
    expect(values.has(item), `expected set to exclude ${item}; got ${[...values].join(", ")}`).toBeFalsy();
  }
}

function parseSSEEvents(raw) {
  const events = [];
  for (const block of String(raw || "").split(/\n\n+/)) {
    const dataLines = block
      .split(/\n/)
      .map((line) => line.trim())
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.replace(/^data:\s*/, ""));
    if (dataLines.length === 0) {
      continue;
    }
    const payload = dataLines.join("\n").trim();
    if (!payload || payload === "[DONE]") {
      continue;
    }
    events.push(JSON.parse(payload));
  }
  return events;
}

function latestCandidateCodes(events) {
  const candidateEvents = events.filter((event) => event?.type === "turn.query_candidates.presented");
  expect(candidateEvents.length, JSON.stringify(events, null, 2)).toBeGreaterThan(0);
  const candidates = candidateEvents.at(-1)?.payload?.candidates || [];
  return codeSet(candidates);
}

async function loadConversation(context, conversationID) {
  const response = await context.request.get(`/internal/cubebox/conversations/${conversationID}`, {
    headers: { Accept: "application/json" }
  });
  const { text, json } = await responseJSON(response);
  expectJSONStatus(response, 200, text);
  return json;
}

async function listScopedOrgUnits(context, runMarker) {
  const query = new URLSearchParams({
    as_of: AS_OF,
    mode: "grid",
    page: "0",
    size: "100",
    q: runMarker,
    sort: "code",
    order: "asc"
  });
  const response = await context.request.get(`/org/api/org-units?${query.toString()}`, {
    headers: { Accept: "application/json" }
  });
  const { text, json } = await responseJSON(response);
  expectJSONStatus(response, 200, text);
  return Array.isArray(json?.org_units) ? json.org_units : [];
}

async function getPrincipalByEmail(adminContext, email) {
  const response = await adminContext.request.get("/iam/api/authz/user-assignments", {
    headers: { Accept: "application/json" }
  });
  const { text, json } = await responseJSON(response);
  expectJSONStatus(response, 200, text);
  const principal = (json?.principals || []).find((item) => item.email === email);
  expect(principal, `principal not found for ${email}; candidates=${JSON.stringify(json?.principals || [])}`).toBeTruthy();
  return principal;
}

async function getAssignment(context, principalID) {
  const query = new URLSearchParams({ principal_id: principalID });
  const response = await context.request.get(`/iam/api/authz/user-assignments?${query.toString()}`, {
    headers: { Accept: "application/json" }
  });
  const { text, json } = await responseJSON(response);
  expectJSONStatus(response, 200, text);
  return json;
}

async function replaceAssignment(context, principalID, roleSlug, orgCode) {
  const assignment = await getAssignment(context, principalID);
  const response = await context.request.put(`/iam/api/authz/user-assignments/${principalID}`, {
    headers: { Accept: "application/json" },
    data: {
      roles: [{ role_slug: roleSlug }],
      org_scopes: [{ org_code: orgCode, include_descendants: true }],
      revision: assignment.revision
    }
  });
  const { text, json } = await responseJSON(response);
  expectJSONStatus(response, 200, text);
  expect(json.org_scopes?.[0]?.org_code).toBe(orgCode);
  return json;
}

async function createConversation(context) {
  const response = await context.request.post("/internal/cubebox/conversations", {
    headers: { Accept: "application/json" },
    data: {}
  });
  const { text, json } = await responseJSON(response);
  expectJSONStatus(response, 201, text);
  expect(json?.conversation?.id).toBeTruthy();
  return json;
}

async function streamCubeBoxOrgList(context, runMarker) {
  const conversation = await createConversation(context);
  const prompt = [
    `列出 2026-01-01 名称包含 ${runMarker} 的全部组织。`,
    "必须使用 orgunit.list，只返回组织清单；如果已有结果足够，请进入 DONE。"
  ].join("");
  const response = await context.request.post("/internal/cubebox/turns:stream", {
    timeout: 120_000,
    headers: { Accept: "text/event-stream" },
    data: {
      conversation_id: conversation.conversation.id,
      prompt,
      next_sequence: conversation.next_sequence
    }
  });
  const body = await response.text();
  expectJSONStatus(response, 200, body);
  const events = parseSSEEvents(body);
  const started = events.find((event) => event?.type === "turn.started");
  expect(started?.payload?.provider_id, body).toBe(CUBEBOX_PROVIDER_ID);
  expect(started?.payload?.model_slug, body).toBe(CUBEBOX_MODEL_SLUG);
  const terminal = events.find((event) => event?.type === "turn.error");
  expect(terminal, body).toBeFalsy();
  expect(events.some((event) => event?.type === "turn.completed"), body).toBeTruthy();
  const replay = await loadConversation(context, conversation.conversation.id);
  return latestCandidateCodes(replay.events || []);
}

async function configureRealCubeBoxProvider(context) {
  const providerBaseURL = process.env.CUBEBOX_BASE_URL || "https://api.deepseek.com";
  const providerType = process.env.CUBEBOX_PROVIDER_TYPE || "openai-compatible";
  const secretRef = process.env.CUBEBOX_SECRET_REF || "env://CUBEBOX_OPENAI_API_KEY";
  const maskedSecret = process.env.CUBEBOX_MASKED_SECRET || "env://CUBEBOX_OPENAI_API_KEY";

  const providerResp = await context.request.post("/internal/cubebox/settings/providers", {
    data: {
      provider_id: CUBEBOX_PROVIDER_ID,
      provider_type: providerType,
      display_name: process.env.CUBEBOX_PROVIDER_DISPLAY_NAME || "DeepSeek",
      base_url: providerBaseURL,
      enabled: true
    }
  });
  expect(providerResp.status(), await providerResp.text()).toBe(200);

  const credentialResp = await context.request.post("/internal/cubebox/settings/credentials", {
    data: {
      provider_id: CUBEBOX_PROVIDER_ID,
      secret_ref: secretRef,
      masked_secret: maskedSecret
    }
  });
  expect(credentialResp.status(), await credentialResp.text()).toBe(200);

  const selectionResp = await context.request.post("/internal/cubebox/settings/selection", {
    data: {
      provider_id: CUBEBOX_PROVIDER_ID,
      model_slug: CUBEBOX_MODEL_SLUG,
      capability_summary: {}
    }
  });
  expect(selectionResp.status(), await selectionResp.text()).toBe(200);
}

test("@live dev481: A/B org scope is enforced by orgunit API and real CubeBox orgunit query", async ({ browser }) => {
  test.setTimeout(300_000);

  const runID = String(Date.now());
  const runSuffix = runID.slice(-6).toUpperCase();
  const runMarker = `DEV481 ${runSuffix}`;
  const tenantHost = `t-dev481-authz-${runID}.localhost`;
  const tenantAdminEmail = `tenant-admin+dev481-${runID}@example.invalid`;
  const viewerEmail = `tenant-viewer+dev481-${runID}@example.invalid`;

  const tenant = await setupTenantAdminSession(browser, {
    tenantName: `DEV481 Authz Scope ${runID}`,
    tenantHost,
    tenantAdminEmail,
    tenantAdminPass: TENANT_ADMIN_PASS,
    sessionLoginRetryTimeoutMs: 15_000
  });

  const adminContext = tenant.appContext;
  const kratosAdminURL = process.env.E2E_KRATOS_ADMIN_URL || "http://localhost:4434";

  await configureRealCubeBoxProvider(adminContext);

  const rootCode = `D481R${runSuffix}`;
  const flowersCode = `D481F${runSuffix}`;
  const flowersChildCode = `D481FC${runSuffix}`;
  const bugsCode = `D481B${runSuffix}`;

  await createOrgUnit(adminContext, {
    org_code: rootCode,
    name: `${runMarker} 全集团`,
    effective_date: AS_OF,
    is_business_unit: true
  });
  await waitForOrgUnitDetails(adminContext, rootCode, AS_OF, 15_000);

  const adminPrincipal = await getPrincipalByEmail(adminContext, tenantAdminEmail);
  await replaceAssignment(adminContext, adminPrincipal.principal_id, "tenant-admin", rootCode);

  await createOrgUnit(adminContext, {
    org_code: flowersCode,
    name: `${runMarker} 鲜花事业部`,
    effective_date: AS_OF,
    parent_org_code: rootCode,
    is_business_unit: true
  });
  await createOrgUnit(adminContext, {
    org_code: flowersChildCode,
    name: `${runMarker} 鲜花下级`,
    effective_date: AS_OF,
    parent_org_code: flowersCode,
    is_business_unit: false
  });
  await createOrgUnit(adminContext, {
    org_code: bugsCode,
    name: `${runMarker} 飞虫事业部`,
    effective_date: AS_OF,
    parent_org_code: rootCode,
    is_business_unit: true
  });

  await waitForOrgUnitDetails(adminContext, bugsCode, AS_OF, 15_000);

  await ensureKratosIdentity(adminContext, kratosAdminURL, {
    traits: { tenant_uuid: tenant.tenantID, email: viewerEmail, role_slug: "tenant-viewer" },
    identifier: `${tenant.tenantID}:${viewerEmail}`,
    password: TENANT_VIEWER_PASS
  });

  const viewerContext = await browser.newContext({
    baseURL: tenant.appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": tenantHost }
  });
  await createIAMSession(viewerContext, viewerEmail, TENANT_VIEWER_PASS);

  const viewerPrincipal = await getPrincipalByEmail(adminContext, viewerEmail);

  await replaceAssignment(adminContext, viewerPrincipal.principal_id, "tenant-viewer", flowersCode);

  const adminAPICodes = codeSet(await listScopedOrgUnits(adminContext, runMarker));
  expectSetIncludesAll(adminAPICodes, [rootCode, flowersCode, flowersChildCode, bugsCode]);

  const viewerAPICodes = codeSet(await listScopedOrgUnits(viewerContext, runMarker));
  expectSetIncludesAll(viewerAPICodes, [flowersCode, flowersChildCode]);
  expectSetExcludesAll(viewerAPICodes, [rootCode, bugsCode]);

  const viewerChildDetails = await viewerContext.request.get(
    `/org/api/org-units/details?org_code=${encodeURIComponent(flowersChildCode)}&as_of=${AS_OF}`,
    { headers: { Accept: "application/json" } }
  );
  expect(viewerChildDetails.status(), await viewerChildDetails.text()).toBe(200);

  const viewerSiblingDetails = await viewerContext.request.get(
    `/org/api/org-units/details?org_code=${encodeURIComponent(bugsCode)}&as_of=${AS_OF}`,
    { headers: { Accept: "application/json" } }
  );
  const siblingBody = await viewerSiblingDetails.text();
  expect(viewerSiblingDetails.status(), siblingBody).toBe(403);
  expect(JSON.parse(siblingBody).code).toBe("authz_scope_forbidden");

  const adminCubeBoxCodes = await streamCubeBoxOrgList(adminContext, runMarker);
  expectSetIncludesAll(adminCubeBoxCodes, [rootCode, flowersCode, flowersChildCode, bugsCode]);

  const viewerCubeBoxCodes = await streamCubeBoxOrgList(viewerContext, runMarker);
  expectSetIncludesAll(viewerCubeBoxCodes, [flowersCode, flowersChildCode]);
  expectSetExcludesAll(viewerCubeBoxCodes, [rootCode, bugsCode]);
  expect([...viewerCubeBoxCodes].sort()).toEqual([...viewerAPICodes].sort());

  await viewerContext.close();
  await adminContext.close();
});
