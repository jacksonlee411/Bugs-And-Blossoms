# DEV-PLAN-370A：Assistant Markdown Knowledge Runtime Phase 1——单主源 Compiler 与 Hard Cut 准备

**状态**: 进行中（2026-04-13 11:08 CST）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M3` 拆分而来，作为 Markdown 单主源、compiler、全量 generated-clean 与反回流门禁的实施 SSOT。  
> `DEV-PLAN-370` 继续持有 Runtime / Knowledge 总边界；本文只承接 hard cut 之前的准备阶段，不承接最终 runtime 切换。

## 1. 背景与定位

1. [ ] 当前仓库已经存在可运行的知识资产，但主入口长期是“手工 JSON + 代码散点 + 局部约定”的混合形态，而不是 Markdown 单主源。
2. [ ] 当前运行时直接或间接消费的结构化资产至少包括：
   - `internal/server/assistant_knowledge/intent_route_catalog.json`
   - `internal/server/assistant_knowledge/interpretation/*.json`
   - `internal/server/assistant_knowledge/action_view/*.json`
   - `internal/server/assistant_knowledge/reply_guidance/*.json`
3. [ ] 历史问题不是“没有 JSON runtime”，而是“JSON runtime 没有统一 compiler 主源”，因此知识仍散在 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 等实现入口。
4. [ ] `370A` 要解决的问题不是“先把 query 迁过去，再靠 legacy overlay 挂住 action”，而是先把所有运行时知识资产统一收敛到 Markdown 单主源，并让 compiler 产物可完整复现当前知识面。
5. [ ] `375M3` 的正式职责是“single-source prep”，不是“final runtime hard cut”；真正的旧入口删除与单消费面切换留给 `370B`。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 建立 `internal/server/assistant_knowledge_md/`，作为全部 Assistant Runtime Knowledge 的唯一人工主源。
2. [ ] 建立 Markdown -> JSON 编译链，并将输出固定到 `internal/server/assistant_knowledge/`。
3. [ ] 让 compiler 一次性生成完整运行时资产集：
   - `intent_route_catalog.json`
   - `interpretation/*.json`
   - `action_view/*.json`
   - `reply_guidance/*.json`
   - `tool_catalog.json`
   - `wiki_index.json`
4. [ ] 建立 generated-clean 闭环，使 `internal/server/assistant_knowledge/` 中所有运行时产物都可稳定复现。
5. [ ] 建立反回流门禁，阻断 overlay、archive 引用、知识型代码文本与 contract/knowledge 混写继续扩散。

### 2.2 非目标

1. [ ] 不在 `370A` 做 query-only partial runtime cutover。
2. [ ] 不在 `370A` 保留 `legacy overlay`、`legacy pass-through`、`partial ownership` 作为正式方案。
3. [ ] 不把 `actions/*.md` 升格为动作 API / Tool API contract 主源。
4. [ ] 不在 `370A` 删除所有旧运行时代码入口；这些删除动作由 `370B` 承接。
5. [ ] 不引入数据库、向量库、外部知识平台依赖。

## 3. `370A` 冻结边界

1. [ ] `370A` 的 ownership 覆盖全部知识资产类别，而不是只覆盖 `knowledge_qa / business_query`。
2. [ ] `370A` 的成果是“知识主源切换完成”，不是“业务 contract 切换完成”；动作 contract、工具名、schema、错误语义继续以 `DEV-PLAN-350` 为单一事实源。
3. [ ] `370A` 允许 runtime 继续通过现有 loader 读取 `internal/server/assistant_knowledge/`，但这些资产必须全部由 compiler 生成；这不构成最终 hard cut。
4. [ ] `370A` 不允许定义“Markdown 拥有 query，JSON 拥有 action”的长期 ownership 分裂。
5. [ ] `370A` 不允许把 archive 文档作为 `source_refs` 或生成输入的一部分。
6. [ ] `assistant_action_registry.go` 中仍可保留 contract 注册与执行装配，但其中的说明性知识必须开始迁往 Markdown，并受新门禁约束。

## 4. 单主源目录与 ownership

### 4.1 正式目录

1. [ ] `internal/server/assistant_knowledge_md/intent/`
2. [ ] `internal/server/assistant_knowledge_md/actions/`
3. [ ] `internal/server/assistant_knowledge_md/replies/`
4. [ ] `internal/server/assistant_knowledge_md/tools/`
5. [ ] `internal/server/assistant_knowledge_md/wiki/`
6. [ ] `internal/server/assistant_knowledge/`

### 4.2 ownership 规则

1. [ ] `assistant_knowledge_md/` 下的 Markdown 文件是唯一允许人工编辑的知识主源。
2. [ ] `assistant_knowledge/` 下的 JSON 文件全部视为 generated artifacts，只允许由 compiler 写入。
3. [ ] `actions/` 不再是预留目录；它是正式 Markdown 主源的一部分，但其中内容属于说明性知识，不拥有动作 contract 裁决权。
4. [ ] `tools/` 中只能描述已经由 `350` 冻结的正式 Tool API，不得借机注册新 tool。
5. [ ] `wiki/` 只承载解释性知识、术语、流程、非实时说明，不承载运行时事实。

## 5. Markdown 源结构与 schema

### 5.1 文件命名与目录规则

1. [ ] `intent/`、`actions/`、`replies/`、`tools/`、`wiki/` 下统一命名为 `<id>.<locale>.md`。
2. [ ] `locale` 只允许 `zh` 或 `en`。
3. [ ] `id` 规则冻结为：
   - `intent`：`intent.*`、`route.*`
   - `action`：`action.*`
   - `reply`：`reply.*`
   - `tool`：使用正式 `tool_name`
   - `wiki`：`wiki.*`

### 5.2 通用 front matter

所有 `*.md` 必须以 YAML front matter 开头，并至少包含下表字段：

| 字段 | 必填 | 规则 |
| --- | --- | --- |
| `id` | 是 | 与文件名主键一致，且全目录唯一 |
| `title` | 是 | 人类可读标题 |
| `locale` | 是 | `zh` / `en` |
| `kind` | 是 | `intent` / `action` / `reply` / `tool` / `wiki` |
| `version` | 是 | `YYYY-MM-DD.vN` |
| `status` | 是 | `active` / `draft` / `deprecated` |
| `source_refs` | 是 | repo 内相对路径数组，必须真实存在，且不得引用 `docs/archive/**` |
| `applies_to` | 是 | 路由种类、模块或 reply kind 的数组，不能为空 |

### 5.3 类型特定字段

| `kind` | 特定字段 | 规则 |
| --- | --- | --- |
| `intent` | `route_kind`、`intent_classes`、`required_slots`、`clarification_prompts`、`keywords`、`tool_refs`、`wiki_refs` | `route_kind` 允许 `business_query / knowledge_qa / business_action / chitchat / uncertain` |
| `action` | `action_key`、`required_checks`、`proposal_template`、`reply_refs`、`tool_refs` | `action_key` 与 `required_checks` 必须能映射到 `350` 已冻结 contract |
| `reply` | `reply_kind`、`guidance_templates`、`error_codes`、`tone_constraints`、`negative_examples` | `reply_kind` 必须稳定可编译 |
| `tool` | `tool_name`、`allowed_route_kinds`、`input_schema_ref`、`output_schema_ref`、`usage_constraints`、`summary` | `tool_name` 必须已注册到正式 readonly Tool API |
| `wiki` | `topic_key`、`retrieval_terms`、`related_topics`、`summary` | `topic_key` 全局唯一 |

### 5.4 正文解析规则

1. [ ] 所有类型都允许保留 Markdown 正文，但 compiler 只提取明确声明的结构化字段与允许的正文片段。
2. [ ] `wiki/*.md` 的正文视为 `body_markdown`，写入 `wiki_index.json`。
3. [ ] `actions/*.md`、`replies/*.md`、`tools/*.md` 的正文可作为说明性文本输入，但不得绕过 front matter 反向定义 contract 字段。
4. [ ] 若后续需要扩张正文 DSL，必须回写新的子计划；`370A` 不在实施期临场发明第二套解析语法。

## 6. Compiler 输出契约

### 6.1 Markdown -> 产物映射

| 源 | 输出 | 规则 |
| --- | --- | --- |
| `intent/*.md` | `intent_route_catalog.json` | 生成全部 route 条目 |
| `intent/*.md` | `interpretation/*.json` | 生成 interpretation pack |
| `actions/*.md` | `action_view/*.json` | 生成动作说明性视图 |
| `replies/*.md` | `reply_guidance/*.json` | 生成回复指导包 |
| `tools/*.md` | `tool_catalog.json` | 生成工具说明目录 |
| `wiki/*.md` | `wiki_index.json` | 生成 wiki 检索目录 |

### 6.2 编译规则

1. [ ] 所有运行时知识产物都必须由同一个 compiler 统一生成，不允许某些文件仍靠手工维护。
2. [ ] `intent_route_catalog.json` 必须完整由 Markdown 生成，不允许再引入 action overlay 或 query overlay。
3. [ ] `action_view/*.json`、`reply_guidance/*.json`、`tool_catalog.json` 中凡涉及正式 contract 的字段，必须与 `350` 已冻结定义做一致性校验。
4. [ ] 编译失败必须 fail-closed；以下情况必须阻断：
   - front matter 缺字段
   - 重复 `id`
   - `source_refs`/`tool_refs`/`wiki_refs`/`reply_refs` 坏引用
   - `tool_name` 未注册
   - `action_key`、`required_checks` 与正式 contract 冲突
   - archive 引用
   - 产物主键冲突
5. [ ] 所有输出 JSON 必须稳定排序并使用统一格式化，确保 `generated-clean` 可重复复现。

### 6.3 审计字段

1. [ ] 继续保留并生成以下字段：
   - `source_refs`
   - `knowledge_snapshot_digest`
   - `route_catalog_version`
   - `resolver_contract_version`
   - `context_template_version`
   - `reply_guidance_version`
2. [ ] 审计字段不得依赖非确定性遍历顺序生成。

## 7. 门禁设计

### 7.1 `make check assistant-knowledge-single-source`

1. [ ] 校验 `assistant_knowledge_md/` 是唯一人工写入口。
2. [ ] 阻断手工修改 `assistant_knowledge/` 生成物。
3. [ ] 阻断第二个 Markdown 之外的知识主源目录或脚本旁路写入。

### 7.2 `make check assistant-knowledge-generated-clean`

1. [ ] 在临时目录运行 compiler，生成完整产物集。
2. [ ] 与仓库内 `internal/server/assistant_knowledge/` 逐文件比较；任何差异都必须失败。
3. [ ] 若发现有人手改生成物而未回写 Markdown，必须失败。

### 7.3 `make check assistant-no-legacy-overlay`

1. [ ] 阻断 overlay、pass-through、partial ownership、mixed-source runtime 等关键词、实现路径与旁路逻辑。
2. [ ] 阻断“Markdown 生成一部分、剩余由 legacy JSON 合并”的方案继续落地。

### 7.4 `make check assistant-no-knowledge-literals`

1. [ ] 扫描 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 等核心入口。
2. [ ] 阻断新增业务知识型摘要、解释、模板、长文案常量。
3. [ ] 技术型最小 fallback 文案允许保留，但必须与业务知识文本区分开。

### 7.5 `make check assistant-knowledge-no-archive-ref`

1. [ ] 阻断 `source_refs`、compiler 输入、生成物元数据引用 `docs/archive/**`。
2. [ ] 历史归档只允许作为调查背景，不得成为运行时知识链的一部分。

### 7.6 `make check assistant-knowledge-contract-separation`

1. [ ] 阻断 Markdown 反向定义动作 contract。
2. [ ] 阻断 contract 代码继续持有说明性知识。
3. [ ] 校验 `action_key/tool_name/required_checks` 只是在引用正式 contract，而不是覆写正式 contract。

### 7.7 `make check assistant-no-knowledge-db`

1. [ ] 阻断 knowledge/compiler 层引入数据库、向量库、外部知识平台依赖。
2. [ ] 首期 deny-list 至少覆盖：
   - `database/sql`
   - `github.com/jackc/pgx`
   - ORM/Redis/vector/RAG 相关依赖
   - 直接连接外部知识平台或 embedding 服务的代码

## 8. 实施拆解与文件落点

### 8.1 基础设施

1. [ ] 建立以下目录：
   - `internal/server/assistant_knowledge_md/intent/`
   - `internal/server/assistant_knowledge_md/actions/`
   - `internal/server/assistant_knowledge_md/replies/`
   - `internal/server/assistant_knowledge_md/tools/`
   - `internal/server/assistant_knowledge_md/wiki/`
2. [ ] 新增 compiler 与测试，建议文件落点：
   - `internal/server/assistant_knowledge_compiler.go`
   - `internal/server/assistant_knowledge_compiler_test.go`
3. [ ] 新增门禁脚本与 Makefile 接线：
   - `scripts/ci/check-assistant-knowledge-single-source.sh`
   - `scripts/ci/check-assistant-knowledge-generated-clean.sh`
   - `scripts/ci/check-assistant-no-legacy-overlay.sh`
   - `scripts/ci/check-assistant-no-knowledge-literals.sh`
   - `scripts/ci/check-assistant-knowledge-no-archive-ref.sh`
   - `scripts/ci/check-assistant-knowledge-contract-separation.sh`
   - `scripts/ci/check-assistant-no-knowledge-db.sh`

### 8.2 内容迁移

1. [ ] 将现有 query、knowledge、action、reply、tool 说明性知识迁入 Markdown。
2. [ ] 对历史散落在 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 的知识型文本建立迁移清单。
3. [ ] 保证迁移完成后，运行时资产可完整由 Markdown 复现。

### 8.3 Runtime 影响面

1. [ ] `370A` 允许继续复用现有 `assistant_knowledge_runtime.go` loader 读取生成物。
2. [ ] `370A` 不要求删除旧消费代码，但要求不得新增新的知识散点。
3. [ ] `370A` 完成后，`370B` 应只剩“切换单消费面 + 删除旧入口”工作，而不再需要回头补 compiler ownership、schema 或门禁定义。

## 9. 验收与测试

1. [ ] 文档与 gate 验收：
   - `make check assistant-knowledge-single-source`
   - `make check assistant-knowledge-generated-clean`
   - `make check assistant-no-legacy-overlay`
   - `make check assistant-no-knowledge-literals`
   - `make check assistant-knowledge-no-archive-ref`
   - `make check assistant-knowledge-contract-separation`
   - `make check assistant-no-knowledge-db`
2. [ ] Go 测试验收：
   - `go test ./internal/server/...`
3. [ ] `370A` 至少需要覆盖以下失败面：
   - front matter 缺字段
   - 重复 `id`
   - 非法 `route_kind`
   - `tool_name` 未注册
   - `action_key` / `required_checks` 与正式 contract 冲突
   - `source_refs` / `wiki_refs` / `tool_refs` / `reply_refs` 坏引用
   - archive 引用
   - generated-clean 不一致
   - 代码入口新增知识型 literals
4. [ ] `370A` 验收口径冻结为：
   - `assistant_knowledge_md/` 成为全部运行时知识的唯一人工主源
   - `assistant_knowledge/` 中全部知识资产都可由 compiler 稳定复现
   - 不再存在 query/action 分裂 ownership
   - 不再允许 overlay / pass-through 作为正式方案

## 10. 完成定义（DoD）

1. [ ] `internal/server/assistant_knowledge_md/` 成为全部 Assistant Runtime Knowledge 的唯一人工主源。
2. [ ] `internal/server/assistant_knowledge/` 中全部运行时知识产物都由 compiler 稳定生成并通过 generated-clean 校验。
3. [ ] 反回流门禁全部接入命令入口并具备 fail-closed 语义。
4. [ ] `370A` 完成后，`370B` 的剩余职责只剩 runtime hard cut、代码散点删除与旧知识入口清理。

## 11. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/370b-assistant-business-action-knowledge-runtime-consumption-plan.md`
4. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
