# DEV-PLAN-438A：CubeBox provider 输入角色收敛与 Codex 摘要语义对齐方案

**状态**: 进行中（2026-04-23 16:35 CST；补充 Codex 对 token guardrail 与重试输入语义的对照结论）

## 0. 适用范围与评审分级

- **评审分级**：`T2`（命中真实 provider 主链、连续对话语义、`DEV-PLAN-438` 的连续性兑现路径与 `DEV-PLAN-437A` 的 prompt view 契约落地）
- **范围一句话**：只修复“CubeBox 内部 `PromptView` 可包含 `summary` 语义，但真实 provider 输入不得出现非法 role”这一处边界问题，并按 Codex 开源资源的最小做法收敛为“标准 role + 摘要前缀”。
- **关联模块/目录**：`modules/cubebox/compaction.go`、`modules/cubebox/gateway.go`、`modules/cubebox/gateway_test.go`
- **关联计划/标准**：`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-430`、`DEV-PLAN-434`、`DEV-PLAN-437A`、`DEV-PLAN-438`
- **证据记录 SSOT**：实现与验证证据回填 `docs/dev-records/DEV-PLAN-437-READINESS.md`；本文件只冻结边界、取舍与验收口径。

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理 provider 输入角色归一化，不重写 compaction 内核，不改数据库事件模型，不新增第二套上下文 DTO。
2. **不变量**：`PromptView` 可以保留仓内语义；但发给 provider 的消息 role 必须收敛为 provider 支持的标准集合，摘要语义通过稳定前缀保留，不得靠非法 role 透传。
3. **可解释**：reviewer 必须能在 5 分钟内说明“为什么仓内可有 `summary` 语义”“为什么 provider 侧不能直接看到它”“修复后摘要语义如何继续保留”。

## 1. 背景

`DEV-PLAN-438` 已经把连续对话主问题收敛为“provider 主链必须真正消费 `PromptView`”。当前实现已完成这一步接线，但在接线后暴露出一个新的、更加具体的边界缺口：

- `PromptView` 的历史重建路径可能包含 `Role: "summary"` 的内部语义项。
- OpenAI-compatible chat completions 通常只接受标准 role 集合。
- 若把内部 `summary` role 原样透传，连续对话链路会在真实 provider 请求处失效。

这不是新的上下文模型问题，而是“内部表达”和“外部协议”之间缺少最后一层最小收敛。

## 2. Codex 开源资源调查结论

本次仅采用与当前问题直接相关的一手事实，不扩成大而全复用矩阵。

### 2.1 结论 A：Codex 不把摘要语义作为非法 role 直接发给模型

根据 `openai/codex` 当前 `main` 分支：

- `codex-rs/core/src/compact.rs`
- `codex-rs/core/templates/compact/summary_prefix.md`

Codex 本地 compaction 路径会：

1. 用稳定 `SUMMARY_PREFIX` 识别摘要消息。
2. 生成 compacted history 时，把摘要放进标准消息结构，而不是自定义 `summary` role 透传给模型。

### 2.2 结论 B：Codex 用“标准消息 + 前缀”或协议内专用 item 保留摘要语义

根据 `codex-rs/core/src/compact_remote.rs`，Codex 远端 compaction 路径也不会把任意内部角色原样透传给 provider；它保留的是协议内受控 item，而不是开放式自定义 role。

### 2.3 对本仓的直接启发

对 CubeBox 当前 chat-completions 路径，最小且贴近 Codex 的收敛方式是：

- 内部 `PromptView` 继续允许存在 `summary` 语义，避免把 compaction 纯函数与 provider 协议硬绑定。
- 在 provider 边界把 `summary` 映射为标准 `user` 消息，并保留 `[[summary]] ` 前缀。

### 2.4 结论 C：Codex 的最近用户消息 guardrail 以 token 截断为单一语义，不做字符级二次放行

根据 `codex-rs/core/src/compact.rs`：

- 先用近似 token 估算计算当前消息是否还能落入剩余额度；
- 超限时直接按 token 额度截断；
- 不再追加“字符长度未超过某阈值就原样放行”的第二判断。

对本仓的直接启发：

- `PromptView` 中最近用户消息的 guardrail 应保持“是否超限”和“如何截断”共用同一套 token 语义；
- 不应先判定超限、再因字符长度边界重新放行。

### 2.5 结论 D：Codex 不靠“最后一条文本相等”去重当前轮用户输入

根据 `codex-rs/core/src/compact.rs` 的当前轮输入装配路径：

- 当前输入先进入权威 history；
- 后续请求/重试统一从 history 采样生成 `for_prompt()` 视图；
- 当前可见开源实现中，没有“若最后一条 user 文本相等就跳过当前轮输入”的文本级 dedupe 规则。

对本仓的直接启发：

- 当前轮用户输入是否进入 provider，必须由“当前轮是否已写入权威输入模型”决定，而不是由“与历史最后一条文本是否相等”决定；
- 相同问题的失败重试、澄清式重复提问、用户有意重复强调，都不应因为文本相等而被吞掉。

## 3. 问题定义

### 3.1 当前风险

当前实现把 `PromptView` 直接映射为 provider `messages`。一旦 `PromptView` 中出现：

- `system`
- `assistant`
- `user`
- `summary`

其中 `summary` 若未被收敛，将形成：

- provider 请求 4xx / 拒绝
- 连续对话在 compact 后重新失忆
- 测试看似通过，但真实请求与测试夹具不一致

### 3.2 根因

根因不是 compaction 算法错误，而是 provider 边界缺少最小消息角色收敛规则。

## 4. 设计决策

1. **选定方案**：仅在 `OpenAICompatibleAdapter` 的消息组装边界增加最小归一化，把内部 `summary` 统一映射为标准 `user` 消息，并保留 `[[summary]] ` 前缀。
2. **不选方案**：不在 `compaction.go` 内部直接取消 `summary` 语义，避免把仓内上下文语义与外部 provider 协议混为一层。
3. **不选方案**：不新增 `ProviderMessage` / `PromptEnvelope` / 第二上下文 DTO。
4. **不选方案**：不增加 provider capability、fallback、feature flag 或多路兼容分支。
5. **失败语义**：除 `summary` 外，如出现未知 role，应 fail-closed 返回 `ErrProviderConfigInvalid`，并沿用现有 provider 请求失败路径对外终止当前 turn；不得静默透传，也不得为此新增第二套错误码体系。
6. **Codex 对齐补充**：最近用户消息 guardrail 采用单一 token 语义；当前轮输入不得因“与历史最后一条文本相同”而被去重吞掉。

## 5. 最小实现路径

### 5.1 Slice A：冻结 provider 可见消息不变量

1. [ ] 明确 provider 可见 role 仅允许标准集合。
2. [ ] 明确 `summary` 只属于仓内 `PromptView` 语义，不属于 provider 外部协议。

### 5.2 Slice B：在 adapter 边界做单点收敛

1. [ ] 仅在 `OpenAICompatibleAdapter` 邻近位置增加纯函数，将 `[]PromptItem` 收敛为 provider request `messages`。
2. [ ] `summary` → `user`
3. [ ] 若内容未带 `[[summary]] ` 前缀，则在收敛时补齐。
4. [ ] 空 role / 空内容继续过滤。
5. [ ] 未知 role 直接返回 `ErrProviderConfigInvalid`，不透传。

### 5.3 Slice C：与 Codex 保持一致的输入与截断语义

1. [ ] 最近用户消息 guardrail 统一为单一 token 语义，不再使用“超限后再按 rune 长度放行”的双重判断。
2. [ ] 当前轮用户输入进入 provider prompt 时，不得按“与历史最后一条 user 文本相等”做内容去重。
3. [ ] 重试、失败后同文重发、强调式重复提问都必须把当前轮输入显式带入 provider 采样视图。

### 5.4 Slice D：测试回归

1. [ ] 补齐“真实 `summary` role 输入”测试，而不是只用 `system` 摘要夹具代替。
2. [ ] 断言 provider 最终请求只含标准 role。
3. [ ] 断言摘要语义仍保留 `[[summary]] ` 前缀。
4. [ ] 断言普通 `system/user/assistant` 不回归。
5. [ ] 断言未知 role 触发 `ErrProviderConfigInvalid`，且不会落出非法 request body。
6. [ ] 补齐最近用户消息 token guardrail 的边界值测试，防止 `tokenLimit * 4` 一类边界重新放行。
7. [ ] 补齐“历史最后一条 user 与当前输入文本相同”场景测试，断言当前轮输入不会被吞掉。

## 6. Owner 边界

| 主题 | owner | 其他方职责 |
| --- | --- | --- |
| compaction 纯函数与 `PromptView` 语义 | `DEV-PLAN-434` | 继续持有 history / summary / canonical reinjection 规则 |
| provider 主链连续性兑现 | `DEV-PLAN-438` | 继续持有“provider 必须真实消费 prompt view”的主问题 |
| provider 输入角色收敛 | `DEV-PLAN-438A` | 只持有 `PromptView -> provider messages` 的最小归一化边界 |

## 7. 停止线

- 不得把这次修复扩张成新的 prompt 协议层。
- 不得为未知未来 provider 预留 capability/fallback/config 链。
- 不得通过前端拼接、字符串拼 prompt 或改写原始事件来绕过 role 收敛问题。
- 不得继续使用“测试夹具里把摘要写成 `system`”来替代真实路径验证。
- 不得继续以文本相等为由跳过当前轮用户输入。
- 不得把最近用户消息 token guardrail 写成“token 判断 + rune 长度二次放行”的双重口径。

## 8. 验收标准

1. [ ] provider 请求体不再出现非法 `summary` role。
2. [ ] compact 后连续追问仍能携带摘要语义。
3. [ ] `summary` 语义通过 `[[summary]] ` 前缀保留，而不是靠自定义 role 保留。
4. [ ] 除 `summary` 外的未知 role 会以 `ErrProviderConfigInvalid` fail-closed。
5. [ ] 最近用户消息 token guardrail 在边界值上仍成立，不会重新放行超限正文。
6. [ ] 相同文本的失败重试或重复提问不会吞掉当前轮输入。
7. [ ] 不新增第二套上下文模型、feature flag 或 fallback 路径。

## 9. 交付物

1. [ ] 代码修复：
   - `modules/cubebox/gateway.go`（`OpenAICompatibleAdapter` 邻近消息收敛逻辑）
   - `modules/cubebox/gateway_test.go`
2. [ ] 文档对齐：
   - 本文件
   - `docs/dev-plans/438-cubebox-conversational-continuity-investigation-and-remediation-plan.md`
   - `AGENTS.md`
