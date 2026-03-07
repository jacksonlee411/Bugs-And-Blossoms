# DEV-PLAN-271：Assistant/LibreChat 跨计划分阶段推进与封板编排计划（223/240/260/280）

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景与立项原因
1. [ ] `DEV-PLAN-240` 已完成 `M0/M1` 契约冻结，但 `M2+` 进入实现阶段前，需要与 `223/260/280/284` 建立可执行的分阶段并行策略，避免返工。
2. [ ] `DEV-PLAN-260` 已冻结 P0 DTO/FSM 契约，但真实 Case 1~4 仍依赖 `223` 事实源落地与 `266/284` UI 单通道收口。
3. [ ] `DEV-PLAN-280` 的子计划中，`281/282/283` 已完成，当前进入 `284` 正式 patch 与 `285` 封板阶段，需与 `240/260/223/237` 对齐 stopline。
4. [ ] 现有计划已具备“局部顺序”，但缺少“跨计划统一推进视图”（阶段入口/出口、并行泳道、回退路径、PR 节奏），导致执行时容易出现并行冲突或门禁口径不一致。

## 2. 目标与非目标

### 2.1 核心目标
1. [ ] 形成单一执行主链：`223 -> 240 + 266/284 -> 260 -> 237 + 285`。
2. [ ] 冻结并行边界：后端泳道（`223/240`）与前端泳道（`266/284`）可并行，但均只消费 `260` 冻结 DTO 字段与 guard。
3. [ ] 建立阶段出口条件：每阶段必须有“可验证产物 + 门禁 + 证据”后才能进入下一阶段。
4. [ ] 明确子计划拆分建议，降低单计划过大导致的合并冲突与回归半径。

### 2.2 非目标
1. [ ] 本计划不替代 `223/240/260/266/280/284/285/237` 的业务契约正文。
2. [ ] 本计划不新增第二事实源、第二写入口或 legacy 兼容链路。
3. [ ] 本计划不直接定义 schema 字段细节、UI patch 细节或模型提示词细节；这些仍由对应计划承接。

## 3. 当前状态快照（2026-03-07）
1. [X] `240`：`M0/M1` 已完成，`M2` 待启动。
2. [ ] `223`：进行中；`phase/DTO` 持久化落地（M2+）仍在推进。
3. [ ] `260`：规划中；P0 契约冻结完成，Case 1~4 真实验收未完成。
4. [ ] `266`：实施中；单通道与气泡内回写前置仍待收口。
5. [ ] `280`：规划中；`281/282/283` 已完成，`284/285` 待完成。
6. [ ] `284`：准备中；prep 完成，待进入正式 patch。
7. [ ] `285`：规划中；封板阶段尚未启动。
8. [ ] `237`：草拟中；升级与 source/runtime compatibility 回归待补齐。
9. [X] `235`：已完成；后续仅作为 `285` 复核项。

## 4. 总体推进策略（先稳后快）
1. [ ] **先稳契约再扩实现**：`223` 先提供可恢复 DTO 事实源，`240 M2+` 再推进核心编排改造。
2. [ ] **双泳道并行**：后端泳道（`223/240`）与前端泳道（`266/284`）并行推进，但以统一 DTO 与 stopline 收口。
3. [ ] **统一验收入口**：所有用户可见验收固定在 `/app/assistant/librechat`，拒绝以历史别名路径替代正式验收。
4. [ ] **封板后置**：`285` 只做封板与复核；发现缺口必须回退到原子子计划修复，不在封板阶段补设计。

## 5. 分阶段执行路线（S1-S6）

### 5.1 S1：事实源与接口壳先行（可立即启动）
1. [ ] `223-M2`：迁移 + sqlc + repository/store 落地，补齐 `current_phase` 与 turn 快照。
2. [ ] `240-M2`：先交 `ActionRegistry/CommitAdapter/OCC` 接口壳，不改变外部行为。
3. [ ] 出口条件：`GET conversation` 可稳定回放 DTO；无第二事实源。

### 5.2 S2：后端编排收敛
1. [ ] `240-M2/M3`：去写死与统一状态机（消除内存/PG 双路径漂移）。
2. [ ] `223-M3`：幂等、恢复、DTO rebuild 切换与租户绑定校验。
3. [ ] 出口条件：`confirm/commit` 统一走状态机，`plan_hash/version_tuple` 强校验有效。

### 5.3 S3：前端主链接管
1. [ ] `266-M2~M4`：收掉官方原始发送链路、统一气泡内回写。
2. [ ] `284` 正式 patch：send/store/render 接管、helper 正式职责失活。
3. [ ] 出口条件：单发送通道、官方消息树唯一落点、无外挂容器。

### 5.4 S4：业务闭环落地
1. [ ] `260-M2~M4`：FSM 推进、对话语义与承载面前置收口。
2. [ ] 出口条件：Case 2~4 在真实入口可运行，且无前端补算业务语义。

### 5.5 S5：质量收敛与回归扩面
1. [ ] `240-M4~M6`：风控左移、耐久执行、MCP 写能力治理闭环。
2. [ ] `260-M5`：Case 1~4 证据固化。
3. [ ] `237`：source/runtime compatibility 与升级回归补齐。
4. [ ] 出口条件：关键失败路径稳定错误码；升级回放可重放。

### 5.6 S6：封板
1. [ ] `285` 执行封板。
2. [ ] 出口条件：无双入口、无双消息落点、无旧桥回流、Case 1~4 证据齐全。

## 6. 并行与禁止并行矩阵
1. [ ] 可并行：`223-M2/M3` 与 `266/284` 的实现可并行。
2. [ ] 可并行：`240-M2`（接口壳）可与 `223-M2` 并行。
3. [ ] 禁止并行：`284` 正式 patch 与“前端本地重算 FSM/提交约束”不得并行存在。
4. [ ] 禁止并行：`240-M2+` 的行为改造与未冻结 DTO 字段语义不得并行。
5. [ ] 禁止并行：`285` 与“仍有旧桥正式职责”不得并行。

## 7. 子计划拆分建议（待创建）
1. [ ] `DEV-PLAN-271A`：`223` phase/DTO 持久化与恢复实现收口（承接 `223-M2/M3`）。
2. [ ] `DEV-PLAN-271B`：`240` ActionRegistry + CommitAdapter + OCC 落地（承接 `240-M2`）。
3. [ ] `DEV-PLAN-271C`：`240` 状态机统一与耐久任务编排收口（承接 `240-M3/M5`）。
4. [ ] `DEV-PLAN-271D`：`266 + 284` 单通道发送与官方消息树唯一落点联动收口。
5. [ ] `DEV-PLAN-271E`：`260` Case 1~4 真实验收与证据固化专项。
6. [ ] `DEV-PLAN-271F`：`237 + 285` 升级兼容与封板复核专项。

## 8. PR 与 worktree 推进节奏（建议）
1. [ ] `wt-dev-a`：后端泳道（`223/240`）连续推进，保持每 PR 只做一个阶段目标。
2. [ ] `wt-dev-b`：前端泳道（`266/284`）连续推进，优先清理 helper 正式职责。
3. [ ] `wt-dev-main`：整合 PR（`260` Case 验收、`237` 回归、`285` 封板证据）。
4. [ ] 每个阶段结束必须执行 `make preflight` 与对应专项门禁，再进入下阶段。

## 9. 停止线（Fail-Closed）
1. [ ] 任一阶段若出现“双发送、双回复、双入口、双事实源”，立即停止推进并回退到上阶段修复。
2. [ ] 若 `GET conversation` 无法稳定重建 DTO，不得进入 `284` 正式验收。
3. [ ] 若前端仍承载业务阶段推进或提交约束，不得宣称 `240/260/284` 达成。
4. [ ] 若升级回归未补齐，不得执行 `285` 封板。

## 10. 门禁与证据口径（SSOT 引用）
1. [ ] 触发器与本地必跑：`AGENTS.md`。
2. [ ] 命令入口：`Makefile`。
3. [ ] CI 门禁：`.github/workflows/quality-gates.yml` 与 `docs/dev-plans/012-ci-quality-gates.md`。
4. [ ] 执行记录文件：`docs/dev-records/dev-plan-271-execution-log.md`（后续创建并持续追加）。

## 11. 交付物
1. [ ] 主计划：`docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`。
2. [ ] 子计划（如采纳）：`271A~271F` 文档。
3. [ ] 里程碑证据：`223/240/260/266/284/237/285` 的阶段截图、测试记录、trace 与门禁结果。
4. [ ] 封板证据：`285` 统一收口记录。

## 12. 关联文档
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
