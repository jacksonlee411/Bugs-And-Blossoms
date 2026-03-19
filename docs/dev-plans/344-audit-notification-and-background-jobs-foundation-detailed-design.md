# DEV-PLAN-344：Audit / Notification / Background Jobs 基座详细设计

**状态**: 规划中（2026-03-18 11:28 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的 `344` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“平台审计底座 + 系统任务 + 通知 + Hangfire 作为后台任务运行基线”的冻结；
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md) 对“当前状态、append-only 回执、证据快照必须分层”的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对“租户隔离、服务身份、密钥与 Assistant 安全治理”的冻结；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对登录、登出、拒绝访问、session 回收等平台审计事件与租户上下文的冻结；
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)、[DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)、[DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 对增强审计、批次任务、通知、异步执行、Assistant 回执与运行治理的依赖。

`340` 已经说明：审计、通知、后台任务属于平台能力，不归属某个 HR 业务域。  
如果没有 `344`，后续实现很容易各自长出：

- 第二套审计日志；
- 第二套任务调度入口；
- 第二套通知发送链路；
- 把 `audit log`、`operation receipt`、`delivery attempt`、`job run` 混成同一个概念。

`344` 的职责就是把这层平台公共能力冻结成 **可复用、可审计、可追踪、可 fail-closed 的共享基座**，供 `343/350/370/380/390` 直接消费。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述平台审计、通知与后台任务，不让 `Hangfire job`、`SMTP provider`、`webhook retry`、`log row` 这些实现词汇喧宾夺主。
- [ ] 冻结 `PlatformAuditEvent`、`NotificationIntent`、`NotificationDelivery`、`BackgroundJob`、`BackgroundJobRun` 的共享业务对象、ownership 与边界。
- [ ] 冻结平台审计日志与 `323` 中共享回执/快照的分工，避免“审计表即当前状态”或“回执流即审计”。
- [ ] 冻结后台任务的租户上下文、服务身份、幂等、重试、人工重试与失败停靠语义。
- [ ] 冻结通知意图、收件人解析、渠道投递、投递回执与用户可见通知中心的共享合同。
- [ ] 为 `343/350/370/380/390` 提供统一业务需求输入，使各模块只能复用平台审计/通知/任务能力，不再自造第二套基座。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)；状态票据、回执与证据快照的共享模式仍以 `323` 为主。
- [ ] 本计划不替代 [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)；租户隔离、service identity、secret 引用与 Assistant 安全 stopline 仍以 `333` 为主。
- [ ] 本计划不定义工作流审批状态机、导入批次领域规则或 Assistant 编排细节；这些分别由 `371/381/392/394` 承接。
- [ ] 本计划不要求第一阶段就引入消息总线、Kafka、复杂事件平台或多活通知编排。
- [ ] 本计划不允许业务模块各自维护 SMTP/webhook sender、偷偷起自建 cron、或把普通业务日志当审计日志。

## 3. “业务规则优先”在平台公共能力中的翻译

### 3.1 用户真正关心的是“系统做了什么、通知了谁、任务跑到哪”，不是底层 runner

用户关心的不是：

- 某个任务是不是 Hangfire worker 执行；
- 邮件是走 SMTP 还是 API provider；
- 某条日志是不是落在单独 schema。

用户真正关心的是：

- 某个高风险操作是否留下了可信审计；
- 某个通知到底有没有发出、发给了谁、为什么失败；
- 某个后台任务现在是否仍在运行、失败后能不能重试；
- 系统能否把这些能力统一暴露给 UI、Assistant、运营和控制面。

### 3.2 审计、回执、通知、任务是四层相关能力，不是一层

`344` 冻结以下分层：

1. 审计回答“系统发生过什么平台级事件”；
2. 回执回答“当前动作/长事务走到哪一步”，由 `323` 统一建模；
3. 通知回答“谁应该被提醒、通过什么渠道被投递、投递结果如何”；
4. 后台任务回答“哪个异步执行被安排、目前在哪次运行、为何失败或成功”。

### 3.3 平台基座拥有运行时能力，不拥有业务主规则

这意味着：

- `344` 可以提供统一调度、统一投递、统一审计入口；
- 但它不决定某个审批是否需要发通知、不决定某个业务动作是否允许执行；
- 业务模块提交的是意图，平台基座执行的是公共运行时职责。

### 3.4 `audit log` 不是 `operation receipt`

`344` 明确禁止：

- 把审计日志直接当作长事务当前状态；
- 把任务执行回执直接当作审计主记录；
- 把通知投递结果直接当作业务动作是否成功的唯一判断。

共享原则是：

- 审计记录“发生了什么”；
- 回执记录“当前走到哪”；
- 通知记录“谁被告知、投递如何”；
- 任务记录“异步执行如何被调度与运行”。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

#### 4.1.1 `300` 已冻结平台公共能力属于 `340`

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确：
  - 平台层拥有 `Audit / Task / Notification`；
  - 后台任务默认运行基线为 `Hangfire`；
  - 审计与通知基座是第一阶段正式能力，不是后补脚手架。

#### 4.1.2 `340` 已明确平台 ownership

- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 已明确：
  - 审计、通知、任务调度属于平台能力，不属于 HR 业务域；
  - 业务模块只能复用基座，不允许重新实现第二套任务系统、第二套审计日志。

#### 4.1.3 `323` 已冻结共享状态与证据语言

- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md) 已明确：
  - 当前状态、append-only 回执、证据快照必须分层；
  - `344` 应把 `BackgroundTask`、平台级 `OperationReceipt` 纳入共享模式；
  - 平台审计日志与共享回执要分层。

#### 4.1.4 `333/341` 已冻结安全与租户边界

- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 已明确：
  - 后台任务、导出、集成、评测任务都必须显式声明 `tenant_id` 或平台级例外；
  - `344` 必须记录密钥读取、租户例外、服务身份与 Assistant 工具调用关键事件。
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 已明确：
  - 登录、登出、拒绝访问、session 回收应成为标准审计事件；
  - 通知与后台任务若代表某租户运行，必须显式声明租户上下文。

### 4.2 当前主要缺口

1. [ ] **平台审计、通知、任务仍只有概念，还缺共享合同**  
   现在只有 `340` 的 ownership，没有一份详细计划解释如何分层与复用。

2. [ ] **通知语义尚未冻结**  
   “站内通知”“邮件”“Webhook”“失败重试”“已读/未读”还没有统一对象模型。

3. [ ] **任务语义尚未冻结**  
   当前缺少“任务定义、任务运行、重试、人工重试、tenant scope、service identity”的统一合同。

4. [ ] **平台审计与增强审计边界尚未冻结**  
   若不先定义，`370` 很容易把平台审计与业务增强审计混在一起。

## 5. Audit / Notification / Background Jobs 的目标业务蓝图

### 5.1 领域使命

`344` 是平台内“**谁记录平台级可审计事件、谁负责通知投递、谁负责异步任务运行，以及这些能力如何在租户边界与共享状态合同内被后续模块复用**”的唯一业务权威。  
它不拥有工作流规则、不拥有业务主数据、不拥有 Assistant 编排语义。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由 `344` 拥有 |
| --- | --- | --- |
| `PlatformAuditEvent` | 平台级 append-only 审计事件，回答“发生过什么” | 是 |
| `AuditActorRef` | 触发事件的主体、服务身份或系统来源引用 | 是（共享合同） |
| `NotificationIntent` | 系统确认“应该通知谁、通知什么主题”的主档 | 是 |
| `NotificationRecipient` | 某次通知的收件对象与渠道偏好解析结果 | 是 |
| `NotificationDelivery` | 渠道级投递尝试与结果历史 | 是 |
| `BackgroundJob` | 被平台调度的异步执行主档 | 是 |
| `BackgroundJobRun` | 某次实际运行尝试与结果历史 | 是 |
| `OperationReceipt` | 长事务/动作票据的阶段回执 | 否，`323` 拥有共享合同，`344` 只消费 |
| `EvidenceSnapshot` | before/after/submitted/decision 等证据快照 | 否，`323` 拥有共享合同，`344` 只消费查询与关联 |

### 5.3 面向系统的主能力

- 记录平台级审计事件
- 查询平台审计时间线
- 发起用户可见通知意图并按渠道投递
- 记录通知投递历史、失败原因与最终状态
- 调度后台任务、记录执行运行历史并支持受控重试
- 为 `370/380/390` 提供可复用的任务、通知、审计运行基座

### 5.4 Greenfield 选定的交付形态

#### 5.4.1 审计：统一平台审计表 + 显式对象引用

- 平台审计以 append-only 形式记录平台层事件；
- 审计事件至少应能关联：
  - actor
  - tenant
  - object refs
  - correlation ids
  - message code
- 高风险动作可关联 `323` 的 `EvidenceSnapshot`，但不能把快照本体混写进审计主表。

#### 5.4.2 通知：`Intent + Delivery` 双层模式

- `NotificationIntent` 回答“系统为什么要通知、通知主题是什么、目标人群是谁”；
- `NotificationDelivery` 回答“某个渠道实际投递了几次、结果如何”；
- 站内通知、邮件、Webhook 是首批正式渠道；
- 渠道失败可重试，但不得改变原始意图。

#### 5.4.3 任务：`BackgroundJob + JobRun` 双层模式

- `BackgroundJob` 回答“平台安排了什么异步执行”；
- `BackgroundJobRun` 回答“实际跑了几次、每次结果如何”；
- `Hangfire` 是首批运行时基线，但不改变业务合同；
- 当前状态可投影，运行历史必须 append-only。

## 6. `344` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 登录/登出/拒绝访问 | 回答“系统是否留下可信入口审计” | 必须生成平台审计事件；可关联 tenant/principal/session refs | 入口事件可追溯 |
| 高风险平台操作 | 回答“谁改了什么、在什么上下文下改的” | 审计事件必须 append-only，关联 actor、tenant、对象引用、message code | 控制面与平台事件可追踪 |
| 发送通知 | 回答“该不该通知、发给谁、结果如何” | 先有 `NotificationIntent`，再有渠道级 `NotificationDelivery`；失败不得覆盖原始意图 | 通知可解释、可重试 |
| 后台异步执行 | 回答“任务是否已被安排并执行到哪” | 先有 `BackgroundJob`，再有 append-only `JobRun`；当前状态可投影 | 任务状态稳定可查 |
| 平台级重试 | 回答“失败任务能否重新跑且不重复副作用” | 重试必须显式触发并绑定幂等语义，不允许私自复制出第二任务链 | 失败修复可控 |
| Assistant/集成高风险链路 | 回答“平台有没有留下运行与安全证据” | 工具调用、租户例外、secret 读取、服务身份任务都应进入平台审计 | AI 与集成可审计 |

## 7. 共享合同、不变量与实现护栏

### 7.1 平台审计合同

每条 `PlatformAuditEvent` 至少应携带：

- `tenant_id` 或显式平台级例外标记；
- actor / service identity 引用；
- 事件类型与 `message_code`；
- 触发时间；
- object refs；
- `request_id` / `trace_id` / `correlation_id`；
- 结果分类（如 `accepted / rejected / failed / completed`）。

并冻结以下规则：

- 平台审计必须 append-only；
- 平台审计不回答长事务当前状态；
- 平台审计不可被普通业务日志替代；
- 高风险事件必须可被控制面与审计页查询。

### 7.2 平台审计与增强审计分层合同

- `344` 拥有平台级、横切性、运行态事件审计；
- `370` 拥有业务增强审计，如 before/after snapshot、审批轨迹、集成执行证据；
- 两者可以通过稳定引用与 `correlation_id` 关联，但不得互相吞并。

### 7.3 通知合同

- `NotificationIntent` 是通知主档，记录 why / who / subject / importance。
- `NotificationDelivery` 是渠道级投递记录，记录 channel / attempt / provider response / final status。
- 不允许直接只记“发了一封邮件”而没有通知主档。
- 站内通知应支持读/未读，但“已读”不改变原始通知意图与投递历史。
- 通知模板与配置可消费 `345` 的配置能力，但 `344` 不拥有模板业务语义本身。

### 7.4 后台任务合同

- `BackgroundJob` 必须显式声明：
  - job kind
  - tenant scope 或平台级例外
  - service identity
  - idempotency boundary
  - trigger source
- `BackgroundJobRun` 必须 append-only 记录每次运行尝试、开始/结束时间、结果分类与错误摘要。
- 当前任务状态可以投影，但运行历史不可被覆盖。
- 不允许任何业务模块偷偷起本地 cron 或直接绕过平台调度器。

### 7.5 安全与租户边界合同

- 后台任务、通知投递、provider 调用都必须遵守 `333` 的 tenant / secret / Assistant 安全治理。
- 平台级例外必须显式标记并可审计，不能默认视为“系统任务所以全租户可见”。
- 密钥只允许通过 `SecretReference` 或等价间接引用注入，不得写入审计、通知 payload 或任务错误详情。

### 7.6 API 与 UI 交付合同

建议至少提供以下平台入口：

#### 7.6.1 API

- `GET /api/admin/audit-logs`
- `GET /api/admin/audit-logs/{id}`
- `GET /api/notifications`
- `POST /api/notifications/{id}:read`
- `GET /api/admin/jobs`
- `GET /api/admin/jobs/{id}`
- `POST /api/admin/jobs/{id}:retry`

#### 7.6.2 UI

- `/admin/audit`
- `/app/notifications`
- `/admin/jobs`
- `/admin/jobs/:id`

### 7.7 实现护栏

- 不允许把通知投递结果直接当作业务动作最终状态。
- 不允许把 `BackgroundJobRun` 当作用户可见长事务票据的唯一接口；用户可见进度仍应消费 `323` 的票据/回执合同。
- 不允许在业务模块里复制第二套审计表、第二套通知发送器、第二套任务调度器。
- 不允许平台基座反向拥有业务动作的主规则。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `343`（Superadmin 控制台与租户生命周期）的输入

- [ ] 控制面高风险操作必须统一进入 `PlatformAuditEvent`。
- [ ] 租户启停、初始化管理员、域名绑定等控制面动作应能发起平台通知与后台任务，而不是自造运行链路。

### 8.2 对 `350`（前端产品壳与交互系统）的输入

- [ ] 审计页、通知中心、任务中心应复用统一的列表/详情/状态徽标/时间线交互语言。
- [ ] 前端不得自己拼第二套“任务当前状态”。

### 8.3 对 `370`（工作流、审计增强与集成）的输入

- [ ] `370` 的增强审计必须建立在 `344` 的平台审计基座之上，而不是重做审计基础设施。
- [ ] 集成执行若需要异步运行，应复用 `BackgroundJob` 基座；集成回执仍通过 `323/370` 共享状态语言对外暴露。

### 8.4 对 `380`（数据工作台与运营分析）的输入

- [ ] 导入、导出、同步批次若需要异步执行，应复用 `BackgroundJob` 基座与通知能力。
- [ ] 数据工作台的行级错误、批次回执与用户提醒可以叠加在 `344` 基座之上，但不得重造任务系统。

### 8.5 对 `390`（Chat Assistant）的输入

- [ ] Assistant 的评测、异步编排、通知提醒与审计记录应复用 `344` 基座。
- [ ] Assistant 的用户可见动作进度仍走 `323/392` 票据与回执合同，`344` 只提供运行底座与平台审计。

## 9. 建议实施分期

1. [ ] `M1`：共享词汇与分层冻结  
   冻结 `PlatformAuditEvent / NotificationIntent / NotificationDelivery / BackgroundJob / BackgroundJobRun` 及其与 `OperationReceipt` 的分层。
2. [ ] `M2`：平台审计基座冻结  
   明确 actor/tenant/correlation/object refs/message code 合同与审计页查询口径。
3. [ ] `M3`：通知基座冻结  
   明确意图、收件人解析、渠道投递、已读状态与失败重试语义。
4. [ ] `M4`：后台任务基座冻结  
   明确任务主档、运行历史、幂等、tenant scope、service identity 与人工重试语义。
5. [ ] `M5`：跨计划接线  
   让 `343/370/380/390` 可以直接以 `344` 为运行时平台基座输入。

## 10. 建议目录与落点

若按 `300` 的模块化单体落地，建议采用以下 ownership 落点：

- `src/Platform/Audit/`：`PlatformAuditEvent`、查询聚合、审计页 API
- `src/Platform/Notifications/`：`NotificationIntent`、`NotificationDelivery`、通知中心 API
- `src/Platform/Jobs/`：`BackgroundJob`、`BackgroundJobRun`、任务管理 API
- `src/Shared/Lifecycle/`：共享状态分类、回执/快照合同类型（消费 `323`）
- `src/Platform/Security/`：service identity、secret reference 与高风险审计接线（消费 `333`）

其中：

- `Platform/*` 拥有运行时基座；
- `Shared/Lifecycle` 只承载共享合同，不拥有平台主表；
- 各业务模块只能消费这些基座，不得复制实现。

## 11. 验收标准

- [ ] `344` 已成为 Greenfield 对平台审计、通知与后台任务的单一事实源。
- [ ] 平台审计、共享回执、通知投递、任务运行四层语义已清晰分离。
- [ ] `343/350/370/380/390` 能直接消费 `344`，而不是继续各自发明第二套基座。
- [ ] 任务、通知、审计都具备显式 tenant scope / platform exception、service identity、correlation ids 与可查询入口。
- [ ] `Hangfire` 只被用作运行时执行基线，不会反向污染业务合同。

## 12. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
