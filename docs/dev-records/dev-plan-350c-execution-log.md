# DEV-PLAN-350C 执行日志：Assistant OrgUnit `disable / enable` 收口

**状态**: 已完成并封账（2026-04-13 16:05 CST）

## 1. 本轮交付范围

1. [X] 为 `disable / enable` 冻结正式 `PolicyContext` / `PrecheckProjection` contract，并沿用 `350A / 350B` 已冻结的 digest/version/snapshot 口径。
2. [X] 将两动作的 dry-run、confirm、commit、task snapshot / submit 与写前 explain 收口到统一 version projection 主链。
3. [X] 将 `disable / enable` 的 action registry metadata、policy precheck binding、projection validation 与测试夹具统一扩展到八动作共用 contract 面。
4. [X] 保持 fail-closed：缺 projection、缺 digest/version、缺状态/有效期事实时，不回退旧平台、不猜测提交语义。
5. [X] 保持单链路：未新增写工具、第二提交入口、legacy fallback 或知识侧回流。

## 2. 关键代码落点

1. [X] 服务层动作级 precheck contract：
   - `modules/orgunit/services/orgunit_maintain_precheck.go`
2. [X] server 侧 adapter / projection snapshot / precheck 接线：
   - `internal/server/orgunit_maintain_precheck_adapter.go`
   - `internal/server/assistant_orgunit_version_projection.go`
   - `internal/server/assistant_orgunit_version_policy_precheck.go`
3. [X] Assistant 主链接线与动作契约：
   - `internal/server/assistant_action_registry.go`
   - `internal/server/assistant_task_store_test.go`
   - `internal/server/assistant_commit_workflow_test.go`
   - `internal/server/assistant_272_task_lifecycle_test.go`
4. [X] 回归测试与 coverage 收口：
   - `internal/server/assistant_orgunit_version_contract_test.go`
   - `modules/orgunit/services/orgunit_maintain_precheck_test.go`
   - `internal/server/orgunit_precheck_adapter_test.go`
   - `internal/server/assistant_knowledge_markdown_runtime_test.go`
   - `internal/server/assistant_knowledge_runtime_more_test.go`
   - `internal/server/assistant_semantic_runtime_test.go`

## 3. 实施过程中的实际问题

1. [X] `disable / enable` 虽然不需要像 create/move 那样处理复杂候选确认，但状态切换与有效期语义必须落回同一 maintain precheck 主链，否则 confirm / commit / task submit 会再次分叉。
2. [X] 本轮将 `disable / enable` 与 `correct / rename / move` 对齐到同一 projection / precheck / policy 接线后，主阻塞转为 coverage 门禁而不是行为错误。
3. [X] `make test` 初始停在 `97.80%`，随后通过补 `assistant knowledge markdown/runtime`、semantic helper 与 `disable / enable` 邻域 contract 测试，将 coverage 逐步抬到 `98.00%`。
4. [X] 覆盖率收口过程中未降低阈值、未扩大排除项，也未通过 legacy 分支或伪 fallback 绕过门禁。

## 4. 验证记录

1. [X] `go test ./modules/orgunit/services/...`
2. [X] `go test ./internal/server/...`
3. [X] `go vet ./...`
4. [X] `make check lint`
5. [X] `make test`（coverage `98.00% >= 98.00%`）
6. [X] `make check doc`

## 5. 结论与后续

1. [X] `DEV-PLAN-350C` 的范围已完成，`disable / enable` 已成为 `business_action` 正式 contract 主链的一部分。
2. [X] `350` 八动作 contract 已全部冻结，`370B` 所需的动作 contract 前置条件已满足。
3. [X] `375M5` 的平台退役代码批次已由 `360 / 360A Phase 3/4` 承接完成；剩余仅主链 E2E 复验作为总体验收尾项。
4. [ ] 可并行推进 `375M6 / DEV-PLAN-370B`，聚焦更深层动作知识散点 hard cut 与 contract / knowledge 强分离。
