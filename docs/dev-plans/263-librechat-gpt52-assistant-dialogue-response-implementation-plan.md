# DEV-PLAN-263：LibreChat GPT-5.2 助手对话回复专线实施方案

**状态**: 规划中（2026-03-06 18:05 CST）

## 1. 背景与问题定义
- 263 的验收目标是单一且不可替代的：在 `/app/assistant/librechat` 输入自然语言后，必须由 **GPT-5.2** 生成该轮回复，且用户可见回复必须出现在 iframe 聊天流内。
- 260/261/262 已改善对话闭环与渲染锚点，但它们的约束不能替代 263 目标本身；凡与 263 硬目标冲突的历史约束，一律以下述目标倒推约束为准。
- 本计划明确采用“双阶段链路”：
  1. 系统先得到机器态执行结果（成功/失败/缺字段/候选/错误码）；
  2. 再由 GPT-5.2 基于系统 prompt 将该结果转写为最终用户回复；
  3. 最终文本进入聊天气泡。
- 当前主要缺口：
  1. [ ] 模型命中仍是“优先级尝试”语义，缺少“非 GPT-5.2 直接判失败”的 stopline。
  2. [ ] 当前业务文案仍存在前端本地拼接，尚未冻结为“由 GPT-5.2 输出用户文案”。
  3. [ ] 用户可见结果仍可能落到页面层 Alert/Notice，未冻结为聊天流唯一业务出口。
  4. [ ] 通过证据缺少“同一 turn 绑定”的硬规则，无法稳定审计“看见回复”和“模型命中”是否属于同轮次。
  5. [ ] 失败态存在技术错误码用户可见风险，未统一为 GPT-5.2 自然语言解释。

## 2. 目标与非目标

### 2.1 目标（必须全部满足）
1. [ ] 用户在 `/app/assistant/librechat` 输入业务句子后，业务回执必须在聊天流气泡中可见。
2. [ ] 用户可见业务回执文本（含成功与失败）必须由 GPT-5.2 生成，不允许前端/后端本地模板直接拼接后展示给用户。
3. [ ] 该轮回执必须满足模型命中约束：`reply_model_name=gpt-5.2`；未命中即该轮失败。
4. [ ] 通过判定必须具备同轮次证据三件套：页面全图、聊天局部图、同轮次 trace（`conversation_id`/`turn_id`/`reply_model_name`）。
5. [ ] 用户界面不允许直出内部技术错误码；技术码仅用于日志与审计。

### 2.2 非目标
1. [ ] 不改业务域 schema / 迁移 / sqlc。
2. [ ] 不引入 legacy 双链路、兼容快路径、页面外层兜底代替聊天回复。
3. [ ] 不修改 LibreChat 上游源码，仅在本仓代理注入、编排与验证层收敛。

## 3. 目标倒推硬约束（Contract Freeze）
1. [ ] **回复生成模型硬约束（H1）**：用户可见业务文案必须由 GPT-5.2 生成并记录 `reply_model_name=gpt-5.2`；未命中即失败（建议错误码：`ai_reply_model_target_mismatch`）。
2. [ ] **回复生成输入契约（H2）**：系统必须先构造机器态上下文，再 prompt 给 GPT-5.2。上下文至少包含：
   - `conversation_id`、`turn_id`、`stage`、`outcome`（success/failure）；
   - `error_code`（失败时）、`missing_fields`、`candidates`、`next_action`；
   - `locale`（`zh`/`en`）与文风约束（简洁、可执行建议、禁止泄露内部实现）。
3. [ ] **文案来源硬约束（H3）**：聊天气泡中的业务文本只允许来自 `reply_nlg.text`（GPT-5.2 输出）；禁止把 `format*Message` 或固定字符串作为最终业务回执直接展示。
4. [ ] **聊天流唯一业务出口（H4）**：业务成功/失败回执仅允许通过 `assistant.flow.dialog` 进入 transcript；`assistant.flow.notice` 与页面 Alert 不得作为业务通过依据。
5. [ ] **同轮次证据绑定（H5）**：每次“通过”必须同时提交：
   - `Screenshot-A`：`/app/assistant/librechat` 页面全图；
   - `Screenshot-B`：iframe 聊天区局部图（含该轮助手气泡）；
   - `Trace-C`：同一 `conversation_id + turn_id` 的后端记录，显式包含 `reply_model_name=gpt-5.2` 与 `reply_prompt_version`。
6. [ ] **失败语义硬约束（H6）**：正常与报错场景都应先走 GPT-5.2 回复生成；若回复生成链路不可用，则该轮判失败且不计通过。
7. [ ] **技术细节隔离（H7）**：内部错误码仅进入日志/trace，不在终端用户可见文本中原样输出（例如 `ai_plan_schema_constrained_decode_failed`）。
8. [ ] **输入基准句**（验收句）：
   - `在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。`

## 4. 实施步骤（按硬约束落地）

### 4.1 M1：回复生成契约与 Prompt 冻结（对应 H1/H2/H3）
1. [ ] 定义 `assistant_reply_context` 结构与版本（建议 `v1`），作为 GPT-5.2 回复生成唯一输入。
2. [ ] 冻结回复生成 system prompt（明确：以用户可理解方式解释结果、禁止输出技术码、提供下一步建议）。
3. [ ] 引入 `reply_nlg` 结构（`text`、`kind`、`stage`、`reply_model_name`、`reply_prompt_version`）。
4. [ ] 补充单测：输入同一上下文时输出结构可解码、字段完整、审计字段齐全。

### 4.2 M2：服务端双阶段编排落地（对应 H1/H3/H6/H7）
1. [ ] 在 create/confirm/commit 各阶段先得到机器态结果，再调用 GPT-5.2 生成用户回复。
2. [ ] 新增回复模型命中校验：`reply_model_name != gpt-5.2` 直接 fail-closed。
3. [ ] 错误场景（含 decode/timeout/state_invalid）统一进入“错误上下文 -> GPT-5.2 回复生成”链路。
4. [ ] 将 `reply_nlg` 结果随 turn 响应返回并写入可对账日志。

### 4.3 M3：前端渲染收敛（对应 H3/H4/H7）
1. [ ] `LibreChatPage`/`AssistantPage` 不再直接拼业务文案；仅渲染后端返回的 `reply_nlg.text`。
2. [ ] `assistant.flow.dialog` 只发送 `reply_nlg`，保持聊天流唯一业务出口。
3. [ ] `Alert/Notice` 仅保留连接态与技术态，不再用于业务结果展示。
4. [ ] 补充 Web 测试：断言业务文案来源为 `reply_nlg`，并验证技术错误码未直出。

### 4.4 M4：证据自动化与 E2E（对应 H5）
1. [ ] 新增 `tp263` 实时用例（非 mock），流程包括：登录 app、登录 iframe、发送基准句、读取同轮次 turn、抓取双截图。
2. [ ] 用例硬断言：
   - 聊天 transcript 内存在该轮助手回复；
   - 回执来源字段满足 `reply_model_name=gpt-5.2`；
   - 页面无技术错误码直出。
3. [ ] 证据文件落盘到 `docs/dev-records/dev-plan-263-execution-log.md`，并按轮次编号可回放。

### 4.5 M5：回归与封板
1. [ ] 回归 `tp260` 与 `tp262`，确认对话状态机和聊天锚点无回归。
2. [ ] 文档改为“已完成”前，逐条核对 H1~H7 证据完备。

## 5. 验收标准（硬门槛）
1. [ ] 基准句输入后，聊天流内出现助手业务回复（非页面外层替代提示）。
2. [ ] 该轮用户可见业务文案可追溯到 `reply_nlg`，且 `reply_model_name=gpt-5.2`。
3. [ ] 成功与失败路径均验证“机器态结果 -> GPT-5.2 转写 -> 聊天气泡展示”。
4. [ ] 双截图与 Trace-C 三件套齐全，且 `conversation_id/turn_id` 一致可对账。
5. [ ] 连续验证至少 3 次，逐次记录成功/失败与失败原因。
6. [ ] 用户界面不出现内部错误码原文。

## 6. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- 263 最低验证集：
  1. [ ] `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase|TestAssistantReplyNLGPipeline|TestAssistantReplyModelTargetGate" -count=1`
  2. [ ] `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js tests/tp262-librechat-dialog-anchor.spec.js`
  3. [ ] `pnpm --dir e2e exec playwright test tests/tp263-librechat-gpt52-dialog-response.spec.js`
  4. [ ] `make check doc`

## 7. 交付物
1. [ ] 计划文档：`docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
2. [ ] 执行日志：`docs/dev-records/dev-plan-263-execution-log.md`
3. [ ] 代码与测试：以 263 实施阶段实际文件清单为准。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/261-librechat-assistant-conversation-failure-investigation-and-remediation-plan.md`
- `docs/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
- `docs/dev-records/dev-plan-262-execution-log.md`
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
