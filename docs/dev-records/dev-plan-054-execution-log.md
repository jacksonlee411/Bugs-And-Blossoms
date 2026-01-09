# DEV-PLAN-054 记录：考勤 Slice 4D——额度与银行闭环（调休/综合工时累加器）执行日志

**状态**: 已完成（2026-01-09）

> 本日志用于跟踪 `docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md` 的实施拆分（§8.2），确保每个 PR 可独立验收、门禁对齐、并在合并后回填完成情况。

## PR-1：Milestone 1 — 文档确认 + 新增表批准记录固化

- **状态**：已完成（2026-01-09）
- **PR**：#147（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/147）
- **Merge Commit**：`bc4111622963f2f7f570d78fc4d06369c0ff4031`
- **合并时间**：2026-01-09T14:27:09Z
- **范围**
  - 冻结决策：仅支持 `MONTH`；调休累积口径固定为“RESTDAY OT200 → comp earned（1:1）”。
  - 按 `AGENTS.md` §3.2 红线记录“新增表已获手工批准”（后续迁移可落地）。

## PR-2：Milestone 2 — DB（Schema SSOT / RLS / 迁移 / Kernel 联动）

- **状态**：已完成（2026-01-09）
- **PR**：#150（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/150）
- **Merge Commit**：`cb015f0ef4b2a6c0204e7ab643e841d9ec1c3a69`
- **合并时间**：2026-01-09T16:14:51Z
- **范围**
  - 新增 `staffing.time_bank_cycles`（RLS ENABLE+FORCE；tenant fail-closed）。
  - 新增/更新 DB Kernel：周期聚合函数 + 在 `recompute_daily_attendance_result(...)` 同事务联动更新。

## PR-3：Milestone 3 — sqlc（生成物对齐）

- **状态**：已完成（2026-01-09）
- **PR**：#151（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/151）
- **Merge Commit**：`7d9f4e1b3a639822938fffb8c99db1338dede478`
- **合并时间**：2026-01-09T16:20:48Z
- **门禁**
  - `make sqlc-generate` 后 `git status --short` 为空（生成物已提交）。

## PR-4：Milestone 4 — Routing/Authz（allowlist + registry/middleware + policy）

- **状态**：已完成（2026-01-09）
- **PR**：#152（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/152）
- **Merge Commit**：`1d71559df30149345da99cb7bf8f16f6fb69366d`
- **合并时间**：2026-01-09T16:31:07Z
- **范围**
  - 新增 `/org/attendance-time-bank` 路由 allowlist。
  - Authz：registry/middleware 映射 + bootstrap policy（`role:tenant-admin` read）。
  - `make authz-pack` 生成物对齐并提交。

## PR-5：Milestone 5 — Go（Store + Handler + UI 可见入口）

- **状态**：已完成（2026-01-09）
- **PR**：#153（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/153）
- **Merge Commit**：`65340ecbb65d44b795c2043b6f114218ae4e6b54`
- **合并时间**：2026-01-09T17:21:12Z
- **范围**
  - UI：新增 `/org/attendance-time-bank` 页面，展示周期汇总 + 有贡献的日结果链接。
  - Store：`GetTimeBankCycleForMonth(...)`（tx + tenant 注入；fail-closed）。
  - Nav：新增中英文入口（`tr(...)` 同构）。
- **门禁（本地）**
  - `go fmt ./... && go vet ./... && make check lint && make test && make check doc`：通过
- **门禁（CI）**
  - GitHub Actions（Quality Gates）：全绿（Code Quality & Formatting / Unit & Integration / Routing Gates / E2E）

## PR-6：Milestone 6 — Tests（口径/联动/RLS/Authz/并发）

- **状态**：已完成（2026-01-09）
- **PR**：#154（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/154）
- **Merge Commit**：`94207c3f9e494505324dfb9430e00d19a8e0aa74`
- **合并时间**：2026-01-09T17:43:54Z
- **范围**
  - DB 集成测试：MONTH 周期聚合 + 联动更新；RLS fail-closed；并发提交不丢失累计。
  - Handler 测试：`AUTHZ_MODE=enforce` 下 policy 缺失/存在时的 403/200。
- **门禁（本地）**
  - `go fmt ./... && go vet ./... && make check lint && make test && make check doc`：通过
- **门禁（CI）**
  - GitHub Actions（Quality Gates）：全绿（Code Quality & Formatting / Unit & Integration / Routing Gates / E2E）

## PR-7：Milestone 7 — 证据回填（执行日志 + 完成项登记）

- **状态**：已完成（本 PR 合并后）
- **PR**：本 PR
- **范围**
  - 新增本执行日志：`docs/dev-records/dev-plan-054-execution-log.md`
  - 在计划文档 `docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md` 勾选 8.2.7
