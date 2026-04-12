# DEV-PLAN-350C：Assistant OrgUnit 八动作统一收口 Phase 5 P3——`disable / enable`

**状态**: 规划中（2026-04-12 10:19 UTC）

> 本文从 `DEV-PLAN-350` Phase 5 与 `DEV-PLAN-375` 的 `375M5` 拆分而来，作为 `disable / enable` 两动作统一收口的实施 SSOT。  
> 前置假设：`350A / 350B` 已完成，统一 projection、snapshot 与写前解释骨架已稳定。

## 1. 背景与定位

1. [ ] `disable / enable` 虽然字段较少，但状态切换、有效期与 fail-closed 叙事更强，适合作为八动作统一收口的最终批次。
2. [ ] 本批同时承担“将 `350` 八动作 contract 全部冻结”的封板前置职责。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 为 `disable / enable` 冻结动作级 `PolicyContext` / `PrecheckProjection` contract。
2. [ ] 统一状态/有效期语义、错误码、snapshot 快照与写前解释。
3. [ ] 让 `350` 的八动作 contract 达到统一冻结状态，为 `370B` 启动提供前置条件。

### 2.2 非目标

1. [ ] 不承担平台退役、compat API 切断与 `runtime-status` 语义，这些仍由 `360 / 360A` 负责。
2. [ ] 不把动作知识消费或 Markdown compiler 变更并入本批。

## 3. 关键边界

1. [ ] `disable / enable` 的有效期与状态裁决仍必须服从统一 `PolicyContext -> 唯一 PDP -> Mutation Policy -> PrecheckProjection` 主链。
2. [ ] 任何状态切换都必须以统一 precheck/写前解释结果为准，不得新增动作私有 shortcut。
3. [ ] `370B` 在本批完成前不得启动。

## 4. 实施步骤

1. [ ] 为两动作补齐动作级 precheck contract、projection 快照与 digest/version 口径。
2. [ ] 将状态/有效期解释统一到 dry-run / confirm / commit / 写前解释链。
3. [ ] 完成八动作统一 contract 的总回归与 stopline 搜索。

## 5. 验收与测试

1. [ ] 执行：
   `go test ./pkg/fieldpolicy ./internal/server/... ./modules/orgunit/infrastructure/persistence/... ./modules/orgunit/services/...`
2. [ ] `disable / enable` 的 dry-run、tool explain、写前解释输出一致，且状态/有效期语义可解释。
3. [ ] `350` 八动作均已进入统一 contract，不存在仅某一动作私有的 server 层解释分支。
4. [ ] `370B` 的前置条件在文档与代码上都已满足。

## 6. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
3. `docs/dev-plans/350a-assistant-orgunit-phase5-p1-add-insert-version-convergence-plan.md`
4. `docs/dev-plans/350b-assistant-orgunit-phase5-p2-correct-rename-move-convergence-plan.md`
