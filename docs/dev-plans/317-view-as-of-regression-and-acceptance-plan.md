# DEV-PLAN-317：View As Of 页面时间语义回归与验收计划

**状态**: 已完成（2026-04-09 CST）

## 背景

`DEV-PLAN-311` 已将 View As Of 专项拆分为多份实施子计划：

- `DEV-PLAN-312`：详情页单历史锚点与首批 A 类页面读写解耦
- `DEV-PLAN-313`：后端显式日期合同、无 fallback、统一错误语义
- `DEV-PLAN-314`：P1 A 类页面批量收口
- `DEV-PLAN-315`：最小 helper 与反回流门禁
- `DEV-PLAN-316`：工具态页面 / 子区收口

到这一步，缺口不再是“再写一份原则文档”，而是需要一份统一验收计划，把这些子计划沉淀为**稳定可回归的证据**：

1. 页面是否真正实现了 `default current`；
2. history 是否不再改写写态；
3. 写成功后是否不再自动跳日；
4. 工具态显式时间是否保留在任务区，而没有回流业务浏览页；
5. 浏览器到后端的显式日期合同是否仍然成立。

本计划用于定义这套跨页面、跨层级的回归矩阵与验收证据，作为 `311` 专项的统一收尾计划。

## 与 `DEV-PLAN-311/312/313/314/315/316` 的关系

- `DEV-PLAN-311` 是本计划的主题来源与上位 SSOT；本计划不得重新定义页面分类。
- `DEV-PLAN-312`、`314`、`316` 提供前端收口对象与行为边界；本计划负责验证这些边界是否稳定成立。
- `DEV-PLAN-313` 提供后端显式日期合同与错误语义；本计划负责验证前端收口后，这些合同仍然成立。
- `DEV-PLAN-315` 提供 helper 与反回流门禁；本计划负责把门禁效果纳入回归范围。

## 目标

1. [x] 建立跨页面统一回归矩阵，覆盖 `default current`、history、写态隔离、工具态例外四类核心行为。
2. [x] 建立分层验收策略：最小直接测试、页面级交互测试、跨页面端到端验收各自负责什么。
3. [x] 冻结“专项完成”的证据标准，避免只靠单页口头验收判断收口完成。
4. [x] 为后续回归与门禁提供统一事实源，而不是每个页面单独维护一套散点验收说明。

## 非目标

- 不在本计划内重新实现页面收口逻辑；逻辑本身由 `312/314/316` 承接。
- 不在本计划内重写后端日期合同；合同本身由 `313` 承接。
- 不在本计划内创建新的时间抽象层、全局状态层或大一统测试 harness。
- 不在本计划内扩大为全站通用回归计划；仅覆盖 View As Of 专项相关页面与合同。

## 核心验收原则

### 1. 页面默认语义必须回到 `default current`

验收解释：

- 用户首次进入业务浏览页时，不应被迫先理解或操作 `As Of`；
- current 模式下，浏览主区 URL 不要求持有 `as_of`；
- 只有进入 history 模式时，才显式出现时间锚点。

### 2. history 只改变“当前在看什么”，不改变“本次准备写什么”

验收解释：

- 切换 history / current、切换历史记录、切换 tab，不得覆盖已输入的写态日期；
- 写表单日期只由动作语义初始化一次；
- 一旦用户修改，读态变化不得自动同步覆盖。

### 3. 写结果可见，但不自动改读态

验收解释：

- 保存成功后可以刷新列表、显示 toast、提供 CTA；
- 但不能自动跳到某个 `as_of`、自动切 history、自动更新浏览主区日期。

### 4. 工具态显式时间只服务于任务区

验收解释：

- `Explain / Release / Governance diagnostics` 的显式时间可保留；
- 但只作为任务参数；
- 不得重新支配业务浏览主区或业务写态默认值。

### 5. 页面收口不能破坏后端显式日期合同

验收解释：

- 虽然产品层 current 可不在 URL 显示 `as_of`；
- 但一旦页面进入受时间切片影响的读请求，边界层仍应向后端形成显式日期参数；
- 后端 `invalid_as_of` / `invalid_effective_date` 等合同不应被前端收口绕开或重新模糊化。

## 验收对象矩阵

### A. P0 / 样板页

1. `OrgUnitDetailsPage`
2. `OrgUnitsPage`
3. `OrgUnitFieldConfigsPage`
4. `SetIDGovernancePage`

重点验收：

- 单历史锚点是否成立
- 浏览主区与写态是否分离
- 浏览主区与工具态子区是否分轨

### B. P1 A 类页面

1. `AssignmentsPage`
2. `PositionsPage`
3. `JobCatalogPage`
4. `DictConfigsPage`

重点验收：

- default current 是否成立
- `as_of -> effective_date/enabled_on/disabled_on` 串线是否已切断
- 写成功后是否仍会自动跳日

### C. 工具态页面 / 子区

1. `SetIDExplainPanel`
2. `SetIDGovernancePage` 的 registry / explain 子区
3. `DictConfigsPage` 的 release 区
4. 嵌入在业务页中的 Explain 区

重点验收：

- 任务态文案是否已收口
- 工具态显式时间是否仍停留在任务区
- allowlist 是否仅覆盖这些对象

## 分层回归策略

### 1. 最小直接测试

适用对象：

- `readViewState`
- `readNavigation`
- helper / allowlist / 文案映射 / 反模式门禁扫描
- 写态初始化与 dirty guard 一类可提纯小逻辑

负责验证：

- current/history 解析
- `as_of` 的有无与 trim/omit-empty
- 工具态对象是否命中 allowlist
- 任务态文案映射是否正确

### 2. 页面级交互测试

适用对象：

- 单页内部存在关键用户行为，无法被纯函数直接覆盖时

负责验证：

- current 模式初始进入行为
- history 切换不改写写态
- dialog / form 在页面内的日期粘性
- 工具区操作不回流宿主页

### 3. 跨页面端到端验收

适用对象：

- 需要同时验证前端行为、路由、请求参数与后端合同的路径

负责验证：

- 浏览区 default current 与下游显式日期合同同时成立
- 工具态显式时间请求能够成功执行，但不改写业务浏览态
- 写成功后仍不自动跳日

## 核心回归场景

### 场景 1：业务浏览页 default current

断言：

- 首次进入页面时，浏览主区不默认常显 `As Of`
- current 模式 URL 不强制带 `as_of`
- 页面仍能加载 current 内容

适用页：

- `OrgUnitsPage`
- `AssignmentsPage`
- `PositionsPage`
- `JobCatalogPage`
- `DictConfigsPage`

### 场景 2：进入 history 后只影响读态

断言：

- 用户进入 history 模式后，当前查看内容发生变化；
- 已输入的写态日期不变；
- history 不自动覆盖 form 已修改值。

适用页：

- `OrgUnitDetailsPage`
- `AssignmentsPage`
- `PositionsPage`
- `JobCatalogPage`

### 场景 3：写成功后不自动跳日

断言：

- mutation 成功后，query 可刷新；
- 允许 toast / CTA；
- 但不自动改 `as_of`、不自动切 history、不中途改浏览主区状态。

适用页：

- `OrgUnitFieldConfigsPage`
- `AssignmentsPage`
- `PositionsPage`
- `JobCatalogPage`
- `DictConfigsPage`

### 场景 4：工具态显式时间保留，但不回流宿主

断言：

- 工具态区可保留显式时间输入；
- 文案已从 `as_of` 收口为任务态表达；
- 工具态提交成功后，宿主页浏览态与写态不被覆盖。

适用页 / 组件：

- `SetIDExplainPanel`
- `SetIDGovernancePage` registry / explain 子区
- `DictConfigsPage` release 区

### 场景 5：前端收口后，后端显式日期合同仍成立

断言：

- 进入 history 或工具态请求时，浏览器到后端的请求仍形成显式日期参数；
- 缺失/非法日期请求仍会命中稳定错误合同；
- 前端不会再通过 page-local fallback 掩盖后端错误。

适用接口：

- Org 读接口
- JobCatalog 读接口
- Staffing 读接口
- SetID Explain / Governance / Release 接口

## 已落地回归基线

### 1. 最小直接测试

- [x] `readViewState`：`apps/web/src/pages/org/readViewState.test.ts`
- [x] `readNavigation`：`apps/web/src/pages/org/orgReadNavigation.test.ts`
- [x] 工具态 Explain 文案与 `initialAsOf` 透传：`apps/web/src/components/SetIDExplainPanel.test.tsx`

### 2. 页面级交互测试

- [x] `OrgUnitsPage`：`default current` 与 history 显式 `as_of`
- [x] `OrgUnitDetailsPage`：单历史锚点、写态日期 sticky、写后不跳日
- [x] `OrgUnitFieldConfigsPage`：当前模式不显式 `as_of`、跳转 SetID Registry 时 current/history 分流
- [x] `AssignmentsPage` / `PositionsPage`：history 不改写 `effective_date`
- [x] `JobCatalogPage`：default current 与 create dialog 不继承 history `as_of`
- [x] `DictConfigsPage`：default current、history URL、release 工具态时间不回流浏览态
- [x] `SetIDGovernancePage`：registry 日期不跟随浏览态串线、工具态日期标签收口

### 3. 门禁 / 合同验证

- [x] `scripts/ci/check-as-of-explicit.sh`
- [x] `scripts/ci/check-view-as-of-frontend.sh`
- [x] 工具态 allowlist 仅保留 `SetIDExplainPanel`，且注释已与 `DEV-PLAN-316` 对齐

## 门禁与证据

### 1. 反回流门禁验收

本计划应将以下门禁效果纳入回归：

- `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf` 不再在业务浏览页新增
- `effectiveDate: asOf` / `setEffectiveDate(asOf)` 不再新增
- 工具态 allowlist 仅覆盖经 `316` 明确登记的对象

### 2. 证据形式

允许的证据包括：

- 直接测试通过
- 页面级交互测试通过
- 关键 E2E 路径通过
- 受影响请求的网络参数与响应错误合同验证

不建议的证据形式：

- 只截图不验证行为
- 只人工口述“看起来已经改了”
- 只依赖单页局部断言，不覆盖跨页边界

## 实施步骤

1. [x] 汇总 `312/314/316` 的页面与组件验收点，形成统一回归矩阵。
2. [x] 将回归矩阵按“最小直接测试 / 页面交互测试 / E2E”三层分配责任。
3. [x] 为 P0 / P1 / 工具态对象分别建立最小必过场景集。
4. [x] 将 `313` 的后端显式日期合同验证并入前端专项验收，而不是独立悬空。
5. [x] 将 `315` 的反回流门禁与 allowlist 验证纳入回归基线。
6. [x] 输出专项完成判定标准：哪些场景通过后，`311` 可视为分解计划已进入可验收状态。

## 测试与覆盖率

覆盖率与门禁口径以仓库 SSOT 为准：

- 入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
- 测试分层导向：`DEV-PLAN-300`、`DEV-PLAN-301`

本计划要求的测试重点：

- 优先用最小直接测试承接可提纯逻辑与门禁扫描；
- 页面级交互测试只覆盖关键用户行为，不用来替代所有小逻辑测试；
- E2E 只承担跨页、跨层合同验收，不承接本可在更小边界稳定验证的逻辑；
- 所有新增测试都应围绕稳定行为组织，而不是围绕 coverage 缺口补点。

测试组织要求：

- 新增测试默认按清晰的行为簇或表驱动场景组织；
- 不得新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类补洞式文件；
- 前端测试仍遵循“职责下沉、最小边界、避免补洞式堆叠”的原则。

## 交付物

1. [x] 一份跨页面回归矩阵。
2. [x] 一份按层次划分的验收责任表（直接测试 / 页面交互测试 / E2E）。
3. [x] 一份专项完成判定清单。
4. [x] 一份关键证据记录模板，供后续执行记录承接。

## 验收标准

- [x] `default current`、history 不改写写态、写后不跳日、工具态不回流宿主等核心行为已有统一回归矩阵覆盖。
- [x] `312/314/316` 的关键页面与组件均已映射到明确的验收层级，而不是重复或遗漏。
- [x] `313` 的后端显式日期合同已被纳入前端专项验收，而非独立悬空。
- [x] `315` 的反回流门禁与 allowlist 效果已有对应回归验证。
- [x] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/archive/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `docs/dev-plans/312-view-as-of-implementation-plan-details-single-history-anchor-and-a-pages-read-write-decoupling.md`
- `docs/archive/dev-plans/313-view-as-of-backend-parallel-convergence-plan-explicit-date-contract-and-no-fallback.md`
- `docs/archive/dev-plans/314-view-as-of-p1-pages-batch-cutover-plan-assignments-positions-jobcatalog-dicts.md`
- `docs/dev-plans/315-view-as-of-minimal-helper-and-anti-regression-gates-plan.md`
- `docs/archive/dev-plans/316-view-as-of-tooling-pages-convergence-plan.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/archive/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- `AGENTS.md`
