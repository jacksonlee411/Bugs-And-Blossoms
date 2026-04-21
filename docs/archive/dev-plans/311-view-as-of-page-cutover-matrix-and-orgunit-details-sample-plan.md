# DEV-PLAN-311：View As Of 页面改造矩阵与 OrgUnitDetails 样板实施计划

**状态**: 已归档（历史来源；原状态：已完成，2026-04-09 CST）

## 背景

`DEV-PLAN-310` 已将全项目时间语义从“一刀切显式 `as_of`”收敛为“两层规则”：

- 产品/UI 层：`current-by-default`
- 服务/集成层：`explicit-by-contract`
- 写链路：`effective_date` 显式必填且不与 `as_of` 混用

当前缺口不再是原则讨论，而是**页面级执行方案**尚未冻结：

1. 哪些页面需要取消 `As Of` 默认常显；
2. 哪些页面属于“支持历史切片，但入口按需出现”；
3. `OrgUnitDetailsPage` 如何在不重做整页布局的前提下，成为第一张样板页；
4. 前端如何建立统一的 current/history 解析与下游显式时间合同边界。

本计划作为 `DEV-PLAN-310` 的执行子计划，负责把页面改造矩阵、样板页实施口径与交付顺序冻结为 SSOT。

## 目标

1. [x] 冻结页面改造矩阵：区分“取消默认常显”“保留 default current 但做结构收口”“保留工具态显式时间输入”。
2. [x] 冻结 `OrgUnitDetailsPage` 样板实施方案：在现有版本时间轴与审计日志基础上完成低增量收口。
3. [x] 冻结前端共享 read-view helper 的职责边界：产品层 current/history 解析与下游显式时间合同之间如何衔接。
4. [x] 给出页面实施优先级（P0/P1/P2）与验收标准，供后续代码改造直接承接。

## 非目标

- 不在本计划内直接修改后端 API/Kernel 合同。
- 不在本计划内创建新的时间抽象层（如 `timeAnchor` / `lastTimeAnchor`）。
- 不在本计划内重做所有页面视觉布局或导航结构。
- 不在本计划内一次性改完全部页面；本计划先冻结执行矩阵与样板页口径。

## 约束来源（SSOT）

- `DEV-PLAN-310`：`docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
- `STD-002`：`docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `DEV-PLAN-102B`：`docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`

## 进一步降复杂度补充冻结

本节用于补充冻结本计划的减法方向，避免后续实施过程中又通过新增状态、抽象或自动推导把复杂度加回去。

### 1. 状态模型减法优先于组件抽象

本计划后续承接实现时，优先删状态、删推导关系，而不是先抽 hook、context 或新的时间对象。

冻结规则：

- 对 `OrgUnitDetailsPage` 这类具备版本时间轴的详情页，历史查看优先由“选中的历史记录/版本”表达，而不是继续维持 `as_of + effective_date` 双读态并存。
- URL/search params 只承载读态：如当前 tab、历史版本、审计事件、是否处于 history 模式。
- 写表单只承载写态：如 `effective_date`、`target_effective_date`、`reason`、form draft；不得通过 URL 反推或重置。
- 若一个时间字段既参与“当前查看什么”，又参与“本次提交何时生效”，则视为状态职责未拆开，必须继续收口。

### 2. `OrgUnitDetailsPage` 样板页采用单历史锚点

`OrgUnitDetailsPage` 的目标不是“更优雅地协调多个时间字段”，而是把历史查看收敛为单一历史锚点。

冻结规则：

- `current` 模式下，不要求页面持有显式 `as_of`。
- `history` 模式下，优先由选中的 `effective_date`/版本记录承担历史查看锚点。
- 不再把“任意 `as_of` 切片”作为详情页主状态持续保留；若仍需兼容历史深链接，应在边界解析后尽快收敛为选中的历史记录。
- `actionEffectiveDate` 必须独立于当前查看锚点，不得由 `selectedVersion`、`requestedEffectiveDate` 或 URL 中的 `as_of` 自动覆盖。

### 3. 共享 helper 延后抽取，且只允许最小纯函数

本计划允许建立共享 helper，但 helper 不是第一步，也不是新的时间框架。

冻结规则：

- 先完成 `OrgUnitDetailsPage` 样板页和至少一张 A 类页面的实现收口，再评估是否抽共享 helper。
- 第一阶段共享能力仅限纯函数级别，例如 `parseReadViewState(...)`、`resolveExplicitAsOf(...)`。
- 禁止为本轮页面改造预先引入 `readViewContext`、全局时间 store、时间状态机、跨页 continuation envelope 等新层。
- 若某个 helper 需要同时理解页面布局、表单默认值、查询缓存和路由跳转，则说明 helper 已越界，必须继续拆小或回退到页面内实现。

### 4. 交互减法：禁止写后自动改读态

页面写操作不应悄悄改变用户当前的查看上下文。

冻结规则：

- 保存/发布/启用/停用/更正成功后，不自动切换页面 `as_of`、history 选中态或当前详情版本。
- 不允许新增“保存成功后自动跳到生效日”的通用模式。
- 如业务上确有必要提示用户查看新的生效记录，应使用显式提示或 CTA，由用户决定是否切换。
- “写结果需要可见”不等于“读态必须自动跳转”；优先通过 toast、提示条、链接按钮实现，而不是直接改 search params。

### 5. 页面分类继续向“两大类”收敛

`A/B/C` 分类用于当前过渡阶段的执行矩阵；团队心智应继续向更简单的两大类收敛：

- 业务浏览页：`default current`
- 工具页：`explicit time`

补充约定：

- `B` 类不是长期独立制度，而是“业务浏览页中的样板页/收口页”。
- 后续若新增页面，默认先按“业务浏览页”处理；只有当显式时间本身就是任务参数时，才归入工具页。
- 不得因为页面支持历史查看，就自动升级为“必须常显时间输入”的特例。

### 6. 配套 stopline（防止复杂度回流）

为避免复杂度从实现层回流，后续承接代码改造时应同步增加或复用门禁，阻断以下模式继续扩散：

- 新增 page-local `todayISO()` / `parseDateOrDefault()` / `fallbackAsOf` 一类读态默认化 helper。
- 新增 `as_of -> effective_date` 或 `effective_date -> as_of` 的自动回填。
- 新增“写成功后自动改 `as_of` / 自动跳日 / 自动改历史选中态”逻辑。
- 新增服务端缺参补 today 的 fail-open 行为。

说明：

- `orgunit` 读链路仍存在服务端默认 today 的历史实现；该问题不在本计划内直接修改 API/Kernel 合同，但必须作为后续实施的并行收口项跟踪，否则前端页面仍会被迫保留防御性时间默认逻辑。

## 页面改造矩阵

### 1. 分类定义

#### A 类：取消 `As Of` 默认常显

适用条件：

- 页面当前把 `as_of` 作为主筛选控件、上下文卡片或主输入字段直接暴露给用户；
- 页面主任务默认应是“浏览 current 内容”；
- 页面虽然支持历史切片，但历史能力不应成为默认入口。

目标改造：

- 默认 current；
- 非历史模式不显示“查看日期 / View As Of”主控件；
- 历史模式按需出现入口或状态；
- 写表单不得继承当前查看日期。

#### B 类：保留 default current，但做结构收口

适用条件：

- 页面当前没有默认常显 `As Of` 主控件；
- 但页面内部仍存在“查看上下文 / 历史版本 / 写入生效日”互相推导的问题。

目标改造：

- 保留现有大体布局；
- 切断读写时间串线；
- 明确 current/history 语义与写侧 `effective_date` 的边界。

#### C 类：工具态显式时间输入保留

适用条件：

- 页面或组件主要承担导出、Explain、治理诊断、同步/发布等工具职责；
- 显式时间输入本身就是任务参数，而不是浏览主心智。

目标改造：

- 不删除显式时间能力；
- 但文案从技术字段名 `as_of` 收敛为任务态文案（如“查看日期 / View As Of”“发布时点”“解释时点”）；
- 不让该输入向业务浏览页蔓延。

### 2. 页面级结论

| 页面/组件 | 当前状态 | 历史切片 | 分类 | 改造动作 | 优先级 |
| --- | --- | --- | --- | --- | --- |
| `OrgUnitsPage` | `As Of` 默认常显；新建表单默认继承 `asOf` | 是 | A | 移除默认常显；历史模式按需出现；切断 `as_of -> effective_date` | P0 |
| `JobCatalogPage` | 顶部 `As Of` DatePicker 常显 | 是 | A | 改为 default current；仅历史模式显示 `View As Of` | P1 |
| `AssignmentsPage` | `Load` 区块常显 `as_of`；写表单默认继承 | 是 | A | 改为 default current；写表单只保留 `effective_date` | P1 |
| `PositionsPage` | `Load` 区块常显 `as_of`；写表单默认继承 | 是 | A | 改为 default current；写表单只保留 `effective_date` | P1 |
| `DictConfigsPage` | 顶部上下文常显 `as_of` | 是 | A | 浏览区改为 default current；保留 release 表单独立时间字段 | P1 |
| `OrgUnitFieldConfigsPage` | 过滤栏常显 `as_of`；保存后会推动页面跳日 | 是 | A | 改为 default current；取消保存后自动切换查看日期 | P0 |
| `SetIDGovernancePage` | 顶部上下文常显 `as_of`；会改写 registry `effectiveDate` | 是 | A/C 混合 | 浏览区 default current；Registry/Explain 保留工具态显式时间，但改文案并切断串线 | P0 |
| `OrgUnitDetailsPage` | 无默认常显控件，但内部状态串线明显 | 是 | B | 作为样板页，优先实施低增量结构收口 | P0 |
| `DictValueDetailsPage` | 无默认常显控件；版本/审计分区较清楚 | 是 | B | 保持 default current；只做边界校验与命名优化 | P2 |
| `SetIDExplainPanel` | 显式 `as_of` 输入 | 是 | C | 保留显式时间能力，但改成任务态文案，不再裸露 `as_of` | P1 |

## `OrgUnitDetailsPage` 样板实施方案

### 1. 样板页定位

`OrgUnitDetailsPage` 是本轮页面改造的 P0 样板页。原因：

- 它同时具备 current/history 查看、版本时间轴、审计日志、写动作区；
- 它最能代表“查看上下文、版本/历史、本次动作生效日”三类时间语义缠绕的问题；
- 它已经具备可复用的历史 UI，不需要另起炉灶，只需低增量收口。

### 2. 样板页保留项

以下现有能力保留，不作为重构目标：

- 版本时间轴/版本列表
- 审计日志/修改日志
- 当前详情展示骨架
- 当前动作区/表单区
- 历史深链接能力

### 3. 样板页必须切断的状态链

必须切断以下推导关系：

1. `asOf -> effectiveDate(详情展示) -> actionWriteEffectiveDate`
2. `selectedVersion/effectiveDate -> actionForm.effectiveDate`
3. `updateSearch(asOf=...) -> actionForm 重置为 asOf`

目标是让三个概念各自独立：

- `viewMode/current-history`：当前在看 current 还是 history
- `selectedRecordVersion`：当前详情展示的是哪条历史记录
- `actionEffectiveDate`：本次动作准备在哪天生效

### 4. 样板页交互要求

#### 4.1 默认模式

- 默认进入 `current`；
- 不常显日期输入框；
- 用户看到的是当前详情、版本时间轴、审计日志、动作区。

#### 4.2 历史模式

- 优先依赖版本时间轴选中态表达“正在查看历史记录”；
- 如果历史来源不明显，或页面同时存在读写混淆风险，才在页头增加轻量状态提示；
- 该提示只表达查看上下文，不携带写表单默认值。

#### 4.3 动作区

- 动作区始终只展示“生效日期”；
- 默认值由动作规则决定；
- 历史查看和版本选择都不得自动覆盖用户已输入的生效日期。

#### 4.4 状态与 URL 收口要求

- history 深链接若包含 `as_of`，进入页面后应尽快解析并收敛为历史版本选中态，而不是把 `as_of` 长期保留为详情页核心状态。
- `effective_date` 不作为详情页读态 query 参数长期保留；若确需保留深链接能力，应仅表达“选中哪条历史记录”，不表达写表单默认值。
- `updateSearch(...)` 只负责读态切换，不得承担动作表单初始化、重置或改写职责。

### 5. 布局影响评估

该样板页不要求新增三张大卡片或重做主栅格。推荐理解为**状态职责拆分优先，视觉布局调整最小化**：

- 版本时间轴继续承担“历史记录入口”；
- 审计日志继续承担“修改记录查看”；
- 只在必要时增加轻量历史状态提示；
- 真正新增的是动作区生效日期边界，而不是新面板。

结论：

- 实现复杂度：中低
- 布局复杂度：低
- 用户认知复杂度：下降

## 前端共享 helper 边界

### 1. 目标

建立一个最小共享 helper，统一处理：

- 页面当前是否处于 `current` 或 `history`；
- 何时显示历史模式入口；
- 何时需要把产品层 current/history 语义转换为下游显式时间合同；
- 非法日期和缺失日期的前端边界处理。

### 2. 非目标

- 不创建新的跨层时间协议；
- 不让 helper 生成新的对外合同字段；
- 不把 helper 变成“自动补 today 工具”。
- 不把页面状态机、表单默认值规则、版本选中逻辑一并塞进一个“大一统 helper”。

### 3. 规则

1. 产品层可以 default current。
2. 一旦进入历史切片、导出、Explain、跨模块读、缓存重放等链路，helper 必须在边界形成明确时间参数。
3. helper 不得把“用户未选日期”直接实现成“服务端自动补 today”。
4. helper 优先只处理“读态解析”和“显式合同形成”，不处理写态默认值。
5. 只有在两个以上页面出现相同纯函数需求时才允许抽出；禁止为了预防未来重复而提前造层。

## 后续建议拆分主题

`DEV-PLAN-311` 作为父计划，负责冻结页面矩阵、样板页方向与减法原则；具体执行方案建议按主题拆分为以下子计划，避免把上位原则文档膨胀成混合型实施文档。

### 1. 已拆分子计划

- `DEV-PLAN-312`：详情页单历史锚点与 A 类页面读写解耦实施计划。

### 2. 建议继续拆分的子计划

#### `DEV-PLAN-313`：后端并行收口计划

主题：

- 移除残余服务端 `today` fallback；
- 对齐 `current-by-default` 与 `explicit-by-contract` 的前后端边界；
- 为前端删掉防御性默认逻辑创造条件。

定位：

- 与 `DEV-PLAN-312` 并行推进；
- 不重定义页面矩阵，只处理服务端残余时间默认化问题。

#### `DEV-PLAN-314`：P1 页面批量收口计划

主题：

- 按 `DEV-PLAN-312` 的收口模板，批量改造 `AssignmentsPage`、`PositionsPage`、`JobCatalogPage`、`DictConfigsPage`。

定位：

- 承接 `DEV-PLAN-312`；
- 面向 A 类页面的第二波批量实施，而非逐页单独起计划。

#### `DEV-PLAN-315`：最小 helper 与反回流门禁计划

主题：

- 在 `DEV-PLAN-312` 首批页面完成后，评估并抽取最小纯函数 helper；
- 增加前端反模式门禁，阻断 `todayISO()` / `parseDateOrDefault()` / `effectiveDate: asOf` 等模式回流。

定位：

- 后置承接 `DEV-PLAN-312`；
- 仅在真实重复出现后才允许抽象，不提前造层。

#### `DEV-PLAN-316`：工具态页面收口计划

主题：

- 收敛 `SetIDExplainPanel` 与治理/Explain/Release 一类工具态页面；
- 保留显式时间能力，但统一任务态文案与边界隔离规则；
- 防止工具态显式时间再次蔓延回业务浏览页。

定位：

- 与 A 类页面收口分轨处理；
- 不把工具页规则错误外溢到业务浏览页。

#### `DEV-PLAN-317`：页面时间语义回归与验收计划

主题：

- 建立跨页面回归矩阵与 E2E 场景；
- 验证 default current、history 不改写写态、写后不跳日、下游显式时间合同仍成立等关键行为。

定位：

- 作为 `DEV-PLAN-311/312/314/316` 的统一验收计划；
- 负责把页面级时间语义收口转化为稳定回归证据。

### 3. 当前不建议继续拆分的主题

- 不建议为 `OrgUnitDetailsPage` 再单起一份计划，`DEV-PLAN-312` 已经足够细化。
- 不建议为每一张 A 类页面分别起单独计划，优先按批量模板推进。
- 不建议当前为 P2 页面单独起计划；其边界校验与命名优化可并入后续批量或收尾计划。

## 实施步骤

1. [x] 基于本计划冻结页面矩阵，确认 P0/P1/P2 排期。
2. [x] 输出 `OrgUnitDetailsPage` 实施说明：单历史锚点、状态拆分、保留区块、切断链路、验收标准。
3. [x] 输出 A 类页面统一减法说明：移除默认常显、禁止写后自动改读态、写表单不继承查看日期。
4. [x] 待样板页与首张 A 类页面收口后，再评估是否需要抽前端共享 helper，并将 helper 约束为最小纯函数。
5. [x] 完成 P0 页面设计评审：`OrgUnitDetailsPage`、`OrgUnitFieldConfigsPage`、`SetIDGovernancePage`、`OrgUnitsPage`。
6. [x] 跟踪并冻结与本计划配套的后端并行收口项：移除残余服务端缺参补 today 的实现。
7. [x] 待后续实施计划承接代码改造与验证。

## 实施结果

1. [x] `DEV-PLAN-311` 作为父计划的拆解链路已全部落地：
   - `DEV-PLAN-312`：详情页单历史锚点与 A 类页面读写解耦
   - `DEV-PLAN-313`：后端显式日期合同、无 fallback、统一错误语义
   - `DEV-PLAN-314`：P1 页面批量收口
   - `DEV-PLAN-315`：最小 helper 与反回流门禁
   - `DEV-PLAN-316`：工具态显式时间语义收口
   - `DEV-PLAN-317`：统一回归矩阵与验收闭环
2. [x] `OrgUnitDetailsPage` 已完成样板职责：旧 `as_of` 入口在页面边界归一化为单一 `effective_date` 历史锚点，写态日期 sticky，写后不自动跳日。
3. [x] 页面矩阵已在实现层稳定成立：
   - 业务浏览页：`default current`
   - 工具页/工具子区：保留显式时间，但仅作为任务参数
4. [x] 前端共享 helper 已按本计划的最小边界承接到 `readViewState` / `readNavigation`，未演化为新的全局时间框架。
5. [x] `DEV-PLAN-317` 已把父计划的关键行为转化为统一回归基线与验收证据。

## 交付物

1. [x] 一份页面改造矩阵（本计划）。
2. [x] 一份 `OrgUnitDetailsPage` 样板实施方案（本计划）。
3. [x] 一份 A 类页面统一减法约束清单（本计划）。
4. [x] 一份前端共享 read-view helper 边界说明（可在后续子计划补充）。
5. [x] 一份按优先级排序的页面实施清单。

## 验收标准

- [x] 新增页面改造讨论不再回到“是否一刀切显式 `as_of`”争论，而以本计划矩阵为准。
- [x] `OrgUnitDetailsPage` 的样板口径被后续实施计划直接引用，无需再次重定义“三区块”的含义。
- [x] 页面级时间语义能够明确区分：default current、按需历史模式、独立 `effective_date` 写链路。
- [x] `OrgUnitDetailsPage` 不再长期维持 `as_of + effective_date` 双读态；历史查看由单一历史锚点表达。
- [x] P0 页面不再出现“写成功后自动切换查看日期/自动跳日”的默认模式。
- [x] 后续实施若抽共享 helper，其职责仍可在 5 分钟内解释清楚，且不引入新的时间框架或全局状态层。
- [x] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/dev-plans/312-view-as-of-implementation-plan-details-single-history-anchor-and-a-pages-read-write-decoupling.md`
- `docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `AGENTS.md`
