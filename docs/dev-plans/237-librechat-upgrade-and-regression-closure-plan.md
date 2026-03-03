# DEV-PLAN-237：LibreChat 升级与回归闭环实施计划

**状态**: 草拟中（2026-03-03 13:50 UTC）

## 1. 背景
- 承接 `DEV-PLAN-230` 的 PR-230-06。
- 目标是让 LibreChat 升级成为“可预期、可回滚、可审计”的例行流程，避免临时补丁式升级。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 固化升级流程：候选评估 -> 集成验证 -> 自动化回归 -> 证据归档 -> 发布。
2. [ ] 固化回滚策略：仅版本回滚，不引入 legacy 双链路。
3. [ ] 固化回归脚本与证据模板。

### 2.2 非目标
1. [ ] 不在本计划扩展新业务能力。
2. [ ] 不以“紧急上线”为由跳过回归门禁。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/233-librechat-single-source-config-convergence-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`

### 3.2 输出
1. [ ] `scripts/librechat/` 升级评估与回归脚本。
2. [ ] `docs/dev-records/` 升级证据模板（版本差异、回归结果、风险评估）。
3. [ ] 升级/回滚 Runbook（包括触发条件与停线标准）。

## 4. 实施步骤（直接落地）
1. [ ] 升级候选评估
   - [ ] 收集上游 changelog、breaking changes、镜像 digest。
   - [ ] 标记对 MCP/Actions/AuthN 边界的潜在影响。
2. [ ] 自动化回归
   - [ ] 必测：assistant-ui 会话边界、旁路写阻断、224/225 契约快照一致性。
   - [ ] 失败即阻断进入发布。
3. [ ] 证据归档
   - [ ] 每次升级输出 `before/after` 差异与结论。
4. [ ] 回滚演练
   - [ ] 验证“仅版本回滚”可恢复；禁止恢复 legacy 路径。

## 5. 验收与门禁
1. [ ] 升级脚本可在本地/CI 重放。
2. [ ] 回归项全部通过后方可发布。
3. [ ] 回滚演练记录完整并归档。
4. [ ] `make check doc` 与 `make e2e` 通过。

## 6. 风险与缓解
1. [ ] 风险：升级回归范围不足。  
   缓解：采用固定“最小必测集”并随事故复盘扩展。
2. [ ] 风险：回滚策略与实际环境脱节。  
   缓解：定期演练并以演练结果更新 runbook。

## 7. SSOT 引用
- `AGENTS.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
