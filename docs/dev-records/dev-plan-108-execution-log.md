# DEV-PLAN-108 执行日志

> 说明：本文件记录 DEV-PLAN-108 的实施证据（命令、门禁结果、关键决策点），作为 readiness/回溯依据。

## 2026-02-18（UTC）

- 2026-02-18：开工，确认以 `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md` 为 SSOT；允许先建执行日志再实现。
- 2026-02-18：门禁（实施中/收口后）：
  - Go：
    - `go vet ./...`
    - `make check lint`
    - `make test`（100% coverage policy）
    - `make check routing`
    - `make check request-code`
    - `make check no-legacy`
    - `make check doc`
  - Web（apps/web）：
    - `pnpm -C apps/web lint`
    - `pnpm -C apps/web typecheck`
    - `pnpm -C apps/web test`
  - E2E（尝试）：
    - `make e2e`：最初失败（`ORGUNIT_CODES_WRITE_FORBIDDEN` + Playwright “Internal error: step id not found: fixture@..” 等），后续已修复并通过（见“补丁/收口”）。

## 补丁/收口（E2E 修复）

- 2026-02-18：
  - 修复 1：`migrations/orgunit/20260218123000_orgunit_update_event_108.sql` 中 `CREATE OR REPLACE` 覆盖了 `submit_org_event(...)` 的 `SECURITY DEFINER/OWNER/search_path` 属性，导致写入 `org_unit_codes` 触发 `ORGUNIT_CODES_WRITE_FORBIDDEN`。
    - 通过新增迁移修复：`migrations/orgunit/20260218161000_orgunit_update_event_108_kernel_privileges_reapply.sql`（并更新 `migrations/orgunit/atlas.sum`）。
  - 修复 2：`orgunit.org_events_effective_for_replay(...)` 将“被更正的 CREATE（仅 ext patch）”重写为 `UPDATE`，replay 时跳过 `apply_create_logic`，从而在更正链路触发 `ORG_NOT_FOUND_AS_OF`。
    - 通过新增迁移修复：`migrations/orgunit/20260218172000_orgunit_correction_create_event_replay_fix.sql`（并同步更新 schema SSOT：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`，更新 `migrations/orgunit/atlas.sum`）。
  - 验证：`make e2e` 通过（7/7 tests passed）。

## 变更清单（待实施后补齐）

- [x] DB：引入 `UPDATE` 事件类型；day-slot 守卫纳入 `UPDATE`；replay/correction 解释对齐 108。（迁移：`migrations/orgunit/20260218123000_orgunit_update_event_108.sql`）
- [x] API：新增 `POST /org/api/org-units/write` 与 `GET /org/api/org-units/write-capabilities`。
- [x] Services：新增 `Write(...)`（统一写核心）+ 新增 `write-capabilities` 策略收敛（intent 维度）。
- [x] Web：统一 patch 构造工具；`OrgUnitDetailsPage`/`OrgUnitsPage` 接入统一 write 与 write-capabilities；删除按版本数自动分流到 `rescinds` / `rescinds/org`。
- [x] Tests：Go/API/FE 单测补齐并通过 `make test`；E2E 已尝试但当前环境失败（见上）。
