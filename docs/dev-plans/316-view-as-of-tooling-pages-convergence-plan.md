# DEV-PLAN-316：View As Of 工具态页面收口计划——Explain / Release / Governance 子区统一任务态时间语义

**状态**: 草拟中（2026-04-08 03:35 UTC）

## 背景

`DEV-PLAN-311` 已明确 `DEV-PLAN-316` 的定位：

- 收敛 `SetIDExplainPanel` 与治理 / Explain / Release 一类工具态页面；
- 保留显式时间能力，但统一任务态文案与边界隔离规则；
- 防止工具态显式时间再次蔓延回业务浏览页。

在前序计划中，这一定位已经被进一步固化：

1. `DEV-PLAN-314` 已将业务浏览页与工具态能力分轨处理，明确 `DictConfigsPage` 的 release 区不属于浏览主区规则。
2. `DEV-PLAN-315` 已为工具态显式时间建立 allowlist 前提，明确门禁不得误伤被 `316` 明确保留的能力。

因此，`DEV-PLAN-316` 的职责不是删除所有显式时间输入，而是回答三个问题：

1. 哪些页面/组件的显式时间本身就是任务参数，应保留；
2. 这些显式时间如何从技术字段名 `as_of` 收敛为用户可理解的任务态文案；
3. 如何保证工具态显式时间不再外溢回业务浏览页或写态默认值。

## 与 `DEV-PLAN-311/314/315` 的关系

- `DEV-PLAN-311` 是本计划来源与主题 SSOT；本计划不得重定义工具态页面的独立轨道定位。
- `DEV-PLAN-314` 负责业务浏览页的 P1 批量收口；本计划不回退 `314` 已冻结的浏览区规则。
- `DEV-PLAN-315` 负责最小 helper 与反回流门禁；本计划负责界定哪些工具态显式时间可进入 allowlist，以及保留条件。
- `DEV-PLAN-317` 预期承接跨页面回归与验收；本计划只冻结工具态页面的目标形态与验收规则。

## 目标

1. [ ] 识别并冻结当前需要保留显式时间输入的工具态页面/组件清单。
2. [ ] 将工具态显式时间从技术字段名 `as_of` 收敛为任务态文案，如“查看日期 / View As Of”“发布时点”“解释时点”。
3. [ ] 冻结工具态时间与浏览主区、写态默认值之间的隔离规则。
4. [ ] 为 `DEV-PLAN-315` 的 allowlist 与门禁接线提供事实源。

## 非目标

- 不在本计划内把业务浏览页重新改回“默认常显 `As Of`”。
- 不在本计划内引入新的全局时间协议、context、store 或 continuation 机制。
- 不在本计划内处理详情页单历史锚点；该主题已由 `DEV-PLAN-312` 承接。
- 不在本计划内把工具态页面错误扩展成另一套全局时间规则；只冻结工具态局部例外。

## 工具态页面定义

工具态页面/组件满足以下任一条件：

- 显式时间本身就是任务参数，而不是浏览主心智；
- 主要目标是 Explain、Release、治理诊断、同步、发布、冲突预览；
- 用户操作的结果依赖“指定哪个时点执行解释/发布/校验”，而不是“查看当前业务数据”。

对应地，以下页面不属于本计划主对象：

- 业务浏览列表页
- 业务详情页主浏览区
- 写表单的业务生效日期输入

## 当前样本与分类

### 1. 纯工具态组件

#### `SetIDExplainPanel`

现状：

- 组件内部持有显式 `asOf`
- 表单当前直接显示字段名 `as_of`
- 该显式时间本质上是 Explain 请求的任务参数，而非业务浏览时间

结论：

- 保留显式时间输入能力；
- 但文案必须从 `as_of` 收敛为任务态表达，如“解释时点”或“查看日期 / View As Of”；
- 其输入不得回流为宿主页面浏览主区的默认读态。

### 2. 业务页面内的工具态子区

#### `DictConfigsPage` 的 release 区

现状：

- 页面浏览主区持有 `asOf`
- release 表单也持有显式 `as_of`
- release 的 `as_of` 是典型任务态参数，但当前容易与浏览主区时间并置

结论：

- release 区显式时间应保留；
- 但必须在 IA 与文案上明确它属于“发布时点”，而不是列表浏览时间；
- 不允许 release 表单的时间重新定义浏览主区的默认模式。

#### `SetIDGovernancePage` 的 registry / explain 子区

现状：

- 页面整体带有浏览语义；
- 局部 registry / explain 子区仍具有工具态显式时间需求；
- 当前历史上存在浏览区 `as_of` 与 registry `effectiveDate` 串线风险

结论：

- 浏览主区与工具态子区必须分轨；
- registry / explain 子区可保留显式时间输入，但要使用任务态文案；
- 工具态子区时间不得再回流覆盖浏览主区或写态默认值。

### 3. 业务页中的嵌入式工具入口

#### `AssignmentsPage` / `JobCatalogPage` 中嵌入的 `SetIDExplainPanel`

结论：

- 宿主页面仍应遵循 `DEV-PLAN-314` 的 default current；
- 嵌入式 Explain 作为附属工具区，可保留显式时间；
- 但工具区时间只服务于 Explain 请求，不得反过来改写宿主页面读态或写态。

## 核心设计决策

### 决策 1：工具态显式时间保留，但只作为任务参数

冻结规则：

- 工具态显式时间不再被解释为“当前页面主浏览时点”；
- 它只表达本次 Explain / Release / Governance action 的任务参数；
- 工具态请求响应中回显时间是合理的，但不能据此推动浏览主区跳日。

### 决策 2：用户可见文案不再裸露技术字段名

冻结规则：

- 不直接以 `as_of` 作为用户可见 label；
- 优先使用任务态文案：
  - `View As Of / 查看日期`
  - `Explain As Of / 解释时点`
  - `Release As Of / 发布时点`
- 是否双语、是否统一成“查看日期”，由现有 i18n 体系承接；本计划只冻结“不再裸露技术字段名”。

### 决策 3：工具态时间与业务写态时间必须隔离

冻结规则：

- `Explain/Release` 的时间输入不得默认填充业务写表单的 `effective_date / enabled_on / disabled_on`
- 工具态时间变化不得自动重置用户已输入的业务日期
- 工具态请求成功后不得自动推动业务浏览页跳到对应日期

### 决策 4：工具态显式时间可进入 allowlist，但条件必须严格

允许进入 allowlist 的对象应满足：

- 其显式时间是任务参数而不是浏览主心智；
- 已在本计划中登记具体文件/组件与保留理由；
- 已通过任务态文案收口，且不存在写态或浏览态串线。

不允许进入 allowlist 的对象：

- 业务浏览页主区
- 通过“工具页例外”逃避 `DEV-PLAN-314` 收口的业务页

## 统一文案与 IA 规则

### 1. 文案规则

必须收口的模式：

- `label='as_of'`
- 文案直接显示 `as_of: 2026-01-01`

建议替代：

- `查看日期 / View As Of`
- `解释时点`
- `发布时点`
- `治理时点`

### 2. IA 规则

- 工具态显式时间应放在其所属任务区域内部；
- 不作为页面顶层全局浏览筛选器复用；
- 若页面同时存在浏览主区与工具子区，应通过区块标题、说明文案与表单边界清晰地区分两者。

## 实施范围

### P0：首批直接收口对象

1. `apps/web/src/components/SetIDExplainPanel.tsx`
2. `apps/web/src/pages/org/SetIDGovernancePage.tsx` 中的 registry / explain 子区

目标：

- 先完成最典型工具态样板；
- 冻结“保留显式时间 + 任务态文案 + 不回流宿主浏览页”的模式。

### P1：业务页中的工具态区块

1. `apps/web/src/pages/dicts/DictConfigsPage.tsx` 的 release 区
2. `apps/web/src/pages/staffing/AssignmentsPage.tsx` 中嵌入的 `SetIDExplainPanel`
3. `apps/web/src/pages/jobcatalog/JobCatalogPage.tsx` 中嵌入的 `SetIDExplainPanel`

目标：

- 统一嵌入式工具区的时间能力与宿主页边界；
- 确保不反向污染 `DEV-PLAN-314` 已冻结的业务浏览规则。

## 配套 stopline

后续承接代码改造时，应阻断以下模式继续回流：

- 在工具态页面继续裸露 `label='as_of'`
- 工具态显式时间回填业务写态默认值
- 工具态请求成功后自动修改宿主页面浏览 `as_of`
- 以“工具态例外”为名，在业务浏览主区重新常显时间输入

说明：

- 具体 allowlist 与门禁接线由 `DEV-PLAN-315` 承接；
- 本计划只冻结工具态例外的成立条件与边界。

## 实施步骤

1. [ ] 盘点所有当前工具态显式时间输入的页面/组件，并登记“保留理由”。
2. [ ] 先以 `SetIDExplainPanel` 形成首个样板：改任务态文案、明确边界、不改宿主浏览态。
3. [ ] 再收口 `SetIDGovernancePage` 的工具子区，完成浏览主区与工具区分轨。
4. [ ] 最后收口 `DictConfigsPage` release 区及嵌入式 Explain 区，统一宿主页/工具区边界。
5. [ ] 为 `DEV-PLAN-315` 输出 allowlist 候选清单与保留理由。

## 测试与覆盖率

覆盖率与门禁口径以仓库 SSOT 为准：

- 入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
- 前端测试与分层导向：`DEV-PLAN-300`、`DEV-PLAN-301`

本计划要求的测试重点：

- 工具态显式时间保留时，文案已从技术字段名收口为任务态表达；
- 工具态时间变化不会覆盖宿主页面写态日期；
- 工具态请求成功不会自动改写宿主页面浏览态；
- allowlist 仅覆盖本计划登记的工具态对象，不误放业务浏览主区。

测试组织要求：

- 优先测试可提纯的小转换器、小文案映射、小边界函数；
- 新增工具态页面测试默认按清晰的行为簇或表驱动场景组织，优先围绕“任务态文案收口、宿主页边界隔离、allowlist 命中/拒绝”三类稳定行为，而不是按页面零散分支补点；
- 仅在这些直接测试无法覆盖关键用户行为时，再补页面级交互测试；
- 新增测试应并入现有正式测试入口，不得新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类补洞式文件。

## 交付物

1. [ ] 一份工具态显式时间 allowlist 候选清单。
2. [ ] 一份任务态文案与 IA 规则说明。
3. [ ] 一份宿主页/工具区边界与禁止串线清单。
4. [ ] 一组工具态样板测试与回归说明。

## 验收标准

- [ ] `SetIDExplainPanel` 不再裸露 `as_of` 作为用户可见主标签。
- [ ] `DictConfigsPage` release 区显式时间保留，但不会重新定义浏览主区模式。
- [ ] `SetIDGovernancePage` 的工具子区与浏览主区完成分轨，不再互相覆盖时间状态。
- [ ] 工具态显式时间能力可被 `DEV-PLAN-315` 以最小 allowlist 方式保留。
- [ ] 工具态例外不会外溢成业务浏览页的通用规则。
- [ ] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `docs/dev-plans/314-view-as-of-p1-pages-batch-cutover-plan-assignments-positions-jobcatalog-dicts.md`
- `docs/dev-plans/315-view-as-of-minimal-helper-and-anti-regression-gates-plan.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- `AGENTS.md`
