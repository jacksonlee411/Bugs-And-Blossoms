# DEV-PLAN-284：LibreChat 发送与渲染主链路源码级接管实施计划

**状态**: 规划中（2026-03-07 23:55 CST）

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280D`。
- 本计划聚焦 send/store/render 三个控制点的源码级接管，完成“前端降权、后端 DTO 驱动、官方消息树唯一渲染面”的收口。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 在 vendored UI 的发送 action / store / renderer 中接入本仓主链路。
2. [ ] 让缺字段、多候选、确认、提交成功/失败全部进入官方消息列表与 assistant message 实体。
3. [ ] 完成前端降权：页面 helper / adapter 不再承载业务 phase 推进、候选裁决或提交约束。

### 2.2 非目标
1. [ ] 不在本计划内重定义业务事实源与 FSM 语义（以 `223/260` 为 SSOT）。
2. [ ] 不在本计划内定义升级/回滚策略（由 `237` 承接）。

## 3. 顺序与 readiness
1. [ ] `284` 只能在 `281` 完成后启动。
2. [ ] `DEV-PLAN-223` 已明确业务事实源字段与回放口径。
3. [ ] `DEV-PLAN-260` 已冻结 phase / candidates / draft / commit-reply DTO 契约。
4. [ ] `DEV-PLAN-283` 已完成正式入口切换，不再依赖旧桥接链路承载发送与回执。
5. [ ] `284` 不应与“旧桥接仍承担正式职责”的状态并行存在。

## 3.1 禁止项
1. [ ] 禁止把旧的页面 helper 业务编排逻辑简单搬运到 vendored UI 中继续存活。
2. [ ] 禁止前端根据文本、局部上下文或 UI 临时状态重新计算 phase、候选裁决或提交约束。
3. [ ] 禁止继续通过 DOM 注入、外挂容器、第二消息流承载用户可见业务回执。

## 3.2 搜索型 stopline
1. [ ] 完成 `284` 后，正式用户可见业务职责不应再由 `assistantDialogFlow`、`assistantAutoRun` 或等价 helper 承担。
2. [ ] 完成 `284` 后，不应再存在 `document.createElement(...)` 式外挂消息流作为正式回执落点的实现口径。
3. [ ] 完成 `284` 后，前端消费契约应可清晰定位为后端 `phase/candidates/draft/commit-reply` DTO，而非散落的本地判定逻辑。

## 4. 实施步骤
1. [ ] 接管发送 action：阻止旧的页面级业务编排继续推进状态。
2. [ ] 接管消息渲染：业务回执全部进入官方消息组件树。
3. [ ] 删除或失活页面 helper 对业务状态推进、候选解析、确认约束的正式职责。
4. [ ] 增加源码级单测/组件测试，覆盖 send/store/render 关键路径。

## 5. 验收标准
1. [ ] 前端只消费后端 DTO，不再重算业务语义。
2. [ ] 官方消息树是唯一用户可见渲染面。
3. [ ] 不再存在页面级 helper 承担正式业务推进职责。
4. [ ] `make check doc` 通过。

## 6. 关联文档
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `AGENTS.md`
