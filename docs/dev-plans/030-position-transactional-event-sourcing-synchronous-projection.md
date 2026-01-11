# DEV-PLAN-030：Position（事务性事件溯源 + 同步投射）方案（去掉 org_ 前缀）

**状态**: 进行中（2026-01-11 10:00 UTC）— M3/M4a 已落地；M4b+ 规划见 §10

> 本计划的定位：作为 Greenfield HR 的 Position/Assignment 子域，提供 **Position/Assignment 的权威契约**（DB Kernel + Go Facade + One Door），并与 `DEV-PLAN-026`（OrgUnit）对齐“事件 SoT + 同步投射 + 可重放”的范式。

## 0. 进度速记（基于实现事实）
- [X] M2（对齐 `DEV-PLAN-009M2`）：Position/Assignments 最小闭环已在主干落地（DB Kernel + UI + Internal API），证据见 §2.4。
- [X] TP-060-02/03：E2E 覆盖“主数据→职位→人员→任职”，Position 作为下游输入已可复现（见 `docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`、`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`）。
- [X] M3：Position 的更新/停用（同 `position_id`）与 UI/Internal API 收敛（见 §10.M3）。
- [X] M4a：`reports_to_position_id` 可编辑 + 无环不变量（forward-only；retro 另拆，见 §10.M4）。
- [ ] M5：与 JobCatalog（`DEV-PLAN-029` 的 SetID 一等公民口径）组合：Position 绑定 Job Profile/Level 的 SetID 口径与落地（见 §3.8 与 §10.M5）。
- [ ] M6：容量模型拆分：M6a `capacity_fte` 可编辑 + `allocated_fte <= capacity_fte`（仍一岗一人）；M6b 多人并发 + `SUM(allocated_fte) <= capacity_fte`（见 §10.M6）。
- [ ] M7：读快照函数/错误码收敛（便于复用与排障，见 §10.M7）。

## 1. 背景与上下文 (Context)
- 本仓库为 Greenfield implementation repo：Position/Assignments 已在 `modules/staffing` 的 DB schema/kernel 与 `/org/*` UI 形成最小闭环（证据见 §2.4）。
- 本计划按 Greenfield（从 0 开始）口径编写：不考虑迁移/兼容；如未来需要承接存量系统的退场/替换，必须另立 dev-plan。
- 本计划采用与 026 相同的 Kernel 边界：DB 负责不变量与投射，Go 只做鉴权/事务/调用与错误映射。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 提供 Position/Assignment 的 schema（events + versions）与最小不变量集合（可识别、可验收、可重放）。
- [X] 与 `DEV-PLAN-026` 的口径对齐：Valid Time=DATE、同日事件唯一、**同事务全量重放（delete+replay）**、versions **no-overlap + gapless**、One Door（`submit_*_event`）。
- [X] 表命名去掉 `org_` 前缀（见 3.2）。
- [ ] 与 Job Catalog（`DEV-PLAN-029`）可组合使用（SetID 口径对齐；见 §3.8 与 §10.M5）。

### 2.2 非目标（明确不做）
- 不提供对旧 API/旧数据的兼容；迁移/退场策略必须另立 dev-plan 承接。
- 不保留/不替代旧的 outbox/audit/settings 等支撑能力（本系列优先收敛 Kernel 最小闭环；如需引入另立 dev-plan）。

### 2.M2 MVP 范围冻结（对齐 `DEV-PLAN-009M2`）

> 目的：把本计划的“大合同”拆出 M2 的最小交付面，避免实现期一次性引入大量动作/不变量导致“Easy but not Simple”。

- [X] 事件类型（M2）：`CREATE`（必选）+ `UPDATE`（可选）；不交付 `CORRECT/RESCIND/SHIFT_BOUNDARY/...`。
- [X] 汇报线（M2）：不交付 `reports_to_position_id` 的编辑能力；写入时将其保持为 `NULL`（使“无环”不变量在 M2 内保持平凡成立，避免引入额外校验复杂度）。
- [X] 占编/容量（M2）：以“每个 position 同一时点最多 1 条 active primary assignment（`allocated_fte∈(0,1]`）”作为最小裁决口径；复杂的 `SUM(allocated_fte) <= capacity_fte` 在后续里程碑再扩展（见 §10.M6）。
- [X] 外部引用（M2）：暂不引入 Job Catalog 绑定字段（避免过早引入 SetID 维度）；M5 再实现（见 §10.M5）。

## 2.3 工具链与门禁（SSOT 引用）
> 本计划仅声明命中项与 SSOT 链接，不复制命令清单。

- **触发器（本计划已命中）**：
  - [X] DB 迁移 / Schema（Atlas+Goose 闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`；模块：staffing）
  - [X] Go 代码（`AGENTS.md`）
  - [X] Routing（`docs/dev-plans/017-routing-strategy.md`）
  - [X] Authz（`docs/dev-plans/022-authz-casbin-toolchain.md`）
  - [X] E2E（`make e2e`：TP-060-02/03）
  - [X] 文档（`make check doc`）
- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁：`.github/workflows/quality-gates.yml`
  - OrgUnit：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
  - Greenfield HR 模块骨架（Position/Assignment 归属 staffing）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
  - 多租户隔离（RLS）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`（对齐 `docs/dev-plans/019-multi-tenant-toolchain.md` / `docs/dev-plans/019A-rls-tenant-isolation.md`）
  - Job Catalog：`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
  - 时间语义（Valid Time=DATE）：`docs/dev-plans/032-effective-date-day-granularity.md`

## 2.4 已落地范围（M2，主干）
> 说明：本节用于把“实现事实”回写到合同文档，避免 009M2/TP060 与 dev-plan 漂移；实现细节以代码/迁移为准。

- [X] 合并记录：PR #43 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/43
- [X] Readiness 证据入口：`docs/dev-records/DEV-PLAN-010-READINESS.md`（§11：009M2）
- [X] Schema SSOT（staffing）：`modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`
- [X] Kernel SSOT（staffing）：`modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`（含 `staffing.submit_position_event` / `staffing.replay_position_versions`）
- [X] 迁移闭环（staffing）：`migrations/staffing/20260106152000_staffing_schema.sql`、`migrations/staffing/20260106152100_staffing_engine.sql`
- [X] DB smoke：`cmd/dbtool/main.go`（`staffing-smoke`）
- [X] UI/Internal API：`internal/server/staffing_handlers.go`、`internal/server/staffing.go`（`/org/positions`、`/org/api/positions`、`/org/assignments`、`/org/api/assignments`）
- [X] E2E：`e2e/tests/tp060-02-master-data.spec.js`、`e2e/tests/tp060-03-person-and-assignments.spec.js`
- [X] TP060 文档证据：`docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`、`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`、`docs/dev-records/dev-plan-063-execution-log.md`

## 2.5 已落地范围（M3，主干）
> 说明：M3 的增量以“生命周期切片 + 交叉不变量（disabled ↔ 任职）”为核心，补齐 UI/Internal API 与最小 E2E 断言。

- [X] 迁移闭环（staffing）：`migrations/staffing/20260111180000_staffing_position_lifecycle_m3.sql`
- [X] Schema/Kernel：`modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`（Position lifecycle + 任职校验）
- [X] UI/Internal API：`internal/server/staffing_handlers.go`、`internal/server/staffing.go`（Position Update/Disable + error code 透传）
- [X] DB smoke：`cmd/dbtool/main.go`（`staffing-smoke` 增加 disable 断言）
- [X] E2E：`e2e/tests/tp060-03-person-and-assignments.spec.js`（disable → assignment fail）

## 2.6 已落地范围（M4a，主干）
> 说明：M4a 先交付 forward-only（只允许追加切片），并在 as-of 下裁决“禁止自指/禁止环/引用必须 as-of active”。

- [X] 迁移闭环（staffing）：`migrations/staffing/20260111193000_staffing_position_reports_to_m4.sql`
- [X] Schema/Kernel：`modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`（reports_to 投射 + 无环裁决 + forward-only）
- [X] UI/Internal API：`internal/server/staffing_handlers.go`、`internal/server/staffing.go`（reports_to 可编辑与展示）
- [X] DB smoke：`cmd/dbtool/main.go`（self/cycle/forward-only 断言）
- [X] E2E：`e2e/tests/tp060-03-person-and-assignments.spec.js`（可见链路 + cycle 负例）

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 Kernel 边界（与 026 对齐）
- **DB = Projection Kernel（权威）**：插入事件（幂等）+ 同步投射 versions + 不变量裁决 + 可 replay。
- **Go = Command Facade**：鉴权/租户与操作者上下文 + 事务边界 + 调 Kernel + 统一 responder 输出（现状：`internal/server` 直接返回/展示错误字符串；M7 收敛稳定错误码）。
- **多租户隔离（RLS）**：tenant-scoped 表默认启用 PostgreSQL RLS（fail-closed；见 `DEV-PLAN-021`），因此运行态必须 `RLS_ENFORCE=enforce`，并在事务内注入 `app.current_tenant`（对齐 `DEV-PLAN-019/019A`）。
- **One Door Policy（写入口唯一）**：除 `submit_*_event` 与运维 replay 外，应用层不得直写事件表/versions 表/identity 表（`positions/assignments`），不得直调 `apply_*_logic`。
- **同步投射机制（选定）**：每次写入都触发**同事务全量重放**（delete+replay），保持逻辑简单，拒绝增量缝补逻辑分叉。

### 3.2 表命名：去掉 `org_` 前缀（评估结论：采用）
**结论（选定）**：Position/Assignment 表统一去掉 `org_` 前缀；实际落点在 `modules/staffing` schema：`staffing.positions/staffing.position_events/staffing.position_versions` 与 `staffing.assignments/staffing.assignment_events/staffing.assignment_versions`。

原因：
- `org_` 在本仓库中已强语义绑定 OrgUnit（组织树）子域；且在 `DEV-PLAN-016` 的模块划分中，Position（`modules/staffing`）与 Job Catalog（`modules/jobcatalog`）为独立子域，继续使用 `org_` 容易造成“权威表达边界”混淆。
- 去前缀后仍保留足够的域前缀（`position_*`/`assignment_*`），可降低与其他模块表名冲突的概率，并为未来从 `modules/org` 抽离模块预留空间。

### 3.3 时间语义（选定）
- Valid Time：`date`；versions 使用 `daterange` 且统一 `[start,end)`（day-range）。
- Audit/Tx Time：`timestamptz`（`transaction_time/created_at`）。

### 3.4 幂等与同日唯一（选定）
- 事件表提供 `event_id` 幂等键（建议应用传入，重试同 `event_id` 不重复投射）。
- 同一 `position_id`（或 `assignment_id`）在同一 `effective_date` 只允许一条事件（不引入 `effseq`）。

### 3.5 gapless（选定，纳入合同）
- `position_versions` / `assignment_versions` 必须无间隙：相邻切片满足 `upper(prev.validity)=lower(next.validity)`，最后一段 `upper_inf(validity)=true`。
- 不允许用“缺行”表达停用/撤销：必须用 `lifecycle_status/status` 的切片表达（保持时间轴连续）。

### 3.6 汇报线无环（M4；M2 保持 `NULL`）
> 说明：M2 阶段不交付 `reports_to_position_id` 编辑能力，因此该字段在投射中固定为 `NULL`，无环不变量在 M2 内平凡成立；M4 再引入“可编辑 + 无环裁决”。

- [X] M2：`reports_to_position_id` 固定为 `NULL`（Position 投射结果不包含汇报线编辑能力）。
- [X] M4a：开放 `reports_to_position_id` 编辑，并由 DB Kernel 裁决“as-of 无环/禁止自指/引用可用性”；写入为 forward-only（见 §10.M4）。
- [ ] M4b（可选，retro）：若要允许 retro 写入，需定义并实现“再验证窗口/锁策略”，确保 retro 不会让未来日期出现环（见 §10.M4）。

### 3.7 占编/容量（M6a/M6b；M2 简化模型）
- [X] M2：同一 position 同一时点最多 1 条 active primary assignment；`allocated_fte` 约束为 `(0,1]`；`capacity_fte` 在投射中固定为 `1.0`。
- [ ] M6a：开放 `capacity_fte` 编辑，并裁决 `allocated_fte <= capacity_fte`（仍保持“一岗同一时点最多 1 条 active primary assignment”）。
- [ ] M6b：允许同一 position 并发多人任职，并裁决 `SUM(allocated_fte) <= capacity_fte`；Position 降容需 fail-closed（避免“事后超编”）；锁/校验策略见 §10.M6。

### 3.8 与 JobCatalog（SetID）组合（M5，必须先定口径）
> 输入事实：Job Catalog（`DEV-PLAN-029`）已将 SetID 视为一等维度，identity/versions 的稳定锚点为 `(tenant_id, setid, id)`。
>
> 因此 Position 若要绑定 Job Profile/Level，必须先明确“SetID 从哪里来/如何落盘/如何校验”的策略；禁止在实现期隐式采用“默认 SHARE”或跨 setid 混用导致历史 as-of 语义不可解释。

- [ ] M5：选定并落地 SetID 传递策略（例如 Position 显式存 `jobcatalog_setid`，或引入 `business_unit_id` 并通过 `orgunit.resolve_setid(...)` 解析），细化见 §10.M5。

### 3.9 与 Payroll/Attendance 的边界（039/050）
- [X] 现状事实：Payroll/Attendance 已落在 `modules/staffing`，且依赖 `staffing.assignment_versions` 的稳定字段（例如 `base_salary/allocated_fte/currency/profile`）与任职写入口（One Door）。
- [ ] 任何在 M3+/M6/M7 中触达 Position/Assignment 的“生命周期/并发/容量/字段”扩展，必须显式评估对 Payroll/Attendance 的兼容性与回归证据（以 `DEV-PLAN-039/050` 与 TP-060/后续 E2E 为验收基线）。

### 3.10 Position 生命周期与任职交叉不变量（M3，先冻结边界）
> 目的：把“Position disable”与“Assignment 引用 Position”的交叉语义先冻结，避免实现期为了让页面能用而临时加后门或分叉逻辑（对齐 `DEV-PLAN-003`）。

- [X] **任职引用（必须 fail-closed）**：任职写入（`submit_assignment_event`）引用的 `position_id` 必须在 `effective_date` as-of 下 **存在且 `lifecycle_status='active'`**；否则拒绝（稳定错误码，见 §7.1）。
- [X] **停用的代价（不做隐式修复）**：停用 Position 不会自动“终止/转移”任职；因此当停用切片会与既有任职发生冲突时，Position 写入必须 fail-closed（由 Kernel 统一裁决，而不是 UI 里堆条件分支）。
- [X] **可发现性（避免僵尸状态）**：disabled 的 Position 必须在 UI 中可见（可被查询/展示），但不得作为新任职的候选项（UI 过滤 + Kernel 校验双重兜底）。

## 4. 数据模型与约束 (Data Model & Constraints)
> 说明：以下为 schema 级合同（字段/约束/索引）；DDL 的 SSOT 为 `modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`。
>
> 约定（与 orgunit/jobcatalog 同构）：Greenfield 业务表不建立到 `iam.tenants(id)` 的 FK；`tenant_id` 仅用于 RLS/隔离与约束，租户注入口径见 `DEV-PLAN-019/021`。

### 4.1 `staffing.positions`（稳定实体）
```sql
CREATE TABLE IF NOT EXISTS staffing.positions (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id)
);
```

合同要点：
- [X] identity 仅承载稳定锚点；展示字段/有效期字段落在 `staffing.position_versions`。
- [ ] M3+ 若需要稳定业务键（如 `position_code`），必须另立迁移并补齐 UI/校验/索引（见 §10.M3 的评估项）。

### 4.2 `staffing.position_events`（Write Side / SoT）
> M2 现状：仅支持 `CREATE/UPDATE`；`payload` 必须为 object；同一实体同日唯一；`event_id` 幂等；`request_id`（按 tenant）唯一。

```sql
CREATE TABLE IF NOT EXISTS staffing.position_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  position_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT position_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT position_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT position_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT position_events_one_per_day_unique UNIQUE (tenant_id, position_id, effective_date),
  CONSTRAINT position_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT position_events_position_fk
    FOREIGN KEY (tenant_id, position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT
);
```

事件合同（M2）：
- [X] `CREATE`：必填 `payload.org_unit_id`；可选 `payload.name`。
- [X] `UPDATE`：payload 为 patch；M2 仅允许 keys：`org_unit_id`、`name`。  
  实现现状：未知 key 不会参与投射（允许存在但会被忽略）；M7 再收敛为“未知 key 拒绝”。

事件合同（M3+ 演进，按里程碑解锁）：
- [ ] M3：允许 `payload.lifecycle_status ∈ {'active','disabled'}`（沿用 `event_type='UPDATE'`，**不新增** `event_type='DISABLE'`），并投射到 `position_versions.lifecycle_status`。
- [ ] M4：允许 `payload.reports_to_position_id`（并执行无环裁决）。
- [ ] M6a：允许 `payload.capacity_fte`（并执行容量裁决）。
- [ ] M5：允许 Position 绑定 JobCatalog（先 `job_profile`，必要时再扩展 `job_level`），并冻结 payload 字段名与校验策略（SetID 一等公民；见 §10.M5）。

### 4.3 `staffing.position_versions`（Read Side / Projection）
```sql
CREATE TABLE IF NOT EXISTS staffing.position_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  position_id uuid NOT NULL,
  org_unit_id uuid NOT NULL,
  reports_to_position_id uuid NULL,
  name text NULL,
  lifecycle_status text NOT NULL DEFAULT 'active',
  capacity_fte numeric(9,2) NOT NULL DEFAULT 1.0,
  profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.position_events(id),
  CONSTRAINT position_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT position_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT position_versions_capacity_fte_check CHECK (capacity_fte > 0),
  CONSTRAINT position_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object'),
  CONSTRAINT position_versions_lifecycle_status_check CHECK (lifecycle_status IN ('active','disabled')),
  CONSTRAINT position_versions_position_fk
    FOREIGN KEY (tenant_id, position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT position_versions_reports_to_fk
    FOREIGN KEY (tenant_id, reports_to_position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT position_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      position_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);
```

合同要点（M2）：
- [X] Valid Time：`validity` 使用 `daterange [start,end)`；由 replay 校验 gapless + 末段 infinity。
- [X] OrgUnit 引用：`org_unit_id` 必须 as-of 存在且 active（由 replay 校验）。
- [X] 汇报线：`reports_to_position_id` 投射固定为 `NULL`（M4 再开放编辑）。
- [X] 生命周期：`lifecycle_status` 投射固定为 `active`（M3 再交付 disable 切片）。
- [X] 容量：`capacity_fte` 投射固定为 `1.0`（M6 再升级容量模型）。
- [X] Profile：`profile` 投射固定为 `{}`（后续如要使用，必须冻结 JSON 形状与迁移策略）。

### 4.4 Assignments（同域能力；与 Position 的交叉不变量）
> 说明：Assignment 的详细合同由 `DEV-PLAN-031` 承接；本节仅记录与 Position 耦合且 M2 已落地的最小形状（SSOT：`modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`）。

- [X] `staffing.assignments`：以 `(tenant_id, person_uuid, assignment_type)` 唯一（当前仅支持 `assignment_type='primary'`）。
- [X] `staffing.assignment_versions`：通过 `EXCLUDE (tenant_id, position_id, validity) WHERE (status='active')` 实现 M2“同一时点一个 position 最多被一个 active assignment 占用”的裁决口径。
- [X] Payroll 输入字段：`base_salary/currency/profile/allocated_fte` 已在 `assignment_versions` 投射（对齐 `DEV-PLAN-039/042`）。
- [ ] M3：任职写入必须校验 Position 在 `effective_date` as-of 下为 active（禁止把任职写到 disabled position）。
- [ ] M6a：任职写入必须校验 `allocated_fte <= capacity_fte`（capacity 由 Position 侧投射与裁决）。
- [ ] M6b（多人并发）若要支持“同一 position 并发多人任职”，需调整上述排他策略并由 Kernel 改为 `SUM(allocated_fte) <= capacity_fte`（见 §10.M6）。

## 5. Kernel 写入口（One Door）
> 选定：**同事务全量重放（delete+replay）**。应用层只调用 `staffing.submit_*_event(...)`；禁止直写 events/versions/identity 表。

### 5.1 并发互斥（Advisory Lock）
- [X] M2 现状实现（SSOT：`modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`）：
  - Position：`staffing:position:<tenant_id>:<position_id>`
  - Assignment：`staffing:assignment:<tenant_id>:<assignment_id>`
- [ ] M3/M4/M6 若引入跨实体不变量（生命周期↔任职、汇报线、多人并发容量），需评估锁粒度升级为 tenant 级锁或多锁策略（见 §10.M3/M4/M6）。

### 5.2 `staffing.submit_position_event`（同事务全量重放）
函数签名（实现现状）：
```sql
CREATE OR REPLACE FUNCTION staffing.submit_position_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_position_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
) RETURNS bigint;
```

合同语义（M2，已落地）：
- [X] RLS 断言：`staffing.assert_current_tenant(p_tenant_id)`。
- [X] 参数校验：`event_id/position_id/effective_date/request_id/initiator_id` 必填；`p_event_type ∈ {'CREATE','UPDATE'}`。
- [X] 互斥锁：按 `staffing:position:<tenant_id>:<position_id>` 获取 `pg_advisory_xact_lock(...)`。
- [X] identity：`INSERT INTO staffing.positions (tenant_id, id) ... ON CONFLICT DO NOTHING`。
- [X] events：写入 `staffing.position_events`（`event_id` 幂等；同日唯一/`request_id` 唯一由约束裁决）。
- [X] 幂等复用校验：同 `event_id` 参数不同 → `STAFFING_IDEMPOTENCY_REUSED`；完全相同 → 返回既有 event db id。
- [X] 写后同步投射：调用 `staffing.replay_position_versions(p_tenant_id, p_position_id)`（同一事务内）。
- [ ] M3：沿用 `event_type='UPDATE'` 承载 `lifecycle_status` patch，并投射到 versions；同时补齐“disabled ↔ 任职”的交叉裁决（见 §3.10 与 §10.M3）。

### 5.3 `staffing.replay_position_versions`
合同语义（M2，已落地）：
- [X] RLS 断言 + 同一把 advisory lock。
- [X] `DELETE FROM staffing.position_versions WHERE tenant_id=? AND position_id=?;` 后按事件序列重建切片。
- [X] 事件约束：`CREATE` 必须为首事件；`UPDATE` 需要 prior state，否则报 `STAFFING_INVALID_EVENT`。
- [X] OrgUnit as-of 校验：`orgunit.org_unit_versions` 在 `effective_date` 下 active 且存在，否则报 `STAFFING_ORG_UNIT_NOT_FOUND_AS_OF`。
- [X] 投射字段（M2）固定值：`reports_to_position_id=NULL`、`lifecycle_status='active'`、`capacity_fte=1.0`、`profile='{}'`。
- [X] gapless 校验：相邻切片无间隙 + 末段 infinity（错误：`STAFFING_VALIDITY_GAP` / `STAFFING_VALIDITY_NOT_INFINITE`）。
- [X] M3：引入 `payload.lifecycle_status` 可编辑与投射；disabled 切片与 active assignment 冲突时 fail-closed（见 §3.10 / §10.M3）。
- [X] M4a：引入 `payload.reports_to_position_id` 可编辑与投射；引用必须 as-of active；禁止自指/禁止环；并限制为 forward-only（见 §10.M4）。
- [ ] M5/M6：引入 jobcatalog/capacity 的可编辑与裁决逻辑（M6a/M6b；见 §10）。

### 5.4 `staffing.submit_assignment_event` / `staffing.replay_assignment_versions`（简述）
> 说明：Assignment 的详细合同以 `DEV-PLAN-031` 为准；此处只记录已落地的写入口形状与 Payroll/Attendance 相关耦合点。

- [X] 函数签名（实现现状，节选）：`staffing.submit_assignment_event(p_event_id,p_tenant_id,p_assignment_id,p_person_uuid,p_assignment_type,p_event_type,p_effective_date,p_payload,p_request_id,p_initiator_id)`。
- [X] 写入会在同事务内重放 `staffing.assignment_versions` 并投射薪酬输入字段（`base_salary/allocated_fte/currency/profile`）。
- [X] 与 Payroll 的耦合：写入后会调用 `staffing.maybe_create_payroll_recalc_request_from_assignment_event(...)`（对齐 `DEV-PLAN-039/045`），因此任职写入口的语义变更必须显式评估 payroll 回溯链路。

## 6. 读模型与查询
- [X] M2 现状：Go 层直接查询 `staffing.position_versions` / `staffing.assignment_versions`（as-of：`validity @> $as_of::date`）。
- [ ] M7：如需复用/优化，补齐 `staffing.get_position_snapshot(...)`、`staffing.get_assignment_snapshot(...)` 并收敛 server 层 SQL 形状。

## 7. Go 层集成（事务 + 调用 DB）
- [X] M2：Go 层所有读写均显式事务，并通过 `set_config('app.current_tenant', $tenant, true)` 注入租户上下文（对齐 `No Tx, No RLS`）。
- [X] M2：实现落点：`internal/server/staffing.go`（PG store）+ `internal/server/staffing_handlers.go`（UI/Internal API）。
- [ ] M7：错误与 HTTP code 收敛：当前 UI 多为展示原始错误字符串；Internal API 多为泛化 `create_failed/upsert_failed`，需补齐稳定映射与测试。

### 7.1 错误契约（现状：以 DB `MESSAGE` 为稳定码）
> 说明：staffing kernel 使用 `RAISE EXCEPTION USING MESSAGE = '<STABLE_CODE>', DETAIL = '<...>'`；HTTP 层目前大多直出 `err.Error()`（UI）或泛化为 internal API code（待 M7 收敛）。

- [X] RLS（fail-closed）：`RLS_TENANT_CONTEXT_MISSING` / `RLS_TENANT_CONTEXT_INVALID` / `RLS_TENANT_MISMATCH`
- [X] 通用参数/事件：`STAFFING_INVALID_ARGUMENT` / `STAFFING_INVALID_EVENT` / `STAFFING_IDEMPOTENCY_REUSED`
- [X] Position：`STAFFING_ORG_UNIT_NOT_FOUND_AS_OF` / `STAFFING_VALIDITY_GAP` / `STAFFING_VALIDITY_NOT_INFINITE`
- [X] Assignment（示例）：`STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID` / `STAFFING_ASSIGNMENT_BASE_SALARY_INVALID` / `STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED` / `STAFFING_ASSIGNMENT_PROFILE_INVALID`
- [X] M3 生命周期/交叉不变量：`STAFFING_POSITION_NOT_FOUND_AS_OF` / `STAFFING_POSITION_DISABLED_AS_OF` / `STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF`
- [X] M4 汇报线：`STAFFING_POSITION_REPORTS_TO_SELF` / `STAFFING_POSITION_REPORTS_TO_CYCLE`（引用不存在/disabled 复用 `STAFFING_POSITION_NOT_FOUND_AS_OF` / `STAFFING_POSITION_DISABLED_AS_OF`）
- [ ] M5 SetID/BU（来自 `orgunit.resolve_setid`）：`BUSINESS_UNIT_NOT_FOUND` / `BUSINESS_UNIT_DISABLED` / `SETID_MAPPING_MISSING` / `SETID_NOT_FOUND` / `SETID_DISABLED`
- [ ] M6 容量：`STAFFING_POSITION_CAPACITY_EXCEEDED`（`allocated_fte` 或 `SUM(allocated_fte)` 超出；含降容 fail-closed）

## 8. 测试与验收标准 (Acceptance Criteria)
- [X] RLS fail-closed：缺失 `app.current_tenant` 时对 tenant-scoped 表的读写必须失败（证据：`cmd/dbtool staffing-smoke`）。
- [X] 事件幂等：同 `event_id` 重试不重复投射，且参数不一致会 fail（`STAFFING_IDEMPOTENCY_REUSED`）。
- [X] 全量重放：每次写入在同一事务内 delete+replay 对应 versions，写后读强一致。
- [X] 同日唯一：`*_events_one_per_day_unique` 约束阻断同一实体同日重复事件（不引入 effseq）。
- [X] versions no-overlap：`*_versions_no_overlap` 排他约束阻断有效期重叠。
- [X] versions gapless：replay 校验相邻切片无间隙 + 末段 infinity（`STAFFING_VALIDITY_GAP/NOT_INFINITE`）。
- [X] M2 汇报线：`reports_to_position_id` 固定为 `NULL`（因此无环不变量在 M2 内平凡成立）。
- [X] M3 生命周期：支持 `disabled` 切片（可写入可读取），且 disabled 在 UI 可见并不可用于新任职。
- [X] M4a 汇报线：开放编辑后仍必须满足 as-of 无环/禁止自指/引用可用性（forward-only）。
- [ ] M4b（可选）：若要支持 retro 写入，必须定义并实现“再验证窗口/锁策略”，确保 retro 不会让未来日期出现环。
- [X] M2 占编：同一时点一个 position 最多被一个 active assignment 占用（`assignment_versions_position_no_overlap`）。
- [ ] M5 SetID：Position 绑定 JobCatalog 时必须 fail-closed（BU/SetID/mapping 缺失或 disabled），且不允许跨 setid 引用。
- [ ] M6a 容量：`capacity_fte` 可编辑且裁决 `allocated_fte <= capacity_fte`（仍一岗一人）。
- [ ] M6b 容量：若允许并发多人任职，必须以 `capacity_fte` + `SUM(allocated_fte) <= capacity_fte` 裁决，并覆盖 Position 降容的 fail-closed。
- [ ] M7 可排障：读快照函数可复用（DB/Go 收敛查询形状），且错误码映射稳定、回归可定位。
- [X] as-of 查询：任意日期快照结果与 versions 语义一致（`validity @> date`），证据：TP-060-02/03 E2E。

## 9. 运维与灾备（Rebuild / Replay）
当投射逻辑缺陷导致 versions 错误时，可通过 replay 重建读模型（versions 可丢弃重建）：
- Position：`SELECT staffing.replay_position_versions('<tenant_id>'::uuid, '<position_id>'::uuid);`
- Assignment：`SELECT staffing.replay_assignment_versions('<tenant_id>'::uuid, '<assignment_id>'::uuid);`

> 多租户隔离（RLS）：replay 必须在显式事务内先注入 `app.current_tenant`，否则会 fail-closed；函数内部也会获取 advisory lock 与在线写入互斥。

## 10. 后续里程碑拆分（仍需开发）
> 说明：M2 已交付（见 §2.4）。以下里程碑按依赖顺序排列；每个里程碑都必须同时满足“Kernel 闭环 + UI 可见 + 门禁证据”。

### 10.M3：Position Update/Disable（生命周期切片）
- [X] 设计收敛：确定 Position “更新同一 `position_id`”的 UI/接口形态（避免新增第二写入口）。
- [X] 契约冻结：明确 disable 与任职的交叉口径（对齐 §3.10），并冻结稳定错误码（见 §7.1）。
- [X] DB（Position）：沿用 `event_type='UPDATE'` 承载 `payload.lifecycle_status`；在 `replay_position_versions` 投射 `lifecycle_status`，并在产生 `disabled` 切片时执行 fail-closed 校验（不得与既有 active assignment 冲突）。
- [X] DB（Assignment）：任职写入必须校验引用的 Position 在 `effective_date` as-of 下为 active（否则 fail-closed）。
- [X] UI：`/org/positions` 支持选择既有 `position_id` 并提交 UPDATE（含 lifecycle_status）；列表可见 status；Assignments 的 position 下拉仅展示 active positions。
- [X] Tests：dbtool smoke/单测补齐；E2E 增加“创建→停用→不可用于新任职”的最小断言。

### 10.M4：汇报线（reports_to）+ 无环裁决
- [X] M4a（先交付，forward-only）：`reports_to_position_id` 的写入只允许**追加事件**（`effective_date` 必须晚于该 position 当前最后一条事件的 `effective_date`），不允许 retro 改写历史切片；并在 `effective_date` as-of 下裁决：禁止自指、引用必须 as-of active、禁止形成环。
- [X] 并发策略：选择“tenant 级锁”（仅在包含 `reports_to_position_id` patch 时启用），避免跨 position 汇报线写入竞态。
- [X] DB：支持写入 `reports_to_position_id` 并实现 as-of 无环裁决；禁止自指；引用必须 as-of active。
- [X] UI：提供最小编辑入口（选择上级岗位）与可见性（展示汇报上级）。
- [X] Tests：环/自指/引用不存在/引用 disabled 负例；E2E 覆盖一条可见链路。
- [ ] M4b（可选，retro）：若要允许 retro 写入，需定义“再验证窗口”（哪些日期需要重检）并实现，确保 retro 不会让未来日期出现环；补齐对应 tests 与证据。

### 10.M5：与 JobCatalog 组合（SetID 一等公民）
- [ ] SetID 策略定稿（见 §3.8）：SetID 从哪里来（推荐 `orgunit.resolve_setid(...)`）、是否引入 `business_unit_id`、如何落盘与迁移（禁止默认 SHARE）。
- [ ] 范围收敛：明确是否先只绑定 `job_profile`（必要时再扩展 `job_level`），避免一次性引入多维组合导致 UI/校验膨胀。
- [ ] DB：为 Position 增加 JobCatalog 绑定字段并以 `(tenant_id, setid, id)` 做 FK 锚点；引用校验口径默认对齐 `DEV-PLAN-029`：**只要求 identity 存在**，不强制 as-of active（如需更强校验另立子里程碑）。
- [ ] UI：`/org/positions` 引入 Job Profile/Level 选择与显示（必须可解释 SetID 来源）。
- [ ] Tests：SetID 不存在/跨 setid 引用/disabled BU 的 fail-closed 等负例。

### 10.M6：容量模型升级（拆分：M6a/M6b）
- [ ] M6a（先交付）：开放 `capacity_fte` 编辑并投射；任职写入时裁决 `allocated_fte <= capacity_fte`（仍保持“一岗一人”的排他约束不变）；Position 降容必须 fail-closed（避免把既有任职“挤爆”）。
- [ ] M6b（多人并发）：允许同一 position 并发多人任职：移除/调整 `assignment_versions_position_no_overlap`；在 DB Kernel 中裁决 `SUM(allocated_fte) <= capacity_fte`（含 Position 降容 fail-closed），并冻结锁粒度/校验算法（避免竞态下“分别通过、合起来超编”）。
- [ ] 兼容性：显式评估并回归 Payroll/Attendance（`DEV-PLAN-039/050`）链路与 E2E 证据。
- [ ] 红线提示：若 M6b 需要新增表/新建迁移中的 `CREATE TABLE`，必须先获得用户手工确认（见 `AGENTS.md`）。

### 10.M7：读快照函数 + 错误码收敛
- [ ] DB：补齐 `staffing.get_position_snapshot(...)` / `staffing.get_assignment_snapshot(...)`（或等价封装）以收敛查询形状。
- [ ] Go：统一错误码映射与 HTTP 行为（UI/Internal API）：Internal API 的 `routing.ErrorEnvelope.Code` 必须携带稳定码（优先复用 DB `MESSAGE`）；避免“页面显示 DB 原始错误 + API 泛化丢失细节”的漂移。
- [ ] Tests：为错误码/快照函数补齐单测与最小 E2E 覆盖，确保回归可定位。

## 11. 评审落地（DEV-PLAN-003，聚焦 M3-M7）
- [X] M4 拆分：先交付 forward-only（M4a），retro 作为可选子里程碑（M4b），避免一次性引入全局图 retro 校验导致偶然复杂度爆炸。
- [X] M6 拆分：先交付 `capacity_fte` 可编辑（M6a），多人并发（M6b）延后并要求先冻结锁与校验算法。
- [X] M3 选择：沿用 `event_type='UPDATE'` + `payload.lifecycle_status` 表达 disable（对齐 `staffing.time_profile_events` 的既有模式），避免为“好看”引入更多 event_type 分支。
- [ ] M5 SetID 策略：在实现前必须定稿“BU/SetID 来源、落盘字段、引用校验口径”（禁止默认 SHARE），并明确是否先只绑定 `job_profile`。
- [ ] M7 错误码收敛：在实现前必须冻结“稳定码集合 + Go 层映射策略 + 最小回归断言”，避免继续累积 `create_failed/upsert_failed` 这类泛化错误。
