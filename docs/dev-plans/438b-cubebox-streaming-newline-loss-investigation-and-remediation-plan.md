# DEV-PLAN-438B：CubeBox 流式回复换行丢失与列表结构压平专项调查与修复方案

**状态**: 规划中（2026-04-23 17:10 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`（命中用户可见回复正文、真实 provider 流式主链、事件落库/回放一致性与页面渲染验收）
- **范围一句话**：只处理 CubeBox 在真实流式回复中出现的“换行/空行/段落边界/编号列表结构丢失”问题，冻结调查范围、根因假设、修复切片与验收口径，确保模型原本输出的结构化文本不会在本地流式链路中被压平成单段。
- **关联模块/目录**：`modules/cubebox/gateway.go`、`modules/cubebox/gateway_test.go`、`modules/cubebox/store.go`、`apps/web/src/pages/cubebox/reducer.ts`、`apps/web/src/pages/cubebox/CubeBoxPanel.tsx`
- **关联计划/标准**：`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-433`、`DEV-PLAN-437A`、`DEV-PLAN-438`、`DEV-PLAN-438A`
- **用户入口/触点**：主应用壳层右侧 CubeBox 抽屉、`/internal/cubebox/turns:stream`、历史会话恢复后的正文回放
- **证据记录 SSOT**：实现与浏览器复验证据统一回填 `docs/dev-records/DEV-PLAN-437-READINESS.md`；本文件只冻结边界、调查结论、修复步骤与验收口径。

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理“文本结构在本地流式链路中丢失”的问题，不重做 Markdown 渲染器，不扩写富文本协议，不引入 HTML reply、双通道消息体或第二套 message schema。
2. **不变量**：只要 provider 返回的 delta 语义上包含换行、空行、列表边界或段落分隔，本地链路不得因为 `TrimSpace`、空白过滤或二次拼接而把这些结构吞掉。
3. **可解释**：reviewer 必须能在 5 分钟内说明“换行是在 provider 返回时丢了、gateway 过滤时丢了、事件落库时丢了，还是前端显示时丢了”，并能指出唯一 owner。

## 1. 背景

在会话 `conv_d1188da5c17744889df7144ae6c6938f` 的真实页面回复中，用户提问“介绍一下你知道的”后，CubeBox 返回了一个包含编号列表的长回复。后续用户追问“为什么没有选项2”时，页面显示的上一条回答中出现了以下症状：

- `1)` 段落后面的内容与 `2)` 开头连在同一段；
- `2) 关于我“知道什么”` 实际存在，但前置换行丢失，视觉上像“没有选项 2”；
- 整体列表结构被压平成一大段，不符合模型原本应输出的结构化正文。

这不是“连续对话失忆”问题，而是“单条消息的正文结构被本地链路破坏”问题，必须从 `438` 主问题中拆出单独 owner，避免继续混在 provider 上下文连续性议题里。

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
4. provider SSE parser 会对每一行做 `TrimSpace`，但当前主要风险不在这里，而在 delta 级过滤。
5. 事件存储/回放层当前基本按原样持久化 payload，不会在读取时自动修复正文结构。

### 2.2 `TrimSpace` 落点分层盘点

当前命中的 `TrimSpace` 不是同一类问题，必须按字段语义拆开，而不是笼统讨论“要不要删 `TrimSpace`”。

| 位置 | 当前对象 | 语义类型 | 当前作用 | 438B 判断 |
| --- | --- | --- | --- | --- |
| `internal/server/cubebox_api.go` | `req.Prompt` | 用户正文 | 校验 + 直接改值 | **应收敛**：只可用于判空，不可直接改写正文 |
| `modules/cubebox/gateway.go` | `chunk.Delta` | 模型正文增量 | 过滤空白 delta | **必须取消**：会吞掉换行/空行语义 |
| `modules/cubebox/gateway.go` | `PromptItem.Content` | provider 输入正文 | 组装请求前裁边 | **应收敛**：需要区分“校验是否空”与“保留正文原值” |
| `modules/cubebox/gateway.go` | `request.BaseURL` / `Model` / `secretRef` | 配置/标识 | 归一化 | **保留** |
| `modules/cubebox/gateway.go` | SSE `line` / `data:` wrapper | 协议外壳 | 解析容错 | **原则上保留**，但不得影响 `delta.content` 本体 |

### 2.3 当前最高置信根因假设

最高置信根因是：

- 模型把正文结构拆成多个 delta；
- 其中某些 delta 只包含换行或空行；
- gateway 把这些 delta 当作“空内容”丢弃；
- 后续编号项文本继续追加，于是出现 `...等2)` 这种被压平的结果。

### 2.4 仍待确认的问题

1. 该具体会话中的 provider 原始 SSE 是否真的存在“newline-only delta”。
2. 是否存在“delta 不是纯换行，但被上游 `TrimSpace` 改写后变空”的情况。
3. 是否存在 `turn.agent_message.completed` 之前最后一段文本未 flush 的边角问题。
4. 历史回放中是否已有旧坏数据；若有，是否需要只修新数据还是补历史修复策略。

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

本计划因此不仅要修 newline-only delta，还要冻结 CubeBox 对 `TrimSpace` 的使用边界，防止同类问题在 prompt、summary、history replay、future renderer 中反复回流。

## 4. 设计决策

1. **选定方案**：把 `turn.agent_message.delta` 视为正文事实流；除非 provider 明确给出无效 chunk，否则不得基于 `TrimSpace(delta)==""` 丢弃任何 delta。
2. **不选方案**：不在前端用正则把 `1)`、`2)`、`3)` 强行重新切段，这会把显示修复伪装成语义修复。
3. **不选方案**：不新增 Markdown AST、HTML 渲染或富文本协议来掩盖流式正文事实损坏。
4. **不选方案**：不在历史回放层自动“猜测性补换行”，避免对 append-only 审计事实做不可解释改写。
5. **失败语义**：若 provider chunk 确认为协议无效，应继续按现有 `ErrProviderStreamInvalid` fail-closed；但“只含换行/空格”不属于协议无效。
6. **分层原则**：`TrimSpace` 只允许用于“标识类、配置类、协议壳层字段”；对“正文类字段”，最多允许“trim 后判空”，不得用 trim 后的值覆盖原文。

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
3. **应继续保留**
   - `conversation_id` / `provider_id` / `model_slug` / `base_url` / `secret_ref`
   - SSE 包裹层 `line` / `data:` 的协议解析 trim

## 5. 最小实现路径

### 5.1 Slice A：补齐调查证据

1. [ ] 增加针对 `newline-only delta` 的 gateway 单测夹具。
2. [ ] 增加针对“列表项被拆为 `文本 chunk + 换行 chunk + 下一项 chunk`”的端到端 reducer/gateway 回归测试。
3. [ ] 若条件允许，补抓一次真实 provider SSE 证据，证明问题不是前端幻觉。

### 5.2 Slice B：修复 gateway 过滤策略

1. [ ] 删除或收敛 `strings.TrimSpace(chunk.Delta) == "" => continue` 这一逻辑。
2. [ ] 保证 `chunk.Delta` 即使只包含 `\n`、`\n\n`、前导缩进，也能按原样写入 `turn.agent_message.delta`。
3. [ ] 明确 adapter/parser 哪一层负责“协议合法性”，哪一层不得碰正文空白语义。

### 5.3 Slice C：收敛 `TrimSpace` 使用边界

1. [ ] `req.Prompt` 改为“trim 后判空，但继续保留原始 prompt 进入 runtime/store”。
2. [ ] `PromptItem.Content` 改为“必要时用 trimmed 判断是否为空，但保留原值作为 provider content”。
3. [ ] 为“标识类字段可 trim，正文类字段不可改值”补纯函数/适配层测试，冻结未来口径。

### 5.4 Slice D：补齐页面与回放回归

1. [ ] 前端 reducer 测试中新增保留换行/空行/编号列表边界的断言。
2. [ ] 页面测试新增“含编号列表与空行”的消息显示断言，证明 `pre-wrap` 路径有效。
3. [ ] 历史恢复测试新增“坏结果不会被新代码继续制造”的断言。

### 5.5 Slice E：浏览器验收

1. [ ] 用真实页面复现“1/2/3 列表”回复。
2. [ ] 验证列表项之间存在稳定换行，不再出现 `...等2)` 贴连。
3. [ ] 验证“为什么没有选项2”这类追问场景下，上一条原文肉眼可见 `2)` 列表项独立成段。

## 6. Owner 边界

| 主题 | owner | 其他方职责 |
| --- | --- | --- |
| provider SSE 协议解析 | `DEV-PLAN-433` 对应 gateway/adapter owner | 只负责协议级解码，不得擅自丢正文空白 |
| 连续对话上下文主链 | `DEV-PLAN-438` | 不持有正文格式修复 |
| provider role 收敛 | `DEV-PLAN-438A` | 不持有换行保真问题 |
| 正文结构保真 | `DEV-PLAN-438B` | 持有 delta 空白语义、回放一致性与页面验收 |

## 7. 停止线

- 不得再把“纯空白 delta”默认视为无意义噪音。
- 不得把显示层正则修补当作根因修复。
- 不得为了这次问题引入 HTML reply、富文本消息体、第二正文字段或双链路 fallback。
- 不得修改已落库历史事件内容来“补出”不存在的换行；若需历史修复，必须另立明确计划。
- 不得继续把正文字段与标识字段混用同一套 `TrimSpace` 改值口径。

## 8. 验收标准

1. [ ] `turn.agent_message.delta` 对 `newline-only delta` 不再丢弃。
2. [ ] gateway / reducer / panel 自动化测试覆盖“换行 chunk 独立到达”的场景。
3. [ ] 浏览器中编号列表能稳定按行展示，不再出现 `...等2)` 粘连。
4. [ ] `prompt/content` 路径完成“trim 仅用于校验，不用于改写正文”的收敛。
5. [ ] 历史恢复后的新会话回复不再继续制造相同结构损坏。
6. [ ] 不引入第二套渲染协议、不新增 fallback、不改写旧事件事实。

## 9. 交付物

1. [ ] 文档：
   - `docs/dev-plans/438b-cubebox-streaming-newline-loss-investigation-and-remediation-plan.md`
   - `AGENTS.md`
2. [ ] 代码修复：
   - `modules/cubebox/gateway.go`
   - `modules/cubebox/gateway_test.go`
   - `internal/server/cubebox_api.go`
   - `apps/web/src/pages/cubebox/reducer.test.ts`
   - `apps/web/src/pages/cubebox/CubeBoxPanel.test.tsx`
3. [ ] 证据回填：
   - `docs/dev-records/DEV-PLAN-437-READINESS.md`

## 10. 关联文档

- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- `docs/dev-plans/438-cubebox-conversational-continuity-investigation-and-remediation-plan.md`
- `docs/dev-plans/438a-cubebox-provider-message-role-normalization-and-codex-summary-alignment-plan.md`
