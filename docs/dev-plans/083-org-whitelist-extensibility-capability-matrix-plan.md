# DEV-PLAN-083：Org 变更能力模型重构（抽象统一 + 策略单点 + 能力外显）

**状态**: 已完成（2026-02-15 23:02 UTC — 收口为 Rewrite/Invalidate（更正/撤销）capabilities + 策略单点；Append 扩展拆分为 DEV-PLAN-083A）

## 0. Stopline（本计划 SSOT 收口范围）

> 目标：本文件作为 Rewrite/Invalidate（`correct_* / rescind_*`）的 SSOT，冻结并落地 `mutation-capabilities` 的**返回结构 + 字段/路径映射 + 组合约束 + deny_reasons 闭集**；Append（`create / event_update`）的扩展口径见 `DEV-PLAN-083A`。

- [X] 冻结并实现 `action_kind/emitted_event_type/target_effective_event_type` 合法组合约束（见 §4.2；落地：`modules/orgunit/services/orgunit_mutation_policy.go`）。
- [X] 冻结并实现 `field_key/field_payload_keys/deny_reasons` 的对外语义（见 §4.3、§5.2、§5.6；落地：policy + API 合约测试）。
- [X] 冻结并实现 `GET /org/api/org-units/mutation-capabilities` 的 Response 200 字段结构（见 §5.2；落地：`internal/server/orgunit_mutation_capabilities_api.go`）。
- [X] 冻结并实现 `correct_event/correct_status/rescind_event/rescind_org` 的最小能力字段（见 §5.3~§5.5）。
- [X] 更正写入链路按 policy 做 fail-closed 校验，并支持 `patch.ext`（承接：`DEV-PLAN-100E1/100E`）。
- [X] Append（create/event_update）策略单点与能力外显扩展：见 `docs/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`。

## 1. 背景

基于 `DEV-PLAN-082`，当前 OrgUnit 的“字段可改范围”虽符合业务语义，但实现上存在结构性问题：

1. 规则分散在 Service/UI/Kernel，难以维护且容易漂移。
2. UI 缺少可编辑能力先验，出现“可输入但提交后失败（`PATCH_FIELD_NOT_ALLOWED`）”。
3. 规则表达停留在“字段白名单”，尚未上升到“变更语义模型”，扩展时容易出现无效组合。

本计划以“抽象统一”作为第一目标：先统一变更语义与策略结构，再落地代码与 API。

## 2. 抽象模型（本计划 SSOT）

## 2.1 核心定义：历史变换语言（History Transformation DSL）

OrgUnit 写入不是“改字段”，而是“对历史事实做受限变换”。

为此，定义四层模型：

- `action_kind`（意图层，Intent）：用户在做什么动作。
- `emitted_event_type`（执行层，Opcode）：系统实际写入哪种事件。
- `field_key/db_payload_key`（参数层，Operands）：对外字段键与写入 Kernel 的 payload 键映射（见 §4.3）。
- `preconditions`（约束层，Guards）：字段白名单之外的前置业务约束。

## 2.2 三类原子变换（最小完备基）

- `Append`（追加事实）
  - 对应动作：`create`、`event_update`
  - 对应事件：`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`
- 本计划收口说明：Append 的策略单点与能力外显扩展在 `DEV-PLAN-083A`；本文仍保留“原子变换”框架作为全局语义背景，但不再在本文冻结/推进 Append 的实现细节。
- `Rewrite`（改写解释）
  - 对应动作：`correct_event`、`correct_status`
  - 对应事件：`CORRECT_EVENT/CORRECT_STATUS`
- `Invalidate`（失效既有事实）
  - 对应动作：`rescind_event`、`rescind_org`
  - 对应事件：`RESCIND_EVENT/RESCIND_ORG`

> 判定规则：仅当新需求不能归入 `Append/Rewrite/Invalidate` 时，才允许新增事件语义。

## 2.3 完备性边界

- 本模型覆盖 **OrgUnit 写入域**（create/update/correct/rescind）。
- 本模型不覆盖读取域（Read）与跨模块语义（person/staffing/jobcatalog）。

## 3. 统一词表与命名冻结

- `action_kind`（本计划范围）：`correct_event` / `correct_status` / `rescind_event` / `rescind_org`
- `emitted_event_type`（本计划范围）：`CORRECT_EVENT/CORRECT_STATUS/RESCIND_EVENT/RESCIND_ORG`
- `target_effective_event_type`：`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`（仅 `correct_*` / `rescind_*` 使用）

命名冻结规则：

- 服务层与内核事件枚举统一采用现有大写常量（例如 `CORRECT_STATUS`、`RESCIND_EVENT`）。
- 禁止引入并行小写事件常量，避免双词表。

## 4. 策略模型设计（Policy Matrix）

在 `modules/orgunit/services/` 新增策略模块（建议：`orgunit_mutation_policy.go`），并作为写入策略单点。

## 4.1 策略键（Policy Key）

统一键：

- `(action_kind, emitted_event_type, target_effective_event_type?)`

语义要求：

- `target_effective_event_type` 仅在需要目标事件的动作中出现（`correct_*` / `rescind_*`）。
- 对不合法组合（如 `create + target_effective_event_type!=null`）直接拒绝。

## 4.2 合法组合约束（无效组合 fail-closed）

- `correct_event -> emitted_event_type=CORRECT_EVENT + target_effective_event_type required`
- `correct_status -> emitted_event_type=CORRECT_STATUS + target_effective_event_type required`
- `rescind_event -> emitted_event_type=RESCIND_EVENT + target_effective_event_type required`
- `rescind_org -> emitted_event_type=RESCIND_ORG + target_effective_event_type must be null`

## 4.3 字段与 payload 映射规则（冻结）

本计划定义两层映射，避免 UI/Service/Kernel 出现“双词表”：

1. **field_key（对外稳定键）**：capabilities/API/UI 统一使用的字段标识（扩展字段也使用该口径）。  
2. **db_payload_key（服务层内部映射）**：写入 Kernel 时实际落入 event payload 的 key（客户端不得使用）。

### 4.3.1 field_key（冻结集合）

- Core（OrgUnit 业务字段，固定集合）：
  - `effective_date`
  - `name`
  - `parent_org_code`
  - `is_business_unit`
  - `manager_pernr`
- Ext（扩展字段，动态集合）：
  - 来自 `orgunit.tenant_field_configs.field_key`（SSOT：`DEV-PLAN-100A/100B/100C/100D`）
  - 在 capabilities 的 `allowed_fields` 中以**裸 `field_key`**出现（例如 `org_type`），不加 `ext.` 前缀。

### 4.3.2 field_payload_keys（capabilities 输出；写入 corrections patch 的路径）

`field_payload_keys[field_key]` 的值是 **dot-path 字符串**，用于指示“UI/调用方应把值写到 corrections 请求的 `patch` 的哪个路径”：

- Core：
  - `effective_date -> effective_date`
  - `name -> name`
  - `parent_org_code -> parent_org_code`
  - `is_business_unit -> is_business_unit`
  - `manager_pernr -> manager_pernr`
- Ext：
  - `<field_key> -> ext.<field_key>`（表示 `patch.ext[<field_key>]`）

约束（冻结）：

- 客户端**不得**提交 `new_name/new_parent_id/parent_id/manager_uuid` 等内部字段；这些属于 `db_payload_key`，由服务层根据目标事件类型映射生成。
- `allowed_fields` 的字段集合与 `field_payload_keys` 必须一致：`allowed_fields` 中出现的每个 `field_key`，在 `field_payload_keys` 中必须存在键；反之 `field_payload_keys` 不得出现 `allowed_fields` 之外的键。

### 4.3.3 db_payload_key（服务层内部映射；写入 Kernel 的 payload key）

> 说明：本节是服务层策略模块与 `buildCorrectionPatch(...)` 的实现依据；**不对 UI 暴露**。

- `effective_date`：
  - target=任意 -> `effective_date`
- `name`：
  - target=`CREATE` -> `name`
  - target=`RENAME` -> `new_name`
- `parent_org_code`：
  - target=`CREATE` -> `parent_id`
  - target=`MOVE` -> `new_parent_id`
- `is_business_unit`：
  - target=`CREATE|SET_BUSINESS_UNIT` -> `is_business_unit`
- `manager_pernr`：
  - target=`CREATE` -> `manager_uuid + manager_pernr`（含人员解析）
- Ext（扩展字段）：
  - target=允许携带 ext 的动作/事件组合 -> `payload.ext[<field_key>]`（DICT 额外写 `payload.ext_labels_snapshot[<field_key>]`；由服务层生成，UI 不提交）

默认规则（冻结）：

- 未显式声明的字段一律拒绝（fail-closed）。

## 4.4 前置约束（preconditions）

`preconditions` 用于表达非字段白名单约束，至少包含：

- `target_exists`
- `target_not_rescinded`
- `root_guard`
- `children_guard`
- `dependency_guard`
- `date_window_guard`
- `same_day_uniqueness_guard`

要求：

- capabilities API 需把不满足的约束转为 `deny_reasons`，供 UI 解释。
- 服务层与 Kernel 错误码保持稳定对齐。

## 4.5 统一策略接口

建议接口：

- `ResolvePolicy(actionKind, emittedEventType, targetEffectiveEventType) (PolicyDecision, error)`
- `AllowedFields(decision) []FieldRule`
- `ValidatePatch(decision, patch) error`

其中 `PolicyDecision` 至少包含：

- `enabled`
- `allowed_fields`
- `field_payload_keys`
- `preconditions`
- `deny_reasons`

## 5. 能力外显 API（Capabilities）

新增只读 API：

- `GET /org/api/org-units/mutation-capabilities?org_code=<...>&effective_date=<...>`

### 5.1 Authz 与失败策略（冻结）

- RouteClass：Internal API（对齐 `DEV-PLAN-017`）。
- Authz：
  - 无 `orgunit.read`：由全局 authz 中间件统一返回 403（SSOT：`DEV-PLAN-022`），不返回本接口 payload。
  - 有 `orgunit.read` 但无 `orgunit.admin`：接口仍返回 200，但所有“写动作”（`correct_*` / `rescind_*`）必须 `enabled=false`，并在 `deny_reasons` 中包含 `FORBIDDEN`（见 §5.6）。
- API 不可用/返回错误：UI 必须 fail-closed（只读/禁用），不做乐观放行（SSOT：`DEV-PLAN-100` D8、`DEV-PLAN-100E`）。

### 5.2 Response 200（冻结字段结构）

```json
{
  "org_code": "A001",
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

字段语义（冻结）：

- `org_code`：规范化后的 org_code（与现有 API 口径一致）。
- `effective_date`：调用方请求的目标生效日（`YYYY-MM-DD`）。
- `effective_target_event_type`：
  - 用于策略判定的目标事件类型；
  - 取值来自 `orgunit.org_events_effective` 视图中“该 `effective_date` 对应的有效事件类型”（例如 status correction 可能把 raw DISABLE 映射为 ENABLE）。
- `raw_target_event_type`：
  - 用于审计对照；
  - 取值来自原始事件（未被 `org_events_effective` 改写前）的事件类型。
- `capabilities`：见 §5.3~§5.5。

排序约束（冻结）：

- `allowed_fields` 必须按 `field_key` 升序排序（稳定输出，避免 UI 抖动/测试不稳定）。
- `deny_reasons` 必须按 §5.6 的优先级顺序输出（稳定输出）。

### 5.3 `capabilities.correct_event`（冻结）

用途：驱动 “更正（CORRECT_EVENT）” UI 的字段可编辑性与 payload 路径映射。

字段（冻结）：

- `enabled: boolean`
- `allowed_fields: string[]`（field_key）
- `field_payload_keys: object`（`field_key -> dot-path`；见 §4.3.2）
- `deny_reasons: string[]`（见 §5.6）

Core 字段允许矩阵（冻结；承接并收敛 `DEV-PLAN-082`，以本文为 SSOT）：

| effective_target_event_type | allowed_fields（core 子集） |
| --- | --- |
| `CREATE` | `effective_date`, `name`, `parent_org_code`, `is_business_unit`, `manager_pernr` |
| `RENAME` | `effective_date`, `name` |
| `MOVE` | `effective_date`, `parent_org_code` |
| `SET_BUSINESS_UNIT` | `effective_date`, `is_business_unit` |
| `DISABLE` / `ENABLE` | `effective_date` |

扩展字段（Ext）合并规则（冻结）：

- 对于当前租户在 `effective_date` 下 enabled 的扩展字段集合 `E`（SSOT：`DEV-PLAN-100D` 元数据解析），将 `E` **并入** `allowed_fields`（与 core 字段同一数组、统一排序）。  
- `field_payload_keys` 对扩展字段必须返回 `ext.<field_key>`（见 §4.3.2）。

> 说明：是否“enabled 的扩展字段一律可更正”是本计划冻结的策略选择；若未来需按动作/事件进一步收紧（例如仅允许 target=CREATE），必须先更新本文并同步更新 Kernel allow-matrix/服务层策略与测试。

### 5.4 `capabilities.correct_status`（冻结）

用途：驱动 “同日状态纠错（CORRECT_STATUS）” UI 的可用性与可选状态列表。

字段（冻结）：

- `enabled: boolean`
- `allowed_target_statuses: string[]`（规范化值，仅允许 `active|disabled`）
- `deny_reasons: string[]`（见 §5.6）

规则（冻结）：

- 当且仅当 `effective_target_event_type in {ENABLE, DISABLE}` 时，`enabled=true` 且 `allowed_target_statuses=["active","disabled"]`。
- 否则 `enabled=false` 且 `deny_reasons` 必须包含 `ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET`。

### 5.5 `capabilities.rescind_event` / `capabilities.rescind_org`（冻结）

字段（冻结）：

- `enabled: boolean`
- `deny_reasons: string[]`（见 §5.6）

说明（冻结）：

- `rescind_event` 表示“撤销单条记录”（`RESCIND_EVENT`）；`rescind_org` 表示“撤销整个组织”（`RESCIND_ORG`）。
- `enabled=false` 时，`deny_reasons` 必须可解释且稳定（例如根组织删除保护、存在子组织、存在下游依赖等；错误码以现有 Kernel/服务层稳定码为准，见 §5.6）。

### 5.6 `deny_reasons`（冻结闭集 + 顺序）

`deny_reasons[]` 是**稳定错误码/原因码**列表（仅 code，不返回自由文本），用于 UI 解释“为何不可用”：

- 取值闭集（冻结；新增 reason code 必须先更新本文）：
  - `FORBIDDEN`（缺少 `orgunit.admin` 等写权限；capabilities API 本身仍需 `orgunit.read`）
  - `ORG_EVENT_NOT_FOUND`
  - `ORG_EVENT_RESCINDED`
  - `ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET`
  - `ORG_ROOT_DELETE_FORBIDDEN`
  - `ORG_HAS_CHILDREN_CANNOT_DELETE`
  - `ORG_HAS_DEPENDENCIES_CANNOT_DELETE`

顺序规则（冻结）：

- `deny_reasons` 必须去重；
- 按以下优先级顺序输出（存在则前置）：  
  1) `FORBIDDEN`  
  2) `ORG_EVENT_NOT_FOUND`  
  3) `ORG_EVENT_RESCINDED`  
  4) `ORG_ROOT_DELETE_FORBIDDEN`  
  5) `ORG_HAS_CHILDREN_CANNOT_DELETE`  
  6) `ORG_HAS_DEPENDENCIES_CANNOT_DELETE`  
  7) `ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET`

### 5.7 错误响应（最小冻结）

- `ORG_INVALID_ARGUMENT` / `invalid_request`：400（`org_code/effective_date` 缺失或格式非法）
- `ORG_EVENT_NOT_FOUND`：404（目标生效日不存在或对调用方不可见）
- `ORG_EVENT_RESCINDED`：409（目标已撤销；仅当实现侧可区分于 NOT_FOUND 时返回）

## 6. 实施步骤

## 6.1 文档与契约先行
1. [X] 固化本文件为策略模型 SSOT（四层模型 + 三类原子变换 + 合法组合约束；并冻结 §5 契约）。
2. [X] 明确 capabilities 响应字段与最小 deny_reasons 闭集（供 `DEV-PLAN-100D/100E` 消费）。

## 6.2 服务层改造

> 说明：`DEV-PLAN-100E1` 已作为 `DEV-PLAN-100E` 的前置改造计划拆出，用于落实“策略单点 + capabilities 对齐 + corrections 支持 `patch.ext`”；本文继续作为能力模型与 capabilities 契约 SSOT。

3. [X] 新增 `orgunit_mutation_policy.go`，实现 `ResolvePolicy/AllowedFields/ValidatePatch`（落地：`DEV-PLAN-100E1`）。
4. [X] 更正写入链路按 policy 做 fail-closed 校验，并支持 `patch.ext`（落地：`DEV-PLAN-100E1`）。
5. [X] capabilities API 复用 policy，输出稳定排序与 deny reasons 闭集（落地：`DEV-PLAN-100E1`）。
6. [X] 保持行为等价或更严格 fail-closed；不引入新业务语义（落地：`DEV-PLAN-100E1/100E`）。

## 6.3 API 与 UI 联动
7. [X] 新增 mutation capabilities API（含 `deny_reasons`）（落地：`internal/server/orgunit_mutation_capabilities_api.go`）。
8. [X] 详情页按 capabilities 执行字段禁用/隐藏与动作可用性控制（落地：`DEV-PLAN-100E`）。
9. [X] 错误提示升级为“不可用原因可解释”，减少提交后失败（落地：`DEV-PLAN-100E`）。

## 6.4 Kernel 对齐与防回归
10. [ ] 复核 `submit_org_event_correction/submit_org_status_correction/submit_org_event_rescind/submit_org_rescind` 与服务层规则对齐。
11. [ ] 保留 Kernel 防守性校验，确保绕过服务层时仍 fail-closed。

## 6.5 开放问题与建议（目标：彻底实现）

> 说明：本节用于把“契约已冻结但实现易漂移”的点显式化；具体执行拆分与落地步骤以 `DEV-PLAN-100E1` 为实施 SSOT。

### Q1：扩展字段集合 `E` 的合并规则要去漂移（禁止按 target=CREATE 特判）

- 风险：capabilities 对 ext 的合并若依赖 `effective_target_event_type=CREATE` 等特判，会与 §5.3 冻结冲突，产生“可见不可写/可写不可见”与安全漂移。
- 建议：
  - `E` 的计算仅依赖 `(tenant_uuid, effective_date)` 的 enabled-as-of（`[enabled_on, disabled_on)` day 粒度，SSOT：`DEV-PLAN-100A`）。
  - 在 policy 的 `AllowedFields(...)` 内把 `E` 并入 `correct_event.allowed_fields`（对所有 `effective_target_event_type` 生效），并生成 `field_payload_keys[field_key]="ext.<field_key>"`。
- 验收/测试：对 `effective_target_event_type in {CREATE,RENAME,MOVE,DISABLE,ENABLE,SET_BUSINESS_UNIT}`，只要存在 enabled 的 ext 字段，该 field_key 必须出现在 `allowed_fields` 且 `field_payload_keys` 里（稳定排序）。

### Q2：Policy 必须是“facts in / decision out”的纯模块，并被 capabilities 与写入双端复用

- 风险：若 deny reasons/allowed fields/映射散落在 `internal/server` 与 `modules/orgunit/services` 两套实现中，必然漂移。
- 建议：
  - policy 只接收已解析的事实输入（例如：`can_admin/target_effective_event_type/root/has_children/has_dependencies/enabled_ext_keys`），输出 `enabled/allowed_fields/field_payload_keys/deny_reasons`；
  - `deny_reasons` 严格限制在 §5.6 闭集，并按优先级稳定排序；实现层新增“闭集断言”测试（未知 code 直接 fail）。

### Q3：`buildCorrectionPatch(...)` 不再硬编码矩阵；只做“值转换”，字段/路径由 policy 决定

- 风险：服务层继续以 `if eventType==...` 的分支来决定“字段可改 + db_payload_key”，会再次与 capabilities 分叉。
- 建议：
  - policy 输出（或可推导）`field_key -> db_payload_key` 映射；`buildCorrectionPatch` 仅负责把 API patch 值转换为 Kernel patch（例如 `parent_org_code -> parent_id/new_parent_id`、`manager_pernr -> manager_uuid+manager_pernr`），并把“是否允许携带该字段”交给 `ValidatePatch(...)`。

### Q4：更正链路的 fail-closed 必须覆盖“解析层”（未知字段/`ext_labels_snapshot`）

- 风险：若 handler 允许未知字段被静默忽略，客户端可偷偷提交不被 policy 覆盖的字段，造成“表面通过、实际丢字段/绕过防线”的灰区。
- 建议：更正请求 JSON 解码启用严格模式（`DisallowUnknownFields`）；并显式拒绝客户端提交 `patch.ext_labels_snapshot`（服务端生成 DICT label 快照，SSOT：`DEV-PLAN-100D`；落地：`DEV-PLAN-100E1`）。

### Q5：Kernel/Service 的 ext 防线对齐要用“反向用例”锁住

- 风险：capabilities 放行但 Kernel 因 `ORG_EXT_*` 错误拒绝，会导致 UI 进入“可编辑但必失败”。
- 建议：以服务层集成测试覆盖典型负例（字段未配置/未启用/类型不匹配/DICT 缺 label snapshot/不允许的组合携带 ext），确保 policy 与服务端补齐逻辑能在进入 Kernel 前 fail-closed 并返回稳定错误码。

## 7. 验收标准

- [ ] 规则判定从散落分支收敛为策略单点，可审计可测试。
- [ ] capabilities API 能返回“目标事件类型 + 动作能力 + 字段映射 + 拒绝原因”。
- [ ] UI 常见路径不再出现“可编辑但必然失败”的字段。
- [ ] UI 对禁用动作给出稳定可解释原因。
- [ ] `PATCH_FIELD_NOT_ALLOWED`、`ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET` 等稳定错误码不漂移。
- [ ] 无 legacy 分支，无第二写入口（One Door 保持）。

## 8. 测试与门禁

### 8.1 本地必跑（触发器命中）
- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 文档：`make check doc`

### 8.2 推荐增量测试
- `modules/orgunit/services/orgunit_mutation_policy_test.go`
  - 合法组合覆盖
  - 无效组合拒绝（table-driven）
  - `field_key -> db_payload_key` / `field_payload_keys` 映射覆盖
- `modules/orgunit/services/orgunit_write_service_test.go`
  - `buildCorrectionPatch` 与策略集成回归
- `internal/server/orgunit_api_test.go`
  - capabilities API 合约测试（含 deny reason）
- `internal/server/orgunit_nodes_test.go`
  - UI 禁用/提示回归

## 9. 风险与权衡

- 风险：抽象层级提升后，短期理解成本上升。  
  缓解：先在 OrgUnit 写入域收口，不跨模块泛化。

- 风险：动作语义与事件语义再次混用，回到“可表达无效组合”。  
  缓解：使用统一策略键并加启动期自检（非法组合即失败）。

- 风险：UI 过度依赖 capabilities API。  
  缓解：API 故障时 fail-closed（只读/禁用），不做隐式放行。

- 风险：服务层与 Kernel 边界口径偏移。  
  缓解：契约测试固定错误码和边界输入，并在评审中同时检查 Service/Kernel。

## 10. 与既有计划关系

- 语义事实源：`docs/dev-plans/082-org-module-field-mutation-rules-investigation.md`
- 状态纠错边界：`docs/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
- correction 失败排障与错误码稳定化：`docs/dev-plans/080b-orgunit-correction-failure-investigation-and-remediation.md`
- 审计快照约束：`docs/dev-plans/080c-orgunit-audit-snapshot-presence-table-constraint-plan.md`

## 11. 关联文档

- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`
- `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- `AGENTS.md`
