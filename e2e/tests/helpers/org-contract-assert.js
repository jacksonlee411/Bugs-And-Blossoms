import { expect } from "@playwright/test";

export const forbiddenLegacyOrgFields = ["org_id", "org_node_key", "org_unit_id"];
export const legacyOrgFieldPattern = /\b(org_id|org_node_key|org_unit_id)\b/i;

export function collectForbiddenOrgFieldPaths(value, path = "$", forbidden = forbiddenLegacyOrgFields) {
  const hits = [];
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      hits.push(...collectForbiddenOrgFieldPaths(item, `${path}[${index}]`, forbidden));
    });
    return hits;
  }
  if (!value || typeof value !== "object") {
    return hits;
  }
  for (const [key, nested] of Object.entries(value)) {
    const nextPath = `${path}.${key}`;
    if (forbidden.includes(String(key || "").trim())) {
      hits.push(nextPath);
    }
    hits.push(...collectForbiddenOrgFieldPaths(nested, nextPath, forbidden));
  }
  return hits;
}

export function expectNoLegacyOrgFields(payload, label) {
  const hits = collectForbiddenOrgFieldPaths(payload);
  expect(hits, `${label} unexpectedly exposed legacy org fields`).toEqual([]);
}
