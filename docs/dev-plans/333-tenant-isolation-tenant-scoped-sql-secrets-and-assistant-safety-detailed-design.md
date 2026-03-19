# DEV-PLAN-333：租户隔离、tenant-scoped SQL、密钥与 Assistant 安全治理详细设计

**状态**: 规划中（2026-03-18 08:28 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 的 `333` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“应用层强租户隔离 + 受控 Assistant + Linux 容器平台”的冻结；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对 `tenant + principal + session` 入口边界与 fail-closed 的冻结；
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)、[DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)、[DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 对导出、集成、后台任务、Assistant 工具调用的安全治理要求。

`330` 已经把安全、合规与数据治理冻结为上层约束，但如果没有 `333`，后续实现很容易继续出现：

- HTTP 入口做了租户校验，但 Raw SQL / Dapper / 后台任务绕过；
- 密钥、连接串、模型 provider key 只有“能用”口径，没有 tenant / environment / rotation 边界；
- Assistant 虽然受控，但缺少“哪些工具能看到什么数据、如何做脱敏与 stopline”的共享治理；
- 导出、集成、服务身份与系统任务默认长出“全租户”后门。

`333` 的职责就是把这几类风险收敛为 **平台级 fail-closed 安全合同**。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结租户隔离的共享治理合同，确保 HTTP、SQL、后台任务、导出、集成、Assistant 都遵守同一条边界。
- [ ] 冻结 `tenant-scoped SQL` 的准入规则、显式上下文要求与审查护栏。
- [ ] 冻结密钥、provider 凭据、连接信息、webhook secret 的分级、引用、注入与 rotation 合同。
- [ ] 冻结 Assistant 工具调用、检索、提示词、回执与导出相关的安全治理边界。
- [ ] 为 `341/342/344/370/380/390` 提供统一输入，避免后续子计划各自再发明第二套安全例外。

### 2.2 非目标

- [ ] 本计划不替代 `341` 的 tenant/session 入口规则；它消费 `341` 的租户与会话主链。
- [ ] 本计划不替代 `342` 的权限矩阵；它只定义安全 stopline 与边界，不定义业务动作授权矩阵。
- [ ] 本计划不把导出保留策略与法务留存细节展开到底；这些与 `332` 协同承接。
- [ ] 本计划不要求第一阶段就引入硬件密钥、MFA、零信任网格或复杂 KMS 编排。
- [ ] 本计划不允许为了实现便利而引入“默认全租户后台任务”“系统用户天然拥有全表 SQL”“Assistant 直连数据库”之类旁路。

## 3. “业务规则优先”在安全治理中的翻译

### 3.1 用户真正关心的是“不会串租户、不会泄密、不会被 AI 越权”

用户关心的不是：

- SQL 是 EF Core 还是 Dapper；
- 密钥存进 env 还是 secret store；
- 模型 provider 用哪家 SDK。

用户真正关心的是：

- 我看到的数据是不是只属于当前租户；
- 导出、集成、后台任务会不会越界拿到不该拿的数据；
- 高风险密钥会不会被错误注入、错误记录或错误暴露；
- Assistant 会不会绕过权限、越过租户边界或泄露敏感上下文。

### 3.2 fail-closed 不是入口一层的事，而是整条执行链的合同

`333` 冻结：

1. 入口确定租户边界；
2. 执行链显式携带租户上下文；
3. SQL / 任务 / 导出 / Assistant 工具调用都必须验证该上下文；
4. 任一环节上下文缺失，都必须拒绝，而不是默认为系统级全量访问。

### 3.3 密钥与 Assistant 都必须被视为高风险执行面

- Provider key、数据库凭据、webhook secret、签名密钥都不能只是“配置项”；
- Assistant 检索、工具调用、提示词组装、回执输出都要进入可审计、可拒绝、可脱敏的治理范围。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确：
  - 应用层租户隔离必须 fail-closed；
  - `tenant_id` 是不可变边界字段；
  - Assistant 只能通过受控 `Action Gateway` 发起可写动作；
  - Phase 1 不以数据库级 `RLS` 为默认前提，但不能因此放松边界。
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 已明确：
  - Dapper / Raw SQL 必须经过 tenant-scoped 查询抽象、审查规则或等效护栏；
  - 对话日志、工具调用、动作建议、回执都属于治理对象。

### 4.2 当前主要缺口

1. [ ] **缺少 tenant-scoped SQL 的共享准入合同**  
   现在只有原则，没有一份详细计划说明 SQL、批处理、只读查询、导出查询如何显式带租户。

2. [ ] **缺少密钥的统一 ownership 与注入边界**  
   provider key、webhook secret、服务连接信息还没有统一的“谁拥有、谁可引用、谁可轮换”合同。

3. [ ] **缺少 Assistant 安全治理的细化 stopline**  
   目前有“受控编排层”的上位原则，但还没有详细到工具、上下文、脱敏、错误回执的共享护栏。

## 5. 安全治理目标蓝图

### 5.1 领域使命

`333` 是平台内“**任何读取、执行、导出、集成或 Assistant 行为在租户边界、SQL 边界、密钥边界与高风险工具边界上应如何 fail-closed**”的共享治理权威。

### 5.2 核心治理对象

| 业务对象 | 业务含义 | 是否由 `333` 拥有共享合同 |
| --- | --- | --- |
| `TenantBoundaryPolicy` | 当前请求/任务/工具调用必须遵守的租户边界 | 是 |
| `TenantScopedQuery` | 允许进入数据库执行面的 tenant-scoped 查询合同 | 是 |
| `ServiceIdentity` | 代表租户或系统运行的后台任务/集成身份 | 是 |
| `SecretMaterial` | 高风险密钥与凭据本体 | 是 |
| `SecretReference` | 服务在运行时引用密钥的间接句柄 | 是 |
| `AssistantCapabilityGuard` | Assistant 工具、检索、动作与上下文装配的安全护栏 | 是 |

### 5.3 面向系统的主能力

- 显式拒绝缺少租户上下文的执行链
- 让所有 Dapper / Raw SQL / 导出查询走 tenant-scoped 护栏
- 让后台任务与服务身份显式声明代表哪个租户或为何允许平台级运行
- 让密钥通过受控引用注入，而不是散落为随手可读的配置文本
- 让 Assistant 的工具调用、检索和输出进入同一套安全 stopline

## 6. `333` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| HTTP/API 请求 | 访问当前租户数据 | 缺少 `TenantContext` 即拒绝；不得默认全租户 | 入口 fail-closed |
| Dapper / Raw SQL | 执行复杂查询或导出 | 查询必须显式绑定租户边界与调用来源；禁止匿名全表查询 | SQL 不穿透租户边界 |
| 后台任务 / 集成 | 代表租户执行批处理 | 任务必须声明 `tenant_id` 或显式平台级例外；例外需可审计 | 服务身份不默认为全租户 |
| 密钥读取 | 使用 provider key、webhook secret、连接串 | 代码只拿 `SecretReference`；原始密钥读取受 owner 与环境边界约束 | 密钥不四处复制 |
| Assistant 检索 | 装配上下文、读业务对象 | 检索必须受租户、权限、脱敏规则与工具 allowlist 约束 | AI 不泄露跨租户或超权限数据 |
| Assistant 可写动作 | 发起 action request | 只能通过注册动作、参数校验、confirm/approve/commit 边界执行 | AI 不越权直写 |

## 7. 共享合同、不变量与实现护栏

### 7.1 租户隔离合同

- 缺失 `TenantContext` 的请求、任务、工具调用一律 fail-closed。
- `tenant_id` 是不可变边界字段，不允许在执行链中被“修正”或“默认补全”。
- 平台级例外必须是显式例外，并带独立服务身份与审计，不允许以普通租户链路偷偷获得。

### 7.2 tenant-scoped SQL 合同

- 任何 Dapper / Raw SQL / 导出查询都必须通过显式 tenant-scoped 查询入口或等价封装。
- 封装至少应显式声明：
  - `tenant_id`
  - 调用来源
  - 查询用途
  - 是否只读
- 不允许出现“调试方便先查全表再过滤”的路径。
- 任何平台级跨租户查询都必须有单独入口、单独审计与最小化结果集。

### 7.3 服务身份与后台任务合同

- 后台任务、集成同步、导入导出、评测任务都必须显式声明自己代表哪个租户，或为何是平台级任务。
- 平台级任务不得复用普通租户用户的会话模型。
- 服务身份不拥有“天然全部能力”；仍需受作用域、用途与审计约束。

### 7.4 密钥与配置合同

- 原始密钥应由受控 secret 存储或等价机制管理，代码与普通配置优先持有 `SecretReference`。
- 密钥至少按环境、用途、owner、rotation policy 分层。
- 日志、回执、错误、快照不得回显原始密钥。
- 失效、轮换、吊销都必须可审计。

### 7.5 Assistant 安全治理合同

- Assistant 工具调用必须有 allowlist、输入校验、输出约束与审计。
- Assistant 读取的上下文必须经过租户边界、权限边界与必要脱敏。
- 提示词、工具参数、回执与错误消息不得泄露 secret、连接串或跨租户敏感数据。
- 模型不得直接持有数据库写权限，不得自由拼接内部 API。

### 7.6 stopline 与故障处置合同

- 一旦发现租户穿透、secret 泄露、Assistant 越权、Raw SQL 未带租户上下文，必须视为 stopline 级问题。
- 处置顺序应为：
  1. 入口/任务停用或只读保护
  2. 前向修复配置与代码
  3. 补审计与证据
  4. 恢复服务
- 不允许以“临时 fallback 到旧实现”作为恢复手段。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `341/342/344` 的输入

- [ ] `341` 的 tenant/session 合同是 `333` 的前置，不允许双向漂移。
- [ ] `342` 的权限矩阵必须建立在 `333` 的 stopline 之上，而不是假设入口天然可信。
- [ ] `344` 的审计、通知、任务基座必须记录密钥读取、租户例外、服务身份与 Assistant 工具调用关键事件。

### 8.2 对 `370/380` 的输入

- [ ] Workflow、集成、导入、导出、同步任务都必须显式声明 tenant scope 或平台级例外。
- [ ] 导出、批处理与集成查询必须走 tenant-scoped SQL 合同，不得把“报表场景”当例外。

### 8.3 对 `390`（Chat Assistant）的输入

- [ ] `390/392/394` 必须把工具 allowlist、数据脱敏、上下文最小化、错误回执安全化纳入主实现合同。
- [ ] Assistant 的检索与动作链路不得绕过 `333` 的租户、SQL 与 secret 护栏。

## 9. 建议目录与落点

若按 `300` 的模块化单体落地，建议采用以下 ownership 落点：

- `src/Platform/Security/Isolation/`：tenant boundary policy、tenant-scoped SQL 守卫
- `src/Platform/Security/Secrets/`：`SecretReference`、rotation policy、provider credentials
- `src/Platform/Security/AssistantSafety/`：tool guard、prompt/output redaction、model/provider access policy
- `src/Shared/Data/TenantScopedSql/`：共享查询合同与审查辅助类型

其中：

- `Shared/Data/TenantScopedSql` 只承载共享合同与 helper，不直接拥有业务查询；
- Assistant 相关安全策略仍由 `src/Assistant/Orchestration/` 消费，不能把 Assistant 主逻辑塞回安全模块。

## 10. 验收标准

- [ ] `333` 已成为 Greenfield 对“租户隔离、tenant-scoped SQL、密钥、Assistant 安全治理”的单一事实源。
- [ ] 入口、SQL、后台任务、导出、集成、Assistant 使用同一条 fail-closed 安全边界。
- [ ] 后续 `341/342/344/370/380/390` 可以直接消费 `333`，而不是继续各自发明例外。
- [ ] 密钥引用、服务身份、Assistant 工具调用与高风险查询都具备可审计 ownership 与 stopline。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
