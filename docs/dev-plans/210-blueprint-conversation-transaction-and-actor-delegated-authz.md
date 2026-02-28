# DEV-PLAN-210：200蓝图 Phase 4 会话事务提交与委托授权同构收口

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
该计划把 AI 从“只读规划”推进到“可提交但可控”，核心是不改变授权本质：AI 代操必须等价于人工直操。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 落地会话状态机（draft/proposed/validated/confirmed/committed）与幂等重试约束。
2. [ ] 落地提交前实时 re-auth，阻断角色漂移与快照过期。
3. [ ] 冻结 Casbin 顺序：Actor Bind -> MapRouteToObjectAction -> Require -> Pre-Commit Re-Auth -> One Door。
4. [ ] 验证 AI/UI 同 actor 同输入下 allow/deny 与错误码一致。

### 2.2 非目标
1. [ ] 不把业务授权裁决迁移到 Temporal。
2. [ ] 不引入 AI 独立写入口或 ai_* 语义。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 4；里程碑映射：M10/M10A/M10B/M10B1/M10C。
- 输入依赖：DEV-PLAN-208/209、DEV-PLAN-206。
- 后续输出依赖：DEV-PLAN-211/212。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-001`：会话提交链路统一 `request_id/trace_id` 协议。
[ ] `STD-004`：AI/UI 同构提交，不新增 ai 专用业务写链路。
[ ] `STD-012`（Authz 顺序与统一拒绝，承接 DEV-PLAN-022）：403 合同与执行顺序冻结。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 实现会话状态机持久化与 checkpoint 迁移规则。
2. [ ] 实现 Pre-Commit Re-Auth Gate 与快照过期/角色漂移拒绝路径。
3. [ ] 统一 AI 与 UI 命令物化为同构提交命令（intent/request_id/trace_id/version）。
4. [ ] 补齐授权回归：系统配置管理员/HR/员工/经理 AI 代操 = 人工直操。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make authz-pack && make authz-test && make authz-lint`
  - [ ] `make test`
  - [ ] `make check no-legacy`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] 未 confirmed 状态禁止提交。
2. [ ] 提交瞬时授权复核强制执行且可审计。
3. [ ] AI/UI 提交结果等价且可回放。

## 8. 风险与缓解
1. [ ] 状态机复杂度上升。缓解：显式状态图 + 终态不可恢复规则。
2. [ ] 授权复核引入时延。缓解：只在 commit 瞬间做强校验。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m10-conversation-transaction-evidence.md`、`docs/dev-records/dev-plan-200-m10b-actor-delegated-authz-evidence.md`、`docs/dev-records/dev-plan-200-m10c-ai-ui-equivalent-execution-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
