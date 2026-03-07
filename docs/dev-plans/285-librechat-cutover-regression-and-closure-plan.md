# DEV-PLAN-285：LibreChat 切换回归闭环与封板实施计划

**状态**: 规划中（2026-03-07 23:55 CST）

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280E`。
- 本计划负责切换后的总体验收：`260` Case 1~4、`266` 单通道、`235` 新入口边界、`237` source/runtime compatibility 回归全部通过后，才能正式封板。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 在新承载面上完成 `260` Case 1~4 真实回归闭环。
2. [ ] 证明 `266` 的单通道、气泡内回写、无外挂容器要求在新结构下仍成立。
3. [ ] 证明 `235` 的正式入口会话/租户边界成立。
4. [ ] 证明 `237` 的 vendored UI source + patch stack + runtime compatibility 回归已接入。
5. [ ] 完成封板：仓库内不再存在旧桥接方案的剩余正式职责。

### 2.2 非目标
1. [ ] 不新增新业务能力。
2. [ ] 不以“局部通过”替代整体 cutover 验收。

## 2.3 顺序与 readiness
1. [ ] `285` 是封板子计划，只能在 `281/282/283/284` 均达到各自 DoD 后启动。
2. [ ] `235` 与 `237` 的对应 stopline 必须在 `285` 执行前完成接线。
3. [ ] `285` 期间若发现旧入口、旧桥接职责或双入口回流，必须回退到对应子计划修复，而不是带缺口封板。

## 2.4 禁止项与封板红线
1. [ ] 禁止以“主要场景能跑”为理由跳过旧入口残留、双入口、双回执、旧桥职责回流等封板红线。
2. [ ] 禁止在 `285` 阶段重新启用旧桥接链路做临时兜底。
3. [ ] 禁止出现“测试通过，但文档与人工验收仍保留旧口径”的封板错位。

## 2.5 搜索型 stopline
1. [ ] 封板时，搜索结果中不应再把历史 `/assistant-ui/*`、旧工作台、bridge/iframe 口径写成正式承载结构。
2. [ ] 封板时，搜索结果中不应再存在“旧桥接方案继续维护”的主口径说明。
3. [ ] 封板证据必须能证明：无双入口、无双消息落点、无旧桥正式职责、无旧测试口径回流。

## 3. 实施步骤
1. [ ] 运行 `260/266` 相关真实页面回归集。
2. [ ] 运行 `235` 相关入口边界与负测。
3. [ ] 运行 `237` 相关 source/runtime compatibility 回归。
4. [ ] 产出封板证据：无双入口、无双回执、无旧桥正式职责、无旧口径测试回流。

## 4. 验收标准
1. [ ] `260` Case 1~4 全部通过。
2. [ ] `266` stopline 全部通过。
3. [ ] `235` 新正式入口边界全部通过。
4. [ ] `237` 升级回归要求全部通过。
5. [ ] 封板时不存在旧桥接方案的正式职责残留。
6. [ ] `make check doc` 通过。

## 5. 关联文档
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `AGENTS.md`
