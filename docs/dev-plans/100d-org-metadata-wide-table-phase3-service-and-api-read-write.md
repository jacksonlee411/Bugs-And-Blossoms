# DEV-PLAN-100D：Org 模块宽表元数据落地 Phase 3：服务层与 API（读写可用）

**状态**: 草拟中（2026-02-13 07:37 UTC）

> 本文从 `DEV-PLAN-100` 的 Phase 3 拆分而来，作为 Phase 3 的 SSOT；`DEV-PLAN-100` 保持为整体路线图。

## 1. 背景与上下文 (Context)

在 `DEV-PLAN-100B`（Phase 1）完成 Schema/元数据骨架、`DEV-PLAN-100C`（Phase 2）完成 Kernel/Projection 扩展后，扩展字段的数据已经具备“可被写入与回放”的基础能力。Phase 3 的目标是把这些能力通过 **服务层与 Internal API** 暴露为可用的读写接口，并在服务端保持：

- allowlist 单点（SSOT：`DEV-PLAN-083` + `DEV-PLAN-100` D7/D8）
- 动态 SQL 的可证明安全（列名/实体来源可枚举，值参数化）
- fail-closed（API 不可用/解析失败/权限不足时不做乐观放行）

本阶段产物会被 `DEV-PLAN-101`（字段配置管理 UI）与 Phase 4（详情页 capabilities 驱动编辑、列表筛选/排序）直接消费。

## 2. 目标与非目标 (Goals & Non-Goals)

- **核心目标**：
  - [ ] 在 `modules/orgunit/services` 增加“元数据解析 + patch 构造器/能力解析器”：
    - 输入：业务字段 + mutation 上下文（action/event/target） + as_of/effective_date；
    - 输出：`payload`（含 `payload.ext`）与 `field_key -> payload_key/physical_col` 映射；
    - 规则 SSOT：写入能力承接 `DEV-PLAN-083`，查询 allowlist 承接 `DEV-PLAN-100` D7/D8。
  - [ ] 在 `internal/server/orgunit_api.go` 增加/扩展 Internal API：
    - 字段配置管理（list/enable/disable；仅管理端可见）
    - 字段定义列表（可启用字段；供 UI 选择）
    - DICT/ENTITY options（支持 keyword + as_of）
    - 详情接口返回扩展字段值与展示值
    - mutation capabilities（承接 `DEV-PLAN-083`，含 `deny_reasons`，并包含扩展字段映射）
  - [ ] 列表接口支持扩展字段筛选/排序（仅 allowlist 字段；列名来源可证明且值参数化）

- **非目标（本阶段不做）**：
  - 不新增/变更 DB schema（Schema 在 Phase 1/2 解决；本阶段只做 Go/路由/authz/SQL 读取逻辑）。  
  - 不实现 UI（Phase 4；IA 见 `DEV-PLAN-101`）。  
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
      "field_key": "org_type",
      "value_type": "text",
      "data_source_type": "DICT",
      "data_source_config": { "dict_code": "org_type" },
      "label_i18n_key": "org.fields.org_type"
    }
  ]
}
```

> 字段集合与 `field_key` 命名冻结：SSOT 为 `DEV-PLAN-100A` 的 Phase 0 字段清单（评审冻结后再实现）。

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
      "field_key": "org_type",
      "value_type": "text",
      "data_source_type": "DICT",
      "data_source_config": { "dict_code": "org_type" },
      "physical_col": "ext_str_01",
      "enabled_on": "2026-02-01",
      "disabled_on": null,
      "updated_at": "2026-02-10T12:00:00Z"
    }
  ]
}
```

#### 5.2.2 Enable（启用字段）

- `POST /org/api/org-units/field-configs`
- **Authz**：`orgunit.admin`
- **Request**：

```json
{
  "field_key": "org_type",
  "enabled_on": "2026-02-01",
  "data_source_config": { "dict_code": "org_type" },
  "request_code": "req-uuid-or-stable-string"
}
```

- **Response 201**：返回新配置行（含 `physical_col`）
- **Error**（示例）：
  - 409：`field_key already enabled`
  - 409：`physical slots exhausted`
  - 403：无权限

#### 5.2.3 Disable（停用字段）

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

> 字段配置写入口必须调用 Phase 1 的 Kernel 函数；应用角色不得直写 `tenant_field_configs`（SSOT：`DEV-PLAN-100B`）。

### 5.3 Options（DICT/ENTITY 双通道）

- `GET /org/api/org-units/fields:options?field_key=<...>&as_of=YYYY-MM-DD&q=<keyword>&limit=<n>`
- **Authz**：`orgunit.read`（或等价 read 权限；具体权限点按 `DEV-PLAN-022` 冻结）
- **Response 200**：

```json
{
  "field_key": "org_type",
  "as_of": "2026-02-13",
  "options": [
    { "value": "DEPARTMENT", "label": "Department" }
  ]
}
```

约束：

- `field_key` 必须在 `as_of` 下 enabled；否则返回 404/403（fail-closed，具体口径冻结后实现）。
- `ENTITY` 的目标实体必须为枚举映射（禁止透传任意表名/列名；SSOT：`DEV-PLAN-100` D7）。

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
      "field_key": "org_type",
      "value_type": "text",
      "data_source_type": "DICT",
      "value": "DEPARTMENT",
      "display_value": "Department"
    }
  ]
}
```

约束：

- `ext_fields` 仅包含 `as_of` 下 enabled 的字段（day 粒度）。  
- `display_value`：
  - DICT：优先来自 `versions.ext_labels_snapshot[field_key]`；
  - ENTITY：通过枚举化实体 join（as_of）获取；
  - 无法解析时 fail-closed：返回错误或返回空并附带可排障信息（口径在实现时冻结，禁止静默“当前名称覆盖历史”）。  

### 5.5 mutation capabilities（承接 DEV-PLAN-083，包含扩展字段）

- `GET /org/api/org-units/mutation-capabilities?org_code=<...>&effective_date=YYYY-MM-DD`
- **Authz**：`orgunit.read`（或等价 read 权限；具体权限点按 `DEV-PLAN-022` 冻结）

返回结构以 `DEV-PLAN-083` 为 SSOT，并补齐扩展字段要求：

- `allowed_fields` 包含扩展字段的 `field_key`（在允许写入的动作里）。  
- `field_payload_keys[field_key]` 对扩展字段统一返回：`ext.<field_key>`。  
- `deny_reasons` 必须可解释且稳定；API 不可用/解析失败时，UI 必须 fail-closed（只读/禁用）。  

### 5.6 列表接口：扩展字段筛选/排序

在现有：

- `GET /org/api/org-units?as_of=YYYY-MM-DD&include_disabled=...`

新增（或冻结）扩展字段相关 query params（MVP 先支持单条件，避免一次性做“动态查询 DSL”）：

- `ext_filter_field_key=<field_key>`（可选）
- `ext_filter_value=<string>`（可选；服务端按 `value_type` 解析与校验）
- `sort=ext:<field_key>`（可选；沿用现有 `sort`/`order` 口径）

约束：

- 仅允许 filter/sort allowlist 字段（SSOT：`DEV-PLAN-100` D7/D8）；否则返回 400/403（fail-closed）。  
- 列名只能来自 `field_key -> physical_col` 映射，且必须通过严格格式校验；值必须参数化。  

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
   - DICT：通过 options resolver 反查 `label`，写入 `payload.ext_labels_snapshot[field_key]=label`（禁止信任 UI 传入 label）。  
3. 组装最终 payload：
   - `payload.ext[field_key]=value`
   - `payload.ext_labels_snapshot[field_key]=label`（仅 DICT）  

### 6.3 Options resolver（DICT/ENTITY）

- DICT：
  - 以 `dict_code` 为枚举键；
  - options 来源必须可审计（MVP 可先用代码内 registry；后续如落库需另立 dev-plan）。  
- ENTITY：
  - 以 `entity_code` 为枚举键，映射到固定 SQL 模板；
  - `as_of` 必须进入查询条件（严格 as_of 语义，禁止“当前名称覆盖历史”）。

### 6.4 列表筛选/排序（动态 SQL 安全拼装）

- 从 query param 得到 `field_key`，通过元数据解析得到 `physical_col`；
- 校验 `physical_col` 符合严格正则（只允许 `ext_(str|int|uuid|bool|date)_\\d{2}`）；
- 仅把 `physical_col` 作为 SQL identifier（`%I`/quote_ident）拼入固定模板；所有值使用占位符参数化；
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
  1. [ ] 服务层：实现元数据解析 + payload/patch 构造器（扩展字段映射与 DICT label snapshot 生成）。  
  2. [ ] API：实现 field-definitions/field-configs（list/enable/disable）。  
  3. [ ] API：实现 fields:options（DICT/ENTITY）。  
  4. [ ] API：扩展 details 返回 ext_fields。  
  5. [ ] API：实现 mutation-capabilities，并包含扩展字段映射（`ext.<field_key>`）。  
  6. [ ] 列表：支持 ext 字段 filter/sort（allowlist + 参数化 + 可审计）。  
  7. [ ] 门禁：`make check routing` + `make authz-pack && make authz-test && make authz-lint` + Go 测试门禁通过。  

## 9. 测试与验收标准 (Acceptance Criteria)

- **API 契约测试**（至少覆盖）：
  - [ ] field-configs：启用/停用/列表（含权限拒绝、槽位耗尽/冲突错误映射）。  
  - [ ] options：DICT/ENTITY（含未启用字段 fail-closed）。  
  - [ ] details：返回 ext_fields（含 DICT display_value 来源）。  
  - [ ] mutation-capabilities：扩展字段进入 `allowed_fields/field_payload_keys`，且 `deny_reasons` 可解释。  
  - [ ] list：ext filter/sort 在 allowlist 内可用，越权字段被拒绝。  

- **安全验收**：
  - [ ] 无 SQL 注入风险（列名与实体来源可证明为 allowlist/枚举；值参数化）。  

- **出口条件（与路线图一致）**：
  - [ ] API 契约测试覆盖新增字段。  
  - [ ] 无 SQL 注入风险（列名与实体来源可证明）。  

## 10. 运维与监控 (Ops & Monitoring)

本阶段不引入运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。
