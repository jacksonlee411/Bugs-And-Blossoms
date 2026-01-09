# DEV-PLAN-044：Payroll P0-4——个税累计预扣法（IIT）与 YTD Balances

**状态**: 规划中（2026-01-08 02:40 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 蓝图合同（范围/不变量/算法基线）：`DEV-PLAN-040`（重点：§3.3、§5.2）  
> 依赖：`DEV-PLAN-041/042/043`（主流程载体 + payslip/pay items + 社保扣缴）  
> 细化模板：`DEV-PLAN-001`（本文按其结构补齐到“可直接编码”级别）

## 0. 可执行方案（本计划合同）

> 本节为实施合同；若与后文细节冲突，以本节为准（对齐 `DEV-PLAN-040` 的合同口径）。

### 0.1 背景与上下文

累计预扣法（综合所得）是“历史依赖 + 强确定性”的典型：本期应预扣税额依赖本年度截止上月的累计收入、累计扣除、累计已预扣税额等历史值（`DEV-PLAN-040` §5.2）。

为满足 P0 性能与可复现性，本切片必须落地 **快照式累加器表** `staffing.payroll_balances`（对齐 `DEV-PLAN-040` §3.3），确保：
- 计算某月工资时，读取“上一月的累计值”仅需 O(1)（一行 balances）；
- balances 的更新与 payroll run 的 **定稿（FINALIZE/Posting）同事务** 完成，避免双权威；
- “当月应预扣为负”按税法口径处理为 **留抵税额**，自动结转后续月份，不做“直接退税”。

### 0.2 目标与非目标（P0-4 Slice）

**目标**
- [ ] 冻结累计预扣法输入口径（字段级）：累计收入、累计免税、累计减除费用（按“本 tax_entity 的任职受雇月份数”计入，见 §6.2）、累计专项扣除（含社保/公积金个人部分）、累计专项附加扣除、累计已预扣税额、留抵税额。
- [ ] 落地 `staffing.payroll_balances`（YTD 累加器）并保证 **O(1) 历史读取**：严禁在算薪热路径对历史工资条做线性聚合回看。
- [ ] 实现累计预扣算法（税率表 + 速算扣除数 + 负数留抵）并生成 payslip IIT 明细项，工资条汇总 `net_pay` 与明细聚合一致（可重算）。
- [ ] 将 balances 更新钩入 payroll run `FINALIZE` 的同一事务内（Posting 语义），保证“定稿后可审计、只读、口径稳定”。
- [ ] 交付“专项附加扣除（月度合计）”最小输入载体（事件 SoT + 同事务投射 + internal API），用于复现留抵税额场景（对齐 `DEV-PLAN-040` §5.2 的负数示例）。

**非目标（Out of Scope）**
- 不实现年度汇算清缴全流程与外部申报对接（后续里程碑）。
- 不实现非居民、劳务报酬/稿酬等非综合所得场景；P0 仅覆盖综合所得累计预扣（居民口径）。
- 不引入多 `tax_entity`/多发薪主体维度（P0 视 `tenant` 为单一扣缴义务人；与 `DEV-PLAN-040` §0.2 保持一致）。

### 0.3 工具链与门禁（SSOT 引用）

> 本计划仅声明命中项与 SSOT 链接，不复制命令清单（避免漂移；见 `AGENTS.md`）。

- **触发器（实施阶段将命中）**
  - [ ] Go 代码（`AGENTS.md`）
  - [ ] DB 迁移 / Schema（`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`）
  - [ ] sqlc（若触及 queries/config）（`docs/dev-plans/025-sqlc-guidelines.md`）
  - [ ] 路由治理（新增 internal API）（`docs/dev-plans/017-routing-strategy.md`）
  - [ ] 文档（`make check doc`）
- **SSOT 链接**
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 蓝图合同（金额/舍入/算法边界）：`docs/dev-plans/040-payroll-social-insurance-module-design-blueprint.md`
  - DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
  - 时间语义（Valid Time）：`docs/dev-plans/032-effective-date-day-granularity.md`

### 0.4 关键不变量与失败路径（停止线）

- **One Door**：所有写入必须通过 DB Kernel `staffing.submit_*_event(...)`；应用层禁止直写 `staffing.*` 读模型表（对齐 `AGENTS.md`、`DEV-PLAN-040` §0.2）。
- **No Tx, No RLS**：访问 payroll 表必须显式事务 + `set_config('app.current_tenant', ...)`；缺失即 fail-closed（对齐 `AGENTS.md`、`DEV-PLAN-019/021`）。
- **O(1) 历史读取**：IIT 计算严禁 `SUM(...)` 扫描历史工资条；必须以 `payroll_balances` 单行读取作为历史输入（对齐 `DEV-PLAN-040` §3.3）。
- **累计减除费用月数口径（冻结）**：`ytd_standard_deduction` 不使用“自然月号 × 5000”；而使用 `first_tax_month` 作为起点：`ytd_standard_deduction = 5000.00 * (tax_month - first_tax_month + 1)`；`first_tax_month` 在该税年首次 posting 时写入并保持不变（用于处理年中入职/跨雇主入职）。
- **负数留抵（不得退税）**：当 `本期应预扣税额 < 0` 时，本期 IIT=0，并把差额存为 `ytd_iit_credit`（留抵税额）；后续月份按累计预扣法自动抵扣（对齐 `DEV-PLAN-040` §5.2）。
- **Posting 同事务**：balances 更新必须与 payroll run `FINALIZE` 同事务完成；任何“先定稿后补 balances”的异步补偿都视为双权威（禁止）。
- **金额无浮点**：金额计算使用 `apd.Decimal`；税额最终量化到“分”（2 位小数）；对外/JSON 传输金额一律 string（对齐 `DEV-PLAN-040` §4.2）。
- **P0 币种冻结**：仅支持 `currency='CNY'`；非 CNY 直接失败（稳定错误码）。
- **新增表需确认（红线）**：实现阶段一旦要落地新的 `CREATE TABLE` 迁移，需你在 PR 前明确确认（SSOT：`AGENTS.md`）。

### 0.5 验收标准（Done 口径）

- [ ] 运行至少两个月 pay period：第 2 月 IIT 计算只读取 `staffing.payroll_balances`（一行）作为历史输入，不依赖聚合扫描历史工资条（可在测试/日志中验证）。
- [ ] 构造“留抵税额”用例：通过录入/更新某月专项附加扣除，制造 `累计应纳税额 < 累计已预扣税额`；本月 IIT=0，`ytd_iit_credit` 保存为正数并影响下月。
- [ ] 年中入职/跨雇主入职：首月 `ytd_standard_deduction = 5000.00`（不是 `5000 * 自然月号`）；balances 的 `first_tax_month` 固定并影响后续累计减除费用。
- [ ] 工资条详情展示 IIT 明细项（本期预扣税额），`net_pay` 与明细聚合一致（可重算）。
- [ ] payroll run `FINALIZE` 后 balances 已更新；定稿事务内断言（或集成测试）不出现“run 已 finalized 但 balances 未推进”的状态。

### 0.6 实施步骤（Checklist）

1. [ ] 冻结税基口径：哪些明细计入“累计收入/免税/专项扣除/专项附加扣除”（见 §6.2）。
2. [ ] Schema SSOT：新增 `staffing.payroll_balances`（见 §4.2），并按 `DEV-PLAN-024` 生成迁移闭环。
3. [ ] Schema SSOT：新增 `staffing.iit_special_additional_deduction_claim_events/claims`（见 §4.2），并提供 Kernel 写入口（见 §6.5）。
4. [ ] Go：实现累计预扣引擎（税率表/速算扣除数/负数留抵/舍入点）（见 §6.3）。
5. [ ] 写入工资条：在 `CALC_FINISH` 生成 IIT payslip item（本期预扣税额），并更新 `payslips.net_pay`（对齐 `DEV-PLAN-042` 的“可对账可重算”）。
6. [ ] Posting：在 `FINALIZE` 同事务更新 `staffing.payroll_balances`（见 §6.4）。
7. [ ] Tests：覆盖跨月累计、留抵、社保扣除影响税基、RLS fail-closed、FINALIZE 幂等与一致性（见 §9）。

## 1. 背景与上下文（Context）

本切片在路线图中处于 P0-4（`DEV-PLAN-039`），其输入侧来自：
- `DEV-PLAN-041`：pay period / payroll run 工作流与定稿语义；
- `DEV-PLAN-042`：payslip 与 pay items（gross pay）；
- `DEV-PLAN-043`：社保扣缴明细（个人/企业）与“专项扣除（个人部分）”口径。

其输出侧必须满足：
- 工资条展示 IIT 明细项与税后实发；
- `staffing.payroll_balances` 作为后续月份累计预扣的唯一历史输入（O(1)）；
- 为 `DEV-PLAN-046`（税后发放/税上税）提供可复用 `IIT(base_income + x)` 计算基准；
- 为 `DEV-PLAN-045`（回溯结转）提供可复用 balances 口径，确保差额进入后续周期时累计口径一致。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 本切片交付物清单（必须可落地）

- DB：`staffing.payroll_balances` + RLS（见 §4）。
- DB：`iit_special_additional_deduction_claim_events/claims` + RLS（见 §4）。
- Go：累计预扣算法（税率表 + 速算扣除数 + 留抵 + 舍入合同），并可被 `046` 复用（见 §6.3）。
- Payroll 集成：
  - `CALC_FINISH`：生成 payslip IIT 明细项；
  - `FINALIZE`：Posting（同事务推进 balances）。
- UI：工资条详情展示 IIT 明细项（对齐 `DEV-PLAN-039` 的“可见可操作”出口）。

### 2.2 非目标（Out of Scope）

同 §0.2。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 数据流（Mermaid）

```mermaid
flowchart TD
  Run[Payroll Run] --> Calc[CALC_FINISH: 计算并写 payslips/items]
  Calc --> UI[UI: Payslip 详情展示 IIT]
  Calc --> Finalize[FINALIZE: Posting]
  Finalize --> Bal[payroll_balances: YTD 累加器]
  Bal --> Next[下月计算：O(1) 读取历史]
```

### 3.2 关键设计决策（ADR 摘要）

- **决策 1：累计值权威存放在 `staffing.payroll_balances`**
  - 目的：把历史依赖从 O(N) 扫描降为 O(1) 读取；对齐 `DEV-PLAN-040` §3.3。
- **决策 2：balances 只在 `FINALIZE`（Posting）推进**
  - 目的：允许“计算/重算”不污染 YTD；定稿后口径稳定，可审计；避免双权威。
- **决策 3：负数预扣不退税，存为留抵 `ytd_iit_credit`**
  - 目的：对齐税法与 `DEV-PLAN-040` §5.2；并为后续月份抵扣提供确定性状态。
- **决策 4：税率表与速算扣除数在 P0 固化为常量（可替换）**
  - 目的：减少 P0 迁移与配置复杂度；后续若政策变动再引入版本化配置表并另立 dev-plan。

## 4. 数据模型与约束（Data Model & Constraints）

> 实现期以 `modules/staffing/infrastructure/persistence/schema/*.sql` 为 Schema SSOT，并按 `DEV-PLAN-024` 生成迁移闭环。

### 4.1 表清单（新增）

> 注意：实现落地 `CREATE TABLE` 前需你明确确认（见 §0.4 “新增表需确认”）。

- `staffing.payroll_balances`（累计预扣法 YTD 累加器）
- `staffing.iit_special_additional_deduction_claim_events`（专项附加扣除录入事件 SoT，append-only）
- `staffing.iit_special_additional_deduction_claims`（专项附加扣除当前值投射，按月唯一）

### 4.2 Schema（SQL 合同草案，按现有 staffing 风格对齐）

#### 4.2.1 `staffing.payroll_balances`

```sql
CREATE TABLE IF NOT EXISTS staffing.payroll_balances (
  tenant_id uuid NOT NULL,
  tax_entity_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  first_tax_month smallint NOT NULL, -- 本 tax_year 内首次 posting 的月份（1-12）
  last_tax_month smallint NOT NULL, -- 本 tax_year 内最后一次 posting 的月份（1-12）

  ytd_income numeric(15,2) NOT NULL DEFAULT 0,
  ytd_tax_exempt_income numeric(15,2) NOT NULL DEFAULT 0,
  ytd_standard_deduction numeric(15,2) NOT NULL DEFAULT 0,
  ytd_special_deduction numeric(15,2) NOT NULL DEFAULT 0,
  ytd_special_additional_deduction numeric(15,2) NOT NULL DEFAULT 0,

  ytd_taxable_income numeric(15,2) NOT NULL DEFAULT 0,
  ytd_iit_tax_liability numeric(15,2) NOT NULL DEFAULT 0,
  ytd_iit_withheld numeric(15,2) NOT NULL DEFAULT 0,
  ytd_iit_credit numeric(15,2) NOT NULL DEFAULT 0,

  last_pay_period_id uuid NULL,
  last_run_id uuid NULL,

  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, tax_entity_id, person_uuid, tax_year),
  CONSTRAINT payroll_balances_tax_year_check CHECK (tax_year >= 2000 AND tax_year <= 9999),
  CONSTRAINT payroll_balances_first_month_check CHECK (first_tax_month >= 1 AND first_tax_month <= 12),
  CONSTRAINT payroll_balances_last_month_check CHECK (last_tax_month >= 1 AND last_tax_month <= 12),
  CONSTRAINT payroll_balances_months_order_check CHECK (last_tax_month >= first_tax_month),
  CONSTRAINT payroll_balances_amounts_nonneg_check CHECK (
    ytd_income >= 0 AND ytd_tax_exempt_income >= 0 AND ytd_standard_deduction >= 0
    AND ytd_special_deduction >= 0 AND ytd_special_additional_deduction >= 0
    AND ytd_taxable_income >= 0 AND ytd_iit_tax_liability >= 0 AND ytd_iit_withheld >= 0 AND ytd_iit_credit >= 0
  )
);

CREATE INDEX IF NOT EXISTS payroll_balances_lookup_btree
  ON staffing.payroll_balances (tenant_id, person_uuid, tax_year);
```

说明：
- `tax_entity_id`：扣缴义务人/发薪主体。P0 冻结为 `tax_entity_id = tenant_id`（`DEV-PLAN-040` §3.3），暂不引入多主体维度。
- `first_tax_month`：本 tax_entity 内该员工在本税年的首次 posting 月份；用于累计减除费用月数口径：`month_count = last_tax_month - first_tax_month + 1`。
- `ytd_iit_credit`：留抵税额（累计应纳税额 < 累计已预扣税额时的差额），用于后续月份抵扣；不做“直接退税”。

#### 4.2.2 `staffing.iit_special_additional_deduction_claim_events`

```sql
CREATE TABLE IF NOT EXISTS staffing.iit_special_additional_deduction_claim_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  tax_month smallint NOT NULL,
  amount numeric(15,2) NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT iit_sad_claim_events_tax_year_check CHECK (tax_year >= 2000 AND tax_year <= 9999),
  CONSTRAINT iit_sad_claim_events_tax_month_check CHECK (tax_month >= 1 AND tax_month <= 12),
  CONSTRAINT iit_sad_claim_events_amount_check CHECK (amount >= 0),
  CONSTRAINT iit_sad_claim_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT iit_sad_claim_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS iit_sad_claim_events_lookup_btree
  ON staffing.iit_special_additional_deduction_claim_events (tenant_id, person_uuid, tax_year, tax_month, id);
```

#### 4.2.3 `staffing.iit_special_additional_deduction_claims`

```sql
CREATE TABLE IF NOT EXISTS staffing.iit_special_additional_deduction_claims (
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  tax_month smallint NOT NULL,
  amount numeric(15,2) NOT NULL DEFAULT 0,
  last_event_id bigint NOT NULL REFERENCES staffing.iit_special_additional_deduction_claim_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, person_uuid, tax_year, tax_month),
  CONSTRAINT iit_sad_claims_tax_year_check CHECK (tax_year >= 2000 AND tax_year <= 9999),
  CONSTRAINT iit_sad_claims_tax_month_check CHECK (tax_month >= 1 AND tax_month <= 12),
  CONSTRAINT iit_sad_claims_amount_check CHECK (amount >= 0)
);
```

### 4.3 RLS（必须）

对新增表启用 RLS 并 `FORCE ROW LEVEL SECURITY`，策略对齐 `staffing.positions`：

```sql
ALTER TABLE staffing.iit_special_additional_deduction_claim_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.iit_special_additional_deduction_claim_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.iit_special_additional_deduction_claim_events;
CREATE POLICY tenant_isolation ON staffing.iit_special_additional_deduction_claim_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.iit_special_additional_deduction_claims ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.iit_special_additional_deduction_claims FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.iit_special_additional_deduction_claims;
CREATE POLICY tenant_isolation ON staffing.iit_special_additional_deduction_claims
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payroll_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_balances FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_balances;
CREATE POLICY tenant_isolation ON staffing.payroll_balances
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
```

### 4.4 迁移策略（按 `DEV-PLAN-024`）

- Schema SSOT：建议新增（文件名可调整）
  - `modules/staffing/infrastructure/persistence/schema/00006_staffing_iit_deduction_claims.sql`
  - `modules/staffing/infrastructure/persistence/schema/00007_staffing_payroll_balances.sql`
- 生成迁移：在 `migrations/staffing/` 生成对应 goose 迁移文件，并更新 `migrations/staffing/atlas.sum`；必须保证 `make staffing plan` 最终输出 No Changes。

## 5. 接口契约（API Contracts）

> 本切片不新增核心 UI 路由（沿用 `DEV-PLAN-041/042` 的 payroll UI）；仅补齐展示内容与 internal API（用于测试与排障）。

### 5.1 UI：Payslip 详情（扩展）

在 `GET /org/payroll-runs/{run_id}` 或 payslip 详情页中展示（由 `DEV-PLAN-042` 负责页面承载）：
- 本期 IIT 预扣税额明细项（pay item code 见 §6.2.1）。
- `net_pay`（税后实发）与明细聚合一致。

### 5.2 Internal API（必须，route_class=`internal_api`）

#### `GET /org/api/payroll-balances`

- **Query**
  - `person_uuid`（必填）
  - `tax_year`（必填）
- **Response (200)**
  ```json
  {
    "tenant_id": "uuid",
    "person_uuid": "uuid",
    "tax_year": 2026,
    "first_tax_month": 1,
    "last_tax_month": 2,
    "ytd_income": "20000.00",
    "ytd_special_deduction": "2000.00",
    "ytd_standard_deduction": "10000.00",
    "ytd_taxable_income": "8000.00",
    "ytd_iit_tax_liability": "240.00",
    "ytd_iit_withheld": "240.00",
    "ytd_iit_credit": "0.00"
  }
  ```
- **Errors**
  - 400：参数缺失/无效
  - 404：balances 不存在（可视为本年尚未 posting）

#### `POST /org/api/payroll-iit-special-additional-deductions`

- **用途**：录入/更新“专项附加扣除（月度合计）”，用于演示与测试留抵税额场景（P0 仅提供 internal API；后续若要 UI 入口需另立 dev-plan）。
- **Request**
  ```json
  {
    "event_id": "uuid",
    "person_uuid": "uuid",
    "tax_year": 2026,
    "tax_month": 2,
    "amount": "10000.00",
    "request_id": "client-generated-string (optional)"
  }
  ```
- **Response (200)**
  ```json
  { "event_id": "uuid", "person_uuid": "uuid", "tax_year": 2026, "tax_month": 2, "amount": "10000.00", "request_id": "client-generated-string" }
  ```
- **幂等**
  - 以 `event_id` 作为幂等键：重复提交同一 `event_id` 必须幂等成功。
  - 若 `event_id` 被复用但 payload 不一致，则返回稳定错误码 `STAFFING_IDEMPOTENCY_REUSED`。
  - 若 `request_id` 缺省，服务端应使用 `event_id` 的 string 作为 `request_id` 传入 Kernel（避免 request_id 漂移）。
- **Errors**
  - 400：参数缺失/无效
  - 409：幂等冲突（`STAFFING_IDEMPOTENCY_REUSED`）
  - 409：该月已定稿（`STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED`）
- **Routing（治理约束）**
  - route_class 必须为 `internal_api`（对齐 `DEV-PLAN-017`）。
  - 实施时需同步更新 `config/routing/allowlist.yaml`，否则 routing gates 会阻断合并（SSOT：`AGENTS.md`）。

## 6. 核心逻辑与算法（Business Logic & Algorithms）

### 6.1 时序与职责边界（冻结）

- `CALC_FINISH`（计算阶段）：生成/刷新 payslip 与明细项（包含 IIT 明细项）；允许在 finalized 前重复计算与覆盖结果（对齐 `DEV-PLAN-041/042` 的语义）。
- `FINALIZE`（Posting 阶段）：**同事务**推进 `staffing.payroll_balances`；finalized 后结果只读。

### 6.2 税基口径冻结（P0 最小集合）

> 本节冻结“哪些金额进入 IIT 口径”。P0 先用最小集合交付可见闭环，后续扩展需另立 dev-plan。

- **累计收入（ytd_income）**
  - P0 取 `payslips.gross_pay` 作为当月收入增量（默认均为应税收入）。
- **累计免税（ytd_tax_exempt_income）**
  - P0 固定为 0（保留字段便于后续扩展）。
- **累计减除费用（ytd_standard_deduction）**
  - P0 固定标准：`5000.00 CNY / 月`（居民综合所得口径）。
  - 月数口径（冻结）：按“本 tax_entity 内本 tax_year 的任职受雇月份数”计入。
    - `tax_month`：取 pay period 的 `lower(period)` 的月份（1-12）。
    - `first_tax_month`：首次 posting 的月份（写入 `payroll_balances.first_tax_month`，首次写入后保持不变）。
    - `month_count = tax_month - first_tax_month + 1`。
    - `ytd_standard_deduction = 5000.00 * month_count`。
- **累计专项扣除（ytd_special_deduction）**
  - 口径冻结为“社保/公积金个人扣款合计”（由 `DEV-PLAN-043` 提供月度个人扣款合计，并落到 payslip 明细可聚合口径）。
- **累计专项附加扣除（ytd_special_additional_deduction）**
  - P0 支持“月度合计”录入（internal API + Kernel 写入口，见 §6.5）；若未录入则默认为 0。
  - 说明：P0 只承载“合计金额”，不拆分子女教育/房贷利息等明细分类；分类化输入需另立 dev-plan 承接。

#### 6.2.1 Payslip 明细项（冻结：pay item code 与归类）

> 对齐 `DEV-PLAN-042` 的 pay item 归类：earnings/deductions/employer_costs；本节冻结 IIT 在 payslip 中的最小表达，避免实现期发明第二套口径。

- **IIT 预扣（员工承担）**
  - `code`: `DEDUCTION_IIT_WITHHOLDING`
  - `category`: deductions
  - `amount`: `numeric(15,2)`（正数表示“扣减金额”）
  - 记账口径：参与 `net_pay` 计算（`net_pay = gross_pay - Σ(deductions)`，其中包含社保个人扣款与 IIT 预扣）。

### 6.3 累计预扣算法（冻结）

#### 6.3.1 税率表（P0 固化常量，可替换）

按综合所得年度税率表（税率、速算扣除数）：

| 累计应纳税所得额（元） | 税率 | 速算扣除数（元） |
| --- | --- | --- |
| ≤ 36,000 | 0.03 | 0 |
| 36,000 - 144,000 | 0.10 | 2,520 |
| 144,000 - 300,000 | 0.20 | 16,920 |
| 300,000 - 420,000 | 0.25 | 31,920 |
| 420,000 - 660,000 | 0.30 | 52,920 |
| 660,000 - 960,000 | 0.35 | 85,920 |
| > 960,000 | 0.45 | 181,920 |

#### 6.3.2 计算公式（对齐 `DEV-PLAN-040` §5.2）

设：
- `I_ytd`：累计收入（`ytd_income`）
- `E_ytd`：累计免税（`ytd_tax_exempt_income`）
- `S_ytd`：累计减除费用（`ytd_standard_deduction`）
- `SD_ytd`：累计专项扣除（`ytd_special_deduction`）
- `SAD_ytd`：累计专项附加扣除（`ytd_special_additional_deduction`）
- `W_ytd`：累计已预扣税额（`ytd_iit_withheld`）

则：
1. `TI_ytd = max(0, I_ytd - E_ytd - S_ytd - SD_ytd - SAD_ytd)`（累计应纳税所得额，负数按 0 处理）
2. `T_ytd = round2( TI_ytd * rate(TI_ytd) - quick_deduction(TI_ytd) )`（累计应纳税额，量化到分）
3. `Δ = T_ytd - W_ytd`（本期应预扣税额，可能为负）
4. 本期预扣税额与留抵：
   - 若 `Δ > 0`：`iit_withhold_this_month = Δ`，`ytd_iit_credit = 0`
   - 若 `Δ <= 0`：`iit_withhold_this_month = 0`，`ytd_iit_credit = -Δ`

注：
- `round2`：量化到 2 位小数（分），采用 `apd` 的确定性舍入（P0 默认 `RoundHalfUp`；若后续统一舍入枚举落地，以统一口径为准）。

### 6.4 Posting：在 `FINALIZE` 同事务更新 balances

在 `staffing.submit_payroll_run_event(..., event_type='FINALIZE')` 的事务内（对齐 `DEV-PLAN-041` §6.2 的 FINALIZE 语义），对本 run 的每张 payslip 执行：

1. 解析 `tax_year/tax_month`：从 pay period 的 `lower(period)` 得到。
   - P0 冻结：pay period 必须为自然月区间（`[YYYY-MM-01, next_month_01)`）；否则 FINALIZE 必须失败并返回稳定错误码（见 §6.6）。
2. `SELECT ... FOR UPDATE` 锁定该 `(tenant_id,tax_entity_id,person_uuid,tax_year)` 的 balances 行（不存在则视为 0 初始化；插入新行时将 `first_tax_month = tax_month`）。
3. 计算本月增量（按 §6.2 冻结口径）并按 §6.3 计算本月预扣税额。
4. 校验：本月计算得到的 `iit_withhold_this_month` 必须与 payslip 中 IIT 明细项金额一致；不一致则 FINALIZE 失败并返回稳定错误码（要求先重新 CALC）。
5. `UPSERT` 写回 balances（推进 `last_tax_month`、更新 YTD 字段、写 `last_pay_period_id/last_run_id`；`first_tax_month` 不得被更新）。

补充冻结点（避免实现期“猜”）：
- **month 推进语义**：`last_tax_month` 必须单调递增；若 `tax_month <= last_tax_month` 且不是同一 `run_id` 的幂等重放，则必须失败（稳定错误码见 §6.6）。
- **SAD 月度合计读取**：Posting 时仅读取当月 `iit_special_additional_deduction_claims.amount`（不存在视为 0），并更新：
  - `ytd_special_additional_deduction = prev_ytd_special_additional_deduction + sad_amount_this_month`
  - `ytd_standard_deduction = 5000.00 * (tax_month - first_tax_month + 1)`（`first_tax_month` 在首次 posting 写入；后续月份不得变化）

并保证：
- 与 run 的 `finalized_at`、pay period 的 `closed_at` 同事务提交；
- FINALIZE 幂等：重复调用同一 `event_id` 不产生重复推进（幂等模式对齐现有 `submit_*_event`）。

### 6.5 Kernel：`submit_iit_special_additional_deduction_claim_event`（新增，P0 最小写入口）

> 目的：遵守 One Door（事件 SoT + 同事务投射），并为“留抵税额”用例提供可复现输入。

**签名（建议）**

```sql
SELECT staffing.submit_iit_special_additional_deduction_claim_event(
  p_event_id     => $1::uuid,
  p_tenant_id    => $2::uuid,
  p_person_uuid  => $3::uuid,
  p_tax_year     => $4::int,
  p_tax_month    => $5::smallint,
  p_amount       => $6::numeric,
  p_request_id   => $7::text,
  p_initiator_id => $8::uuid
);
```

**算法（必须）**
1. `assert_current_tenant(p_tenant_id)`；校验参数（`tax_month` 1-12、`amount >= 0`、必填字段非空）。
2. `pg_advisory_xact_lock(hashtextextended(format('staffing:iit:sad:%s:%s:%s:%s', tenant_id, person_uuid, tax_year, tax_month),0))` 串行化同一月度记录的写入。
3. 写入 `iit_special_additional_deduction_claim_events`（`ON CONFLICT(event_id) DO NOTHING`；幂等对比不一致抛 `STAFFING_IDEMPOTENCY_REUSED`）。
4. 投射到 `iit_special_additional_deduction_claims`：
   - `INSERT ... ON CONFLICT (tenant_id, person_uuid, tax_year, tax_month) DO UPDATE SET amount=EXCLUDED.amount, last_event_id=... , updated_at=now()`

约束：
- 若该 `tax_year/tax_month` 已存在 finalized payroll run（P0 简化口径：存在即可），写入应失败并返回稳定错误码（避免定稿后悄悄改税基，错误码见 §6.6）。

### 6.6 稳定错误码（冻结，供实现与测试对齐）

> 说明：错误码字符串遵循现有 Kernel 风格（`MESSAGE = 'STAFFING_...'`），避免实现期散落硬编码字符串。

- `STAFFING_IIT_PERIOD_NOT_MONTHLY`：pay period 不是自然月区间（IIT P0 仅支持自然月）。
- `STAFFING_IIT_BALANCES_MONTH_NOT_ADVANCING`：`tax_month <= last_tax_month`（非幂等重放），禁止倒序/重复 posting。
- `STAFFING_IIT_PAYSLIP_ITEM_MISSING`：FINALIZE 时未找到 `DEDUCTION_IIT_WITHHOLDING` 明细项（要求先 CALC）。
- `STAFFING_IIT_WITHHOLDING_MISMATCH_RECALC_REQUIRED`：FINALIZE 重新计算得到的 `iit_withhold_this_month` 与 payslip 明细项不一致（要求先重新 CALC）。
- `STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED`：该月已存在 finalized payroll run，禁止录入/更新 SAD 月度合计。
- `STAFFING_IDEMPOTENCY_REUSED`：同一 `event_id` 被复用但 payload 不一致。

## 7. 安全与鉴权（Security & Authz）

- UI：沿用 `DEV-PLAN-041` 的 authz 对象与动作（`staffing.payroll-runs` 等）；IIT 仅作为 run 计算结果展示，不新增额外授权对象。
- Internal API（本计划新增的两个 endpoint）权限口径冻结为：
  - `GET /org/api/payroll-balances`：`Object=staffing.payroll-runs`、`Action=read`
  - `POST /org/api/payroll-iit-special-additional-deductions`：`Object=staffing.payroll-runs`、`Action=admin`
- 数据隔离：balances 与 payslip 结果表全部启用 RLS；应用层不得绕过 RLS 进入业务链路。

## 8. 依赖与里程碑（Dependencies & Milestones）

### 8.1 依赖

- 上游合同与路线图：`DEV-PLAN-039/040`。
- 主流程载体：`DEV-PLAN-041`（尤其 `FINALIZE` 语义与 Kernel 幂等）。
- payslip/pay items：`DEV-PLAN-042`。
- 社保扣缴（专项扣除口径）：`DEV-PLAN-043`。
- 门禁：`DEV-PLAN-012/017/024/025`，以及 `AGENTS.md` 触发器矩阵。

### 8.2 里程碑（实现顺序建议）

1. [ ] Schema SSOT：新增 `iit_special_additional_deduction_claims`（events+projection）+ RLS。
2. [ ] Schema SSOT：新增 `payroll_balances` + RLS。
3. [ ] 迁移闭环：`migrations/staffing/*` + `atlas.sum`（`make staffing plan` No Changes）。
4. [ ] Go：IIT 引擎（纯函数）+ 单元测试（含留抵）。
5. [ ] 集成：CALC 写 IIT 明细项；FINALIZE Posting 推进 balances（含一致性校验）。
6. [ ] UI：工资条展示 IIT 明细项与税后实发（可对账）。

## 9. 测试与验收（Acceptance Criteria）

### 9.1 单元测试（必须）

- [ ] 税率表：各档位边界（36,000/144,000/...）选择正确。
- [ ] 累计减除费用月数：`ytd_standard_deduction = 5000.00 * (tax_month - first_tax_month + 1)`；覆盖年中入职/跨雇主入职（`first_tax_month > 1`）。
- [ ] 负数留抵：通过专项附加扣除（SAD）输入制造 `Δ <= 0`，本期 IIT=0，`ytd_iit_credit > 0`。
- [ ] 舍入：税额在分粒度确定性一致（禁止 float）。

### 9.2 集成测试（必须）

- [ ] 两个月跑通：月 2 计算读取 balances（上一月）并正确影响本月 IIT。
- [ ] 年中入职：首月 FINALIZE 后 `payroll_balances.first_tax_month = tax_month` 且 `ytd_standard_deduction = 5000.00`。
- [ ] FINALIZE 同事务：finalize 后 balances 已推进；若 FINALIZE 失败则 balances 不变（事务原子性）。
- [ ] RLS fail-closed：未设置 `app.current_tenant` 时，对 balances/工资条的读写均失败。

### 9.3 验收脚本（建议）

1. 创建两个月 pay period（`monthly`）。
2. 月 1：计算并定稿；检查 balances 已生成。
3. 月 2：计算并定稿；检查 IIT 计算读取 balances 且结果受累计影响。

## 10. 运维与监控（Ops & Monitoring）

- P0-4 不引入额外监控/开关；仅要求关键日志可定位（建议包含 `tenant_id`, `run_id`, `person_uuid`, `tax_year`, `tax_month`）。

## 11. 备注（与后续切片的边界）

- `DEV-PLAN-046`：税后发放（仅个税）将复用本切片的 IIT 引擎作为 `ΔIIT(x)` 的计算基准（对齐 `DEV-PLAN-040` §5.2.1）。
- `DEV-PLAN-045`：回溯结转会复用本切片的 balances 口径，确保差额进入后续周期时累计口径一致（对齐 `DEV-PLAN-040` §5.3）。
