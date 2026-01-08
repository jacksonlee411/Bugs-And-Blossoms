# DEV-PLAN-041：Payroll P0-1——薪资周期与算薪批次（Pay Period & Payroll Run）

**状态**: 规划中（2026-01-08 02:40 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 蓝图合同（范围/不变量/验收基线）：`DEV-PLAN-040`

## 0. 可执行方案（本计划合同）

### 0.1 背景与上下文

- 薪酬社保落点在 `modules/staffing`，必须复用既有 Kernel/RLS/Valid Time 合同与实现模式（示例：`staffing.submit_position_event` 等）。
- 本切片只交付“主流程壳”：pay period 与 payroll run 的状态机 + 写入口 + UI 可见入口；计算引擎与扣缴明细在后续切片完善（`DEV-PLAN-042+`）。

### 0.2 目标与非目标（P0-1 Slice）

**目标**
- [ ] 设计并冻结最小数据模型：`pay_periods`、`payroll_runs`、`payslips`（可先空壳）与必要的状态字段（列化，便于审计/筛选）。
- [ ] 落地 Kernel 写入口（One Door）：创建 pay period、创建 payroll run、触发计算、定稿（同事务投射读模型）。
- [ ] RLS fail-closed：所有 payroll 相关表启用 RLS，且访问必须显式事务 + tenant 注入（No Tx, No RLS）。
- [ ] UI 可见/可操作：新增“薪酬”入口；提供 pay period 列表/创建，payroll run 列表/详情，含“计算/定稿”动作入口。

**非目标**
- 不在本切片实现真实算薪逻辑（gross/社保/个税），只需保证“计算动作可触发并产生可见结果（即使为空壳/占位）”。
- 回溯重算的执行闭环由 `DEV-PLAN-045` 承接；本切片只需提供承载状态与工作流（例如 runs 的只读定稿语义、以及后续可扩展的 `needs_recalc` 标记位）。

### 0.3 工具链与门禁（SSOT 引用）

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`
- DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`（更新 `config/routing/allowlist.yaml` 并通过 `make check routing`）
- UI Shell：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`

### 0.4 关键不变量与失败路径（停止线）

- **One Door**：禁止直接写读模型表；所有写入必须走 Kernel `submit_payroll_*_event(...)`。
- **No Tx, No RLS**：缺少 tenant context 直接失败（fail-closed），禁止 superuser/bypass RLS 跑业务链路。
- **Valid Time = date**：pay period 边界统一 `[start,end)`，使用 `daterange`。
- **SetID/BU 边界**：本切片不引入 `business_unit_id/setid/record_group`；`pay_group` 仅用于算薪分组，不等价于 BU/SetID（对齐 `DEV-PLAN-040` 与 `DEV-PLAN-028`）。
- **幂等**：所有 submit 函数必须支持 `event_id/request_id` 复用；冲突抛稳定错误码。
- **状态机冻结**：payroll run 的状态必须可列化/可审计；不允许“靠 JSONB 备注字段”表达流程状态。

### 0.5 实施步骤（Checklist）

1. [ ] 明确状态机：`DRAFT -> CALCULATING -> CALCULATED -> FINALIZED`（或等价口径），并冻结允许的状态跃迁与错误码。
2. [ ] 设计最小表（草案即可，进入实现前再冻结字段级合同）：
   - `staffing.pay_periods`：`tenant_id`、`pay_period_id`、`pay_group`、`period daterange`、`status`、审计列。
   - `staffing.payroll_runs`：`tenant_id`、`run_id`、`pay_period_id`、`run_state`、`started_at/finished_at`、`last_event_id`、审计列。
   - `staffing.payslips`：`tenant_id`、`run_id`、`person_id/assignment_id`、`gross_pay/net_pay/employer_total`（可先 0）、`last_event_id`。
3. [ ] 按 `DEV-PLAN-024` 闭环落地迁移与 Kernel 函数：`submit_payroll_pay_period_event`、`submit_payroll_run_event`、`submit_payroll_run_action_event`（命名可调整，但必须 One Door）。
4. [ ] UI：新增 `/org/payroll-*` 路由与页面入口（列表/创建/详情），并登记 allowlist。
5. [ ] 基础测试：状态机跃迁/幂等/RLS fail-closed 的最小可复现测试。

### 0.6 验收标准（Done 口径）

- [ ] UI 可创建一个 pay period，并在列表可见。
- [ ] UI 可创建一个 payroll run，并在详情页触发“计算”动作，run 状态可见且可查询。
- [ ] UI 可对一个 run 执行“定稿”，定稿后 run 与 payslip 进入只读（后续切片可填充真实金额，但只读语义必须先冻结）。
- [ ] 访问缺少 tenant context 时，相关查询/写入 fail-closed（No Tx, No RLS）。

## 1. 备注（与后续切片的边界）

- `DEV-PLAN-042` 负责把 payslip 从“空壳”升级为“可对账 gross pay + 明细项”。
- `DEV-PLAN-043/044/046` 逐步补齐社保、个税与税后发放逻辑；`DEV-PLAN-045` 负责回溯重算闭环；本切片只提供稳定载体与工作流。
