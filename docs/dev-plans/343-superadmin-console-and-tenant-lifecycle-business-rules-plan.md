# DEV-PLAN-343：Superadmin 控制台与租户生命周期业务规则优先蓝图

**状态**: 规划中（2026-03-18 14:21 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的 `M4: Superadmin 控制台` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“多租户 + 本地会话 + Superadmin 控制台 + Linux 容器平台”的总体冻结；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对“Tenant App 与 Control Plane 必须分层、租户停用即时生效、控制面身份链路不得污染租户会话主链”的冻结；
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 对“`platform.control` 是独立高风险权限包、`superadmin` 不是租户业务数据面全能角色”的冻结；
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md) 对“控制面动作必须进入平台审计、通知与后台任务基座”的冻结。

`343` 的职责不是重复定义登录、权限或任务基础设施，而是把“**谁能在全局控制面管理租户、租户何时可开通/停用、域名与首个管理员如何成为正式平台能力、这些高风险动作如何留下可查询证据**”冻结成 Greenfield 平台语言，作为后续实现与其他子计划的共享输入。

本计划不继承现仓历史控制面的实现包袱，尤其不接受：

- 把 `superadmin` 视为默认可读写任意租户业务数据的超级身份；
- 通过 tenant app 的现有 session“顺手切进”控制面；
- 通过手工改库、临时脚本或隐式 fallback 完成租户开通。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 Superadmin 控制面，不让脚本、迁移、运维习惯替代业务合同。
- [ ] 冻结控制面核心业务对象：控制面主体、受管租户、租户生命周期、开通请求、首个管理员初始化、激活前准备状态。
- [ ] 冻结最小控制面闭环：创建租户、绑定域名、初始化租户管理员、激活、停用、重新启用。
- [ ] 冻结 Control Plane 与 Tenant App 的边界，阻断“跨租户控制能力从租户数据面泄漏”的设计漂移。
- [ ] 为 `350/370/380/390` 提供可直接消费的控制面输入，避免后续计划继续发明第二套租户治理语义。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 的 Tenancy / AuthN 主合同。
- [ ] 本计划不替代 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的角色与权限矩阵。
- [ ] 本计划不定义计费、套餐、合同、开票、客户成功等运营商务流程。
- [ ] 本计划不定义物理删库、租户数据清理、法务留存等下线策略；这些由后续专项计划承接。
- [ ] 本计划不引入“控制面 impersonation”“一键进入任意租户作为任意用户”之类便利路径。

## 3. “业务规则优先”在控制面中的翻译

### 3.1 用户真正管理的是“租户是否可被平台承载”，不是脚本或表

Superadmin 真正关心的不是：

- `tenants` 表里有没有插入一行；
- 是否手工写了域名映射；
- 是否通过 SQL 脚本塞了一个管理员账户；
- 某个 job runner 到底是同步还是异步。

用户真正关心的是：

- 这个租户是否已经被平台正式创建；
- 它能否从某个域名正常进入；
- 第一个租户管理员是否已经完成初始化；
- 它现在是可用、未完成开通，还是已被停用；
- 为什么这个租户还不能对业务用户开放访问。

### 3.2 Control Plane 拥有租户生命周期，不拥有租户业务数据

`343` 冻结：

- 控制面拥有“租户是否存在、是否可进入、是否已完成首个管理员初始化、是否被停用”的平台级真值；
- 控制面不拥有组织、人员、职位、任职等租户业务主数据；
- `superadmin` 拥有的是 **平台治理权**，不是“默认跨租户数据面全读写权”；
- 若控制面需要看租户业务状态，只能通过显式暴露的治理投影、审计或健康摘要，而不是直接把 tenant app 当后台数据库浏览器。

### 3.3 高风险跨租户操作必须显式、可审计、可回执

创建租户、绑定域名、初始化管理员、停用租户都不是“后台点一下”的轻量动作，而是平台级高风险控制动作。  
因此 `343` 冻结以下顺序：

1. 先有显式控制面业务意图；
2. 再决定同步执行还是走后台任务；
3. 最后通过审计、回执与通知对外说明结果。

### 3.4 “初始化管理员”是开通步骤，不是手工救火

Greenfield 中，首个租户管理员初始化是正式业务步骤，不允许退回到：

- 手工改库插用户；
- 直接复制 `superadmin` 到租户内；
- 在 tenant app 中临时开一个无边界的“初始化模式”。

## 4. 当前基线：已沉淀的业务规则

### 4.1 已稳定的 Greenfield 方向

#### 4.1.1 `300/340` 已冻结最小控制面闭环

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确 Superadmin 控制台属于平台与基座的一部分。
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 已明确第一阶段最小闭环至少包括：
  - 创建租户
  - 启停租户
  - 域名绑定
  - 初始化租户管理员

#### 4.1.2 `341` 已冻结租户停用与控制面/数据面分层

- 租户停用后，tenant app 不得继续接受新旧 session；
- control plane 仍可治理已停用租户；
- control plane 身份链路不得回流污染 tenant app 的运行态会话主链。

#### 4.1.3 `342` 已冻结控制面权限语义

- `platform.control` 是独立权限包；
- `superadmin` 默认拥有 `platform.control + platform.debug`，而不是租户业务数据面的天然全权；
- 控制面治理与 tenant app 内治理必须分层。

#### 4.1.4 `344` 已冻结控制面运行基座

- 控制面高风险动作必须进入 `PlatformAuditEvent`；
- 需要异步执行的控制面动作必须复用平台任务与通知基座；
- 控制面不应自行发明第二套任务系统、审计表或消息回执。

### 4.2 当前主要缺口

尽管上层方向已经清晰，但仍缺五类关键决定：

1. **还缺一份 Greenfield 语言的 Control Plane SSOT**  
   现在规则散在 `300/340/341/342/344` 中，尚无一份文档回答“平台控制面到底拥有哪类业务真值”。

2. **创建、绑定、初始化、激活、停用之间的边界尚未正式冻结**  
   很容易把这几步混成一个“创建租户成功”黑盒动作。

3. **租户生命周期状态机仍不明确**  
   目前只知道“可启停”，但还没有一份计划说明：创建后何时算真正可用、停用后哪些能力仍保留、哪些动作必须被阻断。

4. **控制面与租户数据面的可见性边界尚未正式冻结**  
   如果不先定义，`superadmin` 很容易在实现中重新变成“顺便读写所有租户业务数据”的万能身份。

5. **控制面 UI、审计与运行回执语言尚未形成统一输入**  
   后续 `350/380/390` 需要消费控制面能力，但当前仍没有一份计划定义控制面的列表、详情、设置、操作回执和失败表达。

## 5. Superadmin 控制台与租户生命周期的目标业务蓝图

### 5.1 领域使命

`343` 是平台内“**哪些租户被平台正式承载、它们是否已完成开通、何时允许 tenant app 接受访问、哪些跨租户治理动作可以在控制面执行**”的唯一业务权威。  
它不拥有登录态主事实源，不拥有细粒度权限矩阵，不拥有租户业务主数据，也不替代平台审计与后台任务系统。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由 `343` 拥有 |
| --- | --- | --- |
| `SuperadminOperator` | 在全局控制面内执行平台治理动作的可信主体 | 是 |
| `ManagedTenant` | 控制面视角的受管租户主档，回答“平台正在管理哪个租户” | 是 |
| `TenantLifecycleState` | 租户在控制面下的可用状态，如 `provisioning / active / suspended` | 是 |
| `TenantProvisioningRequest` | 一次显式租户开通意图，回答“平台要把哪个租户开起来” | 是 |
| `TenantProvisioningChecklist` | 激活前准备清单与 readiness 投影 | 是 |
| `TenantBootstrapAdmin` | 首个租户管理员初始化记录与结果 | 是 |
| `TenantDomain` | 运行态域名到租户的入口事实源 | 否，`341` 拥有主合同 |
| `AuthorizationDecision` | 是否允许当前控制面动作 | 否，`342` 拥有 |
| `PlatformAuditEvent` | 控制面高风险动作的审计事件 | 否，`344` 拥有 |
| `BackgroundJob / OperationReceipt` | 异步执行与用户可见回执 | 否，`344/323` 拥有共享合同 |

### 5.3 面向平台运营者的主能力

- 查看受管租户列表、状态与关键准备度
- 创建新租户并进入 `provisioning` 阶段
- 绑定或调整租户域名
- 初始化首个租户管理员
- 激活租户，使 tenant app 对业务用户可见
- 停用与重新启用租户
- 查询控制面动作的执行状态、审计与失败原因
- 保持控制面治理与 tenant app 数据面严格分层

### 5.4 Greenfield 选定的交付形态

#### 5.4.1 Control Plane 是独立的全局治理入口

- 控制面运行在 `global` 边界；
- 默认通过 `/admin/*` 入口承载；
- 需要 `platform.control` 才能执行治理动作；
- tenant app session 不得直接升级为控制面 session；
- 控制面不通过“进入租户后顺便管理”来表达治理能力。

#### 5.4.2 租户生命周期采用显式状态，而不是隐式布尔位

`343` 在第一阶段冻结三种正式状态：

- `provisioning`
  - 租户已在控制面注册，但尚未对 tenant app 正式开放；
- `active`
  - 租户已满足激活前条件，可接受 tenant app 登录与业务访问；
- `suspended`
  - 租户被平台显式停用，不再接受 tenant app 新旧 session，但控制面仍可治理。

额外冻结：

- 状态迁移必须显式触发并可审计；
- 不允许通过“域名删了所以默认不可用”“管理员没建完但先放开登录”之类隐式推断代替正式状态；
- 第一阶段不定义物理删除或不可恢复归档。

#### 5.4.3 租户开通采用显式步骤，而不是黑盒创建

Greenfield 的最小开通路径冻结为：

1. 创建 `ManagedTenant`
2. 绑定至少一个正式域名入口
3. 初始化首个租户管理员
4. 检查 `TenantProvisioningChecklist`
5. 激活租户

这些步骤可以同步或异步执行，但都必须满足：

- 由显式控制面动作触发；
- 可被平台审计；
- 失败时有明确回执；
- 不允许通过手工脚本补洞后再把平台状态“偷偷改成成功”。

## 6. `343` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 创建租户 | 在平台中登记一个新租户 | 创建后默认进入 `provisioning`；不得自动视为 `active`；租户标识必须稳定且可审计 | 平台获得一个受管租户主档 |
| 绑定域名 | 为租户建立正式入口 | 域名治理动作由控制面发起；域名必须全局唯一；不得配置 fallback 到未知租户 | 该租户获得唯一入口映射 |
| 初始化管理员 | 建立首个租户治理主体 | 必须是显式控制面动作；不得复制 `superadmin` 身份；结果应留审计与回执 | 租户内出现首个管理员入口 |
| 激活租户 | 让 tenant app 正式对业务用户开放 | 激活前必须满足 readiness 清单；未达标时 fail-closed；激活不是“顺便完成”的副作用 | 租户进入 `active` |
| 停用租户 | 暂停该租户的 tenant app 使用 | 停用后 tenant app 新旧 session 统一失效；控制面仍可进入治理 | 租户进入 `suspended` |
| 重新启用租户 | 恢复已停用租户的正常访问 | 只能从 `suspended` 显式恢复；恢复动作必须可审计；不得绕过 readiness 主规则 | 租户重新进入 `active` |
| 查看租户列表 | 快速判断租户当前可用性 | 列表应至少显示生命周期状态、域名绑定摘要、管理员初始化摘要、最近控制动作结果 | 控制面获得可操作的租户全景视图 |
| 查询动作进度 | 看清某次高风险动作跑到哪 | 用户可见进度应消费 `323/344` 的回执与任务基座，而不是临时拼接日志 | 控制动作状态可解释、可追踪 |
| 读取租户业务数据 | 从控制面查看 tenant app 内业务对象 | `superadmin` 默认不拥有租户业务数据面全读写权；若后续需要诊断投影，也必须是显式受控投影 | 控制面与数据面边界保持清晰 |

## 7. 共享合同、不变量与边界

### 7.1 Control Plane 边界合同

- 控制面只在 `global` 边界运行；
- 控制面动作默认需要 `platform.control`；
- tenant app session 不得直接用于控制面高风险动作；
- 控制面不允许提供“切换到某租户继续后台管理”的隐式旁路；
- 控制面页面与 API 是平台治理入口，不是租户业务后台的另一层皮。

### 7.2 租户生命周期合同

- `provisioning`：租户存在，但 tenant app 不得接受正式访问；
- `active`：租户可接受 tenant app 正常访问；
- `suspended`：tenant app 访问被阻断，但控制面治理仍保留；
- 生命周期迁移必须由显式控制动作触发；
- 不允许通过改域名、删管理员、任务失败等旁路隐式改变生命周期状态。

### 7.3 激活前准备合同

租户从 `provisioning` 进入 `active` 前，至少必须满足：

- `ManagedTenant` 主档已稳定创建；
- 至少存在一个有效域名绑定；
- 首个租户管理员初始化已成功，或存在等价且可恢复的正式管理员入口；
- 必要控制动作没有处于未知失败状态；
- `TenantProvisioningChecklist` 明确给出 readiness 通过结论。

并冻结以下规则：

- readiness 不完整时，tenant app 一律 fail-closed；
- 不允许通过“先放行业务访问，后补管理员/域名”的 easy 路径绕开 checklist；
- readiness 是控制面可见产品对象，不是后台实现细节。

### 7.4 域名与入口治理合同

- 运行态入口事实源仍由 `341` 的 `TenantDomain` 合同拥有；
- `343` 拥有“谁可以绑定、变更、解除绑定”的控制面治理语义；
- 域名必须全局唯一；
- 域名变更必须是显式控制面动作并可审计；
- 不允许出现“一个未知 host 自动猜租户”“多个租户共享同一正式入口却靠运行时猜测”。

### 7.5 首个管理员初始化合同

- 首个管理员初始化是正式控制面动作，不是脚本；
- 被初始化出来的是租户内主体，不是控制面超级身份的复制品；
- 临时密钥、激活链接或初始化凭据必须遵守 `333` 的 secret 治理要求；
- 审计中只能记录引用与结果，不得写出明文密钥；
- 若需要重新初始化，必须作为一条新的显式控制动作，而不是覆盖旧结果。

### 7.6 审计、回执、API 与 UI 交付合同

`343` 冻结以下最小交付面：

#### 7.6.1 API

- `GET /api/admin/tenants`
- `POST /api/admin/tenants`
- `GET /api/admin/tenants/{id}`
- `POST /api/admin/tenants/{id}/domains`
- `POST /api/admin/tenants/{id}:initialize-admin`
- `POST /api/admin/tenants/{id}:activate`
- `POST /api/admin/tenants/{id}:suspend`
- `POST /api/admin/tenants/{id}:reactivate`
- `GET /api/admin/tenants/{id}/operations`

#### 7.6.2 UI

- `/admin/tenants`
- `/admin/tenants/:id`
- `/admin/tenants/:id/setup`
- `/admin/tenants/:id/operations`

同时冻结：

- 创建、绑定、初始化、激活、停用、启用都必须留下平台审计；
- 需要异步执行的动作必须复用 `BackgroundJob + OperationReceipt`；
- UI 不得通过猜测日志文本拼接“当前进度”；
- 失败原因必须能区分：权限不足、readiness 未满足、域名冲突、初始化失败、租户已停用等不同类别。

### 7.7 实现护栏

- 不允许通过手工 SQL、临时脚本或环境变量旁路完成正式租户开通。
- 不允许把 `superadmin` 默认提升为租户业务数据面的全局万能角色。
- 不允许引入控制面 impersonation 作为第一阶段便利功能。
- 不允许在 tenant app 中长出第二套“租户启停/域名绑定/管理员初始化”入口。
- 不允许让 `343` 反向拥有登录态主规则、权限矩阵主规则或后台任务基础设施。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `350`（前端产品壳与交互系统）的输入

- [ ] 控制面需要独立的信息架构与导航入口，不能与 tenant app 共用同一套“业务壳即控制壳”心智。
- [ ] 租户列表、租户详情、开通步骤页、操作回执页应复用统一的列表/详情/状态徽标/时间线模式。
- [ ] UI 必须显式表达 `provisioning / active / suspended`，以及“缺管理员、缺域名、待激活、动作失败”等 readiness 差异。
- [ ] 不允许在控制面 UI 中提供隐式“切换进租户继续管理”的入口。

### 8.2 对 `360`（核心 HR 业务域）的输入

- [ ] 租户业务模块不拥有租户创建、启停、域名绑定、首个管理员初始化。
- [ ] 业务模块应把“租户是否可用”视为平台级既成事实，而不是自己发明第二套 tenant availability 逻辑。
- [ ] 业务模块不得因为 `superadmin` 身份就默认绕过租户边界访问业务主数据。

### 8.3 对 `370`（工作流、审计增强与集成）的输入

- [ ] 控制面动作默认不是 tenant app 内业务审批流的一部分；若未来需要双人复核，必须建立显式的 `TenantControlRequest` 审批语义，而不是复用业务审批动作名。
- [ ] `370` 可以叠加控制动作的增强审计与审批轨迹，但不得重写 `343` 的租户生命周期真值。

### 8.4 对 `380`（数据工作台与运营分析）的输入

- [ ] 控制面相关报表与运营看板应消费 `ManagedTenant`、生命周期、域名绑定摘要和开通进度投影，而不是直接下探 tenant app 主数据。
- [ ] 导出租户清单、域名清单、平台运维明细属于高风险动作，必须继续遵守 `342/330` 的权限与导出治理。

### 8.5 对 `390`（Chat Assistant）的输入

- [ ] Assistant 若触发控制面动作，必须显式消费 `platform.control` 与 `343` 的控制动作语义，不能借 tenant app 上下文越权发起全局治理。
- [ ] Assistant 不得把“控制面可治理租户”解释成“可直接读取该租户全部业务数据”。
- [ ] Assistant 的控制面确认摘要必须至少包含：目标租户、生命周期变化、域名变更、管理员初始化、是否触发异步任务、失败风险。

## 9. 建议实施分期

1. [ ] `M1`：控制面词汇与边界冻结  
   统一 `SuperadminOperator / ManagedTenant / TenantLifecycleState / TenantProvisioningChecklist` 语言，明确 control plane 与 tenant app 分层。
2. [ ] `M2`：生命周期与 readiness 冻结  
   冻结 `provisioning / active / suspended` 状态机，以及激活前准备清单。
3. [ ] `M3`：开通闭环冻结  
   冻结创建租户、绑定域名、初始化管理员、激活的正式步骤与失败语义。
4. [ ] `M4`：运行基座接线  
   让控制面动作显式接入 `344` 的审计、通知与后台任务，以及 `323` 的回执语言。
5. [ ] `M5`：首条垂直切片  
   形成“创建租户 -> 绑定域名 -> 初始化管理员 -> 激活 -> 通过正式入口登录 tenant app”的端到端 Slice 输入。

## 10. 建议目录与落点

若按 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的模块化单体落地，建议以 `Platform/*` 内聚 control plane ownership：

- `src/Platform/Tenancy/`
  - `ManagedTenant`
  - 生命周期状态迁移
  - 域名治理投影与 readiness 投影
- `src/Platform/IAM/`
  - `SuperadminOperator` 上下文
  - 首个租户管理员初始化
  - 控制面身份与租户内主体的桥接规则
- `src/Platform/Shell/`
  - `/admin/tenants*` 路由
  - 租户列表/详情/设置/操作回执 UI
- `src/Platform/Audit/`、`src/Platform/Jobs/`、`src/Platform/Notifications/`
  - 由 `344` 拥有
  - `343` 只消费，不复制实现

冻结原则：

- control plane 是一个跨 `Tenancy + IAM + Shell` 的产品切片；
- 平台运行基座继续由 `344` 拥有；
- 任何业务域都不得再长出第二个“租户生命周期中心”。

## 11. 验收标准

- [ ] `343` 已成为 Greenfield 平台控制面与租户生命周期的单一事实源，而不是继续散落在 `300/340/341/342/344` 的交叉引用中。
- [ ] 控制面与 tenant app 的边界清晰到可以独立评审：谁拥有平台治理、谁拥有租户数据面、谁拥有会话主链、谁拥有权限矩阵。
- [ ] 创建租户、绑定域名、初始化管理员、激活、停用、重新启用都已成为显式且可审计的正式动作。
- [ ] `superadmin` 不再被默认理解为租户业务数据面的跨租户全能角色。
- [ ] 生命周期状态与激活前 readiness 清单已经冻结，tenant app 不会在未完成开通时被提前放开。
- [ ] `350/370/380/390` 可以直接引用本计划，不再各自发明控制面语义或租户治理入口。

## 12. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-400](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/400-implementation-roadmap-and-vertical-slice-plan.md)
