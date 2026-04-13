# DEV-PLAN-370B：Assistant Runtime Knowledge 一次性 Hard Cut 计划

**状态**: 规划中（2026-04-13 11:15 CST）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M6` 拆分而来，作为 runtime 单消费面切换、旧知识入口删除、contract / knowledge 强分离的实施 SSOT。  
> 启动前置：`350A / 350B / 350C` 全部完成，`370A` 已完成单主源、compiler、generated-clean 与反回流门禁准备。

## 1. 背景与定位

1. [ ] `370A` 解决的是“知识主源切换”，不是“运行时旧入口删除”；因此 hard cut 仍需单独成批实施。
2. [ ] 当前真正的风险不在于 Markdown 是否存在，而在于 runtime 仍可能混读代码散点文本、手工 JSON、历史兜底逻辑与 compiler 产物。
3. [ ] `370B` 的正式职责是：在 `350` 已冻结动作 contract 后，让 runtime 只消费 compiler 产物，并删除所有旧知识入口。
4. [ ] `370B` 完成后，历史分布式局部 SoT 必须被彻底切断，防止知识回流到 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 等实现入口。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 让 runtime 只消费 compiler 生成的 `internal/server/assistant_knowledge/` 产物。
2. [ ] 删除 overlay、pass-through、mixed-source runtime 与人工 JSON 主源假设。
3. [ ] 让 `assistant_action_registry.go` 只保留 contract / registry / execution wiring，不再持有知识文本。
4. [ ] 清理 `assistant_api.go`、`assistant_reply_nlg.go` 中残留的业务知识型摘要、解释、模板与 fallback 文案。
5. [ ] 证明 `business_action`、`business_query`、`knowledge_qa` 三条主链都已切到统一知识消费面，而没有生成新的 contract 主源。

### 2.2 非目标

1. [ ] 不新增 `ActionSchema`、Tool registry 成员、`PolicyContextContractVersion`、`PrecheckProjectionContractVersion`。
2. [ ] 不把 `actions/*.md` 升格为动作 contract 主源。
3. [ ] 不保留“切不动就回退到代码散点文本”的 fallback 通道。
4. [ ] 不把 archive 文档或历史计划重新引入 runtime 知识链。

## 3. Hard Cut 冻结边界

1. [ ] `assistant_knowledge_md/` 是唯一人工主源；`assistant_knowledge/` 是唯一运行时知识输入面。
2. [ ] runtime 在 hard cut 后不得继续读取或依赖：
   - `assistant_action_registry.go` 中的说明性文本
   - `assistant_api.go` 中的业务知识型解释/提示
   - `assistant_reply_nlg.go` 中的业务知识型 fallback 文案
   - 手工维护的 JSON 主源
   - overlay / pass-through / archive 引用
3. [ ] `assistant_action_registry.go` 中凡属于 contract 的字段继续以 `350` 为准；凡属于说明性知识的字段必须迁入 Markdown 并由 compiler 生成。
4. [ ] 若 compiler 产物与 `350` 已冻结 contract 冲突，runtime 必须 fail-closed，不得以本地兜底文案继续执行。

## 4. 目标状态

### 4.1 代码职责收敛

1. [ ] `assistant_knowledge_runtime.go`
成为知识加载与查询的单一代码入口，只读取 compiler 产物。
2. [ ] `assistant_action_registry.go`
只保留动作注册、contract 绑定、执行装配与引用关系，不再承载 plan 摘要、说明性文本、回复模板。
3. [ ] `assistant_api.go`
只保留 API 协议装配、DTO、错误码映射与最小技术 fallback。
4. [ ] `assistant_reply_nlg.go`
只保留回复组装逻辑、语法拼接与最小技术降级路径，不再内建业务知识文本。

### 4.2 运行时消费规则

1. [ ] `business_query`、`knowledge_qa`、`business_action` 都统一从 compiler 产物读取知识视图。
2. [ ] 实时业务事实继续只从 API / Tool API 获取。
3. [ ] 任何回复、说明、提问模板、proposal 指导都必须来自 compiler 产物或正式错误契约，不得再来自代码散点常量。

## 5. 实施步骤

### 5.1 切换知识消费面

1. [ ] 统一 runtime 读取路径到 `assistant_knowledge_runtime.go`。
2. [ ] 删除 mixed-source 读取逻辑，确保不存在“先看 compiler 产物，缺了再回退代码/旧 JSON”的行为。
3. [ ] 将 `business_action` 的 action/reply/tool 知识消费切到 compiler 产物。

### 5.2 删除旧知识入口

1. [ ] 从 `assistant_action_registry.go` 移除说明性知识字段与文本常量。
2. [ ] 从 `assistant_api.go` 移除计划摘要、验证解释、业务提示等知识型文本。
3. [ ] 从 `assistant_reply_nlg.go` 移除业务知识型 fallback 文案，并把对应内容迁入 Markdown。
4. [ ] 删除与 overlay/pass-through 相关的临时桥接代码、生成脚本或装配逻辑。

### 5.3 contract / knowledge 强分离

1. [ ] 为动作知识建立“引用正式 contract，不覆写正式 contract”的代码与测试约束。
2. [ ] 对 `action_key`、`required_checks`、`tool_name`、schema 引用做一致性校验。
3. [ ] 确保 Markdown 与 compiler 只能表达说明性知识、路由知识、reply guidance，不拥有策略裁决权。

### 5.4 稳定化与观测

1. [ ] 如需暴露知识 digest、compiler version、route catalog version 到 `runtime-status`，必须回写 `DEV-PLAN-360A`。
2. [ ] 清理 hard cut 后遗留死分支、无效 fallback 与重复代码路径。
3. [ ] 将剩余问题收敛为独立缺陷单，而不是继续保留迁移缓冲带。

## 6. 门禁与测试

### 6.1 必跑门禁

1. [ ] `make check assistant-knowledge-single-source`
2. [ ] `make check assistant-knowledge-generated-clean`
3. [ ] `make check assistant-no-legacy-overlay`
4. [ ] `make check assistant-no-knowledge-literals`
5. [ ] `make check assistant-knowledge-no-archive-ref`
6. [ ] `make check assistant-knowledge-contract-separation`
7. [ ] `make check assistant-api-only`

### 6.2 测试重点

1. [ ] 单元测试：
   - runtime 只从 compiler 产物加载知识
   - `assistant_action_registry.go` 不再暴露知识型字段
   - `assistant_api.go` / `assistant_reply_nlg.go` 仅保留最小技术 fallback
2. [ ] 集成测试：
   - `business_query` 读取 compiler 产物并通过 Tool API 取事实
   - `knowledge_qa` 读取 wiki/reply guidance 产物
   - `business_action` 读取 action/reply/tool 产物并遵循 `350` contract
3. [ ] 回归测试：
   - 缺知识产物时 fail-closed
   - contract 冲突时 fail-closed
   - 不存在 overlay 回退路径

## 7. 验收标准

1. [ ] Runtime 只消费 compiler 产物，不再混读代码散点文本、手工 JSON 或 overlay。
2. [ ] `assistant_action_registry.go` 已完成 contract / knowledge 拆离。
3. [ ] `assistant_api.go`、`assistant_reply_nlg.go` 不再持有业务知识型文本。
4. [ ] `business_action` 的知识消费已切到 compiler 产物，但动作 contract 仍只由 `350` 裁决。
5. [ ] 缺产物、产物损坏、contract 冲突、版本不一致时均 fail-closed。
6. [ ] `370B` 完成后，`370` 只剩稳定化或独立缺陷修复，不再存在“以后再迁”的旧知识入口。

## 8. 完成定义（DoD）

1. [ ] Runtime 单消费面切换完成。
2. [ ] 旧知识入口与 bridge code 全部删除。
3. [ ] contract / knowledge 分离门禁已在 CI 生效。
4. [ ] `375M6` 出口条件满足，可进入总体验收与封板准备。

## 9. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/370a-assistant-markdown-knowledge-runtime-phase1-query-and-compiler-plan.md`
4. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. `docs/dev-plans/350a-assistant-orgunit-phase5-p1-add-insert-version-convergence-plan.md`
6. `docs/dev-plans/350b-assistant-orgunit-phase5-p2-correct-rename-move-convergence-plan.md`
7. `docs/dev-plans/350c-assistant-orgunit-phase5-p3-disable-enable-convergence-plan.md`
