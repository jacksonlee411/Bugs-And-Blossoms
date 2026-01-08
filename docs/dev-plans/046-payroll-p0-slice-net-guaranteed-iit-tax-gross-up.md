# DEV-PLAN-046：Payroll P0-6——税后发放（仅个税）的 Tax Gross-up（公司承担税金成本）

**状态**: 草拟中（2026-01-08 05:40 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 蓝图合同（口径/算法合同）：`DEV-PLAN-040` §5.2.1  
> 细化模板：`DEV-PLAN-001`（本文按其结构补齐到“可直接编码”级别）  
> 依赖：`DEV-PLAN-041/042/043/044/045`（主流程载体 + payslip/items 明细 + 社保扣缴 + IIT 累计预扣与 balances + 回溯结转）

## 0. 可执行方案（本计划合同）

> 本节为实施合同；若与后文细节冲突，以本节为准（对齐 `DEV-PLAN-040` 的合同口径）。

### 0.1 背景与上下文

业务要求：HR 可能需要配置某工资项“税后到手=指定净额”（例如长期服务奖 20,000.00）。系统必须保证该工资项在**仅扣除个税（IIT）**后员工到手金额精确等于 `target_net`；为实现该净额保证而产生的全部税金成本（税金成本本质上属于员工收入，因而会触发“税上税”递归）由公司承担（对齐 `DEV-PLAN-040` §5.2.1）。

本切片的关键在于：
- **确定性求解**：在“分（0.01）”粒度内用确定性算法求解（禁止 float、禁止一次近似）。
- **可审计、可解释**：工资条必须能解释每个净额保证项的 `target_net / gross_amount / iit_delta`，且结果可复算（输入/舍入合同冻结）。
- **复用同一 IIT 引擎**：`ΔIIT(x)` 必须直接复用 `DEV-PLAN-044` 的累计预扣引擎（同一舍入合同与 balances 口径），禁止另写一套“简化税表”。

### 0.2 目标与非目标（P0-6 Slice）

**目标**
- [ ] **输入语义冻结**：工资项显式声明 `tax_bearer=employer` 且 `calc_mode=net_guaranteed_iit`，并提供 `target_net`（币种与工资条一致，P0 固定 `CNY`）。
- [ ] **组级求解冻结**：对同一工资条中所有净额保证项合并为一个 group，求解 `x - ΔIIT(x) = T`：
  - `T = Σ target_net`
  - `ΔIIT(x) = IIT(base_income + x) - IIT_base`
  - 全过程以“分”为整数求解，确保收敛与可复现。
- [ ] **分摊合同冻结**：将 `ΔIIT(x)` 按 `target_net` 比例确定性分摊到各工资项：
  - 每项 `tax_i`（即 `iit_delta`）满足 `Σ tax_i = ΔIIT(x)`
  - 每项 `gross_i = target_net_i + tax_i`
  - 展示/计算意义上的逐项 `net_after_iit_i = gross_i - tax_i = target_net_i`。
- [ ] **工资条可解释**：工资条详情展示每项的 `target_net / gross_amount / iit_delta / net_after_iit`，并能看出“公司承担税金成本”的口径。
- [ ] **公司成本口径**：净额保证导致的增量税金成本必须可被审计与汇总（至少可从 payslip items 明细重算；对齐 `DEV-PLAN-039/040` 的“可对账汇总”原则）。

**非目标（Out of Scope）**
- 不覆盖社保/公积金等扣款的“税后净额保证”（口径已在 `DEV-PLAN-040` 冻结：仅个税）。
- 不在 P0 引入通用表达式引擎、通用规则 DSL（对齐 `DEV-PLAN-040` §4.3）。

### 0.3 工具链与门禁（SSOT 引用）

> 本计划仅声明命中项与 SSOT 链接，不复制命令清单（避免漂移；见 `DEV-PLAN-000`）。

- **触发器（实施阶段将命中）**
  - [ ] Go 代码（`AGENTS.md`）
  - [ ] DB 迁移 / Schema（`DEV-PLAN-024`）
  - [ ] sqlc（若触及 queries/config；`DEV-PLAN-025`）
  - [ ] 路由治理（`DEV-PLAN-017`；需更新 `config/routing/allowlist.yaml`）
  - [ ] Authz（`DEV-PLAN-022`；需更新 `pkg/authz/registry.go` 与 `internal/server/authz_middleware.go`）
  - [ ] 文档（`make check doc`）
- **SSOT 链接**
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`
  - DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
  - 路由策略：`docs/dev-plans/017-routing-strategy.md`
  - 金额精度与舍入合同：`docs/dev-plans/040-payroll-social-insurance-module-design-blueprint.md` §4.2

### 0.4 关键不变量与失败路径（停止线）

- **No Legacy**：不得引入任何回退通道/双链路（对齐 `AGENTS.md` 与 `DEV-PLAN-004M1`）。
- **One Door**：净额保证工资项的录入与删除必须走 DB Kernel `submit_*_event(...)`；禁止应用层直写输入表。
- **No Tx, No RLS**：缺少 tenant context 直接失败（fail-closed）；不得用 superuser/bypass RLS 跑业务链路。
- **确定性求解**：禁止 float；禁止一次近似；必须在“分”粒度内收敛或失败（稳定错误码）。
- **禁止静默降级**：求解不收敛/上界无法扩展/舍入合同不一致 → 必须失败并给出稳定错误码；不得退回“按经验系数”或“强行取近似”。
- **显式声明**：仅当 `tax_bearer=employer` 且 `calc_mode=net_guaranteed_iit` 时启用；默认税负仍由员工承担。
- **新增表需确认（红线）**：实现阶段一旦需要落地新的 `CREATE TABLE` 迁移，需你在 PR 前明确确认（SSOT：`AGENTS.md`）。

### 0.5 验收标准（Done 口径）

- [ ] 在同一工资条中录入“长期服务奖（税后）=20,000.00”，计算后该项 `net_after_iit == 20,000.00`（精确到分）。
- [ ] 同一工资条存在 2 个净额保证项时：两项各自 `net_after_iit == target_net`，且 `Σ iit_delta == ΔIIT(group_gross)`。
- [ ] 工资条可解释展示 `target_net / gross_amount / iit_delta / net_after_iit`，且公司成本口径可从明细重算。
- [ ] 求解不收敛时：计算失败并返回稳定错误码（UI 可见），不得生成“看起来能跑”的结果。

## 1. 背景与上下文（Context）

本切片位于 `DEV-PLAN-039` 的 Slice 6（P0-6）：在 `DEV-PLAN-044` 已交付累计预扣 IIT 引擎与 balances、并且 payslip/items 已可对账的前提下，实现“净额保证（仅个税）”的 Tax Gross-up。

为避免歧义，本文冻结以下名词：
- `target_net`：该工资项**仅扣 IIT 后**员工到手金额目标（不含社保/公积金/其他扣款）。
- `gross_amount`：该工资项计入计税收入的总额（员工税前收入）；净额保证项的 `gross_amount >= target_net`。
- `iit_delta`：对“净额保证组”产生的**本期增量预扣税额**按 `target_net` 比例分摊到该项的份额（展示/审计用）。
- `net_after_iit`：展示意义上的逐项净额，定义为 `gross_amount - iit_delta`（必须等于 `target_net`）。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 本切片交付物清单（必须可落地）

- DB：净额保证工资项**输入载体**（事件 SoT + 同事务投射）与必要的列/约束；RLS/幂等/错误码齐全。
- Payroll 计算：在 IIT pipeline 内实现 group-level solver + 分摊，并写入 payslip items（可复算可解释）。
- UI：工资条详情中提供“净额保证（仅个税）”录入入口与结果展示（可发现、可操作）。
- Tests：至少包含 solver/分摊的纯逻辑单测 + 端到端（payslip 场景）集成测试用例。

### 2.2 明确不做（避免范围漂移）

- 不支持“净额保证覆盖社保/公积金/其他扣款”的口径。
- 不支持多币种、多税制并存；P0 固定 `CNY` + 中国综合所得累计预扣法。
- 不实现批量导入/批量录入；P0 允许先以单人/单工资条录入为主。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 决策：组级求解（选定）

**选项 A（不选）**：逐项独立求解 `gross_i - ΔIIT_i(gross_i) = target_net_i`。缺点：累计预扣税是按总收入计算，逐项定义 `ΔIIT_i` 会引入“顺序依赖/非唯一分摊”。

**选项 B（选定）**：将同一工资条的全部净额保证项合并为一个 group：
- 先算 `IIT_base`（不含 group）
- 定义 `ΔIIT(x)` 并求解 `x - ΔIIT(x) = T`
- 再按 `target_net` 比例分摊 `ΔIIT(x)`  
优点：与 `DEV-PLAN-040` 合同一致、可解释、可复算、分摊唯一且确定。

### 3.2 计算位置（选定）

净额保证求解属于 **payroll 计算引擎（Go）** 的职责：
- 需要高频调用 IIT 引擎作为黑盒函数 `IIT(...)`；在 Go 中实现最自然。
- DB Kernel 负责写入口唯一与投射，不负责复杂数值求解（保持 Kernel 简洁）。

### 3.3 组件图（Mermaid）

```mermaid
flowchart TD
  UI[HTMX UI<br/>payslip detail] --> H[internal/server handlers]
  H --> S[PayrollStore (PG)]
  S -->|tx + set_config(app.current_tenant)| K[DB Kernel<br/>submit_payslip_item_input_event]
  K --> E[(payslip_item_input_events)]
  K --> I[(payslip_item_inputs)]

  C[Payroll Calc (Go)] -->|load inputs+items+balances| S
  C -->|call IIT engine| T[IIT Engine<br/>DEV-PLAN-044]
  C -->|write payslip items| K2[DB Kernel<br/>submit_payroll_run_event]
  K2 --> R[(payslip_items/payslips/balances)]
```

## 4. 数据模型与约束（Data Model & Constraints）

> 说明：本节只定义本切片新增/调整的合同。`pay_periods/payroll_runs/payslips` 的基础表合同见 `DEV-PLAN-041`；`payslip_items` 的基础表合同由 `DEV-PLAN-042/043/044/045` 承接，本切片只追加“净额保证所需字段”。

### 4.1 表/字段变更清单（本切片）

> 注意：实现落地 `CREATE TABLE` 前需你明确确认（见 §0.4 “新增表需确认”）。

**新增（输入载体）**
- `staffing.payslip_item_input_events`（SoT）
- `staffing.payslip_item_inputs`（读模型/投射）

**调整（输出明细）**
- `staffing.payslip_items`：追加 `calc_mode/tax_bearer/target_net/iit_delta`（若尚未存在）

### 4.2 Schema（SQL 合同草案）

#### 4.2.1 `staffing.payslip_item_input_events`（SoT）

```sql
CREATE TABLE IF NOT EXISTS staffing.payslip_item_input_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,

  -- “输入归属 payslip”使用 natural key（避免依赖 payslip_id 稳定性）
  run_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,

  event_type text NOT NULL, -- UPSERT / DELETE

  item_code text NOT NULL,
  item_name text NOT NULL,
  direction text NOT NULL,  -- earning / deduction / employer_cost（P0-6 只允许 earning）

  currency char(3) NOT NULL DEFAULT 'CNY',
  calc_mode text NOT NULL,   -- amount / net_guaranteed_iit
  tax_bearer text NOT NULL,  -- employee / employer

  -- amount 语义：
  -- - calc_mode=amount：amount 为该输入项的金额（税前，正数）
  -- - calc_mode=net_guaranteed_iit：amount 为 target_net（仅扣 IIT 后净额目标，正数）
  amount numeric(15,2) NOT NULL,

  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT payslip_item_input_events_event_type_check CHECK (event_type IN ('UPSERT','DELETE')),
  CONSTRAINT payslip_item_input_events_code_check CHECK (btrim(item_code) <> '' AND item_code = btrim(item_code) AND item_code = upper(item_code) AND item_code ~ '^[A-Z0-9_]+$'),
  CONSTRAINT payslip_item_input_events_name_check CHECK (btrim(item_name) <> '' AND item_name = btrim(item_name)),
  CONSTRAINT payslip_item_input_events_direction_check CHECK (direction IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_item_input_events_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_item_input_events_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_item_input_events_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_item_input_events_amount_positive_check CHECK (amount > 0),
  CONSTRAINT payslip_item_input_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT payslip_item_input_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT payslip_item_input_events_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payslip_item_input_events_lookup_btree
  ON staffing.payslip_item_input_events (tenant_id, run_id, person_uuid, assignment_id, item_code, id);
```

#### 4.2.2 `staffing.payslip_item_inputs`（投射/读模型）

```sql
CREATE TABLE IF NOT EXISTS staffing.payslip_item_inputs (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),

  run_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,

  item_code text NOT NULL,
  item_name text NOT NULL,
  direction text NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  calc_mode text NOT NULL,
  tax_bearer text NOT NULL,
  amount numeric(15,2) NOT NULL,

  last_event_id bigint NOT NULL REFERENCES staffing.payslip_item_input_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payslip_item_inputs_code_check CHECK (btrim(item_code) <> '' AND item_code = btrim(item_code) AND item_code = upper(item_code) AND item_code ~ '^[A-Z0-9_]+$'),
  CONSTRAINT payslip_item_inputs_name_check CHECK (btrim(item_name) <> '' AND item_name = btrim(item_name)),
  CONSTRAINT payslip_item_inputs_direction_check CHECK (direction IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_item_inputs_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_item_inputs_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_item_inputs_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_item_inputs_amount_positive_check CHECK (amount > 0),
  CONSTRAINT payslip_item_inputs_natural_unique UNIQUE (tenant_id, run_id, person_uuid, assignment_id, item_code),
  CONSTRAINT payslip_item_inputs_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payslip_item_inputs_by_run_person_btree
  ON staffing.payslip_item_inputs (tenant_id, run_id, person_uuid, assignment_id, item_code);
```

#### 4.2.3 `staffing.payslip_items`（净额保证所需字段增补）

> 说明：`payslip_items` 的基础 schema 由 `DEV-PLAN-042+` 定义；本切片只要求其具备如下字段与约束（可用 `ALTER TABLE` 增补）。

```sql
-- 仅示意：若字段已存在则跳过；最终以 Schema SSOT 为准（DEV-PLAN-024）。
ALTER TABLE staffing.payslip_items
  ADD COLUMN IF NOT EXISTS calc_mode text NOT NULL DEFAULT 'amount',
  ADD COLUMN IF NOT EXISTS tax_bearer text NOT NULL DEFAULT 'employee',
  ADD COLUMN IF NOT EXISTS target_net numeric(15,2) NULL,
  ADD COLUMN IF NOT EXISTS iit_delta numeric(15,2) NULL;

ALTER TABLE staffing.payslip_items
  ADD CONSTRAINT payslip_items_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  ADD CONSTRAINT payslip_items_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  ADD CONSTRAINT payslip_items_iit_delta_nonneg_check CHECK (iit_delta IS NULL OR iit_delta >= 0),
  ADD CONSTRAINT payslip_items_target_net_positive_check CHECK (target_net IS NULL OR target_net > 0),
  ADD CONSTRAINT payslip_items_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      tax_bearer = 'employer'
      AND direction = 'earning'
      AND target_net IS NOT NULL
      AND iit_delta IS NOT NULL
      AND amount = target_net + iit_delta
    )
  );
```

### 4.3 RLS（必须）

对新增的 `payslip_item_input_events/payslip_item_inputs` 启用 RLS 并 `FORCE ROW LEVEL SECURITY`，策略同 `staffing.positions`：

```sql
ALTER TABLE staffing.payslip_item_input_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_item_input_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_item_input_events;
CREATE POLICY tenant_isolation ON staffing.payslip_item_input_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payslip_item_inputs ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_item_inputs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_item_inputs;
CREATE POLICY tenant_isolation ON staffing.payslip_item_inputs
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
```

### 4.4 迁移策略（按 `DEV-PLAN-024`）

- **Schema SSOT**：建议新增
  - `modules/staffing/infrastructure/persistence/schema/0000X_staffing_payroll_inputs.sql`
- **生成迁移**：在 `migrations/staffing/` 生成对应 goose 迁移文件，并更新 `migrations/staffing/atlas.sum`；必须保证 `make staffing plan` 最终输出 No Changes。

## 5. 接口契约（API Contracts）

> UI 为 HTML + HTMX；输入写入必须走 Kernel（One Door）；错误必须稳定可见。

### 5.1 UI：工资条详情（录入净额保证项）

#### `GET /org/payroll-runs/{run_id}/payslips/{payslip_id}`
- **用途**：展示工资条明细与“净额保证（仅个税）”输入区。
- **展示要求**
  - 若 run 已 `finalized`：输入区隐藏或禁用（只读）。
  - 若存在净额保证项：展示 `target_net / gross_amount / iit_delta / net_after_iit`。

#### `POST /org/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items`
- **用途**：新增或更新一条净额保证输入项（写入 `payslip_item_inputs` 的投射）。
- **Form Fields**
  - `item_code`（必填，建议 P0 固定：`EARNING_LONG_SERVICE_AWARD`）
  - `target_net`（必填，string 金额，精确到分，例如 `20000.00`）
  - `request_id`（必填，幂等）
- **固定语义（服务端强制）**
  - `direction=earning`
  - `calc_mode=net_guaranteed_iit`
  - `tax_bearer=employer`
  - `currency=CNY`
  - `item_name`：P0 可固定为“长期服务奖（税后）”（en/zh 由 i18n 承接）
- **成功**：303 跳转回 `GET /org/payroll-runs/{run_id}/payslips/{payslip_id}`。
- **失败（422）**：回显表单错误（稳定错误码映射见 §6.6）。
- **额外行为（冻结）**：若当前 run 状态为 `calculated` 且允许编辑输入，则需在同一事务内将 `payroll_runs.needs_recalc=true`（对齐 `DEV-PLAN-045` 的“需要重算”语义），并在 UI 提示“需重新计算”。

### 5.2 internal API（便于测试/排障，最小）

> internal API 的具体路径格式需与 `internal/routing` 的 pattern 语法兼容（param 必须是完整 segment：`/{id}`）。

- `POST /org/api/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items` → `200 {ok:true}`

## 6. 核心逻辑与算法（Business Logic & Algorithms）

### 6.1 输入校验（必须，失败返回稳定错误码）

对每条净额保证输入项（calc_mode=net_guaranteed_iit）：
- `target_net > 0`
- `currency == 'CNY'`（P0 冻结）
- `tax_bearer == employer`
- `direction == earning`
- `item_code`/`item_name` 非空且 trim

对同一工资条的净额保证组：
- `T = Σ target_net` 必须 > 0
- 组内币种必须一致（P0 即 `CNY`）

### 6.2 基线与增量税函数定义（冻结）

对某一 payslip（同一 tax_year）：

1. 计算不含净额保证组的基线本期预扣税额：
- `IIT_base = IIT(base_income)`

2. 定义增量预扣税函数：
- `ΔIIT(x) = IIT(base_income + x) - IIT_base`

其中：
- `base_income` 为本期计税收入口径（由 `DEV-PLAN-044` 冻结），但**不包含**净额保证组的收入。
- `x` 为净额保证组的计税收入总额（gross）。

### 6.3 求解目标（冻结）

净额保证组目标净额：`T = Σ target_net`。

求解 `x` 使得：

`x - ΔIIT(x) = T`

解释：净额保证组增加了 `x` 的税前收入，同时导致本期预扣税额增加 `ΔIIT(x)`；因此该组对员工净额的贡献是 `x - ΔIIT(x)`，必须精确等于目标 `T`。

### 6.4 求解算法（确定性，分粒度）

约束：实现必须以“分”为整数求解（`int64 cents`），并满足：
- 收敛：在有限步内得到解
- 确定性：同一输入 → 同一输出
- 可审计：输出包含 `x` 与 `ΔIIT(x)`，可复算

**伪代码（冻结）**

```text
T = sum(target_net_cents)
baseIIT = IIT(base_income_cents)

function deltaIIT(x):
  return IIT(base_income_cents + x) - baseIIT

function netFromX(x):
  return x - deltaIIT(x)

lo = T
hi = T
for expand in 1..32:
  if netFromX(hi) >= T:
    break
  hi = hi * 2
if netFromX(hi) < T:
  fail SOLVER_UPPER_BOUND_EXHAUSTED

while lo < hi:
  mid = (lo + hi) / 2
  if netFromX(mid) >= T:
    hi = mid
  else:
    lo = mid + 1

x = lo
D = deltaIIT(x)
if x - D != T:
  fail SOLVER_CONTRACT_VIOLATION
```

> 说明：为保证“最小满足 `>=T` 的 x ⇒ 必然 `==T`”，本切片对 `DEV-PLAN-044` 的 IIT 引擎提出一条**接口级合同**：
> - `IIT(...)` 的输入与输出都必须在“分（0.01）”粒度量化，且对同一输入**确定性**（对齐 `DEV-PLAN-040` §4.2 的舍入合同）。
> - 对固定的 `base_income`，当 `x` 以 1 分递增时，`ΔIIT(x+1)-ΔIIT(x)` 必须属于 `{0,1}`（边界由“最高边际税率 < 100% + 分粒度量化”保证；若实现出现跳增 >1 分，视为舍入点/量化口径漂移）。
> 在该合同成立时，`netFromX(x)=x-ΔIIT(x)` 单调不减且每步增量≤1 分，因此二分得到的最小 `x` 必然满足 `x-ΔIIT(x)==T`；若断言失败，属于“舍入/IIT 引擎接口合同被破坏”，必须 fail（稳定错误码）。

### 6.5 税金分摊算法（确定性，保证 Σ=总额）

输入：`D`（group 增量税金成本，分）、`target_net_i`（分）。输出：每项 `tax_i`（分）。

采用 **Largest Remainder（最大余数）** 的确定性分摊：

```text
for each item i:
  num = D * target_net_i
  q_i = floor(num / T)
  r_i = num % T

residual = D - sum(q_i)

按 (r_i desc, item_code asc) 排序，给前 residual 个 item 的 tax_i += 1

tax_i = q_i (+ 1 if got residual cent)
gross_i = target_net_i + tax_i
```

输出写入 payslip items：
- `amount = gross_i`
- `target_net = target_net_i`
- `iit_delta = tax_i`
- `calc_mode = net_guaranteed_iit`
- `tax_bearer = employer`

### 6.6 错误码与映射（冻结）

> 本切片的错误码有两类来源，必须统一前缀并保持稳定：
> - **DB Kernel `MESSAGE`**：用于“输入写入口”（One Door）与只读裁决（对齐 `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql` 风格）。
> - **Go 计算阶段 error_code**：用于 solver/分摊失败；不得伪装成 DB `MESSAGE`（因为求解不在 Kernel 中）。对外仍以稳定 `error_code` 传递（UI 回显 / internal API JSON）。

**统一前缀（冻结）**：本切片新增/使用的错误码一律以 `STAFFING_PAYROLL_` 开头（与 `DEV-PLAN-041` 对齐）。

**输入写入口（DB Kernel MESSAGE）**
- `STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT` → HTTP 422
- `STAFFING_PAYROLL_NET_GUARANTEED_IIT_CURRENCY_MISMATCH` → HTTP 422
- `STAFFING_PAYROLL_RUN_FINALIZED_READONLY` → HTTP 409/422（与 `DEV-PLAN-041` 的 run 状态机错误码口径保持一致）
- `STAFFING_IDEMPOTENCY_REUSED` → HTTP 409/422（复用既有幂等错误码；对齐 `DEV-PLAN-041` 的幂等语义）

**求解器/分摊（Go 计算阶段 error_code）**
- `STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_UPPER_BOUND_EXHAUSTED` → HTTP 422（输入导致无法找到上界；需带诊断字段）
- `STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_CONTRACT_VIOLATION` → HTTP 500（视为舍入点/IIT 引擎接口合同漂移或实现 bug）

## 7. 安全与鉴权（Security & Authz）

### 7.1 Authz 对象与动作（冻结口径）

按现有实现（`pkg/authz/registry.go` + `internal/server/authz_middleware.go`，并对齐 `DEV-PLAN-041`）：
- 新增对象常量：`staffing.payroll-payslips`（read/admin）。
- UI：`GET /org/payroll-runs/{run_id}/payslips/{payslip_id}` 为 `read`；`POST .../net-guaranteed-iit-items` 为 `admin`（净额保证项的写入与删除统一视为“修改工资条”，不再单独引入第二个 object，避免权限模型增殖）。
- internal API：同 UI。

**Stopline（必须落实到实现与测试）**
- 当前 `internal/server/authz_middleware.go` 的 `authzRequirementForRoute` 采用“path 精确匹配”，对包含 `{run_id}/{payslip_id}` 的路由会 fail-open。
- 本切片新增的 payroll routes 必须在 Authz middleware 中实现 **path pattern 匹配**（语义与 `internal/routing` 的 `{param}` segment 匹配一致即可），并在 `internal/server/authz_middleware_test.go` 增加用例，确保上述 2 条路由在 GET/POST 上都能命中正确的 object/action。

### 7.2 数据隔离

- 新增表必须启用 RLS + FORCE，且通过 `app.current_tenant` fail-closed（对齐 `AGENTS.md` 与 `DEV-PLAN-021`）。

## 8. 依赖与里程碑（Dependencies & Milestones）

### 8.1 依赖

- 工具链/门禁：`DEV-PLAN-012/017/022/024/025`。
- Tenancy/RLS：`DEV-PLAN-019/021`（运行态必须 enforce）。
- payroll 载体与状态机：`DEV-PLAN-041/042`。
- IIT 引擎与 balances：`DEV-PLAN-044`（必须提供 `IIT(...)` 可复用入口）。
- 回溯结转（口径一致性）：`DEV-PLAN-045`（净额保证项在差额结转场景下仍应可复算）。

### 8.2 里程碑（实现顺序建议）

1. [ ] Schema SSOT：新增输入载体表 + `payslip_items` 字段增补 + RLS。
2. [ ] Schema→迁移闭环：按 `DEV-PLAN-024` 生成 `migrations/staffing/*` + `atlas.sum`。
3. [ ] Kernel：实现 `submit_payslip_item_input_event(...)`（幂等 + upsert/delete + finalized 只读裁决）。
4. [ ] Calc：在 IIT pipeline 中接入 solver（base → solve → allocate → final IIT），并写入 payslip items。
5. [ ] UI：工资条详情页录入/展示净额保证项；计算后展示解释字段。
6. [ ] Routing/Authz：更新 `config/routing/allowlist.yaml`、`pkg/authz/registry.go`、`internal/server/authz_middleware.go`。
7. [ ] Tests：覆盖 solver、分摊、错误路径、RLS fail-closed、finalized 只读。

## 9. 测试与验收（Acceptance Criteria）

### 9.1 最小测试矩阵（必须）

- [ ] solver：单项 target_net 可解；多项可解；`Σ tax_i = ΔIIT(x)`；确定性（同输入多次运行结果一致）。
- [ ] 分摊：存在 residual 分时按 tie-breaker（`item_code asc`）稳定分配；无负数；不丢分不多分。
- [ ] 失败路径：输入非法（缺 target_net/负数/币种错）返回 422 + 稳定错误码；不收敛返回稳定错误码；contract violation 返回 500。
- [ ] RLS：不设置 `app.current_tenant` 时，对输入表/事件表读写全部失败（fail-closed）。
- [ ] finalized：run 已定稿时，录入/删除净额保证项失败（只读语义成立）。

### 9.2 验收脚本（建议以 UI 可操作复现）

1. 创建 pay period → 创建 run → 计算生成工资条（对齐 `DEV-PLAN-041/042`）。
2. 打开某人员工资条详情，录入“长期服务奖（税后）=20,000.00”。
3. 重新计算该 run。
4. 验证该项展示：`target_net=20000.00` 且 `net_after_iit=20000.00`；同时 `gross_amount` 与 `iit_delta` 可见且可对账。

## 10. 运维与监控（Ops & Monitoring）

- 不引入 feature flag/复杂监控；但必须在关键日志中包含可排障字段（建议：`tenant_id`, `run_id`, `person_uuid`, `assignment_id`, `T`, `x`, `D`, `iterations`）。
- 若未来需要异步批量算薪/并发 worker，必须另立 dev-plan 定义重试/幂等/告警与回滚策略（避免在 P0 隐式扩张）。
