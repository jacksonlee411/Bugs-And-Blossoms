# DEV-PLAN-244 执行日志：Assistant Interpretation Pack 与 Intent Route Catalog 编译治理

> 对应计划：`docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`

## 1. 执行时间

- 执行日期：2026-03-11（CST）
- 执行范围：`internal/server` 理解层知识编译器、知识资产 JSON、测试矩阵、文档地图与 244 状态回写

## 2. 本次落地内容

1. 编译器分层落地（保持单入口 `assistantCompileKnowledgeRuntime(...)`）。
- 新增 `assistantCompileInterpretationAssets(...)`：解释包结构/语义校验。
- 新增 `assistantCompileIntentRouteCatalog(...)`：route catalog 结构/交叉引用/slot/min_confidence 校验。
- 保留 `assistantLoadInterpretationPacks(...)`、`assistantLoadIntentRouteCatalog(...)` 的读取职责，不改写 `242/243` runtime 职责边界。

2. Interpretation Pack 契约加固。
- 增加 `intent_classes` 必填与合法值校验（仅允许 `business_action/knowledge_qa/chitchat/uncertain`）。
- 增加 `clarification_prompts[].template_id` 非空与 pack+locale 内唯一校验。
- 增加 `negative_examples[]` 去重与空值阻断。
- 增加 `knowledge_version` 必填校验。

3. Intent Route Catalog 契约加固。
- 增加 `min_confidence` 区间校验（`[0,1]`）。
- 增加 `required_slots[]` 去重、空值阻断、业务动作字段合法性校验。
- 增加 `clarification_template_id` 与 interpretation 模板交叉引用 fail-closed。
- 增加非动作意图必须可解析到 interpretation pack 的 fail-closed 校验。
- 增加 `intent_classes` 与 `route_kind` 不相交时阻断。

4. 运行时索引与回退收敛。
- `assistantKnowledgeRuntime` 新增：
  - `routeByIntent`
  - `interpretationTemplateID`
  - `routePackID`
- `routeIntent(...)` 未命中关键词时优先消费 catalog 内 `uncertain` 路由；仅在缺失时回退固定 `route.uncertain`。
- `buildPlanContextV1(...)` 非动作路径改为优先使用已编译 `routePackID` 解析 interpretation pack。

5. 样例资产扩面（动作/非动作/待澄清三类）。
- 新增 `route.uncertain` route entry。
- 新增 `internal/server/assistant_knowledge/interpretation/route.uncertain.zh.json`。
- 新增 `internal/server/assistant_knowledge/interpretation/route.uncertain.en.json`。

6. 测试矩阵补齐（按 244 清单命名）。
- 新增 `internal/server/assistant_knowledge_runtime_244_test.go`，覆盖：
  - 编译成功路径：模板交叉引用、catalog cross-ref、digest 稳定、locale fallback、version 透传。
  - 编译失败路径：重复模板 ID、非法 intent class、未知模板、非法 slot、非法 confidence、非动作缺失解释包、禁止键、source refs 非法。
- 更新 `assistant_knowledge_runtime_test.go` 与 `assistant_knowledge_runtime_more_test.go` 基础样例，适配新增契约校验。

## 3. 迁移清单与 stopline（PR-244-01 / PR-244-04）

| 现有位置 | 处理结果 | 计划归属 |
| --- | --- | --- |
| `internal/server/assistant_knowledge_runtime.go` 非动作 route/模板散点校验 | 已迁入编译层，形成 `assistantCompileInterpretationAssets` 与 `assistantCompileIntentRouteCatalog` fail-closed | 244 |
| `internal/server/assistant_knowledge_runtime.go` 非动作 fallback 文案（兜底） | 保留最小兜底，仅在资产缺失的异常路径触发；主路径已优先消费资产 | 244（最小保留） |
| `internal/server/assistant_api.go` `assistant_intent_unsupported` 错误出口 | 保留 API 错误码映射，不新增理解层 prompt 文案 | 不动（API 契约） |
| `internal/server/assistant_reply_nlg.go` 回复文案统一 | 未在本计划改写，按边界冻结后置到 `245` | 245 |
| `internal/server/assistant_intent_pipeline.go` 本地升级逻辑 | 未新增散点规则；保持 242 主链行为 | 不动（242） |

## 4. 验证结果

1. Go 与 Assistant 回归。
- `gofmt -w internal/server/assistant_knowledge_runtime.go internal/server/assistant_knowledge_runtime_test.go internal/server/assistant_knowledge_runtime_more_test.go internal/server/assistant_knowledge_runtime_244_test.go`：通过。
- `go test ./internal/server -run 'TestAssistantCompileKnowledgeRuntime_|TestAssistantKnowledgeRuntime_'`：通过。
- `go test ./internal/server`：通过。
- `go vet ./...`：通过。

2. CI 门禁相关命令。
- `make check lint`：通过。
- `make check doc`：通过。
- `make check error-message`：通过。
- `make test`：失败（全仓覆盖率门禁，`total 99.90% < threshold 100.00%`，非本计划新增失败）。
- `make check no-legacy`：失败（仓内既有 `legacy` 标识触发，主要位于 `assistant_intent_router.go`/相关测试，非本计划新增）。

## 5. 结论

- DEV-PLAN-244 在“理解层资产可编译、可审计、可迁移、可阻断漂移”范围内已完成。
- `242/243` 的 runtime 主链未被改写，边界保持：`244` 只增强理解资产与编译治理。
- 回复表达主链统一（reply phrasing/NLG）仍以后续 `245` 为唯一承接计划。
