# DEV-PLAN-370：工作流、审计增强与集成子计划

**状态**: 规划中（2026-03-17 03:16 CST）

## 1. 背景与上下文

本计划承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的总体蓝图
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的平台基座
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 的核心 HR 业务模型
- [DEV-PLAN-362](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/362-person-business-rules-and-detailed-design.md) 的 Person 主档、身份锚点与生命周期合同

`370` 关注的是“让系统从能用，走向可治理、可协同、可对外集成”的那一层能力：

- 审批工作流
- 增强审计
- 外部系统集成

这些能力都建立在主业务模型之上，但又不能完全滞后到最后，否则主数据模型会缺少审批、审计和集成约束，后期返工成本会很高。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立关键主数据与事务操作的审批能力。
- [ ] 建立增强审计与操作回执能力，便于监管、定位与运营。
- [ ] 建立外部系统集成边界，支撑后续同步与开放接口。

### 2.2 非目标

- [ ] 本计划不重写 `340/360/362` 的平台与业务基础模型。
- [ ] 本计划不承接导入导出、报表工作台与运营数据工作台，它们由后续独立子计划承接。
- [ ] 本计划不承接 Chat Assistant，对话能力由后续独立子计划承接。
- [ ] 本计划不引入过重的 BPM 大平台或分布式事件总线作为默认前提。

## 3. 能力范围

### 3.1 Workflow / Approval

- 提交审批
- 审批节点
- 审批记录
- 驳回、撤回、重提
- 待办

### 3.2 Audit Enhanced

- 关键操作快照
- 审批轨迹
- 外部同步审计
- 集成任务审计

### 3.3 Integration Hub

- 外部 API
- Webhook
- 文件交换
- 定时同步

## 4. 关键设计决策

### 4.1 工作流边界（选定）

- 工作流是业务变更的“治理层”，不是主数据来源。
- 主数据仍由业务模块维护。
- 工作流只负责：
  - 提交
  - 路由
  - 审批状态
  - 回执
- 审批中、异步执行中和已完成等治理状态，必须以显式状态和回执对外暴露，不能要求调用方去猜业务表变化。

### 4.2 审计策略（选定）

- `340` 提供统一审计底座。
- `370` 在其上增加：
  - 业务前后快照
  - 审批轨迹
  - 集成执行轨迹

### 4.3 集成策略（选定）

- 外部系统接入采用 Integration Hub。
- 不让每个业务模块单独发明同步策略。
- 同步任务必须：
  - 有任务记录
  - 有幂等键
  - 有错误回执

## 5. 功能拆分

### 5.1 M1：Workflow / Approval

- [ ] 审批模板
- [ ] 审批实例
- [ ] 提交 / 驳回 / 撤回 / 重提
- [ ] 审批状态查询
- [ ] 待办中心
- [ ] 审批历史

### 5.2 M2：审计增强

- [ ] 关键动作 before/after 快照
- [ ] 审批轨迹审计
- [ ] 集成执行审计
- [ ] 操作回执与查询
- [ ] 长事务/异步执行状态回执

### 5.3 M3：Integration Hub

- [ ] Outbound API / Webhook
- [ ] Inbound 接口
- [ ] 批量同步任务
- [ ] 集成任务回执与错误追踪

## 6. 关键模型方向

建议至少包括以下附属模型：

- `workflow_definitions`
- `workflow_instances`
- `workflow_steps`
- `workflow_approvals`
- `integration_connections`
- `integration_runs`
- `operation_receipts`

其中工作流实例与操作回执需要对外提供稳定、可查询的生命周期状态，供 UI、Assistant 和集成调用方消费。
- 审批摘要、操作回执与集成执行状态至少应显式回显 `business_object_key / org_context / time anchor`；动作类场景还应补充 `capability_key`，不得以隐藏容器键解释命中上下文。

## 7. 与业务域的关系

### 7.1 Workflow 与业务域

- `Workflow` 不拥有 Org / Person / Staffing 主数据。
- 审批通过后，由业务应用层执行最终变更。
- 流程路由、审批人与只读/可执行结果必须消费 `340/345/347` 提供的统一访问模型与 `OrgContext` 装配，不得把命中上下文重新包装为 `setid/package` 或其他容器键。
- 涉及人员的审批请求，必须直接消费 [DEV-PLAN-362](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/362-person-business-rules-and-detailed-design.md) 冻结的 Person 真值：
  - `person_uuid` 作为稳定身份锚点；
  - `pernr` 作为自然键与展示键；
  - `display_name` 与 `active / inactive` 作为当前主档快照事实。
- `370` 不得自行重算 `pernr -> person_uuid`、不得把任职历史冒充 Person 主档、不得绕开 Person 的 active/inactive 生命周期语义。

### 7.2 Reporting 与业务域

- 报表与数据工作台不属于本计划范围，由 `380` 独立承接。

### 7.3 Assistant 与业务域

- Chat Assistant 不属于本计划范围，由 `390` 独立承接。
- 当 Assistant 发起需审批或异步执行的动作时，`370` 负责提供可查询的审批状态与执行回执，而不是要求 `390` 直接观察业务主表变化。
- `370` 向 `390` 暴露审批状态、执行回执与失败原因时，必须沿用同一 `OrgContext + time anchor` 解释链，不得发明工作流专用容器键。

### 7.4 `370` 对 `362` 的显式消费

- 人员相关审批摘要至少应稳定包含：`person_uuid / pernr / display_name / status`。
- 人员相关 before/after 快照必须以 Person 当前主档快照为基准，不得让 Workflow 或 Integration 层发明第二套人员字段真值。
- 当人员状态变化会影响下游负责人引用、任职资格或集成出站时，`370` 必须把该影响作为治理结果暴露出来，但不拥有决定权；决定权仍由 `362/364/361` 对应业务域拥有。
- 外部集成若消费人员数据，必须显式声明自己读取的是“当前 Person 快照”还是“人员相关操作历史”，不得混淆为 effective-dated 人员版本线。

## 8. 前端交付面

- `/app/workflow/todos`
- `/app/workflow/requests`
- `/app/integrations`
- `/app/audit`

## 9. 验收标准

- [ ] 关键业务变更可进入审批流并保留审批轨迹。
- [ ] 审计增强已能支撑审批、关键动作与集成执行的追踪。
- [ ] 审批中与异步执行中的动作具备统一状态与回执查询路径，供 UI 与 Assistant 复用。
- [ ] 外部系统同步不再散落在各模块内部，而有统一的集成边界。
- [ ] 涉及人员的审批、审计与集成回执已经显式消费 `362` 的身份锚点、当前主档快照与 active/inactive 生命周期，不再各自重写人员语义。
- [ ] 审批摘要、回执与集成执行上下文已显式包含 `org_context` 与时间锚点，不存在隐藏容器键或第二解释链。

## 10. 后续拆分建议

1. [ ] `371`：Workflow / Approval 详细设计（状态模型、审批轨迹与回执接口）
2. [ ] `372`：Audit Enhanced 详细设计（快照、回执与长事务状态追踪）
3. [ ] `373`：Integration Hub 详细设计
