const { chromium } = require("@playwright/test");

(async () => {
  const asOf = new Date().toISOString().slice(0, 10);
  const baseURL = "http://localhost:8080";
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ baseURL });
  const req = context.request;

  const loginResp = await req.post("/iam/api/sessions", {
    data: { email: "admin@localhost", password: "admin123" }
  });
  if (loginResp.status() !== 204) {
    throw new Error(`login failed: ${loginResp.status()} ${await loginResp.text()}`);
  }

  const dictFields = ["d_org_type", "d_test01", "d_test02"];
  const optionsByField = {};
  for (const fieldKey of dictFields) {
    const resp = await req.get(`/org/api/org-units/fields:options?field_key=${encodeURIComponent(fieldKey)}&as_of=${encodeURIComponent(asOf)}`);
    if (!resp.ok()) throw new Error(`options failed for ${fieldKey}: ${resp.status()} ${await resp.text()}`);
    const json = await resp.json();
    if (!Array.isArray(json.options) || json.options.length === 0) {
      throw new Error(`no options for ${fieldKey}`);
    }
    optionsByField[fieldKey] = json.options[0];
  }

  const page = await context.newPage();
  await page.goto("/app/org/units?page=0&node=1");
  await page.getByRole("button", { name: /新建|Create/i }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.waitFor({ state: "visible", timeout: 10000 });

  const name = `108B-UI-${Date.now()}`;
  await dialog.getByLabel(/名称|Name/i).first().fill(name);

  const fieldLabels = {
    d_org_type: /org_type/i,
    d_test01: /字典字段01/i,
    d_test02: /测试02/i
  };

  for (const fieldKey of dictFields) {
    const input = dialog.getByLabel(fieldLabels[fieldKey]).first();
    await input.click();
    await input.fill(optionsByField[fieldKey].label);
    await page.getByRole("option", { name: optionsByField[fieldKey].label }).first().click();
  }

  await dialog.getByRole("button", { name: /确认|Confirm/i }).first().click();

  await page.waitForTimeout(1500);
  const toast = page.getByText(/操作已完成|done|success/i).first();
  const hasToast = await toast.isVisible().catch(() => false);

  const searchResp = await req.get(`/org/api/org-units/search?query=${encodeURIComponent(name)}&as_of=${encodeURIComponent(asOf)}`);
  const searchJson = await searchResp.json();
  const hitCode = typeof searchJson.target_org_code === "string" ? searchJson.target_org_code : null;

  console.log(JSON.stringify({
    asOf,
    name,
    selectedOptions: optionsByField,
    toastVisible: hasToast,
    createdOrgCode: hitCode
  }, null, 2));

  await browser.close();

  if (!hitCode) {
    process.exit(2);
  }
})();
