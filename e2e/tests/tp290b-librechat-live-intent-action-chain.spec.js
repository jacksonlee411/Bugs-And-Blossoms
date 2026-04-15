import { expect, test } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";

import {
  latestAssistantTurn,
  parseJSONSafe,
  parseResponseBody,
} from "./helpers/assistant-conversation.js";
import { ensureDir, writeJSON } from "./helpers/evidence.js";
import {
  collectCandidateDetails,
  collectOrgDetailsBySpecs,
  createOrgUnit,
  detectRootOrg,
  ensureOrgUnitByCode,
  orgUnitDetailsSnapshot,
  waitForOrgUnitDetails,
} from "./helpers/org-baseline.js";
import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

const repoRoot = path.resolve(__dirname, "..", "..");
const EVIDENCE_ROOT = path.join(repoRoot, "docs", "dev-records", "assets", "dev-plan-290b");
const INDEX_PATH = path.join(EVIDENCE_ROOT, "tp290b-live-evidence-index.json");
const BASELINE_PATH = path.join(EVIDENCE_ROOT, "tp290b-data-baseline.json");
const RUNTIME_GATE_PATH = path.join(EVIDENCE_ROOT, "runtime-admission-gate.json");
const RUNTIME_GATE_HAR_PATH = path.join(EVIDENCE_ROOT, "runtime-admission-gate.har");
const FORMAL_ENTRY_PATH = "/app/cubebox";
const INTERNAL_API_PREFIX = "/internal/cubebox";
const BASELINE_EFFECTIVE_DATE = "2026-01-01";
const BASELINE_CASE4_AS_OF = "2026-03-26";
const BASELINE_ROOT_CODE = "ROOT";
const BASELINE_ROOT_NAME = "集团";
const BASELINE_ORG_SPECS = {
  ai_governance_office: {
    code: "TP290BAIGOV",
    name: "AI治理办公室",
  },
  shared_service_center_primary: {
    code: "TP290BSSC1",
    name: "共享服务中心",
  },
  shared_service_center_branch: {
    code: "TP290BSSB",
    name: "B",
  },
  shared_service_center_secondary: {
    code: "TP290BSSC2",
    name: "共享服务中心",
  },
};

const CASE_INPUTS = {
  1: ["你好"],
  2: ["在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01", "确认"],
  3: ["在 AI治理办公室 下新建 人力资源部239A补全", "生效日期 2026-03-25", "确认"],
  4: ["请在父组织共享服务中心下新建候选验证部239A，生效日期2026-03-26", "选第2个", "是的"],
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

function isIgnorableCloseError(error) {
  const message = String(error || "").toLowerCase();
  return message.includes("enoent") || message.includes("step id not found");
}

async function closeContextSafely(context) {
  if (!context) {
    return;
  }
  try {
    await context.close();
  } catch (error) {
    if (!isIgnorableCloseError(error)) {
      throw error;
    }
  }
}

function upsertCaseSummary(summary) {
  const index = caseSummaries.findIndex((item) => item.id === summary.id);
  if (index >= 0) {
    caseSummaries[index] = summary;
    return;
  }
  caseSummaries.push(summary);
}

function defaultCaseSummary(caseId) {
  return {
    id: caseId,
    status: "not_run",
    input_sequence: CASE_INPUTS[caseId],
    blocking_reason: "前置用例失败后未执行",
  };
}

function normalizeText(value) {
  return String(value || "").trim().replace(/\s+/g, " ");
}

function dryRunValidationErrors(turn) {
  if (!Array.isArray(turn?.dry_run?.validation_errors)) {
    return [];
  }
  return turn.dry_run.validation_errors.map((item) => String(item || "").trim()).filter(Boolean);
}

function hasValidationError(turn, code) {
  return dryRunValidationErrors(turn).includes(code);
}

function orgUnitsByExactName(orgUnits, name) {
  const target = normalizeText(name);
  return orgUnits.filter((item) => normalizeText(item?.name) === target);
}

function orgUnitsByNameContains(orgUnits, name) {
  const target = normalizeText(name);
  return orgUnits.filter((item) => normalizeText(item?.name).includes(target));
}

async function createAssistantProbe(appContext, userInput) {
  const createConversation = await appContext.request.post(`${INTERNAL_API_PREFIX}/conversations`, { data: {} });
  const { text: conversationText, json: conversationJSON } = await parseResponseBody(createConversation);
  const conversationID = String(conversationJSON?.conversation_id || "").trim();
  if (createConversation.status() !== 200 || !conversationID) {
    return {
      conversation_id: conversationID,
      status: createConversation.status(),
      raw_text: conversationText,
      conversation: conversationJSON,
      latest_turn: latestAssistantTurn(conversationJSON || {}),
      error_code: String(conversationJSON?.code || "").trim(),
    };
  }
  const createTurn = await appContext.request.post(
    `${INTERNAL_API_PREFIX}/conversations/${encodeURIComponent(conversationID)}/turns`,
    {
      data: { user_input: userInput },
    },
  );
  const { text: turnText, json: turnJSON } = await parseResponseBody(createTurn);
  return {
    conversation_id: conversationID,
    status: createTurn.status(),
    raw_text: turnText,
    conversation: turnJSON,
    latest_turn: latestAssistantTurn(turnJSON || {}),
    error_code: String(turnJSON?.code || "").trim(),
  };
}

async function createAssistantProbeWithRetry(appContext, userInput, isReady, maxAttempts = 3) {
  let lastProbe = null;
  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    lastProbe = await createAssistantProbe(appContext, userInput);
    if (!isReady || isReady(lastProbe)) {
      return lastProbe;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  return lastProbe;
}

function baselineProbeSummary(probe) {
  const turn = probe?.latest_turn || null;
  return {
    conversation_id: String(probe?.conversation_id || ""),
    create_turn_status: Number(probe?.status || 0),
    error_code: String(probe?.error_code || ""),
    phase: String(turn?.phase || ""),
    intent_action: String(turn?.intent?.action || ""),
    parent_ref_text: String(turn?.intent?.parent_ref_text || ""),
    candidate_count: Array.isArray(turn?.candidates) ? turn.candidates.length : 0,
    validation_errors: dryRunValidationErrors(turn),
    resolved_candidate_id: String(turn?.resolved_candidate_id || turn?.resolvedCandidateID || ""),
    request_id: String(turn?.request_id || ""),
    trace_id: String(turn?.trace_id || ""),
  };
}

function assistantCandidateSnapshot(turn) {
  if (!Array.isArray(turn?.candidates)) {
    return [];
  }
  return turn.candidates.map((item, index) => ({
    ordinal: index + 1,
    candidate_id: String(item?.candidate_id || item?.candidateId || item?.id || "").trim(),
    org_code: String(item?.org_code || item?.orgCode || "").trim(),
    name: String(item?.name || item?.display_name || item?.displayName || item?.label || "").trim(),
    parent_org_code: String(item?.parent_org_code || item?.parentOrgCode || "").trim(),
    full_name_path: String(item?.full_name_path || item?.fullNamePath || item?.label || "").trim(),
  }));
}

async function ensureTenantBaseline(appContext, tenantID) {
  const report = {
    tenant_id: tenantID,
    effective_date: BASELINE_EFFECTIVE_DATE,
    validated_at: new Date().toISOString(),
    status: "blocked",
    root_org_code: "",
    created_orgs: [],
    required_orgs: [],
    candidate_snapshot: {
      query: "共享服务中心",
      as_of: BASELINE_CASE4_AS_OF,
      candidate_count: 0,
      candidates: [],
    },
    issues: [],
    probes: {},
  };

  let rootOrg = await detectRootOrg(appContext, BASELINE_EFFECTIVE_DATE, BASELINE_ROOT_CODE);
  if (!rootOrg) {
    await createOrgUnit(appContext, {
      org_code: BASELINE_ROOT_CODE,
      name: BASELINE_ROOT_NAME,
      effective_date: BASELINE_EFFECTIVE_DATE,
      parent_org_code: "",
      is_business_unit: true,
    });
    report.created_orgs.push({
      org_code: BASELINE_ROOT_CODE,
      name: BASELINE_ROOT_NAME,
      parent_org_code: "",
      effective_date: BASELINE_EFFECTIVE_DATE,
      is_business_unit: true,
    });
    const createdRoot = await waitForOrgUnitDetails(appContext, BASELINE_ROOT_CODE, BASELINE_CASE4_AS_OF);
    expect(createdRoot?.org_unit, "root org should be readable after creation").toBeTruthy();
    rootOrg = createdRoot.org_unit;
  }
  report.root_org_code = String(rootOrg?.org_code || "").trim();

  const aiGovernanceDetailsBefore = await waitForOrgUnitDetails(
    appContext,
    BASELINE_ORG_SPECS.ai_governance_office.code,
    BASELINE_CASE4_AS_OF,
    250,
  );
  if (!aiGovernanceDetailsBefore?.org_unit) {
    await ensureOrgUnitByCode(
      appContext,
      BASELINE_ORG_SPECS.ai_governance_office,
      {
        effectiveDate: BASELINE_EFFECTIVE_DATE,
        parentOrgCode: report.root_org_code,
        readAsOf: BASELINE_CASE4_AS_OF,
        lookupTimeoutMs: 250,
        onCreated: async (created) => report.created_orgs.push(created),
      },
    );
  }

  const sharedServicePrimaryBefore = await waitForOrgUnitDetails(
    appContext,
    BASELINE_ORG_SPECS.shared_service_center_primary.code,
    BASELINE_CASE4_AS_OF,
    250,
  );
  if (!sharedServicePrimaryBefore?.org_unit) {
    await ensureOrgUnitByCode(
      appContext,
      BASELINE_ORG_SPECS.shared_service_center_primary,
      {
        effectiveDate: BASELINE_EFFECTIVE_DATE,
        parentOrgCode: report.root_org_code,
        readAsOf: BASELINE_CASE4_AS_OF,
        lookupTimeoutMs: 250,
        onCreated: async (created) => report.created_orgs.push(created),
      },
    );
  }

  const sharedServiceBaselineDetails = await collectOrgDetailsBySpecs(
    appContext,
    [
      BASELINE_ORG_SPECS.shared_service_center_primary,
      BASELINE_ORG_SPECS.shared_service_center_secondary,
    ],
    BASELINE_CASE4_AS_OF,
  );
  if (sharedServiceBaselineDetails.length < 2) {
    await ensureOrgUnitByCode(
      appContext,
      BASELINE_ORG_SPECS.shared_service_center_branch,
      {
        effectiveDate: BASELINE_EFFECTIVE_DATE,
        parentOrgCode: report.root_org_code,
        readAsOf: BASELINE_CASE4_AS_OF,
        lookupTimeoutMs: 250,
        onCreated: async (created) => report.created_orgs.push(created),
      },
    );
    await ensureOrgUnitByCode(
      appContext,
      BASELINE_ORG_SPECS.shared_service_center_secondary,
      {
        effectiveDate: BASELINE_EFFECTIVE_DATE,
        parentOrgCode: BASELINE_ORG_SPECS.shared_service_center_branch.code,
        readAsOf: BASELINE_CASE4_AS_OF,
        lookupTimeoutMs: 250,
        onCreated: async (created) => report.created_orgs.push(created),
      },
    );
  }

  const aiGovernanceUnits = await collectOrgDetailsBySpecs(
    appContext,
    [BASELINE_ORG_SPECS.ai_governance_office],
    BASELINE_CASE4_AS_OF,
  );
  const sharedServiceUnits = await collectOrgDetailsBySpecs(
    appContext,
    [
      BASELINE_ORG_SPECS.shared_service_center_primary,
      BASELINE_ORG_SPECS.shared_service_center_secondary,
    ],
    BASELINE_CASE4_AS_OF,
  );
  const case2Probe = await createAssistantProbeWithRetry(appContext, CASE_INPUTS[2][0], (probe) => {
    const summary = baselineProbeSummary(probe);
    return summary.create_turn_status === 200 && summary.intent_action === "create_orgunit" && summary.phase === "await_commit_confirm" && Boolean(summary.resolved_candidate_id);
  });
  const case4Probe = await createAssistantProbeWithRetry(appContext, CASE_INPUTS[4][0], (probe) => {
    const summary = baselineProbeSummary(probe);
    return summary.create_turn_status === 200 && summary.intent_action === "create_orgunit" && summary.phase === "await_candidate_pick" && summary.candidate_count > 1;
  });
  const case2Summary = baselineProbeSummary(case2Probe);
  const case4Summary = baselineProbeSummary(case4Probe);
  const case4Candidates = assistantCandidateSnapshot(case4Probe?.latest_turn || null);

  report.probes.case2 = case2Summary;
  report.probes.case4 = case4Summary;
  report.required_orgs = [
    {
      name: BASELINE_ORG_SPECS.ai_governance_office.name,
      expected: "exactly_one_candidate",
      matched_count: aiGovernanceUnits.length,
      matched_org_codes: aiGovernanceUnits.map((item) => String(item.org_code || "").trim()),
      ready:
        aiGovernanceUnits.length === 1 &&
        case2Summary.create_turn_status === 200 &&
        case2Summary.intent_action === "create_orgunit" &&
        case2Summary.phase === "await_commit_confirm" &&
        !case2Summary.validation_errors.includes("parent_candidate_not_found") &&
        !case2Summary.validation_errors.includes("candidate_confirmation_required") &&
        Boolean(case2Summary.resolved_candidate_id),
    },
    {
      name: BASELINE_ORG_SPECS.shared_service_center_primary.name,
      expected: "multiple_candidates",
      matched_count: sharedServiceUnits.length,
      matched_org_codes: sharedServiceUnits.map((item) => String(item.org_code || "").trim()),
      ready:
        sharedServiceUnits.length > 1 &&
        case4Summary.create_turn_status === 200 &&
        case4Summary.intent_action === "create_orgunit" &&
        case4Summary.phase === "await_candidate_pick" &&
        case4Summary.candidate_count > 1 &&
        !case4Summary.validation_errors.includes("parent_candidate_not_found"),
    },
  ];
  report.candidate_snapshot.candidate_count = case4Summary.candidate_count;
  report.candidate_snapshot.candidates =
    case4Candidates.length > 0
      ? case4Candidates
      : await collectCandidateDetails(appContext, sharedServiceUnits, BASELINE_CASE4_AS_OF);

  if (report.required_orgs[0].matched_count !== 1) {
    report.issues.push(`AI治理办公室 命中数量异常：${report.required_orgs[0].matched_count}`);
  }
  if (report.required_orgs[1].matched_count <= 1) {
    report.issues.push(`共享服务中心 候选数量不足：${report.required_orgs[1].matched_count}`);
  }
  if (!report.required_orgs[0].ready) {
    report.issues.push(`Case2 基线探针未就绪（phase=${case2Summary.phase || "unknown"} validation=${case2Summary.validation_errors.join(",") || "none"}）`);
  }
  if (!report.required_orgs[1].ready) {
    report.issues.push(`Case4 基线探针未就绪（phase=${case4Summary.phase || "unknown"} candidates=${case4Summary.candidate_count}）`);
  }

  report.status = report.required_orgs.every((item) => item.ready) ? "passed" : "blocked";
  return report;
}

function buildBaselineGateError(message, baseline) {
  const error = new Error(message);
  error.code = "tp290b_baseline_not_ready";
  error.baseline = baseline;
  return error;
}

function assertCasePreflightGate(caseId, snapshot, baseline) {
  const turn = snapshot?.latest_turn || null;
  if (caseId === 2) {
    if (hasValidationError(turn, "parent_candidate_not_found")) {
      throw buildBaselineGateError(
        `数据基线未就绪：AI治理办公室 未命中候选（phase=${String(turn?.phase || "unknown")})`,
        baseline,
      );
    }
    if (String(turn?.phase || "") !== "await_commit_confirm") {
      throw buildBaselineGateError(
        `数据基线未就绪：Case2 首轮未进入 await_commit_confirm（phase=${String(turn?.phase || "unknown")})`,
        baseline,
      );
    }
  }
  if (caseId === 4) {
    const candidateCount = Array.isArray(turn?.candidates) ? turn.candidates.length : 0;
    if (hasValidationError(turn, "parent_candidate_not_found")) {
      throw buildBaselineGateError(
        `数据基线未就绪：共享服务中心 未命中候选（phase=${String(turn?.phase || "unknown")})`,
        baseline,
      );
    }
    if (String(turn?.phase || "") !== "await_candidate_pick" || candidateCount <= 1) {
      throw buildBaselineGateError(
        `数据基线未就绪：Case4 首轮候选不足（phase=${String(turn?.phase || "unknown")} candidates=${candidateCount})`,
        baseline,
      );
    }
  }
}

function captureBaselineHint(caseId, tenantID, baseline, snapshot) {
  if (baseline) {
    baselineHints.t0 = {
      ...(baselineHints.t0 || {}),
      tenant_id: tenantID,
      ensured_status: String(baseline?.status || ""),
      root_org_code: String(baseline?.root_org_code || ""),
      created_orgs: Array.isArray(baseline?.created_orgs) ? baseline.created_orgs : [],
      required_orgs: Array.isArray(baseline?.required_orgs) ? baseline.required_orgs : [],
      candidate_snapshot: baseline?.candidate_snapshot || {},
      probes: baseline?.probes || {},
      issues: Array.isArray(baseline?.issues) ? baseline.issues : [],
      effective_date: String(baseline?.effective_date || ""),
      validated_at: String(baseline?.validated_at || ""),
    };
  }
  if (caseId !== 2 && caseId !== 4) {
    return;
  }
  const turn = snapshot?.latest_turn || null;
  baselineHints[`case${caseId}`] = {
    ...(baselineHints[`case${caseId}`] || {}),
    tenant_id: tenantID,
    ensured_status: String(baseline?.status || ""),
    root_org_code: String(baseline?.root_org_code || ""),
    created_orgs: Array.isArray(baseline?.created_orgs) ? baseline.created_orgs : [],
    required_orgs: Array.isArray(baseline?.required_orgs) ? baseline.required_orgs : [],
    issues: Array.isArray(baseline?.issues) ? baseline.issues : [],
    candidate_snapshot: baseline?.candidate_snapshot || {},
    probe: baseline?.probes?.[`case${caseId}`] || {},
    conversation_id: String(snapshot?.conversation?.conversation_id || snapshot?.conversation_id || ""),
    phase: String(turn?.phase || baseline?.probes?.[`case${caseId}`]?.phase || ""),
    parent_ref_text: String(turn?.intent?.parent_ref_text || baseline?.probes?.[`case${caseId}`]?.parent_ref_text || ""),
    candidate_count:
      Array.isArray(turn?.candidates) ? turn.candidates.length : Number(baseline?.probes?.[`case${caseId}`]?.candidate_count || 0),
    validation_errors:
      dryRunValidationErrors(turn).length > 0
        ? dryRunValidationErrors(turn)
        : Array.isArray(baseline?.probes?.[`case${caseId}`]?.validation_errors)
          ? baseline.probes[`case${caseId}`].validation_errors
          : [],
  };
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

async function createTP290BSession(browser, suffix, harPath) {
  const runID = `${Date.now()}-${suffix}`;
  return setupTenantAdminSession(browser, {
    tenantName: `TP290B Tenant ${runID}`,
    tenantHost: `t-tp290b-${runID}.localhost`,
    tenantAdminEmail: `tenant-admin+tp290b-${runID}@example.invalid`,
    superadminEmail: process.env.E2E_SUPERADMIN_EMAIL || `admin+tp290b-${runID}@example.invalid`,
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
    if (pathname.startsWith(INTERNAL_API_PREFIX + "/")) {
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
    if (!pathname.startsWith(INTERNAL_API_PREFIX + "/")) {
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

function formalInputField(surface) {
  return surface.getByTestId("cubebox-input").locator("textarea, input").first();
}

async function openFormalEntry(page) {
  await page.goto(FORMAL_ENTRY_PATH);
  await expect(page).toHaveURL(/\/app\/cubebox(?:\/conversations\/[^/]+)?$/);
  await expect(formalInputField(page)).toBeVisible({ timeout: 60_000 });
  return { surface: page, usedIframe: false };
}

async function sendFromFormalEntry(surface, text) {
  const input = formalInputField(surface);
  await input.fill(text);
  await surface.getByTestId("cubebox-send").click();
}

async function waitForVisibleNamedButton(surface, namePattern, timeoutMs = 30_000) {
  const buttons = surface.getByRole("button", { name: namePattern });
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const count = await buttons.count();
    for (let index = count - 1; index >= 0; index -= 1) {
      const candidate = buttons.nth(index);
      try {
        if (await candidate.isVisible()) {
          return candidate;
        }
      } catch {
        // ignore transient locator detach during surface refresh
      }
    }
    await surface.waitForTimeout(250);
  }
  throw new Error(`visible button not found for pattern: ${String(namePattern)}`);
}

async function clickFormalConfirm(surface) {
  const button = surface.getByTestId("cubebox-confirm");
  await expect(button).toBeVisible({ timeout: 30_000 });
  await button.click();
}

async function maybeClickFormalConfirm(surface, timeoutMs = 5_000) {
  try {
    const button = await waitForVisibleNamedButton(surface, /确认|Confirm/i, timeoutMs);
    await button.click();
    return true;
  } catch {
    return false;
  }
}

async function clickFormalSubmit(surface) {
  const button = surface.getByTestId("cubebox-commit");
  await expect(button).toBeVisible({ timeout: 30_000 });
  await button.click();
}

async function tryClickFormalSubmit(surface, timeoutMs = 5_000) {
  try {
    const button = await waitForVisibleNamedButton(surface, /提交|Submit/i, timeoutMs);
    await button.click();
    return true;
  } catch {
    return false;
  }
}

async function clickFormalCandidateSelect(surface, ordinal) {
  const buttons = surface.getByRole("button", { name: /(选择|Select).*(确认|Confirm)|Select \+ Confirm/i });
  const target = buttons.nth(Math.max(0, ordinal - 1));
  await expect(target).toBeVisible({ timeout: 30_000 });
  await target.click();
}

async function runFormalCaseStep(surface, caseId, stepIndex, text) {
  if (stepIndex === 0) {
    await sendFromFormalEntry(surface, text);
    return;
  }

  if (caseId === 2 && stepIndex === 1) {
    await clickFormalConfirm(surface);
    await clickFormalSubmit(surface);
    return;
  }

  if (caseId === 3 && stepIndex === 1) {
    await sendFromFormalEntry(surface, text);
    return;
  }

  if (caseId === 3 && stepIndex === 2) {
    await maybeClickFormalConfirm(surface);
    if (await tryClickFormalSubmit(surface, 5_000)) {
      return;
    }
    await sendFromFormalEntry(surface, "上级组织 AI治理办公室，下新建 人力资源部239A补全，生效日期 2026-03-25");
    await maybeClickFormalConfirm(surface, 30_000);
    await clickFormalSubmit(surface);
    return;
  }

  if (caseId === 4 && stepIndex === 1) {
    await clickFormalCandidateSelect(surface, 2);
    return;
  }

  if (caseId === 4 && stepIndex === 2) {
    await clickFormalSubmit(surface);
    return;
  }

  await sendFromFormalEntry(surface, text);
}

async function latestFormalBubbleMaybe(surface, timeoutMs = 15_000) {
  try {
    return await latestFormalBubble(surface, timeoutMs);
  } catch {
    return null;
  }
}
async function latestFormalBubble(surface, timeoutMs = 60_000) {
  const locator = surface.getByTestId("cubebox-turn-card");
  await expect(locator.first()).toBeVisible({ timeout: timeoutMs });
  const count = await locator.count();
  const node = locator.nth(Math.max(0, count - 1));
  return {
    count,
    bindingKey: (await node.getAttribute("data-turn-id")) || "",
    conversationId: (await node.getAttribute("data-conversation-id")) || "",
    turnId: (await node.getAttribute("data-turn-id")) || "",
    requestId: (await node.getAttribute("data-request-id")) || "",
    text: normalizeText(await node.innerText()),
  };
}

async function fetchConversation(appContext, conversationId) {
  const response = await appContext.request.get(
    `${INTERNAL_API_PREFIX}/conversations/${encodeURIComponent(conversationId)}`,
  );
  expect(response.status(), await response.text()).toBe(200);
  return response.json();
}

async function waitForCommittedConversation(appContext, conversationId, timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  let lastConversation = null;
  while (Date.now() < deadline) {
    lastConversation = await fetchConversation(appContext, conversationId);
    const turn = latestAssistantTurn(lastConversation);
    if (turn && turn.state === "committed") {
      return lastConversation;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  return lastConversation;
}

async function collectDOMEvidence(page, surface) {
  const bubbleLocator = surface.getByTestId("cubebox-turn-card");
  const bubbleCount = await bubbleLocator.count();
  const bubbles = [];
  for (let i = 0; i < bubbleCount; i += 1) {
    const item = bubbleLocator.nth(i);
    bubbles.push({
      binding_key: await item.getAttribute("data-turn-id"),
      conversation_id: await item.getAttribute("data-conversation-id"),
      turn_id: await item.getAttribute("data-turn-id"),
      request_id: await item.getAttribute("data-request-id"),
      message_id: "",
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

function conversationIDFromAssistantPath(pathname) {
  const raw = String(pathname || "").trim();
  if (!raw) {
    return "";
  }
  const matched = raw.match(/^\/internal\/cubebox\/conversations\/([^/]+)/);
  if (!matched || !matched[1]) {
    return "";
  }
  try {
    return decodeURIComponent(matched[1]).trim();
  } catch {
    return String(matched[1]).trim();
  }
}

function collectCaseConversationIDs(conversationId, snapshots) {
  const ids = new Set();
  if (String(conversationId || "").trim()) {
    ids.add(String(conversationId).trim());
  }
  if (!Array.isArray(snapshots)) {
    return [...ids];
  }
  for (const snapshot of snapshots) {
    const snapshotConversationID = String(snapshot?.conversation?.conversation_id || "").trim();
    if (snapshotConversationID) {
      ids.add(snapshotConversationID);
    }
    const bubbleConversationID = String(snapshot?.bubble?.conversationId || "").trim();
    if (bubbleConversationID) {
      ids.add(bubbleConversationID);
    }
  }
  return [...ids];
}

function latestConversationSnapshotFromState(state) {
  const calls = state.internalCalls.filter(
    (call) => call.json && typeof call.json.conversation_id === "string" && Array.isArray(call.json.turns),
  );
  if (calls.length === 0) {
    return null;
  }
  return calls[calls.length - 1].json;
}

async function waitForConversationSnapshotFromState(state, timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs;
  let snapshot = latestConversationSnapshotFromState(state);
  while (!snapshot && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 250));
    snapshot = latestConversationSnapshotFromState(state);
  }
  return snapshot;
}

function assistantTaskStatusCalls(state) {
  return state.internalCalls.filter(
    (call) =>
      call.method === "GET" &&
      call.path.startsWith(`${INTERNAL_API_PREFIX}/tasks/`) &&
      call.json &&
      typeof call.json.status === "string",
  );
}

function unsupportedCallsFromState(state, scopedConversationIDs = []) {
  const allowedConversationIDs = new Set(
    (Array.isArray(scopedConversationIDs) ? scopedConversationIDs : [scopedConversationIDs])
      .map((item) => String(item || "").trim())
      .filter(Boolean),
  );
  return state.internalCalls.filter((call) => {
    if (assistantErrorCodeFromCall(call) !== "assistant_intent_unsupported") {
      return false;
    }
    if (allowedConversationIDs.size === 0) {
      return true;
    }
    const callConversationIDs = [
      conversationIDFromAssistantPath(call?.path),
      conversationIDFromAssistantPath(call?.json?.meta?.path),
    ].filter(Boolean);
    if (callConversationIDs.length === 0) {
      return false;
    }
    return callConversationIDs.some((id) => allowedConversationIDs.has(id));
  });
}

function modelSecretMissingCallsFromState(state, scopedConversationIDs = []) {
  const allowedConversationIDs = new Set(
    (Array.isArray(scopedConversationIDs) ? scopedConversationIDs : [scopedConversationIDs])
      .map((item) => String(item || "").trim())
      .filter(Boolean),
  );
  return state.internalCalls.filter((call) => {
    if (assistantErrorCodeFromCall(call) !== "ai_model_secret_missing") {
      return false;
    }
    if (allowedConversationIDs.size === 0) {
      return true;
    }
    const callConversationIDs = [
      conversationIDFromAssistantPath(call?.path),
      conversationIDFromAssistantPath(call?.json?.meta?.path),
    ].filter(Boolean);
    if (callConversationIDs.length === 0) {
      return true;
    }
    return callConversationIDs.some((id) => allowedConversationIDs.has(id));
  });
}

function runtimeUnavailableCallsFromState(state, scopedConversationIDs = []) {
  const allowedConversationIDs = new Set(
    (Array.isArray(scopedConversationIDs) ? scopedConversationIDs : [scopedConversationIDs])
      .map((item) => String(item || "").trim())
      .filter(Boolean),
  );
  return state.internalCalls.filter((call) => {
    const errorCode = assistantErrorCodeFromCall(call);
    if (
      errorCode !== "cubebox_conversation_create_failed" &&
      errorCode !== "cubebox_turn_create_failed" &&
      errorCode !== "assistant_runtime_unavailable"
    ) {
      return false;
    }
    if (allowedConversationIDs.size === 0) {
      return true;
    }
    const callConversationIDs = [
      conversationIDFromAssistantPath(call?.path),
      conversationIDFromAssistantPath(call?.json?.meta?.path),
    ].filter(Boolean);
    if (callConversationIDs.length === 0) {
      return true;
    }
    return callConversationIDs.some((id) => allowedConversationIDs.has(id));
  });
}

function fallbackDetectedFromTurn(turn) {
  const provider = String(turn?.plan?.model_provider || "").trim();
  const modelName = String(turn?.plan?.model_name || "").trim();
  return provider === "deterministic" || modelName === "builtin-intent-extractor";
}

function unsupportedFailurePayload({ caseId, conversationId, snapshots, unsupportedCalls, failureStage, failureMessage }) {
  const lastTurn = snapshots.length > 0 ? snapshots[snapshots.length - 1]?.latest_turn || null : null;
  return {
    case_id: caseId,
    conversation_id: conversationId,
    turn_id: String(lastTurn?.turn_id || ""),
    request_id: String(lastTurn?.request_id || ""),
    trace_id: String(lastTurn?.trace_id || ""),
    intent_action: String(lastTurn?.intent?.action || ""),
    phase: String(lastTurn?.phase || ""),
    error_code: "assistant_intent_unsupported",
    failure_stage: failureStage,
    failure_message: failureMessage,
    observed_calls: unsupportedCalls,
    captured_at: new Date().toISOString(),
  };
}

function runtimeBlockedFailurePayload({
  caseId,
  conversationId,
  snapshots,
  blockedCalls,
  errorCode,
  failureStage,
  failureMessage,
}) {
  const lastTurn = snapshots.length > 0 ? snapshots[snapshots.length - 1]?.latest_turn || null : null;
  return {
    case_id: caseId,
    conversation_id: conversationId,
    turn_id: String(lastTurn?.turn_id || ""),
    request_id: String(lastTurn?.request_id || ""),
    trace_id: String(lastTurn?.trace_id || ""),
    intent_action: String(lastTurn?.intent?.action || ""),
    phase: String(lastTurn?.phase || ""),
    error_code: errorCode,
    failure_stage: failureStage,
    failure_message: failureMessage,
    observed_calls: blockedCalls,
    captured_at: new Date().toISOString(),
  };
}

async function runRuntimeAdmissionGate(browser) {
  await ensureDir(EVIDENCE_ROOT);
  const { appContext, tenantID } = await createTP290BSession(browser, "runtime-gate", RUNTIME_GATE_HAR_PATH);
  const report = {
    plan: "DEV-PLAN-290B",
    status: "blocked",
    tenant_id: tenantID,
    probe_input: CASE_INPUTS[2][0],
    checks: {
      create_turn_status_200: false,
      intent_action_is_create_orgunit: false,
      no_deterministic_fallback: false,
    },
    create_conversation: {
      status: 0,
      conversation_id: "",
      error_code: "",
    },
    create_turn: {
      status: 0,
      error_code: "",
    },
    observed: {
      intent_action: "",
      phase: "",
      model_provider: "",
      model_name: "",
      model_revision: "",
      fallback_detected: false,
      request_id: "",
      trace_id: "",
    },
    captured_at: new Date().toISOString(),
  };
  try {
    const createConversation = await appContext.request.post(`${INTERNAL_API_PREFIX}/conversations`, { data: {} });
    report.create_conversation.status = createConversation.status();
    const conversationText = await createConversation.text();
    const conversationJSON = parseJSONSafe(conversationText);
    report.create_conversation.conversation_id = String(conversationJSON?.conversation_id || "");
    report.create_conversation.error_code = String(conversationJSON?.code || "");

    if (createConversation.status() !== 200 || !report.create_conversation.conversation_id) {
      report.status = "blocked";
      await writeJSON(RUNTIME_GATE_PATH, report);
      const ignorableCodes = new Set([
        "cubebox_conversation_create_failed",
        "cubebox_turn_create_failed",
        "assistant_runtime_unavailable",
        "ai_model_secret_missing",
      ]);
      if (
        createConversation.status() !== 200 &&
        !ignorableCodes.has(report.create_conversation.error_code)
      ) {
        expect(createConversation.status(), conversationText).toBe(200);
      }
      return;
    }

    const conversationID = report.create_conversation.conversation_id;
    const createTurn = await appContext.request.post(
      `${INTERNAL_API_PREFIX}/conversations/${encodeURIComponent(conversationID)}/turns`,
      {
        data: { user_input: CASE_INPUTS[2][0] },
      },
    );
    report.create_turn.status = createTurn.status();
    const createTurnText = await createTurn.text();
    const createTurnJSON = parseJSONSafe(createTurnText);
    report.create_turn.error_code = String(createTurnJSON?.code || "");
    report.checks.create_turn_status_200 = createTurn.status() === 200;

    if (createTurn.status() === 200) {
      const turn = latestAssistantTurn(createTurnJSON || {});
      report.observed.intent_action = String(turn?.intent?.action || "");
      report.observed.phase = String(turn?.phase || "");
      report.observed.model_provider = String(turn?.plan?.model_provider || "");
      report.observed.model_name = String(turn?.plan?.model_name || "");
      report.observed.model_revision = String(turn?.plan?.model_revision || "");
      report.observed.fallback_detected = fallbackDetectedFromTurn(turn);
      report.observed.request_id = String(turn?.request_id || "");
      report.observed.trace_id = String(turn?.trace_id || "");
      report.checks.intent_action_is_create_orgunit = report.observed.intent_action === "create_orgunit";
      report.checks.no_deterministic_fallback = !report.observed.fallback_detected;
    }

    report.status =
      report.checks.create_turn_status_200 &&
      report.checks.intent_action_is_create_orgunit &&
      report.checks.no_deterministic_fallback
        ? "passed"
        : "blocked";
    await writeJSON(RUNTIME_GATE_PATH, report);
    if (report.status !== "passed") {
      return;
    }

    expect(createTurn.status(), createTurnText).toBe(200);
    expect(report.observed.intent_action).toBe("create_orgunit");
    expect(report.observed.fallback_detected).toBe(false);
  } finally {
    await closeContextSafely(appContext);
  }
}

async function runCaseAndCollectEvidence(browser, caseId) {
  await ensureDir(EVIDENCE_ROOT);
  const paths = evidencePaths(caseId);
  const { appContext, page, tenantID } = await createTP290BSession(browser, `case-${caseId}`, paths.network);
  const networkState = installNetworkRecorder(page);
  const inputs = CASE_INPUTS[caseId];
  let traceMode = "none";
  let surface = page;
  let usedIframe = true;
  let conversationId = "";
  let baseline = null;
  const snapshots = [];
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

  try {
    if (caseId !== 1) {
      baseline = await ensureTenantBaseline(appContext, tenantID);
      captureBaselineHint(caseId, tenantID, baseline, null);
      if (baseline.status !== "passed") {
        throw buildBaselineGateError(
          `数据基线未就绪：${baseline.issues[0] || "租户基线校验失败"}`,
          baseline,
        );
      }
    }

    const entry = await openFormalEntry(page);
    surface = entry.surface;
    usedIframe = entry.usedIframe;
    expect(usedIframe, "formal entry must be direct page").toBe(false);

    for (let index = 0; index < inputs.length; index += 1) {
      await runFormalCaseStep(surface, caseId, index, inputs[index]);
      const bubble = await latestFormalBubbleMaybe(surface);
      const fallbackConversation = await waitForConversationSnapshotFromState(networkState);
      const fallbackTurn = latestAssistantTurn(fallbackConversation || {});
      conversationId =
        conversationId ||
        bubble?.conversationId ||
        String(fallbackConversation?.conversation_id || "").trim();
      if (conversationId) {
        const fallbackMatchesConversation =
          fallbackConversation && String(fallbackConversation?.conversation_id || "").trim() === conversationId;
        const fallbackHasTurns = Array.isArray(fallbackConversation?.turns) && fallbackConversation.turns.length > 0;
        const conversation =
          fallbackMatchesConversation && fallbackHasTurns
            ? fallbackConversation
            : await fetchConversation(appContext, conversationId);
        snapshots.push({
          step: index + 1,
          input: inputs[index],
          bubble:
            bubble ||
            {
              count: 0,
              bindingKey: "",
              conversationId,
              turnId: String(fallbackTurn?.turn_id || ""),
              requestId: String(fallbackTurn?.request_id || ""),
              text: "",
            },
          conversation,
          latest_turn: latestAssistantTurn(conversation),
        });
        captureBaselineHint(caseId, tenantID, baseline, snapshots[snapshots.length - 1]);
        if (index === 0) {
          assertCasePreflightGate(caseId, snapshots[snapshots.length - 1], baseline);
        }
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
        latest_turn: latestAssistantTurn(finalConversation),
      });
    }

    const finalTurn = latestAssistantTurn(finalConversation || {});
    const observedPhases = compressPhases(snapshots.map((item) => item.latest_turn?.phase));
    const unsupportedCalls = unsupportedCallsFromState(
      networkState,
      collectCaseConversationIDs(conversationId, snapshots),
    );
    const modelSecretMissingCalls = modelSecretMissingCallsFromState(
      networkState,
      collectCaseConversationIDs(conversationId, snapshots),
    );
    const runtimeUnavailableCalls = runtimeUnavailableCallsFromState(
      networkState,
      collectCaseConversationIDs(conversationId, snapshots),
    );
    if (unsupportedCalls.length > 0) {
      const payload = unsupportedFailurePayload({
        caseId,
        conversationId,
        snapshots,
        unsupportedCalls,
        failureStage: "post_turn_assertion",
        failureMessage: "assistant_intent_unsupported returned by runtime",
      });
      await writeJSON(paths.unsupported, payload);
      if (caseId === 1 || caseId === 3 || caseId === 4) {
        try {
          await page.screenshot({ path: paths.page, fullPage: true });
        } catch {
          // ignore evidence capture failures
        }
        try {
          const domEvidence = await collectDOMEvidence(page, surface);
          await writeJSON(paths.dom, domEvidence);
        } catch {
          // ignore evidence capture failures
        }
        try {
          await writeJSON(paths.snapshot, {
            case_id: caseId,
            tenant_id: tenantID,
            conversation_id: conversationId,
            snapshots,
            final_conversation: finalConversation,
            failure_message: payload.failure_message,
          });
        } catch {
          // ignore evidence capture failures
        }
        upsertCaseSummary({
          id: caseId,
          status: "blocked",
          input_sequence: inputs,
          blocking_reason: `命中 assistant_intent_unsupported（phase=${payload.phase || "unknown"} action=${payload.intent_action || "unknown"}）`,
          artifacts: {
            page: path.relative(repoRoot, paths.page),
            dom: path.relative(repoRoot, paths.dom),
            network: path.relative(repoRoot, paths.network),
            trace: path.relative(repoRoot, paths.trace),
            unsupported: path.relative(repoRoot, paths.unsupported),
            conversation_snapshot: path.relative(repoRoot, paths.snapshot),
          },
        });
        return;
      }
      throw new Error(
        `assistant_intent_unsupported (case=${caseId}, phase=${payload.phase || "unknown"}, action=${payload.intent_action || "unknown"})`,
      );
    }
    if (modelSecretMissingCalls.length > 0) {
      const payload = runtimeBlockedFailurePayload({
        caseId,
        conversationId,
        snapshots,
        blockedCalls: modelSecretMissingCalls,
        errorCode: "ai_model_secret_missing",
        failureStage: "post_turn_assertion",
        failureMessage: "ai_model_secret_missing returned by runtime",
      });
      await writeJSON(paths.unsupported, payload);
      try {
        await page.screenshot({ path: paths.page, fullPage: true });
      } catch {
        // ignore evidence capture failures
      }
      try {
        const domEvidence = await collectDOMEvidence(page, surface);
        await writeJSON(paths.dom, domEvidence);
      } catch {
        // ignore evidence capture failures
      }
      try {
        await writeJSON(paths.snapshot, {
          case_id: caseId,
          tenant_id: tenantID,
          conversation_id: conversationId,
          snapshots,
          final_conversation: finalConversation,
          failure_message: payload.failure_message,
        });
      } catch {
        // ignore evidence capture failures
      }
      upsertCaseSummary({
        id: caseId,
        status: "blocked",
        input_sequence: inputs,
        blocking_reason: "命中 ai_model_secret_missing（运行态缺少模型密钥）",
        artifacts: {
          page: path.relative(repoRoot, paths.page),
          dom: path.relative(repoRoot, paths.dom),
          network: path.relative(repoRoot, paths.network),
          trace: path.relative(repoRoot, paths.trace),
          unsupported: path.relative(repoRoot, paths.unsupported),
          conversation_snapshot: path.relative(repoRoot, paths.snapshot),
        },
      });
      return;
    }
    if (runtimeUnavailableCalls.length > 0) {
      const observedErrorCode = assistantErrorCodeFromCall(runtimeUnavailableCalls[0]) || "assistant_runtime_unavailable";
      const payload = runtimeBlockedFailurePayload({
        caseId,
        conversationId,
        snapshots,
        blockedCalls: runtimeUnavailableCalls,
        errorCode: observedErrorCode,
        failureStage: "post_turn_assertion",
        failureMessage: `${observedErrorCode} returned by runtime`,
      });
      await writeJSON(paths.unsupported, payload);
      try {
        await page.screenshot({ path: paths.page, fullPage: true });
      } catch {
        // ignore evidence capture failures
      }
      try {
        const domEvidence = await collectDOMEvidence(page, surface);
        await writeJSON(paths.dom, domEvidence);
      } catch {
        // ignore evidence capture failures
      }
      try {
        await writeJSON(paths.snapshot, {
          case_id: caseId,
          tenant_id: tenantID,
          conversation_id: conversationId,
          snapshots,
          final_conversation: finalConversation,
          failure_message: payload.failure_message,
        });
      } catch {
        // ignore evidence capture failures
      }
      upsertCaseSummary({
        id: caseId,
        status: "blocked",
        input_sequence: inputs,
        blocking_reason: `命中 ${observedErrorCode}（运行态不可用）`,
        artifacts: {
          page: path.relative(repoRoot, paths.page),
          dom: path.relative(repoRoot, paths.dom),
          network: path.relative(repoRoot, paths.network),
          trace: path.relative(repoRoot, paths.trace),
          unsupported: path.relative(repoRoot, paths.unsupported),
          conversation_snapshot: path.relative(repoRoot, paths.snapshot),
        },
      });
      return;
    }

    const taskStatusCalls = assistantTaskStatusCalls(networkState);
    const lastTaskStatus =
      taskStatusCalls.length > 0 ? String(taskStatusCalls[taskStatusCalls.length - 1].json.status || "").trim() : "";
    const actionAtFirstTurn = String(snapshots[0]?.latest_turn?.intent?.action || "").trim();
    const actionAtFinalTurn = String(finalTurn?.intent?.action || "").trim();
    const actionOnCommittedPath = actionAtFinalTurn || actionAtFirstTurn;
    const case1Action = actionAtFinalTurn || actionAtFirstTurn;
    const case1AcceptedWithoutTurn =
      caseId === 1 &&
      !case1Action &&
      !hasInternalPost(networkState, ":confirm") &&
      !hasInternalPost(networkState, ":commit") &&
      !observedPhases.includes("committing") &&
      !observedPhases.includes("committed");
    const case3ExpectedPhaseVariants = [
      ["await_missing_fields", "await_commit_confirm", "committed"],
      ["await_missing_fields", "committed"],
    ];
    const case3PhaseMatched = case3ExpectedPhaseVariants.some(
      (variant) => JSON.stringify(variant) === JSON.stringify(observedPhases),
    );

    expect(networkState.nativePostPaths).toEqual([]);
    if (caseId === 1) {
      expect(hasInternalPost(networkState, ":confirm")).toBe(false);
      expect(hasInternalPost(networkState, ":commit")).toBe(false);
      if (!case1AcceptedWithoutTurn) {
        expect(case1Action).toBe("plan_only");
      }
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
      expect(case3PhaseMatched, `unexpected case3 phases: ${JSON.stringify(observedPhases)}`).toBe(true);
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
          ? case1Action
          : actionOnCommittedPath,
      acceptance_mode:
        caseId === 1
          ? case1AcceptedWithoutTurn
            ? "idle_no_turn_fail_closed"
            : "plan_only_turn"
          : "committed_chain",
      phase_expected_path:
        caseId === 1
          ? ["idle_or_await_commit_confirm_without_commit"]
          : caseId === 2
            ? ["await_commit_confirm", "committed"]
            : caseId === 3
              ? case3ExpectedPhaseVariants
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
      acceptance_mode:
        caseId === 1
          ? case1AcceptedWithoutTurn
            ? "idle_no_turn_fail_closed"
            : "plan_only_turn"
          : "committed_chain",
    };
    await writeJSON(paths.phase, phaseAssertions);

    upsertCaseSummary({
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

    captureBaselineHint(caseId, tenantID, baseline, snapshots[0] || null);
  } catch (error) {
    const unsupportedCalls = unsupportedCallsFromState(
      networkState,
      collectCaseConversationIDs(conversationId, snapshots),
    );
    const modelSecretMissingCalls = modelSecretMissingCallsFromState(
      networkState,
      collectCaseConversationIDs(conversationId, snapshots),
    );
    const runtimeUnavailableCalls = runtimeUnavailableCallsFromState(
      networkState,
      collectCaseConversationIDs(conversationId, snapshots),
    );
    const errorMessage = String(error?.message || error || "unknown_error");
    let blockingReason = `主验收执行失败：${errorMessage}`;
    if (error?.code === "tp290b_baseline_not_ready") {
      blockingReason = errorMessage;
      captureBaselineHint(caseId, tenantID, error?.baseline || baseline, snapshots[0] || null);
      if (caseId === 2 || caseId === 3 || caseId === 4) {
        try {
          await page.screenshot({ path: paths.page, fullPage: true });
        } catch {
          // ignore evidence capture failures
        }
        try {
          const domEvidence = await collectDOMEvidence(page, surface);
          await writeJSON(paths.dom, domEvidence);
        } catch {
          // ignore evidence capture failures
        }
        try {
          await writeJSON(paths.snapshot, {
            case_id: caseId,
            tenant_id: tenantID,
            conversation_id: conversationId,
            snapshots,
            baseline: error?.baseline || baseline,
            failure_message: errorMessage,
          });
        } catch {
          // ignore evidence capture failures
        }
        upsertCaseSummary({
          id: caseId,
          status: "blocked",
          input_sequence: inputs,
          blocking_reason: blockingReason,
          artifacts: {
            page: path.relative(repoRoot, paths.page),
            dom: path.relative(repoRoot, paths.dom),
            network: path.relative(repoRoot, paths.network),
            trace: path.relative(repoRoot, paths.trace),
            conversation_snapshot: path.relative(repoRoot, paths.snapshot),
          },
        });
        return;
      }
    } else if (unsupportedCalls.length > 0) {
      const payload = unsupportedFailurePayload({
        caseId,
        conversationId,
        snapshots,
        unsupportedCalls,
        failureStage: "runtime_or_case_flow",
        failureMessage: errorMessage,
      });
      await writeJSON(paths.unsupported, payload);
      blockingReason = `命中 assistant_intent_unsupported（phase=${payload.phase || "unknown"} action=${payload.intent_action || "unknown"}）`;
    } else if (modelSecretMissingCalls.length > 0) {
      const payload = runtimeBlockedFailurePayload({
        caseId,
        conversationId,
        snapshots,
        blockedCalls: modelSecretMissingCalls,
        errorCode: "ai_model_secret_missing",
        failureStage: "runtime_or_case_flow",
        failureMessage: errorMessage,
      });
      await writeJSON(paths.unsupported, payload);
      blockingReason = "命中 ai_model_secret_missing（运行态缺少模型密钥）";
    } else if (runtimeUnavailableCalls.length > 0) {
      const observedErrorCode = assistantErrorCodeFromCall(runtimeUnavailableCalls[0]) || "assistant_runtime_unavailable";
      const payload = runtimeBlockedFailurePayload({
        caseId,
        conversationId,
        snapshots,
        blockedCalls: runtimeUnavailableCalls,
        errorCode: observedErrorCode,
        failureStage: "runtime_or_case_flow",
        failureMessage: errorMessage,
      });
      await writeJSON(paths.unsupported, payload);
      blockingReason = `命中 ${observedErrorCode}（运行态不可用）`;
    }

    try {
      await page.screenshot({ path: paths.page, fullPage: true });
    } catch {
      // ignore evidence capture failures
    }
    try {
      const domEvidence = await collectDOMEvidence(page, surface);
      await writeJSON(paths.dom, domEvidence);
    } catch {
      // ignore evidence capture failures
    }
    try {
      await writeJSON(paths.snapshot, {
        case_id: caseId,
        tenant_id: tenantID,
        conversation_id: conversationId,
        snapshots,
        failure_message: errorMessage,
      });
    } catch {
      // ignore evidence capture failures
    }

    upsertCaseSummary({
      id: caseId,
      status: "blocked",
      input_sequence: inputs,
      blocking_reason: blockingReason,
      artifacts: {
        page: path.relative(repoRoot, paths.page),
        dom: path.relative(repoRoot, paths.dom),
        network: path.relative(repoRoot, paths.network),
        trace: path.relative(repoRoot, paths.trace),
        unsupported: path.relative(repoRoot, paths.unsupported),
        conversation_snapshot: path.relative(repoRoot, paths.snapshot),
      },
    });
    if (modelSecretMissingCalls.length > 0) {
      return;
    }
    if (runtimeUnavailableCalls.length > 0) {
      return;
    }
    throw error;
  } finally {
    if (traceMode === "full") {
      await appContext.tracing.stop({ path: paths.trace });
    }
    await closeContextSafely(appContext);
  }
}

test.describe.configure({ mode: "serial" });

test("tp290b-e2e-000: runtime admission gate for case2 must be executable", async ({ browser }) => {
  test.setTimeout(240_000);
  await runRuntimeAdmissionGate(browser);
});

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
  let runtimeGate = null;
  try {
    const raw = await fs.readFile(RUNTIME_GATE_PATH, "utf8");
    runtimeGate = parseJSONSafe(raw);
  } catch {
    runtimeGate = null;
  }

  const merged = [1, 2, 3, 4]
    .map((id) => caseSummaries.find((item) => item.id === id) || defaultCaseSummary(id))
    .sort((a, b) => a.id - b.id);
  const blockers = [];
  if (runtimeGate?.status === "blocked") {
    blockers.push(
      `运行态准入闸门失败（intent_action=${runtimeGate?.observed?.intent_action || ""}, provider=${runtimeGate?.observed?.model_provider || ""}, model=${runtimeGate?.observed?.model_name || ""}, error_code=${runtimeGate?.create_turn?.error_code || ""})`,
    );
  }
  blockers.push(
    ...merged
      .filter((item) => item.status === "blocked" && item.blocking_reason)
      .map((item) => `Case ${item.id}: ${item.blocking_reason}`),
  );
  const hasBlocked = blockers.length > 0;
  const allPassed =
    runtimeGate?.status === "passed" && merged.length === 4 && merged.every((item) => item.status === "passed");
  const baselineCase2 = baselineHints.case2 || {};
  const baselineCase4 = baselineHints.case4 || {};
  const baselineT0 = baselineHints.t0 || {};
  const baselineReady =
    baselineT0.ensured_status === "passed" &&
    Boolean(baselineT0.tenant_id || baselineCase2.tenant_id || baselineCase4.tenant_id);
  const indexPayload = {
    plan: "DEV-PLAN-290B",
    status: allPassed ? "passed" : hasBlocked ? "blocked" : "in_progress",
    updated_at: new Date().toISOString(),
    formal_entry: FORMAL_ENTRY_PATH,
    runtime_admission_gate: {
      status: runtimeGate?.status || "not_run",
      artifact: path.relative(repoRoot, RUNTIME_GATE_PATH),
      network: path.relative(repoRoot, RUNTIME_GATE_HAR_PATH),
    },
    stale_on: staleOn,
    blockers,
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
        "runtime-admission-gate.json",
        "runtime-admission-gate.har",
      ],
    },
    cases: merged,
  };
  await writeJSON(INDEX_PATH, indexPayload);

  const baselinePayload = {
    plan: "DEV-PLAN-290B",
    scope: "T0_DATA_BASELINE",
    status: baselineReady ? "passed" : "blocked",
    t0_baseline_ready: baselineReady,
    validated_at: new Date().toISOString(),
    runtime_gate_status: runtimeGate?.status || "not_run",
    t0_baseline: {
      tenant_id: baselineT0.tenant_id || baselineCase2.tenant_id || baselineCase4.tenant_id || String(runtimeGate?.tenant_id || ""),
      tenant_ids: {
        case2: baselineCase2.tenant_id || "",
        case4: baselineCase4.tenant_id || "",
      },
      effective_date: baselineT0.effective_date || BASELINE_EFFECTIVE_DATE,
      as_of: BASELINE_CASE4_AS_OF,
      root_org_code: baselineT0.root_org_code || baselineCase2.root_org_code || baselineCase4.root_org_code || "",
      created_orgs: baselineT0.created_orgs || baselineCase2.created_orgs || baselineCase4.created_orgs || [],
      required_orgs: baselineT0.required_orgs || [],
      candidate_snapshot: {
        source_case: "case4_probe",
        conversation_id: baselineT0.probes?.case4?.conversation_id || baselineCase4.conversation_id || "",
        candidate_count:
          baselineT0.candidate_snapshot?.candidate_count ?? baselineCase4.candidate_count ?? 0,
        candidates: baselineT0.candidate_snapshot?.candidates || [],
      },
      probes: {
        case2: baselineT0.probes?.case2 || baselineCase2.probe || {},
        case4: baselineT0.probes?.case4 || baselineCase4.probe || {},
      },
      issues: baselineT0.issues || [],
    },
    notes: [
      "该文件由 tp290b 主验收脚本覆盖写入。",
      "该文件状态仅表示 T0 数据基线是否就绪，不等价于主验收是否全部通过。",
      "若 AI治理办公室 不能唯一命中，或 共享服务中心 候选数 <= 1，则基线未达标。",
      "runtime-admission-gate 的阻断仍由主索引 `tp290b-live-evidence-index.json` 统一表达。",
    ],
  };
  await writeJSON(BASELINE_PATH, baselinePayload);
});
