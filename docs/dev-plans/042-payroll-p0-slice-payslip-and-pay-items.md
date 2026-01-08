# DEV-PLAN-042：Payroll P0-2——工资条与工资项（Payslip & Pay Items，Gross Pay）

**状态**: 规划中（2026-01-08 01:56 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 蓝图合同（范围/不变量/验收基线）：`DEV-PLAN-040`  
> 依赖：`DEV-PLAN-041`（pay period / payroll run / payslip 载体 + 状态机）  
> 细化模板：`DEV-PLAN-001`（本文按其结构补齐到“可直接编码”级别）

## 0. 可执行方案（本计划合同）

> 本节为实施合同；若与后文细节冲突，以本节为准（对齐 `DEV-PLAN-040` 的合同口径）。

### 0.1 背景与上下文

- `DEV-PLAN-041` 已提供 payroll 主流程壳（pay period / payroll run / payslip 空壳载体）。本切片将 payslip 升级为**可对账的 gross pay 工资条**：生成工资项明细（子表）并将汇总列化到 `payslips.gross_pay/net_pay/employer_total`（但汇总口径必须可由明细重算）。
- 本切片的输入侧依赖任职（Assignment）投射：`staffing.assignment_versions`，并按 `DEV-PLAN-040` 建议为其补齐最小“定薪字段”（`base_salary/currency/profile`）。
- 本切片冻结定薪语义：`assignment_versions.base_salary` 表示**月薪（FTE=1.0）**，实际计薪金额需乘以 `assignment_versions.allocated_fte`（FTE，`(0,1]`）；本切片仅支持 `pay_periods.pay_group='monthly'` 且 pay period 为自然月 `[YYYY-MM-01, next_month-01)`，否则 calculate 必须 fail（稳定错误码见 §5.3，失败态编排见 §6.4）。
- UI 必须可发现/可操作：用户可在 run 详情进入工资条列表/详情，并看到“基本工资”明细与汇总字段（对齐 `AGENTS.md` 用户可见性原则与 `DEV-PLAN-039` Slice 2 出口条件）。

### 0.2 目标与非目标（P0-2 Slice）

**目标**
- [ ] 冻结工资项明细模型（最小集合）：定义 `payslip_items.item_kind` 枚举与最小 `item_code` 集合（至少含 `EARNING_BASE_SALARY`）。
- [ ] Gross Pay（应发）最小计算可复现：从 `staffing.assignment_versions.base_salary`（月薪，FTE=1.0）× `allocated_fte`，按 pay period（自然月）与 assignment validity 的**日粒度交集**进行 pro-rate，并生成 `staffing.payslip_items`；`payslips.gross_pay` 由明细聚合得到。
- [ ] 对账口径冻结：`gross_pay/net_pay/employer_total` 必须可由明细重算（列化字段仅为快照/查询优化，不得成为第二权威；对齐 `DEV-PLAN-040` §0.4.1 JSONB 矩阵）。
- [ ] UI 工资条详情可解释：展示工资项明细（至少含 code、归类、金额、pro-rate basis 解释字段）。
- [ ] 一致性与隔离：写入口唯一（One Door）、RLS fail-closed、Valid Time=date（对齐 `AGENTS.md` 与 `DEV-PLAN-040`）。

**非目标（Out of Scope）**
- 不在本切片引入社保与个税算法（`DEV-PLAN-043/044`）。
- 不引入回溯计算闭环与差额结转（`DEV-PLAN-045`）。
- 不引入“税后发放/净额保证（仅个税）”工资项（`DEV-PLAN-046`）。
- 不引入通用表达式引擎（对齐 `DEV-PLAN-040` §4.3）。
- P0 不引入 `business_unit_id/setid/record_group`（对齐 `DEV-PLAN-040` §0.2 与 `DEV-PLAN-028`）。
- 本切片不支持非 `monthly` 的 `pay_group` 或非自然月 pay period（该扩展必须另立 dev-plan 冻结口径与验收）。

### 0.3 工具链与门禁（SSOT 引用）

> 本计划仅声明命中项与 SSOT 链接，不复制命令清单（避免漂移；见 `DEV-PLAN-000`）。

- **触发器（实施阶段将命中）**
  - [ ] Go 代码（`AGENTS.md`）
  - [ ] DB 迁移 / Schema（`DEV-PLAN-024`）
  - [ ] sqlc / schema export（`DEV-PLAN-025`；`internal/sqlc/schema.sql` 需保持最新）
  - [ ] `.templ` / Tailwind（若实现 UI 模板，按 `AGENTS.md` 入口执行）
  - [ ] 路由治理（`DEV-PLAN-017`；需更新 `config/routing/allowlist.yaml`）
  - [ ] 文档（`make check doc`）
- **SSOT 链接**
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`
  - DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范（若触及）：`docs/dev-plans/025-sqlc-guidelines.md`
  - 路由策略：`docs/dev-plans/017-routing-strategy.md`
  - 时间语义（Valid Time）：`docs/dev-plans/032-effective-date-day-granularity.md`

### 0.4 关键不变量与失败路径（停止线）

- **No Legacy**：不得引入任何回退通道/双链路（对齐 `AGENTS.md` 与 `DEV-PLAN-004M1`）。
- **One Door**：任何写入（生成 payslips/items、更新汇总）必须走 DB Kernel；应用层禁止直写 `staffing.payslips/payslip_items`。
- **No Tx, No RLS**：所有读写必须显式事务 + 注入 `app.current_tenant`，缺失即 fail-closed（对齐 `AGENTS.md` / `DEV-PLAN-019/021`）。
- **Valid Time = date**：pro-rate 仅基于 `daterange [start,end)` 的日粒度交集；禁止把业务有效期写成 `timestamptz`。
- **P0 pay period 语义冻结**：本切片仅支持 `pay_group='monthly'` 且 period 为自然月 `[YYYY-MM-01, next_month-01)`；否则 `CALC_FINISH` 必须抛稳定错误码并进入失败态（对齐 §6.4 编排）。
- **FTE 语义冻结**：`base_salary` 为月薪（FTE=1.0）；实际计薪金额必须乘以 `allocated_fte`（`(0,1]`），否则计算失败并抛稳定错误码（见 §5.3 / §6.2）。
- **金额无浮点**：DB 使用 `numeric`；Go 侧金额/费率计算与 JSON 传输使用十进制定点（推荐 `apd.Decimal` + JSON string；对齐 `DEV-PLAN-040` §4.2）。
- **舍入即合同**：冻结“舍入点/舍入规则/精度”，并在测试中覆盖；禁止在业务代码散落临时 Round（对齐 `DEV-PLAN-040` §4.2）。
- **JSONB 边界**：工资项明细一律子表；`payslip_items.meta` 仅承载解释/trace（object），不得让 JSONB 成为权威金额明细来源（对齐 `DEV-PLAN-040` §0.4.1）。
- **新增表需确认（红线）**：实现阶段一旦要落地新的 `CREATE TABLE`（本切片包含 `payslip_items`），需你在 PR 前明确确认（SSOT：`AGENTS.md`）。

### 0.5 验收标准（Done 口径）

- [ ] 对任一人员/任职：创建 payroll run 后可计算生成工资条；工资条详情至少包含“基本工资（EARNING_BASE_SALARY）”一项明细。
- [ ] `payslips.gross_pay/net_pay/employer_total` 与明细聚合结果一致（可重算）。
- [ ] pro-rate 在日粒度下可复现（示例：月中入职/离职/调薪导致的时间切片）。
- [ ] FTE 可复现：`allocated_fte=0.5` 时，基本工资明细金额为 `0.5×`（再叠加日期 pro-rate），且汇总与明细聚合一致。
- [ ] pay period 口径受控：`pay_group != 'monthly'` 或 period 非自然月时，calculate 返回 422 + 稳定错误码，并按状态机进入 failed（允许修复后重试）。
- [ ] 工资条列表支持按 `pernr` 定位人员（通过 Person 内部只读入口解析），并在过滤视图中展示 `pernr/display_name`。
- [ ] 缺少 tenant context 时，对 payroll 表读写 fail-closed（No Tx, No RLS）。

## 1. 背景与上下文（Context）

本切片是 `DEV-PLAN-039` 的 Slice 2（P0-2）。其职责是把 `DEV-PLAN-041` 的“workflow 壳”补齐为可对账的工资条结果：

- **输入侧（Staffing）**：来自 `staffing.assignment_versions` 的定薪字段与有效期时间线（Valid Time=date，`daterange [)`）。
- **输出侧（Payroll）**：对每个 run 生成 payslip 与 pay items 明细（`payslip_items`），并把汇总列化到 `payslips`（但汇总口径必须可由明细重算）。
- **UI 可见性**：用户必须能在 UI 查看工资条列表/详情并理解 gross pay 的来源（至少能看到基本工资 + pro-rate basis）。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 本切片交付物清单（必须可落地）

- DB：
  - 扩展 `staffing.assignment_versions`（补齐最小定薪字段：`base_salary/currency/profile`）。
  - 承接 Assignment 写入口与投射：`staffing.submit_assignment_event` / `staffing.replay_assignment_versions` 支持并投射 `base_salary/allocated_fte/currency/profile`（见 §6.1）。
  - 新增 `staffing.payslip_items`（工资项明细子表，RLS + 约束）。
  - 在 Kernel 的 `CALC_FINISH` 路径内生成 payslips/items，并刷新 `payslips.gross_pay/net_pay/employer_total`。
- Go：
  - UI：工资条列表/详情页（基于 run 维度进入）。
  - Store/Queries：读取 payslips 与 items；错误映射为稳定错误码（DB `MESSAGE` + `routing.WriteError`）。
- 路由与鉴权：新增 payslips 页面与 internal API 的路由 allowlist；新增/复用 authz 对象与动作（见 §7）。

### 2.2 非目标（再次强调）

同 §0.2：不含社保/个税/回溯/税后发放/表达式引擎/SetID。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 组件边界（选定）

- **DB Kernel（权威）**：在 `submit_payroll_run_event(event_type='CALC_FINISH')` 同事务内完成：工资项明细生成 + payslip 汇总刷新；保证 One Door 与可重放（按 run 事件重算）。
- **Go/HTTP（编排与展示）**：负责鉴权、事务边界（显式 `BEGIN`）、注入 `app.current_tenant`、触发 calculate、读取结果并渲染 UI。

### 3.2 组件图（Mermaid）

```mermaid
flowchart TD
  UI[HTMX UI<br/>/org/payroll-runs/.../payslips] --> H[internal/server handlers]
  H --> S[PayrollStore (PG)]
  S -->|tx + set_config(app.current_tenant)| K[DB Kernel<br/>submit_payroll_run_event + rebuild_gross]
  K --> E[(payroll_run_events)]
  K --> R[(payslips)]
  K --> I[(payslip_items)]
  K --> A[(assignment_versions)]
```

### 3.3 关键设计决策（ADR 摘要）

- **决策 1：工资项定义不落表（P0）**  
  - 选项 A：新增 `pay_item_definitions` 配置表 + 种子数据。缺点：引入额外迁移与配置演进决策，P0 增量过大。  
  - 选项 B（选定）：工资项最小集合以 Go 枚举冻结（`PayItemCode`），DB 仅存 `item_code/item_kind/amount/meta`。优点：最小闭环、无额外配置系统；与 `DEV-PLAN-040` “先切片后引擎”一致。

- **决策 2：pro-rate 口径**  
  - 选项 A：按自然月天数（实际天数）比例。  
  - 选项 B：按“标准月 30 天/计薪天数表”。缺点：需要额外配置与地区差异决策。  
  - 选项 A（选定）：按 pay period 的 `period_days = upper(period)-lower(period)` 与 assignment validity 的交集天数比例；日粒度、闭开区间 `[start,end)`（对齐 `DEV-PLAN-032`）。

- **决策 3：舍入点（本切片冻结）**  
  - 本切片仅产生 `EARNING_BASE_SALARY` 明细：**在每条 payslip item 上量化到 2 位小数**（`precision=2`，`rounding_rule=ROUND_HALF_UP_2DP_ITEM`），payslip 汇总仅做求和不再二次舍入；从而保证“汇总=明细聚合”可复现。
  - 舍入合同不写入 JSONB：`rounding_rule/precision` 属于合同字段（对齐 `DEV-PLAN-040` §0.4.1），本切片以“全局常量”冻结；若后续出现“不同工资项不同舍入规则”，必须升级为列字段（另立 dev-plan 承接迁移与门禁）。

## 4. 数据模型与约束（Data Model & Constraints）

> 下述为本切片冻结的表合同；实现期以 `modules/staffing/infrastructure/persistence/schema/*.sql` 为 Schema SSOT，并按 `DEV-PLAN-024` 生成迁移闭环。

### 4.1 表清单（新增/变更）

> 注意：实现落地 `CREATE TABLE` 前需你明确确认（见 §0.4 “新增表需确认”）。

- **变更**：`staffing.assignment_versions`（新增 `base_salary/currency/profile`）
- **新增**：`staffing.payslip_items`

### 4.2 Schema（SQL 合同草案，按现有 staffing 风格对齐）

#### 4.2.1 `staffing.assignment_versions`（新增薪酬字段）

对齐 `DEV-PLAN-040` §3.1.1：稳定列承载参与计算/对账字段；JSONB 仅做低频扩展（object）。

```sql
ALTER TABLE staffing.assignment_versions
  ADD COLUMN IF NOT EXISTS base_salary numeric(15,2) NULL,
  ADD COLUMN IF NOT EXISTS currency char(3) NOT NULL DEFAULT 'CNY',
  ADD COLUMN IF NOT EXISTS profile jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE staffing.assignment_versions
  ADD CONSTRAINT IF NOT EXISTS assignment_versions_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  ADD CONSTRAINT IF NOT EXISTS assignment_versions_base_salary_check CHECK (base_salary IS NULL OR base_salary >= 0),
  ADD CONSTRAINT IF NOT EXISTS assignment_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object');
```

说明：
- `base_salary` 在 DB 层允许为 NULL（避免一次性要求存量数据补齐）；但本切片的算薪逻辑对“参与计算的 assignment”强制要求 `base_salary IS NOT NULL`（见 §6）。
- `allocated_fte` 已存在于 `assignment_versions`（stable 列）。本切片冻结其语义为 FTE `(0,1]`，并在算薪时 fail-closed 校验（见 §6.2）。
- P0 仅支持 `currency='CNY'`；若后续引入多币种，需另立 dev-plan 冻结口径与汇率处理。

#### 4.2.2 `staffing.payslip_items`

```sql
CREATE TABLE IF NOT EXISTS staffing.payslip_items (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  item_code text NOT NULL,
  item_kind text NOT NULL,
  amount numeric(15,2) NOT NULL,
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_items_item_code_nonempty_check CHECK (btrim(item_code) <> ''),
  CONSTRAINT payslip_items_item_code_trim_check CHECK (item_code = btrim(item_code)),
  CONSTRAINT payslip_items_item_code_upper_check CHECK (item_code = upper(item_code)),
  CONSTRAINT payslip_items_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_items_meta_is_object_check CHECK (jsonb_typeof(meta) = 'object'),
  CONSTRAINT payslip_items_payslip_fk FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS payslip_items_by_payslip_btree
  ON staffing.payslip_items (tenant_id, payslip_id, id);

CREATE INDEX IF NOT EXISTS payslip_items_by_event_btree
  ON staffing.payslip_items (tenant_id, last_run_event_id, id);
```

说明：
- `meta` 仅用于解释/trace；其中出现的数值（金额/比例/天数）一律用 string 表达（对齐 `DEV-PLAN-040` §0.4.1 通用约束）。
- 本切片只生成 `item_kind='earning'` 的 `EARNING_BASE_SALARY`；`deduction/employer_cost` 为后续切片预留（`DEV-PLAN-043/044/046`）。

### 4.3 RLS（必须）

对 `staffing.payslip_items` 启用 RLS 并 `FORCE ROW LEVEL SECURITY`，策略与 `staffing.assignment_versions` 同构：

```sql
ALTER TABLE staffing.payslip_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_items;
CREATE POLICY tenant_isolation ON staffing.payslip_items
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
```

### 4.4 迁移策略（按 `DEV-PLAN-024`）

- **Schema SSOT**：在 `modules/staffing/infrastructure/persistence/schema/` 中追加上述 DDL（建议与 `DEV-PLAN-041` 的 payroll tables 同一组文件维护）。
- **生成迁移**：在 `migrations/staffing/` 生成对应 goose 迁移文件，并更新 `migrations/staffing/atlas.sum`；必须保证 `make staffing plan` 最终输出 No Changes。

## 5. 接口契约（API Contracts）

> 口径对齐 `DEV-PLAN-041`：UI 为 HTML + HTMX；同时提供最小 internal API 便于 tests 与后续 e2e 复现。

### 5.1 UI：Payslips（按 run 入口）

#### `GET /org/payroll-runs/{run_id}/payslips`
- **用途**：展示某个 run 的工资条列表（每条至少显示 `person_uuid/assignment_id/gross_pay/net_pay/employer_total`）。
- **Query（可选）**
  - `pernr`：用于用户侧定位人员；服务端复用现有 `PersonStore.FindPersonByPernr`（同进程、同 DB tx + tenant 注入）解析为 `person_uuid`，并复用 Person 的 `pernr` 规范化规则（1-8 位数字字符串，去前导 0 后 canonical）。禁止通过 HTTP 自调用 `/person/api/...` 形成第二套失败模式与契约漂移。
  - `person_uuid`：仅用于排障/测试（实现期可保留）。
- **返回**：HTML 页面（列表 + 进入详情链接）。

#### `GET /org/payroll-runs/{run_id}/payslips/{payslip_id}`
- **用途**：工资条详情页：展示 payslip 汇总 + `payslip_items` 明细。
- **明细展示**：每行至少包含：`item_code`、`item_kind`、`amount`、以及 `meta` 中的 pro-rate basis（可读化）。

#### `POST /org/payroll-runs/{run_id}/calculate`
- **语义**：触发本切片的真实计算：生成 payslips + `EARNING_BASE_SALARY` 明细，并刷新汇总字段。
- **成功**：303 跳转回 run 详情页（并可从详情进入工资条列表）。
- **失败**：
  - 422：输入/前置条件不满足（例如缺少 `base_salary`）；回显稳定错误提示。
  - 409：状态机冲突（例如 run 不在 `draft/failed`）。

### 5.2 Internal API（最小）

> internal API route_class=`internal_api`，不作为对外稳定契约；金额字段一律以 string 传输（避免 `float64`）。

- `GET /org/api/payslips?run_id=...` → `[{id,run_id,pay_period_id,person_uuid,assignment_id,currency,gross_pay,net_pay,employer_total}]`
- `GET /org/api/payslips/{payslip_id}` → `{id,...,items:[{id,item_code,item_kind,amount,meta}]}`

金额字段示例：

```json
{
  "gross_pay": "30000.00",
  "items": [{ "amount": "30000.00" }]
}
```

### 5.3 错误契约（DB → HTTP）

为避免实现期“随手拼文案/随手选状态码”导致漂移，本切片冻结以下错误契约：

- **DB Kernel 侧**：用 `RAISE EXCEPTION USING MESSAGE='<STABLE_CODE>', DETAIL='<debug_detail>'` 抛错；Go 侧以 `MESSAGE` 作为稳定识别键（对齐现有 staffing/person 的模式）。
- **HTTP 侧**：internal API 使用 `routing.WriteError` 的 `code` 字段承载稳定错误码；UI 侧回显可读错误文案，但日志与调试信息需保留稳定错误码。

| 场景 | 稳定错误码（DB `MESSAGE` / 解析失败码） | HTTP（UI/internal_api） |
| --- | --- | --- |
| 租户上下文缺失/非法/不匹配 | `RLS_TENANT_CONTEXT_MISSING` / `RLS_TENANT_CONTEXT_INVALID` / `RLS_TENANT_MISMATCH` | 403 |
| pay_group 非 `monthly` | `STAFFING_PAYROLL_UNSUPPORTED_PAY_GROUP` | 422 |
| pay period 非自然月 `[YYYY-MM-01, next_month-01)` | `STAFFING_PAYROLL_UNSUPPORTED_PAY_PERIOD` | 422 |
| allocated_fte 非 `(0,1]` | `STAFFING_PAYROLL_INVALID_ALLOCATED_FTE` | 422 |
| base_salary 缺失（命中计算输入） | `STAFFING_PAYROLL_MISSING_BASE_SALARY` | 422 |
| currency 非 CNY（P0 冻结） | `STAFFING_PAYROLL_UNSUPPORTED_CURRENCY` | 422 |
| run 状态机冲突（例如已 finalized、非法跃迁） | `STAFFING_PAYROLL_RUN_*`（沿用 `DEV-PLAN-041` 的稳定错误码） | 409 |
| 幂等冲突（同 event_id 但 payload 不一致） | `STAFFING_IDEMPOTENCY_REUSED` | 409 |
| pernr 输入非法 | `PERSON_PERNR_INVALID` | 400/422（internal_api 用 400；UI 可回显为表单/提示） |
| pernr 不存在 | `PERSON_NOT_FOUND` | 404 |

### 5.4 Routing 与 Authz 覆盖（Stopline）

为避免“新增动态路由未被鉴权覆盖”的隐性漏洞（对齐 `DEV-PLAN-003` stopline），本切片冻结以下实现要求：

- **routing allowlist（必须）**：在 `config/routing/allowlist.yaml`（entrypoint=`server`）新增：
  - UI：`GET /org/payroll-runs/{run_id}/payslips`
  - UI：`GET /org/payroll-runs/{run_id}/payslips/{payslip_id}`
  - UI：`POST /org/payroll-runs/{run_id}/calculate`
  - internal_api：`GET /org/api/payslips`
  - internal_api：`GET /org/api/payslips/{payslip_id}`
- **Authz 覆盖（必须）**：`internal/server/authz_middleware.go` 当前基于“精确 path”判定是否鉴权；本切片新增路由包含 `{...}` 动态段，因此必须实现与 routing allowlist 相同的“段匹配”能力，确保上述 5 条路由全部可命中鉴权要求，并增加测试覆盖（至少覆盖：GET=read、POST=admin、以及任一动态路由不会落入“默认不鉴权”分支）。

## 6. 核心逻辑与算法（Business Logic & Algorithms）

### 6.1 Salary 字段写入与投射（Assignment → assignment_versions）

为确保 payroll 计算只依赖投射表（而非重放 events 或解析 JSONB），本切片要求：

- `staffing.submit_assignment_event(...)` 的 payload 支持并投射：
  - `base_salary`：string（例如 `"30000.00"`；语义为“月薪（FTE=1.0）”）
  - `allocated_fte`：string（例如 `"1.0"` / `"0.5"`；语义为 FTE，必须在 `(0,1]`）
  - `currency`：string（例如 `"CNY"`；P0 仅允许 CNY）
  - `profile`：object（低频扩展键；不得放权威金额明细）
- `staffing.replay_assignment_versions(...)` 在生成 `assignment_versions` 时把上述字段写入稳定列；缺失键沿用上一版本（UPDATE 语义对齐 position）。

失败路径（建议错误码）：
- `STAFFING_ASSIGNMENT_BASE_SALARY_INVALID`：`base_salary` 非法或 <0。
- `STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID`：`allocated_fte` 非法或不在 `(0,1]`。
- `STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED`：非 `CNY`。
- `STAFFING_ASSIGNMENT_PROFILE_INVALID`：profile 非 object。

### 6.2 Kernel：在 `CALC_FINISH` 内生成 payslips/items（核心）

> 本切片将 `DEV-PLAN-041` 的 `CALC_FINISH` 从“占位”升级为“真实生成 gross pay 结果”。

**约定：最小工资项集合（冻结）**
- `item_kind`：`earning/deduction/employer_cost`
- `item_code`（本切片只用）：`EARNING_BASE_SALARY`

**pro-rate 合同（冻结）**
- pay period：`period daterange`，日粒度、闭开区间 `[start,end)`；本切片仅支持 `pay_group='monthly'` 且 period 为自然月。
- assignment slice：`assignment_versions.validity`，日粒度、闭开区间 `[start,end)`。
- `period_days = upper(period) - lower(period)`（整数，>0）。
- `overlap = [max(lower(validity), lower(period)), min(upper(validity), upper(period)))`；`overlap_days = overlap_end - overlap_start`。
- `amount_raw = base_salary * allocated_fte * overlap_days / period_days`。
- **舍入点**：每条 payslip item 量化到 2 位小数（`precision=2`，`rounding_rule=ROUND_HALF_UP_2DP_ITEM`）。

**生成算法（必须，幂等/可重放）**
1. `assert_current_tenant(p_tenant_id)`；对 `run_id` 继续沿用 `DEV-PLAN-041` 的 `pg_advisory_xact_lock`（状态机串行化）。
2. 读取 `payroll_runs/pay_periods`，取得 `pay_group/period`；校验：
   - `pay_group='monthly'`；否则抛 `STAFFING_PAYROLL_UNSUPPORTED_PAY_GROUP`；
   - `period` 为自然月 `[YYYY-MM-01, next_month-01)`；否则抛 `STAFFING_PAYROLL_UNSUPPORTED_PAY_PERIOD`；
   - `period_days>0`。
3. 选取参与本 run 的 assignment slices：
   - `assignment_versions.status='active'` 且 `assignment_type='primary'`；
   - `validity && period`（有交集）；
   - `allocated_fte > 0 AND allocated_fte <= 1`；否则抛稳定错误码 `STAFFING_PAYROLL_INVALID_ALLOCATED_FTE`；
   - `base_salary IS NOT NULL` 且 `currency='CNY'`；否则计算失败并抛稳定错误码 `STAFFING_PAYROLL_MISSING_BASE_SALARY` / `STAFFING_PAYROLL_UNSUPPORTED_CURRENCY`。
4. upsert payslips（按 `run_id+person_uuid+assignment_id` 唯一）：若已存在则复用；确保 `payslips.last_run_event_id` 被刷新为本次 `CALC_FINISH` 的 event db id。
5. 删除旧明细（仅限本 run 产物）：按 `tenant_id + run_id` 维度删除该 run 下所有 `payslip_items`（通过 join payslips 定位）。
6. 插入 `payslip_items`：对每个 assignment slice 的 overlap 段生成一条 `EARNING_BASE_SALARY` 明细：
   - `amount = round(amount_raw, 2)`；
   - `meta` 至少包含：`pay_group/period_start/period_end_exclusive/segment_start/segment_end_exclusive/base_salary/allocated_fte/overlap_days/period_days/ratio`（数值均为 string）。
7. 刷新 payslip 汇总字段：
   - `gross_pay = SUM(items.amount WHERE item_kind='earning')`；
   - P0-2：`net_pay = gross_pay`；`employer_total = 0`；
   - 汇总仅求和，不再二次舍入，保证“汇总=明细聚合”。

### 6.3 应用层事务与租户注入（必须）

同 `DEV-PLAN-041`：所有 Kernel 调用必须在显式事务内完成：`BEGIN` → `set_config('app.current_tenant', $tenant, true)` → 调用 Kernel → `COMMIT`。

### 6.4 应用层：Calculate 编排（`CALC_START` → `CALC_FINISH` / `CALC_FAIL`）

为保持 `DEV-PLAN-041` 的状态机语义可审计（尤其是失败态），`POST /org/payroll-runs/{run_id}/calculate` 的编排建议为：

1. **Tx#1：提交 `CALC_START`（必提交）**
   - `submit_payroll_run_event(event_type='CALC_START')` → run 进入 `calculating`。
2. **Tx#2：尝试提交 `CALC_FINISH`（同事务内生成 payslips/items）**
   - `submit_payroll_run_event(event_type='CALC_FINISH')`：Kernel 内完成 payslips/items 生成与汇总刷新。
3. **若 Tx#2 失败：Tx#3 提交 `CALC_FAIL`**
   - 捕获错误码（例如 `STAFFING_PAYROLL_MISSING_BASE_SALARY`），在新事务中提交 `submit_payroll_run_event(event_type='CALC_FAIL', payload={error_code,...})`，run 进入 `failed`，允许重试。

说明：
- 若实现选择“预检”，可在 Tx#1 前先校验输入侧完整性；但 Kernel 仍必须做 fail-closed 校验（避免绕过 DB 约束）。
- P0-2 为同步计算；若后续引入异步 worker pool/队列，必须另立 dev-plan 重新冻结“失败态恢复/超时/幂等/并发”口径（对齐 `DEV-PLAN-040`）。

## 7. 安全与鉴权（Security & Authz）

### 7.1 Authz 对象与动作（冻结口径）

按现有实现（`pkg/authz/registry.go` + `internal/server/authz_middleware.go`）：
- 复用 `DEV-PLAN-041` 已冻结的对象常量：`staffing.payroll-runs`（read/admin），本切片不新增新的 object（避免对象增殖/重复权威表达）。
- UI：GET（payslips 列表/详情）为 `read`；POST（`/calculate`）为 `admin`。
- internal API：同 UI（GET=read）。

### 7.2 数据隔离

- RLS 是强隔离底线；应用层不得以“绕过 RLS”作为排障手段进入业务链路。

## 8. 依赖与里程碑（Dependencies & Milestones）

### 8.1 依赖

- 上游合同与路线图：`DEV-PLAN-039/040`。
- 主流程壳与状态机：`DEV-PLAN-041`。
- Tenancy/RLS：`DEV-PLAN-019/021`（运行态必须 enforce）。
- DB 迁移闭环：`DEV-PLAN-024`。
- Valid Time：`DEV-PLAN-032`（daterange `[)`）。

### 8.2 里程碑（实现顺序建议）

1. [ ] Schema SSOT：扩展 `assignment_versions` + 新增 `payslip_items` + RLS。
2. [ ] Schema→迁移闭环：按 `DEV-PLAN-024` 生成 `migrations/staffing/*` + `atlas.sum`。
3. [ ] Kernel：补齐 assignment replay 对 salary 字段的投射；实现/接入“CALC_FINISH 生成 payslips/items”。
4. [ ] Server：新增 payslip list/detail handlers + store queries。
5. [ ] Routing/Authz：更新 `config/routing/allowlist.yaml`、`pkg/authz/registry.go`、`internal/server/authz_middleware.go`。
6. [ ] Tests：覆盖 pro-rate、汇总可重算、缺输入失败路径、RLS fail-closed。

## 9. 测试与验收（Acceptance Criteria）

### 9.1 最小测试矩阵（必须）

- [ ] 生成：对一个 run 计算后生成 payslip + 至少 1 条 `EARNING_BASE_SALARY` 明细。
- [ ] 对账：`payslips.gross_pay/net_pay/employer_total` 与明细聚合一致（可重算）。
- [ ] pro-rate：
  - 整段覆盖（assignment validity 覆盖整个 pay period）= 100%。
  - 月中入职/离职（部分覆盖）= 比例可复现（按天）。
  - 月中调薪（assignment validity 切片）= 多条明细累加后等于预期。
- [ ] FTE：`allocated_fte=0.5` 时，明细与汇总均按 0.5 倍生效（并与按天 pro-rate 组合可复现）。
- [ ] 舍入点：在 item level 量化到 2 位小数，汇总仅求和；验证边界（例如 1/31 的重复小数）。
- [ ] 失败路径：缺少 base_salary 或 currency 非 CNY 时，计算失败且 run 进入 failed（或至少返回稳定错误码；状态机与实现一致）。
- [ ] 失败路径：pay_group 非 monthly 或 period 非自然月时，计算失败且返回稳定错误码（状态机与实现一致）。
- [ ] 鉴权：GET payslips=read、POST calculate=admin；且动态路由（含 `{run_id}`/`{payslip_id}`）不会落入“默认不鉴权”分支（需测试覆盖）。
- [ ] RLS：不设置 `app.current_tenant` 时，对 payroll 表与 `payslip_items` 的读写全部失败（fail-closed）。

### 9.2 验收脚本（建议以 UI 可操作复现）

1. 创建 pay period（对齐 `DEV-PLAN-041` 示例）。
2. 准备 1 条 active assignment（base_salary=30000.00，currency=CNY，validity 覆盖本周期；输入侧由 `DEV-PLAN-031` 的写入口提供）。
3. 创建 run → 计算 → 进入工资条列表/详情，看到基本工资明细与汇总。

## 10. 运维与监控（Ops & Monitoring）

- P0-2 不引入异步计算与复杂监控；仅要求关键日志可定位（建议包含 `tenant_id`, `pay_period_id`, `run_id`, `event_id`, `request_id`, `payslip_count`, `item_count`）。
- 若后续引入异步 worker pool 或外部接口，必须另立 dev-plan 定义重试/幂等/告警与回滚策略（避免在 P0 隐式扩张；对齐 `DEV-PLAN-040`）。

## 11. 备注（与后续切片的边界）

- `DEV-PLAN-043`：在本切片的 payslip 上追加社保扣缴明细与公司成本（`deduction/employer_cost`）。
- `DEV-PLAN-044`：基于本切片应发与社保扣缴计算累计预扣个税，并引入 YTD balances。
- `DEV-PLAN-045`：承接回溯重算闭环（差额 pay item + 结转）。
- `DEV-PLAN-046`：税后发放（仅个税）的净额保证与 tax gross-up 求解。
