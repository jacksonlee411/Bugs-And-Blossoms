import { expect, test } from "@playwright/test";

import { setupTenantAdminSession } from "./helpers/superadmin-tenant.js";

test("smoke: superadmin -> create tenant -> /app (MUI SPA) -> org/person/staffing vertical slice", async ({ browser }) => {
  test.setTimeout(240_000);

  const asOf = "2026-01-07";
  const runID = `${Date.now()}`;
  const tenantHost = `t-${runID}.localhost`;
  const tenantName = `E2E Tenant ${runID}`;

  const tenantAdminEmail = `tenant-admin+smoke-${runID}@example.invalid`;
  const tenantAdminPass = process.env.E2E_TENANT_ADMIN_PASS || "pw";

  const pernr = `${Math.floor(10000000 + Math.random() * 90000000)}`;
  const orgName = `E2E OrgUnit ${runID}`;
  const orgCode = `ORG${runID.slice(-6)}`;
  const posName = `E2E Position ${runID}`;

  const superadminPass = process.env.E2E_SUPERADMIN_PASS || "admin";
  const defaultSuperadminEmail = `admin+smoke-${runID}@example.invalid`;
  const superadminEmail = process.env.E2E_SUPERADMIN_EMAIL || defaultSuperadminEmail;
  const superadminLoginPass = process.env.E2E_SUPERADMIN_LOGIN_PASS || superadminPass;
  const { appContext, tenantID } = await setupTenantAdminSession(browser, {
    tenantName,
    tenantHost,
    tenantAdminEmail,
    tenantAdminPass,
    superadminEmail,
    superadminLoginPass,
    superadminHeadingText: "SuperAdmin Login"
  });
  const page = await appContext.newPage();

  // No Legacy: /login HTML route is removed; MUI login is under /app/login + JSON API.
  const loginGetResp = await appContext.request.get("/login");
  expect(loginGetResp.status()).toBe(404);

  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: tenantAdminEmail, password: tenantAdminPass }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);

  await page.goto(`/app?as_of=${asOf}`);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");

  // OrgUnit: create a dedicated BU root for this smoke run, then bind SetID for jobcatalog resolution.
  const createOrgResp = await appContext.request.post("/org/api/org-units", {
    data: {
      org_code: orgCode,
      name: orgName,
      effective_date: asOf,
      parent_org_code: "",
      is_business_unit: true
    }
  });
  expect(createOrgResp.status(), await createOrgResp.text()).toBe(201);

  const bindResp = await appContext.request.post("/org/api/setid-bindings", {
    data: {
      org_code: orgCode,
      setid: "DEFLT",
      effective_date: asOf,
      request_id: `smoke-bind-root-${runID}`
    }
  });
  expect(bindResp.status(), await bindResp.text()).toBe(201);

  // JobCatalog: use JSON API (MUI-only) rather than legacy /org/job-catalog HTML routes.
  const jobFamilyGroupCode = `JFG-SM-${runID}`;
  const jobFamilyCode = `JF-SM-${runID}`;
  const jobProfileCode = `JP-SM-${runID}`;

  const createJobCatalogItem = async (action, body) => {
    const resp = await appContext.request.post("/jobcatalog/api/catalog/actions", {
      data: {
        package_code: "DEFLT",
        effective_date: asOf,
        action,
        ...body
      }
    });
    expect(resp.status(), await resp.text()).toBe(201);
  };

  await createJobCatalogItem("create_job_family_group", {
    code: jobFamilyGroupCode,
    name: `Smoke Group ${runID}`
  });
  await createJobCatalogItem("create_job_family", {
    code: jobFamilyCode,
    name: `Smoke Family ${runID}`,
    group_code: jobFamilyGroupCode
  });
  await createJobCatalogItem("create_job_profile", {
    code: jobProfileCode,
    name: `Smoke Profile ${runID}`,
    family_codes_csv: jobFamilyCode,
    primary_family_code: jobFamilyCode
  });

  // Person: create via JSON API.
  const createPersonResp = await appContext.request.post("/person/api/persons", {
    data: { pernr, display_name: `E2E Person ${runID}` }
  });
  expect(createPersonResp.status(), await createPersonResp.text()).toBe(201);
  const createdPerson = await createPersonResp.json();
  const personUUID = String(createdPerson.person_uuid || "");
  expect(personUUID).not.toBe("");

  // Staffing: resolve Job Profile UUID via options API, then create Position and Assignment via JSON API.
  const optionsResp = await appContext.request.get(
    `/org/api/positions:options?as_of=${encodeURIComponent(asOf)}&org_code=${encodeURIComponent(orgCode)}`
  );
  expect(optionsResp.status(), await optionsResp.text()).toBe(200);
  const options = await optionsResp.json();
  const jobProfileOpt = (options.job_profiles || []).find((p) => p.job_profile_code === jobProfileCode);
  expect(jobProfileOpt && jobProfileOpt.job_profile_uuid).toBeTruthy();

  const createPosResp = await appContext.request.post(`/org/api/positions?as_of=${encodeURIComponent(asOf)}`, {
    data: {
      effective_date: asOf,
      org_code: orgCode,
      job_profile_uuid: jobProfileOpt.job_profile_uuid,
      name: posName
    }
  });
  expect(createPosResp.status(), await createPosResp.text()).toBe(200);
  const createdPos = await createPosResp.json();
  const positionUUID = String(createdPos.position_uuid || "");
  expect(positionUUID).not.toBe("");

  const createAssignmentResp = await appContext.request.post(`/org/api/assignments?as_of=${encodeURIComponent(asOf)}`, {
    data: {
      effective_date: asOf,
      person_uuid: personUUID,
      position_uuid: positionUUID,
      allocated_fte: "1.0"
    }
  });
  expect(createAssignmentResp.status(), await createAssignmentResp.text()).toBe(200);

  const listAssignmentsResp = await appContext.request.get(
    `/org/api/assignments?as_of=${encodeURIComponent(asOf)}&person_uuid=${encodeURIComponent(personUUID)}`
  );
  expect(listAssignmentsResp.status(), await listAssignmentsResp.text()).toBe(200);
  const listAssignmentsJSON = await listAssignmentsResp.json();
  expect(Array.isArray(listAssignmentsJSON.assignments)).toBeTruthy();
  expect(listAssignmentsJSON.assignments.length).toBeGreaterThan(0);

  // UI sanity checks (MUI-only pages)
  await page.goto(`/app/org/units?as_of=${asOf}`);
  await expect(page.locator("h1")).toContainText("Bugs & Blossoms");
  await page.goto(`/app/jobcatalog?as_of=${asOf}&package_code=DEFLT`);
  await expect(page.getByRole("heading", { level: 2, name: "Job Catalog" })).toBeVisible();
  await page.goto(`/app/staffing/positions?as_of=${asOf}&org_code=${orgCode}`);
  await expect(page.getByRole("heading", { level: 2, name: "Staffing / Positions" })).toBeVisible();
  await page.goto(`/app/staffing/assignments?as_of=${asOf}&pernr=${pernr}`);
  await expect(page.getByRole("heading", { level: 2, name: "Staffing / Assignments" })).toBeVisible();

  await appContext.close();
});
