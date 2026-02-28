# DEV-PLAN-208：200蓝图 Phase 3 Req2Config 只读编排与严格结构化输出

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
该计划把 AI 编排限制在“只读计划与 dry-run”，并以 strict schema decode 阻断幻觉与越权产物。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 落地 `RequirementIntentSpec -> ConfigDeltaPlan -> DryRunResult` 只读闭环。
2. [ ] 启用 strict constrained decode，非法结构在 lint 前即拒绝。
3. [ ] 建立静态 lint 规则：禁止 SQL、禁止未注册 capability、禁止越界字段。
4. [ ] 建立 risk_tier 初版分类但不进入写提交。

### 2.2 非目标
1. [ ] 不落地实际写提交。
2. [ ] 不将 AI 作为独立授权主体。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 3；里程碑映射：M9/M9A。
- 输入依赖：DEV-PLAN-204/206。
- 后续输出依赖：DEV-PLAN-210。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-001`：Req2Config 产物与审计统一 `request_id/trace_id` 命名。
[ ] `STD-004`：AI 仅只读规划，不引入旁路提交。
[ ] `STD-011`：schema 违约、边界违约错误码清晰稳定。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 定义输入输出 schema 与校验器（strict=true）。
2. [ ] 实现 plan 生成 + static lint + dry-run compose 串联链路。
3. [ ] 实现非法输出拒绝路径与稳定错误码。
4. [ ] 补齐样本集与误放行/误拒绝统计基线。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make test`
  - [ ] `make check error-message`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] AI 无任何直写数据库能力。
2. [ ] schema 违约产物稳定拒绝且可解释。
3. [ ] M9/M9A 证据完整可追溯。

## 8. 风险与缓解
1. [ ] schema 过宽导致漏拦截。缓解：最小字段集 + additionalProperties=false。
2. [ ] dry-run 与运行时语义偏差。缓解：复用同一决议器。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m9-ai-plan-boundary-evidence.md`、`docs/dev-records/dev-plan-200-m9a-ai-constrained-decode-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
