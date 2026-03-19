# DEV-PLAN-345：平台配置与策略（Platform Configuration / Policy）业务规则优先蓝图

**状态**: 规划中（2026-03-18 11:40 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的 `345` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“业务规则优先”的总蓝图；
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 与 [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 关于共享数据合同与租户可扩展能力的建模基线；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 对 capability 命名、路由映射与颗粒度词汇的治理底座冻结；
- 当前仓库中已经成熟沉淀的配置、字典、动态策略、Explain、版本激活与 tenant-only 运行时规则。

`300` 已明确 `Platform.Configuration` 是 Greenfield HR 平台的核心基座之一，但目前这层能力仍然分散在多个专项文档里：

- `105/105B`：字典本体与字典值治理
- `120`：默认值规则与 CEL 执行
- `158`：策略激活与版本一致性
- `161/165/184`：字段配置、动态策略、页面职责与 capability 消费
- `200/202/205`：四层 SoT、确定性决议与页面分工
- `102D`：Context + Rule + Eval 规则执行内核
- `321`：租户可扩展能力的共享业务合同

现状问题不是“缺少零件”，而是**缺少一份把这些零件收敛成平台模块语言的业务蓝图**：

- 当前成熟样板主要来自 Org，容易让人把配置与策略误认为某个业务域的私有附属能力；
- 虽然 [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 已经把 Job Catalog 明确为“组合分类体系 + 可配置化消费域”，但 `345` 还没有把它正式提升为平台首批消费样板之一；
- “字段配置 / 字典配置 / Strategy Registry / 规则引擎 / Explain / 发布”之间已经形成稳定关系，但还没有被提升为 `340` 的正式 SSOT；
- `350/360/370/380/390` 尚缺一份统一的业务需求输入，来说明它们应如何消费“可配置化”这项平台基础能力。

`345` 的任务就是完成这层收口：  
把“配置与策略模块”从实现拼图提升为 **300 蓝图下的平台级业务能力**，并明确它如何作为后续子计划的输入，把“可配置化”确立为 Greenfield HR 平台的通用能力和基石之一。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述当前配置与策略体系，不再以页面、表名或历史实现命名作为主叙事。
- [ ] 全面总结当前仓库中已经稳定沉淀的配置与策略业务规则，区分“现行规则”与“目标平台蓝图”。
- [ ] 冻结 `Platform.Configuration / Policy` 的目标业务蓝图：使命、对象、边界、不变量、时间语义、Explain、发布与审计要求。
- [ ] 冻结 `preview / dry-run / commit / explain` 的最小运行时合同，避免 `350/380/390` 再次各自解释一套决议快照。
- [ ] 明确配置模块与策略模块的职责分层，以及它们与 `321` 的关系：`321` 冻结共享业务合同，`345` 冻结平台模块蓝图与产品输入。
- [ ] 为 `350/360/370/380/390` 提供统一的业务需求输入，要求后续子计划消费配置/策略能力，而不是各自发明第二套“动态规则系统”。
- [ ] 将“可配置化”正式提升为平台基础能力，而不是继续停留在“字段配置页/字典页/某个业务域策略页”的局部经验。
- [ ] 与 `347` 保持命名与颗粒度一致：`capability_key`、上下文与时间锚点语义不在 `345` 内部再定义第二套口径。

### 2.2 非目标

- [ ] 本计划不直接定义最终数据库 DDL、ORM 映射、API 路由与代码实现。
- [ ] 本计划不替代 [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 的共享数据合同，而是在其上定义平台模块的业务蓝图与产品边界。
- [ ] 本计划不要求所有业务域采用同一种物理存储模式；共享的是业务合同与模块边界，不是强推某一套宽表/槽位/扩展表实现。
- [ ] 本计划不把工作流、Assistant、报表、导入导出直接并入配置模块；这些子计划只消费配置与策略模块公开的合同。
- [ ] 本计划不重新引入 `scope_code / scope_type / scope_key / package` 等 legacy 体系，也不允许双写、双读或运行时 fallback 回潮。

## 3. “业务规则优先”在配置与策略模块中的翻译

### 3.1 用户真正维护的不是表、脚本或页面，而是业务承诺

平台管理员、租户管理员和 HR 治理人员真正要维护的不是：

- “哪张表写了哪条记录”
- “哪个页面能改这个字段”
- “哪个 handler 调了哪个策略解析器”
- “某个表达式引擎现在支持哪些内部函数”

他们真正维护的是：

- 哪些业务对象或属性是可配置的；
- 它们在哪些业务能力和上下文下生效；
- 候选值从哪里来，哪些值当前允许；
- 哪些字段必填、可见、可编辑、默认如何生成；
- 这些规则从哪一天起生效，到哪一天结束；
- 发布后为什么命中这个结果，如何解释、审计、回滚。

### 3.2 配置定义“可能性边界”，策略定义“当前决议边界”

`345` 冻结以下平台级理解：

- **配置（Configuration）** 负责定义“系统可以被如何配置”：
  - 可配置项是什么；
  - 候选值池是什么；
  - 哪些定义已启用；
  - 哪些共享基线已发布到租户本地。
- **策略（Policy）** 负责定义“在当前业务能力和上下文下，系统应该如何决议”：
  - visible / required / maintainable
  - default / allowed values
  - capability + context + as_of 下的最终行为

两者共同构成“可配置化”，但不能混为一个主写层。

### 3.3 可配置化是通用能力，不等于“所有业务语义都改成元数据”

`345` 不支持“为了灵活就把一切都做成配置”的错误方向。  
平台必须同时坚持两条原则：

- **可配置化是基石**：字典、扩展定义、租户设置、动态策略、Explain、发布与版本一致性都要成为平台能力；
- **核心业务仍需显式建模**：组织、职位、任职、人员等主对象及其关键不变量，不得因为存在配置模块就被偷换成“JSON 万能口袋”或“任意脚本运行时”。

## 4. 当前基线：已沉淀的业务规则

### 4.1 已稳定的配置规则

#### 4.1.1 字典本体与字典值已经成为平台候选值池 SSOT

- `105/105B` 已冻结：字典本体（dict registry）与字典值（dict values）是平台级事实源。
- 运行时读取口径已收敛为 **tenant-only**；共享基线通过“发布到租户本地”实现，不允许 runtime global fallback。
- `as_of` 必须显式传入，缺失或非法 fail-closed。
- 字典支持 Valid Time、审计、停用与历史读取，已经具备平台候选值池的核心语义。

#### 4.1.2 字段与扩展定义已经形成静态配置层

- `184/205/321` 已冻结：字段配置页负责静态元数据，不再承担运行时动态策略主写。
- 字段身份与命名空间已收敛：
  - 平台/模块内置字段：稳定 `field_key`
  - 字典扩展字段：`d_<dict_code>`
  - 租户自定义字段：`x_<custom_key>`
- `enabled_on/disabled_on`、展示元数据、数据源类型/配置等已经具备稳定口径。

#### 4.1.3 候选值池与允许值集合已经分层

- 候选值池由字典或主数据来源定义“可能有哪些值”。
- `allowed_value_codes` 只表达当前能力上下文下的最终允许子集。
- 运行时必须满足：
  - `allowed_value_codes ⊆ candidate_pool`
  - 默认值若非空，必须命中允许集合
  - 必填字段的最终允许集合不能空而不报错

### 4.2 已稳定的策略规则

#### 4.2.1 动态策略层已经成为运行时唯一主写层

- `184/205` 已冻结：
  - `required / visible / maintainable / default_rule_ref / default_value / allowed_value_codes` 属于 Dynamic Policy SoT；
  - 字段页只保留静态定义与动态镜像，不再承担同语义双写。

#### 4.2.2 capability_key + context + as_of 已成为统一决议协议

- `161/165/200/321/102D` 已冻结：
  - `capability_key` 只表达能力动作，不编码租户、BU、SetID、地域等上下文；
  - 运行时必须先解析 context，再按 `tenant + capability_key + applicability + as_of` 命中策略；
  - 客户端只提供最小业务参数，服务端负责权威回填上下文。

#### 4.2.3 决议必须确定、可解释、可重放

- `200/202/102D` 已冻结：
  - 冲突决议算法必须可复算；
  - `allowed_value_codes` 的求值流程必须可解释；
  - Explain 至少要能回显版本、命中链路、原因码和时间上下文；
  - 缺上下文、缺策略、同位冲突无法化解时一律 fail-closed。

#### 4.2.4 策略版本与激活协议已经成型

- `158` 已冻结：
  - 运行时只允许 `active policy_version` 参与判定；
  - `draft -> active` 切换必须原子；
  - 多实例与缓存场景下以 `policy_version` 作为一致性锚点；
  - 回滚、激活、审计证据链已经有明确方向。

#### 4.2.5 规则表达式能力已具备平台化苗头

- `120` 与 `102D` 已冻结：
  - 规则表达式的保存校验与运行解析必须同口径；
  - 禁止非确定性函数、隐式 today、外部 I/O 等破坏回放稳定性的能力；
  - 默认值、资格过滤、命中决策已不再只是页面逻辑，而是走服务端权威规则链路。

### 4.3 当前主要缺口

尽管上述规则已经较成熟，但还存在四类平台级缺口：

1. **仍然过度依赖 Org 样板表达**  
   当前最成熟的字段/字典/策略能力都围绕 Org 叙述，容易误导后续子计划把这套能力继续写成某个业务域私有能力。

2. **模块所有权尚未收口为 `Platform.Configuration / Policy`**  
   当前我们能看到“字典页”“字段配置页”“Strategy Registry”“Explain/Activation”，但看不到“配置与策略模块”本身的业务蓝图和 ownership。

3. **配置、策略、评估、发布、Explain、审计之间的边界仍靠分散文档维持**  
   虽然每个点都已有计划，但平台层尚无一份总文档明确回答：谁拥有定义、谁拥有候选池、谁拥有动态规则、谁拥有版本激活、谁拥有 Explain 与发布。

4. **后续子计划尚缺统一输入**  
   `350/360/370/380/390` 都会消费配置与策略能力，但目前没有一份平台计划明确要求它们“必须如何消费”，容易再次长出第二套“局部规则系统”。

5. **缺少首批非 Org 消费域样板的正式平台口径**  
   现在我们已经知道 Job Catalog 这类“固定骨架强、同时又确实需要共享可配置化能力”的业务域存在，但 `345` 尚未把它正式定义成平台首批消费样板，导致“哪些应该配置化、哪些绝不能元数据化”仍容易在后续设计里摇摆。

### 4.4 现有文档到 `345` 平台语言的提升关系

| 现有来源 | 现行结论 | `345` 提升后的平台语言 |
| --- | --- | --- |
| `105/105B` | 字典与字典值是平台 SSOT，tenant-only 运行时 | `OptionCatalog` 是平台候选值目录 |
| `120` | 默认值规则由服务端权威执行 | `PolicyRule + DecisionService` 负责默认/校验决议 |
| `158` | `draft/active + policy_version` 原子激活 | `PolicyActivation` 是平台治理能力 |
| `161/165/184/205` | 字段页静态、策略页动态、运行时 capability 决议 | `Configuration Catalog` 与 `Policy Registry` 分层且单一主写 |
| `200/202` | 四层 SoT + 决议确定性 | 配置/策略必须可组合、可复算、可 Explain |
| `102D` | context + rule + eval 统一内核 | `DecisionContext + DecisionSnapshot` 是平台运行时合同 |
| `070B` | 共享改发布，不走 global fallback | `PublicationBatch` 是平台共享基线发布能力 |
| `321` | 租户可扩展能力的共享业务合同已成型 | `345` 负责把这套合同变成平台模块蓝图与产品输入 |

## 5. 平台配置与策略业务蓝图

### 5.1 领域使命

平台配置与策略模块是系统内“**哪些能力可配置、候选值从哪里来、在什么上下文下如何决议、为什么得到当前结果、版本如何激活与发布**”的唯一业务权威。  
它不是某个业务域的附属页面，也不是单纯的技术规则引擎，而是所有业务域、工作流、数据工作台和 Assistant 都要建立其上的平台基石。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由平台配置与策略模块拥有 |
| --- | --- | --- |
| `ConfigDefinition` | 可配置项本体：是什么、属于哪个业务对象/能力、值类型与展示语义是什么 | 是 |
| `ConfigActivation` | 某租户或某模块在某一时间窗口内启用了哪些定义 | 是 |
| `OptionCatalog` | 候选值池：字典、本地基线、只读主数据引用等“可能有哪些值” | 是 |
| `PolicySet` | 同一能力上下文下的一组版本化策略集合 | 是 |
| `PolicyRule` | 单条策略：visible/required/maintainable/default/allowed 等判定项 | 是 |
| `DecisionContext` | 决议配置与策略时所依赖的业务上下文 | 共享合同由平台拥有，具体解析逻辑由消费域提供 |
| `DecisionSnapshot` | 运行时合成后的决议快照与 Explain 结果 | 是（派生合同，不应成为第二主写源） |
| `PolicyActivation` | draft/active/rollback 与版本一致性治理 | 是 |
| `PublicationBatch` | 共享基线发布到租户本地的批次、回执与幂等记录 | 是 |
| `ConfigPolicyAuditLog` | 配置变更、发布、激活、Explain、回滚的审计证据 | 是 |
| `DomainBusinessRecord` | 业务域上的真实值，例如 Org/Position/Assignment/Person 的扩展值与业务字段 | 否，领域模块拥有 |

### 5.3 面向用户的主能力

- 新增或启用可配置项
- 管理候选值池与共享基线发布
- 为特定业务能力与上下文配置动态策略
- 预览 `current / as_of / history` 视角下的决议结果
- 查看 Explain、版本、命中链路与审计记录
- 激活、回滚、停用、替换配置与策略版本
- 让业务表单、详情页、列表、导入导出、报表与 Assistant 复用同一套决议结果

### 5.4 平台内部分域（Subdomains）与 ownership

| 子域 | 平台负责什么 | 平台不负责什么 |
| --- | --- | --- |
| `Configuration Catalog` | 可配置项定义、命名空间、启停窗口、展示元数据 | 运行时当前是否必填/可见/可编辑 |
| `Option Catalog` | 候选值池、字典本体、字典值、共享基线发布 | 当前上下文下最终允许哪些值 |
| `Policy Registry` | 动态策略、版本链、能力上下文裁剪、激活状态 | 业务主数据真值 |
| `Decision Service` | 决议预览、Explain、版本绑定、Preview/Dry-run 合同 | 直接拥有业务写模型 |
| `Publication & Governance` | draft/active、发布、回滚、审计、审批输入 | 替代工作流、替代业务模块的最终提交 |

### 5.5 首批消费域样板与边界

`345` 选定两类首批消费样板，用来证明“可配置化是通用能力，但不会吞掉业务骨架”：

| 首批消费域 | 平台配置与策略模块提供什么 | 领域自己必须继续拥有什么 |
| --- | --- | --- |
| `Org` | 扩展字段定义、候选值池、动态策略、Explain、发布与版本治理 | 组织身份、层级、生命周期、生效历史、组织主不变量 |
| `Job Catalog` | Group / Family / Level / Profile 的扩展属性定义、候选值池、动态策略、Explain、共享基线发布 | 统一目录骨架、Group/Family/Level/Profile 固定骨架、Family 归属、Profile 主 Family、按 `OrgContext + as_of` 解释的分类事实 |

这条边界非常关键：

- 平台可以决定“某个分类对象的扩展属性如何被定义、裁剪、解释”；
- 但平台不能把 Job Catalog 的核心分类骨架改造成“任意树 + 任意字段 + 任意脚本”的元数据系统；
- `Group / Family / Level / Profile`、统一目录骨架、`current/as_of/history`、Profile 至少一个 Family 且恰好一个主 Family，仍然属于业务域固定骨架。

## 6. `345` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 定义可配置项 | 告诉系统“哪些属性或设置可以被租户治理” | 定义与运行时策略分层；键稳定；命名空间可解释；未知或冲突键拒绝 | 平台获得可治理的配置目录 |
| 管理候选值池 | 告诉系统“这些属性可从哪些候选值中取值” | 候选池由平台 SSOT 管理；tenant-only 运行时；共享基线通过发布而非 fallback | 平台拥有稳定可解释的选项目录 |
| 配置动态策略 | 告诉系统“在当前能力/上下文下应该如何表现” | `capability_key` 稳定；context 显式；同一语义仅一套动态主写；决议可重放 | 形成可解释、可版本化的行为规则 |
| 预览决议 | 在提交前确认“当前/某日到底会发生什么” | `as_of` 显式；决议输出带版本、Explain、原因码；预览与提交使用同口径 | 用户与系统对规则结果达成一致理解 |
| 激活或回滚版本 | 将 draft 变 active，或恢复旧版本 | 同租户只能有一个 active；切换原子；运行时只读 active；回滚可审计 | 多实例下得到一致的运行时行为 |
| 发布共享基线 | 把平台维护的候选值或定义写入租户本地 | 发布幂等、可回放、可审计；运行时不跨租户读取 | 租户本地拥有可治理基线 |
| 提交业务数据 | 在业务域上真正写入配置化后的结果 | 服务端二次校验；版本过期拒绝；非法值/未启用/未发布/缺上下文一律 fail-closed | 业务数据与当前配置/策略保持一致 |
| 审计与排障 | 回答“为什么现在是这样，谁改了什么” | Explain 与审计并存；版本、命中链路、发布时间、操作者必须可追踪 | 满足治理、排障、合规与 Assistant 解释需求 |

## 7. 共享不变量与边界

### 7.1 五层共享合同

| 层次 | 共享事实源 | 负责回答的问题 | 不负责什么 |
| --- | --- | --- | --- |
| 定义层 | `ConfigDefinition + ConfigActivation` | 这个可配置项是什么、在何时存在 | 当前上下文下是否必填/可见 |
| 候选层 | `OptionCatalog` | 这个配置项可能有哪些值 | 当前上下文下最终允许哪些值 |
| 策略层 | `PolicySet + PolicyRule` | 在当前能力上下文下应该如何裁剪与约束 | 业务值本身的领域真值 |
| 决议层 | `DecisionContext + DecisionSnapshot` | 当前/某日到底命中了什么结果，为什么 | 成为第二主写事实源 |
| 治理层 | `PolicyActivation + PublicationBatch + ConfigPolicyAuditLog` | 版本如何激活、发布、回滚、审计 | 替代业务流程审批与最终提交 |

### 7.2 时间与版本合同

- 业务生效时间统一为 day 粒度；
- 审计时间只回答“何时操作”，不替代业务生效时间；
- 读取必须显式区分：
  - `current`
  - `as_of`
  - `history`
- 运行时只允许 `active` 版本进入命中链路；
- 当提交结果由多层快照组合而成时，必须能绑定稳定的版本锚点，至少包括：
  - `policy_version`
  - 必要时的组合快照版本指纹

### 7.3 上下文合同

`345` 冻结以下最小共享表达：

- `tenant`
- `capability_key`
- `applicability_kind`
- `applicability_ref`
- `as_of`

冻结原则：

- `capability_key` 只表达能力动作，不表达上下文；
- 业务上下文必须通过显式字段解析，不允许回流进 key 命名；
- 客户端不拥有“完整上下文自由上传”权限，服务端必须做权威回填；
- 不得把共享合同重新命名回 legacy 的 `scope/package` 体系。

### 7.4 发布与运行时合同

- 共享基线只能通过发布写入租户本地；
- 运行时只读取租户本地数据；
- 缺基线、缺发布、缺策略、缺上下文一律 fail-closed；
- 发布本身必须具备幂等、回执、审计与可回放能力。

### 7.5 Explain 与 fail-closed 合同

最小 Explain 合同至少要能回答：

- 采用了哪个 `capability_key`
- 使用了哪个 `as_of`
- 命中了哪个版本
- 命中了哪些策略或规则
- 为什么允许/拒绝

缺少这些信息，就不能称为平台级配置与策略能力。

### 7.6 运行时合同（Preview / Dry-run / Commit / Explain）

为阻断“页面、报表、Assistant 先各自拼一版决议，再回头对齐”的漂移，`345` 额外冻结以下最小运行时合同：

- 所有消费方都必须共享同一条解析链路：
  - `DecisionContext`
  - `DecisionSnapshot`
  - `Explain`
  - `commit` 前二次校验
- `DecisionContext` 至少应稳定承载：
  - `tenant`
  - `business_object_key`
  - `capability_key`
  - `org_context`（或可映射到 `applicability_kind/ref` 的等价共享表达）
  - `time anchor`（`as_of` 或 `effective_date`）
  - 请求对象标识与待校验字段集合
- `DecisionSnapshot` 至少应稳定回显：
  - `decision_snapshot_id` 或等价稳定快照标识
  - `policy_version`
  - 必要时的组合快照版本指纹 / `snapshot_hash`
  - 命中的字段决议结果、允许值集合、默认值结果与只读原因
  - 对应 Explain 引用
- `preview`、`dry-run` 与最终 `commit` 必须复用同一套决议语义，不允许：
  - preview 看一套规则
  - 提交时另走一套本地推导
  - 导出/Assistant 再各自补默认值或字段可见性
- 只要提交依赖过运行时决议，`commit` 必须至少绑定：
  - `policy_version`
  - `decision_snapshot_id` 或等价 `snapshot_hash`
  - `org_context + time anchor`
- 当上述锚点任一失效时，提交必须 fail-closed，并明确返回“需要重新预览/重新确认”的稳定错误语义。

### 7.7 字段退役、替换与迁移生命周期

平台级可配置化不只负责“新增字段”，还必须正式拥有“如何安全退役字段”的治理合同：

- 字段生命周期至少要区分：
  - `active`
  - `deprecated`
  - `disabled`
  - `archived`（仅历史可读，不再进入当前运行时）
- `deprecated` 的语义是“未来不再建议新用”，不是立刻删除历史，也不是偷偷从 Explain 中消失。
- 字段替换必须显式声明：
  - 被替换字段
  - 替代字段
  - 生效窗口
  - 是否需要历史迁移 / 回填
  - 对导出、报表、Assistant 的兼容解释窗口
- 字段停用后必须同时满足：
  - 当前运行时不再允许新写入
  - `history` 仍可解释旧值来源
  - 导出与审计仍能说明“该字段为何出现、何时退役、由谁触发”
- 任何字段迁移、改键、替换、批量回填都必须进入审计与发布/治理轨道，不允许通过脚本式暗改形成“僵尸字段”。

### 7.8 领域实现护栏

- 各业务域可以拥有各自的值存储实现，但不能拥有第二套：
  - 可配置项目录
  - 候选值池目录
  - 动态策略系统
  - Explain 体系
- 工作流、报表、Assistant 可以消费配置与策略模块，但不能重写它的主规则；
- 平台配置与策略模块也不能反向拥有业务域主数据真值或审批状态真值。

### 7.9 Job Catalog 对平台可配置化的专项护栏

为了避免后续把“可配置化”错误理解成“Job Catalog 全元数据化”，`345` 对 Job Catalog 额外冻结以下护栏：

- 平台可配置化只能覆盖 Job Catalog 的**扩展属性层**，不能替代其**固定分类骨架**；
- 以下内容不得被配置化侵蚀：
  - `Group / Family / Level / Profile` 四类核心对象
  - `Family -> Group` 的归属语义
  - `Profile -> Families` 的关联语义
  - 至少一个 Family + 恰好一个主 Family
  - 包 ownership、只读共享与 tenant-only 运行时
  - `current / as_of / history` 时间语义
- 平台需要支持 Job Catalog 的首批共享合同：
  - 以 `Group / Family / Level / Profile` 作为可扩展对象
  - 以创建、归属调整、详情编辑、下游引用作为能力上下文
  - 以共享基线发布到租户本地作为运行前提，而不是 runtime fallback

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `340`（平台与 IAM 基座）的输入

- [ ] 平台必须拥有 `Configuration Catalog / Option Catalog / Policy Registry / Decision Service / Publication & Governance` 这五类基座能力。
- [ ] 权限模型至少要能区分：查看定义、管理定义、管理候选值池、管理动态策略、执行发布/激活/回滚、读取 Explain/审计。
- [ ] `tenant-only runtime + active-only policy + publish-not-fallback` 必须由平台底座显式保证。

### 8.2 对 `350`（前端产品壳与交互系统）的输入

- [ ] 需要统一的治理页模式：配置目录、候选值目录、策略页、决议预览、Explain/历史、发布/激活记录。
- [ ] 页面必须显式标识“静态来源 / 动态来源 / 决议快照来源”，防止再次出现“改了但不生效”的认知错位。
- [ ] 业务表单、详情页、列表页只能消费共享决议，不得自行猜测字段行为。
- [ ] `350` 必须把 `DecisionContext / DecisionSnapshot / Explain` 翻译为稳定的产品语言与页面槽位，默认通过 `351 / 352 / 353` 接线，不允许在前端层重写第二套规则解释。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] 各业务域只能消费平台配置与策略能力，不重复发明本模块专属的字段策略/字典/Explain 系统。
- [ ] 各业务域需要显式声明自己支持哪些 `applicability_kind`，以及如何把业务上下文映射到共享合同。
- [ ] 每个新接入共享可配置化能力的业务域，都必须提交一份最小接入清单：`business_object_key`、支持的 `applicability_kind`、扩展值承载位置、`current/as_of/history` 语义、导出扁平化方式、Assistant 回落面与所需 Explain 槽位。
- [ ] 领域模块拥有业务值本身与领域不变量，但“字段/选项/策略为何如此”由平台模块提供合同。
- [ ] `360` 中 Job Catalog 必须作为首批正式消费域之一：`Group / Family / Level / Profile` 的扩展属性走平台共享能力，但分类包、分类骨架、Family 归属与 Profile 主 Family 仍由业务域拥有。
- [ ] `360` 中 Job Catalog 的 `current / as_of / history`、owner/read-only、共享基线发布语义，必须能被平台配置与策略模块消费和 Explain，但不得被平台模块重写为第二主模型。
- [ ] `360` 中 Job Catalog 与 Staffing 的衔接，必须走“共享决议 + 业务域固定骨架”的双层模式：平台负责解释哪些扩展属性和允许值生效，业务域负责解释哪个 Profile/Level 在某日有效。

### 8.4 对 `370`（工作流、审计增强与集成）的输入

- [ ] 工作流至少要区分：定义变更、候选池变更、策略变更、发布、激活、回滚六类治理动作。
- [ ] 工作流/审计还必须正式支持字段退役、字段替换、批量迁移/回填三类治理动作，并在回执中回显受影响 `field_key`、替代关系与生效窗口。
- [ ] 审计增强不得重写配置与策略模块的主规则，只能叠加审批轨迹、执行回执与外部集成回执。
- [ ] 外部系统若消费配置与策略结果，必须显式声明采用 `current`、`as_of` 还是 `history` 视图。

### 8.5 对 `380`（数据工作台与运营分析）的输入

- [ ] 导入、导出、报表和查询工作台必须理解配置/策略的 `current / as_of / history` 语义。
- [ ] 数据质量工作台至少要能发现：未发布即使用、策略缺失、允许值越界、版本过期、租户基线缺失、Explain 缺失。
- [ ] 查询与导出不得重新拼出第二套“动态字段字典”或“局部白名单规则”。
- [ ] 查询与导出必须消费统一的动态字段合同，至少能稳定回显：列标识、展示名、值类型、来源对象、是否可筛选/可排序、字段血缘与退役状态。
- [ ] 数据工作台应具备“决议健康度”视角，至少能发现：`required=true` 但允许集合为空、字段已退役仍被活跃使用、preview/commit 锚点漂移、基线缺失、Explain 缺失。

### 8.6 对 `390`（Chat Assistant）的输入

- [ ] Assistant 读取配置与策略时，必须通过目录、预览、Explain、版本和候选池合同，不得凭提示词猜测字段语义。
- [ ] Assistant 发起可写动作时，必须遵循与 UI 完全一致的版本绑定、Dry-run、Explain 与 fail-closed 约束。
- [ ] 任何由共享决议驱动的 Assistant 写动作，都必须绑定 `decision_snapshot_id` 或等价 `snapshot_hash`，并与 `policy_version + org_context + time anchor` 一起进入最终确认/提交锚点。
- [ ] 当字段、候选值、上下文或版本存在歧义时，Assistant 必须先澄清，而不是自行补完。

## 9. 建议实施分期

1. [ ] `M1`：词汇与 ownership 冻结  
   统一“定义 / 候选池 / 策略 / 决议 / Explain / 发布 / 激活 / 回滚”词汇，停止继续用 Org 私有页面命名替代平台语言。
2. [ ] `M2`：配置目录与候选目录合同冻结  
   抽出统一 `ConfigDefinition + OptionCatalog` 合同，并明确 tenant-only + publish-not-fallback 边界。
3. [ ] `M3`：策略目录与决议合同冻结  
   抽出统一 `PolicySet + PolicyRule + DecisionContext + DecisionSnapshot` 合同，并冻结版本锚点。
4. [ ] `M4`：运行时提交与字段生命周期合同冻结
   冻结 `preview / dry-run / commit / explain` 最小合同、提交锚点、字段退役/替换/迁移语义与审计边界。
5. [ ] `M5`：平台治理能力冻结
   冻结 `draft/active/rollback/publication/audit/explain` 平台能力与审批输入边界。
6. [ ] `M6`：首批消费域收敛
   以 Org 与 Job Catalog 为首批样板，把现有字段、字典、策略、Explain 路径正式收敛为平台模块语言与可复用合同，并验证“固定骨架 + 共享可配置层”边界可同时成立。
7. [ ] `M7`：跨子计划接线
   将 `350/360/370/380/390` 中涉及配置与策略的描述统一引用 `345`，不再各自重写主规则。

## 10. 验收标准

- [ ] `Platform.Configuration / Policy` 已能脱离当前 Org 实现细节被独立理解和评审。
- [ ] 当前仓库中已成熟的配置/策略规则已被系统总结，并明确区分为“配置层 / 候选层 / 策略层 / 决议层 / 治理层”。
- [ ] `345` 与 `321` 的分工清晰：`321` 冻结共享业务合同，`345` 冻结平台模块蓝图与后续子计划输入。
- [ ] `preview / dry-run / commit / explain` 已具备单一运行时合同，`350/380/390` 不再各自推导第二套提交锚点或决议快照结构。
- [ ] 字段退役、字段替换、批量迁移/回填已经具备正式治理语义，不再以脚本暗改或页面隐藏代替生命周期管理。
- [ ] `350/360/370/380/390` 可直接引用 `345` 作为配置与策略模块的业务需求输入，而不是继续自行发明第二套规则系统。
- [ ] “可配置化是平台基石之一”已在文档结构、模块边界、产品输入和治理口径上被正式确立。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-105](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/105-dict-config-platform-module.md)
- [DEV-PLAN-105B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/105b-dict-code-management-and-governance.md)
- [DEV-PLAN-120](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md)
- [DEV-PLAN-158](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/158-capability-key-m6-policy-activation-and-version-consistency.md)
- [DEV-PLAN-161](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md)
- [DEV-PLAN-165](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md)
- [DEV-PLAN-184](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md)
- [DEV-PLAN-200](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/200-composable-building-block-architecture-blueprint.md)
- [DEV-PLAN-202](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/202-blueprint-policy-resolution-and-allowed-values-determinism.md)
- [DEV-PLAN-205](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/205-blueprint-page-responsibility-convergence-static-dynamic-sot.md)
- [DEV-PLAN-102D](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md)
- [DEV-PLAN-070B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md)
