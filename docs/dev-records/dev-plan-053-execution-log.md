# DEV-PLAN-053 记录：考勤 Slice 4C——主数据与合规闭环（TimeProfile / HolidayCalendar）执行日志

**状态**: 已完成（2026-01-09）

> 本日志用于跟踪 `docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md` 的实施拆分（§8.2），确保每个 PR 可独立验收、门禁对齐、并在合并后回填完成情况。

## PR-0：计划文档收敛到可实施颗粒度

- **状态**：已完成（2026-01-09）
- **PR**：#122
- **交付物**
  - 计划文档：`docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- **本地门禁**
  - `make check doc`：通过

## PR-1：建表批准记录固化 + 执行日志落地

- **状态**：已完成（2026-01-09）
- **PR**：#131（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/131）
- **范围**
  - 回填“新增表手工批准”的记录入口（对齐 `AGENTS.md` §3.2 红线）
  - 新增本执行日志，后续每个 PR 合并后回填链接与门禁结果
- **本地门禁**
  - `make check doc`：通过

## PR-2：Milestone 1 — Schema SSOT / RLS / 迁移闭环（staffing）

- **状态**：已完成（2026-01-09）
- **PR**：#132（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/132）

## PR-3：Milestone 2 — Kernel（submit_time_profile_event / submit_holiday_day_event）

- **状态**：已完成（2026-01-09）
- **PR**：#133（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/133）

## PR-4：Milestone 3 — 日结果投射（day_type / OFF / OT 分桶）

- **状态**：已完成（2026-01-09）
- **PR**：#136（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/136）
- **说明**
  - 修复 E2E：新租户创建时 bootstrap 默认 TimeProfile（避免 /attendance 日结果重算链路缺少主数据导致失败）
  - 合并方式：merge commit
- **门禁**
  - GitHub Actions（Quality Gates）：全绿（Code Quality & Formatting / Unit & Integration / Routing Gates / E2E）

## PR-4b：执行日志回填（登记 PR-4 完整完成情况）

- **状态**：已完成（2026-01-09）
- **PR**：#137（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/137）
- **本地门禁**
  - `make check doc`：通过

## PR-5：Milestone 4 — UI（TimeProfile / HolidayCalendar）

- **状态**：已完成（2026-01-09）
- **PR**：#140（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/140）
- **范围**
  - UI：新增 `/org/attendance-time-profile`、`/org/attendance-holiday-calendar`（含 CSV 粘贴导入）
  - Routing/Authz：allowlist + authz registry + middleware mapping + bootstrap policy（为保证路由门禁与授权 fail-closed）
  - Tests：为满足仓库 100% coverage 门禁补齐最小单测
- **门禁**
  - GitHub Actions（Quality Gates）：全绿（Code Quality & Formatting / Unit & Integration / Routing Gates / E2E）

## PR-5b：执行日志回填（登记 PR-5 完整完成情况）

- **状态**：已完成（2026-01-09）
- **PR**：#143（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/143）
- **本地门禁**
  - `make check doc`：通过

## PR-6：Milestone 5 — Authz / Routing（allowlist + bootstrap policy）

- **状态**：已完成（并入 PR-5，2026-01-09）
- **PR**：#140（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/140）
- **说明**
  - 为保证路由门禁与授权 fail-closed，本里程碑已与 UI 同 PR 交付，不再单独拆分。

## PR-7：Milestone 6 — Tests（核心口径 + 负例）

- **状态**：已完成（并入 PR-5，2026-01-09）
- **PR**：#140（https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/140）
- **说明**
  - 为满足仓库 100% coverage 与 Quality Gates，本里程碑已与 UI 同 PR 交付，不再单独拆分。
