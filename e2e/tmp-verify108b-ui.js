const { chromium } = require("@playwright/test");

(async () => {
  const baseURL = "http://localhost:8080";
  const asOf = new Date().toISOString().slice(0, 10);
  const orgCode = `O9${Date.now().toString().slice(-5)}`;
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ baseURL });
  const req = context.request;

  const loginResp = await req.post("/iam/api/sessions", { data: { email: "admin@localhost", password: "admin123" } });
  if (loginResp.status() !== 204) {
    throw new Error(`login failed: ${loginResp.status()} ${await loginResp.text()}`);
  }

  const cfgResp = await req.get(`/org/api/org-units/field-configs?as_of=${encodeURIComponent(asOf)}&status=enabled`);
  if (!cfgResp.ok()) throw new Error(`field-configs failed: ${cfgResp.status()} ${await cfgResp.text()}`);
  const cfgJson = await cfgResp.json();
  const dictFieldLabels = (cfgJson.field_configs || [])
    .filter((f) => String(f.data_source_type || "").toUpperCase() === "DICT")
    .map((f) => {
      const key = (f.label_i18n_key || "").trim();
      const label = (f.label || "").trim();
      return { key: f.field_key, label: key ? key : (label || f.field_key) };
    });

  const capResp = await req.get(
    `/org/api/org-units/write-capabilities?intent=create_org&org_code=${encodeURIComponent(orgCode)}&effective_date=${encodeURIComponent(asOf)}`
  );
  if (!capResp.ok()) throw new Error(`write-capabilities failed: ${capResp.status()} ${await capResp.text()}`);
  const capJson = await capResp.json();
  const allowed = new Set(capJson.allowed_fields || []);
  const expected = dictFieldLabels.filter((f) => allowed.has(f.key));

  const page = await context.newPage();
  await page.goto(`/app/org/units?page=0&node=1`);
  await page.getByRole("button", { name: /新建|Create/i }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.waitFor({ state: "visible", timeout: 10000 });
  const orgCodeInput = dialog.getByLabel(/编码|Code/i).first();
  const orgCodeDisabled = await orgCodeInput.isDisabled();
  if (!orgCodeDisabled) {
    await orgCodeInput.fill(orgCode);
  }
  await page.waitForTimeout(1200);

  const labelTexts = await dialog.locator("label").allTextContents();
  const normalized = labelTexts.map((s) => s.trim()).filter(Boolean);

  const missing = expected.filter((f) => !normalized.some((txt) => txt.includes(f.label) || txt.includes(f.key)));
  const inputRoles = [];
  for (const field of expected) {
    const candidate = normalized.find((txt) => txt.includes(field.label) || txt.includes(field.key));
    if (!candidate) {
      inputRoles.push({ key: field.key, label: field.label, role: null });
      continue;
    }
    const role = await dialog.getByLabel(candidate).first().getAttribute("role");
    inputRoles.push({ key: field.key, label: field.label, role });
  }
  const nonCombobox = inputRoles.filter((item) => item.role !== "combobox");

  console.log(JSON.stringify({
    asOf,
    orgCode,
    orgCodeDisabled,
    expectedDictFields: expected,
    dialogLabelCount: normalized.length,
    missing,
    inputRoles,
    nonCombobox
  }, null, 2));

  if (missing.length > 0 || nonCombobox.length > 0) {
    throw new Error(`missing dict fields in create dialog: ${missing.map((m) => m.key).join(", ")}`);
  }

  await browser.close();
})();
