# DEV-PLAN-106A：Org 扩展字段启用收敛（统一为字典字段方式 + 启用时自定义描述；保留 PLAIN）

**状态**: 已完成（2026-02-18；最后更新：2026-02-18）

## 1. 背景

`DEV-PLAN-106` 把“DICT 字段可引用任意 dict_code”打通为：配置员先启用某个 **field_key**，再在对话框里选择 `dict_code`。

这在用户语言里很容易产生混淆：配置员的表达往往是“我要启用某个字典（例如 测试01号/test01）”，而不是“我要启用某个内置 field_key，然后再绑定字典”。

为避免“field_key（字段本体）”与“dict_code（字典本体）”两套概念并存导致的理解成本，本计划要求对 DICT 能力做收敛：

- **取消内置 DICT 字段（built-in DICT field_key）的启用方式**；
- **统一改为“字典字段方式”**：字典模块的 `dict_code` 直接成为第三步可选的字段（field_key），并允许启用时写入自定义描述（展示名）；
- **保留 PLAIN（自定义 `x_...`）方式**（对齐 `DEV-PLAN-106` 方式 2）。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. **统一 DICT 为“字典字段方式”（取消内置 DICT 字段）**：
   - “启用字段”对话框第三步，DICT 类字段只允许从“字典字段”（来自字典模块 dict registry 的 dict_code 列表）中选择；
   - 不再存在“先选内置 DICT field_key、再选 dict_code”的启用路径。
2. **字典字段出现在第三步（field_key 候选）**：
   - 字典模块的 `dict_code` 被推导为可启用的 `field_key`（见 §4.1），配置员可直接选择，例如选择 `test01` 就是启用“test01 字典字段”。
3. **启用时可自定义描述（display label）**：
   - 配置员在启用“字典字段”时可填写一个描述/展示名（单语言 canonical string），用于字段配置列表与 OrgUnit 详情页字段展示。
4. **保留 PLAIN（自定义字段）**：
   - 保留 `x_...` 自定义 PLAIN(text) 字段启用与写入闭环（对齐 `DEV-PLAN-106`）。
5. **明确 SSOT 与关系**（本计划必须显式写清）：
   - 字典字段（`d_<dict_code>`）：其“可用全集/可用性（as_of）”以字典模块 registry 为唯一事实源（SSOT：`DEV-PLAN-105B`）；Org 不复制 dict_code 枚举（对齐 No Legacy）。
   - 自定义 PLAIN 字段（`x_...`）：定义由“field-config 行本身”隐式承载（仅 PLAIN(text)，对齐 `DEV-PLAN-106`）。
   - 内置字段（built-in，来自 `field-definitions`）：仍作为内置字段元数据 SSOT（`DEV-PLAN-100D/100D2`），但 **内置 DICT field_key 必须被迁移并下线**（见 §4.4），启用入口需 fail-closed。
6. **fail-closed 一致性**：
   - 启用时：dict_code 不存在/在 enabled_on 不可用 => 拒绝；
   - 写入/查询时：dict_code 不可用 => 按既有 DICT 口径 fail-closed。

### 2.2 非目标（Stopline）

1. 不引入“租户可编辑的多语言 label 存储结构”（仍坚持 i18n 仅 en/zh，且不做业务数据多语言；若需要双语描述，另起 dev-plan）。
2. 不在本计划允许“修改已启用字段配置的 dict_code 绑定”（保持 field-config 不可变/审计链一致性）。
3. 不引入新表（若实现阶段发现必须新增表/CREATE TABLE，则必须先更新本计划并按仓库红线取得用户手工确认）。
4. 不在本计划改变内置 PLAIN/ENTITY 字段的启用方式与语义；本计划仅收敛 **DICT 启用方式** 与“启用时自定义展示名”能力。

## 3. 术语与不变量（对齐 SSOT）

- **field-config（tenant_field_configs）**：租户启用配置（占用槽位 + 绑定数据源 + 有效期窗口）（SSOT：`DEV-PLAN-100B/100D`）。
- **dict registry**：字典模块对 dict_code 的治理与可用性计算（SSOT：`DEV-PLAN-105B`；`GET /iam/api/dicts?as_of=...`）。
- **Valid Time**：day 粒度（date）（SSOT：`AGENTS.md` §3.5）。

## 4. 方案设计（高层）

### 4.1 字典字段如何成为 field_key

本计划引入“字典字段命名空间”，用 field_key 前缀表达其来源与推导规则：

- `d_<dict_code>`：表示一个“字典字段”（DICT/text），其 `dict_code=<dict_code>`
- `x_...`：保留给自定义 PLAIN 字段（对齐 `DEV-PLAN-106`）

约束（冻结）：

1. 系统内置字段 **不得** 使用 `d_` 前缀（避免歧义）。
2. `dict_code` 的合法性以字典模块为准（SSOT：`DEV-PLAN-105B`；DB check 为 `^[a-z][a-z0-9_]{0,63}$`）。
3. **可推导为字段 key 的额外约束（由 Org 侧强制，fail-closed）**：
   - 为满足 `tenant_field_configs.field_key` 的 DB check（`^[a-z][a-z0-9_]{0,62}$`），要求 `len("d_"+dict_code) <= 63`，等价于 `len(dict_code) <= 61`；
   - 若 dict_code 合法但过长导致无法推导为 `d_...`，则：
     - enable-candidates 不返回该项（避免 UI 误选）；
     - 若客户端仍提交该 dict_code（通过 `field_key` 或其他路径注入），服务端必须返回稳定错误码并拒绝（fail-closed），且输出可排障日志（包含 dict_code 与长度）。
4. 禁止与任何已存在的 `tenant_field_configs.field_key` 冲突（无论状态如何），沿用既有“同一 field_key 只能配置一次”的约束。
5. **去重不变量（防止“双写同一事实”漂移）**：当 `field_key` 形如 `d_<dict_code>` 时：
   - `data_source_type` 固定为 `DICT`；
   - `value_type` 固定为 `text`；
   - `data_source_config` 固定为 `{"dict_code":"<dict_code>"}`，且 `<dict_code>` 必须与 `field_key` suffix 一致（不一致即拒绝）。

### 4.2 “字典字段”与后端字段定义清单的关系（SSOT 边界）

冻结结论：

- **DICT 字段不再依赖内置 field-definitions 清单**：DICT 的“可启用候选集合”完全由 dict registry 决定，并以 `d_<dict_code>` 的规则推导（见 §4.1）。
- `field-definitions` 仍作为 **内置字段（built-in）元数据** 的 SSOT（`DEV-PLAN-100D/100D2`），但其中的 DICT 类内置 field_key 进入本计划的“迁移下线清单”（见 §4.4）：
  - 启用阶段：内置 DICT field_key 一律拒绝（fail-closed）；
  - 读/展示阶段：迁移完成后不再依赖内置 DICT field_key。
- “启用字段”对话框第三步需要的是“可启用候选集合”，因此新增（或扩展）一个专用读接口，至少返回：
  - 字典字段候选（来自 `GET /iam/api/dicts?as_of=enabled_on`，推导为 `d_<dict_code>`）
  - 自定义 PLAIN 的输入规则提示（`x_...`；无需后端候选列表）

> 注：关键意图是：dict registry 是“字典字段候选”的 SSOT；不把字典模块的全集复制进 Org 的静态定义清单。

### 4.3 启用时自定义描述（display label）

启用“字典字段”（`d_...`）时允许携带可选 `label`（display label）：

- 若用户未提供 `label`：默认使用字典模块返回的 dict name（或 fallback 到 dict_code）。
- 若用户提供 `label`：用于 UI 展示；不参与 DICT 校验逻辑；不影响 options/label 快照生成（DICT label 快照仍由服务端 resolver 生成）。

落地建议（不在本文强行冻结具体字段名，但冻结“必须可持久化”）：

- 在 `orgunit.tenant_field_configs` 增加可空列（例如 `display_label`），用于存储该启用配置的展示名；
- 在 `tenant_field_config_events.payload` 写入 `display_label` 以便审计回放。
- **不可变性（冻结）**：display label 作为 enable 时输入的一部分，与 field-config 映射共同视为不可变；若后续需要“仅更正 label”能力，必须另起 dev-plan 冻结 event/API/审计口径，禁止绕过事件链直接 UPDATE。

### 4.4 收敛与迁移（必须有明确策略）

由于 `DEV-PLAN-106` 已经允许“内置 DICT field_key + 绑定 dict_code”的配置在生产形态存在，本计划要求最终收敛为单一方式（字典字段）。

冻结要求：

1. **最终形态**：租户侧 DICT 字段只存在 `d_<dict_code>` 形式的 field_key，不再存在内置 DICT field_key 的启用配置。
2. **迁移策略必须可执行且可审计**：需要一条明确的迁移路径，把存量 DICT field-config 收敛到 `d_...`，并保留审计证据（事件链/记录）。
3. **不引入长期双链路**：允许短期迁移窗口，但必须有退出条件与门禁（对齐 `DEV-PLAN-004M1` No Legacy）。

> 关键约束说明（冻结）：  
> 1) `orgunit.tenant_field_configs` 的映射在 DB 侧被防御性 trigger 强约束为不可变，且 `(tenant_uuid, physical_col)` 唯一。  
> 2) OrgUnit 写入事件的 `payload.ext`/`payload.ext_labels_snapshot` 以 `field_key` 为键，并在 DB engine 中校验“field_key 必须在 tenant_field_configs 中存在且在 effective_date 下 enabled”。  
> 因此迁移必须做到：**在不改变 physical_col 的前提下完成 field_key rekey，并修正历史事件 payload 与投射快照中使用 field_key 作为键的所有位置**，否则 replay/回放会 fail-closed。

#### 4.4.1 迁移的可执行口径（冻结）

迁移目标：把存量内置 DICT field-config（`data_source_type=DICT` 且 `field_key` 非 `d_...`）收敛为 `d_<dict_code>`。

迁移必须同时覆盖三类数据面（同租户范围，且必须幂等）：

1. **field-config 当前态**：`orgunit.tenant_field_configs.field_key` 从旧 key 改为新 key（同一行原地 rekey；`physical_col/value_type/data_source_config/enabled_on` 不变）。
2. **业务事件 SoT（org events）**：将 `orgunit.org_events.payload` 中的：
   - `payload.ext.<old_key>` 改名为 `payload.ext.<new_key>`；
   - `payload.ext_labels_snapshot.<old_key>` 改名为 `payload.ext_labels_snapshot.<new_key>`；
   - 并保持不变量：label snapshot 的 key 集合必须是 ext key 的子集（否则 engine 会拒绝）。
3. **投射快照（org_unit_versions）**：将 `orgunit.org_unit_versions.ext_labels_snapshot` 中的 `<old_key>` 改名为 `<new_key>`，保持历史版本展示一致性（不改变 label 值本身）。

失败/冲突口径（冻结，fail-closed）：
- 若 `dict_code` 过长导致 `d_<dict_code>` 不合法：迁移必须失败并输出可排障信息（该租户需先治理 dict_code 命名后再迁移）。
- 若 `<new_key>` 已存在（无论 enabled/disabled）：迁移必须失败（防止“同 key 不同义”的第二套表达）。
- 若同一 payload/labels 中同时存在 old/new 两个 key：迁移必须失败（避免覆盖与不可解释状态）。

审计证据（冻结）：
- 迁移必须通过 **专用 Kernel 入口**执行，并追加一条可审计事件（建议新增 `tenant_field_config_events.event_type` 为 `REKEY`/`MIGRATE_REKEY`，payload 记录 `{old_field_key,new_field_key,dict_code,physical_col,enabled_on}`）；禁止“静默 UPDATE”导致审计链断裂。

## 5. 契约变化（草案）

> SSOT 以 `DEV-PLAN-100D/100D2` 为准；本节仅列出本计划新增/调整点。

1. 新增启用候选 API（建议）：
   - `GET /org/api/org-units/field-configs:enable-candidates?enabled_on=YYYY-MM-DD`
   - Response（草案）：
     - `dict_fields[]`：`{field_key:"d_<dict_code>", dict_code, name, value_type:"text", data_source_type:"DICT"}`
     - （可选）`plain_custom_hint`：`{pattern:"^x_[a-z0-9_]{1,60}$", value_type:"text"}`
2. 扩展 enable field-config 请求（仅对 `d_...` 生效）：
   - `POST /org/api/org-units/field-configs`
   - 新增可选字段：`label`（canonical string；长度上限实现阶段冻结）
3. 扩展 field-config list/详情返回展示名与 queryability 元数据能力（用于 UI 收敛，避免前端维护 join/映射）：
   - `GET /org/api/org-units/field-configs`
   - `GET /org/api/org-units/details`
   - 对 `d_...` 字段：返回 `label_i18n_key=null` + `label=<display_label or default>`
   - 并补齐 queryability 元数据（冻结）：
     - `allow_filter/allow_sort`：对 `d_...` 固定为 `true`；对 `x_...` 固定为 `false`；对 built-in 继承 `field-definitions`

### 5.2 错误码与 HTTP 状态（冻结）

> 说明：仓库内 Internal API 的错误返回由统一封装生成；本节只冻结“对外可观察的稳定 code 与语义”，避免 UI/调用方靠猜。  
> 细节实现以 `internal/server/setid_api.go:298` 的 `writeInternalAPIError` 与 OrgUnit API 的错误映射为准。

1. `GET /org/api/org-units/field-configs:enable-candidates`：
   - `400 invalid_request`：`enabled_on` 缺失/非法；
   - `500 tenant_missing`：租户上下文缺失（框架/中间件问题）；
   - `500 dict_store_missing`：服务端未配置 dict registry 依赖；
   - `500 orgunit_field_enable_candidates_failed`：调用 dict registry 失败（内部错误）。
2. `POST /org/api/org-units/field-configs`（启用）：
   - `400 invalid_request`：必填字段缺失/日期非法/自定义 key 不合法；
   - `404 ORG_FIELD_DEFINITION_NOT_FOUND`：built-in 不存在且非合法 `x_.../d_...`；
   - `400 ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG`：
     - built-in DICT field_key（必须 fail-closed）；
     - `d_...` 但提交的 `data_source_config` 与 suffix 不一致；
     - built-in PLAIN/ENTITY 提交了不符合约束的 config；
   - `422`（来自 DB Kernel 的稳定 code）：例如
     - `ORG_FIELD_CONFIG_ALREADY_ENABLED`
     - `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`
     - `ORG_REQUEST_ID_CONFLICT`
3. `GET /org/api/org-units/fields:options`：
   - `404 ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF`：as_of 下该 field_key 未启用；
   - `404 ORG_FIELD_OPTIONS_NOT_SUPPORTED`：非 DICT 或 key/config 不一致（fail-closed）；
   - `500 orgunit_field_options_failed`：调用 dict resolver 失败（内部错误）。

### 5.1 主要下线能力与接口语义收敛（冻结）

1. **删除内置 DICT 字段启用能力（冻结）**：`POST /org/api/org-units/field-configs`
   - 保留 enable 接口，但对 DICT 的口径收敛为：仅允许 `field_key` 为 `d_<dict_code>`（见 §4.1）；任何内置 DICT field_key 一律拒绝（fail-closed）。
2. **收紧 options 解析口径（冻结）**：`GET /org/api/org-units/fields:options`
   - 保留接口用于字典字段值选项查询；
   - 统一按 `d_<dict_code>` 解析 dict_code；
   - （迁移期）若系统仍存在存量内置 DICT field_key（`data_source_type=DICT` 且 `field_key` 非 `d_...`），允许 options 暂时保留“built-in DICT 定义校验”分支以避免断链；完成 §4.4 rekey 后必须移除该兼容分支，禁止长期双链路。

## 6. UI 交互（配置员视角）

在字段配置页点击「启用字段」后：

1. 第三步“字段”下拉/选择器分组展示：
   - 内置字段（来自 `field-definitions`，仅展示非 DICT；DICT 内置字段必须迁移下线）
   - 字典字段（来自 dict registry；以名称+dict_code 展示；field_key 为 `d_<dict_code>`）
   - 自定义 PLAIN 字段（输入 `x_...`）
2. 当选择“字典字段”时显示“描述/展示名”输入框（可选），用于该字段在列表与详情页的展示。

## 7. 需要变更的历史契约文档（登记清单）

> 目的：把“106A 触发的历史契约修改”登记为单点台账，避免遗漏与漂移。  
> 状态口径：`待更新` / `进行中` / `已完成`。

| 状态 | 文档 | 需登记的变更点（摘要） |
| --- | --- | --- |
| 已完成 | `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md` | 增补 enable-candidates 契约；明确 DICT 字段候选只来自 dict registry（`d_<dict_code>`），不再提供内置 DICT field_key 启用路径；保留自定义 PLAIN（`x_...`）。 |
| 已完成 | `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md` | 冻结 `d_<dict_code>` 命名空间、冲突规则、`label`（启用时描述）与返回口径；登记“取消内置 DICT 字段”的收敛与迁移要求。 |
| 已完成 | `docs/dev-plans/101-orgunit-field-config-management-ui-ia.md` | 更新启用对话框 IA：第三步分组展示（内置非 DICT + 字典字段 + 自定义 PLAIN）；字典字段可填写描述；内置 DICT 必须迁移下线。 |
| 已完成 | `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md` | 补充详情页展示契约：`d_...` 字段 `label_i18n_key=null` + `label` 展示规则。 |
| 已完成 | `docs/dev-plans/105-dict-config-platform-module.md` | 补充 dict registry 对上游“字典字段候选”能力约束（as_of、可用性、fail-closed）。 |
| 已完成 | `docs/dev-plans/105b-dict-code-management-and-governance.md` | 明确 `GET /iam/api/dicts?as_of=...` 作为 106A 第三步候选字典字段的唯一来源。 |
| 已完成 | `docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md` | 增补“被 106A 收敛替代”的说明：取消内置 DICT 字段启用路径，统一为 `d_<dict_code>`；PLAIN 自定义（`x_...`）保留。 |
| 已完成 | `docs/dev-plans/100g-org-metadata-wide-table-phase4c-orgunits-list-ext-query-i18n-closure.md` | 更新“列表页 ext filter/sort 元数据来源”：启用字段的 queryability 元数据必须能覆盖 `d_...`，避免 UI 依赖 field-definitions join 丢失字典字段。 |

> 落地记录：契约对齐（PR #374），实现落地（PR #375）。

## 8. 实施步骤（草案）

> 说明：本节是“可直接照着做”的执行清单；具体门禁命令与触发器以 `AGENTS.md` 与 `DEV-PLAN-012` 为准。

### 8.1 契约先行（Contract First）

1. [x] 更新第 7 节登记清单中的 SSOT 文档（逐条落地，不合并“口头共识”）。
2. [x] 在契约中明确并冻结以下关键点（避免实现阶段漂移）：
   - DICT 启用候选只来自 dict registry，并推导为 `d_<dict_code>`（DICT 候选不再来自 field-definitions）。
   - enable/disable 的参数与错误码（尤其是“DICT 仅允许 d_...，内置 DICT 一律拒绝”的 fail-closed）。
   - `label`（启用时描述）的来源、存储与返回优先级。

### 8.2 后端 API 与路由改造

1. [x] 新增（或扩展）启用候选 API：
   - `GET /org/api/org-units/field-configs:enable-candidates?enabled_on=YYYY-MM-DD`
   - 数据源：`GET /iam/api/dicts?as_of=enabled_on`（dict registry SSOT；fail-closed）。
2. [x] 保留 `GET /org/api/org-units/field-definitions` 作为 built-in 字段元数据 SSOT，但收敛 DICT 语义：
   - 启用阶段不得通过内置 DICT field_key 启用（统一走 `d_...`）；
   - （迁移完成后）可将内置 DICT 字段从返回中移除或标记为 deprecated（具体字段形态在 SSOT 文档中冻结）。
3. [x] 收紧 enable API（接口保留、语义收敛）：
   - `POST /org/api/org-units/field-configs` 对 DICT 的新口径：
     - `field_key=d_<dict_code>`：服务端解析 suffix 作为 dict_code，并强制 `value_type=text`、`data_source_type=DICT`、`data_source_config={"dict_code":"<dict_code>"}`（客户端若显式传入不一致则拒绝）；
     - 任何内置 DICT field_key 一律拒绝（fail-closed）。
   - `x_...`：延续 `DEV-PLAN-106`（固定 `value_type=text`、`data_source_type=PLAIN`、`data_source_config={}`）。
   - 其他 built-in：仍由 `field-definitions` SSOT 管理（不在本计划改变其语义）。
4. [x] 收紧 options API：
   - `GET /org/api/org-units/fields:options`：仅支持 `d_...`（解析出 dict_code 后调用 `pkg/dict`），不再支持“内置 DICT field_key”分支。

### 8.3 数据库与 Kernel/Projection（不新增表）

1. [x] 为 `orgunit.tenant_field_configs` 增加可空 display label 列（例如 `display_label`）（不新增表）。
2. [x] 更新 enable/disable 的 kernel 写入路径：
   - enable `d_...` 时：写入/更新 display label（若提供），并把该值写入事件 payload 以便审计。
   - enable `x_...` 时：display label 允许为空（由 UI 直接展示 field_key 或固定推导）。
3. [x] 更新读模型返回：
   - `GET /org/api/org-units/field-configs` 与 `GET /org/api/org-units/details` 对 `d_...` 返回 `label_i18n_key=null` + `label=<display_label or default>`。
4. [x] 更新 list/详情返回 queryability 元数据（对齐 §5 第 3 条）：
   - 对 `d_...` 固定 `allow_filter=true`、`allow_sort=true`；
   - 对 `x_...` 固定 `allow_filter=false`、`allow_sort=false`；
   - 对 built-in：沿用 `field-definitions`。

### 8.4 迁移与收敛（消除“内置需启用字段”）

> 目标：最终租户侧 **DICT 扩展字段** 启用配置只允许 `d_...`（字典字段），并消除内置 DICT field_key；`x_...`（自定义 PLAIN）保留。迁移需可审计、可回放、无长期双链路。

1. [x] 设计并冻结迁移映射表（写在文档或 migration 注释中）：
   - 既有 DICT field-config：从“内置 DICT field_key + data_source_config.dict_code”迁移为 `field_key=d_<dict_code>`；
   - 不迁移 built-in PLAIN/ENTITY（本计划非目标）；
   - 冲突处理（例如目标 field_key 已存在）必须 fail-closed，并输出可排障信息。
2. [x] 实现一次性迁移（module=orgunit）：
   - 必须保证幂等（可重复执行不产生副作用）。
   - 必须保留审计证据（field-config rekey 事件）并在迁移脚本中产出可追溯日志。
   - 必须同步修正：
     - `orgunit.org_events.payload` 的 `ext/ext_labels_snapshot` 键（old -> new）；
     - `orgunit.org_unit_versions.ext_labels_snapshot` 键（old -> new）。
   - 推荐执行入口（冻结）：通过 DB Kernel 的 `orgunit.rekey_tenant_field_config(...)` 逐条 rekey（其内部会：原地 rekey `tenant_field_configs`、改写 `org_events`、改写 `org_unit_versions`、写入 `tenant_field_config_events(REKEY)`）。
3. [x] 迁移完成后，打开门禁：任何 **内置 DICT field_key** 的 enable 请求均被拒绝（DICT 仅允许 `d_...`；对齐 §5.1）。

### 8.5 前端（MUI）收敛改造

1. [x] 字段配置页：
   - 启用对话框第三步分组展示：
     - 内置字段（来自 `field-definitions`，仅展示非 DICT）；
     - 字典字段（来自 enable-candidates 的 dict_fields）；
     - 自定义 PLAIN（输入 `x_...`）。
   - 选择字典字段时显示“描述/展示名”输入框（可选），并随 enable 请求提交。
2. [x] 移除“内置 DICT field_key + 绑定 dict_code”的 UI 路径与相关依赖：
   - 启用 DICT 时不再展示“先选内置字段再选 dict_code”的交互；
   - 列表页/筛选排序（ext query）元数据需覆盖 `d_...`，避免依赖 `field-definitions` join 导致缺字段（对齐 §5 第 3 条）。
3. [x] 详情页：
   - 渲染 ext_fields 时按新口径展示 label（优先 i18n key；否则展示 label；再 fallback field_key）。
   - 字典字段的值编辑：使用 `fields:options`（由 `d_...` 解析 dict_code）。

### 8.6 测试与门禁对齐

1. [x] 单测/契约测试：覆盖
   - enable-candidates 返回 dict_fields；
   - enable：DICT 仅允许 `d_...`；内置 DICT field_key 一律 fail-closed；
   - `fields:options` 仅支持 d_...；
   - display label 的存储与返回。
   - 迁移：old->new rekey 后，engine 校验与 replay/回放路径不失败（重点覆盖 `org_events.payload.ext/ext_labels_snapshot` 的 key 一致性）。
2. [x] E2E：至少覆盖
   - 启用 `d_test01` + 填写描述 -> 列表可见；
   - OrgUnit 详情页写入该字段值 -> 回显 + label 解析。
3. [x] 本地门禁：按 `AGENTS.md` 触发器矩阵跑到与 CI 对齐（推荐 `make preflight`）。

## 9. 验收标准（DoD）

1. 配置员在 `2026-02-17` 这类 `as_of` 场景下，能在第三步直接看到并选择 `test01` 对应的“字典字段”（而不是先找内置 field_key 再选 dict_code）。
2. 启用时填写的描述/展示名在字段配置列表与 OrgUnit 详情页可见。
3. dict_code 不存在/不可用/停用时，在启用/写入/查询三个环节均 fail-closed，错误码稳定且可排障。
4. 迁移后可重放（replay）并保持历史版本展示一致：不再出现 “old field_key 的 ext/labels 无法通过 engine 校验” 的失败路径。
