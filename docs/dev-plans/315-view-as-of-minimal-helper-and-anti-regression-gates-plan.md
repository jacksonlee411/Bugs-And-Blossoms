# DEV-PLAN-315：View As Of 最小 helper 与反回流门禁计划

**状态**: 草拟中（2026-04-08 03:10 UTC）

## 背景

`DEV-PLAN-311` 已为 `DEV-PLAN-315` 预留明确主题：

- 在 `DEV-PLAN-312` 首批页面完成后，评估并抽取最小纯函数 helper；
- 增加前端反模式门禁，阻断 `todayISO()` / `parseDateOrDefault()` / `effectiveDate: asOf` 等模式回流。

截至当前，前端时间语义收口已经形成三层前置事实：

1. `DEV-PLAN-312` 已冻结“共享 helper 延后抽取、且只允许最小纯函数”的边界。
2. `DEV-PLAN-314` 已冻结 P1 A 类页面的统一减法模板，并明确其只为 `DEV-PLAN-315` 提供真实重复样本，不在 `314` 内提前造层。
3. `org` 页面已经存在一组低复杂度样板：
   - `apps/web/src/pages/org/readViewState.ts`
   - `apps/web/src/pages/org/orgReadNavigation.ts`

与此同时，仓库里仍存在可量化的前端时间反模式样本：

- `AssignmentsPage` / `PositionsPage` / `JobCatalogPage` 仍保留 page-local `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf`
- 多页仍存在 `effectiveDate` 与 `asOf` 的直接初始化或持续同步
- 工具态页面如 `SetIDExplainPanel` 仍显式暴露 `as_of`，需要后续与业务浏览页门禁区分对待

本计划用于回答两个后置问题：

1. 在哪些重复样本已经成立后，才允许抽最小 helper；
2. 抽完之后，如何用轻量门禁阻断旧模式重新长回来。

## 与 `DEV-PLAN-311/312/314/316` 的关系

- `DEV-PLAN-311` 是本计划的来源与定位 SSOT；本计划不得重新定义“helper 延后抽取”的原则。
- `DEV-PLAN-312` 是首批实施模板 SSOT；本计划只承接其后置抽象与反回流部分。
- `DEV-PLAN-314` 提供 P1 批量页面的真实重复样本；本计划应在这些页面至少完成首轮收口后再启动抽象。
- `DEV-PLAN-316` 负责工具态页面收口；本计划的门禁不得误伤被 `316` 明确保留的工具态显式时间能力。

## 目标

1. [ ] 基于已完成的页面收口结果，评估并抽取最小前端纯函数 helper。
2. [ ] 冻结 helper 的允许范围、禁止范围与命名边界，防止演化为新时间框架。
3. [ ] 增加轻量反回流门禁，阻断 page-local 时间默认化与读写串线模式重新扩散。
4. [ ] 为后续页面持续收口提供可复用、可解释、可测试的最小前端基建。

## 非目标

- 不在本计划内提前推动 `DEV-PLAN-314` 的页面实现；页面减法仍由 `312/314` 承接。
- 不在本计划内引入全局时间 store、context、状态机或跨页 continuation envelope。
- 不在本计划内把写态初始化、dirty guard、布局状态、查询缓存与路由跳转强行揉成一个“大一统 helper”。
- 不在本计划内对工具态页面一刀切删除显式时间能力；是否保留由 `DEV-PLAN-316` 判定。

## 启动前提

本计划必须满足以下前提后方可进入实施：

1. [ ] `DEV-PLAN-312` 的 `OrgUnitDetailsPage` 样板页已完成首轮收口，且未再回退到双读态。
2. [ ] `DEV-PLAN-314` 至少完成一组真实重复样本页面的收口，优先是：
   - `AssignmentsPage`
   - `PositionsPage`
3. [ ] 已能证明重复样本不是“看起来相似”，而是至少在两个页面中出现了相同的纯函数需求。

说明：

- 若上述前提不满足，则维持页面内实现，禁止为了“看起来更整洁”而提前抽象。

## 现状证据

### 1. 仓库中已存在可复用样板，但作用域仍局限于 `org`

当前样板：

- `readViewState.ts`
- `orgReadNavigation.ts`

它们已经证明以下能力可以被提纯为小函数：

- `current/history` 解析
- `as_of` 是否出现在 URL 中
- 可选读态参数的去空与拼装

结论：

- `315` 不需要从零设计 helper；
- 更自然的方向是评估：这些样板是否可以以更中性的命名与作用域承接 A 类页面的共同需求。

### 2. 重复反模式已出现，且集中于 P1 页面

当前重复样本包括：

- page-local `todayISO()`
- page-local `parseDateOrDefault()`
- page-local `fallbackAsOf`
- `effectiveDate` 初始化直接取 `asOf`
- `useEffect(() => setEffectiveDate(asOf), [asOf])`

结论：

- 这些反模式已经具备被门禁阻断的价值；
- 但门禁必须建立在 `312/314` 已确认的正确替代方案之上，不能只会“报错”，不会引导。

## 允许抽取的 helper 范围

### 1. 读态解析 helper

允许范围：

- `isDay(...)`
- `parseRequestedAsOf(...)`
- `resolveReadViewState(...)`
- `parseReadMode(...)`
- `resolveHistoryAnchor(...)`

职责边界：

- 只处理 current/history 读态解析；
- 只处理 `as_of` 缺失、非法、历史模式判断；
- 不处理写态默认值；
- 不直接发请求；
- 不感知页面布局、query cache 或 dialog 开关状态。

### 2. 读态导航 helper

允许范围：

- “history 模式才带 `as_of`” 的 URL 参数构建
- 可选读态参数的 trim / omit-empty 逻辑
- current/history 页面间跳转时的最小 search params 生成

职责边界：

- 只负责导航参数表达；
- 不做页面业务判断；
- 不负责表单初始化；
- 不负责写成功后的跳转策略。

### 3. 允许存在但必须保持页面内的逻辑

以下逻辑即使出现重复，也**不得**在本计划内抽成通用 helper：

- 写表单 `effective_date / enabled_on / disabled_on` 默认值策略
- dirty guard / “用户已修改则不覆盖” 逻辑
- dialog 打开关闭状态
- query cache invalidation 规则
- 页面布局、提示文案、CTA 触发

原因：

- 它们仍带有明显动作语义或页面语义；
- 过早提取会把局部复杂度升级成跨页复杂度。

## 明确禁止的 helper 方向

- `readViewContext`
- 全局时间 store
- `timeAnchor` / `lastTimeAnchor`
- 同时理解读态、写态、布局、路由、缓存、表单默认值的“大一统 helper”
- “用户没选日期就自动 today”的默认化工具
- “写成功后自动跳到某天”的导航辅助器

## 候选 helper 清单

本计划建议仅评估以下最小候选项：

1. [ ] `readViewState` 通用化
   - 从 `org` 范围提升为更通用的页面读态解析 helper
2. [ ] `readNavigation` 通用化
   - 将“current 不带 `as_of` / history 才带 `as_of`”提纯成更通用的 search params builder
3. [ ] `trim / omit-empty` 一类 URL 参数纯函数
   - 仅当已在两个以上页面出现相同写法时才抽

不建议列入首批候选：

- 写态初始化 helper
- dialog default date helper
- mutation 成功后导航 helper

## 反回流门禁范围

### 1. 必须阻断的反模式

- 新增 page-local `todayISO()`
- 新增 page-local `parseDateOrDefault()`
- 新增 page-local `fallbackAsOf`
- 新增 `effectiveDate: asOf`
- 新增 `useEffect(() => setEffectiveDate(asOf), [asOf])`
- 新增写成功后 `updateSearch({ asOf: ... })` 或等价自动跳日逻辑

### 2. 门禁表达方式

允许的接线方式：

- 前端测试中的反模式扫描
- lint/grep 规则
- 受控 allowlist + CI 校验

说明：

- 具体接线由实现阶段决定；
- 本计划只冻结“哪些模式必须被阻断”，不复制门禁脚本实现。

### 3. Allowlist 原则

工具态页面若被 `DEV-PLAN-316` 明确保留显式时间能力，可进入受控 allowlist，例如：

- `SetIDExplainPanel`
- release / explain / diagnostics 一类明确以时间为任务参数的页面或组件

要求：

- allowlist 必须最小化；
- 必须显式登记文件路径与保留理由；
- 不得把业务浏览页放进 allowlist 逃避收口。

## 实施步骤

1. [ ] 确认 `312/314` 的前置页面已形成真实重复样本。
2. [ ] 盘点现有 `org` 样板与 P1 页面重复逻辑，输出“可抽 / 不可抽”分类表。
3. [ ] 抽取首批最小纯函数 helper，并保持命名与职责边界最小化。
4. [ ] 为 helper 增加直接测试，优先围绕纯函数与导航参数构建行为。
5. [ ] 接入轻量反模式门禁，先阻断最明确的回流模式。
6. [ ] 为工具态显式时间建立最小 allowlist，避免误伤 `DEV-PLAN-316` 范围。
7. [ ] 形成“替代路径”说明：发现旧模式时，开发者应改用什么，而不是只看到门禁报错。

## 测试与覆盖率

覆盖率与门禁口径以仓库 SSOT 为准：

- 入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
- 前端测试与分层导向：`DEV-PLAN-300`、`DEV-PLAN-301`

本计划要求的测试重点：

- 对最小 helper 优先做直接测试，而不是通过页面间接覆盖；
- current/history 解析、`as_of` 去留、可选参数 trim/omit-empty 等纯函数行为应有稳定单测；
- 反模式门禁应有最小自测，保证能捕获明确禁止的模式；
- allowlist 逻辑若存在，也应有最小回归覆盖，避免误伤或静默放宽。

测试组织要求：

- 前端测试优先围绕可提纯的小函数、小状态机、小转换器；
- 仅在纯函数无法覆盖关键用户行为时，再补页面级交互测试；
- 新增测试应并入现有正式测试入口，不得新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类补洞式文件。

## 交付物

1. [ ] 一份最小 helper 候选清单与“可抽/不可抽”分类表。
2. [ ] 一份 helper 职责边界说明与命名约束。
3. [ ] 一份前端反模式门禁清单与 allowlist 规则说明。
4. [ ] 一组 helper 直接测试与门禁自测说明。

## 验收标准

- [ ] 首批 helper 仍可在 5 分钟内解释清楚，不引入新的时间框架或全局状态层。
- [ ] `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf` 等旧模式在业务浏览页不再新增。
- [ ] `effectiveDate: asOf` 与 `setEffectiveDate(asOf)` 一类读写串线模式被门禁阻断。
- [ ] 工具态显式时间能力可以通过最小 allowlist 保留，不会误伤 `DEV-PLAN-316` 范围。
- [ ] helper 测试优先是直接测试，而不是依赖大而重的页面交互脚本间接覆盖。
- [ ] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `docs/dev-plans/312-view-as-of-implementation-plan-details-single-history-anchor-and-a-pages-read-write-decoupling.md`
- `docs/dev-plans/314-view-as-of-p1-pages-batch-cutover-plan-assignments-positions-jobcatalog-dicts.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- `AGENTS.md`
