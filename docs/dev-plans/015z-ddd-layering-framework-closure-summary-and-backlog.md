# DEV-PLAN-015Z：DDD 分层框架收尾盘点与封板清单（承接 DEV-PLAN-015B）

**状态**: 已完成（2026-04-09 18:28 CST，已按 `015Z4/015Z5/015Z6` 更新总账）

## 背景

自 `DEV-PLAN-015A/015B` 之后，仓库已连续完成一批 `015C` 到 `015Y` 的小切片收口，用于把 DDD 蓝图从“目录骨架”逐步推向“实际默认装配、模块入口、分层边界”。

截至当前，需要一份单文档把以下问题一次讲清：

1. [ ] `015` 系列已经实际完成了哪些收口。
2. [ ] 当前还剩哪些尾巴没有收。
3. [ ] 剩余工作大约占多少，以及风险主要集中在哪里。
4. [ ] 如果要继续推进，下一阶段应如何封板，而不是再零散追加切片。

## 目标与非目标

### 目标

1. [X] 汇总 `015C` 至 `015Y` 的实际完成面，给出单文档盘点。
2. [X] 明确 `015` 当前剩余 backlog，按风险与优先级分层。
3. [X] 给出“是否已进入后半程”的可辩护判断。
4. [X] 形成后续封板/继续实施时可直接引用的总清单。

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
- 后续实施切片（已归档）：`docs/archive/dev-plans/015d-*.md` 至 `docs/archive/dev-plans/015y-*.md`
- 仓库规则入口：`AGENTS.md`

## 当前总体判断

### 阶段性结论

当前 `015` 的更准确状态是：

1. [X] 已明显走出“只有蓝图、没有收口”的阶段。
2. [X] 已完成大部分低风险、可稳定切分的默认装配与模块入口收口。
3. [X] 原先最重的 `setid` / `dict` server store 已收缩为兼容薄壳，不再是主要厚实现入口。
4. [ ] 尚未完成最后一轮“总账归档 + 口径封板”收尾。

### 完成度判断

以 `015B` 的 `P0/P1/P2` 路线图为口径，当前完成度可保守判断为：

1. [X] `P0`：基本完成。
2. [X] `P1`：核心结构收口已完成，原先最重的 `setid/dict` 存量已压薄。
3. [X] `P2`：已完成第一轮可执行封板，新增代码已受更细颗粒度组合根门禁约束。

因此，当前更可辩护的管理口径是：

**`015` 的主体结构收口已基本完成，剩余约 5%~10% 的工作主要集中在文档口径封板、剩余薄壳是否继续下沉的取舍，以及 `links.go` 名实一致性收尾。**

## 已完成收口面

### 1. P0 止血已建立

1. [X] `015C` 已新增并接入 DDD layering P0 anti-drift gate。
2. [X] 当前新增代码已不能继续随意把模块级 PG store / Kernel 访问堆回 `internal/server`。
3. [X] 本轮后续切片已被该门禁真实约束，说明门禁已进入日常生效状态。
4. [X] `015Z4` 已补上 `ddd-layering-p2`，使“模块扩张时组合根不得继续空壳”进入实际门禁。

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
3. [X] `015Z1/015Z2/015Z3/015Z6` 已使 `setid` 的默认装配、PG 实现与兼容包装进一步回收到 `modules/orgunit/module.go`。

### 6. Dict 与 SetID 高耦合尾巴已显著压薄

1. [X] `015Z5` 已将 `dict` 的 PG/Memory helper 包装前移到 `modules/iam/module.go`，`internal/server/dicts_store.go` 收缩为兼容薄壳。
2. [X] `015Z6` 已将 `setid` 的 PG/Memory helper 包装前移到 `modules/orgunit/module.go`，`internal/server/setid.go` 收缩为兼容薄壳。
3. [X] `handler` 对 `setid/dict` 的默认装配已主要经模块入口返回实例，而不是继续在 `internal/server` 内堆主实现。

## 当前剩余尾巴

### A. 高优先级剩余项

#### A1. `internal/server` 仍保留少量兼容薄壳

当前 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go) 与 [`internal/server/dicts_store.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_store.go) 仍保留：

1. [ ] 兼容类型名：
   - [ ] `setidPGStore` / `setidMemoryStore`
   - [ ] `dictPGStore` / `dictMemoryStore`
2. [ ] 兼容构造入口：
   - [ ] `newSetIDPGStore(...)` / `newSetIDMemoryStore(...)`
   - [ ] `newDictPGStore(...)` / `newDictMemoryStore(...)`
3. [ ] 少量为照顾历史测试而保留的 helper / 状态镜像字段。

这意味着：

1. [X] 这些块已不再是主要生产实现入口，而是兼容薄壳。
2. [ ] 若要继续推进到更“纯”的终局，还可以再评估是否把这些兼容壳后移到测试侧。
3. [ ] 但这一步已不再是高收益主风险项。

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

其主要残量已集中在兼容薄壳与少量适配入口，因此这已不是“厚实现未迁走”的问题，而是“是否继续追求更纯边界”的取舍问题。

### C. P2 门禁剩余项

当前门禁状态可概括为：

1. [X] 已能阻断新增 `internal/server` 分层漂移。
2. [X] 已能阻断“模块扩张但 `module.go/links.go` 继续空壳”的新增漂移。
3. [ ] 仍未把全部理想边界都转成机器可验证规则。

仍待收口的 `P2` 项包括：

1. [ ] 进一步识别哪些规则无法由 `.gocleanarch.yml` 表达。
2. [ ] 评估是否需要追加脚本门禁来兜底：
   - [X] `module.go` / `links.go` 长期空壳扩散
   - [ ] `internal/server` 继续持有新的兼容壳/具体实现
   - [ ] 更细的 `infrastructure -> services` 回流
3. [ ] 将 `015/015B` 与实际 gate 口径做更正式的统一封板。

## 为什么剩余工作会更难

当前剩余尾巴之所以比前几刀更像“收尾”而不是“大块迁移”，核心原因是：

1. [X] 原先最大的 `setid` / `dict` 厚块已经收薄。
2. [ ] 剩余问题更多是“兼容壳是否继续后移”“links.go 是否名实一致”“门禁是否继续加严”。
3. [ ] 这些问题都存在收益递减，需要在“更纯边界”与“继续推进主业务计划”之间做取舍。

因此，当前 `015` 的难点已从“高耦合 server store 退出策略”转移为“最后一轮封板标准怎么定义”。

## 建议的封板顺序

### 方案一：继续完成 `015` 主收尾

若希望继续把 `015` 主体推到更完整状态，建议顺序为：

1. [ ] 先更新 `015Z` 总账与剩余百分比，形成新的封板口径。
2. [ ] 再决定是否要继续做：
   - [ ] 兼容壳后移到测试侧
   - [ ] `links.go` 名实一致性收尾
   - [ ] 更细的 P2/P3 反漂移门禁
3. [ ] 若这些收益有限，可直接将 `015` 视为“主体完成，进入维护期”。

### 方案二：先封板，再开新主计划

若希望避免 `015` 编号继续无限延长，也可以：

1. [ ] 以本文为界，将 `015` 定位为“第一阶段收口已基本完成”。
2. [ ] 将剩余 `links.go` / 薄壳纯化 / 更细门禁另立新主计划承接。
3. [ ] `015` 保留为“蓝图 + 第一轮结构收口 + 第一轮门禁封板”的总账。

## 当前推荐判断

基于当前代码树与已完成切片，我推荐的正式结论是：

1. [X] `015` 不应再被定性为“只有蓝图”。
2. [X] `015` 的第一阶段结构收口已经形成连续成果。
3. [ ] `015` 尚未彻底封板。
4. [X] 其剩余工作已不再是高耦合厚实现迁移，而主要是文档与边界纯化收尾。

因此，当前最稳妥的表述应固定为：

**`015` 已完成主体结构收口与第一轮门禁封板，剩余约 5%~10% 的工作主要集中在文档口径统一、兼容薄壳是否继续后移，以及 `links.go` 名实一致性收尾。**

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
3. [X] 仅执行文档门禁自检：`make check doc`（2026-04-09 16:33 CST，本地通过）。

## 验收标准

1. [X] 已形成一份单文档的 `015` 收尾盘点与 backlog 汇总。
2. [X] 文档已清楚区分“已完成收口面”与“剩余高耦合尾巴”。
3. [X] 文档已给出当前完成度与风险集中点的正式口径。
4. [X] 文档已加入 `AGENTS.md` 的 Doc Map，可作为后续封板或新主计划的引用入口。
