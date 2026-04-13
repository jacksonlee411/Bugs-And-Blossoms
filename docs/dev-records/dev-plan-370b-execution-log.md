# DEV-PLAN-370B 执行日志：Assistant Action Knowledge Hard Cut

**状态**: 已完成并封账（2026-04-13 15:13 CST）

## 1. 本轮交付范围

1. [X] 从 `assistant_action_registry.go` 移除 `PlanTitle / PlanSummary` 说明性知识字段，只保留动作 contract、registry 与 execution wiring。
2. [X] 将 plan 标题/摘要与 semantic prompt 的动作摘要统一改为从 `assistant_knowledge_md/` direct runtime 解析，不再从代码常量散点提供。
3. [X] 在 `assistant_knowledge_runtime.go` 中新增 plan presentation 解析与 fail-closed 校验，缺 action doc / action view / intent doc 时不再回退到 registry 文案。
4. [X] 从动作 Markdown 的 `template_fields` 中移除 `contract_projection.action_spec_summary`，阻断 contract 摘要反向回流为知识模板变量。
5. [X] 将 `assistant_reply_nlg.go` 的业务型 fallback 收口为最小技术降级路径，保留动态事实拼装，不再硬编码业务说明句子。

## 2. 关键代码落点

1. [X] contract / registry / plan 接线：
   - `internal/server/assistant_action_registry.go`
   - `internal/server/assistant_api.go`
   - `internal/server/assistant_semantic_state.go`
2. [X] Markdown runtime / fail-closed / contract separation：
   - `internal/server/assistant_knowledge_runtime.go`
   - `internal/server/assistant_knowledge_md/actions/action.org.orgunit_*.zh.md`
3. [X] reply fallback 收口：
   - `internal/server/assistant_reply_nlg.go`
4. [X] 回归测试：
   - `internal/server/assistant_knowledge_runtime_more_test.go`
   - `internal/server/assistant_semantic_runtime_test.go`
   - `internal/server/assistant_reply_realizer_test.go`
   - `internal/server/assistant_reply_extra_test.go`
   - `internal/server/assistant_reply_more_test.go`
   - `internal/server/assistant_runtime_closure_test.go`
   - `internal/server/assistant_api_turn_error_mappings_test.go`

## 3. 关键实现决策

1. [X] `assistantBuildPlan(...)` 不再从 action registry 取知识摘要；正式 turn 仍经 `buildPlanContextV1(...)` 再次覆盖为 Markdown runtime 的标题/摘要。
2. [X] `assistantSemanticPromptActions()` 不再读取 registry 中文案，而是读取 runtime 的 action/intention presentation，避免 semantic prompt 成为新的知识散点。
3. [X] `buildPlanContextV1(...)` 从“create 缺包时报错、其它动作回退 spec 文案”收紧为统一 fail-closed，阻断 hard cut 后的隐性 fallback。
4. [X] reply fallback 只保留最小技术降级与动态事实文本；候选列表、缺字段、提交结果等信息改为尽量复用动态状态，不再携带知识型业务说明句子。

## 4. 验证记录

1. [X] `gofmt -w internal/server/assistant_action_registry.go internal/server/assistant_api.go internal/server/assistant_knowledge_runtime.go internal/server/assistant_reply_nlg.go internal/server/assistant_semantic_state.go internal/server/assistant_runtime_closure_test.go internal/server/assistant_api_turn_error_mappings_test.go internal/server/assistant_semantic_runtime_test.go internal/server/assistant_reply_realizer_test.go internal/server/assistant_reply_extra_test.go internal/server/assistant_reply_more_test.go internal/server/assistant_knowledge_runtime_more_test.go`
2. [X] `go test ./internal/server/...`
3. [X] `go vet ./...`
4. [X] `make check assistant-knowledge-single-source assistant-knowledge-runtime-load assistant-knowledge-no-json-runtime assistant-no-legacy-overlay assistant-no-knowledge-literals assistant-knowledge-no-archive-ref assistant-knowledge-contract-separation`

## 5. 结论与后续

1. [X] `370B` 的核心 hard cut 已完成：动作知识不再从 action registry / API / reply fallback 散点回流。
2. [X] `370` 侧当前已不再存在“后续再迁 plan summary / reply business prose / action spec summary”的遗留缓冲带。
3. [ ] `375M6` 仍需等待 `375M5` 的平台退役封板项完成后，再进入总体验收与路线图封板。
