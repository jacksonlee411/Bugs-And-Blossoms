# DEV-PLAN-370B：Assistant Action Knowledge Hard Cut 与代码散点清理计划

**状态**: 已完成并封账（2026-04-13 15:13 CST；`assistant_action_registry.go` 已移除 `PlanTitle/PlanSummary` 知识字段，plan/semantic prompt 已切到 Markdown runtime，reply fallback 已收口为最小技术降级，缺 action/intention doc 时改为 fail-closed）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M6` 拆分而来，作为 `business_action` 剩余知识散点清理、contract / knowledge 强分离的实施 SSOT。  
> 启动前置：`350A / 350B / 350C` 全部完成，`370A` 已完成 Direct Markdown Runtime 基座与 `assistant_knowledge/*.json` 切断。

## 1. 背景与定位

1. [X] `370A` 解决的是“Markdown 单主源 + direct runtime + JSON cutoff”，不是“动作知识散点全部清理完成”。
2. [X] `370A` 完成后，最大的剩余风险将不再是 JSON 中间层，而是动作链上仍残留在代码里的知识字段与业务文本。
3. [X] `370B` 的正式职责是：在 `350` 已冻结动作 contract 后，完成 `business_action` 的 contract / knowledge 强分离，并清理 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 中的剩余知识散点。

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 让 `business_action` 的说明性知识完全来自 `assistant_knowledge_md/` 的 direct runtime 索引。
2. [X] 让 `assistant_action_registry.go` 只保留 contract / registry / execution wiring，不再持有知识文本。
3. [X] 清理 `assistant_api.go`、`assistant_reply_nlg.go` 中残留的业务知识型摘要、解释、模板与 fallback 文案。
4. [X] 证明 `business_action` 不再依赖代码散点知识，而动作 contract 仍只由 `350` 裁决。

### 2.2 非目标

1. [ ] 不新增 `ActionSchema`、Tool registry 成员、`PolicyContextContractVersion`、`PrecheckProjectionContractVersion`。
2. [ ] 不把 `actions/*.md` 升格为动作 contract 主源。
3. [ ] 不恢复 `assistant_knowledge/*.json` 或任何导出快照目录。
4. [ ] 不保留“切不动就回退到代码散点文本”的 fallback 通道。

## 3. Hard Cut 冻结边界

1. [X] `assistant_knowledge_md/` 是唯一人工主源，也是运行时唯一知识输入面。
2. [X] runtime 在 `370B` 期间不得继续读取或依赖：
   - `assistant_action_registry.go` 中的说明性文本
   - `assistant_api.go` 中的业务知识型解释/提示
   - `assistant_reply_nlg.go` 中的业务知识型 fallback 文案
   - `assistant_knowledge/*.json`
   - overlay / pass-through / archive 引用
3. [X] `assistant_action_registry.go` 中凡属于 contract 的字段继续以 `350` 为准；凡属于说明性知识的字段必须迁入 Markdown。
4. [X] 若 Markdown 内容与 `350` 已冻结 contract 冲突，runtime 必须 fail-closed，不得以本地兜底文案继续执行。

## 4. 目标状态

### 4.1 代码职责收敛

1. [X] `assistant_knowledge_runtime.go`
成为 direct Markdown loader/indexer 的单一代码入口。
2. [X] `assistant_action_registry.go`
只保留动作注册、contract 绑定、执行装配与引用关系，不再承载 plan 摘要、说明性文本、回复模板。
3. [X] `assistant_api.go`
只保留 API 协议装配、DTO、错误码映射与最小技术 fallback。
4. [X] `assistant_reply_nlg.go`
只保留回复组装逻辑、语法拼接与最小技术降级路径，不再内建业务知识文本。

### 4.2 运行时消费规则

1. [X] `business_query`、`knowledge_qa`、`business_action` 都统一从 direct Markdown runtime 索引读取知识视图。
2. [X] 实时业务事实继续只从 API / Tool API 获取。
3. [X] 任何回复、说明、提问模板、proposal 指导都必须来自 Markdown 索引或正式错误契约，不得再来自代码散点常量。

## 5. 实施步骤

### 5.1 清理动作知识散点

1. [X] 从 `assistant_action_registry.go` 移除说明性知识字段与文本常量。
2. [X] 从 `assistant_api.go` 移除计划摘要、验证解释、业务提示等知识型文本。
3. [X] 从 `assistant_reply_nlg.go` 移除业务知识型 fallback 文案，并把对应内容迁入 Markdown。

### 5.2 contract / knowledge 强分离

1. [X] 为动作知识建立“引用正式 contract，不覆写正式 contract”的代码与测试约束。
2. [X] 对 `action_key`、`required_checks`、`tool_name`、schema 引用做一致性校验。
3. [X] 确保 Markdown 与 direct runtime loader 只能表达说明性知识、路由知识、reply guidance，不拥有策略裁决权。

### 5.3 稳定化与观测

1. [ ] 如需暴露知识 digest、markdown version、route catalog version 到 `runtime-status`，必须回写 `DEV-PLAN-360A`。
2. [X] 清理 hard cut 后遗留死分支、无效 fallback 与重复代码路径。
3. [X] 将剩余问题收敛为独立缺陷单，而不是继续保留迁移缓冲带。

## 6. 门禁与测试

### 6.1 必跑门禁

1. [X] `make check assistant-knowledge-single-source`
2. [X] `make check assistant-knowledge-runtime-load`
3. [X] `make check assistant-knowledge-no-json-runtime`
4. [X] `make check assistant-no-legacy-overlay`
5. [X] `make check assistant-no-knowledge-literals`
6. [X] `make check assistant-knowledge-no-archive-ref`
7. [X] `make check assistant-knowledge-contract-separation`
8. [ ] `make check assistant-api-only`

### 6.2 测试重点

1. [ ] 单元测试：
   - runtime 只从 Markdown 索引加载知识
   - `assistant_action_registry.go` 不再暴露知识型字段
   - `assistant_api.go` / `assistant_reply_nlg.go` 仅保留最小技术 fallback
2. [ ] 集成测试：
   - `business_query` 读取 Markdown 索引并通过 Tool API 取事实
   - `knowledge_qa` 读取 wiki/reply 索引
   - `business_action` 读取 action/reply/tool 索引并遵循 `350` contract
3. [ ] 回归测试：
   - 缺 Markdown 知识文件时 fail-closed
   - contract 冲突时 fail-closed
   - 不存在 JSON / overlay 回退路径

## 7. 验收标准

1. [X] Runtime 只消费 direct Markdown runtime 索引，不再混读代码散点文本、`assistant_knowledge/*.json` 或 overlay。
2. [X] `assistant_action_registry.go` 已完成 contract / knowledge 拆离。
3. [X] `assistant_api.go`、`assistant_reply_nlg.go` 不再持有业务知识型文本。
4. [X] `business_action` 的知识消费已切到 Markdown 索引，但动作 contract 仍只由 `350` 裁决。
5. [X] 缺知识、知识损坏、contract 冲突、版本不一致时均 fail-closed。
6. [X] `370B` 完成后，`370` 只剩稳定化或独立缺陷修复，不再存在“以后再迁”的旧知识入口。

## 8. 完成定义（DoD）

1. [X] `business_action` 知识散点清理完成。
2. [X] 旧知识入口与 bridge code 全部删除。
3. [X] contract / knowledge 分离门禁已在 CI 生效。
4. [ ] `375M6` 出口条件满足，可进入总体验收与封板准备。

## 9. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/370a-assistant-markdown-knowledge-runtime-phase1-query-and-compiler-plan.md`
4. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. `docs/dev-plans/350a-assistant-orgunit-phase5-p1-add-insert-version-convergence-plan.md`
6. `docs/dev-plans/350b-assistant-orgunit-phase5-p2-correct-rename-move-convergence-plan.md`
7. `docs/dev-plans/350c-assistant-orgunit-phase5-p3-disable-enable-convergence-plan.md`
