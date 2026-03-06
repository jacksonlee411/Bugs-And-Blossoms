# DEV-PLAN-261：LibreChat 助手对话失败问题排查与修复实施计划

**状态**: 进行中（2026-03-06 07:32 CST）

## 1. 背景与问题定义
- 现象：在 `http://localhost:8080/app/assistant/librechat` 的“助手对话”场景中，用户反馈“对话不成功”（无回复、未进入草案确认链路、或会话状态异常）。
- 上下文：`DEV-PLAN-260` 已完成“对话优先自动执行”重构，但当前反馈说明仍存在运行态失败点或回归点。
- 本计划目标是以“先定位根因，再一次性修复并补足防回归证据”为原则收口问题。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 明确“对话不成功”的根因分类，并给出可复现证据（页面、接口、日志三方对齐）。
2. [ ] 修复 `/app/assistant/librechat` 主链路，恢复“发起对话 -> AI回执 ->（必要时）确认 -> 提交结果”闭环。
3. [ ] 修复后补齐自动化回归（Web/Go/E2E），防止同类问题再次漂移。
4. [ ] 保持 One Door：写入仍仅走既有 `/internal/assistant/*` 链路，不新增第二写入口。

### 2.2 非目标
1. [ ] 不引入 legacy 回退通道、双链路或兼容快路径。
2. [ ] 不修改业务域数据模型语义（Org/SetID/RLS/Authz 契约不扩张）。
3. [ ] 不新增数据库表；若确认必须新增迁移，先提交独立契约并获得用户确认。

## 3. 失败面分类与排查假设
1. [ ] **Bridge 层**：`assistant.prompt.sync`/`assistant.flow.dialog` 消息未正确收发，导致前端无业务回执。
2. [ ] **会话层**：`create turn`/`confirm`/`commit` 状态机错位，返回 `conversation_state_invalid` 等错误后未被正确呈现。
3. [ ] **运行时依赖层**：LibreChat upstream、模型提供方或代理健康状态抖动，触发超时/限流/结构化解码失败。
4. [ ] **身份与边界层**：`/assistant-ui` 会话、cookie、csrf 或租户注入异常，导致请求被拒绝或短路。
5. [ ] **可观测性层**：失败信息未透出到聊天流，用户感知为“无响应”。

## 4. 实施步骤

### 4.1 M1：复现基线与证据采集
1. [X] 固化最小复现脚本（提示词、账号、页面入口、期望结果、实际结果）。
2. [X] 采集同一轮次的前端事件轨迹（bridge 消息）、后端接口响应、服务日志，建立统一 `trace_id` 对照。
3. [X] 输出“失败分类矩阵 v1”（失败点 -> 错误码 -> 用户可见症状）。

### 4.2 M2：根因定位与修复设计
1. [X] 对照 `assistantDialogFlow` 状态迁移，确认失败是“消息丢失/状态不合法/依赖失败/鉴权失败”哪一类。
2. [X] 给出单主因或主因+次因组合，并形成可审计修复方案（含受影响文件、风险点、回归面）。
3. [X] 修复方案需满足“Simple > Easy”：不堆补丁分支，不复制第二套编排逻辑。

### 4.3 M3：代码修复与错误回执收敛
1. [X] 修复主链路配置（运行态模型优先级回退收敛，按根因最小变更）。
2. [X] 确保失败场景在聊天流中有明确错误回执（沿用既有错误码与回执链路）。
3. [X] 保持 `AssistantPage` 与 `LibreChatPage` 使用统一 helper，避免逻辑漂移。

### 4.4 M4：回归验证与证据闭环
1. [ ] 补齐/更新单测与集成测试，覆盖失败场景与修复后主路径。
2. [ ] 新增或更新 E2E 用例，覆盖“真实对话成功 + 关键失败提示可见”。
3. [X] 记录执行证据到 `docs/dev-records/dev-plan-261-execution-log.md`，并更新计划状态。

## 5. 验收标准（必须全部满足）
1. [ ] `/app/assistant/librechat` 可稳定完成首轮对话回执（不再出现“发送后无业务响应”）。
2. [ ] Case 2~4（完整信息、缺字段补全、多候选二次确认）在修复后可按既有契约通过。
3. [ ] 失败场景出现时，聊天流展示可操作提示（错误码语义化映射），不再仅靠控制台日志。
4. [ ] 会话列表可见最新会话与最后一轮输入，且与实际交互一致。
5. [ ] 相关门禁通过（按 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 执行）。

## 6. 测试与覆盖率
- 覆盖率口径与阈值遵循仓库现行门禁（以 `Makefile`/CI 为 SSOT，保持覆盖率 gate 不退化）。
- 本计划至少覆盖：
  1. [ ] Web：FSM 状态迁移、确认词门槛、失败回执渲染。
  2. [ ] Go：`assistant_ui_proxy` 注入/代理关键分支与错误透传。
  3. [ ] E2E：真实页面链路从输入到回执的闭环验证。

## 7. 风险与约束
1. [ ] 若根因为第三方模型瞬时不可用，仍需保证用户可见错误与可重试指引，禁止静默失败。
2. [ ] 不以“临时兼容开关”替代根因修复，避免违反 No Legacy 原则。
3. [ ] 若定位到跨模块契约变更，先更新对应 dev-plan 再实施代码。

## 8. 交付物
1. [ ] 计划文档：`docs/dev-plans/261-librechat-assistant-conversation-failure-investigation-and-remediation-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-261-execution-log.md`
3. [ ] 代码与测试：以实际修复文件清单为准，执行阶段在日志中逐项固化。

## 9. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-records/dev-plan-260-execution-log.md`
- `docs/dev-records/dev-plan-261-execution-log.md`
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
