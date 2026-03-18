# DEV-PLAN-342：AuthZ 与平台权限矩阵业务规则优先蓝图

**状态**: 规划中（2026-03-18 11:36 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的 `342` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“`role + permission`、policy-based authorization、权限与审批分层”的冻结；
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 对“导出是高风险动作、Assistant 纳入治理、租户隔离必须 fail-closed”的治理边界；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对 `tenant + principal + session` 可信来源的冻结；
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 对“权限感知 UI”的产品交付要求；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 对能力命名、路由映射与颗粒度词汇的治理底座冻结；
- 现仓 [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md) 已沉淀出来的 `subject/object/action/domain`、policy SSOT 与门禁经验。

但 `342` 不是把 [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md) 改写一遍。  
`022` 更偏 **工具链与实现口径**，而 `342` 需要回答的是 Greenfield 平台层的问题：

- 平台里到底有哪些角色；
- 各角色默认承担什么业务责任；
- 哪些能力是高风险能力，不能被普通“admin”笼统吞掉；
- 权限差异如何在 UI、API、审批、导出、Assistant 中被一致表达。

`342` 的职责是把“**谁能在什么边界里做什么、哪些事必须显式分离、为什么用户在界面上会看到不同权限表达**”冻结成 Greenfield 平台的业务语言，作为 `343/345/350/360/370/380/390` 的共享输入。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 AuthZ，不让 `Casbin model.conf`、`policy.csv`、middleware 和 helper 命名喧宾夺主。
- [ ] 冻结 Greenfield 平台的角色语义、权限包（permission bundle）语义与初始角色矩阵。
- [ ] 冻结权限边界与高风险能力分层，至少明确区分：读取、日常维护、历史更正、审批、导出、租户治理、控制面治理、调试诊断。
- [ ] 冻结 tenant app 与 control plane 的授权边界，避免 `superadmin` 被误解为“默认拥有所有租户数据面的万能权限”。
- [ ] 冻结前端与 API 的权限感知表达：隐藏、只读、禁用、403 拒绝各自代表什么业务语义。
- [ ] 为 `343/345/350/360/370/380/390` 提供统一的权限输入，避免各模块继续发明自己的角色名、动作名和旁路授权规则。
- [ ] 与 `347` 对齐能力命名与颗粒度语义，避免授权矩阵再长出第二套 capability 与路由映射词汇。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md) 的 Casbin 工具链、pack/lint/test、生成物与 CI 门禁。
- [ ] 本计划不直接定义最终 `policy.csv` 内容、脚本实现或 ASP.NET Core 中间件代码；这些由后续实现与 `022` 工具链承接。
- [ ] 本计划不重写 `341` 的 tenant/session 主链；所有授权都默认建立在 `341` 已冻结的可信身份来源之上。
- [ ] 本计划不把审批、导出治理、Assistant 治理直接并入权限系统；它们仍由 `370/380/390` 拥有各自主规则，但必须消费本计划的权限边界。
- [ ] 本计划不允许用“一个万能 admin 动作”掩盖审批、历史改写、导出、跨租户控制等本应分离的高风险能力。

## 3. “业务规则优先”在 AuthZ 中的翻译

### 3.1 权限真正回答的是“谁能在什么边界里做什么”

平台用户关心的不是：

- 某条 Casbin policy 长什么样；
- `subject` 是 `role:xxx` 还是别的字符串；
- 某个 endpoint 映射到了哪一行 CSV；
- 某个按钮是不是前端自己 `if` 掉了。

用户真正关心的是：

- 我在这个租户里到底能看什么、改什么、批准什么；
- 为什么我能发起一件事，却不能审批它；
- 为什么我能看见数据，却不能导出；
- 为什么我能进入租户后台，却不能创建或停用别的租户；
- 为什么 Assistant 只能建议，不能替我直接提交。

### 3.2 授权不是租户隔离，不是审批，也不是 UI 装饰

`342` 冻结三条分层原则：

- 租户隔离回答“你在谁的边界里”，由 `341` 先解决；
- 授权回答“你在这个边界里能做什么”；
- 审批回答“这件高风险业务动作是否经过额外治理”。

因此：

- “已登录”不等于“已授权”；
- “有编辑权限”不等于“能改历史生效记录”；
- “有发起权限”不等于“有审批权限”；
- “能读取”不等于“能导出”；
- “Assistant 能建议”不等于“Assistant 能执行”。

### 3.3 角色表达默认职责，权限包表达真实边界

角色是平台对“默认职责分工”的表达，例如：

- `tenant_viewer`
- `tenant_hr`
- `tenant_admin`
- `superadmin`

但真正决定放行与拒绝的，是这些角色背后的 **权限包组合**，而不是角色名字本身。  
否则角色会迅速膨胀成一堆难以解释的特例。

### 3.4 权限差异必须用户可感知

权限不是后端拒绝之后才被发现的惊喜。

在产品层，用户应该能感知：

- 自己是否能看到某模块；
- 是否只能查看、不能编辑；
- 是否能编辑当前态、但不能改历史；
- 是否能发起、但不能审批；
- 是否能查看、但不能导出；
- 是否因为跨租户或控制面边界而根本不应看到入口。

## 4. 当前基线：已沉淀的业务规则

### 4.1 已稳定的 Greenfield 方向

#### 4.1.1 `300` 已冻结权限主方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确：
  - 授权采用 `role + permission`；
  - 使用 policy-based authorization；
  - 权限矩阵应当可存储、可治理；
  - “有权限发起”不等于“有权限审批”；
  - “有权限编辑”不等于“能修改历史生效记录”；
  - “Assistant 能建议”不等于“Assistant 能提交”。

#### 4.1.2 `340` 已冻结初始角色清单

- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 已列出：
  - `superadmin`
  - `tenant_admin`
  - `tenant_hr`
  - `tenant_viewer`

这说明 Greenfield 已经承认平台至少存在“控制面管理员、租户治理管理员、业务 HR 操作员、只读用户”四类职责，不应再退回只有两三个泛化角色的表达。

#### 4.1.3 `341` 已冻结可信身份来源

- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 已冻结：
  - TenantContext 先于授权；
  - 本地可撤销 session 是运行态唯一事实源；
  - AuthN 不替代 AuthZ；
  - tenant app 与 control plane 必须分层。

#### 4.1.4 `022` 已验证工具链与实现语汇

- [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md) 已沉淀：
  - `subject/object/action/domain` 的实现框架；
  - `module.resource` 作为资源命名；
  - Git 管理 policy + pack/lint/test 门禁；
  - 匿名白名单、统一 403、`domain=tenant/global` 等经验。

### 4.2 当前主要缺口

尽管方向已较清晰，但仍缺五类关键决定：

1. **还缺 Greenfield 平台自己的角色语义定义**  
   `340` 列出了角色名，但没有回答这些角色在业务上分别承担什么职责。

2. **高风险能力还没有被正式拆出**  
   审批、历史改写、导出、平台治理、控制面治理仍容易被粗暴塞进一个 `admin` 动作里。

3. **`tenant_hr` 尚未被纳入现有最小工具链角色矩阵**  
   现有 `022` 的最小闭环更偏 `tenant_admin / tenant_viewer / superadmin`，尚缺对 `tenant_hr` 的 Greenfield 业务定位。

4. **Control Plane 与 Tenant App 的授权边界还缺正式定义**  
   如果不冻结，`superadmin` 很容易被误用为“默认读取所有租户业务数据”的超级角色。

5. **权限感知 UI 仍缺共享表达**  
   `350` 已说要做 permission-aware UI，但还没有一份计划明确隐藏、只读、禁用、403 各自代表什么。

## 5. AuthZ 的目标业务蓝图

### 5.1 领域使命

AuthZ 是平台内“**在既定租户边界和可信身份前提下，谁能查看、维护、治理、审批、导出或诊断哪些资源**”的唯一业务权威。  
它不拥有租户解析、不拥有登录态主事实源，也不替代审批流和审计系统。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由 `342` 拥有 |
| --- | --- | --- |
| `RoleProfile` | 平台对一类默认职责的抽象，如 `tenant_hr`、`tenant_admin` | 是 |
| `PermissionBundle` | 一组稳定、可解释的权限语义包 | 是 |
| `ResourceCapability` | 某类资源需要保护的稳定能力边界 | 是 |
| `AuthorizationRequest` | 基于当前身份、边界、资源与动作意图的授权请求 | 是 |
| `AuthorizationDecision` | allow / deny 及其原因分类 | 是 |
| `PermissionMatrix` | 角色、权限包与资源之间的映射矩阵 | 是 |
| `DomainBoundary` | `tenant` 或 `global` 控制面边界 | 是 |
| `RoleAssignment` | 某主体在某边界内被授予了哪类角色 | 是 |
| `WorkflowApprovalState` | 某请求是否已经通过审批 | 否，`370` 拥有 |
| `ExportPolicy` | 导出内容与风险治理规则 | 否，`330/380` 拥有 |
| `AssistantActionRequest` | Assistant 的动作请求与确认状态 | 否，`390` 拥有 |

### 5.3 平台级权限包

`342` 冻结以下共享权限包语义：

- `access.public`
  - 匿名或未登录用户可访问的必要入口，如登录页、健康检查页
- `tenant.read`
  - 读取租户内业务对象与页面
- `tenant.maintain`
  - 维护租户内当前态业务对象
- `tenant.history-maintain`
  - 修改 effective-dated 历史、更正、插入历史版本等高风险历史动作
- `tenant.approve`
  - 审批工作流请求，而不是仅仅发起请求
- `tenant.export`
  - 导出、下载高风险数据集
- `tenant.govern`
  - 租户内平台治理，如配置、策略、用户/角色治理、租户级设置
- `platform.control`
  - 控制面跨租户治理，如创建/停用租户、域名绑定、初始化租户管理员
- `platform.debug`
  - 受控诊断与调试能力

### 5.4 初始角色矩阵

`342` 冻结初始角色的默认职责如下：

| 角色 | 默认权限包 | 核心业务含义 |
| --- | --- | --- |
| `anonymous` | `access.public` | 仅可访问必要公开入口 |
| `tenant_viewer` | `tenant.read` | 租户内只读查看者 |
| `tenant_hr` | `tenant.read + tenant.maintain` | 租户内日常 HR 业务操作员 |
| `tenant_admin` | `tenant.read + tenant.maintain + tenant.govern` | 租户内平台治理管理员 |
| `superadmin` | `platform.control + platform.debug` | 平台控制面管理员 |

额外冻结：

- `tenant.approve` 不是任何基础角色的隐含能力，必须显式纳入矩阵；
- `tenant.export` 不是 `tenant.read` 的自然扩展，必须显式纳入矩阵；
- `tenant.history-maintain` 不是 `tenant.maintain` 的自然扩展，必须显式纳入矩阵；
- `superadmin` 默认不是租户业务数据面的全能角色；
- 若未来需要更细粒度角色，应在本矩阵之上扩展，而不是推翻其语义。

## 6. `342` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 匿名访问登录入口 | 进入系统并完成认证 | `access.public` 仅允许必要公开入口；不得因为“还没登录”就隐式放大可访问面 | 公开入口最小化且可解释 |
| 只读查看 | 看列表、详情、历史 | `tenant.read` 只回答可见性，不自动包含修改、导出、审批 | 只读权限不会被误当成操作权限 |
| 日常业务维护 | 维护当前态业务对象 | `tenant.maintain` 只覆盖当前态维护；历史改写必须单独授权 | 普通操作不自动升级为高风险历史写 |
| 历史更正 | 修改 effective-dated 历史 | `tenant.history-maintain` 必须独立存在；不能被普通编辑吞掉 | 历史改写被明确治理 |
| 审批工作流 | 批准或驳回请求 | `tenant.approve` 与 `tenant.maintain` 分离；有发起权不等于有审批权 | 审批链路与业务编辑链路不混用 |
| 导出数据 | 下载高风险数据 | `tenant.export` 与 `tenant.read` 分离；导出还需遵守 `330/380` 的治理要求 | 查看和下载被清晰区分 |
| 租户内治理 | 管理配置、策略、角色与租户级设置 | `tenant.govern` 与业务 HR 操作分离；不等于控制面能力 | 租户治理与业务操作边界清楚 |
| 控制面运营 | 创建/停用租户、绑定域名 | `platform.control` 仅属于 control plane；不自动拥有租户数据面能力 | 高风险跨租户能力被收口 |
| 调试诊断 | 读取诊断与调试入口 | `platform.debug` 必须受控，不得默认暴露给普通角色 | 诊断能力不外溢 |
| Assistant 代办 | 通过 Assistant 建议或执行动作 | Assistant 必须消费与 UI/API 相同的权限矩阵；建议不等于执行 | 对话入口不会绕过权限 |

## 7. 共享合同、不变量与边界

### 7.1 边界合同

- 所有 AuthZ 都建立在 `341` 已解析好的 TenantContext 之上；
- `tenant` 与 `global` 是两类不同授权边界；
- tenant app 只在 `tenant` 边界内运行；
- control plane 只在 `global` 边界内运行；
- 不允许把 hostname、路由层级或页面位置当成授权边界本身。

### 7.2 角色合同

- 角色是默认职责表达，不是临时拼装字符串；
- 角色名必须稳定、可解释、可在 UI 与审计中复述；
- 角色不能直接替代权限包；
- 新增角色必须回答：它比现有角色新增了什么责任，而不是“实现方便就新建一个”。

### 7.3 权限包合同

- 权限包是稳定的业务语义包；
- 读取、维护、历史更正、审批、导出、治理、控制面、调试必须能被清晰区分；
- 若某能力在产品上需要单独治理，它就不能被 generic `admin` 动作继续吞并；
- 高风险能力默认独立，不作自然继承。

### 7.4 资源合同

- 受保护资源使用稳定 `module.resource` 语言；
- 资源名回答“保护什么业务对象/能力”，而不是“保护哪个路由片段”；
- 页面、API、后台任务、Assistant 工具若指向同一业务能力，应尽量映射到同一资源语义，而不是各自长一套名字。

### 7.5 决策合同

- 统一的授权决策至少回答：
  - 当前边界是 `tenant` 还是 `global`
  - 当前主体的角色/权限包是什么
  - 命中的资源能力是什么
  - 为什么 allow 或 deny
- 403 只是结果表现，不是唯一解释；
- 权限缺失、边界错误、未登录、租户停用等原因不能在产品层混成同一类提示。

### 7.6 UI 合同

`342` 冻结四类权限感知表达：

- **隐藏**：当前角色根本不应感知该入口存在
- **只读**：可查看但不可修改
- **禁用**：用户能理解该动作存在，但当前缺权限或缺治理前提
- **403 拒绝**：直接 URL/API 命中未授权资源时的最终拒绝

冻结原则：

- 不允许“前端只隐藏、后端不校验”；
- 也不允许“所有按钮都显示，点击后全靠 403 教育用户”；
- 同一能力在列表、详情、表单、Assistant 确认页上的权限表达应一致。

### 7.7 Assistant 合同

- Assistant 只能消费与 UI/API 同源的授权决策；
- Assistant 有建议资格，不等于拥有执行资格；
- 需要审批、导出、历史更正、平台治理等高风险动作时，Assistant 必须经过同一权限边界与治理边界。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `343`（Superadmin 控制台与租户生命周期）的输入

- [ ] `343` 必须以 `platform.control` 为控制面主权限包，而不是把 `superadmin` 当成对所有租户业务数据天然放行。
- [ ] `343` 需要清楚区分控制面治理与 tenant app 内治理。

### 8.2 对 `344`（Audit / Notification / Background Jobs）的输入

- [ ] 授权决策、拒绝原因、角色与边界信息应成为可审计事件。
- [ ] 后台任务若代表某边界执行，也必须带上相应的授权上下文，而不是“系统默认全权”。

### 8.3 对 `345`（Platform Configuration / Policy）的输入

- [ ] 配置、策略、发布、激活、回滚等平台治理动作应默认消费 `tenant.govern`，而不是 `tenant_hr`。
- [ ] Explain/预览可见性与修改/发布权限必须分层。

### 8.4 对 `350`（前端产品壳与交互系统）的输入

- [ ] UI 必须能表达隐藏/只读/禁用/403 四类差异。
- [ ] 导航、菜单、操作按钮、详情页编辑态必须建立在同一权限矩阵之上。
- [ ] 不能靠路由守卫单独承担全部权限表达。

### 8.5 对 `360`（核心 HR 业务域）的输入

- [ ] 每个业务模块都必须声明：
  - 哪些资源需要 `tenant.read`
  - 哪些需要 `tenant.maintain`
  - 哪些需要 `tenant.history-maintain`
- [ ] 业务模块不得重新定义“审批权限”“导出权限”“治理权限”。

### 8.6 对 `370`（工作流、审计增强与集成）的输入

- [ ] 发起审批与审批通过必须消费不同权限包。
- [ ] 审批状态不能被泛化为“有 admin 就都能过”。

### 8.7 对 `380`（数据工作台与运营分析）的输入

- [ ] 导出、下载、批量查询与工作台诊断必须显式决定是否需要 `tenant.export` 或更高治理权限。
- [ ] 读取报表不等于导出原始数据。

### 8.8 对 `390`（Chat Assistant）的输入

- [ ] Assistant 的只读检索至少需要 `tenant.read`；
- [ ] Assistant 发起维护动作需要匹配 `tenant.maintain`；
- [ ] Assistant 不得因为“对话更自然”而绕过 `tenant.history-maintain / tenant.approve / tenant.export / tenant.govern` 的高风险边界。

## 9. 建议实施分期

1. [ ] `M1`：角色语义与权限包冻结  
   明确 `anonymous / tenant_viewer / tenant_hr / tenant_admin / superadmin` 与各权限包的业务含义。
2. [ ] `M2`：资源能力矩阵冻结  
   把 `module.resource` 与权限包的映射抽成平台级矩阵，停止各模块自行拼权限名。
3. [ ] `M3`：高风险能力分层冻结  
   正式把 `history-maintain / approve / export / govern / control` 从 generic admin 中剥离。
4. [ ] `M4`：UI 与 API 权限表达冻结  
   冻结隐藏/只读/禁用/403 的产品口径，并让 `350` 直接消费。
5. [ ] `M5`：工具链投影接线  
   让 `022` 的实现与门禁开始消费 `342 + 347` 的角色语义、能力命名与映射约束，而不是继续停留在最小实验状态。

## 10. 验收标准

- [ ] `342` 已成为 Greenfield 平台权限矩阵的单一事实源，而不是继续分散在 `300/340/341/022/350` 中。
- [ ] 初始角色、权限包与高风险能力分层已经冻结，`tenant_hr` 的业务语义不再空缺。
- [ ] `superadmin`、`tenant_admin`、`tenant_hr`、`tenant_viewer` 的边界足够清晰，可直接用于产品评审和实现评审。
- [ ] 审批、导出、历史更正、平台治理、控制面治理已被正式从 generic `admin` 中分离出来。
- [ ] `343/345/350/360/370/380/390` 可以直接引用 `342`，不再各自发明角色名和旁路权限。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md)
