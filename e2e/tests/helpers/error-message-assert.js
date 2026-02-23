import { expect } from "@playwright/test";

function isGenericErrorMessage(code, message) {
  const normalizedCode = String(code ?? "").trim().toLowerCase();
  const normalizedMessage = String(message ?? "").trim().toLowerCase();
  if (normalizedMessage.length === 0) {
    return true;
  }
  if (normalizedCode.length > 0 && normalizedCode === normalizedMessage) {
    return true;
  }
  if (/^[a-z0-9_]+_failed$/.test(normalizedMessage)) {
    return true;
  }
  if (/^[a-z]+(?: [a-z]+){0,2} failed$/.test(normalizedMessage)) {
    return true;
  }
  if (normalizedMessage === "invalid_request" || normalizedMessage === "request failed.") {
    return true;
  }
  return false;
}

export async function expectExplicitError(resp, { status, code } = {}) {
  if (typeof status === "number") {
    expect(resp.status(), await resp.text()).toBe(status);
  }
  const payload = await resp.json();
  if (typeof code === "string" && code.length > 0) {
    expect(payload.code).toBe(code);
  }
  expect(typeof payload.message).toBe("string");
  expect(isGenericErrorMessage(payload.code, payload.message)).toBe(false);
  return payload;
}
