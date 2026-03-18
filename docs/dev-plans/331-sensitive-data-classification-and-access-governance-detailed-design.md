# DEV-PLAN-331：敏感数据分级与访问治理详细设计

**状态**: 规划中（2026-03-18 15:34 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 的 `331` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“企业级 HR 平台天然承载敏感组织与人员数据”的冻结；
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 对“敏感数据必须显式分级、导出是高风险动作、Assistant 纳入治理”的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对“上下文最小化、必要脱敏、日志/提示词/回执不得泄露高风险信息”的冻结；
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 对“查看、维护、导出、控制面治理必须显式分离”的冻结；
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md) 与 [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 对导出、查询工作台与 Assistant 检索的高风险治理需求。

`330` 已经把“敏感数据必须分级”写成父约束，但如果没有 `331`，后续实现很容易继续出现：

- 字段是否敏感只停留在个人经验，而不是系统合同；
- UI、API、导出、审计、Assistant 对同一字段采用不同暴露口径；
- 普通读取、批量导出、控制面排障与 AI 检索共用同一披露等级；
- 业务模块各自决定哪些字段需要脱敏，导致最终用户看到互相冲突的规则。

`331` 的职责就是把这些问题收敛为 **Greenfield 敏感数据分级与访问治理 SSOT**。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述敏感数据治理，不让字段注解、DTO 命名或日志实现细节喧宾夺主。
- [ ] 冻结 Greenfield 的数据分级矩阵、最小披露原则与不同暴露面的共享口径。
- [ ] 冻结普通查看、维护、高风险查看、导出、控制面访问、审计读取与 Assistant 检索之间的治理差异。
- [ ] 冻结脱敏、掩码、截断、摘要化与“不可返回原值”这些共享表达。
- [ ] 为 `342/344/360/380/390` 提供统一输入，阻断各模块继续私造“自己的敏感字段规则”。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的角色与权限矩阵；它只定义数据分级与访问治理边界。
- [ ] 本计划不替代后续 `332` 对导出留存、审计留存与删除策略的细化；`331` 只定义“什么数据有多敏感、默认怎么暴露”。
- [ ] 本计划不替代 [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 的 tenant-scoped SQL、secret 与 Assistant stopline。
- [ ] 本计划不直接定义具体字段清单的最终 schema 实现或代码注解形式。
- [ ] 本计划不允许用“内部系统/管理员/调试需要”作为无限制暴露敏感数据的默认例外。

## 3. “业务规则优先”在敏感数据治理中的翻译

### 3.1 用户真正关心的是“谁能看到多少、为什么”，不是字段是不是打了某个标签

平台用户真正关心的是：

- 我现在看到的人员与任职信息，是否超出了我应该知道的范围；
- 为什么我能查到对象，却不能看到原始敏感字段；
- 为什么列表页、详情页、导出和 Assistant 对同一信息的展示程度不同；
- 为什么某些高风险数据必须经过额外治理才能读取或导出。

### 3.2 数据分级首先回答“暴露风险是什么”，不是“字段长什么样”

`331` 冻结：

- 分级首先围绕披露风险、聚合风险与误用风险；
- 同一字段在不同暴露面可以有不同披露等级；
- “能读取对象”不等于“能读取对象上的全部原始字段”。

### 3.3 最小披露是默认原则，不是高风险场景的额外优化

系统默认应回答：

- 当前页面是否真的需要展示原值；
- 当前角色是否只需要摘要、掩码或存在性判断；
- 当前场景是否属于高风险批量读取或高风险聚合输出；
- 当前 Assistant / 导出 / 审计读取是否需要进一步降敏。

### 3.4 Control Plane、导出与 Assistant 是高风险暴露面

`331` 明确：

- `superadmin` 不是默认全量租户敏感数据阅读权；
- 导出、运营分析与批量工作台天然比单条详情页更高风险；
- Assistant 因为会进行上下文拼装与文本输出，默认属于高风险披露面之一。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 已明确：
  - 敏感数据必须显式分级；
  - 导出是高风险动作；
  - Assistant 纳入治理；
  - 数据保留与删除需要单独策略。
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 已明确：
  - Assistant 上下文必须经过必要脱敏；
  - 提示词、工具参数、回执与错误消息不得泄露高风险信息；
  - 高风险查询与执行链必须可审计。
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 已明确：
  - 查看、维护、导出、审批、控制面治理是不同能力；
  - `ExportPolicy` 不属于纯权限矩阵，而要依赖 `330/380` 治理。
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 已明确：
  - Assistant 只可在受控编排层内读取与建议；
  - 所有可写动作都要经确认、审批与回执边界。

### 4.2 当前主要缺口

1. [ ] **缺少系统级数据分级矩阵**  
   目前只有原则，没有把“普通业务数据、敏感人员数据、高风险聚合/导出数据”落成统一语言。

2. [ ] **缺少跨暴露面的统一披露规则**  
   列表页、详情页、导出、工作台、日志、审计与 Assistant 仍缺少统一的最小披露合同。

3. [ ] **缺少控制面与租户应用之间的数据可见性边界**  
   如果不提前冻结，后续很容易把平台控制面默认为“天然看见全部数据”。

4. [ ] **缺少对领域模块的标注与 ownership 输入**  
   `360` 中每个业务域都需要知道自己该如何声明字段分级，但当前没有 SSOT。

## 5. 敏感数据分级与访问治理蓝图

### 5.1 领域使命

`331` 是平台内“**什么数据属于哪一级风险、在不同暴露面上默认应该呈现到什么程度、哪些场景必须降敏或拒绝**”的共享治理权威。

### 5.2 核心治理对象

| 治理对象 | 治理含义 | 是否由 `331` 拥有共享合同 |
| --- | --- | --- |
| `DataClassification` | 数据分类等级与风险含义 | 是 |
| `SensitiveFieldCatalog` | 需要被分级治理的字段/属性目录 | 是（共享目录） |
| `DisclosureSurface` | 数据暴露发生在哪个面：UI、API、导出、审计、Assistant 等 | 是 |
| `DisclosurePolicy` | 某分类在某暴露面上的默认披露规则 | 是 |
| `MaskingRule` | 掩码、截断、摘要化、置空等降敏表达 | 是 |
| `AccessPurpose` | 高风险读取或批量读取的业务目的声明 | 是（共享治理对象） |
| `HighRiskDataset` | 聚合、批量、跨对象组合后的高风险结果集 | 是 |
| `RetentionPolicy` | 留存与删除策略 | 否，`332` 拥有 |

### 5.3 面向系统的主能力

- 为业务字段与派生结果建立统一分级；
- 为列表、详情、编辑、导出、日志、审计与 Assistant 建立不同披露面规则；
- 让高风险批量读取、跨对象聚合与导出拥有更严格治理；
- 让控制面、支持面与租户应用面维持清晰可解释的数据可见性边界；
- 让下游权限、导出、审计与 Assistant 都消费同一条敏感数据语言。

## 6. `331` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心治理规则 | 治理结果 |
| --- | --- | --- | --- |
| 列表页查看 | 快速识别对象与状态 | 默认最小披露；敏感字段优先展示摘要、掩码或派生状态 | 列表不泄露过量原值 |
| 详情页查看 | 理解单对象完整上下文 | 仅在业务必要范围内提升披露级别；高敏原值需显式授权 | 详情可读但不过曝 |
| 导出/批量下载 | 获取可离线流转的数据集 | 导出视为高风险暴露面；字段集需要额外治理与审计 | 导出与在线查看明确分层 |
| 审计/回执阅读 | 理解发生过什么 | 审计与回执默认保留足够证据，但不默认回显全部高敏原值 | 证据与降敏并存 |
| Assistant 检索与回答 | 通过自然语言获取帮助 | 上下文最小化、必要脱敏、禁止把高敏原值拼进回复或提示词 | AI 不过曝 |
| 控制面/支持排障 | 平台层理解租户状态 | control plane 不天然拥有 tenant app 的全部敏感数据原值访问权 | 平台治理不越权 |

## 7. 共享合同、不变量与实现护栏

### 7.1 分级合同

`331` 建议至少冻结以下共享等级：

- `Class A`：平台公开或低敏元数据
- `Class B`：普通租户业务数据
- `Class C`：敏感人员与组织关联数据
- `Class D`：高风险聚合、批量导出或跨对象组合结果

冻结原则：

- 字段、派生字段、聚合结果与导出结果都需要被分级；
- 同一原始字段在不同暴露面可映射到不同披露规则；
- 未分级数据默认不得进入高风险暴露面。

### 7.2 暴露面合同

`331` 冻结的典型暴露面包括：

- 租户应用列表页
- 租户应用详情/表单页
- JSON API 返回
- 导出文件
- 审计日志与回执
- 控制面排障界面
- Assistant 检索上下文、提示词与最终回复

冻结原则：

- 暴露面不同，默认披露级别不同；
- 不允许把“详情页能看”直接等价为“导出也能看、Assistant 也能说”。

### 7.3 最小披露与降敏合同

- 默认优先返回：
  - 摘要；
  - 掩码；
  - 截断；
  - 分类标签；
  - 是否存在的布尔判断；
  - 可解释但不回显原值的错误信息。
- 高敏原值不是默认输出；只有在满足明确治理前提时才允许提升暴露级别。
- 日志、错误、提示词、回执与快照不得因为“调试方便”默认回显高敏原值。

### 7.4 高风险数据集合同

- 当普通数据经过批量导出、跨模块聚合、筛选组合或 Assistant 上下文拼装后，可能升级为更高风险数据集。
- `HighRiskDataset` 的风险等级不得简单继承最低字段等级，而应按组合风险重新评估。
- 批量读取与离线流转默认比在线单条查看更严格。

### 7.5 Control Plane 与 Support 合同

- `superadmin` 与平台支持面默认不等于租户内高敏数据阅读权。
- 平台治理场景应优先消费状态、摘要、计数与健康度，而不是高敏原值。
- 若存在例外读取，需要显式目的、显式审计与显式最小化范围。

### 7.6 领域 ownership 合同

- 各业务域拥有其字段与对象本身，但字段分级语言必须引用 `331`。
- 业务域不得各自发明“本模块敏感字段”的私有等级体系。
- 共享目录、导出中心、审计中心与 Assistant 编排都必须消费同一条分级合同。

### 7.7 stopline

- 不允许未分级字段直接进入导出、Assistant 高上下文、日志或控制面高风险读取。
- 不允许把 `superadmin`、后台任务或调试工具默认视为“可见全部原值”。
- 不允许列表、详情、导出、Assistant 对同一字段长期维持互相冲突的披露口径。
- 不允许用“内部使用”作为泄露敏感数据的永久豁免理由。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `342`（AuthZ 与平台权限矩阵）的输入

- [ ] `342` 的权限包需要明确区分：普通读取、高敏查看、导出、控制面治理与调试诊断。
- [ ] 权限矩阵不得绕过 `331` 的最小披露原则，直接把“能看对象”等价为“能看全部字段原值”。

### 8.2 对 `344`（审计、通知与后台任务基座）的输入

- [ ] 审计、通知与任务回执应能承载降敏后的摘要、分类标签与可解释失败原因，而不是默认记录高敏原值。
- [ ] 后台任务与通知模板若使用高敏数据，必须消费 `331` 的披露规则。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] `361/362/363/364` 需要显式声明对象字段的分级与默认暴露面。
- [ ] 领域模块可以拥有对象事实，但不能自造第二套敏感数据等级体系。

### 8.4 对 `380`（数据工作台与运营分析）的输入

- [ ] 导出、报表、工作台聚合查询必须基于 `331` 的高风险数据集与暴露面规则工作。
- [ ] Query Workspace 与运营面板应优先消费摘要化、聚合化输出，而不是默认回显高敏原值。

### 8.5 对 `390`（Chat Assistant）的输入

- [ ] Assistant 检索、提示词拼装与最终回复必须消费 `331` 的分类与降敏合同。
- [ ] 当用户请求高敏信息时，Assistant 必须遵守与 UI/导出一致的最小披露与拒绝规则，而不是单独例外。

## 9. 建议实施分期

1. [ ] `M1`：分级语言冻结  
   统一数据分类等级与风险含义，停止各模块口头判断。
2. [ ] `M2`：暴露面矩阵冻结  
   明确 UI、API、导出、审计、控制面与 Assistant 的默认披露差异。
3. [ ] `M3`：降敏规则冻结  
   统一掩码、摘要化、置空、截断与“不可返回原值”的共享表达。
4. [ ] `M4`：首批领域对象接线  
   选择 Org / Person / Assignment 等高价值对象作为字段分级样板。
5. [ ] `M5`：下游计划引用收口  
   让 `342/344/360/380/390` 正式引用 `331`，停止各自重写敏感数据主规则。

## 10. 验收标准

- [ ] `331` 已成为 Greenfield 对敏感数据分级与访问治理的单一事实源。
- [ ] 系统已形成统一的数据分级矩阵与暴露面语言，而不再依赖个人经验判断。
- [ ] 列表、详情、导出、审计、控制面与 Assistant 对同类数据的披露规则已可解释且互不冲突。
- [ ] 高风险聚合/导出/AI 上下文已被明确视为高风险暴露面，而不是普通读取的自然延伸。
- [ ] 下游权限、导出、审计与 Assistant 子计划可以直接消费 `331`，不再各自发明敏感数据治理规则。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
