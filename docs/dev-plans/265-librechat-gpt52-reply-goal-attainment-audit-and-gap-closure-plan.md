# DEV-PLAN-265：LibreChat 回复经 GPT-5.2 单链路目标达成度审计与缺口收敛方案

**状态**: 规划中（2026-03-06 18:54 CST）

## 1. 背景与唯一目标口径
- 本计划以用户明确补充的“最初目标”为唯一比对基线：**针对回复，系统消息必须先 prompt 给 GPT-5.2；无论正常回复还是报错回复，都要先经过 GPT-5.2，再由 GPT-5.2 告诉用户。**
- `DEV-PLAN-264` 已将口径从 mock 取证纠偏到“真实链路 + 同轮次证据”，但其“已完成”状态并不自动等于“最初目标已经完全达成”；必须重新按上述唯一目标逐项审计。
- 本计划（265）不直接宣布 264 失败或通过，而是把“**已实现 / 偏离 / 缺口**”拆开固化，作为后续修复与重新验收的契约入口。

### 1.1 当前验收入口与前置门槛
- 当前用户入口以 `http://localhost:8080/app/assistant/librechat` 为准；`/assistant-ui` 只可作为历史证据与调试入口。
- 265 聚焦“回复必须先经 GPT-5.2 再展示”，但它的通过证据仍必须建立在 `DEV-PLAN-266` 的 UI / 通道前置门槛之上：无官方原始发送、无官方 `Connection error`、无页面外挂回复容器、同轮唯一 assistant 回复。
- 若 `266` 的门槛不满足，则即使 `reply_source=model`、`used_fallback=false` 成立，也不得单独宣称当前用户体验达成。

## 2. 调查范围与证据基线
1. [X] 契约文档基线：`docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`、`docs/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`。
2. [X] 执行记录基线：`docs/archive/dev-records/dev-plan-263-execution-log.md`、`docs/archive/dev-records/dev-plan-264-execution-log.md`。
3. [X] 真实证据基线：`docs/archive/dev-records/assets/dev-plan-264/run-2026-03-06T03-13-36-135Z/trace-reply.json` 与同目录截图。
4. [X] 实现基线：`internal/server/assistant_reply_nlg.go`、`internal/server/assistant_reply_model_gateway.go`、`apps/web/src/pages/assistant/LibreChatPage.tsx`、`e2e/tests/tp264-librechat-gpt52-dialog-response-real.spec.js`。

## 3. 264 目标实现情况审计

### 3.1 已实现部分（可保留）
1. [X] 已存在独立 `:reply` 链路，前端会在业务对话回执展示前调用 `/internal/assistant/conversations/{conversation_id}/turns/{turn_id}:reply`。
2. [X] 后端已存在回复模型目标门禁：`reply_model_name != gpt-5.2` 时返回 `ai_reply_model_target_mismatch`，具备基本 fail-closed 意图。
3. [X] 已有一条真实非 mock 的 happy path E2E，用例 `e2e/tests/tp264-librechat-gpt52-dialog-response-real.spec.js` 能证明“真实页面链路下确实命中过 `:reply`，且返回字段包含 `reply_model_name=gpt-5.2`”。
4. [X] 技术错误码净化已有基础实现：后端对 `ai_*`、`trace_id`、`request_id` 等技术信号做用户文案收敛，降低内部错误码直出风险。

### 3.2 偏离最初目标（与“必须先 prompt 给 GPT-5.2，再由 GPT-5.2 告诉用户”不一致）
1. [X] **本地模板仍是回复输入与潜在最终输出来源**：`LibreChatPage` 仍先生成本地业务文案（如 `formatCommitSuccessMessage(...)`、`errorMessage(...)`），再把该文本作为 `fallback_text` / `error_message` 送入 `:reply`；这意味着系统当前仍依赖本地模板构造可直接展示的完整句子，而非只把机器态事实 prompt 给 GPT-5.2。
2. [X] **模型解码失败时仍可回退到本地文案，但对外仍声明命中 GPT-5.2**：`assistantDecodeOpenAIReplyResult(...)` 在 OpenAI 返回无法解码或 `parsed.Text` 为空时，会直接使用 `prompt.FallbackText` 生成 `Text`，同时把 `ReplyModelName` 固定写为 `gpt-5.2`。这与“用户可见回复必须由 GPT-5.2 生成”直接冲突。
3. [X] **缺轮次时仍保留 synthetic turn 通道**：`LibreChatPage` 仍存在 `allowMissingTurn=true` + `assistantSyntheticReplyTurnID` 的显式路径；`assistant_reply_nlg.go` 也允许 `AllowMissingTurn` 绕过真实 turn 查找。这与 264 自身 stopline 中“取消 `allow_missing_turn=true` 作为通过证据容忍”不一致，也偏离了“同轮次、真实 turn、先 prompt 后回复”的目标。
4. [X] **264 的真实证据无法证明“最终展示文案确由 GPT 改写产生”**：现有 `trace-reply.json` 中，`reply_request.body.fallback_text` 与 `reply_response.body.text` 完全相同；结合当前网关允许 fallback 的实现，证据只能证明“走过 `:reply`”，不能充分证明“该最终文本一定由 GPT-5.2 生成而非本地文案透传”。
5. [X] **回复生成失败时，前端仍会直接展示本地错误 notice**：`postBridgeDialog(...)` 捕获 `renderAssistantTurnReply(...)` 异常后，直接输出“回复生成失败，请稍后重试。”；这条用户可见文案并未经过 GPT-5.2，不满足“报错也要先 prompt 给 GPT-5.2 再告诉用户”。

### 3.3 实现缺口（尚未覆盖或尚未证明）
1. [ ] **缺少真实报错链路 E2E**：`tp264` 仅覆盖一条成功草案路径；未证明 create/confirm/commit 的失败路径、模型异常路径、上游错误路径都先进入 GPT-5.2 回复链路。
2. [ ] **缺少 264 自述要求的“三轮真实证据三件套”**：执行日志记录了 `--repeat-each=3` 通过，但 `docs/archive/dev-records/assets/dev-plan-264/` 当前仅见一组截图 + trace，不满足“每轮落盘三件套”的硬要求。
3. [ ] **缺少“回复来源”可审计字段**：当前证据只记录 `reply_model_name`，未记录 `reply_source=model` / `used_fallback=false` / 上游响应指纹等字段，因此无法审计最终用户文案究竟来自模型输出还是本地 fallback。
4. [ ] **缺少会话主对象中的最终回复 SoT**：前端类型声明了 `turn.reply_nlg`，但服务端 `assistantTurn` 结构未落该字段；现状下最终用户可见回复并非对话 turn 的稳定事实源，后续审计只能依赖旁路 trace 文件。
5. [ ] **缺少 stopline 级门禁**：当前没有自动化测试明确阻断“allow_missing_turn 进入验收路径”“模型解码失败却仍回退本地文案并宣称 gpt-5.2 命中”“render reply 失败时页面直接输出本地业务文案”等情况。

## 4. 265 目标（本计划要达成什么）
1. [ ] 以“正常与报错回复都必须先 prompt 给 GPT-5.2，再由 GPT-5.2 告诉用户”为唯一验收目标，重写 264 的完成判定。
2. [ ] 收敛为严格单链路：用户可见业务回执只能来自真实 turn 上下文 + GPT-5.2 输出，禁止 fallback 冒充模型输出。
3. [ ] 补齐可审计证据：每轮必须能证明“真实 turn / 真实模型 / 非 fallback / 最终展示文本”属于同一轮。
4. [ ] 形成新的 stopline 与回归集，阻断后续再次以“走过 `:reply` 但实际仍是本地文案”为通过依据。
5. [ ] 265 的复验与后续封板必须回到 `/app/assistant/librechat` 真实入口，并继承 `266` 的共通 stopline。

## 5. 冻结契约（265 SSOT）
1. [ ] **C1 回复来源契约**：用户可见业务回复必须来自 GPT-5.2 实际输出；若模型输出缺失、不可解码或超时，则该轮回复生成失败，不得用本地 `fallback_text` 冒充最终业务回复。
2. [ ] **C2 输入语义契约**：送入 GPT-5.2 的应是机器态事实（`stage/kind/outcome/error_code/error_message/next_action/conversation_id/turn_id/...`），而不是已完成的最终用户文案模板。
3. [ ] **C3 同轮次契约**：验收仅接受真实 `conversation_id + turn_id`；禁止 synthetic turn、禁止 `allow_missing_turn` 进入通过证据。
4. [ ] **C4 失败态契约**：create/confirm/commit 失败与回复渲染失败应明确区分；前者必须先进入 GPT-5.2 回复链路，后者则直接判该轮失败，不得计入“回复链路达成”。
5. [ ] **C5 审计契约**：每次验收必须记录 `reply_model_name`、`reply_prompt_version`、`reply_source`、`used_fallback`、同轮次 `conversation_id/turn_id`、页面图、聊天图、trace。
6. [ ] **C6 SoT 契约**：最终用户可见回复应能在 turn 主对象中稳定取到（例如 `reply_nlg`），不得只存在于旁路 trace 或前端临时 bridge 消息中。

## 6. 实施步骤

### 6.1 M1：回复来源与 fail-closed 收紧
1. [ ] 移除 `assistantDecodeOpenAIReplyResult(...)` 中“解码失败/空文本时直接采用 `prompt.FallbackText` 但仍标记 `gpt-5.2`”的行为。
2. [ ] 区分 `model_render_failed` 与 `business_failed`：前者直接判回复链路失败，后者才允许把机器态错误事实送给 GPT-5.2 转写。
3. [ ] 为 `:reply` 增加 `reply_source`、`used_fallback`（预期恒为 `false`）等可审计字段，并在测试中硬断言。

### 6.2 M2：前端输入语义去模板化
1. [ ] 收敛 `LibreChatPage`：本地模板仅可作为机器态事实的临时调试文本，不得再作为最终用户文案候选。
2. [ ] 移除或封禁 `allow_missing_turn` + synthetic turn 在正式验收链路中的使用。
3. [ ] `renderAssistantTurnReply(...)` 失败时，不再用本地业务文案兜底冒充 GPT 回复；前端只能显示技术态失败提示，并把该轮标记为未达成。

### 6.3 M3：SoT 与审计补齐
1. [ ] 在服务端 turn 结构中补齐 `reply_nlg`（或等价稳定字段），使最终回复成为对话主对象的一部分。
2. [ ] 让 create/confirm/commit 成功与失败都能落同轮回复事实，避免仅靠前端 bridge 侧消息保留展示结果。
3. [ ] 为同轮 trace 增加“上游模型响应摘要/哈希/来源标记”，用于证明该轮文本确为模型输出，而不是本地透传。

### 6.4 M4：真实验收与证据重做
1. [ ] 新增真实成功路径 E2E：证明正常 create/confirm/commit 回执先经 GPT-5.2 再展示。
2. [ ] 新增真实失败路径 E2E：至少覆盖一个 create/confirm/commit 失败场景，并证明报错文案也先经 GPT-5.2。
3. [ ] 连续执行至少 3 轮真实验收并逐轮落盘三件套；每轮必须单独落目录，禁止只记录 `repeat-each=3` 汇总结果。
4. [ ] 在执行日志中逐轮登记 `conversation_id/turn_id/reply_model_name/reply_source/used_fallback`。
5. [ ] 真实验收需从 `/app/assistant/librechat` 入口启动，并同时断言 `266` 的共通 stopline 仍成立。

### 6.5 M5：门禁与封板
1. [ ] 新增单测/集成测试：阻断 fallback 冒充 GPT、阻断 `allow_missing_turn`、阻断 reply render 失败后直出本地业务文案。
2. [ ] 更新 264/265 相关执行日志与状态判定口径；未满足 265 契约前，不得再将“264 已完成”解释为“最初目标已完全达成”。

## 7. 验收标准（硬门槛）
1. [ ] 正常路径与报错路径都能证明：先形成机器态事实，再请求 GPT-5.2，再向用户展示。
2. [ ] 任一轮若发生模型解码失败、空文本、超时或缺 turn，上层不得用本地业务文案冒充 GPT 回复；该轮直接判失败。
3. [ ] 验收证据必须能证明最终展示文本不是 fallback 透传；至少包含 `reply_source=model`、`used_fallback=false` 或等价强证明。
4. [ ] 连续 3 轮真实验收均具备独立三件套证据，且至少 1 轮为真实失败路径。
5. [ ] 对话主对象能够回放最终回复事实，不再依赖单次抓包或旁路 trace 才能知道用户看到什么。
6. [ ] 任一轮若未满足 `266` 的单通道、气泡内回写、无外挂容器、无官方原始错误体验门槛，则该轮直接判失败。

## 8. 交付物
1. [ ] 计划文档：`docs/dev-plans/265-librechat-gpt52-reply-goal-attainment-audit-and-gap-closure-plan.md`
2. [ ] 后续执行日志：`docs/archive/dev-records/dev-plan-265-execution-log.md`
3. [ ] 补强后的测试：以实施阶段新增/修订文件为准。
4. [ ] 补强后的真实证据目录：`docs/archive/dev-records/assets/dev-plan-265/`

## 9. 结论
- 以“正常/报错回复都必须先 prompt 给 GPT-5.2，再由 GPT-5.2 告诉用户”为唯一目标口径审计，264 **并非完全未做成**，但当前最多只能判定为“已打通 `:reply` 基础链路、已具备部分真实证据”；**尚不能判定为最初目标已完整达成**。
- 265 的职责不是重复 264，而是把“264 已经做到什么、哪些偏离目标、哪些还是缺口”一次性说清，并把后续修复收敛到可验证、可阻断漂移的契约上。
