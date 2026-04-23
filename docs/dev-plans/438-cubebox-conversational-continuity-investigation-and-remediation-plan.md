# DEV-PLAN-438：CubeBox 连贯对话失效调查与修复方案

**状态**: 实施中（2026-04-23 10:35 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`（命中用户可见主链、会话上下文、provider 输入语义、恢复/压缩协同与 430/434 契约兑现）
- **范围一句话**：专门记录 CubeBox 在真实页面验证中暴露出的“对话不连贯/上一轮上下文未被模型消费”问题，冻结调查结论、根因、owner 边界、修复步骤与验收口径，确保 `DEV-PLAN-430` 定义的连续对话能力真正落到 provider 主链，而不是只停留在会话存储、恢复和 compaction 辅助层。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`、`docs/dev-plans/433a-cubebox-real-provider-browser-validation-findings-and-remediation-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`、`modules/cubebox`、`internal/server`、`apps/web/src/pages/cubebox`
- **关联计划/标准**：`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-432`、`DEV-PLAN-433A`、`DEV-PLAN-434`、`DEV-PLAN-437A`、`DEV-PLAN-438A`、`DEV-PLAN-438B`
- **用户入口/触点**：登录页、主应用壳层右侧 CubeBox 抽屉、历史恢复、新建会话、发送消息、manual `/compact`、pre-turn auto compact、`/internal/cubebox/turns:stream`
- **证据记录 SSOT**：本计划的实施与页面复验证据统一回填 `docs/dev-records/DEV-PLAN-437-READINESS.md` 的 CubeBox Phase 段；本文件只冻结调查结论、修复步骤与验收口径，不承载零散运行日志。

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理“连贯对话没有真正进入 provider 输入”的调查与修复；不重写完整 UI 协议，不重写完整会话持久化，不额外引入 remote compaction、summary model、route alias、fallback/failover。
2. **不变量**：连续对话必须通过“append-only 原始事件 + 每轮重建 prompt view + canonical context reinjection + provider 主链消费同一份 prompt view”达成；禁止再退回为“只把当前轮 prompt 直接发给模型”的假连续对话。
3. **可解释**：reviewer 必须能在 5 分钟内说明“会话恢复为什么存在”“compaction 为什么存在”“它们为什么目前没真正生效到模型输入”“修复后 provider 具体会消费什么”。

## 1. 背景

`DEV-PLAN-430` 已把 CubeBox 的连续对话能力定义为首期核心目标之一：会话必须可恢复、上下文必须有 token budget、历史必须可压缩、每轮必须按固定顺序组装 prompt，而不是无限堆叠聊天记录。

`DEV-PLAN-434` 也已按 Codex 主参考实现了首期 compaction 纯函数、prompt view builder、recent message 保留、canonical context reinjection、manual compact API 与 pre-turn auto compact。

但在 2026-04-22 的真实页面验证中，出现了以下用户可见现象：

- 用户先输入 `a`
- 模型回答 “Hi—what would you like to do with `a`?...”
- 用户再输入 “我是。回答你前面的。问题”
- 模型回答 “我还看不到你说的前面的那个问题具体是哪一句”

这说明：

1. 会话主链、恢复链、抽屉 UI、streaming、history list、compact API 都已经存在。
2. 但“前文连续性”并没有真正进入模型输入。
3. 当前实现更接近“有会话存储的单轮问答”，而不是 `430` 定义的连续对话内核。

## 2. 已完成调查（事实证据）

### 2.1 页面级现象

- 页面登录、抽屉入口、会话新建、消息发送、历史恢复、manual compact API 均可通过真实浏览器触发。
- 真实页面中可看到旧会话被恢复，说明 `432` 的会话恢复链路本身存在。
- 但连续追问时，模型无法理解“前面的那个问题”，说明恢复出来的会话历史并未成为下一轮模型输入的一部分。

### 2.2 网络级现象

- `POST /internal/cubebox/conversations => 201`
- `POST /internal/cubebox/turns:stream => 200`
- `POST /internal/cubebox/conversations/{id}:compact => 200`
- `GET /internal/cubebox/conversations/{id} => 200`

这说明链路问题不在“接口不存在”或“页面没触发”，而在服务端实际提交给 provider 的请求内容。

### 2.3 代码级事实

#### 事实 A：`434` 已能生成 prompt view

`modules/cubebox/compaction.go` 中的 `BuildPromptViewWithCompaction(...)` 已经具备：

- 读取历史 timeline
- 判断是否 compact
- 生成 `summary_text`
- reinject canonical context
- 保留 recent user/assistant timeline
- 拼出 `PromptView []PromptItem`

这说明连续对话所需的“有效上下文视图”构造器已经存在。

#### 事实 B：`store.CompactConversation(...)` 已返回 `PromptView`

`modules/cubebox/store.go` 的 `CompactConversation(...)` 返回：

- `Conversation`
- `PromptView`
- `NextSequence`
- 可选 `Event`

这说明 compaction 结果并不是只存在于 UI，而是服务层可直接消费。

#### 事实 C：gateway 在 pre-turn 确实会调用 compact

`modules/cubebox/gateway.go` 在 `sequence > 1` 时会先执行：

- `store.CompactConversation(..., "pre_turn_auto")`

这说明发送前自动压缩已经接到了 turn 主链。

#### 事实 D：provider 真实调用仍只用当前轮 `turn.Prompt`

当前最关键的事实在这里：

- `ProviderChatRequest` 只有 `Input string`
- `GatewayService.StreamTurn(...)` 在真正调用 provider 时传的是：
  - `Input: turn.Prompt`

也就是说，虽然 compaction / replay / prompt view builder 已存在，但真正喂给 provider 的仍只是“本轮用户输入”，而不是“重建后的会话上下文视图”。

这就是本次问题的直接根因。

## 3. 根因结论

### 3.1 一级根因：连续对话设计与 provider 主链脱节

当前系统已经实现了三分之二：

- 会话可持久化与恢复
- 历史可压缩与重建 prompt view

但缺失最后一刀：

- provider 主链没有消费 `PromptView`

因此现状是：

- “连续对话的设计”存在
- “连续对话的存储形态”存在
- “连续对话的压缩形态”存在
- 但“连续对话的模型输入兑现”不存在

### 3.2 二级根因：provider 请求模型过于单薄

当前 `ProviderChatRequest` 只接受一个 `Input string`，这天然鼓励 gateway 直接发送当前轮 prompt，而不是发送结构化上下文。

这与 `430` 的 prompt 组装顺序和 `434` 的 `PromptView` 设计不匹配。

### 3.3 三级根因：页面通过了“链路存在性验证”，但没有通过“语义连贯性验证”

此前页面验证确认了：

- 可以登录
- 可以打开抽屉
- 可以新建会话
- 可以恢复历史
- 可以调用 compact API

但还没有把下面这个场景纳入阻断验收：

1. 先问一个模糊问题
2. 再用省略主语/代词/“前面那个问题”继续追问
3. 模型必须能把后一轮绑定到前一轮上下文

这导致“有会话”被误判成了“会连续对话”。

## 4. 与 430/434 契约的偏离点

### 4.1 偏离 `430` 的 prompt 组装要求

`430` 要求每轮请求都按固定顺序组装：

1. 系统基线指令
2. 模块上下文
3. 历史压缩摘要
4. 结构化状态对象
5. 工具输出压缩结果
6. 最近 3-5 轮原文
7. 当前用户输入

当前 provider 主链实际只用了第 7 项。

### 4.2 偏离 `434` 的 prompt view replacement 设计

`434` 的核心不是“生成 compact event”，而是“生成给模型使用的有效上下文视图”。

当前实现只复用了 `434` 的：

- summary prefix
- compact event
- no-op compaction
- recent message trimming

但没有复用最关键的：

- prompt view replacement 进入 provider 调用

### 4.3 偏离 Codex 主参考的连贯对话实现方式

Codex 的连贯对话不是“保存历史即可”，而是：

- 保留 history
- 必要时 compact
- reinject authoritative context
- 每轮用 replacement history / prompt view 重新采样

当前 CubeBox 停在了第 2、3 步，没有走到第 4 步。

## 5. 修复方案（冻结执行清单）

### 5.0 与 Codex 对齐差距矩阵

| 能力点 | Codex 做法 | CubeBox 修复前 | 当前收敛方案 |
| --- | --- | --- | --- |
| history 与 prompt view 分离 | `ContextManager` 持有 history，`for_prompt()` 输出采样视图 | 已有 `CanonicalEvent` / `PromptView` 分离 | 保持现状，不新增第二套上下文模型 |
| compact 生成 replacement history | compact turn 后用 replacement history 驱动下一轮 | 已能生成 `PromptView`，但未进入 provider | 由 gateway 直接消费 `PromptView` |
| canonical context reinjection | compact 后重新插入当前 authoritative context | 已实现 | 继续沿用 `434` builder |
| 下一轮真正吃到 replacement view | `sess.replace_compacted_history(...)` 写回 session，下一轮按新视图采样 | 未实现，provider 仍只吃 `turn.Prompt` | provider 主链改为优先消费结构化 `messages` |
| handoff 型 compact prompt | 用 compact prompt 强调进展/约束/下一步/引用 | 已有最小 handoff 摘要 | 继续保持最小摘要，不升级为重型模板 |

### 5.1 设计决策

1. **选定方案**：provider 主链改为消费 `PromptView`，而不是只消费 `turn.Prompt`。
2. **不选方案**：不通过把历史直接拼成一个长字符串临时塞进 `turn.Prompt` 来“假装修复”。
3. **不选方案**：不新增第二套“provider 专用 DTO”绕开 `434` 的 `PromptView` 结果。
4. **不选方案**：不把连续性修复下沉到前端，不让前端自行拼 prompt。
5. **对齐口径**：优先对齐 Codex 的“history/replacement view 分离 + 下一轮真实采样替换”思路；由于 CubeBox 需要 append-only 审计事实源，因此只替换 provider 采样视图，不替换数据库原始事件。
6. **子问题拆分**：provider 输入只接受标准 role 集合这一收敛问题，由 `DEV-PLAN-438A` 单独持有，避免在 `438` 主计划中继续膨胀实现细节。
7. **Codex 对齐补充**：当前轮输入是否进入 provider，必须由权威 history / prompt view 语义决定，不得按“与历史最后一条文本相同”做文本级去重；最近用户消息 guardrail 也必须保持单一 token 语义。

### 5.2 最小实现路径

#### Slice A：补 provider 输入契约

1. [x] 扩展 `ProviderChatRequest`
   - 允许承载结构化 `PromptView`，至少支持有序消息列表，而不再只剩单个 `Input string`。
2. [x] 明确字段 owner
   - `PromptView` 的生成仍由 `434` owner 的 context/compaction 层负责。
   - `gateway` 只消费 `PromptView` 并把它映射到 provider adapter 所需格式。

#### Slice B：在 gateway 中真正接线

1. [x] 在 `GatewayService.StreamTurn(...)` 中，把 pre-turn auto compact 的返回值接成“本轮实际采样输入”。
2. [x] 若未发生 compact，也从当前 canonical context + 当前用户输入构造最小 `PromptView`，不再退回单独发 `turn.Prompt`。
3. [x] `turn.user_message.accepted` 仍写 append-only event log；但 provider 采样输入改为：
   - canonical context
   - existing compact summary
   - recent history
   - current user input

#### Slice C：adapter 层映射

1. [x] OpenAI-compatible adapter 支持把 `PromptView` 映射为 provider 请求 `messages` 形状。
2. [x] current active model、provider_id、provider_type、runtime 继续沿用 `433/437A` 的 lifecycle 字段，不新增第二套运行时语义。

#### Slice D：语义验收与页面回归

1. [ ] 增加页面级连续对话验收样本：
   - 第一轮：`a`
   - 第二轮：`回答你前面的那个问题`
   - 期望：不得回复“看不到前面的问题”这类失忆性回答。
2. [ ] 增加 compact 后连续追问样本：
   - 先形成足够历史
   - manual compact 或 pre-turn auto compact
   - 再继续追问
   - 期望：压缩后仍能理解前文指代。
3. [ ] 增加“相同文本重试”样本：
   - 上一轮停在用户消息后失败或被中断
   - 用户再次发送相同文本
   - 期望：当前轮输入仍进入 provider，不因文本相等被吞掉。

## 6. Owner 边界

| 主题 | owner | 其他方职责 |
| --- | --- | --- |
| 会话恢复 / replay 输出 shape | `432` | 输出继续对齐 `437A`，不单独发明 provider 输入 DTO |
| prompt view 语义与 compaction 规则 | `434` | 持有 replacement history、canonical reinjection、recent window 规则 |
| provider 主链接线 | `433/运行时` | 负责把 `PromptView` 真正送入 provider adapter |
| provider 输入角色收敛 | `438A` | 负责 `summary` 等仓内语义到 provider 标准 role 的最小映射 |
| UI 抽屉 / composer 命令入口 | `431` | 只消费已生效的连续对话能力，不负责前端拼 prompt |

## 7. 停止线

- 不得再把“会话已恢复”当成“连续对话已实现”。
- 不得用前端拼接历史字符串替代服务端 `PromptView`。
- 不得为 provider 输入引入一套绕开 `434` 的第二上下文模型。
- 不得通过 prompt 工程兜底文案掩盖“前文根本没进模型”的实现缺口。
- 不得因为 compact API 已可调用，就把 434 判定为“连贯对话已实现”。

## 8. 验收标准

1. [x] provider 主链不再直接使用裸 `turn.Prompt` 作为唯一输入。
2. [x] `PromptView` 在真实 provider 路径被实际消费。
3. [ ] 页面连续追问场景不再出现“看不到前面的问题/不知道你指哪一句”的失忆性回答。
4. [ ] compact 前后都能维持指代连续性。
5. [x] 会话恢复、manual compact、pre-turn auto compact 与真实 provider 路径共用同一套连续对话输入模型。
6. [ ] 自动化至少覆盖：
   - gateway 单测
   - provider adapter 映射测试
   - 页面级连续追问回归测试
   - 相同文本重试不吞当前轮输入的回归测试

## 9. 交付物

1. [ ] 代码修复：
   - `modules/cubebox/gateway.go`
   - `modules/cubebox/compaction.go` 或其配套 context builder
   - `modules/cubebox` provider adapter 相关文件
   - 必要的 `internal/server` / `apps/web` 测试
2. [ ] 自动化验证：
   - `go test ./modules/cubebox ./internal/server`
   - `pnpm --dir apps/web test -- --run ...`（按实际命中文件补充）
3. [ ] 页面复验证据回填到 `docs/dev-records/DEV-PLAN-437-READINESS.md`

## 10. 关联文档

- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- `docs/dev-plans/433a-cubebox-real-provider-browser-validation-findings-and-remediation-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
