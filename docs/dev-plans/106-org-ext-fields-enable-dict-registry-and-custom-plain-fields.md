# DEV-PLAN-106：Org 模块扩展字段启用方式改造（DICT 全量引用 + 自定义 PLAIN 字段）

**状态**: 已完成（2026-02-17）

> 2026-02-17 补充：本计划已完成“DICT 全量引用（dict registry SSOT）+ 自定义 PLAIN（x_ 命名空间）”的最小闭环。  
> 为进一步降低“field_key（字段） vs dict_code（字典）”双概念成本，DICT 启用方式后续由 `DEV-PLAN-106A` 收敛为“字典字段方式”：`field_key=d_<dict_code>`（并支持启用时自定义展示名）。本文件中与 106A 冲突的 UI/契约描述，以 106A 为准。

> 目标：把 Org 模块“启用扩展字段”的两类配置能力补齐为可治理、可审计、可验证的契约：  
> 1) **DICT 全量引用**：字典模块中已配置/治理的全部 `dict_code` 都可以被 Org 的 DICT 扩展字段引用（不再被 Org 代码内枚举/预设卡死）。  
> 2) **自定义 PLAIN 字段**：支持租户管理员新增自定义的 PLAIN 扩展字段（不再只能使用代码内预设字段清单）。

## 1. 背景

`DEV-PLAN-100` 系列已把 OrgUnit 的扩展字段体系冻结为“宽表预留槽位 + 元数据驱动”，并提供了 Phase 3/4 的 API 与 UI 联动闭环（`DEV-PLAN-100D/100E/101`）。同时，`DEV-PLAN-105/105B` 已把 DICT 值与 dict_code 的生命周期治理收敛到平台级字典模块（iam）。

当前仍存在两个与“配置化/可扩展”目标冲突的硬点：

1. **DICT 引用范围仍被 Org 自身的 field-definitions 枚举约束**：启用字段时要求 `data_source_config` 命中 `field-definitions.data_source_config_options`（`DEV-PLAN-100D/100D2`），这会把“可用 dict_code 的全集”变相复制进 Org（易漂移、需要发版）。
2. **PLAIN 字段集合仍然偏“预设清单”**：字段定义清单（MVP 2~5 个）冻结后，新增 PLAIN 字段往往需要改代码/补 i18n key，难以做到“租户自助扩展”。

本计划在不破坏 One Door / No Tx, No RLS / fail-closed / No Legacy 的前提下，改造“启用扩展字段”的契约与实现边界，使其与字典模块（`DEV-PLAN-105/105B`）的治理能力对齐。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. **DICT 全量引用**：Org 的 DICT 扩展字段在启用时可以引用任意 `dict_code`（来源为字典模块 registry；`as_of` 语义与 `DEV-PLAN-105B` 对齐），并在启用/写入/查询三个环节保持一致的 fail-closed 校验。
2. **自定义 PLAIN 字段（不新增表）**：租户管理员可通过“启用字段配置（field-config）”直接引入自定义 PLAIN 字段（最小闭环：`value_type=text`），复用既有预配置槽位（`ext_str_XX`），不新增 DB 表；最终在详情页可见、可写（按 capabilities 决定）。
3. **契约单点**：
   - dict_code 的“存在性/可用性”以字典模块 registry 为唯一事实源（SSOT：`DEV-PLAN-105B`），Org 不再复制 dict_code 枚举。
   - 扩展字段的“字段定义（field_key/value_type/data_source_type/label）”对**内置字段**仍以 `field-definitions` 为 SSOT；对**自定义 PLAIN 字段**，定义由“field-config 行本身”隐式承载（仅 PLAIN(text)，并受严格命名/冲突约束）。
4. **No Legacy**：不保留“旧的静态枚举/registry”作为长期兜底；迁移与回滚只允许走环境级保护（对齐 `DEV-PLAN-004M1`）。

### 2.2 非目标（Stopline）

1. 不在本计划引入“自定义 DICT 字段”（即允许新增 data_source_type=DICT 的新字段定义）；本计划只把“DICT 引用的 dict_code”解耦为可选择全集，并把自定义能力限定在 PLAIN。
2. 不在本计划引入“扩展字段标题的多语言配置”（custom label 的 i18n）。自定义字段 label 先采用单一 canonical label（与 DICT option label 一样不随 locale 变化）；如需 en/zh 双语配置，另起 dev-plan。
3. 不扩展 ENTITY（保持既有 fail-closed 口径）。

## 3. 术语与不变量（对齐既有 SSOT）

- **field-definition**：字段元数据定义（`field_key/value_type/data_source_type/...`），供“启用配置（field-config）”与 UI 渲染使用（SSOT：`DEV-PLAN-100D`）。
- **field-config（tenant_field_configs）**：某租户在某个有效期窗口内启用该字段，并绑定 `physical_col` 与 `data_source_config`（SSOT：`DEV-PLAN-100B/100D`）。
- **DICT**：字段值为 code（通常 text），label 快照由服务端生成并写入 `ext_labels_snapshot`（SSOT：`DEV-PLAN-100C/100E1`）。
- **Valid Time**：一律 day 粒度（date）（SSOT：`AGENTS.md` §3.5）。

## 4. 方案总览

本计划把“启用扩展字段”拆成两条改造路径（对应用户诉求的 方式 1/方式 2）：

1. **方式 1（DICT 全量引用）**：把 DICT 字段启用时 `data_source_config.dict_code` 的校验来源，从“命中 field-definitions 枚举候选”改为“命中字典模块 registry（as_of 校验）”。UI 选择器的数据源从 Org 的 `field-definitions.data_source_config_options` 切换为字典模块的 dict list。
2. **方式 2（自定义 PLAIN 字段）**：不新增 field-definitions 写入口；通过 enable `field-configs` 时输入自定义 `field_key`（`x_` 命名空间）来引入自定义字段；其余 enable/disable/详情/ext 写入等链路仍沿用既有 `tenant_field_configs` 与 ext 槽位分配逻辑。

## 5. 方式 1：DICT 全量引用（dict registry 作为 dict_code SSOT）

### 5.1 契约变化（高层）

1. **enable field-config 的校验口径调整**：
   - 旧：`DICT/ENTITY` 必须命中 `field-definitions.data_source_config_options`（枚举化候选）。
   - 新（本计划冻结）：`DICT` 的 `data_source_config.dict_code` 必须在字典模块 registry 中存在，且在 `enabled_on`（day）下可用（fail-closed）。不再要求命中 Org 自身的枚举候选（避免第二套 SSOT）。
2. **field-definitions 的 DICT 配置来源声明**：
   - `field-definitions` 仍声明 `data_source_type=DICT` 与 `data_source_config` 的形状（`{dict_code: string}`；对齐 `DEV-PLAN-100B` 的 DB check）。
   - 但不再承担“列举所有可选 dict_code”的职责（可选 dict_code 的全集来源为字典模块）。
3. **UI 选择器数据源切换**：
   - 字段配置管理页（`DEV-PLAN-101`）中 DICT 的 dict_code 选择器，从读取 `field-definitions.data_source_config_options` 改为读取字典模块 dict list（`DEV-PLAN-105B`）。

### 5.2 后端依赖边界（模块解耦）

对 Org 的 Go 代码侧，DICT 校验与 options/label 解析仍必须通过 `pkg/**` 的门面完成（对齐 `DEV-PLAN-105`），避免跨模块直接 import `modules/iam/**` 造成边界漂移。

> 注：本计划只冻结“契约与依赖边界”，具体 `pkg/dict` 需要补齐哪些接口（list dicts / dict exists as_of / resolve label）在实现阶段以 `DEV-PLAN-105/105B` 为 SSOT。

## 6. 方式 2：自定义 PLAIN 字段（通过启用配置引入；不新增表）

### 6.1 契约冻结（最小闭环）

1. **不新增“字段定义表/写入口”**：自定义 PLAIN 字段不单独存储 field-definitions；其“定义”由 `POST /org/api/org-units/field-configs` 的 enable 行为隐式创建（复用既有 `tenant_field_configs` 与槽位分配）。
2. **自定义字段 key 规则（冻结）**：
   - 自定义 PLAIN 字段 `field_key` 必须满足 `x_[a-z0-9_]{1,60}`（且满足既有 lower_snake 约束）；
   - 禁止与任何系统内置字段同名（命中则拒绝；避免“同名不同义”的第二套权威表达）。
   - `x_` 前缀为自定义字段保留命名空间；系统内置字段 **不得** 使用 `x_` 前缀。
3. **enable field-config 对自定义字段的特殊口径（冻结）**：
   - 当 `field_key` 以 `x_` 前缀开头时，视为“自定义 PLAIN 字段”，允许 **不出现在** `field-definitions` 列表中；
   - 该路径下 `data_source_type` 固定为 `PLAIN`、`value_type` 固定为 `text`、`data_source_config` 必须为 `{}`（缺失由服务端补齐为 `{}`）。
4. **label 策略（冻结）**：
   - 内置字段继续返回 `label_i18n_key`；
   - 自定义字段不提供 i18n key：服务端在 `details.ext_fields[]` 中返回 `label`（推荐直接使用 `field_key` 作为 label，或按固定规则从 `field_key` 推导），并将 `label_i18n_key` 置空；
   - UI 展示：优先 `label_i18n_key`，否则展示 `label`（或 fallback 到 `field_key`）。UI 禁止维护第二套字段映射表。

### 6.2 约束与 fail-closed

1. 槽位限制：自定义 PLAIN 字段启用后仍占用既有 `ext_str_XX` 槽位；槽位耗尽返回既有稳定错误码（SSOT：`DEV-PLAN-100B/100D`）。
2. 字段 key 治理：禁止任意注入“看似字段但不可写/不可查”的僵尸定义；自定义字段必须能在 UI 中被发现、被启用/停用并完成至少一次端到端写入与回显（对齐 `AGENTS.md` §3.8）。

## 7. 需要变更的历史契约文档（清单）

> 说明：以下为“必须同步更新以避免契约漂移”的 SSOT 文档；实现落地前应先更新这些文档，再改代码（Contract First）。

1. `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
   - 更新 ADR-100D-04（field-definitions 作为 SSOT 的边界）：DICT 的 dict_code 候选全集不再由 field-definitions 枚举，而由字典模块 registry 提供；enable 校验口径相应调整。
   - 更新 `field-definitions`/enable 契约：`data_source_config_options` 对 DICT 不再要求返回“可选 dict_code 枚举”（或调整为“来源声明”字段）。
2. `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
   - 100D2 当前冻结“DICT/ENTITY options 必为非空数组”的口径需要修订，以匹配“DICT 全量引用”的新契约。
3. `docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
   - 更新 DICT 的 `dict_code` 选择器数据源：改为字典模块 dict list（`DEV-PLAN-105B`）。
   - 增加“自定义 PLAIN 字段（输入自定义 `field_key`）”的 UI 入口与 IA（最小可用：输入 -> 启用 -> 详情页可写可见）。
4. `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
   - 更新 ext_fields 展示 label 的契约：支持 `label`（literal）与 `label_i18n_key`（i18n）两种来源，避免自定义字段无法显示。
5. `docs/dev-plans/105-dict-config-platform-module.md`
   - 补充“作为全模块 DICT SSOT”对上游（Org 字段启用）的契约要求：dict list/registry 能力必须支撑“DICT 全量引用”的选择与校验闭环。
6. `docs/dev-plans/105b-dict-code-management-and-governance.md`
   - 明确 dict_code registry 的读口径（`as_of` 必填、tenant 覆盖 global、停用语义）对 Org 字段启用校验与 UI 选择器的约束。
7. （可能需要）`docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
   - 100A 的“字段定义清单（2~5）冻结”需补充：内置字段清单仍冻结，但允许在此基础上引入“租户自定义 PLAIN 字段（`x_` 命名空间）”（并明确其治理边界与 DoD）。
8. （不再需要新增表假设）本计划冻结为：自定义 PLAIN 字段不新增 DB 表；若实现阶段发现仍必须新增表/列/Kernel 写入口，则必须先更新本计划并按仓库红线取得用户手工确认。

## 8. 实施步骤（草案）

1. [x] 更新第 7 节列出的 SSOT 文档，冻结新契约（先文档后代码）。
2. [x] 方式 1：改造 enable 校验与 UI 选择器来源（dict registry as_of 校验；替换掉 Org 的 dict_code 枚举依赖）。
3. [x] 方式 2：打通“自定义 PLAIN 字段”的 UI 闭环（输入自定义 `field_key` -> 启用 -> 写入 -> 回显），并补齐对应的 fail-closed 校验与测试。
4. [x] 补齐契约测试与门禁证据（命中项以 `AGENTS.md` 与 `DEV-PLAN-012` 为准）。

## 9. 验收标准（DoD）

1. DICT 全量引用：
   - 启用 DICT 字段时可从 dict list 选择任意 dict_code，并通过 enable 校验；
   - 写入 DICT 值时，dict_code 不存在/已停用/`as_of` 不可用一律 fail-closed，错误码稳定；
   - options/label 快照生成不依赖代码内静态 registry（对齐 No Legacy）。
2. 自定义 PLAIN 字段：
   - 管理员可创建自定义 PLAIN(text) 字段定义，并在字段配置页启用；
   - 详情页在 enabled window 内可见该字段，且在 capabilities 允许时可写入并回显；
   - 槽位耗尽/字段 key 冲突/非法输入均有稳定错误码且可排障。
