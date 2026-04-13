# DEV-PLAN-263：LibreChat 真实大模型助手对话回复专线实施方案

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 规划中（2026-03-06 18:54 CST）

## 1. 背景与问题定义
- 263 的验收目标是单一且不可替代的：在 `http://localhost:8080/app/assistant/librechat` 输入自然语言后，**用户可见回复必须来自真实大模型生成**，且回复必须出现在官方聊天流内。
- 260/261/262 已改善对话闭环与渲染锚点，但它们的约束不能替代 263 目标本身；凡与 263 硬目标冲突的历史约束，一律以下述目标倒推约束为准。
- 263 不负责替代 `266` 的 UI / 通道前置门槛；若官方单通道、气泡内回写、无外挂容器、无官方原始错误体验未达成，则 263 不得单独宣布“用户体验通过”。
- 本计划明确采用“双阶段链路”：
  1. 系统先得到机器态执行结果（成功/失败/缺字段/候选/错误码）；
  2. 再由**真实大模型**基于系统 prompt 将该结果转写为最终用户回复；
  3. 最终文本进入聊天气泡。
- 当前主要缺口：
  1. [ ] 现有约束仍把模型命中写死为 `gpt-5.2`，不符合“真实大模型即可”的新口径。
  2. [ ] 当前业务文案仍存在前端本地拼接，尚未冻结为“由真实大模型输出用户文案”。
  3. [ ] 用户可见结果仍可能落到页面层 Alert/Notice，未冻结为聊天流唯一业务出口。
    4. [ ] 通过证据缺少“同一 turn 绑定”的硬规则，无法稳定审计“看见回复”和“模型命中”是否属于同轮次。
    5. [ ] 失败态存在技术错误码用户可见风险，未统一为大模型自然语言解释。

### 1.1 验收入口与前置依赖
- 当前用户验收入口冻结为：`http://localhost:8080/app/assistant/librechat`。
- `/assistant-ui` 仅可作为桥接/iframe/代理调试入口，**不得单独作为 263 通过依据**。
- 263 的任何“通过”都必须同时满足 `DEV-PLAN-266` 第 6.6 节“用户可见交互与体验变化”与第 7 节“验收标准（硬门槛）”。

## 2. 目标与非目标

### 2.1 目标（必须全部满足）
1. [ ] 用户在 `/app/assistant/librechat` 入口输入业务句子后，业务回执必须在官方聊天流同一 assistant 气泡体系中可见。
2. [ ] 用户可见业务回执文本（含成功与失败）必须由**真实大模型**生成，不允许前端/后端本地模板直接拼接后展示给用户。
3. [ ] 该轮回执必须满足模型命中约束：`reply_source=model`、`used_fallback=false`、`reply_model_name` 非空；任一不满足即该轮失败。
4. [ ] 通过判定必须具备同轮次证据三件套：页面全图、聊天局部图、同轮次 trace（`conversation_id`/`turn_id`/`reply_model_name`/`reply_source`/`used_fallback`）。
5. [ ] 用户界面不允许直出内部技术错误码；技术码仅用于日志与审计。
6. [ ] 263 的通过必须建立在 `266` 门槛之上：无官方原始发送、无官方 `Connection error`、无页面外挂回复容器、同轮唯一 assistant 回复。

### 2.2 非目标
1. [ ] 不改业务域 schema / 迁移 / sqlc。
2. [ ] 不引入 legacy 双链路、兼容快路径、页面外层兜底代替聊天回复。
3. [ ] 不修改 LibreChat 上游源码，仅在本仓代理注入、编排与验证层收敛。
4. [ ] 不将“指定某个固定模型名”作为通过条件；通过条件应是“真实大模型命中 + 无 fallback 冒充”。

## 3. 目标倒推硬约束（Contract Freeze）
1. [ ] **回复生成模型硬约束（H1）**：用户可见业务文案必须由真实大模型生成，并记录 `reply_model_name`、`reply_source=model`、`used_fallback=false`；未满足即失败。
2. [ ] **回复生成输入契约（H2）**：系统必须先构造机器态上下文，再 prompt 给真实大模型。上下文至少包含：
   - `conversation_id`、`turn_id`、`stage`、`outcome`（success/failure）；
   - `error_code`（失败时）、`missing_fields`、`candidates`、`next_action`；
   - `locale`（`zh`/`en`）与文风约束（简洁、可执行建议、禁止泄露内部实现）。
3. [ ] **文案来源硬约束（H3）**：聊天气泡中的业务文本只允许来自 `reply_nlg.text`（大模型输出）；禁止把 `format*Message` 或固定字符串作为最终业务回执直接展示。
4. [ ] **聊天流唯一业务出口（H4）**：业务成功/失败回执仅允许通过 `assistant.flow.dialog` 进入官方同一 transcript / assistant 气泡体系；`assistant.flow.notice`、页面 Alert、外挂容器不得作为业务通过依据。
5. [ ] **同轮次证据绑定（H5）**：每次“通过”必须同时提交：
   - `Screenshot-A`：`/app/assistant/librechat` 页面全图；
   - `Screenshot-B`：官方聊天区局部图（含该轮助手气泡）；
   - `Trace-C`：同一 `conversation_id + turn_id` 的后端记录，显式包含 `reply_model_name`、`reply_source`、`used_fallback` 与 `reply_prompt_version`。
6. [ ] **失败语义硬约束（H6）**：正常与报错场景都应先走大模型回复生成；若回复生成链路不可用，则该轮判失败且不计通过。
7. [ ] **技术细节隔离（H7）**：内部错误码仅进入日志/trace，不在终端用户可见文本中原样输出（例如 `ai_plan_schema_constrained_decode_failed`）。
8. [ ] **用户体验前置硬约束（H8）**：任一轮若出现官方原始发送漏网、官方 `Connection error`、页面外挂回复容器或同轮多份 assistant 回复，则该轮即使模型命中也判失败。
8. [ ] **输入基准句**（验收句）：
   - `在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过对话页面，调用相关能力完成部门的创建任务。`

## 4. 实施步骤（按硬约束落地）

### 4.1 M1：回复生成契约与 Prompt 冻结（对应 H1/H2/H3）
1. [ ] 定义 `assistant_reply_context` 结构与版本（建议 `v1`），作为真实大模型回复生成唯一输入。
2. [ ] 冻结回复生成 system prompt（明确：以用户可理解方式解释结果、禁止输出技术码、提供下一步建议）。
3. [ ] 引入 `reply_nlg` 结构（`text`、`kind`、`stage`、`reply_model_name`、`reply_prompt_version`、`reply_source`、`used_fallback`）。
4. [ ] 补充单测：输入同一上下文时输出结构可解码、字段完整、审计字段齐全。

### 4.2 M2：服务端双阶段编排落地（对应 H1/H3/H6/H7）
1. [ ] 在 create/confirm/commit 各阶段先得到机器态结果，再调用真实大模型生成用户回复。
2. [ ] 新增回复命中校验：`reply_source != model`、`used_fallback == true` 或 `reply_model_name` 为空时直接 fail-closed。
3. [ ] 错误场景（含 decode/timeout/state_invalid）统一进入“错误上下文 -> 大模型回复生成”链路。
4. [ ] 将 `reply_nlg` 结果随 turn 响应返回并写入可对账日志。

### 4.3 M3：前端渲染收敛（对应 H3/H4/H7）
1. [ ] `LibreChatPage`/`AssistantPage` 不再直接拼业务文案；仅渲染后端返回的 `reply_nlg.text`。
2. [ ] `assistant.flow.dialog` 只发送 `reply_nlg`，保持官方聊天流唯一业务出口。
3. [ ] `Alert/Notice` 仅保留连接态与技术态，不再用于业务结果展示。
4. [ ] 补充 Web 测试：断言业务文案来源为 `reply_nlg.text`，且技术错误码不会直接透出。
5. [ ] 补充 Web/E2E 断言：无官方 `Connection error`、无页面外挂回复容器、同轮仅一份 assistant 回复。

### 4.4 M4：真实验收与证据收敛（对应 H5）
1. [ ] 新增/更新真实 E2E，禁止以 mock `/internal/assistant/**` 结果充当通过证据。
2. [ ] 每轮落盘三件套证据，并绑定同一 `conversation_id + turn_id`。
3. [ ] 将执行记录写入 `docs/archive/dev-records/dev-plan-263-execution-log.md`。
4. [ ] E2E 必须从 `/app/assistant/librechat` 真实入口启动，并同时断言 `266` 的共通 stopline。

## 5. 验收标准（硬门槛）
1. [ ] 正常与错误两类场景都满足“先得到机器态结果，再经真实大模型转写并展示”的单链路。
2. [ ] 业务回执只在聊天流气泡中判定通过，页面外层提示不计入通过。
3. [ ] 通过证据必须显示：`reply_source=model`、`used_fallback=false`、`reply_model_name` 非空。
4. [ ] 任意一轮出现本地模板直出、技术码直出或 fallback 冒充模型回复，整轮判失败。
5. [ ] 任意一轮若未满足 `266` 的单通道、气泡内回写、无外挂容器、无官方原始错误体验门槛，则该轮直接判失败。

## 6. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- 263 最低验证集：
1. [ ] `go test ./internal/server -run 'TestAssistantReplyNLGPipeline|TestAssistantReplyModelSourceGate|TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase' -count=1`
2. [ ] `pnpm --dir apps/web test -- src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/AssistantPage.test.tsx`
3. [ ] `pnpm --dir e2e exec playwright test tests/tp263-librechat-gpt52-dialog-response.spec.js`，并以 `/app/assistant/librechat` 入口断言 `266` 的共通 stopline。
4. [ ] `make check doc`

## 7. 交付物
1. [ ] 计划文档：`docs/archive/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
2. [ ] 执行日志：`docs/archive/dev-records/dev-plan-263-execution-log.md`
3. [ ] 真实证据目录：`docs/archive/dev-records/assets/dev-plan-263/`
4. [ ] 相关后端 / Web / E2E 测试补强。

## 8. 关联文档
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/261-librechat-assistant-conversation-failure-investigation-and-remediation-plan.md`
- `docs/archive/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
- `docs/archive/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`
- `docs/archive/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
