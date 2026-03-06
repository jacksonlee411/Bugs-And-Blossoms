# DEV-PLAN-263：LibreChat GPT-5.2 助手对话回复专线实施方案

**状态**: 规划中（2026-03-06 08:49 CST）

## 1. 背景与问题定义
- 用户对齐后的需求是明确且单一的：在 `/app/assistant/librechat` 输入自然语言后，必须由 **GPT-5.2** 进行理解并产出回复，且回复必须以助手对话形式出现在 LibreChat 聊天流内部。
- 现状问题：
  1. [ ] 当前链路存在模型响应波动（部分请求出现 `ai_plan_schema_constrained_decode_failed`）。
  2. [ ] 既有验证口径混入“页面提示/技术提示”，未严格等价于“GPT-5.2 聊天回复”。
  3. [ ] 证据口径不够硬：未把“聊天截图 + 后端 `model_name=gpt-5.2` 同轮次证据”冻结为硬验收。
  4. [ ] 用户可见层仍可能出现技术错误直出，未收敛为“助手自然语言解释 + 下一步建议”。

## 2. 目标与非目标

### 2.1 目标（必须全部满足）
1. [ ] 在 `/app/assistant/librechat` 中，用户输入业务句子后，助手回复必须出现在聊天对话流内部（聊天气泡语义）。
2. [ ] 该轮回复必须由 `model_name=gpt-5.2` 产生（非其他模型）。
3. [ ] 输出证据必须同轮次可追溯：
   - 聊天界面截图（可见助手回复）；
   - 后端接口/日志证据（同一轮 `conversation_id/turn_id`，`model_name=gpt-5.2`）。
4. [ ] 成功与失败都必须由助手聊天气泡返回自然语言结果，不允许“无可见回复”或裸露技术错误码。

### 2.2 非目标
1. [ ] 不改业务域 schema / 迁移 / sqlc。
2. [ ] 不引入 legacy 双链路、兼容快路径、页面外层兜底提示替代聊天回复。
3. [ ] 不修改 LibreChat 上游源码，仅在本仓代理注入 + 编排 + 验证层收敛。

## 3. 需求冻结（Contract）
1. [ ] **模型约束**：本验收链路默认强制 `gpt-5.2` 为首选并要求命中；若未命中则判失败，不计通过。
2. [ ] **回复形态约束**：用户可见回复必须在 iframe 聊天流（chat transcript）内；禁止仅在页面 Alert/Notice 层显示即判通过。
3. [ ] **证据约束**：每次“通过”至少包含：
   - `Screenshot-A`：`/app/assistant/librechat` 页面全图；
   - `Screenshot-B`：iframe 聊天区局部图；
   - `Trace-C`：同轮次后端响应（含 `conversation_id`/`turn_id`/`model_name=gpt-5.2`）。
4. [ ] **用户文案约束**：用户界面只展示 GPT-5.2 生成的业务对话文案；技术错误码仅写日志/trace，不直接暴露给最终用户。
5. [ ] **输入基准句**（用户原始验收句）：
   - `在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。`

## 4. 实施步骤

### 4.1 M1：模型命中强约束
1. [ ] 梳理 assistant model 路由，确保 263 验收链路首选并实际命中 `gpt-5.2`。
2. [ ] 增加“命中校验”检查点：请求成功但 `model_name != gpt-5.2` 时，返回可诊断错误并计为失败。
3. [ ] 记录 `model_name` 到可对账路径（会话 turn 响应/日志）以便截图对齐。
4. [ ] 将模型输出统一归一到“assistant_dialog_message”结构，确保前端始终渲染为聊天气泡。

### 4.2 M2：对话输出形态收敛
1. [ ] 保证助手业务回复通过 `assistant.flow.dialog` 进入聊天流容器。
2. [ ] 禁止将业务成功/失败仅落到页面 Alert/Notice；必须有聊天流内可见文案。
3. [ ] 在聊天根节点未就绪时，采用队列+重试；超时 fail-closed 并输出助手自然语言解释，不输出裸技术细节。
4. [ ] `ai_plan_schema_constrained_decode_failed` 等内部错误统一转译为“助手可理解回复 + 补充信息指引”。

### 4.3 M3：验证脚本与证据自动化
1. [ ] 新增 263 实时验证脚本（非 mock）：
   - 登录 app；
   - 登录 iframe（LibreChat）；
   - 发送基准句；
   - 抓取聊天气泡与后端 turn 响应；
   - 导出双截图 + JSON 证据。
2. [ ] 新增 E2E 用例断言：
   - 聊天流内存在助手回执；
   - 同轮次 `model_name=gpt-5.2`。
   - UI 中不存在原始技术错误码直出（如 `ai_plan_schema_constrained_decode_failed`）。
3. [ ] 将证据落盘到 `docs/dev-records/dev-plan-263-execution-log.md`。

### 4.4 M4：收口与回归
1. [ ] 回归 260/262 既有闭环用例，确认无回归。
2. [ ] 文档状态更新为“已完成”前，确保本计划所有硬验收项都有证据。

## 5. 验收标准（硬门槛）
1. [ ] 基准句输入后，聊天区出现助手回复（非页面外层提示）。
2. [ ] 对应 turn 响应包含 `model_name=gpt-5.2`。
3. [ ] 两类截图与同轮次 trace 证据齐全且可互相对账。
4. [ ] 同一环境连续验证通过（建议至少 3 次），成功率与失败原因可审计。
5. [ ] 用户界面无技术错误码直出，失败也由助手聊天文案承载。

## 6. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- 263 最低验证集：
  1. [ ] `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase" -count=1`
  2. [ ] `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js tests/tp262-librechat-dialog-anchor.spec.js`
  3. [ ] `pnpm --dir e2e exec playwright test tests/tp263-librechat-gpt52-dialog-response.spec.js`（新增）
  4. [ ] `make check doc`

## 7. 交付物
1. [ ] 计划文档：`docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
2. [ ] 执行日志：`docs/dev-records/dev-plan-263-execution-log.md`
3. [ ] 代码与测试：以 263 执行阶段实际文件清单为准。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
- `docs/dev-records/dev-plan-262-execution-log.md`
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
