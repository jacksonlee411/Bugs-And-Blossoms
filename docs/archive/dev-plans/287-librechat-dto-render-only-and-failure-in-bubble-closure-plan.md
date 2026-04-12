# DEV-PLAN-287：266 剩余项 B——失败回包同落点与前端 DTO-only 降权收口

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-08 CST）

## 1. 背景
1. [X] `DEV-PLAN-266` 中“成功/失败同落点”与“前端不补算业务语义”两条 stopline，已由本计划完成实现收口。
2. [X] 本轮已去除正式链路中的外挂失败回执与前端补算依赖，`266` 与 `223/260/284` 口径重新对齐。

## 2. 目标与非目标
### 2.1 目标
1. [X] 将成功与失败场景统一到官方消息树同一落点体系。
2. [X] 失败时仅保留“官方消息流内技术态提示/可重试状态”，不再输出页面外 notice/overlay。
3. [X] 完成前端降权：渲染层只消费后端 DTO/phase，不在 helper/store/adapter 重新推导业务语义。

### 2.2 非目标
1. [X] 不承担“同轮唯一气泡映射”主实现（由 `DEV-PLAN-286` 承接）。
2. [X] 不承担真实入口证据封板与归档（由 `DEV-PLAN-288/285` 承接）。
3. [X] 不更改 `223/260` 的业务语义定义。

## 3. 266 剩余项映射（本计划承接）
1. [X] `266 §6.3-4`：错误场景与正常场景使用同一消息落点，并保留同轮追溯映射。
2. [X] `266 §6.3-6`：消息落点接管只做绑定/显示，不在 UI 层补算业务含义。
3. [X] `266 §6.4-3`：失败时只允许官方消息流内技术态失败提示或可重试状态。
4. [X] `266 §7-8`：正式主链路已不依赖注入脚本/DOM hack/HTML rewrite。
5. [X] `266 §7-9`：前端已改为只按后端 `phase` 与 DTO 渲染，不再基于局部状态补算业务约束。

## 4. 顺序与 readiness
1. [X] `DEV-PLAN-284` 正式 patch 已接管 send/store/render 主链路。
2. [X] `DEV-PLAN-286` 已提供稳定消息锚点，避免失败态收口时引入双泡回归。
3. [X] 本计划已完成；`266` 是否完成仍取决于 `288` 的真实入口证据封板。

## 5. 实施步骤
1. [X] 统一错误写回入口：失败回复走官方消息树同一 assistant message 更新链路。
2. [X] 下线外挂失败显示：正式入口不再额外调用页面外 notice/overlay/bridge 失败回执职责。
3. [X] DTO-only 审计：移除前端 `renderAssistantFormalReply(...)` 补渲染依赖，并将按钮/块显隐改为后端 `phase` 驱动。
4. [X] 增补守卫：已通过搜索型检查确认 `renderAssistantFormalReply(...)` 在正式渲染路径无调用、formal 路径无 `createElement/notice/overlay/toast` 回流。
5. [X] 补齐回归：新增失败态官方气泡测试，并更新候选确认路径测试以锁住 DTO-only 口径。

## 6. 验收标准
1. [X] 成功/失败均在官方消息树内显示，且失败不落页面外挂容器。
2. [X] 失败路径保持同一官方气泡落点，不绕回旧桥接链路。
3. [X] 前端代码面可证明“只消费 DTO/phase，不补算业务语义”。
4. [X] 本计划断言已通过；若后续回归破坏同落点或 DTO-only 约束，仍需 fail-closed。

## 7. 测试与门禁（SSOT 引用）
1. [X] 文档门禁：`make check doc`。
2. [X] 前端与 E2E 回归按 `AGENTS.md` 触发器矩阵执行。
3. [X] 本轮已补搜索型断言与定向测试结果；`288` 再统一补充截图/trace 与执行记录汇总。

## 8. 交付物
1. [X] 本计划文档：`docs/archive/dev-plans/287-librechat-dto-render-only-and-failure-in-bubble-closure-plan.md`。
2. [X] 失败同落点与 DTO-only 回归用例。
3. [X] 旧桥失败回执职责下线证据与搜索型 stopline 记录。

## 9. 关联文档
- `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/archive/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/archive/dev-plans/286-librechat-official-message-binding-and-single-bubble-closure-plan.md`
- `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `AGENTS.md`
