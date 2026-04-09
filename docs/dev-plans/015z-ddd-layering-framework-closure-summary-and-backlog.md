# DEV-PLAN-015Z：DDD 分层框架收尾盘点与封板清单（承接 DEV-PLAN-015B）

**状态**: 草拟中（2026-04-09 16:32 CST）

## 背景

自 `DEV-PLAN-015A/015B` 之后，仓库已连续完成一批 `015C` 到 `015Y` 的小切片收口，用于把 DDD 蓝图从“目录骨架”逐步推向“实际默认装配、模块入口、分层边界”。

截至当前，需要一份单文档把以下问题一次讲清：

1. [ ] `015` 系列已经实际完成了哪些收口。
2. [ ] 当前还剩哪些尾巴没有收。
3. [ ] 剩余工作大约占多少，以及风险主要集中在哪里。
4. [ ] 如果要继续推进，下一阶段应如何封板，而不是再零散追加切片。

## 目标与非目标

### 目标

1. [ ] 汇总 `015C` 至 `015Y` 的实际完成面，给出单文档盘点。
2. [ ] 明确 `015` 当前剩余 backlog，按风险与优先级分层。
3. [ ] 给出“是否已进入后半程”的可辩护判断。
4. [ ] 形成后续封板/继续实施时可直接引用的总清单。

### 非目标

1. [ ] 本文不直接迁移剩余 `setid` / `dict` 代码。
2. [ ] 本文不修改现有门禁实现。
3. [ ] 本文不宣告 `015` 已全部完成。
4. [ ] 本文不替代已有子计划；其职责是统一汇总与收尾判断。

## 事实源

- 主蓝图：`docs/dev-plans/015-ddd-layering-framework.md`
- 缺口评估：`docs/dev-plans/015a-ddd-layering-framework-implementation-gap-assessment.md`
- 收口路线图：`docs/dev-plans/015b-ddd-layering-framework-remediation-roadmap.md`
- P0 门禁：`docs/dev-plans/015c-ddd-layering-framework-p0-anti-drift-gate-plan.md`
- 后续实施切片：`docs/dev-plans/015d-*.md` 至 `docs/dev-plans/015y-*.md`
- 仓库规则入口：`AGENTS.md`

## 当前总体判断

### 阶段性结论

当前 `015` 的更准确状态是：

1. [X] 已明显走出“只有蓝图、没有收口”的阶段。
2. [X] 已完成大部分低风险、可稳定切分的默认装配与模块入口收口。
3. [ ] 尚未完成少数高耦合存量块的退出，尤其是 `setid` / `dict`。
4. [ ] 尚未完成 `P2` 层面的更细颗粒度门禁封板。

### 完成度判断

以 `015B` 的 `P0/P1/P2` 路线图为口径，当前完成度可保守判断为：

1. [X] `P0`：基本完成。
2. [X] `P1`：已完成主要部分，但仍有少量高耦合尾巴未收。
3. [ ] `P2`：只完成了第一层止血门禁，尚未完成更细规则的系统化兜底。

因此，当前更可辩护的管理口径是：

**`015` 已进入后半程，剩余约 20%~25% 的收尾工作，风险主要集中在 `setid/dict` 两块高耦合 server store，以及 `P2` 门禁封板。**

## 已完成收口面

### 1. P0 止血已建立

1. [X] `015C` 已新增并接入 DDD layering P0 anti-drift gate。
2. [X] 当前新增代码已不能继续随意把模块级 PG store / Kernel 访问堆回 `internal/server`。
3. [X] 本轮后续切片已被该门禁真实约束，说明门禁已进入日常生效状态。

### 2. Staffing 主要收口已完成

1. [X] `015D` 已修复 `infrastructure -> services` 反向依赖问题。
2. [X] `015I/O/P/Q/R/S/T/W` 已将 `staffing` 的 assignment / position 契约、memory/PG 默认装配与主要实现收回模块侧。
3. [X] `015U/V` 已把 `staffing` 的测试兼容壳逐步移出生产代码。
4. [X] 当前 `handler` 对 `staffing` 的默认装配已主要通过 `modules/staffing/module.go` 承接。

### 3. Person 主要收口已完成

1. [X] `015E/F/K/N` 已将 `person` 的默认装配、模块入口与 server 侧兼容包装收回模块侧或测试侧。
2. [X] 当前 `person` 在 `internal/server` 中已不再是主要历史风险点。

### 4. JobCatalog 主要收口已完成

1. [X] `015G/H/J/L/M/Y` 已将 `jobcatalog` 的 memory/PG store、默认装配、server 构造包装与部分服务适配入口迁回模块侧。
2. [X] 当前 `jobcatalog` 在 `internal/server` 中已不再保留主要 PG 生产实现。
3. [X] `internal/server/jobcatalog.go` 仍存在的逻辑已大幅收缩为 server 薄适配，而非模块内部实现主入口。

### 5. OrgUnit 组合根已开始承接职责

1. [X] `015X` 已使 `modules/orgunit/module.go` 不再为空壳。
2. [X] `OrgUnitWriteService` 的默认装配已从 `internal/server/handler.go` 收到模块入口。
3. [ ] 但 `orgunit` 仍未完全实现更广义的模块组合根收口。

## 当前剩余尾巴

### A. 高优先级剩余项

#### A1. SetID 仍是最大未收口块

当前 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go) 仍保留：

1. [ ] `setidPGStore`
2. [ ] `setidMemoryStore`
3. [ ] `newSetIDPGStore(...)`
4. [ ] `newSetIDMemoryStore(...)`
5. [ ] 多处直接 Kernel 写入口：
   - [ ] `submit_setid_event`
   - [ ] `submit_global_setid_event`
   - [ ] `submit_scope_package_event`
   - [ ] `submit_scope_subscription_event`
   - [ ] `submit_global_scope_package_event`

这意味着：

1. [ ] `setid` 仍是 `internal/server` 中最重的模块内部实现存量。
2. [ ] `handler` 仍直接承担 `setid` 的默认装配。
3. [ ] 若要继续推进 `015`，`setid` 几乎不可绕过。

#### A2. Dict 仍是第二块明显存量

当前 [`internal/server/dicts_store.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_store.go) 仍保留：

1. [ ] `dictPGStore`
2. [ ] `dictMemoryStore`
3. [ ] `newDictPGStore(...)`
4. [ ] `newDictMemoryStore(...)`

同时 [`internal/server/handler.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/handler.go) 仍直接装配 `DictStore`。

这说明：

1. [ ] `dict` 仍未进入模块侧组合根语义。
2. [ ] 它与 `setid` 一起构成当前 `internal/server` 最明显的剩余默认装配存量。

### B. 中优先级剩余项

#### B1. module.go / links.go 仍未全部“名实相符”

当前：

1. [X] `staffing/module.go`、`jobcatalog/module.go`、`person/module.go`、`orgunit/module.go` 已不同程度承接职责。
2. [ ] 各模块 `links.go` 仍大多为空壳。
3. [ ] `iam/module.go` / `iam/links.go` 仍为空壳。

这类问题当前不是主要风险源，但说明：

1. [ ] `015` 所要求的 Composition Root 语义尚未全量“名实相符”。
2. [ ] 仍需要一个收尾阶段决定哪些 `links.go` 应真正承接连接职责，哪些应保持空壳并明示其存在意义。

#### B2. `internal/server` 仍不是纯薄壳

虽然主要低风险块已经收掉，但 `internal/server` 仍未完全退化为：

1. [ ] 纯 HTTP / 路由 / context / auth / tenant 适配层。
2. [ ] 完全不持有模块内部 store 具体实现的薄壳。

其主要残量已集中在 `setid`、`dict`，因此这不是“到处零散”的问题，而是“少量剩余块仍偏重”的问题。

### C. P2 门禁剩余项

当前门禁状态可概括为：

1. [X] 已能阻断新增漂移。
2. [ ] 仍不能系统验证更细颗粒度目标。

仍待收口的 `P2` 项包括：

1. [ ] 进一步识别哪些规则无法由 `.gocleanarch.yml` 表达。
2. [ ] 评估是否需要追加脚本门禁来兜底：
   - [ ] `module.go` / `links.go` 长期空壳扩散
   - [ ] `internal/server` 继续持有模块内部实现
   - [ ] 更细的 `infrastructure -> services` 回流
3. [ ] 将 `015/015B` 与实际 gate 口径做更正式的统一封板。

## 为什么剩余工作会更难

当前剩余尾巴之所以没有像前几刀一样快速继续收掉，核心原因不是“不知道怎么做”，而是：

1. [ ] `setid` / `dict` 的测试对具体类型和内部字段耦合明显更深。
2. [ ] 若直接大搬，容易把“实现迁移”和“测试体系重写”绑成一刀，风险显著上升。
3. [ ] 这类块更适合先拆成新的专项子计划，再逐步退出，而不是继续沿用纯通用小切片命名直接硬推。

因此，当前 `015` 的难点已从“默认装配收口”转移为“高耦合 server store 退出策略”。

## 建议的封板顺序

### 方案一：继续完成 `015` 主收尾

若希望继续把 `015` 主体推到更完整状态，建议顺序为：

1. [ ] 先新建 `setid` 专项收口计划。
2. [ ] 将 `setid` 拆成更小的子域切片，而不是整体迁移。
   - [ ] 例如：默认装配入口先模块侧化
   - [ ] PG 实现再分块退出
   - [ ] 测试兼容壳最后后移
3. [ ] 再评估 `dict` 是否复用同样策略。
4. [ ] 最后补 `P2` 封板门禁。

### 方案二：先封板，再开新主计划

若希望避免 `015` 编号继续无限延长，也可以：

1. [ ] 以本文为界，将 `015` 定位为“第一阶段收口已基本完成”。
2. [ ] 剩余 `setid/dict` 另立新主计划承接。
3. [ ] `015` 仅保留为“蓝图 + 第一轮结构收口”的总账。

## 当前推荐判断

基于当前代码树与已完成切片，我推荐的正式结论是：

1. [X] `015` 不应再被定性为“只有蓝图”。
2. [X] `015` 的第一阶段结构收口已经形成连续成果。
3. [ ] `015` 尚未彻底封板。
4. [ ] 其剩余工作已集中为少量高耦合尾巴，而非全仓普遍失控。

因此，当前最稳妥的表述应固定为：

**`015` 已完成大部分结构收口，剩余约 20%~25% 收尾工作，主要集中在 `setid/dict` 两块高耦合 server store 与 `P2` 门禁封板。**

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`

本次变更为文档汇总类收尾：

1. [ ] 不修改生产代码。
2. [ ] 不调整覆盖率阈值与统计范围。
3. [ ] 仅执行文档门禁自检。

## 验收标准

1. [ ] 已形成一份单文档的 `015` 收尾盘点与 backlog 汇总。
2. [ ] 文档已清楚区分“已完成收口面”与“剩余高耦合尾巴”。
3. [ ] 文档已给出当前完成度与风险集中点的正式口径。
4. [ ] 文档已加入 `AGENTS.md` 的 Doc Map，可作为后续封板或新主计划的引用入口。
