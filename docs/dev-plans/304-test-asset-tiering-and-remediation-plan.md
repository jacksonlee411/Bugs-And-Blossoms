# DEV-PLAN-304：全仓测试资产分级治理与高信号回归收敛方案

**状态**: 规划中（2026-04-21 23:21 CST）

## 背景

`DEV-PLAN-300` 已建立全仓测试体系问题基线，`DEV-PLAN-301` 与 `DEV-PLAN-302` 作为首轮 Go 测试分层整改与尾项收口方案已归档，`DEV-PLAN-303` 已完成全仓 `gap/coverage` 命名测试尾项清零。

但当前代码树仍存在新的治理需求：仓库已经不再以“是否仍有 `*_coverage_test.go` 文件”作为主要问题，而是进入“哪些测试资产值得长期信任、哪些测试资产应继续拆分收敛、哪些测试资产只是在门禁压力下勉强存活”的第二阶段治理。

本计划用于承接本轮最新盘点结果，建立当前活体测试资产的分级口径、整改优先级与验收标准，避免测试体系再次滑回“文件名虽然已去 coverage，但结构仍然 coverage-first”的旧状态。

## 与 `300/301/304` 的 owner 关系

1. [X] `DEV-PLAN-300` 是问题基线与调查事实源。
2. [X] `DEV-PLAN-301` 是首轮 Go 测试分层整改的历史来源，不再作为当前资产治理执行 owner。
3. [X] `DEV-PLAN-304` 是当前“测试资产分级、巨型文件拆分优先级、低信号文件清理、反回流冻结规则”的现行 owner。
4. [X] 仓库级活体入口对测试治理的引用口径应统一表达为：`300` 负责问题基线、`301` 负责历史来源、`304` 负责当前资产治理。

## 编号裁决

1. [X] `DEV-PLAN-301` 不可复用。
2. [X] 原因：`301` 已存在且位于归档目录，当前文件为 [301-go-test-layering-and-best-practices-remediation-plan.md](/home/lee/Projects/Bugs-And-Blossoms/docs/archive/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md)。
3. [X] 当前测试治理新方案使用空闲编号 `304`。

## 目标

1. [ ] 建立当前活体测试资产的统一四级分层：`A 高价值`、`B 中价值`、`C 低价值/补洞型`、`H 辅助/基础设施`。
2. [ ] 明确当前最值得信任的高信号回归资产集合，供后续回归、重构与 PR 评审优先依赖。
3. [ ] 明确当前最该拆分、合并、降白盒耦合的测试文件集合，形成专项整改 owner 列表。
4. [ ] 在不降低覆盖率门禁、不扩大排除项的前提下，把测试治理目标从“文件命名收口”升级为“职责与信号质量收口”。
5. [ ] 为后续新增测试提供冻结规则，阻断 `extra/more/branches` 风格测试继续扩散。

## 非目标

1. [ ] 不重开 `DEV-PLAN-301/302/303` 的历史完成态。
2. [ ] 不通过降低 coverage 阈值、扩大排除项、缩小统计范围来获得“治理完成”结论。
3. [ ] 不在本计划中直接重写全仓测试；本计划只定义分级、优先级、stopline 与验收口径。
4. [ ] 不把 benchmark、helper、test harness 文件误包装成“高价值业务测试”。

## 事实基线（2026-04-21 快照）

### 1. 当前测试资产规模

1. [X] 活体测试文件总数：`102`
2. [X] 其中 Go 测试文件：`97`
3. [X] 其中 Playwright E2E spec：`5`
4. [X] Go `Test...` 函数：`709`
5. [X] Playwright `test(...)` 用例：`5`
6. [X] benchmark：`6`
7. [X] fuzz：`1`

### 2. 当前目录分布失衡

1. [X] `internal/server`：`48` 个测试文件，`346` 个 Go 测试函数
2. [X] `modules/orgunit`：`14` 个测试文件，`163` 个 Go 测试函数
3. [X] `internal/superadmin`：`11` 个测试文件，`119` 个 Go 测试函数
4. [X] `internal/routing`：`8` 个测试文件，`33` 个 Go 测试函数

结论：测试压力仍明显偏向 `internal/server`，与 `DEV-PLAN-300` 中“server 层过重”的历史结论保持一致，只是问题已从“文件名层面的 coverage/gap 漂移”转为“职责集中与大文件化”。

### 3. 当前资产分级结果

1. [X] `A 高价值`：`56` 个文件
2. [X] `B 中价值`：`34` 个文件
3. [X] `C 低价值/补洞型`：`3` 个文件
4. [X] `H 辅助/基础设施`：`9` 个文件

当前结论：仓库测试主体不是“纯凑数”，主体仍是有真实回归价值的测试资产；但 `internal/server` 及个别辅助文件中，仍残留明显的 `coverage-first` 组织方式。

### 3.1 盘点口径与复核命令

本轮基线采用以下统计口径，防止后续数字漂移却无法解释：

1. [X] 统计对象：
   - Go：`*_test.go`
   - E2E：`e2e/tests/*.spec.js`
2. [X] 排除目录：
   - `e2e/node_modules`
   - `e2e/_artifacts`
   - `e2e/playwright-report`
   - `e2e/test-results`
3. [X] `H` 类文件纳入“测试文件总数”统计，但不纳入“高价值业务测试资产”判断。
4. [X] benchmark-only 文件与 helper-only `_test.go` 文件保留在基线清单中，但分类为 `H`。

复核命令（2026-04-21 盘点所用）：

1. [X] `rg --files -g '*_test.go' -g 'e2e/tests/*.spec.js' . | rg -v '(^|/)e2e/node_modules/|(^|/)e2e/_artifacts/|(^|/)playwright-report/|(^|/)test-results/'`
2. [X] `rg -n --glob '*_test.go' '^func Test' . | rg -v '(^|/)e2e/node_modules/|(^|/)e2e/_artifacts/|(^|/)playwright-report/|(^|/)test-results/'`
3. [X] `rg -n --glob '*_test.go' '^package .*_test$|^func Fuzz|^func Benchmark' pkg modules internal`
4. [X] helper-only 文件识别口径：存在 `_test.go` 文件但不含 `^func Test` / `^func Benchmark` / `^func Fuzz` 的直接测试入口。

### 3.2 分类映射记录要求

1. [X] `A/B/C/H` 计数必须可回溯到文件级清单，不能只保留汇总数字。
2. [X] 后续若基线数字变化，必须同时更新：
   - 分类全量映射
   - 复核命令
   - 变化说明
3. [X] 若某文件从 `B` 升为 `A` 或从 `B/C` 并回主文件，必须在对应批次记录“为何调整分类”。

### 4. 当前最明显的问题文件

以下文件在本轮盘点中被判定为优先整改对象：

1. [X] [orgunit_write_service_test.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/services/orgunit_write_service_test.go)
2. [X] [orgunit_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_api_test.go)
3. [X] [orgunit_field_metadata_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api_test.go)
4. [X] [handler_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/handler_test.go)
5. [X] [orgunit_nodes_pgstore_write_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_nodes_pgstore_write_test.go)
6. [X] [dicts_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_api_test.go)
7. [X] [orgunit_nodes_store_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_nodes_store_test.go)
8. [X] [orgunit_nodes_pgstore_read_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_nodes_pgstore_read_test.go)
9. [X] [orgunit_field_metadata_store_pg_methods_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_store_pg_methods_test.go)
10. [X] [tenant_console_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/superadmin/tenant_console_test.go)

### 5. 当前最明显的低信号文件

1. [X] [layering_wrapper_coverage_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/layering_wrapper_coverage_test.go)
2. [X] [orgunit_field_metadata_api_coverage_extra_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api_coverage_extra_test.go)
3. [X] [responder_extra_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/routing/responder_extra_test.go)

## 分级标准（冻结）

### A：高价值

满足以下至少一项：

1. [ ] 验证真实业务链路或真实用户可见行为。
2. [ ] 验证外部契约、不变量、协议边界、错误码边界、RLS/authn/authz fail-closed 语义。
3. [ ] 即使被测实现重构，只要外部行为不变，测试主体仍应成立。

### B：中价值

满足以下特征：

1. [ ] 仍有真实回归价值。
2. [ ] 但测试对内部 stub、包级状态、一次性 fake、分支拼接、巨型子测试依赖较重。
3. [ ] 更适合作为过渡资产继续拆分，而不是长期终态。

### C：低价值/补洞型

满足以下任一项：

1. [ ] 主要只是在补某几个边角分支或补齐孤立 coverage。
2. [ ] 对业务回归的实际信号极弱。
3. [ ] 放进主测试文件或直接删除，对长期回归质量几乎无损。

### H：辅助/基础设施

1. [ ] 仅提供 helper、fixture、harness、benchmark、TestMain 或集成测试接线能力。
2. [ ] 不单独计入“高价值业务测试资产”。

## 冻结规则

### 1. 新增测试命名冻结

1. [ ] 未经用户明确批准，不得新增 `*_extra_test.go`、`*_more_test.go`、`*_gap_test.go`、`*_coverage_test.go`。
2. [ ] 未经充分 stopline 说明，不得新增 `Test...Coverage`、`Test...MoreBranches`、`Test...Covers...` 风格函数名。

冻结说明：

1. [ ] 若确因历史并回需要临时保留旧文件名，必须在同一批次内完成 rename 或删除，不得跨批次长期停留。
2. [ ] “函数名未命中旧词，但整体结构仍为 coverage-first”不视为合规；仍需按职责拆分原则裁决。

### 2. 文件体量冻结

1. [ ] 单测试文件若超过 `800` 行，应在对应实施记录中说明为何仍保持单文件。
2. [ ] 单测试文件若超过 `1500` 行，默认进入专项拆分候选池。
3. [ ] 单测试文件若同时满足“超过 `1000` 行 + 超过 `20` 个测试函数”，默认视为结构性整改对象。

### 3. 职责边界冻结

1. [ ] `pkg/**` 优先承载黑盒、纯函数、解析/归一化、错误 helper。
2. [ ] `modules/*/services` 优先承载业务规则、默认值、策略、状态推进。
3. [ ] `internal/server` 只应保留协议适配、错误映射、租户/鉴权/RLS 边界、中间件与跨模块编排。
4. [ ] 若测试主要断言业务规则，而不是协议/适配行为，则应优先迁回 service 层。

## 专项门禁与 Stopline

本计划不要求先实现所有新脚本再开始整改，但要求门禁语义先冻结，避免执行中再次漂移。

### 1. 命名与反回流门禁

1. [ ] `make check test-asset-naming`
2. [ ] 语义：
   - 阻断新增 `*_extra_test.go`、`*_more_test.go`、`*_gap_test.go`、`*_coverage_test.go`
   - 阻断新增 `Test...Coverage`、`Test...MoreBranches`、`Test...Covers...`
3. [ ] 最小 stopline 可先用 `rg` 规则实现，后续如需脚本化再收敛到 `scripts/checks/**`

### 2. 体量与拆分候选门禁

1. [ ] `make check test-asset-size`
2. [ ] 语义：
   - 报告超过 `800` 行的测试文件
   - 对超过 `1500` 行的测试文件要求在当前批次记录拆分理由或拆分方案
3. [ ] 该门禁初期可为报告型，不必一开始就硬阻断全部历史文件；但对新增长文件应 fail-closed

### 3. 活体 owner 文档门禁

1. [ ] `make check test-plan-owner`
2. [ ] 语义：
   - 活体测试治理文档不得把 `301` 直接写成现行 owner
   - 活体入口必须采用“`300` 问题基线 + `301` 历史来源 + `304` 当前资产治理”的表达

### 4. 当前人工复核 stopline

在专项门禁脚本落地前，当前批次至少执行以下人工 stopline：

1. [ ] `rg --files . | rg '(_extra|_more|_gap|_coverage)_test\\.go$'`
2. [ ] `rg -n --glob '*_test.go' '^func Test.*(Coverage|MoreBranches|Covers)' .`
3. [ ] `git diff --name-only | rg '(_test\\.go$|\\.spec\\.js$|docs/dev-plans/304-|AGENTS\\.md$)'`

## 整改优先级

### P0：高风险高体量文件

1. [X] `modules/orgunit/services/orgunit_write_service_test.go`
2. [ ] `internal/server/orgunit_api_test.go`
3. [ ] `internal/server/orgunit_field_metadata_api_test.go`
4. [ ] `internal/server/dicts_api_test.go`

目标：先把这些文件从“大而全的回归总入口”拆成按职责组织的多个稳定测试文件。

### P1：辅助层与适配层去重

1. [ ] `internal/server/handler_test.go`
2. [ ] `internal/server/orgunit_nodes_store_test.go`
3. [ ] `internal/server/orgunit_nodes_pgstore_read_test.go`
4. [ ] `internal/server/orgunit_nodes_pgstore_write_test.go`
5. [ ] `internal/superadmin/tenant_console_test.go`

目标：收敛重复 stub/fake/recorder，降低白盒耦合与重构摩擦。

### P2：低信号文件并回/删除

1. [X] `internal/server/layering_wrapper_coverage_test.go`
2. [X] `internal/server/orgunit_field_metadata_api_coverage_extra_test.go`
3. [X] `internal/routing/responder_extra_test.go`

目标：删除或并回主职责文件，不再让低信号文件独立占据测试入口。

## 高信号回归资产池（冻结清单）

以下资产作为当前优先信任的高信号回归入口：

1. [X] `e2e/tests/tp060-01-tenant-login-authz-rls-baseline.spec.js`
2. [X] `e2e/tests/tp060-02-orgunit-record-wizard.spec.js`
3. [X] `e2e/tests/tp060-02-orgunit-ext-query.spec.js`
4. [X] `e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`
5. [X] `e2e/tests/tp070b-dict-release-ui.spec.js`
6. [X] `pkg/orgunit/normalize_blackbox_test.go`
7. [X] `pkg/orgunit/nodekey_test.go`
8. [X] `pkg/orgunit/resolve_test.go`
9. [X] `pkg/authz/authz_test.go`
10. [X] `pkg/uuidv7/uuidv7_test.go`
11. [X] `internal/routing/router_test.go`
12. [X] `internal/routing/gates_test.go`
13. [X] `internal/routing/classifier_test.go`
14. [X] `internal/server/day_contract_test.go`
15. [X] `internal/server/orgunit_effective_date_sticky_sql_test.go`
16. [X] `internal/server/orgunit_allocator_integration_test.go`
17. [X] `internal/server/tenancy_middleware_test.go`
18. [X] `internal/server/orgunit_ext_payload_schema_test.go`
19. [X] `modules/orgunit/services/orgunit_write_service_test.go`
20. [X] `modules/orgunit/services/orgunit_mutation_policy_test.go`

说明：其中部分文件同时位于“优先整改对象”与“高信号资产池”中，表示它们的业务价值高，但组织形式差；整改目标是拆分与收敛，而不是简单删除。

## 分类全量映射（2026-04-21 基线）

### A：高价值

1. [X] `cmd/dbtool/orgunit_code_validate_test.go`
2. [X] `cmd/dbtool/orgunit_snapshot_bootstrap_test.go`
3. [X] `cmd/dbtool/orgunit_snapshot_test.go`
4. [X] `e2e/tests/tp060-01-tenant-login-authz-rls-baseline.spec.js`
5. [X] `e2e/tests/tp060-02-orgunit-ext-query.spec.js`
6. [X] `e2e/tests/tp060-02-orgunit-record-wizard.spec.js`
7. [X] `e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`
8. [X] `e2e/tests/tp070b-dict-release-ui.spec.js`
9. [X] `internal/routing/allowlist_test.go`
10. [X] `internal/routing/classifier_test.go`
11. [X] `internal/routing/error_catalog_test.go`
12. [X] `internal/routing/gates_test.go`
13. [X] `internal/routing/pattern_test.go`
14. [X] `internal/routing/responder_test.go`
15. [X] `internal/routing/router_test.go`
16. [X] `internal/server/authz_middleware_test.go`
17. [X] `internal/server/day_contract_test.go`
18. [X] `internal/server/host_test.go`
19. [X] `internal/server/identity_provider_test.go`
20. [X] `internal/server/orgunit_allocator_integration_test.go`
21. [X] `internal/server/orgunit_audit_snapshot_schema_test.go`
22. [X] `internal/server/orgunit_corrections_kernel_privileges_test.go`
23. [X] `internal/server/orgunit_effective_date_sticky_sql_test.go`
24. [X] `internal/server/orgunit_ext_payload_schema_test.go`
25. [X] `internal/server/orgunit_field_configs_schema_test.go`
26. [X] `internal/server/orgunit_field_metadata_validation_test.go`
27. [X] `internal/server/orgunit_list_ext_query_test.go`
28. [X] `internal/server/orgunit_projection_integration_test.go`
29. [X] `internal/server/pg_errors_test.go`
30. [X] `internal/server/tenancy_middleware_test.go`
31. [X] `internal/server/tenancy_test.go`
32. [X] `internal/superadmin/authz_middleware_test.go`
33. [X] `internal/superadmin/authz_test.go`
34. [X] `internal/superadmin/basic_auth_test.go`
35. [X] `internal/superadmin/identity_provider_test.go`
36. [X] `modules/iam/infrastructure/kratos/client_test.go`
37. [X] `modules/orgunit/domain/fieldmeta/fieldmeta_test.go`
38. [X] `modules/orgunit/infrastructure/persistence/orgunit_pg_store_policy_test.go`
39. [X] `modules/orgunit/infrastructure/persistence/orgunit_pg_store_test.go`
40. [X] `modules/orgunit/services/orgunit_append_version_precheck_test.go`
41. [X] `modules/orgunit/services/orgunit_maintain_precheck_test.go`
42. [X] `modules/orgunit/services/orgunit_mutation_policy_test.go`
43. [X] `modules/orgunit/services/orgunit_write_capabilities_test.go`
44. [X] `modules/orgunit/services/orgunit_write_service_dict_test.go`
45. [X] `modules/orgunit/services/orgunit_write_service_policy_defaults_test.go`
46. [X] `modules/orgunit/services/orgunit_write_service_test.go`
47. [X] `modules/orgunit/services/orgunit_write_unified_test.go`
48. [X] `pkg/authz/authz_test.go`
49. [X] `pkg/dict/dict_blackbox_test.go`
50. [X] `pkg/dict/dict_test.go`
51. [X] `pkg/httperr/httperr_test.go`
52. [X] `pkg/orgunit/nodekey_test.go`
53. [X] `pkg/orgunit/normalize_blackbox_test.go`
54. [X] `pkg/orgunit/resolve_query_test.go`
55. [X] `pkg/orgunit/resolve_test.go`
56. [X] `pkg/uuidv7/uuidv7_test.go`

### B：中价值

1. [X] `internal/server/convert_test.go`
2. [X] `internal/server/date_helpers_test.go`
3. [X] `internal/server/db_env_test.go`
4. [X] `internal/server/dicts_api_test.go`
5. [X] `internal/server/dicts_release_api_test.go`
6. [X] `internal/server/dicts_release_test.go`
7. [X] `internal/server/dicts_store_test.go`
8. [X] `internal/server/handler_registerresolver_error_test.go`
9. [X] `internal/server/handler_test.go`
10. [X] `internal/server/handler_utils_test.go`
11. [X] `internal/server/orgunit_api_test.go`
12. [X] `internal/server/orgunit_details_ext_fields_test.go`
13. [X] `internal/server/orgunit_field_metadata_api_test.go`
14. [X] `internal/server/orgunit_field_metadata_store_pg_methods_test.go`
15. [X] `internal/server/orgunit_field_metadata_store_test.go`
16. [X] `internal/server/orgunit_field_metadata_test.go`
17. [X] `internal/server/orgunit_nodes_ext_snapshot_test.go`
18. [X] `internal/server/orgunit_nodes_pgstore_read_test.go`
19. [X] `internal/server/orgunit_nodes_pgstore_write_test.go`
20. [X] `internal/server/orgunit_nodes_store_test.go`
21. [X] `internal/server/orgunit_write_api_test.go`
22. [X] `internal/server/session_store_test.go`
23. [X] `internal/server/tenant_context_test.go`
24. [X] `internal/superadmin/authn_test.go`
25. [X] `internal/superadmin/db_env_test.go`
26. [X] `internal/superadmin/handler_build_test.go`
27. [X] `internal/superadmin/handler_utils_test.go`
28. [X] `internal/superadmin/pg_store_test.go`
29. [X] `internal/superadmin/tenant_console_test.go`
30. [X] `modules/iam/infrastructure/persistence/dicts_store_test.go`
31. [X] `modules/iam/module_test.go`
32. [X] `modules/orgunit/services/orgunit_083b_latency_baseline_test.go`
33. [X] `modules/orgunit/services/orgunit_write_unified_error_paths_test.go`
34. [X] `modules/orgunit/services/orgunit_write_unified_validation_test.go`

### C：低价值/补洞型

1. [X] `internal/routing/responder_extra_test.go`
2. [X] `internal/server/layering_wrapper_coverage_test.go`
3. [X] `internal/server/orgunit_field_metadata_api_coverage_extra_test.go`

### H：辅助/基础设施

1. [X] `internal/server/orgunit_nodes_compat_test.go`
2. [X] `internal/server/pg_integration_test_helpers_test.go`
3. [X] `internal/server/pg_test_helpers_test.go`
4. [X] `internal/server/rand_test_helpers_test.go`
5. [X] `internal/server/test_errors_test.go`
6. [X] `internal/server/test_helpers_test.go`
7. [X] `internal/server/testmain_test.go`
8. [X] `internal/superadmin/test_helpers_test.go`
9. [X] `pkg/dict/dict_benchmark_test.go`

## 实施步骤

### Phase 0：分级冻结

1. [X] 完成本轮全仓测试资产盘点。
2. [X] 明确 `A/B/C/H` 四级分类口径。
3. [X] 确认 `301` 不可复用，新方案编号使用 `304`。

### Phase 1：文档与 owner 收口

1. [X] 在 `AGENTS.md` 文档地图中登记 `DEV-PLAN-304`。
2. [X] 将 `AGENTS.md` 的仓库级测试设计原则补记为“`300` 问题基线 + `301` 历史来源 + `304` 当前资产治理 owner”。
3. [ ] 对现有活体测试治理文档做一次 grep 自检，确认不存在把 `301` 单独写成现行 owner 的表达。

### Phase 2：P0 文件拆分试点

1. [X] 先选择 `orgunit_write_service_test.go` 作为最大体量试点。
2. [X] 再选择 `orgunit_api_test.go` 作为 `internal/server` 最大体量试点。
3. [X] 每个试点必须提交：
   - 拆分前后的文件映射
   - 迁移的职责边界
   - 是否减少 branch/coverage 风格函数
   - 是否降低 stub/fake 重复
4. [X] P0/P1 任何命中 Go 测试文件的批次，最小验证集固定为：
   - `go fmt ./...`
   - `go vet ./...`
   - `make check lint`
   - `make test`

#### `orgunit_write_service_test.go` 试点记录（2026-04-22）

1. [X] 拆分前文件：
   - `modules/orgunit/services/orgunit_write_service_test.go`（单文件承载共享 fixture、Create、Rename、Move、Enable、Disable、SetBusinessUnit、Correct、buildCorrectionPatch）
2. [X] 拆分后文件映射：
   - `modules/orgunit/services/orgunit_write_service_test.go`：只保留共享 fixture / stub / helper / `TestMain`
   - `modules/orgunit/services/orgunit_write_service_create_test.go`：承接 `Create` 入口测试
   - `modules/orgunit/services/orgunit_write_service_actions_test.go`：承接 `Rename / Move / Enable / Disable / SetBusinessUnit`
   - `modules/orgunit/services/orgunit_write_service_correct_test.go`：承接 `Correct` 主路径与 `buildCorrectionPatch`
   - `modules/orgunit/services/orgunit_write_service_ext_test.go`：继续承接扩展字段与扩展分支
   - `modules/orgunit/services/orgunit_write_service_status_rescind_test.go`：继续承接 `CorrectStatus / Rescind / helper`
3. [X] 职责边界说明：
   - 共享 stub/fake 不再散落复制，统一留在 `orgunit_write_service_test.go`
   - 入口测试按写动作职责拆分，避免继续把不同 mutation action 堆在单个总入口文件
   - `Correct` 主路径与扩展字段/状态撤销路径分离，减少跨语义跳转
4. [X] branch/coverage 风格检查：
   - 本批次未新增 `*_coverage_test.go` / `*_more_test.go` / `*_gap_test.go` / `*_extra_test.go`
   - 本批次仅做职责搬移，不通过新增补洞式测试函数维持覆盖率
5. [X] stub/fake 重复评估：
   - 共享 `orgUnitWriteStoreStub`、UUID / JSON hook、node key helper 继续单点复用
   - 新拆文件未复制新的 store fake 结构

#### `orgunit_api_test.go` 试点记录（2026-04-22）

1. [X] 拆分前文件：
   - `internal/server/orgunit_api_test.go`（单文件承载共享 fixture、读接口、`set-business-unit`、写动作、更正/撤销与错误映射）
2. [X] 拆分后文件映射：
   - `internal/server/orgunit_api_test.go`：只保留共享 fixture / stub / helper 壳文件
   - `internal/server/orgunit_business_unit_api_test.go`：承接 `set-business-unit` API
   - `internal/server/orgunit_read_api_test.go`：承接列表、详情、版本、审计、搜索等读接口
   - `internal/server/orgunit_write_actions_api_test.go`：承接创建、rename、move、enable、disable、correct、rescind 与错误映射
3. [X] 职责边界说明：
   - `internal/server/orgunit_write_api_test.go` 继续聚焦 `/org/api/org-units/write`，不与本批次拆分职责重叠
   - `orgunit_api_test.go` 不再继续堆放入口测试，仅作为共享 fake/store/helper 单点复用容器
   - 读接口、业务单元切换、写动作与错误映射按协议职责拆分，降低跨场景跳转成本
4. [X] branch/coverage 风格检查：
   - 本批次未新增 `*_coverage_test.go` / `*_more_test.go` / `*_gap_test.go` / `*_extra_test.go`
   - 本批次仅做职责搬移与文件收口，不通过补洞式测试维持覆盖率
5. [X] stub/fake 重复评估：
   - 共享 `resolveOrgCodeStore`、`orgUnitListPageReaderStore`、`orgUnitDetailsExtStoreStub`、`orgUnitWriteServiceStub` 继续集中在 `orgunit_api_test.go`
   - 新拆文件未复制新的 API fake / helper 结构

### Phase 3：低信号文件清理

1. [X] 评估 `C` 类文件是否可以直接并回主职责文件。
2. [X] 若无法直接删除，必须记录保留理由与退出条件。
3. [X] 不得新增新的 `C` 类独立测试文件替代旧文件。
4. [ ] 若本 phase 仅改文档与门禁说明，最小验证集固定为 `make check doc`。

#### Phase 3 执行记录（2026-04-22）

1. [X] `internal/server/layering_wrapper_coverage_test.go`
   - 已并回 `internal/server/dicts_store_test.go`
   - 承接方式：将 `TestDictCompatibilityWrappers` 作为 `TestDictPGStore_ListDictValues_AndOptions_Coverage` 的同职责子场景收口
   - 退出理由：该文件只承载字典 store 兼容包装路径，不需要继续独立占据测试入口
2. [X] `internal/server/orgunit_field_metadata_api_coverage_extra_test.go`
   - 已并回 `internal/server/orgunit_field_metadata_api_test.go`
   - 承接方式：将 `dict list error => 500` 场景并入 `TestHandleOrgUnitFieldOptionsAPI`
   - 退出理由：该文件只补一个 API 错误分支，直接并回主职责文件后信号更高
3. [X] `internal/routing/responder_extra_test.go`
   - 已并回 `internal/routing/responder_test.go`
   - 承接方式：将 AI reply 错误码映射断言并入 `TestKnownErrorMessage_AllCases`
   - 退出理由：该文件只补一个消息映射断言，不需要继续独立占据 routing 测试入口
4. [X] 本批次未新增新的 `*_coverage_test.go` / `*_extra_test.go` / `*_gap_test.go` / `*_more_test.go`

## 验收标准

1. [X] `DEV-PLAN-304` 已进入 `AGENTS.md` 文档地图。
2. [X] `AGENTS.md` 的活体测试设计入口已完成 owner 收口，不再把 `301` 单独表述为现行执行 owner。
3. [X] 至少完成 `2` 个 P0 巨型文件的职责拆分试点。
4. [X] `C` 类低信号文件数量相对本计划基线下降。
5. [ ] 至少一个专项门禁或人工 stopline 已落地并有执行记录，能够阻断新的 `extra/more/gap/coverage` 风格文件或函数名。
6. [ ] 本计划实施批次未通过降低 coverage 阈值、扩大排除项来达成“治理完成”。

## 测试与覆盖率

- **覆盖率口径**：沿用仓库当前 SSOT，不在本计划中改变 line coverage 阈值与统计口径。
- **统计范围**：沿用 `Makefile`、`scripts/ci/test.sh`、`scripts/ci/coverage.sh` 与 `config/coverage/policy.yaml`。
- **目标阈值**：保持当前仓库门禁要求，不降级。
- **证据记录**：本计划只固化治理策略；实际执行记录按批次写入对应 dev-record 或后续子计划。

## 关联文档

1. [X] `docs/dev-plans/300-test-system-investigation-report.md`
2. [X] `docs/archive/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
3. [X] `docs/archive/dev-plans/302-internal-server-residual-gap-coverage-closure-plan.md`
4. [X] `docs/dev-plans/303-repo-final-gap-coverage-test-tail-closure-plan.md`
5. [X] `AGENTS.md`
