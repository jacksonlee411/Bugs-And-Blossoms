# DEV-PLAN-370A：Assistant Markdown Knowledge Runtime Phase 1——compiler + `knowledge_qa / business_query`

**状态**: 规划中（2026-04-12 10:19 UTC）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M3` 拆分而来，作为 Markdown source、compiler、`knowledge_qa` 与 `business_query` 收口的实施 SSOT。  
> `DEV-PLAN-370` 继续持有 Runtime / Knowledge 总边界；本文只承接第一批次的范围、步骤、验收与证据。

## 1. 背景与定位

1. [ ] 当前知识资产已有结构化运行时产物，但作者体验仍偏 JSON-first / 硬编码。
2. [ ] 本批目标是先完成 Markdown source 与 compiler，并把 `knowledge_qa / business_query` 的运行时消费切到编译产物。
3. [ ] 本批明确不承接 `business_action` contract 扩张，也不改 `assistantActionSpec` 或 Tool registry 的裁决关系。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 建立 `assistant_knowledge_md/` 目录与 front matter/schema 规则。
2. [ ] 建立 Markdown -> 结构化资产编译链，并冻结生成物一致性门禁。
3. [ ] 让 `knowledge_qa / business_query` runtime 消费编译产物，而不是人工 JSON。
4. [ ] 建立 `assistant-knowledge-md`、`assistant-knowledge-generated-clean`、`assistant-no-knowledge-db` 等门禁。

### 2.2 非目标

1. [ ] 不承接 `business_action` 的正式 contract、Tool registry 或 `assistantActionSpec` 裁决。
2. [ ] 不将 `actions/*.md` 升格为动作 API / Tool API 契约主源。
3. [ ] 不新增数据库、向量库、外部知识平台依赖。

## 3. 关键边界

1. [ ] 正式 Tool API 的工具名、schema、错误语义继续以 `DEV-PLAN-350` 为准；本批只能消费，不得新增 registry 成员。
2. [ ] `360A` 冻结的 successor `/internal/assistant/*` 与 `runtime-status` 合同视为前置边界，不在本批重复定义。
3. [ ] `knowledge_qa` 与 `business_query` 可以调用已冻结只读工具，但不能借此扩张 `business_action` contract。

## 4. 实施步骤

1. [ ] 建立 Markdown source 目录、front matter 规则与 compiler schema。
2. [ ] 生成并校验 `tool_catalog.json`、`wiki_index.json`、`intent_route_catalog.json`、`reply_guidance/*.json` 等结构化资产。
3. [ ] 将 `assistant_knowledge_runtime.go`、`assistant_reply_nlg.go` 的上游输入切为编译产物。
4. [ ] 补齐 `knowledge_qa / business_query` 分流、compiler fail-closed 与 generated-clean 回归。

## 5. 验收与测试

1. [ ] 执行：
   - `make check assistant-knowledge-md`
   - `make check assistant-knowledge-generated-clean`
   - `make check assistant-no-knowledge-db`
2. [ ] `knowledge_qa / business_query` 可通过编译产物完成运行时消费，不再依赖人工 JSON 主源。
3. [ ] 生成物缺失、front matter 非法、未注册 tool、版本不一致时必须 fail-closed。
4. [ ] 本批不产生新的动作 contract、Tool registry 成员或 `assistantActionSpec` 字段扩张。

## 6. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
4. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
