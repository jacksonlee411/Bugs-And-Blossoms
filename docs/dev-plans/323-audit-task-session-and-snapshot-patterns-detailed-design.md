# DEV-PLAN-323：审计、任务、会话与快照模式详细设计

**状态**: 规划中（2026-03-18 08:28 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 的 `323` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“关系型主模型 + 审计日志 + 后台任务 + 受控 Assistant”的冻结；
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 对 `Pattern B = 主档 + 操作快照`、`current / as_of / history` 与显式时间上下文的冻结；
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 对平台公共能力中 `audit / jobs / session` 的 ownership；
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)、[DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)、[DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 对“回执、批次、状态票据、会话、快照”已经提出的显式需求。

`320` 已经说明：如果没有一份单独的共享计划冻结“审计、任务、会话与快照模式”，后续模块会各自发明：

- 什么才算“当前状态票据”；
- 什么才算“append-only 回执”；
- before/after snapshot 与审计日志之间是什么关系；
- 会话、后台任务、导入批次、审批请求、Assistant `action request` 到底哪些应共享模式，哪些只能复用术语；
- UI、Assistant、集成与运维应该查询哪一个权威状态入口。

`323` 的职责就是把这层语义收敛为 **平台级共享状态与证据合同**，让 `340/370/380/390` 在同一套语言上落地，而不是继续产生第二事实源。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述会话、任务、回执与快照，不让 `job table`、`audit row`、`webhook callback`、`polling flag` 这些实现词汇喧宾夺主。
- [ ] 冻结交互会话、后台任务、导入批次、导出任务、审批请求、Assistant `action request`、操作回执、证据快照的共享业务对象与边界。
- [ ] 冻结“当前状态投影”与“append-only 证据流”的分工，避免后续子计划把业务主表、审计表、任务表混作同一语义层。
- [ ] 冻结长事务与异步任务的共享状态分类、查询合同、相关标识与幂等边界。
- [ ] 冻结 before / after / submitted / decision 等快照类型的业务语义，确保审计、审批、Assistant、导入导出使用同一套证据语言。
- [ ] 为 `340/350/370/380/390` 提供统一业务需求输入，保证 UI、Workflow、Workbench、Assistant 都通过显式票据与回执观察进度，而不是猜测业务表变化。

### 2.2 非目标

- [ ] 本计划不直接定义最终数据库 DDL、ORM 映射、API 路由与消息基础设施实现；这些由后续 `344/371/372/381/382/391/392/394` 承接。
- [ ] 本计划不替代 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)；`current / as_of / history` 与 effective-dated 规则仍以 `322` 为主。
- [ ] 本计划不要求所有状态对象物理落到同一张“万能状态表”；共享的是业务合同与模式，而不是强推单表实现。
- [ ] 本计划不允许把“业务主表是否变化”重新当作长事务状态的唯一查询方式。
- [ ] 本计划不为追求开发省事引入 legacy 双轨、隐式 fallback 或“前端自己拼状态”的旁路。

## 3. “业务规则优先”在共享状态与证据合同中的翻译

### 3.1 用户真正关心的是“现在进展到哪里、为什么”，不是底层日志表

用户关心的不是：

- 某次审批落在哪张表；
- 后台任务是不是由 Hangfire 驱动；
- Assistant 回执是从哪条内部消息拼出来的；
- 某个导入批次是不是轮询了 5 次。

用户真正关心的是：

- 我发起的动作现在处于什么阶段；
- 为什么被卡在待审批、执行中、失败或已完成；
- 系统能否给出最近一次明确回执；
- 我能否看到这次变更的关键证据快照；
- 失败后下一步应该修什么，而不是去猜业务表是否已经被写入。

### 3.2 会话、任务、状态票据、回执、快照都是一级业务对象

`323` 冻结以下分层：

1. 会话回答“当前交互上下文是否仍然可信”；
2. 状态票据回答“这次动作或长事务当前走到哪一步”；
3. 回执回答“系统在什么时间明确确认了什么阶段性事实”；
4. 快照回答“这次动作围绕什么对象、以什么证据执行或失败”。

### 3.3 “当前状态”与“证据历史”必须分层

`323` 不允许把以下几层混在一起：

- 把 append-only 审计流直接当作当前状态；
- 把当前状态字段改写成唯一历史；
- 把业务主表是否已变更当作回执接口；
- 把快照直接当成主业务写模型。

共享原则是：

- “当前状态”可以是可重建的投影；
- “证据历史”必须是 append-only；
- “快照”是证据，不是第二写入口。

### 3.4 轮询、推送、通知只是传输方式，不是权威状态语义

后续可以有：

- `GET` 查询；
- SSE / WebSocket / webhook 推送；
- 通知中心提醒。

但它们都不能改变一个主合同：

- **每个长事务/异步动作都必须有稳定、可查询的权威状态入口。**

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

#### 4.1.1 `340` 已拥有平台公共能力入口

- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 已明确平台层拥有：
  - `audit_logs`
  - `background_jobs`
  - 本地可撤销 `sessions`
  - 平台通知与系统任务

#### 4.1.2 `370/380/390` 已把显式回执设为上层要求

- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md) 已明确：
  - 工作流实例与 `operation_receipts` 必须提供稳定、可查询的生命周期状态；
  - 审计增强必须覆盖 before/after 快照、审批轨迹与长事务状态回执。
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md) 已明确：
  - 导入按批次；
  - 导出按任务；
  - 失败必须有行级错误回执。
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 已明确：
  - `assistant_action_requests` 是显式状态票据；
  - `assistant_action_receipts` 承载审批结果、执行回执与异步任务反馈；
  - Assistant 不允许通过猜业务表变化来判断动作是否完成。

#### 4.1.3 `322` 已把 Pattern B 指向 `323`

- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 已冻结：
  - 并非所有对象都走 effective-dated 主模型；
  - 审批记录、会话、任务、配置项等更接近 `主档 + 操作快照`；
  - 审计留存、任务回执、会话与操作快照的独立模式由 `323` 承接。

### 4.2 当前主要缺口

1. [ ] **共享词汇仍未冻结**  
   `session / job / batch / request / receipt / snapshot` 已在多个计划出现，但还没有一份 SSOT 说明各自回答什么业务问题。

2. [ ] **状态分类尚未统一**  
   后续模块容易各自长出“pending/running/approved/done/error”之类枚举，但没有共享的上层语义分类。

3. [ ] **相关标识与幂等边界尚未统一**  
   `tenant_id / principal_id / request_id / trace_id / ticket_id / receipt_id / idempotency_key` 的组合关系尚未冻结。

4. [ ] **状态查询入口尚未统一**  
   UI、Assistant、集成、后台任务都需要查询权威进度，但当前仍缺共享查询合同。

## 5. 共享状态与证据合同蓝图

### 5.1 领域使命

`323` 是平台内“**哪些对象拥有当前状态、哪些对象负责回执历史、哪些对象保存关键快照证据，以及不同消费者应通过什么入口观察它们**”的共享建模权威。  
它不拥有业务主数据写入，也不替代具体业务域的状态机细节。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由 `323` 拥有共享合同 |
| --- | --- | --- |
| `UserSession` | 租户内交互式登录会话的当前真值 | 是 |
| `ConversationSession` | Assistant 会话与 turn 的容器对象 | 是 |
| `BackgroundTask` | 平台后台异步任务的执行主档 | 是 |
| `ProcessingBatch` | 导入、导出、同步等按批次治理的主档 | 是 |
| `OperationTicket` | 面向 UI / Assistant / 集成暴露的显式状态票据 | 是 |
| `OperationReceipt` | append-only 阶段性回执与错误/结果证据流 | 是 |
| `EvidenceSnapshot` | before / after / submitted / decision 等关键证据快照 | 是 |
| `WorkflowInstance` | 审批流程主档 | 否，`371` 拥有业务状态机，`323` 只拥有共享模式 |
| `AssistantActionRequest` | Assistant 发起的可写动作票据 | 否，`392` 拥有业务状态机，`323` 只拥有共享模式 |

### 5.3 面向用户的主能力

- 查询某个长事务“当前在哪一步”
- 查看最近一次明确回执与失败原因
- 查看关键 before / after / submitted / decision 快照
- 在审批、异步执行、批量处理之间复用同一套“状态票据 + 回执”交互语言
- 让 UI、Assistant、集成与运维都消费同一条权威状态查询路径

### 5.4 共享模式族

#### 5.4.1 Pattern S：会话模式

适用对象：

- 交互式登录会话
- Assistant 对话会话

共享要求：

- 有稳定身份；
- 有当前有效性；
- 可被撤销或结束；
- 与 `tenant + principal` 或等价上下文显式绑定。

#### 5.4.2 Pattern T：任务 / 批次模式

适用对象：

- 后台任务
- 导入批次
- 导出任务
- 集成执行批次

共享要求：

- 有稳定主档；
- 有当前状态；
- 有 append-only 回执流；
- 失败时能暴露结构化错误与可见证据。

#### 5.4.3 Pattern R：状态票据 + 回执模式

适用对象：

- 审批请求
- 长事务请求
- Assistant `action request`
- 需要给外部调用方持续查询的执行请求

共享要求：

- 存在可查询的 `OperationTicket`；
- 所有阶段变化写入 `OperationReceipt`；
- 最新状态可投影，但不得丢失历史回执；
- 调用方不应通过读取业务主表推断是否完成。

#### 5.4.4 Pattern P：证据快照模式

适用对象：

- 关键写动作的 before / after
- 审批提交时的 submitted payload
- 决议、拒绝、失败时的 decision / error context

共享要求：

- 快照面向证据与解释；
- 快照与主业务对象通过稳定引用关联；
- 快照可存摘要与外部化大对象引用，但不能变成第二主写事实源。

## 6. `323` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 读取会话 | 回答“我当前这次交互是否仍然有效” | 会话必须有显式失效/撤销语义；不得通过客户端自猜会话真值 | 会话有效性可统一判断 |
| 查询审批/异步进度 | 回答“它现在卡在哪一步” | 必须查权威票据或任务主档；不得直接观察业务主表变化 | UI 与 Assistant 看到同一进度 |
| 读取最近回执 | 回答“系统最后明确确认了什么” | 回执必须 append-only，并带时间、阶段、原因、来源 | 失败与成功都可解释 |
| 查看变更证据 | 回答“到底改了什么、按什么提交的” | 快照必须与动作/票据稳定绑定；before/after 与 decision 不能混写 | 审批、审计、导入导出可复用同一证据语言 |
| 后台批处理 | 回答“这一批处理了多少、错在哪里” | 批次有主档；行级错误以回执或结构化错误清单对外可见 | 数据工作台可追踪批次结果 |
| Assistant 可写动作 | 回答“模型发起的动作现在执行到哪” | `action request` 只观察票据、审批状态、回执；不得绕回业务表猜结果 | Assistant 不越权也不失联 |

## 7. 共享合同、不变量与实现护栏

### 7.1 身份与 ownership 合同

每个共享状态对象都必须显式声明：

- `tenant_id` 或等价租户边界；
- `owner_module` 或等价 owning bounded context；
- `created_by_principal`、`created_by_system` 或等价来源；
- 与 `request_id / trace_id / correlation_id` 的关联。

### 7.2 状态生命周期合同

- 每个 `OperationTicket`、`BackgroundTask`、`ProcessingBatch` 都必须有可查询的当前状态。
- 当前状态可以是投影，但必须能从 append-only 回执重建。
- 共享上层状态分类至少应覆盖：
  - `pending`
  - `waiting_external`
  - `waiting_approval`
  - `running`
  - `succeeded`
  - `failed`
  - `cancelled`
  - `expired`
- 具体领域子计划可以细化枚举，但不得跳出这套共享语义分类。

### 7.3 回执合同

每条 `OperationReceipt` 至少应携带：

- 所属主档或票据引用；
- 发生时间；
- 阶段类型；
- 上层状态分类；
- 结构化 `message_code`；
- 人类可读摘要；
- 可选的结构化详情与错误上下文；
- 触发来源与相关标识。

并冻结以下规则：

- 回执必须 append-only；
- 回执不可偷偷改写历史；
- 最近一次回执可投影为“当前状态摘要”，但历史回执不可丢失。

### 7.4 快照合同

`EvidenceSnapshot` 的共享类型至少包括：

- `submitted`
- `before`
- `after`
- `decision`
- `error_context`

并冻结以下规则：

- 快照只回答“证据是什么”，不回答“当前状态是什么”；
- 快照必须与主档/票据/回执至少一种稳定引用绑定；
- 快照可保存摘要与外部对象存储引用，但引用必须可审计追溯。

### 7.5 查询与订阅合同

- 每个长事务/异步对象都必须存在稳定 `GET` 查询入口。
- 推送、通知、webhook、SSE 都只能作为加速或提醒手段，不能替代权威 `GET`。
- UI、Assistant、集成与运维看见的“当前状态”必须来自同一条权威查询路径。

### 7.6 幂等、关联与重试合同

- 发起动作时必须显式绑定 `idempotency_key` 或等价重试保护键。
- 同一逻辑动作的重试，不得导致第二条主业务写链路。
- 重试可以追加回执，但不能制造“同一票据对应多次互相冲突的成功写入”。

### 7.7 实现护栏

- 不允许把业务主表变化重新当作票据完成状态的唯一判断条件。
- 不允许把审计日志表直接充当当前状态表。
- 不允许为了图省事，把所有状态对象物理并入一张无 ownership 的“万能流水表”。
- 不允许在没有票据的前提下，只靠前端局部状态拼出长事务进度。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `340`（平台与 IAM 基座）的输入

- [ ] `340/344` 应把 `UserSession`、`BackgroundTask`、平台级 `OperationReceipt` 纳入共享模式，而不是各自发明字段与状态语言。
- [ ] 平台审计日志与共享回执要分层：审计回答“发生过什么”，票据/回执回答“现在进行到哪里”。

### 8.2 对 `350`（前端产品壳与交互系统）的输入

- [ ] 列表、详情、工作台页的“状态徽标 / 时间线 / 失败原因 / 回执抽屉”应复用同一套状态与证据语言。
- [ ] 前端不得自己把多个接口返回拼成另一套“影子状态”。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] 业务模块若需要暴露长事务、异步写、批处理或关键证据快照，应直接消费 `323` 的共享模式。
- [ ] 业务主对象的当前事实仍由业务域拥有，不被 `323` 替代。

### 8.4 对 `370`（工作流、审计增强与集成）的输入

- [ ] `371/372/373` 必须沿用 `OperationTicket / OperationReceipt / EvidenceSnapshot` 共享语言。
- [ ] 审批实例状态机可以特化，但必须映射回 `323` 的上层状态分类。

### 8.5 对 `380`（数据工作台与运营分析）的输入

- [ ] `381/382/383` 必须让导入批次、导出任务、同步批次具备统一的任务/回执/快照模式。
- [ ] 行级错误与批次摘要应能通过共享回执或其稳定扩展形态查询。

### 8.6 对 `390`（Chat Assistant）的输入

- [ ] `391/392/394` 必须把 `conversation / action request / action receipt` 放在 `323` 共享模式之上实现。
- [ ] Assistant 对长事务的观察不得越过票据与回执接口，直接推断底层业务表。

## 9. 建议目录与落点

若按 `300` 的模块化单体落地，建议采用以下 ownership 落点：

- `src/Platform/IAM/`：`UserSession`
- `src/Platform/Jobs/`：`BackgroundTask`
- `src/Platform/Audit/`：`EvidenceSnapshot` 查询与审计聚合
- `src/Coordination/Workflow/`：workflow-specific `OperationTicket`
- `src/DataWorkbench/Import/` 与 `src/DataWorkbench/Export/`：`ProcessingBatch`
- `src/Assistant/Orchestration/`：`ConversationSession`、assistant-specific `OperationTicket`
- `src/Shared/Lifecycle/`：共享状态分类、相关标识、票据/回执/快照合同

其中：

- `Shared/Lifecycle` 只拥有共享合同与类型，不拥有业务主表；
- 各 bounded context 只拥有自己的主档与特化状态机，不得反向改写共享语义。

## 10. 验收标准

- [ ] `323` 已成为 Greenfield 对“会话、任务、票据、回执、快照”关系的单一事实源。
- [ ] `340/370/380/390` 能直接引用 `323`，而不是继续各自发明第二套状态语言。
- [ ] 当前状态、append-only 回执、证据快照三层语义已清晰分离。
- [ ] 长事务、审批、异步任务、导入导出、Assistant 可写动作都具备稳定权威查询入口。
- [ ] 共享状态分类、相关标识与幂等边界足够明确，可以进入后续实现计划而不再依赖口头解释。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
