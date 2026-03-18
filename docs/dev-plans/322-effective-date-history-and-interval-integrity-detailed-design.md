# DEV-PLAN-322：历史、生效日期、区间完整性与 `current / as_of / history` 详细设计

**状态**: 规划中（2026-03-18 07:48 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 的 `322` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“关系型主模型 + effective-dated history + audit log”的冻结；
- [DEV-PLAN-032](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/032-effective-date-day-granularity.md) 对 `Valid Time = date`、`Audit/Tx Time = timestamptz` 的时间口径冻结；
- [DEV-PLAN-102B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md) 对“显式时间上下文、禁止隐式 today”的现行仓库口径；
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 与 [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md) 已经采用的 `current / as_of / history` 业务语言。

`320` 已经说明：如果没有一份单独的共享计划冻结“历史、effective date、区间完整性与读视图”的合同，后续 `340/350/360/370/380/390` 会各自发明：

- “当前”到底是什么意思；
- “某日视图”与“历史视图”到底怎么区分；
- 哪些对象应该是 `主表 + 版本表`，哪些对象应该是 `主档 + 操作快照`；
- 同主体时间段能否重叠、是否允许断档、是否要求最后一段永远开放；
- 更正、插入、停用、撤销这些动作如何影响时间线。

`322` 的任务就是把这些问题收敛为 **平台级、业务规则优先的共享时间合同**，让后续子计划引用同一套语言，而不是继续在各模块里发明第二事实源。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述时间与历史能力，不让 `daterange`、`exclude`、`current_date`、`end_date` 之类实现词汇喧宾夺主。
- [ ] 冻结 `current / as_of / history` 三类读取视图的业务含义、显式输入要求与共享输出语义。
- [ ] 冻结 effective-dated 对象的共享建模合同：什么是稳定身份、什么是版本切片、什么是时间线、什么是区间完整性。
- [ ] 冻结 Pattern A（`主表 + 版本表`）与 Pattern B（`主档 + 操作快照`）的适用边界，避免所有模块都被强推成同一种时间模型。
- [ ] 冻结新增、插入、更正、停用、撤销等时间相关写意图的共享规则，使后续业务计划不再各自定义“时间操作”。
- [ ] 为 `340/350/360/370/380/390` 提供统一业务需求输入，确保 UI、Workflow、Workbench、Assistant 都建立在同一时间合同之上。

### 2.2 非目标

- [ ] 本计划不直接定义最终数据库 DDL、迁移脚本、EF Core mapping 与 Dapper 查询实现细节；这些实现边界由后续 `324` 与各领域子计划承接。
- [ ] 本计划不替代 `323`；审计留存、任务回执、会话与操作快照的独立模式由后续 `323` 冻结。
- [ ] 本计划不要求所有业务对象都采用 effective-dated 主模型；对于不以“按日演化的业务事实”为核心的问题，可以采用 `主档 + 操作快照` 模式。
- [ ] 本计划不允许把“当前视图”重新解释为“服务端缺省 today”；任何依赖业务日的行为都必须显式表达时间上下文。
- [ ] 本计划不为测试便利而引入 legacy fallback、兼容双轨或“缺参数就取最近一条”的隐式回退。

## 3. “业务规则优先”在共享时间合同中的翻译

### 3.1 用户真正维护的是“业务对象如何随日期演化”，不是时间字段

用户关心的不是：

- 某张表是否有 `end_date`；
- 版本表有没有 `daterange`；
- SQL 是否用了 `max(effective_date)`；
- ORM 是否能自动帮忙拼历史。

用户真正关心的是：

- 这个业务对象在今天是否有效；
- 在某一天它到底是什么样；
- 它是如何一步一步演化到现在的；
- 我现在插入或更正一条历史，会不会破坏前后记录；
- 系统能否明确阻止冲突，并解释为什么不允许。

### 3.2 `current / as_of / history` 是一级业务产品，而不是查询参数技巧

`322` 冻结三类用户可感知的读视图：

- `current`：回答“现在业务上看到的有效状态是什么”；
- `as_of`：回答“在某个显式日期看到的状态是什么”；
- `history`：回答“这个对象完整时间线如何演化”。

同时冻结以下翻译规则：

- `current` 是**产品语言**，不是服务端随手补的默认分支；
- 一旦进入服务层、仓储层、SQL 层，`current` 必须先被归一为显式时间锚点的 `as_of` 快照视图或显式 `history` 视图；
- 因此，服务端不得把“缺失日期参数”自动解释为 `current`。

### 3.3 业务有效时间与审计时间是两条不同坐标轴

`322` 继续沿用 [DEV-PLAN-032](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/032-effective-date-day-granularity.md) 的冻结口径：

- `Valid Time` 回答“业务上从哪一天开始算有效”，粒度必须是 `date`；
- `Audit/Tx Time` 回答“系统在什么时刻处理了这次操作”，粒度是 `timestamptz`；
- 审计时间不能拿来替代业务有效时间；
- 业务有效时间也不能假装等于“提交那一刻”。

### 3.4 区间完整性是业务承诺，不是数据库小技巧

时间区间完整性要回答的是：

- 同一主体在同一自然键下，能不能同日冲突；
- 同一主体在相邻日期上，能不能时间段重叠；
- 某类对象是否允许断档；
- 某类对象的最后一段是否必须保持开放；
- 撤销错误记录后，原先占用的时间槽位是否应释放。

数据库约束只是这套业务承诺的技术承载，而不是规则本身。

## 4. 当前基线：已沉淀的共享结论

### 4.1 现行且已稳定的规则

#### 4.1.1 时间语义已经形成单主源

- [DEV-PLAN-032](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/032-effective-date-day-granularity.md) 已冻结：
  - `Valid Time = date`
  - `Audit/Tx Time = timestamptz`
  - 区间约束推荐统一映射到半开区间
- [DEV-PLAN-102B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md) 已冻结：
  - 读路径时间上下文必须显式；
  - 写路径 `effective_date` 必须显式；
  - 禁止隐式 today。

#### 4.1.2 Greenfield 主模型已经明确偏向 effective-dated history

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确：
  - Greenfield 默认主模型不是事件溯源，而是关系型主模型 + effective-dated history + audit log；
  - `Org / JobCatalog / Position / Assignment` 优先采用 `主表 + 版本表`；
  - `Person / 配置项 / 审批记录` 默认更接近 `主档 + 操作快照`。

#### 4.1.3 下游计划已经开始依赖统一时间产品语言

- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 已将 `current / as_of / history` 视为租户可扩展能力的一级能力；
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md) 已将“树视图、某日版本、完整历史”作为组织域的标准交付语言。

### 4.2 当前仍然缺失的共享决定

尽管基线已经较清晰，但仍缺四类缺口：

1. **缺少一份 Greenfield 视角的“共享时间合同”总文档**  
   `032` 更像底层时间标准，`102B` 更像现仓收口规则，`321/361` 是消费域表达，仍少一份把它们统一翻译为 Greenfield 平台语言的 SSOT。

2. **`current` 的产品语义与服务语义尚未显式分层**  
   现在大家都在说 `current / as_of / history`，但尚未统一说明：`current` 在产品层可见，但一旦进入服务与数据层必须归一为显式锚点。

3. **哪些对象要求 no-overlap / gapless / last-infinite 尚未形成共享模板**  
   目前这些约束仍容易在具体模块里临时决定。

4. **时间相关写意图仍缺共享字典**  
   新增、插入、更正、停用、撤销都涉及时间线变化，但尚未形成平台级定义。

## 5. 共享时间与历史能力的目标业务蓝图

### 5.1 领域使命

共享时间与历史能力是平台内“**对象如何按日生效、如何按日被观察、如何维护前后时间线完整性、何时应该拒绝冲突**”的唯一业务权威。  
它不是某个领域私有的实现技巧，而是 `360/370/380/390` 共同依赖的共享合同。

### 5.2 核心业务对象

| 业务对象 | 业务含义 | 是否由 `322` 共享合同拥有 |
| --- | --- | --- |
| `StableIdentity` | 不因版本切换而改变的稳定业务身份 | 是（共享建模概念） |
| `VersionSlice` | 某一业务对象在某一有效起点上的切片记录 | 是 |
| `BusinessTimeline` | 由多个 `VersionSlice` 组成的完整业务时间线 | 是 |
| `TimeViewRequest` | 调用方显式选择 `current / as_of / history` 的请求合同 | 是 |
| `AsOfSnapshot` | 针对单一业务日求值得到的观察结果 | 是 |
| `HistoryView` | 按时间顺序展示所有版本切片的结果 | 是 |
| `IntervalIntegrityRule` | no-overlap / gapless / last-infinite / 同日唯一等时间完整性规则 | 是 |
| `TemporalMutationIntent` | `add / insert / correct / close / rescind` 等时间写意图 | 是 |
| `AuditRecord` | 谁在何时进行了什么操作 | 否，由 `323/370` 拥有 |
| `DomainBusinessRecord` | 组织、职位、任职、配置等领域上的真实业务数据 | 否，由消费域拥有 |

### 5.3 面向用户的主能力

- 查看“当前有效状态”
- 查看“某日状态”
- 查看“完整历史”
- 追加未来版本
- 在历史区间中插入版本
- 对错误记录做受控更正
- 结束当前有效段或停用对象
- 撤销错误建档或错误事件
- 在冲突时得到明确、可解释的拒绝原因

### 5.4 共享建模模式

#### 5.4.1 Pattern A：`主表 + 版本表`

适用于：

- 业务价值天然建立在“某天有效状态”之上的对象；
- 用户明确需要 `current / as_of / history` 三视图；
- 需要插入历史、更正日期、控制时间区间完整性的对象。

默认适用对象：

- `Org`
- `JobCatalog`
- `Position`
- `Assignment`

业务含义：

- 主表只负责稳定身份；
- 版本表负责“某段业务日上它是什么样”；
- 任何当前态都只是时间线上的派生结果，而不是第二事实源。

#### 5.4.2 Pattern B：`主档 + 操作快照`

适用于：

- 用户关心“当前事实 + 关键操作留痕”，而不是逐日演化；
- 历史主要回答“谁改了什么”，不是“某日到底哪一版有效”；
- 对象本身不需要完整的日粒度时间线插入/更正。

默认适用对象：

- `Person`
- `ConfigDefinition`
- `WorkflowRequest`

业务含义：

- 主档承载当前事实；
- 快照回答变更证据；
- 不应为了统一而强行伪装成 effective-dated 版本线。

## 6. `322` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 查看当前状态 | 回答“现在业务上有效的是什么” | `current` 是显式视图意图；进入服务层前必须归一为显式时间锚点的快照视图 | 当前结果可解释、可复算、不可依赖隐式 today |
| 查看某日状态 | 回答“在某一天它是什么样” | `as_of` 必须显式；同一输入跨天重放结果不变 | 某日切片可稳定重放 |
| 查看完整历史 | 回答“它是如何演化到今天的” | `history` 不得偷偷裁剪为单条当前记录；时间线必须可排序、可解释 | 用户能看到完整演化链 |
| 追加未来版本 | 在最晚有效段之后增加新状态 | 新版本起点必须晚于当前最晚版本；不得与现有时间段重叠 | 形成新的最晚版本 |
| 插入历史版本 | 在两个相邻版本之间插入一条记录 | 插入点必须落在前后边界之间；不得制造同日冲突或重叠 | 中间时间段被合法切分 |
| 更正历史版本 | 修正错误日期或字段值 | 更正必须在相邻边界内进行；不得借“更正”偷渡跨边界重排 | 错误被修正但时间线仍合法 |
| 结束当前有效段 | 将对象在某日后终止或停用 | 终止日期必须显式；后续是否允许再开新段由领域计划声明 | 有效段被受控关闭 |
| 撤销错误记录 | 撤销一条本不该存在的历史 | 撤销语义必须与停用/终止分离；若合同要求，撤销后应释放时间槽位 | 错误事实被移除且时间线恢复一致 |
| 读取跨系统结果 | 报表、导出、集成、Assistant 消费时间视图 | 消费方必须显式声明 `current`、`as_of` 或 `history`；不得自造局部默认 | 平台内外对同一时间语义保持一致 |

## 7. 共享合同、不变量与边界

### 7.1 读视图合同

`322` 冻结三类用户可见视图：

- `current`
- `as_of`
- `history`

同时冻结两条内部实现护栏：

1. 对服务层、仓储层、SQL 层而言，允许的内部时间形态只有：
   - 锚点快照视图（显式 `as_of`）
   - 完整历史视图（显式 `history`）
2. `current` 只能作为显式产品意图存在，不能在下层实现中以“缺参数时默认 today”出现。

### 7.2 时间轴合同

- `Valid Time` 一律是 `date`，表达业务生效日；
- `Audit/Tx Time` 一律是 `timestamptz`，表达系统处理时间；
- 任何需要“按业务日重放”的行为，必须只依赖显式业务日；
- 审计时间差异不得改变业务结果。

### 7.3 区间完整性合同

`322` 冻结以下共享规则：

- **same-day unique**：同主体、同自然键下同一业务日不得有两个有效槽位；
- **no-overlap**：同主体、同自然键下有效区间不得重叠；
- **gapless**：是否要求无缝衔接，不是默认值，必须由具体领域计划显式声明；
- **last-infinite**：最后一段是否必须保持开放，不是默认值，必须由具体领域计划显式声明；
- **adjacent-boundary correction**：插入与更正必须受相邻边界约束；
- **referential validity**：在某日生效的记录，其引用对象也必须在该日有效。

### 7.4 时间写意图合同

`322` 冻结五类共享写意图：

- `add`：在时间线末尾追加新版本；
- `insert`：在两段版本之间插入新版本；
- `correct`：修正既有版本的日期或字段值；
- `close`：结束某段开放式有效区间；
- `rescind`：撤销错误建档或错误事件。

冻结原则：

- 任何领域若声明支持某种时间写意图，就必须同时声明该意图对时间线的影响；
- 不允许继续用模糊“update/save”掩盖不同时间语义；
- `rescind` 不得被偷换成 `close` 或 `disable`。

### 7.5 查询与导出合同

- 所有查询、导出、报表、集成、Assistant 检索都必须显式声明时间视图；
- 不允许通过“取最新记录”“按 ID 查当前态”“缺参时默认 today”模拟时间语义；
- `history` 输出必须带稳定排序与版本关系，不能只返回一堆无序快照。

### 7.6 实现护栏

- 各领域可以有各自的物理存储，但不能各自重写：
  - `current / as_of / history` 的定义；
  - 同日唯一与不重叠规则；
  - 时间写意图词汇；
  - “当前视图”的默认语义。
- `322` 是共享时间合同，不拥有各领域主数据真值；
- `323/370` 可以叠加审计与工作流轨迹，但不能反向重写业务时间规则。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `340`（平台与 IAM 基座）的输入

- [ ] 平台入口必须拥有统一的时间视图解析与归一能力，禁止各模块各自解析“当前/某日/历史”。
- [ ] 请求上下文中必须区分业务时间锚点与审计时间，不得混用。
- [ ] 平台错误模型需要能表达：缺失时间视图、非法日期、区间冲突、相邻边界越界等共享错误。

### 8.2 对 `350`（前端产品壳与交互系统）的输入

- [ ] UI 必须显式呈现“当前 / 某日 / 历史”选择，不得把时间视图藏在实现默认值里。
- [ ] 任何支持历史的页面，都必须把“用户正在看哪个时间视图”表达清楚。
- [ ] 插入、更正、终止、撤销等动作的交互文案，必须直接映射共享时间写意图，而不是笼统叫“编辑”。

### 8.3 对 `360`（核心 HR 业务域）的输入

- [ ] 每个业务对象都必须声明自己属于 Pattern A 还是 Pattern B。
- [ ] 每个 effective-dated 对象都必须显式声明：
  - 是否要求 `gapless`
  - 是否要求 `last-infinite`
  - 支持哪些时间写意图
- [ ] `362/363/364` 不得重新定义 `current / as_of / history`。

### 8.4 对 `370`（工作流、审计增强与集成）的输入

- [ ] 审批与长事务状态必须绑定明确业务日，不得仅凭提交时间推断业务有效时间。
- [ ] 审计增强需要同时呈现：操作发生时间、业务生效时间、时间写意图。
- [ ] 集成出站与回执必须声明自己采用哪种时间视图。

### 8.5 对 `380`（数据工作台与运营分析）的输入

- [ ] Query Workspace、导入导出与运营报表必须显式支持 `current / as_of / history`。
- [ ] 数据质量工作台至少要能发现：重叠区间、同日冲突、断档、越界更正、引用对象失效。
- [ ] 报表与导出不得自己拼“当前记录”逻辑而绕开共享时间合同。

### 8.6 对 `390`（Chat Assistant）的输入

- [ ] 当用户问题依赖时间上下文时，Assistant 必须先澄清是在问“当前、某日还是历史”。
- [ ] Assistant 在调用业务检索或动作预览前，必须把 `current` 归一为显式时间锚点或显式 `history` 视图。
- [ ] Assistant 生成确认摘要时，必须回显业务生效日与时间写意图，不能只说“已更新”。

## 9. 建议实施分期

1. [ ] `M1`：时间产品语言冻结  
   统一 `current / as_of / history`、`Valid Time / Audit Time`、`add / insert / correct / close / rescind` 词汇，不再让各模块继续自定义时间语义。
2. [ ] `M2`：读视图合同冻结  
   明确产品层三视图与服务层两类内部时间形态的归一关系，冻结禁止隐式 today 的护栏。
3. [ ] `M3`：共享区间完整性模板冻结  
   冻结 same-day unique、no-overlap、gapless、last-infinite、adjacent-boundary correction、referential validity 规则模板。
4. [ ] `M4`：Pattern A / Pattern B 适用矩阵冻结  
   为首批核心对象明确采用哪种时间模型，并给出原因。
5. [ ] `M5`：首批消费域接线  
   由 `361/362/363/364` 以及 `380/390` 正式引用 `322`，停止在各自计划中重写时间主规则。

## 10. 验收标准

- [ ] `322` 已成为 Greenfield 共享时间合同的单一事实源，而不是继续分散在 `032/102B/321/361` 的交叉引用里。
- [ ] `current / as_of / history` 的产品语义与下层实现语义已经显式分层，不再存在“current = 缺省 today”的漂移空间。
- [ ] Pattern A 与 Pattern B 的适用边界已冻结，后续子计划不再把所有对象强行塞进同一种历史模型。
- [ ] 区间完整性模板已明确区分“共享必选规则”与“由具体领域声明的可选规则”。
- [ ] `340/350/360/370/380/390` 能直接把 `322` 作为时间与历史能力输入，而不再各自发明第二套时间合同。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md)
- [DEV-PLAN-032](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/032-effective-date-day-granularity.md)
- [DEV-PLAN-102B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md)
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
