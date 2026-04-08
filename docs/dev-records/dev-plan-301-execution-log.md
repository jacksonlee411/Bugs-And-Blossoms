# DEV-PLAN-301 执行日志：Go 测试分层整治与官方最佳实践落地

**状态**: 已完成（2026-04-08 CST）

补记（2026-04-08 CST）：

1. [X] 本执行日志记录的首轮实施批次仍保持完成态。
2. [X] 但恢复后的当前代码树里，`internal/server` 仍残留 `28` 个 `gap/coverage` 测试文件，其中 `18` 个在本日志或 `301` 主计划中曾被表述为“已收口”。
3. [X] 这批残留项不再回灌到 `301`，统一转由 `DEV-PLAN-302` 继续承接：`docs/dev-plans/302-internal-server-residual-gap-coverage-closure-plan.md`

## 1. 执行范围（与 301 对齐）

1. [X] 为 `DEV-PLAN-301 Phase 0/1` 建立专门执行记录入口。
2. [X] 完成 `pkg/**` 层首批黑盒化样板实施与验证。
3. [X] 已推进 `pkg/dict` 这一类存在全局状态例外的包，并验证 `301` 的例外策略可执行。
4. [X] 已在各阶段退出条件满足后，回写 `301` 计划状态与完成态。

## 2. Phase 0 试点记录

1. [X] 试点对象：`pkg/orgunit.NormalizeOrgCode`
2. [X] 试点文件：`pkg/orgunit/normalize_blackbox_test.go`
3. [X] 落地模式：
   - `package orgunit_test`
   - 表驱动子测试
   - `t.Parallel()`
   - fuzz
   - benchmark
4. [X] 验证命令：
   - `go test ./pkg/orgunit -count=1`
   - `go test ./pkg/orgunit -run '^$' -fuzz FuzzNormalizeOrgCode_BlackBox -fuzztime=3s`
   - `go test ./pkg/orgunit -run '^$' -bench BenchmarkNormalizeOrgCode_BlackBox -benchmem -count=1`

### 2.1 Phase 0 模板冻结补记

1. [X] 已把 [normalize_blackbox_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/orgunit/normalize_blackbox_test.go) 冻结为 `Phase 0` 的最小 Go 测试模板主参考。
2. [X] 冻结后的最小模板要点：
   - `package xxx_test` 黑盒入口；
   - 顶层行为簇测试 + 表驱动 `t.Run(...)`；
   - 仅在隔离成立时使用 `t.Parallel()`；
   - 按价值补最小 fuzz / benchmark，而不是机械全补。
3. [X] 已把仓库内并行 stopline 写回 `301`：
   - 使用 `t.Setenv(...)` / `os.Setenv(...)` 的测试不得与 `t.Parallel()` 混用；
   - 修改包级变量、全局 map/registry、时间源、随机源的测试不得与 `t.Parallel()` 混用；
   - 依赖共享 DB / 文件路径 / 监听端口 / 工作目录等外部资源的测试，若未显式隔离，也不得并行化。
4. [X] 处理原则已冻结：
   - 同文件若同时存在纯函数并行分支与 `Setenv`/全局状态分支，应拆成不同顶层职责簇；
   - 默认先保持顺序执行，只有在消除共享状态后才重新评估是否引入并行。
5. [X] 该补记不新增代码改动，只补齐 `Phase 0` 模板与 stopline 的完成态文档。
6. [X] 验证命令：
   - `make check doc`

## 3. Phase 1 已执行批次

### 3.1 `pkg/setid`

1. [X] 已将 `pkg/setid/setid_test.go` 改为 `package setid_test` 黑盒测试。
2. [X] 已按行为场景收敛为 `TestEnsureBootstrap_BlackBox` 与 `TestResolve_BlackBox`。
3. [X] 已启用 `t.Parallel()`；原因：该包测试不依赖环境变量与共享全局状态。
4. [X] 未补 fuzz/benchmark；理由：当前实现是薄 DB wrapper，随机输入与微基准收益有限。
5. [X] 验证命令：
   - `go test ./pkg/setid -count=1`

### 3.2 `pkg/authz`

1. [X] 已将 `pkg/authz/authz_test.go` 改为 `package authz_test` 黑盒测试。
2. [X] 已按行为场景收敛 `ModeFromEnv`、`NewAuthorizer`、`Authorize`、`SubjectFromRoleSlug`、`DomainFromTenantID`。
3. [X] 未启用 `t.Parallel()`；理由：`ModeFromEnv` 测试依赖 `t.Setenv`，按 Go 官方 `testing` 约束不得与 parallel test 混用。
4. [X] 已补 `BenchmarkSubjectFromRoleSlug_BlackBox`。
5. [X] 未补 fuzz；理由：核心输入空间已由枚举模式与文件加载错误覆盖，随机输入收益低。
6. [X] 验证命令：
   - `go test ./pkg/authz -count=1`
   - `go test ./pkg/authz -run '^$' -bench BenchmarkSubjectFromRoleSlug_BlackBox -benchmem -count=1`

### 3.3 `pkg/httperr`

1. [X] 已核验 `pkg/httperr/httperr_test.go` 当前满足黑盒样板要求：
   - `package httperr_test`
   - 子测试
   - `t.Parallel()`
   - benchmark
2. [X] 当前批次未对 `pkg/httperr` 追加代码变更；仅作为 Phase 1 已满足标准的现状核验项。

### 3.4 `pkg/dict`

1. [X] 已将 `pkg/dict` 收敛为“最小内部测试 + 黑盒行为测试”混合样板。
2. [X] 同包内部测试仅保留 `resolver 未配置 / nil resolver` 这类必须依赖内部全局状态的路径。
3. [X] 已新增黑盒测试覆盖 `RegisterResolver`、`ResolveValueLabel`、`ListOptions` 的导出行为与 trim 语义。
4. [X] benchmark 已改为黑盒形态：
   - `BenchmarkResolveValueLabel_BlackBox`
   - `BenchmarkListOptions_BlackBox`
5. [X] 未启用 `t.Parallel()`；理由：测试依赖全局注册表 `registry`，并行化会引入共享状态时序耦合。
6. [X] 未补 fuzz；理由：当前关键复杂度不在开放输入空间，而在 resolver 注册/未注册语义。
7. [X] 验证命令：
   - `go test ./pkg/dict -count=1`
   - `go test ./pkg/dict -run '^$' -bench 'Benchmark(ResolveValueLabel_BlackBox|ListOptions_BlackBox)$' -benchmem -count=1`

### 3.5 `pkg/orgunit`

1. [X] 已将 `pkg/orgunit/resolve_test.go` 改为 `package orgunit_test` 黑盒测试。
2. [X] 已按行为场景收敛 `ResolveOrgID`、`ResolveOrgCode`、`ResolveOrgCodes`，并统一命名为 `*_BlackBox`。
3. [X] 已启用 `t.Parallel()`；原因：当前测试仅使用局部 stub tx/rows，不依赖环境变量、包级全局状态或共享外部资源。
4. [X] 已删除只靠篡改包内 `orgCodePattern` 才能成立的 `NormalizeOrgCode` 白盒测试分支，归一化行为继续由 [normalize_blackbox_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/orgunit/normalize_blackbox_test.go) 单点承接。
5. [X] 未补新的 fuzz/benchmark；理由：`NormalizeOrgCode` 已在 `Phase 0` 提供标准样板，而 resolve 系列仍是薄 DB wrapper，随机输入与微基准收益有限。
6. [X] 验证命令：
   - `go test ./pkg/orgunit -count=1`

### 3.6 `pkg/uuidv7`

1. [X] 已将 `pkg/uuidv7/uuidv7_test.go` 改为 `package uuidv7_test` 黑盒测试。
2. [X] 已统一 `New`、`NewString` 与随机源错误路径测试命名为 `*_BlackBox`。
3. [X] 未启用 `t.Parallel()`；理由：错误路径测试需要临时替换全局 `rand.Reader`，按 `Phase 0` 并行 stopline 不应混用。
4. [X] 未补 fuzz/benchmark；理由：当前关键价值在 UUIDv7 版本位、variant 与错误透传，随机输入与微基准收益有限。
5. [X] 验证命令：
   - `go test ./pkg/uuidv7 -count=1`

## 4. 例外策略落地记录

1. [X] 已确认 `pkg/authz` 是“不得机械并行化”的例外样板：
   - 触发原因：`t.Setenv`
   - 处理方式：保持顺序执行，不用全局锁、睡眠重试或脆弱时序硬上并行
2. [X] `pkg/dict` 已完成“全局注册表例外”处理：
   - 保留极小同包内部测试守住全局状态路径，再补黑盒测试覆盖导出行为
   - 判定依据：完全外部化后将无法稳定验证“resolver 未配置”路径

## 5. 提交记录

1. [X] `296db4d5` `docs: add test remediation plans and baseline sample`
2. [X] `5a76323b` `test: convert setid tests to black-box style`
3. [X] `4a43d8e5` `test: convert authz tests to black-box style`
4. [X] `890ef6fc` `test: convert httperr tests to black-box style`
5. [X] `3a5af67d` `test: add dict mixed registry test pattern`
6. [X] `6ba202c1` `test: downshift orgunit policy coverage to services`
7. [X] `9dc2cdea` `test: trim orgunit append rule duplication in server`
8. [X] `8bea7d5a` `test: downshift orgunit write capability rules`
9. [X] `210b0916` `test: merge orgunit service coverage files by responsibility`
10. [X] `56ea931b` `test: consolidate handler route wiring coverage`
11. [X] `7e86c155` `test: merge setid strategy registry coverage cases`
12. [X] `c35e798d` `test: merge dict coverage cases into api tests`
13. [X] `ee29b618` `test: merge orgunit field metadata coverage cases`
14. [X] `5158cdc9` `test: merge orgunit nodes store coverage cases`
15. [X] `59a6995e` `test: merge orgunit pgstore coverage cases`
16. [X] `16ec5920` `test: merge assistant model gateway coverage cases`
17. [X] `bbd7fc0c` `test: merge assistant api gap coverage cases`
18. [X] `1a22130c` `test: merge assistant persistence gap cases`
19. [X] `61ccef19` `test: merge assistant persistence coverage cases`
20. [X] `78057d4f` `test: merge assistant api gap cases`
21. [X] `f6eca39a` `test: merge assistant api coverage cases`
22. [X] `assistant_task_store_gap_test.go` 命名收敛样板已完成，并在后续延伸收口中并入正式主测试文件与单文件职责细化记录。

## 6. 当前结论

1. [X] `DEV-PLAN-301 Phase 1` 的首个退出条件已满足：`pkg/setid` 与 `pkg/authz` 两个目标包已完成黑盒化样板。
2. [X] `301` 中定义的例外策略不是纸面规则，已经在 `pkg/authz` 上得到首次验证。
3. [X] `pkg/dict` 的“最小内部测试 + 黑盒行为测试”混合样板已完成，并已回写到 `301`。
4. [X] `pkg/httperr` 已核验满足 Phase 1 黑盒样板要求。
5. [X] `pkg/orgunit` 已完成 resolve 边界黑盒化样板，并回收一段依赖内部正则篡改的白盒测试。
6. [X] `pkg/uuidv7` 已完成 UUID 生成边界黑盒化样板，并提供“全局随机源不可并行”的 stopline 实例。
7. [X] 当前仓库 `pkg/**` inventory 首轮样板化已完成，现有 6 个工具包均已纳入黑盒/例外策略口径。
8. [X] `modules/orgunit/services` 已完成首个从 `internal/server` 下沉的规则样板：`ResolvePolicy` 的未知事件回退规则现仅由服务层测试承载。
9. [X] `staffing -> jobcatalog -> person` 首批服务层样板顺序已完成。
10. [X] 已进入 `Phase 4`，并完成前两批 E2E fixture/helper 与分层职责边界收口。
11. [X] 已把两份 live spec 中重复的 baseline org/unit 初始化逻辑收敛到共享 helper。
12. [X] 已把 assistant conversation 的基础 helper 收敛为共享基建。
13. [X] 已完成 `Phase 4` 阶段性盘点，并冻结“默认不再继续机械抽高层 orchestrator”的边界。
14. [X] `301` 已转为关闭态；后续若再推进测试分层，只在发现新的跨层重复证据时另开增量计划或新执行记录承接，而不再机械扩大 `301` 范围。

## 36. 关闭记录

1. [X] 关闭判断：
   - `Phase 0` 的模板冻结、`parallel/Setenv` stopline 与证据链已完成；
   - `Phase 1` 的当前仓库 `pkg/**` inventory 首轮样板化已完成；
   - `Phase 2/3` 的服务层样板迁移与 `internal/server` 高频规则测试下沉已完成；
   - `Phase 4` 的 E2E helper 收敛已完成并封板。
2. [X] 关闭范围说明：
   - 本次关闭并不表示未来不再改进测试体系；
   - 仅表示 `DEV-PLAN-301` 这份“首轮分层整治与官方最佳实践落地”计划已完成其既定目标与 stopline。
3. [X] 后续承接原则：
   - 若后续只是在既有边界内做零星测试整理，可直接在对应功能计划或执行记录中处理；
   - 若再次出现明显的跨层重复、补洞文件回潮或新的仓库级测试规则缺口，应新开增量计划，而不是继续扩张本计划。
4. [X] 关闭验证：
   - `make check doc`

## 7. Phase 2 首个样板记录

1. [X] 目标：把不应留在 server 层的服务规则矩阵断言迁回 `modules/orgunit/services`。
2. [X] 执行动作：
   - 在 `modules/orgunit/services/orgunit_mutation_policy_test.go` 增补未知目标事件 `UNKNOWN` 的 allowed fields 回退测试；
   - 删除 `internal/server/orgunit_mutation_capabilities_api_test.go` 中直接调用 `orgunitservices.ResolvePolicy(...)` 的重复规则矩阵测试。
3. [X] 边界结论：
   - 服务层负责 `ResolvePolicy` 的规则矩阵与回退语义；
   - server 层负责 API 参数校验、状态码映射、错误分支与 response envelope。
4. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`
   - `go test ./internal/server -run 'TestHandleOrgUnitMutationCapabilitiesAPI_' -count=1`

## 8. Phase 2 第二个样板记录

1. [X] 目标：继续减少 `internal/server` 对 append/mutation 业务规则细节的重复断言。
2. [X] 执行动作：
   - 在 `modules/orgunit/services/orgunit_mutation_policy_test.go` 增补 `Create` 动作的组合 deny reason 规则：`FORBIDDEN,ORG_ALREADY_EXISTS`；
   - 在 `internal/server/orgunit_append_capabilities_api_test.go` 删除纯服务规则细节断言，仅保留 API 契约断言。
3. [X] 边界结论：
   - 服务层负责组合 deny reason、根节点移动限制、空扩展字段过滤等规则细节；
   - server 层负责 envelope、状态码、disabled/fail-closed 表现与 seam 错误分支。
4. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`
   - `go test ./internal/server -run 'TestHandleOrgUnitAppendCapabilitiesAPI_' -count=1`

## 9. Phase 2 第三个样板记录

1. [X] 目标：继续减少 `internal/server` 对 write capabilities 业务规则细节的重复断言。
2. [X] 执行动作：
   - 在 `modules/orgunit/services/orgunit_write_capabilities_test.go` 增补 `Create` 动作的 ext key / payload key 实际行为与组合 deny reason + fail-closed 语义；
   - 在 `internal/server/orgunit_write_capabilities_api_test.go` 删除 exact deny reason 与规则细节断言，仅保留 API 契约断言。
3. [X] 边界结论：
   - 服务层负责 allowed fields、payload key 映射、组合 deny reason 与 fail-closed 规则；
   - server 层负责 envelope、状态码、enabled/disabled 表现与 seam 错误分支。
4. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`
   - `go test ./internal/server -run 'TestHandleOrgUnitWriteCapabilitiesAPI_' -count=1`

## 10. Phase 2 第四个样板记录

1. [X] 目标：把 `modules/orgunit/services` 中的补洞型 coverage 文件按职责收口为正式测试文件，避免继续分散维护。
2. [X] 执行动作：
   - 将 `orgunit_write_unified_coverage_test.go` 重组为 `orgunit_write_unified_validation_test.go`；
   - 将 `orgunit_write_unified_more_coverage_test.go` 重组为 `orgunit_write_unified_error_paths_test.go`；
   - 将 `orgunit_write_service_dict_coverage_test.go` 重组为 `orgunit_write_service_dict_test.go`。
3. [X] 边界结论：
   - `orgunit_write_unified_validation_test.go` 负责 `Write(...)` 统一入口的校验与基础分流；
   - `orgunit_write_unified_error_paths_test.go` 负责 `Write(...)` 的错误路径矩阵；
   - `orgunit_write_service_dict_test.go` 负责 write service 的字典解析、修正日期与 ext payload 规则。
4. [X] 结果：
   - `modules/orgunit/services` 目录下当前已无残留 `orgunit_write_*coverage*_test.go`；
   - 这是 `DEV-PLAN-301` 中首个“删除 coverage 命名并改为职责测试文件”的服务层样板。
5. [X] 验证命令：
   - `go test ./modules/orgunit/services -count=1`

## 11. 当前结论补充

1. [X] `modules/orgunit/services` 已完成首轮补洞文件收敛，证明 `301` 中“按职责重组而非继续堆 coverage 文件”的策略可直接落地。
2. [X] `internal/server` 已完成首个路由装配文件族的职责归并样板，证明 `Phase 3` 可以先从边界最清晰的 handler family 启动。
3. [X] `setid_strategy_registry_api_coverage_test.go` 已成功并回主测试文件，证明 `Phase 3` 能够处理非路由类的 API/store 复合测试族。
4. [X] `dicts_extra_coverage_test.go` 已成功并回主测试文件，说明 `dict` 这一类 API/store/helper 复合测试族也能按职责收口。
5. [X] `orgunit_field_metadata_api_106a_coverage_test.go` 已成功并回主测试文件，证明较大体量的 feature coverage 文件也可以按职责并入主测试入口。
6. [X] `orgunit_nodes_store_coverage_test.go` 已成功并回主测试文件，说明 `orgunit_nodes` 这类基础设施+memory store 复合测试族也能按职责收口。
7. [X] `orgunit_nodes_pgstore_coverage_test.go` 与 `orgunit_nodes_pgstore_read_paths_coverage_test.go` 已成功并回主测试文件，`orgunit_nodes` 文件族的主干 coverage 文件已经收口完成。
8. [X] `assistant_model_gateway_coverage_test.go` 已成功并回主测试文件，`assistant` 文件族已经拿到首个可复制的收口样板。
9. [X] `assistant_api_243_gap_test.go` 已成功并回主测试文件，说明 assistant API 主簇可以先按子计划/错误映射子簇分步收口。
10. [X] `assistant_api_reply_extra_test.go` 已成功并回主测试文件，说明 assistant API 不仅能按子计划收口，也能按 reply/action 小簇继续细拆。
11. [X] `assistant_persistence_243_gap_test.go` 已成功并回主测试文件，说明 assistant persistence 主簇也可以先按 243 子计划与 PG persistence 子簇分步收口。
12. [X] `assistant_persistence_gap_test.go` 已成功并回主测试文件，说明 assistant persistence 主簇已经可以在单一主测试文件下继续收口。
13. [X] `assistant_api_gap_test.go` 已成功并回主测试文件，说明 assistant API 主簇可以继续沿 `assistant_api_test.go` 吸收 handler/helper 子簇。
14. [X] `assistant_api_coverage_test.go` 已成功并回主测试文件，assistant API 文件族当前已完成主干收口。
15. [X] `assistant_task_store_gap_test.go` 已删除，assistant task store 文件族不再保留 `gap` 命名入口。
16. [X] `assistant_task_store_test.go` 继续作为唯一正式主测试文件，承接 utility / PG store / dispatch-execute / residual error matrix 的单文件职责组织。
17. [X] 已盘点 `internal/server` 中可下沉到服务层的高频规则测试清单，并锁定下批顺序 `staffing -> jobcatalog -> person`。

## 12. Phase 3 首个样板记录

1. [X] 目标：把 `internal/server` 中分散的“路由是否装配” coverage 文件按 handler 职责归并，避免每个 feature 各自维护一份登录+路由探活样板。
2. [X] 执行动作：
   - 将 5 个 `handler_*_routes_coverage_test.go` 文件归并到 `handler_test.go`；
   - 提升共享 helper：`mustGetwd`、`mustAllowlistPathFromWd`、`stringsReader`、`loginTenantAdminCookie`。
3. [X] 边界结论：
   - `handler_test.go` 负责 `NewHandlerWithOptions` 的装配默认值、路由注册与认证后入口可达性；
   - 具体 feature 的 API 契约仍留在 `assistant_api_test.go`、`orgunit_write_api_test.go` 等文件中。
4. [X] 对外契约不变性说明：
   - 未修改任何生产代码，仅改变测试组织方式；
   - 保留原先对 `assistant`、`dict`、`orgunit field config`、`orgunit field policy`、`orgunit write` 五组入口的可达性断言与代表性状态码断言。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestNewHandlerWithOptions_RouteFamilies_AreWired$' -count=1`

## 13. Phase 3 第二个样板记录

1. [X] 目标：把 `setid_strategy_registry` 这类非路由类 server coverage 文件并回主 API 测试文件，验证 `Phase 3` 不只是整理装配层。
2. [X] 执行动作：
   - 将 `setid_strategy_registry_api_coverage_test.go` 归并到 `setid_strategy_registry_api_test.go`；
   - 保留原有主测试文件中的 runtime/store/API 责任，不新增新的 coverage 入口。
3. [X] 边界结论：
   - `setid_strategy_registry_api_test.go` 统一承载 normalize、validate、runtime、PG store 与 API error mapping；
   - 不再把 baseline candidate / redundant intent override / catalog mismatch 这类同职责分支散落在 coverage 文件中。
4. [X] 对外契约不变性说明：
   - 未改动任何生产代码；
   - 保留原有 `500` / `422` / disable 语义与 redundant-check 失败映射断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(CollectCapabilityResolutionItems_BaselineError|StrategySourceTypeForCapabilityKey_Baseline|ValidateStrategyRegistryItem_DefinitionAndCatalogBranches|ValidateStrategyRegistryDisableRequest_DefinitionAndCatalogBranches|ResolveFieldDecisionFromItems_EmptyCapabilityAndFieldMismatch|FieldDecisionSemanticallyEqual_DiffBranches|MergeStrategyItemsWithUpsert_Replace|EnsureStrategyResolvableAfterDisable_IntentUsesBaselineCandidate|SetIDStrategyRegistryPGStore_Disable_IntentIncludesBaselineCandidate|IsRedundantIntentOverride_ErrorAndNoBaselineBranches|HandleSetIDStrategyRegistryAPI_RedundantCheckError)$' -count=1`

## 14. Phase 3 第三个样板记录

1. [X] 目标：把 `dicts` 这一组 API/store/helper 的 extra coverage 文件并回主测试文件，验证非 orgunit / non-route 测试族也能按职责收口。
2. [X] 执行动作：
   - 将 `dicts_extra_coverage_test.go` 归并到 `dicts_api_test.go`；
   - 保留原有 `dicts_release_api_test.go`、`dicts_release_test.go` 的 release 专项职责，不把 release 语义混入 dict CRUD/API 测试入口。
3. [X] 边界结论：
   - `dicts_api_test.go` 统一承载 dict CRUD、dict value、dict helper、dict store 与 dict memory store 的同职责分支；
   - `dicts_release_*` 文件继续只承载 baseline release 预览/发布相关职责。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 create/disable、helper error branch、memory store 冲突/缺失分支断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(DictPGStore_ExtraCoverage|DictMemoryStore_ExtraCoverage)$' -count=1`

## 15. Phase 3 第四个样板记录

1. [X] 目标：把 `orgunit field metadata` 这类较大体量的 feature coverage 文件并回主测试文件，验证 `Phase 3` 可处理更接近真实业务特性的 server 文件族。
2. [X] 执行动作：
   - 将 `orgunit_field_metadata_api_106a_coverage_test.go` 归并到 `orgunit_field_metadata_api_test.go`；
   - 在主测试文件中补齐该 coverage 文件依赖的 dict/org_code/setid stub 类型。
3. [X] 边界结论：
   - `orgunit_field_metadata_api_test.go` 统一承载 field config、field option、enable-candidates 与 metadata helper 的同职责分支；
   - 不再把 `106a` 阶段性 coverage 分支散落在独立测试文件中。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 retry、status mapping、dict/setid source 映射与 helper 边界断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(HandleOrgUnitFieldConfigsEnableCandidatesAPI_BranchCoverage|HandleOrgUnitFieldConfigsAPI_WasRetryAndMethodNotAllowed|HandleOrgUnitFieldOptionsAPI_MoreBranches|NormalizeOrgUnitEnableDataSourceConfig_EntityMarshalErrorIsSkipped|NormalizeOrgUnitEnableDataSourceConfig_PlainInvalidJSON|NormalizeOrgUnitEnableDataSourceConfigForDictFieldKey_MoreBranches|OrgUnitFieldConfigPresentation_Branches)$' -count=1`

## 16. Phase 3 第五个样板记录

1. [X] 目标：把 `orgunit_nodes_store_coverage_test.go` 这类基础设施+memory store 复合 coverage 文件并回主测试文件，验证 `orgunit_nodes` 族也可以按职责收口。
2. [X] 执行动作：
   - 将 `orgunit_nodes_store_coverage_test.go` 归并到 `orgunit_nodes_store_test.go`；
   - 在主测试文件中补齐新增测试所需的 import。
3. [X] 边界结论：
   - `orgunit_nodes_store_test.go` 统一承载 node helper、memory store、部分 node audit helper 与日期边界逻辑；
   - `pgstore` 的重读路径与写路径 coverage 文件继续留待下一批分开处理。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 helper 语义、memory store 行为与 audit 边界断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(NewOrgNodeRequestID_Prefix|ParseOrgID8|ParseOptionalOrgID8|CanEditOrgNodes|OrgUnitInitiatorUUID_PrefersValidPrincipalID|OrgUnitInitiatorUUID_FallsBackToTenantID|OrgNodeWriteErrorMessage|IncludeDisabledHelpers|OrgNodeAuditLimitAndTabFromURL|OrgUnitLabelsAndTargetStatus|OrgUnitMemoryStore_BasicCRUDAndResolve|OrgUnitMemoryStore_ResolveErrors|OrgUnitMemoryStore_VisibilityMethodsAndVersions|OrgUnitMemoryStore_SearchCandidatesAndRenameErrors|OrgUnitMemoryStore_SearchCandidates_LimitBreak|OrgUnitMemoryStore_CreateAndSearch_IDConversionErrors|OrgUnitMemoryStore_ResolveOrgCodes|OrgUnitMemoryStore_ResolveSetID|OrgUnitMemoryStore_MoveDisableSetBusinessUnitErrors|OrgUnitMemoryStore_ListChildrenAndDetailsAndSearch|VisibilityWrappers|ListNodeAuditEventsHelper|OrgUnitPGStore_ListNodeAuditEvents|OrgUnitMemoryStore_AppendFactsHelpers|OrgUnitPGStore_MaxEffectiveDateOnOrBefore|OrgUnitPGStore_MinEffectiveDate|OrgUnitMemoryStore_MaxEffectiveDateOnOrBefore|OrgUnitMemoryStore_MinEffectiveDate)$' -count=1`

## 17. Phase 3 第六个样板记录

1. [X] 目标：把 `orgunit_nodes` 剩余两份 PG store coverage 文件并回主测试文件，完成该文件族的主干收口。
2. [X] 执行动作：
   - 将 `orgunit_nodes_pgstore_coverage_test.go` 与 `orgunit_nodes_pgstore_read_paths_coverage_test.go` 归并到 `orgunit_nodes_store_test.go`；
   - 在主测试文件中补齐 `encoding/json` import，并保留 `recordRows` / `auditRows` 这类扫描辅助类型。
3. [X] 边界结论：
   - `orgunit_nodes_store_test.go` 统一承载 memory store、PG store、read paths、audit helper 与 visibility 相关分支；
   - `orgunit_nodes_ext_snapshot_test.go` 继续保持专项、窄职责定位。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 PG 读写路径、search/versions、visibility 与 audit 分支断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(OrgUnitPGStore_ResolveSetID|OrgUnitPGStore_ResolveOrgID|OrgUnitPGStore_ResolveOrgCode|OrgUnitPGStore_ResolveOrgCodes|OrgUnitPGStore_SetBusinessUnitCurrent_Errors|OrgUnitPGStore_SetBusinessUnitCurrent_Success|OrgUnitPGStore_SetBusinessUnitCurrent_Idempotent|OrgUnitPGStore_SetBusinessUnitCurrent_RollbackError|OrgUnitPGStore_CorrectNodeEffectiveDate_Errors|OrgUnitPGStore_CorrectNodeEffectiveDate_Success|OrgUnitPGStore_UsesQuotedCurrentTenantKey|OrgUnitPGStore_ListNodesCurrent_AndCreateCurrent|OrgUnitPGStore_ListBusinessUnitsCurrent|OrgUnitPGStore_ListBusinessUnitsCurrent_Errors|OrgUnitPGStore_ListNodesCurrent_Errors|OrgUnitPGStore_CreateNodeCurrent_Errors|OrgUnitPGStore_RenameMoveDisableCurrent|OrgUnitPGStore_ListNodesCurrentWithVisibility_Coverage|OrgUnitPGStore_ListChildren_AndVisibility_Coverage|OrgUnitPGStore_GetNodeDetails_AndVisibility_Coverage|OrgUnitPGStore_SearchNode_AndCandidates_AndVersions_Coverage)$' -count=1`

## 18. Phase 3 第七个样板记录

1. [X] 目标：在 `assistant` 文件族中选择一组边界最清晰的 coverage 文件先行收口，验证 `Phase 3` 可以进入 assistant 域。
2. [X] 执行动作：
   - 将 `assistant_model_gateway_coverage_test.go` 归并到 `assistant_model_gateway_more_test.go`；
   - 保留已有 `assistant_model_gateway_more_test.go` 的主职责入口，不新增新 coverage 文件。
3. [X] 边界结论：
   - `assistant_model_gateway_more_test.go` 统一承载 provider adapter、gateway config/health、resolve-intent retry 与 helper 分支；
   - 后续更大的 `assistant_api_*` / `assistant_persistence_*` / `assistant_task_store_*` 可以沿相同方法继续按子簇推进。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 adapter/probe/status/retry 分支断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'Test(AssistantDeterministicProviderAdapter_InvokeProbeMissingBranches|AssistantOpenAIProviderAdapter_InvokeSecondPassError|AssistantOpenAIProviderAdapter_ProbeBranches|AssistantModelGateway_HelperCoverage|AssistantModelGateway_NewGatewayProbeHealthAndURLCoverage|AssistantModelGateway_HelperCoverage_Additional|AssistantModelGateway_ResolveIntentRetriesTransientInvoke|AssistantModelGateway_ResolveIntentRetriesStrictDecodeFailure|AssistantModelGateway_BranchCoverage|AssistantModelGateway_RuntimeEndpointValidation|AssistantModelGateway_NoDeterministicSwapInTestEnv|AssistantModelGateway_ListProviderStatus_ProbeConnectivity|AssistantOpenAIProviderAdapter_InvokeAndParseContentArray|AssistantModelGateway_DefaultConfigFollowsRuntime|AssistantOpenAIProviderAdapter_ErrorBranches|AssistantModelGateway_ResolveIntentRetryAndGuardBranches|AssistantModelGateway_HelperFunctions_ExtraBranches)$' -count=1`

## 19. Phase 3 第八个样板记录

1. [X] 目标：从 `assistant_api` 主簇中选择一个边界相对清晰的子计划 coverage 文件先行收口，降低一次性处理整簇的风险。
2. [X] 执行动作：
   - 将 `assistant_api_243_gap_test.go` 归并到 `assistant_api_test.go`；
   - 保留现有 `assistant_api_test.go` 作为 conversation/turn API 的主入口测试文件。
3. [X] 边界结论：
   - `assistant_api_test.go` 统一承载 conversation flow、API 契约与 243 子计划的错误映射/helper 分支；
   - 更大的 `assistant_api_coverage_test.go` 与 `assistant_api_gap_test.go` 仍留待下一批继续按子簇分步处理。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 create-turn、turn action、route/clarification/contract error mapping 断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantAPI243_' -count=1`

## 20. Phase 3 第九个样板记录

1. [X] 目标：继续验证 assistant API 可按更小的 action/reply 子簇收口，而不必总是处理整个大文件族。
2. [X] 执行动作：
   - 将 `assistant_api_reply_extra_test.go` 归并到 `assistant_api_reply_more_test.go`；
   - 保留 `assistant_api_reply_more_test.go` 作为 reply API 的主职责入口。
3. [X] 边界结论：
   - `assistant_api_reply_more_test.go` 统一承载 reply 成功路径、权限/缺失分支与 fallback/error branch；
   - 更大的 `assistant_api_coverage_test.go` / `assistant_api_gap_test.go` 仍留待下一批继续按子簇处理。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 reply success/fallback/permission/error mapping 断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestHandleAssistantTurnActionAPIReply' -count=1`

## 21. Phase 3 第十个样板记录

1. [X] 目标：把 `assistant_persistence` 主簇中边界最清晰的 243 gap 文件并回主 coverage 文件，验证 assistant persistence 也能按子簇逐步收口。
2. [X] 执行动作：
   - 将 `assistant_persistence_243_gap_test.go` 归并到 `assistant_persistence_coverage_test.go`；
   - 复用主文件中既有的 `assistFakeTx*` / `assistFakeRow*` / `assignScan` 等共享 persistence fake，不新增第二套 stub/fake。
3. [X] 边界结论：
   - `assistant_persistence_coverage_test.go` 统一承载 persistence helper、`loadConversationTx`、`upsertTurnTx`、PG create/confirm/commit 与 243 子计划相关的 contract/clarification 分支；
   - 更大的 `assistant_persistence_gap_test.go` 仍留待下一批继续按职责收口。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 route builder error、plan boundary violation、route audit mismatch、action gate denied、clarification runtime invalid 等断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantPersistence243_' -count=1`

## 22. Phase 3 第十一个样板记录

1. [X] 目标：继续收口 `assistant_persistence` 主簇，把剩余的 `gap` 覆盖文件并回主测试文件，避免 persistence 域继续分裂维护。
2. [X] 执行动作：
   - 将 `assistant_persistence_gap_test.go` 归并到 `assistant_persistence_coverage_test.go`；
   - 在主测试文件中补齐 `encoding/base64` import，并保留单一 `assistantPersistenceConversationRow(...)` helper 作为共享入口。
3. [X] 边界结论：
   - `assistant_persistence_coverage_test.go` 统一承载 create/get/load/list、confirm/commit idempotency、cursor codec 与 submit-commit-task/commit-core 的 persistence/error matrix；
   - assistant persistence 文件族当前不再保留 `gap`/`243_gap` 这类独立补洞文件。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 create/confirm/commit/list/query/scan/finalize/commit failure、cursor decode、gate reject 与 expiry 分支断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantPersistence_' -count=1`

## 23. Phase 3 第十二个样板记录

1. [X] 目标：从 assistant API 主簇中继续收口 `gap` 文件，把 handler/helper 子簇并回主测试文件。
2. [X] 执行动作：
   - 将 `assistant_api_gap_test.go` 归并到 `assistant_api_test.go`；
   - 将 `assistantReqWithContext(...)` 与 `assistantDecodeErrCode(...)` 提升为 `assistant_api_test.go` 的共享 helper，并从 `assistant_api_coverage_test.go` 删除重复定义。
3. [X] 边界结论：
   - `assistant_api_test.go` 统一承载 conversation/turn handler 的 tenant mismatch、runtime/model error mapping、request-in-progress/idempotency conflict、list/cursor/path helper 与 route error mapping；
   - `assistant_api_coverage_test.go` 暂时保留更大的 coverage matrix 与专用 store/write stub，留待下一批继续收口。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 tenant mismatch、pg begin failure、runtime config、model gateway、request in progress、idempotency conflict、list pagination、route handler mapping 等断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistant(ConversationHandlers_ExtraErrorBranches|ConversationTurns_ModelGatewayErrorMappings|TurnAction_RequestInProgressMappings|TurnAction_IdempotencyConflictMappings|TurnAction_RequiresIntentClarificationBeforeConfirm|ConversationTurns_RuntimeConfigErrorMappings|ServiceHelpers_PoolWrappersAndPathEdges|ConversationsList_HandlerAndServiceBranches|Helper_LatestTurnAndTaskActionPathBranches|CreateTurn_KnowledgeRuntimeErrorBranches|IntentClarificationAndDryRunNonBusinessCoverage|RouteHandlerMappings)$' -count=1`

## 24. Phase 3 第十三个样板记录

1. [X] 目标：完成 assistant API 主簇最后一块 `coverage` 文件的收口，把 API family 全部归到单一主测试文件。
2. [X] 执行动作：
   - 将 `assistant_api_coverage_test.go` 归并到 `assistant_api_test.go`；
   - 将顶部 7 个专用 store/write stub 一并归入主测试文件，移除独立 `coverage` 入口。
3. [X] 边界结论：
   - `assistant_api_test.go` 统一承载 conversation/turn handler coverage matrix、service helper/utilities、resolve-commit error mapping 与 confirm-window helper；
   - assistant API 文件族当前不再保留 `gap`/`coverage` 这类独立补洞文件。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 保留原有 create/detail/turn-action matrix、candidate resolve 变体、confirm/commit direct branch、risk gate、resolve-commit mapping 与 confirm-window helper 断言，只改变测试组织方式。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistant(ConversationHandlers_CoverageMatrix|TurnActionHandler_CoverageMatrix|ServiceHelpersAndUtilities|ResolveCommitError_CoverageMatrix|ConfirmWindowHelpers_Coverage)$' -count=1`

## 25. Phase 3 第十四个样板记录

1. [X] 目标：处理 `assistant_task_store` 这一类“重复遗留 gap 入口仍存在”的收口尾项，消除重复定义并收敛到唯一主测试文件。
2. [X] 执行动作：
   - 删除 `assistant_task_store_gap_test.go`；
   - 保留既有 `assistant_task_store_test.go` 作为唯一正式主测试文件；
   - 不改生产代码，只做测试入口收口。
3. [X] 边界结论：
   - `assistant_task_store_test.go` 作为 task store 的正式主测试入口，统一承载 utility/validation、record/sql helper、submit/get/cancel/dispatch/execute 分支矩阵；
   - assistant task store 文件族当前不再保留 `gap` 命名测试文件。
4. [X] 对外契约不变性说明：
   - 未改动生产代码；
   - 仅删除与主测试文件重复的旧入口，不影响对外行为。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantTaskStore_' -count=1`

## 26. Phase 3 第十四个样板延伸收口记录

1. [X] 目标：在保持 `assistant_task_store_test.go` 为唯一正式主测试文件的前提下，继续做单文件内职责细化，避免后续回退到多入口测试族。
2. [X] 执行动作：
   - 将 6 个顶层测试重命名为稳定职责簇：`UtilityValidationAndWrappers`、`RecordScanAndSQLHelpers`、`SubmitTaskPG`、`GetTaskAndCancelTaskPG`、`DispatchAndExecute`、`ResidualErrorMatrix`；
   - 对原先偏重的后半段测试矩阵补上 `t.Run(...)` 子层级，显式区分 `get/cancel`、`dispatch/execute` 与 residual stopline 分支。
3. [X] 边界结论：
   - `assistant_task_store_test.go` 仍是唯一主测试入口，不新增 `assistant_task_store_*_test.go`；
   - 共享 helper 保持顶部集中，局部逻辑改为在对应职责簇内部就近组织。
4. [X] 对外契约不变性说明：
   - 未修改生产代码、测试断言语义与测试入口前缀；
   - 外部 `go test ./internal/server -run 'TestAssistantTaskStore_' -count=1` 调用口径保持不变。
5. [X] 验证命令：
   - `go test ./internal/server -run 'TestAssistantTaskStore_' -count=1`
   - `make check doc`

## 27. Phase 2/3 衔接候选盘点记录

1. [X] 目标：完成 `internal/server` 高频规则测试盘点，形成“哪些应迁到 `modules/*/services`、哪些暂留 server”的首批清单，给下一批样板迁移提供明确入口。
2. [X] 执行动作：
   - 盘点 `staffing` / `jobcatalog` / `person` / `assistant` / `functional area governance` 相关测试族的顶层测试职责；
   - 按“业务规则 / 输入校验 / helper 归一化 / 组合适配”分类，标注目标承接层与暂留理由；
   - 锁定下批实施顺序为 `modules/staffing/services` → `modules/jobcatalog/services` → `modules/person/services`。
3. [X] 边界分类表：

| 当前测试文件 / 分支 | 主断言类型 | 目标层 | 结论 |
| --- | --- | --- | --- |
| `internal/server/staffing_canonicalize_test.go` | JSON canonicalize / validator 纯规则 | `modules/staffing/services` | 首批迁移 |
| `internal/server/staffing_correct_rescind_store_test.go` 中事务前校验、memory store 语义分支 | assignment correct/rescind 业务规则 | `modules/staffing/services` | 首批迁移 |
| `internal/server/staffing_test.go` 中 `UpsertPrimaryAssignmentForPerson` 输入校验与默认值语义 | staffing 写规则 | `modules/staffing/services` | 首批迁移 |
| `internal/server/jobcatalog_test.go` 中 `jobCatalogView` / owner-setid / package resolve helper 分支 | jobcatalog 读写规则与选择策略 | `modules/jobcatalog/services` | 第二批迁移 |
| `internal/server/person_test.go` 中 `normalizePernr` 与 create/find 输入校验分支 | person 标识规范化与输入校验 | `modules/person/services` | 第三批迁移 |
| `internal/server/assistant_clarification_policy_test.go` | assistant 运行态澄清策略 | `internal/server` | 暂留，当前无独立 assistant services 承接 |
| `internal/server/assistant_create_policy_precheck_test.go` | assistant create/confirm 组合预检 | `internal/server` | 暂留，仍强绑定对话运行态与 gateway/store 组合 |
| `internal/server/functional_area_governance_test.go` | capability 注册表 + functional area gate | `internal/server` | 暂留，当前依赖全局 capability/runtime 边界 |

4. [X] 边界结论：
   - `staffing` 是下一批最适合下沉的样板模块，因为 `modules/staffing/services/assignments_facade.go` 已存在最小服务入口，可先承接校验与 canonicalize 规则；
   - `jobcatalog` 与 `person` 目前 `services/` 仍接近空目录，更适合在 `staffing` 样板验证后按同样方法建立首个服务层测试入口；
   - `assistant` 与 `functional area governance` 暂不作为本批 `services` 样板，避免在未抽出稳定模块边界前引入更大抽象面。
5. [X] 对外契约不变性说明：
   - 本批次仅完成候选盘点与执行顺序冻结，不修改生产代码与测试断言；
   - 仅为后续迁移批次提供 decision-complete 的目标清单与边界说明。
6. [X] 验证命令：
   - `make check doc`

## 28. Phase 3 staffing 首个服务层样板迁移

1. [X] 目标：将 `internal/server` 中最适合下沉的 staffing assignment 纯规则与首批前置校验分支，迁移到 `modules/staffing/services`，建立后续 `jobcatalog` / `person` 可复用的服务层样板。
2. [X] 执行动作：
   - 新增 `modules/staffing/services/assignment_rules.go`，统一收敛 assignment upsert 输入规范化、correct/rescind 输入校验、JSON canonicalize、确定性 correction/rescind event id 生成；
   - 新增 `modules/staffing/services/assignments_facade_test.go`，承接 canonicalize helper 分支，以及 `AssignmentsFacade` 的输入校验、默认值、payload canonicalize/normalize 分支；
   - `internal/server/staffing.go` 与 `modules/staffing/infrastructure/persistence/assignment_pg_store.go` 改为复用 `services` 规则 helper；
   - 删除 `internal/server/staffing_canonicalize_test.go` 与 `modules/staffing/infrastructure/persistence/assignment_pg_store_test.go`，避免 pure-rule 测试在 server/persistence 双处漂移；
   - 从 `internal/server/staffing_correct_rescind_store_test.go`、`internal/server/staffing_test.go` 删除已由 `services` 承接的前置校验分支，保留 adapter/tx/memory 行为语义分支。
3. [X] 边界分类表：

| 规则 / 测试簇 | 新归属层 | 保留层 | 说明 |
| --- | --- | --- | --- |
| assignment canonicalize / JSON helper / deterministic correct-rescind event id | `modules/staffing/services` | - | 纯规则单点化，供 facade、server memory store、persistence 复用 |
| upsert 输入校验与默认 `status=active` 语义 | `modules/staffing/services` | - | 通过 `AssignmentsFacade` 与 `PrepareUpsertPrimaryAssignment` 承接 |
| correct/rescind 前置输入校验与 payload normalize | `modules/staffing/services` | - | 通过 facade + prepare helper 承接，并在 store 调用前 fail-fast |
| PG tx begin / set tenant / submit / commit 失败分支 | - | `internal/server` | 仍属于 adapter / persistence 接线验证 |
| memory store 更新/撤销语义与 stopline 行为 | - | `internal/server` | 仍属于运行态 store 行为验证，不纳入本批 pure-rule 下沉 |

4. [X] 对外契约不变性说明：
   - 未新增新的 server 测试入口，也未修改对外 API、数据库 schema 或 handler 契约；
   - 仅调整规则承接层与测试分层，`staffingPGStore` / `staffingMemoryStore` 的调用口径保持不变。
5. [X] 验证命令：
   - `go test ./modules/staffing/services -count=1`
   - `go test ./modules/staffing/infrastructure/persistence -count=1`
   - `go test ./internal/server -run 'TestStaffing(PGStore_UpsertPrimaryAssignmentForPerson|PGStore_CorrectRescindAssignmentEvent|MemoryStore|Handlers_JSONRoundTrip)' -count=1`
   - `make check doc`

## 29. Phase 3 jobcatalog 首个服务层样板迁移

1. [X] 目标：将 `internal/server/jobcatalog.go` 中 package/view 纯规则 helper 与 owned package 权限判断，迁移到 `modules/jobcatalog/services`，为下一批 `person` 服务层样板提供同类模板。
2. [X] 执行动作：
   - 新增 `modules/jobcatalog/services/view_rules.go`，定义最小服务层模型与规则函数，承接 `NormalizePackageCode`、`CanEditDefltPackage`、`OwnerSetIDEditable`、`LoadOwnedJobCatalogPackages`、`ResolveJobCatalogView`；
   - 新增 `modules/jobcatalog/services/view_rules_test.go`，迁移原 `internal/server/jobcatalog_test.go` 中对应 helper/view 测试分支；
   - `internal/server/jobcatalog.go` 改为通过 adapter 将 `server` 层 `Principal`、`SetID`、`OwnedScopePackage`、`JobCatalogPackage` 映射到 `services` 最小模型，再调用服务层规则；
   - 从 `internal/server/jobcatalog_test.go` 删除已由服务层承接的 pure-rule 测试段，保留 PG store / API / error mapping 行为测试。
3. [X] 边界分类表：

| 规则 / 测试簇 | 新归属层 | 保留层 | 说明 |
| --- | --- | --- | --- |
| `jobCatalogView.ListSetID` / `NormalizePackageCode` | `modules/jobcatalog/services` | - | 纯 helper/view 规则，脱离 HTTP 与 PG 依赖 |
| owned package 编辑权限判断、`CanEditDefltPackage`、`LoadOwnedJobCatalogPackages` | `modules/jobcatalog/services` | - | 归入服务层策略，server 仅负责上下文 principal 抽取 |
| `ResolveJobCatalogView` 的 package-code / owner-setid / DEFLT / mismatch 分支 | `modules/jobcatalog/services` | - | 归为 jobcatalog view 选择与权限策略 |
| PG package resolve / setid validation / DB error/status 映射 | - | `internal/server` | 仍属于 adapter / persistence / handler 行为验证 |
| write API 的 action 分发与响应码契约 | - | `internal/server` | 仍属于 HTTP 层与组合逻辑，不纳入本批 helper 下沉 |

4. [X] 对外契约不变性说明：
   - 未新增新的 jobcatalog server 测试入口，也未修改 public API、DB schema 或 handler 响应契约；
   - 仅调整 helper 所在层与测试分层，server 仍保留最小包装函数以维持调用口径稳定。
5. [X] 验证命令：
   - `go test ./modules/jobcatalog/services -count=1`
   - `go test ./internal/server -run 'Test(JobCatalogStatusForError|JobCatalogPGStore_SetIDValidation|ResolveJobCatalogPackageByCode_PG|JobCatalogPGStore_ResolvePackages|HandleJobCatalogAPI_Get|HandleJobCatalogWriteAPI_Post)' -count=1`
   - `make check doc`

## 30. Phase 3 person 首个服务层样板迁移

1. [X] 目标：将 `internal/server/person.go` 中 `normalizePernr` 与 create/find 输入校验前置规则迁移到 `modules/person/services`，完成首批既定顺序中的第三个服务层样板。
2. [X] 执行动作：
   - 新增 `modules/person/services/person_rules.go`，定义最小服务层 `Person`/`Store`/`Facade` 与 `NormalizePernr`、`PrepareCreatePerson`、`PrepareFindPersonByPernr`；
   - 新增 `modules/person/services/person_rules_test.go`，迁移原 `internal/server/person_test.go` 中的 `normalizePernr`、create invalid pernr、missing display_name、find invalid pernr 等 pure-rule 分支；
   - `internal/server/person.go` 改为复用 `services` 规则 helper，PG store / memory store 不再各自维护重复的 `pernr` 与 display name 规范化逻辑；
   - 从 `internal/server/person_test.go` 删除已由服务层承接的 pure-rule 测试段，保留 PG、memory、handler 与 options 行为测试。
3. [X] 边界分类表：

| 规则 / 测试簇 | 新归属层 | 保留层 | 说明 |
| --- | --- | --- | --- |
| `NormalizePernr` | `modules/person/services` | - | 纯标识规范化规则，脱离 HTTP / PG 依赖 |
| `CreatePerson` 输入校验：invalid pernr / missing display_name / trimmed canonical pernr | `modules/person/services` | - | 通过 `PrepareCreatePerson` 与最小 facade 承接 |
| `FindPersonByPernr` 输入校验：invalid pernr / canonical pernr | `modules/person/services` | - | 通过 `PrepareFindPersonByPernr` 与 facade 承接 |
| PG begin / set tenant / insert / no rows / row / commit / list options 分支 | - | `internal/server` | 仍属于 persistence / adapter 行为验证 |
| memory duplicate / not found / options 限流与前缀搜索 / handler 响应码 | - | `internal/server` | 仍属于运行态 store 与 HTTP 层测试，不纳入本批 pure-rule 下沉 |

4. [X] 对外契约不变性说明：
   - 未新增新的 person server 测试入口，也未修改 public API、DB schema 或 handler 响应契约；
   - 仅调整输入规则所在层与测试分层，server 仍保持原有 `PersonStore` 对外接口。
5. [X] 验证命令：
   - `go test ./modules/person/services -count=1`
   - `go test ./internal/server -run 'Test(PersonPGStore_ListPersons|PersonPGStore_CreatePerson|PersonPGStore_FindPersonByPernr|PersonPGStore_ListPersonOptions|PersonMemoryStore|HandlePersonsAPI_Branches|PersonHandlers)' -count=1`
   - `make check doc`

## 31. Phase 4 首批 E2E helper 收敛

1. [X] 目标：把 `Phase 3` 完成后的分层结论继续衔接到 E2E，明确“E2E 是验收层，不是 Go 单测缺口兜底场”，并先抽出至少 2 个高频重复 helper。
2. [X] 执行动作：
   - 新增 `e2e/tests/helpers/kratos-identity.js`，统一承载 `ensureKratosIdentity(...)`；
   - 新增 `e2e/tests/helpers/superadmin-tenant.js`，统一承载 `loginSuperadmin(...)`、`createTenantAndGetID(...)`、`setupTenantAdminSession(...)`；
   - 将 `e2e/tests/m3-smoke.spec.js`、`e2e/tests/tp220-assistant.spec.js`、`e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`、`e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js` 接入共享 helper，去掉重复的 superadmin/Kratos setup；
   - 在本执行记录中补充“Go 单测 / 服务层 / E2E”分工表，冻结 Phase 4 的首批边界。
3. [X] 边界分类表：

| 断言类型 | `pkg/**` Go 单测 | `modules/*/services` | E2E |
| --- | --- | --- | --- |
| 纯归一化 / parser / validator / error helper | 主承接层 | - | 不重复兜底 |
| 业务规则、默认值、策略选择、前置校验 | - | 主承接层 | 仅通过最终用户路径间接验收，不重复枚举细分矩阵 |
| HTTP handler、路由、状态码、租户上下文、authn/authz 组合适配 | - | - | 以 server Go 测试为主承接，E2E 只保留关键端到端入口 |
| 登录、租户创建、正式页面入口、跨模块用户流程、静态资源/会话边界 | - | - | 主承接层 |
| 轮询、证据写入、基线数据 fixture | - | - | 允许继续在 `e2e/tests/helpers` 收敛，但不得演化成 legacy 双链路 |

4. [X] 对外契约不变性说明：
   - 未修改任何生产代码、路由、数据库、会话协议或 Assistant 行为；
   - 本批次只收敛 Playwright 测试基建与文档分层说明，不改变现有 E2E 断言语义。
5. [X] 验证命令：
   - `node --check e2e/tests/helpers/kratos-identity.js`
   - `node --check e2e/tests/helpers/superadmin-tenant.js`
   - `node --check e2e/tests/m3-smoke.spec.js`
   - `node --check e2e/tests/tp220-assistant.spec.js`
   - `node --check e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`
   - `node --check e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`
   - `make check doc`

## 32. Phase 4 第二批 E2E helper 收敛

1. [X] 目标：继续减少 live/evidence 类 E2E spec 中的重复测试基建，把 `IAM session retry`、`evidence I/O` 与 `assistant task polling` 从大文件内部提取为共享 helper。
2. [X] 执行动作：
   - 新增 `e2e/tests/helpers/iam-session.js`，统一承载 `createIAMSession(...)` 与 `createIAMSessionWithRetry(...)`；
   - 新增 `e2e/tests/helpers/evidence.js`，统一承载 `ensureDir(...)` 与 `writeJSON(...)`；
   - 新增 `e2e/tests/helpers/assistant-task.js`，统一承载 `pollAssistantTask(...)`；
   - 扩展 `e2e/tests/helpers/superadmin-tenant.js`，支持 `sessionLoginRetryTimeoutMs` 与 `recordHar` 等上下文选项；
   - 将 `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`、`e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`、`e2e/tests/tp290-librechat-real-case-matrix.spec.js`、`e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`、`e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js` 接入共享 helper；
   - 修复 `e2e/tests/tp220-assistant.spec.js` 在首批收敛后残留的旧 `setupTenantAdminSession(...)` 调用，统一回到 `createTP220Session(...)`。
3. [X] 边界分类表：

| helper / 模式 | 新归属 | 覆盖 spec | 说明 |
| --- | --- | --- | --- |
| `createIAMSession` / `createIAMSessionWithRetry` | `e2e/tests/helpers/iam-session.js` | `tp288b` / `tp290b` 等 live spec | 会话建立基建，不再在各 spec 顶部复制 invalid-credentials retry 循环 |
| `ensureDir` / `writeJSON` | `e2e/tests/helpers/evidence.js` | `tp288` / `tp288b` / `tp290` / `tp290b` | 证据目录与 JSON 落盘保持单点实现 |
| `pollAssistantTask` | `e2e/tests/helpers/assistant-task.js` | `tp290b-neg` | task terminal 轮询从单 spec 内联逻辑收敛为共享 helper |
| `setupTenantAdminSession` + `recordHar`/retry 选项 | `e2e/tests/helpers/superadmin-tenant.js` | `tp288` / `tp288b` / `tp290` / `tp290b` / `tp220` | superadmin/tenant-admin 初始化继续收敛为单入口，允许 live spec 注入 HAR 与登录重试策略 |

4. [X] 对外契约不变性说明：
   - 未修改任何生产代码、路由、租户会话协议、Assistant 业务行为或测试断言语义；
   - 本批次仅进一步收敛 Playwright 测试基建，保留原有 evidence/live 场景的业务断言与落盘路径。
5. [X] 验证命令：
   - `node --check e2e/tests/helpers/iam-session.js`
   - `node --check e2e/tests/helpers/evidence.js`
   - `node --check e2e/tests/helpers/assistant-task.js`
   - `node --check e2e/tests/helpers/superadmin-tenant.js`
   - `node --check e2e/tests/tp220-assistant.spec.js`
   - `node --check e2e/tests/tp288-librechat-real-entry-evidence.spec.js`
   - `node --check e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
   - `node --check e2e/tests/tp290-librechat-real-case-matrix.spec.js`
   - `node --check e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`
   - `node --check e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`
   - `make check doc`

## 33. Phase 4 第三批 E2E baseline helper 收敛

1. [X] 目标：继续减少 live spec 中重复的 baseline org/unit 初始化逻辑，把 `orgunit list/detail/create/wait/root-detect` 这类基础操作提炼为共享 helper，同时保留各 spec 自己的 report/probe 语义。
2. [X] 执行动作：
   - 新增 `e2e/tests/helpers/org-baseline.js`，统一承载 `listOrgUnits(...)`、`getOrgUnitDetails(...)`、`waitForOrgUnitDetails(...)`、`createOrgUnit(...)`、`detectRootOrg(...)`、`ensureOrgUnitByCode(...)`、`orgUnitDetailsSnapshot(...)`、`collectOrgDetailsBySpecs(...)`、`collectCandidateDetails(...)`；
   - 将 `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js` 改为复用共享的 root-detect / create / wait / ensure helper，保留本地 `ensureTenantBaseline(...)` 的场景语义；
   - 将 `e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js` 改为复用共享的 org baseline helper，保留本地 baseline report、candidate probe、runtime gate 等专项断言；
   - 本批次刻意不抽 `baselineProbeSummary`、`required_orgs` report 组装与 case-specific probe 判定，避免把不同 live 场景的业务语义混成一个过重 helper。
3. [X] 边界分类表：

| helper / 模式 | 新归属 | 覆盖 spec | 说明 |
| --- | --- | --- | --- |
| orgunit list/detail/create/wait/root-detect | `e2e/tests/helpers/org-baseline.js` | `tp288b` / `tp290b-chain` | 纯 baseline 数据准备基础操作，适合跨 spec 复用 |
| `ensureOrgUnitByCode` + created callback | `e2e/tests/helpers/org-baseline.js` | `tp288b` / `tp290b-chain` | 兼顾“只建缺失节点”与各 spec 自己的 created-org report 记录 |
| baseline report/probe/required-orgs 组装 | 保留在各 spec | `tp290b-chain` | 强场景化语义，暂不抽象，避免 helper 变成杂糅 orchestrator |
| receipt-contract 场景下的单父组织 baseline 语义 | 保留在各 spec | `tp288b` | 仍通过本地 `ensureTenantBaseline(...)` 表达最直接的测试意图 |

4. [X] 对外契约不变性说明：
   - 未修改任何生产代码、Org API、Assistant 行为、baseline org code/name 约定或 evidence 路径；
   - 本批次只收敛 E2E baseline fixture 的内部实现位置，不改变原有用例的断言目标与落盘内容。
5. [X] 验证命令：
   - `node --check e2e/tests/helpers/org-baseline.js`
   - `node --check e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
   - `node --check e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`
   - `make check doc`

## 34. Phase 4 第四批 assistant conversation helper 收敛

1. [X] 目标：继续压缩 live spec 中重复出现的 assistant conversation 基础工具，但不抽走具体场景语义，只收敛 `parseJSONSafe`、`parseResponseBody` 与 `latestTurn` 这一层。
2. [X] 执行动作：
   - 新增 `e2e/tests/helpers/assistant-conversation.js`，统一承载 `parseJSONSafe(...)`、`parseResponseBody(...)`、`latestAssistantTurn(...)`；
   - 将 `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js` 改为复用共享的 response/body 解析与 latest-turn helper；
   - 将 `e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js` 改为复用共享的 response/body 解析与 latest-turn helper；
   - 将 `e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js` 改为复用共享的 `parseJSONSafe(...)` 与 `latestAssistantTurn(...)`。
3. [X] 边界分类表：

| helper / 模式 | 新归属 | 覆盖 spec | 说明 |
| --- | --- | --- | --- |
| `parseJSONSafe` | `e2e/tests/helpers/assistant-conversation.js` | `tp288b` / `tp290b-chain` / `tp290b-neg` | 纯响应解析基础工具，适合跨 live spec 复用 |
| `parseResponseBody` | `e2e/tests/helpers/assistant-conversation.js` | `tp288b` / `tp290b-chain` | HTTP response 文本+JSON 解包统一入口 |
| `latestAssistantTurn` | `e2e/tests/helpers/assistant-conversation.js` | `tp288b` / `tp290b-chain` / `tp290b-neg` | conversation/turn 读取基础工具，不再在各 spec 顶部重复定义 |
| case-specific probe / receipt / failure handling | 保留在各 spec | 各自 spec | 仍属于强场景语义，不与基础 helper 混合 |

4. [X] 对外契约不变性说明：
   - 未修改任何生产代码、Assistant API、任务状态语义或 evidence 输出结构；
   - 本批次只调整 E2E 基础 helper 的实现位置与复用方式，不改变断言目标。
5. [X] 验证命令：
   - `node --check e2e/tests/helpers/assistant-conversation.js`
   - `node --check e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
   - `node --check e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`
   - `node --check e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`
   - `make check doc`

## 35. Phase 4 阶段性盘点与封板

1. [X] 目标：在连续四批 helper 收敛后，明确 `Phase 4` 已形成的稳定基建边界，避免继续把 spec 专属场景语义过度抽象成新的“大而全 helper”。
2. [X] 盘点结论：
   - 当前稳定的共享 helper 已收敛为 7 个：`kratos-identity`、`superadmin-tenant`、`iam-session`、`evidence`、`assistant-task`、`org-baseline`、`assistant-conversation`；
   - 这些 helper 分别覆盖“身份建立 / tenant-admin session / 会话重试 / 证据落盘 / task 轮询 / baseline org 数据准备 / conversation 基础解析”七类真正跨 spec 复用的基建；
   - `tp290b-chain` 的 baseline report、probe summary、required-orgs readiness、runtime gate 组装，以及 `tp288b` 的 receipt contract 断言，仍保留在 spec 内，作为场景层语义，不再默认继续上提。
3. [X] helper 稳定态矩阵：

| helper | 稳定职责 | 代表性接入 spec | 封板结论 |
| --- | --- | --- | --- |
| `kratos-identity.js` | Kratos identity 建立 | `m3` / `tp220` / `tp283` / live specs 通过 session helper 间接复用 | 保持 |
| `superadmin-tenant.js` | superadmin 登录、建租户、tenant-admin session | `m3` / `tp220` / `tp283` / `tp288*` / `tp290*` | 保持 |
| `iam-session.js` | `/iam/api/sessions` 直登与 retry | `tp288b` / `tp290b-chain` | 保持 |
| `evidence.js` | `ensureDir` / `writeJSON` | `tp288` / `tp288b` / `tp290` / `tp290b` | 保持 |
| `assistant-task.js` | task polling | `tp290b-neg` | 保持，按需扩展 |
| `org-baseline.js` | org baseline list/detail/create/wait/root-detect | `tp288b` / `tp290b-chain` | 保持，不上提 report 组装 |
| `assistant-conversation.js` | response decode / latest turn | `tp288b` / `tp290b-chain` / `tp290b-neg` | 保持 |

4. [X] stopline 结论：
   - 不再为了“看起来更整洁”继续上提 `baselineProbeSummary`、`required_orgs` report、receipt assertion、runtime gate report 等强场景语义；
   - 只有在未来出现跨 2 个以上 spec 的稳定重复，并且抽出后不会削弱可读性时，才允许继续新增 helper；
   - 现阶段优先把 `Phase 4` 视为“已收敛到稳定基线”，而不是持续追求抽象纯度。
5. [X] 对外契约不变性说明：
   - 本批次仅回写阶段性盘点与 stopline，不修改任何生产代码、E2E 断言、helper 行为或证据路径；
   - 目的是防止后续偏离 `301` 的“按职责收敛，而非继续堆抽象”原则。
6. [X] 验证命令：
   - `make check doc`
