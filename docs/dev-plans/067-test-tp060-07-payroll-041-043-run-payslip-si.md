# DEV-PLAN-067：全链路业务测试子计划 TP-060-07——薪酬 041-043（主流程 + 工资条 + 社保）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（薪酬输入：base_salary/allocated_fte）。

## 1. 背景

`DEV-PLAN-039` 的薪酬路线图要求每切片可见可操作：P0-1（period/run）→ P0-2（payslip/items）→ P0-3（社保政策与扣缴情形）。本子计划验证：
- pay period / payroll run 的创建、计算、定稿（定稿后只读）；
- payslips 列表/详情可见，且汇总可由明细对账；
- 社保政策（单城市）配置与扣缴计算符合“clamp + 舍入合同”。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] pay period/run：可创建、可计算、可定稿；定稿后只读（再次计算/再次定稿失败且有稳定错误码）。
- [ ] payslips：run 下列表/详情可见；`gross_pay/net_pay/employer_total` 与明细可对账。
- [ ] 社保：险种分项明细可见；覆盖基数下限/上限 clamp 与两种舍入策略的可判定断言。

### 2.2 非目标

- 不在本子计划验证 IIT/回溯/净额保证（由 TP-060-08 承接）。

## 3. 契约引用（SSOT）

- 路线图：`docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`
- P0-1：`docs/dev-plans/041-payroll-p0-slice-pay-period-and-payroll-run.md`
- P0-2：`docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`
- P0-3：`docs/dev-plans/043-payroll-p0-slice-social-insurance-policy-and-calculation.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- Pay group：`monthly`
- Pay periods：
  - `PP-2026-01`：`[2026-01-01, 2026-02-01)`
- 人员/任职：10 人 assignments 已具备：
  - `base_salary`（CNY）
  - `allocated_fte`（包含 E04=0.5）
- 社保政策（单城市，P0）：`city_code=CN-310000`，且 6 个险种均有 policy+version（可将不关心险种费率设为 0）。
  - 建议测试政策值（用于可判定断言；对齐 `DEV-PLAN-043`）：
    - `base_floor=5000.00`、`base_ceiling=30001.00`
    - PENSION：`employer_rate=0.160000`、`employee_rate=0.080000`、`rounding_rule=HALF_UP`、`precision=2`
    - MEDICAL：`employer_rate=0.095530`、`employee_rate=0.020070`、`rounding_rule=CEIL`、`precision=2`

## 5. 测试步骤（执行时勾选）

1. [ ] 配置社保政策：`/org/payroll-social-insurance-policies?as_of=2026-01-01`（确保 6 个险种均可 as-of 命中）。
2. [ ] 创建 pay period：`/org/payroll-periods?as_of=2026-01-01`（pay_group=monthly，period 为自然月）。
3. [ ] 创建 payroll run：`/org/payroll-runs?as_of=2026-01-01`（关联 `PP-2026-01`）。
4. [ ] 计算 run：在 run 详情执行 calculate，确保状态进入 `calculated`（失败必须有稳定错误码与可复现证据）。
5. [ ] payslips 列表/详情：进入 `/org/payroll-runs/{run_id}/payslips?as_of=...`，抽样至少 2 人查看详情与明细。
6. [ ] 可判定断言：
   - 断言 A（042/FTE）：E04 的 `EARNING_BASE_SALARY` 明细金额应为 `10,000.00`（`20,000.00 × 0.5`，有效期覆盖整月）。
   - 断言 B（043/clamp+舍入；按 §4 的测试政策值）：
     - E03（`gross_pay=3,000.00`，clamp 到 `5,000.00`）：
       - PENSION：employee=`400.00`，employer=`800.00`
       - MEDICAL：employee=`100.35`，employer=`477.65`
     - E02（`gross_pay=80,000.00`，clamp 到 `30,001.00`）：
       - PENSION：employee=`2,400.08`，employer=`4,800.16`
       - MEDICAL：employee=`602.13`（CEIL），employer=`2,866.00`（CEIL）
7. [ ] 定稿 run：执行 finalize，状态进入 `finalized`；再次 calculate/finalize 必须失败（稳定错误码）。

## 6. 验收证据（最小）

- pay period/run 列表与 run 详情（含状态与时间戳）。
- payslip 列表/详情证据（含明细与汇总）。
- 社保险种分项明细证据 + 可判定断言计算结果证据。
- finalized 只读证据（重复操作失败）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

