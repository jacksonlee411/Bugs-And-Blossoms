# DEV-PLAN-352：列表/详情/历史页面模式详细设计

**状态**: 规划中（2026-03-18 14:27 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 的 `M2: Page Patterns` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“列表 / 详情 / 历史是统一产品语言、生效日期是 UI 一级概念”的冻结；
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 对 `current / as_of / history`、`Valid Time / Audit Time` 的共享时间合同；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对租户/用户上下文、失败态区分的前端输入；
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 对权限感知 UI 的共享表达要求；
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md) 对审计页、任务中心、通知中心状态表达的共享输入；
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 对配置/策略治理页与 Explain 页模式的共享输入；
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md) 与 [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 已经沉淀出的业务页面样板。

`350` 已明确系统需要统一的页面模式，但当前仓库里仍缺一份真正拥有“**列表页回答什么、详情页回答什么、历史页回答什么、它们如何共享时间上下文与状态表达**”的文档。  
没有这份计划，后续 `360/370/380/390` 很容易继续各自发明：

- 列表页到底是“当前集合”还是“某日切片”；
- 详情页到底是在看“对象身份”还是“某个生效版本”；
- 历史页到底展示业务有效历史、审计时间线，还是两者混在一起；
- `read_only / disabled / 403 / empty / loading` 到底应该怎么落在页面骨架上；
- 工作台页究竟是独立第四套页面，还是由列表/详情/历史组合而来。

`352` 的职责就是把这些问题收敛为 **Greenfield HR 平台的页面模式 SSOT**，让后续子计划引用同一套产品语言，而不是继续围绕单个业务样板补丁式复制。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述列表页、详情页、历史页，不让组件名、路由碎片或临时布局成为主叙事。
- [ ] 冻结三类基础页面模式各自回答的用户问题、时间语义、状态表达与组合边界。
- [ ] 明确 `PageHeader / ContextBar / FilterBar / List / Detail / Timeline / Status Surface` 的职责分层，避免后续页面各自重组骨架。
- [ ] 明确生效日期对象与普通对象在页面模式上的共同部分与差异化要求。
- [ ] 为 `351/353/360/370/380/390` 提供可直接消费的页面模式输入，避免后续子计划再发明第四套对象页骨架。

### 2.2 非目标

- [ ] 本计划不承接模块导航、路由树与应用壳布局；这些由后续 `351` 承接。
- [ ] 本计划不冻结按钮、表单字段级校验、确认弹层与权限行为细则；这些由后续 `353` 承接。
- [ ] 本计划不替代 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 的时间合同，只定义页面如何表达它。
- [ ] 本计划不承接具体业务规则、数据库 DDL 或前端代码实现。
- [ ] 本计划不把“工作台页”定义成脱离列表/详情/历史的第四套基础页面模式。

## 3. “业务规则优先”在页面模式中的翻译

### 3.1 页面首先回答业务问题，而不是展示组件拼装

`352` 冻结以下产品语言：

- **列表页** 回答“在当前观察上下文里，有哪些对象可看、可筛、可选”；
- **详情页** 回答“这个对象在当前观察点上是什么样”；
- **历史页** 回答“这个对象是如何演进的、谁在什么时候改了什么”。

因此，页面模式的主顺序必须是：

1. 先定义用户问题；
2. 再定义页面骨架；
3. 最后才决定使用 `DataGrid`、双栏、Tabs、Timeline 还是其它组件组合。

### 3.2 `current / as_of / history` 是产品语言，不是查询实现细节

页面必须显式承接 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 的共享时间合同：

- `current` 是产品表达；
- `as_of` 是显式日期视图；
- `history` 是完整演进链。

页面层不得把“没有传日期”默认为“今天”；也不得把业务有效时间与审计时间混为同一条时间线。

### 3.3 工作台是组合态，不是第四套独立骨架

`350` 在大类上提到工作台页，但 `352` 选定的做法是：

- 工作台页本质上是“列表 + 详情 + 历史/回执”的组合；
- 组合顺序可以因场景不同而变化，但基础语义不能另起炉灶；
- `380/390` 可以在此基础上叠加批次、任务、评测等工作台能力，但不能因此长出一套与对象页完全断裂的新页面语言。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的样板与结论

#### 4.1.1 Org 已验证“双栏详情 + 双时间轴”模式

- [DEV-PLAN-081](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/081-orgunit-records-version-selector-two-pane-alignment.md) 已验证：
  - 生效记录可使用“左侧生效日期列表 + 右侧详情”的双栏模式；
  - 变更日志可使用“左侧修改时间列表 + 右侧事件详情”的双栏模式；
  - `tree_as_of` 与 `effective_date` 必须解耦。

#### 4.1.2 Job Catalog 已验证“上下文栏 + Tabs + DataGrid”工作区模式

- [DEV-PLAN-104](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104-jobcatalog-ui-optimization.md) 与 [DEV-PLAN-104A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md) 已验证：
  - `as_of + package + read_only` 需要稳定可见的上下文栏；
  - 主列表应优先走 `DataGrid` 基线，而不是裸表格；
  - 列表选择、写入口与只读表达必须始终挂在同一上下文上。

#### 4.1.3 MUI X 页面基线已经明确

- [DEV-PLAN-090](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/090-mui-x-frontend-upgrade-plan.md) 已明确平台组件基线应收敛到：
  - `AppShell`
  - `PageHeader`
  - `DataGridPage`
  - `DetailPanel`
  - `FilterBar`

#### 4.1.4 基础 UI 语义已冻结

- [DEV-PLAN-002](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/002-ui-design-guidelines.md) 已冻结：
  - `disabled` 与 `read-only` 需要明确区分；
  - 空状态、错误状态、加载状态必须有一致语义；
  - 数据密集页面在 `md/lg` 优先双栏，在 `sm/xs` 可退化为纵向堆叠。

### 4.2 当前主要缺口

1. [ ] **页面意图仍漂移**  
   现在大家知道要做“列表/详情/历史”，但还没有统一回答每一页到底解决什么用户问题。
2. [ ] **时间语义仍可能在页面层被混写**  
   `as_of`、`effective_date`、`tx_time`、`last_updated_at` 很容易在同一视觉区里互相冒充。
3. [ ] **业务页样板仍靠局部经验维持**  
   Org 和 Job Catalog 已经各自成熟，但还没有被提升为共享页面模式语言。
4. [ ] **状态表达尚无统一骨架**  
   加载、空态、错误、只读、无权限、回执状态目前还缺一套“页面必须预留哪些位置”的约束。
5. [ ] **工作台模式仍可能被误做成独立页面体系**  
   若不收敛，`380/390` 很容易用“这是工作台”作为理由绕过统一对象页语言。

## 5. 页面模式目标蓝图

### 5.1 领域使命

`352` 是 Greenfield HR 平台内“**对象列表如何被发现、对象详情如何被阅读、对象历史如何被解释，以及这些页面如何共享上下文、时间与状态表达**”的唯一页面模式权威。  
它不拥有业务规则、不拥有表单提交细节、不拥有导航壳，但拥有所有后续业务页共同依赖的骨架语言。

### 5.2 核心页面对象

| 页面对象 | 业务含义 | 是否由 `352` 拥有 |
| --- | --- | --- |
| `PageHeader` | 回答“我现在处于哪个模块/对象空间，正在做什么” | 是 |
| `ContextBar` | 回答“我当前观察的是哪个租户上下文、哪个日期、哪个包或哪个只读状态” | 是 |
| `ListSurface` | 回答“在当前上下文里有哪些对象可选、可筛、可排序” | 是 |
| `SelectionState` | 回答“当前选中了哪个对象/哪条记录，以及切换后页面如何联动” | 是 |
| `DetailSurface` | 回答“当前对象在当前观察点上是什么样” | 是 |
| `VersionSelector` | 回答“当前正在看哪个生效版本或哪个历史节点” | 是 |
| `HistorySurface` | 回答“对象如何沿业务时间或审计时间演进” | 是 |
| `StatusSurface` | 回答“当前页面是否处于 loading / empty / error / read_only / no_access / receipt_pending 等状态” | 是 |
| `FormSurface` | 回答“如何编辑、校验、确认并提交” | 否，`353` 拥有交互细则 |
| `RouteGroup` | 回答“这个页面挂在哪个导航与路由分组下” | 否，`351` 拥有 |

### 5.3 面向用户的主能力

- 在一个稳定上下文内浏览对象集合；
- 从列表快速定位并切换对象；
- 查看某对象在某个观察日期上的详情；
- 在业务有效时间线上切换版本；
- 在审计时间线上查看事件、快照与回执；
- 明确知道当前页面是可写、只读、无权限，还是仅仅尚未命中数据；
- 在桌面与移动端都能保持“先看上下文，再看集合，再看详情/历史”的同一心智。

### 5.4 选定的基础页面模式

#### 5.4.1 列表页：回答“当前上下文里有哪些对象”

列表页必须具备：

- 稳定可见的 `PageHeader + ContextBar`；
- 集合视图本体，优先采用 `DataGrid / Tree + List` 这类可索引、可筛选、可排序的结构；
- 明确的选中态，而不是把“点击某行后跳去了哪里”作为唯一反馈；
- 空态、错误态、无结果态与只读态的首屏落点。

列表页允许的首批变体：

- **L1：平面列表**  
  适用于 Person、Position、Assignment、导出任务等平面集合。
- **L2：树/层级列表**  
  适用于 Org、分类树等需要层级浏览的对象集合。
- **L3：上下文工作区内的分列表**  
  适用于 Job Catalog 这类“一个上下文下有多类对象”的页签工作区。

#### 5.4.2 详情页：回答“这个对象在当前观察点上是什么样”

详情页必须具备：

- 对象身份锚点：当前看的到底是谁；
- 观察点锚点：当前看的到底是哪个 `as_of` 或哪个 `effective_date`；
- 结构化信息分区，而不是长表单式堆砌；
- 在复杂对象上优先使用双栏或独立详情页，而不是靠多个零散弹窗拼出完整维护体验。

详情页允许的首批变体：

- **D1：单快照详情**  
  适用于无生效日期版本切换，或当前只查看单时间点快照的对象。
- **D2：双栏版本详情**  
  左侧版本/记录选择，右侧详情内容；适用于 effective-dated 主对象和审计密集对象。

#### 5.4.3 历史页：回答“它是如何演进的”

历史页必须显式区分两种时间线：

- **H1：业务有效历史（Valid Time）**  
  重点回答“哪一天起它是什么样”；
- **H2：审计/操作历史（Audit Time）**  
  重点回答“谁在什么时刻做了什么、为什么、有没有回执/快照”。

因此，历史页禁止：

- 用“最后修改时间”替代业务有效历史；
- 用“生效日期”冒充审计时间轴；
- 把完整历史裁剪成“当前记录 + 一条备注”。

#### 5.4.4 工作台页：回答“我如何在同一上下文里完成密集观察”

`352` 选定：

- 工作台页是 `ListSurface + DetailSurface + HistorySurface/ReceiptSurface` 的组合态；
- 它可以是 Tabs、双栏或上下分区，但不改变三类基础页面对象的语义；
- `380` 与 `390` 若需要批次、评测、回执工作台，必须在此基础上组合，而不是另发明一套页面语言。

## 6. `352` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心页面规则 | 业务结果 |
| --- | --- | --- | --- |
| 浏览集合 | 看清当前上下文下有哪些对象 | 列表页必须先显示上下文，再显示集合；不得隐藏时间/包/只读等关键观察条件 | 用户知道自己在看哪一层集合 |
| 定位对象 | 从集合中选中一个对象继续查看 | 选中态必须可见；列表与详情联动不能靠隐式跳转猜测 | 用户知道当前正在看谁 |
| 查看某日详情 | 看清对象在某个日期上的状态 | 详情页必须显式展示当前观察点；`as_of` 与 `effective_date` 不得混写 | 当前快照可解释 |
| 切换版本 | 在多个生效版本间切换 | 版本选择器必须是显式页面构件；复杂对象优先双栏，不依赖上一条/下一条心智 | 版本切换稳定、可追溯 |
| 查看有效历史 | 理解对象沿业务时间如何演进 | `history` 不能退化为只看当前；有效历史必须按业务日期组织 | 历史链可读、可复算 |
| 查看审计历史 | 理解谁在何时做了什么 | 审计时间线必须独立于业务有效时间线；快照/回执只作证据，不替代详情事实 | 审计链可解释 |
| 空结果与错误 | 区分“没有数据”“筛选无命中”“无权限”“系统失败” | 页面必须有独立状态承载面，不允许所有异常都只弹 toast | 用户知道下一步该做什么 |
| 只读与受限 | 理解为什么当前不能写 | 页面骨架必须预留只读/禁用/403 的表达位置；具体语义由 `342/353` 承接 | 权限差异可见而不误导 |
| 移动端退化 | 在小屏继续完成核心观察 | 双栏可退化为纵向堆叠，但上下文、选中对象与历史入口不能消失 | 页面模式在移动端仍成立 |

## 7. 共享合同、不变量与实现护栏

### 7.1 页面意图合同

三类基础页面各自拥有稳定职责：

- 列表页拥有“集合与筛选”；
- 详情页拥有“对象快照与结构化说明”；
- 历史页拥有“时间演进与证据说明”。

它们可以组合，但不得互相吞并到无法辨认：

- 列表页不能假装自己已经完整回答历史问题；
- 详情页不能把所有历史塞成一小段备注；
- 历史页不能越权成为新的主详情事实源。

### 7.2 时间上下文合同

页面必须显式承接三类时间语义：

- `as_of`：当前观察日期；
- `effective_date`：当前版本或写意图对应的业务生效日；
- `tx_time / changed_at`：审计时间。

实现护栏：

- 不得用缺省 today 隐式补齐；
- 不得把列表观察日期强制覆盖详情版本日期；
- 不得让“最近修改时间”冒充“当前业务有效时间”。

### 7.3 状态承载面合同

每个基础页面都必须为以下状态预留明确位置：

- `loading`
- `empty`
- `filtered_empty`
- `error`
- `read_only`
- `no_access`
- `receipt_pending / receipt_failed`（当页面观察异步或治理状态时）

其中：

- 这些状态在页面上必须有稳定落点；
- 具体按钮禁用、只读表现、403 文案与确认交互细则由 `342/353` 冻结；
- `352` 只冻结“页面必须看得见这些状态，且不能互相混淆”。

### 7.4 URL、上下文与选中态合同

页面必须满足：

- 用户刷新后可恢复当前观察上下文；
- URL 只承载稳定观察条件与可分享的选中锚点；
- 本地存储不得成为第二套权威上下文事实源；
- 当上下文切换导致原选中对象不再命中时，页面必须显式清空或重置选中态，而不是悄悄展示过期对象。

### 7.5 组件与响应式合同

`352` 推荐但不强绑具体实现，首批组件基线应优先复用：

- `PageHeader`
- `FilterBar`
- `DataGridPage`
- `DetailPanel`
- 时间轴/列表式版本选择器

实现护栏：

- 主列表不得回退为裸 `<table>`；
- 复杂对象优先独立页或双栏，不鼓励以多个弹窗拼装主流程；
- `md/lg` 优先双栏或并列观察；`sm/xs` 可改为堆叠，但不得牺牲上下文锚点与历史入口。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `351`（Product Shell 与路由信息架构）的输入

- [ ] 路由与壳层必须为 `PageHeader + ContextBar + 主内容区` 预留稳定位置，而不是让每个模块自搭页头。
- [ ] 模块导航负责“去哪里”，页面模式负责“到了以后怎么读”；两者不得相互替代。

### 8.2 对 `353`（表单与权限感知交互）的输入

- [ ] `353` 必须在 `352` 已冻结的页面骨架上定义编辑、确认、校验、`hidden / read_only / disabled / 403` 的交互细则。
- [ ] 表单与按钮权限表达不得破坏列表/详情/历史三类页面的主问题边界。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] `361/362/363/364` 必须从 `352` 选择页面模式，而不是再各自发明对象页骨架。
- [ ] effective-dated 主对象默认应支持“列表/树 + 详情 + 历史”三件套。
- [ ] 详情页必须显式表达身份锚点、观察点和当前版本，不得把时间语义藏进按钮或二级弹窗。

### 8.4 对 `370`（工作流、审计增强与集成）的输入

- [ ] Workflow/Audit 页需要把“业务有效历史”和“审计/审批时间线”分开展示，不得混成一条模糊历史。
- [ ] 回执、快照与长事务状态应挂在历史/证据视图上解释，而不是污染业务详情主事实。

### 8.5 对 `380`（数据工作台与运营分析）的输入

- [ ] Query Workspace、批次中心、导出中心应复用列表/详情/历史的组合模式，而不是重新定义“工作台就可以不讲对象页语言”。
- [ ] 数据质量与批次回执页若需要密集观察，可在工作台中组合多个基础页面对象，但上下文与状态承载面仍需一致。

### 8.6 对 `390`（Chat Assistant）的输入

- [ ] Assistant 的历史、动作回执、评测页应复用历史/工作台模式，不得发明与主产品割裂的页面语言。
- [ ] `action request` 的状态追踪应作为历史/回执视图的一部分呈现，而不是只靠聊天气泡猜状态。

## 9. 建议实施步骤

1. [ ] `M1`：页面意图与时间上下文冻结  
   统一列表/详情/历史三类页面各自回答的问题，以及 `as_of / effective_date / tx_time` 的展示边界。
2. [ ] `M2`：列表模式冻结  
   明确平面列表、树列表、工作区分列表的共享骨架、上下文栏与选中态合同。
3. [ ] `M3`：详情模式冻结  
   明确单快照详情与双栏版本详情的适用边界，收敛复杂对象优先走详情页的规则。
4. [ ] `M4`：历史模式冻结  
   明确有效历史与审计历史的差异、时间轴组织方式与证据呈现规则。
5. [ ] `M5`：下游引用收口  
   让 `351/353/361/363/370/380/390` 正式引用 `352`，不再各自重写页面模式语言。

## 10. 验收标准

- [ ] `352` 已成为 Greenfield HR 平台列表/详情/历史页面模式的单一事实源，而不是继续分散在 `081/104/350/361/363` 的局部样板中。
- [ ] 后续子计划在描述页面时，可以直接引用 `352`，而不是再发明第四套对象页骨架。
- [ ] 业务有效时间与审计时间在页面层已有明确可见的分层，不再存在“看起来像历史，其实不知道是哪条时间线”的漂移空间。
- [ ] 工作台页已被正式收敛为列表/详情/历史的组合态，而不是新的独立页面体系。
- [ ] `351` 与 `353` 能以 `352` 为前置输入继续细化，而不与它重复或打架。

## 11. 关联文档

- [DEV-PLAN-002](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/002-ui-design-guidelines.md)
- [DEV-PLAN-081](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/081-orgunit-records-version-selector-two-pane-alignment.md)
- [DEV-PLAN-090](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/090-mui-x-frontend-upgrade-plan.md)
- [DEV-PLAN-104](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104-jobcatalog-ui-optimization.md)
- [DEV-PLAN-104A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
