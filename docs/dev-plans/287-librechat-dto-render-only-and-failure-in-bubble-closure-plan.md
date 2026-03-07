# DEV-PLAN-287：266 剩余项 B——失败回包同落点与前端 DTO-only 降权收口

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `DEV-PLAN-266` 尚未完成“成功/失败同落点”与“前端不补算业务语义”两条 stopline。
2. [ ] 若失败态仍可回落外挂提示、或前端继续本地重算 phase/候选/确认约束，则 `266` 与 `223/260/284` 口径不一致。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 将成功与失败场景统一到官方消息树同一落点体系。
2. [ ] 失败时仅保留“官方消息流内技术态提示/可重试状态”，不再输出页面外 notice/overlay。
3. [ ] 完成前端降权：渲染层只消费后端 DTO，不在 helper/store/adapter 重新推导业务语义。

### 2.2 非目标
1. [ ] 不承担“同轮唯一气泡映射”主实现（由 `DEV-PLAN-286` 承接）。
2. [ ] 不承担真实入口证据封板与归档（由 `DEV-PLAN-288/285` 承接）。
3. [ ] 不更改 `223/260` 的业务语义定义。

## 3. 266 剩余项映射（本计划承接）
1. [ ] `266 §6.3-4`：错误场景与正常场景使用同一消息落点，并保留同轮追溯映射。
2. [ ] `266 §6.3-6`：消息落点接管只做绑定/显示，不在 UI 层补算业务含义。
3. [ ] `266 §6.4-3`：失败时只允许官方消息流内技术态失败提示或可重试状态。
4. [ ] `266 §7-8`：若仍依赖注入脚本/DOM hack/HTML rewrite 为正式主链路则不得通过。
5. [ ] `266 §7-9`：若前端仍基于局部状态补算业务 phase/候选/确认约束则不得通过。

## 4. 顺序与 readiness
1. [ ] `DEV-PLAN-284` 正式 patch 已接管 send/store/render 主链路。
2. [ ] `DEV-PLAN-286` 已提供稳定消息锚点，避免失败态收口时引入双泡回归。
3. [ ] 通过本计划前，不得将 `266` 状态更新为“已完成”。

## 5. 实施步骤
1. [ ] 统一错误写回入口：失败回复走官方消息树同一 assistant message 更新链路。
2. [ ] 下线外挂失败显示：删除/失活页面外 notice、overlay、bridge 专属失败回执职责。
3. [ ] DTO-only 审计：梳理并清理前端本地 phase/候选/确认推导逻辑，只保留 DTO 显示与事件分发。
4. [ ] 增补守卫：加入搜索型 stopline 与测试断言，阻断 DOM 注入回流与前端业务重算回流。
5. [ ] 补齐回归：覆盖失败首轮、失败重试、失败后恢复成功等路径。

## 6. 验收标准
1. [ ] 成功/失败均在官方消息树内显示，且失败不落页面外挂容器。
2. [ ] 失败路径可重试但不绕回旧桥接链路。
3. [ ] 前端代码面可证明“只消费 DTO，不补算业务语义”。
4. [ ] 若任一断言失败，直接 fail-closed。

## 7. 测试与门禁（SSOT 引用）
1. [ ] 文档门禁：`make check doc`。
2. [ ] 前端与 E2E 回归按 `AGENTS.md` 触发器矩阵执行。
3. [ ] 相关搜索型断言、用例结果与截图/trace 写入 `docs/dev-records/dev-plan-266-execution-log.md`。

## 8. 交付物
1. [ ] 本计划文档：`docs/dev-plans/287-librechat-dto-render-only-and-failure-in-bubble-closure-plan.md`。
2. [ ] 失败同落点与 DTO-only 回归用例。
3. [ ] 旧桥失败回执职责下线证据与搜索型 stopline 记录。

## 9. 关联文档
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/286-librechat-official-message-binding-and-single-bubble-closure-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `AGENTS.md`
