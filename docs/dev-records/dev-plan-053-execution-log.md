# DEV-PLAN-053 记录：考勤 Slice 4C——主数据与合规闭环（TimeProfile / HolidayCalendar）执行日志

**状态**: 进行中（2026-01-09）

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

- **状态**：进行中

## PR-3：Milestone 2 — Kernel（submit_time_profile_event / submit_holiday_day_event）

- **状态**：未开始

## PR-4：Milestone 3 — 日结果投射（day_type / OFF / OT 分桶）

- **状态**：未开始

## PR-5：Milestone 4 — UI（TimeProfile / HolidayCalendar）

- **状态**：未开始

## PR-6：Milestone 5 — Authz / Routing（allowlist + bootstrap policy）

- **状态**：未开始

## PR-7：Milestone 6 — Tests（核心口径 + 负例）

- **状态**：未开始
