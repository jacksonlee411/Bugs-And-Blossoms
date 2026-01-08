# DEV-PLAN-043：Payroll P0-3——社保政策（单城市）配置与扣缴计算

**状态**: 草拟中（2026-01-08 06:07 UTC）

> 上游路线图：`DEV-PLAN-039`  
> 蓝图合同（范围/不变量/验收基线）：`DEV-PLAN-040`  
> 依赖：`DEV-PLAN-041/042`（主流程 + 工资条载体 + Gross Pay）  
> 细化模板：`DEV-PLAN-001`（本文按其结构补齐到“可直接编码”级别）

## 0. 可执行方案（本计划合同）

> 本节为实施合同；若与后文细节冲突，以本节为准（对齐 `DEV-PLAN-040` 的合同口径）。

### 0.1 背景与上下文

- 中国社保政策高度碎片化，但 P0 采取“单城市起步”：同一 tenant 仅允许配置一个 `city_code`（所有险种共享），`hukou_type` 先固定为 `default`（对齐 `DEV-PLAN-040` §0.2/§5.1）。
- 本切片交付两件事：
  1) 社保政策可配置（Valid Time 日粒度 + no-overlap）；  
  2) payroll run 计算时写入“险种分项明细”（个人扣款 + 企业成本）并更新工资条汇总口径。
- 写入口与一致性：社保政策与算薪结果均必须遵守 One Door / No Tx, No RLS / 金额无浮点 / 舍入即合同（SSOT：`AGENTS.md`、`DEV-PLAN-040`）。

### 0.2 目标与非目标（P0-3 Slice）

**目标**
- [ ] 冻结“社保政策（单城市）”最小数据模型：关键字段列化（费率/基数上下限/舍入合同），低频扩展仅允许 JSONB object（对齐 `DEV-PLAN-040` §0.4.1）。
- [ ] 落地 DB Kernel One Door：`staffing.submit_social_insurance_policy_event(...)`（append-only events + 同事务投射 versions）。
- [ ] 冻结并实现扣缴计算合同：基数 clamp + 逐险种个人/企业金额计算 + 舍入点与舍入枚举显式化（至少覆盖 2 种舍入策略）。
- [ ] 生成可对账险种明细：写入 `staffing.payslip_social_insurance_items`（权威明细子表），并更新 `staffing.payslips.net_pay` 与 `staffing.payslips.employer_total`（可由明细重算）。
- [ ] 为 `DEV-PLAN-044` 提供可复用口径：工资条的“专项扣除（社保个人部分）”= `Σ payslip_social_insurance_items.employee_amount`（冻结为权威读口径）。
- [ ] UI 可发现/可操作：提供“社保政策（单城市）”配置页，并在工资条详情展示险种分项（个人/企业）与汇总。

**非目标（Out of Scope）**
- 不实现 300+ 城市全量规则；不做外部社保申报系统集成。
- 不在 P0 引入 SetID/Business Unit 维度；如需 BU 级差异必须先按 `DEV-PLAN-028` 扩展 stable record group 并接入权威解析入口，再另立 dev-plan 承接迁移与门禁（对齐 `DEV-PLAN-040` §0.2）。
- 不在本切片引入通用表达式引擎（对齐 `DEV-PLAN-040` §4.3）。
- 不支持 pay period 内的社保政策变更与分段计算：P0 固定按 `as_of=lower(pay_period.period)` 选择政策；若在 `(period_start, period_end_exclusive)` 内存在任何险种政策变更（生效日落在区间内），run 计算必须 fail-closed（见 §6.3）。

### 0.3 工具链与门禁（SSOT 引用）

> 本计划仅声明命中项与 SSOT 链接，不复制命令清单（避免漂移；见 `DEV-PLAN-000`）。

- **触发器（实施阶段将命中）**
  - [ ] Go 代码（`AGENTS.md`）
  - [ ] DB 迁移 / Schema（`DEV-PLAN-024`）
  - [ ] sqlc（若触及 queries/config；`DEV-PLAN-025`）
  - [ ] 路由治理（`DEV-PLAN-017`；需更新 `config/routing/allowlist.yaml`）
  - [ ] 文档（`make check doc`）
- **SSOT 链接**
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`
  - DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
  - 路由策略：`docs/dev-plans/017-routing-strategy.md`
  - UI Shell：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
  - 时间语义（Valid Time）：`docs/dev-plans/032-effective-date-day-granularity.md`

### 0.4 关键不变量与失败路径（停止线）

- **No Legacy**：不得引入任何回退通道/双链路（SSOT：`AGENTS.md`、`DEV-PLAN-004M1`）。
- **One Door**：社保政策与算薪结果的写入必须通过 Kernel `submit_*_event(...)`；应用层禁止直写 `staffing.social_insurance_policy_*` / `staffing.payslip_social_insurance_items`（对齐 `DEV-PLAN-040` §0.4）。
- **No Tx, No RLS**：缺少 `app.current_tenant` 必须 fail-closed（`RLS_TENANT_CONTEXT_MISSING`）；所有新表启用 `FORCE ROW LEVEL SECURITY`。
- **Valid Time = date**：政策版本化用 `effective_date date`（events）+ `validity daterange`（versions，`[start,end)`）；同 key 不得重叠（no-overlap）。
- **金额无浮点**：金额与费率一律 `numeric`；对外/JSON 传输金额/费率一律用 string（避免进入 `float64`）。
- **舍入即合同**：冻结 `rounding_rule` 枚举与精度 `precision`；舍入点固定为“逐险种金额行”层面（`employer_amount`/`employee_amount`），汇总通过行求和得到。
- **单城市冻结（P0）**：同一 tenant 的社保政策只允许一个 `city_code`，且 `hukou_type` 仅允许 `default`；违反必须抛稳定错误码（见 §6.1/§6.2）。
- **可对账**：险种分项必须子表；禁止 JSONB array 作为权威明细（对齐 `DEV-PLAN-040` §0.4.1）。
- **新增表需确认（红线）**：实现阶段一旦要落地新的 `CREATE TABLE` 迁移，需你在 PR 前明确确认（SSOT：`AGENTS.md`）。

### 0.5 验收标准（Done 口径）

- [ ] UI 可配置一套社保政策（单城市，覆盖 P0 险种集合）并在指定 pay period 生效（Valid Time 日粒度）。
- [ ] 对任一 payroll run：计算后工资条详情展示各险种个人扣款与企业成本明细；`net_pay/employer_total` 与明细聚合一致（可重算）。
- [ ] 舍入规则可复现：至少覆盖 `HALF_UP(precision=2)` 与 `CEIL(precision=1)` 两种策略，并在测试中锁定输入/输出。
- [ ] `DEV-PLAN-044` 可读取“专项扣除（社保个人部分）”口径：`Σ employee_amount`（同一 payslip，按险种求和）。

## 1. 背景与上下文（Context）

本仓库按 `DEV-PLAN-039` 切片式交付薪酬社保。`DEV-PLAN-043` 的定位：

- `DEV-PLAN-041`：提供 payroll run 状态机与结果载体（`payslips`）。
- `DEV-PLAN-042`：提供 gross pay 与工资条可对账基础。
- **本切片**：引入“社保政策（单城市）”并在 run 计算时生成险种扣缴情明细；同时冻结供 `DEV-PLAN-044` 复用的专项扣除口径。

## 2. 目标与非目标（Goals & Non-Goals）

> 以 `DEV-PLAN-040` 的 P0 合同为上位约束；本节补齐到“实现时无需再做关键设计决策”的粒度。

### 2.1 P0 险种集合（冻结）

P0 固定支持 6 个险种（insurance_type）：
- `PENSION`（养老）
- `MEDICAL`（医疗）
- `UNEMPLOYMENT`（失业）
- `INJURY`（工伤）
- `MATERNITY`（生育）
- `HOUSING_FUND`（公积金）

说明：允许 `employee_rate=0`（例如工伤/生育在多数城市个人为 0）；但险种必须存在政策版本，否则算薪必须 fail-closed（避免静默缺项导致对账漂移）。

### 2.2 舍入合同（冻结）

- `rounding_rule` 枚举（P0 仅 2 种）：
  - `HALF_UP`：四舍五入（正数等价于 round-half-up）
  - `CEIL`：向上取整（用于“见分进角”等）
- `precision`：小数位数（P0 冻结为 `0..2`，对齐金额落库 `numeric(15,2)`）
- 舍入点：**逐险种金额行**（employee/employer 各自金额分别舍入），汇总通过行求和得到。

### 2.3 单城市/户口口径（冻结）

- `city_code`：格式冻结为大写字符串（示例：`CN-310000`）；同一 tenant 仅允许一个 `city_code`。
- `hukou_type`：P0 固定为 `default`（未来扩展 `local/non_local` 需另立 dev-plan）。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 架构图（Mermaid）

```mermaid
flowchart TD
  UI[UI: 社保政策页/工资条详情] --> C[Controllers]
  C --> S[Services]
  S -->|tx + set_config(app.current_tenant)| K1[DB Kernel<br/>staffing.submit_social_insurance_policy_event]
  S -->|tx + set_config(app.current_tenant)| K2[DB Kernel<br/>staffing.submit_payroll_run_event (CALC_FINISH)]
  K1 --> T1[(social_insurance_policy_events/versions)]
  K2 --> T2[(payslips + payslip_social_insurance_items)]
```

### 3.2 关键设计决策（ADR 摘要）

- **决策 1：社保政策采用“事件 SoT + versions 投射”**
  - 原因：政策按有效期版本化、禁止重叠、需要可审计与可回放；与现有 `staffing.position/assignment` 一致。
- **决策 2：险种分项明细落地为专用子表 `payslip_social_insurance_items`**
  - 原因：对账与专项扣除需要权威明细；避免把一对多集合塞入 JSONB array（对齐 `DEV-PLAN-040` §0.4.1）。
- **决策 3：明细引用“policy_last_event_id”而不是 versions 行 id**
  - 原因：versions 表可重建，行 id 不稳定；events 表 append-only，可作为稳定审计锚点（模式同 `payslips.last_run_event_id`）。
- **决策 4：P0 单城市冻结在写入口**
  - 原因：避免“看似支持多城市但实际选择逻辑含糊”的漂移；未来扩展必须另立 dev-plan。

## 4. 数据模型与约束（Data Model & Constraints）

> 实现期以 `modules/staffing/infrastructure/persistence/schema/*.sql` 为 Schema SSOT，并按 `DEV-PLAN-024` 生成迁移闭环。

### 4.1 表清单（新增）

> 注意：实现落地 `CREATE TABLE` 前需你明确确认（见 §0.4 “新增表需确认”）。

- `staffing.social_insurance_policies`（政策身份表：单城市 + 户口 + 险种唯一）
- `staffing.social_insurance_policy_events`（write side / SoT）
- `staffing.social_insurance_policy_versions`（read side / versions）
- `staffing.payslip_social_insurance_items`（险种分项明细：个人扣款 + 企业成本）

### 4.2 Schema（SQL 合同草案，按现有 staffing 风格对齐）

#### 4.2.1 `staffing.social_insurance_policies`

```sql
CREATE TABLE IF NOT EXISTS staffing.social_insurance_policies (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT social_insurance_policies_city_code_nonempty_check CHECK (btrim(city_code) <> ''),
  CONSTRAINT social_insurance_policies_city_code_trim_check CHECK (city_code = btrim(city_code)),
  CONSTRAINT social_insurance_policies_city_code_upper_check CHECK (city_code = upper(city_code)),
  CONSTRAINT social_insurance_policies_hukou_type_nonempty_check CHECK (btrim(hukou_type) <> ''),
  CONSTRAINT social_insurance_policies_hukou_type_trim_check CHECK (hukou_type = btrim(hukou_type)),
  CONSTRAINT social_insurance_policies_hukou_type_lower_check CHECK (hukou_type = lower(hukou_type)),
  CONSTRAINT social_insurance_policies_insurance_type_check CHECK (
    insurance_type IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND')
  ),
  CONSTRAINT social_insurance_policies_identity_unique UNIQUE (tenant_id, city_code, hukou_type, insurance_type)
);

CREATE INDEX IF NOT EXISTS social_insurance_policies_lookup_btree
  ON staffing.social_insurance_policies (tenant_id, city_code, hukou_type, insurance_type);
```

#### 4.2.2 `staffing.social_insurance_policy_events`

```sql
CREATE TABLE IF NOT EXISTS staffing.social_insurance_policy_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  policy_id uuid NOT NULL,
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT social_insurance_policy_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT social_insurance_policy_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT social_insurance_policy_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT social_insurance_policy_events_one_per_day_unique UNIQUE (tenant_id, policy_id, effective_date),
  CONSTRAINT social_insurance_policy_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT social_insurance_policy_events_city_code_trim_check CHECK (city_code = btrim(city_code)),
  CONSTRAINT social_insurance_policy_events_city_code_upper_check CHECK (city_code = upper(city_code)),
  CONSTRAINT social_insurance_policy_events_hukou_type_trim_check CHECK (hukou_type = btrim(hukou_type)),
  CONSTRAINT social_insurance_policy_events_hukou_type_lower_check CHECK (hukou_type = lower(hukou_type)),
  CONSTRAINT social_insurance_policy_events_insurance_type_check CHECK (
    insurance_type IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND')
  ),
  CONSTRAINT social_insurance_policy_events_policy_fk
    FOREIGN KEY (tenant_id, policy_id) REFERENCES staffing.social_insurance_policies(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS social_insurance_policy_events_tenant_policy_effective_idx
  ON staffing.social_insurance_policy_events (tenant_id, policy_id, effective_date, id);
```

#### 4.2.3 `staffing.social_insurance_policy_versions`

```sql
CREATE TABLE IF NOT EXISTS staffing.social_insurance_policy_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  policy_id uuid NOT NULL,
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  employer_rate numeric(9,6) NOT NULL,
  employee_rate numeric(9,6) NOT NULL,
  base_floor numeric(15,2) NOT NULL,
  base_ceiling numeric(15,2) NOT NULL,
  rounding_rule text NOT NULL,
  precision smallint NOT NULL DEFAULT 2,
  rules_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.social_insurance_policy_events(id),
  CONSTRAINT social_insurance_policy_versions_rules_is_object_check CHECK (jsonb_typeof(rules_config) = 'object'),
  CONSTRAINT social_insurance_policy_versions_rate_check CHECK (
    employer_rate >= 0 AND employer_rate <= 1 AND employee_rate >= 0 AND employee_rate <= 1
  ),
  CONSTRAINT social_insurance_policy_versions_base_check CHECK (
    base_floor >= 0 AND base_ceiling >= base_floor
  ),
  CONSTRAINT social_insurance_policy_versions_rounding_rule_check CHECK (rounding_rule IN ('HALF_UP','CEIL')),
  CONSTRAINT social_insurance_policy_versions_precision_check CHECK (precision >= 0 AND precision <= 2),
  CONSTRAINT social_insurance_policy_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT social_insurance_policy_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT social_insurance_policy_versions_policy_fk
    FOREIGN KEY (tenant_id, policy_id) REFERENCES staffing.social_insurance_policies(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT social_insurance_policy_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      policy_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS social_insurance_policy_versions_lookup_btree
  ON staffing.social_insurance_policy_versions (tenant_id, policy_id, lower(validity));
```

#### 4.2.4 `staffing.payslip_social_insurance_items`

```sql
CREATE TABLE IF NOT EXISTS staffing.payslip_social_insurance_items (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  run_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  base_amount numeric(15,2) NOT NULL,
  employee_amount numeric(15,2) NOT NULL,
  employer_amount numeric(15,2) NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  policy_id uuid NOT NULL,
  policy_last_event_id bigint NOT NULL REFERENCES staffing.social_insurance_policy_events(id),
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_social_insurance_items_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_social_insurance_items_meta_is_object_check CHECK (jsonb_typeof(meta) = 'object'),
  CONSTRAINT payslip_social_insurance_items_amounts_check CHECK (base_amount >= 0 AND employee_amount >= 0 AND employer_amount >= 0),
  CONSTRAINT payslip_social_insurance_items_insurance_type_check CHECK (
    insurance_type IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND')
  ),
  CONSTRAINT payslip_social_insurance_items_payslip_fk
    FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE CASCADE,
  CONSTRAINT payslip_social_insurance_items_run_fk
    FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslip_social_insurance_items_period_fk
    FOREIGN KEY (tenant_id, pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslip_social_insurance_items_policy_fk
    FOREIGN KEY (tenant_id, policy_id) REFERENCES staffing.social_insurance_policies(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslip_social_insurance_items_identity_unique UNIQUE (tenant_id, payslip_id, insurance_type)
);

CREATE INDEX IF NOT EXISTS payslip_social_insurance_items_by_run_btree
  ON staffing.payslip_social_insurance_items (tenant_id, run_id, person_uuid, assignment_id, insurance_type);
```

### 4.3 RLS（必须）

对本计划新增的 4 张表全部启用 RLS 并 `FORCE ROW LEVEL SECURITY`，策略同 `staffing.positions`（示例）：

```sql
ALTER TABLE staffing.social_insurance_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.social_insurance_policies FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.social_insurance_policies;
CREATE POLICY tenant_isolation ON staffing.social_insurance_policies
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- 其余表 social_insurance_policy_events/social_insurance_policy_versions/payslip_social_insurance_items 同构添加。
```

### 4.4 迁移策略（按 `DEV-PLAN-024`）

- **Schema SSOT**：建议新增
  - `modules/staffing/infrastructure/persistence/schema/00006_staffing_payroll_social_insurance_tables.sql`
  - `modules/staffing/infrastructure/persistence/schema/00007_staffing_payroll_social_insurance_engine.sql`
- **生成迁移**：在 `migrations/staffing/` 生成对应 goose 迁移文件，并更新 `migrations/staffing/atlas.sum`；必须保证 `make staffing plan` 最终输出 No Changes。

## 5. 接口契约（API Contracts）

> 口径对齐 `DEV-PLAN-041`：UI 为 HTML + HTMX；同时提供最小 internal API 便于 tests 与排障（route_class=`internal_api`）。

### 5.1 UI：社保政策（单城市）

#### `GET /org/payroll-social-insurance-policies`
- **用途**：展示社保政策列表（按险种）+ 新增/更新政策版本表单。
- **Query（可选）**：`as_of=YYYY-MM-DD`（默认今日；用于查看某日生效的版本）。
- **返回**：HTML 页面（列表 + 表单）。

#### `POST /org/payroll-social-insurance-policies`
- **语义**：为某险种创建（或追加）一个政策版本（事件写入 + versions 投射）。
- **Form Fields**
  - `city_code`（必填；P0 同 tenant 只允许一个；大写；trim 后非空）
  - `hukou_type`（必填；P0 固定为 `default`）
  - `insurance_type`（必填；见 §2.1 枚举）
  - `effective_date`（必填；`YYYY-MM-DD`，日粒度）
  - `employer_rate` / `employee_rate`（必填；decimal string，范围 `[0,1]`）
  - `base_floor` / `base_ceiling`（必填；decimal string，`0 <= floor <= ceiling`）
  - `rounding_rule`（必填；`HALF_UP`/`CEIL`）
  - `precision`（必填；0..2）
  - `rules_config_json`（可选；JSON object；P0 可先只支持空对象或缺省）
- **成功**：303 跳转回 `GET /org/payroll-social-insurance-policies`。
- **失败（422）**：回显表单错误（包括：多城市不支持、户口类型不支持、参数非法、有效期冲突等）。

### 5.2 UI：工资条详情的险种分项展示

> `DEV-PLAN-042` 将提供工资条列表/详情页；本切片在详情页追加“社保扣缴情明细”区块。

- **展示字段（每险种一行）**
  - `insurance_type`（展示名由 i18n 映射）
  - `base_amount`
  - `employee_amount`（个人扣款）
  - `employer_amount`（企业成本）
  - （可选）`rounding_rule/precision` 与 policy 的 `effective_date`（用于解释/排障）
- **汇总字段**
  - `employee_total = Σ employee_amount`
  - `employer_total_add = Σ employer_amount`

### 5.3 Internal API（最小）

- `GET /org/api/payroll-social-insurance-policies?as_of=YYYY-MM-DD`
  - 返回：`[{policy_id,city_code,hukou_type,insurance_type,effective_date,employer_rate,employee_rate,base_floor,base_ceiling,rounding_rule,precision}]`
- `POST /org/api/payroll-social-insurance-policies`
  - JSON body（字段同 UI form；金额/费率用 string）
  - 成功：`201 {policy_id,last_event_db_id,...}`

## 6. 核心逻辑与算法（Business Logic & Algorithms）

### 6.1 Kernel：`submit_social_insurance_policy_event`

**签名（建议）**

```sql
SELECT staffing.submit_social_insurance_policy_event(
  p_event_id      => $1::uuid,
  p_tenant_id     => $2::uuid,
  p_policy_id     => $3::uuid,
  p_city_code     => $4::text,
  p_hukou_type    => $5::text,
  p_insurance_type=> $6::text,
  p_event_type    => $7::text,       -- 'CREATE'/'UPDATE'
  p_effective_date=> $8::date,
  p_payload       => $9::jsonb,      -- keys: employer_rate/employee_rate/base_floor/base_ceiling/rounding_rule/precision/rules_config
  p_request_id    => $10::text,
  p_initiator_id  => $11::uuid
);
```

**算法（必须）**
1. `assert_current_tenant(p_tenant_id)`；校验参数（非空/trim/大小写规范；insurance_type 枚举）。
2. **校验 payload（P0 冻结为“全量字段必填”）**：`employer_rate/employee_rate/base_floor/base_ceiling/rounding_rule/precision` 必须全部存在且可解析；`rules_config` 若存在必须为 JSON object；否则抛 `STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED`。
3. **P0 单城市裁决**：若该 tenant 已存在任一政策且 `city_code <> p_city_code` → 抛 `STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED`；若 `p_hukou_type <> 'default'` → 抛 `STAFFING_PAYROLL_SI_HUKOU_TYPE_NOT_SUPPORTED`。
4. identity upsert（模式同 `submit_assignment_event`）：
   - `INSERT INTO staffing.social_insurance_policies (tenant_id,id,city_code,hukou_type,insurance_type) ... ON CONFLICT (tenant_id, city_code, hukou_type, insurance_type) DO NOTHING;`
   - 读取该 identity 的 `id`，若与 `p_policy_id` 不一致 → 抛 `STAFFING_SOCIAL_INSURANCE_POLICY_ID_MISMATCH`。
5. 对 `policy_id` 加锁：`pg_advisory_xact_lock(hashtextextended(format('staffing:social_insurance_policy:%s:%s', tenant_id, policy_id),0))`。
6. 插入 `social_insurance_policy_events`（`ON CONFLICT(event_id) DO NOTHING`）；若 `event_id` 已存在则做幂等对比，不一致抛 `STAFFING_IDEMPOTENCY_REUSED`。
   - 若触发 `social_insurance_policy_events_one_per_day_unique`（同一 policy 同一天重复写入）→ 抛 `STAFFING_PAYROLL_SI_POLICY_EVENT_ONE_PER_DAY_CONFLICT`（稳定错误码；避免暴露 PG constraint）。
7. `replay_social_insurance_policy_versions(p_tenant_id, p_policy_id)`：同事务删除并重建 versions（并做参数/规则校验）；失败则整笔事务失败（fail-closed）。

**稳定错误码（新增/复用，口径冻结）**
- `STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED`：同 tenant 已存在不同 `city_code` 的政策（P0 单城市停止线）。
- `STAFFING_PAYROLL_SI_HUKOU_TYPE_NOT_SUPPORTED`：`hukou_type <> 'default'`（P0 冻结）。
- `STAFFING_SOCIAL_INSURANCE_POLICY_ID_MISMATCH`：同一 `(tenant,city_code,hukou_type,insurance_type)` 的 identity 已绑定其他 `policy_id`。
- `STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED`：policy event payload 缺少 P0 必填字段或类型非法（P0 不支持 partial update）。
- `STAFFING_PAYROLL_SI_POLICY_EVENT_ONE_PER_DAY_CONFLICT`：同一 policy 在同一天重复写入（`effective_date` 冲突）。
- `STAFFING_PAYROLL_SI_POLICY_MISSING`：算薪时发现 tenant 未配置任何社保政策。
- `STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF`：某险种在 `as_of` 找不到生效版本（fail-closed）。
- `STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD`：pay period 内存在政策变更（P0 不支持分段，必须 fail-closed）。
- `STAFFING_IDEMPOTENCY_REUSED`：`event_id` 被复用但载荷/字段不一致（既有模式复用）。

### 6.2 Kernel：`replay_social_insurance_policy_versions`

**核心规则（必须）**
- `CREATE` 必须为首事件；`UPDATE` 必须有 prior state。
- versions 的 `validity` 必须 gapless，且最后一段必须 `upper_inf(validity)=true`（对齐 `staffing.position/assignment`）。
- **P0 不支持 partial update**：`CREATE/UPDATE` 的 payload 都必须包含完整字段集合：
  - 必填：`employer_rate/employee_rate/base_floor/base_ceiling/rounding_rule/precision`
  - 可选：`rules_config`（若存在必须是 JSON object；缺省 `{}`）
- payload 字段解析（建议全部 `->>` 转 string 再 cast；解析失败即失败）：
  - `employer_rate/employee_rate` → `numeric(9,6)`（`0..1`）
  - `base_floor/base_ceiling` → `numeric(15,2)`（`0 <= floor <= ceiling`）
  - `rounding_rule` → `text`（`HALF_UP`/`CEIL`）
  - `precision` → `smallint`（`0..2`）
  - `rules_config` → `jsonb object`

### 6.3 Payroll Run 计算集成点（对齐 `DEV-PLAN-041`）

> 本切片将 `CALC_FINISH` 的投射逻辑扩展为：在 gross pay 已生成 payslips 的前提下，追加社保扣缴情明细并更新汇总字段。

**入口（建议）**：在 `staffing.submit_payroll_run_event(... event_type='CALC_FINISH' ...)` 内部调用：

- `staffing.payroll_apply_social_insurance(p_tenant_id, p_run_id, p_run_event_db_id)`

**算法（必须，确定性 + 可重算）**
1. 读取 run 与 pay period：
   - `SELECT r.pay_period_id, p.period FROM staffing.payroll_runs r JOIN staffing.pay_periods p ... FOR UPDATE;`
   - `period_start := lower(p.period)`；`period_end_exclusive := upper(p.period)`；`as_of := period_start`（P0 固定使用 period start 作为政策选择锚点）。
2. 推导 P0 `city_code/hukou_type`：
   - `SELECT DISTINCT city_code FROM staffing.social_insurance_policies WHERE tenant_id=...` 必须恰好 1 行，否则抛 `STAFFING_PAYROLL_SI_POLICY_MISSING` / `STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED`。
   - `hukou_type` 必须恰好为 `default`。
3. **P0 禁止 pay period 内政策变更（fail-closed）**：
   - 对每个 `insurance_type`（§2.1），若存在 `social_insurance_policy_events.effective_date` 满足 `period_start < effective_date AND effective_date < period_end_exclusive`（同 tenant/city_code/hukou_type/insurance_type）→ 抛 `STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD`。
4. 清理并重建该 run 的险种明细：
   - `DELETE FROM staffing.payslip_social_insurance_items WHERE tenant_id=... AND run_id=...;`
5. 对每个 payslip（同一 run）：
   - `actual := payslip.gross_pay`（P0 合同：以 `DEV-PLAN-042` 的 gross pay 作为基数“实际收入”输入）
   - 对每个 `insurance_type`（§2.1）：
     - 取政策版本：`SELECT * FROM staffing.social_insurance_policy_versions WHERE tenant_id=... AND city_code=... AND hukou_type='default' AND insurance_type=... AND validity @> as_of ORDER BY lower(validity) DESC LIMIT 1;`；若不存在 → 抛 `STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF`。
     - 基数：`base := GREATEST(base_floor, LEAST(actual, base_ceiling))`
     - 金额：
       - `employee_amount := round_by_rule(base * employee_rate, rounding_rule, precision)`
       - `employer_amount := round_by_rule(base * employer_rate, rounding_rule, precision)`
     - 写入 `payslip_social_insurance_items`：
       - `policy_last_event_id = policy_versions.last_event_id`
       - `last_run_event_id = p_run_event_db_id`
6. 汇总并更新 `payslips`（本切片冻结口径）：
   - `employee_total := Σ employee_amount`
   - `employer_total := Σ employer_amount`
   - `payslips.net_pay := payslips.gross_pay - employee_total`
   - `payslips.employer_total := employer_total`（覆盖写；后续切片会在此基础上追加 IIT / tax gross-up / retro 等口径）

**round_by_rule（必须，确定性实现建议）**

- 签名建议：`staffing.round_by_rule(p_value numeric, p_rounding_rule text, p_precision smallint) RETURNS numeric`
- 实现口径（P0）：
  - `HALF_UP`：`round(p_value, p_precision)`（对正数等价于四舍五入到指定位数）
  - `CEIL`：`scale := power(10::numeric, p_precision)`；`ceiling(p_value * scale) / scale`
- 参数非法（未知 rule / precision 越界）必须抛稳定错误（可复用 `STAFFING_INVALID_ARGUMENT`）。

### 6.4 `DEV-PLAN-044` 专项扣除口径（冻结）

- 对任一 payslip：`special_deduction_social_insurance = Σ payslip_social_insurance_items.employee_amount`
- 查询模板（稳定口径）：

```sql
SELECT COALESCE(sum(employee_amount), 0) AS social_insurance_employee_total
FROM staffing.payslip_social_insurance_items
WHERE tenant_id = $1 AND payslip_id = $2;
```

## 7. 安全与鉴权（Security & Authz）

### 7.1 Authz 对象与动作（冻结口径）

按现有实现（`pkg/authz/registry.go` + `internal/server/authz_middleware.go`）：

- 新增对象常量：
  - `staffing.payroll-social-insurance-policies`（read/admin）
- UI：GET 为 `read`；POST（创建/更新政策版本）为 `admin`。
- internal API：同 UI。

### 7.2 数据隔离

- 新增表全部启用 RLS；应用层不得以“绕过 RLS”作为排障手段进入业务链路。

## 8. 依赖与里程碑（Dependencies & Milestones）

### 8.1 依赖

- 上游合同与路线图：`DEV-PLAN-039/040`。
- 主流程与载体：`DEV-PLAN-041/042`。
- Tenancy/RLS：`DEV-PLAN-019/021`（运行态必须 enforce）。
- DB 迁移闭环：`DEV-PLAN-024`；sqlc（若触及）：`DEV-PLAN-025`。
- 路由与 UI Shell：`DEV-PLAN-017/018`。

### 8.2 里程碑（实现顺序建议）

1. [ ] Schema SSOT：新增社保政策表 + 明细表 + RLS（`modules/staffing/.../schema`）。
2. [ ] Schema→迁移闭环：按 `DEV-PLAN-024` 生成 `migrations/staffing/*` + `atlas.sum`。
3. [ ] Kernel：实现 `submit_social_insurance_policy_event` + replay；补齐 `CALC_FINISH` 中的 `payroll_apply_social_insurance`。
4. [ ] Server：实现 UI 页面与 internal API（含路由 allowlist 与 authz registry）。
5. [ ] Tests：覆盖政策版本化、单城市裁决、舍入合同、算薪写入与汇总一致性、RLS fail-closed。

## 9. 测试与验收标准（Acceptance Criteria）

### 9.1 最小测试矩阵（必须）

- [ ] 政策：创建 6 个险种政策成功；同一 policy 在同一天重复 `effective_date` 被阻断并返回 `STAFFING_PAYROLL_SI_POLICY_EVENT_ONE_PER_DAY_CONFLICT`；policy versions no-overlap（约束/重放一致）。
- [ ] 单城市：创建不同 `city_code` 的政策必须失败（稳定错误码 `STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED`）。
- [ ] 舍入：同一输入在 `HALF_UP(2)` 与 `CEIL(1)` 下输出可复现（锁定用例）。
- [ ] 算薪：给定 gross_pay 与政策参数，生成的 `payslip_social_insurance_items` 行数=险种数；`net_pay/employer_total` 与行求和一致。
- [ ] fail-closed：缺少任一险种 policy（as_of）时，run 计算必须失败（稳定错误码 `STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF`）。
- [ ] fail-closed：pay period 内存在任一险种 policy 变更（生效日落在 `(period_start, period_end_exclusive)`）时，run 计算必须失败（稳定错误码 `STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD`）。
- [ ] RLS：不设置 `app.current_tenant` 时，对新增表的读写全部失败（fail-closed）。

### 9.2 验收脚本（建议以 UI 可操作复现）

1. 在“社保政策”页配置 `CN-310000`（`hukou_type=default`）的 6 险种政策（effective_date=pay period start）。
2. 创建 pay period + payroll run → 计算。
3. 打开工资条详情：看到 6 行险种分项（个人/企业）与汇总；`net_pay = gross_pay - Σ个人扣款`。

## 10. 运维与监控（Ops & Monitoring）

- P0 不引入复杂监控/开关；要求关键日志可定位（建议包含 `tenant_id`, `policy_id`, `run_id`, `payslip_id`, `event_id`, `request_id`）。
- 若后续引入异步计算/外部接口，必须另立 dev-plan 定义重试/幂等/告警与回滚策略。

## 11. 备注（与后续切片的边界）

- `DEV-PLAN-044`：会以本切片冻结的“专项扣除（社保个人部分）”口径进入累计预扣法；不得在 044 内另造第二套求和口径。
- `DEV-PLAN-045/046`：会进一步改变 `net_pay/employer_total` 的组成（回溯结转/税金成本）；本切片只冻结社保部分的明细与求和规则，不提前引入后续复杂度。
