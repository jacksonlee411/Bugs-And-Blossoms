# DEV-PLAN-438B：CubeBox 流式回复换行丢失与列表结构压平专项调查与修复方案

**状态**: 规划中（2026-04-23 17:10 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`（命中用户可见回复正文、真实 provider 流式主链、事件落库/回放一致性与页面渲染验收）
- **范围一句话**：处理 CubeBox 所有正文类字段上的 `TrimSpace` 误用问题，覆盖流式回复、provider 输入、prompt view 构造、history/replay 收集与 compact summary 构造，冻结调查范围、根因假设、修复切片与验收口径，确保模型原本输出或用户原本输入的结构化文本不会在本地链路中被压平成单段。
- **关联模块/目录**：`modules/cubebox/gateway.go`、`modules/cubebox/gateway_test.go`、`modules/cubebox/compaction.go`、`modules/cubebox/compaction_test.go`、`modules/cubebox/store.go`、`internal/server/cubebox_api.go`、`internal/server/cubebox_api_test.go`、`apps/web/src/pages/cubebox/reducer.ts`、`apps/web/src/pages/cubebox/reducer.test.ts`、`apps/web/src/pages/cubebox/CubeBoxPanel.tsx`、`apps/web/src/pages/cubebox/CubeBoxPanel.test.tsx`
- **关联计划/标准**：`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-433`、`DEV-PLAN-437A`、`DEV-PLAN-438`、`DEV-PLAN-438A`
- **用户入口/触点**：主应用壳层右侧 CubeBox 抽屉、`/internal/cubebox/turns:stream`、历史会话恢复后的正文回放
- **证据记录 SSOT**：实现与浏览器复验证据统一回填 `docs/dev-records/DEV-PLAN-437-READINESS.md`；本文件只冻结边界、调查结论、修复步骤与验收口径。

### 0.1 Simple > Easy 三问

1. **边界**：本计划处理“正文类字段在本地链路中被 `TrimSpace` 错误改写”的问题，覆盖流式正文、provider 采样输入、prompt view、history/replay 与 compact summary；不重做 Markdown 渲染器，不扩写富文本协议，不引入 HTML reply、双通道消息体或第二套 message schema。
2. **不变量**：只要 provider 返回的 delta 语义上包含换行、空行、列表边界或段落分隔，本地链路不得因为 `TrimSpace`、空白过滤或二次拼接而把这些结构吞掉。
3. **可解释**：reviewer 必须能在 5 分钟内说明“换行是在 provider 返回时丢了、gateway 过滤时丢了、事件落库时丢了，还是前端显示时丢了”，并能指出唯一 owner。

## 1. 背景

在会话 `conv_d1188da5c17744889df7144ae6c6938f` 的真实页面回复中，用户提问“介绍一下你知道的”后，CubeBox 返回了一个包含编号列表的长回复。后续用户追问“为什么没有选项2”时，页面显示的上一条回答中出现了以下症状：

- `1)` 段落后面的内容与 `2)` 开头连在同一段；
- `2) 关于我“知道什么”` 实际存在，但前置换行丢失，视觉上像“没有选项 2”；
- 整体列表结构被压平成一大段，不符合模型原本应输出的结构化正文。

这不是“连续对话失忆”问题，而是“正文类字段在多条链路中被本地实现改写”问题，必须从 `438` 主问题中拆出单独 owner，避免继续混在 provider 上下文连续性议题里。

## 2. 现状研究摘要

### 2.1 已确认事实

1. 前端展示层本身保留换行。
   - `apps/web/src/pages/cubebox/CubeBoxPanel.tsx` 使用 `whiteSpace: 'pre-wrap'` 渲染消息正文。
   - 这意味着只要 `item.text` 中存在 `\n`，页面就会正常显示换行。
2. 前端 reducer 不会主动删除换行，但会把 delta 原样追加。
   - `apps/web/src/pages/cubebox/reducer.ts` 中 `mergeAgentDelta(...)` 采用字符串拼接：`existing + delta`。
   - 因此若上游把“单独的换行 delta”丢掉，前端不会补回段落边界。
3. gateway 当前会过滤纯空白 delta。
   - `modules/cubebox/gateway.go` 中存在 `if strings.TrimSpace(chunk.Delta) == "" { continue }`。
   - 这会直接跳过仅包含 `\n` 或 `\n\n` 的 chunk。
4. provider SSE parser 会对每一行做 `TrimSpace`，这是协议外壳层处理；当前正文风险不仅在 delta 级过滤，还在 prompt / prompt view / replay / summary 构造链路。
5. 事件存储/回放层当前基本按原样持久化 payload，但 compaction/replay 读取正文后会再次进入 `TrimSpace` 路径，因此不会自动保真。

### 2.2 `TrimSpace` 落点分层盘点

当前命中的 `TrimSpace` 不是同一类问题，必须按字段语义拆开，而不是笼统讨论“要不要删 `TrimSpace`”。

| 位置 | 当前对象 | 语义类型 | 当前作用 | 438B 判断 |
| --- | --- | --- | --- | --- |
| `internal/server/cubebox_api.go` | `req.Prompt` | 用户正文 | 校验 + 直接改值 | **必须收敛**：只可用于判空，不可直接改写正文 |
| `modules/cubebox/gateway.go` | `chunk.Delta` | 模型正文增量 | 过滤空白 delta | **必须取消**：会吞掉换行/空行语义 |
| `modules/cubebox/gateway.go` | `PromptItem.Content` | provider 输入正文 | 组装请求前裁边 | **必须收敛**：需要区分“校验是否空”与“保留正文原值” |
| `modules/cubebox/gateway.go` | `currentUserInput` -> `PromptItem{Content: ...}` | 当前轮 provider 输入正文 | prompt view 拼接前裁边 | **必须收敛**：当前轮正文不得在 provider 采样前被改写 |
| `modules/cubebox/compaction.go` | `currentUserInput` | 当前轮 prompt view 正文 | prompt view 构造前裁边 | **必须收敛**：当前轮正文不得在 compaction builder 被改写 |
| `modules/cubebox/compaction.go` | `turn.user_message.accepted` / `turn.agent_message.completed` / `summary_text` | replay/history/summary 正文 | 收集 timeline 时裁边 | **必须收敛**：原始正文与 summary 正文的裁边规则要显式化，不得继续隐式丢边界 |
| `modules/cubebox/compaction.go` | `buildSummaryText` 输入 | summary 派生正文 | 拼接摘要前裁边 | **必须裁决**：要么显式允许“摘要标准化”，要么承接保真规则，不能保持模糊 |
| `modules/cubebox/gateway.go` | `request.BaseURL` / `Model` / `secretRef` | 配置/标识 | 归一化 | **保留** |
| `modules/cubebox/gateway.go` | SSE `line` / `data:` wrapper | 协议外壳 | 解析容错 | **原则上保留**，但不得影响 `delta.content` 本体 |

### 2.3 当前最高置信根因假设

最高置信根因是：

- 模型把正文结构拆成多个 delta；
- 其中某些 delta 只包含换行或空行；
- gateway 把这些 delta 当作“空内容”丢弃；
- 其他正文路径又继续用 `TrimSpace` 改写 prompt / prompt view / replay / summary；
- 后续编号项文本继续追加，于是出现 `...等2)` 这种被压平的结果。

### 2.4 仍待确认的问题

1. 该具体会话中的 provider 原始 SSE 是否真的存在“newline-only delta”。
2. 是否存在“delta 不是纯换行，但被上游 `TrimSpace` 改写后变空”的情况。
3. 是否存在 `turn.agent_message.completed` 之前最后一段文本未 flush 的边角问题。
4. compact summary 是否允许保留“标准化/有损收敛”语义；若允许，边界必须明确写清。
5. 历史回放中是否已有旧坏数据；若有，是否需要只修新数据还是补历史修复策略。

## 3. 问题定义

### 3.1 用户可见问题

- 编号列表、项目符号、空行分段、引用块、代码块等结构化正文无法稳定显示。
- 用户会误以为模型“漏答了某个选项”或“没有按要求输出结构化内容”。
- 历史会话回放会稳定复现坏结果，因为坏文本已经按 append-only event log 落库。

### 3.2 技术问题

- 当前系统把“只含空白的 delta”视为可安全丢弃，但对 LLM 流式输出而言，空白本身可能就是语义边界。
- 这违反了“流式事件必须忠实承载模型正文”的基本不变量。

### 3.3 `TrimSpace` 使用原则失焦

当前问题进一步暴露出一个更底层的实现口径错误：

- `TrimSpace` 被同时用于“标识归一化”和“正文处理”；
- 但这两类字段的语义完全不同；
- 一旦把“正文字段”也按“标识字段”的方式 trim，就会把合法内容错误当成噪音。

本计划因此不仅要修 newline-only delta，还要冻结 CubeBox 对所有正文类字段的 `TrimSpace` 使用边界，防止同类问题在 prompt、provider input、summary、history replay、future renderer 中反复回流。

### 3.4 本计划承接的“所有正文类字段”定义

本计划中“所有正文类字段”专指以下文本载体：

- HTTP 入站的用户 `prompt`
- runtime/gateway 侧当前轮 `currentUserInput`
- provider 返回的 `delta`
- `PromptItem.Content`
- event log 中 `turn.user_message.accepted.payload.text`
- event log 聚合出的 assistant message 正文
- `turn.context_compacted.payload.summary_text`
- `buildSummaryText(...)` 生成的 summary 派生正文

不包括：

- `conversation_id`
- `provider_id`
- `provider_type`
- `model_slug`
- `base_url`
- `secret_ref`
- URL path/query 中的资源标识

这些仍归类为标识/配置字段，不属于本计划的正文保真范围。

## 4. 设计决策

1. **选定方案**：把 `turn.agent_message.delta` 视为正文事实流；除非 provider 明确给出无效 chunk，否则不得基于 `TrimSpace(delta)==""` 丢弃任何 delta。
2. **不选方案**：不在前端用正则把 `1)`、`2)`、`3)` 强行重新切段，这会把显示修复伪装成语义修复。
3. **不选方案**：不新增 Markdown AST、HTML 渲染或富文本协议来掩盖流式正文事实损坏。
4. **不选方案**：不在历史回放层自动“猜测性补换行”，避免对 append-only 审计事实做不可解释改写。
5. **失败语义**：若 provider chunk 确认为协议无效，应继续按现有 `ErrProviderStreamInvalid` fail-closed；但“只含换行/空格”不属于协议无效。
6. **分层原则**：`TrimSpace` 只允许用于“标识类、配置类、协议壳层字段”；对“正文类字段”，最多允许“trim 后判空”，不得用 trim 后的值覆盖原文。
7. **完整承接**：本计划不只修 `delta`，而是完整承接所有正文类字段的 `TrimSpace` 收敛；若某处必须保留有损标准化，必须在文档中被点名批准。
8. **摘要裁决**：`compact summary` 可允许“有损重组”，但不得再以“隐式 `TrimSpace` 裁边”方式发生；若需要标准化，只能作为显式摘要策略，而不是正文保真链路中的副作用。
9. **真实返回裁决**：以真实 provider 返回为准；若 provider 事实层未返回换行，则 CubeBox 不做额外补换行、列表重排或显示层猜测性修复。

### 4.1 `TrimSpace` 分层规则（本计划冻结）

#### 规则 A：标识/配置字段

适用对象：

- `conversation_id`
- `provider_id`
- `provider_type`
- `model_slug`
- `base_url`
- `secret_ref`
- query/path 中的资源标识

处理规则：

- 允许 `TrimSpace`
- 允许用 trim 后结果回写
- 允许以 trim 后结果判空/校验/归一化

#### 规则 B：正文字段

适用对象：

- 用户输入 `prompt`
- provider 返回 `delta`
- `PromptItem.Content`
- replay/history 中的消息正文
- compact summary 正文

处理规则：

- 不允许用 `TrimSpace` 后的结果直接覆盖原值
- 若业务需要“拒绝纯空输入”，只能使用 `strings.TrimSpace(raw) == ""` 做校验
- 实际写库、传递、拼接、展示必须保留原始正文

#### 规则 C：协议外壳字段

适用对象：

- SSE 每行 `data:` 之前的包裹文本
- response line wrapper

处理规则：

- 允许 `TrimSpace`
- 但只限于协议解析，不得改变 `delta.content` 本体

### 4.2 本计划对 `TrimSpace` 的明确裁决

1. **必须取消**
   - `chunk.Delta` 上的 `TrimSpace(... ) == "" => continue`
2. **必须收敛为“只校验不改值”**
   - `req.Prompt = strings.TrimSpace(req.Prompt)`
   - `PromptItem.Content` 在 provider request 组装边界的直接 trim 覆盖
   - `promptViewForProvider(...)` 中 `currentUserInput` 追加前的 trim 覆盖
   - `BuildPromptViewWithCompaction(...)` 中当前用户输入追加前的 trim 覆盖
3. **必须显式裁决**
   - `collectPromptTimeline(...)` 中 user/assistant/summary 文本的 trim 逻辑
   - `buildSummaryText(...)` 中派生 summary 的标准化策略
4. **应继续保留**
   - `conversation_id` / `provider_id` / `model_slug` / `base_url` / `secret_ref`
   - SSE 包裹层 `line` / `data:` 的协议解析 trim

## 5. 最小实现路径

### 5.1 Slice A：补齐调查证据

1. [x] 增加针对 `newline-only delta` 的 gateway 单测夹具。
2. [x] 增加针对“列表项被拆为 `文本 chunk + 换行 chunk + 下一项 chunk`”的端到端 reducer/gateway 回归测试。
3. [ ] 若条件允许，补抓一次真实 provider SSE 证据，证明问题不是前端幻觉。
4. [x] 盘点并落证据：列出所有正文类字段上的 `TrimSpace` 命中点，确保实施不漏改。

### 5.2 Slice B：修复 gateway 过滤策略

1. [x] 删除或收敛 `strings.TrimSpace(chunk.Delta) == "" => continue` 这一逻辑。
2. [x] 保证 `chunk.Delta` 即使只包含 `\n`、`\n\n`、前导缩进，也能按原样写入 `turn.agent_message.delta`。
3. [x] 明确 adapter/parser 哪一层负责“协议合法性”，哪一层不得碰正文空白语义。

### 5.3 Slice C：收敛 `TrimSpace` 使用边界

1. [x] `req.Prompt` 改为“trim 后判空，但继续保留原始 prompt 进入 runtime/store”。
2. [x] `PromptItem.Content` 改为“必要时用 trimmed 判断是否为空，但保留原值作为 provider content”。
3. [x] `promptViewForProvider(...)` 改为“必要时用 trimmed 判空，但保留原始当前轮正文追加到 provider 采样输入”。
4. [x] `BuildPromptViewWithCompaction(...)` 改为“必要时用 trimmed 判空，但保留原始当前轮正文追加到 prompt view”。
5. [x] 为“标识类字段可 trim，正文类字段不可改值”补纯函数/适配层测试，冻结未来口径。

### 5.4 Slice D：收敛 replay / compaction / summary 路径

1. [x] `collectPromptTimeline(...)` 中 user/assistant message 收集链路改为“判空与原值保留”分离。
2. [x] 明确 `summary_text` 的策略：
   - 若归类为正文保真对象，则不得再被隐式 trim 改值；
   - 若允许摘要标准化，则需把标准化写成显式摘要策略并单测冻结。
3. [x] `buildSummaryText(...)` 的输入标准化规则显式化，不再依赖隐式 `TrimSpace` 副作用。
4. [x] 补齐 compaction/history 回归测试，覆盖“列表/空行进入 replay 后不再被无意压平”的场景。

### 5.5 Slice E：补齐页面与回放回归

1. [x] 前端 reducer 测试中新增保留换行/空行/编号列表边界的断言。
2. [x] 页面测试新增“含编号列表与空行”的消息显示断言，证明 `pre-wrap` 路径有效。
3. [ ] 历史恢复测试新增“坏结果不会被新代码继续制造”的断言。

### 5.6 Slice F：浏览器验收

1. [ ] 用真实页面复现“1/2/3 列表”回复。
2. [ ] 验证列表项之间存在稳定换行，不再出现 `...等2)` 贴连。
3. [ ] 验证“为什么没有选项2”这类追问场景下，上一条原文肉眼可见 `2)` 列表项独立成段。
4. [ ] 验证 compact 前后，原始 message 回放与新生成 message 的结构边界都符合预期；若 summary 本身采用标准化文本，也必须符合文档定义。

## 6. Owner 边界

| 主题 | owner | 其他方职责 |
| --- | --- | --- |
| provider SSE 协议解析 | `DEV-PLAN-433` 对应 gateway/adapter owner | 只负责协议级解码，不得擅自丢正文空白 |
| 连续对话上下文主链 | `DEV-PLAN-438` | 不持有正文格式修复 |
| provider role 收敛 | `DEV-PLAN-438A` | 不持有换行保真问题 |
| 正文结构保真与正文类字段 `TrimSpace` 收敛 | `DEV-PLAN-438B` | 持有 prompt/delta/prompt view/replay/summary 的正文边界、回放一致性与页面验收 |

## 7. 停止线

- 不得再把“纯空白 delta”默认视为无意义噪音。
- 不得把显示层正则修补当作根因修复。
- 不得为了这次问题引入 HTML reply、富文本消息体、第二正文字段或双链路 fallback。
- 不得修改已落库历史事件内容来“补出”不存在的换行；若需历史修复，必须另立明确计划。
- 不得继续把正文字段与标识字段混用同一套 `TrimSpace` 改值口径。
- 不得把 `compact summary` 的有损整理继续伪装成“正文 trim 很正常”；若允许摘要标准化，必须显式声明为摘要策略。
- 不得因为真实 provider 未换行，就在 CubeBox 本地额外插入换行、重排列表或伪造新的正文结构。

## 8. 验收标准

1. [x] `turn.agent_message.delta` 对 `newline-only delta` 不再丢弃。
2. [x] gateway / reducer / panel 自动化测试覆盖“换行 chunk 独立到达”的场景。
3. [ ] 浏览器中编号列表能稳定按行展示，不再出现 `...等2)` 粘连；若真实 provider 返回本身不换行，则按事实展示，不视为本地正文保真缺陷。
4. [x] `prompt/currentUserInput/PromptItem.Content` 路径完成“trim 仅用于校验，不用于改写正文”的收敛；其中浏览器发送主链与 server/runtime/gateway 主链都已纳入。
5. [x] replay/history 路径完成正文类字段的 `TrimSpace` 收敛；`recent user prompt view` 现已改为“仅在超出 token budget 时显式截断，否则保留原始首尾空白”。
6. [x] `compact summary` 的标准化规则被显式冻结，不再通过隐式 trim 副作用发生。
7. [x] 历史恢复后的新会话回复不再继续制造相同结构损坏。
8. [x] 不引入第二套渲染协议、不新增 fallback、不改写旧事件事实。

## 9. 交付物

1. [ ] 文档：
   - `docs/dev-plans/438b-cubebox-streaming-newline-loss-investigation-and-remediation-plan.md`
   - `AGENTS.md`
2. [ ] 代码修复：
   - `modules/cubebox/gateway.go`
   - `modules/cubebox/gateway_test.go`
   - `modules/cubebox/compaction.go`
   - `modules/cubebox/compaction_test.go`
   - `internal/server/cubebox_api.go`
   - `internal/server/cubebox_api_test.go`
   - `apps/web/src/pages/cubebox/reducer.test.ts`
   - `apps/web/src/pages/cubebox/CubeBoxPanel.test.tsx`
3. [x] 证据回填：
   - `docs/dev-records/DEV-PLAN-437-READINESS.md`

## 10. 关联文档

- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- `docs/dev-plans/438-cubebox-conversational-continuity-investigation-and-remediation-plan.md`
- `docs/dev-plans/438a-cubebox-provider-message-role-normalization-and-codex-summary-alignment-plan.md`
