# DEV-PLAN-043：Payroll P0-3——社保政策（单城市）配置与扣缴计算

**状态**: 规划中（2026-01-08 01:56 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 依赖：`DEV-PLAN-041/042`（主流程 + 工资条载体与应发）  
> 蓝图合同：`DEV-PLAN-040`

## 0. 可执行方案（本计划合同）

### 0.1 背景与上下文

中国社保政策高度碎片化，但 P0 Slice 以“单城市起步”：支持按 `city_code + validity` 选择一套政策，计算个人/企业缴费并落地为工资条明细（可对账）。

### 0.2 目标与非目标（P0-3 Slice）

**目标**
- [ ] 冻结社保政策的最小数据模型（列化关键字段 + JSONB 扩展边界），并通过 Kernel One Door 写入。
- [ ] 实现个人/企业缴费计算（基数上下限、费率、舍入规则）并写入明细子表。
- [ ] 工资条可解释：展示各险种个人扣款与企业成本，且企业成本计入 `employer_total`。
- [ ] 为 `DEV-PLAN-044` 个税计算提供可复用的“专项扣除（社保个人部分）”数据口径。

**非目标**
- 不实现 300+ 城市全量规则；不实现与外部社保申报系统的集成。

### 0.3 工具链与门禁（SSOT 引用）

- DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
- Valid Time：`docs/dev-plans/032-effective-date-day-granularity.md`

### 0.4 关键不变量与失败路径（停止线）

- **Valid Time**：社保政策版本使用 `daterange`（`[start,end)`），同一 `tenant/city_code/insurance_type` 下不得重叠。
- **SetID/BU 边界**：P0 社保政策不引入 `business_unit_id/setid` 维度（按 `tenant_id + city_code + validity` 生效）；如需 BU 级差异必须先按 `DEV-PLAN-028` 扩展 stable record group 并接入权威解析入口，再另立 dev-plan 承接迁移与门禁。
- **列化关键字段**：费率/上下限/舍入规则必须列化；JSONB 仅承载低频扩展。
- **可对账**：社保明细必须子表；禁止 JSONB array 承载权威明细。

### 0.5 实施步骤（Checklist）

1. [ ] 冻结最小险种集合（P0）：例如 `PENSION/MEDICAL/UNEMPLOYMENT/INJURY/MATERNITY/HF`（若 P0 仅做部分，需在本计划明示）。
2. [ ] 冻结政策字段：`employer_rate/employee_rate/base_floor/base_ceiling/rounding_rule` 等（列化）。
3. [ ] Kernel：`submit_social_insurance_policy_event(...)` + 同事务投射 `*_policy_versions`。
4. [ ] 计算：Base = MAX(Floor, MIN(Actual, Ceiling))；按险种计算个人/企业金额并按规则舍入。
5. [ ] 写入工资条：个人部分作为扣款明细；企业部分作为公司成本明细并计入 `employer_total`。
6. [ ] UI：社保政策配置页（单城市）与工资条详情展示。

### 0.6 验收标准（Done 口径）

- [ ] UI 可配置一套社保政策（单城市）并在指定 pay period 生效。
- [ ] 工资条展示各险种个人扣款与企业成本，且汇总口径可由明细重算。
- [ ] 舍入规则在测试中可复现（至少覆盖 2 种舍入策略）。

## 1. 备注（与后续切片的边界）

`DEV-PLAN-044` 的个税计税基础需要显式使用“社保个人扣款”作为专项扣除（口径冻结点在该计划中明确）。
