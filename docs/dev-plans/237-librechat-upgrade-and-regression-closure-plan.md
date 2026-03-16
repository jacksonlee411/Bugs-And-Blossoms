# DEV-PLAN-237：LibreChat 升级与回归闭环实施计划

**状态**: 长期维护（2026-03-17 02:48 CST 修订；`DEV-PLAN-291` 已完成 `237` 进入 `DEV-PLAN-285` 所需的升级兼容回归前置，`DEV-PLAN-285` 已于 2026-03-09 完成封板；本文继续保留为 LibreChat 后续版本升级、回归、回滚与证据归档的长期契约）

> 口径说明：本文未勾选条目表示每一轮 LibreChat upstream runtime / vendored UI source / patch stack 变更都必须重复满足的持续门禁，不表示当前主线仍停留在“草拟待启动”。

## 1. 背景
- 承接 `DEV-PLAN-230` 的 PR-230-06。
- 目标是让 LibreChat 升级成为“可预期、可回滚、可审计”的例行流程，避免临时补丁式升级。

## 1.1 当前角色（2026-03-17 冻结）
1. [X] `237` 主计划正文继续保留为长期升级与回归闭环契约，不因单轮主线封板而废弃。
2. [X] 本轮主线中，`237` 进入 `285` 的可执行前置已由 `291` 具体化并完成证据收敛。
3. [X] 后续若发生 LibreChat upstream runtime、vendored UI 来源版本或 patch stack 的影响性合入，应继续按本文约束与 `291` 证据模板重跑，而不是恢复旧入口、旧桥接链路或双链路回滚。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 固化升级流程：候选评估 -> 集成验证 -> 自动化回归 -> 证据归档 -> 发布。
2. [ ] 固化回滚策略：仅版本回滚，不引入 legacy 双链路，不为旧 UI 入口或旧桥接链路保留回滚特权。
3. [ ] 固化回归脚本与证据模板。
4. [ ] 将 vendored LibreChat Web UI 来源元数据、patch stack、source/runtime compatibility 纳入升级闭环。

### 2.2 非目标
1. [ ] 不在本计划扩展新业务能力。
2. [ ] 不以“紧急上线”为由跳过回归门禁。
3. [ ] 不以“升级风险较高”为由恢复旧正式入口、旧桥接链路或双入口并存。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/233-librechat-single-source-config-convergence-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`

### 3.2 输出
1. [ ] `scripts/librechat/` 升级评估与回归脚本。
2. [ ] `scripts/librechat-web/` 或等价目录下的 vendored UI 来源校验、patch 回放、构建与兼容性回归脚本。
3. [ ] 旧入口/旧桥接残留检查脚本：确认无双入口、无双回执、无旧桥正式职责回流。
4. [ ] `docs/dev-records/` 升级证据模板（版本差异、回归结果、风险评估）。
5. [ ] 升级/回滚 Runbook（包括触发条件与停线标准）。

## 4. 实施步骤（直接落地）
1. [ ] 升级候选评估
   - [ ] 收集上游 changelog、breaking changes、镜像 digest。
   - [ ] 收集 vendored UI 来源版本、patch 清单、潜在冲突点。
   - [ ] 标记对 MCP/Actions/AuthN 边界的潜在影响。
2. [ ] 自动化回归
   - [ ] 必测：LibreChat UI 正式入口/历史别名入口的会话边界、旁路写阻断、223/260/266 契约一致性。
   - [ ] 必测：vendored UI source + patch stack + runtime compatibility。
   - [ ] 必测：不存在旧正式入口回流、旧桥接职责回流、双消息落点回流。
   - [ ] 失败即阻断进入发布。
3. [ ] 证据归档
   - [ ] 每次升级输出 `before/after` 差异与结论。
   - [ ] 证据至少包含 runtime 版本差异、vendored UI 来源差异、patch 变更与回归结论。
4. [ ] 回滚演练
   - [ ] 验证“仅版本回滚”可恢复；禁止恢复 legacy 路径。
   - [ ] 若涉及 vendored UI 变更，必须验证来源基线 + patch stack 的回退可重放。
   - [ ] 回滚只允许恢复到上一版新主链路，不允许借回滚名义重新启用旧正式入口或旧桥接体系。

## 5. 验收与门禁
1. [ ] 升级脚本可在本地/CI 重放。
2. [ ] 回归项全部通过后方可发布。
3. [ ] vendored UI 来源元数据、patch stack 与 runtime 版本三者均可审计、可回放。
4. [ ] 发布时不存在旧正式入口、旧桥接职责或双入口并存。
5. [ ] 回滚演练记录完整并归档。
6. [ ] `make check doc` 与 `make e2e` 通过。

## 6. 风险与缓解
1. [ ] 风险：升级回归范围不足。  
   缓解：采用固定“最小必测集”并随事故复盘扩展。
2. [ ] 风险：团队以“升级求稳”为名恢复旧入口或双链路。  
   缓解：把“无旧正式入口回流”纳入升级必测项与发布 stopline。
3. [ ] 风险：回滚策略与实际环境脱节。  
   缓解：定期演练并以演练结果更新 runbook。

## 7. SSOT 引用
- `AGENTS.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
