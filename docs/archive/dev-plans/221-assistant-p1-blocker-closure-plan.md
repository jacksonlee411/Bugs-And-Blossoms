# DEV-PLAN-221：Assistant P1 Blocker 收口实施计划（按 DEV-PLAN-003 细化）

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 实施完成（2026-03-02 07:10 UTC）

## 1. 背景与研究结论（Stage 1 / Research）
- `DEV-PLAN-220A` 已确认 P1 Blocker：状态机终态缺失、版本漂移回退缺失、候选固化不足、strict decode/边界违约错误码缺失。
- 当前目标是“先把提交链路收敛为可解释、可拒绝、可回退到安全态（validated）”，不引入 P2 编排。
- 现状约束：
  1. [ ] 不改变 assistant 路由入口与 capability 映射拓扑。
  2. [ ] 不引入 legacy 双链路/旧实现兜底（对齐 `DEV-PLAN-004M1`）。
  3. [ ] 不新增会话持久化迁移（由 `DEV-PLAN-223` 负责）。

## 2. 目标与非目标（Stage 2 / Plan）
### 2.1 目标（P1 必达）
1. [X] 补齐会话状态机终态（`canceled`/`expired`）与提交拒绝语义（`conversation_state_invalid`）。
2. [X] 落地提交前版本漂移检测（`policy/composition/mapping`）并 fail-closed 回退 `validated`。
3. [X] 固化候选主键：`confirmed` 后不可静默改写 `resolved_candidate_id`。
4. [X] 落地错误码：`ai_plan_schema_constrained_decode_failed`、`ai_plan_boundary_violation`。
5. [X] 用自动化证据关闭 `TC-220-BE-003/004/006/008/015`。
6. [ ] 明确与 222/223/224 的测试责任边界，避免 220 矩阵“无人认领”。

### 2.2 非目标（本计划明确不做）
1. [ ] 不实现会话持久化表结构、迁移与审计回放（`DEV-PLAN-223`）。
2. [ ] 不实现前端全量 E2E 收口（`DEV-PLAN-222`）。
3. [ ] 不引入任务 API / Temporal 工作流（`DEV-PLAN-225`）。

## 3. 边界、职责与不变量（Simple 核心）
### 3.1 边界定义
1. [ ] **状态机边界**：仅在 assistant conversation/turn 提交流程中生效。
2. [ ] **漂移检测边界**：仅比较 `policy/composition/mapping` 快照。
3. [ ] **候选固化边界**：仅约束同一 conversation + turn 的已确认候选。
4. [ ] **strict decode 边界**：仅约束 AI 输出解析与 capability 边界校验。

### 3.2 冻结不变量（必须始终成立）
1. [ ] 终态（`committed/canceled/expired`）不可再次 `:commit`。
2. [ ] `:commit` 仅允许在 `confirmed` 且“无漂移”时执行。
3. [ ] 同一 turn 一旦 `confirmed`，`resolved_candidate_id` 不可被二次确认改写。
4. [ ] schema 解析失败与 capability 边界违规必须 fail-closed。
5. [ ] 不新增任何 legacy fallback（旧链路、旧别名、read=legacy）。

## 4. 契约细化（接口/状态机/错误码）
### 4.1 状态机契约
1. [X] 有效主链：`validated -> confirmed -> committed`。
2. [X] 新增终态：`validated|confirmed -> canceled|expired`。
3. [X] 非法提交返回 `conversation_state_invalid`，且状态不变。

### 4.2 漂移检测契约
1. [X] `:confirm` 时固化快照：`policy_version`、`composition_version`、`mapping_version`。
2. [X] `:commit` 前比较快照与当前版本：任一不一致即拒绝提交。
3. [X] 拒绝动作原子执行：回退 `validated` + 返回“需重确认”错误。

### 4.3 候选固化契约
1. [X] `confirmed` 首次写入 `resolved_candidate_id` 后冻结。
2. [X] 二次 `:confirm`：同候选幂等成功，不同候选拒绝。

### 4.4 strict decode / boundary 契约
1. [X] schema 非法 -> `ai_plan_schema_constrained_decode_failed`。
2. [X] boundary 违规 -> `ai_plan_boundary_violation`。
3. [X] 错误码必须进入统一错误目录与前端映射。

## 5. 标准对齐（DEV-PLAN-005）
1. [ ] `STD-001`：继续使用 `request_id` / `trace_id`。
2. [ ] `STD-003`：命名单一权威表达，不增同义字段。
3. [ ] `STD-004`：不暴露版本噪音。
4. [ ] `STD-006`：API 未登录保持 401 口径。
5. [ ] `DEV-PLAN-004M1`：回滚与故障处置不允许 legacy 双链路。

## 6. 实施分解（确定性步骤）
### M1：契约冻结与拒绝路径先行
1. [X] 冻结状态机转移表、漂移字段、候选固化规则、错误码映射表。
2. [X] 先补失败路径测试（非法状态/漂移/候选改写/decode/边界违规）。

### M2：后端实现收口
1. [X] 在 `internal/server/assistant_api.go` 落地状态机终态与 commit 拒绝。
2. [X] 落地 `confirm snapshot` 与 `commit drift compare` 原子回退。
3. [X] 落地候选不可变校验与 strict decode/boundary 错误映射。

### M3：契约测试与回归闭环
1. [X] 对齐 `TC-220-BE-003/004/006/008/015` 自动化。
2. [X] 回归 assistant 既有测试，确认无行为回退。
3. [X] 产出执行记录到 `docs/dev-records/`。

## 7. 失败路径与恢复策略（No-Legacy）
1. [ ] 触发条件：误拒绝、状态机误转移、错误码错映射。
2. [ ] 处置顺序：环境级保护（限制 `:commit`）→ 只读/停写 → 修复 → 重试/重放 → 恢复。
3. [ ] 禁止措施：不得回退旧提交实现，不得双链路兜底。
4. [ ] 恢复判定：目标用例 + `make preflight` 全绿后解除保护。

## 8. 门禁与证据（触发器对齐 AGENTS）
1. [ ] Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`。
2. [ ] 路由与 capability：`make check routing && make check capability-route-map`。
3. [ ] 鉴权：`make authz-pack && make authz-test && make authz-lint`。
4. [ ] 错误提示契约：`make check error-message`。
5. [ ] PR 前总闸：`make preflight`。
6. [ ] 覆盖率口径：遵循仓库 100% 覆盖门禁。

## 9. 验收标准（含 DEV-PLAN-003“简单性”）
1. [X] Blocker 全关闭：状态机终态、漂移回退、候选固化、strict decode/boundary 错误码全部可测可证。
2. [ ] 可替换性：改动局限在 assistant 边界，不影响无关模块。
3. [ ] 局部性：新增需求主要影响状态机/错误映射/测试三处。
4. [ ] 可解释性：5 分钟内可讲清主流程 + 失败路径 + 恢复步骤。
5. [ ] 门禁全绿且无 legacy/no-route/no-authz 漂移。

## 10. 跨计划覆盖映射（与 220 对齐）
1. [X] `TC-220-BE-003/004/006/008/015`：由本计划（221）负责关闭。
2. [ ] `TC-220-FE-001~007`、`TC-220-E2E-001~008`、`TC-220-E2E-101~104`：由 222 负责关闭。
3. [ ] `TC-220-BE-009/011` 与审计证据：由 223 负责关闭。
4. [ ] `TC-220-BE-010`（LibreChat 越权阻断）与多模型链路：由 224 负责关闭。
5. [ ] `TC-220-BE-012`、`TC-220-TMP-001~006`：由 225 负责关闭。

## 11. 交付物
1. [X] assistant API 与测试代码改动（PR 载体）。
2. [X] `DEV-PLAN-221` 执行记录文档。
3. [X] 与 `DEV-PLAN-220A` 缺口项逐条关闭映射表。

## 12. 关联文档
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/archive/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
- `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/archive/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/archive/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `AGENTS.md`
