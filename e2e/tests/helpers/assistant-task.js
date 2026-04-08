import { expect } from "@playwright/test"

export async function pollAssistantTask(
  appContext,
  taskID,
  timeoutMs,
  {
    intervalMs = 500,
    terminalStatuses = ["succeeded", "failed", "manual_takeover_required", "canceled"]
  } = {},
) {
  const startAt = Date.now()
  const terminal = new Set(terminalStatuses)
  const statuses = []

  while (Date.now() - startAt < timeoutMs) {
    const resp = await appContext.request.get(`/internal/assistant/tasks/${encodeURIComponent(taskID)}`)
    expect(resp.status(), await resp.text()).toBe(200)
    const detail = await resp.json()
    statuses.push({
      status: detail.status || "",
      dispatch_status: detail.dispatch_status || "",
      last_error_code: detail.last_error_code || "",
      updated_at: detail.updated_at || ""
    })
    if (terminal.has(detail.status)) {
      return {
        timed_out: false,
        terminal_status: detail.status || "",
        statuses
      }
    }
    await new Promise((resolve) => setTimeout(resolve, intervalMs))
  }

  return {
    timed_out: true,
    terminal_status: "",
    statuses
  }
}
