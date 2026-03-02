# DEV-PLAN-221：Assistant P1 Blocker 收口实施计划（状态机 × 漂移回退 × Strict Decode）

**状态**: 规划中（2026-03-02 05:37 UTC）

## 1. 背景
- `DEV-PLAN-220A` 已识别 P1 阶段存在 Blocker：状态机终态缺失、版本漂移回退缺失、候选固化不足、strict decode/边界违约错误码缺失。
- 本计划聚焦“先让提交链路合规”，不引入 P2 异步编排能力。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 补齐会话状态机终态与提交拒绝语义（`canceled/expired`、`conversation_state_invalid`）。
2. [ ] 落地提交前版本漂移检测（`policy/composition/mapping`）与 fail-closed 回退 `validated`。
3. [ ] 固化候选主键：`confirmed` 后不可静默改写 `resolved_candidate_id`。
4. [ ] 落地 `ai_plan_schema_constrained_decode_failed` 与 `ai_plan_boundary_violation` 错误码链路。

### 2.2 非目标
1. [ ] 不在本计划引入会话持久化迁移（由 `DEV-PLAN-223` 负责）。
2. [ ] 不在本计划引入 E2E 全量落地（由 `DEV-PLAN-222` 负责）。

## 3. 实施范围
- 后端：`internal/server/assistant_api.go`、相关 handler/service 测试。
- 错误映射：统一错误目录与前端映射契约（仅新增本计划必需错误码）。
- 路由与授权：保持现有 assistant 路由与 capability 映射不漂移。

## 4. 实施步骤
1. [ ] 扩展状态机：定义 `canceled/expired` 终态、终态提交拒绝分支与标准错误码。
2. [ ] 引入版本快照对比：提交前比较 `policy/composition/mapping`；不一致即回退 `validated` 并要求重确认。
3. [ ] 固化候选协议：`confirmed` 后二次确认仅允许幂等，不允许改写目标候选。
4. [ ] strict decode + boundary lint：
   - 输入/输出 schema 不合法 -> `ai_plan_schema_constrained_decode_failed`
   - capability 未注册/边界违规 -> `ai_plan_boundary_violation`
5. [ ] 补齐单测与契约测试（拒绝路径优先）。

## 5. 测试与验收
1. [ ] 对齐 `TC-220-BE-003/004/006/008/015` 自动化测试。
2. [ ] 回归已存在助手测试与质量门禁（门禁入口以 `AGENTS.md` 与 `Makefile` 为准）。
3. [ ] 验收标准：P1 Blocker 项全部关闭，且无路由/权限/错误码回归。

## 6. 风险与缓解
- **状态机分支增多导致回归**：先补拒绝路径测试再改实现（TDD）。
- **错误码扩散/文案漂移**：统一走错误目录与 error-message 门禁。
- **漂移检测误报**：采用最小可解释字段对比，并提供日志证据。

## 7. 交付物
1. [ ] 代码与测试改动（assistant API + 测试）。
2. [ ] `DEV-PLAN-221` 执行记录文档（实施时新增到 `docs/dev-records/`）。

## 8. 关联文档
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `AGENTS.md`
