# DEV-PLAN-026B：OrgUnit 外部ID兼容（org_code 映射）方案

**状态**: 草拟中（2026-02-02 07:19 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：客户从原有系统迁移，要求保留原组织 ID 作为对外标识。
- **现状约束**：DEV-PLAN-026A 已将 OrgUnit 结构性标识统一为 8 位 `int4`（`org_id/parent_id/root_org_id/path_ids`），并明确 `_code` 用于业务编码。
- **问题本质**：既要保留客户原 ID，又不能破坏 026A 的数值型路径与索引收益。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 在不改变 026A 结构性标识规则的前提下，支持客户“原组织 ID”对外可见。
- [ ] 明确 **org_id（内部结构标识）** 与 **org_code（外部业务标识）** 的刚性边界，避免混淆。
- [ ] 保证每个租户内 `org_code` 唯一且可解析到唯一 `org_id`。
- [ ] 保持事件溯源与投射流程不退化（性能/索引/路径）。

### 2.2 非目标
- 不将 `org_id` 替换为 `org_code`（保持 026A 的 8 位 `int4` 设计）。
- 不支持同一组织同时绑定多个外部 ID（一期不支持多对多）。
- 不在本方案内引入新的运维作业或数据修复工具链。

## 3. 方案概述 (Proposed Design)
### 3.1 方案选择（对比结论）
- **采用方案 2**：保留 `org_id` 作为内部结构标识；新增 `org_code` 作为外部业务标识，并建立映射关系。
- 设计原则：
  - **对外**：只出现 `org_code`（API/UI/导入导出）。
  - **对内**：只使用 `org_id`（事件、投射、路径、索引）。
  - **边界转换**：进入服务边界即 `org_code -> org_id` 解析。

### 3.2 org_code 的刚性规则
- **默认不可变**：org_code 在创建时确定，后续不允许修改（降低一致性风险）。
- **统一归一化**：写入前进行 `trim + upper`；允许输入 `a-z`，存储统一为 `A-Z`；含首尾空白直接判 `org_code_invalid`。
- **允许字符**：`A-Z`、`a-z`、`0-9`、`-`、`_`（允许连续分隔符）。
- **格式约束**：长度 1~16；禁止全空白；禁止首尾空白（由归一化与 DB 约束保证）。
- **推荐正则（输入校验）**：`^[A-Za-z0-9_-]{1,16}$`
- **推荐正则（存储约束）**：`^[A-Z0-9_-]{1,16}$`
- 若未来需要“改码”，必须新增显式事件类型并执行专项方案（另立 dev-plan）。

### 3.3 写入口唯一性与权威来源
- org_code 的**权威来源是事件流**：`CREATE` 事件 payload 必须携带 org_code。
- `org_unit_codes` 是投射产物，仅能由投射器写入；**禁止**旁路写入或手工 UPDATE。
- 迁移/回填通过事件导入完成，避免形成第二写入口（对齐 One Door 不变量）。
- **同事务要求**：`submit_org_event` 的事件写入与同步投射（含 `org_unit_codes`）必须在同一事务内完成；任何写入失败必须回滚事件。
- **强制机制（必须落地）**：
  - 仅投射器使用 `orgunit_kernel` role 具备 `INSERT/UPDATE` 权限；应用读写角色对 `org_unit_codes` 仅 `SELECT`。
  - 生成 SQL/DAO 禁止直接写入 `org_unit_codes`（代码层硬拒绝）。
  - 如需额外防护，可增加 DB 触发器拒绝 `UPDATE/DELETE`（除投射器 role 以外）。

### 3.4 org_id 分配策略（租户隔离 + DB 内部分配）
- **目标**：每个租户独立的 8 位号段（`10000000~99999999`），避免全局序列耗尽。
- **分配入口**：`CREATE` 事件由 DB 内部分配 `org_id`；应用层不再直接调用 `nextval`。
- **契约**：
  - `submit_org_event(...)` 在 `event_type='CREATE'` 且 `p_org_id IS NULL` 时分配 `org_id`。
  - 其他事件类型必须显式提供 `org_id`，否则 `ORG_INVALID_ARGUMENT`。
  - 超出 8 位上限时报错 `ORG_ID_EXHAUSTED`。
- **序列处置（选定）**：**物理清理** `org_id_seq`（删除序列并移除所有引用），确保无残留双轨。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 新增映射表（需用户确认）
> 新增数据库表前需用户确认（仓库规则）。

```sql
CREATE TABLE orgunit.org_unit_codes (
  tenant_uuid    uuid NOT NULL,
  org_id         int  NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  org_code       text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, org_id),
  CONSTRAINT org_unit_codes_org_code_format CHECK (
    length(org_code) BETWEEN 1 AND 16
    AND org_code = upper(btrim(org_code))
    AND org_code ~ '^[A-Z0-9_-]{1,16}$'
  ),
  CONSTRAINT org_unit_codes_org_code_unique UNIQUE (tenant_uuid, org_code)
);
```

- **唯一性**：租户内 `org_code` 唯一、`org_id` 唯一。
- **RLS**：与 `org_events` / `org_unit_versions` 同步租户隔离规则（tenant_uuid）。

### 4.2 org_id 分配器（需用户确认）
> 新增数据库表前需用户确认（仓库规则）。

```sql
CREATE TABLE orgunit.org_id_allocators (
  tenant_uuid uuid NOT NULL,
  next_org_id int NOT NULL CHECK (next_org_id BETWEEN 10000000 AND 100000000),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid)
);
```

- **含义**：`next_org_id` 指向“下一个可分配的 org_id”；分配时返回 `next_org_id` 并自增。
- **并发**：采用单语句 `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`，避免首次初始化并发冲突。
- **RLS**：同样使用 `tenant_uuid` 的强租户隔离。

#### 4.2.1 分配函数（DB 内部）
```sql
CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_next int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
  VALUES (p_tenant_uuid, 10000001)
  ON CONFLICT (tenant_uuid) DO UPDATE
  SET next_org_id = orgunit.org_id_allocators.next_org_id + 1,
      updated_at = now()
  WHERE orgunit.org_id_allocators.next_org_id <= 99999999
  RETURNING next_org_id - 1 INTO v_next;

  IF v_next IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant=%s', p_tenant_uuid);
  END IF;

  RETURN v_next;
END;
$$;
```

### 4.3 是否需要冗余字段（可选）
- 读性能需要时，可在 `org_unit_versions` 中冗余 `org_code`（由投射时写入）。
- 冗余并不改变“权威来源”：权威仍是 `org_unit_codes`。

## 5. 接口与边界契约 (API & Boundary Contract)
### 5.1 外部接口规则（刚性）
- **对外请求/响应仅使用 `org_code`**。
- **禁止** 在外部接口中暴露 `org_id`。
- 同时传 `org_id` 与 `org_code` 视为错误（400）。
- 对外输入允许 `a-z`，服务端会 `trim + upper` 归一化；若含首尾空白或非法字符则 400。
- **响应一律返回归一化后的 `org_code`（全大写）**，避免大小写歧义。

### 5.2 对外契约示例（必须对齐既有路由体系）
> 本节在不改变既有路由的前提下，**冻结字段名与参数名**；路径命名仍需遵循 `DEV-PLAN-017`。

#### 5.2.1 UI/HTMX（RouteClassUI）
**`GET /org/nodes?as_of=YYYY-MM-DD`**
- Query: `as_of`（必填；缺失时 302 重定向到当日 UTC 日期）。
- Response: HTML 页面（列表与表单中仅展示/提交 `org_code`）。

**`POST /org/nodes?as_of=YYYY-MM-DD`**
- Form（字段名冻结为 `org_code` / `parent_code` / `new_parent_code`，不再出现 `org_id`）：  
  - 通用：`action`（可选，默认 `create`），`effective_date`（可选，默认 `as_of`）。  
  - `create`：`org_code`（必填，1~16；可输入小写但会被归一化为大写）、`name`（必填），`parent_code`（可选），`is_business_unit`（可选，bool）。  
  - `rename`：`org_code`（必填），`new_name`（必填）。  
  - `move`：`org_code`（必填），`new_parent_code`（可选）。  
  - `disable`：`org_code`（必填）。  
  - `set_business_unit`：`org_code`（必填），`is_business_unit`（必填，bool）。  
- Response: 303 重定向到 `/org/nodes?as_of=<effective_date>`；校验失败返回 HTML 错误提示。

#### 5.2.2 Internal API（RouteClassInternalAPI）
**`POST /org/api/org-units`（创建示例）**  
Request JSON（字段名冻结为 `org_code` / `parent_code`）：  
```json
{
  "org_code": "BU-001",
  "name": "Business Unit 001",
  "parent_code": "ROOT",
  "effective_date": "2026-01-01",
  "is_business_unit": true,
  "request_code": "REQ-20260101-0001"
}
```
Response 201：  
```json
{
  "org_code": "BU-001",
  "name": "Business Unit 001",
  "effective_date": "2026-01-01",
  "is_business_unit": true
}
```

**`POST /org/api/org-units/move`（移动示例）**  
```json
{
  "org_code": "BU-001",
  "new_parent_code": "HQ",
  "effective_date": "2026-01-15",
  "request_code": "REQ-20260115-0003"
}
```
Response 200：  
```json
{
  "org_code": "BU-001",
  "new_parent_code": "HQ",
  "effective_date": "2026-01-15"
}
```

**`POST /org/api/org-units/rename`（重命名示例）**  
```json
{
  "org_code": "BU-001",
  "new_name": "Business Unit 001 (New)",
  "effective_date": "2026-01-20",
  "request_code": "REQ-20260120-0002"
}
```
Response 200：  
```json
{
  "org_code": "BU-001",
  "new_name": "Business Unit 001 (New)",
  "effective_date": "2026-01-20"
}
```

**`POST /org/api/org-units/disable`（停用示例）**  
```json
{
  "org_code": "BU-001",
  "effective_date": "2026-01-31",
  "request_code": "REQ-20260131-0004"
}
```
Response 200：  
```json
{
  "org_code": "BU-001",
  "effective_date": "2026-01-31",
  "status": "disabled"
}
```

**`POST /org/api/org-units/set-business-unit`（设置业务单元示例）**  
```json
{
  "org_code": "BU-001",
  "effective_date": "2026-01-31",
  "is_business_unit": true,
  "request_code": "REQ-20260131-0005"
}
```
Response 200：  
```json
{
  "org_code": "BU-001",
  "effective_date": "2026-01-31",
  "is_business_unit": true
}
```

> 任何对外接口若需要标识节点，**只允许 `org_code`**；出现 `org_id` 视为契约违规。

### 5.3 错误码与失败路径（对外契约）
- `org_code_invalid`：格式/归一化失败（400）。
- `org_code_conflict`：租户内重复（409）。
- `org_code_not_found`：解析失败（404）。
- `org_code_ambiguous`：理论上不应发生（500，需排障）。

#### 5.3.1 JSON 错误响应格式（RouteClassInternalAPI）
统一使用 `routing.ErrorEnvelope`：
```json
{
  "code": "org_code_invalid",
  "message": "org_code invalid",
  "request_id": "",
  "meta": {
    "path": "/org/api/org-units",
    "method": "POST"
  }
}
```
> 说明：`request_id` 预留，当前可能为空；后续与 tracing/request 号接入时填充。

### 5.4 内部逻辑规则
- 所有领域逻辑使用 `org_id`。
- 进入服务层第一步执行 `ResolveOrgID(tenant, org_code)`。
- 需要对外输出时执行 `ResolveOrgCode(tenant, org_id)`。

### 5.5 映射解析器（建议）
- `ResolveOrgID` 与 `ResolveOrgCode` 作为公共组件（pkg 级），由 presentation/service 层调用，避免跨层泄漏。
- **调用约束（硬性）**：仅接受“显式事务句柄 + tenant_uuid”，函数内部必须 `assert_current_tenant`；无事务或缺 tenant 直接失败（fail-closed）。
- 默认不启用缓存；如需缓存需显式说明失效策略（创建 org 时失效或短 TTL）。

### 5.6 UI/表单字段迁移清单（从 id → code）
- 列表/详情展示：`org_id` 列替换为 `org_code`（如需保留内部 ID，仅允许在 debug/内部视图展示）。
- `/org/nodes` 表单字段替换：
  - `parent_id` → `parent_code`
  - `org_id` → `org_code`
  - `new_parent_id` → `new_parent_code`
- 表单/校验提示文案同步调整（明确 1~16、允许小写输入会转大写）。
- 相关测试与文档同步更新：
  - `internal/server/orgunit_nodes_test.go`、`e2e/tests/*`、`docs/dev-records/DEV-PLAN-010-READINESS.md` 中涉及 `org_unit_id/org_id` 的断言与示例。

### 5.7 路由/Allowlist/Authz 变更清单（对齐 DEV-PLAN-017）
- `config/routing/allowlist.yaml`：
  - 新增 `/org/api/org-units`（POST）、`/org/api/org-units/move`（POST）、`/org/api/org-units/rename`（POST）、`/org/api/org-units/disable`（POST）、`/org/api/org-units/set-business-unit`（POST）。
  - 现有 `/org/nodes` UI 路由保持不变，仅字段变更。
- `internal/server/authz_middleware.go`：
  - 新增 `/org/api/org-units*`（create/move/rename/disable/set-business-unit）的鉴权映射：`authz.ObjectOrgUnitOrgUnits` + `authz.ActionAdmin`。
  - **移除** `/orgunit/api/*` 的写入口与鉴权映射（若仍存在只读需求，需另立计划并严格限定）。

## 6. 事件与投射 (Event & Projection)
- `CREATE` 事件 payload 必须包含 `org_code`，投射时写入 `org_unit_codes`。
- 其他事件类型不变；若 org_code 不变则不需要额外事件。
- 若未来支持“改码”，需新增事件类型并同步更新 `org_unit_codes` 与投射逻辑。
- **写入前校验**：事件写入前先校验 org_code 格式与唯一性，避免投射阶段失败。
- **投射失败处理**：若仍发生冲突/格式错误，按“失败即阻断”处理（停止投射并记录错误），不得继续推进后续事件；修复后可重放。
- **原子性**：事件提交与同步投射（含 `org_unit_codes`）必须在同事务内完成；投射失败需回滚事件。
- **CREATE 分配**：`submit_org_event` 内部分配 `org_id`（租户隔离），并返回该 ID 供后续流程使用。

## 7. 迁移策略 (Migration)
### 7.1 新客户迁移
- 将“原系统组织 ID”写入 `org_code`。
- `org_id` 由 DB 内部按租户分配；映射关系持久化在 `org_unit_codes`。

### 7.2 既有数据回填
- 若已有组织数据：批量回填 `org_code`（来源于原系统 ID 或迁移清单）。
- 回填必须先做归一化与唯一性校验；冲突数据阻断迁移并输出清单。
- 若无可用外部 ID，可暂以格式化 `org_id` 作为占位，但需标记来源（避免误当真实外部 ID）。
- 初始化 `org_id_allocators.next_org_id`：按租户取 `max(org_id)+1`，空表则使用 `10000000`。

### 7.3 迁移校验
- 租户内 `org_code` 唯一性校验。
- `org_code -> org_id` 可逆性校验（双向解析一致）。

## 8. 风险与缓解 (Risks & Mitigations)
- **混淆风险**：通过“外部只见 code、内部只见 id”的刚性边界解决。
- **性能风险**：映射表加索引；必要时冗余 `org_code` 到投射表。
- **改码需求**：默认不支持，需单独方案；避免在一期引入复杂性。
- **迁移脏数据**：提供校验清单与失败回滚策略（导入前阻断）。
- **号段耗尽**：当 `next_org_id > 99999999` 时报错并阻断写入；提前监控预警。

## 9. 验收标准 (Acceptance)
- [ ] 使用 `org_code` 能完成组织创建/查询/移动/禁用全流程。
- [ ] UI/公开 API 不出现 `org_id`。
- [ ] 内部事件与投射仍以 `org_id` 作为结构标识。
- [ ] `org_code` 唯一性与解析一致性通过校验。
- [ ] `org_code` 归一化与格式校验按契约拒绝（400/409/404）。
- [ ] 对外响应回显的 `org_code` 一律为大写。
- [ ] CREATE 不再使用全局 `org_id_seq`，`org_id` 按租户隔离分配。
- [ ] 并发分配不产生重复/冲突；号段耗尽返回 `ORG_ID_EXHAUSTED`。

## 9.1 门禁对齐（触发器与验证入口）
- Go：`go fmt ./... && go vet ./... && make check lint && make test`
- 迁移（orgunit）：`make orgunit plan && make orgunit lint && make orgunit migrate up`
- sqlc：`make sqlc-generate`（生成后 `git status --short` 必须为空）
- Routing：`make check routing`
- Authz：`make authz-pack && make authz-test && make authz-lint`
- 文档：`make check doc`

## 10. 实施步骤 (Steps)
1. [x] Schema：新增 `org_unit_codes` 表与 RLS 策略（需用户确认）。
2. [x] Go/投射：在 `CREATE` 投射时写入 `org_code` 映射，拒绝冲突。
3. [x] Resolver：新增 `ResolveOrgID/ResolveOrgCode` 组件，并在边界层统一调用。
4. [x] API/UI：对外仅暴露 `org_code`，禁止 `org_id` 透出；输入需归一化且回显大写。（完成：2026-02-02，已通过 `make test` / `make check doc`）
5. [ ] 写入口唯一性：落地 DB 权限/触发器/代码层禁写措施。
6. [ ] org_id 分配器：新增 `org_id_allocators` 表 + 分配函数；`submit_org_event` 内部分配。
7. [ ] 移除应用层对 `org_id_seq` 的直接依赖并物理删除序列。
8. [ ] 移除旧写入口：删除 `/orgunit/api/*` 相关路由、handler、authz 映射与 allowlist。
9. [ ] hierarchy_type 彻底移除：更新 026/026A 中的 schema、函数签名、索引与锁粒度（单树模型）。
10. [ ] 迁移与校验：回填、归一化、唯一性校验与冲突清单。
11. [ ] 测试：覆盖解析器、唯一性、归一化、并发分配、号段耗尽与边界错误路径。
12. [ ] 文档对齐：同步更新 `DEV-PLAN-026A/026` 中的 org_id 分配说明，避免契约漂移。
13. [ ] 物理清理：全仓库禁止出现 `org_id_seq` 引用（SQL/Go/测试/脚手架）。

### 10.1 实施拆分清单（Work Breakdown）
- **DB/Schema**
  - 新增 `org_unit_codes`、`org_id_allocators` 与 `allocate_org_id(...)`。
  - `submit_org_event`：`CREATE` 时 `p_org_id` 可空，内部调用 `allocate_org_id` 分配。
  - hierarchy_type 清理：移除 `org_trees/org_events/org_unit_versions` 的相关列/约束/索引，并同步更新函数签名。
  - 新增/更新 RLS 策略与权限（`orgunit_kernel` 可写，`app_runtime` 只读）。
  - 处理 `org_id_seq`：删除序列并移除全仓引用（含脚手架/测试）。
- **Go/服务层**
  - 移除所有 `nextval('orgunit.org_id_seq')` 直接取号（`orgunit_nodes`/`orgunit_snapshot` 等）。
  - 新增 `org_code` 解析与归一化入口（resolver），所有外部输入统一走解析。
  - OrgUnit UI/HTMX 与 Internal API 接受/返回 `org_code`（仅内部使用 `org_id`）。
- **DB/函数/查询**
  - 清理 `submit_org_event/replay_org_unit_versions/apply_*` 的 `hierarchy_type` 参数与调用点。
  - 更新查询与锁粒度：`tenant_id` 维度锁；去除所有 `hierarchy_type` 过滤条件。
- **路由/Authz**
  - allowlist 新增 `/org/api/org-units*` 路由。
  - `authz_middleware` 增加 `/org/api/org-units*` 的授权映射。
  - 移除 `/orgunit/api/*` 写入口相关 allowlist/handler/authz。
- **UI/表单**
  - `/org/nodes` 表单字段改为 `org_code/parent_code/new_parent_code`。
  - 列表与详情展示 `org_code`（如需内部 ID，仅限 debug）。
- **测试/文档**
  - 更新 `internal/server/orgunit_nodes_test.go`、`internal/server/orgunit_api_test.go`、`e2e/tests/*`。
  - 更新 `docs/dev-records/DEV-PLAN-010-READINESS.md` 示例与校验口径。

### 10.2 迁移脚本设计（Atlas/Goose）
> 按 `DEV-PLAN-024` 闭环生成 goose 迁移；以下为设计清单（文件名以实际 UTC 时间戳为准）。

**迁移 A：org_code + allocator 基础设施**
- 文件名示例：`migrations/orgunit/YYYYMMDDHHMMSS_orgunit_org_code_mapping_and_allocator.sql`
- Up：
  1) `CREATE TABLE orgunit.org_unit_codes` + RLS。
  2) `CREATE TABLE orgunit.org_id_allocators` + RLS。
  3) `CREATE FUNCTION orgunit.allocate_org_id(...)`。
  4) `GRANT/REVOKE` 权限：`orgunit_kernel` 可写，运行角色只读。
  5) **DROP SEQUENCE orgunit.org_id_seq**，并移除所有引用（含脚手架/测试）。
- Down：
  1) DROP `orgunit.allocate_org_id`。
  2) DROP `orgunit.org_id_allocators` / `orgunit.org_unit_codes`。

**迁移 B：submit_org_event 行为调整**
- 文件名示例：`migrations/orgunit/YYYYMMDDHHMMSS_orgunit_submit_event_allocate_org_id.sql`
- Up：
  1) `CREATE OR REPLACE FUNCTION orgunit.submit_org_event(...)`：  
     - `CREATE` 且 `p_org_id IS NULL` 时调用 `allocate_org_id`；  
     - 其他事件要求 `p_org_id NOT NULL`。
- Down：
  1) 回退为“必须传入 org_id”的版本（如需回滚）。

**迁移 C：存量数据初始化（可选）**
- 仅在已有数据时执行（生产前置作业）：  
  - 为每个租户插入 `org_id_allocators`：`next_org_id = max(org_id)+1`，无数据则 `10000000`。  
  - `org_unit_codes` 的回填通过“迁移清单或事件导入”完成，避免旁路写入。

**迁移 D：移除 hierarchy_type（对齐 026/026A）**
- 文件名示例：`migrations/orgunit/YYYYMMDDHHMMSS_orgunit_remove_hierarchy_type.sql`
- Up：
  1) 移除 `org_trees/org_events/org_unit_versions` 的 `hierarchy_type` 列与相关 CHECK/UNIQUE/INDEX。
  2) 调整 `submit_org_event/replay_org_unit_versions/apply_*` 等函数签名与内部查询条件。
  3) 更新锁粒度为 `tenant_id` 维度（锁 key 不再含 hierarchy_type）。
- Down：
  1) 仅当确有回滚需求时另立方案（避免引入双轨/兼容层）。

> 触发器与门禁以 `AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml` 为准（SSOT）。

## 11. 交付物 (Deliverables)
- 计划文档：`DEV-PLAN-026B`（本文件）
- Schema 迁移与 RLS 更新
- Resolver 组件与边界层改造
- 迁移校验流程与测试用例
