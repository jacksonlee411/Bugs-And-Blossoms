# DEV-PLAN-100B：Org 模块宽表元数据落地 Phase 1：Schema 与元数据骨架（最小数据库闭环）

**状态**: 草拟中（2026-02-13 07:16 UTC）

> 本文从 `DEV-PLAN-100` 的 Phase 1 拆分而来，作为 Phase 1 的 SSOT；`DEV-PLAN-100` 保持为整体路线图。

## 1. 背景与上下文 (Context)

`DEV-PLAN-100A` 已定义 Phase 0 的“契约冻结与就绪检查”（先文档后代码）。在契约冻结完成后，Phase 1 的目标是把“宽表预留字段 + 元数据驱动”的最小数据库骨架落地为可迁移、可回滚、可门禁验证的模块闭环（Atlas+Goose）。

本阶段只做数据库层的最小闭环（Schema / RLS / Kernel 写入口与约束），为后续：

- Phase 2：Kernel/Projection 扩展（投射扩展字段）
- Phase 3：Service/API（读写可用）
- Phase 4：UI（用户可见闭环）

提供稳定的基础设施。

## 2. 目标与非目标 (Goals & Non-Goals)

- **核心目标**：
  - [ ] 新增元数据表 `orgunit.tenant_field_configs`（按 `DEV-PLAN-100A`/`DEV-PLAN-100` 的契约），并完成 RLS、唯一约束、不可变映射约束等数据库级防线。  
  - [ ] 在 `orgunit.org_unit_versions` 增加第一批扩展槽位列（仅覆盖 MVP 字段所需的类型与数量，遵循 `DEV-PLAN-100` D2 的命名规则）。  
  - [ ] 在 `orgunit.org_unit_versions` 增加 `ext_labels_snapshot jsonb`（DICT label 快照；大小与键集合受控，口径见 `DEV-PLAN-100` D3/D4）。  
  - [ ] 提供字段配置管理 Kernel 写入口（单写入口）：启用字段/停用字段（应用层必须调用函数，不允许直写表）。  
  - [ ] 权限与隔离 fail-closed：新表强制 RLS；应用角色不允许通过直接 DML 绕过管理入口（对齐 `AGENTS.md` 与 `DEV-PLAN-021`）。

- **非目标（本阶段不做）**：
  - 不扩展 `orgunit.submit_org_event(...)` payload 校验与投射逻辑（Phase 2）。  
  - 不新增/调整 HTTP API（Phase 3）。  
  - 不实现字段配置 UI（Phase 4；IA 见 `DEV-PLAN-101`）。  
  - 不引入“legacy 回退通道/双链路”（No Legacy，SSOT：`AGENTS.md` / `DEV-PLAN-004M1`）。

## 2.1 工具链与门禁（SSOT 引用）

> 目的：只声明命中哪些触发器与门禁入口，不在本文复制脚本细节（SSOT：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`）。

- **触发器清单（勾选本计划命中的项）**：
  - [X] 文档（`make check doc`）
  - [X] DB 迁移 / Schema（模块级闭环：`make orgunit plan && make orgunit lint && make orgunit migrate up`；SSOT：`DEV-PLAN-024`）
  - [X] sqlc（若 schema/queries/config 受影响：`make sqlc-generate`，并确保 `git status --short` 为空；SSOT：`DEV-PLAN-025`）
  - [ ] 路由治理（本阶段不新增路由；如实现过程中引入管理 API，请按 `make check routing` 自检）
  - [ ] Authz（本阶段不改策略；如新增权限点或策略需对齐 `make authz-pack && make authz-test && make authz-lint`）

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - CI 门禁定义：`docs/dev-plans/012-ci-quality-gates.md`
  - Atlas + Goose 模块闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范与门禁：`docs/dev-plans/025-sqlc-guidelines.md`

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)

```mermaid
graph TD
  API[Admin API (Phase 3)] --> KERNEL[DB Kernel: field_config_* functions]
  KERNEL --> META[(orgunit.tenant_field_configs)]
  KERNEL --> VERS[(orgunit.org_unit_versions ext_* + ext_labels_snapshot)]
  API -->|read| META
  API -->|read| VERS
```

### 3.2 关键设计决策 (ADR 摘要)

- **ADR-100B-01：元数据写入口唯一（Kernel Function）**
  - 选定：字段启用/停用必须通过 `orgunit_kernel` 所属的 `SECURITY DEFINER` Kernel 函数；应用角色对 `tenant_field_configs` 仅允许读，不允许直写（对齐 `DEV-PLAN-100` 约束 5）。  

- **ADR-100B-02：映射不可变**
  - 选定：`(tenant_uuid, field_key) -> physical_col` 启用后不可变；停用不等于可复用（对齐 `DEV-PLAN-100` D2）。  
  - 落地：DB trigger 拒绝修改映射字段；槽位冲突由唯一约束阻断。  

- **ADR-100B-03：status 不落库（按 as_of 推导）**
  - 选定：`tenant_field_configs` 不增加冗余 `status` 列；`enabled/disabled` 由 `as_of` 与 `enabled_on/disabled_on` 推导得到。  
  - 目的：避免“同一行同时存在日期区间语义 + 绝对状态”的双口径漂移。  

## 4. 数据模型与约束 (Data Model & Constraints)

### 4.1 新增表：`orgunit.tenant_field_configs`

> 命名与字段口径以 `DEV-PLAN-100A` 为准；本节补齐 Phase 1 的数据库级约束与迁移落点。

**建议 Schema（SQL 摘要）**：

```sql
CREATE TABLE orgunit.tenant_field_configs (
  tenant_uuid uuid NOT NULL,
  field_key text NOT NULL,
  physical_col text NOT NULL,
  value_type text NOT NULL,
  data_source_type text NOT NULL,
  data_source_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled_on date NOT NULL,
  disabled_on date NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz NULL,
  PRIMARY KEY (tenant_uuid, field_key),
  CONSTRAINT tenant_field_configs_value_type_check CHECK (value_type IN ('text','int','uuid','bool','date')),
  CONSTRAINT tenant_field_configs_data_source_type_check CHECK (data_source_type IN ('DICT','ENTITY')),
  CONSTRAINT tenant_field_configs_data_source_config_is_object_check CHECK (jsonb_typeof(data_source_config) = 'object'),
  CONSTRAINT tenant_field_configs_disabled_on_check CHECK (disabled_on IS NULL OR disabled_on >= enabled_on),
  CONSTRAINT tenant_field_configs_physical_col_unique UNIQUE (tenant_uuid, physical_col)
);
```

**必须具备的 DB 约束/防线**：

- 唯一性：
  - `(tenant_uuid, field_key)`：主键；
  - `(tenant_uuid, physical_col)`：槽位唯一。
- 不可变映射（最小集）：
  - 禁止修改：`field_key/physical_col/value_type/data_source_type/data_source_config/enabled_on`；
  - 允许修改：`disabled_on/disabled_at/updated_at`（其中 `disabled_on` 仅允许 `NULL -> <date>`，不允许回滚为 NULL；是否允许调整“未来 disabled_on”由 Phase 0 冻结）。  
- 语义约束：
  - `disabled_on` 为空表示“未计划停用”；
  - `disabled_on` 与 `disabled_at` 区分：前者为 Valid Time（day）；后者为审计时间（timestamptz）（SSOT：`DEV-PLAN-032`）。

### 4.2 扩展列：`orgunit.org_unit_versions`

- 本阶段只增加“第一批”扩展槽位列：覆盖 MVP 字段所需类型与数量（2~5 个字段），遵循 `DEV-PLAN-100` D2 的命名规则：`ext_str_01..` / `ext_int_01..` / `ext_uuid_01..` / `ext_bool_01..` / `ext_date_01..`。  
- 同步增加 DICT 快照列：
  - `ext_labels_snapshot jsonb`（只存 DICT 字段 label 快照；键集合应限定在“启用字段列表”内，避免任意膨胀；读取优先级见 `DEV-PLAN-100` D4）。

**索引策略（最小预置）**：

- 仅对“明确作为 MVP 列表筛选/排序条件”的少量扩展槽位列增加索引；避免一次性铺满造成维护负担（对齐 `DEV-PLAN-100` Phase 1 原则）。  
- 默认索引形态：
  - `(tenant_uuid, <ext_col>)`；
  - 若列稀疏：允许使用 `WHERE <ext_col> IS NOT NULL` 的部分索引。

### 4.3 迁移文件落点（Atlas + Goose）

> 具体编号以当前 `modules/orgunit/infrastructure/persistence/schema/` 的序号为准；遵循 `DEV-PLAN-024` 的模块闭环流程。

- 新增 schema 迁移：新增“下一号”迁移文件（示例命名：`00016_orgunit_field_configs_schema.sql`）
  - 创建 `orgunit.tenant_field_configs`
  - 为新表启用 RLS（见 §7）
  - 增加不可变映射 trigger（见 §6）
- 变更 org schema：建议在新迁移中对 `orgunit.org_unit_versions` 执行 `ALTER TABLE ... ADD COLUMN ...`
  - 增加第一批 `ext_*` 列
  - 增加 `ext_labels_snapshot jsonb`
- 新增 kernel privileges：新增“下一号”迁移文件（示例命名：`00017_orgunit_field_configs_kernel_privileges.sql`）
  - `orgunit_kernel` 角色授权/收口
  - Kernel 函数 `OWNER`/`SECURITY DEFINER`/`search_path` 设置
  - 对 app/runtime 等角色撤销 DML（与现有 orgunit 口径一致）

## 5. 接口契约 (API Contracts)

本阶段不新增 HTTP API；接口契约在 Phase 3 统一冻结与实现（见 `DEV-PLAN-100` Phase 3）。

本阶段新增的是 DB Kernel 写入口（供 Phase 3 的 API 调用），建议契约：

- `orgunit.enable_tenant_field_config(p_tenant_uuid uuid, p_field_key text, p_enabled_on date, p_data_source_type text, p_data_source_config jsonb, p_request_code text, p_initiator_uuid uuid) RETURNS void`
- `orgunit.disable_tenant_field_config(p_tenant_uuid uuid, p_field_key text, p_disabled_on date, p_request_code text, p_initiator_uuid uuid) RETURNS void`

> 注：函数签名与 request_code/initiator_uuid 的幂等与审计口径需在 Phase 0 冻结（SSOT：`DEV-PLAN-100A`）；Phase 1 实现时不得再临时改口径。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 槽位分配（启用字段）

1. 开启事务（显式 tx + tenant 注入；fail-closed）。  
2. 校验 `field_key` 合法（来源于 Phase 0 冻结的字段定义列表；对齐 `DEV-PLAN-101`）。  
3. 依据 `value_type/data_source_type` 确定槽位分组（例如 `DICT(text) -> ext_str_*`；`ENTITY(uuid) -> ext_uuid_*`）。  
4. 在该分组内选择最小可用槽位（同租户下 `physical_col` 未占用）。  
5. 写入 `tenant_field_configs`（并写 `created_at/updated_at`；必要时写审计字段/日志）。  

### 6.2 不可变映射与“只允许停用”规则（DB Trigger）

- `BEFORE UPDATE`：
  - 若修改了映射字段（`physical_col` 等），直接抛错（fail-closed）。  
  - 若 `disabled_on` 从非空改为 NULL，抛错。  
  - 若 `disabled_on` 从 `<date>` 改为另一个 `<date>`：仅在 Phase 0 明确允许时放行，否则抛错。  

### 6.3 禁止绕过管理入口的写入（DB Guard Trigger）

参照现有模式（例如 `orgunit.guard_org_unit_codes_write()`），对 `tenant_field_configs` 增加 guard trigger：

- 仅允许 `current_user = 'orgunit_kernel'` 的写入（INSERT/UPDATE/DELETE）；否则抛错。  

## 7. 安全与鉴权 (Security & Authz)

### 7.1 RLS（强租户隔离）

对 `orgunit.tenant_field_configs`：

- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY;`
- `ALTER TABLE ... FORCE ROW LEVEL SECURITY;`
- policy 采用与 orgunit 既有表一致的 `tenant_isolation`：
  - `USING/WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid)`（SSOT：`DEV-PLAN-021`；实现时直接复用现有口径，避免自造第二套 tenant 判定）。

### 7.2 权限收口（禁止直写）

- `orgunit_kernel`：
  - 对新表授予 `SELECT/INSERT/UPDATE`（按需要最小化，优先通过 kernel 函数写入）。
  - Kernel 管理函数必须 `OWNER TO orgunit_kernel` 且 `SECURITY DEFINER`，并固定 `search_path`。  
- `app_runtime/app_nobypassrls/superadmin_runtime` 等应用角色：
  - 对新表仅授予 `SELECT`（或不授予，由 API 统一读），并显式 `REVOKE INSERT/UPDATE/DELETE/TRUNCATE`（对齐现有 orgunit 口径）。  

## 8. 依赖与里程碑 (Dependencies & Milestones)

- **前置依赖**：
  - `DEV-PLAN-100A`（Phase 0：字段清单/契约冻结）
  - `DEV-PLAN-024`（Atlas + Goose 模块闭环）
  - `DEV-PLAN-021`（RLS 强租户隔离口径）
  - `DEV-PLAN-032`（Valid Time day 粒度）
  - `DEV-PLAN-101`（字段配置 UI IA：字段定义列表/交互口径）

- **实施步骤（Phase 1）**：
  1. [ ] **Stopline：用户手工确认**  
     - 本阶段将新建表 `orgunit.tenant_field_configs`，并对 `orgunit.org_unit_versions` 新增列；执行前必须获得用户手工确认（遵循 `AGENTS.md` 红线）。  
  2. [ ] 新增 schema 迁移：创建 `tenant_field_configs`（含约束、索引、RLS）。
  3. [ ] 新增 schema 迁移：为 `org_unit_versions` 增加第一批 `ext_*` 槽位列与 `ext_labels_snapshot jsonb`。
  4. [ ] 按 MVP 热点预置少量索引（仅在 Phase 0 字段清单中明确需要 filter/sort 的列；默认 `(tenant_uuid, ext_col)`，必要时部分索引）。
  5. [ ] 新增 triggers：不可变映射（拒绝修改映射字段）+ guard（禁止绕过 kernel 写入）。
  6. [ ] 新增 Kernel 函数：启用字段/停用字段（写入口唯一），并补齐最小审计（时间戳/幂等键）。
  7. [ ] 新增 kernel privileges 迁移：收口权限并对齐现有 `orgunit_kernel` 口径。
  8. [ ] 本地闭环验证：
     - `make orgunit plan && make orgunit lint && make orgunit migrate up`
     - 若触发 sqlc：`make sqlc-generate` 且 `git status --short` 为空

## 9. 测试与验收标准 (Acceptance Criteria)

- **出口条件（Phase 1）**：
  - [ ] `make orgunit plan && make orgunit lint && make orgunit migrate up` 在本地通过。  
  - [ ] 新表具备 RLS + FORCE RLS，且 policy 口径与 orgunit 既有表一致。  
  - [ ] 不可变映射与“禁止绕过管理入口写入”具备数据库级防线（触发器/权限收口）。  
  - [ ] 若触发 sqlc，`make sqlc-generate` 后 `git status --short` 为空。  

## 10. 运维与监控 (Ops & Monitoring)

本阶段不引入额外运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。
