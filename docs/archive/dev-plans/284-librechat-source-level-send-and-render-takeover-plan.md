# DEV-PLAN-284：LibreChat 发送与渲染主链路源码级接管实施计划

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-08 CST；正式入口下的 send/store/render 已切到本仓 Assistant 主链，业务回执统一进入官方消息树，同泡确认/提交已可落地）

## 0. 284-prep 当前进度（2026-03-08）
- [x] 冻结 `284-prep` 边界：当前只做实施前准备，不落 send/store/render 主链路行为变更代码。
- [x] 产出控制点扫描脚本：`scripts/librechat-web/scan-284-entrypoints.sh`（用于定位 send/store/render 与 SSE 入口）。
- [x] 产出 patch 清单草案：`third_party/librechat-web/patches/284-send-render-takeover.patchset-draft.txt`（尚未接入 `patches/series`）。
- [x] 产出 E2E 骨架：`e2e/tests/tp284-librechat-send-render-takeover.prep.spec.js`（`fixme` 占位，不进入通过口径）。
- [x] 在本计划内补齐“接口映射草案 + patch 主题草案 + 测试骨架清单”，用于 `283` 完成后的快速切入。

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280D`。
- 本计划聚焦 send/store/render 三个控制点的源码级接管，完成“前端降权、后端 DTO 驱动、官方消息树唯一渲染面”的收口。

## 2. 目标与非目标
### 2.1 目标
1. [X] 在 vendored UI 的发送 action / store / renderer 中接入本仓主链路。
2. [X] 让缺字段、多候选、确认、提交成功/失败全部进入官方消息列表与 assistant message 实体。
3. [X] 完成前端降权：页面 helper / adapter 不再承载业务 phase 推进、候选裁决或提交约束。

### 2.2 非目标
1. [X] 不在本计划内重定义业务事实源与 FSM 语义（以 `223/260` 为 SSOT）。
2. [X] 不在本计划内定义升级/回滚策略（由 `237` 承接）。

## 3. 顺序与 readiness
1. [X] `284` 只能在 `281` 完成后启动。
2. [X] `DEV-PLAN-223` 已明确业务事实源字段与回放口径。
3. [X] `DEV-PLAN-260` 已冻结 DTO 契约：`phase / missing_fields / candidates / pending_draft_summary / selected_candidate_id / commit_reply / error_code`。
4. [X] `DEV-PLAN-283` 已完成正式入口切换，不再依赖旧桥接链路承载发送与回执。
5. [X] `284` 不应与“旧桥接仍承担正式职责”的状态并行存在。

### 3.3 准备阶段交付边界（本次）
1. [x] 允许交付：控制点定位、接口映射草案、patch 清单草案、测试骨架。
2. [x] 禁止交付：正式发送接管、正式渲染接管、正式 helper 失活与任何会改变用户可见行为的改动。
3. [x] patch 草案仅记录在草案文件中，未进入 `patches/series`，不会被构建链自动回放。

## 3.1 禁止项
1. [X] 禁止把旧的页面 helper 业务编排逻辑简单搬运到 vendored UI 中继续存活。
2. [X] 禁止前端根据文本、局部上下文或 UI 临时状态重新计算 phase、候选裁决或提交约束。
3. [X] 禁止继续通过 DOM 注入、外挂容器、第二消息流承载用户可见业务回执。

## 3.2 搜索型 stopline
1. [X] 完成 `284` 后，正式用户可见业务职责不应再由 `assistantDialogFlow`、`assistantAutoRun` 或等价 helper 承担。
2. [X] 完成 `284` 后，不应再存在 `document.createElement(...)` 式外挂消息流作为正式回执落点的实现口径。
3. [X] 完成 `284` 后，前端消费契约应可清晰定位为后端 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` DTO，而非散落的本地判定逻辑。

### 3.4 `260` P0 契约对齐结果（2026-03-08）
1. [X] DTO 字段命名与适用范围改为引用 `260` 第 4.3 与第 5.5 节，不再使用 `draft/commit-reply` 简写。
2. [X] 阶段推进与 guard 改为引用 `260` 第 5.4 节；`284` 不再重复定义业务 FSM。
3. [X] 接口对接以 `260` 第 6.1 节契约矩阵为准，`284` 只定义 send/store/render 控制点接管。
4. [X] 前端降权 stopline 改为引用 `260` 第 5.6 节，作为 patch 验收前置断言。

## 4. 实施步骤
1. [X] 接管发送 action：阻止旧的页面级业务编排继续推进状态。
2. [X] 接管消息渲染：业务回执全部进入官方消息组件树。
3. [X] 删除或失活页面 helper 对业务状态推进、候选解析、确认约束的正式职责。
4. [X] 增加源码级单测/组件测试，覆盖 send/store/render 关键路径。

### 4.1 接口映射草案（284-prep）
| 控制点 | vendored UI 入口（当前源码） | 本仓接口/契约（目标对接） | 备注 |
| --- | --- | --- | --- |
| 发送入口（form submit） | `client/src/components/Chat/Input/ChatForm.tsx` `onSubmit={methods.handleSubmit(submitMessage)}` | `POST /internal/assistant/conversations/{conversation_id}/turns` | 入口锚点已固定，后续 patch 在 `submitMessage/ask` 之间接入 adapter。 |
| 发送组装（ask 前） | `client/src/hooks/Messages/useSubmitMessage.ts` `submitMessage()` | `request_id/trace_id + user_input`（`223/260` 口径） | 当前只定位入口，不引入业务重算。 |
| 提交载荷组装（store 前） | `client/src/hooks/Chat/useChatFunctions.ts` `ask()` 组装 `submission` | DTO：`phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code`（以 `260` 冻结版为准） | `284` 正式阶段将把本地语义判断收敛为 DTO 消费。 |
| 流式事件接入（SSE） | `client/src/hooks/SSE/useSSE.ts`（`created/sync/event/type/final/error`） | `GET /internal/assistant/conversations/{conversation_id}`（回读）+ turn action 接口 | 本次只冻结事件接入点，不改事件语义。 |
| 消息落盘与渲染前更新 | `client/src/hooks/SSE/useEventHandlers.ts` `setMessages/queryClient.setQueryData` | 对齐 `223` 事实源字段（`conversation_id/turn_id/request_id/trace_id`） | 正式阶段要求所有业务回执进官方消息数组。 |
| 官方消息渲染面 | `client/src/components/Chat/Messages/ui/MessageRender.tsx` + `MessageContent` | 仅消费后端 DTO，不再前端重算阶段/候选/确认 | 本次只确认唯一渲染面入口。 |
| 确认/提交动作 | 本仓 API SDK：`apps/web/src/api/assistant.ts` | `POST .../turns/{turn_id}:confirm`、`POST .../turns/{turn_id}:commit`、`POST ...:reply` | 作为 `284` patch 接入后可直接复用的动作接口。 |

### 4.2 patch 清单草案（284-prep）
1. [x] 草案文件：`third_party/librechat-web/patches/284-send-render-takeover.patchset-draft.txt`。
2. [ ] `0002-hook-submit-message-to-assistant-adapter.patch`：发送动作接管入口（`ChatForm/useSubmitMessage`）。
3. [ ] `0003-restrict-local-fsm-and-consume-dto.patch`：前端降权，禁止本地业务语义重算（`useChatFunctions/useEventHandlers`）。
4. [ ] `0004-unify-assistant-receipts-into-official-message-tree.patch`：回执统一进入官方消息树（`useEventHandlers/MessageRender`）。
5. [ ] `0005-disable-legacy-helper-business-role.patch`：失活旧 helper 的正式业务职责（落点以扫描脚本结果为准）。

### 4.3 测试骨架清单（284-prep）
1. [x] `e2e/tests/tp284-librechat-send-render-takeover.prep.spec.js` 已创建（`fixme` 占位，含 4 个 case 骨架）。
2. [X] readiness 前置已满足：`283` 已完成正式入口切换；进入 `284` 正式 patch 后把 `fixme` 转实测（单通道发送、DTO 驱动渲染、官方消息树唯一渲染面、helper 失活验证）。
3. [X] 已补组件/单测覆盖 send/store/render 关键路径与错误分支（本次新增 runtime 单测 + assistant message 组件测试）。

### 4.4 本次正式 patch 交付（2026-03-08）
1. [X] `useSSE` 在 `/app/assistant/librechat` 正式入口下接管发送，不再走上游原始 SSE 业务链。
2. [X] Assistant DTO 通过同一条官方 assistant message 实体渲染，缺字段/候选/确认/提交结果均在官方消息树内完成。
3. [X] 候选确认与提交动作已落到同泡按钮交互，并直接调用 `/internal/assistant/*` 正式接口。
4. [X] `ChatView/useChatHelpers` 已停用正式入口下的上游 conversation/messages 拉取，避免第二消息事实源回流。
5. [X] 本地验证已通过：`npm --workspace client run test:ci -- --runInBand src/assistant-formal/__tests__/runtime.test.ts src/components/Chat/Messages/Content/__tests__/AssistantFormalMessage.test.tsx`、`make librechat-web-build`。

## 5. 验收标准
1. [X] 前端只消费后端 DTO，不再重算业务语义。
2. [X] 官方消息树是唯一用户可见渲染面。
3. [X] 不再存在页面级 helper 承担正式业务推进职责。
4. [X] `make check doc` 通过。

## 6. 关联文档
- `docs/archive/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `AGENTS.md`
