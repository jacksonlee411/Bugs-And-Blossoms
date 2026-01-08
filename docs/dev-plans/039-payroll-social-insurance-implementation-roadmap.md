# DEV-PLAN-039：薪酬社保（DEV-PLAN-040）实施路线图（切片式，041+）

**状态**: 草拟中（2026-01-08 02:40 UTC）

> 目标：将 `DEV-PLAN-040`（薪酬社保蓝图/合同）拆为可执行的切片式交付序列，并明确先后顺序 + 串行/并行边界。  
> 本文只做 **路线图编排**，不复制各子计划细节；实现合同以 `DEV-PLAN-040` 与 `DEV-PLAN-041+` 为准。  
> 说明：薪酬社保落点在 `modules/staffing`（不新增第 5 个业务模块），必须遵守 One Door / No Tx, No RLS / Valid Time 日粒度 / No Legacy 等全仓不变量（SSOT：`AGENTS.md`）。

## 0. 进度速记

1. [X] 新增 `DEV-PLAN-039` 并登记到 `AGENTS.md` Doc Map。
2. [X] 将 `DEV-PLAN-040` P0 Slice 拆分为 `DEV-PLAN-041`～`DEV-PLAN-046` 子计划（切片式）。
3. [ ] 按本文顺序实施 `DEV-PLAN-041`～`DEV-PLAN-046`，每个切片均满足“可发现/可操作”的 UI 验收。
4. [ ] 将关键门禁/验收证据（时间戳/命令/结果）沉淀到 `docs/dev-records/`（P0-Ready 与后续迭代可审计）。

## 1. 背景（为何需要单独路线图）

`DEV-PLAN-040` 已冻结了薪酬社保的 P0 合同与 stopline，但该领域天然包含：
- DB Kernel 写入口（事件 SoT + 同事务同步投射）；
- 强租户隔离与 RLS fail-closed；
- 金额精度（`numeric` + `apd.Decimal`）与“舍入即合同”；
- 个税累计预扣法（历史依赖）与“税后发放（仅个税）”的税上税递归；
- 回溯计算（Retroactive Accounting）：定稿后历史生效变更触发重算请求，并以可审计差额结转闭环；
- UI 可见性要求（避免“只有后端没有入口”的僵尸功能）。

因此必须采用**切片式**交付：每个切片同时落地“数据模型 + 写入口 + 计算/投射 + 页面入口”，并由既有门禁阻断漂移。

## 2. 目标与非目标（路线图层）

### 2.1 Done 的定义（本路线图出口）

- [ ] 用户在 UI 可完成一条端到端链路：配置社保政策（单城市）→ 创建 pay period → 创建算薪批次 → 计算 → 查看工资条列表/详情 → 定稿（定稿后只读）。
- [ ] 工资条具备可对账明细与汇总：`gross_pay` / `net_pay` / `employer_total` 可由明细重算（不依赖 JSONB 作为权威）。
- [ ] 回溯计算可用：定稿后提交历史生效变更会生成 `payroll_recalc_request`，差额以补发/扣回 pay item 结转到后续周期，并与累计口径（含 IIT balances）一致。
- [ ] 个税支持累计预扣法，且“税后发放/净额保证（仅个税）”可复现：工资项 `target_net` 精确满足，税金成本（含税上税）由公司承担。
- [ ] 全链路满足不变量：One Door / No Tx, No RLS / Valid Time=date / No Legacy / 金额无浮点 / 幂等可复现。

### 2.2 非目标（本文不解决）

与 `DEV-PLAN-040` 一致：不交付金税直连、银企直连、ESOP/LTI、多城市全量规则。

## 3. 执行原则（对齐 DEV-PLAN-009）

1. **切片优先**：每个 `DEV-PLAN-041+` 必须包含可操作 UI 入口与验收脚本，不允许“先堆后端、最后补 UI”。
2. **先门禁、后扩展**：路由治理/迁移闭环/sqlc/生成物一致性等必须从第一条 PR 起就受 CI 阻断（SSOT：`AGENTS.md`、`DEV-PLAN-012/017/024/025`）。
3. **One Door / No Tx, No RLS**：任何 payroll 相关写入一律走 Kernel `submit_*_event(...)`；任何访问必须显式事务 + 租户注入，fail-closed。
4. **确定性金额与求解**：金额全部 `numeric` + `apd.Decimal`；税后发放（仅个税）必须用确定性求解覆盖税上税，不得用一次近似或经验系数。
5. **避免“第二套权威表达”**：关键字段列化；JSONB 仅承载解释/展示快照（权威口径可重算）。

## 4. 依赖速览（谁阻塞谁）

> 本节仅列路线图层关键依赖；具体实现与验收以各子计划为准。

- **平台与门禁前置（必须已就绪）**
  - `DEV-PLAN-012`：CI 质量门禁（required checks）。
  - `DEV-PLAN-017`：路由策略与 routing gates（并更新 `config/routing/allowlist.yaml`）。
  - `DEV-PLAN-019/021`：Tenancy/AuthN + RLS 强隔离（No Tx, No RLS）。
  - `DEV-PLAN-024/025`：Atlas+Goose 闭环 + sqlc 规范与门禁。
  - `DEV-PLAN-028`：SetID（P0 选定不依赖：Payroll 不引入 `business_unit_id/setid/record_group` 维度，配置按 tenant 共享；若未来需要按 BU 分化政策/工资项/规则，必须先扩展 stable record group 并接入 `pkg/setid` 权威解析入口，再另立 dev-plan 承接迁移与门禁）。

- **业务数据依赖（Payroll 的输入侧）**
  - `DEV-PLAN-031`：Assignment/Job Data（定薪字段/有效期）作为 gross pay 与社保/个税输入。
  - `DEV-PLAN-032`：Valid Time 日粒度（`daterange`，统一 `[start,end)`）。

- **UI 基线**
  - `DEV-PLAN-018/020`：AHA UI Shell + en/zh i18n 口径。

## 5. 切片路线图（DEV-PLAN-041+）

> 说明：切片按“用户可见闭环”排序；实现期可并行开发，但合并顺序需保证依赖不反转。

### Slice 1（P0-1）：薪资周期与算薪批次主流程壳

- 计划：`DEV-PLAN-041`
- 目标：跑通“pay period → payroll run → 计算/定稿”的状态机与写入口；UI 能创建/查看批次并触发计算（即使计算结果先为空壳也必须可见）。
- 出口条件（最小可演示）：
  - UI 出现“薪酬”入口与列表/创建页（对齐 `DEV-PLAN-040` §0.5）。
  - Kernel 写入口（One Door）可复用 `request_id` 幂等，且 RLS fail-closed 可被验证。

### Slice 2（P0-2）：工资条与工资项（Gross Pay）最小可对账闭环

- 计划：`DEV-PLAN-042`
- 目标：生成工资条（payslip）与工资项明细（pay items），至少可从 Assignment 定薪字段计算 `gross_pay` 并在 UI 工资条详情展示明细与汇总。
- 出口条件：
  - `gross_pay/net_pay/employer_total` 可由明细重算（不依赖 JSONB）。
  - 金额精度与舍入合同可测试、可复现。

### Slice 3（P0-3）：社保政策（单城市）配置与扣缴计算

- 计划：`DEV-PLAN-043`
- 目标：支持按 `city_code + validity` 选择社保政策并计算个人/企业扣缴，明细可出现在工资条。
- 出口条件：
  - UI 可配置单城市社保政策并生效（Valid Time 日粒度）。
  - 社保个人扣款进入扣缴明细；企业部分计入 `employer_total` 口径。

### Slice 4（P0-4）：个税累计预扣法（含 YTD balances）

- 计划：`DEV-PLAN-044`
- 目标：实现累计预扣法 + O(1) 的 `payroll_balances` 增量读取；工资条展示本期个税与税后实发。
- 出口条件：
  - 个税负数留抵逻辑可复现；`IIT` 结果不依赖线性回看历史工资条聚合。

### Slice 5（P0-5）：回溯计算（Retroactive Accounting）

- 计划：`DEV-PLAN-045`
- 目标：支持定稿后历史生效变更触发 `payroll_recalc_request`，并在后续周期以补发/扣回差额 pay item 结转闭环（可审计、幂等、不重写已定稿工资条）。
- 出口条件：
  - 已定稿 pay period 后补提交更早生效变更 → 生成请求 → 执行结转 → 差额出现在后续工资条且可追溯来源，并与 balances 口径一致。

### Slice 6（P0-6）：税后发放（仅个税）的 Tax Gross-up（公司承担税金成本）

- 计划：`DEV-PLAN-046`
- 目标：支持工资项 `target_net`（仅扣个税后到手净额）并由公司承担税金成本（含税上税递归）；工资条可解释该项 `gross_amount` 与 `iit_delta`。
- 出口条件：
  - 示例“长期服务奖（税后）=20,000.00”可复现：该项 `net_after_iit == 20,000.00`，且税金成本计入公司成本口径。

## 6. 关键路径（建议优先保证不阻塞）

1. `041(主流程壳) => 042(工资条/工资项) => 043(社保) => 044(个税) => 045(回溯) => 046(税后发放)`

## 7. 依赖草图（Mermaid）

```mermaid
flowchart TD
  Base[平台门禁与基线<br/>012/017/018/019/020/021/024/025] --> P41[041 Pay period & Payroll run]
  P41 --> P42[042 Payslip & Pay items (gross)]
  P42 --> P43[043 Social insurance (single city)]
  P43 --> P44[044 IIT cumulative + balances]
  P44 --> P45[045 Retroactive accounting]
  P45 --> P46[046 Net-guaranteed IIT (tax gross-up)]
```

## 8. 对应计划文档索引（本仓库路径）

- `docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`
- `docs/dev-plans/040-payroll-social-insurance-module-design-blueprint.md`
- `docs/dev-plans/041-payroll-p0-slice-pay-period-and-payroll-run.md`
- `docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`
- `docs/dev-plans/043-payroll-p0-slice-social-insurance-policy-and-calculation.md`
- `docs/dev-plans/044-payroll-p0-slice-iit-cumulative-withholding-and-balances.md`
- `docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md`
- `docs/dev-plans/046-payroll-p0-slice-net-guaranteed-iit-tax-gross-up.md`
