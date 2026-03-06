import { expect, test } from "@playwright/test";

async function setupTenantAdminSession(browser) {
  const appBaseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const appContext = await browser.newContext({
    baseURL: appBaseURL,
    extraHTTPHeaders: { "X-Forwarded-Host": "localhost" }
  });
  const loginResp = await appContext.request.post("/iam/api/sessions", {
    data: { email: "admin@localhost", password: "admin123" }
  });
  expect(loginResp.status(), await loginResp.text()).toBe(204);
  const page = await appContext.newPage();
  return { appContext, page };
}

function frameHTML({ delayedRoot }) {
  const delayedScript = delayedRoot
    ? `
  <script>
    window.setTimeout(function () {
      var chat = document.createElement('div');
      chat.setAttribute('data-testid', 'chat-container');
      var log = document.createElement('div');
      log.setAttribute('role', 'log');
      chat.appendChild(log);
      document.body.appendChild(chat);
    }, 600);
  </script>
`
    : `
  <div data-testid="chat-container">
    <div role="log"></div>
  </div>
`;
  return `<!doctype html>
<html>
  <head><meta charset="utf-8" /></head>
  <body>
${delayedScript}
    <script src="/assistant-ui/bridge.js"></script>
  </body>
</html>`;
}

async function sendDialogMessage(page, channel, nonce) {
  await page.evaluate(
    ({ c, n }) => {
      const frame = document.querySelector("#tp262-frame");
      if (!frame || !frame.contentWindow) {
        return;
      }
      frame.contentWindow.postMessage(
        {
          type: "assistant.flow.dialog",
          channel: c,
          nonce: n,
          payload: {
            message_id: "tp262_dialog_probe",
            kind: "info",
            stage: "draft",
            text: "tp262 anchor probe",
            meta: {}
          }
        },
        window.location.origin
      );
    },
    { c: channel, n: nonce }
  );
}

test("tp262-e2e-001: dialog stream mounts in chat root, never under body", async ({ browser }) => {
  const { appContext, page } = await setupTenantAdminSession(browser);

  await page.route("**/tp262-frame-ready.html**", async (route) => {
    await route.fulfill({ status: 200, contentType: "text/html", body: frameHTML({ delayedRoot: false }) });
  });
  await page.route("**/tp262-frame-delayed.html**", async (route) => {
    await route.fulfill({ status: 200, contentType: "text/html", body: frameHTML({ delayedRoot: true }) });
  });

  const readyChannel = "tp262_ready_channel";
  const readyNonce = "tp262_ready_nonce";
  await page.goto("/app/login");
  await page.setContent(
    `<iframe id="tp262-frame" src="/tp262-frame-ready.html?channel=${readyChannel}&nonce=${readyNonce}" style="width:1000px;height:800px;border:0"></iframe>`
  );
  await page.waitForTimeout(1000);
  await sendDialogMessage(page, readyChannel, readyNonce);
  await expect
    .poll(async () => await page.frameLocator("#tp262-frame").locator('[role="log"] [data-assistant-dialog-stream="1"]').count())
    .toBe(1);
  await expect(page.frameLocator("#tp262-frame").locator('body > [data-assistant-dialog-stream="1"]')).toHaveCount(0);

  const delayedChannel = "tp262_delayed_channel";
  const delayedNonce = "tp262_delayed_nonce";
  await page.setContent(
    `<iframe id="tp262-frame" src="/tp262-frame-delayed.html?channel=${delayedChannel}&nonce=${delayedNonce}" style="width:1000px;height:800px;border:0"></iframe>`
  );
  await page.waitForTimeout(200);
  await sendDialogMessage(page, delayedChannel, delayedNonce);
  await expect
    .poll(async () => await page.frameLocator("#tp262-frame").locator('[role="log"] [data-assistant-dialog-stream="1"]').count())
    .toBe(1);
  await expect(page.frameLocator("#tp262-frame").locator('body > [data-assistant-dialog-stream="1"]')).toHaveCount(0);

  await appContext.close();
});
