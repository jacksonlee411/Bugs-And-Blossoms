# DEV-PLAN-321：租户可扩展能力（字段/字典/策略）业务规则优先蓝图与共享模型方案

**状态**: 规划中（2026-03-17 22:41 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 的 `321` 子计划，同时承接 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“业务规则优先”的方法论。

`300` 对共享能力的核心要求不是“先把字段和表设计出来”，而是：

- 先定义租户真正维护的业务对象与业务承诺，再决定技术实现形态。
- 生效日期、历史、审计、Explain 是主能力，不是补充字段。
- 工作流、报表、Assistant 与各业务域都只能建立在清晰的主规则之上，不能反向发明字段语义。

当前仓库中，与“字段配置/租户扩展能力”相关的规则已经分散沉淀在：

- `105/105B`：字典本体与字典值治理
- `106/106A`：扩展字段命名空间与启用方式
- `161/165/184/185`：动态策略、页面职责与 SetID/候选值可见性
- `200`：积木式页面与四层 SoT 蓝图
- `070B`：tenant-only 运行时与“共享改发布”
- `102C6`：`capability_key` 防退化与上下文解耦

但这些规则目前仍然以 Org 样板为主，尚缺一份平台级 SSOT，把“字段配置”提升为 **租户可扩展能力（Tenant Extensibility）** 的通用业务蓝图。  
`321` 的任务就是完成这件事：总结现行规则、冻结目标蓝图，并把它作为 `340/350/360/370/380/390` 的业务需求输入之一，而不是继续停留在某个模块的页面能力。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述当前字段配置体系，不让 `field-config / dict / strategy / slots / API` 这些实现词汇喧宾夺主。
- [ ] 总结当前仓库已经稳定沉淀的业务规则，并区分“现行规则”与“待抽象为平台通用能力的部分”。
- [ ] 冻结租户可扩展能力的目标业务蓝图：对象、场景、边界、不变量、时间语义、Explain 与审计要求。
- [ ] 作为 `320` 的共享建模子计划，定义通用建模边界：静态定义、候选值池、动态策略、运行时决议、历史/审计之间如何分层。
- [ ] 为 `340/350/360/370/380/390` 提供统一的业务需求输入，避免后续子计划再次把“扩展能力”写回某个模块私有实现。

### 2.2 非目标

- [ ] 本计划不直接定义最终数据库 DDL、迁移脚本、ORM 映射与 API 路由实现。
- [ ] 本计划不替代 [DEV-PLAN-105](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/105-dict-config-platform-module.md)、[DEV-PLAN-106A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md)、[DEV-PLAN-184](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md) 等已冻结专项契约，而是把它们收敛成平台级语言。
- [ ] 本计划不要求所有业务域采用同一种“扩展值物理存储结构”；共享的是业务合同，不是强推某一个模块的宽表/槽位实现。
- [ ] 本计划不把 `SetID / business_unit / org_node` 之类 Org 语义写死为全平台唯一上下文维度；这些只是“可扩展能力”在具体业务域中的一个实例。
- [ ] 本计划不重新引入 `scope_code / scope_type / scope_key / package` 等 legacy 语义，也不允许双写或双读回退链路。

## 3. “业务规则优先”在租户可扩展能力中的翻译

### 3.1 先表达租户想管理什么，再表达技术如何承载

租户管理员真正维护的不是：

- “某列映射到哪个 physical column”
- “某条策略表里写了哪几个枚举”
- “某个页面发哪个接口”

他们真正维护的是：

- 我们这个租户需要哪些可扩展业务属性；
- 这些属性在哪些业务场景中可见、必填、可改、默认是什么；
- 它们从哪一天开始生效，到哪一天结束；
- 候选值从哪里来，哪些值在当前场景真正允许；
- 出问题时，系统能否解释“为什么是这个结果”。

### 3.2 字段、字典、策略、历史、Explain 都是一级业务能力

“租户可扩展能力”不是“给表单加几个动态字段”的 UI 附件，而是平台级业务能力，至少包括：

- 扩展项定义与启停
- 候选值池治理与发布
- 上下文化规则决议
- 当前 / 指定日期 / 历史视角
- 审计、Explain 与回放

### 3.3 业务域是消费者，不是第二所有者

组织、职位、人员、任职等业务域可以消费这套能力，但不应各自拥有第二套：

- 字段元数据系统
- 动态规则系统
- 字典候选治理系统
- Explain 体系

领域模块拥有自己的主业务对象与值存储；租户可扩展能力拥有“可扩展属性如何被定义、裁剪、解释”的平台合同。

## 4. 当前基线：已沉淀的业务规则

### 4.1 现行且已稳定的规则

#### 4.1.1 双层 SoT 已经成立

- 字段配置页（Field Config）已经被收敛为 **静态元数据治理面**：
  - `field_key`
  - `value_type`
  - `data_source_type`
  - `data_source_config`
  - `enabled_on/disabled_on`
  - 展示与列表元数据
- Strategy Registry 已经被收敛为 **动态策略治理面**：
  - `visible`
  - `required`
  - `maintainable`
  - `default_rule_ref/default_value`
  - `allowed_value_codes`
  - `policy_version`
- 同一语义不允许两边双写；字段页只能镜像动态结果，动态项唯一主写入口是策略面。

#### 4.1.2 候选值池与允许值集合已经分层

- 字典本体与字典值的存在性/可用性以平台字典模块为唯一事实源。
- 运行时读取口径已经收敛为 **tenant-only**；共享基线通过“发布到租户本地”实现，不允许 runtime global fallback。
- `allowed_value_codes` 不是候选值池事实源，只是当前能力上下文下的最终允许子集。
- 因此已经形成稳定分层：
  - 候选值池：由字典/主数据来源定义“可能有哪些值”
  - 允许值集合：由动态策略裁剪“当前真正允许哪些值”

#### 4.1.3 字段身份与命名空间已经冻结

- 平台/模块内置字段：继续使用稳定 `field_key`，不得占用扩展前缀。
- 字典字段：`d_<dict_code>`。
- 租户自定义字段：`x_<custom_key>`。
- 这意味着当前平台已经具备“内置能力 + 字典派生能力 + 租户自定义能力”三种来源。
- 未知字段、未启用字段、命名冲突字段必须 fail-closed。

#### 4.1.4 时间与历史语义已经成型

- 有效时间使用 day 粒度：
  - `enabled_on`
  - `disabled_on`
  - `effective_date`
  - `end_date`
- `as_of` 必须显式提供，禁止 default today。
- 配置与策略都必须支持：
  - `current`
  - `as_of`
  - `history`
- 审计时间与业务生效时间分离：前者用于“何时操作”，后者用于“何时生效”。

#### 4.1.5 运行时决议已经转向 capability 驱动

- `capability_key` 只表达“能力动作”，不编码租户、BU、SetID、地域等上下文。
- 运行时需要先解析上下文，再按 `tenant + capability_key + context + as_of + field_key` 命中策略。
- `policy_version` 已经被明确为提交一致性锚点，用于阻断 TOCTOU。
- 默认值与校验顺序已经有明确口径：
  - 先决议最终值
  - 再做 `required`
  - 再做允许值校验

#### 4.1.6 Fail-Closed、Explain 与审计已经是刚性要求

- 缺租户、缺上下文、缺策略、缺基线、版本过期、值不在允许集合时都必须拒绝。
- 不能靠“页面没展示这个字段”来推断可写性；可信边界始终在服务端。
- 决议必须可 Explain，至少能回答：
  - 命中了哪条规则
  - 为什么允许/拒绝
  - 采用了哪个版本
- 配置变更、发布行为、迁移改键、停用行为都必须可审计。

#### 4.1.7 可发现、可操作、不可僵尸

- 扩展能力不是“后端先埋一层元数据”就算交付。
- 字段、候选值、策略都必须有明确的用户入口、可预览、可生效、可被业务页面实际消费。
- 这条规则已经在现有 Org 样板中被证明是必要的，`321` 将其提升为平台通用要求。

### 4.2 当前仍然带模块耦合的部分

尽管上述规则已较稳定，但它们目前仍然主要借助 Org 样板表达，存在以下模块耦合：

- “扩展能力”仍经常被叫做“Org 字段配置”，容易让人误以为这是组织模块私有能力。
- 最成熟的上下文维度目前是 `SetID / business_unit / parent_org_code`，但这些是 Org 语义，不是全平台共享词汇。
- 一些列表筛选、查询能力、详情页回显规则仍绑定在 Org 的页面模型与 API 路径上。
- 物理存储讨论经常与 Org 的宽表/槽位实现绑在一起，掩盖了“共享的是业务合同，不是存储形态”。

`321` 的目的不是否定这些样板，而是把它们从“Org 已实现经验”提升为“平台共享能力蓝图”。

### 4.3 现有规则到平台语言的提升关系

| 现有来源 | 现行结论 | `321` 提升后的平台语言 |
| --- | --- | --- |
| `105/105B` | 字典是平台级 SSOT，tenant-only 运行时 | `OptionCatalog` 是平台共享候选值池 |
| `106/106A` | `d_` / `x_` 命名空间冻结 | 扩展项身份必须可解释、可区分来源 |
| `165/184` | 字段页静态、策略页动态 | 静态定义与动态约束分层，不允许双主写 |
| `161` | capability 驱动运行时决议 | 扩展行为必须由业务能力上下文解析，而非页面硬编码 |
| `185` | SetID 是候选/取数可见性样板 | 业务上下文必须显式解析并可回显，但上下文维度不局限于 SetID |
| `200` | 四层 SoT + 组合式运行时 | 租户可扩展能力必须可组合、可 Explain、可回放 |
| `070B` | 共享改发布，不走 global fallback | 共享基线是发布时能力，不是运行时跨租户读取能力 |
| `102C6` | `capability_key` 禁止上下文编码 | 行为键稳定，业务上下文单独建模 |

## 5. 租户可扩展能力的目标业务蓝图

### 5.1 领域使命

租户可扩展能力是平台内“租户可增加哪些业务属性、这些属性在哪些场景下如何表现、候选值从哪里来、规则为什么这样决议、历史如何解释”的唯一业务权威。  
它不是某个模块的附属页面，而是所有业务域共享的治理与运行时能力。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 321 是否将其视为平台拥有 |
| --- | --- | --- |
| `ExtensionDefinition` | 扩展项本体：这个属性是什么、值类型是什么、数据源类型是什么 | 是 |
| `ExtensionActivation` | 某租户在某业务对象/页面/场景上启用了哪个扩展项，以及生效窗口 | 是 |
| `OptionCatalog` | 扩展项可使用的候选值池，如字典、本地基线、只读主数据引用 | 是 |
| `ExtensionPolicy` | 某个业务能力上下文下，该扩展项是否可见/必填/可维护/默认值/允许值 | 是 |
| `ApplicabilityContext` | 决议扩展行为时所依赖的业务上下文 | 是（共享合同），但具体解析逻辑由消费域提供 |
| `ExtensionDecision` | 运行时合成后的决议快照与 Explain 结果 | 是（共享合同），但通常为派生结果而非独立主写事实源 |
| `ExtensionHistory` | current / as-of / history 视角下的扩展定义、启用状态与策略版本 | 是 |
| `ExtensionAuditLog` | 配置变更、发布、停用、改键、迁移与 Explain 追踪证据 | 是 |
| `DomainBusinessRecord` | 具体业务域上的值本身，例如组织记录、职位记录、人员记录的扩展值 | 否，领域模块拥有 |

### 5.3 面向用户的主能力

- 在租户内引入平台内置扩展项
- 新建租户自定义扩展项
- 为扩展项选择/发布候选值池
- 按业务能力上下文配置必填、可见、默认值、允许值
- 在 current / as-of / history 视角下预览结果
- 查看 Explain 与审计记录，理解“为什么如此决议”
- 停用、替换、迁移或发布扩展能力
- 让这些能力在业务表单、详情、列表、导入导出、报表和 Assistant 中真实可用

## 6. 321 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 引入平台内置扩展项 | 让某个租户开始使用平台已提供的扩展属性 | 扩展项身份稳定；启用窗口明确；未知/冲突键拒绝；启用后必须可被业务页面消费 | 形成租户内可见、可用的标准扩展项 |
| 新建租户自定义扩展项 | 为本租户补充平台未内置的业务属性 | 仅允许受控命名空间；必须定义值类型与展示语义；不能与既有键冲突；启用后必须可见可操作 | 形成租户私有扩展项 |
| 启用候选值池 | 让扩展项从字典/主数据引用中获得候选值 | 候选值池来自平台 SSOT；tenant-only 读取；运行时不回退 global；候选池与允许值集合分层 | 扩展项有稳定可解释的候选来源 |
| 配置动态策略 | 让同一扩展项在不同业务场景下行为不同 | 静态定义与动态策略分层；`capability_key` 稳定；上下文显式解析；`policy_version` 可追踪 | 不同场景命中可解释的行为规则 |
| 运行时预览 | 在提交前看清当前/某日结果及原因 | `as_of` 显式；current/as-of/history 同时成立；Explain 必须回显命中链路 | 用户与系统对决议达成同一理解 |
| 提交业务数据 | 在业务对象上真正写入扩展值 | 服务端二次校验；未知字段/未启用字段/非法值拒绝；默认值与必填顺序稳定；版本过期拒绝 | 扩展值写入与规则保持一致 |
| 停用或退役扩展项 | 结束某个扩展项在未来的使用 | 停用是时间语义，不等于删除历史；历史仍需可读可审计；不能留下“已停用但仍可写”的幽灵通道 | 扩展项停止未来生效，但历史可追溯 |
| 发布共享基线到租户 | 将平台维护的共享定义/字典能力落到租户本地 | 共享通过发布实现，不通过运行时跨租户读取；发布也必须审计、幂等、可回放 | 租户本地拥有可治理的扩展基线 |
| 审计与排障 | 看清谁改了什么、为什么生效/不生效 | 配置/策略/发布/改键都要有审计；Explain 至少能回显来源层、版本、原因 | 满足治理、排障、合规和对话解释需求 |

## 7. 共享建模边界与不变量

### 7.1 共享建模层次

`321` 冻结以下四层共享语言：

| 层次 | 共享事实源 | 负责回答的问题 | 不负责什么 |
| --- | --- | --- | --- |
| 静态定义层 | `ExtensionDefinition + ExtensionActivation` | 这个扩展项是什么、在哪些租户/对象/时间窗口内存在 | 不负责当前场景下是否必填/可见 |
| 候选值层 | `OptionCatalog` | 这个扩展项的候选值池是什么 | 不负责当前场景下最终允许哪些值 |
| 动态策略层 | `ExtensionPolicy` | 在当前能力上下文下，这个扩展项应该如何表现 | 不负责扩展值本身的领域业务真值 |
| 决议与 Explain 层 | `ExtensionDecision` | 当前/某日究竟采用了什么结果，为什么 | 不成为第二主写事实源 |

### 7.2 时间与版本合同

- 业务生效时间统一为 day 粒度。
- 配置、候选值、策略都必须支持显式有效窗口。
- 审计时间只回答“何时操作”，不替代业务生效时间。
- 读取必须显式区分：
  - `current`
  - `as_of`
  - `history`
- 运行时提交至少要能绑定 `policy_version`；当页面或对话基于多层组合快照生成结果时，还应能绑定组合版本指纹，防止“旧决议提交新数据”。

### 7.3 业务上下文合同

`321` 采用平台级的共享表达：

- `tenant`
- `capability_key`
- `applicability_kind`
- `applicability_ref`
- `as_of`

冻结原则：

- `capability_key` 只表达能力动作，不表达上下文。
- 上下文必须通过独立字段解析，不允许回流到 key 命名里。
- `applicability_kind` 是共享概念，具体业务域可以实例化为：
  - `tenant`
  - `module`
  - `setid`
  - `business_unit`
  - `legal_entity`
  - `org_node`
  - 其他被正式计划冻结的业务维度
- 不得把这个共享合同重新命名回 legacy 的 `scope_type/scope_key/package` 体系。

### 7.4 身份、命名与边界合同

- 内置扩展项、字典扩展项、租户自定义扩展项必须可从命名空间上区分来源。
- 字典扩展项的键与字典本体之间必须一一可推导、可解释。
- 候选值池与允许值集合必须保持父子关系：允许值集合永远只能是候选值池的子集。
- 共享基线只能发布到租户本地，不能作为 runtime 跨租户读取借口。
- 未启用、未发布、版本过期、值非法、上下文缺失的场景一律 fail-closed。

### 7.5 领域实现护栏

- `321` 冻结的是平台共享业务合同，不强推所有业务域采用同一种“值存储模式”。
- 业务域可以根据自身需要选择显式列、扩展表、宽表槽位等方式承载“扩展值”，但必须满足：
  - 不反向改写平台共享定义
  - 不形成第二套候选值池/动态策略/Explain 体系
  - 关键业务字段仍应显式建模，不把平台共享能力偷换成 JSON 万能口袋
- 因此，Org 当前的宽表方案是一个实现样板，而不是 `321` 对所有域的唯一物理要求。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `340`（平台与 IAM 基座）的输入

- [ ] 平台侧需要拥有租户级扩展目录、候选值目录、策略目录、发布与审计底座。
- [ ] 权限模型至少要能区分：查看扩展定义、管理扩展定义、管理动态策略、管理共享发布。
- [ ] tenant-only 运行时、发布幂等、Explain 读取与审计追踪必须由平台能力提供底座支持。

### 8.2 对 `350`（前端产品壳与交互系统）的输入

- [ ] 需要统一的管理页模式：扩展目录、候选值目录、策略治理、决议预览、Explain/历史。
- [ ] 页面必须明确区分“静态定义来源”和“动态决议来源”，避免再次出现双写认知。
- [ ] 运行时表单、详情、列表都应能消费共享决议，而不是各页面自行猜测字段行为。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] 各业务域只消费共享扩展能力，不重复发明本模块专属的字段策略系统。
- [ ] 每个业务域需要显式声明自己支持哪些 `applicability_kind`，以及如何把业务上下文解析到共享合同。
- [ ] 领域模块拥有值本身与领域不变量，但“字段是什么、如何裁剪、为何这样显示/校验”属于共享能力。

### 8.4 对 `370`（工作流、审计增强与集成）的输入

- [ ] 工作流至少要区分：定义变更、启用停用、策略变更、发布动作四类扩展治理请求。
- [ ] 审计增强不得重写共享能力的主规则，只能叠加审批轨迹、回执与集成结果。
- [ ] 对外集成若消费扩展能力，必须显式声明使用 `current`、`as_of` 还是 `history` 视图。

### 8.5 对 `380`（数据工作台与运营分析）的输入

- [ ] 查询、导出、报表必须支持扩展能力的 current / as-of / history 语义。
- [ ] 需要可发现的扩展目录与字段血缘，避免导出侧重新拼一套“动态字段字典”。
- [ ] 数据质量检查至少能发现：未发布即使用、策略缺失、允许值越界、版本漂移、租户基线缺失。

### 8.6 对 `390`（Chat Assistant）的输入

- [ ] Assistant 读取扩展能力时，必须通过共享目录、候选值池、Explain 与策略快照，不得直接猜测字段语义。
- [ ] Assistant 写入业务数据时，必须遵循与 UI 相同的 `policy_version` / Explain / fail-closed 约束。
- [ ] 当字段或上下文有歧义时，Assistant 必须先澄清，不得自行补完或跨越租户/策略边界。

## 9. 建议实施分期

1. [ ] `M1`：词汇与边界冻结  
   统一“扩展定义 / 候选值池 / 动态策略 / 决议 / Explain / 审计 / 发布”词汇，停止把共享能力写成 Org 私有概念。
2. [ ] `M2`：共享上下文合同冻结  
   由各消费域声明自身支持的 `applicability_kind`，并对齐 `capability_key + context + as_of` 决议协议。
3. [ ] `M3`：共享 API / DTO / Explain 合同冻结  
   抽出通用的元数据读取、候选值读取、策略决议、Explain 与版本一致性协议。
4. [ ] `M4`：首批消费域收敛  
   以 Org 为样板，把已有字段配置/策略/字典/Explain 路径按共享能力语言重命名、收口与复用。
5. [ ] `M5`：跨子计划对齐  
   将 `340/350/360/370/380/390` 中涉及扩展能力的内容引用 `321`，不再各自重写主规则。

## 10. 验收标准

- [ ] 后续平台/业务域子计划在描述租户扩展能力时，引用 `321` 作为主业务蓝图，而不是重新发明术语。
- [ ] “字段配置”不再被误认为某个模块私有能力，而被正式提升为平台共享的租户可扩展能力。
- [ ] 共享业务合同已经明确区分：静态定义、候选值池、动态策略、决议与 Explain。
- [ ] `capability_key`、业务上下文、时间语义、tenant-only 边界、发布口径、fail-closed 原则在下游计划中保持一致。
- [ ] 各业务域可以选择不同的值存储实现，但不能再拥有第二套字段规则/候选值/策略事实源。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-105](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/105-dict-config-platform-module.md)
- [DEV-PLAN-105B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/105b-dict-code-management-and-governance.md)
- [DEV-PLAN-106A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md)
- [DEV-PLAN-161](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md)
- [DEV-PLAN-165](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md)
- [DEV-PLAN-184](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md)
- [DEV-PLAN-185](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/185-field-config-dict-values-setid-column-and-master-data-fetch-control.md)
- [DEV-PLAN-200](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/200-composable-building-block-architecture-blueprint.md)
- [DEV-PLAN-070B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md)
- [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md)
