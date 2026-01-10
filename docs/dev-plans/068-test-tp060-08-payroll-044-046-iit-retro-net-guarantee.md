# DEV-PLAN-068：全链路业务测试子计划 TP-060-08——薪酬 044-046（个税累计预扣 + 回溯 + 税后发放）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/067-test-tp060-07-payroll-041-043-run-payslip-si.md`（已定稿 `PP-2026-01`，并创建 `PP-2026-02` 的 run 作为承载）。

## 1. 背景

本子计划覆盖薪酬路线图的后半段：
- `DEV-PLAN-044`：个税累计预扣（O(1) balances）与专项附加扣除（SAD）输入载体；
- `DEV-PLAN-045`：回溯计算（定稿后历史生效变更 → 生成 recalc request → 结转差额到后续周期）；
- `DEV-PLAN-046`：税后发放（仅 IIT）的净额保证（分位精确）。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] IIT：工资条展示 IIT 明细与税后实发；balances 可通过 internal API O(1) 读取并满足“累计减除费用月数”口径（可判定断言）。
- [ ] 回溯：已定稿后提交更早生效的任职/定薪变更 → 同事务生成 recalc request；apply 后差额结转到后续周期且可追溯 origin（不覆盖已定稿工资条）。
- [ ] 净额保证（仅 IIT）：`target_net=20,000.00` 的工资项在计算后 `net_after_iit` 精确等于 `20,000.00`（可判定断言）。

### 2.2 非目标

- 不覆盖多 tax_entity/多币种/多城市扩展（P0 边界见 `DEV-PLAN-040/039`）。

## 3. 契约引用（SSOT）

- IIT：`docs/dev-plans/044-payroll-p0-slice-iit-cumulative-withholding-and-balances.md`
- 回溯：`docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md`
- 净额保证：`docs/dev-plans/046-payroll-p0-slice-net-guaranteed-iit-tax-gross-up.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- Pay periods：
  - `PP-2026-01`：已 finalize（来自 TP-060-07）
  - `PP-2026-02`：存在 run（draft/failed）用于承载回溯结转与继续累计
- 人员样例：
  - E06：`effective_date=2026-01-15` 入职（用于 first_tax_month/standard deduction 月数口径）
  - E07：录入净额保证工资项（`target_net=20,000.00`）
  - E05：用于回溯触发（定稿后提交更早生效变更）

## 5. 测试步骤（执行时勾选）

1. [ ] 创建 `PP-2026-02` 与 run（若尚未存在），确保 run 为 `draft/failed`（可重新 calculate）。
2. [ ] SAD 输入（可选但建议）：按 `DEV-PLAN-044` 通过 internal API 录入 E06 的专项附加扣除（月度合计），记录 event_id 与返回结果。
3. [ ] 净额保证输入：按 `DEV-PLAN-046` 录入 E07 的净额保证工资项（`target_net=20,000.00`）。
4. [ ] 计算 `PP-2026-01`（若未计算/未定稿则先完成 TP-060-07）；确保 `PP-2026-01` 已 finalize（Posting 已发生）。
5. [ ] 断言（IIT balances，可判定，044）：
   - 对 E06，执行 `GET /org/api/payroll-balances?person_uuid=<E06_UUID>&tax_year=2026`：
     - `PP-2026-01` 定稿后：`first_tax_month=1`、`last_tax_month=1`、`ytd_standard_deduction="5000.00"`
     - `PP-2026-02` 定稿后：`last_tax_month=2`、`ytd_standard_deduction="10000.00"`
6. [ ] 回溯触发（045）：
   - 在 `PP-2026-01` finalize 后，为 E05 提交“更早 effective_date 的 assignment 变更”（示例：`effective_date=2026-01-15` 加薪）。
   - 断言：`/org/payroll-recalc-requests?as_of=...` 可见新请求；详情可定位 `hit_pay_period_id` 为 `PP-2026-01`。
7. [ ] 回溯 apply（045）：
   - 对该 recalc request 执行 apply，目标为 `PP-2026-02` 的 run；
   - 重新计算 `PP-2026-02` run；
   - 断言：`PP-2026-02` 工资条出现差额 pay item，且可追溯 origin（不覆盖已定稿工资条）。
8. [ ] 断言（净额保证，可判定，046）：
   - E07 的净额保证项计算后 `net_after_iit == 20,000.00`（精确到分），工资条展示 `gross_amount/iit_delta`。

## 6. 验收证据（最小）

- IIT：payslip 详情中 IIT 明细项证据 + `GET /org/api/payroll-balances` 结果（含 first/last_tax_month 与 ytd_standard_deduction）。
- 回溯：recalc request 列表/详情证据 + apply 成功证据 + 后续周期差额 pay items 证据。
- 净额保证：E07 工资条中 `target_net/gross_amount/iit_delta/net_after_iit` 的可解释证据。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

