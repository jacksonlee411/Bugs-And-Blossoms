# DEV-PLAN-204：200蓝图 Phase 1 组合 DTO、Explain 与版本快照协议

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
该计划把 200 第 7 节 DTO 契约转为实现级接口定义，确保“可解释 + 可回放 + 可比对”。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 落地 `PageCompositionSnapshot/IntentDecisionSnapshot/ComposedFieldDecision/AllowedValueDecision`。
2. [ ] 冻结 explain 结构与版本字段（`policy_version/composition_version/mapping_version`）。
3. [ ] 冻结 `composition_version` 计算输入（含 `resolved_setid/as_of/intent`）。
4. [ ] 统一响应中的版本冲突表达与审计记录字段。

### 2.2 非目标
1. [ ] 不引入新业务语义字段。
2. [ ] 不改动 Casbin 授权流程。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 1；里程碑映射：M2/M7。
- 输入依赖：DEV-PLAN-203。
- 后续输出依赖：DEV-PLAN-206/210。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-001`（request_id/trace_id）：DTO 与审计记录保持命名一致。
[ ] `STD-002`：`composition_version` 必须纳入 `resolved_setid + as_of + intent`。
[ ] `STD-011`：版本冲突与 explain 错误提示清晰可追踪。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 定义 DTO 与 JSON schema，补齐字段级来源标记（`source_layer`）。
2. [ ] 实现 explain 生成器，输出命中记录、决议轨迹、最终决策与版本快照。
3. [ ] 实现 `composition_version` 计算器并提供测试向量（同输入同 hash）。
4. [ ] 补充审计记录结构，保证故障排查可复现。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make test`
  - [ ] `make check error-message`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] DTO 字段覆盖 200 第 7 节冻结项。
2. [ ] explain 输出可直接支撑排障与审计。
3. [ ] 版本冲突可稳定复现并定位。

## 8. 风险与缓解
1. [ ] DTO 演进导致前后端漂移。缓解：schema 版本化与兼容测试。
2. [ ] hash 输入遗漏上下文。缓解：固定测试向量覆盖 setid/as_of/intent 变化。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m5-version-consistency-evidence.md`、`docs/dev-records/dev-plan-200-m7-explain-evidence.md`（新增）
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/140-error-message-clarity-and-gates.md`
