# DEV-PLAN-042：Payroll P0-2——工资条与工资项（Payslip & Pay Items，Gross Pay）

**状态**: 规划中（2026-01-08 01:56 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 依赖：`DEV-PLAN-041`（pay period / payroll run / payslip 载体）  
> 蓝图合同：`DEV-PLAN-040`

## 0. 可执行方案（本计划合同）

### 0.1 背景与上下文

本切片把 `DEV-PLAN-041` 的“主流程壳”补齐为可对账的工资条：至少能从 Assignment 定薪字段生成一张工资条、若干工资项明细，并在 UI 展示明细与汇总字段（列化）。

### 0.2 目标与非目标（P0-2 Slice）

**目标**
- [ ] 冻结工资项模型（最小集合）：明确哪些属于应发（earnings）、哪些属于扣款（deductions）、哪些属于公司成本（employer costs）。
- [ ] 实现 Gross Pay（应发）最小计算：从 `staffing.assignment_versions.base_salary`（及必要的 pro-rate）计算当期应发，并落到 `payslip_items` 与 `payslips.gross_pay`。
- [ ] 工资条对账口径可重算：`gross_pay/net_pay/employer_total` 必须可由明细聚合得到（列化字段仅为快照/查询优化，不得成为第二权威）。
- [ ] UI 工资条详情可解释：展示工资项明细（至少含工资项 code、名称、金额、方向/归类）。

**非目标**
- 不在本切片引入社保与个税算法（见 `DEV-PLAN-043/044`）。
- 不引入通用表达式引擎（对齐 `DEV-PLAN-040` §4.3）。

### 0.3 工具链与门禁（SSOT 引用）

- 触发器矩阵与本地必跑：`AGENTS.md`
- DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
- 时间语义（Valid Time）：`docs/dev-plans/032-effective-date-day-granularity.md`

### 0.4 关键不变量与失败路径（停止线）

- **金额无浮点**：计算必须使用 `apd.Decimal`；对外/JSON 传输金额使用 string。
- **舍入合同冻结**：必须定义“舍入点”与枚举口径，并在测试中覆盖（避免隐式 Round 漂移）。
- **JSONB 边界**：工资项明细一律子表；禁止把权威金额明细塞入 JSONB array。

### 0.5 实施步骤（Checklist）

1. [ ] 冻结 pay item 最小枚举（示例口径）：
   - `EARNING_BASE_SALARY`（应发）
   - `EARNING_ALLOWANCE_*`（可选，若 P0 需要）
   - `DEDUCTION_PLACEHOLDER`（为后续社保/个税预留类型占位，但本切片不计算）
2. [ ] 定义 pro-rate 合同（最小可用）：按 pay period 与 assignment validity 的交集天数比例计算（Valid Time，日粒度）。
3. [ ] 生成 payslip items：对每个在 pay period 内有效的 assignment 生成一张 payslip，写入 base salary 项，并汇总 `gross_pay`。
4. [ ] UI：工资条列表/详情展示明细与汇总；对账字段与明细聚合结果一致。
5. [ ] 测试：pro-rate 边界（整月/半月/跨月）、金额精度与舍入点可复现。

### 0.6 验收标准（Done 口径）

- [ ] 对任一人员/任职：创建 payroll run 后可生成工资条；工资条详情至少包含“基本工资”一项明细。
- [ ] `payslips.gross_pay` 等于该工资条明细项聚合结果（可重算）。
- [ ] pro-rate 在日粒度下可复现（示例：月中入职/离职）。

## 1. 备注（与后续切片的边界）

- `DEV-PLAN-043` 将在本切片的 payslip 上追加社保扣缴明细与公司成本。
- `DEV-PLAN-044` 将使用本切片的应发与社保扣缴作为个税计税基础。
