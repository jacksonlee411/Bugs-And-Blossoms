# DEV-PLAN-363：职位分类（Job Catalog）业务规则优先蓝图与可配置化基座方案

**状态**: 规划中（2026-03-18 15:20 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 的 `M3: Job Catalog` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“业务规则优先”的方法论；
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 关于“租户可扩展能力是共享合同”的抽象；
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 关于“可配置化是平台基础能力”的模块蓝图；
- 当前仓库中已经落地的 Job Catalog 规则、界面与下游引用契约。

`300` 对职位分类域最大的提醒，不是“先决定表结构或接口动作”，而是：

- 先定义企业真正维护的分类体系是什么；
- 先定义分类在某一天如何生效、如何被职位/任职引用、如何被用户理解；
- 再决定内部如何映射为写入口、版本切片、配置项、Explain 与审计。

当前仓库中，职位分类相关规则已经分别沉淀在：

- `029`：Job Catalog DB Kernel、事件/版本、不变量与快照；
- `104/104A`：Job Catalog UI 的上下文与交互模式；
- `102B`：`current / as_of / history` 时间语义；
- `030`：Position 对 JobCatalog 的引用约束；
- `321/345`：可配置化、字段/字典/策略/Explain 的共享合同与平台蓝图；
- `060/062`：端到端业务验证中的职位分类样板。

但目前仓库里仍缺一份以**业务语言**表达的 Job Catalog SSOT。  
`363` 的任务就是完成这层收口：把职位分类模块从“已有实现集合”提升为 **360 的正式子计划 + 下游子计划的业务需求输入**，并明确“可配置化”如何成为这个领域的通用能力和基石之一，而不是局部补丁。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述职位分类域，不再以 `CREATE/UPDATE/DISABLE`、表名、函数名作为主叙事。
- [ ] 全面总结当前仓库中已经稳定沉淀的 Job Catalog 业务规则，并区分“现行规则”与“目标蓝图”。
- [ ] 冻结职位分类域的目标业务蓝图：对象、场景、边界、不变量、时间语义、组织上下文、下游引用语义。
- [ ] 明确职位分类域的“固定骨架”与“可配置化层”边界，避免把可配置化误做成模块私有动态字段补丁。
- [ ] 把 Job Catalog 冻结为首批共享可配置化样板，补齐接入清单与样板验收矩阵，供 `345/350/380/390` 复用。
- [ ] 作为 `360` 的详细拆分，为 `345/350/364/370/380/390` 提供可直接消费的业务需求输入。

### 2.2 非目标

- [ ] 本计划不直接定义最终数据库 DDL、迁移脚本、ORM 映射与 API 路由实现；这些仍以 [DEV-PLAN-029](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md) 及后续实施为准。
- [ ] 本计划不替代 [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 与 [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 的共享合同/平台蓝图，而是定义 Job Catalog 如何消费它们。
- [ ] 本计划不把职位分类域重写成“任意树 + 任意 JSON + 任意脚本”的元数据系统。
- [ ] 本计划不重新引入 legacy 双读、global fallback、第二写入口或 `package_uuid/setid` 双主源语义。

## 3. “业务规则优先”在职位分类域中的翻译

### 3.1 用户维护的是分类体系，不是底层动作名

职位分类域里，用户真正维护的不是：

- `create_job_family_group`
- `update_job_family_group`
- `submit_job_profile_event`
- `job_profile_version_job_families`

他们真正维护的是：

- 某个租户/组织上下文下有哪些职位分类维度；
- 某个 `as_of` 日期下，哪些 Group、Family、Level、Profile 有效；
- 某个 Profile 归属于哪些 Family，哪个是主 Family；
- 某个分类视图当前是否可编辑，还是因安全/策略/共享基线而只读；
- 某些附加属性、候选值、默认值、可见性和可编辑性为什么是这样。

因此，`363` 冻结以下表达顺序：

1. 先定义业务对象；
2. 再定义业务规则；
3. 最后才定义内部如何映射到事件、SQL、API 和 UI 动作。

### 3.2 职位分类不是“一棵单树”，而是组合分类体系

当前实现已经说明，Job Catalog 的真实结构不是单一树，而是：

- `Group -> Family` 的层级骨架；
- `Profile -> Families` 的多归属关系；
- `Level` 的平行维度；
- `OrgContext + 统一安全/策略` 的治理上下文。

也就是说，职位分类域是一个**组合分类系统**，而不是“把所有节点强行塞进一棵树”。

### 3.3 生效日期与组织上下文是一级业务能力

在 `300` 与当前实现中，职位分类都不是“当前态字典”：

- `effective_date` 是业务生效时间；
- `as_of` 是读取观察时间；
- `current / as_of / history` 必须显式区分；
- `OrgContext + 权限/策略 + 发布状态` 决定当前维护的是哪套分类事实与是否可写。

### 3.4 可配置化是共享基础能力，不是模块局部补丁

职位分类天然需要“可配置化”，但正确方向不是把核心结构做成任意元数据，而是：

- 核心分类骨架必须显式建模；
- 可扩展属性、候选值、动态规则、Explain 与发布能力由平台共享提供；
- Job Catalog 负责消费共享能力，不得自建第二套字段/策略/Explain 系统。

## 4. 职位分类业务蓝图（目标形态）

### 4.1 领域使命

职位分类域是平台内“**岗位分类骨架、分类层级关系、组织上下文下的分类视图、按日生效的分类记录、职位画像归属语义及其对下游引用的唯一业务权威**”。  
它既服务于 HR 管理者维护分类体系，也服务于 Staffing、Reporting、Workflow 与 Assistant 统一消费“某天到底有效的分类事实”。

### 4.2 核心业务对象

| 业务对象 | 业务含义 | 363 是否视为 JobCatalog 拥有 |
| --- | --- | --- |
| `JobCatalogViewContext` | 某次读取/维护时的 `OrgContext + as_of + read_only` 视图上下文 | 否，由平台访问模型提供，JobCatalog 消费 |
| `JobFamilyGroup` | 职位族群，承接 Family 的上层分类 | 是 |
| `JobFamily` | 职位族，归属于某个 Group | 是 |
| `JobLevel` | 职级/等级维度，独立于 Group/Family 树 | 是 |
| `JobProfile` | 职位画像，可归属多个 Family 且必须有一个主 Family | 是 |
| `JobProfileFamilyLink` | Profile 与 Families 的版本化关联 | 是 |
| `JobCatalogRecord` | 某对象在某个生效日起的一条业务记录 | 是 |
| `JobCatalogView` | 某个 `current/as_of/history` 口径下对分类体系的观察结果 | 是 |
| `ClassificationAttributeDefinition` | 分类对象的扩展属性定义 | 否，由 `321/345` 拥有，JobCatalog 消费 |
| `ClassificationPolicyDecision` | 某能力上下文下字段可见/必填/默认/允许值的运行时决议 | 否，由 `345` 提供共享决议合同，JobCatalog 消费 |
| `Position / Assignment` | 下游职位与任职事实 | 否，由 `364`/现有 staffing 契约拥有 |

### 4.3 面向用户的主能力

- 选择并进入某个组织上下文下的职位分类视图；
- 浏览某日有效的 Groups / Families / Levels / Profiles；
- 新建 Group、Family、Level、Profile；
- 调整 Family 归属 Group；
- 查看分类在某日是否有效以及从哪天开始生效；
- 让下游 Position/Assignment 能引用稳定的 Job Profile 事实；
- 通过共享配置能力看到附加属性、候选值、默认值和 Explain；
- 在审计、报表、Assistant 中复用同一套分类语义。

### 4.4 页面与交付语言

按照 `300 + 350 + 104` 的产品语言，职位分类最终应统一呈现为：

- 上下文工作台：回答“我正在维护哪个组织上下文下的分类体系、哪一天的分类事实、当前是否只读”
- 列表：回答“当前有哪些 Group / Family / Level / Profile”
- 详情：回答“这个分类对象在某个生效日是什么样”
- 历史：回答“这个分类对象是如何演进的”
- Explain / 审计：回答“为什么这个字段可写/不可写、为什么这个 Profile 属于这些 Family”

## 5. 当前基线：已沉淀的业务规则

### 5.1 现行且已落地的规则

#### 5.1.1 组织上下文、可写性与共享基线语义已经成立

- Job Catalog 现有实现曾使用 `package_uuid/setid` 承载上下文与可写性，但在 `348C` 裁决后，这部分必须统一收口为 `OrgContext` 入口与领域对象主键，不再保留容器键。
- 读取与写入都必须先在指定 `as_of/effective_date` 与 `OrgContext` 下解析出唯一有效的分类视图。
- 运行时不允许 global fallback；共享基线必须通过发布/本地化落地。
- `read_only` 必须由统一安全/策略/发布状态决定，而不是再借由 owner package 隐式表达。
- 无法形成唯一视图时必须澄清或拒绝，不允许默认进入某个隐藏目录容器。

#### 5.1.2 身份、编码与不可变语义已经成立

- Group / Family / Level / Profile 都有稳定 identity（UUID）与业务 code。
- 当前按 `package` 分隔 `code` 唯一性的做法不再作为目标口径；目标是 `code` 按对象类型在租户内稳定唯一，或由显式业务规则说明其适用上下文，但不得再依赖隐藏容器键制造同码并存。
- `code` 只允许在创建时给定，后续更新不得修改。
- 未知字段、空名称、非法类型、非法 UUID、非法 payload 一律 fail-closed。
- 同一实体同一生效日只允许一条事件；重复 `event_uuid` 但参数不同会被视为幂等复用错误。

#### 5.1.3 有效期模型已经稳定

- 业务时间统一为 `date` 日粒度。
- 读口径必须显式传 `as_of`，写口径必须显式传 `effective_date`。
- 版本切片遵循：
  - `no-overlap`
  - `gapless`
  - 最后一段 `upper_inf(validity)=true`
- `DISABLE` 通过 `is_active=false` 的有效期切片表达，而不是靠“缺行”表达停用。
- `CREATE` 必须是首个事件；`UPDATE/DISABLE` 必须建立在既有状态之上。

#### 5.1.4 分类骨架与引用规则已经稳定

- `JobFamilyGroup` 是 `JobFamily` 的上层容器。
- 新建 Family 时必须引用同一 tenant 下、在当前 `OrgContext + as_of` 可见的 Group。
- Family 支持按 effective date 发生 reparenting，这说明“Group 归属”是有效期业务事实，而不是静态元数据。
- `JobLevel` 当前是平行维度，不挂在 Group / Family 下。
- `JobProfile` 必须满足：
  - 至少关联 1 个 Family；
  - `job_family_uuids` 不可为空；
  - 不可重复；
  - 必须恰好有 1 个主 Family；
  - 主 Family 必须属于关联集合；
- 所有关联 Family 必须存在于同一 tenant 下，且在当前 `OrgContext + as_of` 可见。

#### 5.1.5 UI 上下文与写入模式已经稳定

- `/app/jobcatalog` 已经收敛为“上下文栏 + Tabs 工作区”模式，而不是 API 调试面板。
- `as_of + org_context + read_only` 是页面顶层上下文，不能在滚动中丢失。
- 无选择上下文时返回空列表，而不是偷偷帮用户补出默认分类视图。
- 每次写操作都必须单独选择 `effective_date`；不能依赖页面隐式默认写入日期。
- 每个局部任务域只保留一个 Primary action；写操作走短事务 Dialog。

#### 5.1.6 事实源与下游引用规则已经成立

- Job Catalog 的快照读取已经基于统一版本事实源生成，而不是前端拼装临时结构。
- Position 等下游对象应引用“某日有效的 JobCatalog 事实”，而不是自行复制分类语义。
- 领域写入口已经遵守 One Door，不能直写 versions/relations 表绕过规则。

### 5.2 当前已冻结、需由 363 正式承接的目标规则

以下结论在现有实现和计划中已经隐含成立，但仍需要 `363` 正式收口为业务 SSOT：

- 职位分类域应被描述为“组合分类体系”，而不是笼统的“分类树”。
- 用户语言应从 `create/update/disable` 升级为“建立分类项 / 调整归属 / 生效于某日 / 只读共享 / 配置属性与规则”。
- 组织上下文、统一安全/策略导致的 `read_only`、共享基线发布等治理语义需要从实现细节提升为正式业务规则。
- Job Catalog 的可配置化不应停留在某个页面/某几个扩展字段，而应被纳入 `321/345` 的通用能力体系。
- 下游 `364/370/380/390` 必须消费这套职位分类语义，不能各自重新定义“职位族/等级/画像”的含义。

## 6. `363` 冻结的目标业务规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 进入分类视图 | 决定当前维护哪套职位分类事实 | 必须显式确认 `OrgContext`；`as_of` 明确；`read_only` 由统一安全/策略结果决定；无唯一视图时必须澄清 | 用户进入正确且可解释的分类上下文 |
| 新建 Group | 为当前分类视图建立新的族群维度 | code 按对象类型在租户内稳定唯一；名称必填；自生效日起可见 | 形成新的 Group 身份与首条生效记录 |
| 新建 Family | 在某个 Group 下建立职位族 | 必须归属已存在 Group；code 按对象类型在租户内稳定唯一；按日生效 | 形成可被 Profile/Staffing 引用的 Family |
| 调整 Family 归属 | 改变某个 Family 在某日起属于哪个 Group | 归属变更是 effective-dated 事实；不能破坏历史；目标 Group 必须存在 | 同一 Family 在不同日期可有不同归属 |
| 新建 Level | 建立等级维度 | Level 是平行维度，不依赖 Group/Family 树；code 按对象类型在租户内稳定唯一 | 形成可复用的等级主数据 |
| 新建/维护 Profile | 定义职位画像与其 Family 归属 | 至少一个 Family；恰好一个主 Family；关联必须在当前 `OrgContext + as_of` 下可见且有效；按日生效 | 形成可被 Position/Assignment 稳定引用的 Profile |
| 停用分类项 | 让某个分类项从某日起不再可用 | 停用是有效期状态，不是删除历史；历史仍需可读 | 分类项停止未来使用但保留历史 |
| 浏览 current/as_of/history | 看清“现在/某日/全部历史” | 不允许隐式 today；三种时间视角必须共存；列表、详情、快照口径一致 | 用户、报表和 Assistant 看到同一事实 |
| 使用共享基线 | 浏览平台/共享基线发布来的分类体系 | 共享通过发布进入租户本地；可见性与只读状态由统一安全/策略模型决定；不能 runtime fallback | 共享基线可用但不制造第二事实源 |
| 配置分类扩展能力 | 为分类对象增加扩展属性、候选值、默认值、约束与 Explain | 核心骨架显式建模；扩展定义/候选值/策略由 `321/345` 提供；服务端 fail-closed | 职位分类既稳定又可按租户治理扩展 |

## 7. 可配置化作为通用能力与基石之一

### 7.1 先冻结“固定骨架”，再开放“可配置层”

`363` 选定的方向不是“把 Job Catalog 全做成元数据”，而是“双层结构”：

- **固定骨架（JobCatalog 自己拥有）**
  - `Group`
  - `Family`
  - `Level`
  - `Profile`
  - `Profile -> Families`
  - `effective_date / as_of / history`
  - `OrgContext + read_only + published baseline`
- **可配置层（共享能力提供，JobCatalog 消费）**
  - 扩展属性定义
  - 候选值池
  - `visible / required / maintainable`
  - `default / allowed values`
  - Explain / 预览 / 审计 / 发布

### 7.2 Job Catalog 对共享可配置化能力的正式需求

Job Catalog 不是可配置化能力的所有者，但它需要把以下需求正式输入给 `321/345`：

| 共享层 | Job Catalog 需要什么 | 作用对象 |
| --- | --- | --- |
| 定义层 | 为 Group / Family / Level / Profile 定义可扩展属性 | 分类对象本体 |
| 候选值层 | 为扩展属性提供字典/主数据候选池 | 分类对象的扩展字段 |
| 策略层 | 按能力上下文裁剪字段可见性、必填、默认值、允许值 | 新建、调整归属、详情编辑、下游引用 |
| 决议层 | 返回运行时最终结果与 Explain | UI、导入、Assistant、API 提交 |
| 治理层 | 提供版本激活、发布、回滚、审计 | 分类配置变更与共享基线 |

### 7.3 首批应纳入共享可配置化能力的场景

`363` 建议将以下场景作为 Job Catalog 对平台可配置化能力的首批正式输入：

- Group / Family / Level / Profile 的扩展属性定义与启停窗口；
- 分类属性的候选值来源管理，例如字典、租户本地主数据、只读引用；
- 按业务能力上下文裁剪字段行为，例如“新建 Profile 时哪些扩展字段可见/必填/可维护”；
- 下游引用场景中的策略决议，例如“某天某组织/某能力下哪些 Profile/Level 可被选中”；
- 列表/详情/导出/Assistant 所需的 Explain 结果回显；
- 共享分类基线发布到租户本地后的审计与版本治理。

### 7.4 严禁被“可配置化”侵蚀的核心不变量

以下规则是 `363` 明确禁止配置化冲掉的固定骨架：

- `effective_date / as_of / history` 的时间语义；
- 包 ownership、只读共享与 tenant-only 运行时；
- Group / Family / Level / Profile 四类核心业务对象；
- Family 必属某个 Group；
- Profile 至少一个 Family 且恰好一个主 Family；
- 核心 code 的不可变与包内唯一；
- 服务端 fail-closed 与统一事实源；
- 下游对 JobCatalog 主语义的引用边界。

### 7.5 结论：可配置化是基石，但不是“无限元数据化”

对于职位分类域，正确的结论是：

- **可配置化必须被提升为平台通用能力和基石之一**；
- **但 Job Catalog 的核心分类骨架仍然必须显式建模并由业务规则直接拥有**。

这正是 `300 / 321 / 345` 与当前实现共同指向的方向。

### 7.6 Job Catalog 作为首批样板的接入清单

为了让 Job Catalog 真正成为“平台共享可配置化能力”的首批正式样板，而不是只停留在口头样板，`363` 额外冻结以下接入清单：

- 每个可扩展对象都必须显式声明：
  - `business_object_key`
  - 支持的 `applicability_kind`
  - 固定骨架字段与可扩展字段边界
  - `current / as_of / history` 语义
  - 导出扁平化与列表回显方式
  - Assistant 支持级别与回落面
- 首批对象默认包括：
  - `Group`
  - `Family`
  - `Level`
  - `Profile`
- 若某对象尚未完成上述声明，则只能视为“领域骨架已冻结”，不得对外宣称其共享可配置化链路已完成。

### 7.7 Job Catalog 首批样板验收矩阵

`363` 要求至少通过以下样板矩阵，才能证明“固定骨架 + 共享可配置层”真的成立：

| 验收面 | 必须验证的问题 | 通过标准 |
| --- | --- | --- |
| 创建/编辑 | 新建或编辑分类对象时，扩展字段是否只消费共享决议 | 页面与 API 都使用同一 `DecisionSnapshot` / Explain 结果 |
| 下游引用 | Position / Assignment 选择 Profile/Level 时，是否复用共享候选与允许值决议 | `364` 不重建第二套候选逻辑，且失败原因可解释 |
| 导出/报表 | 导出分类对象时，扩展字段是否有稳定列标识、展示名与历史语义 | `380` 能直接消费，不需要本地再拼动态字段字典 |
| 审批/治理 | 分类配置变更、共享基线发布、字段退役是否进入治理轨道 | `370` 能提供审批摘要、回执与审计链 |
| Assistant | 对话检索、确认摘要、受控动作是否绑定共享决议锚点 | `390` 使用同源 `org_context + time anchor + decision anchor`，无第二解释链 |

## 8. 职位分类域边界与跨模块约束

### 8.1 Job Catalog 拥有的内容

- 组织上下文下的分类视图与分类事实的业务语义；
- Group / Family / Level / Profile 的稳定身份；
- Group 与 Family 的层级关系；
- Profile 与 Families 的归属与主归属；
- 分类记录的生效历史；
- 分类对象对下游引用的权威事实。

### 8.2 Job Catalog 不拥有的内容

- 扩展属性定义、候选值池、动态策略、Explain 引擎与发布治理：由 `321/345` 拥有；
- Position / Assignment 的主写模型：由 `364`/现有 staffing 契约拥有；
- 审批流状态、审批单生命周期：由 `370` 拥有；
- 报表工作台、导入批次、运营统计：由 `380` 拥有；
- 对话理解、澄清、确认摘要和动作编排：由 `390` 拥有。

### 8.3 跨模块调用原则

- `Staffing` 只能引用“某日有效的分类事实”，不能重建第二套 Family/Profile 语义。
- `Platform.Configuration / Policy` 负责“分类对象如何被扩展和裁剪”，不负责重写 Job Catalog 的核心骨架。
- `Workflow` 可以治理分类变更流程，但不能替代职位分类主写模型。
- `Assistant` 只能消费公开的事实与受控动作，不得自由拼接底层动作类型绕过规则。

## 9. 作为其他子计划的业务需求输入

### 9.1 对 `321 / 345` 的输入

- [ ] 平台可配置化能力必须支持以 Group / Family / Level / Profile 作为可扩展对象。
- [ ] 决议协议必须支持 `current / as_of / history`，并能解释某个分类属性为什么只读/必填/可见。
- [ ] 候选值池与允许值集合必须分层；共享基线必须通过发布而非 runtime fallback。
- [ ] Job Catalog 不接受模块私有的第二套“动态字段/策略页/Explain”实现。

### 9.2 对 `350 / 364` 的输入

- [ ] 前端页面模式必须表达“组合分类体系”，不能把 Job Catalog 简化成单棵树。
- [ ] 页面必须稳定呈现 `as_of + org_context + read_only` 上下文，并支持列表/详情/历史统一模式。
- [ ] `350` 必须把 Job Catalog 作为首批正式样板，对齐 `345` 的共享决议合同与 `347` 的 capability/颗粒度边界，而不是把职位分类页继续做成模块私有特例。
- [ ] Staffing 在创建/维护 Position、Assignment 时，必须消费某日有效的 JobCatalog 事实与共享策略决议。
- [ ] 下游引用页面需要能解释“为什么当前能选这些 Profile/Level”。

### 9.3 对 `370 / 380` 的输入

- [ ] Workflow 至少要区分：建立分类项、调整 Family 归属、停用分类项、共享基线发布、分类配置变更。
- [ ] 审批摘要至少应包含：`org_context`、对象类型、对象 code、目标生效日、归属变化、是否影响下游引用。
- [ ] Reporting / Data Workbench 必须支持按 `org_context / group / family / level / profile / current-as_of-history` 读取分类事实。
- [ ] Reporting / Data Workbench 对分类扩展字段必须回显稳定列标识、展示名、值类型与字段血缘，不能把 Job Catalog 动态字段重新降级成局部报表列。
- [ ] 数据质量检查至少要能发现：上下文无法解析、缺失主 Family、跨上下文错误引用、同日冲突、失效对象仍被下游引用等问题。

### 9.4 对 `390` 的输入

- [ ] Assistant 至少需要稳定支持：搜索分类项、解释规则、选择分类视图、新建 Group/Family/Level/Profile、调整 Family 归属、解释只读原因。
- [ ] Assistant 的确认摘要必须显式包含：`org_context`、目标对象、目标日期、归属变化、下游影响、是否触发审批。
- [ ] 面对多义 `OrgContext`、多候选 Family、多条历史记录时，Assistant 必须先澄清，不得自行猜测提交。
- [ ] Assistant 的写出口必须绑定共享策略版本与 `decision_snapshot_id`/等价决议锚点，不能绕过 Job Catalog 与平台配置能力的 fail-closed 校验。

## 10. 实施步骤

1. [ ] 冻结职位分类域业务词汇表：`Package / Group / Family / Level / Profile / ProfileFamilyLink / JobCatalogRecord / JobCatalogView`。
2. [ ] 将 `029/104/102B/030/321/345/060/062` 中已稳定规则映射到本计划，形成“现行规则 / 目标规则”双视图。
3. [ ] 以本计划为依据，补齐 `360` 中 M3 的详细任务拆分，避免继续使用“分类树”这种过宽泛表述。
4. [ ] 以本计划为输入，推动 `345/350/364/370/380/390` 显式消费 Job Catalog 的业务语义与可配置化边界。

## 11. 验收标准

- [ ] 职位分类域的业务对象、业务场景、边界与不变量已经可以脱离当前实现细节独立理解。
- [ ] 当前已落地规则与目标蓝图被明确区分，不再散落在 DB、UI、测试文档中各自表述。
- [ ] “可配置化是通用能力与基石之一”的结论已经被明确落到 Job Catalog 场景，而不是停留在平台抽象口号。
- [ ] Job Catalog 已完成首批样板接入清单与验收矩阵，能够作为 `345/350/380/390` 的复用样板，而不是只在模块内自洽。
- [ ] `345/350/364/370/380/390` 已可直接引用本计划作为职位分类域输入，而不是各自重写 Job Catalog 语义。
- [ ] `363` 与 `300/321/345/360` 保持同一口径，不引入新的名词漂移或第二事实源。

## 12. 关联文档

- [DEV-PLAN-029](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md)
- [DEV-PLAN-030](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md)
- [DEV-PLAN-060](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/060-business-e2e-test-suite.md)
- [DEV-PLAN-062](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md)
- [DEV-PLAN-102B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md)
- [DEV-PLAN-104](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104-jobcatalog-ui-optimization.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
