# DEV-PLAN-469：CubeBox 模型驱动会话压缩批判与重构方案

**状态**: 规划中（2026-04-25 14:32 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：批判并重构 CubeBox 当前由本地代码拼接摘要的 compaction 实现，尽量参照 OpenAI Codex 的模型参与式压缩：代码负责预算、上下文选择、事件审计与 replacement history，模型负责语义摘要或通过 compact endpoint 返回压缩后的上下文。
- **关联模块/目录**：`modules/cubebox/compaction.go`、`modules/cubebox/store.go`、`modules/cubebox/gateway.go`、`modules/cubebox/turn_prep.go`、`internal/server/cubebox_api.go`、`apps/web/src/pages/cubebox`、`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-430`、`DEV-PLAN-434`、`DEV-PLAN-438A`、`DEV-PLAN-460`、`DEV-PLAN-462`、`DEV-PLAN-468`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、manual `/compact`、pre-turn auto compact、会话恢复、`/internal/cubebox/conversations/{conversation_id}:compact`、`/internal/cubebox/turns:stream`
- **证据记录 SSOT**：后续实现、真实模型 smoke、fixture、网络请求和门禁结果统一登记到 `docs/dev-records/DEV-PLAN-469-READINESS.md`；本文件只冻结批判结论、目标架构、实施切片与验收口径。

### 0.1 Simple > Easy 三问

1. **边界**：compaction 的 owner 仍是 `modules/cubebox` 会话上下文层；provider adapter 只负责调用模型或 remote compact 能力；store 只负责 append-only 原始事件、压缩事件落库和序号事务安全；query 链不得把 compaction summary 当查询事实源。
2. **不变量**：原始会话事件必须 append-only、可恢复、可审计；压缩结果只是 provider prompt view 的替代输入，不替代 canonical events；当前租户、当前用户、权限、页面/业务上下文必须由服务端每轮重新注入；模型不得获得密钥或未授权数据；失败时不得引入 legacy 双链路。
3. **可解释**：reviewer 必须能在 5 分钟内说明：为什么当前本地拼接摘要不是语义压缩；Codex 为什么把语义摘要交给模型或 compact endpoint；CubeBox 如何保留审计事实、预算边界和 canonical context reinjection。

### 0.2 现状研究摘要

- **现状实现**：
  - `BuildPromptViewWithCompaction(...)` 从 canonical events 收集 timeline，默认只保留最近 `4` 个 item。
  - 更早 timeline 交给 `buildSummaryText(...)` 拼成 `role: content` 文本。
  - recent user message 通过估算 token 做字符级截断。
  - `CompactConversation(...)` 在事务内读取事件、调用本地 prompt builder、按需写入 `turn.context_compacted`。
  - pre-turn auto compact 通过 `PrepareTurnStream(...)` 调用 store compact，再把 `PromptView` 送到 provider。
- **现状约束**：
  - 原始事件 append-only 的方向正确。
  - canonical context reinjection 的方向正确。
  - no-op compaction 不落空事件、sequence 单事务推进的方向正确。
  - 但 `summary_text` 的语义质量完全由本地拼接决定，缺少模型对任务进展、约束、用户意图、失败路径和下一步状态的压缩判断。
- **最容易出错的位置**：
  - 误把本地拼接文本称为“语义摘要”。
  - 长线程里把旧事实、用户决策、工具结果和未完成任务压成低信噪比流水账。
  - 把 provider/runtime/model 运维元数据注入业务 prompt view 后再用禁词防泄露。
  - 后续为补救摘要质量继续堆 regex、模板或本地规则，进一步挤压模型语义能力。
- **本次不沿用的容易做法**：
  - 不继续扩大 `buildSummaryText(...)` 的本地模板复杂度。
  - 不通过更多字符截断、关键词提取或 role 拼接伪装语义摘要。
  - 不新增第二套 conversation store、memory pipeline、向量库或长期用户画像。
  - 不把 summary 当查询 planner 的事实源。

## 1. 批判结论

当前 CubeBox compaction 的问题不是“没有压缩”，而是**把压缩误降级成了本地文本整理**。

`buildSummaryText(...)` 把较早 timeline 串成：

```text
user: ...
assistant: ...
summary: ...
```

这种做法只能减少 prompt item 数量，不能真正完成语义取舍。它不会识别哪些事实是长期约束，哪些只是临时噪声；不会总结任务状态、用户决策、失败原因、未完成事项、工具调用结果和下一步意图；也不会把多个重复回合折叠成稳定的 handoff state。

这与 CubeBox “数字助手长线程能力”的目标不匹配。长线程里最有价值的不是保留所有话术，而是保留：

- 当前任务目标和用户最后确认的决策。
- 已执行动作、结果和失败原因。
- 仍需遵守的约束、权限、租户、页面和业务对象。
- 未完成事项和下一步。
- 不能再重复询问或不能静默猜测的上下文。

这些属于语义摘要工作，应优先交给模型或 provider 的 compact 能力；本地代码应退回到 guardrail、预算、事件和恢复语义。

## 2. Codex 参考结论

OpenAI Codex 开源实现不是用本地代码简单拼接摘要。

参考入口：

- `openai/codex`：`codex-rs/core/src/compact.rs`
- `openai/codex`：`codex-rs/core/src/compact_remote.rs`
- `openai/codex`：`codex-rs/core/src/client.rs`
- OpenAI 工程说明：`https://openai.com/index/unrolling-the-codex-agent-loop/`
- OpenAI Responses Compact API：`https://platform.openai.com/docs/api-reference/responses/compact?api-mode=responses`

Codex 的关键分层是：

1. **代码负责触发与装配**：判断是否需要 compact、选择历史窗口、构造 compact input、接收 replacement history、替换 active history。
2. **模型负责语义压缩**：
   - remote compact 路径调用 provider compact endpoint，例如 `/responses/compact`。
   - local fallback 路径也会构造 summarization prompt 并发起模型请求，取模型返回的 assistant summary，而不是由本地代码拼接摘要。
3. **原始历史不等于 active prompt view**：压缩后 active history 可替换，但原始事件仍可作为审计和恢复事实。
4. **压缩不是无限记忆**：仍需要预算、窗口、工具结果裁剪、canonical context reinjection 和失败降级。

CubeBox 应学习的是这个职责分配，而不是只复制文件名或事件名。

## 3. 设计目标

1. [ ] 先建立一个“无摘要基线阶段”：停用 `compaction summary` 生产语义，不做本地摘要，只把完整历史视图 + canonical context 继续喂给模型，用于先验证连贯性主链和 owner 边界。
2. [ ] 在无摘要基线稳定后，把 CubeBox compaction 从“本地 role/content 拼接”升级为“模型参与式语义摘要或 provider remote compact”。
3. [ ] 保留 append-only 原始事件、canonical context reinjection、manual compact、pre-turn auto compact 和 sequence 单事务安全；`turn.context_compacted` 是否写入取决于当前阶段能力，不得伪造摘要事件。
4. [ ] 建立 provider capability：能 remote compact 则优先 remote compact；不能 remote compact 则用当前 active model 执行 summarization prompt；没有 provider runtime 时仅允许 deterministic fixture 测试路径，不伪装为真实语义摘要。
5. [ ] 明确 compact summary 只服务 provider prompt view，不作为查询 planner、授权、RLS、业务事实、页面状态或执行参数的事实源。
6. [ ] 修正 `DEV-PLAN-430/434/462/468` 之间关于“当前 active model 执行 compaction”的文档漂移，避免文档宣称模型摘要而代码仍是本地拼接。
7. [ ] 设计可测试的 prompt shape、summary quality fixture、fallback 语义和失败事件，避免把真实外部模型调用变成阻断式 CI 前置。

## 4. 非目标

- 不建设跨会话长期记忆。
- 不引入向量数据库、外部 RAG、Redis、外部缓存或用户画像系统。
- 不新增独立 summary model 配置链；首期只使用当前 active model，remote compact 属于 provider capability。
- 不让模型直接读取数据库、绕过 API、绕过权限或访问密钥。
- 不把 compact summary 作为查询事实源；查询连续性仍应使用结构化 query dialogue context 和 canonical events。
- 不把真实外部模型调用纳入阻断式 CI；真实模型只作为手工 smoke、nightly 或 readiness 补充证据。
- “无摘要基线阶段”不是永久方案；它只用于验证当前连贯性问题是否主要来自本地伪摘要，而不是为了长期放弃预算治理。

## 5. 目标架构

### 5.1 新增 compaction 执行分层

建议把当前 `BuildPromptViewWithCompaction(...)` 拆成三类职责：

1. **History selector**
   - 输入 canonical events、current user input、预算策略。
   - 输出 `CompactionCandidate`：待摘要历史、recent window、source range、source digest、token estimate。
   - 纯函数，可稳定测试。

2. **Compaction engine**
   - 输入 `CompactionCandidate`、canonical context、provider runtime。
   - 输出 `CompactionSummary` 或 `CompactedReplacementHistory`。
   - 支持两种实现：
     - `RemoteCompactionEngine`：provider 支持 remote compact 时调用 compact endpoint。
     - `ModelSummaryCompactionEngine`：用当前 active model + summarization prompt 生成语义 summary。

3. **Prompt view assembler**
   - 输入 recent window、summary/replacement history、canonical context、current user input。
   - 输出 provider prompt view。
   - 永远由服务端重新注入 canonical context，不信任旧 summary 自带的上下文。

### 5.1A 首阶段：无摘要基线（No-Summary Baseline）

在真正引入模型摘要或 remote compact 之前，先冻结一个更简单的过渡阶段：

1. 停用 `buildSummaryText(...)` 的生产语义，不再把 `role: content` 拼接结果作为 provider prompt view 的摘要输入。
2. pre-turn auto compact 与 manual compact 仍可保留统一入口，但首阶段只负责：
   - 读取 canonical events
   - 重建完整历史视图
   - 重新注入 canonical context
   - 把完整历史视图继续送到 provider
3. 首阶段不得伪造“已经完成语义压缩”的 `turn.context_compacted` 事件；是否完全不写事件，或只写显式 `disabled/no_summary_baseline` 语义，必须在实现前冻结且前后一致。
4. 首阶段的目标不是优化 token 成本，而是先验证：
   - provider 主链确实持续消费完整历史视图
   - 连贯性缺陷是否主要来自本地伪摘要
   - query 链与普通对话链都没有把 summary 当事实源
5. 首阶段允许暴露“长会话预算风险”，但不允许再把本地拼接文本伪装成语义摘要来掩盖该风险。

### 5.2 Provider capability

`ProviderAdapter` 或相邻能力接口应表达：

- `SupportsRemoteCompaction() bool`
- `CompactConversationHistory(ctx, request) (response, error)`
- `StreamChatCompletion(ctx, request) (...)`

分阶段策略：

1. **Phase 1 / No-Summary Baseline**：不调用 remote compact，不调用模型摘要，只把完整历史视图送到 provider。
2. **Phase 2 / Model Summary Fallback**：provider 不支持 remote compact，但 chat completion 可用时，使用当前 active model 执行 summarization prompt。
3. **Phase 3 / Remote Compact Capability**：provider 支持 remote compact 时，优先使用 remote compact。
4. provider 不可用：manual compact 返回明确错误；pre-turn auto compact 不应伪造模型摘要，可退回未压缩 prompt view 或终止当前 turn，具体失败语义需在实现前冻结。

### 5.3 Summary prompt 方向

模型摘要 prompt 不应要求“压缩所有文本”，而应输出 handoff state：

- 用户目标。
- 已确认事实。
- 已执行动作与结果。
- 重要约束与不能违反的边界。
- 未完成事项。
- 最近用户意图和下一步。
- 必须避免重复询问的内容。

同时 prompt 必须明确：

- 不引入原文没有的业务事实。
- 不把 provider/runtime/secret 写入摘要。
- 不把内部执行 JSON、API key、payload、results、权限细节暴露给用户可见回答。
- 摘要是给后续模型继续对话使用，不是审计事实源。

### 5.4 事件契约

`turn.context_compacted` 可以保持最小 envelope，但需要增加足够区分来源的字段，避免把本地拼接和模型摘要混为一谈。

候选 payload：

```json
{
  "summary_id": "summary_xxx",
  "source_range": [1, 12],
  "summary_text": "...",
  "source_digest": "...",
  "reason": "manual|pre_turn_auto|history_limit_exceeded",
  "compaction_mode": "remote|model_summary|deterministic_fixture",
  "provider_id": "provider-1",
  "model_slug": "gpt-5.2"
}
```

注意：`provider_id/model_slug` 可作为事件 metadata 和运维诊断字段，但默认不应进入后续业务 prompt 的 canonical block。

## 6. 实施切片

### Slice A：文档与漂移收口

1. [ ] 更新 `DEV-PLAN-430/434/462/468` 中与 compaction 实现不一致的表述。
2. [ ] 新增 `DEV-PLAN-469-READINESS.md` 证据入口。
3. [ ] 明确当前实现状态：本地拼接摘要是已知缺陷，不再称为模型语义摘要。

### Slice B：Phase 1 / No-Summary Baseline

1. [ ] 停用 `buildSummaryText(...)` 的生产语义，不再向 provider prompt view 注入本地拼接 `summary_text`。
2. [ ] 保留完整历史视图重建、canonical context reinjection 和当前 user input 拼装，先验证无摘要状态下的连续性主链。
3. [ ] 冻结 manual compact / pre-turn auto compact 在首阶段的语义：
   - 可以继续作为统一入口存在
   - 但不得再伪造“已完成语义压缩”的事件或 UI 文案
4. [ ] 明确首阶段验收：
   - 普通连续追问不因停用本地伪摘要而退化
   - query 链不依赖 compact summary
   - provider 持续消费完整历史视图而不是裸 `turn.Prompt`

### Slice C：纯函数边界拆分

1. [ ] 从 `BuildPromptViewWithCompaction(...)` 中拆出 history selector。
2. [ ] 保留 source range、source digest、recent window、token estimate 的纯函数测试。
3. [ ] 删除或降级 `buildSummaryText(...)` 的生产语义地位；它最多保留为 deterministic fixture 或 debug fallback，不再作为真实 compact summary。

### Slice D：模型摘要 fallback

1. [ ] 定义 `ModelSummaryCompactionEngine`。
2. [ ] 复用当前 active model provider runtime，不新增独立 summary model 配置链。
3. [ ] 新增 summarization prompt 模板和 prompt shape fixture。
4. [ ] 失败时返回明确错误码，manual compact 向 UI 显示可理解失败；pre-turn auto compact 失败策略在代码实现前冻结。

### Slice E：remote compact capability

1. [ ] 在 provider adapter 层表达 remote compact capability。
2. [ ] 对支持 `/responses/compact` 或等价 compact endpoint 的 provider，优先走 remote compact。
3. [ ] 对不支持 remote compact 的 provider，落到 Slice C 的 active model summary。
4. [ ] 测试覆盖 capability 选择、错误映射、超时、无效 response 和 replacement history 组装。

### Slice F：Prompt view 与安全边界

1. [ ] compact 后 prompt view 继续由服务端注入 canonical context。
2. [ ] 从普通业务 prompt view 中移除不必要的 provider/runtime/model 运维元数据；这类信息保留在 event metadata、日志或 UI 状态。
3. [ ] 保证 summary role 到 provider role 的映射仍符合 `DEV-PLAN-438A`。
4. [ ] 保证 query planner 不读取 compaction summary 作为事实源。

### Slice G：验证与回归

1. [ ] 单元测试覆盖 history selector、prompt view assembler、model summary engine fake adapter、remote compact fake adapter。
2. [ ] store 测试覆盖 compact event payload、no-op、并发 sequence。
3. [ ] gateway 测试先覆盖 Phase 1 无摘要完整历史视图，再覆盖后续模型摘要/replacement history。
4. [ ] 前端测试覆盖 manual compact 成功、失败、恢复展示。
5. [ ] readiness 记录真实 provider smoke，但不作为阻断式 CI。

## 7. 风险与决策

| 风险 | 判断 | 收敛方式 |
| --- | --- | --- |
| 无摘要基线导致长会话超预算 | 首阶段明确存在 | 作为已知风险暴露；先验证连贯性主链，后续用模型摘要或 remote compact 解决预算问题 |
| 模型摘要幻觉 | 真实风险 | 原始事件 append-only；summary 不作事实源；prompt 明确不得引入原文无事实；关键查询状态走结构化 canonical events |
| 成本与延迟增加 | 可接受但需预算 | 只在 manual 或阈值触发；设置 timeout；记录 token estimate；失败可清晰降级 |
| provider 不支持 remote compact | 常态 | active model summary fallback |
| CI 依赖外部模型不稳定 | 不允许 | fake adapter + fixture 为 required gate，真实模型仅 readiness smoke |
| 文档再次漂移 | 高风险 | `469` 接管 compaction 语义摘要重构 owner；`430/434/462/468` 只引用，不复制实现细节 |
| summary 泄露内部信息 | 可控风险 | summary prompt、输出校验、event metadata 与 prompt content 分离 |

## 8. 验收口径

1. [ ] Phase 1 中，provider request 不再消费本地拼接 `summary_text`，而是继续消费完整历史视图 + canonical context + 当前 user input。
2. [ ] reviewer 能从代码中看到后续 compaction summary 来自 remote compact 或 active model summary，而不是 `role: content` 拼接。
3. [ ] manual compact 与 pre-turn auto compact 都使用同一 compaction engine 分层；Phase 1 允许该 engine 返回“无摘要完整历史视图”。
4. [ ] 原始 conversation events 不被覆盖，恢复链路仍可重放。
5. [ ] compact 后 provider request 包含完整历史视图或后续语义摘要/replacement history、当前 user input 和重新注入的 canonical context。
6. [ ] query 链不把 compaction summary 当事实源。
7. [ ] fake adapter 测试能稳定验证成功、失败、超时和无效响应。
8. [ ] 文档地图、关联 dev-plan 和 readiness 入口全部对齐。

## 9. 本地必跑与门禁

本计划当前为文档方案，命中 `文档` 触发器：

- `make check doc`

后续命中 Go / UI / routing / authz / i18n / E2E 时，按 `AGENTS.md` 触发器矩阵与对应 SSOT 执行，不在本文复制命令矩阵。

## 10. Stopline

- 不得继续把本地拼接 `buildSummaryText(...)` 描述为模型语义摘要。
- 不得把“无摘要基线阶段”偷换成“直接关闭历史视图重建，只发当前轮 prompt”。
- 不得为了“看起来像 Codex”只改事件名或文档名，而不让模型参与摘要。
- 不得把 remote compact 或 model summary 的输出作为授权、查询事实、RLS、业务执行参数或页面状态的事实源。
- 不得新增独立 summary model 配置链，除非另立计划重新评审模型治理、权限、健康检查和管理 UI。
- 不得引入 legacy fallback、双链路、旧 assistant surface 或第二套 conversation store。
- 不得让真实外部模型调用成为 required CI gate。

## 11. 参考链接

- OpenAI Codex compact：`https://github.com/openai/codex/blob/main/codex-rs/core/src/compact.rs`
- OpenAI Codex remote compact：`https://github.com/openai/codex/blob/main/codex-rs/core/src/compact_remote.rs`
- OpenAI Codex client compact endpoint：`https://github.com/openai/codex/blob/main/codex-rs/core/src/client.rs`
- OpenAI Codex agent loop：`https://openai.com/index/unrolling-the-codex-agent-loop/`
- OpenAI Responses Compact API：`https://platform.openai.com/docs/api-reference/responses/compact?api-mode=responses`
- DEV-PLAN-430：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- DEV-PLAN-434：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- DEV-PLAN-462：`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`
- DEV-PLAN-468：`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`
