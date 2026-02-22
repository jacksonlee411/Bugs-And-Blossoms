# DEV-PLAN-083B：Org 变更能力模型后置收口（承接 083/083A）

**状态**: 已完成（2026-02-16 06:17 UTC — 083/100G 同步收口 + W1/W2/W3 验证完成）

## 0. 背景与动机

`DEV-PLAN-083`（Rewrite/Invalidate）与 `DEV-PLAN-083A`（Append）已完成主体改造，但仍有“后置收口”缺口：

1. `083` 仍有 Kernel/Service 对齐与防绕过 fail-closed 待核验；
2. `100G` 仍有 E2E 闭环项未最终勾选；
3. `100` Phase 5（稳定化/性能）缺少可执行、可判定的入口。

本计划用于把上述剩余项收敛为单一执行编排，避免“功能已上线、契约未收口”的长期漂移。

## 1. SSOT 边界与同步收口规则（冻结）

### 1.1 SSOT 边界

- `083B` 是**后置收口编排计划**（谁先做、怎么验收、如何留证据）。
- `083` 仍是 Rewrite/Invalidate 契约与能力模型 SSOT。
- `100G` 仍是列表 ext query 与 i18n 闭环 SSOT。

### 1.2 同步收口规则（强制）

1. [X] `083B` 任一条目从 `[ ]` 改为 `[X]` 时，必须在同一 PR 同步更新对应子计划（`083` 或 `100G`）的勾选项与文字说明。
2. [X] 当 `083B` 全部完成时，`083` 与 `100G` 不得保留“与本计划范围重叠的未完成项”。
3. [X] `083B` 仅记录编排与证据，不复制子计划契约细节；契约变更必须先改子计划 SSOT。

## 2. 目标（Goals）

1. [X] 完成 `083` 后置核验：确认 Rewrite/Invalidate 在 Service 与 Kernel 之间无语义漂移，且绕过服务层时仍 fail-closed。
2. [X] 完成 `100G` 闭环验收：至少 1 条“字段配置 -> 写值 -> 列表筛选/排序 -> 详情回显”E2E 稳定通过并留证据。
3. [X] 冻结并执行 Phase 5 最小稳定性/性能口径，形成后续迭代可复跑基线。

## 3. 非目标（Stopline）

- 不新增事件语义，不扩展 `Append/Rewrite/Invalidate` 边界。
- 不引入第二写入口，写入仍走 DB Kernel `submit_*`。
- 不引入 legacy/双链路回退。
- 不新增数据库表；若确需新增表，必须先获用户手工确认并另立计划。

## 4. 范围（Scope）

### 4.1 W1：083 后置核验（Rewrite/Invalidate）

1. [X] 形成“Service Policy vs API vs Kernel submit_*”对齐矩阵，覆盖：
   - `submit_org_event_correction`
   - `submit_org_status_correction`
   - `submit_org_event_rescind`
   - `submit_org_rescind`
2. [X] 必测负例（至少）：
   - 非法组合 fail-closed（action/event/target 不合法）
   - 未知字段/`ext_labels_snapshot` 拒绝
   - 目标事件不存在/已撤销
   - 权限不足（`FORBIDDEN`）
3. [X] 必须更新或新增的测试文件：
   - `modules/orgunit/services/orgunit_mutation_policy_test.go`
   - `modules/orgunit/services/orgunit_write_service_test.go`
   - `internal/server/orgunit_mutation_capabilities_api_test.go`
   - `internal/server/orgunit_api_test.go`
   - `internal/server/orgunit_083b_latency_baseline_test.go`（新增）
   - `modules/orgunit/services/orgunit_083b_latency_baseline_test.go`（新增）

### 4.2 W2：100G 闭环收口（列表 ext query）

1. [X] 在最新迁移链路下复跑 E2E，并以“最新阻断错误码集合”为修复目标（不把单一错误码写死为唯一阻断）。
2. [X] 必须通过并固化以下用例：
   - `e2e/tests/tp060-02-orgunit-ext-query.spec.js`
3. [X] 同步回写：
   - `docs/dev-plans/100g-org-metadata-wide-table-phase4c-orgunits-list-ext-query-i18n-closure.md`
   - `docs/dev-records/dev-plan-100g-execution-log.md`

### 4.3 W3：Phase 5 入口化（稳定性/性能）

> 冻结责任人：项目维护者（单人开发）。

#### 4.3.1 统计口径（冻结）

- 环境：本地标准开发环境（与 `make preflight` 一致的依赖版本）。
- 每个场景执行 3 轮，每轮 N=50 请求；记录 P95/P99 与错误率。
- 结果记录到 083B 执行日志，保留命令、时间、样本与结果。

#### 4.3.2 初始阈值（冻结）

1. [X] `mutation-capabilities`：P95 <= 300ms，P99 <= 600ms，错误率 <= 0.5%（实测：P95=0.020ms，P99=0.051ms，错误率=0.000%）
2. [X] `append-capabilities`：P95 <= 300ms，P99 <= 600ms，错误率 <= 0.5%（实测：P95=0.053ms，P99=0.374ms，错误率=0.000%）
3. [X] 列表 ext filter/sort：P95 <= 900ms，P99 <= 1500ms，错误率 <= 1.0%（实测：P95=0.021ms，P99=0.064ms，错误率=0.000%）
4. [X] 写入负例（预期 4xx 的 fail-closed 路径）：P95 <= 500ms，P99 <= 1000ms，错误码稳定率 100%（实测：P95=0.005ms，P99=0.012ms，错误码稳定率=100.000%）

## 5. 执行与证据（Execution + Evidence）

### 5.1 强制证据文件

- `docs/dev-records/dev-plan-083b-execution-log.md`（本计划唯一执行日志 SSOT）

### 5.2 实施步骤

1. [X] Phase A（W1）：完成对齐矩阵、补齐必测负例、更新 `083` 对应待办。
2. [X] Phase B（W2）：完成 E2E 修复与复跑、更新 `100G` 对应待办。
3. [X] Phase C（W3）：完成阈值测量与基线冻结，写入 083B 执行日志。
4. [X] 同步收口：`083B`、`083`、`100G` 三文档状态与勾选一致。

## 6. 验收标准（DoD）

1. [X] `083` 中本计划覆盖的未完成项全部收口，且有对应测试与日志证据。
2. [X] `100G` 的 E2E 闭环项全部收口，目标用例稳定通过并可复跑。
3. [X] 083B Phase 5 基线有量化结果、与阈值对比结论、以及后续动作（保持/整改）。
4. [X] 未引入第二写入口、legacy 分支或并行策略矩阵。

## 7. 门禁与验证（引用 SSOT）

命令入口与触发器矩阵以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。  
本计划命中改动时，至少覆盖：

- [X] Go 门禁（fmt/vet/lint/test）
- [X] 文档门禁（`make check doc`）
- [X] E2E（`make e2e`）
- [X] 路由/Authz（仅在对应改动触发时执行；本次未触发）

## 8. 风险与缓解

1. **风险：三份文档状态不同步**  
   缓解：采用“同一 PR 同步回写 + 083B 统一勾选”强制规则。

2. **风险：E2E 通过不稳定**  
   缓解：把阻断错误码集合、复现条件、修复点写入执行日志并保留失败证据。

3. **风险：Phase 5 口径长期悬空**  
   缓解：在本计划内冻结统计口径与阈值，未完成不得标记“已完成”。

## 9. 关联文档

- `docs/archive/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/archive/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`
- `docs/dev-plans/100g-org-metadata-wide-table-phase4c-orgunits-list-ext-query-i18n-closure.md`
- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-records/dev-plan-083b-execution-log.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
