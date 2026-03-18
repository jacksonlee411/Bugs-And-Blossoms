# DEV-PLAN-351：Product Shell 与路由信息架构详细设计

**状态**: 规划中（2026-03-18 14:21 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 的 `M1: Product Shell` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“React SPA + 统一应用壳 + 列表/详情/历史页面模式 + Assistant 结果回落到明确业务 UI”的冻结；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对“登录页、产品壳、租户/用户上下文展示必须建立在同一 TenantContext 与当前用户上下文上”的冻结；
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 对“导航、菜单、操作入口必须建立在同一权限矩阵上，且不能靠路由守卫单独承担全部权限表达”的冻结；
- [DEV-PLAN-343](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/343-superadmin-console-and-tenant-lifecycle-business-rules-plan.md) 对“控制面必须拥有独立信息架构与导航入口，且不允许在 UI 中提供隐式切进 tenant app 的入口”的冻结；
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md) 对“审计、通知、任务中心应有统一可发现入口”的冻结；
- [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md) 对“壳层必须为 `PageHeader + ContextBar + 主内容区` 预留稳定位置，而页面模式负责‘到了以后怎么读’”的冻结。

`350` 已明确前端需要统一的信息架构、导航和 Product Shell，但当前仍缺一份真正拥有“**系统到底分几个壳、路由分组如何命名、公共入口 / tenant app / control plane 如何分层、导航如何与权限和上下文对齐**”的文档。  
如果没有 `351`，后续计划很容易继续各自发明：

- `/login`、`/app`、`/admin` 到底是不是同一套壳；
- 模块导航到底按业务域分组，还是按实现模块、表单、页面碎片分组；
- 控制面和 tenant app 是否可以共用同一条会话与侧栏；
- 登录失效、租户停用、403、404、未知租户到底应该落到哪一层页面；
- 页面模式的页头/上下文栏由谁拥有、由谁保留位置。

`351` 的职责就是把这些问题收敛为 **Greenfield HR 平台的 Product Shell 与路由信息架构 SSOT**，供后续子计划直接消费。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 Product Shell 与路由信息架构，不让 sidebar、layout 组件名或临时路由碎片喧宾夺主。
- [ ] 冻结三层应用边界：`Public Entry / Tenant App / Control Plane`。
- [ ] 冻结路由分组、导航模型、默认落点、面包屑与上下文承载面的共享语言。
- [ ] 明确壳层与页面模式的边界：壳层回答“我在哪、能去哪里、当前是谁”，页面模式回答“到了之后怎么读对象”。
- [ ] 为 `352/353/360/370/380/390` 提供稳定 IA 输入，阻断模块自造第二套侧栏、顶部导航或模块首页。

### 2.2 非目标

- [ ] 本计划不承接列表/详情/历史页面骨架；这些由 [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md) 拥有。
- [ ] 本计划不承接表单布局、确认弹层、字段级交互、按钮权限细则；这些由后续 `353` 承接。
- [ ] 本计划不替代 [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 的租户/会话主合同，也不替代 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的权限矩阵。
- [ ] 本计划不定义具体 React 组件实现、代码目录或最终视觉稿。
- [ ] 本计划不支持“一个壳里顺手切换所有边界”“控制面 impersonation”“隐式跨租户跳转”之类便利路径。

## 3. “业务规则优先”在 Product Shell 中的翻译

### 3.1 壳层首先回答“我在哪个边界里”，不是“我看到哪个侧栏”

用户进入系统后最先需要知道的不是：

- 当前是不是 `AppShell.tsx`；
- 左侧有几个菜单组；
- 顶栏用了哪些组件。

用户真正需要知道的是：

- 我现在是在公共入口、租户业务应用，还是平台控制面；
- 我当前属于哪个租户、以谁的身份进入；
- 我在这个边界里能去哪些模块；
- 为什么某些入口看不到、只读或直接被拒绝。

因此 `351` 冻结：**边界先于布局，布局先于页面内容。**

### 3.2 导航回答“去哪里”，页面模式回答“到了以后怎么看”

`351` 与 `352` 的边界必须明确：

- `351` 拥有：
  - 路由树
  - 导航分组
  - 默认落点
  - 面包屑与当前模块定位
  - 当前租户/当前用户/当前边界的壳层上下文
- `352` 拥有：
  - `PageHeader`
  - `ContextBar`
  - 列表/详情/历史/工作台骨架

也就是说：

1. `351` 负责把人送到正确空间；
2. `352` 负责定义到了以后怎么读这个空间里的对象。

### 3.3 路由守卫不是全部权限表达，导航也不是权限判断本身

`351` 冻结：

- 访问控制的真值来自 `341 + 342`；
- 路由守卫负责阻断错误边界和明显不可达入口；
- 导航负责表达“当前用户通常能去哪里”；
- 最终 403 仍必须存在，但不能把所有权限体验都退化成“点了才知道不行”。

### 3.4 Control Plane 与 Tenant App 必须是两套壳，而不是一个侧栏开关

Greenfield 中：

- `Tenant App` 是租户内业务操作空间；
- `Control Plane` 是跨租户平台治理空间；
- 两者可以共享设计语言，但不能共用同一条运行态壳语义；
- 不允许通过某个“租户切换器”把 tenant app 直接变成 control plane。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

#### 4.1.1 `300/350` 已冻结“壳层先行”的实施顺序

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确 `340/350` 是 `360/370/380/390` 的平台前置；
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 已明确 Product Shell、Routing & Navigation 是独立范围，不应由业务模块分头补齐。

#### 4.1.2 `341` 已冻结入口与上下文主事实源

- 登录页、产品壳、用户菜单、租户上下文展示必须建立在同一 `TenantContext + CurrentUser` 上；
- 未识别租户、登录失效、租户停用、无访问权限必须是可区分失败态；
- 前端不得私自缓存第二套“当前租户/当前用户”事实源。

#### 4.1.3 `342` 已冻结权限感知导航原则

- 导航、菜单、操作按钮、详情页编辑态必须建立在同一权限矩阵之上；
- UI 需要稳定表达 `hidden / read_only / disabled / 403` 四类差异；
- 不能靠路由守卫单独承担全部权限表达。

#### 4.1.4 `343` 已冻结控制面 IA 输入

- 控制面需要独立信息架构与导航入口；
- 租户列表、详情、开通步骤、操作回执页应复用统一页面模式；
- 不允许在控制面 UI 中提供隐式“切进 tenant app 继续管理”的入口。

#### 4.1.5 `352` 已冻结壳层与页面骨架的交接面

- 壳层必须为 `PageHeader + ContextBar + 主内容区` 预留稳定位置；
- 模块导航负责“去哪里”，页面模式负责“到了以后怎么读”；
- 工作台页也必须建立在同一壳层与基础页面对象之上。

### 4.2 当前主要缺口

1. [ ] **还缺一份 Greenfield Shell SSOT**  
   现在大家知道要做 `Product Shell`，但还没有一份文档正式回答“系统到底有几套壳、它们分别拥有什么边界”。

2. [ ] **路由树仍可能按实现碎片而非业务空间生长**  
   如果不先冻结，后续很容易继续长出按组件、页面、临时功能命名的路由结构。

3. [ ] **控制面与 tenant app 仍有壳层混写风险**  
   `343` 已经冻结边界，但 `350` 体系内还没有把它翻译成实际信息架构语言。

4. [ ] **默认落点与失败态落点尚未统一**  
   未登录、未知租户、租户停用、403、404、空模块首页都还缺共同落点约定。

5. [ ] **通知、审计、任务、Assistant 入口仍可能各自游离**  
   若没有 `351`，这些横切能力很容易变成“哪里需要就塞到哪里”的散入口。

## 5. Product Shell 与路由信息架构目标蓝图

### 5.1 领域使命

`351` 是 Greenfield HR 平台内“**用户当前处于哪个应用边界、可以去哪些业务空间、当前租户与当前用户上下文如何被稳定承载，以及不同页面如何挂接到统一路由信息架构上**”的唯一壳层权威。  
它不拥有业务对象页面骨架，不拥有表单交互细节，也不拥有认证和授权的主真值。

### 5.2 核心壳层对象

| 壳层对象 | 业务含义 | 是否由 `351` 拥有 |
| --- | --- | --- |
| `AppBoundary` | 当前用户处于 `public`、`tenant_app` 或 `control_plane` 哪个应用边界 | 是 |
| `RouteGroup` | 一组共享边界、导航归属和落点语义的路由分组 | 是 |
| `NavigationNode` | 用户可见的一级/二级导航节点 | 是 |
| `DefaultLanding` | 某边界或某角色进入后的默认落点 | 是 |
| `ShellContextSurface` | 壳层稳定展示的当前租户、当前用户、当前边界、当前模块信息 | 是 |
| `RouteGuardSurface` | 路由层对未登录、无权限、错误边界、停用租户等结果的 UI 投影 | 是 |
| `BreadcrumbModel` | 模块内当前位置的稳定定位语言 | 是 |
| `WorkspaceSlot` | 为 `PageHeader / ContextBar / MainContent / Side Utilities` 预留的壳层槽位 | 是 |
| `PageHeader / ContextBar` | 页面头部与上下文条的内容骨架 | 否，`352` 拥有 |
| `AuthorizationDecision` | 当前主体是否被允许访问 | 否，`341/342` 拥有 |

### 5.3 面向用户的主能力

- 进入公共入口并完成登录
- 进入 tenant app 并看到与角色匹配的模块导航
- 进入 control plane 并看到平台治理专属导航
- 在任一边界内保持稳定的当前租户/当前用户/当前模块感知
- 在导航、直接 URL、失败态之间获得一致的壳层反馈
- 让通知、审计、任务、Assistant 等横切入口有可发现的固定位置
- 在桌面和移动端保持相同的信息架构，只改变布局密度

### 5.4 Greenfield 选定的壳层形态

#### 5.4.1 三层边界模型：`public / tenant_app / control_plane`

`351` 第一阶段冻结三套正式壳：

- `public`
  - 承载登录、未知租户、会话失效、租户停用等公共入口与前置失败页；
- `tenant_app`
  - 承载租户内业务空间与租户级治理空间；
- `control_plane`
  - 承载跨租户平台治理空间。

冻结原则：

- 一个路由只能属于一个 `AppBoundary`；
- 不允许同一条业务路由既被当成 tenant app，又被当成 control plane；
- 壳层切换必须是显式边界切换，不是菜单折叠逻辑。

#### 5.4.2 顶层路径收敛：`/login`、`/app/*`、`/admin/*`

第一阶段冻结以下顶层路径归属：

- `public`
  - `/login`
  - `/unknown-tenant`
  - `/session-expired`
  - `/tenant-suspended`
- `tenant_app`
  - `/app`
  - `/app/org/*`
  - `/app/person/*`
  - `/app/job-catalog/*`
  - `/app/staffing/*`
  - `/app/workflow/*`
  - `/app/data/*`
  - `/app/assistant/*`
  - `/app/platform/*`
  - `/app/notifications`
- `control_plane`
  - `/admin`
  - `/admin/tenants/*`
  - `/admin/audit`
  - `/admin/jobs`

额外冻结：

- `tenant_app` 与 `control_plane` 不共享顶层前缀；
- 控制面不走 `/app/admin/*` 这类混合表达；
- 公共入口不挤进 `/app/*` 内部做条件渲染。

#### 5.4.3 路由分组按业务空间命名，而不是按实现碎片命名

`351` 冻结首批 `RouteGroup` 语言：

- `entry.auth`
- `entry.errors`
- `tenant.home`
- `tenant.org`
- `tenant.person`
- `tenant.job-catalog`
- `tenant.staffing`
- `tenant.workflow`
- `tenant.data`
- `tenant.assistant`
- `tenant.platform`
- `tenant.notifications`
- `control.home`
- `control.tenants`
- `control.audit`
- `control.jobs`

冻结原则：

- `RouteGroup` 回答“用户正在进入哪个业务空间”；
- 不允许出现 `tenant.page1`、`shell-v2`、`org-detail-new` 这类实现味命名；
- 页面级别可继续细化，但不得推翻顶层业务空间分组。

#### 5.4.4 默认落点必须显式

`351` 冻结：

- 匿名或未登录访问系统时，默认落到 `/login`；
- 登录成功后，tenant app 默认落到 `/app` 的角色感知首页；
- control plane 主体默认落到 `/admin` 的平台概览或租户列表；
- 没有模块权限时，不自动进入第一个可访问 URL 之外的隐藏页面；
- 默认落点必须可解释，而不是依赖前端遍历菜单“碰巧找到第一个”。

## 6. `351` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心壳层规则 | 业务结果 |
| --- | --- | --- | --- |
| 打开系统入口 | 知道自己从哪里开始进入 | 公共入口必须使用 `public` 壳；未知租户与登录入口不能混进 `/app/*` | 用户先进入正确边界 |
| 登录成功进入 tenant app | 看到租户内工作空间 | 必须落在 `tenant_app` 壳；当前租户、当前用户与默认模块落点稳定可见 | 用户进入正确业务空间 |
| 进入 control plane | 做跨租户治理 | 必须进入 `control_plane` 壳；不复用 tenant app 侧栏与模块入口 | 控制面与数据面边界清晰 |
| 租户停用后访问旧链接 | 理解为什么进不去 | tenant app 路由必须投影到 `tenant-suspended` 或等价失败页；不能只留空白壳 | 用户获得可解释失败态 |
| 直接命中无权限路由 | 理解“不可见”和“被拒绝”的区别 | 常规导航应提前隐藏不可达入口；直接 URL 命中时仍需明确 403 壳层结果 | 壳层权限表达一致 |
| 查看通知中心 | 找到个人通知入口 | 个人通知属于 tenant app 可发现入口，不应漂到控制面工具栏或随机模块内 | 横切能力入口稳定 |
| 查看平台审计/任务 | 找到平台运行入口 | 平台审计和任务属于 control plane，可在 `/admin` 内发现，不应散落到 tenant app | 运行治理入口稳定 |
| 进入对象页 | 保持壳层与页面骨架分工 | 壳层提供边界、导航、面包屑、上下文槽位；具体列表/详情/历史由 `352` 承接 | 页面语言不漂移 |
| 移动端访问 | 保持相同 IA | 可折叠导航和堆叠布局，但 `AppBoundary / RouteGroup / Context` 不改变 | 小屏不破坏主心智 |

## 7. 共享合同、不变量与实现护栏

### 7.1 应用边界合同

- `public`、`tenant_app`、`control_plane` 是三类正式应用边界；
- 每个边界都拥有自己的默认落点、失败态和导航模型；
- `tenant_app` 与 `control_plane` 不得通过“同壳切换模式”互相伪装；
- 壳层不允许偷偷跨边界继承上下文。

### 7.2 路由分组合同

- 每条正式路由都必须归属到稳定的 `RouteGroup`；
- `RouteGroup` 按业务空间命名，而不是按实现碎片命名；
- 一级导航和顶层面包屑必须能映射到 `RouteGroup`；
- 新增模块入口必须先回答“属于哪个业务空间”，再决定路由细节。

### 7.3 导航模型合同

- 一级导航回答“当前边界下有哪些核心空间”；
- 二级导航回答“该空间下有哪些主要对象或工作区”；
- 导航节点必须能消费 `hidden / read_only / disabled` 等权限表达；
- 导航不可见不等于后端授权不存在，导航只是 UI 投影。

### 7.4 壳层上下文合同

壳层必须稳定承载：

- 当前边界
- 当前租户
- 当前用户
- 当前模块/路由分组
- 壳层级状态提示（如会话失效、租户停用、全局只读）

同时冻结：

- 页面级 `as_of / effective_date / package / read_only` 等观察上下文由 `352` 的 `ContextBar` 进一步承接；
- 壳层不得与页面上下文互相覆盖；
- 不允许每个模块自造“当前租户/当前用户”显示区。

### 7.5 路由守卫与失败态合同

- 未登录、未知租户、租户停用、无权限、找不到资源必须是不同壳层结果；
- 路由守卫负责阻断错误边界与显式失败态；
- 不允许把所有错误都挤成统一 404；
- 也不允许先渲染业务壳，再在内容区临时弹一个“你没权限”。

### 7.6 响应式与可达性合同

- 桌面端优先左侧导航 + 顶部上下文；
- 小屏可以折叠导航、改抽屉或堆叠，但不改变路由信息架构；
- 键盘导航、焦点顺序和当前所在模块高亮必须保持一致；
- 壳层切换和导航收起不能让当前边界感知消失。

### 7.7 实现护栏

- 不允许业务模块复制第二套 `AppShell`、第二套路由守卫或第二套侧栏模型。
- 不允许用运行时拼接字符串方式生成无 ownership 的临时路由组。
- 不允许在 control plane 中直接嵌入 tenant app 页面作为第一阶段默认能力。
- 不允许因为某模块复杂就绕开 `RouteGroup` 直接把页面挂到根路径。
- 不允许壳层反向拥有对象页的列表/详情/历史骨架。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `352`（列表/详情/历史页面模式）的输入

- [ ] 壳层必须为 `PageHeader + ContextBar + MainContent` 提供稳定槽位，但不拥有这些槽位内部对象语义。
- [ ] 模块导航负责“去哪里”，页面模式负责“到了以后怎么读”；两者不得重新混写。

### 8.2 对 `353`（表单与权限感知交互）的输入

- [ ] `353` 必须在 `351 + 352` 的共同骨架上定义按钮、编辑态、确认和字段交互，不得反向改写导航与边界。
- [ ] 表单级权限表达不得破坏导航级 `hidden / disabled / 403` 语义。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] `Org / Person / Job Catalog / Staffing` 必须挂在稳定 tenant app 路由组下，不得各自创造根级前缀。
- [ ] 模块首页、列表页、详情页、历史页应复用统一壳层与面包屑语言。
- [ ] 业务对象页不得自行发明“模块壳”来包裹主内容。

### 8.4 对 `370`（工作流、审计增强与集成）的输入

- [ ] Workflow 属于 tenant app 正式业务空间，不应伪装成全局控制面工具页。
- [ ] 审计增强与审批工作区应挂在稳定路由组下，并复用壳层上下文与页面槽位。

### 8.5 对 `380`（数据工作台与运营分析）的输入

- [ ] 数据工作台属于 tenant app 的正式业务空间，应有稳定一级导航归属，而不是作为各模块的“高级按钮”零散出现。
- [ ] 平台任务与平台审计属于 control plane；租户内导入导出和查询工作区属于 tenant app，两者不得混壳。

### 8.6 对 `390`（Chat Assistant）的输入

- [ ] Assistant 应有 tenant app 内稳定可发现入口；
- [ ] Assistant 结果页、回执页、评测页必须复用既有壳层和路由分组，而不是外挂一套独立 mini-app；
- [ ] 若未来存在 control plane 级 Assistant，也必须作为单独边界入口，而不是借 tenant app 壳越权。

## 9. 建议实施步骤

1. [ ] `M1`：应用边界冻结  
   冻结 `public / tenant_app / control_plane` 三层边界及其失败态落点。
2. [ ] `M2`：路由分组与默认落点冻结  
   冻结顶层前缀、`RouteGroup` 命名和角色感知首页。
3. [ ] `M3`：导航模型冻结  
   冻结一级/二级导航、面包屑、横切入口与权限感知投影。
4. [ ] `M4`：页面槽位交接冻结  
   明确壳层与 `352/353` 的交界面，避免壳层与页面模式重叠。
5. [ ] `M5`：跨计划引用收口  
   让 `343/352/353/360/370/380/390` 统一消费 `351`，不再各自发明壳层与路由。

## 10. 建议目录与落点

若按 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的模块化单体落地，建议以 `Shell` 作为前端平台层所有权落点：

- `src/Web/Shell/PublicEntry/`
  - 登录入口
  - 未知租户 / 会话失效 / 租户停用等失败页
- `src/Web/Shell/TenantApp/`
  - `/app/*` 顶层壳
  - tenant app 导航、顶栏、用户菜单、通知入口
- `src/Web/Shell/ControlPlane/`
  - `/admin/*` 顶层壳
  - 租户治理、平台审计、平台任务导航
- `src/Web/Routing/`
  - `RouteGroup` 注册
  - 默认落点
  - 边界守卫与失败态映射

冻结原则：

- Shell 层拥有边界、导航和路由注册；
- 页面层拥有对象阅读骨架；
- 业务模块只消费，不复制壳层。

## 11. 验收标准

- [ ] `351` 已成为 Greenfield HR 平台 Product Shell 与路由信息架构的单一事实源。
- [ ] `public / tenant_app / control_plane` 三层边界已经冻结，控制面与 tenant app 不再存在壳层混写空间。
- [ ] 顶层路由前缀、首批 `RouteGroup`、默认落点、失败态落点已经冻结，后续模块能直接引用。
- [ ] 壳层与页面模式的边界清晰：`351` 不再试图拥有对象页骨架，`352/353` 也不会反向改写导航和路由树。
- [ ] `343/352/353/360/370/380/390` 可以直接以 `351` 为输入，不再各自发明第二套侧栏、顶栏或根级路由模型。

## 12. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-343](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/343-superadmin-console-and-tenant-lifecycle-business-rules-plan.md)
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md)
