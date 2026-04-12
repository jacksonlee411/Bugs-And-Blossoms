# DEV-PLAN-350A 执行日志：Assistant OrgUnit `add_version / insert_version` 收口

**状态**: 已完成并封账（2026-04-12 21:35 CST）

## 1. 本轮交付范围

1. [X] 为 `add_version / insert_version` 新增正式 `PolicyContext` / `PrecheckProjection` contract，并将 digest/version 口径冻结到动作级快照。
2. [X] 将两动作的 Assistant dry-run、confirm、commit、task snapshot / submit request 与 phase / missing-fields 推导切到同一 projection contract。
3. [X] 将两动作的写前 explain / precheck 统一到同一服务层核心，不再依赖动作私有解释分支。
4. [X] 为两动作补齐 `assistantActionSpec` 的策略契约字段、只读工具声明、mutation policy key 与 capability bucket key。
5. [X] 保持 fail-closed：历史无 projection snapshot 的 turn / task 继续不可 confirm、不可 commit、不可 submit。

## 2. 关键代码落点

1. [X] 服务层动作级 precheck contract：
   - `modules/orgunit/services/orgunit_append_version_precheck.go`
2. [X] server 侧 adapter / projection snapshot：
   - `internal/server/orgunit_append_version_precheck_adapter.go`
   - `internal/server/assistant_orgunit_version_projection.go`
   - `internal/server/assistant_orgunit_version_policy_precheck.go`
3. [X] Assistant 主链接线：
   - `internal/server/assistant_semantic_state.go`
   - `internal/server/assistant_api.go`
   - `internal/server/assistant_persistence.go`
   - `internal/server/assistant_task_store.go`
   - `internal/server/assistant_phase_snapshot.go`
   - `internal/server/assistant_action_registry.go`
4. [X] 回归测试与测试夹具：
   - `internal/server/assistant_272_api_matrix_test.go`
   - `internal/server/assistant_272_task_lifecycle_test.go`
   - `internal/server/assistant_action_registry_test.go`
   - `internal/server/assistant_phase_snapshot_test.go`
   - `internal/server/assistant_runtime_proposal_gate_test.go`
   - `internal/server/assistant_task_store_test.go`
   - `internal/server/assistant_test_helpers_test.go`

## 3. 实施过程中的实际问题

1. [X] 初次接线后，`TestAssistant272TurnAPI_CreateAndConfirmMatrix` 的 `add_version / insert_version` 在 confirm 阶段仍返回 `conversation_confirmation_required`。
2. [X] 通过直接检查 validated turn 的 dry-run / projection 状态，定位到根因并非 projection 缺失，而是 projection 已生成但被 `policy_missing` 拒绝。
3. [X] 根因进一步收敛为：测试态 `setid` registry fallback 只为 `create` 提供默认 field decision，未覆盖 `add_version / insert_version` 共享的 baseline write field policy。
4. [X] 修复方式是在测试态 registry fallback 中补齐 `org.orgunit_write.field_policy` 的默认字段裁决，使 `name / parent_org_code / status / is_business_unit / manager_pernr` 在 in-memory 场景下也能得到与正式主链一致的 field decision。

## 4. 验证记录

1. [X] `go test ./pkg/fieldpolicy ./internal/server/... ./modules/orgunit/infrastructure/persistence/... ./modules/orgunit/services/...`
2. [X] `go fmt ./...`
3. [X] `go vet ./...`
4. [X] `make check lint`
5. [X] `make check doc`
6. [X] `make test`（coverage `98.00% >= 98.00%`）
7. [X] `internal/server` 中原先阻塞 `350A` 收口的 add/insert confirm matrix 已恢复通过。
8. [X] 相关 contract / task snapshot / phase / gate / action registry 回归已并入同一批次执行，无额外测试入口或临时补洞文件。

## 5. 结论与后续

1. [X] `DEV-PLAN-350A` 的范围已完成，`375M2` 出口条件达成。
2. [X] `add_version / insert_version` 已成为 `business_action` 正式 contract 主链的一部分，可作为后续 `350B / 350C` 的直接样板。
3. [X] `350A` 的代码、文档、证据与 coverage 门禁已完成同批封账。
4. [X] 下一步第一优先级：`375M4 / DEV-PLAN-350B`，已于 2026-04-13 完成动作 contract 收口，执行记录见 `docs/dev-records/dev-plan-350b-execution-log.md`。
5. [ ] 可并行启动：`375M3 / DEV-PLAN-370A`。
6. [ ] `375M4` 剩余 compat API 硬切继续由 `360 / 360A Phase 2` 承接。
