# DEV-PLAN-045：Payroll P0-5——回溯计算（Retroactive Accounting）

**状态**: 规划中（2026-01-08 02:40 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 蓝图合同（口径/不变量/验收基线）：`DEV-PLAN-040` §5.3  
> 依赖：`DEV-PLAN-041/042/043/044`（主流程载体 + 明细可对账 + 社保 + IIT 累计预扣与 balances）

## 0. 可执行方案（本计划合同）

### 0.1 背景与上下文

回溯计算是 HRMS 的核心能力之一：当 HR 在后续日期提交“更早生效”的任职/定薪变更，且该变更命中已定稿 pay period 时，系统必须能自动识别影响范围、生成重算请求、并在后续周期把差额闭环，避免人工手算差额或直接改读模型。

P0 的交付边界是：以 **Forwarding（结转法）** 为主完成闭环（差额作为独立 pay item 进入后续周期），并保持全链路可审计与幂等；不在 P0 交付银行文件/财务凭证/外部申报等 Post-Process。

### 0.2 目标与非目标（P0-5 Slice）

**目标**
- [ ] 回溯触发可捕获：当通过 Kernel `submit_*_event` 提交的 payroll 输入类事件（例如 `staffing.submit_assignment_event` 的定薪字段变更）命中已定稿 pay period 时，必须生成 `payroll_recalc_request`（append-only）并可追溯关联触发事件。
- [ ] 影响范围可解释：重算请求必须包含“命中哪一个（或哪些）已定稿 pay period / payslip”的可定位信息（至少包含最早命中周期与定位键）。
- [ ] 差额闭环可执行：对处于开放状态的后续 pay period（或指定目标周期），系统可基于请求生成补发/扣回差额 pay items，并保持 `origin_pay_period`/`origin_payslip` 关联，确保审计可追溯。
- [ ] 累计口径一致：差额进入后续周期后，`DEV-PLAN-044` 的 IIT balances 与累计预扣计算口径必须与“差额 pay item”一致（不得出现“工资条已补差但 balances 未更新”的双权威）。
- [ ] UI 可发现/可操作：回溯请求在 UI 上可见（列表/详情/状态），可触发“执行结转”动作，并可在工资条详情查看差额来源。

**非目标**
- 不在 P0 重写已定稿工资条（禁止对 finalized payslip 做 in-place 更新）；历史期的“更正后工资条”如需表达，应以“差额结转 + 可追溯来源”实现。
- 不在 P0 交付批量回溯全员/全年度的自动重算调度；P0 先以“按人员/按请求”可执行为准。

### 0.3 工具链与门禁（SSOT 引用）

- 触发器矩阵与本地必跑：`AGENTS.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`（必要时更新 `config/routing/allowlist.yaml`）
- DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
- No Tx, No RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`

### 0.4 关键不变量与失败路径（停止线）

- **One Door**：生成回溯请求/结转差额 pay item 必须走 Kernel 写入口；禁止手工 `INSERT/UPDATE` 读模型表闭环。
- **append-only**：`payroll_recalc_requests` 必须 append-only；已定稿工资条不得被覆盖式更新。
- **幂等**：回溯请求创建与“执行结转”动作必须幂等（`event_id/request_id` 复用；重复执行不得产生重复差额项）。
- **No Tx, No RLS**：缺少 tenant context 直接失败（fail-closed）；回溯链路不得绕过 RLS。
- **失败可见**：任何无法定位命中周期/无法生成结转差额/无目标周期等情况必须失败并返回稳定错误码（不得静默吞掉回溯请求）。

### 0.5 实施步骤（Checklist）

1. [ ] 冻结“命中已定稿 pay period”的判定口径：给定 `effective_date` 与租户/人员/任职，如何定位“被影响的已定稿周期”（Valid Time：`daterange [start,end)`）。
2. [ ] 设计 `staffing.payroll_recalc_requests`（append-only）最小字段集（草案，进入实现前需冻结字段级合同）：
   - 定位键：`tenant_id`、`request_id`、`trigger_event_id`、`person_id`、`assignment_id`（或等价主键）。
   - 命中信息：`hit_pay_period_id`（最早命中）/（可选）`hit_payslip_id`、`hit_tax_year`。
   - 状态机：`request_state`（PENDING/APPLIED/FAILED/CANCELED 等）与 `last_error_code`（若失败）。
   - 审计：`created_at/created_by`（Tx Time）。
3. [ ] Kernel：在 payroll 输入事件写入口中完成“命中已定稿周期”的判定，并在同一事务内写入 `payroll_recalc_requests`；必要时将命中的 `payroll_runs` 标记 `needs_recalc`（或等价列化字段）。
4. [ ] 结转执行入口：提供 Kernel 动作（例如 `submit_payroll_recalc_apply_event(...)`），将差额写入“目标周期”的 payslip items：
   - 差额项必须带上 `recalc_request_id` 与 `origin_pay_period_id`/`origin_payslip_id` 以供审计。
   - 目标周期的选择：P0 可先限定为“下一次未定稿周期/当前正在计算的 run”，若不存在则返回稳定错误码并保持请求为 PENDING。
5. [ ] 与 balances 的一致性：在差额项落盘时同步更新 `payroll_balances`（或标记需要重算 balances），确保 IIT 累计口径一致。
6. [ ] UI：提供回溯请求列表/详情页与“执行结转”按钮；在工资条详情中展示差额来源（原事件/原周期）。
7. [ ] 测试：覆盖命中判定、幂等、无目标周期、重复执行不重复计入、RLS fail-closed。

### 0.6 验收标准（Done 口径）

- [ ] 已定稿 pay period 后补提交更早生效的定薪/任职变更：系统生成 `payroll_recalc_request` 且 UI 可见。
- [ ] 在存在目标周期的情况下执行“结转”：目标周期工资条出现补发/扣回差额 pay item，并可追溯关联原事件/原周期/原工资条。
- [ ] 差额落盘后，`payroll_balances` 与 IIT 计算口径一致（不出现“差额已入工资条但累计表未更新”的不一致）。

