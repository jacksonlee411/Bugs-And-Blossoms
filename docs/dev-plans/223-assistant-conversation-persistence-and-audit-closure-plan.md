# DEV-PLAN-223：Assistant 会话持久化与审计闭环实施计划

**状态**: 规划中（2026-03-02 05:37 UTC）

## 1. 背景
- `DEV-PLAN-220A` 指出当前 assistant 会话为内存态，重启丢失，无法满足 P1 回放、幂等与审计可追踪要求。
- 本计划聚焦“最小持久化闭环”，保持 One Door 与现有授权边界不变。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 设计并落地会话/回合/状态转移/幂等最小持久化模型。
2. [ ] 支持 `tenant_id/actor_id/conversation_id/turn_id/request_id/trace_id` 全链路追踪。
3. [ ] 支持会话恢复与提交幂等验证，不因进程重启丢失上下文。

### 2.2 非目标
1. [ ] 不在本计划引入 Temporal 任务模型（由 `DEV-PLAN-225` 负责）。
2. [ ] 不扩展业务能力面，仅保障 assistant 事务链路。

## 3. 前置与硬闸门
1. [ ] 若涉及新增表或 `CREATE TABLE`，必须先获得用户书面确认后再执行迁移。
2. [ ] 迁移与 schema 门禁遵循仓库 SSOT（`AGENTS.md`、`Makefile`、CI 门禁）。

## 4. 实施范围
- 数据层：assistant 会话最小表集合与查询接口。
- 服务层：内存 map -> 持久化 store；提交幂等判定与恢复。
- 审计层：关键追踪字段完整落盘与查询可见。

## 5. 实施步骤
1. [ ] 冻结数据契约：表结构、唯一键、索引、状态转移约束、审计字段。
2. [ ] 评审并获得用户确认后，执行迁移与 lint。
3. [ ] 改造 assistant service 为持久化读写路径，保留 fail-closed 行为。
4. [ ] 补齐幂等约束（`conversation_id + turn_id + request_id`）与冲突处理策略。
5. [ ] 落地恢复与回放接口验证（重启后会话连续性）。
6. [ ] 补齐执行证据文档：
   - `docs/dev-records/dev-plan-220-execution-log.md`
   - `docs/dev-records/dev-plan-220-m0-chat-readonly-evidence.md`
   - `docs/dev-records/dev-plan-220-m1-conversation-commit-evidence.md`

## 6. 测试与验收
1. [ ] 后端测试覆盖持久化分支、幂等分支、恢复分支。
2. [ ] 对齐 `TC-220-BE-009` 与审计回放相关验收。
3. [ ] 验收标准：服务重启后仍可查询会话历史并保持提交幂等。

## 7. 风险与缓解
- **迁移风险**：先做最小模型，避免一次性大表扩张。
- **一致性风险**：提交与状态转移必须同事务提交。
- **性能风险**：仅新增必要索引，压测后再扩展。

## 8. 交付物
1. [ ] assistant 持久化 schema 与代码改造。
2. [ ] 对应测试、门禁与 evidence 文档。
3. [ ] `DEV-PLAN-223` 执行记录文档（实施时新增到 `docs/dev-records/`）。

## 9. 关联文档
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
- `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
- `AGENTS.md`
