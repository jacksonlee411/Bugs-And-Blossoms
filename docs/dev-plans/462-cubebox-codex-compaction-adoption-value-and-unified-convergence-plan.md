# DEV-PLAN-462：CubeBox 借鉴 Codex 成熟压缩机制的统一收敛方案

**状态**: 已完成（2026-04-23 21:10 CST；与 `460/461` 联动封板）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结“本项目为何引入 Codex 成熟 compaction/summary 思路、它在产品层/系统层/查询层/评审层分别解决什么问题、以及如何与 `434/460/461` 分层协同”的统一方案，避免把压缩收益继续散落在实现细节和局部修补里。
- **关联模块/目录**：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`modules/cubebox`、`internal/server`、`apps/web`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-430`、`DEV-PLAN-434`、`DEV-PLAN-437A`、`DEV-PLAN-460`、`DEV-PLAN-461`
- **用户入口/触点**：Web Shell 右侧 `CubeBox` 抽屉、`/internal/cubebox/**`、`turn.context_compacted`、查询回答流式事件

### 0.1 Simple > Easy 三问

1. **边界**：`462` 只冻结“引入 Codex 成熟压缩机制对本项目的意义、分层落点与统一收敛方案”；`434` 继续持有会话级 compaction 内核，`460` 持有数字助手定位，`461` 持有查询最小契约。
2. **不变量**：长线程收敛必须优先走统一上下文机制和统一回答收敛入口，不能继续在 capability-specific 分支、局部 DTO 或零散字符串拼装中扩散。
3. **可解释**：reviewer 必须能在 5 分钟内讲清：引入 Codex compact 机制到底解决了什么；哪些收益已经由 `434` 落地；哪些收益应继续在 `461` 收口；哪些能力仍明确后移而不构成当前前置。

### 0.2 现状研究摘要

- **现状实现**：`434` 已完成 CubeBox 会话级上下文压缩内核，覆盖 recent message 保留、summary prefix、canonical context reinjection、manual/pre-turn compact 与最小 event envelope；`460` 已冻结数字助手定位；`461` 已冻结查询结果进入回答前的最小返回边界。
- **现状约束**：本仓要求原始消息 append-only、权限/租户/页面上下文每轮重新注入、查询执行必须复用现有只读 API、查询回答不得继续长出 capability-specific 特判面。
- **最容易出错的位置**：把 `434` 已有 compact 内核误理解为“只影响会话压缩、不影响查询回答”；把 `461` 的回答预算误做成局部 patch；把 Codex 的成熟机制扩写成过重的 telemetry/UI/memory 方案。
- **本次不沿用的“容易做法”**：不把 Codex compact 的意义简化成“多一个摘要器”；不把查询返回边界继续塞回 `buildCubeboxQueryAnswer` 一类总线特判；不为 `461` 单独新开一套会话级 compaction 子系统。

## 1. 背景与上下文

`CubeBox` 当前已进入“数字助手主链逐步成形、查询编排与结果解释开始接入”的阶段。随着：

- 会话持续增长，
- 查询能力增加，
- 页面/租户/权限上下文叠加，
- 以及查询结果列表、审计摘要、解释文本进入流式回答，

系统面临的核心问题已经不再只是“能否回答”，而是“如何在不破坏权限、审计和可恢复语义的前提下，稳定收敛长上下文与长结果”。

Codex 开源仓库的成熟价值在于：它没有把长线程问题理解为某个能力自己的展示问题，而是把它上移为统一的 compaction / summary / budget 机制。本项目已经在 `434` 中复用这一路径处理会话级 prompt view；`462` 的作用是把这种成熟机制带来的**产品收益、系统收益、查询收益和评审收益**冻结为上位方案，避免后续各层继续各写一套“局部收敛技巧”。

## 2. 目标与非目标

### 2.1 核心目标

- [x] 冻结借鉴 Codex 成熟 compact 机制对本项目的四类核心收益：产品层、系统层、查询层、评审层。
- [x] 明确 `434/460/461` 的 owner 分工，阻断“同一个收敛问题在多个计划里重复发明边界”。
- [x] 冻结统一收敛方案：会话级上下文压缩走 `434`，查询结果进入回答前的最小返回边界走 `461`，两者共享“统一预算、统一降级、canonical context reinjection”的方法论。
- [x] 明确哪些 Codex 能力对本项目有意义，哪些仍明确后移，防止过度设计。

### 2.2 非目标

- 不重新定义 `434` 已完成的会话级 compaction 内核实现细节。
- 不在本计划内新增数据库表、事件模型、前端 reducer 或 provider runtime。
- 不在本计划内引入 remote compaction、独立 summary model、完整 telemetry 数据面、memory pipeline 或 compacted summary UI 全可视化。
- 不把 `462` 扩张成新的“AI 平台总计划”；它只解决“为什么引入 Codex 成熟压缩机制、以及如何在本项目内统一收敛”的问题。

### 2.3 用户可见性交付

- **用户可见入口**：仍以现有 `CubeBox` 抽屉和查询回答流为主。
- **最小可操作闭环**：用户在长线程中继续提问时，系统能够在不复活旧权限/旧页面上下文的前提下维持上下文连续性；查询结果较大时，系统能返回稳定摘要而不是退化成整页列表灌入。
- **本计划交付形态**：文档契约 SSOT；后续 `434/461` 的实现和评审按此方案判断“是否真正复用了 Codex 成熟价值，而不是只是借名义做局部 patch”。

## 3. 对本项目最实际的收益

### 3.1 产品层收益：减少长线程失忆、重复追问、上下文污染

引入 Codex 成熟 compact 机制的第一价值，不是“压缩得更漂亮”，而是降低以下产品退化：

- 长线程中模型忘记刚确认过的关键上下文；
- 历史消息太长后开始重复追问已明确的信息；
- 旧租户、旧页面、旧权限、旧对象上下文从历史或摘要中复活；
- 查询结果过长时把真正有用的要点淹没在大段列表文本中。

对 `CubeBox` 来说，这些问题会直接损害“数字助手”的可信度。`434` 已用 recent message 保留、summary prefix 和 canonical context reinjection 解决会话侧风险；`461` 则应把同样的方法论落到查询结果进入回答前的边界控制。

### 3.2 系统层收益：把上下文压缩从零散技巧变成可治理的内核

第二价值是系统治理。

若没有统一 compact 机制，不同团队很容易分别在：

- gateway 调用前，
- 查询回答拼接时，
- UI 展示时，
- 恢复/replay 链路中，

各自发明一套“保留哪些内容、裁掉哪些内容、出错时怎么退”的局部技巧。

这种做法短期能跑，长期会造成：

- 多套预算语义；
- 多套摘要来源；
- 多套错误与回退路径；
- reviewer 无法判断哪个入口才是权威收敛点。

Codex 的成熟价值在于它证明了：收敛问题应优先进入统一机制。对本项目而言，这一统一机制已经在 `434` 的会话级 prompt view 上建立，后续应继续复用而不是被局部实现绕开。

### 3.3 查询层收益：为 `461` 的 fail-closed 返回边界提供上位方法论

`461` 当前最现实的问题，不是“列表怎么显示更好看”，而是“长结果如何在不扩张本地解释责任的前提下，继续保持 fail-closed 返回边界”。

Codex 的启发在于：

- 不把长结果看成某个能力独有的问题；
- 不优先报错，而优先统一降级；
- 不把本地结果收口继续做成 capability-specific prose 系统。

因此，对 `461` 的价值是：

- 让 fail-closed 返回语义和最小降级原则成为正式边界；
- 防止 `orgunit.list` 一类能力继续通过局部摘要器绕开模型主叙述路径；
- 把“结果返回边界”从点状 patch 拉回到方法论一致的正式边界。

### 3.4 评审层收益：让 reviewer 判断“是否走统一收敛入口”

第四价值是评审效率与稳定性。

没有这层上位方案时，reviewer 往往只能逐个检查：

- 这里是不是又多了一个 `switch api_key`；
- 那里是不是又补了一个字符串模板；
- 某个列表结果是不是偷偷把整页原样展开；
- 某个 compact/summary 特例是不是又绕过了 canonical context reinjection。

有了统一方案后，reviewer 可以优先问：

1. 这个问题应该属于 `434` 的会话级 compaction，还是 `461` 的查询返回边界？
2. 是否走了统一预算与统一降级入口？
3. 是否又把本地结果收口扩写成 capability-specific 解释系统？
4. 是否破坏了 `460` 冻结的当前用户/当前租户/当前 session 不变量？

这正是 `DEV-PLAN-003` 所强调的“让评审语言统一，尽量在计划阶段消除结构性歧义”。

## 4. 统一收敛方案

### 4.1 分层职责

`462` 冻结后的正式分层如下：

- **`434`：会话级上下文压缩内核**
  - 负责 prompt view replacement
  - 负责 recent user messages 保留
  - 负责 summary prefix
  - 负责 canonical context reinjection
  - 负责 manual/pre-turn compact 与最小 compact 事件语义

- **`460`：数字助手产品与权限上位契约**
  - 负责“当前用户的数字助手”定位
  - 负责权限、租户、session、审计归属不变量
  - 负责文档驱动编排不是授权来源的上位约束

- **`461`：查询返回边界**
  - 负责查询结果返回的 fail-closed 边界
  - 负责正常超长结果不直接扩写或直出 raw payload
  - 负责阻断 capability-specific 本地渲染继续扩张

- **`462`：上位意义与协同方案**
  - 负责解释“为什么这些分层存在”
  - 负责冻结四类收益与跨计划协同规则
  - 负责阻断各层再次各写一套收敛逻辑

### 4.2 最小统一原则

本项目后续所有 compact / summary / budget 相关实现，必须同时满足以下原则：

1. **统一降级**：正常超长结果优先收敛与裁剪，不优先报错。
2. **统一权威上下文**：任何压缩或裁剪后，当前权威租户/权限/页面/业务对象上下文必须重新注入，不得依赖旧摘要延续。
3. **统一事实源**：原始会话消息与业务数据仍各自保留原始事实源；压缩与裁剪只服务于 prompt view 或 fail-closed 返回边界，不得篡改审计事实。

### 4.2A 实施顺序与迁移要求

本计划额外冻结以下顺序要求，防止“上位方案尚未落地时，`461` 继续在局部实现里打补丁”：

1. 后续涉及查询返回边界的实现，必须先以 `462` 作为上位方案对齐分层、统一原则和 reviewer 口径，再进入 `461` 的具体收口实施。
2. `461` 的后续实现目标不是“在当前实现上继续叠加一个更复杂的专用摘要分支”，而是依据 `462` 的统一原则，删除现有 capability-specific 硬编码特判，并把结果叙述 owner 回交给模型。
3. 当前已暴露的硬编码实现包括但不限于：
   - 在 `buildCubeboxQueryAnswer` / `summarizeQueryResult` 总线中直接按 `api_key` 分支
   - 在 `summarizeStructuredQueryResult` 中对 `orgunit.list` 进行局部特判
   - 在 `internal/server` 中继续扩张查询回答专用字符串模板，而不经过统一收敛入口
4. `462` 落地后的 `461` 收口必须以“替换并删除当前硬编码实现”为目标，而不是保留旧特判再叠加新入口。
5. 在 `461` 对应实现完成前，任何新增查询能力默认不得继续沿用当前硬编码模式扩张回答主链。

### 4.3 与 Codex 的对应关系

本项目借鉴 Codex 的，不是完整 runtime，而是以下成熟思路：

- replacement history
- recent-user-message budget
- summary prefix
- canonical initial context reinjection
- pre-turn/manual compact
- compact 纯函数测试与 snapshot 思路

本项目当前明确不引入的，包括：

- remote compaction
- model downshift compact
- 独立 summary model
- 完整 telemetry 数据面
- memory pipeline
- compacted prompt 全量 UI 可视化

## 5. 关键设计决策

### 5.1 不把 Codex compact 的价值误解为“某个能力的高级摘要器”

这是本计划的首要决策。

若把 Codex compact 理解为“给 `orgunit.list` 或未来某个能力加专用摘要器”，只会把局部 patch 合法化，无法获得会话内核与查询返回边界的统一收益。

因此，本项目正式冻结：

- 会话收敛优先走 `434`
- 查询返回边界优先走 `461`
- 两者共享同一套方法论，但不互相吞并 owner

### 5.2 不把正常超预算定义为错误

Codex 的成熟价值之一，是把超预算优先视为“需要收敛”的系统状态，而不是用户错误。

本项目正式冻结：

- 正常超长结果应优先降级；
- 只有无法保持 fail-closed 返回时，才允许返回错误；
- 该语义对 `434` 的会话级 compact 和 `461` 的查询返回边界都适用。

### 5.3 不把上位意义写成过重前置条件

`462` 必须符合 `DEV-PLAN-003`，因此它虽然是上位方案，但不得要求：

- 先补 telemetry 全链路数据面才能实施；
- 先补 compacted item UI 可视化才能算闭环；
- 先补 memory pipeline 才允许查询返回边界收口；
- 先把所有能力都改造成统一的本地结果解释系统才能封板。

`462` 只冻结当前闭环真正需要的最小协同原则，其余能力继续后移。

## 6. 实施步骤

### 6.1 Step 1：冻结四类收益与 owner 分层

- [x] 在本计划中冻结产品层、系统层、查询层、评审层四类收益
- [x] 明确 `434/460/461/462` 的 owner 分层
- [x] 明确本计划不承接会话级内核和查询执行细节

### 6.2 Step 2：把查询返回边界口径正式压回 `461`

- [x] 确认 `461` 已冻结查询结果返回的 schema / fail-closed 边界
- [x] 确认 `461` 没有把这些能力重新扩写成新的会话级 compaction 子系统
- [x] 确认 `461` 的验收与 `462` 的上位收益表述一致
- [x] 冻结“`462` 先行对齐后，再进入 `461` 实施；`461` 实施完成时必须删除当前 capability-specific 硬编码特判”的迁移要求

### 6.3 Step 3：把 reviewer 口径显式化

- [x] 后续涉及 compact / summary / budget / 查询返回边界的 PR，评审时必须显式判断“它属于 `434` 还是 `461`”
- [x] reviewer 必须先检查是否走统一收敛入口，再检查局部展示格式
- [x] 对 capability-specific 特判默认给出 Warning，除非作者能证明其属于最小安全边界所必需

### 6.4 Step 4：Readiness 与文档收口

- [x] 更新文档地图与关联计划，使 `462` 可发现
- [x] 在对应 readiness/评审记录中引用 `462` 作为上位方案
- [x] 若后续进入 session-level advanced compaction / memory pipeline，再转由 `434` 子计划或后续 owner 方案承接，不在 `462` 继续膨胀

## 7. 验收标准

- [x] reviewer 能清楚区分 `434`、`460`、`461`、`462` 的 owner 边界，不再把 compact / summary / budget 问题混成一类实现细节。
- [x] `CubeBox` 的会话级上下文收敛、查询回答级结果收敛和数字助手权限边界形成连续叙事，而不是三份互相打架的计划。
- [x] 后续查询能力扩展时，默认先走 `461` 的统一收敛口径，而不是在回答主链继续补 capability-specific 特判。
- [x] 评审评论可以直接依据“是否走统一收敛入口”做判断，不再只能逐个扫描字符串拼装和分支特判。
- [x] `462` 没有把 remote compaction、memory pipeline、完整 telemetry、UI 可视化 compacted prompt 等后续能力提前写成首期前置。
- [x] `462` 已明确要求：`461` 的后续实现必须删除当前 `buildCubeboxQueryAnswer` / `summarizeStructuredQueryResult` 一类 capability-specific 硬编码分支，而不是与其并存。

## 8. 反模式与禁止项

- 不得把 `462` 写成对 `434/460/461` 的重复摘要或副本
- 不得借 `462` 之名重新定义 `434` 已完成的会话级 compaction 内核
- 不得把 `461` 的回答收敛再次扩写成会话级 compaction 子系统
- 不得把 Codex compact 的价值降格为“某个能力的摘要器优化”
- 不得把“统一收敛方案”膨胀成完整 telemetry、memory、UI 可视化、provider capability 平台
- 不得在 reviewer 口径上继续容忍“先补一个特判，后面再统一”的默认路径

## 9. 关联文档

- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
- `docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`
- `docs/dev-records/DEV-PLAN-462-READINESS.md`
