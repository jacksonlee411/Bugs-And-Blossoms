# DEV-PLAN-063：全链路业务测试子计划 TP-060-03——人员与任职（Person + Assignments）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`（positions 作为 assignments 输入来源）。

## 1. 背景

本子计划用于验证 Person 身份锚点与 Staffing 任职时间线是否按契约工作：
- `pernr` 为 1-8 位数字字符串，且**前导 0 同值**（`DEV-PLAN-027`）；
- Assignment UI 时间线仅展示 `effective_date`（`DEV-PLAN-031` 的 UI 合同）；
- 任职输入侧满足后续 payroll/attendance 的可复用性（`DEV-PLAN-042` 需要 `base_salary/allocated_fte`）。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] 可创建 10 个 Person（含前导 0 输入样例），并可按 pernr 精确解析到同一 `person_uuid`。
- [ ] 可为 10 人创建/更新 Assignment（绑定 position_id），时间线可见且仅展示 `effective_date`。
- [ ] Assignment 输入满足薪酬计算前置：每人设置 `base_salary` 与 `allocated_fte`（其中至少 1 人为 0.5）。

### 2.2 非目标

- 不验证 payroll/attendance 的计算结果（由后续子计划承接）。

## 3. 契约引用（SSOT）

- Person identity：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- Assignments：`docs/dev-plans/031-greenfield-assignment-job-data.md`
- Valid Time（日粒度）：`docs/dev-plans/032-effective-date-day-granularity.md`
- Payroll 输入语义（base_salary/FTE）：`docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- `as_of`：建议 `2026-01-01`
- 10 个 position_id（来自 TP-060-02）
- 10 个员工（E01~E10，见 `docs/dev-plans/060-business-e2e-test-suite.md` §5.8）

## 5. 测试步骤（执行时勾选）

1. [ ] **创建 Person（10 人）**
   - 入口：`/person/persons?as_of=2026-01-01`
   - 创建 E01~E10（记录每人的 `person_uuid`）。
   - 前导 0 断言：用 `pernr=00000103` 创建后，再用 `pernr=103` 进行查询/解析必须命中同一人（对齐 `DEV-PLAN-027`“canonical pernr”）。
2. [ ] **创建/更新 Assignment（10 人）**
   - 入口：`/org/assignments?as_of=2026-01-01&pernr=<...>`
   - 为每人绑定一个 `position_id` 并提交（记录 assignment 行/版本可见）。
3. [ ] **断言：时间线展示口径**
   - 任一员工的 timeline：
     - 必须包含 `effective_date`；
     - 页面不得展示 `end_date`（对齐 `DEV-PLAN-031`/UI 合同）。
4. [ ] **补齐薪酬输入字段（为后续子计划准备）**
   - 在 Assignment 上设置：
     - `base_salary`（CNY）
     - `allocated_fte`（至少包含 0.5 样例）
   - 记录：E04 的 `allocated_fte=0.5`（用于 TP-060-07 的可判定断言）。

## 6. 验收证据（最小）

- 10 个 Person 的列表证据（含 `person_uuid/pernr`）。
- 前导 0 同值断言证据（同一 uuid）。
- Assignment timeline 证据（含 `effective_date`，且无 `end_date`）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

