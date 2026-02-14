# DEV-PLAN-100A：Org 模块宽表元数据落地 Phase 0：契约冻结与就绪检查（先文档后代码）

**状态**: 冻结（2026-02-14 01:51 UTC）

> 本文从 `DEV-PLAN-100` 的 Phase 0 拆分而来，作为 Phase 0 的 SSOT；`DEV-PLAN-100` 保持为整体路线图。

## 1. 背景与上下文 (Context)

- **承接**：`DEV-PLAN-098`（架构评估） -> `DEV-PLAN-100`（整体路线图）。  
- **问题**：宽表预留字段 + 元数据驱动会同时触发：Valid Time（day）、One Door、RLS、动态 SQL allowlist、安全审计等多条不变量；若不先冻结契约，后续 Phase 1~4 的 schema/API/UI 很容易 drift，导致返工与审计口径不一致。  
- **Phase 0 定位**：先“文档与契约冻结”，再进入任何 DB/代码实现；本阶段产物是后续实现的输入与门禁前置检查清单。

## 2. 目标与非目标 (Goals & Non-Goals)

- **核心目标**：
  - [X] 冻结 `DEV-PLAN-100` §4 的 D1~D8（关键设计决策）。  
  - [X] 对齐不变量检查清单（One Door / Valid Time / RLS / No Legacy），并明确本计划落地时的 fail-closed 位置与责任边界。  
  - [X] 冻结 MVP 字段定义清单（2~5 个），并为每个字段确定：`field_key`、`value_type`、`PLAIN/DICT/ENTITY`、options 数据源配置、读写能力边界。  
  - [X] 冻结扩展字段 payload 契约（命名空间、类型编码/序列化、错误码口径），并证明与现有 `orgunit.submit_*` payload key 不冲突。  
  - [X] 冻结能力模型契约（写入/查询）：
    - 写入：在 `DEV-PLAN-083` 的策略矩阵下，补齐扩展字段的 `field_key -> payload_key` 规则、`deny_reasons` 与错误码对齐。  
    - 查询：补齐扩展字段 `field_key -> physical_col` 的 allowlist 口径（filter/sort/options）与 fail-closed 规则（承接 `DEV-PLAN-100` D7/D8）。  
  - [X] 冻结字段配置生命周期契约（启用/停用/停用后只读/不可复用/槽位耗尽），并明确 day 粒度生效与审计时间分离（SSOT：`DEV-PLAN-032`）。  
  - [X] 冻结“按阶段命中门禁清单”（routing/authz/sqlc/doc 等）并声明 SSOT 引用入口。  
- **非目标（本阶段不做）**：
  - 不做任何 DB 迁移（不新建表/不加列）。  
  - 不改 Kernel/Service/API 代码。  
  - 不实现 UI（字段配置页/详情编辑/列表筛选排序留到 Phase 4）。  
  - 不引入 feature flag/运维开关（遵循 `AGENTS.md`）。

## 2.1 工具链与门禁（SSOT 引用）

> 目的：只声明“本计划命中哪些触发器/门禁”，不在本文复制脚本细节（SSOT：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`）。

- **触发器清单（勾选本计划命中的项）**：
  - [X] 文档（`make check doc`）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] 路由治理（`make check routing`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [ ] DB 迁移 / Schema（按模块 `make <module> plan/lint/migrate ...`）
  - [ ] sqlc（`make sqlc-generate`，并确保生成物提交）

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - CI 门禁定义：`docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)

```mermaid
graph TD
  UI[Web UI (MUI): Field Configs + OrgUnit Details/List] --> API[Internal API: /org/api/org-units/*]
  API --> SVC[Service: capability resolver + payload builder]
  SVC --> KERNEL[DB Kernel: submit_*_event(...)]
  KERNEL --> EVTS[(orgunit.org_events.payload + snapshots)]
  KERNEL --> PROJ[Projection/Replay]
  PROJ --> VERS[(orgunit.org_unit_versions + ext cols)]
  SVC --> META[(orgunit.tenant_field_configs)]
  API --> META
```

### 3.2 关键设计决策 (ADR 摘要)

- **ADR-100A-01：冻结 D1~D8（SSOT 在 DEV-PLAN-100）**
  - 选定：以 `DEV-PLAN-100` §4（D1~D8）为 SSOT；本阶段完成“冻结确认”与缺失项补齐。  
  - 变更规则：若后续发现需调整 D1~D8，必须同步更新 `DEV-PLAN-100` 与本文，并在 PR 里写明原因与影响范围。

- **ADR-100A-02：扩展字段 payload 命名空间（避免与既有键冲突）**
  - 选定：扩展字段值统一放在 `payload.ext` 下；DICT 的展示快照放在 `payload.ext_labels_snapshot` 下（按 `field_key` 做 key）。  
  - 原则：现有顶层键（如 `name/new_name/parent_id/new_parent_id/is_business_unit/manager_uuid/org_code`）保持不变；扩展字段不得新增新的顶层键。

- **ADR-100A-03：MVP 写入能力边界（先小闭环）**
  - 选定（MVP 默认）：扩展字段仅在 `CREATE` 与 `CORRECT_EVENT(target=CREATE)` 两条写路径可写；其余事件型更新（`RENAME/MOVE/SET_BUSINESS_UNIT/...`）不接受扩展字段（fail-closed）。  
  - 说明：该边界与现有“按目标事件类型白名单”的写入模型一致（参考 `DEV-PLAN-082`/`DEV-PLAN-083`），避免为扩展字段引入新的事件语义。

- **ADR-100A-04：字段定义来源（避免变成全量动态平台）**
  - 选定（MVP）：后端提供“可启用字段定义列表”（2~5 个），UI 只能从该列表选择 `field_key`（对齐 `DEV-PLAN-101`）。  
  - 非目标：本期不支持租户自由创建任意 `field_key`。

- **ADR-100A-05：options 数据源类型（补齐 PLAIN，避免把“无 options 字段”误塞进 DICT/ENTITY）**
  - 选定：`data_source_type` 枚举冻结为：`PLAIN|DICT|ENTITY`。  
    - `PLAIN`：自由输入字段（无 options）；`data_source_config` 必须为 `{}`。  
    - `DICT`：值为 code（通常 `text`），options 来自 `dict_code` 枚举；DICT label 快照写入 `payload.ext_labels_snapshot`（见 `DEV-PLAN-100` D3/D4）。  
    - `ENTITY`：值为实体主键（`uuid|int`），options 来自 `entity` 枚举 + 固定 SQL 模板（`DEV-PLAN-100` D7）。  
  - 目的：让“是否有 options/如何取 options”成为显式契约，避免后续 UI/API/Kernel 各自发明隐式规则。

## 4. 数据模型与约束 (Data Model & Constraints)

> 本阶段不做迁移，但必须冻结 Phase 1 将要落地的数据模型与约束口径，避免实现时再做设计决策。

### 4.1 元数据表：`orgunit.tenant_field_configs`（契约草案）

- **字段（建议）**：
  - `tenant_uuid uuid not null`
  - `field_key text not null`（稳定业务键；MVP 仅来自“字段定义列表”）
  - `physical_col text not null`（例如 `ext_str_01`；由后端分配）
  - `value_type text not null`（`text|int|uuid|bool|date`）
  - `data_source_type text not null`（`PLAIN|DICT|ENTITY`；见 §4.3）
  - `data_source_config jsonb not null default '{}'::jsonb`（见 §4.3）
  - `enabled_on date not null`
  - `disabled_on date null`
  - `created_at timestamptz not null`
  - `updated_at timestamptz not null`
  - `disabled_at timestamptz null`

- **约束（必须）**：
  - 唯一：`(tenant_uuid, field_key)`  
  - 槽位唯一：`(tenant_uuid, physical_col)`  
  - 映射不可变：`field_key/physical_col/value_type/data_source_type/data_source_config/enabled_on` 启用后不可修改（DB trigger 拒绝）。  
  - 停用规则：
    - `disabled_on` 允许从 `NULL -> <date>`（允许未来停用排程）。  
    - `disabled_on` 不允许回溯（MVP 冻结）：必须满足 `disabled_on >= current_date`（UTC day；允许 `disabled_on = current_date` 表示“立即停用生效”）。  
    - `disabled_on` 允许从 `<date> -> <date>` 调整，但必须同时满足：
      - 原 `disabled_on` **尚未生效**（冻结口径：以 UTC day 判定，要求 `current_date < old_disabled_on`）。  
      - 仅允许 **向后延迟**（`new_disabled_on > old_disabled_on`）。  
    - 不允许 `disabled_on` 从非空回滚为 `NULL`（避免“看起来重新启用”的隐式语义漂移）。  
  - 日期约束：`disabled_on is null OR disabled_on >= enabled_on`（day 粒度）。

- **有效期语义（必须冻结，避免同日歧义）**：
  - 采用 day 粒度半开区间模型：字段在某日 `d` 生效当且仅当：`enabled_on <= d AND (disabled_on IS NULL OR d < disabled_on)`。  
  - 说明：这是 `[enabled_on, disabled_on)` 模型，与仓库 day 粒度 Valid Time 约定一致（SSOT：`DEV-PLAN-032`）；并与 UI 口径对齐：当 `as_of >= disabled_on` 时该字段不可写/不可见，且 details 的 `ext_fields[]` 不再返回/不再展示该字段；若需查看历史值，应切换 `as_of` 或查看 Audit（变更日志）。

- **RLS（必须）**：
  - `ENABLE ROW LEVEL SECURITY` + `FORCE ROW LEVEL SECURITY`；
  - policy 口径与 OrgUnit 既有表一致（SSOT：`DEV-PLAN-021`）。

### 4.2 宽表扩展列：`orgunit.org_unit_versions`（契约引用）

- 槽位命名与类型分组：见 `DEV-PLAN-100` D2。  
- DICT 快照：新增 `ext_labels_snapshot jsonb`（只存 DICT 字段的 label 快照，控制键集合与大小；见 `DEV-PLAN-100` D3/D4）。

### 4.3 `data_source_config` 约束（禁止任意表/列透传）

- `PLAIN`：
  - 形状：`{}`
  - 约束：必须为空对象；不允许透传任意 options 配置。
- `DICT`：
  - 形状：`{"dict_code":"<enum>"}`  
  - 约束：`dict_code` 必须为枚举（来源与加载策略由 Phase 0 冻结；不得传任意 SQL 片段/表名）。
- `ENTITY`：
  - 形状：`{"entity":"<enum>","id_kind":"uuid|int"}`  
  - 约束：`entity` 必须为枚举；实际 join 模板由后端固定映射（`DEV-PLAN-100` D7）。

### 4.4 审计与幂等（`request_code` / `initiator_uuid`）——冻结

> 背景：Phase 1 的 Kernel 函数签名已包含 `p_request_code/p_initiator_uuid`（SSOT：`DEV-PLAN-100B`），因此 Phase 0 必须冻结其“幂等键与审计落点”，避免实现期临时发明口径。

- **幂等键（必须）**：
  - `request_code` 为同租户内的幂等键：同一 `tenant_uuid + request_code` 的重复请求必须幂等（重试不应产生第二次写入/第二个映射）。  
  - 若 `request_code` 已被用于不同的请求输入（例如不同 `field_key` 或不同 `enabled_on/disabled_on/data_source_*`），必须拒绝并抛稳定错误码（建议复用 `ORG_REQUEST_ID_CONFLICT`，与 OrgUnit 既有 request_code 冲突口径一致）。
- **审计落点（必须可追溯）**：
  - 必须在 DB 中可追溯：`request_code`、`initiator_uuid`、`tx_time`、`action(enable|disable)`、`field_key`、`enabled_on/disabled_on`、`physical_col`。  
  - 推荐实现方式：新增事件/审计表 `orgunit.tenant_field_config_events`，并对 `(tenant_uuid, request_code)` 加唯一约束；表级启用并强制 RLS，且仅允许 `orgunit_kernel` 写入（实现细节在 Phase 1 落地）。

## 5. 接口契约 (API Contracts)

> 本阶段不实现 API，但必须冻结“后续实现将遵循的契约”，避免 UI/Service/Kernel 三方漂移。

### 5.1 扩展字段写入 payload（Kernel 输入）

- **约定**：
  - 扩展字段值统一写入 `payload.ext`，key 为 `field_key`。  
  - DICT 的 label 快照写入 `payload.ext_labels_snapshot`，key 同为 `field_key`。  
  - `payload.ext_labels_snapshot` 为 Kernel/审计所需输入：**由服务层生成并写入**（UI 不提交该字段）；Kernel 对 DICT 字段缺失 label 快照必须 fail-closed（细节在 Phase 2/3 落地，SSOT：`DEV-PLAN-100C/100D`）。
- **示例（CREATE）**：

```json
{
  "org_code": "R&D",
  "name": "R&D",
  "parent_id": "10000001",
  "ext": {
    "short_name": "R&D",
    "org_type": "DEPARTMENT"
  },
  "ext_labels_snapshot": {
    "org_type": "Department"
  }
}
```

### 5.2 字段定义列表（供 UI 启用字段选择）

- `GET /org/api/org-units/field-definitions`
- **Response（抽象）**：
  - `fields[]`：
    - `field_key`（稳定键）
    - `value_type`
    - `data_source_type`
    - `data_source_config`（默认/固定配置；形状见 §4.3）
    - `data_source_config_options`（仅 DICT/ENTITY；非空数组；若固定则为单元素数组，若可选则包含多个候选；启用字段时必须从中选择并提交）
    - `label_i18n_key`（或直接返回 `label`；但需对齐 `DEV-PLAN-020`）

> 约束：当 `data_source_type=PLAIN` 时，该字段无 options，`data_source_config` 必须为 `{}`。

### 5.3 字段配置管理（启用/停用/列表）

- `GET /org/api/org-units/field-configs?status=all|enabled|disabled&as_of=YYYY-MM-DD`
- `POST /org/api/org-units/field-configs`（启用字段，后端分配 `physical_col`）
- `POST /org/api/org-units/field-configs:disable`（停用字段，设置 `disabled_on`；路径形态在 Phase 0 评审中冻结，并需通过 `make check routing`）

> 约束：启用/停用写请求必须携带 `request_code`；`initiator_uuid` 由服务端会话上下文注入并传递给 Kernel（不得由 UI 提交/伪造）。
>
> 启用字段时的数据来源配置（冻结）：
>
> - `PLAIN`：无 options，`data_source_config` 必须为 `{}`（可缺省，由服务端补齐为空对象）。  
> - `DICT/ENTITY`：`data_source_config` 由租户管理员在启用字段时选择并提交，且必须命中 `field-definitions.data_source_config_options`；禁止任意输入/透传。

### 5.4 Mutation capabilities 扩展字段口径（承接 DEV-PLAN-083）

- `GET /org/api/org-units/mutation-capabilities?org_code=<...>&effective_date=<...>`（SSOT：`DEV-PLAN-083`）
- **扩展字段要求**：
  - `allowed_fields` 必须包含扩展字段的 `field_key`（在允许写入的动作里）。
  - `field_payload_keys[field_key]` 对扩展字段统一返回 `ext.<field_key>`（dot-path 字符串；表示 payload 内 `ext` 对象下的 key）。
  - `deny_reasons` 与错误码口径保持稳定（服务层与 Kernel 对齐，fail-closed）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 字段启用（物理槽位分配）——伪代码

1. 开启事务（显式 tx + tenant 注入；fail-closed）。
2. 校验 `field_key` 在 `field-definitions` 列表中，且该租户未存在同 `field_key` 配置。  
3. 校验 `data_source_config`：  
   - `PLAIN`：必须为 `{}`；  
   - `DICT/ENTITY`：必须命中该 `field_key` 在 `field-definitions` 返回的 `data_source_config_options`。  
4. 根据 `value_type/data_source_type` 选择槽位分组（例如 `PLAIN(text)/DICT(text) -> ext_str_*`；`ENTITY(uuid) -> ext_uuid_*`）。
5. 分配第一个空闲 `physical_col`（同租户下未占用）。
6. 写入 `tenant_field_configs`（由 Kernel 函数执行；应用层禁止直写）。
7. 提交事务。

### 6.2 扩展字段写入校验（服务层 + Kernel）

- 服务层：
  - 基于 `DEV-PLAN-083` ResolvePolicy 输出的 `allowed_fields` 过滤 patch；扩展字段不在 allowlist 则拒绝（`PATCH_FIELD_NOT_ALLOWED` 或专用错误码，Phase 0 冻结）。  
  - 将扩展字段写入统一映射到 `payload.ext[field_key]`。
- Kernel：
  - 仅允许 `payload.ext` 出现已启用字段（按 `effective_date` 判断 `enabled_on/disabled_on`）；否则拒绝（fail-closed）。
  - 将 `payload.ext[field_key]` 投射到 `org_unit_versions.<physical_col>`。

## 7. 安全与鉴权 (Security & Authz)

- 字段配置管理：
  - UI 与 API 必须要求管理员权限（对齐 `DEV-PLAN-022`）；无权限统一拒绝（fail-closed）。
  - DB 层：`tenant_field_configs` 强制 RLS，且应用角色不得直接 DML（必须走 Kernel 管理函数）。
- 动态 SQL：
  - 列名仅允许由 `field_key -> physical_col` 映射得到；不得拼接用户输入列名/表名（对齐 `DEV-PLAN-100` D7）。

## 8. 依赖与里程碑 (Dependencies & Milestones)

- **依赖**：
  - `DEV-PLAN-098`（评估结论）
  - `DEV-PLAN-100`（整体路线图；D1~D8 冻结项）
  - `DEV-PLAN-083`（策略矩阵与 mutation capabilities SSOT）
  - `DEV-PLAN-101`（字段配置 UI IA）
  - `DEV-PLAN-032`（Valid Time day 粒度）
  - `DEV-PLAN-021`（RLS 强租户隔离）
  - `DEV-PLAN-017`（路由策略与门禁）

- **里程碑（Phase 0 待办）**：
  1. [X] 冻结 D1~D8（在 `DEV-PLAN-100` 与本文标记为冻结，并评审确认）。
  2. [X] 在 `DEV-PLAN-098` 增加“由 DEV-PLAN-100 承接实施”链接（保持文档可追踪）。  
  3. [X] 冻结 MVP 字段定义清单（2~5 个，见 §9.1 表格）。  
  4. [X] 冻结 payload 契约（`payload.ext`/`payload.ext_labels_snapshot`）与错误码口径。  
  5. [X] 冻结字段配置生命周期（启用/停用/只读/不可复用/槽位耗尽）。  
  6. [X] 冻结 capabilities 扩展字段口径（`allowed_fields/field_payload_keys/deny_reasons`）。  
  7. [X] 冻结“按阶段命中门禁清单”（routing/authz/sqlc/doc）并对齐 SSOT 引用入口。  
  8. [X] 通过文档门禁：`make check doc`（2026-02-14 01:51 UTC；`[doc] OK`）。

## 9. 测试与验收标准 (Acceptance Criteria)

### 9.1 MVP 字段定义清单（已冻结）

| field_key | value_type | data_source_type | data_source_config | 读能力（filter/sort/options） | 写能力（MVP） |
| --- | --- | --- | --- | --- | --- |
| `short_name` | `text` | `PLAIN` | `{}` | `filter: no; sort: no; options: n/a` | `CREATE + CORRECT_EVENT(target=CREATE)` |
| `description` | `text` | `PLAIN` | `{}` | `filter: no; sort: no; options: n/a` | `CREATE + CORRECT_EVENT(target=CREATE)` |
| `org_type` | `text` | `DICT` | `{"dict_code":"org_type"}` | `filter: yes; sort: yes; options: yes` | `CREATE + CORRECT_EVENT(target=CREATE)` |
| `location_code` | `text` | `PLAIN` | `{}` | `filter: no; sort: no; options: n/a` | `CREATE + CORRECT_EVENT(target=CREATE)` |
| `cost_center` | `text` | `PLAIN` | `{}` | `filter: no; sort: no; options: n/a` | `CREATE + CORRECT_EVENT(target=CREATE)` |

> `data_source_config` 口径说明（冻结）：表中 `data_source_config` 表示该字段的默认配置；当 `data_source_type IN (DICT, ENTITY)` 且该字段允许多种来源配置时，`field-definitions` 必须返回 `data_source_config_options[]`，租户管理员在“启用字段”时从中选择并提交（否则启用应失败）。

> MVP 约定：列表筛选/排序命中扩展字段的最小闭环字段为 `org_type`（DICT，含 options + label snapshot），其余字段先走“详情可见/可写”闭环，避免一次性扩大动态 SQL/索引面。

> 注：字段候选来源可参考 `DEV-PLAN-073` “组织单元属性扩展候选集合”；最终以 Phase 0 评审冻结为准。

### 9.2 本阶段 DoD（完成即允许进入 Phase 1）

- [X] `DEV-PLAN-100A` 完成评审：字段清单、payload 契约、能力模型口径、生命周期契约全部冻结。  
- [X] `DEV-PLAN-100` 的 Phase 0 已指向本文，且不再重复维护 Phase 0 细节。  
- [X] `AGENTS.md` Doc Map 已收录本文链接（可发现性）。  
- [X] `make check doc` 通过（2026-02-14 01:51 UTC；`[doc] OK`；建议在 PR 描述或 `docs/dev-records/` 留证）。

## 10. 运维与监控 (Ops & Monitoring)

- 本阶段不引入运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。
