# DEV-PLAN-370A：Assistant Markdown Knowledge Runtime Phase 1——compiler + `knowledge_qa / business_query`

**状态**: 进行中（2026-04-13 08:28 CST；已补齐 `375M3` Phase 1 的实施细节，待按 contract 落地）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M3` 拆分而来，作为 Markdown source、compiler、`knowledge_qa` 与 `business_query` 收口的实施 SSOT。  
> `DEV-PLAN-370` 继续持有 Runtime / Knowledge 总边界；本文只承接第一批次的范围、步骤、验收与证据。

## 1. 背景与定位

1. [ ] 当前仓库已经存在可运行的知识资产，但主入口仍是 `internal/server/assistant_knowledge/*.json` 与运行时代码散点，而不是 Markdown 主源。
2. [ ] 当前运行时直接消费的结构化资产至少包括：
   - `internal/server/assistant_knowledge/intent_route_catalog.json`
   - `internal/server/assistant_knowledge/interpretation/*.json`
   - `internal/server/assistant_knowledge/action_view/*.json`
   - `internal/server/assistant_knowledge/reply_guidance/*.json`
3. [ ] 本批要解决的问题不是“从零设计知识运行时”，而是把人工维护入口从 JSON-first 收敛为 Markdown-first，同时保持运行时继续消费稳定、可嵌入的 JSON 产物。
4. [ ] `375M3` 的正式职责是“compiler + query/runtime 基座”，不是“趁机重写 `business_action` contract”；凡涉及动作 contract 扩张、`assistantActionSpec`、Tool registry 裁决的事项，一律留给 `350`/`370B`。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 建立 `internal/server/assistant_knowledge_md/` 目录，作为 Runtime Knowledge 的唯一人工主源。
2. [ ] 建立 Markdown -> JSON 编译链，并将输出固定到 `internal/server/assistant_knowledge/`。
3. [ ] 为 `knowledge_qa / business_query` 建立可验证的结构化输入：
   - `intent_route_catalog.json`
   - `interpretation/*.json`
   - `reply_guidance/*.json`
   - `tool_catalog.json`
   - `wiki_index.json`
4. [ ] 建立 `assistant-knowledge-md`、`assistant-knowledge-generated-clean`、`assistant-no-knowledge-db` 三道门禁，并让其具备 fail-closed 语义。
5. [ ] 保持 `assistant_knowledge_runtime.go`、`assistant_reply_nlg.go` 继续消费结构化产物，而不是直接消费原始 Markdown。

### 2.2 非目标

1. [ ] 不承接 `business_action` 的正式 contract、Tool registry 或 `assistantActionSpec` 裁决。
2. [ ] 不把 `actions/*.md` 升格为动作 API / Tool API 契约主源。
3. [ ] 不新增数据库、向量库、外部知识平台依赖。
4. [ ] 不在本批引入新的前台 successor API，也不改写 `360A` 已冻结的 `runtime-status`、compat 生死表与 DTO 口径。

## 3. Phase 1 冻结边界

1. [ ] `370A` 只正式承接 `route_kind in {business_query, knowledge_qa, chitchat, uncertain}` 的 Markdown 主源与 compiler；`business_action` 的知识消费收口明确留给 `370B`。
2. [ ] `370A` 允许新增 `business_query` 运行时分流，但只能消费已经冻结的只读 Tool API；不得新增 Tool registry 成员，也不得借机定义新的动作字段。
3. [ ] `assistant_knowledge_runtime.go` 与 `assistant_reply_nlg.go` 在 `370A` 的接口职责保持不变；本批只允许替换其“上游输入来源”，不允许无契约地改 JSON 字段形状。
4. [ ] `internal/server/assistant_knowledge/action_view/*.json` 与动作相关知识包在 `370A` 内继续按“legacy pass-through”处理，不在本批变更 ownership。
5. [ ] `370A` 可以为最终 `intent_route_catalog.json` 建立“Markdown 生成 + legacy overlay”模式，但 overlay 仅允许覆盖 `business_action` 项；任何 query/non-business 条目若继续依赖人工 JSON，视为本计划未达标。
6. [ ] 编译失败、front matter 非法、重复 ID、未注册 tool、版本不一致、legacy overlay 丢失或冲突时，必须 fail-closed。

## 4. 资产 ownership 与输出边界

### 4.1 资产归属矩阵

| 运行时产物 | `370A` ownership | 说明 |
| --- | --- | --- |
| `intent_route_catalog.json` | 部分拥有 | `370A` 拥有 `business_query / knowledge_qa / chitchat / uncertain` 条目；`business_action` 条目暂走 legacy overlay，待 `370B` 接管 |
| `interpretation/knowledge.*.json` | 拥有 | 由 `intent/*.md` 生成 |
| `interpretation/query.*.json` | 拥有 | `business_query` pack 由 `intent/*.md` 生成 |
| `interpretation/route.uncertain.*.json` | 拥有 | 由 `intent/*.md` 生成 |
| `interpretation/org.*.json` | 不拥有 | 动作相关 pack 继续保留现有 JSON，待 `370B` 接手 |
| `reply_guidance/non_business_route.*.json` | 拥有 | 由 `replies/*.md` 生成 |
| `reply_guidance` 中 query/knowledge 专用包 | 拥有 | 由 `replies/*.md` 生成 |
| `reply_guidance` 中动作专用包 | 不拥有 | 继续保留现有 JSON，待 `370B` 接手 |
| `action_view/*.json` | 不拥有 | 全部留给 `370B` |
| `tool_catalog.json` | 拥有 | 本批新增产物，仅描述已冻结只读 Tool API |
| `wiki_index.json` | 拥有 | 本批新增产物，承载解释性知识与 topic 检索入口 |

### 4.2 目录 ownership

| 目录 | `370A` 角色 | 规则 |
| --- | --- | --- |
| `internal/server/assistant_knowledge_md/intent/` | 正式主源 | 允许 `knowledge_qa / business_query / chitchat / uncertain` |
| `internal/server/assistant_knowledge_md/replies/` | 正式主源 | 仅限 query/non-business 回复包 |
| `internal/server/assistant_knowledge_md/tools/` | 正式主源 | 只能描述已冻结 readonly Tool API |
| `internal/server/assistant_knowledge_md/wiki/` | 正式主源 | 解释性知识、制度流程、产品说明 |
| `internal/server/assistant_knowledge_md/actions/` | 预留目录 | 目录必须存在，但 `370A` 编译器不得消费其中 runtime 文档；动作知识 ownership 留给 `370B` |
| `internal/server/assistant_knowledge/` | 运行时产物目录 | 只允许由 compiler 生成，不允许人工编辑 |

## 5. Markdown 源结构与 schema

### 5.1 文件命名与目录规则

1. [ ] `intent/`、`replies/`、`tools/`、`wiki/` 下统一命名为 `<id>.<locale>.md`。
2. [ ] `locale` 只允许 `zh` 或 `en`。
3. [ ] `id` 规则冻结为：
   - `intent`：`knowledge.*`、`query.*`、`chat.*`、`route.*`
   - `reply`：`reply.*`
   - `tool`：使用正式 `tool_name`
   - `wiki`：`wiki.*`
4. [ ] `actions/` 在 `370A` 只允许占位文件（如 `README.md` 或 `.keep`）；任何实质性动作知识文档都应阻断并回写 `370B`。

### 5.2 通用 front matter

所有 `*.md` 必须以 YAML front matter 开头，并至少包含下表字段：

| 字段 | 必填 | 规则 |
| --- | --- | --- |
| `id` | 是 | 与文件名主键一致，且全目录唯一 |
| `title` | 是 | 人类可读标题 |
| `locale` | 是 | `zh` / `en` |
| `kind` | 是 | `intent` / `reply` / `tool` / `wiki` |
| `version` | 是 | `YYYY-MM-DD.vN` |
| `status` | 是 | `active` / `draft` / `deprecated` |
| `source_refs` | 是 | repo 内相对路径数组，必须真实存在 |
| `applies_to` | 是 | 路由种类、模块或 reply kind 的数组，不能为空 |

### 5.3 类型特定字段

| `kind` | 特定字段 | 规则 |
| --- | --- | --- |
| `intent` | `route_kind`、`intent_classes`、`required_slots`、`min_confidence`、`clarification_prompts`、`clarification_template_id`、`keywords`、`tool_refs`、`wiki_refs` | `route_kind` 只允许 `business_query / knowledge_qa / chitchat / uncertain`；`business_action` 在 `370A` 禁止出现在 Markdown ownership 中 |
| `reply` | `reply_kind`、`guidance_templates`、`error_codes`、`tone_constraints`、`negative_examples` | `reply_kind` 只允许 query/non-business 相关种类；动作专用回复由 `370B` 接手 |
| `tool` | `tool_name`、`allowed_route_kinds`、`input_schema_ref`、`output_schema_ref`、`usage_constraints`、`summary` | `tool_name` 必须已注册为 readonly Tool API；`allowed_route_kinds` 不得包含 `business_action` |
| `wiki` | `topic_key`、`retrieval_terms`、`related_topics`、`summary` | `topic_key` 全局唯一；`retrieval_terms` 至少 1 项 |

### 5.4 正文解析规则

1. [ ] `370A` 的 compiler 只强制消费 `front matter + wiki 正文`；非 wiki 文档正文允许存在，但不作为正式 contract 字段。
2. [ ] `wiki/*.md` 的正文视为 `body_markdown`，保留 Markdown 原文并写入 `wiki_index.json`。
3. [ ] 若后续需要从 `intent/reply/tool` 正文提取结构化片段，应在 `370B` 或后续子计划中显式回写，而不是在 `370A` 实现期临场扩张。
4. [ ] 该规则的目的，是让 `370A` 先用最小解析面完成 compiler 与 generated-clean 闭环，避免把“正文 DSL 设计”与“query/runtime 基座”耦合在同一 PR。

## 6. 编译器输出契约

### 6.1 Markdown -> 产物映射

| 源 | 输出 | 规则 |
| --- | --- | --- |
| `intent/*.md` | `interpretation/*.json` | 生成 `assistantInterpretationPack` 兼容字段 |
| `intent/*.md` | `intent_route_catalog.json` | 生成 query/non-business 条目；最终产物再与 legacy `business_action` overlay 合并 |
| `replies/*.md` | `reply_guidance/*.json` | 生成 `assistantReplyGuidancePack` 兼容字段 |
| `tools/*.md` | `tool_catalog.json` | 新增产物，供 `business_query` runtime 使用 |
| `wiki/*.md` | `wiki_index.json` | 新增产物，供 `knowledge_qa` runtime 使用 |

### 6.2 `intent_route_catalog.json` 合并规则

1. [ ] `370A` compiler 必须把 Markdown ownership 条目与 legacy `business_action` 条目合并为最终单文件 `intent_route_catalog.json`。
2. [ ] 合并时强制满足：
   - `intent_id` 全局唯一
   - 同一 `intent_id` 不允许同时出现在 Markdown ownership 与 legacy overlay
   - `business_action` 条目只能来自 legacy overlay
   - `business_query / knowledge_qa / chitchat / uncertain` 条目只能来自 Markdown ownership
3. [ ] 若 overlay 缺失现有动作条目，或 Markdown 试图声明动作条目，编译必须失败。

### 6.3 新增产物最小字段

`tool_catalog.json` 最小字段冻结为：

| 字段 | 说明 |
| --- | --- |
| `asset_type` | 固定为 `tool_catalog` |
| `catalog_version` | 生成版本 |
| `source_refs` | 所有参与生成的 Markdown/source ref |
| `entries[].tool_name` | readonly Tool API 名称 |
| `entries[].locale` | `zh` / `en` |
| `entries[].summary` | 人类说明 |
| `entries[].allowed_route_kinds` | 允许的路由种类 |
| `entries[].input_schema_ref` | 输入 schema 引用 |
| `entries[].output_schema_ref` | 输出 schema 引用 |
| `entries[].usage_constraints` | 何时允许/禁止调用 |

`wiki_index.json` 最小字段冻结为：

| 字段 | 说明 |
| --- | --- |
| `asset_type` | 固定为 `wiki_index` |
| `catalog_version` | 生成版本 |
| `source_refs` | 所有参与生成的 Markdown/source ref |
| `entries[].topic_key` | wiki 主题主键 |
| `entries[].locale` | `zh` / `en` |
| `entries[].title` | 标题 |
| `entries[].summary` | 摘要 |
| `entries[].retrieval_terms` | 检索词 |
| `entries[].related_topics` | 关联主题 |
| `entries[].body_markdown` | Markdown 正文 |

### 6.4 规范化与 generated-clean 规则

1. [ ] 所有输出 JSON 必须稳定排序并使用统一格式化，确保 `generated-clean` 可重复复现。
2. [ ] 稳定排序至少覆盖：
   - 文件输出顺序
   - `entries` 按主键排序
   - `source_refs` 去重后字典序排序
   - `error_codes`、`keywords`、`related_topics`、`retrieval_terms` 去重后稳定排序
3. [ ] `knowledge_snapshot_digest`、`route_catalog_version`、`reply_guidance_version` 等现有审计口径不得因“生成顺序不稳定”而漂移。

## 7. 门禁设计

### 7.1 `make check assistant-knowledge-md`

1. [ ] 校验 `internal/server/assistant_knowledge_md/` 目录结构、文件命名、front matter、主键唯一性、locale/version/status 合法性。
2. [ ] 校验 cross-reference：
   - `tool_refs` 必须能在 `tools/*.md` 中找到
   - `wiki_refs` / `related_topics` 必须能在 `wiki/*.md` 中找到
   - `source_refs` 必须指向仓库内真实文件
3. [ ] 校验 scope stopline：
   - Markdown ownership 中不得出现 `business_action`
   - `actions/` 不得出现 runtime 有效文档
   - `tool_name` 不得指向未冻结的 Tool API

### 7.2 `make check assistant-knowledge-generated-clean`

1. [ ] 在临时目录运行 compiler，生成完整产物集。
2. [ ] 与仓库内 `internal/server/assistant_knowledge/` 逐文件比较；任何差异都必须失败。
3. [ ] 若发现有人手改生成物而未回写 Markdown，必须失败。
4. [ ] 若生成脚本会覆盖 legacy overlay 但结果与当前仓库动作资产不一致，也必须失败。

### 7.3 `make check assistant-no-knowledge-db`

1. [ ] 阻断 knowledge/compiler 层引入数据库、向量库、外部知识平台依赖。
2. [ ] 首期扫描范围冻结为：
   - `internal/server/assistant_knowledge*.go`
   - `internal/server/*knowledge*compiler*.go`
   - `cmd/*assistant*`
   - `scripts/ci/check-assistant-knowledge-*.sh`
3. [ ] 首期 deny-list 至少覆盖：
   - `database/sql`
   - `github.com/jackc/pgx`
   - ORM/Redis/vector/RAG 相关依赖
   - 直接连接外部知识平台或 embedding 服务的代码
4. [ ] 该门禁只约束知识主源与 compiler 层；正常的 `/internal/assistant/*` 运行时 API 不在本 gate 的 DB 禁止面内。

## 8. 实施拆解与文件落点

### 8.1 `375M3 foundation` 最小交付

1. [ ] 建立以下文件与目录：
   - `internal/server/assistant_knowledge_md/intent/`
   - `internal/server/assistant_knowledge_md/replies/`
   - `internal/server/assistant_knowledge_md/tools/`
   - `internal/server/assistant_knowledge_md/wiki/`
   - `internal/server/assistant_knowledge_md/actions/`
2. [ ] 用 Markdown 迁入并生成首批 query/non-business 资产：
   - `knowledge.general_qa`
   - `route.uncertain`
   - `non_business_route`
   - 至少一个 `business_query` 样例
3. [ ] 新增 compiler 与测试，建议文件落点：
   - `internal/server/assistant_knowledge_compiler.go`
   - `internal/server/assistant_knowledge_compiler_test.go`
   - 如需 CLI：`go run ./cmd/server` 之外新增单独可执行入口，或在现有受控命令中暴露 compile/check 子命令
4. [ ] 新增门禁脚本与 Makefile 接线：
   - `scripts/ci/check-assistant-knowledge-md.sh`
   - `scripts/ci/check-assistant-knowledge-generated-clean.sh`
   - `scripts/ci/check-assistant-no-knowledge-db.sh`
   - `Makefile`

### 8.2 `375M3 runtime` 收口批次

1. [ ] 在 foundation 之后，再推进 `business_query` 路由与 `knowledge_qa` 的 runtime 消费收口。
2. [ ] 该批的正式职责是让运行时读取：
   - `tool_catalog.json`
   - `wiki_index.json`
   - Markdown 生成的 query/non-business `interpretation` 与 `reply_guidance`
3. [ ] 该批仍不得改写 `business_action` 的 `action_view`、Tool registry、`assistantActionSpec`、`PolicyContext` 或 `PrecheckProjection` contract。

## 9. 验收与测试

1. [ ] 文档与 gate 验收：
   - `make check assistant-knowledge-md`
   - `make check assistant-knowledge-generated-clean`
   - `make check assistant-no-knowledge-db`
2. [ ] Go 测试验收：
   - `go test ./internal/server/...`
3. [ ] `370A` 至少需要覆盖以下失败面：
   - front matter 缺字段
   - 重复 `id`
   - 非法 `route_kind`
   - query/non-business 条目误声明为 `business_action`
   - 未注册 `tool_name`
   - `source_refs` / `wiki_refs` / `tool_refs` 坏引用
   - generated-clean 不一致
   - compiler 合并 legacy overlay 时主键冲突
4. [ ] 运行时验收口径冻结为：
   - `knowledge_qa` 能从 `wiki_index.json` / `reply_guidance/*.json` 读取解释性知识
   - `business_query` 能从 `tool_catalog.json` 驱动只读 Tool API 查询
   - `business_action` 在 `370A` 期间继续走现有动作 contract，不得因 compiler 基座落地而漂移

## 10. 完成定义（DoD）

1. [ ] `internal/server/assistant_knowledge_md/` 成为 query/non-business 知识的唯一人工主源。
2. [ ] `internal/server/assistant_knowledge/` 中由 `370A` ownership 管辖的产物全部可由 compiler 稳定复现。
3. [ ] 三道门禁都已接入仓库命令入口，并能在错误场景下 fail-closed。
4. [ ] `370A` 完成后，`370B` 的剩余职责只剩动作知识消费收口，不再需要回头定义 Markdown schema、compiler ownership 或 gate 口径。

## 11. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/370b-assistant-business-action-knowledge-runtime-consumption-plan.md`
4. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
