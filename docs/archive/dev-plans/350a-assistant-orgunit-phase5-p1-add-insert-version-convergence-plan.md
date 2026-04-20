# DEV-PLAN-350A：Assistant OrgUnit 八动作统一收口 Phase 5 P1——`add_version / insert_version`

**状态**: 已完成并封账（2026-04-12 21:35 CST）

> 本文从 `DEV-PLAN-350` Phase 5 与 `DEV-PLAN-375` 的 `375M2` 拆分而来，作为 `add_version / insert_version` 两动作统一收口的实施 SSOT。  
> `DEV-PLAN-350` 继续持有总 contract 裁决权；本文只负责本批次的范围、步骤、验收与证据。

## 1. 背景与定位

1. [X] `create_orgunit / create_org` 已完成统一 `PolicyContext -> 唯一 PDP -> Mutation Policy -> PrecheckProjection` 样板。
2. [X] `add_version / insert_version` 已补齐与 create 等级一致的动作 contract、projection 快照与统一写前解释收口。
3. [X] 本批验证通过：新增动作可通过补 `ActionSchema + Projection mapping + CommitAdapter` 收口主链，而不需要再引入新的运行时回退链路。

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 为 `add_version / insert_version` 冻结与 create 对齐的动作级 `PolicyContext` / `PrecheckProjection` contract。
2. [X] 将两动作的 dry-run / confirm / commit / task submit / 写前解释收口到同一 projection contract。
3. [X] 为两动作补齐 `assistantActionSpec` 的策略契约字段、digest/version 快照与 fail-closed 语义。
4. [X] 在不新增工具名的前提下，让 `orgunit_action_precheck`、`orgunit_field_explain` 返回两动作的正式受控视图。

### 2.2 非目标

1. [X] 未覆盖 `correct / rename / move / disable / enable`。
2. [X] 未引入 Markdown action source 或 `370` 的 compiler/runtime 变更。
3. [X] 未新增写工具、第二提交入口或 compat API。

## 3. 关键边界

1. [X] `ActionSchema`、`PolicyContextContractVersion`、`PrecheckProjectionContractVersion` 的正式裁决继续以 `DEV-PLAN-350` 为准。
2. [X] `assistantActionSpec` 与现有 `assistant_action_registry.go` 继续作为本批执行面 SSOT，未引入 Markdown registry。
3. [X] 历史无 projection snapshot 的两动作 turn/task 继续 fail-closed，只允许读展示，不允许 confirm/commit/submit。
4. [X] 只读工具名继续沿用：
   - `orgunit_candidate_lookup`
   - `orgunit_candidate_snapshot`
   - `orgunit_action_precheck`
   - `orgunit_field_explain`

## 4. 实施步骤

1. [X] 在 `modules/orgunit/services` 为两动作补齐动作级 `PolicyContext`、`PrecheckProjection`、digest/version 口径。
2. [X] 在 `internal/server` 把两动作的 dry-run / confirm / commit / task submit 切到统一 projection contract。
3. [X] 在写服务前置解释路径中复用同一 precheck/裁决核心，消除动作私有解释分支。
4. [X] 补齐 `assistantActionSpec` 的策略契约字段与回归测试，证明主链不再需要新增 server 层特判。

## 5. 验收与测试

1. [X] 批次验证：
   `go test ./pkg/fieldpolicy ./internal/server/... ./modules/orgunit/infrastructure/persistence/... ./modules/orgunit/services/...`
2. [X] 最终收口验证：
   `go fmt ./...`
   `go vet ./...`
   `make check lint`
   `make check doc`
   `make test`（coverage `98.00% >= 98.00%`）
3. [X] 同一输入下，Assistant dry-run、tool explain、写前解释输出同一结论、错误码、`effective_policy_version`、`resolved_setid`、`precheck_projection_digest`。
4. [X] 历史无 projection snapshot 的两动作 turn/task 继续 fail-closed。
5. [X] `assistantActionSpec` 对两动作补齐后，Runtime 主链继续沿用统一 projection / gate 消费路径，不再新增额外回退链路。

## 6. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
3. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
4. `docs/dev-records/dev-plan-350a-execution-log.md`
