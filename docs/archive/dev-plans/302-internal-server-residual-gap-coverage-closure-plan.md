# DEV-PLAN-302：`internal/server` 残留 `gap/coverage` 测试文件收口计划

**状态**: 已归档（历史来源；原状态：已完成，2026-04-08 CST）

执行补记（2026-04-08 CST）：

1. [X] `Phase A` 已完成，首批 `10` 个低风险残留文件已收口。
2. [X] `Phase B` 已完成，`staffing / assistant reply / assistant 272 / mixed additional` 这批 `4` 个中体量残留文件已收口。
3. [X] `Phase C` 已完成，`orgunit pgstore` 两个残留 `coverage` 文件已改为正式 `read/write` 职责文件。
4. [X] `Phase D` 已完成，`assistant 240/268` 这批计划编号文件已改为职责化正式测试文件名。
5. [X] `Phase E` 已完成，`assistant API / persistence` 主簇残留文件已全部改为职责化正式测试文件名。
6. [X] `internal/server` 中残留 `gap/coverage` 文件已由初始盘点的 `28` 个降到 `0` 个。
7. [X] 当前 `internal/server` 目录下已无残留 `*_gap_test.go` / `*_coverage_test.go` 文件。
8. [X] 验证已通过：
   - `go test ./internal/server -count=1`
   - `make check doc`

## 背景

`DEV-PLAN-300` 已完成全仓测试体系调查，`DEV-PLAN-301` 已完成首轮 Go 测试分层治理与样板落地。  
2026-04-08 对当前树再次盘点后确认：

1. [X] `pkg/**` 层与 `modules/orgunit/services` 的本轮补口已完成。
2. [X] `internal/server/assistant_task_store_gap_test.go` 已删除，重复定义编译冲突已清除。
3. [X] 初始盘点时，`internal/server` 目录下共有 `28` 个 `*_gap_test.go` / `*_coverage_test.go` 文件。
4. [X] 其中 `18` 个文件已经被 `DEV-PLAN-301` 文档写成“已并回主测试文件/已完成收口”，说明当前问题不只是历史技术债，也包含一次“文档完成态领先于当前代码树”的漂移。
5. [X] 当前在 `Phase A/B/C/D/E` 完成后，残留文件数已降为 `0`。

本计划用于承接这批残留项，避免继续扩大 `301` 范围，同时把“恢复带回的旧补洞文件”按真实现状重新拉回可执行轨道。

## 目标

1. [X] 将 `internal/server` 残留的 `*_gap_test.go` / `*_coverage_test.go` 文件按职责收口，不再继续堆补洞入口。
2. [X] 纠正 `301` 已完成态与当前代码树之间的漂移，恢复“文档状态 = 代码真实状态”。
3. [X] 在不降低覆盖率门禁、不扩大排除项的前提下，优先消除低风险、已知承接目标明确的残留文件。
4. [X] 对大体量 `assistant API / persistence` 文件族采用“先拆职责，再吸收旧 coverage/gap 文件”的方式收口，禁止机械并回单个巨型主文件。

## 非目标

1. [X] 不重新打开 `DEV-PLAN-301`，`301` 保持关闭态，本计划仅作为其后的增量承接。
2. [X] 不通过降低 coverage 阈值、扩大忽略项、临时跳过测试来消化残留文件。
3. [X] 不为收口测试而引入新的 legacy 测试入口、双链路验证或生产代码旁路。

## 当前盘点

### 1. 总量与家族分布

截至 2026-04-08 CST，`internal/server` 残留文件共 `28` 个：

| 家族 | 数量 | 说明 |
| --- | --- | --- |
| `assistant` | 16 | 体量最大，且包含多个子计划编号文件、主簇 `api/persistence` 文件与 mixed helper 文件 |
| `handler route wiring` | 5 | 目标主文件非常明确，应优先消化 |
| `orgunit` | 4 | 其中 2 个是 `pgstore` 大文件，需单独按读写职责拆分 |
| `dict/setid` | 2 | 当前主测试文件已存在，属于低风险收口项 |
| `staffing` | 1 | 目标范围清晰，可在首轮或第二轮收口 |

### 2. 已被 `301` 文档写成“已完成”但当前仍存在的文件

以下 `18` 个文件已在 `301` 文档中被表述为“已并回主测试文件/已收口”，但当前代码树仍保留原文件：

1. [ ] `assistant_api_243_gap_test.go`
2. [ ] `assistant_api_coverage_test.go`
3. [ ] `assistant_api_gap_test.go`
4. [ ] `assistant_model_gateway_coverage_test.go`
5. [ ] `assistant_persistence_243_gap_test.go`
6. [ ] `assistant_persistence_coverage_test.go`
7. [ ] `assistant_persistence_gap_test.go`
8. [ ] `dicts_extra_coverage_test.go`
9. [ ] `handler_assistant_routes_coverage_test.go`
10. [ ] `handler_dicts_routes_coverage_test.go`
11. [ ] `handler_orgunit_field_config_routes_coverage_test.go`
12. [ ] `handler_orgunit_field_policy_routes_coverage_test.go`
13. [ ] `handler_orgunit_write_routes_coverage_test.go`
14. [ ] `orgunit_field_metadata_api_106a_coverage_test.go`
15. [ ] `orgunit_nodes_pgstore_coverage_test.go`
16. [ ] `orgunit_nodes_pgstore_read_paths_coverage_test.go`
17. [ ] `orgunit_nodes_store_coverage_test.go`
18. [ ] `setid_strategy_registry_api_coverage_test.go`

这意味着 `302` 的第一原则不是“继续写新故事”，而是先把当前树恢复为与 `301` 文档口径一致，或把旧文档显式改为由 `302` 承接。

## 分批策略

### Phase A：低风险漂移回收批

目标：优先收口“主文件目标明确、`301` 已写过目标、改动风险低”的残留文件。

本批次范围：

1. [X] `handler_assistant_routes_coverage_test.go` → `handler_test.go`
2. [X] `handler_dicts_routes_coverage_test.go` → `handler_test.go`
3. [X] `handler_orgunit_field_config_routes_coverage_test.go` → `handler_test.go`
4. [X] `handler_orgunit_field_policy_routes_coverage_test.go` → `handler_test.go`
5. [X] `handler_orgunit_write_routes_coverage_test.go` → `handler_test.go`
6. [X] `setid_strategy_registry_api_coverage_test.go` → `setid_strategy_registry_api_test.go`
7. [X] `dicts_extra_coverage_test.go` → `dicts_api_test.go`
8. [X] `orgunit_field_metadata_api_106a_coverage_test.go` → `orgunit_field_metadata_api_test.go`
9. [X] `orgunit_nodes_store_coverage_test.go` → `orgunit_nodes_store_test.go`
10. [X] `assistant_model_gateway_coverage_test.go` → `assistant_model_gateway_more_test.go`

执行原则：

1. [X] 以上文件未新增新的 `coverage`/`gap` 替代文件。
2. [X] 本批次优先吸收到既有正式主文件，未保留原 `coverage` 文件名。
3. [X] 本批次完成后已回写 `302` 执行进度，后续可继续推进更大体量文件。

`Phase A` 完成说明：

1. [X] 路由装配类 5 个 coverage 文件已收进 `handler_test.go`。
2. [X] `setid/dict/orgunit_field_metadata/orgunit_nodes_store/assistant_model_gateway` 5 个 coverage 文件已吸收到对应正式测试文件。
3. [X] 当前剩余文件已收敛到 `assistant` 主簇、`orgunit pgstore` 与 `staffing positions` 三个更清晰的后续批次。

### Phase B：中体量单家族职责批

目标：处理已有同家族正式测试文件、但仍需要按职责拆分吸收的中等体量文件。

本批次范围：

1. [ ] `staffing_handlers_positions_api_coverage_test.go`
   - 推荐承接：`staffing_positions_api_test.go` 与 `staffing_test.go`
2. [ ] `assistant_reply_more_coverage_test.go`
   - 推荐承接：`assistant_reply_more_test.go`、`assistant_reply_extra_test.go`、`assistant_reply_nlg_test.go`
3. [ ] `assistant_272_coverage_test.go`
   - 推荐承接：`assistant_272_helper_test.go`、`assistant_272_api_matrix_test.go`、`assistant_272_task_lifecycle_test.go`
4. [ ] `assistant_additional_coverage_test.go`
   - 推荐承接：按职责拆分到 `handler_test.go`、`orgunit_field_policy_api_test.go`、`orgunit_field_metadata_api_test.go`

执行原则：

1. [ ] `assistant_additional_coverage_test.go` 属于跨家族 mixed 文件，禁止整文件改名后继续保留。
2. [ ] `reply`、`272`、`staffing positions` 应保持“按职责承接”，不要再形成新的 plan-number 覆盖文件。

`Phase B` 完成说明：

1. [X] `staffing_handlers_positions_api_coverage_test.go` 已收口为 `staffing_positions_api_test.go`。
2. [X] `assistant_reply_more_coverage_test.go` 已收口为 `assistant_reply_more_test.go`。
3. [X] `assistant_272_coverage_test.go` 已收口为 `assistant_272_helper_test.go`。
4. [X] `assistant_additional_coverage_test.go` 已按职责拆分到 `handler_test.go`、`orgunit_field_policy_api_test.go` 与 `orgunit_field_metadata_api_test.go`。

### Phase C：`orgunit pgstore` 大文件批

目标：收口 `orgunit_nodes` 家族中仍残留的两个大体量 `pgstore` coverage 文件。

本批次范围：

1. [ ] `orgunit_nodes_pgstore_coverage_test.go`
2. [ ] `orgunit_nodes_pgstore_read_paths_coverage_test.go`

执行原则：

1. [ ] 不建议机械并入 `orgunit_nodes_store_test.go`，因为 `memory store` 与 `pgstore` 的职责边界不同。
2. [ ] 如当前仓库不存在正式 `pgstore` 主测试文件，允许新增：
   - `orgunit_nodes_pgstore_write_test.go`
   - `orgunit_nodes_pgstore_read_test.go`
3. [ ] 本批次的目标是去掉 `coverage` 命名并按读/写职责分组，而不是把所有 PG 路径硬塞到单一大文件。

`Phase C` 完成说明：

1. [X] `orgunit_nodes_pgstore_coverage_test.go` 已收口为 `orgunit_nodes_pgstore_write_test.go`。
2. [X] `orgunit_nodes_pgstore_read_paths_coverage_test.go` 已收口为 `orgunit_nodes_pgstore_read_test.go`。
3. [X] `orgunit_nodes` 文件族当前已不再残留 `coverage` 命名测试文件。

### Phase D：`assistant` 子计划编号文件批

目标：处理 `assistant` 家族中仍以计划编号命名的残留补洞文件，避免继续以子计划编号当测试入口。

本批次范围：

1. [ ] `assistant_240a_gap_test.go`
2. [ ] `assistant_240c_coverage_test.go`
3. [ ] `assistant_240d_additional_coverage_test.go`
4. [ ] `assistant_240d_commit_path_coverage_test.go`
5. [ ] `assistant_268_runtime_coverage_test.go`
6. [ ] `assistant_268_semantic_closure_coverage_test.go`

推荐承接方向：

1. [ ] `240a`：拆入 `assistant_action_registry_test.go`、`assistant_api_test.go`、`assistant_persistence_*` 家族
2. [ ] `240c/268`：拆入 `assistant_intent_pipeline_test.go`、`assistant_knowledge_runtime_test.go`、`assistant_model_gateway_more_test.go`
3. [ ] `240d`：拆入 `assistant_tasks_api_test.go`、`assistant_task_store_test.go`、`assistant_api_test.go`、`assistant_persistence_*` 家族

执行原则：

1. [ ] 不保留 `assistant_240*` / `assistant_268*` 这类“计划编号即测试入口”的形态。
2. [ ] 先按当前正式文件族拆目标，再删除旧文件；不要直接把整份文件并到 `assistant_api_test.go`。

`Phase D` 完成说明：

1. [X] `assistant_240a_gap_test.go` 已收口为 `assistant_action_flow_additional_test.go`。
2. [X] `assistant_240c_coverage_test.go` 已收口为 `assistant_runtime_closure_test.go`。
3. [X] `assistant_240d_additional_coverage_test.go` 已收口为 `assistant_idempotency_helper_test.go`。
4. [X] `assistant_240d_commit_path_coverage_test.go` 已收口为 `assistant_commit_workflow_test.go`。
5. [X] `assistant_268_runtime_coverage_test.go` 已收口为 `assistant_semantic_runtime_test.go`。
6. [X] `assistant_268_semantic_closure_coverage_test.go` 已收口为 `assistant_semantic_closure_test.go`。
7. [X] 当前残留已不再包含 `240*` / `268*` 这类计划编号测试入口。

### Phase E：`assistant API / persistence` 主簇重组批

目标：处理当前体量最大、风险最高、但收益也最高的残留文件族。

本批次范围：

1. [ ] `assistant_api_243_gap_test.go`
2. [ ] `assistant_api_gap_test.go`
3. [ ] `assistant_api_coverage_test.go`
4. [ ] `assistant_persistence_243_gap_test.go`
5. [ ] `assistant_persistence_gap_test.go`
6. [ ] `assistant_persistence_coverage_test.go`

当前规模：

1. [ ] `assistant_api_coverage_test.go` 约 `1494` 行
2. [ ] `assistant_persistence_gap_test.go` 约 `1620` 行
3. [ ] `assistant_persistence_coverage_test.go` 约 `989` 行

执行原则：

1. [ ] 禁止把更多 case 机械追加到单一 `assistant_api_test.go` 或 `assistant_persistence_coverage_test.go`。
2. [ ] 先建立职责文件，再吸收旧 coverage/gap 文件。推荐职责切分：
   - `assistant API`：conversation handlers、turn action handlers、service helpers、error mapping、list/cursor helper
   - `assistant persistence`：tx/db helpers、create/load paths、confirm/commit、idempotency、task dispatch/execute、cursor/listing
3. [ ] `243` 子簇只表示历史来源，不应继续作为独立测试文件名。

`Phase E` 完成说明：

1. [X] `assistant_api_243_gap_test.go` 已收口为 `assistant_api_turn_error_mappings_test.go`。
2. [X] `assistant_api_gap_test.go` 已收口为 `assistant_api_error_paths_test.go`。
3. [X] `assistant_api_coverage_test.go` 已收口为 `assistant_api_matrix_test.go`。
4. [X] `assistant_persistence_243_gap_test.go` 已收口为 `assistant_persistence_turn_branches_test.go`。
5. [X] `assistant_persistence_gap_test.go` 已收口为 `assistant_persistence_error_paths_test.go`。
6. [X] `assistant_persistence_coverage_test.go` 已收口为 `assistant_persistence_flow_test.go`。
7. [X] 当前 `assistant` 主簇已不再残留 `gap/coverage` 命名测试文件。

## 执行顺序建议

1. [ ] 先完成 `Phase A`，尽快回收最明显的文档-代码漂移。
2. [ ] 再做 `Phase B`，把中等体量、目标主文件明确的文件族收口掉。
3. [ ] 然后处理 `Phase C`，单独把 `orgunit pgstore` 树从 `coverage` 命名迁出。
4. [ ] 最后推进 `Phase D/E`，集中处理 `assistant` 家族的大体量残留。

选择这个顺序的原因：

1. [ ] 先做低风险项，可以最快恢复文档可信度。
2. [ ] `assistant API / persistence` 当前文件体量太大，若先做，容易把新文档再次写成“计划完成”但代码还没真正落地。
3. [ ] `orgunit pgstore` 与 `assistant` 主簇都需要先定职责边界，再谈并入，不适合作为第一批“快收口”目标。

## 验收标准

1. [X] `internal/server` 目录下已不再残留本计划覆盖范围内的 `*_gap_test.go` / `*_coverage_test.go` 文件。
2. [X] 所有被收口的测试族都存在明确的正式承接入口，不再以 `gap/coverage` 作为长期主文件。
3. [X] `DEV-PLAN-301` 与 `DEV-PLAN-302` 的文档口径已与当前代码树对齐，不再出现“文档已完成但文件仍在”的漂移。
4. [X] 相关 `go test` 与 `make check doc` 已通过。

## 建议验证命令

1. [X] `go test ./internal/server -count=1`
2. [ ] 每批执行时追加对应家族的定向 `-run` 命令，避免一次性吞掉全量回归。
3. [X] `make check doc`

## 附录：当前残留文件清单

1. [ ] `assistant_240a_gap_test.go`
2. [ ] `assistant_240c_coverage_test.go`
3. [ ] `assistant_240d_additional_coverage_test.go`
4. [ ] `assistant_240d_commit_path_coverage_test.go`
5. [ ] `assistant_268_runtime_coverage_test.go`
6. [ ] `assistant_268_semantic_closure_coverage_test.go`
7. [ ] `assistant_272_coverage_test.go`
8. [ ] `assistant_additional_coverage_test.go`
9. [ ] `assistant_api_243_gap_test.go`
10. [ ] `assistant_api_coverage_test.go`
11. [ ] `assistant_api_gap_test.go`
12. [ ] `assistant_model_gateway_coverage_test.go`
13. [ ] `assistant_persistence_243_gap_test.go`
14. [ ] `assistant_persistence_coverage_test.go`
15. [ ] `assistant_persistence_gap_test.go`
16. [ ] `assistant_reply_more_coverage_test.go`
17. [ ] `dicts_extra_coverage_test.go`
18. [ ] `handler_assistant_routes_coverage_test.go`
19. [ ] `handler_dicts_routes_coverage_test.go`
20. [ ] `handler_orgunit_field_config_routes_coverage_test.go`
21. [ ] `handler_orgunit_field_policy_routes_coverage_test.go`
22. [ ] `handler_orgunit_write_routes_coverage_test.go`
23. [ ] `orgunit_field_metadata_api_106a_coverage_test.go`
24. [ ] `orgunit_nodes_pgstore_coverage_test.go`
25. [ ] `orgunit_nodes_pgstore_read_paths_coverage_test.go`
26. [ ] `orgunit_nodes_store_coverage_test.go`
27. [ ] `setid_strategy_registry_api_coverage_test.go`
28. [ ] `staffing_handlers_positions_api_coverage_test.go`
