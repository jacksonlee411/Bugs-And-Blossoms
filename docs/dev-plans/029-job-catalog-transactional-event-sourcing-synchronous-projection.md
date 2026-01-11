# DEV-PLAN-029：Job Catalog（事务性事件溯源 + 同步投射）方案（去掉 org_ 前缀）

**状态**: 部分完成（009M1：Job Family Group 最小闭环；M2：Job Family Group 合同对齐补丁；M3：Job Family；M4：Job Level；2026-01-11）

> 本计划的定位：作为 Greenfield HR 的 Job Catalog 子域，提供 **Job Catalog 权威契约**（DB Kernel + Go Facade + One Door），并与 `DEV-PLAN-026/030` 对齐“事件 SoT + 同步投射 + 可重放”的范式。

## 1. 背景与上下文 (Context)
- 历史背景：早期 Job Catalog 曾与 Org/Position 的展示链路强耦合；本计划按 Greenfield（从 0 开始）口径编写，目标是将 Job Catalog 收敛为独立子域的 Kernel（事件 SoT + 同事务同步投射 + 可 replay）。
- 本计划暂不考虑迁移/兼容；如需承接存量退场/替换，必须另立 dev-plan。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 提供 Job Catalog 的 schema（events + identity + versions）与最小不变量集合（可识别、可验收、可重放）。（009M1：仅覆盖 `job_family_groups` 样板）
- [X] 与 026/030 对齐：Valid Time=DATE、同日事件唯一、**同事务全量重放（delete+replay）**、versions **no-overlap + gapless**、One Door（各实体 `submit_*_event`）。（009M1：仅覆盖 `submit_job_family_group_event(...)`）
- [X] 表命名去掉 `org_` 前缀（见 3.2），并与 Position 可组合（030 的 FK 以 `(tenant_id, id)` 为基准）。

### 2.2 非目标（明确不做）
- 不提供对旧 API/旧数据的兼容；迁移/退场策略必须另立 dev-plan 承接。
- 不保留/不替代旧的 outbox/audit/settings 等支撑能力（本系列优先收敛 Kernel 最小闭环；如需引入另立 dev-plan）。

## 2.3 工具链与门禁（SSOT 引用）
> 本计划仅声明命中项与 SSOT 链接，不复制命令清单。

- **触发器（实施阶段将命中）**：
  - [X] DB 迁移 / Schema（Atlas + Goose 闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`）
  - [X] Go 代码（`AGENTS.md`）

## 2.4 009M1 已落地范围（最小样板：Job Family Group）

- schema/迁移：`modules/jobcatalog/infrastructure/persistence/schema/00001_jobcatalog_schema.sql`、`modules/jobcatalog/infrastructure/persistence/schema/00002_jobcatalog_job_family_groups.sql`、`modules/jobcatalog/infrastructure/persistence/schema/00003_jobcatalog_engine.sql`、`migrations/jobcatalog/20260106102000_jobcatalog_schema.sql`、`migrations/jobcatalog/20260106102500_jobcatalog_engine.sql`
- UI 闭环入口：`/org/job-catalog`（实现：`internal/server/jobcatalog.go`；allowlist：`config/routing/allowlist.yaml`）
- SetID 解析依赖：`pkg/setid/setid.go` + `orgunit.resolve_setid(...)`
- 证据：`docs/dev-records/DEV-PLAN-010-READINESS.md`（第 10 节）
- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁：`.github/workflows/quality-gates.yml`
  - Greenfield HR 模块骨架（Job Catalog 归属 jobcatalog）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
  - OrgUnit：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
  - Position：`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
  - 多租户隔离（RLS）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`（认证与租户上下文：`docs/dev-plans/019-tenant-and-authn.md`）
  - 时间语义（Valid Time=DATE）：`AGENTS.md`（时间语义章节）、`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`（Valid Time 口径）与 `docs/dev-plans/032-effective-date-day-granularity.md`

## 2.5 合同更新（对齐 009M1：SetID 一等公民）

> 009M1 的落地样板已把 `setid` 引入 Job Catalog 的 schema 与写入口（并通过 OrgUnit 的 Set Control Mapping 解析得到）。为避免后续实现继续出现“文档与实现分叉”，本计划自此将 `setid` 视为 Job Catalog 的一等维度：
- **所有 Job Catalog identity/events/versions 表均包含 `setid`**（同一租户内允许不同 `setid` 下 code 重名）。
- **所有 Kernel 写入口/读快照均显式接收 `p_setid`**；Go/HTTP 层通过 BU→SetID 解析来填充该参数（对齐 `DEV-PLAN-028` 的 SetID 语义与 `internal/server/jobcatalog.go` 的现状）。

## 2.6 落地路径（可验收分步）
> 目标：把实现拆成“每步可验收”的闭环，避免实现期即兴补丁与契约漂移。

建议按以下顺序推进（每一步都可单独验收并回滚）：
1) **Schema 落盘（不含函数逻辑）**：identity / events / versions / 关系表与必要扩展（例如 `btree_gist` 以支持 `gist_uuid_ops`）；确认约束命名与错误映射口径可稳定识别（见 7.1）。
2) **RLS 落盘**：对所有 tenant-scoped 表开启 RLS 与 fail-closed 策略；定义“tenant 注入缺失/不一致”时的稳定失败形状（见 `DEV-PLAN-021`）。
3) **Kernel 写入口函数（先闭环入库与拒绝）**：实现 `submit_*_event` 的参数校验、幂等与同日唯一（依赖唯一约束）；业务级拒绝必须使用 `MESSAGE` 稳定 code + `DETAIL` 动态信息（见 7.1）。
4) **replay（投射）闭环**：实现 `replay_*_versions`（delete+rebuild）与 gapless/no-overlap 校验；并在 `replay_job_profile_versions` 内裁决 `job_profile_version_job_families` 的“至少一个 family + 恰好一个 primary”不变量（v1 默认不引入触发器分支，保持简单）。
5) **Go Facade 闭环**：实现最小命令层（事务 + tenant 注入 + 调 `submit_*_event`），并把 DB 错误稳定映射到 `pkg/serrors`（见 7.1）。
6) **读模型快照（SQL）**：实现 `get_job_catalog_snapshot(p_tenant_id, p_setid, p_query_date)`，并提供最小查询验收（as-of 一致性）。
7) **端到端最小可发现入口（可选，另计划承接）**：若需要用户可见能力，请在 jobcatalog 模块 presentation 增加最小页面/路由入口或明确由 `DEV-PLAN-009M1` 承接（避免“僵尸能力”）。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 Kernel 边界（与 026/030 对齐）
- **DB = Projection Kernel（权威）**：插入事件（幂等）+ 同步投射 versions + 不变量裁决 + 可 replay。
- **Go = Command Facade**：鉴权/租户与操作者上下文 + 事务边界 + 调 Kernel + 错误映射到 `pkg/serrors`。
- **多租户隔离（RLS）**：tenant-scoped 表默认启用 PostgreSQL RLS（fail-closed；见 `DEV-PLAN-021`），因此运行态必须 `RLS_ENFORCE=enforce`，并在事务内注入 `app.current_tenant`（对齐 `DEV-PLAN-019`）。
- **One Door Policy（写入口唯一）**：除各实体 `submit_*_event` 与运维 replay 外，应用层不得直写事件表/versions 表/identity 表（`job_family_groups/job_families/job_levels/job_profiles`）及关系表，不得直调 `apply_*_logic`。
- **同步投射机制（选定）**：每次写入都触发**同事务全量重放**（delete+replay），保持逻辑简单，拒绝“增量缝补”分支。

### 3.2 表命名：去掉 `org_` 前缀（评估结论：采用）
**结论（选定）**：Job Catalog 表统一去掉 `org_` 前缀，采用 `job_*` 命名（例如 `job_profile_events/job_profiles/job_profile_versions` 等）。

原因：
- `org_` 在本仓库中已强语义绑定 OrgUnit（组织树）子域；Job Catalog 属于独立主数据子域，继续使用 `org_` 会扩大“Org”概念边界并制造漂移。
- 采用 `job_*` 域前缀可避免与其他模块通用表名冲突，并降低未来模块抽离的迁移成本。

### 3.3 时间语义（选定）
- Valid Time：`date`；versions 使用 `daterange` 且统一 `[start,end)`（day-range）。
- Audit/Tx Time：`timestamptz`（`transaction_time/created_at`）。

### 3.4 幂等与同日唯一（选定）
- 事件表提供 `event_id` 幂等键。
- 同一张 events 表内（即每类实体各自的 events 表），同一实体在同一 `effective_date` 只允许一条事件（不引入 `effseq`）。

### 3.5 gapless（选定，纳入合同）
- 各 `*_versions` 必须无间隙：相邻切片满足 `upper(prev.validity)=lower(next.validity)`，最后一段 `upper_inf(validity)=true`。
- 不允许用“缺行”表达停用/撤销：必须用 `is_active/status` 的切片表达（保持时间轴连续）。

### 3.6 为什么“分类数据”也需要 versions + replay（意义与边界）
> 直觉上 Job Catalog 像“字典/分类”，但在 HR 领域它更接近“有效期主数据（SCD2）”：它的变化会影响 Position/Assignment 的 as-of 语义与历史报表一致性。

- **避免“改字典=改历史”**：若只保留当前态（identity 行），任何重命名/停用/归属变更都会让历史快照被动改变，破坏可追溯性与 retro 计算可复现性。
- **支持有效期归属/属性**：例如 `job_family_group_id` 的有效期归属（reparenting）天然是 valid-time 事实，应落在 `job_family_versions`，而不是更新 identity。
- **保持写入简单**：采用“事件入库 → 全量重放生成切片”的固定机制，避免在实现期引入区间 split/merge 的增量缝补算法分叉。
- **成本可控**：Job Catalog 单个实体的事件通常很少（低频变更），按实体 replay 的 delete+rebuild 量级可预期且小于 Position/Assignment 的时间线规模。

## 4. 数据模型与约束 (Data Model & Constraints)
> 说明：以下为 schema 级合同（字段/约束/索引）；具体 DDL 以实施阶段落盘的 schema SSOT 文件为准（对齐 `DEV-PLAN-016`：`modules/jobcatalog/infrastructure/persistence/schema/`）。

### 4.1 Events（Write Side / SoT）
> 决策：不使用 `entity_type/entity_id` 分发器的共享事件表，避免“多主体共用事件表”引入的复杂度；每类实体独立 events 表以满足“同表同日唯一”的合同。

```sql
-- 说明：所有 events 表形状同构；每类实体独立表以保持简单。
-- - 幂等：UNIQUE(event_id)
-- - 同日唯一：UNIQUE(tenant_id, <entity_id>, effective_date)

CREATE TABLE job_family_group_events (
  id               bigserial PRIMARY KEY,
  event_id         uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL,
  setid            text NOT NULL,
  job_family_group_id uuid NOT NULL,
  event_type       text NOT NULL,
  effective_date   date NOT NULL,
  payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id       text NOT NULL,
  initiator_id     uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_group_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT job_family_group_events_one_per_day_unique UNIQUE (tenant_id, setid, job_family_group_id, effective_date),
  CONSTRAINT job_family_group_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT job_family_group_events_group_fk FOREIGN KEY (tenant_id, setid, job_family_group_id) REFERENCES job_family_groups(tenant_id, setid, id) ON DELETE RESTRICT
);

CREATE TABLE job_family_events (
  id               bigserial PRIMARY KEY,
  event_id         uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL,
  setid            text NOT NULL,
  job_family_id    uuid NOT NULL,
  event_type       text NOT NULL,
  effective_date   date NOT NULL,
  payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id       text NOT NULL,
  initiator_id     uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT job_family_events_one_per_day_unique UNIQUE (tenant_id, setid, job_family_id, effective_date),
  CONSTRAINT job_family_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT job_family_events_family_fk FOREIGN KEY (tenant_id, setid, job_family_id) REFERENCES job_families(tenant_id, setid, id) ON DELETE RESTRICT
);

CREATE TABLE job_level_events (
  id               bigserial PRIMARY KEY,
  event_id         uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL,
  setid            text NOT NULL,
  job_level_id     uuid NOT NULL,
  event_type       text NOT NULL,
  effective_date   date NOT NULL,
  payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id       text NOT NULL,
  initiator_id     uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_level_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT job_level_events_one_per_day_unique UNIQUE (tenant_id, setid, job_level_id, effective_date),
  CONSTRAINT job_level_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT job_level_events_level_fk FOREIGN KEY (tenant_id, setid, job_level_id) REFERENCES job_levels(tenant_id, setid, id) ON DELETE RESTRICT
);

CREATE TABLE job_profile_events (
  id               bigserial PRIMARY KEY,
  event_id         uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL,
  setid            text NOT NULL,
  job_profile_id   uuid NOT NULL,
  event_type       text NOT NULL,
  effective_date   date NOT NULL,
  payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id       text NOT NULL,
  initiator_id     uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_profile_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT job_profile_events_one_per_day_unique UNIQUE (tenant_id, setid, job_profile_id, effective_date),
  CONSTRAINT job_profile_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT job_profile_events_profile_fk FOREIGN KEY (tenant_id, setid, job_profile_id) REFERENCES job_profiles(tenant_id, setid, id) ON DELETE RESTRICT
);
```

事件类型与 payload 合同（v1 最小集，选定以可实现为准）：
- **统一约束（所有实体）**：
  - `event_type` 仅允许：`CREATE/UPDATE/DISABLE`。
  - `payload` 必须为 JSON object；未知 key 必须拒绝（稳定错误码见 7.1）。
  - `code` 为 identity 字段，仅允许在 `CREATE` 的 payload 中出现；其余事件若包含 `code` 必须拒绝（identity 不可变）。

#### 4.1.1 `UPDATE` patch 语义（强制，避免实现分叉）
> 统一口径：`UPDATE` 的 `payload` 是“字段级 patch”，只改变出现的 key；未出现的字段保持不变。

- 所有实体：
  - `name`：若出现则全量替换；必须为非空字符串。
  - `description`：若出现则全量替换；允许显式 `null` 表示清空。
  - `is_active`：若出现则全量替换；必须为 boolean。
  - `external_refs`：若出现则全量替换；必须为 JSON object（不做 merge，避免隐藏复杂度与冲突语义）。
- Job Family：
  - `job_family_group_id`：若出现则视为 reparenting（有效期属性变更）；必须为 uuid，且在同一 `tenant_id` 下对应 group identity 存在。
- Job Profile：
  - `job_family_ids`：若出现则语义为“该版本的 families 集合整体替换”（非增量 add/remove）；必须为非空集合且元素不重复；每个 id 必须在同一 `tenant_id` 下存在对应 family identity。
  - `primary_job_family_id`：若出现则必须包含于 `job_family_ids`（若同时出现）；并要求在该版本中满足“恰好一个 primary”（4.4）。

- **Job Family Group（`job_family_group_*`）**
  - `CREATE`：必填 `payload.code`、`payload.name`；可选 `payload.description`、`payload.external_refs`。
  - `UPDATE`：patch；允许 keys：`name`、`description`、`is_active`、`external_refs`。
  - `DISABLE`：等价于 `UPDATE` 设置 `is_active=false`（仍保持 gapless）。

- **Job Family（`job_family_*`，支持 effective-dated reparenting）**
  - `CREATE`：必填 `payload.code`、`payload.name`、`payload.job_family_group_id`；可选 `payload.description`、`payload.external_refs`。
  - `UPDATE`：patch；允许 keys：`name`、`description`、`is_active`、`external_refs`、`job_family_group_id`（reparenting）。
  - `DISABLE`：等价于 `UPDATE` 设置 `is_active=false`。

- **Job Level（`job_level_*`）**
  - `CREATE`：必填 `payload.code`、`payload.name`；可选 `payload.description`、`payload.external_refs`。
  - `UPDATE`：patch；允许 keys：`name`、`description`、`is_active`、`external_refs`。
  - `DISABLE`：等价于 `UPDATE` 设置 `is_active=false`。

- **Job Profile（`job_profile_*`）**
  - `CREATE`：必填 `payload.code`、`payload.name`、`payload.job_family_ids`、`payload.primary_job_family_id`；可选 `payload.description`、`payload.external_refs`。
  - `UPDATE`：patch；允许 keys：`name`、`description`、`is_active`、`external_refs`、`job_family_ids`、`primary_job_family_id`。
    - 若出现 `job_family_ids`：语义为“该版本的 families 集合整体替换”（非增量 add/remove），并要求包含 `primary_job_family_id`。
  - `DISABLE`：等价于 `UPDATE` 设置 `is_active=false`（families 仍需满足“至少一个/恰好一个 primary”）。

不变量（必须）：
- versions 侧 `no-overlap + gapless`（3.5）。
- `job_profile_version_job_families`：每个 `job_profile_versions.id` **至少一个 family** 且 **恰好一个 primary**（4.4）。

### 4.2 Identity（code 唯一性事实源）
> 说明：identity 表用于承载 **稳定 ID**（被外部引用的锚点）与 **code 唯一性**；所有有效期属性与可变关系统一落在 versions 表。
>
> **SetID 口径（选定）**：Job Catalog 的 code 唯一性以 `(tenant_id, setid, code)` 为事实源；同一租户下不同 `setid` 允许 code 重名（符合 Set Control 语义）。

```sql
CREATE TABLE job_family_groups (
  tenant_id uuid NOT NULL,
  setid     text NOT NULL,
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code      varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_groups_tenant_setid_id_key UNIQUE (tenant_id, setid, id),
  CONSTRAINT job_family_groups_tenant_setid_code_key UNIQUE (tenant_id, setid, code)
);

CREATE TABLE job_families (
  tenant_id uuid NOT NULL,
  setid     text NOT NULL,
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code      varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_families_tenant_setid_id_key UNIQUE (tenant_id, setid, id),
  CONSTRAINT job_families_tenant_setid_code_key UNIQUE (tenant_id, setid, code)
);

CREATE TABLE job_levels (
  tenant_id uuid NOT NULL,
  setid     text NOT NULL,
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code      varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_levels_tenant_setid_id_key UNIQUE (tenant_id, setid, id),
  CONSTRAINT job_levels_tenant_setid_code_key UNIQUE (tenant_id, setid, code)
);

CREATE TABLE job_profiles (
  tenant_id uuid NOT NULL,
  setid     text NOT NULL,
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code      varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_profiles_tenant_setid_id_key UNIQUE (tenant_id, setid, id),
  CONSTRAINT job_profiles_tenant_setid_code_key UNIQUE (tenant_id, setid, code)
);
```

> 备注：v1 不强制在各模块 schema 中增加 `tenant_id -> iam.tenants(id)` 的跨 schema FK（避免引入额外耦合）；租户隔离以 RLS fail-closed 为主（对齐 `DEV-PLAN-021`）。

**选定（避免边界漂移）**：
- **支持 effective-dated reparenting**：`job_family_group_id` 作为有效期属性，落在 `job_family_versions`（而不是 identity），通过 `job_family_events → replay_job_family_versions` 变更。
- **code 唯一性口径（对齐 SetID）**：`job_*` 的 `code` 在 schema 层以 `(tenant_id, setid, code)` 唯一；不引入“按 group 维度的时态唯一性”。

> v1 约束（建议固化以保持简单）：identity 的 `code` 视为不可变；如需更换 code，采用“新建实体 + disable 旧实体（versions）”，避免更新 identity 引入第二事实源。

identity 合同补充（v1）：
- identity 行仅允许由各自 `submit_*_event(event_type='CREATE')` 创建；应用层禁止直写。
- `job_*.(tenant_id,setid,code)` 的唯一性是 schema 层强约束；SetID 变化视为“不同维度的主数据”，不在 v1 里做跨 setid 的同实体迁移/重映射。

### 4.3 Versions（Read Side / Projection）
> 说明：各实体 versions 使用 `daterange validity` + EXCLUDE no-overlap，并由 replay 生成 gapless（相邻切片无间隙且末段 infinity）。

示例（Job Profile）：
```sql
CREATE TABLE job_profile_versions (
  id              bigserial PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  setid           text NOT NULL,
  job_profile_id  uuid NOT NULL,

  name            text NOT NULL,
  description     text NULL,
  is_active       boolean NOT NULL DEFAULT TRUE,
  external_refs   jsonb NOT NULL DEFAULT '{}'::jsonb,

  validity        daterange NOT NULL,
  last_event_id   bigint NOT NULL REFERENCES job_profile_events(id),

  CONSTRAINT job_profile_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT job_profile_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT job_profile_versions_external_refs_is_object_check CHECK (jsonb_typeof(external_refs) = 'object'),
  CONSTRAINT job_profile_versions_profile_fk FOREIGN KEY (tenant_id, setid, job_profile_id) REFERENCES job_profiles(tenant_id, setid, id) ON DELETE RESTRICT
);

ALTER TABLE job_profile_versions
  ADD CONSTRAINT job_profile_versions_tenant_setid_id_key UNIQUE (tenant_id, setid, id);

ALTER TABLE job_profile_versions
  ADD CONSTRAINT job_profile_versions_no_overlap
  EXCLUDE USING gist (
    tenant_id gist_uuid_ops WITH =,
    setid gist_text_ops WITH =,
    job_profile_id gist_uuid_ops WITH =,
    validity WITH &&
  );
```

特别：Job Family 归属 Group（effective-dated reparenting）：
```sql
CREATE TABLE job_family_versions (
  id              bigserial PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  setid           text NOT NULL,
  job_family_id   uuid NOT NULL,
  job_family_group_id uuid NOT NULL,

  name            text NOT NULL,
  description     text NULL,
  is_active       boolean NOT NULL DEFAULT TRUE,
  external_refs   jsonb NOT NULL DEFAULT '{}'::jsonb,

  validity        daterange NOT NULL,
  last_event_id   bigint NOT NULL REFERENCES job_family_events(id),

  CONSTRAINT job_family_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT job_family_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT job_family_versions_external_refs_is_object_check CHECK (jsonb_typeof(external_refs) = 'object'),
  CONSTRAINT job_family_versions_family_fk FOREIGN KEY (tenant_id, setid, job_family_id) REFERENCES job_families(tenant_id, setid, id) ON DELETE RESTRICT,
  CONSTRAINT job_family_versions_group_fk FOREIGN KEY (tenant_id, setid, job_family_group_id) REFERENCES job_family_groups(tenant_id, setid, id) ON DELETE RESTRICT
);

ALTER TABLE job_family_versions
  ADD CONSTRAINT job_family_versions_no_overlap
  EXCLUDE USING gist (
    tenant_id gist_uuid_ops WITH =,
    setid gist_text_ops WITH =,
    job_family_id gist_uuid_ops WITH =,
    validity WITH &&
  );
```

其余实体（同构）：
- `job_family_group_versions`（FK→`job_family_groups`；`last_event_id`→`job_family_group_events`）
- `job_level_versions`（FK→`job_levels`；`last_event_id`→`job_level_events`）

索引建议（实现期以 `EXPLAIN` 验证）：
- `*_versions_no_overlap` 会生成 GiST 索引（`tenant_id + <entity_id> + validity`），可保证 as-of 点查命中至多 1 行。
- 若大量查询按租户 + day 拉全量快照，可考虑补充 `gist(tenant_id, validity)` 的 partial 索引（例如 `WHERE is_active = true`），避免扫描大量历史切片。

### 4.4 `job_profile_version_job_families`（ProfileVersion↔Families 多值关系）
> 语义：每个 `job_profile_versions.id` 必须关联 **至少一个** family，且 **恰好一个** `is_primary=true`。

```sql
CREATE TABLE job_profile_version_job_families (
  tenant_id            uuid NOT NULL,
  setid                text NOT NULL,
  job_profile_version_id bigint NOT NULL,
  job_family_id        uuid NOT NULL,
  is_primary           boolean NOT NULL DEFAULT FALSE,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT job_profile_version_job_families_pkey PRIMARY KEY (tenant_id, setid, job_profile_version_id, job_family_id),
  CONSTRAINT job_profile_version_job_families_profile_version_fk
    FOREIGN KEY (tenant_id, setid, job_profile_version_id) REFERENCES job_profile_versions(tenant_id, setid, id) ON DELETE CASCADE,
  CONSTRAINT job_profile_version_job_families_family_fk FOREIGN KEY (tenant_id, setid, job_family_id) REFERENCES job_families(tenant_id, setid, id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX job_profile_version_job_families_primary_unique
  ON job_profile_version_job_families (tenant_id, setid, job_profile_version_id)
  WHERE is_primary = TRUE;
```

> “恰好一个 primary”的 v1 落地方式（选定）：在 `replay_job_profile_versions` 内做计数裁决并用稳定错误码拒绝；不引入 trigger 分支以保持实现简单、可解释。

## 5. Kernel 写入口（One Door）
> 选定：**同事务全量重放（delete+replay）**。每类实体各自 `submit_*_event`，并在同一事务内完成：事件入库（幂等）→ 全量重放（删除并重建对应 `*_versions`/关系表）→ 不变量裁决（含 gapless/primary family 等）。

### 5.1 并发互斥（Advisory Lock）
**锁粒度（选定）**：同一 `tenant_id` 的 Job Catalog 写入串行化，避免跨实体依赖（family↔group、profile↔families）在实现期引入死锁与漂移。

锁 key（文本，选定）：`jobcatalog:write-lock:<tenant_id>:JobCatalog`

### 5.2 写入口（按实体 One Door）
函数签名（建议，与 026/030 对齐）：
```sql
CREATE OR REPLACE FUNCTION submit_job_family_group_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_setid text,
  p_job_family_group_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
) RETURNS bigint;

CREATE OR REPLACE FUNCTION submit_job_family_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_setid text,
  p_job_family_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
) RETURNS bigint;

CREATE OR REPLACE FUNCTION submit_job_level_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_setid text,
  p_job_level_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
) RETURNS bigint;

CREATE OR REPLACE FUNCTION submit_job_profile_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_setid text,
  p_job_profile_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
) RETURNS bigint;
```

统一合同语义（必须）：
0) 多租户上下文（RLS）：写入口函数开头必须断言 `p_tenant_id` 与 `app.current_tenant` 一致（对齐 `DEV-PLAN-021`）。
1) 获取互斥锁：`jobcatalog:write-lock:<tenant_id>:JobCatalog`（同一事务内）。
2) 参数校验：`p_event_type` 必须为 `CREATE/UPDATE/DISABLE`；`p_payload` 必须为 object（空则视为 `{}`）。
   - `p_setid` 必须可 normalize（格式校验与大小写收敛）；写入口应统一使用 `v_setid := normalize_setid(p_setid)`。
3) identity 处理：
   - `CREATE`：从 `payload.code` 创建对应 identity 行（包含 `tenant_id/setid/id/code`）；code 冲突应可稳定映射（推荐用 `23505 + constraint name`）。
   - 非 `CREATE`：要求 identity 行已存在（至少满足 `(tenant_id,setid,id)`）；否则拒绝（稳定错误码见 7.1）。
4) 引用字段校验（选定，见 10.2）：仅校验被引用实体的 identity 存在且属于同一 `p_tenant_id`（`job_family_group_id/job_family_ids/primary_job_family_id`）；不强制 referenced entity 在 `effective_date` 上 `is_active=true` 或“存在有效 versions”。
   - 若在同一事务内既创建依赖方 identity 又创建引用方（例如先建 group 再建 family），必须保证调用顺序先依赖后引用。
5) 写入对应 `*_events`（以 `event_id` 幂等；同一实体同日唯一由约束拒绝）。
6) 幂等复用校验：若 `event_id` 已存在但参数不同，拒绝；若完全相同则返回既有 event 行 id（不重复投射）。
7) 插入成功后调用对应 `replay_*_versions(p_tenant_id, v_setid, <entity_id>)`（同一事务内）生成 gapless versions，并裁决 `job_profile_version_job_families` 等不变量（4.4）。

> 说明：不提供 `submit_job_catalog_event(entity_type, ...)` 这种分发器入口，避免多主体共享事件流带来的复杂度与漂移。

### 5.3 replay（按实体，全量重放）
- `replay_job_family_group_versions(p_tenant_id uuid, p_setid text, p_job_family_group_id uuid)`
- `replay_job_family_versions(p_tenant_id uuid, p_setid text, p_job_family_id uuid)`
- `replay_job_level_versions(p_tenant_id uuid, p_setid text, p_job_level_id uuid)`
- `replay_job_profile_versions(p_tenant_id uuid, p_setid text, p_job_profile_id uuid)`：重建 `job_profile_versions` 与 `job_profile_version_job_families`（删除旧 versions 行后可依赖 FK `ON DELETE CASCADE` 清理旧关系）。

> `replay_*` / `apply_*_logic` 属于 Kernel 内部实现细节：用于把事件投射到各 `*_versions` 与关系表，禁止应用角色直接执行。

> 多租户隔离（RLS，见 `DEV-PLAN-021`）：`replay_*` 函数开头必须断言 `p_tenant_id` 与 `app.current_tenant` 一致。

## 6. 读模型封装与查询
函数签名（建议）：
```sql
CREATE OR REPLACE FUNCTION get_job_catalog_snapshot(
  p_tenant_id uuid,
  p_setid text,
  p_query_date date
) RETURNS TABLE (...);
```

语义：
- `get_job_catalog_snapshot(p_tenant_id, p_setid, p_query_date)`：返回指定 `setid` 下、as-of 的 group/family/level/profile（含 profile↔families 关系）。
  - 返回结果应同时包含：identity 的稳定锚点（`<entity_id>` + `code`）与 versions 的有效期属性（`name/description/is_active/external_refs/validity/last_event_id`）。
  - v1 不强制按 `is_active` 过滤：快照返回“事实”（含 `is_active` 值），展示/筛选由上层决定（对齐 10.2 的“identity-only 引用校验”口径）。

## 7. Go 层集成（事务 + 调用 DB）
- Go 仅负责：鉴权 → 开事务 → 调对应实体的 `submit_*_event` → 提交。
- 错误契约对齐 026：优先用 `SQLSTATE + constraint name` 做稳定映射；业务级拒绝必须使用“稳定 code（`MESSAGE`）+ 动态信息（`DETAIL`）”的异常形状（Go 只解析 `MESSAGE` 做映射）。
- 多租户隔离（RLS）相关失败路径与稳定映射对齐 `DEV-PLAN-021`（fail-closed 缺 tenant 上下文 / tenant mismatch / policy 拒绝）。

### 7.1 错误契约（DB → Go → serrors）
约定（实现阶段建议遵守，避免字符串匹配与即兴漂移）：
- Go 侧对 Postgres 错误优先用 `SQLSTATE`（例如 `23505`、`23P01`）+ `ConstraintName` 做稳定映射。
- 对于业务级拒绝（not-found/already-exists/idempotency-reused/invalid-argument/reference-not-found 等），DB 必须使用机器可识别异常：
  - `RAISE EXCEPTION USING MESSAGE = '<STABLE_CODE>', DETAIL = '<dynamic details>'`；
  - `MESSAGE` 必须是稳定 code（不拼接动态内容），动态信息放在 `DETAIL`；
  - Go 侧只解析 `MESSAGE` 做映射，不依赖自然语言与字符串包含关系。

最小映射表（v1，示例 code；落地时以模块错误码表收敛为准）：

| 场景 | DB 侧来源 | 识别方式（建议） | Go `serrors` code |
| --- | --- | --- | --- |
| Job Catalog 实体不存在 | `submit_*_event` 明确拒绝 | DB exception `MESSAGE` | `JOBCATALOG_NOT_FOUND` |
| 参数/事件类型/payload 不合法 | `submit_*_event` 明确拒绝 | DB exception `MESSAGE` | `JOBCATALOG_INVALID_ARGUMENT` |
| 幂等键复用但参数不同 | `submit_*_event` 明确拒绝 | DB exception `MESSAGE` | `JOBCATALOG_IDEMPOTENCY_REUSED` |
| 引用字段指向不存在的 identity | `submit_*_event` 参数校验 | DB exception `MESSAGE` | `JOBCATALOG_REFERENCE_NOT_FOUND` |
| code 唯一性冲突 | identity 表唯一约束 | `23505` + constraint name | `JOBCATALOG_CODE_CONFLICT` |
| 同一实体同日重复事件 | `*_events_one_per_day_unique` | `23505` + constraint name | `JOBCATALOG_EVENT_CONFLICT_SAME_DAY` |
| 有效期重叠（破坏 no-overlap） | `*_versions_no_overlap` | `23P01` + constraint name | `JOBCATALOG_VALIDITY_OVERLAP` |
| gapless 被破坏（出现间隙/末段非 infinity） | `replay_*_versions` 校验失败 | DB exception `MESSAGE` | `JOBCATALOG_VALIDITY_GAP` / `JOBCATALOG_VALIDITY_NOT_INFINITE` |
| profile↔families 违反“至少一个/恰好一个 primary” | replay 裁决或约束失败 | DB exception `MESSAGE` 或 `23514/23505` | `JOBCATALOG_PROFILE_FAMILY_CONSTRAINT_VIOLATION` |

## 8. 测试与验收标准 (Acceptance Criteria)
- [ ] RLS（对齐 021）：缺失 `app.current_tenant` 时对 tenant-scoped 表的读写必须 fail-closed；tenant mismatch 必须稳定失败可映射。
- [ ] 事件幂等：同 `event_id` 重试不重复投射。
- [ ] 全量重放：每次写入都在同一事务内 delete+replay 对应 versions，且写后读强一致。
- [ ] 同日唯一：同一实体同日提交第二条事件被拒绝且可稳定映射错误码（每类实体独立 events 表）。
- [ ] 引用校验（选定，见 10.2）：仅要求被引用 identity 存在；不强制 referenced entity 在 `effective_date` 上 `is_active=true` 或存在有效 versions（失败必须稳定映射到错误码）。
- [ ] versions no-overlap：任一实体不会产生重叠有效期。
- [ ] versions gapless：相邻切片无间隙且末段到 infinity（失败可稳定映射错误码）。
- [ ] profile↔families：每个 profile version 恰好一个 primary family（DB 约束可验收）。
- [ ] as-of 查询：任意日期快照结果与 versions 语义一致（`validity @> date`）。

## 9. 运维与灾备（Rebuild / Replay）
当投射逻辑缺陷导致 versions 错误时，可通过 replay 重建读模型（versions 可丢弃重建）：
- Group：`SELECT replay_job_family_group_versions('<tenant_id>'::uuid, '<setid>'::text, '<job_family_group_id>'::uuid);`
- Family：`SELECT replay_job_family_versions('<tenant_id>'::uuid, '<setid>'::text, '<job_family_id>'::uuid);`
- Level：`SELECT replay_job_level_versions('<tenant_id>'::uuid, '<setid>'::text, '<job_level_id>'::uuid);`
- Profile：`SELECT replay_job_profile_versions('<tenant_id>'::uuid, '<setid>'::text, '<job_profile_id>'::uuid);`

> 建议在执行前复用同一把维护互斥锁（`jobcatalog:write-lock:<tenant_id>:JobCatalog`）确保与在线写入互斥。

> 多租户隔离（RLS，见 `DEV-PLAN-021`）：replay 必须在显式事务内先注入 `app.current_tenant`，否则会 fail-closed。

## 10. 已选定决策（防止实现期漂移）
> 本节把“容易在实现期即兴决定”的点固化为合同；若未来结论变化，应先更新本计划再改实现。

### 10.1 互斥锁粒度
**结论（选定）**：tenant 内 Job Catalog 写全串行（`jobcatalog:write-lock:<tenant_id>:JobCatalog`）。

- 理由：Job Catalog 通常低频变更；先用最小策略换取实现简单与可解释性，避免跨实体依赖导致死锁与一致性漂移。
- 备选（未选定）：按实体类型拆锁（group/family/level/profile）。如未来需要提升并发，必须先补齐锁顺序规则与跨实体校验边界，并更新本计划。

### 10.2 跨实体 as-of 引用校验
**结论（选定，Simple）**：写入口仅校验被引用实体的 identity 存在（FK/显式检查，且必须属于同一 `tenant_id`）；不强制 referenced entity 在 `effective_date` 上 `is_active=true` 或“存在有效 versions”。

- 理由：减少跨实体耦合与顺序依赖，避免把“引用校验=隐式业务规则”埋入 Kernel 导致分叉；需要更强一致性时再通过更新本计划引入（并补齐同事务多事件顺序约束）。
- 影响：可能出现“profile 在某日引用了当日已禁用的 family/group”。该状态对 as-of 可解释（读侧可通过 `is_active` 决策展示/过滤），但不由 Kernel 在 v1 强制阻断。

## 11. 实施里程碑（029 剩余任务拆分）

> 目标：把 029 的“剩余落地”拆成若干可独立验收、可独立回滚、门禁对齐的里程碑序列；每个里程碑完成后都应回写本计划的验收清单（§8）与 readiness 证据（`docs/dev-records/`）。
>
> 说明：以下里程碑是**实施编排**，不替代本计划既有合同条款；若实施过程中需要改动合同（字段/约束/函数签名/错误码等），必须先更新本计划再进入实现（Contract First）。

- 新增表/迁移（红线）：已获得用户手工确认允许新增表（包括 `job_families/job_levels/job_profiles` 等）。

- [x] **M2：合同对齐补丁（在既有 009M1 上收口）**
  - 范围：在不扩展业务实体的前提下，先把已落地的 `job_family_groups` 补齐到“可作为模板复用”的合同口径。
  - 交付物（至少）：
    - schema：为 `jobcatalog.job_family_groups` 增加 `UNIQUE (tenant_id, setid, id)`（用于后续复合 FK 的稳定锚点），并将 events/versions 侧 FK/唯一约束命名收敛到可稳定映射。
    - kernel：为 `submit_job_family_group_event` 引入 tenant 内写互斥锁（`jobcatalog:write-lock:<tenant_id>:JobCatalog`），并补齐 `event_id` 幂等语义（同 `event_id` 完全相同则返回既有 event_db_id；参数不同则拒绝 `JOBCATALOG_IDEMPOTENCY_REUSED`）。
    - replay：确保 delete+replay 仍保持 gapless/no-overlap 的裁决路径与稳定错误形状。
  - Done（最小验收）：
    - §8 中 “事件幂等/全量重放/同日唯一/versions no-overlap/gapless/RLS” 对 group 至少可验证（允许其余实体未实现）。
  - 记录：已通过 `make jobcatalog plan && make jobcatalog lint && make jobcatalog migrate up`（含 `jobcatalog-smoke`）验证。

- [x] **M3：Job Family（`job_families`，含 effective-dated reparenting）**
  - 范围：落地 Job Family 的 identity/events/versions + submit/replay；支持 `job_family_group_id` 的有效期归属变更（reparenting）。
  - 交付物（至少）：
    - schema：`jobcatalog.job_families/job_family_events/job_family_versions`（含 `setid`、RLS、约束、索引）。
    - kernel：`submit_job_family_event(...)` + `replay_job_family_versions(...)`（同事务 delete+replay；引用校验按 §10.2）。
  - Done（最小验收）：
    - 具备：CREATE/UPDATE/DISABLE 写入 → as-of 读取（直接查 versions 或通过快照函数；快照可留到 M6）。
  - 记录：已通过 `make jobcatalog plan && make jobcatalog lint && make jobcatalog migrate up`（含 `jobcatalog-smoke`）验证（覆盖 reparenting 与 DISABLE）。

- [x] **M4：Job Level（`job_levels`）**
  - 范围：落地 Job Level 的 identity/events/versions + submit/replay。
  - 交付物（至少）：
    - schema：`jobcatalog.job_levels/job_level_events/job_level_versions`（含 `setid`、RLS、约束、索引）。
    - kernel：`submit_job_level_event(...)` + `replay_job_level_versions(...)`（幂等/同日唯一/全量重放对齐 M2 口径）。
  - Done（最小验收）：
    - 具备：CREATE/UPDATE/DISABLE 写入 → as-of 读取闭环。
  - 记录：已通过 `make jobcatalog plan && make jobcatalog lint && make jobcatalog migrate up`（含 `jobcatalog-smoke`）验证（覆盖 level CREATE/UPDATE/DISABLE）。

- [ ] **M5：Job Profile（`job_profiles`）+ Profile↔Families 关系**
  - 范围：落地 Job Profile 的 identity/events/versions + submit/replay，并实现 `job_profile_version_job_families` 关系表与“至少一个 family + 恰好一个 primary”的不变量裁决。
  - 交付物（至少）：
    - schema：`jobcatalog.job_profiles/job_profile_events/job_profile_versions/job_profile_version_job_families`（含 `setid`、RLS、约束、索引）。
    - kernel：`submit_job_profile_event(...)` + `replay_job_profile_versions(...)`，在 replay 内裁决：
      - families 非空、去重；
      - primary 恰好一个且属于 families；
      - 违反时以稳定错误码拒绝（见 §7.1）。
  - Done（最小验收）：
    - 具备：CREATE/UPDATE/DISABLE 写入 → as-of 读取闭环（含 profile↔families 关系可验收）。

- [ ] **M6：读模型快照（`get_job_catalog_snapshot`）**
  - 范围：实现 `get_job_catalog_snapshot(p_tenant_id, p_setid, p_query_date)`，返回 as-of 的 group/family/level/profile（含 profile↔families）。
  - 交付物（至少）：
    - SQL：快照函数本体（RLS 口径 fail-closed），并提供最小查询验收脚本/示例。
  - Done（最小验收）：
    - §8 的 “as-of 查询一致性” 可通过快照函数稳定验收。

- [ ] **M7：Go Facade + UI 可见闭环（可选扩展，但推荐）**
  - 说明：本里程碑用于满足“用户可见性原则”，避免长期积累只有 DB 没有入口的僵尸能力；若短期不做 UI，也必须在 `DEV-PLAN-018`/测试计划中明确验收方式（例如仅用 curl/SQL 验收）。
  - 范围：在 Go 层提供最小闭环入口（事务 + tenant 注入 + 调 `submit_*_event`）并完成错误映射；必要时扩展 `/org/job-catalog` 的 UI 分页/section 或新增子页面（按路由治理门禁）。
  - Done（最小验收）：
    - 至少一条端到端路径：写入（family/level/profile 任一）→ 列表读取（as-of）→ 页面可见。
