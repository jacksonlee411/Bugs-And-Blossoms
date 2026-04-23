const fs = require("fs/promises");
const path = require("path");
const { chromium } = require("@playwright/test");

async function main() {
  const baseURL = process.env.E2E_BASE_URL || "http://localhost:8080";
  const artifactDir = path.join(__dirname, "_artifacts");
  await fs.mkdir(artifactDir, { recursive: true });

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ baseURL, viewport: { width: 1440, height: 1200 } });
  const page = await context.newPage();

  try {
    await page.goto("/app/login", { waitUntil: "networkidle" });
    await page.getByLabel(/邮箱|Email/i).fill("admin@localhost");
    await page.getByLabel(/密码|Password/i).fill("admin123");
    await page.getByRole("button", { name: /登录|Login/i }).click();
    await page.waitForURL(/\/app(?:\?.*)?$/, { timeout: 15000 });

    await page.getByRole("button", { name: /打开 CubeBox 抽屉|Open CubeBox Drawer/i }).click();
    await page.getByRole("complementary", { name: /CubeBox/i }).waitFor({ state: "visible", timeout: 10000 });
    const prompt = "请严格原样输出以下内容，不要添加解释：\n1) AAA\n\n2) BBB\n\n3) CCC";
    let lastPayload = null;

    for (let attempt = 1; attempt <= 3; attempt++) {
      await page.getByRole("button", { name: /新建对话|New Chat/i }).click();
      await page.waitForTimeout(400);
      await page.getByLabel(/输入消息|Message/i).fill(prompt);
      await page.getByRole("button", { name: /发送|Send/i }).click();

      await page.waitForFunction(() => {
        const items = Array.from(document.querySelectorAll("li"));
        return items.some((item) => {
          const label = item.querySelector("span");
          const content = item.querySelector("p");
          return (label?.textContent || "").includes("CubeBox") && (content?.textContent || "").includes("1)");
        });
      }, null, { timeout: 30000 });

      await page.waitForFunction(() => {
        const items = Array.from(document.querySelectorAll("li"));
        const assistantItems = items.filter((item) => (item.querySelector("span")?.textContent || "").includes("CubeBox"));
        if (assistantItems.length === 0) {
          return false;
        }
        const last = assistantItems[assistantItems.length - 1];
        const spans = Array.from(last.querySelectorAll("span"));
        const status = spans.length > 1 ? spans[1].textContent || "" : "";
        return status.includes("已完成") || status.includes("completed") || status.includes("失败") || status.includes("error");
      }, null, { timeout: 30000 }).catch(() => {});

      await page.waitForTimeout(1200);

      const timelineTexts = await page.locator("li").evaluateAll((nodes) =>
        nodes.map((node) => {
          const spans = Array.from(node.querySelectorAll("span"));
          const labelNode = spans[0] || null;
          const statusNode = spans[1] || null;
          const contentNode = node.querySelector("p");
          return {
            label: labelNode?.textContent || "",
            text: contentNode?.textContent || "",
            status: statusNode?.textContent || "",
            whiteSpace: contentNode ? window.getComputedStyle(contentNode).whiteSpace : window.getComputedStyle(node).whiteSpace
          };
        })
      );

      const assistantItems = timelineTexts.filter((item) => item.label.includes("CubeBox"));
      const target = assistantItems[assistantItems.length - 1] || null;
      lastPayload = {
        executed_at: new Date().toISOString(),
        baseURL,
        prompt,
        attempt,
        target,
        timelineTexts
      };

      if (target && target.status.includes("已完成")) {
        const hasOption2Standalone =
          target.text.includes("\n\n2) BBB\n\n3) CCC") ||
          target.text.includes("\n\n2) BBB") ||
          /1\) AAA\s+2\) BBB\s+3\) CCC/s.test(target.text);
        lastPayload.hasOption2Standalone = hasOption2Standalone;
        if (hasOption2Standalone) {
          break;
        }
      }
    }

    const screenshotPath = path.join(artifactDir, "cubebox-newline-live-validation.png");
    await page.screenshot({ path: screenshotPath, fullPage: true });

    const resultPath = path.join(artifactDir, "cubebox-newline-live-validation.json");
    await fs.writeFile(resultPath, JSON.stringify(lastPayload, null, 2), "utf8");

    if (!lastPayload || !lastPayload.hasOption2Standalone) {
      throw new Error(`option 2 standalone check failed: ${JSON.stringify(lastPayload, null, 2)}`);
    }
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
