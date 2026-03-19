# DEV-PLAN-364：Staffing（Position / Assignment）业务规则优先蓝图与详细设计

**状态**: 规划中（2026-03-18 15:00 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 的 `M4: Staffing` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“业务规则优先”的方法论；
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 关于 effective-dated 主对象、`current / as_of / history` 与区间完整性的共享合同；
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md) 关于组织事实的上游边界；
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 关于职位分类事实与可配置化输入的上游边界；
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) / [DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md) 关于列表/详情/历史/表单与权限感知交互的统一产品语言；
- 当前仓库中已经沉淀的 Position / Assignment 实现、测试与历史计划。

`300` 对 Staffing 域最大的提醒，不是“先决定事件名、表名或函数名”，而是：

- 先定义企业到底在维护什么职位事实、任职事实和汇报关系；
- 先定义某一天这条职位/任职是否有效、是否允许新增/停用/更正；
- 再决定内部如何映射到事件、投射、API、页面和审批。

当前仓库中，Staffing 规则已经分散沉淀在：

- `030`：Position 时间线、状态、汇报线、容量与 Job Catalog 引用；
- `031`：Assignment 时间线、Person identity 锚点、同日唯一与后续更正/撤销方向；
- `027`：`pernr -> person_uuid` 的 Person Identity 最小合同；
- `063`：Person + Assignment 的 E2E/手工验证口径；
- `069`：移除 payroll/attendance 后的 Staffing 收敛结果。

但仓库里仍缺一份以**业务语言**表达的 Staffing SSOT。  
`364` 的任务就是把这些已有实现与计划收敛成 `360` 的正式子计划，并作为 `350/353/370/380/390` 的直接业务输入。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 Staffing 域，不再让 DB Kernel 细节、事件名和旧页面路径成为主叙事。
- [ ] 总结当前仓库中已经稳定沉淀的 Position / Assignment 业务规则，并明确区分“现行规则”与“已冻结目标规则”。
- [ ] 冻结 Staffing 域的目标业务蓝图：对象、场景、边界、不变量、时间语义、查询语义与用户交互。
- [ ] 明确 Staffing 如何消费 Org / Person / JobCatalog 的上游真值，而不是重建第二套身份、组织或分类语义。
- [ ] 作为 `360` 的详细拆分，为 `350/353/370/380/390` 提供可直接消费的业务需求输入。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 的共享时间建模合同；`364` 只声明 Staffing 如何采用它。
- [ ] 本计划不直接定义最终数据库 DDL、迁移脚本、函数签名或 ORM 映射；实现细节仍以既有/后续实施计划为准。
- [ ] 本计划不把 Staffing 改写成“任意多人并发任职”的开放模型；第一阶段仍以现行一岗一人为主，`M6b` 另行冻结。
- [ ] 本计划不把 Person、Org、JobCatalog 的真值所有权拉进 Staffing 域内部。
- [ ] 本计划不重新引入 legacy 双读、第二写入口、隐式 fallback 或绕过 fail-closed 的临时后门。

## 3. “业务规则优先”在 Staffing 域中的翻译

### 3.1 用户维护的是职位与任职事实，不是底层动作名

Staffing 域里，用户真正维护的不是：

- `submit_position_event`
- `submit_assignment_event`
- `replay_position_versions`
- `assignment_event_corrections`

他们真正维护的是：

- 某个职位在某一天是否存在、属于哪个组织、是否可用、向谁汇报、容量是多少；
- 某个人在某一天是否任职于某个职位、状态是什么、占用多少 FTE；
- 某次职位停用是否会与既有任职冲突；
- 某个 `as_of` 日期下，当前任职、历史任职、组织内职位分布分别是什么。

因此，`364` 冻结以下表达顺序：

1. 先定义业务对象；
2. 再定义业务规则与冲突拦截；
3. 最后才定义内部如何映射到事件、SQL、API 和页面动作。

### 3.2 生效日期、冲突拦截与 `current / as_of / history` 是一级业务能力

Position 与 Assignment 都不是“当前态记录 + 几个补充字段”，而是：

- 稳定身份；
- 按日生效的业务记录序列；
- 必须支持 `current / as_of / history` 三视图的主对象；
- 必须明确同日唯一、区间不重叠、引用在某日是否有效。

依 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)，`364` 明确：

- `Position` 与 `Assignment` 都属于 Pattern A：`主表 + 版本表`；
- `current / as_of / history` 的定义直接消费 `322`，不得在 Staffing 域重新定义；
- `same-day unique` 与 `no-overlap` 是强制共享规则；
- `gapless` 与 `last-infinite` 作为 Staffing 时间线的读模型预期显式成立，不再依赖隐式 today 或页面猜测。

### 3.3 Staffing 是“引用整合域”，但不是上游真值所有者

- `Person` 拥有人员身份真值，Staffing 只接收 `person_uuid` 作为写侧身份锚点。
- `Org` 拥有组织层级真值，Staffing 只引用“某日有效的组织事实”。
- `JobCatalog` 拥有职位分类真值，Staffing 只引用“某日有效且允许被引用的分类事实与策略决议”。

这意味着 Staffing 的职责是**拥有 Position / Assignment 主写模型并整合上游真值**，而不是复制上游领域。

### 3.4 Staffing 扩展值默认服从 Position / Assignment 的 effective-dated 切片

- `345` 提供的字段决议可以作用于 Position / Assignment，但扩展值一旦进入 Staffing，默认就必须跟随对应 `PositionRecord / AssignmentRecord` 的生效切片。
- 这意味着 Staffing 扩展值天然属于 Pattern A 时间线，而不是“当前快照字段 + 历史补丁”。
- 与 Person 组合展示时，必须明确区分：
  - Person 扩展值属于当前主档快照
  - Position / Assignment 扩展值属于 `current / as_of / history` 切片结果

## 4. Staffing 业务蓝图（目标形态）

### 4.1 领域使命

Staffing 域是平台内“**职位身份、职位汇报线、职位容量、任职时间线、职位与任职之间交叉约束，以及它们在某日的可查询可解释事实**”的唯一业务权威。  
它既服务于 HR 管理者维护岗位与任职，也服务于 Workflow、Reporting 与 Assistant 统一消费“某天到底谁在什么职位上”的真值。

### 4.2 核心业务对象

| 业务对象 | 业务含义 | `364` 是否视为 Staffing 拥有 |
| --- | --- | --- |
| `Position` | 稳定职位身份，不随版本切换而改变 | 是 |
| `PositionRecord` | 职位在某一生效日上的业务记录 | 是 |
| `PositionHierarchy` | 职位汇报线与上下级关系 | 是 |
| `PositionCapacity` | 职位在某日可承载的 FTE 容量 | 是 |
| `Assignment` | 某人某类任职时间线的稳定身份 | 是 |
| `AssignmentRecord` | 任职在某一生效日上的业务记录 | 是 |
| `PrimaryAssignment` | 当前第一阶段主交付的任职类型 | 是 |
| `StaffingView` | `current / as_of / history` 视图下的职位/任职观察结果 | 是 |
| `PersonRef` | 被任职引用的人员身份 | 否，由 `362`/Person 拥有，Staffing 只引用 |
| `OrgUnitRef` | 被职位引用的组织事实 | 否，由 `361`/Org 拥有，Staffing 只引用 |
| `JobProfileRef` | 被职位/任职消费的职位分类事实 | 否，由 `363`/JobCatalog 拥有，Staffing 只引用 |
| `JobCatalogPolicyDecision` | 某天某上下文下哪些分类事实可选、为什么可选 | 否，由 `345`/共享策略层提供 |

### 4.3 面向用户的主能力

- 浏览某日有效的职位列表与任职列表；
- 新建职位、调整职位字段、设置汇报线、停用职位；
- 为某个人建立/更新 primary assignment；
- 查看某个职位当前由谁占用、历史如何变化；
- 查看某个人当前任职、历史任职以及对应职位；
- 按组织子树、人员、职位三个维度查询 staffing 事实；
- 看清为什么某个职位不能停用、为什么某个职位不能被新任职引用、为什么某个字段不可改。

### 4.4 页面与交付语言

按照 `300 + 350 + 353` 的产品语言，Staffing 最终应统一呈现为：

- 职位列表：回答“某天有哪些职位、分别属于哪个组织、状态如何”
- 职位详情：回答“这个职位在某个生效日是什么样，向谁汇报，容量多少”
- 任职列表/时间线：回答“某人/某职位在某天有什么任职事实”
- 历史：回答“这个职位/任职是如何演进到今天的”
- Explain / 权限反馈：回答“为什么此时不能新建任职、不能停用职位、不能修改某字段”

## 5. 当前基线：已沉淀的业务规则

### 5.1 现行且已落地的规则

#### 5.1.1 Position 的身份、状态与容量基线已经成立

- Position 具有稳定 identity，业务记录按 `effective_date` 日粒度演进。
- `lifecycle_status` 已是稳定业务字段，`active / disabled` 通过有效期切片表达，而不是靠“缺行”表达停用。
- `capacity_fte` 已开放编辑，但当前现行口径仍是一岗一人，不默认支持同一职位同一时点多人并发任职。
- `allocated_fte <= capacity_fte` 已是稳定约束；Position 降容若会压穿既有任职，必须 fail-closed。

#### 5.1.2 Assignment 的身份锚点与时间线基线已经成立

- Assignment 写侧权威输入是 `person_uuid`，不是 `pernr`。
- `pernr` 仅用于 UI 查询、筛选与展示；精确解析必须由 Person 模块提供，Staffing 不自建第二套解析真值。
- Assignment 当前以 effective-dated 时间线表达，展示层只展示 `effective_date`，不展示 `end_date`。
- 当前最小可用交付聚焦于 `primary assignment` 的 create/update（upsert）闭环。

#### 5.1.3 Position 与 Assignment 的交叉不变量已经成立

- Assignment 引用的 `position_id` 必须在该 `effective_date as_of` 下存在且 `lifecycle_status='active'`，否则 fail-closed。
- 停用 Position 不会隐式终止、转移或修复既有 Assignment。
- 若某个 Position 停用切片会与既有 active assignment 冲突，Position 写入必须 fail-closed。
- disabled Position 在 UI 中仍可见，但不得作为新任职候选项。

#### 5.1.4 职位汇报线的最小业务规则已经成立

- `reports_to_position_id` 是 Position 的业务事实，不是页面临时关系。
- 汇报线变更必须满足：禁止自指、禁止成环、引用目标必须在该 `effective_date as_of` 下 active。
- 当前已交付口径是 forward-only：只允许在更晚生效日追加汇报线变更，不支持 retro 改写历史。

#### 5.1.5 Position / Assignment 的时间模型已经稳定到可被 `364` 正式承接

- Position 与 Assignment 都属于 Pattern A 主对象。
- `current / as_of / history` 已是共享产品语义，Staffing 不得把 `current` 偷换成“缺省 today”。
- 同主体同一业务日只允许一个有效槽位，同日冲突必须显式拒绝。
- 读模型按时间线解释当前、某日与历史，不允许列表、详情、时间线各自拼装不同事实源。

#### 5.1.6 最小 UI/API 闭环与去 payroll 收口已经成立

- 现行仓库已有最小可见入口 `/org/positions` 与 `/org/assignments`，证明 Position / Assignment 已具备可操作样板闭环。
- Assignment 的最小字段合同已稳定收敛到：`assignment_id / person_uuid / position_id / effective_date / status / allocated_fte`。
- `base_salary`、`currency` 等 payroll 字段已被移出 Staffing 现行合同，不得在 `364` 之后以兼容名义回流。

### 5.2 当前已冻结、需由 `364` 正式承接的目标规则

以下方向已经在现有计划中形成共识，但仍需要 `364` 作为正式业务 SSOT 收口：

- Staffing 应被描述为“职位 + 任职 + 交叉约束 + 查询语义”的业务域，而不是 Position/Assignment 两份零散实现。
- 第一阶段口径明确保持“一岗一人”；多人并发与 `SUM(allocated_fte) <= capacity_fte` 属于后续 `M6b`，不得被页面或下游默认假定为已开放。
- 同一 `person_uuid` + 同一 `effective_date` 的重复 upsert 需要明确区分“相同输入幂等成功”与“不同输入 fail-closed”，但该口径仍需由 `364` 作为正式目标规则承接。
- Correct / Rescind / Transfer / Terminate 的业务语言已经出现，但除最小 upsert 外仍需分阶段落地，不应假装全部已在现网成立。
- 最终产品 IA 应从现行 `/org/positions`、`/org/assignments` 收敛到 `350/351` 下的 `/app/staffing/positions`、`/app/staffing/assignments`。

## 6. `364` 冻结的目标业务规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 新建职位 | 在某组织下建立一个可被任职引用的职位 | 组织引用必须在该日有效；职位身份稳定；分类引用必须消费某日有效的 JobCatalog 事实；写入走统一主链路 | 形成新的 Position 身份与首条记录 |
| 调整职位属性 | 在某日起修改职位名称、分类、汇报线、容量等 | 必须显式给出 `effective_date`；字段是否可改由服务端决议；汇报线禁止自指/成环；容量不能压穿既有任职 | 形成新的 PositionRecord |
| 停用职位 | 让职位从某日起不再可用于新任职 | 停用是时间线事实，不是删除；若与既有 active assignment 冲突必须 fail-closed；不做隐式转移 | Position 在未来不可再被新任职引用，历史仍可读 |
| 建立 primary assignment | 让某人在某日起任职到某个职位 | 写侧身份锚点必须是 `person_uuid`；Position 必须在该日 active；`allocated_fte <= capacity_fte`；第一阶段仍一岗一人 | 形成新的 Assignment 时间线或新切片 |
| 更新任职时间线 | 在更晚日期变更某人的职位/状态/FTE | 必须显式给出 `effective_date`；遵守 same-day unique 与 no-overlap；历史通过时间线解释，不靠覆盖旧记录 | Assignment 形成新的有效切片 |
| 同日重复提交 | 再次提交同一人的同一日任职 | 相同输入应幂等成功；不同输入必须 fail-closed，并引导用户改用新的 `effective_date`；不引入隐式 `effseq` | 同日唯一保持稳定且可解释 |
| 浏览 `current / as_of / history` | 看现在、某日和全部历史 | 直接消费 `322` 共享定义；列表、详情、时间线口径一致；不得隐式 today | 用户、报表、Assistant 看到同一 Staffing 真值 |
| 按组织/人员/职位查询 | 从不同入口定位 staffing 事实 | 组织子树口径必须消费 `361`；人员解析必须消费 `362/027`；职位分类解释必须消费 `363/345` | 查询入口不同，但命中同一事实源 |
| 解释不能操作的原因 | 理解为什么不能停用/不能任职/不能改字段 | 区分对象只读、动作禁用、直接无权访问；错误反馈必须可解释且 fail-closed | 用户知道该改什么、该向谁求助 |

## 7. Staffing 域边界与跨模块约束

### 7.1 Staffing 拥有的内容

- Position 的稳定身份、业务字段、状态时间线；
- Position 的汇报线与容量语义；
- Assignment 的稳定身份、业务字段、状态时间线；
- Position 与 Assignment 之间的交叉约束；
- `current / as_of / history` 下的 Staffing 查询真值；
- Staffing 域内的业务错误语义与冲突拦截。

### 7.2 Staffing 不拥有的内容

- 人员身份真值、`pernr -> person_uuid` 精确解析：由 `362`/Person 拥有；
- 组织树真值、组织层级解释、组织字段规则：由 `361`/Org 拥有；
- 职位分类骨架、组织上下文下的分类视图与分类共享治理：由 `363`/JobCatalog 拥有；
- 动态字段/候选值/Explain/策略引擎：由 `321/345` 拥有；
- 审批单与审批生命周期：由 `370` 拥有；
- 报表工作台、导出任务、运营统计：由 `380` 拥有；
- 对话理解、澄清、确认摘要和动作编排：由 `390` 拥有。

### 7.3 跨模块调用原则

- `Staffing` 只能引用“某日有效的组织事实”，不能自定义组织层级或组织树语义。
- `Staffing` 只能引用“某日有效的 Person identity”，不能自己解析 `pernr` 作为写侧真值。
- `Staffing` 只能引用“某日有效且允许被引用的 JobCatalog 事实”，不能重建第二套 Family/Profile 语义。
- `Workflow` 可以治理 Staffing 变更流程，但不能替代 Position / Assignment 主写模型。
- `Assistant` 只能消费公开事实与受控动作，不得自由拼接底层动作类型绕过 Staffing 校验。

### 7.4 Staffing 扩展值的时间语义边界

- Position 扩展值必须跟随对应 `PositionRecord` 的 `effective_date` 切片一起读取、导出和审计。
- Assignment 扩展值必须跟随对应 `AssignmentRecord` 的 `effective_date` 切片一起读取、导出和审计。
- 不允许把某条 Position/Assignment 历史切片上的扩展值，降级解释成“对象当前默认字段”。
- 当 Staffing 与 Person 在报表、Assistant 或确认摘要中组合出现时，必须显式说明：
  - 哪些字段来自 Person 当前快照
  - 哪些字段来自 Position/Assignment 某日切片

## 8. 作为其他子计划的业务需求输入

### 8.1 对 `350 / 353` 的输入

- [ ] `/app/staffing/positions` 与 `/app/staffing/assignments` 必须收敛为“列表 + 详情 + 历史”统一页面模式。
- [ ] Staffing 的 effective-dated 写操作必须显式展示 `effective_date`，并在提交前回显“将对哪一天生效、会影响什么”。
- [ ] 表单必须消费服务端字段决议，不得自行猜测 Position/Assignment 哪些字段可改。
- [ ] 业务页必须区分“对象只读”“动作禁用”“直接无权访问”，并解释 disabled position / 容量冲突 / 引用无效等原因。

### 8.2 对 `370`（Workflow / Audit / Integration）的输入

- [ ] 审批/审计至少要区分：新建职位、职位停用、职位汇报线变更、容量变更、新建任职、任职状态变更六类 Staffing 请求。
- [ ] 审批摘要至少应包含：目标人员、目标职位、目标组织、目标生效日、FTE 变化、是否影响现有 active assignment。
- [ ] 审计增强不得重写 Position / Assignment 主语义，只能在其上叠加审批轨迹、回执与集成状态。
- [ ] 外部集成消费 Staffing 数据时，必须显式声明采用 `current`、`as_of` 还是 `history` 视图。

### 8.3 对 `380`（Data Workbench / Reporting）的输入

- [ ] 查询工作台必须支持按组织子树、人员、职位三个维度读取 `current / as_of / history` Staffing 事实。
- [ ] 导出/报表至少要支持：职位状态、所属组织、汇报线、容量、当前任职人、任职状态、有效期、最后变更时间。
- [ ] 数据质量检查至少要能发现：无效职位引用、disabled position 仍被尝试任职、容量冲突、汇报线成环、同日冲突、人员身份不一致等问题。
- [ ] 报表不得绕过 Staffing 域主规则直接拼装时间线，否则会造成列表/详情/报表口径漂移。
- [ ] 若导出包含 Staffing 扩展字段，必须显式标识其所属对象和时间切片，不得与 Person 当前快照字段混成一组无时间语义的列。

### 8.4 对 `390`（Assistant）的输入

- [ ] Assistant 至少需要稳定支持：搜索职位、搜索任职、解释规则、新建职位、调整职位、停用职位、建立 primary assignment、解释失败原因。
- [ ] Assistant 的确认摘要必须显式包含：目标人员、目标职位、目标组织、目标日期、字段差异、容量/冲突风险、是否触发审批。
- [ ] 面对多候选人员、多候选职位、多条历史记录、多种允许动作时，Assistant 必须先澄清，不得自行猜测提交。
- [ ] Assistant 的可写出口必须绑定服务端策略版本与 Staffing fail-closed 校验，不能绕过 Position / Assignment 交叉约束。
- [ ] 当确认摘要同时出现 Person 与 Staffing 字段时，Assistant 必须显式区分“人员当前快照字段”和“职位/任职某日切片字段”。

## 9. 与现有计划的关系

- `300` 提供“业务规则优先、effective-dated、受控 Assistant”的上层思想。
- `322` 提供 Staffing 必须复用的时间语义与 Pattern A 合同。
- `350/353` 提供列表/详情/历史、表单与权限感知交互模式；`364` 只定义 Staffing 必须呈现什么。
- `360` 拥有核心 HR 业务域总拆分；`364` 是其中 `M4: Staffing` 的正式详细设计。
- `361/363` 分别提供 Org 与 JobCatalog 的上游真值边界，`364` 只能消费，不得重写。
- `370/380/390` 只能消费 `364` 冻结的 Staffing 业务语义，不能重新定义 Position / Assignment 主规则。

## 10. 实施步骤

1. [ ] 冻结 Staffing 域业务词汇表：`Position / PositionRecord / PositionHierarchy / PositionCapacity / Assignment / AssignmentRecord / PrimaryAssignment / StaffingView / PersonRef / OrgUnitRef / JobProfileRef`。
2. [ ] 将 `027/030/031/063/069` 中已稳定规则映射到本计划，形成“现行规则 / 目标规则”双视图。
3. [ ] 以本计划为依据，收敛 Position / Assignment 的页面 IA、查询口径与错误解释语言，避免继续沿用零散旧实现叙事。
4. [ ] 以本计划为输入，推动 `350/353/370/380/390` 显式消费 Staffing 的业务对象、时间语义与交叉约束。
5. [ ] 对多人并发任职、Correct / Rescind / Transfer / Terminate 等扩展能力，按本计划定义的边界另行拆分后续子计划，不得直接混入第一阶段合同。

## 11. 验收标准

- [ ] Staffing 域的业务对象、业务场景、边界与不变量已经可以脱离当前实现细节独立理解。
- [ ] Position / Assignment 的现行规则与目标规则被明确区分，不再散落在旧实现计划、测试说明与 UI 样板里各自表述。
- [ ] Staffing 扩展值的时间语义已经明确跟随 Position / Assignment 切片，不再与 Person 当前快照或局部页面状态混写。
- [ ] `364` 与 `300/322/350/353/360/361/363` 口径一致，不引入新的时间语义漂移、第二事实源或边界越权。
- [ ] `350/353/370/380/390` 已可直接引用本计划作为 Staffing 域输入，而不是各自重写 Position / Assignment 语义。
- [ ] 一岗一人、容量约束、职位停用冲突、`person_uuid` 身份锚点与 `current/as_of/history` 等关键规则已经被明确冻结。

## 12. 关联文档

- [DEV-PLAN-027](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/027-person-minimal-identity-for-staffing.md)
- [DEV-PLAN-030](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md)
- [DEV-PLAN-031](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/031-greenfield-assignment-job-data.md)
- [DEV-PLAN-063](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/063-test-tp060-03-person-and-assignments.md)
- [DEV-PLAN-069](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/069-remove-payroll-attendance.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
