# DEV-PLAN-350B：Assistant OrgUnit 八动作统一收口 Phase 5 P2——`correct / rename / move`

**状态**: 已完成并封账（2026-04-13 06:45 CST）

> 本文从 `DEV-PLAN-350` Phase 5 与 `DEV-PLAN-375` 的 `375M4` 拆分而来，作为 `correct / rename / move` 三动作统一收口的实施 SSOT。  
> 前置假设：`350A` 已完成并验证 create/add/insert 的统一 contract 骨架可复用。

## 1. 背景与定位

1. [X] `correct / rename / move` 同时引入“目标版本选择”“新父组织候选确认”“maintainable patch”三类中等复杂度场景。
2. [X] 本批的目标不是扩张新工具面，而是证明统一 projection 可以覆盖这些非 create-only 的动作语义。

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 为 `correct / rename / move` 冻结动作级 `PolicyContext` / `PrecheckProjection` contract。
2. [X] 将三动作的版本选择、候选确认、字段 maintainable 判定与写前解释纳入统一投影。
3. [X] 让 Assistant dry-run / confirm / commit 与写服务前置解释继续共享同一裁决核心。

### 2.2 非目标

1. [X] 不覆盖 `disable / enable`。
2. [X] 不在本批引入 `business_action` Markdown 主源。
3. [X] 不新增第二套候选确认或 patch 校验语义。

## 3. 关键边界

1. [X] `350A` 已冻结的 digest/version/snapshot 口径必须复用，不重新发明第二套动作快照模型。
2. [X] `move` 涉及的新父组织候选确认仍走既有只读工具与 gate 机制，不新增新工具名。
3. [X] `correct / rename / move` 的正式 contract 扩张仍由 `DEV-PLAN-350` 裁决；本文只承接本批实施与验收。

## 4. 实施步骤

1. [X] 为三动作补齐动作级 precheck contract 与 projection 生成。
2. [X] 将版本选择、maintainable patch、候选确认接入统一 projection 与 fail-closed 快照。
3. [X] 统一写服务前置解释、Assistant dry-run 与工具 explain 的消费链。
4. [X] 补齐动作扩展成本测试，证明新增动作无需改多处主链分支。

## 5. 验收与测试

1. [X] 执行：
   `go test ./modules/orgunit/services/...`
   `go test ./internal/server/...`
   `go vet ./...`
   `make check lint`
   `make test`（coverage `98.00% >= 98.00%`）
   `make check doc`
2. [X] `correct / rename / move` 在同一输入下，Assistant dry-run、tool explain、写前解释输出一致。
3. [X] `move` 的新父组织候选确认必须可回放、可拒绝、不可猜测。
4. [X] 动作新增成本保持在 `ActionSchema + Projection mapping + CommitAdapter` 范围内。

## 6. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
3. `docs/dev-plans/350a-assistant-orgunit-phase5-p1-add-insert-version-convergence-plan.md`
