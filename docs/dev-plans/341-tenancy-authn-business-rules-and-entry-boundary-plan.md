# DEV-PLAN-341：Tenancy / AuthN 业务规则优先蓝图与入口边界详细设计

**状态**: 规划中（2026-03-18 08:04 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的 `341` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“应用层强租户隔离 + 本地可撤销 session + OIDC 扩展位”的冻结；
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 对“租户隔离必须 fail-closed”的治理边界；
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 对“产品壳、登录页、租户/用户上下文展示”的前端交付语言；
- 现仓 [DEV-PLAN-019](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/019-tenant-and-authn.md)、[DEV-PLAN-021](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md)、[DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md) 已沉淀出来的 fail-closed、不默认租户、AuthN/AuthZ 分层经验。

但 `341` 不直接继承现仓的具体实现形态。  
尤其是：

- 不把 `Kratos` 写死成 Greenfield 第一阶段唯一实现；
- 不把数据库级 `RLS` 作为第一阶段交付阻塞项；
- 不把现仓的控制面/数据面历史包袱带入 `300` 蓝图。

`341` 的职责是：把“**租户是谁、请求属于哪个租户、用户以什么身份进入、为什么必须在平台入口就 fail-closed**”冻结成 Greenfield 语言，作为 `342/343/350/360/370/380/390` 的共享输入。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 Tenancy / AuthN，不让中间件、cookie、claims、provider SDK 这些实现细节喧宾夺主。
- [ ] 冻结租户解析、租户上下文、Principal、Session、登录/登出/续期、租户停用等核心业务对象与主规则。
- [ ] 冻结平台入口的 fail-closed 边界：未知租户、串租户 session、失效 session、停用租户、停用主体都必须在入口被显式拒绝。
- [ ] 冻结本地可撤销 session 作为运行态唯一会话事实源的合同，不允许同时长出第二套“直接信任外部 IdP token”的运行态主链。
- [ ] 冻结本地密码登录为 Phase 0/1 主链、标准 OIDC/OAuth2 为后续扩展位的分阶段口径。
- [ ] 为 `342`、`343`、`350` 以及后续所有业务模块提供稳定输入，避免每个模块各自解析租户、拼接登录态或自造跨租户兜底。

### 2.2 非目标

- [ ] 本计划不定义细粒度权限矩阵与 `role + permission` 策略细节；这些由后续 `342` 冻结。
- [ ] 本计划不展开 Superadmin 控制台的完整生命周期与运营流程；这些由后续 `343` 承接。
- [ ] 本计划不把数据库级 `RLS` 设为第一阶段默认前提；如未来需要更强隔离，作为后续强化项或 `333` 的输入评估。
- [ ] 本计划不引入 MFA、SCIM、目录同步、多 IdP 编排或复杂企业 SSO 编排。
- [ ] 本计划不允许通过“默认租户、自动切租户、邮箱全局查人、共享 session”之类 easy 路径换取短期便利。

## 3. “业务规则优先”在 Tenancy / AuthN 中的翻译

### 3.1 用户真正管理的是“进入哪个租户、以谁的身份进入”，不是鉴权中间件

平台用户关心的不是：

- 某个 cookie 叫不叫 `sid`；
- 是不是某个 IdP SDK 返回的 token；
- session 存在 Redis 还是数据库；
- 某个 controller 用了哪段 middleware。

用户真正关心的是：

- 我当前进入的是哪个租户；
- 为什么这个账号能进入这个租户；
- 为什么这个请求被系统认定为无效或越权；
- 为什么同一个人不能带着 A 租户的登录态进入 B 租户；
- 当租户被停用、账号被停用、会话被回收时，系统能否立刻拒绝访问。

### 3.2 Tenancy 先回答“你在谁的边界里”，AuthN 再回答“你是谁”

`341` 冻结以下顺序：

1. 先确定请求属于哪个租户边界；
2. 再判断这个请求携带的会话是否属于该租户；
3. 最后才把请求还原为一个可信的 `Principal`。

这意味着：

- 租户上下文不是登录后的附属字段，而是平台入口一级概念；
- “先全局找用户，再猜他属于哪个租户”属于禁止路径；
- “带着已有 session 自动切到另一个租户”属于禁止路径。

### 3.3 AuthN 解决“可信身份建立”，不解决“是否允许做事”

`341` 冻结：

- AuthN 只负责建立 `tenant + principal + session` 这三元可信来源；
- `342` 再决定这个主体能做什么；
- 业务模块不应自己解析 tenant，也不应自己判定登录态真假；
- 未来若引入数据库级强化隔离，它也只是加固租户边界，不替代 AuthN。

### 3.4 fail-closed 是产品契约，不是安全附加项

Tenancy / AuthN 的 fail-closed 需要回答的是：

- 未知域名是否应该直接拒绝；
- session 缺失、过期、被回收时是否立刻失效；
- session 所属租户与当前请求租户不一致时是否立刻拒绝；
- 停用租户、停用主体是否应该把旧会话一并视为无效；
- 登录页、壳、业务 API、Assistant 入口是否都遵守同一条边界。

这不是“实现时补一下”的安全细节，而是平台入口的主规则。

## 4. 当前基线：已沉淀的业务规则

### 4.1 已稳定的 Greenfield 方向

#### 4.1.1 `300` 已冻结主方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确：
  - 默认单库多租户；
  - 所有业务表显式带 `tenant_id`；
  - 应用层统一 tenant context 注入；
  - `tenant_id` 是不可变边界字段；
  - 跨租户访问必须在应用层入口显式拒绝；
  - Phase 0/1 先完成本地用户名密码登录 + 服务器端可撤销 session；
  - Phase 2+ 再补标准 OIDC/OAuth2 扩展。

#### 4.1.2 `340` 已把 Tenancy / AuthN 确认为平台前置

- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 已明确：
  - `341` 承接 `tenant context`、不可变边界与 fail-closed 入口；
  - `340` 是 `350/360/370/380/390` 的平台前置；
  - 后续业务模块默认依赖 tenancy 与 auth 基座，而不是各自重做。

### 4.2 现仓已经验证过、可被提炼的不变量

#### 4.2.1 未知租户必须拒绝，不能 fallback

- [DEV-PLAN-019](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/019-tenant-and-authn.md) 已验证：
  - `Host -> tenant domain` 可作为运行态租户解析入口；
  - 未知租户 fail-closed 比“默认租户/全局查人”更稳定；
  - 运行态 tenant 解析必须有单一事实源。

#### 4.2.2 本地 session 必须是运行态唯一事实源

- 现仓的关键经验不是 `Kratos` 本身，而是：
  - 外部身份提供方负责“认人”；
  - 应用侧本地 session 负责“运行态可撤销会话”；
  - 不能同时维护第二套并行运行态登录事实源。

#### 4.2.3 AuthN / AuthZ / Tenant Isolation 必须分层

- [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md) 已冻结：
  - AuthN 负责可信主体来源；
  - AuthZ 负责是否允许做事；
  - 不能把 tenant 边界、session 校验和权限判断混成一层。

### 4.3 当前主要缺口

尽管方向已经清晰，但仍有四类缺口：

1. **还缺一份 Greenfield 语言的 Tenancy / AuthN SSOT**  
   现在规则散在 `300/340/330/019/022` 中，后续计划仍可能各自翻译出不同版本。

2. **租户入口与会话入口的共享词汇尚未冻结**  
   大家都在说 tenant、principal、session，但还没有一份计划明确它们在 Greenfield 中分别回答什么业务问题。

3. **Phase 0/1 与 Phase 2+ 的认证边界尚未显式冻结**  
   虽然 `300` 已说本地登录优先、OIDC 后补，但没有详细说明扩展不能改变什么主合同。

4. **Tenant App 与 Superadmin 的边界尚未在 Tenancy/AuthN 层明确**  
   如果不先冻结，会很容易在 tenant 会话里长出跨租户控制能力。

## 5. Tenancy / AuthN 的目标业务蓝图

### 5.1 领域使命

Tenancy / AuthN 是平台内“**请求属于哪个租户、当前主体是谁、运行态会话是否可信、何时必须在入口拒绝访问**”的唯一业务权威。  
它不拥有细粒度权限矩阵，不拥有业务主数据，也不替代工作流或审计系统。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由 `341` 拥有 |
| --- | --- | --- |
| `Tenant` | 平台中的租户主体，定义隔离边界与可用状态 | 是 |
| `TenantDomain` | 将外部请求入口映射到租户的域名/主机名事实源 | 是 |
| `Principal` | 某租户内的登录主体 | 是 |
| `CredentialBinding` | Principal 与本地密码或外部 IdP 身份之间的绑定关系 | 是 |
| `LoginSession` | 运行态唯一可信会话票据，可撤销、可过期 | 是 |
| `TenantContext` | 由平台入口解析出的当前租户上下文 | 是 |
| `AuthIdentityProvider` | 外部身份提供方扩展位的抽象合同 | 是（扩展位） |
| `RoleAssignment` | 主体在运行态会话中的授权角色标识 | 否，`342` 拥有主规则 |
| `SuperadminOperator` | 控制面的跨租户操作主体 | 否，`343` 拥有主规则 |
| `AuditEvent` | 登录、登出、拒绝访问等审计记录 | 否，`344`/`370` 承接 |

### 5.3 面向用户的主能力

- 按租户入口进入系统
- 使用本地账号密码登录
- 获取当前租户与当前用户上下文
- 维持、续期与撤销登录会话
- 在租户被停用、账号被停用、会话失效时明确拒绝访问
- 为未来标准 OIDC/OAuth2 接入保留扩展位
- 让 Tenant App、Shell、业务 API、Assistant 共用同一条会话与租户边界

### 5.4 Greenfield 选定的交付形态

#### 5.4.1 Phase 0/1：本地登录 + 服务器端可撤销 session

这是默认主链，原因是：

- 它最直接支撑第一条产品切片“登录进入平台并看到组织列表”；
- 它不依赖外部 IdP 的接线复杂度；
- 它能最早冻结 session 回收、租户停用、入口拒绝这些平台规则。

#### 5.4.2 Phase 2+：标准 OIDC/OAuth2 扩展

这是正式扩展位，但必须遵守：

- 外部 IdP 负责“认人”，不直接成为运行态 session 主事实源；
- TenantContext 仍由平台入口决定；
- OIDC 接入不得改变“租户先于登录态”的入口顺序；
- OIDC 接入不得引入“同一会话跨租户漫游”。

## 6. `341` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 解析租户入口 | 回答“我正在进入哪个租户” | `TenantDomain` 是单一事实源；未知域名 fail-closed；不得配置 fallback | 请求一开始就落在唯一租户边界里 |
| 本地登录 | 在当前租户内建立可信身份 | 登录必须先有 TenantContext；认证成功后生成本地可撤销 session；不得直接把密码校验结果当长期运行态事实源 | 用户在当前租户下获得可信登录态 |
| OIDC 登录 | 用外部身份提供方辅助登录 | 外部 IdP 只负责认人；本地 session 仍是运行态唯一事实源 | 扩展不会破坏主会话合同 |
| 校验会话 | 回答“这个请求是否仍然可信” | session 必须绑定 `tenant + principal`；过期/撤销/停用一律失效 | 无效会话被统一拒绝 |
| 串租户访问 | 阻止 A 租户会话访问 B 租户 | `session.tenant_id` 与 `TenantContext` 不一致即 fail-closed；不得自动切租户 | 平台入口阻断跨租户访问 |
| 停用租户 | 回答“被停用后还能不能继续访问” | 停用租户不得继续建立或维持 tenant app 登录态；旧 session 应视为不可继续使用 | 租户停用即时生效 |
| 停用主体 | 回答“账号被禁用后还能不能继续访问” | Principal 被停用后，其现有 session 应统一失效 | 账号停用即时生效 |
| 读取当前上下文 | 回答“我是谁、我在哪个租户” | `/me` 或等价入口只能返回当前租户内的上下文，不得暴露跨租户切换语义 | 前后端共享一致上下文 |
| 进入控制面 | 使用跨租户运营能力 | Tenant App session 不得直接拥有控制面能力；控制面身份链路必须独立 | 高风险能力不从 tenant app 泄漏 |

## 7. 共享合同、不变量与边界

### 7.1 Tenant 入口合同

`341` 冻结：

- TenantContext 必须在平台入口解析完成；
- 租户解析必须有单一事实源；
- 未知租户、停用租户、禁用入口都必须 fail-closed；
- 平台不得提供“默认租户”“自动租户选择”“按邮箱全局猜租户”。

### 7.2 Principal 合同

- Principal 是“某租户内的登录主体”，不是全平台裸用户；
- Principal 的生命周期受租户边界约束；
- Principal 必须能被显式停用；
- AuthN 建立的是 `tenant + principal` 的可信绑定，而不是抽象全局用户身份。

### 7.3 Session 合同

- 本地可撤销 session 是运行态唯一事实源；
- session 必须至少绑定：
  - `tenant_id`
  - `principal_id`
  - `expires_at`
- session 缺失、失效、撤销、串租户、主体停用、租户停用都必须统一失效；
- 不允许在运行态同时信任两套会话来源。

### 7.4 AuthN / AuthZ 分层合同

- AuthN 只回答“你是谁、属于哪个租户、这张会话票据是否可信”；
- `342` 再回答“你被允许做什么”；
- 业务模块不得跳过平台入口直接自行判定登录态；
- `342` 也不得反向承担 tenant 解析或 session 修复职责。

### 7.5 Tenant App 与 Control Plane 分层合同

- Tenant App 只处理租户内访问；
- Control Plane 处理跨租户运营；
- 两者不得共用同一条运行态会话主链；
- 不允许在 tenant app 的 session 上叠加“顺便跨租户管理一下”的旁路能力。

### 7.6 扩展合同

- 本地密码登录是第一阶段主链；
- OIDC/OAuth2 是后续扩展位；
- 无论扩展到何种 IdP，都不得改变：
  - 租户先解析
  - 本地 session 唯一运行态事实源
  - fail-closed 的入口拒绝

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `342`（AuthZ 与平台权限矩阵）的输入

- [ ] `342` 必须消费 `341` 提供的 `tenant + principal + session` 可信来源，而不是自己重建登录态。
- [ ] `342` 的授权主体表达必须建立在 `341` 的 Principal/Session 合同之上。
- [ ] `342` 不得通过“未登录也视为某默认角色”来规避入口 fail-closed。

### 8.2 对 `343`（Superadmin 控制台与租户生命周期）的输入

- [ ] `343` 必须沿用 `341` 冻结的租户状态语义与停用生效规则。
- [ ] `343` 若引入控制面身份链路，也不得回流污染 tenant app 会话主链。
- [ ] `343` 负责控制面操作面，不改写 tenant app 的登录合同。

### 8.3 对 `344`（Audit / Notification / Background Jobs）的输入

- [ ] 登录、登出、拒绝访问、session 回收应成为标准审计事件。
- [ ] 通知与后台任务若代表某租户运行，必须显式声明租户上下文。
- [ ] `344` 可以记录 AuthN 事件，但不能重定义会话真值。

### 8.4 对 `350`（前端产品壳与交互系统）的输入

- [ ] 登录页、产品壳、用户菜单、租户上下文展示必须建立在同一 TenantContext 与当前用户上下文上。
- [ ] UI 必须能明确表达“未识别租户”“登录失效”“租户已停用”“无访问权限”这些不同失败原因。
- [ ] 前端不得私自缓存第二套“当前租户/当前用户”事实源。

### 8.5 对 `360`（核心 HR 业务域）的输入

- [ ] 业务模块只能消费平台注入的 TenantContext，不能自行解析租户。
- [ ] 业务模块不得把 `tenant_id` 作为可随意改写的业务字段。
- [ ] 任何业务 API 的跨租户拒绝都应尽量在平台入口完成，而不是落到领域层才发现。

### 8.6 对 `370/380/390` 的输入

- [ ] Workflow、Data Workbench、Assistant 都必须建立在同一 tenant/session 边界上。
- [ ] Assistant 不得绕开 `341` 的 tenant/session 主链直接持有跨租户能力。
- [ ] 导出、集成、工作台若以服务身份运行，必须显式声明租户上下文，而不是“系统默认全租户”。

## 9. 建议实施分期

1. [ ] `M1`：词汇与入口边界冻结  
   统一 `Tenant / TenantDomain / Principal / LoginSession / TenantContext` 语言，明确禁止 fallback 与自动切租户。
2. [ ] `M2`：本地登录与 session 合同冻结  
   冻结本地密码登录、session 生命周期、撤销语义与停用生效规则。
3. [ ] `M3`：控制面/数据面分层冻结  
   明确 tenant app 与 control plane 的身份与入口边界。
4. [ ] `M4`：OIDC 扩展位冻结  
   冻结未来 IdP 接入不允许改变的主合同。
5. [ ] `M5`：首条垂直切片接线  
   让“登录 -> 进入应用壳 -> 获取当前租户与当前用户上下文”成为可直接实现的 Slice 输入。

## 10. 验收标准

- [ ] `341` 已成为 Greenfield Tenancy / AuthN 的单一事实源，而不是继续分散在 `300/340/330/019/022` 的交叉引用中。
- [ ] 租户先解析、session 唯一、入口 fail-closed、AuthN/AuthZ 分层这四条主规则已经冻结。
- [ ] 本地登录与 OIDC 扩展的分阶段边界清晰，不会在 Phase 0/1 就被外部 IdP 复杂度反向阻塞。
- [ ] `342/343/350/360/370/380/390` 可以直接把 `341` 作为输入，而不再各自发明 tenant/session 口径。
- [ ] Tenant App 与 Control Plane 的边界已清晰到可以独立评审和实现。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-019](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/019-tenant-and-authn.md)
- [DEV-PLAN-021](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md)
- [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md)

## 12. 建议目录与落点

若按 `300` 的模块化单体落地，建议采用以下 ownership 落点：

- `src/Platform/Tenancy/`：`Tenant`、`TenantDomain`、`TenantContext`
- `src/Platform/IAM/`：`Principal`、`CredentialBinding`、`LoginSession`
- `src/Platform/Bff/` 或 `src/Web/AppShell/`：登录入口、`/me`、租户上下文装配与入口 fail-closed 适配层
- `src/Shared/Security/Identity/`：共享身份与会话合同、错误码与上下文类型

其中：

- `Platform/Tenancy` 拥有“请求属于哪个租户”的解析真值；
- `Platform/IAM` 拥有“当前主体是谁、会话是否有效”的运行态真值；
- `Shared/Security/Identity` 只承载共享合同，不拥有控制器或数据表。
