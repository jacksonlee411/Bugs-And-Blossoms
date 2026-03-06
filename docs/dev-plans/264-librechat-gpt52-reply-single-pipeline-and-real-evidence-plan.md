# DEV-PLAN-264：LibreChat GPT-5.2 回复单链路与真实验收证据收敛方案

**状态**: 已完成（2026-03-06 11:18 CST）

## 1. 背景与纠偏
- 263 的核心目标没有变化：用户在 `/app/assistant/librechat` 输入业务句子后，系统必须先构造机器态上下文，再 prompt 给 GPT-5.2，最后由 GPT-5.2 输出用户可见回复（正常与报错同口径）。
- 现有实现与证据存在偏差：出现了 mock 取证、固定造数 `conversation_id/turn_id`、非真实链路截图等情况，不能作为 263 目标达成证据。
- 本计划（264）用于替代 263 的实施与验收口径：**以真实链路、同轮次可对账证据、模型命中硬约束为唯一通过标准**。

## 2. 目标与非目标
### 2.1 目标（必须同时满足）
1. [X] 正常路径与错误路径都先进入“机器态上下文 -> GPT-5.2 回复转写 -> 聊天气泡展示”的单链路。
2. [X] 聊天气泡中的业务文案只允许来自 `reply_nlg.text`，禁止前端/后端本地模板直出。
3. [X] 每轮通过必须满足 `reply_model_name=gpt-5.2`，未命中直接判失败。
4. [X] 验收证据必须是同轮次三件套：页面全图、聊天气泡图、后端同轮 Trace（含 `conversation_id/turn_id/reply_model_name/reply_prompt_version`）。
5. [X] 用户界面禁止出现内部技术错误码原文（例如 `ai_plan_schema_constrained_decode_failed`）。

### 2.2 非目标
1. [X] 不修改业务域 schema / 迁移 / sqlc。
2. [X] 不引入 legacy 回退通道、双链路、页面外层业务兜底提示。
3. [X] 不修改 LibreChat 上游源码，仅在本仓网关/编排/测试与证据层收敛。

## 3. 必须取消的旧限制与宽松条件（Stopline）
1. [X] 取消“前端可直接拼业务回复文案”的容忍。
2. [X] 取消“`:reply` 可跳过”的容忍。
3. [X] 取消“回复模型可回退到非 gpt-5.2 仍算通过”的容忍。
4. [X] 取消“报错场景可直接透传 fallback/error_message 给用户”的容忍。
5. [X] 取消“`allow_missing_turn=true` + `turn_id=system` 也可作为通过证据”的容忍。
6. [X] 取消“Alert/Notice 可作为业务通过依据”的容忍。
7. [X] 取消“mock `/internal/assistant/**` 结果可作为 264 验收证据”的容忍。
8. [X] 取消“只看截图不核对同轮 trace 字段”的容忍。
9. [X] 取消“用户文案可直出内部技术错误码”的容忍。

## 4. 冻结契约（264 SSOT）
1. [X] **C1 单链路契约**：业务回复统一走 `:reply`；create/confirm/commit 成功与失败均触发 `:reply`。
2. [X] **C2 输入上下文契约**：`stage/kind/outcome/error_code/error_message/next_action/locale/fallback_text/conversation_id/turn_id` 必须完整传入 GPT-5.2 回复渲染链路。
3. [X] **C3 输出契约**：`reply_nlg = { text, kind, stage, reply_model_name, reply_prompt_version, conversation_id, turn_id }`。
4. [X] **C4 模型命中契约**：`reply_model_name` 必须严格等于 `gpt-5.2`；否则返回 `ai_reply_model_target_mismatch` 并判失败。
5. [X] **C5 展示契约**：聊天流 `assistant.flow.dialog.text` 仅可映射 `reply_nlg.text`。
6. [X] **C6 技术码隔离契约**：技术错误码仅用于 trace/log；用户可见文本必须是 GPT-5.2 转写自然语言。
7. [X] **C7 证据契约**：每次验收必须同轮次绑定 `conversation_id + turn_id`，并记录 `reply_model_name + reply_prompt_version`。

## 5. 实施步骤
### 5.1 M1：契约冻结与无效证据处置
1. [X] 将 263 阶段 mock 产出的“通过证据”标记为无效（仅保留排障用途，不计入通过）。
2. [X] 在 264 文档与执行日志中明确“验收证据必须来源于真实链路”。

### 5.2 M2：后端编排收敛（单链路）
1. [X] 统一 create/confirm/commit 的成功和失败出口到 `renderTurnReply(...)`。
2. [X] 回复网关严格校验 `reply_model_name=gpt-5.2`，不满足即 fail-closed。
3. [X] `:reply` 输出补齐审计字段并与 turn 绑定。

### 5.3 M3：前端展示收敛（唯一出口）
1. [X] `LibreChatPage` 与 `AssistantPage` 禁止使用本地业务模板作为最终回复。
2. [X] `assistant.flow.dialog` 只投递 `reply_nlg` 结果。
3. [X] Alert/Notice 仅保留连接态/技术态，不参与业务通过判定。

### 5.4 M4：真实验收用例（非 mock）
1. [X] 新增 `tp264-real` 用例，禁止拦截 `/internal/assistant/**` 业务 API。
2. [X] 测试必须走真实 `/app/assistant/librechat` 链路并输入基准句。
3. [X] 用例硬断言：
   - [X] 聊天气泡出现业务回执文本；
   - [X] 同轮 trace 存在 `reply_model_name=gpt-5.2`；
   - [X] 页面无技术错误码原文；
   - [X] `conversation_id/turn_id` 与截图对应轮次一致。

### 5.5 M5：证据固化与封板
1. [X] 连续执行至少 3 轮真实验收，全部通过后方可封板。
2. [X] 每轮落盘三件套到 `docs/dev-records/assets/dev-plan-264/`。
3. [X] 将执行记录写入 `docs/dev-records/dev-plan-264-execution-log.md`。

## 6. 验收标准（硬门槛）
1. [X] 正常与错误两类场景均满足“先 prompt GPT-5.2，再对用户展示”的单链路。
2. [X] `reply_model_name=gpt-5.2` 为通过前置条件，不满足立即失败。
3. [X] 业务回执只在聊天流气泡中判定通过，页面外层提示不计入通过。
4. [X] 三件套证据完整且同轮次可对账。
5. [X] 任意一轮出现 mock 证据或固定造数冒充真实链路，整轮判无效。

## 7. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- 264 最低验证集：
  1. [X] `go test ./internal/server -run "TestAssistantReplyNLGPipeline|TestAssistantReplyModelTargetGate|TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase" -count=1`
  2. [X] `pnpm --dir apps/web test -- src/pages/assistant/LibreChatPage.test.tsx`
  3. [X] `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js tests/tp262-librechat-dialog-anchor.spec.js`
  4. [X] `pnpm --dir e2e exec playwright test tests/tp264-librechat-gpt52-dialog-response-real.spec.js`
  5. [X] `make check doc`

## 8. 交付物
1. [X] 计划文档：`docs/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`
2. [X] 执行日志：`docs/dev-records/dev-plan-264-execution-log.md`
3. [X] 证据目录：`docs/dev-records/assets/dev-plan-264/`
4. [X] 测试用例：`e2e/tests/tp264-librechat-gpt52-dialog-response-real.spec.js`

## 9. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/261-librechat-assistant-conversation-failure-investigation-and-remediation-plan.md`
- `docs/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
- `docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
