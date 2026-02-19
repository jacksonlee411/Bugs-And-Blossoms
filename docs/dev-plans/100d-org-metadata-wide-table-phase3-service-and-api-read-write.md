# DEV-PLAN-100D：Org 模块宽表元数据落地 Phase 3：服务层与 API（读写可用）

**状态**: 已完成（2026-02-14）

> 本文从 `DEV-PLAN-100` 的 Phase 3 拆分而来，作为 Phase 3 的 SSOT；`DEV-PLAN-100` 保持为整体路线图。
>
> 2026-02-15 补充（承接 `DEV-PLAN-100G`）：为支持列表页 4C 的 UI 收口，本 SSOT 追加冻结：
> - `field-definitions` 返回 `allow_filter/allow_sort`；
> - list 的 ext query 仅限制在 `mode=grid`/分页模式（不再因 `parent_org_code` 而拒绝）；TreePanel 懒加载仍不得携带 ext query 参数。

## 1. 背景与上下文 (Context)

在 `DEV-PLAN-100B`（Phase 1）完成 Schema/元数据骨架、`DEV-PLAN-100C`（Phase 2）完成 Kernel/Projection 扩展后，扩展字段的数据已经具备“可被写入与回放”的基础能力。Phase 3 的目标是把这些能力通过 **服务层与 Internal API** 暴露为可用的读写接口，并在服务端保持：

- allowlist 单点（SSOT：`DEV-PLAN-083` + `DEV-PLAN-100` D7/D8）
- 动态 SQL 的可证明安全（列名/实体来源可枚举，值参数化）
- fail-closed（API 不可用/解析失败/权限不足时不做乐观放行）

本阶段产物会被 `DEV-PLAN-101`（字段配置管理 UI）与 Phase 4（详情页 capabilities 驱动编辑、列表筛选/排序）直接消费。

## 2. 目标与非目标 (Goals & Non-Goals)

- **核心目标**：
  - [X] 在 `internal/server` 实现“元数据解析 + ext 查询 allowlist 守卫 + 详情 ext_fields 解析器”：
    - 输入：`tenant_id + as_of/effective_date` +（可选）ext query params；
    - 输出：enabled field configs（field_key/value_type/data_source_type/data_source_config/physical_col）与安全可证明的查询拼装（列名来源可枚举，值参数化）。
    - 规则 SSOT：写入能力承接 `DEV-PLAN-083`（capabilities 映射），查询 allowlist 承接 `DEV-PLAN-100` D7/D8（通过字段定义 allow flags + physical_col 正则守卫）。
  - [X] 在 `internal/server` 增加/扩展 Internal API：
    - 字段配置管理（list/enable/disable；仅管理端可见）
    - 字段定义列表（可启用字段；供 UI 选择）
    - DICT options（支持 keyword + as_of；PLAIN 无 options；ENTITY 预留，MVP 字段清单未命中）
    - 详情接口返回扩展字段值与展示值
    - mutation capabilities（承接 `DEV-PLAN-083`，含 `deny_reasons`，并包含扩展字段映射）
  - [X] 列表接口支持扩展字段筛选/排序（仅 allowlist 字段；列名来源可证明且值参数化）

- **非目标（本阶段不做）**：
  - 不新增/变更 DB schema（Schema 在 Phase 1/2 解决；本阶段只做 Go/路由/authz/SQL 读取逻辑）。  
  - 不实现 UI（Phase 4；IA 见 `DEV-PLAN-101`）。  
  - 不在本阶段改造 OrgUnit 写接口以支持扩展字段写入（payload builder/DICT label snapshot 写入随 Phase 4 编辑态闭环落地）。  
  - 不引入第二写入口：OrgUnit 业务写入仍只走 `orgunit.submit_org_event(...)` 体系（One Door）。  

## 2.1 工具链与门禁（SSOT 引用）

> 目的：只声明命中哪些触发器/门禁入口，不在本文复制脚本细节（SSOT：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`）。

- **触发器清单（勾选本计划命中的项）**：
  - [X] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [X] 路由治理（新增 Internal API 路由：`make check routing`）
  - [X] Authz（新增/调整路由权限：`make authz-pack && make authz-test && make authz-lint`）
  - [X] 文档（`make check doc`）
  - [ ] sqlc（若引入/调整 sqlc queries 或导出 schema：`make sqlc-generate`，并确保 `git status --short` 为空）

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - CI 门禁定义：`docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)

```mermaid
graph TD
  UI[Web UI (Phase 4)] --> API[Internal API: /org/api/org-units/*]
  API --> SVC[Service: metadata/capabilities resolver + patch builder]
  SVC --> STORE[Store/Repo]
  STORE --> META[(tenant_field_configs)]
  STORE --> VERS[(org_unit_versions.ext_* + ext_labels_snapshot)]
  SVC --> KERNEL[DB Kernel: submit_org_event/submit_*_correction]
```

### 3.2 关键设计决策 (ADR 摘要)

- **ADR-100D-01：扩展字段 payload 统一命名空间**
  - 选定：扩展字段值写入 `payload.ext`，DICT label 快照写入 `payload.ext_labels_snapshot`（SSOT：`DEV-PLAN-100A/100C`）。

- **ADR-100D-02：allowlist/能力解析器单点化**
  - 选定：mutation 能力（写入字段白名单 + payload key 映射 + deny_reasons）以 `DEV-PLAN-083` 为 SSOT；扩展字段 query 能力（filter/sort/options）以 `DEV-PLAN-100` D7/D8 为 SSOT。  
  - 约束：UI/API/SQL 不得各自维护第二套白名单或字段映射。

- **ADR-100D-03：动态 SQL 仅允许“枚举化实体 + allowlist 列名 + 参数化值”**
  - 选定：所有动态列名/实体名必须来自服务端枚举映射（field_key -> physical_col；entity_code -> 固定 SQL 模板），值一律参数化；任何解析失败直接拒绝（fail-closed）。

- **ADR-100D-04：字段定义与数据源的权威边界（避免第二套 SSOT）**
  - 选定：`field-definitions` 是**内置字段**的权威来源（metadata SSOT）：
    - `value_type/data_source_type/label_i18n_key/allow_filter/allow_sort` 由 `field-definitions` 冻结；
    - **DICT**：`dict_code` 的“可选全集/可用性”以字典模块 registry 为 SSOT（`DEV-PLAN-105/105B`），`field-definitions` 不再枚举可选 dict_code（避免把 dict_code 复制进 Org 形成第二套 SSOT）；
    - **ENTITY**：`data_source_config` 仍要求命中 `field-definitions.data_source_config_options`（枚举化候选；禁止任意表/列透传）。
  - 约束：
    - `field-configs` 的 enable 请求禁止客户端提交或覆盖 `physical_col`；
    - 允许一个受控例外：自定义 PLAIN 字段走 `x_` 命名空间（见 §5.2.2），其 metadata 由 enable 行为隐式承载（`data_source_type=PLAIN` 固定；`value_type` 扩展由 `DEV-PLAN-110` 冻结）。
  - 原因：让“字段元数据/数据源/可用性”各自只有一种权威表达，避免 drift（对齐 `DEV-PLAN-003` Simple > Easy）。

- **ADR-100D-05：DICT 展示值遵循 D4 兜底链路，但必须显式标记来源**
  - 选定：DICT 的 `display_value` 读取遵循 `DEV-PLAN-100` D4：`versions 快照 -> events 快照 -> 当前字典 label`；但必须通过 `display_value_source`（与可选 warning code）显式标记兜底路径，禁止静默“当前名称覆盖历史”。

- **ADR-100D-06：options 的 label 为“规范化展示名”（不做 i18n）**
  - 选定：`fields:options` 返回的 `label` 与写入 `payload.ext_labels_snapshot` 的内容均为**非本地化**的规范化展示名（canonical label），不随 UI 语言变化；字段标题的 i18n 仍通过 `label_i18n_key`（`DEV-PLAN-020`）。  
  - 说明：如未来需要“业务枚举值多语言 label”，必须另立 dev-plan（不在 Phase 3 扩展，避免把业务数据 i18n 混入现有契约）。

- **ADR-100D-07：`request_code` 幂等语义与 HTTP 状态码冻结**
  - 选定：`request_code` 是同租户内的幂等键（SSOT：`DEV-PLAN-100A/100B`）。enable 的首次创建返回 201；同 `request_code` 的重试（输入完全一致）返回 200 且回放同一行（含同一 `physical_col`）；输入不一致返回 409 `ORG_REQUEST_ID_CONFLICT`。

## 4. 数据模型与约束 (Data Model & Constraints)

本阶段不新增 schema；依赖：

- `orgunit.tenant_field_configs`（Phase 1）
- `orgunit.org_unit_versions` 的 `ext_*` 槽位列 + `ext_labels_snapshot`（Phase 1）
- Kernel 对 `payload.ext` 的校验与投射/回放一致性（Phase 2）

服务层必须把“运行态可见字段集合”解释为：

- `as_of` 下 enabled 的字段配置集合（day 粒度半开区间口径见 `DEV-PLAN-100C` §4.2）
- 且对不同 API/权限视图做裁剪（例如 admin-only API 才能看到 `physical_col`）

## 5. 接口契约 (API Contracts)

> 说明：以下均为 Internal API（`routing.RouteClassInternalAPI`）。路径命名、HTTP method 与 query param 变更需通过 `make check routing`。

### 5.1 字段定义列表（可启用字段）

- `GET /org/api/org-units/field-definitions`
- **Authz**：`orgunit.admin`（仅管理端可见；对齐 `DEV-PLAN-101`）
- **Response 200**：

```json
{
  "fields": [
    {
      "field_key": "short_name",
      "value_type": "text",
      "data_source_type": "PLAIN",
      "data_source_config": {},
      "label_i18n_key": "org.fields.short_name",
      "allow_filter": false,
      "allow_sort": false
    },
    {
      "field_key": "org_type",
      "value_type": "text",
      "data_source_type": "DICT",
      "data_source_config": { "dict_code": "org_type" },
      "label_i18n_key": "org.fields.org_type",
      "allow_filter": true,
      "allow_sort": true
    }
  ]
}
```

> 字段集合与 `field_key` 命名冻结：SSOT 为 `DEV-PLAN-100A` 的 Phase 0 字段清单（评审冻结后再实现）。
>
> `data_source_config_options` 口径（冻结）：
>
> - 仅当 `data_source_type='ENTITY'` 时返回，且必须为非空数组（枚举化候选，禁止任意透传）。
> - `data_source_type='DICT'` **不返回** `data_source_config_options`：DICT 的 `dict_code` 选择来源为字典模块 dict list（SSOT：`DEV-PLAN-105B`）。

新增字段（冻结）：

- `allow_filter` / `allow_sort`：
  - 事实源为服务端 fieldmeta（SSOT），用于驱动列表页展示“可筛选/可排序”的扩展字段入口；
  - UI 不得维护第二套 allowlist（对齐 `DEV-PLAN-100` D7/D8）。

### 5.2 字段配置管理（list/enable/disable）

#### 5.2.1 List

- `GET /org/api/org-units/field-configs?as_of=YYYY-MM-DD&status=all|enabled|disabled`
- **Authz**：`orgunit.admin`
- **Response 200**：

```json
{
  "as_of": "2026-02-13",
  "field_configs": [
    {
      "field_key": "d_org_type",
      "value_type": "text",
      "data_source_type": "DICT",
      "data_source_config": { "dict_code": "org_type" },
      "label_i18n_key": null,
      "label": "组织类型（示例）",
      "allow_filter": true,
      "allow_sort": true,
      "physical_col": "ext_str_01",
      "enabled_on": "2026-02-01",
      "disabled_on": null,
      "updated_at": "2026-02-10T12:00:00Z"
    }
  ]
}
```

status 口径冻结：

- `enabled`：`enabled_on <= as_of` 且（`disabled_on IS NULL OR as_of < disabled_on`）（半开区间；对齐 `DEV-PLAN-100C` §4.2）
- `disabled`：不满足 `enabled` 条件但行存在（包括“未来生效（as_of < enabled_on）”与“已停用（disabled_on <= as_of）”）
- `all`：返回全部行

#### 5.2.2 Enable Candidates（启用候选；106A 新增）

- `GET /org/api/org-units/field-configs:enable-candidates?enabled_on=YYYY-MM-DD`
- **Authz**：`orgunit.admin`
- **Response 200（草案；冻结为“最小够用”）**：

```json
{
  "enabled_on": "2026-02-01",
  "dict_fields": [
    {
      "field_key": "d_org_type",
      "dict_code": "org_type",
      "name": "Org Type",
      "value_type": "text",
      "data_source_type": "DICT"
    }
  ],
  "plain_custom_hint": {
    "pattern": "^x_[a-z0-9_]{1,60}$",
    "value_types": ["text", "int", "uuid", "bool", "date", "numeric"],
    "default_value_type": "text"
  }
}
```

约束（冻结）：

- `dict_fields` 的事实源为字典模块 dict registry：`GET /iam/api/dicts?as_of=enabled_on`（SSOT：`DEV-PLAN-105B`；对齐 `DEV-PLAN-106A`）。
- 仅返回可推导为 `d_<dict_code>` 且满足 `tenant_field_configs.field_key` DB check 的候选；不可推导项必须排除并输出可排障日志（对齐 `DEV-PLAN-106A`）。

#### 5.2.3 Enable（启用字段）

- `POST /org/api/org-units/field-configs`
- **Authz**：`orgunit.admin`
- **Request**：

```json
{
  "field_key": "d_org_type",
  "enabled_on": "2026-02-01",
  "request_code": "req-uuid-or-stable-string",
  "label": "组织类型（示例）"
}
```

- **Response 201**：返回新配置行（含 `physical_col`）
- **Response 200（幂等重试）**：同 `request_code` 且输入一致时返回已存在行（禁止重新分配 `physical_col`）

约束（冻结）：

- `initiator_uuid` 必须由服务端会话上下文注入并传递给 Kernel；**不得由 UI 提交/伪造**（SSOT：`DEV-PLAN-100A`）。
- `field_key` 分类（冻结）：
  - **内置字段**：必须来自 `field-definitions`（ADR-100D-04）；若 `field_key` 不在定义列表，返回 404 `ORG_FIELD_DEFINITION_NOT_FOUND`。
  - **字典字段（DICT）**：当 `field_key` 为 `d_<dict_code>` 时，视为“字典字段”，允许 **不在** `field-definitions` 中；该路径下由服务端强制：
    - `value_type='text'`、`data_source_type='DICT'`；
    - `data_source_config={"dict_code":"<dict_code>"}`，且 `<dict_code>` 必须在字典模块 registry 中存在并在 `enabled_on` 下可用（fail-closed；SSOT：`DEV-PLAN-105B`；收敛目标见 `DEV-PLAN-106A`）。
  - **自定义 PLAIN 字段**：当 `field_key` 满足 `x_[a-z0-9_]{1,60}` 时，允许 **不在** `field-definitions` 中；该路径下 `data_source_type='PLAIN'`（固定），`value_type` 由请求显式给定（`text/int/uuid/bool/date/numeric`），且 `data_source_config` 必须为 `{}`（缺失由服务端补齐为 `{}`）。
- `data_source_config`：
  - `PLAIN`：必须为 `{}`（可缺省，由服务端补齐为空对象）。  
  - `DICT（字典字段）`：服务端从 `field_key=d_<dict_code>` 推导 `dict_code`，并按 dict registry 校验（fail-closed）；客户端若显式提交 `data_source_config`，也必须与推导结果一致（不一致即拒绝），避免“双写同一事实”漂移。
  - `ENTITY`：必须命中 `field-definitions.data_source_config_options`（枚举化候选；禁止任意透传）。
- `label`（display label）：
  - 仅当 `field_key` 为 `d_...` 时允许提交；用于字段配置列表与详情页字段标题展示，不参与 DICT 校验逻辑（SSOT：`DEV-PLAN-106A`）。

#### 5.2.4 Disable（停用字段）

- `POST /org/api/org-units/field-configs:disable`
- **Authz**：`orgunit.admin`
- **Request**：

```json
{
  "field_key": "org_type",
  "disabled_on": "2026-03-01",
  "request_code": "req-uuid-or-stable-string"
}
```

- **Response 200**：返回更新后的配置行
- **Response 200（幂等重试）**：同 `request_code` 且输入一致时返回更新后的配置行

> 字段配置写入口必须调用 Phase 1 的 Kernel 函数；应用角色不得直写 `tenant_field_configs`（SSOT：`DEV-PLAN-100B`）。

### 5.3 Options（DICT/ENTITY；PLAIN 无 options）

- `GET /org/api/org-units/fields:options?field_key=<...>&as_of=YYYY-MM-DD&q=<keyword>&limit=<n>`
- **Authz**：`orgunit.read`（或等价 read 权限；具体权限点按 `DEV-PLAN-022` 冻结）
- **Response 200**：

```json
{
  "field_key": "d_org_type",
  "as_of": "2026-02-13",
  "options": [
    { "value": "DEPARTMENT", "label": "Department" }
  ]
}
```

约束：

- `field_key` 必须在 `as_of` 下 enabled；否则返回 404（fail-closed；错误码见 §5.7）。
- 若该字段 `data_source_type=PLAIN`：options 不适用，必须 fail-closed（推荐返回 404，避免 UI/调用方误用）。
- `ENTITY` 的目标实体必须为枚举映射（禁止透传任意表名/列名；SSOT：`DEV-PLAN-100` D7）。
- `label` 为 **canonical label**（ADR-100D-06）：不随 UI locale 变化，且用于生成 `payload.ext_labels_snapshot`（DICT）。
- `q`（keyword）可选：服务端对输入做 trim；为空时返回“前 N 个”（按稳定排序）。
- `limit` 可选：缺失或非法时默认 `10`；最大 `50`（超出按 `50` 处理），避免 options 无界返回。
- `options` 返回顺序必须稳定：默认按 `label` 升序，其次按 `value` 升序（便于缓存与测试）。

### 5.4 详情接口：返回扩展字段值 + 展示值

在现有：

- `GET /org/api/org-units/details?org_code=<...>&as_of=YYYY-MM-DD&include_disabled=...`

基础字段保持不变，在响应中新增 `ext_fields`（数组），用于 UI 动态渲染：

```json
{
  "as_of": "2026-02-13",
  "org_unit": {
    "org_id": 10000001,
    "org_code": "R&D",
    "name": "R&D"
  },
  "ext_fields": [
    {
      "field_key": "d_org_type",
      "label_i18n_key": null,
      "label": "组织类型（示例）",
      "value_type": "text",
      "data_source_type": "DICT",
      "value": "DEPARTMENT",
      "display_value": "Department",
      "display_value_source": "versions_snapshot"
    },
    {
      "field_key": "x_cost_center",
      "label_i18n_key": null,
      "label": "x_cost_center",
      "value_type": "text",
      "data_source_type": "PLAIN",
      "value": "CC-001",
      "display_value": "CC-001",
      "display_value_source": "plain"
    }
  ]
}
```

约束：

- `ext_fields` 必须包含 `as_of` 下 enabled 的字段全集（day 粒度）；即使当前无值也必须返回（`value=null`），避免 UI 出现“字段已启用但不可见/不可编辑”。  
- 当 `as_of >= disabled_on` 时该字段不属于 enabled 集合，因此 **不得** 出现在 `ext_fields`；用户若需查看历史值，应切换 `as_of` 到有效期内或查看 Audit（变更日志）。  
- `ext_fields` 的返回顺序必须稳定：按 `field_key` 升序排序（避免 UI 抖动与测试不稳定）。  
- label（冻结）：
  - 内置字段：`label_i18n_key` 必须稳定（i18n SSOT：`DEV-PLAN-020`），用于 UI 动态渲染；
  - 自定义字段（`x_` 命名空间）：允许 `label_i18n_key=null`，但必须提供 `label`（canonical string；UI 不做 i18n）。
  - 字典字段（`d_` 命名空间）：允许 `label_i18n_key=null`，且必须提供 `label`（优先启用时 `label`，否则使用 dict name 或 fallback 到 dict_code；SSOT：`DEV-PLAN-106A`）。
  - 字段 key 到 label 的映射不得由 UI 另建第二套规则（UI 仅消费服务端返回的 `label_i18n_key/label`）。
- `display_value` 与 `display_value_source`（冻结）：
  - PLAIN：`display_value` 为 `value` 的规范化字符串表示；`display_value_source="plain"`。  
  - DICT：遵循 `DEV-PLAN-100` D4（ADR-100D-05）：
    - 优先：`versions.ext_labels_snapshot[field_key]` → `display_value_source="versions_snapshot"`；
    - 兜底 1：`events.payload.ext_labels_snapshot[field_key]` → `display_value_source="events_snapshot"`；
    - 兜底 2：按 `value(code)` 查当前字典 label → `display_value_source="dict_fallback"`（必须显式标记，禁止静默覆盖历史）。  
  - ENTITY：按 `as_of` join 枚举化实体模板获取 → `display_value_source="entity_join"`（严格 as_of 语义，禁止“当前名称覆盖历史”）。  
  - 无法解析时 fail-closed：`display_value=null` 且 `display_value_source="unresolved"`（不得返回误导性值）。

### 5.5 mutation capabilities（承接 DEV-PLAN-083，包含扩展字段）

- `GET /org/api/org-units/mutation-capabilities?org_code=<...>&effective_date=YYYY-MM-DD`
- **Authz**：`orgunit.read`（或等价 read 权限；具体权限点按 `DEV-PLAN-022` 冻结）

返回结构以 `DEV-PLAN-083` 为 SSOT，并补齐扩展字段要求：

- `allowed_fields` 包含扩展字段的 `field_key`（在允许写入的动作里）。  
- `field_payload_keys[field_key]` 对扩展字段统一返回：`ext.<field_key>`。  
- `deny_reasons` 必须可解释且稳定；API 不可用/解析失败时，UI 必须 fail-closed（只读/禁用）。  
- 对扩展字段：`allowed_fields` 只能包含在该 `effective_date` 下 enabled 的字段（避免 UI 出现“可编辑但必失败”）。

### 5.6 列表接口：扩展字段筛选/排序

在现有：

- `GET /org/api/org-units?as_of=YYYY-MM-DD&include_disabled=...`

新增（或冻结）扩展字段相关 query params（MVP 先支持单条件，避免一次性做“动态查询 DSL”）：

- `ext_filter_field_key=<field_key>`（可选）
- `ext_filter_value=<string>`（可选；服务端按 `value_type` 解析与校验）
- `sort=ext:<field_key>`（可选；沿用现有 `sort`/`order` 口径）

约束：

- **适用模式冻结**：扩展字段 filter/sort 仅支持 “grid/list 查询”模式（例如 `mode=grid` 或显式分页参数）；对 roots/children 兼容路径（未进入 list 模式）必须返回 400（避免把动态 SQL 带入树懒加载链路）。  
- `ext_filter_field_key` 与 `ext_filter_value` 必须成对出现；仅出现其一返回 400。  
- 仅允许 filter/sort allowlist 字段（SSOT：`DEV-PLAN-100` D7/D8），且字段必须在 `as_of` 下 enabled；否则返回 400（fail-closed）。  
- 列名只能来自 `field_key -> physical_col` 映射，且必须通过严格格式校验；值必须参数化。  

解析规则（冻结）：

- `value_type=text`：按原样（trim）作为 string；
- `value_type=int`：十进制整数；
- `value_type=uuid`：标准 UUID 字符串；
- `value_type=bool`：仅接受 `true|false|1|0`（大小写不敏感）；
- `value_type=date`：`YYYY-MM-DD`。

### 5.7 稳定错误码与 HTTP 映射（Phase 3 冻结）

> 说明：本节冻结“错误码（`code`）→ HTTP status”的对外契约；实现时应在 `orgUnitAPIStatusForCode(...)` 或对应 handler 中确保一致，避免 UI/测试漂移。

| 场景 | code（稳定） | HTTP | 备注 |
| --- | --- | --- | --- |
| enable：field_key 不在 field-definitions（且不符合自定义 `x_` 规则） | `ORG_FIELD_DEFINITION_NOT_FOUND` | 404 | 管理端可见；用于防止 UI 启用未知字段 |
| enable：自定义 field_key 规则不满足（非 `x_...` / 超长 / 非法字符） | `ORG_INVALID_ARGUMENT` / `invalid_request` | 400 | 仅允许 `x_[a-z0-9_]{1,60}`（且满足 field_key 格式 check） |
| enable：data_source_config 不合法（缺失/形状非法/DICT dict_code 不可用/ENTITY 不命中 options） | `ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG` | 400 | SSOT：`DEV-PLAN-100B`；DICT 的 dict_code 可用性以字典模块 registry 为准（`enabled_on` 视图，fail-closed） |
| enable/disable：request_code 幂等键冲突 | `ORG_REQUEST_ID_CONFLICT` | 409 | SSOT：`DEV-PLAN-100B` 错误码表 |
| enable：field_key 已启用 | `ORG_FIELD_CONFIG_ALREADY_ENABLED` | 409 | SSOT：`DEV-PLAN-100B` |
| enable：槽位耗尽 | `ORG_FIELD_CONFIG_SLOT_EXHAUSTED` | 409 | SSOT：`DEV-PLAN-100B` |
| disable：配置不存在 | `ORG_FIELD_CONFIG_NOT_FOUND` | 404 | SSOT：`DEV-PLAN-100B` |
| disable：disabled_on 规则不满足 | `ORG_FIELD_CONFIG_DISABLED_ON_INVALID` | 409 | SSOT：`DEV-PLAN-100B` |
| 参数缺失/格式非法（日期/必填） | `ORG_INVALID_ARGUMENT` / `invalid_request` | 400 | `ORG_INVALID_ARGUMENT` 为 Kernel 稳定码；handler 可用 `invalid_request` |
| options：字段未启用（as_of 不在 enabled 区间） | `ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF` | 404 | fail-closed；避免 UI 误用 |
| options：PLAIN 字段请求 options | `ORG_FIELD_OPTIONS_NOT_SUPPORTED` | 404 | fail-closed（推荐 404） |
| list：ext filter/sort 不允许（非 allowlist 或未 enabled） | `ORG_EXT_QUERY_FIELD_NOT_ALLOWED` | 400 | 仅用于调试；UI 不应构造该请求 |
| list：ext_filter 参数不成对/格式非法 | `invalid_request` | 400 | |

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 元数据解析（field_key -> payload_key/physical_col）

输入：`tenant_id + as_of`  
输出：

- `enabledFieldConfigs[]`：`field_key/value_type/data_source_type/data_source_config/physical_col`  
- `payloadKey(field_key)`：统一为 `ext.<field_key>`（扩展字段）  
- `physicalCol(field_key)`：来自 `tenant_field_configs`（同租户）  

失败策略：任何缺失/冲突（重复 field_key、physical_col 不合法、配置不在 as_of 生效）一律 fail-closed。

### 6.2 写入 payload 构造（Create/Correct patch）

1. 将“核心字段 patch”与“扩展字段 patch”分离（核心字段规则以 `DEV-PLAN-083` 为 SSOT）。  
2. 对扩展字段：
   - 校验字段在 `effective_date` enabled；否则拒绝。  
   - 依据 `value_type` 做类型解析（uuid/int/bool/date/text），失败拒绝。  
   - DICT：通过 options resolver 反查 **canonical label**，写入 `payload.ext_labels_snapshot[field_key]=label`（禁止信任 UI 传入 label；ADR-100D-06）。  
3. 组装最终 payload：
   - `payload.ext[field_key]=value`
   - `payload.ext_labels_snapshot[field_key]=label`（仅 DICT）  

### 6.3 Options resolver（DICT/ENTITY；PLAIN 不支持）

- DICT：
  - 以 `dict_code` 为枚举键；
  - options/label 来源以字典模块为 SSOT（`DEV-PLAN-105/105B`），业务模块通过 `pkg/dict` 门面访问；禁止静默降级到代码内静态 registry（对齐 No Legacy）。
- ENTITY：
  - 以 `entity_code` 为枚举键，映射到固定 SQL 模板；
  - `as_of` 必须进入查询条件（严格 as_of 语义，禁止“当前名称覆盖历史”）。

### 6.4 列表筛选/排序（动态 SQL 安全拼装）

- 从 query param 得到 `field_key`，通过元数据解析得到 `physical_col`；
- 校验 `physical_col` 符合严格正则（只允许 `ext_(str|int|uuid|bool|date)_\\d{2}`）；
- 仅把 `physical_col` 作为 SQL identifier 拼入固定模板，且必须先做 identifier quoting（例如 `pgx.Identifier{physical_col}.Sanitize()` 或在 DB 侧 `quote_ident/format('%I', ...)`）；所有值使用占位符参数化；
- 任何解析失败直接拒绝（fail-closed），并打审计日志：tenant_uuid/field_key/physical_col/query_mode。

## 7. 安全与鉴权 (Security & Authz)

- **Authz**：
  - 字段配置管理（field-definitions/field-configs/enable/disable）：必须 `orgunit.admin`（对齐 `DEV-PLAN-101`/`DEV-PLAN-022`）。  
  - 详情/列表/options/mutation-capabilities：至少要求 `orgunit.read`（具体权限点在实现时冻结，禁止“默认放行”）。  
- **RLS/租户隔离**：
  - 所有访问 `tenant_field_configs` / 读模型必须在显式事务内注入 `app.current_tenant`（SSOT：`AGENTS.md` “No Tx, No RLS”）。  
- **SQL 注入防护**：
  - 禁止拼接用户输入列名/表名；
  - 列名仅来自 allowlist + 元数据映射；
  - ENTITY join 目标实体仅来自枚举映射；禁止透传任意表名/列名（SSOT：`DEV-PLAN-100` D7）。  

## 8. 依赖与里程碑 (Dependencies & Milestones)

- **依赖**：
  - `DEV-PLAN-100A`（Phase 0：字段清单/契约冻结）
  - `DEV-PLAN-100B`（Phase 1：tenant_field_configs + ext_* 槽位列）
  - `DEV-PLAN-100C`（Phase 2：Kernel 校验/投射/回放一致性）
  - `DEV-PLAN-083`（mutation capabilities 策略矩阵 SSOT）
  - `DEV-PLAN-101`（字段配置 UI IA；决定需要哪些 API/交互口径）
  - `DEV-PLAN-017`（路由策略与门禁）
  - `DEV-PLAN-022`（Authz）

- **里程碑（Phase 3 待办）**：
  1. [X] 服务层：实现元数据解析 + ext 查询 allowlist 守卫 + details ext_fields 解析（含 DICT display_value_source）。  
  2. [X] API：实现 field-definitions/field-configs（list/enable/disable）。  
  3. [X] API：实现 fields:options（DICT；PLAIN 必拒绝；ENTITY 预留）。  
  4. [X] API：扩展 details 返回 ext_fields。  
  5. [X] API：实现 mutation-capabilities，并包含扩展字段映射（`ext.<field_key>`）。  
  6. [X] 列表：支持 ext 字段 filter/sort（allowlist + 参数化 + 可审计）。  
  7. [X] 门禁：`make check routing` + `make authz-pack && make authz-test && make authz-lint` + Go 测试门禁通过。  

## 9. 测试与验收标准 (Acceptance Criteria)

- **API 契约测试**（至少覆盖）：
  - [X] field-configs：启用/停用/列表（含权限拒绝、槽位耗尽/冲突错误映射）。  
  - [X] field-configs：幂等重试（同 `request_code` 同输入返回 200，且 `physical_col` 不变化；不同输入返回 409 `ORG_REQUEST_ID_CONFLICT`）。  
  - [X] options：DICT（含未启用字段 fail-closed；PLAIN 字段必须拒绝；ENTITY 预留且 fail-closed）。  
  - [X] options：未启用字段返回 404 `ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF`；PLAIN 返回 404 `ORG_FIELD_OPTIONS_NOT_SUPPORTED`。  
  - [X] details：返回 ext_fields（全集 + 稳定排序；DICT display_value_source 覆盖 snapshot/fallback/unresolved）。  
  - [X] mutation-capabilities：扩展字段进入 `allowed_fields/field_payload_keys`，且 `deny_reasons` 可解释。  
  - [X] list：ext filter/sort 在 allowlist 内可用；非 allowlist/未 enabled 返回 400 `ORG_EXT_QUERY_FIELD_NOT_ALLOWED`；非 grid 模式请求 ext filter/sort 返回 400。  

- **安全验收**：
  - [X] 无 SQL 注入风险（列名与实体来源可证明为 allowlist/枚举；值参数化）。  

- **出口条件（与路线图一致）**：
  - [X] API 契约测试覆盖新增字段。  
  - [X] 无 SQL 注入风险（列名与实体来源可证明）。  

## 10. 运维与监控 (Ops & Monitoring)

本阶段不引入运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。
