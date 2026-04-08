# DEV-PLAN-310：全项目 view/as_of 时间语义专项检视与最小收敛方案

**状态**: 草拟中（2026-04-08 00:09 UTC）

## 背景

本专项用于沉淀当前仓库对 `view/as_of` 时间语义的全项目检视结果，并给出**不引入新线复杂度**前提下的最小收敛方案。

本次检视覆盖：

- Org / Dict / SetID / JobCatalog / Staffing 的读链路；
- Assistant 与这些读链路交叉处的时间上下文消费；
- 前端 URL、页面本地 state、后端 query/body 契约、控制器默认化逻辑。

本专项不追求一次性重建新的时间抽象层，也**不引入**：

- `timeAnchor`
- `lastTimeAnchor`
- runtime continuation envelope
- surface catalog / local choice scope / controlled options 等新线复杂对象

本专项的目标不是“把整个仓库升级成 `view + asOf` 大一统体系”，而是先批判并修复当前最突出的时间语义问题：`as_of` 语义过载、隐式 today、读写串线、页面各自定义口径。

同时，本专项补充一条用户体验收敛目标：**按接近 Workday 的方式重建用户对时间语义的心智模型**，即把“查看日期”“版本/历史”“生效日期”明确分层，而不是继续让 `as_of` 直接暴露为主界面概念。

本专项进一步明确：当前时间语义需要按**产品/UI 层**与**服务/集成层**分层治理，而不是继续用“一条规则覆盖所有场景”。

## 现状判断

### 1. 当前仓库并不存在稳定统一的 `view + asOf` 体系

多数模块当前真实使用的是：

- `as_of` 单字段 query/body；
- “current” 通过缺省 today 回填被隐式表达；
- 个别页面/功能对 `view` 的讨论并未形成仓库级契约。

因此，本仓当前主要问题不是“`view/asOf` 设计得太复杂”，而是：

**尚未完成 `view/asOf` 建模，却已经让 `as_of` 在全仓承担了过多职责。**

### 2. `as_of` 已经同时承担了至少四类含义

1. 用户请求的读切片日期；
2. “当前视角”的隐式 today 替身；
3. 页面路由和查询缓存的状态总线；
4. 写表单 `effective_date` 的默认来源。

这四类职责没有被清晰区分，导致时间语义从读取条件泄漏成了页面和写入默认值。

## 主要发现

### 1. 项目标准已禁止隐式回填 today，但代码仍存在 fail-open 默认 today

现行标准明确禁止：

1. `if asOf == "" { asOf = time.Now().UTC().Format("2006-01-02") }`
2. `if req.EffectiveDate == "" { req.EffectiveDate = ... }`
3. 以 `as_of` 回填 `effective_date`

见：[005-project-standards-and-spec-adoption.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/005-project-standards-and-spec-adoption.md)

但当前代码仍存在后端默认 today 回填：

- [orgunit_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_api.go#L1512)

前端也在多个页面通过 `todayISO()` / `fallbackAsOf` 隐式补今天：

- [OrgUnitsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.tsx#L89)
- [AssignmentsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/AssignmentsPage.tsx#L15)
- [DictValueDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictValueDetailsPage.tsx#L61)
- [SetIDGovernancePage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/SetIDGovernancePage.tsx#L291)

这说明当前时间口径尚未成为真正的仓库级不变量。

### 2. 模块间对 `as_of` 的必填性完全不一致

部分读接口要求 `as_of` 必填并 fail-closed：

- [jobcatalog_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/jobcatalog_api.go#L65)
- [staffing_handlers.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/staffing_handlers.go#L73)
- [setid_scope_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_scope_api.go#L154)

但 Org 读链允许缺失 `as_of` 并自动补今天：

- [orgunit_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_api.go#L1512)

这种差异意味着：同样是“按日期看切片”，不同模块对“缺少时间参数”采取了不同所有权和失败策略。调用方无法形成稳定预期。

### 3. 前端没有共享读视图解析层，页面各自定义 `as_of`

当前存在多处 page-local helper：

- `parseDateOrDefault`
- `todayISO`
- `fallbackAsOf`

样例：

- [OrgUnitsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.tsx#L89)
- [AssignmentsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/AssignmentsPage.tsx#L15)
- [JobCatalogPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/jobcatalog/JobCatalogPage.tsx#L102)

结果是：

- 非法值如何处理由页面自己决定；
- 缺失值是否补今天由页面自己决定；
- URL 中 `as_of` 是否被当作深链接真值、临时 state 还是默认输入，也由页面自己决定。

进一步的问题是：不少页面实际上把 `as_of` 直接暴露成主操作心智词，而不是对用户呈现为“查看日期 / View As Of”。这会把实现字段名错误提升成用户必须理解的概念。

### 4. 当前项目并未建立统一 `view=current|as_of` 合同

当前主链路更像是“只有 `as_of`，没有显式 `view`”。  
所谓 `current`，很多时候只是“没传 `as_of`，然后补 today”。

这会导致两个问题：

1. `current` 不是一个清晰的业务语义，而只是一个缺省策略；
2. `current` 和 “看今天这一天的切片” 在实现层被压成同一种东西。

### 5. `as_of` 已泄漏到写侧，形成读写串线

Org 列表页新建表单默认把 `effectiveDate` 设为 `asOf`：

- [OrgUnitsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.tsx#L353)

详情页动作表单也用 `asOf` 初始化：

- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L508)

字段策略保存后还会推动页面 `as_of` 自动切换：

- [orgUnitFieldPolicyAsOf.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/orgUnitFieldPolicyAsOf.ts#L7)

这说明 `as_of` 已不只是“怎么看”，还在悄悄影响“怎么写”和“页面后续走向”。

### 6. 详情页将读时间、版本选择、写日期计划缠在一起

详情页先拿 `asOf`，再结合版本推导 `effectiveDate`，最后再驱动写侧字段选项和动作表单：

- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L589)
- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L1955)
- [orgUnitRecordDateRules.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/orgUnitRecordDateRules.ts#L78)

`orgUnitRecordDateRules` 本身是合理的写侧规则模块，但它当前被页面上的 `as_of` 语义牵引，从而放大了读写边界混用。

更具体地说，详情页当前缺少三个独立区块：

- 查看上下文（当前/历史查看日期）
- 版本/历史（版本选择、审计、变更链）
- 本次动作生效日（写表单自己的业务日期）

这三类信息当前会互相推导，导致用户必须在脑中自己分辨“我现在看到的日期”和“我即将提交的日期”是否还是同一个东西。

### 7. Assistant 不应把时间视角当主上下文锚点

当前仓虽然没有真正落地 `timeAnchor`，但围绕 `as_of` 的设计讨论已经暴露出一个方向性风险：

- 容易把时间视角误当成对话主上下文；
- 容易让“对象 / 任务 / 权限”退居次位；
- 容易让后续追问被无条件绑定到上一次的时间条件。

本专项明确反对在当前仓引入 `timeAnchor` 或等价压缩字段。  
对话上下文里的时间语义应保持从属地位，而不是成为主锚点。

### 8. 当前前端页面盘点：多数页面应取消 `as_of` 默认常显

按“默认页面不展示查看日期主控件；只有确实支持历史切片时才提供 `查看日期 / View As Of` 入口”的原则，当前前端页面可分为三类：

#### A. 当前存在 `as_of` 默认常显，后续应取消“默认常显”

这些页面当前把 `as_of` 作为筛选栏、上下文卡片或主表单字段直接暴露给用户；后续应改为：

- 默认进入 current 观察语义；
- 非历史模式下不显示“查看日期”主控件；
- 仅在用户主动进入历史模式后，才按需显示“查看日期 / View As Of”入口；若历史来源已由版本时间轴选中态清晰表达，则不额外增加重复提示。

页面清单：

- [OrgUnitsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.tsx#L1353)
- [JobCatalogPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/jobcatalog/JobCatalogPage.tsx#L937)
- [AssignmentsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/AssignmentsPage.tsx#L135)
- [PositionsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/PositionsPage.tsx#L129)
- [DictConfigsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictConfigsPage.tsx#L494)
- [OrgUnitFieldConfigsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx#L117)
- [SetIDGovernancePage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/SetIDGovernancePage.tsx#L1005)

补充说明：

- [SetIDExplainPanel.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/SetIDExplainPanel.tsx#L281) 当前也直接显示 `as_of`，但它更接近诊断/Explain 工具而非业务浏览页；此处不按“删除日期能力”处理，而是后续改为任务态文案“查看日期 / View As Of”，避免暴露技术字段名。

#### B. 当前没有默认常显 `as_of` 主控件，但需要保留 default current，并做结构收口

这些页面当前不是“删掉某个 `as_of` 输入框”就能完成收敛，而是要解决内部语义串线：

- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L508)
- [DictValueDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictValueDetailsPage.tsx#L215)

其中：

- `OrgUnitDetailsPage` 当前已有版本时间轴与审计日志，但“查看上下文、版本选择、本次动作生效日”仍在状态与推导链上互相影响，需要做结构收口；
- `DictValueDetailsPage` 更接近目标形态：默认不常显 `as_of` 主控件，而是以版本列表/审计页签承担历史查看能力。

#### C. 当前页面确实支持历史切片

这些页面的主数据查询或判定结果会随 `as_of` 改变，因此属于“支持历史切片”的页面；后续不是删除历史能力，而是改变其默认入口和默认呈现方式：

- [OrgUnitsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.tsx#L492)
- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L597)
- [OrgUnitFieldConfigsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx#L464)
- [JobCatalogPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/jobcatalog/JobCatalogPage.tsx#L304)
- [AssignmentsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/AssignmentsPage.tsx#L67)
- [PositionsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/PositionsPage.tsx#L55)
- [DictConfigsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictConfigsPage.tsx#L184)
- [DictValueDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictValueDetailsPage.tsx#L72)
- [SetIDGovernancePage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/SetIDGovernancePage.tsx#L379)
- [SetIDExplainPanel.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/SetIDExplainPanel.tsx#L140)

结论：

- 这些页面不是“不支持历史切片”，而是“历史切片入口不应默认常显”；
- 默认 current + 按需进入历史模式，应成为新的页面基线。

## 核心批判

### 1. 仓库现在缺的不是更多时间表达，而是更少的时间表达

当前最严重的问题不是“时间语义没定义”，而是：

- 已有 `as_of` 被过度使用；
- `current` 被隐含编码；
- 读写时间职责未拆开；
- 缺失统一消费层。

在这种情况下再引入 `view` 强制迁移、`timeAnchor`、`lastTimeAnchor` 或新的压缩语义，只会复制复杂度。

### 2. `as_of` 被错误提升成页面和会话的主状态总线

一个真正稳定的设计里，`as_of` 只应表达：

- 显式请求的读切片日期。

而不应自动承担：

- current 的等价表达；
- 页面恢复和跳转的主要真值；
- 写表单默认值；
- 后续交互流程的自动迁移触发器。

对用户界面而言，还不应把 `as_of` 当成主心智词直接展示。用户该看到的是“查看日期”或 `View As Of`，而不是实现字段名。

### 3. `current` 不应被实现成“今天”

`current` 是业务观察语义，不是 `todayISO()` 的语法糖。  
把 `current` 偷偷编码成 today，会导致：

- “现在看”和“看今天这条切片”无法区分；
- 回放和重放难以稳定；
- 上下文恢复时语义漂移。

### 4. 时间语义应服从读写边界，而不是反过来

应先明确：

- 谁表达读视图；
- 谁表达写入日期；
- 谁做页面恢复；
- 谁能把读取上下文转成用户可见的默认值。

当前仓的问题正好相反：先让 `as_of` 到处流动，再由各层自行解释它。

### 5. 接近 Workday 的关键不是暴露更多日期，而是让日期各归其位

用户并不需要理解 `as_of`、`view`、`timeAnchor` 这些实现层术语。更接近 Workday 的收敛方式应是：

- 非必要场景默认不把“查看日期”做成主控件；
- 需要查看历史时，再显式进入 `View As Of / 查看日期` 语义；
- 写动作始终只围绕“生效日期”展开；
- 历史版本与审计链作为独立区块存在，而不是混成写表单的一部分。

### 6. 现行 `STD-002` 需要从“一刀切”修订为“两层规则”

本专项判断：

- 若把 `STD-002` 理解成“所有用户可见读接口都必须显式 `as_of`”，它不符合 Workday/PeopleSoft/Oracle/SuccessFactors 一类主流 HR 产品的常见产品交互；
- 若把 `STD-002` 理解成“历史切片、导出、审计、同步、跨模块读、可回放链路必须显式时间锚点，且服务端禁止静默 today”，它非常合理，而且是更强的工程约束。

因此，现行时间语义应收敛为：

1. 产品/UI 层
   - 默认 current；
   - 非必要不展示日期控件；
   - 历史查看时才显式进入“查看日期 / View As Of”。

2. 服务/集成层
   - 历史切片、导出、审计、同步、跨模块读、可缓存重放链路必须显式 `as_of`；
   - 禁止服务端静默补 today；
   - 保证可重放、可审计、可解释。

3. 写链路
   - 继续保持 `effective_date` 显式必填；
   - 不与 `as_of` 混用；
   - 不从查看上下文继承。

## 最小收敛原则

### 1. 当前阶段不引入 `timeAnchor`

明确禁止在当前仓新增：

- `timeAnchor`
- `lastTimeAnchor`
- 任意把 `view + asOf` 压成字符串再跨层传播的字段

如果后端内部确实需要一个值对象，也只能是**内部结构体**，不能升级成对外合同字段。

### 2. 当前阶段不强推全仓 `view + asOf`

当前仓最紧迫的是修复 `as_of` 语义过载，不是做一轮仓库级 `view=current|as_of` 大迁移。

最小策略应是：

- 历史切片型、导出型、审计型、同步型、跨模块读接口：显式要求 `as_of`，缺失 fail-closed；
- 真正 current-first 的产品浏览场景：允许 default current，但不得通过服务端 today 回填伪装成 `as_of`；
- 在未建立共享消费层前，不扩大 `view` 的暴露面。

### 3. 用户界面不以 `as_of` 作为主心智词

对用户可见层冻结以下约束：

- 不直接把 `as_of` 作为页面主标签、主字段名、主说明文案；
- 统一对用户展示为“查看日期”或 `View As Of`；
- “查看日期”仅在确有历史查看需求时才显式出现，非必要页面默认不提供该控件；
- 默认进入 current 观察语义时，不要求用户先理解或操作日期控件。

说明：

- 这里的“默认”是产品默认观察方式，不是服务端/前端继续使用 implicit today 回填；
- 技术合同层仍应保持显式时间语义与 fail-closed，不得因为 UI 默认化而恢复默认 today；
- 产品层隐藏日期控件，不等于下游服务可以失去时间边界。

### 4. `as_of` 只允许表达读切片日期

明确禁止：

- 用 `as_of` 默认填充 `effective_date`
- 用 `as_of` 代替写侧业务日期
- 用 `as_of` 触发隐式写入推理

同时冻结一条更严格的 UI 规则：

- 写表单默认不继承当前查看日期；
- 当前阶段暂不提供“使用当前查看日期”之类显式复制按钮，避免重新把读时间偷渡回写时间。

### 5. 写表单只围绕“生效日期”组织

写表单必须满足：

- 永远单独展示“生效日期”；
- 由写侧规则、动作类型与业务约束决定其默认值/可编辑性；
- 不从页面查看上下文、历史版本选择器、URL 中的 `as_of` 自动继承；
- 不因查看上下文变化而自动改写用户已输入的生效日期。

### 6. 隐式 today 必须被消除

后端和前端都不应再出现：

- 缺参补今天后继续执行；
- 非法日期自动吞掉后退回今天；
- URL 不带 `as_of` 但页面假定自己处于历史切片模式。

### 7. 读视图解析应有共享入口

前端至少需要一个共享 helper，统一处理：

- `as_of` 缺失；
- `as_of` 非法；
- 是否处于 current 模式还是历史模式；
- 页面应该进入何种初始态；
- 何时需要把产品层的 current/history 语义转成下游显式时间合同。

但这个 helper 不应额外制造新概念；只需围绕 `as_of` 与显式 current 策略工作。

### 8. 详情页必须拆成三个独立区块，禁止互相推导

详情页冻结以下 IA/交互约束：

1. 查看上下文区块
   - 仅负责表达当前是否处于历史查看、当前查看日期是多少；
   - 默认弱化展示，非历史查看场景可折叠或不作为主控件呈现；
   - 不直接驱动写表单字段赋值。

2. 版本/历史区块
   - 仅负责版本选择、记录列表、审计与变更链查看；
   - 选择某个历史版本时，只影响详情展示内容；
   - 不自动改写“本次动作生效日”输入值。

3. 本次动作生效日区块
   - 仅服务于当前动作提交；
   - 独立展示 `effective_date`；
   - 由动作规则单独校验，不从“查看上下文”或“版本/历史”自动推导。

禁止以下链路继续存在：

- `查看日期 -> 写表单默认生效日期`
- `版本选择 -> 自动覆盖本次动作生效日`
- `保存某类配置后 -> 自动推动页面查看日期跳转`

说明：

- 这里的“三区块”首先是**状态职责拆分**，其次才是视觉布局拆分；
- 不要求页面一定新增三张大卡片或三列布局；
- 对于已有版本时间轴和审计日志的详情页，真正要补的是“查看上下文”和“本次动作生效日”的边界明确，而不是重复再造一套历史 UI。

### 9. `OrgUnitDetailsPage` 采用低增量收口，不做整页重构

针对 [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx)，本专项选定**低增量改造**，而不是重做页面布局：

#### 9.1 保留现有能力

- 保留现有版本时间轴/版本列表；
- 保留现有修改日志/Audit 能力；
- 保留现有详情展示与动作区的大体布局；
- 保留 URL 中的历史切片能力与详情深链接能力。

#### 9.2 新增/调整的只应是边界，而不是大规模 UI

1. 查看上下文
   - 默认 current 模式下，不新增常显日期输入框；
   - 当页面实际处于历史切片时，优先依赖现有版本时间轴/选中态表达“正在查看历史记录”；
   - 仅当历史来源不明显，或页面同时存在读写混淆风险时，才在页头或详情主区上方增加轻量状态提示，例如“查看日期 / View As Of: 2026-01-01”；
   - 该提示只说明当前查看语义，不承载写入默认值。

2. 版本/历史
   - 继续使用现有版本时间轴与审计日志；
   - 版本选择仅改变“当前详情展示的是哪条记录”；
   - 不再承担“替动作表单决定默认生效日期”的职责。

3. 本次动作生效日
   - 继续放在当前动作区/表单区；
   - 始终以独立字段“生效日期”展示；
   - 默认值由动作规则自己决定，不回退到查看日期，不回退到当前选中版本日期；
   - 一旦用户输入，切换历史版本或切换历史查看上下文都不得自动覆盖。

#### 9.3 必须切断的状态链

在实现上，应明确切断以下推导链：

- `asOf -> effectiveDate(详情展示) -> actionWriteEffectiveDate`
- `selectedVersion/effectiveDate -> actionForm.effectiveDate`
- `updateSearch(asOf=...) -> actionForm 重置为 asOf`

对应当前风险点：

- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L589)
- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L610)
- [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx#L518)

#### 9.4 布局影响判断

该方案的预期是**小幅调整、降低混乱，而不是增加布局复杂度**：

- 默认模式下，页面可比现在更简洁，因为“查看日期”不再作为常显主控件；
- 历史模式下，优先复用现有时间轴选中态；仅在必要时新增轻量状态提示，不要求增加新的大面板；
- 版本时间轴和修改日志复用现有区块，不额外制造第二套历史 UI；
- 真正新增的是“动作区生效日期”的边界清晰度，而不是视觉层级。

因此，本专项判断：

- **实现复杂度**：中低，主要是状态解耦与默认值收口；
- **布局复杂度**：低，不要求重构主栅格；
- **用户认知复杂度**：下降，因为“我现在在看什么”和“我这次要提交哪一天生效”不再混在一起。

## 非目标

本专项明确不包含以下事项：

1. [ ] 不引入 `Ficeae` 的 workspace / controlledOptions / local choice scope 体系。
2. [ ] 不将本仓立即迁移为仓库级 `view + asOf + handoff` 统一合同。
3. [ ] 不新增 `timeAnchor` 相关字段。
4. [ ] 不把 Assistant 的时间上下文提升为主上下文锚点。
5. [ ] 不通过“继续加 helper / 再包一层字符串协议”掩盖现有语义过载。

## 建议实施步骤

1. [ ] 盘点并删除所有“缺失 `as_of` 自动补 today”的后端逻辑，优先从 Org 读接口开始。
2. [ ] 盘点并收敛前端页面内的 `parseDateOrDefault / todayISO / fallbackAsOf` 逻辑，建立共享 read-date parser。
3. [ ] 收敛用户可见文案：页面不再直接暴露 `as_of`，统一改成“查看日期 / View As Of”。
4. [ ] 建立页面级默认策略：非必要页面默认不提供“查看日期”主控件；只有历史切片确有价值时才显式展示。
5. [ ] 切断 `as_of -> effective_date` 的默认传染；写表单默认值由写侧规则决定，而不是由读上下文继承。
6. [ ] 对详情页执行三区块拆分：查看上下文 / 版本历史 / 本次动作生效日，各自单向负责，不再互相推导。
7. [ ] 对每个模块显式声明：`as_of` 是必填、可选还是不适用，并统一错误语义。
8. [ ] 将 Assistant 中的时间上下文限定为从属条件，只允许消费显式 `as_of`，不新增新的时间锚点协议。
9. [ ] 以上述页面盘点为基线，输出一份“取消默认常显 / 保留 default current 但做结构收口 / 历史模式按需出现”的页面改造清单。
10. [ ] 对 `OrgUnitDetailsPage` 输出低增量实施说明：保留现有版本时间轴与日志区，只补历史模式提示与动作区生效日期边界，并切断现有状态推导链。

## 交付物

1. [ ] 一份全仓读时间语义清单：模块、接口、页面、必填策略、缺省策略、失败策略。
2. [ ] 一份最小共享前端 read-date helper 设计说明。
3. [ ] 一份“查看日期 / 生效日期 / 版本历史”用户心智与 IA 约束说明，作为页面改造基线。
4. [ ] 一份后端 `as_of` fail-open 清理清单。
5. [ ] 一份读写时间边界检查清单，覆盖 Org / Dict / SetID / JobCatalog / Staffing / Assistant。
6. [ ] 一份 `OrgUnitDetailsPage` 低增量收口说明，明确保留区块、状态切断点与布局影响评估。

## 结论

本仓当前不适合继续扩张时间抽象。  
最需要做的不是引入 `timeAnchor` 或大规模推广 `view + asOf`，而是：

- 承认当前主问题是 `as_of` 语义过载；
- 先消除隐式 today；
- 先恢复读写边界；
- 先建立共享解析入口；
- 再决定是否真的需要更高阶的时间表达。

在这些前提满足前，任何新增时间表达层都应被视为复杂度放大器，而不是收敛工具。

按接近 Workday 的用户心智收口后，页面时间语义应稳定为：

- 默认场景：用户不需要先操作日期控件；
- 历史查看场景：优先由版本时间轴/选中态表达当前历史上下文，仅在来源不明显或存在读写混淆风险时补充“查看日期 / View As Of”提示；
- 写动作场景：用户只面对“生效日期”；
- 详情页结构：查看上下文、版本/历史、本次动作生效日三者分区明确、互不偷渡。
