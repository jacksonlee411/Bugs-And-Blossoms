import { test } from "@playwright/test";

test.describe("tp284-prep: librechat send/render takeover skeleton", () => {
  test.fixme(
    "tp284-e2e-001: single send channel only",
    "Enable after DEV-PLAN-283 is completed and 284 implementation starts."
  );

  test.fixme(
    "tp284-e2e-002: frontend consumes backend DTO without local phase recompute",
    "Enable after DEV-PLAN-223/260 DTO contract freeze."
  );

  test.fixme(
    "tp284-e2e-003: all business receipts render in official message tree",
    "Enable after source-level render takeover patch lands."
  );

  test.fixme(
    "tp284-e2e-004: legacy helper no longer owns formal business progression",
    "Enable after helper-role retirement patch lands."
  );
});

