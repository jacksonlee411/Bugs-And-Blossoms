# DEV-PLAN-100E：Org 模块宽表元数据落地 Phase 4A：OrgUnit 详情页扩展字段展示与 Capabilities 驱动编辑（MUI）

**状态**: 草拟中（2026-02-13；已对齐 `DEV-PLAN-100D` Phase 3 冻结契约）

> 本文从 `DEV-PLAN-100` Phase 4 的 4A 拆分而来，作为 4A 的 SSOT；`DEV-PLAN-100` 继续保持为整体路线图。  
> 本文聚焦 **UI 侧**的“详情页扩展字段展示 + 编辑态能力外显（fail-closed）”，并明确：开展 4A 前必须具备 `DEV-PLAN-083` 的核心产物可用（mutation policy 单点 + capabilities API）；其后端前置改造执行计划见 `DEV-PLAN-100E1`。

## 1. 背景与上下文 (Context)

`DEV-PLAN-100` 在 OrgUnit 引入“宽表预留字段 + 元数据驱动”的扩展字段体系，并在 Phase 4 要求形成用户可见闭环。  
其中 4A 的目标是：在 OrgUnit 详情页把扩展字段“看得见、能编辑、且不会出现可输必败”。

当前痛点（对齐 `DEV-PLAN-083` 背景）：

- UI 无法提前得知“当前记录/动作下哪些字段可更正”，用户可输入但提交后被服务端拒绝（典型：`PATCH_FIELD_NOT_ALLOWED`）。
- 扩展字段引入后，字段集合随租户与 as_of 变化；若 UI 自行维护白名单/映射，将不可避免地产生漂移与安全风险。

因此：4A 必须以 **capabilities 外显**为前置，确保 UI 以服务端策略为唯一事实源（SSOT：`DEV-PLAN-083` + `DEV-PLAN-100` D8）。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标

- [ ] OrgUnit 详情页在 `as_of`（本页为 `effective_date`）下展示扩展字段（动态渲染）。
- [ ] OrgUnit 详情页“更正（Correct）”编辑态严格消费 `mutation-capabilities`：
  - 字段是否可编辑由 `allowed_fields` 决定（UI 不维护第二套白名单）。
  - 动作是否可用由 `capabilities.*.enabled` 决定，并展示 `deny_reasons`（可解释）。
  - capabilities API 不可用/解析失败时：UI **fail-closed**（只读/禁用，不做乐观放行）。
- [ ] Select 字段（`DICT/ENTITY`）在编辑态接入 options endpoint（支持 `q` 搜索 + `as_of`）。
- [ ] 更正支持修改“更正后生效日”（`patch.effective_date`），且写入成功后 UI 自动切换到新版本（URL `effective_date` 更新），避免“成功了但仍停留在旧版本”的错觉。

### 2.2 非目标 (Out of Scope)

- 不实现字段配置管理页（Phase 4B，SSOT：`DEV-PLAN-101`）。
- 不实现 OrgUnit 列表页扩展字段筛选/排序入口（Phase 4C）。
- 不在本计划内设计/变更 DB schema、Kernel 函数或元数据表结构（其契约归 `DEV-PLAN-100` Phase 1/2/3）。
- 不实现“任意租户自定义 label 的业务数据多语言存储结构”（对齐 `DEV-PLAN-020` 边界）。
- MVP 不启用 `data_source_type=PLAIN` 的扩展字段；本计划仅要求实现 `DICT/ENTITY` 扩展字段的编辑控件。若 details 仍返回 `PLAIN`，UI 必须 fail-closed：只读展示 + 明确 warning，不允许提交该字段的更正。

### 2.3 工具链与门禁（SSOT 引用）

> 本文不复制命令矩阵；触发器与门禁入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。

- 触发器（本计划实施通常会命中）：
  - [ ] 文档：`make check doc`（本文 + 引用更新）
  - [ ] Web UI（`apps/web-mui`）：以 CI 前端门禁为准（Typecheck/Lint/Test/Build）
  - [ ] 多语言 JSON：`make check tr`（扩展字段 label i18n key 与 deny reason 文案）
  - [ ] （依赖项）路由治理：若实现中补齐 capabilities/options/details 等后端路由变更，需通过 `make check routing`（SSOT：`DEV-PLAN-017`）
  - [ ] （依赖项）Authz：若新增权限点/策略，需通过 `make authz-pack && make authz-test && make authz-lint`（SSOT：`DEV-PLAN-022`）

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图（4A 关注链路）

```mermaid
graph TD
  UI[apps/web-mui OrgUnitDetailsPage] -->|GET details(as_of)| Details[/org/api/org-units/details/]
  UI -->|GET mutation-capabilities(effective_date)| Caps[/org/api/org-units/mutation-capabilities/]
  UI -->|GET fields:options(field_key, as_of, q)| Opts[/org/api/org-units/fields:options/]
  UI -->|POST corrections| Correct[/org/api/org-units/corrections/]

  Correct --> SVC[OrgUnit Write Service]
  SVC --> POL[Mutation Policy SSOT<br/>DEV-PLAN-083]
  SVC --> META[tenant_field_configs + ext slots<br/>DEV-PLAN-100B]
  SVC --> KERNEL[submit_* (One Door)<br/>DEV-PLAN-100C]
```

### 3.2 关键设计决策（ADR 摘要）

- **ADR-100E-01：UI 对写入能力的唯一事实源是 capabilities API**
  - 选定：不在 UI 侧维护字段白名单/动作可用性分支；全部由 `mutation-capabilities` 决定（对齐 `DEV-PLAN-083`）。
  - 失败策略：capabilities 不可用时 fail-closed（只读/禁用）。

- **ADR-100E-02：扩展字段 label 使用 i18n key，避免引入“业务数据多语言”**
  - 选定：扩展字段展示使用 `label_i18n_key`（或可由 `field_key` 推导的稳定 key）；具体输出契约见 §5.1。
  - 约束：不得引入 `label_zh/label_en` 的租户可编辑持久化结构（非本计划）。

- **ADR-100E-03：编辑态不信任 UI 提交 label 快照**
  - 选定：UI 只提交扩展字段的值（DICT 提交 code）；DICT label 快照由服务端 options resolver 生成并写入（SSOT：`DEV-PLAN-100D`）。

## 4. 数据模型与约束 (Data Model & Constraints)

> 本计划不新增数据库结构；本节冻结 UI 需要的运行时数据形状（TypeScript 侧）。

### 4.1 扩展字段（详情接口输出）

UI 需要以下字段以实现“展示 + 编辑控件选择 + options 调用”：

```ts
export type ExtValueType = 'text' | 'int' | 'uuid' | 'bool' | 'date'
export type ExtDataSourceType = 'PLAIN' | 'DICT' | 'ENTITY'
export type ExtDisplayValueSource =
  | 'plain'
  | 'versions_snapshot'
  | 'events_snapshot'
  | 'dict_fallback'
  | 'entity_join'
  | 'unresolved'
export type ExtScalarValue = string | number | boolean | null

export interface OrgUnitExtField {
  field_key: string
  label_i18n_key: string
  value_type: ExtValueType
  data_source_type: ExtDataSourceType
  value: ExtScalarValue
  display_value: string | null
  display_value_source: ExtDisplayValueSource
}
```

约束：

- `value` 的解析与校验以服务端为准；UI 仅做基本格式约束（例如 date 输入必须为 `YYYY-MM-DD`）。
- `value` 的 JSON 标量类型口径（对齐 `DEV-PLAN-100D`）：
  - `text/uuid/date`：`string | null`
  - `int`：`number | null`
  - `bool`：`boolean | null`
- MVP 约束：不启用 `data_source_type=PLAIN` 的字段；若出现，UI 只读展示并给出 warning（不提供输入控件，不允许提交更正）。
- 当 `data_source_type=PLAIN`：options 不适用，UI 禁止调用 options endpoint（fail-closed）。
- `ext_fields` 必须包含 `as_of` 下 **enabled 的字段全集**（即使该字段当前无值，`value=null` 也必须返回），避免出现“字段已启用但页面不可见/不可编辑”的僵尸体验。
- 当 `as_of >= disabled_on` 时该字段不属于 enabled 集合，因此不应出现在 `ext_fields`；若需查看历史值，请切换 `as_of` 或查看 Audit（变更日志）。
- `field_key` 由服务端保证稳定与可校验（field-definitions SSOT）；UI 将其视为不透明标识，**不得**在前端维护“保留字列表”或二次推导 payload 路径，路径一律以 `field_payload_keys` 为准。

### 4.2 Capabilities（编辑态能力外显）

UI 最小需要：

- `enabled`：动作是否可用；
- `allowed_fields[]`：字段是否可编辑（包含扩展字段的 `field_key`）；
- `field_payload_keys{}`：字段到 **corrections 请求 `patch` 内路径**的映射（扩展字段必须为 `ext.<field_key>`；基础字段映射为其 patch key，例如 `name -> name`）；
- `deny_reasons[]`：动作不可用时的原因列表（可直接展示）。

> 具体 JSON 形状由 `DEV-PLAN-083` 冻结；本计划在 §5.2 给出 UI 所需的最小合约示例，作为 4A 的 readiness 前置条件。

## 5. 接口契约 (API Contracts)

> 本节是“4A UI 联调所需的最小合约”，后端实现 SSOT 分别在 `DEV-PLAN-083` 与 `DEV-PLAN-100D`；若后端契约发生变更，应先更新对应 SSOT 文档，再回写本节。

### 5.1 Details：扩展字段展示（Read）

- `GET /org/api/org-units/details?org_code=<...>&as_of=YYYY-MM-DD&include_disabled=...`
- Authz：`orgunit.read`
- Response 200（`ext_fields` 必须包含 `label_i18n_key`）：

```json
{
  "as_of": "2026-02-13",
  "org_unit": {
    "org_id": 10000001,
    "org_code": "R&D",
    "name": "R&D",
    "status": "active",
    "parent_org_code": "ROOT",
    "parent_name": "Root",
    "is_business_unit": false,
    "manager_pernr": "00000001",
    "manager_name": "Alice",
    "full_name_path": "Root / R&D",
    "created_at": "2026-02-01T00:00:00Z",
    "updated_at": "2026-02-10T12:00:00Z",
    "event_uuid": "..."
  },
  "ext_fields": [
    {
      "field_key": "org_type",
      "label_i18n_key": "org.fields.org_type",
      "value_type": "text",
      "data_source_type": "DICT",
      "value": "DEPARTMENT",
      "display_value": "Department",
      "display_value_source": "versions_snapshot"
    }
  ]
}
```

约束：

- `ext_fields` 必须包含 `as_of` 下 enabled 的字段全集（即使 `value=null`）；day 粒度口径见 `DEV-PLAN-100D`。
- `label_i18n_key` 必须稳定（i18n SSOT：`DEV-PLAN-020`）；服务端必须返回该字段，UI 不维护第二套“字段 -> label”映射。
- `ext_fields` 排序必须稳定（按 `field_key` 升序；对齐 `DEV-PLAN-100D`），避免 UI 抖动与测试不稳定。
- `display_value/display_value_source` 必须成对使用（对齐 `DEV-PLAN-100D`）：
  - DICT 允许：`versions_snapshot/events_snapshot/dict_fallback/unresolved`；
  - PLAIN：`plain`；ENTITY：`entity_join`；
  - `display_value_source='unresolved'` 时允许 `display_value=null`，UI 必须展示可排障提示，不得静默伪造值。
- UI 展示层仅允许兜底：若 `label_i18n_key` 缺失/为空/找不到翻译，则显示 `field_key` 并展示 warning（可排障），但不得静默吞掉。

### 5.2 Mutation Capabilities：编辑态能力外显（SSOT：DEV-PLAN-083）

- `GET /org/api/org-units/mutation-capabilities?org_code=<...>&effective_date=YYYY-MM-DD`
- Authz：`orgunit.read`（或等价 read 权限；最终以 `DEV-PLAN-083/022` 冻结为准）

UI 期望最小响应（示例；字段名最终以 `DEV-PLAN-083` 为 SSOT）：

```json
{
  "org_code": "R&D",
  "effective_date": "2026-02-13",
  "effective_target_event_type": "RENAME",
  "raw_target_event_type": "RENAME",
  "capabilities": {
    "correct_event": {
      "enabled": true,
      "allowed_fields": ["effective_date", "name", "org_type"],
      "field_payload_keys": {
        "effective_date": "effective_date",
        "name": "name",
        "org_type": "ext.org_type"
      },
      "deny_reasons": []
    },
    "correct_status": {
      "enabled": false,
      "allowed_target_statuses": [],
      "deny_reasons": ["ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET"]
    },
    "rescind_event": {
      "enabled": true,
      "deny_reasons": []
    },
    "rescind_org": {
      "enabled": false,
      "deny_reasons": ["ORG_ROOT_DELETE_FORBIDDEN"]
    }
  }
}
```

约束（对齐 `DEV-PLAN-083` 与 `DEV-PLAN-100` D8）：

- `allowed_fields` 必须包含扩展字段的 `field_key`（当该动作允许写入时）。
- `field_payload_keys[field_key]` 对扩展字段必须为 `ext.<field_key>`。
- `allowed_fields` 与 `field_payload_keys` 必须一致（双向包含），且输出顺序稳定（避免 UI 抖动/测试不稳定）。
- `enabled` 必须已纳入 Authz 结果（例如调用方缺少 `orgunit.admin` 时，`correct_event.enabled=false`，并返回稳定 `deny_reasons`；避免 UI “按钮禁用但无解释”或“可输必败”）。
- capabilities API 不可用/返回错误时，UI 必须 fail-closed（只读/禁用）。

### 5.3 Options：DICT/ENTITY（PLAIN 必拒绝）

- `GET /org/api/org-units/fields:options?field_key=<...>&as_of=YYYY-MM-DD&q=<keyword>&limit=<n>`
- Authz：至少 `orgunit.read`（SSOT：`DEV-PLAN-100D/022`）
- Response 200（SSOT：`DEV-PLAN-100D`）：

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

- 若字段在 `as_of` 下未启用：必须 fail-closed，返回 404 `ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF`（SSOT：`DEV-PLAN-100D` §5.7）。
- 若 `data_source_type=PLAIN`：options 不适用，必须 fail-closed，返回 404 `ORG_FIELD_OPTIONS_NOT_SUPPORTED`（SSOT：`DEV-PLAN-100D` §5.7）。
- `q` 可选，服务端对输入做 trim；为空时返回“前 N 个”。
- `limit` 可选：缺失/非法默认 `10`，上限 `50`。
- 返回顺序稳定：按 `label` 升序，其次 `value` 升序。
- `label` 为 canonical label（非本地化展示名），UI 不对其做 i18n 替换；字段标题仍使用 `label_i18n_key`。

### 5.4 写入：更正接口扩展字段 patch（依赖项，SSOT：DEV-PLAN-083/100D）

本计划的 UI 写入动作仍复用现有更正接口：

- `POST /org/api/org-units/corrections`

为支持扩展字段写入，patch 需要支持 `ext` 子对象（字段集合与校验以服务端为准）：

示例 A：更正扩展字段（不改生效日）：

```json
{
  "org_code": "R&D",
  "effective_date": "2026-02-13",
  "request_id": "req-...",
  "patch": {
    "name": "R&D - Updated",
    "ext": {
      "org_type": "DEPARTMENT"
    }
  }
}
```

示例 B：仅更正“更正后生效日”（不携带其它字段）：

```json
{
  "org_code": "R&D",
  "effective_date": "2026-02-13",
  "request_id": "req-...",
  "patch": {
    "effective_date": "2026-02-15"
  }
}
```

约束：

- UI **不得**提交 `ext_labels_snapshot`；DICT label 快照必须由服务端生成（SSOT：`DEV-PLAN-100D`）。
- 服务端必须基于 capabilities 的 `allowed_fields` 对 patch 做 fail-closed 校验（SSOT：`DEV-PLAN-083`）。
- UI 提交的 `patch` 必须满足“最小变更 + 权限裁剪”：
  - 仅包含 **发生变更**且 **在 `allowed_fields` 内**的字段；
  - 对扩展字段：仅允许写入 `patch.ext`，且 `patch.ext` 只包含允许且变更的 `field_key`；
  - 任何需要但缺失的字段由服务端按策略拒绝（fail-closed），UI 不做“猜测补齐”。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 详情页加载（展示态）

1. 以 URL 的 `effective_date` 作为 details 的 `as_of`（既有页面行为保持）。
2. 渲染基础字段（既有）。
3. 渲染 `ext_fields[]`：
   - label：`t(label_i18n_key)`；若缺失，回退展示 `field_key`（并显示 warning badge，避免静默漂移）。
   - value：优先展示 `display_value`；若 `display_value=null` 且 `value!=null`，则展示 `value` 的字符串形式（便于排障；禁止静默丢失 code/id）。
   - value：当 `display_value=null` 且 `value=null`，展示 `-`。
   - source（`display_value_source`）：
     - `dict_fallback`：展示“历史标签兜底”warning（非阻断）；
     - `unresolved`：展示“展示值不可解析”warning（可排障，禁止静默）。

### 6.2 更正（Correct）弹窗（编辑态，capabilities-driven）

打开弹窗前置：

- 以当前选中版本的 `effective_date` 调用 `mutation-capabilities` 获取 `capabilities.correct_event`。
- “更正”按钮可见性与可用性分离：
  - 可见：具备 `orgunit.read` 即可可见（便于解释不可用原因）；
  - 可用：仅当 `capabilities.correct_event.enabled=true`。
- 若用户无 `orgunit.admin`，应由 capabilities 返回 `enabled=false + deny_reasons`；UI 仅消费结果，不自造第二套授权判定。

弹窗字段渲染规则：

1. 基础字段集合：`name/parent_org_code/manager_pernr/is_business_unit/target_effective_date(corrected_effective_date)`（既有表单字段；其中 target_effective_date 为只读展示，corrected_effective_date 写入 `patch.effective_date`）。
2. 扩展字段集合：以 details 的 `ext_fields[]` 为准（动态）。
3. 对每个字段：
   - 若 `field_key` 不在 `allowed_fields`：字段**禁用但仍可见**（不隐藏，避免“字段消失/不可解释”）；并展示统一 helperText：`不允许更正该字段（PATCH_FIELD_NOT_ALLOWED）`。
4. 若 `capabilities.correct_event.enabled=false`：
   - 禁用“确认”按钮；
   - 弹窗顶部展示 `deny_reasons`（按列表展示，或映射为 i18n 文案）。
5. 若 capabilities 请求失败：
   - 弹窗顶部展示错误；
   - 全部输入禁用 + 禁用确认按钮（fail-closed，不做本地乐观放行）。
6. 当用户填写了 `corrected_effective_date` 且其值与 target_effective_date 不同：
   - 弹窗进入“生效日更正”模式：仅允许修改 `corrected_effective_date`，其余基础字段与全部扩展字段一律禁用；
   - 弹窗顶部展示明确提示：更正生效日需单独提交（避免与扩展字段 enabled 集合随 day 变化产生漂移与“写了但回显消失”风险）。

提交（构造 patch）规则（关键：避免“禁用但仍提交”）：

1. 以 details 中当前值作为“原值快照”（含 ext_fields），并以 capabilities 的 `allowed_fields/field_payload_keys` 作为“唯一可写集合”。
2. **生效日更正模式**（`corrected_effective_date != ""` 且 `!= target_effective_date`）：
   - patch 只允许包含 `effective_date` 一个字段（对应 `field_payload_keys["effective_date"]="effective_date"`）；
   - 其它字段（含 ext）即使被编辑也不得进入 patch（fail-closed）。
3. **普通更正模式**（未进入生效日更正模式）：
   - 对每个表单项：
     - 若该字段不在 `allowed_fields`：**不进入 patch**（无论 UI 是否有值）。
     - 若字段值与原值一致：**不进入 patch**（最小变更）。
   - 写入位置必须由 `field_payload_keys[field_key]` 决定（禁止 UI 自行拼路径）：
     - 若 `field_payload_keys[field_key]` 以 `ext.` 开头：写入 `patch.ext[field_key]=value`；
     - 否则：写入 `patch[ field_payload_keys[field_key] ]=value`（例如 `name`、`parent_org_code`、`effective_date`）。
4. `is_business_unit` 等布尔字段必须采用“可判定是否变更”的策略（对比原值；未变更则不提交），避免当前实现那种“总是提交”导致策略收紧后必失败。
5. 伪代码（用于实现与单测对齐；非运行时 contract）：

```ts
type Patch = Record<string, unknown> & { ext?: Record<string, unknown> }

function buildPatch(input: {
  allowedFields: string[]
  fieldPayloadKeys: Record<string, string>
  original: Record<string, unknown> & { ext?: Record<string, unknown> }
  next: Record<string, unknown> & { ext?: Record<string, unknown> }
}): Patch {
  const patch: Patch = {}
  for (const fieldKey of input.allowedFields) {
    const payloadKey = input.fieldPayloadKeys[fieldKey]
    if (!payloadKey) continue

    const prevValue = payloadKey.startsWith('ext.') ? input.original.ext?.[fieldKey] : input.original[fieldKey]
    const nextValue = payloadKey.startsWith('ext.') ? input.next.ext?.[fieldKey] : input.next[fieldKey]

    if (prevValue === nextValue) continue

    if (payloadKey.startsWith('ext.')) {
      patch.ext ??= {}
      patch.ext[fieldKey] = nextValue
      continue
    }

    patch[payloadKey] = nextValue
  }
  if (patch.ext && Object.keys(patch.ext).length === 0) delete patch.ext
  return patch
}
```

Select 字段（DICT/ENTITY）控件策略：

- 使用 `Autocomplete`（或 `Select + async`）：
  - 输入时按 `q` 触发 options endpoint；
  - query 固定携带 `as_of=<effective_date>`；
  - 选中后在 form state 保存 `option.value`（DICT 为 code；ENTITY 为 id）。
- 清空选择（clear）时：必须将对应字段写为 `null`（进入 patch，表示显式清空），不得“忽略不提交”。
- 任何 options 请求失败：该字段进入只读并提示错误（避免提交无效值）。
- options 调用需做最小抖动控制：建议 debounce（200~300ms）+ 按 `(field_key, as_of, q)` 作为缓存 key（至少在弹窗生命周期内缓存），避免每次按键都打满请求。

## 7. 安全与鉴权 (Security & Authz)

- 页面路由（既有）：`RequirePermission permissionKey='orgunit.read'`（只读可访问）。
- 写入动作（既有）：仅 `orgunit.admin` 可触发（按钮禁用 + 服务端 403 双重保证）。
- 能力外显：
  - UI 不做“默认放行”；capabilities 缺失或异常时 fail-closed。
  - UI 可将 `hasPermission('orgunit.admin')` 作为“预判提示”，但**不得**替代 capabilities/服务端最终判定。
  - UI 不拼装 SQL / 不透传列名/表名；所有动态查询由后端 allowlist/枚举映射保证（SSOT：`DEV-PLAN-100` D7）。
  - capabilities 响应必须体现 Authz（`enabled/deny_reasons`），避免 UI 侧硬编码复杂“写权限判定”分支。

## 8. 依赖与里程碑 (Dependencies & Milestones)

### 8.1 强依赖（4A 开工前必须满足）

- [ ] `DEV-PLAN-083` 核心产物可用：
  - [ ] mutation policy 单点（`ResolvePolicy/AllowedFields/ValidatePatch`）已落地并有单测覆盖（最少覆盖 `correct_event`）。
  - [ ] `GET /org/api/org-units/mutation-capabilities` 已实现并冻结返回字段（含 `allowed_fields/field_payload_keys/deny_reasons`），且错误码稳定（避免 UI 猜测）。
- [ ] `DEV-PLAN-100D` 提供 4A 所需接口：
  - [ ] details 返回 `ext_fields[]`（含 `label_i18n_key/value_type/data_source_type/value/display_value/display_value_source`），且 **enabled 字段全集必须返回（即使 value=null）**。
  - [ ] options endpoint 可用（DICT/ENTITY；PLAIN 必拒绝），并冻结 404 错误码与 `q/limit` 口径。
  - [ ] corrections 写入链路可接收扩展字段 patch（`patch.ext`），并与 capabilities 校验一致（fail-closed）。

### 8.2 里程碑（本计划待办）

1. [ ] Web API client：在 `apps/web-mui/src/api/orgUnits.ts` 增加（或拆分新文件）：
   - `getOrgUnitMutationCapabilities(...)`
   - `getOrgUnitFieldOptions(...)`
   - 更新 `getOrgUnitDetails(...)` 类型以包含 `ext_fields` + `display_value_source`
2. [ ] 详情页展示：在 `apps/web-mui/src/pages/org/OrgUnitDetailsPage.tsx` profile 区新增 ext_fields 展示区块（与既有两栏布局一致）。
3. [ ] 更正弹窗改造：
   - 引入 capabilities fetch（按 `effective_date`）。
   - 动态渲染扩展字段表单项。
   - 按 `allowed_fields/enabled/deny_reasons` 控制字段与确认按钮（fail-closed）。
4. [ ] i18n：
   - [ ] 增加扩展字段 label 的 i18n key（en/zh 同步，SSOT：`DEV-PLAN-020`）。
   - [ ] deny reason 的展示策略冻结（可先展示 reason code，后续逐步补齐映射）。
5. [ ] 测试：
   - [ ] 前端单测：capabilities 不可用时 fail-closed；allowed_fields 控制输入禁用；DICT options 错误态可解释。
   - [ ] 前端单测：`display_value_source` 的渲染分支（`versions_snapshot/dict_fallback/unresolved`）可解释且稳定。
   - [ ] 前端单测：DICT/ENTITY clear 动作会提交 `patch.ext[field_key]=null`（显式清空）。
   - [ ] E2E（若命中 TP-060 相关场景）：至少覆盖“字段可见 -> 可更正 -> 保存成功 -> 详情回显”一条路径。

## 9. 测试与验收标准 (Acceptance Criteria)

- [ ] 详情页能展示扩展字段（至少 1 个 DICT 字段），并随 `effective_date` 切换正确刷新。
- [ ] `orgunit.admin` 在“更正”弹窗中：
  - [ ] capabilities 返回 enabled 时：allowed_fields 内字段可编辑；非 allowed 字段禁用且原因可解释。
  - [ ] capabilities 返回 disabled 时：确认按钮禁用，且 deny_reasons 可见。
  - [ ] capabilities API 失败时：全表单只读/禁用（fail-closed），不允许提交。
- [ ] 更正支持修改“更正后生效日”：提交 `patch.effective_date` 成功后，UI 自动切换到新 `effective_date` 版本并刷新 details。
- [ ] 当进入“生效日更正模式”时：除 `patch.effective_date` 外，任何字段均不可编辑且不会进入 patch（避免隐式联动与 drift）。
- [ ] 非 `orgunit.admin` 用户可见“更正”入口但默认不可用，并能看到稳定 deny_reasons（避免“为什么不能改”不可解释）。
- [ ] DICT/ENTITY 字段 options 可搜索；options 失败时该字段不可编辑且有明确错误提示。
- [ ] 写入后刷新：成功后 details 的 ext_fields 回显新值（且不出现“看似成功但实际未生效”）。
- [ ] 提交更正时，HTTP 请求的 `patch` 只包含 **变更字段** 且严格受 `allowed_fields` 裁剪；不得出现“字段被禁用但仍随请求提交”。
- [ ] 已启用但当前无值的扩展字段在 details 中仍可见（`value=null`），并在编辑态可按 capabilities 允许进行赋值（避免僵尸字段）。
- [ ] DICT/ENTITY 字段支持显式清空：清空后提交 `patch.ext[field_key]=null` 并在详情中回显为空值。

## 10. 运维与监控 (Ops & Monitoring)

遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束：本计划不引入新开关与监控面板。必要的排障信息以 `deny_reasons` 与稳定错误码呈现即可。

## 11. 关联文档

- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- `docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/097-orgunit-details-drawer-to-page-migration.md`
- `docs/dev-plans/099-orgunit-details-two-pane-info-audit-mui.md`
- `docs/dev-plans/020-i18n-en-zh-only.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `AGENTS.md`
