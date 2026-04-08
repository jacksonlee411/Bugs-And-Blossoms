# DEV-PLAN-314：View As Of P1 页面批量收口计划——Assignments / Positions / JobCatalog / DictConfigs

**状态**: 已实施（2026-04-09，`AssignmentsPage` / `PositionsPage` / `JobCatalogPage` / `DictConfigsPage` 已按计划完成 `default current + 读写解耦` 收口，并补齐页面级回归测试）

## 背景

`DEV-PLAN-311` 已冻结页面改造矩阵，并明确 `DEV-PLAN-314` 的定位应是 **P1 页面批量收口计划**：承接 `DEV-PLAN-312` 的收口模板，批量改造以下四张 A 类页面：

- `AssignmentsPage`
- `PositionsPage`
- `JobCatalogPage`
- `DictConfigsPage`

`DEV-PLAN-312` 已完成两件上位冻结：

1. `A` 类页面必须改为 `default current`，而不是继续默认常显 `As Of`；
2. 写态必须与读态解耦，不允许 `as_of -> effective_date` 一类默认传染链继续存在。

因此，`DEV-PLAN-314` 的职责不是重新讨论时间语义原则，也不是逐页重做 IA，而是把 `DEV-PLAN-312` 的减法模板批量落到 P1 页面，形成统一可执行方案。

当前这四张页面的共性问题已经比较明确：

1. 都属于 `A` 类页面：主任务默认应是浏览 current 内容，而不是让用户先理解 `as_of`。
2. 都仍保留 page-local 时间默认化逻辑，如 `todayISO()`、`parseDateOrDefault()`、`fallbackAsOf`。
3. 都不同程度存在“读态驱动写态”的默认链路：
   - `AssignmentsPage` / `PositionsPage` 直接把 `effectiveDate` 初始化为 `asOf`，并在 `asOf` 变化时持续覆盖；
   - `JobCatalogPage` 在多个 dialog 打开时用 `asOf` 预填 `effectiveDate`；
   - `DictConfigsPage` 将浏览态 `asOf`、业务写态 `enabled_on/disabled_on` 与工具态 release 的 `as_of` 混在一页中。

本计划用于冻结这四页的统一收口模板、局部特例与实施顺序，避免后续逐页实现时重新争论“当前页是否要默认显式 `As Of`”。

## 与 `DEV-PLAN-311/312/313` 的关系

- `DEV-PLAN-311` 是页面矩阵 SSOT；本计划不得重新定义页面分类与优先级。
- `DEV-PLAN-312` 是首批实施模板 SSOT；本计划直接继承其“default current + 读写解耦 + 写后不改读态”规则。
- `DEV-PLAN-313` 负责后端并行收口；本计划负责前端 P1 页面批量落地。
- 若后续发现需要改变 A 类页面的统一规则，必须先更新 `DEV-PLAN-311/312`，再回写本计划。

## 目标

1. [x] 批量完成四张 P1 A 类页面的 `default current` 收口，不再默认常显 `As Of`。
2. [x] 批量切断这四页中“读态驱动写态”的默认链路。
3. [x] 冻结四页统一的页面模板：浏览区 default current、历史模式按需进入、写表单只围绕业务日期组织。
4. [x] 为后续 `DEV-PLAN-315` 的最小 helper 抽取创造稳定重复样本，但本计划本身不提前造层。

## 非目标

- 不在本计划内重做 `OrgUnitDetailsPage`；该页由 `DEV-PLAN-312` 承接。
- 不在本计划内处理工具态页面，如 `SetIDExplainPanel`；该主题由 `DEV-PLAN-316` 承接。
- 不在本计划内引入新的全局时间 store、context、状态机或 `view + asOf` 新协议。
- 不在本计划内大改主栅格、导航结构或整体视觉布局；优先做状态职责减法。

## 页面共性与收口模板

### 1. 浏览态统一改为 `default current`

冻结规则：

- 默认进入 current 浏览语义；
- 非 history 模式下，不默认常显 `As Of` 主控件；
- history 模式按需出现入口或状态提示；
- current 模式下，URL 不要求持有 `as_of`。

说明：

- 这里的“default current”是产品层默认观察方式；
- 不等于恢复服务端 today fallback；
- 浏览器真正发起受时间切片影响的请求时，仍由边界层形成显式日期合同，相关后端收口由 `DEV-PLAN-313` 承接。

### 2. 写态不得继承浏览态

冻结规则：

- 写表单日期只允许由动作语义初始化一次；
- 初始化后，`as_of`、history 切换、URL search params 变化都不得覆盖用户已输入日期；
- 禁止继续存在以下模式：
  - `const [effectiveDate, setEffectiveDate] = useState(asOf)`
  - `useEffect(() => setEffectiveDate(asOf), [asOf])`
  - dialog 打开时默认 `effectiveDate: toDateValue(asOf)`

### 3. 写成功后不自动改读态

冻结规则：

- 保存成功后只刷新必要 query；
- 不自动跳 `as_of`、不自动切 history、不中途改浏览态；
- 如需引导用户查看新结果，应使用 toast、CTA、显式链接，而不是直接改 search params。

### 4. 工具态显式时间只允许留在工具区

该规则主要适用于 `DictConfigsPage`：

- 浏览主区改为 default current；
- release 表单的 `as_of` 保留，因为它是工具态任务参数；
- 工具态显式时间不得重新支配列表浏览态或业务写态日期。

## 页面分组

### A 组：同构页面模板

1. `AssignmentsPage`
2. `PositionsPage`

共性：

- 都有 `Load` 区块常显 `as_of`；
- 都有 page-local `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf`；
- 都将 `effectiveDate` 直接初始化为 `asOf`；
- 都在 `useEffect` 中随着 `asOf` 变化持续重置写态。

结论：

- 这两页应采用同一套 “Staffing A 类页面” 收口模板；
- 先完成这两页，可为 `DEV-PLAN-315` 是否抽最小 helper 提供真实重复样本。

### B 组：异构但可批量收口页面

1. `JobCatalogPage`
2. `DictConfigsPage`

共性：

- 都仍把 `As Of` 暴露为主浏览输入的一部分；
- 都在浏览区之外带有较多动作区/对话框/工具态表单；
- 都需要把浏览态时间与局部写态/工具态时间拆开，而不是简单“删一个日期控件”。

结论：

- 这两页不适合套同一组件模板；
- 但适合归入同一批量计划，冻结相同原则、不同局部特例。

## 页面级实施要求

### 1. `AssignmentsPage`

实施要求：

- 删除默认常显 `Load.as_of`；
- 列表默认 current，history 模式按需进入；
- `effectiveDate` 不再初始化为 `asOf`；
- 移除 `useEffect(() => setEffectiveDate(asOf), [asOf])` 一类持续同步逻辑；
- `SetIDExplainPanel` 继续允许显式时间，但只作为工具态附属能力。

验收要点：

- 切换浏览态不会改写已输入的 `effective_date`；
- 写成功后不会自动跳到新的 `as_of`；
- current 模式下页面主浏览不要求用户先操作日期。

### 2. `PositionsPage`

实施要求：

- 与 `AssignmentsPage` 采用同模板收口；
- `org_code` 保留业务筛选职责，但不与读态时间绑定；
- create 表单 `effective_date` 只按动作规则初始化一次。

验收要点：

- 改 `org_code` 或 history/current 不会重置写态日期；
- options query 仍可工作，但浏览态与写态分离；
- 页面默认不再以 `As Of` 作为主筛选入口。

### 3. `JobCatalogPage`

实施要求：

- 顶部 `As Of` DatePicker 改为 history 模式入口，而不是默认常显筛选器；
- 所有 create/move dialog 的 `effectiveDate` 不再由 `asOf` 预填；
- `updateQuery(...)` 继续只负责读态导航，不参与 dialog 默认值；
- `setid / tab / group_code` 保留业务导航职责，但不再与读态时间默认化耦合。

验收要点：

- 打开 dialog 时默认日期不再取当前浏览 `asOf`；
- 切换 tab / group / history 不会覆盖已打开 dialog 中的日期输入；
- 页面默认浏览 current，不再默认要求先选日期。

### 4. `DictConfigsPage`

实施要求：

- 浏览列表改为 default current；
- 列表级 `asOf` 不再作为主浏览输入常显；
- `createDictEnabledOn` / `disableDictDay` / `createValueEnabledOn` 继续作为业务写态日期独立存在；
- release 表单保留显式 `as_of`，但明确归类为工具态任务参数；
- 从列表跳详情时，仅在 history 模式下携带 `as_of`。

验收要点：

- 浏览态日期不再支配 create/disable/value 的写态日期；
- release 区时间能力保留，但不会把浏览页重新变成“显式 `as_of` 驱动”页面；
- 列表浏览 current 与 release 工具态显式时间完成双轨拆分。

## 实施顺序

1. [x] `AssignmentsPage`
2. [x] `PositionsPage`
3. [x] `JobCatalogPage`
4. [x] `DictConfigsPage`

排序原因：

- `AssignmentsPage` / `PositionsPage` 结构最接近，适合作为批量模板验证；
- `JobCatalogPage` 复杂度更高，但仍是典型 A 类页面；
- `DictConfigsPage` 混有浏览态与工具态双轨，适合作为本批收尾页。

## 实施分解建议

为避免后续把 `314` 错做成“逐页各自理解”的散点改造，本计划将实施过程进一步分解为 5 个连续步骤。其目标不是增加新抽象，而是把 `312` 已冻结的减法模板按复杂度逐层落地。

### 步骤 1：先冻结统一减法模板，再进入页面改造

本步骤的重点不是改代码，而是先在实现层统一三条判断标准：

- 浏览主区默认进入 `current`，而不是默认常显 `As Of`；
- 写态日期不再继承浏览态日期，也不再随着 `as_of` 变化持续重置；
- 写成功后允许刷新 query、显示 toast 或 CTA，但不自动改浏览态。

只有先冻结这三条标准，后续每一页的改造才不会重新争论“这页是否例外”。

### 步骤 2：先完成 A 组同构页面，形成真实重复样本

首批应优先落地：

1. `AssignmentsPage`
2. `PositionsPage`

原因：

- 两页都属于同一类 staffing A 页面；
- 都存在 page-local `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf`；
- 都存在 `effectiveDate` 直接取 `asOf`，以及 `asOf` 变化时持续覆盖写态的逻辑；
- 先做完这两页，才能为后续 `DEV-PLAN-315` 判断“哪些重复足够真实，值得抽最小 helper”提供事实样本。

本步骤的目标不是立刻抽象，而是先把重复反模式批量删掉。

### 步骤 3：在模板稳定后收口 `JobCatalogPage`

`JobCatalogPage` 应作为第二阶段实施对象，原因不是它不重要，而是它比 staffing 两页多了一层复杂度：

- 顶部 `As Of` 目前仍是默认常显浏览输入；
- 多个 create / move dialog 直接使用 `asOf` 预填 `effectiveDate`；
- `tab / group_code / setid` 等导航参数与读态时间同时存在，容易在改造时重新串线。

因此，这一页的收口重点应是：

- 把顶部 `As Of` 从默认筛选器改为 history 模式入口；
- 把 dialog 默认值从“取页面 `asOf`”改为“按动作规则初始化一次”；
- 保留 `tab / group_code / setid` 的业务导航职责，但不再让它们与读态时间默认化耦合。

### 步骤 4：最后处理 `DictConfigsPage`，完成浏览态 / 写态 / 工具态三轨拆分

`DictConfigsPage` 应作为本批最后一页，因为它同时包含三种时间语义：

- 浏览主区的列表时间；
- 业务写态的 `enabled_on / disabled_on`；
- release 工具区的显式 `as_of`。

这一页不能按“删掉一个日期控件”来理解，而应按“三轨拆分”实施：

- 浏览主区改为 `default current`；
- `createDictEnabledOn` / `disableDictDay` / `createValueEnabledOn` 保持业务写态独立；
- release 表单保留显式时间，但明确归类为工具态任务参数，而不是浏览主区时间。

这一步完成后，`314` 与 `316` 的边界也会更稳定：业务浏览规则留在 `314`，工具态例外留给 `316`。

### 步骤 5：以 `314` 的重复样本衔接 `315/316/317`

`314` 的最后一步不是继续加层，而是把结果交给后续计划承接：

- 向 `DEV-PLAN-315` 提供真实重复样本与反模式清单，用于最小 helper 与 stopline 接线；
- 向 `DEV-PLAN-316` 明确 `DictConfigsPage` release 区与嵌入式 Explain 区的工具态边界；
- 向 `DEV-PLAN-317` 提供统一验收对象与关键回归场景。

因此，`314` 的完成标准应理解为：

- 四张页面已按统一减法模板收口；
- 业务浏览页与工具态时间边界已清楚；
- 已形成足够稳定的重复样本，但尚未在本计划内提前造层。

## 配套 stopline

后续承接代码改造时，应阻断以下模式继续回流：

- 新增 page-local `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf`
- 新增 `effectiveDate: asOf`
- 新增 `useEffect(() => setEffectiveDate(asOf), [asOf])`
- 新增写成功后 `updateSearch({ asOf: ... })` 或等价自动跳日逻辑
- 新增工具态显式时间回流到浏览主区

说明：

- stopline 的具体接线由后续 `DEV-PLAN-315` 承接；
- 本计划只冻结需要阻断的前端反模式。

## 测试与覆盖率

覆盖率与门禁口径以仓库 SSOT 为准：

- 入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
- 前端测试与分层导向：`DEV-PLAN-300`、`DEV-PLAN-301`

本计划要求的测试重点：

- 前端测试同样遵循职责下沉原则：优先将 current/history 导航参数构建、写态初始化、dirty guard、工具态/浏览态切换等可提纯逻辑拆成最小直接测试；仅在这些直接测试无法覆盖关键用户行为时，再补页面级交互测试。
- 页面行为测试优先围绕稳定行为，而不是视觉细节截图；
- current 模式下不强制携带 `as_of`；
- history 模式下才携带 `as_of`；
- 读态变化不覆盖写态日期；
- 写成功后不自动改读态；
- `DictConfigsPage` 中工具态显式时间不回流到浏览主区。

测试组织要求：

- 新增测试应并入现有正式测试入口，使用清晰的行为簇或表驱动子测试；
- 不得为本计划新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类补洞式文件；
- 若后续抽最小 helper，由 `DEV-PLAN-315` 承接其纯函数测试与反回流门禁，本计划不提前造层。

## 交付物

1. [x] 一份 P1 页面批量收口方案（本计划）。
2. [x] 一份四页统一减法模板与局部特例说明。
3. [x] 一份按页面排序的实施顺序与验收清单。
4. [x] 一份前端反模式 stopline 清单，供 `DEV-PLAN-315` 承接。

## 验收标准

- [x] 四张 P1 页面默认进入 current，不再默认常显 `As Of`。
- [x] 四张页面不再存在 `as_of -> effective_date/enabled_on/disabled_on` 的自动继承链。
- [x] 写成功后不再自动跳日、自动切 history 或自动更新浏览态日期。
- [x] `JobCatalogPage` 多个 dialog 不再用 `asOf` 预填 `effectiveDate`。
- [x] `DictConfigsPage` 完成“浏览态 default current / release 工具态显式时间”双轨拆分。
- [x] 后续若抽 helper，应由 `DEV-PLAN-315` 承接，而不是在本计划内提前引入新层。
- [ ] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `docs/dev-plans/312-view-as-of-implementation-plan-details-single-history-anchor-and-a-pages-read-write-decoupling.md`
- `docs/dev-plans/313-view-as-of-backend-parallel-convergence-plan-explicit-date-contract-and-no-fallback.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- `AGENTS.md`
