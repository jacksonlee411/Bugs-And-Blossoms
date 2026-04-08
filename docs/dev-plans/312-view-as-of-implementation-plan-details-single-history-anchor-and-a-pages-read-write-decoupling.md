# DEV-PLAN-312：View As Of 收口实施计划——详情页单历史锚点与 A 类页面读写解耦

**状态**: 已完成（2026-04-09 CST）

## 背景

`DEV-PLAN-311` 已冻结页面改造矩阵、`OrgUnitDetailsPage` 样板页方向，以及“继续做减法”的复杂度收敛原则。

当前缺口不再是“是否要继续讨论时间语义原则”，而是**如何以最低实现复杂度把这些原则落到代码**：

1. `OrgUnitDetailsPage` 仍存在 `as_of + effective_date` 双读态；
2. 多张 A 类页面仍存在“读态驱动写态”的普遍模式；
3. 页面本地仍重复出现 `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf` 一类默认化 helper；
4. 若不明确实施顺序与 stopline，后续容易一边删复杂度、一边通过新 helper 或自动跳日逻辑把复杂度加回去。

本计划作为 `DEV-PLAN-311` 的实施子计划，负责把“先改什么、如何改、何时允许抽 helper、哪些模式必须禁止”冻结为可执行方案。

## 与 `DEV-PLAN-311` 的关系

- `DEV-PLAN-311` 是本专项的父计划与页面矩阵 SSOT，负责冻结页面分类、收敛原则与样板页方向。
- 本计划是 `DEV-PLAN-311` 的实施子计划，负责把其原则转译成代码改造顺序、状态模型、页面级策略与验收步骤。
- 本计划**不得**重新定义 `A/B/C` 页面分类，也不得重新发明新的时间语义框架。
- 若后续实施发现需要改变页面分类或上位原则，必须先更新 `DEV-PLAN-311`，再回写本计划。

## 目标

1. [x] 冻结 `OrgUnitDetailsPage` 的单历史锚点实施方案，彻底移除 `as_of + effective_date` 双读态。
2. [x] 冻结 A 类页面统一收口方案，消除“读态驱动写态”的普遍模式。
3. [x] 冻结最小共享 helper 的抽取时机与职责边界，避免提前造层。
4. [x] 冻结实施顺序、页面优先级、测试要点与 stopline，供后续代码改造直接承接。

## 非目标

- 不在本计划内重定义 `DEV-PLAN-311` 已冻结的页面矩阵。
- 不在本计划内引入新的全局时间框架、context、store 或状态机。
- 不在本计划内直接修改后端 API/Kernel 契约；后端残余 `today` 默认化仅作为并行收口项跟踪。
- 不在本计划内一次性完成所有页面改造；优先完成 P0，再为 P1/P2 提供统一模板。

## 约束来源（SSOT）

- `DEV-PLAN-311`：`docs/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `DEV-PLAN-310`：`docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
- `STD-002`：`docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `DEV-PLAN-102B`：`docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `DEV-PLAN-003`：`docs/dev-plans/003-simple-not-easy-review-guide.md`

## 现状问题归类

### 1. `OrgUnitDetailsPage`：局部重灾区，表现为“双读态”

当前详情页同时持有：

- `as_of`：页面读切片输入；
- `effective_date`：详情版本选择/深链接输入；
- `actionEffectiveDate`：写动作表单生效日。

这会导致页面需要额外的“解析层”在多个时间字段之间来回折中，形成最重的认知复杂度和状态复杂度。

### 2. A 类页面：更普遍的问题是“读态驱动写态”

虽然多数 A 类页面并不存在详情页那种“双读态”，但普遍存在以下模式：

- 创建/启用/发布表单默认把 `effective_date` 设为 `as_of`；
- 当 `as_of` 改变时，写表单自动重置为新 `as_of`；
- 保存成功后，页面自动切换 `as_of` 或跳到新生效日。

因此，A 类页面的核心问题不是“双读态”，而是**读写时间职责未拆开**。

### 3. 仓库级共因：`as_of` 被过载成页面状态总线

页面本地反复出现：

- `todayISO()`
- `parseDateOrDefault()`
- `fallbackAsOf`

说明当前仓仍有较强的 page-local 默认化倾向，`as_of` 仍在承担“默认浏览 current、深链接状态、写表单默认值、写后页面跳转”的混合职责。

## 关键设计决策

### 决策 1：详情页采用“单历史锚点”，不再保留“双读态”

选定方案：

- `OrgUnitDetailsPage` 只保留一个历史查看锚点；
- `current` 模式下不持有显式 `as_of`；
- `history` 模式下由“选中的历史版本记录”承担历史查看锚点；
- 若需兼容历史深链接，应在进入页面边界时把 `as_of` 解析为历史版本选中态，而不是长期保留为详情页核心状态。

不选方案：

- 继续保留 `as_of + effective_date` 双读态，再通过 helper 协调。

原因：

- 该方案表面上能兼容更多输入，但会把复杂度从页面搬到解析层，并持续扩大状态面。

### 决策 2：A 类页面统一切断“读态驱动写态”

选定方案：

- A 类页面的 `as_of` 只影响读模型与历史查看；
- 写表单 `effective_date` 由动作规则初始化一次；
- 初始化完成后，用户已修改的写态不再被 `as_of`、history 切换或 search params 自动覆盖。

不选方案：

- 保留“读态变化时自动同步写态”，只是在 UI 上加更多提示。

原因：

- 这会继续保留“查看什么”和“准备写什么”之间的隐式耦合，用户仍需自己判断两者是否是同一个日期。

### 决策 3：写成功后不自动改读态

选定方案：

- 写成功后不自动切换 `as_of`、history 选中态或详情版本；
- 若确有必要引导用户查看新结果，使用显式 CTA 或 toast。

不选方案：

- 保存成功后自动跳到新生效日或自动切换历史版本。

原因：

- 自动跳转会继续把页面读态当作写结果展示容器，放大页面状态串线。

### 决策 4：共享 helper 延后抽取，且只允许最小纯函数

选定方案：

- 先落样板页与首张 A 类页面；
- 证明存在重复后，再抽最小纯函数；
- helper 仅处理“读态解析”和“显式合同形成”，不处理写态默认值、页面布局或动作区状态机。

不选方案：

- 先建设通用 `view/as_of` helper、全局时间 store 或 context。

原因：

- 目前问题首先是状态太多，而不是抽象太少；先造层会把局部复杂度升级成全局复杂度。

## 状态模型方案

### 1. `OrgUnitDetailsPage` 目标状态

页面状态只保留三类：

- `readMode`：`current | history`
- `selectedRecordVersion`：当前查看的历史记录
- `actionEffectiveDate`：动作区本次写入生效日

约束：

- `selectedRecordVersion` 属于读态；
- `actionEffectiveDate` 属于写态；
- 两者之间不得存在持续同步关系，只允许在动作区打开时进行一次初始化。

### 2. 详情页 URL 约束

冻结规则：

- 详情页 URL 不再长期并存 `as_of + effective_date` 两个读态键。
- 若仍需历史深链接，URL 只表达“选中的历史记录”，不表达写表单默认值。
- `updateSearch(...)` 只能更新读态，不得承担动作表单初始化、重置或改写职责。

### 3. A 类页面写态初始化规则

冻结规则：

- 写表单打开时，由动作规则初始化 `effective_date` 一次；
- 初始化值来源于动作语义，而不是页面当前 `as_of`；
- 用户修改后，页面任何读态变化都不得覆盖已输入值。

说明：

- 若某类动作确需以“当前选中的历史记录”为操作对象，例如更正某条记录，则初始化来源应是“动作目标记录”，而不是页面通用 `as_of`。
- “初始化一次”不等于“强制要求用户手动输入空值”；是否预填由动作规则决定，但该默认值不得与页面查看日期自动绑定。

## 页面实施范围与优先级

### P0：本计划直接覆盖

1. `OrgUnitDetailsPage`
2. `OrgUnitsPage`
3. `OrgUnitFieldConfigsPage`
4. `SetIDGovernancePage`

### P1：按 P0 模式扩展

1. `AssignmentsPage`
2. `PositionsPage`
3. `JobCatalogPage`
4. `DictConfigsPage`

### P2：边界校验与命名优化

1. `DictValueDetailsPage`
2. `SetIDExplainPanel`

## 页面级实施建议

### 1. `OrgUnitDetailsPage`

实施要求：

- 移除详情页“双读态”；
- history 进入方式优先由版本时间轴驱动；
- 动作区打开时按动作语义初始化 `actionEffectiveDate`；
- 历史切换、tab 切换、审计切换不得重置动作区表单。

验收要点：

- 页面长期状态不再同时依赖 `as_of` 与读态 `effective_date`；
- 用户修改过的动作区生效日期不会被历史查看行为覆盖。

### 2. `OrgUnitsPage`

实施要求：

- 默认浏览 current，不常显 `As Of`；
- 新建表单默认值不再继承页面 `as_of`；
- history 模式按需进入，并与新建表单解耦。

验收要点：

- 不再出现 `effectiveDate: asOf` 一类初始化；
- 切换 history/current 不会改动新建表单已输入的 `effective_date`。

### 3. `OrgUnitFieldConfigsPage`

实施要求：

- 默认浏览 current；
- 保存策略后不自动切换查看日期；
- 如需查看未来生效结果，使用显式提示或 CTA。

验收要点：

- 不再出现“保存成功后页面自动跳日”的默认模式。

### 4. `SetIDGovernancePage`

实施要求：

- 浏览区 default current；
- Registry/Explain 仍可保留工具态显式时间；
- Registry 写表单 `effectiveDate` 不再继承浏览区 `as_of`。

验收要点：

- 浏览态与 Registry 写态显式解耦；
- 不再把浏览区 `as_of` 当作 Registry 写表单默认值来源。

## 共享 helper 策略

### 1. 允许抽取的最小纯函数

后续如确有重复，可仅抽取以下纯函数级能力：

- `parseReadMode(...)`
- `resolveHistoryAnchor(...)`
- `resolveExplicitAsOf(...)`
- `deriveInitialActionEffectiveDate(...)`

### 2. 禁止抽取的内容

- 全局时间 store
- `readViewContext`
- 页面级时间状态机
- 同时理解读态、写态、布局、路由跳转的大一统 helper

### 3. 抽取时机

- 至少完成 `OrgUnitDetailsPage` 和一张 A 类页面收口后，再决定是否抽 helper。

## 配套 stopline

后续代码改造时应阻断以下模式继续扩散：

- 新增 page-local `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf`
- 新增 `effectiveDate: asOf`
- 新增 `useEffect(() => setEffectiveDate(asOf), [asOf])`
- 新增保存成功后 `updateSearch({ asOf: ... })` 或等价自动跳日逻辑
- 新增详情页 `as_of + effective_date` 双读态

说明：

- 本计划建议后续补充前端规则扫描或测试门禁，但门禁如何接线不在本计划复制，以 `AGENTS.md`、`Makefile` 与相关质量门禁文档为 SSOT。

## 实施步骤

1. [x] 冻结本计划，作为 `DEV-PLAN-311` 的实施承接文档。
2. [x] 输出 `OrgUnitDetailsPage` 单历史锚点实施说明：状态模型、URL 收口、动作区粘性、验收点。
3. [x] 输出 A 类页面统一收口说明：默认 current、读写解耦、禁止写后自动改读态。
4. [x] 先完成 P0 页面设计评审，再决定是否需要抽共享 helper。
5. [x] 后续实施计划/PR 按本计划顺序承接代码改造与验证。
6. [x] 并行跟踪后端残余 `today` 默认化收口项，避免前端继续保留防御性默认逻辑。

## 实施结果

1. [x] `OrgUnitDetailsPage` 已按“单历史锚点”收口：历史查看统一落到 `effective_date`，旧 `as_of` 仅在页面边界做一次归一化，不再与 `effective_date` 长期并存。
2. [x] `OrgUnitDetailsPage` 动作区写态已与读态解耦：切换历史版本不会覆盖已输入的动作生效日期，写成功后也不会自动跳日。
3. [x] A 类页面读写解耦模板已由后续批次承接：
   - `OrgUnitsPage` 默认 current
   - `OrgUnitFieldConfigsPage` 不再把浏览态作为写态默认值来源
   - `AssignmentsPage` / `PositionsPage` / `JobCatalogPage` / `DictConfigsPage` 已沿用“读态不驱动写态”的统一模式
4. [x] 最小 helper 与门禁已由后续 `DEV-PLAN-315` 承接，但职责边界仍遵循本计划冻结口径：只处理读态解析与显式合同，不承接写态默认值或页面状态机。

## 交付物

1. [x] 一份 `OrgUnitDetailsPage` 单历史锚点实施说明。
2. [x] 一份 A 类页面读写解耦统一约束清单。
3. [x] 一份共享 helper 最小职责边界说明。
4. [x] 一份 P0/P1/P2 页面承接顺序与验收清单。

## 验收标准

- [x] `OrgUnitDetailsPage` 不再长期维持 `as_of + effective_date` 双读态。
- [x] A 类页面不再出现“读态变化自动覆盖写态”的默认模式。
- [x] P0 页面不再出现“写成功后自动切换查看日期/自动跳日”的默认模式。
- [x] 后续若抽共享 helper，其职责仍可在 5 分钟内解释清楚，且不引入新的全局时间框架。
- [x] 新增页面改造讨论默认引用 `DEV-PLAN-311 + DEV-PLAN-312`，不再临时重定义页面时间语义与实施顺序。
- [x] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `AGENTS.md`
