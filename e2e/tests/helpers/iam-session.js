import { expect } from "@playwright/test"

function parseJSONSafe(raw) {
  const body = String(raw || "").trim()
  if (!body) {
    return null
  }
  try {
    return JSON.parse(body)
  } catch {
    return null
  }
}

export async function createIAMSession(appContext, email, password) {
  const resp = await appContext.request.post("/iam/api/sessions", {
    data: { email, password }
  })
  expect(resp.status(), await resp.text()).toBe(204)
}

export async function createIAMSessionWithRetry(appContext, email, password, timeoutMs = 15_000) {
  const deadline = Date.now() + timeoutMs
  let lastStatus = 0
  let lastBody = ""

  while (Date.now() < deadline) {
    const resp = await appContext.request.post("/iam/api/sessions", {
      data: { email, password }
    })
    lastStatus = resp.status()
    lastBody = await resp.text()
    if (lastStatus === 204) {
      return
    }

    const parsed = parseJSONSafe(lastBody)
    const code = String(parsed?.code || "").trim()
    if (!(lastStatus === 422 && code === "invalid_credentials")) {
      break
    }
    await new Promise((resolve) => setTimeout(resolve, 500))
  }

  expect(lastStatus, lastBody).toBe(204)
}
