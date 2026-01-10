# DEV-PLAN-055 记录：考勤 Slice 4E——纠错与审计闭环（更正事件 + 重算）执行日志

**状态**: 已完成（2026-01-09）

> 本日志用于跟踪 `docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md` 的实施拆分（§8.2），确保每个 PR 可独立验收、门禁对齐、并在合并后回填完成情况。

## PR-1：Milestone 1 — 文档确认 + 新增表批准记录固化

- **状态**：已完成（2026-01-09）
- **PR**：#156（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/156）
- **Merge Commit**：`0a271217359d4e2718abdad1b684eb169f31b25e`
- **合并时间**：2026-01-09T18:05:38Z
- **范围**
  - 按 DEV-PLAN-055 §8.2.1 固化：切片范围/不变量确认。
  - 在计划文档 §4.1.0 登记“新增表已获手工批准”（用于后续迁移落地）。
- **门禁**
  - `make check doc`

## PR-2：Milestone 2 — 路由（UI detail POST + internal_api 端点）

- **状态**：已完成（2026-01-09）
- **PR**：#157（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/157）
- **Merge Commit**：`7cd88bf2590c4999f1adc8af10ccd913fd20aa87`
- **合并时间**：2026-01-09T18:12:03Z
- **范围**
  - allowlist：为日结果详情页增加 POST；新增 internal_api 端点（void/recalc）。
- **门禁**
  - `make check routing && make check doc`

## PR-3：Milestone 3 — Authz（registry/middleware/policy + pack）

- **状态**：已完成（2026-01-09）
- **PR**：#158（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/158）
- **Merge Commit**：`80f0b5fe1c0189b37df667851fdb9a1283496035`
- **合并时间**：2026-01-09T18:24:06Z
- **范围**
  - Authz：为日结果详情页 POST 增加 admin；新增 internal_api（void/recalc）映射。
  - 更新 bootstrap policy 并执行 authz-pack（生成 `config/access/policy.csv + .rev`）。
- **门禁**
  - `go fmt ./... && go vet ./...`
  - `make authz-pack authz-test authz-lint && make check lint && make test && make check doc`

## PR-4：Milestone 4 — DB（Schema/RLS/迁移/Kernel）

- **状态**：已完成（2026-01-09）
- **PR**：#159（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/159）
- **Merge Commit**：`b6a114a4a8794fa8cbc7bf6c6322c7be0de1d688`
- **合并时间**：2026-01-09T18:55:19Z
- **范围**
  - 新增表：`staffing.time_punch_void_events` / `staffing.attendance_recalc_events`（RLS + 索引 + 约束）。
  - Kernel：
    - `get_time_profile_for_work_date(...)`
    - `recompute_daily_attendance_result(...)` 过滤 voided punches
    - `submit_time_punch_void_event(...)` / `submit_attendance_recalc_event(...)`
- **门禁**
  - `make staffing plan && make staffing lint && make staffing migrate up && make check doc`

## PR-5：Milestone 5 — Go（Store + Handler/UI + internal_api）

- **状态**：已完成（2026-01-09）
- **PR**：#160（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/160）
- **Merge Commit**：`632050c015fa48a603984fced3478fa6afd75fa7`
- **合并时间**：2026-01-09T20:23:53Z
- **范围**
  - Store：新增作废打卡与日结果重算写入口（显式 tx + tenant 注入 + kernel `submit_*_event`）。
  - Handler/UI：日结果详情页支持 void/recalc；新增审计区块（time profile window、punches（含 void 标记）、recalc events）。
  - Internal API：新增 `POST /org/api/attendance-punch-voids` 与 `POST /org/api/attendance-recalc`。
- **门禁（本地）**
  - `go fmt ./... && go vet ./... && make check lint && make check routing && make test && make check doc`

## PR-6：Milestone 6 — Tests（覆盖 + DB 行为）

- **状态**：已完成（2026-01-09）
- **PR**：#161（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/161）
- **Merge Commit**：`1dfb6aeef15fe81b744847625110cd01d05d624e`
- **合并时间**：2026-01-09T20:44:45Z
- **范围**
  - DB 集成测试：void OUT 后日结果变为 `EXCEPTION + MISSING_OUT`。
  - 覆盖 void/recalc 幂等（`STAFFING_IDEMPOTENCY_REUSED`）。
  - 新表 RLS fail-closed 覆盖；测试 schema 补齐 kernel `submit_*_event`。
- **门禁（本地）**
  - `go fmt ./... && go vet ./... && make check lint && make test && make check doc`

## PR-7：Milestone 7 — E2E（可选 smoke）

- **状态**：已完成（2026-01-09）
- **PR**：#162（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/162）
- **Merge Commit**：`36da3b1cd477e4ed11757fea7ae07a72826ab42a`
- **合并时间**：2026-01-09T20:56:57Z
- **范围**
  - E2E：纳入“作废 OUT punch → 日结果变为 EXCEPTION/MISSING_OUT + 审计可见 VOIDED”。
- **门禁（本地）**
  - `make e2e && make check doc`

## PR-8：Milestone 8 — 证据回填（执行日志 + 完成项登记）

- **状态**：已完成（本 PR 合并后）
- **PR**：本 PR
- **范围**
  - 新增本执行日志：`docs/dev-records/dev-plan-055-execution-log.md`
  - 在计划文档 `docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md` 勾选 8.2.8，并更新状态为“已实施”
