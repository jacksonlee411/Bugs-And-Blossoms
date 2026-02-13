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
  - [ ] 新增审计/幂等事件表 `orgunit.tenant_field_config_events`（按 `DEV-PLAN-100A` §4.4 冻结口径），用于：
    - 通过 `(tenant_uuid, request_code)` 唯一约束实现幂等键占位与复用检测；
    - 提供 DB 内可追溯审计链（action/field_key/physical_col/生效日等）。  
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
  KERNEL --> EVTS[(orgunit.tenant_field_config_events)]
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
	  -- field_key 将作为 payload.ext 的 key 使用，需限制形状以降低注入与漂移风险
	  CONSTRAINT tenant_field_configs_field_key_format_check CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'),
	  CONSTRAINT tenant_field_configs_value_type_check CHECK (value_type IN ('text','int','uuid','bool','date')),
	  CONSTRAINT tenant_field_configs_data_source_type_check CHECK (data_source_type IN ('PLAIN','DICT','ENTITY')),
	  CONSTRAINT tenant_field_configs_data_source_config_is_object_check CHECK (jsonb_typeof(data_source_config) = 'object'),
	  -- 形状冻结（SSOT：DEV-PLAN-100A §4.3）
	  CONSTRAINT tenant_field_configs_plain_config_check CHECK (
	    data_source_type <> 'PLAIN' OR data_source_config = '{}'::jsonb
	  ),
	  CONSTRAINT tenant_field_configs_dict_config_check CHECK (
	    data_source_type <> 'DICT' OR (
	      value_type = 'text'
	      AND data_source_config ? 'dict_code'
	      AND jsonb_typeof(data_source_config->'dict_code') = 'string'
	      AND NULLIF(btrim(data_source_config->>'dict_code'), '') IS NOT NULL
	      -- 禁止额外 keys（冻结形状），避免“隐藏配置”漂移
	      AND data_source_config = jsonb_build_object('dict_code', data_source_config->'dict_code')
	    )
	  ),
	  CONSTRAINT tenant_field_configs_entity_config_check CHECK (
	    data_source_type <> 'ENTITY' OR (
	      data_source_config ? 'entity'
	      AND jsonb_typeof(data_source_config->'entity') = 'string'
	      AND NULLIF(btrim(data_source_config->>'entity'), '') IS NOT NULL
	      AND data_source_config ? 'id_kind'
	      AND jsonb_typeof(data_source_config->'id_kind') = 'string'
	      AND (data_source_config->>'id_kind') IN ('uuid','int')
	      AND (
	        ((data_source_config->>'id_kind') = 'uuid' AND value_type = 'uuid')
	        OR
	        ((data_source_config->>'id_kind') = 'int' AND value_type = 'int')
	      )
	      -- 禁止额外 keys（冻结形状），避免“隐藏配置”漂移
	      AND data_source_config = jsonb_build_object(
	        'entity', data_source_config->'entity',
	        'id_kind', data_source_config->'id_kind'
	      )
	    )
	  ),
	  -- 物理列格式（防注入 + 防漂移；实际列集合由 Kernel 分配保证）
	  CONSTRAINT tenant_field_configs_physical_col_format_check CHECK (
	    physical_col ~ '^ext_(str|int|uuid|bool|date)_[0-9]{2}$'
	  ),
	  CONSTRAINT tenant_field_configs_physical_col_group_check CHECK (
	    (value_type = 'text' AND physical_col LIKE 'ext_str_%')
	    OR (value_type = 'int' AND physical_col LIKE 'ext_int_%')
	    OR (value_type = 'uuid' AND physical_col LIKE 'ext_uuid_%')
	    OR (value_type = 'bool' AND physical_col LIKE 'ext_bool_%')
	    OR (value_type = 'date' AND physical_col LIKE 'ext_date_%')
	  ),
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
  - 允许修改：`disabled_on/disabled_at/updated_at`（其中 `disabled_on` 仅允许 `NULL -> <date>`，不允许回滚为 NULL；若允许调整“未来 disabled_on”，必须满足“未生效 + 仅向后延迟”（SSOT：`DEV-PLAN-100A`）。）  
- 语义约束：
  - `disabled_on` 为空表示“未计划停用”；
  - `disabled_on` 与 `disabled_at` 区分：前者为 Valid Time（day）；后者为审计时间（timestamptz）（SSOT：`DEV-PLAN-032`）。

### 4.2 新增表：`orgunit.tenant_field_config_events`（审计 + 幂等）

> SSOT：`DEV-PLAN-100A` §4.4（`request_code/initiator_uuid` 的幂等与审计口径）。

**建议 Schema（SQL 摘要）**：

```sql
CREATE TABLE orgunit.tenant_field_config_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  event_type text NOT NULL,
  field_key text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_code text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tenant_field_config_events_event_type_check CHECK (event_type IN ('ENABLE','DISABLE')),
  CONSTRAINT tenant_field_config_events_field_key_format_check CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'),
  CONSTRAINT tenant_field_config_events_request_code_unique UNIQUE (tenant_uuid, request_code),
  CONSTRAINT tenant_field_config_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT tenant_field_config_events_event_uuid_unique UNIQUE (event_uuid)
);
```

**payload 形状（冻结口径，便于对齐幂等）**：

- `ENABLE`：必须包含 `physical_col/value_type/data_source_type/data_source_config/enabled_on/disabled_on`（其中 `disabled_on` 可为 `null`）。  
- `DISABLE`：必须包含 `disabled_on`。

**索引（建议）**：

- `UNIQUE(event_uuid)`  
- `(tenant_uuid, transaction_time DESC, id DESC)`

### 4.3 扩展列：`orgunit.org_unit_versions`

- 本阶段只增加“第一批”扩展槽位列：覆盖 MVP 字段所需类型与数量（2~5 个字段），遵循 `DEV-PLAN-100` D2 的命名规则：`ext_str_01..` / `ext_int_01..` / `ext_uuid_01..` / `ext_bool_01..` / `ext_date_01..`。  
- 同步增加 DICT 快照列：
  - `ext_labels_snapshot jsonb`（只存 DICT 字段 label 快照；键集合应限定在“启用字段列表”内，避免任意膨胀；读取优先级见 `DEV-PLAN-100` D4）。

**索引策略（最小预置）**：

- 仅对“明确作为 MVP 列表筛选/排序条件”的少量扩展槽位列增加索引；避免一次性铺满造成维护负担（对齐 `DEV-PLAN-100` Phase 1 原则）。  
- 默认索引形态：
  - `(tenant_uuid, <ext_col>)`；
  - 若列稀疏：允许使用 `WHERE <ext_col> IS NOT NULL` 的部分索引。

### 4.4 迁移文件落点（Atlas + Goose）

> 具体编号以当前 `modules/orgunit/infrastructure/persistence/schema/` 的序号为准；遵循 `DEV-PLAN-024` 的模块闭环流程。

- 新增 schema 迁移：新增“下一号”迁移文件（示例命名：`00016_orgunit_field_configs_schema.sql`）
  - 创建 `orgunit.tenant_field_config_events`（含约束、索引、RLS）
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

- `orgunit.enable_tenant_field_config(p_tenant_uuid uuid, p_field_key text, p_value_type text, p_enabled_on date, p_data_source_type text, p_data_source_config jsonb, p_request_code text, p_initiator_uuid uuid) RETURNS void`
- `orgunit.disable_tenant_field_config(p_tenant_uuid uuid, p_field_key text, p_disabled_on date, p_request_code text, p_initiator_uuid uuid) RETURNS void`

> 注：函数签名与 request_code/initiator_uuid 的幂等与审计口径需在 Phase 0 冻结（SSOT：`DEV-PLAN-100A`）；Phase 1 实现时不得再临时改口径。

### 5.1 稳定错误码（Phase 1 冻结项）

> 目的：避免 Phase 1 实现阶段“撞出来”的隐式错误形状；对齐 `DEV-PLAN-003`（失败路径必须可解释）。

| 场景 | 错误码（建议） | 说明 |
| --- | --- | --- |
| request_code 复用但参数不同 | `ORG_REQUEST_ID_CONFLICT` | 对齐 OrgUnit 既有口径（同租户幂等键冲突）。 |
| 启用字段：field_key 已存在 | `ORG_FIELD_CONFIG_ALREADY_ENABLED` | `tenant_field_configs` 已存在该 `field_key`。 |
| 启用字段：槽位耗尽 | `ORG_FIELD_CONFIG_SLOT_EXHAUSTED` | 目标槽位分组无可用 `physical_col`。 |
| 启用/停用：参数缺失/格式非法 | `ORG_INVALID_ARGUMENT` | 复用 OrgUnit 既有“参数非法”口径。 |
| 启用/停用：data_source_config 形状不合法 | `ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG` | 必须满足 `PLAIN/DICT/ENTITY` 的冻结形状（§4.1）。 |
| 停用字段：不存在 | `ORG_FIELD_CONFIG_NOT_FOUND` | `tenant_field_configs` 未找到该 `field_key`。 |
| 停用字段：disabled_on 不合法 | `ORG_FIELD_CONFIG_DISABLED_ON_INVALID` | 包含：`disabled_on < enabled_on`、`disabled_on < current_date`、或“仅向后延迟/未生效”规则不满足。 |
| 禁止修改映射字段（不可变） | `ORG_FIELD_CONFIG_MAPPING_IMMUTABLE` | DB trigger fail-closed；启用后禁止修改 `field_key/physical_col/value_type/data_source_* /enabled_on`。 |
| 直写表绕过 Kernel（trigger/权限） | `ORGUNIT_FIELD_CONFIGS_WRITE_FORBIDDEN` | 参考现有 `ORGUNIT_CODES_WRITE_FORBIDDEN` guard pattern。 |

> 备注：错误码只是“稳定标识”；HTTP 映射在 Phase 3 统一落地（SSOT：`DEV-PLAN-100D` / routing responder 契约）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 槽位分配（启用字段）

> 关键点（Simple > Easy）：**先做幂等键检查，再做槽位分配**，避免重试时“重新分配槽位导致误判冲突”。

1. 开启事务（显式 tx + tenant 注入；fail-closed）。  
2. 获取租户级写锁（建议 advisory lock）：  
   - `v_lock_key := format('org:field-config-lock:%s', p_tenant_uuid)`  
   - `pg_advisory_xact_lock(hashtextextended(v_lock_key, 0))`  
3. 幂等键检查（按 `(tenant_uuid, request_code)`）：  
   - 若已存在事件：逐字段比对（`event_type/field_key/value_type/data_source_type/data_source_config/enabled_on/disabled_on/initiator_uuid`；**忽略 physical_col**）。  
     - 完全一致：直接返回成功（幂等重试）。  
     - 任一不一致：抛 `ORG_REQUEST_ID_CONFLICT`。  
4. 参数与形状校验：  
   - `field_key/value_type/data_source_type/request_code/initiator_uuid` 必须非空；  
   - `value_type` 必须在 `text|int|uuid|bool|date`；`data_source_type` 必须在 `PLAIN|DICT|ENTITY`；  
   - `data_source_config` 形状必须满足 §4.1（否则 `ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG`）。  
   - `field_key` 是否在“字段定义列表”由服务层保证；Kernel 只做最小防线与不变量保护（避免引入第二套字段定义 SSOT）。  
5. 冲突检查：若 `(tenant_uuid, field_key)` 已存在，抛 `ORG_FIELD_CONFIG_ALREADY_ENABLED`（不允许“删除后复用”）。  
6. 槽位分配（确定性 + 并发安全）：  
   - 依据 `value_type` 决定槽位分组：`text->ext_str_*`、`int->ext_int_*`、`uuid->ext_uuid_*`、`bool->ext_bool_*`、`date->ext_date_*`；  
   - 在该分组的 **固定候选列表** 内，选择“最小可用”槽位（同租户 `tenant_field_configs.physical_col` 未占用）；  
   - 无可用槽位：抛 `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`。  
7. 写入审计事件（`tenant_field_config_events`）：  
   - `event_type='ENABLE'`；`payload` 写入冻结形状（含 `physical_col/value_type/data_source_type/data_source_config/enabled_on/disabled_on(null)`）；  
   - 若写入时发现 `(tenant_uuid, request_code)` 冲突：回退到第 3 步的比对逻辑（防御性；正常情况下锁已避免并发）。  
8. 写入 `tenant_field_configs`（由 Kernel 执行；应用层禁止直写）：  
   - 写入 `physical_col/value_type/data_source_type/data_source_config/enabled_on/disabled_on(null)`；  
   - `created_at/updated_at` 置 `now()`。

### 6.1.1 停用字段（DISABLE）

1. 开启事务 + 获取同一把租户级锁（同 §6.1）。  
2. 幂等键检查：若 request_code 已存在，按输入比对（`event_type=DISABLE/field_key/disabled_on/initiator_uuid`；一致则返回，不一致则 `ORG_REQUEST_ID_CONFLICT`）。  
3. 查询 `tenant_field_configs`：不存在则 `ORG_FIELD_CONFIG_NOT_FOUND`。  
4. 校验 `disabled_on`：  
   - 必须满足：`disabled_on >= enabled_on`；  
   - MVP 冻结：`disabled_on >= current_date`（UTC day；禁止回溯停用，避免历史语义漂移）；  
   - 若旧值 `disabled_on` 非空：仅允许“未生效 + 向后延迟”（SSOT：`DEV-PLAN-100A`），否则 `ORG_FIELD_CONFIG_DISABLED_ON_INVALID`。  
5. 写入审计事件（`tenant_field_config_events`，`event_type='DISABLE'`，payload 至少含 `disabled_on`）。  
6. 更新 `tenant_field_configs`：设置 `disabled_on/disabled_at/updated_at`。

### 6.2 不可变映射与“只允许停用”规则（DB Trigger）

- `BEFORE UPDATE`：
  - 若修改了映射字段（`physical_col` 等），直接抛错（fail-closed）。  
  - 若 `disabled_on` 从非空改为 NULL，抛错。  
  - 若 `disabled_on` 从 `<date>` 改为另一个 `<date>`：仅在 Phase 0 明确允许时放行，且必须满足“仅向后延迟 + 原 disabled_on 尚未生效”（否则抛错）。  

### 6.3 禁止绕过管理入口的写入（DB Guard Trigger）

参照现有模式（例如 `orgunit.guard_org_unit_codes_write()`），对以下表增加 guard trigger：

- `orgunit.tenant_field_config_events`
- `orgunit.tenant_field_configs`

规则：

- 仅允许 `current_user = 'orgunit_kernel'` 的写入（INSERT/UPDATE/DELETE）；否则抛 `ORGUNIT_FIELD_CONFIGS_WRITE_FORBIDDEN`。  

## 7. 安全与鉴权 (Security & Authz)

### 7.1 RLS（强租户隔离）

对 `orgunit.tenant_field_config_events` 与 `orgunit.tenant_field_configs`：

- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY;`
- `ALTER TABLE ... FORCE ROW LEVEL SECURITY;`
- policy 采用与 orgunit 既有表一致的 `tenant_isolation`：
  - `USING/WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid)`（SSOT：`DEV-PLAN-021`；实现时直接复用现有口径，避免自造第二套 tenant 判定）。

### 7.2 权限收口（禁止直写）

- `orgunit_kernel`：
  - 对新表授予 `SELECT/INSERT/UPDATE`（按需要最小化，优先通过 kernel 函数写入）。  
  - 说明：`tenant_field_config_events` 必须允许 `orgunit_kernel` INSERT，用于幂等键占位与审计落点。  
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
     - 本阶段将新建表 `orgunit.tenant_field_config_events` 与 `orgunit.tenant_field_configs`，并对 `orgunit.org_unit_versions` 新增列；执行前必须获得用户手工确认（遵循 `AGENTS.md` 红线）。  
  2. [ ] 新增 schema 迁移：创建 `tenant_field_config_events` 与 `tenant_field_configs`（含约束、索引、RLS）。
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
  - [ ] 新表（`tenant_field_config_events/tenant_field_configs`）具备 RLS + FORCE RLS，且 policy 口径与 orgunit 既有表一致。  
  - [ ] 事件表具备 `(tenant_uuid, request_code)` 唯一约束，且幂等复用检测口径不退化（冲突报 `ORG_REQUEST_ID_CONFLICT`）。  
  - [ ] 不可变映射与“禁止绕过管理入口写入”具备数据库级防线（触发器/权限收口）。  
  - [ ] 若触发 sqlc，`make sqlc-generate` 后 `git status --short` 为空。  

### 9.1 最小手工验证清单（建议写入 PR 证据块）

> 目的：在尚未补齐自动化测试前，至少能用一致的步骤证明“不变量确实被 DB 强制”。

1. **RLS fail-closed**：在未设置 `app.current_tenant` 的连接中尝试 `SELECT` 新表，应失败或返回空（按现有 orgunit policy 口径）。  
2. **禁止直写**：以非 `orgunit_kernel` 角色对新表执行 `INSERT/UPDATE/DELETE`，应抛 `ORGUNIT_FIELD_CONFIGS_WRITE_FORBIDDEN`。  
3. **映射不可变**：对 `tenant_field_configs.physical_col/value_type/data_source_type/data_source_config/enabled_on` 做 `UPDATE`，应失败。  
4. **disabled_on 规则**：  
   - `NULL -> <date>` 允许；  
   - `<date> -> NULL` 禁止；  
   - `<date_future> -> <date_future_later>` 允许（仅向后延迟）；  
   - `<date_future> -> <date_earlier_or_equal>` 禁止。  
5. **幂等键冲突**：同一 `(tenant_uuid, request_code)` 以不同参数调用 Kernel 函数，应抛 `ORG_REQUEST_ID_CONFLICT`。  

## 10. 运维与监控 (Ops & Monitoring)

本阶段不引入额外运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。
