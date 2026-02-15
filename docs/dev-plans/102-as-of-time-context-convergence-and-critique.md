# DEV-PLAN-102：全项目 as_of 时间上下文收敛与批判（承接 DEV-PLAN-076）

**状态**: 已完成（2026-02-14 — 路由矩阵冻结 + 去壳层全局 as_of + MUI Org 整改 + 测试/证据）

## 1. 背景与问题陈述
- 本计划承接 `DEV-PLAN-076` 对 OrgUnit 的问题识别：当“树日期”“详情版本”“全局页面日期”混用时，会出现丢焦、全页刷新、版本列表跳变等稳定性问题。
- 当前仓库中，`as_of` 在不同层承担了不同语义：
  1) DB/领域查询时点（valid-time slice）；
  2) UI 壳层全局日期上下文；
  3) 某些页面默认写入日期的回填来源。
- 该“一个参数多重语义”已导致跨模块契约漂移，典型表现：
  - 同为 Org 模块，`/org/nodes` 走 `tree_as_of + effective_date`，而 MUI 页仍走 `as_of + effective_date`；
  - 壳层 Topbar 统一产出 `as_of`，与局部页面“拒绝/废弃 as_of”的策略冲突；
  - 默认值算法在不同路由存在差异（系统日、最近可用、最早回退等）。
- 由于仓库正在推进“UI 收敛为 MUI X SPA、移除 Astro/HTMX”（见 `DEV-PLAN-103`），本计划需要同时给出：
  - **过渡口径**：旧壳层仍存在时，禁止 Topbar 继续强灌/改写页面时间上下文；
  - **最终口径**：删除旧壳层后，时间上下文完全由页面（MUI 路由）自管理。

## 2. 目标与非目标

### 2.1 核心目标
- [ ] 冻结“时间上下文词汇表”与参数职责，避免继续新增同义/歧义参数。
- [ ] 形成“按路由分类”的时间参数契约矩阵（单上下文 / 双上下文 / 无时间上下文）。
- [ ] 收敛 Shell/Topbar 与页面参数的耦合，消除“全局 as_of 强灌”导致的冲突。
- [ ] 禁止“伪版本注入”类实现（版本列表只能来自真实版本集合）。
- [ ] 统一默认日期策略与错误码口径（缺失/非法参数、默认回退、重定向行为）。
- [ ] 建立门禁与测试断言，阻断 as_of 语义回漂。

### 2.2 非目标
- 不改变 `DEV-PLAN-032` 已冻结的日粒度有效期语义（Valid Time 仍为 `date`）。
- 不在本计划直接引入新的业务模块或导航结构。
- 不通过 legacy 双链路实现兼容（遵守 `DEV-PLAN-004M1` No Legacy）。

## 2.3 SSOT 引用
- 时间语义：`docs/dev-plans/032-effective-date-day-granularity.md`
- OrgUnit 双上下文基线：`docs/dev-plans/076-orgunit-version-switch-selection-retention.md`
- OrgUnit 现有契约：`docs/dev-plans/073-orgunit-crud-implementation-status.md`
- Routing 与 CI 门禁：`docs/dev-plans/017-routing-strategy.md`、`docs/dev-plans/012-ci-quality-gates.md`
- 仓库规则入口：`AGENTS.md`
- UI 收敛为 MUI-only：`docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`

## 3. 现状盘点（2026-02-14）

### 3.1 as_of 现状矩阵
| 层级 | 当前参数 | 当前语义 | 代表路径/能力 | 主要问题 |
| --- | --- | --- | --- | --- |
| DB Kernel / SQL 函数 | `p_as_of` / `p_as_of_date` | 查询某日有效切片 | `validity @> as_of`、SetID/Scope/引用校验 | 语义相对清晰，但命名与 UI 语义重名 |
| Astro Shell + Topbar | `as_of` | 全局页面上下文（导航/顶栏） | `/ui/nav` `/ui/topbar` `/app?as_of=...` | 对所有页面一刀切注入，和局部页面契约冲突；且与 MUI-only 方向不一致（见 `DEV-PLAN-103`） |
| Org HTMX 页面 | `tree_as_of` + `effective_date` | 树时点 + 详情版本 | `/org/nodes*` | 已较清晰，但仍受壳层 `as_of` 外溢影响 |
| Org MUI 页面 | `as_of` + `effective_date` | **视图日期**（`as_of`，等价于 tree_as_of 概念）+ **详情版本**（`effective_date`；URL query param 可缺省：例如从列表进入详情默认不带，详情页会根据 `as_of + versions` 算法选中默认版本） | `/app/org/units*` | 名称与 HTMX 不一致（tree_as_of vs as_of）；若把 as_of 误当成版本会引入“伪版本/跳变”风险 |
| 其他 UI 页面（SetID/JobCatalog/Staffing/Person） | `as_of` | 单查询时点 | `/org/setid` `/org/job-catalog` `/org/positions` `/person/persons` | 部分页面业务上并不需要强制全局 as_of |

### 3.2 问题分型（批判）
1. **语义过载**：`as_of` 同时扮演“查询时点 + UI全局状态 + 默认写入来源”。
2. **分层泄漏**：壳层参数策略覆盖页面局部契约，导致路由间冲突。
3. **默认值不统一**：不同 handler 对缺省日期采用不同算法，行为不可预测。
4. **同域异构**：Org 在 HTMX 与 MUI 两套参数口径并存，增加维护和测试复杂度。
5. **可观测性不足**：缺少统一的“时间参数契约基线”与自动化门禁，回漂风险高。

### 3.3 术语澄清（避免误读）
- 本计划中提到的“Org MUI 列表列 effectiveDate/Effective Date”，指 MUI OrgUnits 列表页 DataGrid 的字段/列名；当前实现把“视图日期（as_of）”填到该列，因此它不是“记录版本生效日（effective_date）”。
- 本计划中提到“effective_date 可缺省”，是指 **URL query param 可缺省**（页面仍会基于 `as_of + versions` 选择并高亮一个默认版本），而不是说“系统没有版本/没有生效日”。
- 本计划中提到的“批量启用/停用”，指当前页面出现的 UI 元素与前端批量调用路径；它不代表该能力已经作为业务功能交付/冻结，因此需要移除以避免“僵尸功能”。

## 4. 目标契约（拟冻结）

### 4.1 时间上下文词汇表
- `as_of`：**查询时点**（读模型切片语义），仅用于“单上下文页面/API”。
- `tree_as_of`：**树/层级视图时点**，仅影响树、懒加载、树搜索。
- `effective_date`：**记录版本生效日**，仅用于版本选择/写入生效日。

### 4.2 不变量
- [ ] 页面若为双上下文，必须显式分离参数；禁止用 `as_of` 代替 `effective_date`。
- [ ] 禁止构造“伪版本”用于填充版本列表；版本列表只能来源真实 versions。
- [ ] URL 参数必须与页面职责一致，禁止“壳层统一注入 + 页面本地反向拒绝”的对冲实现。
- [ ] 缺失/非法日期的行为（302/400）需在路由契约中逐条冻结。
- [ ] 禁止把“视图日期（as_of/tree_as_of）”伪装为“记录版本生效日（effective_date）”展示在 UI（例如列表列名/文案误导）。

### 4.3 路由分类（计划内要产出的契约清单）
- A 类：单上下文（保留 `as_of`）。
- B 类：双上下文（`tree_as_of + effective_date`）。
- C 类：无时间上下文（不应强制日期参数）。

### 4.4 建议落地口径（最省事且不回退，拟冻结）
> 本节描述“最小变更但能阻断回漂”的落地策略；具体实现拆分按里程碑执行。

1) **移除壳层全局日期选择器**：Topbar 不再提供/提交 `as_of`，也不再主动改写当前页面的时间参数。  
2) **URL 仍可分享**：保留“URL 可带 `as_of`（可分享/可复现）”，缺省按当天（UTC）回退。  
3) **视图页自带视图日期**：对“按时点浏览”的列表/树视图，在页面内提供“视图日期”控件（Org MUI 列表页已具备）。  
4) **配置页不强制视图日期**：SetID/职位分类等配置型页面，优先通过“版本/生效日期（effective_date）”浏览历史与未生效记录；如确需 `as_of` 切片语义，必须显式定义（不能依赖壳层全局 as_of）。  

### 4.5 路由-时间参数矩阵（冻结）
> 目标：把“缺省行为/错误码/重定向”固化为可测试的基线，阻断语义回漂。

| 路由 | 分类 | 时间参数 | 缺省行为 | 非法/冲突行为 | 备注 |
| --- | --- | --- | --- | --- | --- |
| `/ui/nav` | C（无时间上下文） | 无 | 200 | 忽略 `as_of` | 壳层不再注入日期 |
| `/ui/topbar` | C | 无 | 200 | 忽略 `as_of` | Topbar 不再提供全局日期选择器 |
| `/ui/flash` | C | 无 | 200 | - | |
| `/app`、`/app/*` | C | 无 | 200（SPA index） | - | MUI SPA 自管理 URL |
| `/login`、`/logout`、`/lang/*` | C | 无 | 200/302 | - | |
| `/org/nodes` (GET) | B（双上下文） | `tree_as_of`（树） | 缺省/非法：**302** 补齐 `tree_as_of=<resolved>` | `as_of`：400 `deprecated_as_of` | `resolved` 算法见 `resolveTreeAsOfForPage` |
| `/org/nodes` (POST) | B | `tree_as_of`（树） | 缺省：400 `invalid_tree_as_of` | `as_of`：400 `deprecated_as_of` | 写入回跳仍带 `tree_as_of` |
| `/org/nodes/children` | B | `tree_as_of`（必填） | 缺省：400 `invalid_tree_as_of` | 非法：400 `invalid_tree_as_of` | |
| `/org/nodes/search` | B | `tree_as_of`（必填） | 缺省：400 `invalid_tree_as_of` | 非法：400 `invalid_tree_as_of` | `format=panel` 仍遵循同口径 |
| `/org/nodes/details`、`/org/nodes/view` | B | `effective_date`（可缺省）、`tree_as_of`（可缺省） | 缺省：回退 `currentUTCDateString()`（不重定向） | `as_of`：400 `deprecated_as_of`；`effective_date` 非法：400 `invalid_effective_date` | 详情版本与树日期解耦 |
| `/org/snapshot`、`/org/setid`、`/org/job-catalog`、`/org/positions`、`/org/assignments`、`/person/persons` | A（单上下文） | `as_of` | GET/HEAD 缺省：**302** 补齐 `as_of=<currentUTCDateString()>` | 非法：400 `invalid_as_of` | 统一走 `requireAsOf` |
| `/org/api/org-units`、`/org/api/org-units/search` | A | `as_of` | 缺省：回退为 UTC 今日（不重定向） | 非法：400 `invalid_as_of` | 内部 API：A 类切片语义 |
| `/org/api/org-units/versions`、`/org/api/org-units/audit` | C | 无 | 200 | - | 版本列表只来源真实版本集合 |

### 4.6 跨层语义映射（UI ↔ 服务 ↔ SQL）
| 词汇 | UI/URL 语义 | 服务层入口（示例） | SQL/Kernel 语义（示例） | 备注 |
| --- | --- | --- | --- | --- |
| `as_of` | 查询时点（读模型切片） | `requireAsOf(...)`；`/org/api/org-units?as_of=...` | `validity @> as_of_date` / `p_as_of(_date)` | 仅用于 A 类 |
| `tree_as_of` | 树/层级视图时点 | `resolveTreeAsOfForPage(...)`、`requireTreeAsOf(...)` | 同为 `validity @> tree_as_of` | 仅用于 B 类的树/搜索 |
| `effective_date` | 记录版本生效日（版本选择/写入） | `/org/nodes/details?effective_date=...`；写入 event `effective_date` | event 生效日、版本集合主键之一 | 禁止用 `as_of` 冒充 |

### 4.7 Stopline（禁止新增）
- 禁止在 Shell/Topbar 恢复“全局 `as_of` 选择器/透传/强灌”实现；`/ui/nav`、`/ui/topbar` 不得再要求 `as_of`。
- 禁止在双上下文页面把 `as_of` 当 `effective_date` 使用（包括“伪版本”注入列表）。
- 禁止新增“缺省回退规则”分叉：缺省/重定向/错误码必须回到本矩阵更新后再落码。

## 5. 实施路线（文档先行）

### M0：契约冻结与基线清单
- [X] 输出《路由-时间参数矩阵》：逐条标注 A/B/C 类、默认值、错误码、重定向规则（见 §4.5）。
- [X] 输出《跨层语义映射》：UI 参数 ↔ 服务参数 ↔ SQL 参数映射表（见 §4.6）。
- [X] 为已有实现标注 stopline 规则（见 §4.7）。

### M1：去壳层全局日期（过渡策略）
- [X] Topbar 移除“有效日期（as_of）”选择器：不再提交/改写 `as_of`。
- [X] 保留 URL 可带 `as_of` 的可分享性；缺省回退到当天（UTC）（行为在矩阵中冻结）。
- [X] 禁止旧壳层“强灌 as_of 到所有页面”的行为：Shell 改为仅加载 `/ui/nav`、`/ui/topbar`（无 `as_of`）；导航不再拼接时间参数。

### M2：MUI X Org 模块整改（优先）
- [X] 固化 Org MUI 的“双上下文口径”：`as_of`=视图日期、`effective_date`=详情版本；`effective_date` 可缺省且默认选中算法有单测（承接 `DEV-PLAN-076`）。
- [X] 移除越权/未交付能力：删除 OrgUnits 列表页“批量启用/停用”相关 UI 与逻辑（checkbox selection、bulk buttons、前端批量调用循环）。
- [X] 修正文案/字段语义：移除 OrgUnits 列表中误导性的 `effectiveDate` 列（此前实际展示的是 `as_of`）。
- [X] 固化“版本列表稳定性”断言：仅展示真实版本集合；默认选中算法覆盖缺省与 miss（见 `orgUnitVersionSelection.test.ts`）。

### M3：跨模块治理
- [X] 逐个评估 SetID/JobCatalog/Staffing/Person：当前均为 A 类（单上下文 `as_of`），继续保留；不再依赖壳层注入，由各自 handler 负责缺省/校验。
- [X] 对 C 类页面（Shell/Topbar/Nav 等）移除 `as_of` 依赖（见 §4.5）。

### M4：门禁与证据
- [X] 新增/强化测试：参数校验、默认日期、重定向、列表稳定性（Go handler/shell tests + MUI unit tests）。
- [X] 在 `docs/dev-records/` 记录执行日志与回归结果：`docs/dev-records/dev-plan-102-execution-log.md`。

## 6. 影响范围（草案）
- 文档：`docs/dev-plans/073`、`docs/dev-plans/076`、本计划（102）及后续执行日志。
- 后端：`internal/server/as_of.go`、`internal/server/handler.go`、`internal/server/orgunit_nodes.go`、相关模块 handler。
- 前端：`internal/server/assets/astro/app.html`、`apps/web/src/pages/org/*`、相关 API 调用层。
- 测试：`internal/server/*_test.go`、MUI 页面与版本选择相关测试。

## 7. 验收标准
- [ ] 存在并评审通过《路由-时间参数矩阵》（A/B/C 分类完整）。
- [ ] 双上下文页面不再出现“伪版本”“切换丢焦”“全页刷新回根”等问题。
- [ ] Topbar/Shell 不再提供全局日期选择器，也不再向不兼容页面注入冲突参数。
- [ ] 同一页面的默认日期算法、错误码、重定向行为可由测试稳定复现。
- [ ] Org MUI 列表页不存在“批量启用/停用”入口；列表中不再出现“把 as_of 标成 effective_date”的误导列。
- [ ] `make check doc` 通过；相关实现阶段触发器与门禁按 `AGENTS.md` 与 `DEV-PLAN-012` 执行。

## 8. 风险与约束
- 若直接全仓统一到单一参数，可能引发大范围回归；应按路由分类渐进收敛。
- 禁止引入 legacy 回退通道（如 `read=legacy`、双链路兜底）。
- 回滚策略仅允许环境级保护（只读/停写/修复后重试），不允许恢复旧契约并长期并存。

## 9. 交付物
- `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`（本文件）。
- 后续执行证据：`docs/dev-records/dev-plan-102-execution-log.md`（实施时创建）。
