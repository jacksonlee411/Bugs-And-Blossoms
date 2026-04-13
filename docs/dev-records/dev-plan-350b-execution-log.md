# DEV-PLAN-350B 执行日志：Assistant OrgUnit `correct / rename / move` 收口

**状态**: 已完成并封账（2026-04-13 06:45 CST）

## 1. 本轮交付范围

1. [X] 为 `correct / rename / move` 新增正式 `PolicyContext` / `PrecheckProjection` contract，并复用 `350A` 已冻结的 digest/version/snapshot 口径。
2. [X] 将三动作的 Assistant dry-run、confirm、task snapshot / submit request、phase 推导与写前 explain 收口到同一 version projection 主链。
3. [X] 将 `correct / rename / move` 的 action registry metadata、policy precheck binding、projection validation 与测试夹具统一扩展到同一 contract 面。
4. [X] 保持 fail-closed：缺 projection snapshot、缺 digest/version、缺目标版本事实时，不回退旧平台、不猜测提交语义。
5. [X] 保持单链路：未新增写工具、第二提交入口或长期 compat API/legacy 分支。

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
   - `internal/server/assistant_test_helpers_test.go`
   - `internal/server/assistant_272_task_lifecycle_test.go`
4. [X] 运行态 / in-memory 事实补齐：
   - `internal/server/orgunit_nodes.go`
   - `internal/server/orgunit_nodes_store_test.go`
5. [X] 回归测试与 helper 覆盖补齐：
   - `modules/orgunit/services/orgunit_maintain_precheck_test.go`
   - `modules/orgunit/services/orgunit_append_version_precheck_test.go`
   - `internal/server/assistant_orgunit_version_contract_test.go`
   - `internal/server/orgunit_precheck_adapter_test.go`

## 3. 实施过程中的实际问题

1. [X] 初次接线后，`correct` 在 Assistant confirm matrix 中返回 `409`，根因并非 projection 缺失，而是测试态/in-memory store 无法为 target effective date 提供 mutation target event。
2. [X] 修复方式是在 `orgUnitMemoryStore` 中补齐 `ResolveMutationTargetEvent(...)`，对已有节点返回 `CREATE` 事件事实，使 `correct` 的 write capability / mutation policy 能沿正式 contract 主链裁决。
3. [X] 通过后，`make test` 仍仅因总覆盖率失败：先停在 `97.80%`，补相邻 helper / clone / adapter / node-store 分支后提升到 `97.90%`。
4. [X] 最后一轮通过补 `missing_target_effective_date` 映射、adapter 错误透传与 memory store 非法 node key 分支，将 coverage 拉到 `98.00%`，未降低阈值、未扩大排除项。

## 4. 验证记录

1. [X] `go test ./modules/orgunit/services/...`
2. [X] `go test ./internal/server/...`
3. [X] `go vet ./...`
4. [X] `make check lint`
5. [X] `make test`（coverage `98.00% >= 98.00%`）
6. [X] `make check doc`

## 5. 结论与后续

1. [X] `DEV-PLAN-350B` 的范围已完成，`correct / rename / move` 已成为 `business_action` 正式 contract 主链的一部分。
2. [X] `375M4` 中“动作 contract 收口”子目标已达成；`compat API` 硬切与 runtime 主链硬切仍由 `360 / 360A Phase 2` 继续承接，因此 `375M4` 尚未整体封账。
3. [ ] 可并行启动：`375M3 / DEV-PLAN-370A`。
4. [ ] `375M4` 剩余 compat API 硬切完成后，再进入 `375M5 / DEV-PLAN-350C`。
