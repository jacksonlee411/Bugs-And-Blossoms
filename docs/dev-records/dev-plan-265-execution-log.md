# DEV-PLAN-265 执行日志：GPT-5.2 回复目标达成缺口修复

**状态**: 已完成（2026-03-06 15:05 CST）

## 1. 记录口径
- 本日志记录 265 阶段围绕“正常/报错回复都必须先 prompt 给 GPT-5.2，再由 GPT-5.2 告诉用户”的修复与验证。
- 重点记录三类收口：本地聊天页恢复、fallback 冒充 GPT 回复根因修复、265 缺口验证补齐。

## 2. 执行记录
1. [X] 2026-03-06 12:18 CST：确认 `http://localhost:8080/app/assistant/librechat` 无法正常聊天的直接根因不是前端假死，而是 `/assistant-ui` 上游 `http://127.0.0.1:3080` 不可达；iframe 实际只显示“聊天服务暂不可用，请稍后重试。”。
2. [X] 2026-03-06 12:28 CST：在 `internal/server/assistant_ui_proxy.go` 增加 upstream 不可达时的仓内最小聊天壳层（同桥接协议、同 iframe 路径），恢复本地真实输入能力；同时在 `internal/server/assistant_runtime_status.go` 增加 live probe，避免运行态继续误报 `healthy`。
3. [X] 2026-03-06 12:34 CST：收紧回复链路 fail-closed：移除 `assistantDecodeOpenAIReplyResult(...)` 中“本地 `fallback_text` 冒充 GPT-5.2 回复”的逻辑，补充 `reply_source` / `used_fallback` 审计字段；从此无论正常还是异常回复，用户可见文案都不能再伪装成本地 GPT 输出。
4. [X] 2026-03-06 12:52 CST：在回复模型网关增加“结构化 JSON 优先，必要时同模型二次请求纯自然语言”的模型内收口；保持最终用户文案仍由 GPT-5.2 生成，而不是回退本地模板。
5. [X] 2026-03-06 13:18 CST：使用 `sudo -n ./scripts/librechat/up.sh` 拉起真实 upstream，确认 `bugs-and-blossoms-librechat-api-1` 等容器 healthy，`/assistant-ui` 已切回官方 LibreChat HTML，而不是 fallback shell。
6. [X] 2026-03-06 13:27 CST：定位官方 UI 仍停在 `/assistant-ui/login` 的直接根因：`/assistant-ui` 代理只做本仓 `sid` 校验，没有为已登录用户自动补发 LibreChat `refreshToken` / `token_provider` 会话。
7. [X] 2026-03-06 14:10 CST：在 `internal/server/assistant_ui_proxy.go` 增加 upstream 会话 bootstrap：
   - 进入 `/assistant-ui` / `/assistant-ui/login` 时，基于当前 tenant + principal 自动注册/登录 LibreChat 本地用户；
   - 将上游 `refreshToken` / `token_provider` 回写到 `/assistant-ui` path cookie；
   - 命中 `/assistant-ui/login` 时自动 302 回 `/assistant-ui`，恢复官方聊天页落点；
   - 保持 upstream 不可达时仍回退到仓内 fallback shell。
8. [X] 2026-03-06 14:28 CST：针对 265 缺口补齐“首轮无真实 turn 的错误回复也要走 GPT-5.2”能力：
   - 后端 `renderTurnReply(...)` 在 `allow_missing_turn=true` 时允许缺 turn 的 error-only reply 渲染；
   - 前端仅在“首轮结构化失败/阻塞回复”分支使用 `missing-turn-context` + `allow_missing_turn=true`；
   - 该分支仍返回 `reply_model_name=gpt-5.2`、`reply_source=model`、`used_fallback=false`，不再退回本地 notice 冒充 GPT。
9. [X] 2026-03-06 14:40 CST：增强首轮完整意图稳定性：`LibreChatPage` / `AssistantPage` 对“父组织+部门名+日期齐全”的创建请求先做本地归一，再送入 `createAssistantTurn(...)`；同时把 `composeStructuredIntentRetryPrompt(...)` 从空泛 `plan_only` 收敛为携带已解析字段的严格 JSON，提高真实 turn 生成成功率。
10. [X] 2026-03-06 14:56 CST：用登录后 API 重放验证 `:reply` 已恢复为 200，并返回：
   - `reply_model_name=gpt-5.2`
   - `reply_source=model`
   - `used_fallback=false`
11. [X] 2026-03-06 15:00 CST：补充并通过真实 E2E：
   - `e2e/tests/tp264-librechat-gpt52-dialog-response-real.spec.js`
   - `e2e/tests/tp265-librechat-gpt52-blocked-reply-real.spec.js`
   两条用例均为“真实 official iframe 输入 + 点击发送”，其中 `tp265` 覆盖首轮阻塞回复经 `missing-turn-context` 走 GPT-5.2 的缺口修复。
12. [X] 2026-03-06 15:03 CST：执行验证命令并通过：
   - `go test ./internal/server -run 'TestAssistantUIProxy|TestModifyAssistantUIProxyResponse|TestAssistantRenderReply|TestAssistantReply' -count=1`
   - `pnpm --dir apps/web test -- src/pages/assistant/assistantAutoRun.test.ts src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/AssistantPage.test.tsx`
   - `make css`
   - `pnpm --dir e2e exec playwright test tests/tp264-librechat-gpt52-dialog-response-real.spec.js tests/tp265-librechat-gpt52-blocked-reply-real.spec.js --reporter=line`

## 3. 当前结论
- `localhost:8080/app/assistant/librechat` 已从“无法正常聊天”恢复为“真实 upstream + 官方 UI + 可输入 + 可得到 GPT-5.2 回复”。
- `/assistant-ui` 已恢复到官方 LibreChat UI，不再停在 `/assistant-ui/login`；已登录本仓用户会自动 bootstrap LibreChat 上游会话。
- “fallback 冒充 GPT 回复”这一根因已移除：
  - 正常回复走真实 turn + `:reply` + `gpt-5.2`；
  - 首轮阻塞/错误回复走 `missing-turn-context` + `allow_missing_turn=true` + `:reply` + `gpt-5.2`；
  - 两条路径都返回 `reply_source=model`、`used_fallback=false`。
- 265 中最关键的缺口已补齐：
  - [X] 官方 UI 回切成功
  - [X] upstream 自动登录/会话桥接
  - [X] 回复来源审计字段补齐（`reply_source` / `used_fallback`）
  - [X] 正常路径真实 E2E
  - [X] 阻塞/失败类回复真实 E2E
- 后续可继续增强的仅是“更多轮次证据三件套”沉淀，不影响 265 当前目标达成。
