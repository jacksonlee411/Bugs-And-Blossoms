# DEV-PLAN-370A：Assistant Markdown Knowledge Runtime Phase 1——Direct Runtime 基座与 JSON 切断

**状态**: 进行中（2026-04-13 12:40 CST；Direct Markdown Runtime、JSON cutoff、semantic parity 与反回流门禁已落地，剩余为 `370B` 散点知识继续硬切）

> 本文从 `DEV-PLAN-370` 与 `DEV-PLAN-375` 的 `375M3` 拆分而来，作为 Direct Markdown Runtime 基座、Markdown 单主源、运行时校验/索引与 `assistant_knowledge/*.json` 切断的实施 SSOT。  
> `DEV-PLAN-370` 继续持有 Runtime / Knowledge 总边界；本文只承接 direct runtime 基座与 JSON cutoff，不承接 `350` 的动作 contract 裁决。

## 1. 背景与定位

1. [ ] 当前仓库已经存在可运行的知识资产，但主入口长期是“手工 JSON + 代码散点 + 局部约定”的混合形态，而不是 Markdown 单主源。
2. [ ] 历史问题不是“没有运行时资产”，而是“多了一层长期存在的 JSON 中间层，却没有消除散点知识源”。
3. [ ] `370A` 要解决的问题不是“再做一个 Markdown compiler”，而是让 runtime 直接读取 Markdown，并把 `assistant_knowledge/*.json` 从仓库与运行时中切掉。
4. [ ] `375M3` 的正式职责是“Direct Markdown Runtime foundation + JSON cutoff”，不是 query-only partial cutover，也不是导出快照建设。

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 建立 `internal/server/assistant_knowledge_md/`，作为全部 Assistant Runtime Knowledge 的唯一人工主源。
2. [X] 建立 Direct Markdown Runtime loader：运行时直接读取 Markdown、校验 front matter 与引用，并建立内存索引。
3. [X] 删除 `internal/server/assistant_knowledge/*.json` 及其 embed/load 依赖。
4. [X] 建立反回流门禁，阻断 JSON 快照、archive 引用、知识型代码文本与 contract/knowledge 混写继续扩散。
5. [X] 为 `370B` 留下最小剩余工作：只处理 `business_action` 剩余知识散点与 contract / knowledge 分离。

### 2.2 非目标

1. [ ] 不在 `370A` 做 query-only partial runtime cutover。
2. [ ] 不在 `370A` 保留 `assistant_knowledge/*.json` 作为桥接层、快照层或导出层。
3. [ ] 不在 `370A` 引入 Markdown -> JSON compiler、导出脚本或 generated-clean 闭环。
4. [ ] 不把 `actions/*.md` 升格为动作 API / Tool API contract 主源。
5. [ ] 不引入数据库、向量库、外部知识平台依赖。

## 3. `370A` 冻结边界

1. [ ] `370A` 的 ownership 覆盖全部知识资产类别，而不是只覆盖 `knowledge_qa / business_query`。
2. [ ] `370A` 的成果是“知识主源切换完成 + JSON 中间层删除”，不是“业务 contract 切换完成”；动作 contract、工具名、schema、错误语义继续以 `DEV-PLAN-350` 为单一事实源。
3. [ ] `370A` 不允许定义“Markdown 拥有 query，代码/JSON 拥有 action”的长期 ownership 分裂。
4. [ ] `370A` 不允许把 archive 文档作为 `source_refs` 或运行时输入的一部分。
5. [ ] `assistant_action_registry.go` 中仍可保留 contract 注册与执行装配，但其中说明性知识应开始迁往 Markdown，并受新门禁约束。

## 4. 单主源目录与 ownership

### 4.1 正式目录

1. [X] `internal/server/assistant_knowledge_md/intent/`
2. [X] `internal/server/assistant_knowledge_md/actions/`
3. [X] `internal/server/assistant_knowledge_md/replies/`
4. [X] `internal/server/assistant_knowledge_md/tools/`
5. [X] `internal/server/assistant_knowledge_md/wiki/`

### 4.2 ownership 规则

1. [X] `assistant_knowledge_md/` 下的 Markdown 文件是唯一允许人工编辑的知识主源。
2. [X] `assistant_knowledge/*.json` 必须被删除，不再保留任何 checked-in 生成物目录。
3. [X] `actions/` 是正式 Markdown 主源的一部分，但其中内容属于说明性知识，不拥有动作 contract 裁决权。
4. [X] `tools/` 中只能描述已经由 `350` 冻结的正式 Tool API，不得借机注册新 tool。
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
| `reply` | `reply_kind`、`guidance_templates`、`error_codes`、`tone_constraints`、`negative_examples` | `reply_kind` 必须稳定可索引 |
| `tool` | `tool_name`、`allowed_route_kinds`、`input_schema_ref`、`output_schema_ref`、`usage_constraints`、`summary` | `tool_name` 必须已注册到正式 readonly Tool API |
| `wiki` | `topic_key`、`retrieval_terms`、`related_topics`、`summary` | `topic_key` 全局唯一 |

### 5.4 正文解析规则

1. [ ] 所有类型都允许保留 Markdown 正文，但 runtime loader 只提取明确声明的结构化字段与允许的正文片段。
2. [ ] `wiki/*.md` 的正文作为 `body_markdown` 进入运行时 wiki 索引。
3. [ ] `actions/*.md`、`replies/*.md`、`tools/*.md` 的正文可作为说明性文本输入，但不得绕过 front matter 反向定义 contract 字段。
4. [ ] 若后续需要扩张正文 DSL，必须回写新的子计划；`370A` 不在实施期临场发明第二套解析语法。

## 6. Direct Runtime Loader 契约

### 6.1 Markdown -> 内存索引映射

| 源 | 运行时索引 | 规则 |
| --- | --- | --- |
| `intent/*.md` | `intent route index` | 建立全部 route 条目 |
| `intent/*.md` | `interpretation index` | 建立意图说明与澄清模板索引 |
| `actions/*.md` | `action knowledge index` | 建立动作说明性视图 |
| `replies/*.md` | `reply guidance index` | 建立回复指导索引 |
| `tools/*.md` | `tool docs index` | 建立工具说明索引 |
| `wiki/*.md` | `wiki retrieval index` | 建立 wiki 检索索引 |

### 6.2 加载规则

1. [ ] 所有运行时知识都必须直接从同一组 Markdown 文件加载，不允许某些类别继续依赖 JSON 快照。
2. [ ] runtime loader 负责解析、校验与建索引，但不产生 checked-in 产物。
3. [ ] 凡涉及正式 contract 的字段，必须与 `350` 已冻结定义做一致性校验。
4. [ ] 加载失败必须 fail-closed；以下情况必须阻断：
   - front matter 缺字段
   - 重复 `id`
   - `tool_name` 未注册
   - `action_key`、`required_checks` 与正式 contract 冲突
   - archive 引用
   - 索引主键冲突
5. [ ] 任何形式的 JSON 快照导出都不属于本计划。

### 6.3 审计字段

1. [ ] 继续保留并生成运行时级审计信息：
   - `source_refs`
   - `knowledge_snapshot_digest`
   - `resolver_contract_version`
   - `context_template_version`
   - `markdown_version`
2. [ ] 审计字段只作为 runtime 观测值存在，不要求写回 checked-in 快照文件。

## 7. 门禁设计

### 7.1 `make check assistant-knowledge-single-source`

1. [ ] 校验 `assistant_knowledge_md/` 是唯一人工写入口。
2. [ ] 阻断第二个 Markdown 之外的知识主源目录或脚本旁路写入。

### 7.2 `make check assistant-knowledge-runtime-load`

1. [ ] 对全部 Markdown 执行运行时级解析、引用校验与索引构建。
2. [ ] 任何加载错误都必须失败。

### 7.3 `make check assistant-knowledge-no-json-runtime`

1. [ ] 阻断 `assistant_knowledge/*.json` 回流。
2. [ ] 阻断 `go:embed assistant_knowledge/*.json`、JSON loader、快照目录或相关测试夹带回流。

### 7.4 `make check assistant-no-legacy-overlay`

1. [ ] 阻断 overlay、pass-through、partial ownership、mixed-source runtime 等关键词、实现路径与旁路逻辑。
2. [ ] 阻断“Markdown 读一部分、剩余由旧 JSON 合并”的方案继续落地。

### 7.5 `make check assistant-no-knowledge-literals`

1. [ ] 扫描 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 等核心入口。
2. [ ] 阻断新增业务知识型摘要、解释、模板、长文案常量；同时覆盖 `assistant_model_gateway.go` 中独立维护的 route/action 映射与语义提示词真相源。
3. [ ] 技术型最小 fallback 文案允许保留，但必须与业务知识文本区分开。

### 7.6 `make check assistant-knowledge-no-archive-ref`

1. [ ] 阻断 `source_refs` 与运行时输入引用 `docs/archive/**`。
2. [ ] 历史归档只允许作为调查背景，不得成为运行时知识链的一部分。

### 7.7 `make check assistant-knowledge-contract-separation`

1. [ ] 阻断 Markdown 反向定义动作 contract。
2. [ ] 阻断 contract 代码继续持有说明性知识。
3. [ ] 校验 `action_key/tool_name/required_checks` 只是在引用正式 contract，而不是覆写正式 contract。

### 7.8 `make check assistant-no-knowledge-db`

1. [ ] 阻断 knowledge/runtime loader 层引入数据库、向量库、外部知识平台依赖。
2. [ ] 首期 deny-list 至少覆盖：
   - `database/sql`
   - `github.com/jackc/pgx`
   - ORM/Redis/vector/RAG 相关依赖
   - 直接连接外部知识平台或 embedding 服务的代码

## 8. 实施拆解与文件落点

### 8.1 基础设施

1. [X] 建立以下目录：
   - `internal/server/assistant_knowledge_md/intent/`
   - `internal/server/assistant_knowledge_md/actions/`
   - `internal/server/assistant_knowledge_md/replies/`
   - `internal/server/assistant_knowledge_md/tools/`
   - `internal/server/assistant_knowledge_md/wiki/`
2. [X] 新增 direct runtime loader 与测试，建议文件落点：
   - `internal/server/assistant_knowledge_markdown_runtime.go`
   - `internal/server/assistant_knowledge_markdown_runtime_test.go`
3. [X] 新增门禁脚本与 Makefile 接线：
   - `scripts/ci/check-assistant-knowledge-single-source.sh`
   - `scripts/ci/check-assistant-knowledge-runtime-load.sh`
   - `scripts/ci/check-assistant-knowledge-no-json-runtime.sh`
   - `scripts/ci/check-assistant-no-legacy-overlay.sh`
   - `scripts/ci/check-assistant-no-knowledge-literals.sh`
   - `scripts/ci/check-assistant-knowledge-no-archive-ref.sh`
   - `scripts/ci/check-assistant-knowledge-contract-separation.sh`
   - `scripts/ci/check-assistant-no-knowledge-db.sh`

### 8.2 内容迁移

1. [X] 将现有 query、knowledge、action、reply、tool 说明性知识迁入 Markdown。
2. [X] 对历史散落在 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 的知识型文本建立迁移清单，并先收口 runtime/prompt 直邻散点。
3. [X] 删除 `assistant_knowledge/*.json`，不保留镜像文件、导出物或 snapshot 目录。

### 8.3 Runtime 影响面

1. [ ] `370A` 必须把 `assistant_knowledge_runtime.go` 改为直接读取 Markdown。
2. [ ] `370A` 不要求完全清空所有代码散点知识，但要求禁止新增新的散点入口。
3. [ ] `370A` 必须同步收敛 `assistant_model_gateway.go` 的 semantic route/action 口径，避免 Markdown 与模型提示词并存两套路由真相源。
4. [ ] `370A` 完成后，`370B` 应只剩“清理动作知识散点 + contract / knowledge 强分离”工作，而不再需要回头补 JSON cutoff 或 direct runtime 基座。

## 9. 验收与测试

1. [X] 文档与 gate 验收：
   - `make check assistant-knowledge-single-source`
   - `make check assistant-knowledge-runtime-load`
   - `make check assistant-knowledge-no-json-runtime`
   - `make check assistant-no-legacy-overlay`
   - `make check assistant-no-knowledge-literals`
   - `make check assistant-knowledge-no-archive-ref`
   - `make check assistant-knowledge-contract-separation`
   - `make check assistant-no-knowledge-db`
2. [X] Go 测试验收：
   - `go test ./internal/server/...`
3. [ ] `370A` 至少需要覆盖以下失败面：
   - front matter 缺字段
   - 重复 `id`
   - 非法 `route_kind`
   - `tool_name` 未注册
   - `action_key` / `required_checks` 与正式 contract 冲突
   - `source_refs` / `wiki_refs` / `tool_refs` / `reply_refs` 坏引用
   - archive 引用
   - Markdown runtime load 失败
   - `assistant_knowledge/*.json` 回流
   - 代码入口新增知识型 literals
   - semantic prompt route/action 枚举与 active Markdown 索引不一致
4. [ ] `370A` 验收口径冻结为：
   - `assistant_knowledge_md/` 成为全部运行时知识的唯一人工主源
   - runtime 能直接从 Markdown 建立完整知识索引
   - `assistant_knowledge/*.json` 已被删除
   - 不再存在 query/action 分裂 ownership
   - 不再允许 overlay / pass-through / JSON 快照作为正式方案
   - semantic prompt 不再形成第二套路由真相源

## 10. 完成定义（DoD）

1. [ ] `internal/server/assistant_knowledge_md/` 成为全部 Assistant Runtime Knowledge 的唯一人工主源。
2. [ ] runtime 直接从 Markdown 读取知识，不再依赖 `assistant_knowledge/*.json`。
3. [ ] `assistant_knowledge/*.json` 已从仓库与运行时路径中删除。
4. [ ] 反回流门禁全部接入命令入口并具备 fail-closed 语义。
5. [ ] `370A` 完成后，`370B` 的剩余职责只剩动作知识散点清理与 contract / knowledge 强分离。

## 11. 关联事实源

1. `docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md`
2. `docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md`
3. `docs/dev-plans/370b-assistant-business-action-knowledge-runtime-consumption-plan.md`
4. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
