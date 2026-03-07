# DEV-PLAN-283：LibreChat 正式入口直接切换实施计划

**状态**: 规划中（2026-03-07 23:55 CST）

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280C`。
- 本计划目标是让新 LibreChat Web UI 承载面成为唯一正式用户入口，并切断旧 `/assistant-ui/*` 与旧 `/app/assistant` 工作台的正式交互职责。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 新正式入口成为唯一用户可见业务交互入口。
2. [ ] 历史 `/assistant-ui/*` 与旧 `/app/assistant` 不再承担正式业务交互职责。
3. [ ] 若历史入口短期保留，仅允许调试/审计用途，不得作为正式验收入口。

### 2.2 非目标
1. [ ] 不在本计划内定义会话/租户边界算法细节（由 `DEV-PLAN-235` 提供 SSOT）。
2. [ ] 不在本计划内定义业务 FSM 语义（由 `DEV-PLAN-260` 提供 SSOT）。

## 3. 顺序与 readiness
1. [ ] `283` 只能在 `281` 完成后启动；否则没有冻结的新主链路可切。
2. [ ] `DEV-PLAN-235` 已补齐新正式入口的会话/租户边界。
3. [ ] `DEV-PLAN-282` 已清理旧桥接的正式职责。
4. [ ] 新入口构建与静态发布链路已就绪。
5. [ ] `283` 与旧入口正式职责不可并行存在；进入切换后，旧入口只能降级为调试/审计用途或直接删除。

## 3.1 禁止项
1. [ ] 禁止切换后继续保留两个正式入口并把其解释为“平滑迁移”。
2. [ ] 禁止旧 `/assistant-ui/*` 或旧 `/app/assistant` 工作台继续承担正式验收入口职责。
3. [ ] 禁止文档、导航、E2E、人工验收口径不一致。

## 3.2 搜索型 stopline
1. [ ] 完成 `283` 后，正式入口说明、导航文案、测试入口说明应只指向当前真实入口。
2. [ ] 完成 `283` 后，旧入口若仍存在，搜索结果中必须只出现“调试/审计/历史别名”语义，而不再出现“正式入口”语义。

## 4. 实施步骤
1. [ ] 切换导航、页面入口、正式验收入口与相关说明文案。
2. [ ] 将历史 `/assistant-ui/*` 与旧 `/app/assistant` 明确降级为调试/审计角色，或直接移出正式入口链路。
3. [ ] 补齐正式入口的 E2E 与路径边界验证。
4. [ ] 清理双入口、双文案、双通过口径残留。

## 4.1 归档前置校验（交给 285 执行物理归档）
1. [ ] 入口切换完成后，确认以下文档只保留历史证据语义，不再出现“正式入口/正式链路”描述：
   - `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
   - `docs/archive/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
   - `docs/archive/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md`
   - `docs/archive/dev-plans/239b-239a-direct-validation-report-and-implementation-gaps.md`
   - `docs/archive/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
2. [X] 上述文档的物理迁移（`docs/dev-plans -> docs/archive/dev-plans`）已完成；`DEV-PLAN-285` 仅保留一致性复核。

## 5. 验收标准
1. [ ] 不再存在两个同时承担正式职责的 UI 入口。
2. [ ] 用户真实入口、测试入口、文档入口三者口径一致。
3. [ ] 历史入口若仍存在，不再承担正式验收职责。
4. [ ] `make check doc` 通过。

## 6. 关联文档
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `AGENTS.md`
