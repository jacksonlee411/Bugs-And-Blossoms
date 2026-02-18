# DEV-PLAN-083A：OrgUnit Append 写入动作能力外显与策略单点扩展（create / event_update）

**状态**: 已完成（2026-02-16 03:44 UTC — 后端策略/API/写入收敛 + 前端 create/append capabilities 全接入 + 测试与门禁通过）；2026-02-18 起由 `DEV-PLAN-108` 统一写入口取代为主口径

## 0.3 与 DEV-PLAN-108 的关系（2026-02-18 补充）

本计划冻结的 append 形态是“动作型 endpoint + append-capabilities”，且明确“不引入 UPDATE”。
该口径在 108 生效后变为历史实现（仍可兼容保留），新的主口径为：

- append intent（add_version/insert_version）统一走 `POST /org/api/org-units/write` 并落 `UPDATE` 单事件多字段；
- capabilities 迁移到 `GET /org/api/org-units/write-capabilities?intent=...`；
- `append-capabilities` 若保留，应改为 `write-capabilities` 的薄包装，避免双规则漂移。

> 本计划是 `DEV-PLAN-083` 的扩展篇：在 `083/100E1/100E` 已完成“Rewrite/Invalidate（更正/撤销）capabilities + 策略单点 + patch.ext”之后，继续把 **Append（create / event_update）** 纳入同一套能力模型与策略单点，避免 UI/Service/Kernel 继续出现“散落规则 + 漂移”。

> 实施结果（本次收口）：
> - 后端：Append policy 单点、`append-capabilities` API、Append 写入口（含 set-business-unit）全部走统一校验与 fail-closed。
> - 前端：OrgUnits（create）+ OrgUnitDetails（rename/move/enable/disable/set BU）全部按 capabilities 控制动作和字段可编辑性，并按 `field_payload_keys` 组装请求。
> - 测试：policy/API/write-service/前端 append intent 均补充测试；相关 lint/typecheck/test/build 通过。

## 0. Stopline（本计划明确不做）

- 不新增/变更 DB schema、迁移、sqlc（如需新增表/列必须另起 dev-plan 且先获用户手工确认）。
- 不新增事件语义（仍只使用既有事件类型：`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`；不得引入并行小写枚举）。
- 不引入 legacy/双链路；写入仍必须走 DB Kernel 的 `submit_*`（One Door，见 `AGENTS.md`、`DEV-PLAN-026/100C`）。

## 0.1 “彻底收敛”在本计划中的定义（目标口径）

> 说明：这里的“彻底”指 **应用层（UI/Service）不再散落规则**，并且 **capabilities 与写入校验共享同一套 policy**；Kernel 仍保留防守性校验，但应用层不得再复制/分叉规则。

- 单一策略入口：Append 的“动作可用性 / 字段可写范围 / 字段→payload 映射 / deny_reasons”必须由 `modules/orgunit/services/orgunit_mutation_policy.go` 单点输出。
- UI 只信 capabilities：UI 不允许维护“字段白名单/动作可用性”的并行 if-else；capabilities 缺失/失败时必须 fail-closed（禁用/只读）。
- 写入双端一致：所有 Append 写入口（create/rename/move/disable/enable/set_business_unit）必须在进入 Kernel 前调用 policy 做 fail-closed 校验；禁止在 handler/service 里再写一套“允许字段矩阵”。
- Kernel 是最后防线：Kernel 仍可拒绝（例如 cycle/missing-as-of/高风险重排等）；但应用层必须保证“常规原因可解释 + 最小化‘capabilities 放行但必失败’”。

## 0.2 管理员视角：我到底能“配置”什么？不能配置什么？

> 这段用“管理员用户语言”把困惑点说清楚：字段是否可编辑，最终来自两类规则叠加 —— **系统固定规则（core）** + **管理员可配置扩展字段（ext）**。

管理员“能配置”的只有两件事：

1. **启用/停用 ext 字段**（“字段配置”页面/接口）：  
   - 你可以把某个扩展字段（例如 `org_type`）在某个生效日之后启用；从那一天起，UI 表单里就会出现它，并且在本计划口径下它会被并入各个 Append 动作的 `allowed_fields`。  
   - 这属于“字段出现/是否可填写”的**配置能力**。
2. **权限（Casbin）**：  
   - 没有写权限时，capabilities 会返回 `enabled=false + deny_reasons=["FORBIDDEN"]`，UI 只读/禁用提交。

管理员“不能配置”的是 core 字段规则本身：

- `name` 能不能改、`parent_org_code` 能不能改、`is_business_unit` 能不能改 —— **不是管理员可配置项**，而是系统对不同事件语义的固定约束：  
  - 例如 `RENAME` 只能改 `name`，`MOVE` 只能改 `parent_org_code`。  
  - 这些规则会以“固定矩阵”的形式存在于 policy 中，并由本文件作为 SSOT 维护（见 §4.2 与 §4.2.3）。

## 1. 背景

当前仓库已将 OrgUnit 的 **Rewrite/Invalidate**（`correct_* / rescind_*`）收敛为：

- 策略单点（`modules/orgunit/services/orgunit_mutation_policy.go`）
- `GET /org/api/org-units/mutation-capabilities`（capabilities 外显）
- corrections 支持 `patch.ext` 且 fail-closed（`DEV-PLAN-100E1/100E`）

但 **Append**（`create / event_update`）仍主要依赖“各写入 endpoint 自己的 request 校验 + 内核防守性错误”，存在以下问题：

1. UI 无法在提交前获得稳定、可解释的“为何不可用/为何字段不可填”信息（尤其是 root guard、依赖/子节点约束等）。
2. Service 层对字段可写/前置约束仍可能散落在多个 handler/service 中，易与 capabilities/Kernel 漂移。
3. 扩展字段（ext）在“追加事实”动作中的接入缺少统一策略口径（哪些动作允许携带 ext、如何 fail-closed）。

## 2. 目标（Goals）

1. 将 `create / event_update` 纳入策略单点（与 `DEV-PLAN-083` 同一抽象框架），做到“facts in / decision out”。
2. 为 Append 动作提供 capabilities 外显（最小可用即可），使 UI 能 fail-closed 且可解释。
3. 收敛字段键口径：对外统一 `field_key`，内部映射到 Kernel payload（禁止 UI 透传内部字段）。
4. 不改变现有业务语义（行为等价或更严格 fail-closed），不引入第二写入口。

## 3. 范围（Scope）

### 3.1 覆盖的动作

- `create` → emitted `CREATE`
- `event_update` → emitted in `{MOVE, RENAME, DISABLE, ENABLE, SET_BUSINESS_UNIT}`

### 3.2 不在本计划覆盖的动作

- `correct_* / rescind_*`：SSOT 仍在 `DEV-PLAN-083`，且已由 `DEV-PLAN-100E1/100E` 落地。

## 4. 契约与设计（Contract & Design）

### 4.1 统一词表（本计划冻结）

- `action_kind`：`create` / `event_update`
- `emitted_event_type`：
  - `create`：`CREATE`
  - `event_update`：`MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`

> 说明：本计划不新增事件类型；若新增需求无法归入既有事件语义，必须先更新 `DEV-PLAN-083` 的“原子变换”定义并另立 dev-plan。

### 4.2 字段口径（复用 083 的 field_key；本计划补齐 Append 的矩阵 + payload 映射）

复用 `DEV-PLAN-083` 的 `field_key` 定义（core + ext），并新增 Append 的“动作→允许字段”矩阵（冻结）：

- `create`（CREATE）：
  - core：`org_code`, `effective_date`, `name`, `parent_org_code`, `is_business_unit`, `manager_pernr`
  - ext：按 `(tenant_uuid, effective_date)` enabled-as-of 的集合 `E` 并入（与 083 的合并规则一致）
- `event_update`：
  - `RENAME`：`effective_date`, `name` + `E`
  - `MOVE`：`effective_date`, `parent_org_code` + `E`
  - `SET_BUSINESS_UNIT`：`effective_date`, `is_business_unit` + `E`
  - `DISABLE/ENABLE`：`effective_date` + `E`

> 注：将 `E` 并入所有 target 的策略是 `DEV-PLAN-083` 已冻结选择；若未来要收紧，必须先更新 SSOT（禁止代码先改）。

#### 4.2.1 field_payload_keys（冻结；用于 UI 构造请求，不允许 UI 透传 Kernel 内部字段）

Append 的 `field_payload_keys[field_key]` 用 **dot-path 字符串**表达“该 field_key 在请求 JSON 中应写到哪个路径”。本计划冻结为“与既有 API request shape 兼容，且允许扩展 ext”：

- `create`（`POST /org/api/org-units`）：
  - `org_code -> org_code`
  - `effective_date -> effective_date`
  - `name -> name`
  - `parent_org_code -> parent_org_code`
  - `is_business_unit -> is_business_unit`
  - `manager_pernr -> manager_pernr`
  - ext：`<field_key> -> ext.<field_key>`（即请求体新增 `ext`；服务端生成 `ext_labels_snapshot`，客户端不得提交）
- `event_update`（现有 endpoint 继续存在，但统一以 field_key 驱动 UI）：
  - `RENAME`（`POST /org/api/org-units/rename`）：`name -> new_name`；`effective_date -> effective_date`；ext 同上
  - `MOVE`（`POST /org/api/org-units/move`）：`parent_org_code -> new_parent_org_code`；`effective_date -> effective_date`；ext 同上
  - `DISABLE/ENABLE`（`POST /org/api/org-units/disable|enable`）：仅 `effective_date -> effective_date`；ext 同上
  - `SET_BUSINESS_UNIT`（`POST /org/api/org-units/set-business-unit`）：`is_business_unit -> is_business_unit`；`effective_date -> effective_date`；ext 同上

约束（冻结）：

- `allowed_fields` 与 `field_payload_keys` 必须一致：`allowed_fields` 中出现的每个 `field_key`，在 `field_payload_keys` 中必须存在键；反之不得出现多余键。
- 客户端不得提交 `ext_labels_snapshot`；服务端生成（对齐 `DEV-PLAN-100E1/100D`）。

#### 4.2.2 reserved field_key（冻结；防止 ext 与 core/系统字段冲突）

为避免管理员配置的 ext 字段与 core 字段/系统字段发生同名冲突，冻结以下约束（任一命中即 fail-closed，拒绝启用/拒绝合并）：

- 禁止 ext field_key 与 core 字段同名：`org_code/effective_date/name/parent_org_code/is_business_unit/manager_pernr`
- 禁止 ext field_key 使用系统保留名：`ext/ext_labels_snapshot`（以及未来在 request/patch 中出现的保留顶层字段）

落地建议（冻结）：

- 首选：在启用 field-config（`POST /org/api/org-units/field-configs`）时拒绝（更早反馈给管理员）。
- 兜底：policy 合并 `enabled_ext_field_keys` 时若发现冲突 key，直接将该 key 视为不可用并返回稳定 deny reason（或直接从 allowed_fields 剔除并记录错误日志）；不得静默把 ext 覆盖到 core 上。

#### 4.2.3 固定矩阵从哪里来？如何维护？如何更新？

结论（冻结）：

- Append 的 **core 字段矩阵**是系统固定业务规则（写在 policy 表中），不是管理员配置项；其来源是“事件语义 + 既有写入 endpoint 的请求形状 + Kernel 允许的原子变换”。  
- 只有 **ext 字段集合 `E`** 是管理员可通过“字段配置”来控制的（enabled-as-of 合并进 `allowed_fields`）。

维护/更新流程（冻结；也是未来 code review 的检查清单）：

1. **先改 SSOT**：只要想改矩阵（例如 `SET_BUSINESS_UNIT` 是否允许携带 ext），必须先改本文件（并写清楚原因与边界），禁止“代码先行”。  
2. **再改契约测试**：更新/新增 contract tests（policy/API golden），让 CI 能阻断漂移。  
3. **最后改实现**：同步修改 `modules/orgunit/services/orgunit_mutation_policy.go`（以及受影响的 handler/service），确保 capabilities 与写入校验仍复用同一 policy。  
4. **门禁**：至少跑 `go fmt ./... && go vet ./... && make check lint && make test`；若新增路由还需 `make check routing`（对齐 `AGENTS.md`/`DEV-PLAN-012/017`）。

### 4.3 policy facts（“facts in / decision out”冻结输入；避免 handler/service 再造规则）

为实现“彻底收敛”，Append 的 policy 必须只依赖可审计的事实输入；这些 facts 的获取也必须单点化（由 store 提供查询，避免到处 SQL/if-else）。

冻结的最小 facts（建议结构；以实现为准，但字段语义不得漂移）：

- Authz facts：
  - `can_admin`（缺少写权限则 `enabled=false` 且 `deny_reasons` 包含 `FORBIDDEN`）
- 元数据 facts：
  - `enabled_ext_field_keys`（按 `(tenant_uuid, effective_date)` enabled-as-of 计算的集合 `E`；并入 allowed_fields）
- 目标组织 facts（对 event_update 必需；对 create 可选）：
  - `target_exists_as_of`（组织在该 effective_date 是否存在/可见）
  - `target_status_as_of`（`active|disabled`；用于 enable/disable 的可用性）
  - `is_root`（用于 root guard：MOVE 禁止；SET_BUSINESS_UNIT 受约束）
  - `tree_initialized`（用于 “ORG_TREE_NOT_INITIALIZED” 的可解释拒绝）

> 说明：复杂边界（例如 cycle move、validity gap、高风险重排）可先作为“Kernel-only”拒绝（见 §4.4.3）；但常规约束必须尽量在 facts 里可计算，以减少“capabilities 放行但必失败”。

### 4.4 deny_reasons（Append 专用闭集 + 稳定顺序；不再强制复用 083 的闭集）

Append capabilities 的 `deny_reasons[]` 是稳定原因码列表（仅 code，不返回自由文本），用于 UI 解释“为何该动作不可用”。本计划冻结一个 **Append 专用闭集**（允许包含 Kernel 现有稳定码；不要求与 083 完全同一闭集）：

#### 4.4.1 deny_reasons 取值闭集（冻结）

- `FORBIDDEN`（缺少 `orgunit.admin` 等写权限；capabilities API 本身仍需 `orgunit.read`）
- `ORG_TREE_NOT_INITIALIZED`（组织树未初始化，Append 动作不可用）
- `ORG_NOT_FOUND_AS_OF`（目标组织在该 effective_date 不存在/不可见；event_update 专用）
- `ORG_ROOT_CANNOT_BE_MOVED`（root guard：MOVE 禁止）
- `ORG_ALREADY_EXISTS`（create 专用：创建目标已存在）
- `ORG_ROOT_ALREADY_EXISTS`（create 专用：尝试创建 root 且 root 已存在）

> 注：以上 reason code 均来自 Kernel 现有 MESSAGE（或服务层/HTTP 层既有稳定码）。若后续发现必须新增 reason code，必须先更新本文件再改代码与测试（禁止“代码先行”）。

#### 4.4.2 输出顺序规则（冻结）

- `deny_reasons` 必须去重；
- 按以下优先级稳定输出（存在则前置）：  
  1) `FORBIDDEN`  
  2) `ORG_TREE_NOT_INITIALIZED`  
  3) `ORG_NOT_FOUND_AS_OF`  
  4) `ORG_ROOT_CANNOT_BE_MOVED`  
  5) `ORG_ALREADY_EXISTS`
  6) `ORG_ROOT_ALREADY_EXISTS`

#### 4.4.3 Kernel-only 失败（允许存在，但必须最小化）

以下错误更适合由 Kernel 最后防线拒绝（capabilities 不强制完全覆盖），但 UI 必须能把错误码直接展示出来，且实现侧应逐步把“常见场景”前移到 facts/deny_reasons：

- `ORG_PARENT_NOT_FOUND_AS_OF`（parent 不存在于该 effective_date）
- `ORG_CYCLE_MOVE`（cycle move）
- `ORG_VALIDITY_GAP` / `ORG_VALIDITY_NOT_INFINITE`
- `ORG_HIGH_RISK_REORDER_FORBIDDEN`

#### 4.4.4 值相关约束（不纳入 deny_reasons；由 UI 约束 + Kernel 兜底）

本计划的 capabilities 主要表达“哪些字段能编辑”，而不是“某字段取某个值是否允许”。因此以下“值相关约束”不进入 deny_reasons 闭集（否则 capabilities 必须携带候选值上下文，复杂度与漂移面会显著增加）：

- root 必须保持 business unit：当管理员尝试把 root 的 `is_business_unit` 设为 false 时，Kernel 会拒绝（例如 `ORG_ROOT_BUSINESS_UNIT_REQUIRED`）。

要求（冻结）：

- UI 若能在本地判断（例如 root 详情页显示明确），可做前置禁用/提示；但不得替代 Kernel 最终判定。
- 服务端必须把 Kernel 稳定错误码透传为稳定 `code`（按既有 `writeOrgUnitServiceError(...)` 口径），避免出现“吞错/改写为无意义 message”。

### 4.5 Append Capabilities API（本计划冻结：新增独立 endpoint，避免与 mutation-capabilities 混杂）

为避免与既有 `mutation-capabilities`（Rewrite/Invalidate）契约混杂，本计划冻结新增只读 API（Internal API，RouteClass 对齐 `DEV-PLAN-017`）：

- `GET /org/api/org-units/append-capabilities?org_code=<...>&effective_date=<...>`

请求参数（冻结）：

- `org_code`：目标组织 code（对 `create` 表示“要创建的 org_code”，对 `event_update` 表示“要操作的 org_code”）；**必填**。
- `effective_date`：目标生效日（day 粒度，`YYYY-MM-DD`）；**必填**。

> 说明：本 endpoint 不是“查 org_code 是否存在”的单一用途，它要返回“如果你现在要 create / rename / move / …，哪些字段可填、哪些动作可用、为什么不可用”。因此 `org_code` 必须输入，才能做到可解释（例如 `ORG_ALREADY_EXISTS`）。

Response 200（冻结最小字段结构）：

```json
{
  "org_code": "A001",
  "effective_date": "2026-02-13",
  "capabilities": {
    "create": {
      "enabled": true,
      "allowed_fields": ["effective_date","is_business_unit","manager_pernr","name","org_code","parent_org_code","org_type"],
      "field_payload_keys": {
        "effective_date": "effective_date",
        "is_business_unit": "is_business_unit",
        "manager_pernr": "manager_pernr",
        "name": "name",
        "org_code": "org_code",
        "parent_org_code": "parent_org_code",
        "org_type": "ext.org_type"
      },
      "deny_reasons": []
    },
    "event_update": {
      "RENAME": {
        "enabled": true,
        "allowed_fields": ["effective_date","name","org_type"],
        "field_payload_keys": {
          "effective_date": "effective_date",
          "name": "new_name",
          "org_type": "ext.org_type"
        },
        "deny_reasons": []
      },
      "MOVE": {
        "enabled": false,
        "allowed_fields": [],
        "field_payload_keys": {},
        "deny_reasons": ["FORBIDDEN"]
      },
      "DISABLE": {
        "enabled": true,
        "allowed_fields": ["effective_date","org_type"],
        "field_payload_keys": {
          "effective_date": "effective_date",
          "org_type": "ext.org_type"
        },
        "deny_reasons": []
      },
      "ENABLE": {
        "enabled": true,
        "allowed_fields": ["effective_date","org_type"],
        "field_payload_keys": {
          "effective_date": "effective_date",
          "org_type": "ext.org_type"
        },
        "deny_reasons": []
      },
      "SET_BUSINESS_UNIT": {
        "enabled": true,
        "allowed_fields": ["effective_date","is_business_unit","org_type"],
        "field_payload_keys": {
          "effective_date": "effective_date",
          "is_business_unit": "is_business_unit",
          "org_type": "ext.org_type"
        },
        "deny_reasons": []
      }
    }
  }
}
```

排序约束（冻结）：

- `allowed_fields` 必须按 `field_key` 升序排序（稳定输出，避免 UI 抖动/测试不稳定）。
- `deny_reasons` 必须按 §4.4.2 的优先级顺序输出（稳定输出）。

错误响应（冻结最小口径）：

- 参数非法：400（`invalid_request`）
- `org_code` 无法解析：400（`org_code_invalid` 或 `invalid_request`，以实现为准，但必须稳定）。

> 约束：对于 “event_update 的 target 不存在于该 effective_date（ORG_NOT_FOUND_AS_OF）” 这种情况，本 endpoint **倾向返回 200**，并在对应 capability 的 `enabled=false + deny_reasons=["ORG_NOT_FOUND_AS_OF"]` 体现；避免 UI 把“不可用”误认为“系统错误”。

### 4.6 写入侧请求解析（冻结：严格解码 + ext 支持，避免静默忽略）

为实现“彻底收敛（fail-closed）”，Append 写入口（create/rename/move/disable/enable/set_business_unit）必须满足：

- JSON 解码启用严格模式：`DisallowUnknownFields`（与 corrections 口径一致，避免未知字段被静默忽略）。
- 显式声明并接收 `ext`（object），并显式声明 `ext_labels_snapshot` 字段用于拒绝客户端提交（服务端生成）。
- 若客户端提交 `ext_labels_snapshot`：返回 400（建议沿用 `PATCH_FIELD_NOT_ALLOWED` 或对齐本模块既有稳定码，最终以实现为准，但必须稳定）。

## 5. 实施步骤（Execution Plan）

> 说明：本节按“能直接开工”的粒度拆解到文件/接口级别，并给出建议的分阶段落地顺序（每阶段都能独立验收并合入 main）。

### 5.1 契约先行（Contract First）

1. [X] 冻结 Append capabilities 的 endpoint/请求参数/响应字段（本文件 §4.5），并明确“200 vs error”的边界：仅参数非法/格式非法返回 400；其余尽量以 `enabled=false + deny_reasons` 表达不可用。
2. [X] 回写 `DEV-PLAN-083`：Append 相关内容仅保留“原子变换语义背景”，实现与契约统一引用本文件（避免双 SSOT）。
3. [X] 将本文件中的示例响应固化为 contract tests 的断言（稳定排序 + 字段一致性），避免未来 UI/后端互相猜测。

建议 PR-0（仅文档/测试骨架，不改生产逻辑）：

- `docs/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`：冻结本计划口径（本文件）。
- 新增测试用例占位（可以先只断言排序/闭集，不必一次性覆盖所有分支）：
  - `modules/orgunit/services/orgunit_mutation_policy_test.go`
  - `internal/server/orgunit_append_capabilities_api_test.go`（若 API 阶段暂未实现，可先写为 TODO/skipped）

### 5.2 策略单点扩展（services）

4. [X] 扩展 `modules/orgunit/services/orgunit_mutation_policy.go`（Append 逻辑进入同一 policy）
   - 增加 Append 的 `action_kind` 与 `emitted_event_type`（建议复用既有 key 结构：Append 不使用 `target_effective_event_type`，保持 nil）。
   - 为每个 emitted event type（`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`）输出：
     - `allowed_fields`（含 ext 集合 `E` 合并，且稳定排序）
     - `field_payload_keys`（满足 §4.2.1 的映射约束；与 allowed_fields 完全一致）
     - `deny_reasons`（严格限制在 §4.4.1 闭集，并按 §4.4.2 稳定排序）
   - 约束：policy 遇到未知/非法组合不得 panic；必须 fail-closed（返回错误或返回 enabled=false，但输出必须稳定且可测试）。
5. [X] 扩展写入侧校验（`modules/orgunit/services/orgunit_write_service.go`）
   - 为 `Create/Rename/Move/Disable/Enable/SetBusinessUnit` 增加 policy 驱动的 fail-closed 校验（禁止在 service 内再写“允许字段矩阵”）。
   - 将 Append 的 ext 写入逻辑抽成可复用 helper（建议复用 corrections 的实现方式）：
     - 读取 enabled field configs（复用 `ListEnabledTenantFieldConfigsAsOf`）
     - 生成 `payload.ext` 与 `payload.ext_labels_snapshot`（DICT 由服务端生成；客户端不得提交）
     - 对 reserved field_key 做兜底阻断（§4.2.2）
6. [X] 收敛 `set-business-unit` 写入口（必须完成，否则“策略单点”不成立）
   - `internal/server/orgunit_api.go` 的 `/org/api/org-units/set-business-unit` 需要改为调用 `OrgUnitWriteService.SetBusinessUnit(...)`（并走 policy + ext + 严格解码）；不得继续绕过 write service 直连 store。

建议 PR-1（先把 policy + 写入校验收敛；不要求 UI 改动）：

- `modules/orgunit/services/orgunit_mutation_policy.go`
- `modules/orgunit/services/orgunit_write_service.go`
- `modules/orgunit/services/orgunit_mutation_policy_test.go`
- `modules/orgunit/services/orgunit_write_service_test.go`

### 5.3 API 与 UI 联动

7. [X] 新增 `append-capabilities` API（建议文件：`internal/server/orgunit_append_capabilities_api.go`）
   - 路由注册：`internal/server/handler.go`
   - Authz：`orgunit.read`（与 `mutation-capabilities` 同口径）；缺少写权限时返回 200，但写动作 `enabled=false + deny_reasons=["FORBIDDEN"]`
   - Store 依赖（建议以接口形式冻结，便于 stub 测试）：
     - `ResolveOrgID`（用于判断 org_code 是否已存在，从而决定 `ORG_ALREADY_EXISTS`）
     - `ListEnabledTenantFieldConfigsAsOf`（用于 ext 集合 `E`）
     - Append facts 查询（建议新增一个 store 方法返回 `tree_initialized/is_root/target_exists_as_of`，具体 SQL 由 PG store 实现；必须事务 + tenant 注入，No Tx No RLS）
   - 输出必须完全由 policy 决定（server 只负责“取事实 → 调 policy → 返回 JSON”）。
8. [X] UI 联动（按“先可发现、再可操作”推进）
   - Create 表单：先调用 append-capabilities，按 `allowed_fields/field_payload_keys` 控制字段禁用与提交 payload；capabilities 失败时 fail-closed（禁用提交）。
   - 详情页动作（rename/move/disable/enable/set BU）：同样先看 append-capabilities 决定动作可用性与字段编辑态；不得再在 UI 侧维护白名单。

建议 PR-2（API + contract tests；UI 可选独立 PR）：

- `internal/server/orgunit_append_capabilities_api.go`
- `internal/server/handler.go`（路由挂载）
- `internal/server/orgunit_append_capabilities_api_test.go`
- 若新增路由：补齐 `make check routing` 所需的路由分类/allowlist（按仓库既有门禁）

### 5.4 Kernel 对齐与防回归

9. [X] 复核 Kernel 对 Append 动作的防守性约束与错误码
   - 对于能够在 facts 层计算的高频失败（例如 root move、tree 未初始化、target 不存在 as-of），优先前移为 deny_reasons（减少“capabilities 放行但必失败”）。
   - 对值相关约束（§4.4.4）保持 Kernel 兜底，服务端透传稳定错误码，UI 展示 code 并给出可理解提示。

### 5.5 实施产物（2026-02-16 收口）

- 后端（策略/API/写入）：
  - `modules/orgunit/services/orgunit_mutation_policy.go`
  - `modules/orgunit/services/orgunit_write_service.go`
  - `internal/server/orgunit_append_capabilities_api.go`
  - `internal/server/orgunit_api.go`
- 前端（capabilities 接入与 fail-closed）：
  - `apps/web/src/pages/org/OrgUnitsPage.tsx`
  - `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - `apps/web/src/pages/org/orgUnitAppendIntent.ts`
  - `apps/web/src/i18n/messages.ts`
- 测试：
  - `internal/server/orgunit_append_capabilities_api_test.go`
  - `modules/orgunit/services/orgunit_mutation_policy_test.go`
  - `modules/orgunit/services/orgunit_write_service_test.go`
  - `apps/web/src/pages/org/orgUnitAppendIntent.test.ts`

## 6. 验收标准（Acceptance Criteria）

- [X] Append 动作的可用性/字段允许范围/字段→payload 映射有单一策略入口，且被 API/UI/写入路径复用（无漂移）。
- [X] UI 常见路径显著减少“可提交但必失败”的体验落差；capabilities 缺失时 fail-closed。
- [X] 无新增 DB schema/迁移；无第二写入口；无 legacy 回退通道。

## 7. 风险与反模式清单（评审结论冻结）

- 禁止旁路写入口：`/org/api/org-units/set-business-unit` 不得继续绕过 `OrgUnitWriteService`（否则策略单点必然失败）。
- 禁止双套矩阵：capabilities 与写入校验必须复用同一 policy；禁止 handler/service 内再写 “if eventType==... 允许字段”。
- 禁止静默忽略：Append 写入口必须严格解码；未知字段/`ext_labels_snapshot` 必须 fail-closed。
- 禁止 ext 覆盖 core：reserved field_key 必须阻断（见 §4.2.2）。
- 允许 Kernel-only 兜底但必须最小化：常见拒绝原因应尽量前移为 deny_reasons（否则 UI 仍会频繁“看似可用但提交失败”）。
- 禁止 deny_reasons “静默扩展闭集”：任何 deny reason code 必须属于 §4.4.1 闭集；出现未知 code 必须 fail-fast（至少单测 fail；允许实现侧在运行时直接拒绝/报警，避免悄悄漂移）。
- 禁止把“可解释的不可用”升级成“系统错误”：除 §4.5 明确的 400（参数非法）外，Append capability 的不可用应尽量用 `enabled=false + deny_reasons` 表达；不得把常规不可用（例如 tree 未初始化/target 不存在 as-of/root move）变成 500。
- 禁止 policy 变成“半函数”：对本计划覆盖的所有 emitted event type + facts 组合，policy 必须返回稳定 decision（不允许 panic；不允许依赖 handler/service 的分叉分支才能成立）。

## 8. 测试与门禁

- 文档：`make check doc`
- 若涉及 Go 改动：按 `AGENTS.md` 触发器矩阵执行对应门禁（建议 `make preflight` 对齐 CI）。
- 推荐新增测试（作为“彻底收敛”的锁）：
  - `modules/orgunit/services/orgunit_mutation_policy_test.go`：Append 合法组合/无效组合、allowed_fields/field_payload_keys、deny_reasons 闭集与顺序。
  - `internal/server/orgunit_append_capabilities_api_test.go`：append-capabilities 合约测试（稳定排序、错误码分支）。
  - `modules/orgunit/services/orgunit_write_service_test.go`：Append 写入口必须调用 policy 校验（含 ext 正/负例）。
- 必须覆盖的“防漂移断言”（冻结为必测项）：
  - deny reasons 闭集断言：任何输出 deny reason 若不在 §4.4.1 中，测试必须直接失败（阻断“静默扩展闭集”）。
  - policy 总是可解析：对本计划矩阵里的每个 emitted event type，都必须能从 policy 得到 decision（不得返回 error/panic）。
  - allowed_fields 与 field_payload_keys 一致性断言：两者键集合必须完全相同（见 §4.2.1）。

已执行（本轮实现）：

- [X] `go fmt ./... && go vet ./... && make check lint && make test`
- [X] `make check routing`
- [X] `make check doc`
- [X] `pnpm --dir apps/web lint && pnpm --dir apps/web typecheck && pnpm --dir apps/web test && pnpm --dir apps/web build`

## 9. 关联文档

- `docs/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
