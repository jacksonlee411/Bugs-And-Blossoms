# DEV-PLAN-283：LibreChat 正式入口直接切换实施计划

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-07 CST；导航、真实入口与 E2E 验收口径已统一到 `/app/assistant/librechat`）

## 1. 背景
- 承接 `DEV-PLAN-280` 的子计划拆分与执行顺序约束：`283` 负责“正式入口唯一化”，不再沿用“新旧正式入口并存”的迁移口径。
- `DEV-PLAN-281` 已冻结 vendored Web UI 来源、patch stack 与静态产物目录；`DEV-PLAN-282` 已将旧 `/app/assistant/librechat` 降为历史入口占位页并下线旧桥接正式职责。
- 因此 `283` 的任务不是继续讨论是否切换，而是冻结“切到哪里、静态资源从哪里出、旧入口切换后如何拒绝/跳转、什么证据算切换完成”。

## 2. 目标与非目标
### 2.1 目标
1. [X] 将 `/app/assistant/librechat` 冻结为 LibreChat 正式用户交互唯一入口。
2. [X] 将 `/assets/librechat-web/**` 冻结为正式入口唯一静态资源前缀，并与历史 `/assistant-ui/*` 代理解耦。
3. [X] 将历史 `/assistant-ui/*` 与旧 `/app/assistant` 工作台从“正式业务交互入口”语义中彻底移除。
4. [X] 冻结切换后的失败语义、路径边界与最小验收证据，避免实现阶段再发明第二套入口口径。

### 2.2 非目标
1. [ ] 不在本计划内定义会话/租户边界算法细节；相关算法与错误码口径以 `DEV-PLAN-235` 为 SSOT。
2. [ ] 不在本计划内定义业务 FSM、回执 DTO 与审计状态流转语义；相关业务事实源以 `DEV-PLAN-223`、`DEV-PLAN-260` 为 SSOT。
3. [ ] 不在本计划内完成发送/渲染主链路源码级接管；该部分由 `DEV-PLAN-284` 承接。
4. [ ] 不在本计划内承担全量升级回归封板；全量回归闭环与封板由 `DEV-PLAN-285` 承接。

## 3. 边界冻结与不变量
### 3.1 路径边界冻结
```yaml
formal_entry_prefixes:
  - /app/assistant/librechat
formal_static_prefixes:
  - /assets/librechat-web
historical_alias_prefixes:
  - /assistant-ui
historical_audit_prefixes:
  - /app/assistant
```

约束：
1. [X] `/app/assistant/librechat` 是唯一正式聊天交互入口；导航、文档、E2E、人工验收口径必须一致指向该路径。
2. [X] `/assets/librechat-web/**` 仅服务 vendored LibreChat Web UI 静态资源；正式入口可用性不得依赖 `/assistant-ui/*` 代理。
3. [X] `/assistant-ui/*` 在 `283` 完成后仅允许保留为历史别名/调试语义，不再承担正式聊天交互或正式验收职责。
4. [X] `/app/assistant` 若继续存在，仅允许承载日志/审计/运行态等非正式聊天职责；不得重新挂回正式聊天承载面。

### 3.2 不变量
1. [X] 同一时刻只能存在一个正式用户可见聊天入口，不得把双入口解释为“平滑迁移”。
2. [X] 正式入口、正式静态资源前缀、历史别名三者职责必须互斥，不得出现第二套权威表达。
3. [X] 正式入口切换不得回流 `iframe`、`bridge.js`、HTML 注入、页面级业务编排等已由 `DEV-PLAN-282` 退役的复杂度。
4. [X] 业务事实源仍以本仓 `conversation_id/turn_id/request_id/trace_id` 及其审计状态流转为准；`283` 只切换入口，不改变事实源主从关系。

## 4. 顺序与 readiness
1. [X] `DEV-PLAN-281` 已完成；否则不存在已冻结的新主链路与静态产物出口。
2. [X] `DEV-PLAN-282` 已完成“旧桥正式职责去除”；否则 `283` 会退化成双入口并存。
3. [X] `DEV-PLAN-235` 已补齐 `/app/assistant/librechat` 与 `/assets/librechat-web/**` 的会话/租户边界，且不再依赖 `/assistant-ui/*` 代理充当正式入口边界。
4. [X] `DEV-PLAN-223` 与 `DEV-PLAN-260` 已冻结正式业务交互所需的业务事实源与 FSM/DTO 契约；否则不得宣称“正式业务入口已切换完成”。
5. [X] vendored UI 构建、静态发布与入口 handler 已具备最小可运行闭环。
6. [X] `283` 与旧入口正式职责不可并行存在；一旦进入切换，旧入口只能被重定向、降级为历史别名/调试入口，或直接删除。

## 4.1 切换后行为矩阵（失败语义冻结）
| 场景 | 输入 | 期望结果 |
| --- | --- | --- |
| 未登录访问正式入口 | `GET /app/assistant/librechat` 且无有效 SID | `302 -> /app/login` |
| 未登录访问正式静态资源 | `GET /assets/librechat-web/**` 且无有效 SID | `302 -> /app/login` |
| SID 无效 / 跨租户 / 主体失效 | 访问正式入口或正式静态资源 | 清理 SID + `302 -> /app/login` |
| 已登录同租户访问正式入口 | `GET /app/assistant/librechat` | 返回 vendored LibreChat Web UI 入口页面 |
| 已登录同租户访问正式静态资源 | `GET /assets/librechat-web/**` | 返回正式入口所需静态资源 |
| 访问历史别名 | `GET/HEAD /assistant-ui` 或 `/assistant-ui/**` | 不再进入正式代理主链路；统一 `302 -> /app/assistant/librechat`，并仅保留历史别名语义 |
| 非允许方法访问历史别名 | `POST/PUT/PATCH/DELETE /assistant-ui/**` | `405`，不得借历史别名恢复正式职责 |
| 访问旧 `/app/assistant` | `GET /app/assistant` | 若页面保留，仅展示日志/审计/运行态；不得出现正式聊天交互入口文案与操作 |

说明：
1. [X] 上表冻结的是 `283` 完成后的对外语义；若实现选择“删除历史别名”而非“302 重定向”，必须先更新本计划再改代码。
2. [X] 历史别名的一切保留都属于过渡性调试/历史入口语义，不得重新扩展为用户正式路径。

## 4.2 禁止项
1. [ ] 禁止切换后继续保留两个正式入口，并把其解释为“平滑迁移”或“灰度观察”。
2. [ ] 禁止旧 `/assistant-ui/*` 或旧 `/app/assistant` 工作台继续承担正式验收入口职责。
3. [ ] 禁止正式入口静态资源仍依赖 `/assistant-ui/*` 代理、HTML rewrite 或桥接脚本。
4. [ ] 禁止文档、导航、E2E、人工验收、错误提示口径不一致。
5. [ ] 禁止在 `283` 落地过程中重新引入 legacy 双链路、双文案、双通过口径。

## 5. 实施步骤
1. [X] 将 `/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*`、`/app/assistant` 的职责边界落成代码字面量与路由配置。
2. [X] 将导航、页面入口、正式验收入口、测试说明与用户说明统一切到 `/app/assistant/librechat`。
3. [X] 启用正式入口 handler，使 vendored UI 与 `/assets/librechat-web/**` 形成不依赖 `/assistant-ui/*` 的主链路。
4. [X] 将历史 `/assistant-ui/*` 收敛为历史别名：仅保留 `302 -> /app/assistant/librechat` 或经文档批准的更严格拒绝语义，不再承载正式代理主链路。
5. [X] 保持 `/app/assistant` 为日志/审计/运行态页，或在后续计划中显式退役；无论哪种方式，都不得再承担正式聊天交互。
6. [X] 补齐正式入口最小 E2E 与路径边界验证，证明“真实入口、测试入口、文档入口”三者一致。
7. [X] 清理双入口、双文案、双通过口径与历史文档残留。

## 5.1 搜索型 stopline
1. [X] 完成 `283` 后，搜索“正式入口”“验收入口”“LibreChat 入口”等口径时，仓库中只能把 `/app/assistant/librechat` 描述为正式聊天入口。
2. [X] 完成 `283` 后，搜索 `/assistant-ui` 时，只允许出现“历史别名”“调试/审计”“重定向”“归档证据”语义；不得再出现“正式入口”“正式链路”“主验收入口”语义。
3. [X] 完成 `283` 后，搜索 `/app/assistant` 时，只允许出现日志/审计/运行态页语义；不得出现“正式聊天承载面”“发送入口”“业务交互主路径”语义。

## 5.2 与 `DEV-PLAN-285` 的边界
1. [X] `283` 负责“入口唯一化 + 最小路径边界验证 + 文档口径统一”。
2. [X] `285` 负责“全量回归、归档一致性复核、封板证据收口”。
3. [X] `283` 未完成前，`285` 不得宣称正式切换已封板；`283` 完成后，也不得跳过 `285` 直接宣称全量回归完成。

## 6. 归档前置校验（交给 `285` 执行物理归档）
1. [ ] 入口切换完成后，确认以下文档只保留历史证据语义，不再出现“正式入口/正式链路”描述：
   - `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
   - `docs/archive/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
   - `docs/archive/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md`
   - `docs/archive/dev-plans/239b-239a-direct-validation-report-and-implementation-gaps.md`
   - `docs/archive/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
2. [X] 上述文档的物理迁移（`docs/dev-plans -> docs/archive/dev-plans`）已完成；`DEV-PLAN-285` 仅保留一致性复核。

## 7. 验收标准
1. [X] 不再存在两个同时承担正式职责的 UI 入口。
2. [X] `/app/assistant/librechat` 已成为唯一正式聊天入口，且 `/assets/librechat-web/**` 已成为唯一正式静态资源前缀。
3. [X] 正式入口与静态资源前缀的会话/租户边界已按 `DEV-PLAN-235` 收口，不再依赖 `/assistant-ui/*` 代理才能成立。
4. [X] 历史 `/assistant-ui/*` 若仍存在，只呈现历史别名/调试语义；不得再承担正式验收职责。
5. [X] `/app/assistant` 若仍存在，只呈现日志/审计/运行态语义；不得再出现正式聊天交互入口。
6. [X] 用户真实入口、测试入口、文档入口三者口径一致。
7. [X] 至少提供一条正式入口 smoke E2E 与一组路径边界验证证据；全量回归由 `DEV-PLAN-285` 承接。
8. [X] `make check doc` 通过；若本计划落地命中 Go/路由/E2E 变更，再按 `AGENTS.md` 命中的触发器矩阵补齐对应验证。

## 7.1 本轮落地证据（2026-03-07）
1. [X] 前端导航统一：`AI 助手` 顶层导航已改为 `/assistant/librechat`，并通过整页跳转进入 `/app/assistant/librechat`。
2. [X] 真实入口统一：`/app/assistant/librechat` 由服务端正式入口 handler 承载，静态资源仅走 `/assets/librechat-web/**`。
3. [X] 历史别名收口：`GET/HEAD /assistant-ui/*` 统一 `302 -> /app/assistant/librechat`；非允许方法保持 `405`。
4. [X] E2E 验收口径统一：新增 `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`，并将 `tp220-e2e-007` 入口改为 `/app/assistant/librechat`。
5. [X] 文档门禁：`make check doc` 已通过。

## 8. 关联文档
- `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/archive/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/archive/dev-plans/281-librechat-web-ui-source-vendoring-and-mainline-freeze-plan.md`
- `docs/archive/dev-plans/282-librechat-old-bridge-deletion-plan.md`
- `docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
