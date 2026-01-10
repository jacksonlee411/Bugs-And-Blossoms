# DEV-PLAN-065：全链路业务测试子计划 TP-060-05——考勤 4B-4E（日结果/配置/时间银行/更正与审计）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`（已准备 punches 输入）。

## 1. 背景

本子计划覆盖考勤从 punches → daily results → 配置（TimeProfile/HolidayCalendar）→ 时间银行 → 更正与审计的端到端闭环（对齐 `DEV-PLAN-052~055`）。其中：
- 4B 已确认 Option A：不提供 UI 手工范围重算入口；
- 4E 在详情页承接“作废/更正/重算”的最小链路，并要求 bounded replay。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] 日结果列表/详情可见，并可解释异常原因（至少覆盖 PRESENT 与 EXCEPTION）。
- [ ] 配置页可操作：可配置租户默认 TimeProfile 与 HolidayCalendar 覆盖项，并能驱动日结果解释字段变化。
- [ ] 时间银行页可见：月度汇总与 trace 可跳转到日结果详情。
- [ ] 更正与审计闭环：作废 punch 后日结果更新可见，且 audit 记录可追溯。

### 2.2 非目标

- 不验证外部对接摄入（由 TP-060-06 承接）。

## 3. 契约引用（SSOT）

- 4B 日结果：`docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`
- 4C 配置：`docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- 4D 时间银行：`docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`
- 4E 更正与审计：`docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- `as_of`：建议 `2026-01-02`
- punches 输入：
  - E01：`2026-01-02 09:00 IN / 18:00 OUT`（来自 TP-060-04）
  - E03：`2026-01-02 09:00 IN`（缺卡样例，来自 TP-060-04）
- 配置输入：
  - TimeProfile：配置租户默认标准班次（09:00-18:00，午休 60m，Asia/Shanghai）
  - HolidayCalendar：将 `2026-01-01` 标记为 Holiday（按月覆盖项）

### 4.1 数据保留（强制）

- 本子计划创建/修改的配置（TimeProfile/HolidayCalendar）、作废记录（VOIDED）与日结果变化必须保留，供后续排障与 TP-060-06/07/08 的复用与回归（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

## 5. 测试步骤（执行时勾选）

1. [ ] 配置 TimeProfile：`/org/attendance-time-profile?as_of=2026-01-01`（保存成功且页面可回显）。
2. [ ] 配置 HolidayCalendar：`/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01`，将 `2026-01-01` 标记为 Holiday。
3. [ ] 日结果列表：`/org/attendance-daily-results?as_of=2026-01-02&work_date=2026-01-02`（至少包含 E01/E03 行）。
4. [ ] 日结果断言（可判定，4B/4E）：
   - E01：应为 `PRESENT`。
   - 进入 E01 详情页，作废（void）`18:00 OUT` 后，summary 必须变为 `EXCEPTION` 且包含 `MISSING_OUT`；punch 审计表中该记录标记为 `VOIDED`。
5. [ ] 时间银行页：`/org/attendance-time-bank?as_of=2026-01-02&month=2026-01&person_uuid=<...>`（可见月度汇总与 trace 链接）。

## 6. 验收证据（最小）

- TimeProfile/HolidayCalendar 配置保存成功证据（截图或回显）。
- 日结果列表/详情证据（含 PRESENT/EXCEPTION 与原因字段）。
- 作废 punch 后的 audit 证据（含 VOIDED 标记）。
- 时间银行页面证据（含 trace 链接）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
