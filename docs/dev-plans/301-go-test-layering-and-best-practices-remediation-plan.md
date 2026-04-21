# DEV-PLAN-301：Go 测试分层整治与官方最佳实践落地方案

**状态**: 已完成（2026-04-08 CST）

补记（2026-04-08 CST）：

1. [X] `301` 的首轮目标仍视为已完成。
2. [X] 但恢复后的当前代码树里，`internal/server` 仍残留 `28` 个 `gap/coverage` 测试文件，其中 `18` 个曾在 `301` 文档中被写成“已收口”。
3. [X] 上述残留不再回灌到 `301` 范围，统一由 `DEV-PLAN-302` 增量承接：`docs/dev-plans/302-internal-server-residual-gap-coverage-closure-plan.md`

## 背景

`DEV-PLAN-300` 已完成对全仓测试体系的调查，确认当前存在以下核心问题：

1. [X] 测试压力过度集中在 `internal/server`。
2. [X] `*_coverage_test.go` / `*_gap_test.go` 等补洞型测试占比高。
3. [X] 缺少 `package xxx_test` 形式的黑盒测试。
4. [X] `t.Parallel()`、fuzz、benchmark 等官方标准能力使用不足。
5. [X] 全局状态覆盖、环境变量覆盖与重复 stub/fake 较多。

同时，基于 Go 官方资料补充核对后，可以确认本仓当前测试风格与 Go 官方推荐之间存在直接偏差，尤其体现在黑盒边界、子测试组织、并行友好性、fuzz 能力与集成覆盖率路径上。

本计划用于承接 `DEV-PLAN-300`，把“问题调查”推进为“实施方案”，形成后续分阶段整治的唯一事实源。

## 目标与非目标

### 核心目标

1. [X] 已建立全仓 Go 测试分层策略：`pkg/**` → `modules/*/services` → `internal/server`，并明确每层验证责任。
2. [X] 已将 Go 官方推荐的测试模式收敛为本仓标准做法：黑盒测试、子测试、并行测试、fuzz、benchmark、集成覆盖率。
3. [X] 已在不降低覆盖率门禁、不扩大排除项的前提下，收敛“补洞式测试”继续扩散。
4. [X] 已为后续 `internal/server` 与模块服务层重构降低测试摩擦。
5. [X] 已形成一组可复制的最小模板，供后续包级整改复用。

### 非目标

1. [ ] 本计划不在当前阶段修改 `100%` 覆盖率门禁阈值，也不新增覆盖率排除项。
2. [ ] 本计划不直接重写全仓测试；仅定义优先级、阶段边界、stopline 与验收口径。
3. [ ] 本计划不承担大规模 Playwright 场景重写；仅在 `Phase 4` 做与 Go 测试分层直接相关的共享 fixture/helper 收敛与职责分工衔接。
4. [ ] 本计划不新增 legacy 回退测试通道、双链路测试入口或旁路门禁。

## 事实源与官方依据

### 仓库内事实源

1. [X] 调查基线：`docs/dev-plans/300-test-system-investigation-report.md`
2. [X] CI/门禁 SSOT：`docs/dev-plans/012-ci-quality-gates.md`
3. [X] 仓库触发器矩阵：`AGENTS.md`
4. [X] 执行入口：`Makefile`、`scripts/ci/test.sh`、`scripts/ci/coverage.sh`

### Go 官方依据

1. [X] `testing` 包文档：<https://pkg.go.dev/testing>
2. [X] Go 官方博客《Using Subtests and Sub-benchmarks》：<https://go.dev/blog/subtests>
3. [X] Go 官方教程《Fuzzing》：<https://go.dev/doc/tutorial/fuzz>
4. [X] Go 官方博客《Code coverage for Go integration tests》：<https://go.dev/blog/integration-test-coverage>

## 设计原则

### 1. 黑盒优先，白盒克制

1. [ ] 对导出 API、纯函数、归一化/解析器、错误映射器，优先采用 `package xxx_test`。
2. [ ] 仅当需要验证未导出不变量、复杂状态推进或内部适配层时，才保留同包白盒测试。
3. [ ] 不再把“因为写起来快”作为默认使用同包测试的理由。

### 2. 场景优先，补洞文件收敛

1. [ ] 新增测试优先使用表驱动 + `t.Run` 组织业务场景。
2. [ ] 禁止继续以 `more` / `extra` / `gap` / `coverage` 命名追加同质测试文件，除非用户明确批准且有充分 stopline 理由。
3. [ ] 既有补洞文件的收敛方式是“按职责重组”，不是“换个名字继续堆”。

### 3. 并行测试只在隔离成立时启用

1. [ ] 仅纯函数、只读依赖、无共享状态的测试允许引入 `t.Parallel()`。
2. [ ] 使用 `t.Setenv` / `os.Setenv`、改包级变量、改全局 map/时间源的测试，不得与 parallel test 混用。
3. [ ] 并行化前必须先消除共享可变状态，而不是先加锁再把测试复杂度转嫁给测试本身。

### 4. 用标准能力覆盖鲁棒性与性能

1. [ ] 输入归一化、解析、分类、canonicalize、validator 这类路径优先补 fuzz。
2. [ ] 高频纯函数和热点小组件优先补 benchmark。
3. [ ] fuzz/benchmark 使用 Go 标准测试框架，不引入额外自定义 harness。

### 5. 集成覆盖率是补充，不是借口

1. [ ] 覆盖率可以来自单元、服务层、集成路径的组合，不要求所有行为都挤进白盒单测。
2. [ ] 但不得以“未来会被集成覆盖”为由长期放弃边界清晰的直接测试。
3. [ ] 覆盖率门禁继续遵循现有 SSOT，不做阈值降级与口径漂移。

## 边界冻结表（实施期强约束）

### A. 必须留在 `pkg/**` 的测试类型

1. [ ] 纯函数输入/输出归一化、解析、canonicalize、validator。
2. [ ] 导出错误类型与 helper 的对外行为断言。
3. [ ] 无 DB、无 HTTP、无全局运行态依赖的通用工具逻辑。

冻结要求：

1. [ ] 默认使用 `package xxx_test`。
2. [ ] 默认使用表驱动子测试。
3. [ ] 若无共享可变状态，默认启用并行子测试。
4. [ ] 若输入空间开放且失败样本价值高，补最小 fuzz。

### B. 必须下沉到 `modules/*/services` 的测试类型

1. [ ] 业务规则校验。
2. [ ] 默认值填充与策略选择。
3. [ ] 领域状态推进与错误分流。
4. [ ] 可通过 interface/port 隔离外部依赖的服务逻辑。

冻结要求：

1. [ ] 新增或重组测试时，不得再优先把上述规则留在 `internal/server`。
2. [ ] 若当前规则测试位于 `internal/server`，整改目标是迁回 service 层，而不是原地继续扩充。

### C. 必须留在 `internal/server` 的测试类型

1. [ ] HTTP 路由注册、方法/路径绑定、请求解析与响应编码。
2. [ ] 错误码到 HTTP 状态/文案/响应体的映射。
3. [ ] 租户上下文注入、中间件、authn/authz、RLS 边界的适配层断言。
4. [ ] 跨模块编排、handler 到 service 的组合调用。

冻结要求：

1. [ ] `internal/server` 只承载适配层与组合层职责，不作为业务规则的一线验证层。
2. [ ] 若某测试主要断言的是业务规则而不是协议/边界，则应迁往 service 层。

### D. 暂不迁移的允许项

1. [ ] 强依赖 SQL 形态且暂无稳定 seam 的历史测试。
2. [ ] 强依赖包级全局状态，短期内无法在不扩散抽象的前提下拆开的历史测试。
3. [ ] 为高风险回归兜底、但尚未完成下层承接的组合测试。

临时保留条件：

1. [ ] 必须在对应整改批次中登记“为何暂不迁移”。
2. [ ] 必须说明后续承接层级与退出条件。
3. [ ] 不得以“先留着更容易”为理由无限期保留。

## 分层策略（仓库标准）

### L1：`pkg/**` 工具层与纯函数层

适用对象：

1. [ ] 归一化函数
2. [ ] 解析器 / 分类器
3. [ ] 错误类型 / 鉴权 mode 解析
4. [ ] SetID / OrgCode / UUID / HTTP error 等通用包

标准做法：

1. [ ] 默认黑盒测试。
2. [ ] 默认表驱动子测试。
3. [ ] 满足隔离条件时默认并行子测试。
4. [ ] 对输入空间开放的函数补 fuzz。
5. [ ] 对热点函数补 benchmark。

### L2：`modules/*/services` 规则与端口层

适用对象：

1. [ ] 模块级写规则
2. [ ] 业务校验与默认值策略
3. [ ] 通过 interface/port 与存储、外部系统隔离的服务逻辑

标准做法：

1. [ ] 优先围绕 service 对外职责组织测试，而不是直接穿透到 handler/server。
2. [ ] 对重复 stub/fake 做共享 helper 收敛。
3. [ ] 优先通过可注入端口替代直接篡改包级变量。
4. [ ] 同一服务的规则测试应集中于单一职责文件，不再通过多个补洞文件分散维护。

### L3：`internal/server` 组合与适配层

适用对象：

1. [ ] HTTP handler
2. [ ] 路由拼装
3. [ ] 中间件
4. [ ] 跨模块组合编排

标准做法：

1. [ ] 仅验证适配层职责：协议、绑定、错误映射、组合调用、权限/租户边界。
2. [ ] 不再把本应在模块服务层验证的规则长期压在 `internal/server`。
3. [ ] 优先抽公共 fixture/helper，减少一次性 fake/recorder。
4. [ ] 对历史大文件以“拆职责”优先，而不是“分裂更多 coverage 文件”。

## 例外策略与失败语义

### 1. 何时允许继续使用白盒测试

1. [ ] 需要验证未导出不变量，且把不变量强行导出会破坏封装。
2. [ ] 需要验证复杂内部状态推进，而该状态本身就是实现契约的一部分。
3. [ ] 当前处于历史收口阶段，尚未完成 seam 提取，但已有明确迁移计划。

执行要求：

1. [ ] 必须在测试文件或对应整改记录中写明“为何不能改为黑盒”。
2. [ ] 不得仅因“写起来更快”保留白盒。

### 2. 何时可以不补 fuzz

1. [ ] 输入空间封闭，且边界枚举已明显完备。
2. [ ] 函数主要价值在固定编排，不存在高收益随机输入空间。
3. [ ] fuzz 会显著放大外部依赖成本，且无法在小范围内隔离。

执行要求：

1. [ ] 需要给出一句话理由，并说明为何常规边界测试足够。

### 3. 何时可以不补 benchmark

1. [ ] 函数不在热点路径，或执行频率/复杂度极低。
2. [ ] benchmark 数据无法指导任何后续决策。

执行要求：

1. [ ] 不补 benchmark 不需要额外审批，但在阶段记录中要标明“不适用”。

### 4. 并行测试失败时的处理顺序

1. [ ] 先识别共享状态点：环境变量、包级变量、全局 map、时间源、共享 DB/文件系统。
2. [ ] 若能低成本消除共享状态，则先整改再引入 `t.Parallel()`。
3. [ ] 若消除共享状态会引入更复杂抽象，则保留顺序执行，并登记“不并行”的原因。
4. [ ] 不允许通过全局锁、睡眠重试、增加时序脆弱性来“硬上并行”。

### 5. 服务层下沉失败时的处理顺序

1. [ ] 先判断测试断言的是业务规则还是适配层协议。
2. [ ] 若是业务规则但缺少 seam，则先补最小 interface/port。
3. [ ] 若补 seam 会显著扩大抽象面，则允许阶段性留在 `internal/server`，但必须登记退出条件。
4. [ ] 不允许在未做分类的情况下继续追加 `*_coverage_test.go`。

## 分阶段实施

### Phase 0：试点与模板冻结

1. [X] 基于 `pkg/orgunit.NormalizeOrgCode` 建立最小试点：
   - 黑盒测试；
   - 表驱动子测试；
   - `t.Parallel()`；
   - fuzz；
   - benchmark。
2. [X] 已冻结一个最小测试模板，作为后续 `pkg/**` 复制基线（2026-04-08 CST）。
3. [X] 已明确“哪些场景禁止 parallel / 禁止 Setenv 混用”的仓库内约束说明（2026-04-08 CST）。

退出条件：

1. [X] 至少 1 个 `pkg/**` 函数完成黑盒 + 子测试 + fuzz/benchmark 试点。
2. [X] 试点包含实际命令与通过结果。
3. [X] 已形成可复制模板。

Phase 0 证据：

1. [X] 试点测试文件：[normalize_blackbox_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/orgunit/normalize_blackbox_test.go)
2. [X] 常规测试：`go test ./pkg/orgunit -count=1` 通过（2026-04-08 CST）
3. [X] Fuzz：`go test ./pkg/orgunit -run '^$' -fuzz FuzzNormalizeOrgCode_BlackBox -fuzztime=3s` 通过（2026-04-08 CST）
4. [X] Benchmark：`go test ./pkg/orgunit -run '^$' -bench BenchmarkNormalizeOrgCode_BlackBox -benchmem -count=1` 通过（2026-04-08 CST）
5. [X] 已在本计划中冻结“最小 Go 测试模板 + parallel/Setenv stopline”，并同步执行日志（2026-04-08 CST）

Phase 0 最小模板冻结（2026-04-08 CST）：

1. [X] 基线样板以 [normalize_blackbox_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/orgunit/normalize_blackbox_test.go) 为主参考，后续 `pkg/**` 优先复制这类结构，而不是重新发明测试骨架。
2. [X] 最小模板约定：
   - `package xxx_test`，优先从导出边界做黑盒断言；
   - 顶层 `TestXxx_BlackBox(t *testing.T)` 承接行为簇，内部用表驱动 `t.Run(...)` 组织分支；
   - 只有当每个子测试都满足“无共享状态、无环境变量写入、无全局副作用”时，才允许在父/子测试使用 `t.Parallel()`；
   - 若输入空间开放且失败样本能提供真实回归价值，则补 `FuzzXxx_BlackBox(f *testing.F)`；
   - 若性能数据对后续决策有意义，则补 `BenchmarkXxx_BlackBox(b *testing.B)`；否则登记“不适用”理由。
3. [X] 最小模板冻结口径：
   - 优先让测试名表达职责簇，如 `TestNormalizeOrgCode_BlackBox`，而不是 `TestCoverage` / `TestMore` 一类兜底命名；
   - 表驱动用例需就地表达输入、期望与错误语义，避免通过额外 helper 隐藏断言意图；
   - fuzz 与 benchmark 是“标准能力补齐”，不是机械要求；未补时必须说明不适用理由。

parallel / Setenv 仓库内约束冻结（2026-04-08 CST）：

1. [X] 以下场景默认禁止与 `t.Parallel()` 混用：
   - 使用 `t.Setenv(...)` / `os.Setenv(...)`；
   - 修改包级变量、全局 map/registry、全局时间源、全局随机源或其他进程级共享状态；
   - 依赖共享 DB、共享文件路径、共享监听端口、共享工作目录等会产生时序耦合的外部资源。
2. [X] 若同一测试文件同时存在“可安全并行的纯函数分支”和“依赖 `Setenv` / 全局状态的分支”，应拆成不同顶层测试簇分别组织；不得在父测试统一 `t.Parallel()` 后再混入这些受限子测试。
3. [X] 遇到上述受限场景时，默认策略是保持顺序执行；不得用全局锁、睡眠重试或脆弱时序来“硬并行化”。
4. [X] 若后续能低成本消除共享状态，则先做隔离整改，再评估是否引入 `t.Parallel()`，而不是先并行再补救。

### Phase 1：`pkg/**` 清理与补强

目标包优先级：

1. [X] `pkg/setid` —— 已完成黑盒化样板（2026-04-08 CST）
2. [X] `pkg/authz` —— 已完成黑盒化样板（2026-04-08 CST）
3. [X] `pkg/httperr` —— 已完成黑盒化样板（2026-04-08 CST）
4. [X] `pkg/dict` —— 已完成“最小内部测试 + 黑盒行为测试”混合样板（2026-04-08 CST）
5. [X] `pkg/orgunit` —— 已完成 `ResolveOrgID/ResolveOrgCode/ResolveOrgCodes` 黑盒化样板（2026-04-08 CST）
6. [X] `pkg/uuidv7` —— 已完成 UUID 生成边界黑盒化样板（2026-04-08 CST）
7. [X] 当前仓库 `pkg/**` inventory 首轮盘点已完成，暂无更高优先级剩余工具包（2026-04-08 CST）

交付要求：

1. [X] 已为当前高价值导出边界补上黑盒测试。
2. [X] 已对适合 fuzz 的函数补最小 fuzz；其余包已登记“不适用”理由。
3. [X] 已对适合 benchmark 的函数补标准 benchmark；其余包已登记“不适用”理由。
4. [X] 未通过新增包级覆盖点来换取“看起来更容易写”的白盒测试。

退出条件：

1. [X] 至少完成 `pkg/setid`、`pkg/authz`、`pkg/httperr` 三个目标包中的两个。
2. [X] 每个完成的包都要有黑盒测试；若不做 fuzz/benchmark，需登记“不适用”理由。
3. [X] 不新增新的 `*_coverage_test.go` / `*_gap_test.go` 补洞文件。

本批次执行记录（2026-04-08 CST）：

1. [X] `pkg/setid/setid_test.go` 已改为 `package setid_test` 黑盒测试。
2. [X] 已将原先分散测试收敛为 `TestEnsureBootstrap_BlackBox` 与 `TestResolve_BlackBox` 两组子测试，并启用 `t.Parallel()`。
3. [X] 未补 fuzz/benchmark，理由：`pkg/setid` 当前仅为薄 DB wrapper；随机输入与性能数据对决策价值有限，优先保留黑盒边界与行为断言。
4. [X] 验证命令：
   - `go test ./pkg/setid -count=1`
   - `go test ./pkg/orgunit -count=1`

增量执行记录（2026-04-08 08:07 CST）：

1. [X] `pkg/authz/authz_test.go` 已改为 `package authz_test` 黑盒测试。
2. [X] `ModeFromEnv`、`NewAuthorizer`、`Authorize`、`SubjectFromRoleSlug`、`DomainFromTenantID` 已按行为场景重组为子测试。
3. [X] 未启用 `t.Parallel()`，理由：`ModeFromEnv` 测试依赖 `t.Setenv`，按 Go 官方 `testing` 约束不得与 parallel test 混用。
4. [X] 已补 `BenchmarkSubjectFromRoleSlug_BlackBox`。
5. [X] 未补 fuzz，理由：当前核心输入空间主要由显式枚举模式与文件加载错误组成，随机输入收益低于边界子测试。
6. [X] 验证命令：
   - `go test ./pkg/authz -count=1`
   - `go test ./pkg/authz -run '^$' -bench BenchmarkSubjectFromRoleSlug_BlackBox -benchmem -count=1`

增量执行记录（2026-04-08 08:16 CST）：

1. [X] 已核验 `pkg/httperr/httperr_test.go` 满足黑盒样板要求：
   - `package httperr_test`
   - 子测试
   - `t.Parallel()`
   - benchmark
2. [X] `pkg/dict` 已按例外策略完成混合样板：
   - 保留极小同包内部测试，守住全局注册表未配置路径；
   - 新增黑盒测试覆盖 `RegisterResolver`、`ResolveValueLabel`、`ListOptions` 导出行为；
   - benchmark 改为黑盒形态。
3. [X] `pkg/dict` 未启用 `t.Parallel()`，理由：依赖全局注册表 `registry`，并发化会引入共享状态耦合。
4. [X] `pkg/dict` 未补 fuzz，理由：主要复杂度不在开放输入空间，而在全局 resolver 注册语义；边界子测试收益更高。
5. [X] 验证命令：
   - `go test ./pkg/dict -count=1`
   - `go test ./pkg/dict -run '^$' -bench 'Benchmark(ResolveValueLabel_BlackBox|ListOptions_BlackBox)$' -benchmem -count=1`
   - `go test ./pkg/httperr -count=1`

增量执行记录（2026-04-08 13:35 CST）：

1. [X] `pkg/orgunit/resolve_test.go` 已改为 `package orgunit_test` 黑盒测试。
2. [X] 已将 `ResolveOrgID`、`ResolveOrgCode`、`ResolveOrgCodes` 按行为场景重组为 `*_BlackBox` 顶层测试，并启用 `t.Parallel()`。
3. [X] 已删除仅靠篡改包内 `orgCodePattern` 才能成立的 `NormalizeOrgCode` 白盒分支，`NormalizeOrgCode` 行为样板继续由 [normalize_blackbox_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/orgunit/normalize_blackbox_test.go) 单点承接。
4. [X] 未补新的 fuzz/benchmark，理由：
   - `NormalizeOrgCode` 的 fuzz/benchmark 已在 `Phase 0` 完成；
   - `ResolveOrgID/ResolveOrgCode/ResolveOrgCodes` 仍是薄 DB wrapper，随机输入与微基准收益有限，优先保留黑盒边界与错误映射断言。
5. [X] 验证命令：
   - `go test ./pkg/orgunit -count=1`

增量执行记录（2026-04-08 13:42 CST）：

1. [X] `pkg/uuidv7/uuidv7_test.go` 已改为 `package uuidv7_test` 黑盒测试。
2. [X] 已将 `New`、`NewString` 与读随机数失败分支统一命名为 `*_BlackBox`。
3. [X] 未启用 `t.Parallel()`，理由：错误路径测试需要临时替换全局 `rand.Reader`，按 `Phase 0` stopline 不应与 parallel test 混用。
4. [X] 未补 fuzz/benchmark，理由：
   - 当前核心价值在版本号、variant 与错误透传语义，而不在开放输入空间；
   - `uuidv7.New/NewString` 依赖时间与随机源，微基准对当前分层治理决策价值有限。
5. [X] 验证命令：
   - `go test ./pkg/uuidv7 -count=1`

增量执行记录（2026-04-08 08:11 CST）：

1. [X] `pkg/httperr/httperr_test.go` 已改为 `package httperr_test` 黑盒测试。
2. [X] 已按行为场景重组为 `TestIsBadRequest_BlackBox` 与 `TestNewBadRequest_BlackBox`，并启用 `t.Parallel()`。
3. [X] 已补 `BenchmarkIsBadRequest_BlackBox`。
4. [X] 未补 fuzz，理由：该包输入空间极窄，随机输入收益低于显式边界场景。
5. [X] 验证命令：
   - `go test ./pkg/httperr -count=1`
   - `go test ./pkg/httperr -run '^$' -bench BenchmarkIsBadRequest_BlackBox -benchmem -count=1`

### Phase 2：模块服务层下沉

目标模块优先级：

1. [X] `modules/orgunit/services` —— 已完成首个“从 server 下沉到 services”的规则样板（2026-04-08 CST）
2. [X] `modules/staffing/services` —— 历史样板已完成，但模块已由 `DEV-PLAN-450` 删除
3. [X] `modules/jobcatalog` —— 历史样板已完成，但模块已由 `DEV-PLAN-450` 删除
4. [X] `modules/person` —— 历史样板已完成，但模块已由 `DEV-PLAN-450` 删除

交付要求：

1. [ ] 为服务层建立按职责组织的测试入口。
2. [ ] 把现在挤在 `internal/server` 的一部分业务规则测试迁回服务层。
3. [ ] 收敛重复 stub/fake/recorder。
4. [ ] 为可注入依赖建立小而明确的 seam，避免继续改包级变量。

退出条件：

1. [X] 至少完成 1 个模块服务层样板包。
2. [X] 至少完成 1 组从 `internal/server` 下沉到 `modules/*/services` 的业务规则测试。
3. [X] 被选样板包中，不再新增新的补洞型测试文件。

增量执行记录（2026-04-08 08:23 CST）：

1. [X] 已将 `internal/server/orgunit_mutation_capabilities_api_test.go` 中直接断言 `orgunitservices.ResolvePolicy` 规则矩阵的测试迁回 `modules/orgunit/services/orgunit_mutation_policy_test.go`。
2. [X] 服务层新增覆盖：`CorrectEvent` 对未知 `TargetEffectiveEventType("UNKNOWN")` 的 allowed fields 回退语义。
3. [X] server 层保留 API 适配职责测试，仅保留参数校验、状态码映射、store/seam 错误分支与响应 envelope 断言。
4. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`
   - `go test ./internal/server -run 'TestHandleOrgUnitMutationCapabilitiesAPI_' -count=1`

增量执行记录（2026-04-08 08:28 CST）：

1. [X] 已继续清理 `internal/server/orgunit_append_capabilities_api_test.go` 中的服务规则重复断言。
2. [X] 服务层新增覆盖：`Create` 动作在 `CanAdmin=false && OrgAlreadyExists=true` 时的组合 deny reason 顺序 `FORBIDDEN,ORG_ALREADY_EXISTS`。
3. [X] server 层保留 API 契约断言：
   - 响应状态码；
   - capability 项是否 enabled/disabled；
   - fail-closed 字段响应；
   - seam 注入导致的 500 分支。
4. [X] server 层删除的重复规则细节断言：
   - `ORG_NOT_FOUND_AS_OF`
   - `ORG_ROOT_CANNOT_BE_MOVED`
   - `FORBIDDEN,ORG_ALREADY_EXISTS`
   - 空扩展字段 key 过滤细节
5. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`
   - `go test ./internal/server -run 'TestHandleOrgUnitAppendCapabilitiesAPI_' -count=1`

增量执行记录（2026-04-08 08:33 CST）：

1. [X] 已继续清理 `internal/server/orgunit_write_capabilities_api_test.go` 中的服务规则细节断言。
2. [X] 服务层新增覆盖：
   - `Create` 动作在启用 ext key 时的实际 allowed fields / payload key 映射；
   - `Create` 动作在 `CanAdmin=false && OrgAlreadyExists=true` 时的组合 deny reason 与 fail-closed 语义。
3. [X] server 层保留 API 契约断言：
   - 响应状态码；
   - success envelope；
   - disabled/enabled 表现；
   - fail-closed 字段响应；
   - seam 注入导致的 500 分支。
4. [X] server 层删除的重复规则细节断言：
   - `ORG_ALREADY_EXISTS`
   - `ORG_NOT_FOUND_AS_OF`
   - `ORG_EVENT_NOT_FOUND`
   - `ORG_EVENT_RESCINDED`
   - deny reason 中 `FORBIDDEN` 的具体存在性
5. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`
   - `go test ./internal/server -run 'TestHandleOrgUnitWriteCapabilitiesAPI_' -count=1`

增量执行记录（2026-04-08 08:40 CST）：

1. [X] 已将 `modules/orgunit/services/orgunit_write_unified_coverage_test.go` 重组为正式职责文件 `modules/orgunit/services/orgunit_write_unified_validation_test.go`。
2. [X] 已将 `modules/orgunit/services/orgunit_write_unified_more_coverage_test.go` 重组为正式职责文件 `modules/orgunit/services/orgunit_write_unified_error_paths_test.go`，并将 `modules/orgunit/services/orgunit_write_service_dict_coverage_test.go` 重组为 `modules/orgunit/services/orgunit_write_service_dict_test.go`。
3. [X] 本批次在 `modules/orgunit/services` 完成“删除 coverage 命名并按职责收口”的样板：
   - `Write` 统一入口的校验与分流行为分别落在 `orgunit_write_unified_validation_test.go` 与 `orgunit_write_unified_error_paths_test.go`；
   - 字典相关的 write service 行为落在 `orgunit_write_service_dict_test.go`。
4. [X] `modules/orgunit/services` 目录下当前已无残留 `orgunit_write_*coverage*_test.go` 文件。
5. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`

### Phase 3：`internal/server` 收口

1. [X] 已识别并持续按 handler、适配器、错误映射、权限边界分组重构高体量补洞文件（2026-04-08 CST）
2. [X] 已抽首批公共 fixture/helper，减少重复 setup（2026-04-08 CST）
3. [X] 已删除首组可证明属于“路由装配”职责的冗余补洞测试（2026-04-08 CST）
4. [ ] 保留必要的组合回归测试，但不承担全部业务规则验证责任。

退出条件：

1. [X] 已至少完成 1 个高体量文件族的按职责重组（2026-04-08 CST）
2. [X] 至少提取 1 套共享 fixture/helper。
3. [X] 至少删除或归并 1 组冗余补洞测试，并补齐对外契约不变性的说明。

增量执行记录（2026-04-08 08:52 CST）：

1. [X] 已将以下 `internal/server` 路由装配 coverage 文件按职责归并到 `internal/server/handler_test.go`：
   - `handler_assistant_routes_coverage_test.go`
   - `handler_dicts_routes_coverage_test.go`
   - `handler_orgunit_field_config_routes_coverage_test.go`
   - `handler_orgunit_field_policy_routes_coverage_test.go`
   - `handler_orgunit_write_routes_coverage_test.go`
2. [X] 已把原先绑在 coverage 文件中的共享 helper 提升为正式测试基建：
   - `mustGetwd`
   - `mustAllowlistPathFromWd`
   - `stringsReader`
   - `loginTenantAdminCookie`
3. [X] 职责边界已收敛：
   - `handler_test.go` 负责 `NewHandlerWithOptions` 的路由装配与认证后入口可达性；
   - 各 feature API 测试文件继续负责请求解析、状态码映射、错误分支与响应契约。
4. [X] 该批次是 `Phase 3` 的首个“按 handler 职责归并多个 coverage 文件”的样板。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestNewHandlerWithOptions_RouteFamilies_AreWired$' -count=1`

增量执行记录（2026-04-08 09:05 CST）：

> 当前态说明（2026-04-21 CST）：本节提到的 `setid_strategy_registry_api*`、`staffing`、`jobcatalog`、`person` 相关测试样板均已转为历史记录；对应模块或 API 已由 `DEV-PLAN-450` 删除，不再构成当前测试分层整改待办。

1. [X] 已将 `internal/server/setid_strategy_registry_api_coverage_test.go` 归并到 `internal/server/setid_strategy_registry_api_test.go`。
2. [X] 归并后的职责边界：
   - `setid_strategy_registry_api_test.go` 统一承载 strategy registry 的 normalize/validate/runtime/store/API error branches；
   - 不再通过单独 coverage 文件为同一 API 补分支。
3. [X] 本批次补齐的分支类型包括：
   - baseline candidate/intent override 的错误与缺基线路径；
   - definition/catalog 不一致分支；
   - PG store disable 的 baseline candidate 路径；
   - redundant check 失败导致的 API `500` 映射路径。
4. [X] 该批次是 `Phase 3` 的第二个样板，说明“coverage 文件归并到主 API 测试文件”可以在非路由类 server 文件上继续推进。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(CollectCapabilityResolutionItems_BaselineError|StrategySourceTypeForCapabilityKey_Baseline|ValidateStrategyRegistryItem_DefinitionAndCatalogBranches|ValidateStrategyRegistryDisableRequest_DefinitionAndCatalogBranches|ResolveFieldDecisionFromItems_EmptyCapabilityAndFieldMismatch|FieldDecisionSemanticallyEqual_DiffBranches|MergeStrategyItemsWithUpsert_Replace|EnsureStrategyResolvableAfterDisable_IntentUsesBaselineCandidate|SetIDStrategyRegistryPGStore_Disable_IntentIncludesBaselineCandidate|IsRedundantIntentOverride_ErrorAndNoBaselineBranches|HandleSetIDStrategyRegistryAPI_RedundantCheckError)$' -count=1`

增量执行记录（2026-04-08 09:18 CST）：

1. [X] 已将 `internal/server/dicts_extra_coverage_test.go` 归并到 `internal/server/dicts_api_test.go`。
2. [X] 归并后的职责边界：
   - `dicts_api_test.go` 统一承载 dict API、dict store、dict memory store 与 dict helper 的同职责分支；
   - 不再额外保留 `dicts_extra_coverage_test.go` 作为独立补洞入口。
3. [X] 本批次补齐的代表性分支包括：
   - `dictPGStore` 的 create/disable wrapper 与 submit event 错误分支；
   - `resolveDictSourceTenantTx` / `resolveValueLabelByTenant` / `getDictFromEventTx` 等 helper 分支；
   - `dictMemoryStore` 的租户/全局合并、停用冲突与 not found 分支。
4. [X] 该批次是 `Phase 3` 的第三个样板，说明 server 层的 store/helper coverage 文件同样可以并回主 API 测试文件。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(DictPGStore_ExtraCoverage|DictMemoryStore_ExtraCoverage)$' -count=1`

增量执行记录（2026-04-08 09:31 CST）：

1. [X] 已将 `internal/server/orgunit_field_metadata_api_106a_coverage_test.go` 归并到 `internal/server/orgunit_field_metadata_api_test.go`。
2. [X] 归并后的职责边界：
   - `orgunit_field_metadata_api_test.go` 统一承载 field configs / field options / enable-candidates / metadata helper 的 API、store stub 与 helper 分支；
   - 不再单独保留 `106a_coverage` 文件作为补洞入口。
3. [X] 本批次补齐的代表性分支包括：
   - `enable-candidates` 在 dict 过滤、org_code/setid 解析、setid source 映射上的分支；
   - `field-configs` 的 retry/invalid config/store error 分支；
   - `field-options` 的 resolver/setid/org_code error 分支；
   - `normalizeOrgUnitEnableDataSourceConfig*` 与 `orgUnitFieldConfigPresentation` 的辅助分支。
4. [X] 该批次是 `Phase 3` 的第四个样板，说明较大体量的 server feature coverage 文件也能并回主测试文件。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(HandleOrgUnitFieldConfigsEnableCandidatesAPI_BranchCoverage|HandleOrgUnitFieldConfigsAPI_WasRetryAndMethodNotAllowed|HandleOrgUnitFieldOptionsAPI_MoreBranches|NormalizeOrgUnitEnableDataSourceConfig_EntityMarshalErrorIsSkipped|NormalizeOrgUnitEnableDataSourceConfig_PlainInvalidJSON|NormalizeOrgUnitEnableDataSourceConfigForDictFieldKey_MoreBranches|OrgUnitFieldConfigPresentation_Branches)$' -count=1`

增量执行记录（2026-04-08 09:42 CST）：

1. [X] 已将 `internal/server/orgunit_nodes_store_coverage_test.go` 归并到 `internal/server/orgunit_nodes_store_test.go`。
2. [X] 归并后的职责边界：
   - `orgunit_nodes_store_test.go` 统一承载 orgunit node helper、memory store、部分 PG store audit helper 与日期边界分支；
   - 不再保留 `orgunit_nodes_store_coverage_test.go` 作为独立补洞入口。
3. [X] 本批次补齐的代表性分支包括：
   - request id / org id parse / include_disabled / active tab / target status 等 helper 分支；
   - memory store 的 CRUD、resolve、visibility、search、append facts 分支；
   - node audit helper 与 `ListNodeAuditEvents` 的主要边界分支。
4. [X] 该批次是 `Phase 3` 的第五个样板，说明 `orgunit_nodes` 这类基础设施+memory store 复合文件也能按职责并回主测试文件。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(NewOrgNodeRequestID_Prefix|ParseOrgID8|ParseOptionalOrgID8|CanEditOrgNodes|OrgUnitInitiatorUUID_PrefersValidPrincipalID|OrgUnitInitiatorUUID_FallsBackToTenantID|OrgNodeWriteErrorMessage|IncludeDisabledHelpers|OrgNodeAuditLimitAndTabFromURL|OrgUnitLabelsAndTargetStatus|OrgUnitMemoryStore_BasicCRUDAndResolve|OrgUnitMemoryStore_ResolveErrors|OrgUnitMemoryStore_VisibilityMethodsAndVersions|OrgUnitMemoryStore_SearchCandidatesAndRenameErrors|OrgUnitMemoryStore_SearchCandidates_LimitBreak|OrgUnitMemoryStore_CreateAndSearch_IDConversionErrors|OrgUnitMemoryStore_ResolveOrgCodes|OrgUnitMemoryStore_ResolveSetID|OrgUnitMemoryStore_MoveDisableSetBusinessUnitErrors|OrgUnitMemoryStore_ListChildrenAndDetailsAndSearch|VisibilityWrappers|ListNodeAuditEventsHelper|OrgUnitPGStore_ListNodeAuditEvents|OrgUnitMemoryStore_AppendFactsHelpers|OrgUnitPGStore_MaxEffectiveDateOnOrBefore|OrgUnitPGStore_MinEffectiveDate|OrgUnitMemoryStore_MaxEffectiveDateOnOrBefore|OrgUnitMemoryStore_MinEffectiveDate)$' -count=1`

增量执行记录（2026-04-08 09:51 CST）：

1. [X] 已将 `internal/server/orgunit_nodes_pgstore_coverage_test.go` 与 `internal/server/orgunit_nodes_pgstore_read_paths_coverage_test.go` 归并到 `internal/server/orgunit_nodes_store_test.go`。
2. [X] 归并后的职责边界：
   - `orgunit_nodes_store_test.go` 统一承载 orgunit nodes 的 memory store、PG store、read paths、audit helper 与 visibility/read-model 边界分支；
   - `orgunit_nodes_ext_snapshot_test.go` 继续仅负责 ext snapshot 专项职责。
3. [X] 本批次补齐的代表性分支包括：
   - `ResolveSetID` / `ResolveOrgID` / `ResolveOrgCode(s)` 与 `SetBusinessUnitCurrent` / `CorrectNodeEffectiveDate` 的 PG store 写侧分支；
   - `ListNodesCurrentWithVisibility` / `ListChildren(WithVisibility)` / `GetNodeDetails(WithVisibility)` / `SearchNode` / `SearchNodeCandidates` / `ListNodeVersions` 等读路径分支；
   - `recordRows` 与 `auditRows` 这类 PG 行扫描测试辅助类型的统一归位。
4. [X] 该批次是 `Phase 3` 的第六个样板，说明 `orgunit_nodes` 整个文件族已经可以在单一职责主测试文件下收口。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(OrgUnitPGStore_ResolveSetID|OrgUnitPGStore_ResolveOrgID|OrgUnitPGStore_ResolveOrgCode|OrgUnitPGStore_ResolveOrgCodes|OrgUnitPGStore_SetBusinessUnitCurrent_Errors|OrgUnitPGStore_SetBusinessUnitCurrent_Success|OrgUnitPGStore_SetBusinessUnitCurrent_Idempotent|OrgUnitPGStore_SetBusinessUnitCurrent_RollbackError|OrgUnitPGStore_CorrectNodeEffectiveDate_Errors|OrgUnitPGStore_CorrectNodeEffectiveDate_Success|OrgUnitPGStore_UsesQuotedCurrentTenantKey|OrgUnitPGStore_ListNodesCurrent_AndCreateCurrent|OrgUnitPGStore_ListBusinessUnitsCurrent|OrgUnitPGStore_ListBusinessUnitsCurrent_Errors|OrgUnitPGStore_ListNodesCurrent_Errors|OrgUnitPGStore_CreateNodeCurrent_Errors|OrgUnitPGStore_RenameMoveDisableCurrent|OrgUnitPGStore_ListNodesCurrentWithVisibility_Coverage|OrgUnitPGStore_ListChildren_AndVisibility_Coverage|OrgUnitPGStore_GetNodeDetails_AndVisibility_Coverage|OrgUnitPGStore_SearchNode_AndCandidates_AndVersions_Coverage)$' -count=1`

增量执行记录（2026-04-08 10:00 CST）：

1. [X] 已将 `internal/server/assistant_model_gateway_coverage_test.go` 归并到 `internal/server/assistant_model_gateway_more_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_model_gateway_more_test.go` 统一承载 deterministic provider、OpenAI provider adapter、gateway helper 与 resolve-intent retry/health 分支；
   - 不再为 model gateway 额外保留独立 coverage 文件。
3. [X] 本批次补齐的代表性分支包括：
   - deterministic provider 的 timeout/rate-limit/unavailable/probe 分支；
   - OpenAI provider 的 second-pass/fallback/probe/status mapping 分支；
   - gateway 的 helper normalize/decode、new gateway probe health、strict decode retry 分支。
4. [X] 该批次是 `Phase 3` 的第七个样板，也是 `assistant` 文件族的首个按职责并回主测试文件样板。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(AssistantDeterministicProviderAdapter_InvokeProbeMissingBranches|AssistantOpenAIProviderAdapter_InvokeSecondPassError|AssistantOpenAIProviderAdapter_ProbeBranches|AssistantModelGateway_HelperCoverage|AssistantModelGateway_NewGatewayProbeHealthAndURLCoverage|AssistantModelGateway_HelperCoverage_Additional|AssistantModelGateway_ResolveIntentRetriesTransientInvoke|AssistantModelGateway_ResolveIntentRetriesStrictDecodeFailure|AssistantModelGateway_BranchCoverage|AssistantModelGateway_RuntimeEndpointValidation|AssistantModelGateway_NoDeterministicSwapInTestEnv|AssistantModelGateway_ListProviderStatus_ProbeConnectivity|AssistantOpenAIProviderAdapter_InvokeAndParseContentArray|AssistantModelGateway_DefaultConfigFollowsRuntime|AssistantOpenAIProviderAdapter_ErrorBranches|AssistantModelGateway_ResolveIntentRetryAndGuardBranches|AssistantModelGateway_HelperFunctions_ExtraBranches)$' -count=1`

增量执行记录（2026-04-08 10:08 CST）：

1. [X] 已将 `internal/server/assistant_api_243_gap_test.go` 归并到 `internal/server/assistant_api_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_api_test.go` 统一承载 assistant conversation/turn API 的 create/confirm/commit 路径与 243 子计划引入的错误映射分支；
   - 不再为这组 API 额外保留单独的 `243_gap` coverage 文件。
3. [X] 本批次补齐的代表性分支包括：
   - create-turn handler 的 action spec missing / risk gate denied 映射；
   - turn action handler 的 confirm/commit clarification 与 route conflict 映射；
   - createTurn 在 route audit mismatch、capability unregistered、refresh version tuple error 等 helper/contract 分支。
4. [X] 该批次是 `Phase 3` 的第八个样板，也是 assistant API 主簇进入“按职责并回主测试文件”的首个子样板。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantAPI243_' -count=1`

增量执行记录（2026-04-08 10:15 CST）：

1. [X] 已将 `internal/server/assistant_api_reply_extra_test.go` 归并到 `internal/server/assistant_api_reply_more_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_api_reply_more_test.go` 统一承载 turn reply API 的 success、validation、not found/forbidden 与 error/fallback 分支；
   - 不再为 reply API 保留独立的 `extra` coverage 文件。
3. [X] 本批次补齐的代表性分支包括：
   - reply projection/fallback 路径下的 success 与 bad json / unsupported action 分支；
   - conversation not found / tenant mismatch / forbidden / turn not found 分支；
   - reply render target mismatch / provider unavailable / timeout / rate limit / secret missing 等回退分支。
4. [X] 该批次是 `Phase 3` 的第九个样板，说明 assistant API 可以继续按更小的 reply/action 子簇逐步收口。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestHandleAssistantTurnActionAPIReply' -count=1`

增量执行记录（2026-04-08 09:51 CST）：

1. [X] 已将 `internal/server/assistant_persistence_243_gap_test.go` 归并到 `internal/server/assistant_persistence_coverage_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_persistence_coverage_test.go` 统一承载 assistant persistence 的 conversation load、turn upsert、PG create/confirm/commit 与 243 子计划引入的 contract/clarification branches；
   - 不再为 persistence 243 子簇额外保留独立的 `243_gap` coverage 文件。
3. [X] 本批次补齐的代表性分支包括：
   - `createTurnPG` 的 route builder error、capability unregistered、route audit mismatch 与 action gate denied；
   - 语义编排路径下 clarification resume hook 不再恢复 local action/candidate selection 的回归断言；
   - `loadConversationTx` 保留 clarification、`upsertTurnTx` 的 clarification runtime invalid 与 route audit mismatch 分支。
4. [X] 该批次是 `Phase 3` 的第十个样板，说明 assistant 文件族可以继续按 persistence 子簇逐步收口，而不必一次吞掉整个主 coverage 文件族。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantPersistence243_' -count=1`

增量执行记录（2026-04-08 10:02 CST）：

1. [X] 已将 `internal/server/assistant_persistence_gap_test.go` 归并到 `internal/server/assistant_persistence_coverage_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_persistence_coverage_test.go` 统一承载 persistence helper、conversation load/list、confirm/commit idempotency、cursor codec，以及 create/confirm/commit/submit-commit-task 的 error matrix；
   - `assistant` persistence 文件族当前不再保留额外的 `gap`/`243_gap` 覆盖文件。
3. [X] 本批次补齐的代表性分支包括：
   - `createConversationPG` / `createTurnPG` / `loadConversationTx` 的 insert/query/scan/commit 错误矩阵；
   - `confirmTurnPG` / `commitTurnPG` 的 idempotency conflict/in-progress/done、权限漂移、persist/finalize/commit failure 与 expired branches；
   - `submitCommitTaskPG` 的 gate reject stopline、`executeCommitCoreTx` 的 committed persist path、conversation cursor codec/list pagination 边界。
4. [X] 该批次是 `Phase 3` 的第十一个样板，说明 assistant persistence 主簇已经可以在单一主测试文件下持续收口。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantPersistence_' -count=1`

增量执行记录（2026-04-08 10:11 CST）：

1. [X] 已将 `internal/server/assistant_api_gap_test.go` 归并到 `internal/server/assistant_api_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_api_test.go` 统一承载 assistant conversation/turn handler 的 tenant mismatch、runtime/model error mapping、request-in-progress/idempotency conflict、list/cursor、route handler mapping 与 helper branches；
   - `assistantReqWithContext` 与 `assistantDecodeErrCode` 已提升为主测试文件共享 helper，不再依附 `assistant_api_coverage_test.go`。
3. [X] 本批次补齐的代表性分支包括：
   - conversation detail/create/list 与 turn create/confirm/commit 的 tenant mismatch、pg begin failure、runtime config error、model provider timeout/rate-limit/provider unavailable 映射；
   - turn action 的 request in progress / idempotency conflict / confirmation required / route error mapping；
   - list pagination、cursor/path helper、knowledge runtime load error 与 non-business dry-run explain 分支。
4. [X] 该批次是 `Phase 3` 的第十二个样板，说明 assistant API 主簇已经可以沿主测试文件持续吸收子簇 `gap` 文件。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistant(ConversationHandlers_ExtraErrorBranches|ConversationTurns_ModelGatewayErrorMappings|TurnAction_RequestInProgressMappings|TurnAction_IdempotencyConflictMappings|TurnAction_RequiresIntentClarificationBeforeConfirm|ConversationTurns_RuntimeConfigErrorMappings|ServiceHelpers_PoolWrappersAndPathEdges|ConversationsList_HandlerAndServiceBranches|Helper_LatestTurnAndTaskActionPathBranches|CreateTurn_KnowledgeRuntimeErrorBranches|IntentClarificationAndDryRunNonBusinessCoverage|RouteHandlerMappings)$' -count=1`

增量执行记录（2026-04-08 10:14 CST）：

1. [X] 已将 `internal/server/assistant_api_coverage_test.go` 归并到 `internal/server/assistant_api_test.go`。
2. [X] 归并后的职责边界：
   - `assistant_api_test.go` 统一承载 assistant conversation/turn handler 的 coverage matrix、service helper/utilities、resolve-commit error mapping 与 confirm-window helper 分支；
   - assistant API 文件族当前不再保留额外的 `gap`/`coverage` 测试文件。
3. [X] 本批次补齐的代表性分支包括：
   - conversation create/detail/turn-create/turn-action 的 method/tenant/auth/json/path/error envelope matrix；
   - candidate resolve 变体、confirm/commit direct branch、risk gate、unsupported intent、drift/expiry/idempotent commit；
   - request body/path helper、resolve-commit error mapping 与 confirm window deadline/expiry helper。
4. [X] 该批次是 `Phase 3` 的第十三个样板，说明 assistant API 主簇已经可以在单一主测试文件下完成收口。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistant(ConversationHandlers_CoverageMatrix|TurnActionHandler_CoverageMatrix|ServiceHelpersAndUtilities|ResolveCommitError_CoverageMatrix|ConfirmWindowHelpers_Coverage)$' -count=1`

增量执行记录（2026-04-08 10:42 CST）：

1. [X] 已删除重复定义的 `internal/server/assistant_task_store_gap_test.go`，由既有 `internal/server/assistant_task_store_test.go` 继续作为正式主测试入口。
2. [X] 归并后的职责边界：
   - `assistant_task_store_test.go` 作为 task store 的正式主测试文件，统一承载 utility/validation、record/sql helper、submit/get/cancel/dispatch/execute 的分支矩阵；
   - `assistant_task_store` 文件族当前不再保留 `gap` 命名测试文件。
3. [X] 本批次补齐的代表性收口说明：
   - 本次不改生产代码，只删除和正式主测试文件重复的旧 `gap` 入口；
   - 其目的在于消除重复定义编译冲突，并把 task store 文件族稳定收敛到唯一主测试入口。
4. [X] 该批次是 `Phase 3` 的第十四个样板，说明“重复遗留 gap 文件”可以通过删除旧入口并保留唯一主测试文件完成收口。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantTaskStore_' -count=1`

增量执行记录（2026-04-08 11:20 CST）：

1. [X] 已采用 `assistant_task_store_test.go` 单文件细化策略，不新增 `assistant_task_store_*_test.go` 分文件。
2. [X] 主测试文件内部职责已稳定收敛为 6 个顶层主簇：
   - `UtilityValidationAndWrappers`
   - `RecordScanAndSQLHelpers`
   - `SubmitTaskPG`
   - `GetTaskAndCancelTaskPG`
   - `DispatchAndExecute`
   - `ResidualErrorMatrix`
3. [X] 本批次补齐的代表性收口说明：
   - 仅重排测试组织方式与 `t.Run(...)` 子层级，不改生产代码、不改断言语义；
   - 将原先偏重的 `get/cancel/dispatch/execute` 与 residual stopline 矩阵明确分簇，避免单一“大杂项”函数继续吸收新分支。
4. [X] 该批次是 `Phase 3` 第十四个样板后的延伸收口，说明“单一主测试文件”仍可继续通过文件内职责细化提升可维护性，而无需回退到多入口测试族。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantTaskStore_' -count=1`
   - `make check doc`

增量执行记录（2026-04-08 11:41 CST，历史样板）：

1. [X] 已完成 `internal/server` 高频规则测试的下沉候选盘点，并明确目标承接层。
2. [X] 首批高优先候选清单：
   - `modules/staffing/services`：
     `internal/server/staffing_canonicalize_test.go`；
     `internal/server/staffing_correct_rescind_store_test.go` 中“事务前输入校验 / memory store 语义校验”分支；
     `internal/server/staffing_test.go` 中 `UpsertPrimaryAssignmentForPerson` 的输入校验与默认值语义分支。
   - `modules/jobcatalog/services`：
     `internal/server/jobcatalog_test.go` 中 `jobCatalogView` / `ownerSetIDEditable` / `loadOwnedJobCatalogPackages` / `canEditDefltPackage` / `resolveJobCatalogView` / `normalizePackageCode` 等业务规则 helper 分支。
   - `modules/person/services`：
     `internal/server/person_test.go` 中 `normalizePernr` 及 `CreatePerson` / `FindPersonByPernr` 的输入校验分支。
3. [X] 本批次明确暂留 `internal/server` 的测试族：
   - `assistant_clarification_policy_test.go`、`assistant_create_policy_precheck_test.go` 仍与 assistant 运行态组合逻辑强绑定，当前无独立模块服务层承接；
   - `functional_area_governance_test.go` 仍绑定 capability 注册表与全局 functional area gate，暂不作为 `services` 首批样板。
4. [X] 已确定当时的下批实施顺序为：
   - `modules/staffing/services` → `modules/jobcatalog/services` → `modules/person/services`
5. [X] 验证命令：
   - `make check doc`

增量执行记录（2026-04-08 11:56 CST，历史样板）：

1. [X] 已完成 `modules/staffing/services` 首个服务层样板迁移，并将 `AssignmentsFacade` 从透传 facade 收敛为最小规则承接点。
2. [X] 本批次下沉内容：
   - 新增 `modules/staffing/services/assignment_rules.go`，统一承接 assignment upsert 输入规范化、correct/rescind 输入校验、JSON canonicalize 与确定性 event id 生成；
   - 新增 `modules/staffing/services/assignments_facade_test.go`，承接原 `internal/server/staffing_canonicalize_test.go` 以及 `internal/server/staffing_correct_rescind_store_test.go` / `internal/server/staffing_test.go` 中首批前置校验与默认值语义分支；
   - `internal/server/staffing.go` 与 `modules/staffing/infrastructure/persistence/assignment_pg_store.go` 改为复用 `services` 规则 helper，消除 server/persistence 双份 pure-rule 漂移。
3. [X] 测试边界重排结果：
   - 删除 `internal/server/staffing_canonicalize_test.go`，其 pure helper 断言改由 `modules/staffing/services` 承接；
   - 删除 `modules/staffing/infrastructure/persistence/assignment_pg_store_test.go`，避免 persistence 层继续承担 canonicalize 纯规则测试；
   - `internal/server` 保留 PG tx begin/set tenant/submit/commit 适配错误与 memory store 行为语义分支，不再重复承担已下沉的纯规则前置校验。
4. [X] 对外契约不变性说明：
   - 未新增新的 `staffing_*_test.go` server 入口，也未修改 public API、数据库或路由契约；
   - 仅调整测试分层与规则归属，外部 `go test` 调用方式保持不变。
5. [X] 当时的下一自然顺序是推进到 `modules/jobcatalog/services` 的 helper/规则样板迁移；该顺序现已失去当前执行意义。
6. [X] 验证命令：
   - `go test ./modules/staffing/services -count=1`
   - `go test ./modules/staffing/infrastructure/persistence -count=1`
   - `go test ./internal/server -run 'TestStaffing(PGStore_UpsertPrimaryAssignmentForPerson|PGStore_CorrectRescindAssignmentEvent|MemoryStore|Handlers_JSONRoundTrip)' -count=1`
   - `make check doc`

增量执行记录（2026-04-08 12:06 CST，历史样板）：

1. [X] 已完成 `modules/jobcatalog/services` 首个服务层样板迁移，并将 package/view 相关 helper 从 `internal/server` 收敛到服务层。
2. [X] 本批次下沉内容：
   - 新增 `modules/jobcatalog/services/view_rules.go`，统一承接 `normalizePackageCode`、owned package 编辑权限判断、`loadOwnedJobCatalogPackages` 与 `resolveJobCatalogView` 规则；
   - 新增 `modules/jobcatalog/services/view_rules_test.go`，承接原 `internal/server/jobcatalog_test.go` 中 `jobCatalogView`、`ownerSetIDEditable`、`loadOwnedJobCatalogPackages`、`canEditDefltPackage`、`resolveJobCatalogView`、`normalizePackageCode` 分支；
   - `internal/server/jobcatalog.go` 改为只负责 `Principal`/`SetID`/`JobCatalogPackage` 的上下文抽取与类型适配，纯规则逻辑统一委托 `modules/jobcatalog/services`。
3. [X] 测试边界重排结果：
   - `internal/server/jobcatalog_test.go` 删除已下沉的 pure-rule helper 测试段；
   - `internal/server` 保留 PG store、API handler、DB error/status 映射与 write path 行为测试，不再重复承担已进入 `services` 的 view/helper 规则。
4. [X] 对外契约不变性说明：
   - 未修改 public API、数据库 schema、路由或 jobcatalog handler 的入参/出参契约；
   - 仅调整规则承接层与测试分层，外部 `go test ./internal/server` 调用口径不变。
5. [X] 当时的下一自然顺序是推进到 `modules/person/services` 的 `normalizePernr` 与 create/find 输入校验样板迁移；该顺序现已失去当前执行意义。
6. [X] 验证命令：
   - `go test ./modules/jobcatalog/services -count=1`
   - `go test ./internal/server -run 'Test(JobCatalogStatusForError|JobCatalogPGStore_SetIDValidation|ResolveJobCatalogPackageByCode_PG|JobCatalogPGStore_ResolvePackages|HandleJobCatalogAPI_Get|HandleJobCatalogWriteAPI_Post)' -count=1`
   - `make check doc`

增量执行记录（2026-04-08 12:12 CST，历史样板）：

1. [X] 已完成 `modules/person/services` 首个服务层样板迁移，并将 `normalizePernr` 与 create/find 输入规范化从 `internal/server` 收敛到服务层。
2. [X] 本批次下沉内容：
   - 新增 `modules/person/services/person_rules.go`，统一承接 `NormalizePernr`、`PrepareCreatePerson`、`PrepareFindPersonByPernr` 以及最小 `Facade`；
   - 新增 `modules/person/services/person_rules_test.go`，承接原 `internal/server/person_test.go` 中 `normalizePernr` 与 create/find 输入校验分支；
   - `internal/server/person.go` 改为复用 `services` 规则 helper，PG store / memory store 不再内嵌重复的 `pernr` 与 display name 规范化逻辑。
3. [X] 测试边界重排结果：
   - `internal/server/person_test.go` 删除已下沉的 pure-rule 测试段：`TestNormalizePernr`、PG store create/find 输入校验、memory store 对应输入校验分支；
   - `internal/server` 保留 PG tx、query/scan/commit、memory duplicate/not-found、handler 响应码与 options 查询分支，不再重复承担已进入 `services` 的输入规则测试。
4. [X] 对外契约不变性说明：
   - 未修改 public API、数据库 schema、handler 路由或 `PersonStore` 对外调用口径；
   - 仅调整规则承接层与测试分层，`go test ./internal/server` 调用方式保持不变。
5. [X] 首批既定顺序 `staffing -> jobcatalog -> person` 已全部完成服务层样板迁移。
6. [X] 验证命令：
   - `go test ./modules/person/services -count=1`
   - `go test ./internal/server -run 'Test(PersonPGStore_ListPersons|PersonPGStore_CreatePerson|PersonPGStore_FindPersonByPernr|PersonPGStore_ListPersonOptions|PersonMemoryStore|HandlePersonsAPI_Branches|PersonHandlers)' -count=1`
   - `make check doc`

### Phase 4：E2E 衔接

1. [X] 已将 E2E 明确为验收层，而非 Go 单测缺口的兜底场（2026-04-08 CST）。
2. [X] 已抽首批共享 fixture/helper：`Kratos identity`、`superadmin 登录 + 建 tenant + tenant-admin session`（2026-04-08 CST）。
3. [X] 已补“Go 单测 / 服务层 / E2E”分工表，并挂到执行记录，避免同一规则在多层重复以低质量方式验证（2026-04-08 CST）。
4. [X] 已完成第二批高频 fixture/helper 收敛：`IAM session retry`、`evidence I/O`、`assistant task polling`（2026-04-08 CST）。
5. [X] 已完成第三批 baseline helper 收敛：`orgunit baseline` 的 list/detail/create/wait/root-detect 等基础操作已抽为共享 helper（2026-04-08 CST）。
6. [X] 已完成第四批 assistant conversation helper 收敛：`parseJSONSafe`、`parseResponseBody`、`latestAssistantTurn` 已抽为共享 helper（2026-04-08 CST）。
7. [X] 已完成 `Phase 4` 阶段性盘点：当前 7 个共享 helper 已形成稳定基建，后续默认不再继续机械上提高层场景 orchestrator（2026-04-08 CST）。
8. [ ] 少数 live spec 中更高层的 baseline report/probe 组装仍保留在各自 spec 内；仅当后续出现跨 spec 稳定复用证据时，才重新评估是否继续抽取。

退出条件：

1. [X] 已抽取 2 个高频重复 helper 为共享 fixture：`e2e/tests/helpers/kratos-identity.js`、`e2e/tests/helpers/superadmin-tenant.js`。
2. [X] 已明确 1 份“Go 单测/服务层/E2E 分工表”并挂到执行记录。

## Stopline 与禁止事项

1. [ ] 未经用户明确批准，不得降低覆盖率阈值、扩大覆盖率排除项或缩小统计范围。
2. [ ] 不得新增 `legacy` 测试入口、双链路 fixture 或兼容旧实现的回退分支。
3. [ ] 不得为了并行测试而引入更难维护的全局锁/睡眠重试。
4. [ ] 不得把 fuzz 当成替代常规边界测试的理由。
5. [ ] 不得在同一问题上继续新增 `*_coverage_test.go` / `*_gap_test.go`，除非先说明为什么不能按职责重组。

## 测试与覆盖率

### 覆盖率口径

1. [X] 沿用仓库现行 line coverage 口径。
2. [X] 阈值沿用 `config/coverage/policy.yaml` 的单主源定义。

### 统计范围

1. [X] 以现有 `scripts/ci/coverage.sh` 与 coverage policy 为 SSOT。
2. [X] 不在本计划中新增临时排除项或“专项豁免”。

### 本计划验收方式

1. [X] 文档层：本计划文档与 Doc Map 建立完成。
2. [X] 试点层：至少完成一个可运行的黑盒 + fuzz + benchmark 试点。
3. [X] 扩展层：各已执行阶段均附带实际测试命令与通过证据，未以“仅文档声明完成”代替实施。
4. [X] 每个已完成 phase 均满足本计划对应退出条件。

## Readiness 与执行记录要求

1. [ ] 每次进入新的 phase 前，需在对应 dev-plan 或 dev-record 中登记目标包、命令、结果与时间戳。
2. [ ] 任何“删死分支/删冗余测试”的动作，都必须同时说明不可达原因与对外契约不变性。
3. [ ] 任何“并行化改造”都必须明确列出被消除的共享状态点。
4. [ ] 任何“引入可注入依赖”的改造都必须说明为何这是更小、更可解释的 seam，而不是新抽象堆叠。
5. [ ] 每个整改批次都必须附一份“边界分类表”，说明哪些测试留在 `pkg` / `services` / `server`，以及例外项。

## 近期优先顺序

1. [X] 已完成 `pkg/setid` 的黑盒化与子测试收敛（2026-04-08 CST）。
2. [X] 已评估 `pkg/authz` 的 fuzz 适用性，并登记“不补 fuzz”的理由（2026-04-08 CST）。
3. [X] 已盘点并完成 `modules/orgunit/services` 中首批 `*_coverage_test.go` 的按职责重组样板（2026-04-08 CST）。
4. [X] 已列出 `internal/server` 中可下沉到服务层的高频规则测试清单；其中 `staffing -> jobcatalog -> person` 的历史样板当时已完成，后续又由 `DEV-PLAN-450` 删除（2026-04-08 / 2026-04-21 CST）。
5. [X] `assistant_task_store_test.go` 已采用单文件细化策略，并完成 utility / PG store / dispatch-execute / residual error matrix 的文件内职责重排（2026-04-08 CST）。
6. [X] 已完成 `modules/staffing/services` / `modules/jobcatalog/services` / `modules/person/services` 的历史样板迁移，但这些模块已不再属于当前仓库活体执行面。
9. [X] 已完成 `Phase 4` 首批 E2E helper 收敛：`Kratos identity` 与 `superadmin tenant session` 已抽为共享基建，并接入代表性 spec（2026-04-08 CST）。
10. [X] 已完成 `Phase 4` 第二批 E2E helper 收敛：`iam-session`、`evidence`、`assistant-task` 已抽为共享基建，并接入 `tp288* / tp290* / tp290b*` 代表性 live spec（2026-04-08 CST）。
11. [X] 已完成 `Phase 4` 第三批 E2E helper 收敛：`org-baseline` 已抽为共享基建，并接入 `tp288b` / `tp290b` 的 baseline org 初始化路径（2026-04-08 CST）。
12. [X] 已完成 `Phase 4` 第四批 E2E helper 收敛：`assistant-conversation` 已抽为共享基建，并接入 `tp288b / tp290b-chain / tp290b-neg`（2026-04-08 CST）。
13. [X] 已完成 `Phase 4` 阶段性封板：helper 基建边界、暂不继续抽象的场景层语义与后续 stopline 已冻结（2026-04-08 CST）。

## 交付物

1. [X] 本方案文档：`docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
2. [X] 后续 phase 的执行记录与证据文档已形成并收敛到 [dev-plan-301-execution-log.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-records/dev-plan-301-execution-log.md)
3. [X] 已形成可复用的测试模板与 E2E helper 收敛清单：
   - Go 最小模板主参考：[normalize_blackbox_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/orgunit/normalize_blackbox_test.go)
   - E2E helper：`e2e/tests/helpers/kratos-identity.js`、`e2e/tests/helpers/superadmin-tenant.js`、`e2e/tests/helpers/iam-session.js`、`e2e/tests/helpers/evidence.js`、`e2e/tests/helpers/assistant-task.js`、`e2e/tests/helpers/org-baseline.js`、`e2e/tests/helpers/assistant-conversation.js`
4. [X] 已形成 `Phase 4` 阶段性封板结论：稳定 helper 基建清单 + 暂不继续抽象的场景边界说明

## 关闭结论

1. [X] `Phase 0` 已完成最小模板冻结与 `parallel/Setenv` stopline 落板。
2. [X] `Phase 1` 已完成当前仓库 `pkg/**` inventory 首轮样板化收口。
3. [X] `Phase 2/3` 已完成首批服务层样板迁移与 `internal/server` 高频规则测试下沉。
4. [X] `Phase 4` 已完成与 Go 分层直接相关的 E2E helper 收敛，并冻结“不再机械继续抽高层 orchestrator”的边界。
5. [X] 本计划自本次回写起转为关闭态；后续若再出现新的跨层重复证据，应新开增量计划或在新的执行记录中承接，而不再继续扩张 `301` 的范围。
