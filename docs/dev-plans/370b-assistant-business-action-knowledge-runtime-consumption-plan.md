# DEV-PLAN-370B：Assistant `business_action` Knowledge Runtime 消费收口计划

**状态**: 规划中（2026-04-12 10:19 UTC）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M6` 拆分而来，作为 `business_action` 的 action/reply/tool 知识消费收口实施 SSOT。  
> 启动前置：`350A / 350B / 350C` 全部完成，`370A` 已完成 compiler 与 query/runtime 主线收口。

## 1. 背景与定位

1. [ ] `370` 的长期目标包括让 `business_action` 消费 Markdown 编译产物，但该目标不能反向驱动动作 contract 扩张。
2. [ ] 本批的正式职责是：在 `350` 已冻结动作 contract 后，让 action/reply/tool 的知识消费切到编译产物，同时保持 `350` 为正式 contract 母法。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 将 `business_action` 的 action/reply/tool 知识消费接到 Markdown 编译产物。
2. [ ] 允许推进 `assistant_action_registry.go` 的“编译产物驱动消费”收口，但只消费已冻结 contract。
3. [ ] 证明 `business_action` runtime 可以消费编译产物而不生成新的动作 API / Tool API 契约。

### 2.2 非目标

1. [ ] 不新增 `ActionSchema`、Tool registry 成员、`PolicyContextContractVersion`、`PrecheckProjectionContractVersion`。
2. [ ] 不把 `actions/*.md` 升格为动作 contract 主源。
3. [ ] 不绕开 `350` 已冻结的 Gate / proposal / precheck / commit 正式语义。

## 3. 关键边界

1. [ ] 动作 contract、工具名、schema、错误语义继续以 `DEV-PLAN-350` 为单一事实源。
2. [ ] Markdown `actions/*.md` 只承载动作说明、槽位引导、reply/proposal 模板与工具消费编排，不承载正式 API / Tool API 主写语义。
3. [ ] 若编译产物与 `350` 的动作 contract 不一致，必须 fail-closed，而不是以知识文件为准覆盖正式 contract。

## 4. 实施步骤

1. [ ] 在 compiler 产物中建立 action/reply/tool 消费所需的结构化视图。
2. [ ] 调整 `assistant_action_registry.go` 与运行时消费路径，让其读取编译产物中的说明性元数据，但继续以 `350` 的 contract 字段为准。
3. [ ] 补齐 `business_action` 只消费已冻结 contract 的回归测试与 generated-clean 测试。
4. [ ] 完成 `375M6` 的总体验收准备，清零“路线图未说明但实现期临场决定”的灰区。

## 5. 验收与测试

1. [ ] 执行：
   - `make check assistant-knowledge-md`
   - `make check assistant-knowledge-generated-clean`
   - `go test ./internal/server/... ./modules/orgunit/services/...`
2. [ ] `business_action` 的 action/reply/tool 运行时已消费编译产物，但未新增任何动作 contract 主源。
3. [ ] 若编译产物与 `350` 已冻结 contract 冲突，运行时必须 fail-closed。
4. [ ] `375M6` 完成时，`350 / 360 / 360A / 370` 的剩余工作都能映射到单一子计划或独立缺陷单。

## 6. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/370a-assistant-markdown-knowledge-runtime-phase1-query-and-compiler-plan.md`
4. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. `docs/dev-plans/350a-assistant-orgunit-phase5-p1-add-insert-version-convergence-plan.md`
6. `docs/dev-plans/350b-assistant-orgunit-phase5-p2-correct-rename-move-convergence-plan.md`
7. `docs/dev-plans/350c-assistant-orgunit-phase5-p3-disable-enable-convergence-plan.md`
