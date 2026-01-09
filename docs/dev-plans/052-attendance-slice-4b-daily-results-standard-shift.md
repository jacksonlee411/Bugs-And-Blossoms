# DEV-PLAN-052：考勤 Slice 4B——日结果计算闭环（标准班次）

**状态**: 草拟中（2026-01-09 02:17 UTC）

## 1. 背景与上下文

- 本计划承接 `DEV-PLAN-051` 的“打卡流水闭环”，将流水同步投射为**按人按日（`date` 粒度）**的权威日结果读模（对齐 `DEV-PLAN-050` §0.3 的“读模水位线”策略）。
- 路线图颗粒度对齐 `DEV-PLAN-009` Phase 4（垂直切片交付）。
- 模块落点：优先在 `modules/staffing` 内作为子域实现（对齐 `DEV-PLAN-015/016`）。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [ ] **权威读模**：形成 `date` 粒度的日结果读模（示例：`t_daily_attendance_result`），并记录：
  - `ruleset_version`（规则版本标识，便于追溯）
  - `input_watermark_*`（输入水位线，例如已纳入计算的最大 `punch_time`/事件序号）
  - `computed_at`（计算时间）
- [ ] **同步投射**：在 `submit_*_event(...)` 同一事务内更新日结果（写后读强一致，不引入异步权威写）。
- [ ] **标准班次最小算法**：完成最小配对（含跨天）与基础异常标记（缺勤/迟到/早退/漏打卡）。
- [ ] **UI 可见**：日结果列表/详情可见，并能解释“为何异常”（来自读模而非临时计算）。
- [ ] **可测**：至少覆盖跨天/漏打卡/多次打卡的核心用例；并包含 RLS 负例（fail-closed）。

### 2.2 非目标（本计划不做）

- 不引入可配置 TimeProfile/Shift/HolidayCalendar（见 `DEV-PLAN-053`）；本切片先固定“标准班次”最小口径（规则版本可用常量标识）。
- 不做加班费率分桶与调休选择（见 `DEV-PLAN-053/054`）。
- 不做纠错事件链/审计 UI（见 `DEV-PLAN-055`）。

## 3. 关键设计决策（草案）

- **Valid Time**：日结果以 `work_date date` 作为业务有效期粒度（对齐 `AGENTS.md` 与 `DEV-PLAN-032`），避免在业务层引入“秒级生效期”。
- **受影响范围（recalc boundary）**：一次打卡写入至少影响 `work_date` 与相邻日期（用于跨天 OUT），推荐以“标准班次最大跨天窗口”定义重算边界，避免全量重放。
- **时区口径（待确认）**：`work_date` 的归属需要明确时区；若无租户时区配置，默认 `Asia/Shanghai`（待在 `DEV-PLAN-053` 中固化为主数据）。

## 4. 数据模型（草案；落地迁移前需手工确认）

> 红线：新增数据库表/迁移落地前必须获得你手工确认。

- 候选表（示例）：
  - `daily_attendance_result`（权威读模，主键 `tenant_id + person_uuid + work_date`）
  - `daily_attendance_result_events`（可选）：若需要把“更正/重算”的意图事件化，可在 `DEV-PLAN-055` 决策（本计划先不强制）。

## 5. 工具链与门禁（SSOT 引用）

### 5.1 触发器（勾选本计划命中的项）

- [ ] Go 代码
- [ ] DB 迁移 / Schema（Atlas+Goose）
- [ ] sqlc
- [ ] 路由治理（Routing）
- [ ] Authz（Casbin）
- [ ] `.templ` / Tailwind（UI 资源）

### 5.2 SSOT 链接

- 触发器矩阵：`AGENTS.md`
- CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
- RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- 分层/边界：`docs/dev-plans/015-ddd-layering-framework.md`、`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- 路由：`docs/dev-plans/017-routing-strategy.md`
- Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
- 蓝图：`docs/dev-plans/050-hrms-attendance-blueprint.md`

## 6. 实施步骤

1. [ ] 明确“标准班次”的最小合同（开始/结束/容差/最大跨天窗口），并为后续 TimeProfile 化预留 `ruleset_version` 口径。
2. [ ] 设计并评审 `daily_attendance_result`（字段、索引、RLS、重算边界）；落迁移前获得手工确认。
3. [ ] 在 Kernel 内实现“按影响范围重算”的同步投射：提交打卡事件后，重算受影响的 `work_date` 结果并写入读模。
4. [ ] 用 sqlc 暴露读模查询与写入入口（Go 仅调用 `submit_*_event`，不绕过 Kernel 写读模）。
5. [ ] UI：新增“日结果”页面（列表/详情），并能从读模展示异常原因与构成。
6. [ ] 测试：覆盖跨天、漏打卡、多次打卡；并包含 RLS fail-closed 与跨租户隔离负例。
7. [ ] 运行命中触发器门禁并记录结果（必要时补 `docs/dev-records/`）。

## 7. 验收标准

- [ ] 端到端：新增一条打卡 → 对应 `work_date` 的日结果可见且可解释。
- [ ] 一致性：写后读强一致（事务提交后刷新页面即为最新）。
- [ ] 可重算：再次补打卡/更正同日数据后，读模会被重算覆盖且 `computed_at` 更新。
- [ ] 安全：RLS/Authz/路由门禁全绿。

## 8. 开放问题

- [ ] 时区与 `work_date` 归属口径（租户时区的 SSOT 与默认值策略）。
- [ ] `ruleset_version` 的表达形式（常量/表驱动/哈希），以及与 `DEV-PLAN-053` 的衔接。

