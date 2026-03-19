# DEV-PLAN-361：组织架构（Org Structure）业务规则优先蓝图与详细设计

**状态**: 规划中（2026-03-17 20:33 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 的 `M1: Org Structure` 子计划，同时承接 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“业务规则优先”的方法论。

`300` 对组织域给出的核心启发不是“先选技术动作”，而是：

- 先定义用户真正维护的是什么业务对象，再决定内部写入动作如何映射。
- 生效日期、历史、审计是主功能，不是补充字段。
- 工作流、报表、Assistant 都必须建立在清晰的主业务规则之上，不能反向重写组织域。

当前仓库中，组织架构相关规则已经分散沉淀在 `073/075/075C/080/081/100E1/106B/108/130/181` 等计划里。  
`361` 的任务不是复制这些实现细节，而是把它们收敛为一套**以业务语言表达的组织架构蓝图与规则 SSOT**，作为：

- `360` 的详细设计拆分；
- `370` 的审批/审计增强输入；
- `380` 的查询/导出/数据质量输入；
- `390` 的检索、澄清、确认摘要与受控动作输入。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述组织架构域，不让内部事件模型喧宾夺主。
- [ ] 总结当前仓库已经稳定沉淀的组织架构业务规则，并区分“现行规则”与“已冻结目标规则”。
- [ ] 冻结组织架构的业务蓝图：对象、场景、边界、不变量、用户交互与治理约束。
- [ ] 为 `370/380/390` 提供可直接消费的业务需求输入，避免后续计划各自发明组织语义。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 的共享数据建模规范。
- [ ] 本计划不替代 [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 的通用前端交互规范。
- [ ] 本计划不直接定义最终数据库 DDL、迁移脚本与代码实现。
- [ ] 本计划不实现审批流、导入导出和 Assistant 编排，只定义它们必须遵守的组织域输入。

## 3. “业务规则优先”在组织域的翻译

### 3.1 先表达业务对象，再映射技术动作

组织架构域中，用户维护的不是 `RENAME`、`MOVE`、`SET_BUSINESS_UNIT` 这些技术动作，而是：

- 某个组织是否存在；
- 它在某一天的名称、上级、状态、负责人、业务单元属性是什么；
- 某条历史记录是新增、插入、修正，还是错误数据撤销；
- 这次变更是否影响后续版本、审批、报表和 AI 摘要。

因此，`361` 冻结以下表达顺序：

1. 先定义业务意图；
2. 再定义校验规则；
3. 最后才定义内部如何映射到事件/写接口/重放。

### 3.2 生效日期是一级业务能力

组织架构不是“当前树 + 若干备注”，而是“稳定组织身份 + 按日生效的记录序列”。

这意味着：

- `effective_date` 是业务时间；
- `as_of` 是树/列表观察时间；
- `tx_time` 是审计时间；
- “当前 / 指定日期 / 历史”三种视角必须同时成立。

### 3.3 审计、工作流、Assistant 只能建立在组织主规则之上

- 工作流拥有治理状态，不拥有组织主数据。
- 报表拥有聚合视图，不拥有组织主写模型。
- Assistant 可以理解、检索、建议、生成确认摘要，但不能跳过组织域的确定性校验。

## 4. 组织架构业务蓝图（目标形态）

### 4.1 领域使命

组织架构域是平台内“组织身份、层级关系、生效历史、状态生命周期、结构性变更与错误数据撤销”的唯一业务权威。  
它既是 HR 主数据的锚点，也是 Staffing、Person、Workflow、Reporting、Assistant 的上游事实源。

### 4.2 核心业务对象

| 业务对象 | 业务含义 | 组织域是否拥有 |
| --- | --- | --- |
| `OrgUnit` | 稳定组织身份，不随版本切换而改变 | 是 |
| `OrgRecord` | 某一生效日上的组织业务记录 | 是 |
| `OrgTree` | 某日可见的上下级结构 | 是 |
| `OrgLifecycle` | active / inactive / rescinded 等生命周期状态 | 是 |
| `OrgChangeLog` | 组织变更的全量可追溯事实 | 是 |
| `OrgMutationPolicy` | 某意图、某日期下哪些字段能改 | 是 |
| `Manager(Person)` | 负责人身份主档 | 否，组织域只引用 |
| `Dict / FieldConfig` | 扩展字段定义、字典选项、启用窗口 | 否，组织域只消费 |
| `WorkflowRequest` | 审批单、审批状态 | 否，`370` 拥有 |
| `AssistantActionRequest` | 对话式动作请求与确认摘要 | 否，`390` 拥有 |

### 4.3 面向用户的主能力

- 建树与首个 root 自举
- 组织树浏览、搜索与定位
- 组织详情查看
- 生效版本新增、插入、修正
- 状态启用/停用
- 错误记录删除、错误建档删除
- 变更日志与历史审计
- 扩展字段按策略可见、可编、可审计

### 4.4 页面与交付语言

按照 `300 + 350` 的产品语言，组织架构最终应统一呈现为：

- 列表/树：回答“某天有哪些组织、层级如何”
- 详情：回答“这个组织在某个生效日是什么样”
- 历史/版本：回答“这个组织是如何演进到今天的”
- 审计/变更日志：回答“是谁在什么时间以什么理由改了什么”

## 5. 当前基线：已沉淀的业务规则

### 5.1 现行且已落地的规则

#### 5.1.1 组织树与身份

- 组织树“可展开性”以 `has_children` 为单一事实源，不能由前端是否已加载子节点推断。
- 空树租户必须走与常规创建同一条写链路完成首个 root 自举，不能靠第二写入口或手工改库。
- 空树自举态下，父组织为空、`is_business_unit=true`、前端给出明确 bootstrap 提示。
- 根组织禁止删除；存在子组织的组织禁止执行“删除组织（错误建档）”。

#### 5.1.2 时间与版本

- 业务有效期统一为 `date` 日粒度。
- 同一组织在同一生效日只允许一个有效业务槽位；同日冲突必须被显式拒绝，而不是靠下游自行容错。
- `tree_as_of`（树）与 `effective_date`（详情版本）解耦，切树不应强制改详情版本。
- 更正与插入都允许受控回溯，但必须满足“落在前后相邻记录区间内、不得同日冲突、父组织在该日有效”。
- 最晚记录执行 `insert_version` 视同 `add_version`。
- 一旦某条记录发生过“生效日更正”，后续仅改字段值的更正不得让 `effective_date` 回退（sticky 语义）。

#### 5.1.3 生命周期与错误数据治理

- `停用/启用` 与 `删除记录/删除组织` 必须是两类不同语义。
- 删除记录的业务含义是“撤销错误事件”，不是把该记录停用。
- 删除组织的业务含义是“撤销错误建档的整条组织历史”，且仅允许在“非根组织、无子组织、无依赖”条件下执行。
- 已撤销事件不应继续占用同日写入槽位。

#### 5.1.4 字段可编辑性与 fail-closed

- 字段是否可编辑不能靠前端猜测，必须由服务端 capability/policy 明确返回。
- 写入意图必须带上与 capability 对齐的 `policy_version`，防止页面拿旧策略提交。
- 启用中的扩展字段集合必须并入 `allowed_fields`。
- DICT 扩展字段的 label snapshot 由服务端生成，客户端不得提交 `ext_labels_snapshot`。
- 未知字段、未启用字段、越权字段一律 fail-closed 拒绝。

#### 5.1.5 页面交互

- 组织详情与变更日志都已收敛为“左时间轴 / 右详情”的双栏心智。
- 记录版本切换以左侧生效日期列表为主入口，不再依赖“上一条/下一条”。
- 复杂对象优先在详情页完成，不把主业务维护拆散到零碎弹窗。

#### 5.1.6 一致性与事实源

- 树、详情、版本列表必须来自同一套组织有效事实，不能一边看读模型、一边用事件表现拼业务字段。
- 删除/撤销类操作若 replay 失败，整笔业务必须回滚，不能留下“审计已写、业务未重建”的半成品状态。
- 审计日志需要能解释 CORRECT/RESCIND 的目标对象与原因，避免用户只能从技术字段猜测业务含义。

### 5.2 当前已冻结、需由 361 承接的目标规则

以下规则已经在现有计划中完成业务冻结，但仍需要 `361` 作为 `360` 的正式业务 SSOT 进行收口：

- CRUD 用户语言收敛为 `新建组织 / 新建版本 / 插入版本 / 更正 / 删除` 五类主操作。
- 用户按“字段编辑”而不是按 `rename/move/set_business_unit/...` 技术动作思考。
- 统一写入链路收敛为 `write(intent + patch)` 语义；删除保留为单独撤销语义。
- `correct` 作为用户语义，需要支持“状态 + 其它字段”同次提交；审计链仍保持可解释。
- 组织审计链目标收敛为单一 append-only 事实源，并显式暴露 CORRECT/RESCIND 的目标关联。

## 6. 361 冻结的目标业务规则矩阵

| 场景 | 用户真正要做什么 | 关键业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 建立首个 root | 在空树租户中建立组织起点 | 不得被 `tree_not_initialized` 反向阻断；必须走同一条创建链路；父组织为空；进入常规树后不保留特殊分支 | 形成可继续扩展的组织树根 |
| 新建组织 | 在某个父组织下建立新组织身份 | 父组织必须存在且在该日有效；编码/名称遵循租户规则；创建后立即可出现在对应 `as_of` 树中 | 新组织身份 + 首条生效记录 |
| 新建版本 | 在现有组织上追加未来记录 | 生效日必须晚于当前最晚记录；一次提交可改多个字段；结果是新版本，不是历史覆盖 | 形成新的未来记录 |
| 插入版本 | 在两条记录之间补一条历史记录 | 生效日必须位于相邻记录区间内；可早于当前选中记录；若选中最晚记录则退化为新建版本 | 形成中间版本且不破坏时间轴 |
| 更正记录 | 修正某条既有记录 | 修正应以“当前选中记录”为锚点；改生效日必须受区间约束；后续更正必须继承 sticky effective_date；字段编辑按 policy 决定 | 原记录被业务上修正，审计可追溯 |
| 状态变更 | 让组织启用或停用 | 状态是生命周期语义，不等于删除；状态变化必须在时间轴上可解释；涉及 disabled 记录的后续行为必须有明确限制 | 组织在某些日期 active/inactive |
| 删除记录 | 删除错误录入的某条记录 | 本质是 rescind target event；要求 reason/request_code；必须 replay 成功才提交；失败整笔回滚 | 错误记录从有效历史中移除但审计保留 |
| 删除组织 | 删除错误建档的整个组织 | 根组织禁止；有子组织禁止；有依赖禁止；必须同事务撤销 + replay | 整个错误组织从有效业务视图中移除 |
| 搜索与定位 | 快速找到目标组织 | 支持按 `org_code/name` 搜索；多命中时必须澄清；定位后树、详情、版本列表保持一致 | 用户进入正确的组织上下文 |
| 审计与追责 | 看清谁改了什么、为什么改 | 变更日志必须保留全量事实；CORRECT/RESCIND 必须显式显示目标事件；撤销状态不可隐身 | 满足审计、排障、合规与解释需求 |

## 7. 组织域边界与跨模块约束

### 7.1 组织域拥有的内容

- 组织稳定身份（OrgUnit）
- 组织层级关系
- 组织有效期记录
- 组织状态生命周期
- 组织业务字段与扩展字段在某日的可见/可编结果
- 组织变更日志与错误数据撤销语义

### 7.2 组织域不拥有的内容

- 人员主档与负责人身份真值：由 `Person` 拥有
- 职位/任职主写模型：由 `Staffing` 拥有
- 扩展字段定义、字典值发布、策略版本激活：由平台配置/字段配置模块拥有
- 审批流状态：由 `370` 拥有
- 导入批次、导出任务、运营统计：由 `380` 拥有
- 对话语义理解、确认文案生成、动作编排：由 `390` 拥有

### 7.3 跨模块调用原则

- `Staffing` 只能引用“某日有效的组织事实”，不能自定义组织层级口径。
- `Person` 可以展示“某人担任哪些组织负责人”，但不拥有组织层级真值。
- `Workflow` 可以冻结/放行业务动作，但不能替代组织主写模型。
- `Assistant` 只能消费组织域公开的只读事实与受控动作，不得自由拼接内部动作类型。

## 8. 作为 370 / 380 / 390 的业务需求输入

### 8.1 对 `370`（Workflow / Audit / Integration）的输入

- [ ] 审批粒度至少要区分：结构变更、状态变更、错误数据删除、字段修正四类组织请求。
- [ ] 审批摘要必须至少包含：`org_code`、组织名称、目标生效日、字段差异、是否影响层级、是否涉及删除/撤销。
- [ ] 审计增强不得重写组织域事件语义，只能在其上叠加审批轨迹、回执与集成执行结果。
- [ ] 外部集成若消费组织数据，必须显式声明采用 `as_of` 视图、当前视图还是变更日志视图。

### 8.2 对 `380`（Data Workbench / Reporting）的输入

- [ ] 查询工作台必须支持至少三类时间口径：`current`、`as_of`、`history`。
- [ ] 导出/报表至少要支持以下维度：组织编码、名称、上级、状态、业务单元标记、负责人、有效期、扩展字段、最后变更时间。
- [ ] 数据质量检查至少要能发现：同日冲突、父组织无效、树未初始化、被撤销记录误占位、扩展字段未启用却被引用等问题。
- [ ] 报表不得绕过组织域主规则直接拼装历史，否则会造成树/详情/报表口径漂移。

### 8.3 对 `390`（Assistant）的输入

- [ ] Assistant 对组织域至少需要稳定支持以下意图：搜索组织、解释规则、新建组织、新建版本、插入版本、更正记录、停用/启用、删除记录、删除组织。
- [ ] Assistant 的确认摘要必须显式包含：目标组织、目标日期、动作类型、字段差异、是否触发审批、失败风险与原因。
- [ ] 当存在多候选组织、多生效记录、多种允许动作时，Assistant 必须先澄清，不得自行猜测提交。
- [ ] Assistant 的可写出口必须对齐组织域的 capability/policy_version，不得绕过字段策略与 fail-closed 校验。

## 9. 与现有计划的关系

- `300` 提供“业务规则优先、effective-dated、受控 Assistant”的上层思想。
- `320` 提供共享建模语汇（effective date / history / audit / ORM-SQL 边界）。
- `350` 提供列表/详情/历史的页面模式；`361` 只定义组织域必须呈现什么。
- `360` 拥有核心 HR 业务域总拆分；`361` 是其中 `Org Structure` 的详细设计与业务输入。
- `370/380/390` 只能消费 `361` 冻结的组织语义，不能重新定义组织主规则。

## 10. 实施步骤

1. [ ] 冻结组织域业务词汇表：OrgUnit / OrgRecord / OrgTree / OrgLifecycle / OrgChangeLog / OrgMutationPolicy。
2. [ ] 将 `073/075/075C/080/081/100E1/106B/108/130/181` 中的组织规则映射到本计划，形成“现行规则 / 目标规则”双视图。
3. [ ] 基于本计划收敛 `360` 中 Org Structure 的详细任务拆分与交付顺序。
4. [ ] 以本计划为输入，补充 `370/380/390` 的组织域依赖与契约引用，避免后续重复定义。

## 11. 验收标准

- [ ] 组织架构域的业务对象、业务场景、边界与不变量已经可以脱离当前实现细节独立理解。
- [ ] 当前已落地规则与已冻结目标规则被明确区分，不再混杂在多个实施计划里。
- [ ] `370/380/390` 已能直接引用本计划作为组织域业务输入，而不是各自解释 Org 语义。
- [ ] `361` 与 `300/320/350/360` 的口径一致，不引入新的组织域名词漂移或第二事实源。

## 12. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
- [DEV-PLAN-073](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/073-orgunit-crud-implementation-status.md)
- [DEV-PLAN-075](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md)
- [DEV-PLAN-075C](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md)
- [DEV-PLAN-080](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/080-orgunit-audit-chain-consolidation.md)
- [DEV-PLAN-081](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/081-orgunit-records-version-selector-two-pane-alignment.md)
- [DEV-PLAN-100E1](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md)
- [DEV-PLAN-106B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/106b-orgunit-corrections-effective-date-sticky-semantics.md)
- [DEV-PLAN-108](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md)
- [DEV-PLAN-130](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/130-orgunit-tree-initialization-recovery-and-bootstrap.md)
- [DEV-PLAN-181](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/181-orgunit-details-form-capability-mapping-implementation.md)
